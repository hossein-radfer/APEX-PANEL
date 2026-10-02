package service

import (
	"errors"
	"fmt"
	"sync"
	"time"

	"go.uber.org/zap"
	"gorm.io/gorm"
	"gorm.io/gorm/clause"

	"github.com/maahdima/mwp/api/common"
	"github.com/maahdima/mwp/api/dataservice/model"
	"github.com/maahdima/mwp/api/http/schema"
	"github.com/maahdima/mwp/api/utils"
	"github.com/maahdima/mwp/api/utils/timehelper"
)

// ApplicationService owns the "Application" bundle -- a single mobile-app
// end-user identity that fans out to a peer/account/package on every
// WireGuard interface, User Manager group, and x-ui panel it's been
// granted. It orchestrates the three existing per-protocol services
// (WgPeer/UserManagerService/V2RayPackageService) by calling their own
// public CreatePeer/CreateAccount/CreatePackage methods directly, exactly
// like V2RayPackageService.BulkCreatePackages calls CreatePackage in a
// loop -- it deliberately never duplicates any of their provisioning
// logic (Mikrotik/x-ui calls, scheduler/queue objects, RouterOS profile
// activation, etc).
type ApplicationService struct {
	db             *gorm.DB
	peers          *WgPeer
	userMgr        *UserManagerService
	v2ray          *V2RayPackageService
	openVpnTmpl    *ApplicationOpenVpnTemplateService
	auditLog       *AuditLog
	billing        *ResellerBillingService
	plans          *ApplicationPlanService
	licenseLimiter freeTierLimiter
	logger         *zap.Logger
}

func NewApplicationService(db *gorm.DB, peers *WgPeer, userMgr *UserManagerService, v2ray *V2RayPackageService, openVpnTmpl *ApplicationOpenVpnTemplateService, auditLog *AuditLog) *ApplicationService {
	return &ApplicationService{
		db:          db,
		peers:       peers,
		userMgr:     userMgr,
		v2ray:       v2ray,
		openVpnTmpl: openVpnTmpl,
		auditLog:    auditLog,
		logger:      zap.L().Named("ApplicationService"),
	}
}

// SetLicenseLimiter wires the free-tier cap source after construction --
// mirrors V2RayPackageService.SetLicenseLimiter exactly. Safe to leave
// unset (nil licenseLimiter disables enforcement entirely).
func (s *ApplicationService) SetLicenseLimiter(limiter freeTierLimiter) {
	s.licenseLimiter = limiter
}

// ErrFreeTierApplicationLimitReached is returned by CreateApplication when
// the license is Restricted and the configured "max_applications" cap has
// already been reached.
var ErrFreeTierApplicationLimitReached = errors.New("free tier limit reached: maximum applications")

// SuspendOldestForFreeTier mirrors WgPeer.SuspendOldestForFreeTier exactly
// (see its own doc comment for the full DB-only, external-call-deferred
// rationale) -- the oldest excess Application rows (by CreatedAt) beyond
// cap are force-suspended at the DB level via the same
// Status/SuspendedByQuota/WasActiveBeforeSuspend columns this feature's
// existing quota-suspend paths already write, so the periodic quota job
// reconciles the actual underlying WireGuard/UserManager/V2Ray resource
// state on its own next tick.
func (s *ApplicationService) SuspendOldestForFreeTier(cap int64) (int, error) {
	if cap <= 0 {
		return 0, nil
	}

	var activeCount int64
	if err := s.db.Model(&model.Application{}).Where("status = ?", "active").Count(&activeCount).Error; err != nil {
		return 0, err
	}

	excess := activeCount - cap
	if excess <= 0 {
		return 0, nil
	}

	var toSuspend []model.Application
	if err := s.db.Where("status = ?", "active").Order("created_at ASC").Limit(int(excess)).Find(&toSuspend).Error; err != nil {
		return 0, err
	}

	suspended := 0
	for _, app := range toSuspend {
		if err := s.db.Model(&model.Application{}).Where("id = ?", app.ID).Updates(map[string]interface{}{
			"status":                    "suspended",
			"suspended_by_quota":        true,
			"was_active_before_suspend": true,
		}).Error; err != nil {
			s.logger.Error("failed to suspend application for free-tier cap", zap.Uint("application_id", app.ID), zap.Error(err))
			continue
		}
		suspended++
	}

	return suspended, nil
}

// SetBilling wires the same Payment-based (wallet-debit-per-GB) billing path
// WireGuard/User Manager/V2Ray already use into Applications -- optional,
// nil-safe collaborator (mirrors Reseller.SetWallet/SetV2RayResumer's own
// established pattern) rather than a constructor parameter, since
// ApplicationService and ResellerBillingService would otherwise need a
// construction-order dependency neither currently has on the other.
func (s *ApplicationService) SetBilling(billing *ResellerBillingService) {
	s.billing = billing
}

// SetPlanService wires the Application tier catalog in -- optional,
// nil-safe collaborator mirroring DNSAccountService.SetPlanService exactly
// (same construction-order rationale: ApplicationService and
// ApplicationPlanService would otherwise need a dependency neither
// currently has on the other).
func (s *ApplicationService) SetPlanService(plans *ApplicationPlanService) {
	s.plans = plans
}

// applyPlanIfSet, when planID is non-nil and s.plans is wired, fetches that
// ApplicationPlan and copies its bundle into the given request pointers --
// but ONLY into fields the caller left at their zero value, so anything the
// admin/reseller explicitly typed in the same request always wins (manual
// override). Mirrors DNSAccountService.applyPlanIfSet's own doc comment
// exactly, including the "missing/stale plan reference never blocks
// creation" behavior.
func (s *ApplicationService) applyPlanIfSet(planID *uint, totalVolumeBytes *int64, durationDays, maxOnlineUsers *int, downloadSpeedLimitMbps, uploadSpeedLimitMbps **int) {
	if planID == nil || s.plans == nil {
		return
	}
	plan, err := s.plans.GetPlan(*planID)
	if err != nil {
		s.logger.Warn("application plan referenced by request not found, ignoring", zap.Uint("plan_id", *planID), zap.Error(err))
		return
	}
	if totalVolumeBytes != nil && *totalVolumeBytes == 0 {
		*totalVolumeBytes = plan.TotalVolumeBytes
	}
	if durationDays != nil && *durationDays == 0 {
		*durationDays = plan.DurationDays
	}
	if maxOnlineUsers != nil && *maxOnlineUsers == 0 {
		*maxOnlineUsers = plan.MaxOnlineUsers
	}
	if downloadSpeedLimitMbps != nil && *downloadSpeedLimitMbps == nil {
		*downloadSpeedLimitMbps = plan.DownloadSpeedLimitMbps
	}
	if uploadSpeedLimitMbps != nil && *uploadSpeedLimitMbps == nil {
		*uploadSpeedLimitMbps = plan.UploadSpeedLimitMbps
	}
}

func (s *ApplicationService) logResellerAction(resellerID *uint, action, description string) {
	if resellerID == nil || s.auditLog == nil {
		return
	}
	var reseller model.Reseller
	name := "unknown"
	if err := s.db.Select("name").First(&reseller, *resellerID).Error; err == nil {
		name = reseller.Name
	}
	s.auditLog.Log(*resellerID, name, action, description)
}

// ensureResellerCanCreateApplications mirrors UserManagerService.
// ensureResellerCanCreateUserManagerAccounts / V2RayPackageService.
// ensureResellerCanResellV2Ray exactly -- the top-level on/off gate for
// the Applications product, admin-configured via Reseller.
// CanCreateApplications (see that field's own doc comment for why
// Applications has no separate quota-bytes pool alongside this gate).
func (s *ApplicationService) ensureResellerCanCreateApplications(resellerID uint) error {
	var reseller model.Reseller
	if err := s.db.First(&reseller, resellerID).Error; err != nil {
		return err
	}
	if !reseller.CanCreateApplications {
		return fmt.Errorf("reseller is not permitted to create applications")
	}
	return nil
}

// ensureResellerUnderApplicationLimit mirrors V2RayPackageService.
// ensureResellerUnderV2RayPackageLimit exactly, enforcing
// Reseller.ApplicationMaxCount (see that field's own doc comment --
// Applications otherwise reuse Reseller.MaxPeers-style count gating is
// deliberately NOT introduced beyond this single count ceiling; a
// reseller's ability to create Applications is also governed by which
// interfaces/groups/panels they've already been granted
// (ResellerInterface/ResellerUserManagerGroup/ResellerXuiPanelAccess),
// since an Application's real cost is the underlying resources it
// requests, which are already capped by each protocol's own existing
// per-reseller limit (MaxPeers/UserManagerMaxAccounts/V2RayMaxPackages).
func (s *ApplicationService) ensureResellerUnderApplicationLimit(resellerID uint) error {
	var reseller model.Reseller
	if err := s.db.First(&reseller, resellerID).Error; err != nil {
		return err
	}
	if reseller.ApplicationMaxCount == nil {
		return nil
	}

	var count int64
	if err := s.db.Model(&model.Application{}).Where("reseller_id = ?", resellerID).Count(&count).Error; err != nil {
		return err
	}
	if int(count) >= *reseller.ApplicationMaxCount {
		return fmt.Errorf("reseller has reached its maximum allowed applications (%d)", *reseller.ApplicationMaxCount)
	}
	return nil
}

func (s *ApplicationService) ensureResellerCanUseInterface(resellerID uint, interfaceID uint) error {
	var count int64
	if err := s.db.Model(&model.ResellerInterface{}).
		Where("reseller_id = ? AND interface_id = ?", resellerID, interfaceID).
		Count(&count).Error; err != nil {
		return err
	}
	if count == 0 {
		return fmt.Errorf("reseller is not permitted to use interface id %d", interfaceID)
	}
	return nil
}

func (s *ApplicationService) ensureResellerCanUsePanel(resellerID uint, panelID uint) error {
	var count int64
	if err := s.db.Model(&model.ResellerXuiPanelAccess{}).
		Where("reseller_id = ? AND panel_id = ?", resellerID, panelID).
		Count(&count).Error; err != nil {
		return err
	}
	if count == 0 {
		return fmt.Errorf("reseller is not permitted to use panel id %d", panelID)
	}
	return nil
}

func (s *ApplicationService) ensureUsernameIsUnique(username string) error {
	var existing model.Application
	if err := s.db.Where("app_username = ?", username).First(&existing).Error; err == nil {
		return fmt.Errorf("app username %s is already in use", username)
	} else if !errors.Is(err, gorm.ErrRecordNotFound) {
		return err
	}
	return nil
}

// randomAppCredential generates a 5-character random value, retrying on
// (astronomically unlikely) collision -- mirrors generateBulkPackageLabel's
// own "guarantee outright rather than leave to chance" stance, but for a
// LOGIN credential (where a collision would actually break something,
// unlike a display label), so this one actually checks uniqueness against
// the DB instead of merely disambiguating within one batch.
func (s *ApplicationService) randomUniqueAppUsername() (string, error) {
	for i := 0; i < 20; i++ {
		candidate := utils.RandomString(5)
		if err := s.ensureUsernameIsUnique(candidate); err == nil {
			return candidate, nil
		}
	}
	return "", fmt.Errorf("failed to generate a unique app username after 20 attempts")
}

// CreateApplication provisions a peer/account/package on every requested
// WireGuard interface / User Manager group / x-ui panel, all owned by one
// new Application row carrying its own separate mobile-app login
// credential. Each protocol's own existing service does the real
// provisioning work (Mikrotik/x-ui calls, DB writes) -- this method only
// resolves permissions, generates the Application's identity, and loops.
//
// Mirrors BulkCreatePackages' error contract: if a specific resource
// fails partway through, already-created resources are kept (not rolled
// back -- the underlying Mikrotik/x-ui objects would need their own
// teardown calls to undo, which is unnecessary complexity for what the
// admin can already fix by hand via the ordinary Peers/User Manager/
// V2Ray Packages pages), and the error names exactly which resource
// failed so the admin knows what to retry.
func (s *ApplicationService) CreateApplication(req *schema.CreateApplicationRequest, resellerID *uint) (*schema.ApplicationResponse, error) {
	if s.licenseLimiter != nil {
		if maxApps, ok := s.licenseLimiter.GetFreeTierLimit("max_applications"); ok {
			var count int64
			if err := s.db.Model(&model.Application{}).Count(&count).Error; err != nil {
				return nil, err
			}
			if count >= maxApps {
				return nil, ErrFreeTierApplicationLimitReached
			}
		}
	}

	s.applyPlanIfSet(req.ApplicationPlanID, &req.TotalVolumeBytes, &req.DurationDays, &req.MaxOnlineUsers, &req.DownloadSpeedLimitMbps, &req.UploadSpeedLimitMbps)

	if resellerID != nil {
		if err := s.ensureResellerCanCreateApplications(*resellerID); err != nil {
			return nil, err
		}
		if err := s.ensureResellerUnderApplicationLimit(*resellerID); err != nil {
			return nil, err
		}
		for _, ifaceID := range req.InterfaceIDs {
			if err := s.ensureResellerCanUseInterface(*resellerID, ifaceID); err != nil {
				return nil, err
			}
		}
		for _, panelID := range req.XuiPanelIDs {
			if err := s.ensureResellerCanUsePanel(*resellerID, panelID); err != nil {
				return nil, err
			}
		}
		for _, g := range req.UserManagerGroups {
			if err := s.userMgr.ensureResellerCanUseGroup(*resellerID, g.GroupName); err != nil {
				return nil, err
			}
			if err := s.userMgr.ensureResellerCanUseProfile(*resellerID, g.ProfileName); err != nil {
				return nil, err
			}
		}
	}

	appUsername := ""
	if req.AppUsername != nil && *req.AppUsername != "" {
		if err := s.ensureUsernameIsUnique(*req.AppUsername); err != nil {
			return nil, err
		}
		appUsername = *req.AppUsername
	} else {
		generated, err := s.randomUniqueAppUsername()
		if err != nil {
			return nil, err
		}
		appUsername = generated
	}

	appPassword := ""
	if req.AppPassword != nil && *req.AppPassword != "" {
		appPassword = *req.AppPassword
	} else {
		appPassword = utils.RandomString(5)
	}

	now := time.Now()
	expireAt := now.AddDate(0, 0, req.DurationDays)

	app := model.Application{
		ResellerID:             resellerID,
		Name:                   req.Name,
		ApplicationPlanID:      req.ApplicationPlanID,
		AppUsername:            appUsername,
		AppPassword:            appPassword,
		TotalVolumeBytes:       req.TotalVolumeBytes,
		DurationDays:           req.DurationDays,
		StartAt:                &now,
		ExpireAt:               &expireAt,
		MaxOnlineUsers:         req.MaxOnlineUsers,
		Status:                 "active",
		DownloadSpeedLimitMbps: req.DownloadSpeedLimitMbps,
		UploadSpeedLimitMbps:   req.UploadSpeedLimitMbps,
	}
	if err := s.db.Create(&app).Error; err != nil {
		s.logger.Error("failed to create application", zap.Error(err))
		return nil, fmt.Errorf("failed to create application: %w", err)
	}

	var server model.Server
	hasServer := s.db.First(&server).Error == nil

	// Each resource is provisioned best-effort -- one failure (e.g. an
	// IP-pool collision on a single WireGuard interface) must never
	// abort the OTHER requested interfaces, let alone the entirely
	// separate UserManager/V2Ray protocol loops below. Confirmed,
	// reported bug this fixes: a mid-loop provisionInterface failure
	// previously did `return s.buildResponse(app)` immediately, silently
	// skipping every UserManagerGroups/XuiPanelIDs entry the admin had
	// also selected -- an Application created with all three protocols
	// checked ended up with only whichever WireGuard interfaces
	// succeeded before the first failure, and nothing else. Matches this
	// codebase's own established "one dead resource never blocks the
	// rest of the batch" convention (see e.g. GetConnectConfigs' own doc
	// comment). Every failure is still logged and surfaced to the admin
	// via provisioningErrors so partial success is visible, not silent.
	//
	// All three loops -- and every item within each loop -- run
	// CONCURRENTLY, not sequentially. Confirmed, reported bug this fixes:
	// provisioning a UserManager account takes 4 sequential MikroTik round
	// trips (CreateUserManagerUser/SetUserManagerUserGroup/
	// CreateUserManagerUserProfile/ActivateUserManagerUserProfile), each
	// its own several-second RouterOS call. Running the old sequential
	// for-loops meant an Application created with WireGuard+UserManager+
	// V2Ray all checked summed every one of those round trips into a
	// single request -- reproduced live at 30+ seconds, past the admin
	// panel's own 30s axios timeout, so the browser cancelled the request
	// before the server finished, then a same-name-conflict retry 500'd
	// immediately. WireGuard-only creation stayed fast (far fewer round
	// trips) and never surfaced this, which is why the bug looked like
	// "only WireGuard ever gets created" rather than what it actually was.
	// Mirrors CreatePackage's own identical fix for its multi-panel
	// fan-out (see that function's own doc comment) -- bounding total wall
	// time to the SLOWEST single resource, not their sum. Every goroutine
	// writes only to its own pre-sized result slot (never a shared
	// variable), so no mutex is needed; all app.ID-scoped DB writes still
	// happen serially afterward on the calling goroutine, matching
	// CreatePackage's own "network calls run in parallel, DB writes stay
	// sequential" split.
	var provisioningErrors []string
	var errMu sync.Mutex
	addProvisioningError := func(msg string) {
		errMu.Lock()
		defer errMu.Unlock()
		provisioningErrors = append(provisioningErrors, msg)
	}

	var wg sync.WaitGroup

	for _, ifaceID := range req.InterfaceIDs {
		wg.Add(1)
		go func(ifaceID uint) {
			defer wg.Done()
			if err := s.provisionInterface(&app, ifaceID, hasServer, server); err != nil {
				s.logger.Error("failed to provision application peer, continuing with remaining resources", zap.Uint("application_id", app.ID), zap.Uint("interface_id", ifaceID), zap.Error(err))
				addProvisioningError(fmt.Sprintf("وایرگارد (%d): %v", ifaceID, err))
			}
		}(ifaceID)
	}

	for _, g := range req.UserManagerGroups {
		wg.Add(1)
		go func(g schema.ApplicationGroupProfile) {
			defer wg.Done()
			if err := s.provisionUserManagerGroup(&app, g); err != nil {
				s.logger.Error("failed to provision application user manager account, continuing with remaining resources", zap.Uint("application_id", app.ID), zap.String("group", g.GroupName), zap.Error(err))
				addProvisioningError(fmt.Sprintf("یوزرمنجیر (%s): %v", g.GroupName, err))
			}
		}(g)
	}

	for _, panelID := range req.XuiPanelIDs {
		wg.Add(1)
		go func(panelID uint) {
			defer wg.Done()
			if err := s.provisionXuiPanel(&app, panelID); err != nil {
				s.logger.Error("failed to provision application v2ray package, continuing with remaining resources", zap.Uint("application_id", app.ID), zap.Uint("panel_id", panelID), zap.Error(err))
				addProvisioningError(fmt.Sprintf("V2Ray (%d): %v", panelID, err))
			}
		}(panelID)
	}

	wg.Wait()

	if len(provisioningErrors) > 0 {
		s.logger.Warn("application created with partial provisioning failures", zap.Uint("application_id", app.ID), zap.Strings("errors", provisioningErrors))
	}

	s.logResellerAction(resellerID, AuditActionApplicationCreated, fmt.Sprintf(
		"ساخت اپلیکیشن «%s» | یوزرنیم %s | %.1f گیگابایت | %d روز",
		app.Name, app.AppUsername, float64(app.TotalVolumeBytes)/(1024*1024*1024), app.DurationDays,
	))

	resp, err := s.buildResponse(app)
	if err != nil {
		return nil, err
	}
	resp.ProvisioningErrors = provisioningErrors
	return resp, nil
}

// provisionInterface creates one WireGuard peer for this Application on
// the given interface, reusing WgPeer's own key generation
// (GetPeerCredentials) and free-address resolution (GetNewPeerAllowedAddress)
// exactly the way the ordinary peer-creation form does, then calls
// CreatePeer with them.
func (s *ApplicationService) provisionInterface(app *model.Application, interfaceID uint, hasServer bool, server model.Server) error {
	if !hasServer {
		return fmt.Errorf("no mikrotik server configured")
	}

	var iface model.Interface
	if err := s.db.First(&iface, interfaceID).Error; err != nil {
		return fmt.Errorf("interface %d not found: %w", interfaceID, err)
	}

	creds, err := s.peers.GetPeerCredentials()
	if err != nil {
		return fmt.Errorf("failed to generate wireguard keypair: %w", err)
	}

	addr, err := s.peers.GetNewPeerAllowedAddress(interfaceID, app.ResellerID)
	if err != nil {
		return fmt.Errorf("failed to resolve a free address on interface %d: %w", interfaceID, err)
	}
	if addr.AllowedAddress == "" {
		return fmt.Errorf("interface %d has no IP pool configured", interfaceID)
	}

	// Naming order is app_username FIRST, then a 4-char random suffix --
	// confirmed, reported requirement: an admin searching the panel by
	// this Application's own username/name should find its underlying
	// peers with a simple prefix match, not have to know a random
	// prefix first.
	peerName := fmt.Sprintf("%s_%s", app.AppUsername, utils.RandomString(4))
	resp, err := s.peers.CreatePeer(&schema.CreatePeerRequest{
		Name:                peerName,
		InterfaceId:         interfaceID,
		PrivateKey:          creds.PrivateKey,
		PublicKey:           creds.PublicKey,
		AllowedAddress:      addr.AllowedAddress,
		Endpoint:            server.IPAddress,
		PersistentKeepAlive: utils.Ptr("00:00:25"),
		ExpireTime:          applicationExpireTimeString(app),
		TrafficLimit:        nil, // enforced in aggregate by EnforceApplicationQuotas, not per-peer
	}, app.ResellerID)
	if err != nil {
		return err
	}

	peerID := resp.Id
	return s.db.Create(&model.ApplicationInterface{
		ApplicationID: app.ID,
		InterfaceID:   interfaceID,
		PeerID:        &peerID,
	}).Error
}

// provisionUserManagerGroup creates one User Manager account for this
// Application in the given group/profile, pushing MaxOnlineUsers straight
// onto SharedUsers -- the admin's own explicit "شیر یوزر" requirement
// (RouterOS enforces this natively, unlike WireGuard/V2Ray, see
// AppOnlineCountResponse's own doc comment).
func (s *ApplicationService) provisionUserManagerGroup(app *model.Application, g schema.ApplicationGroupProfile) error {
	// Same naming order as provisionInterface -- app_username first, then
	// a 4-char random suffix.
	username := fmt.Sprintf("%s_%s", app.AppUsername, utils.RandomString(4))
	resp, err := s.userMgr.CreateAccount(&schema.CreateUserManagerAccountRequest{
		Username:    username,
		Password:    utils.RandomString(12),
		Group:       g.GroupName,
		Profile:     g.ProfileName,
		Protocols:   []string{"l2tp", "pptp", "sstp", "openvpn"},
		SharedUsers: &app.MaxOnlineUsers,
		ExpireTime:  applicationExpireTimeString(app),
	}, app.ResellerID)
	if err != nil {
		return err
	}

	accountID := resp.Id
	return s.db.Create(&model.ApplicationUserManagerGroup{
		ApplicationID: app.ID,
		GroupName:     g.GroupName,
		AccountID:     &accountID,
	}).Error
}

// provisionXuiPanel creates one V2Ray package for this Application,
// scoped to exactly one panel -- the admin's own explicit "داخل هر پنل
// جدا ساخته بشه" requirement (one full package per allowed panel, not
// one package fanned out across all of them the way an ordinary V2Ray
// Packages page create does).
func (s *ApplicationService) provisionXuiPanel(app *model.Application, panelID uint) error {
	// Same naming order as provisionInterface -- app_username first, then
	// a 4-char random suffix.
	label := fmt.Sprintf("%s_%s", app.AppUsername, utils.RandomString(4))
	resp, err := s.v2ray.CreatePackage(&schema.CreateV2RayPackageRequest{
		CustomerLabel:    &label,
		TotalVolumeBytes: app.TotalVolumeBytes,
		DurationDays:     app.DurationDays,
		PanelIDs:         []uint{panelID},
	}, app.ResellerID)
	if err != nil {
		return err
	}

	packageID := resp.Id
	return s.db.Create(&model.ApplicationXuiPanel{
		ApplicationID: app.ID,
		PanelID:       panelID,
		PackageID:     &packageID,
	}).Error
}

// reconcileApplicationInterfaces replaces app's full set of WireGuard
// interfaces with requestedInterfaceIDs -- see UpdateApplication's own
// call site doc comment for the reported gap this fixes. Newly-requested
// interfaces are provisioned via provisionInterface (CreateApplication's
// own per-interface logic, unchanged), de-selected ones have their peer
// deleted via WgPeer.DeletePeer (DeleteApplication's own per-resource
// cleanup, unchanged) and their join-table row removed; an interface ID
// present both before and after this call is left completely untouched
// (no pointless peer delete+recreate on a resource that isn't actually
// changing -- doing so would rotate the peer's keys/address and break
// any client already configured with it).
func (s *ApplicationService) reconcileApplicationInterfaces(app *model.Application, requestedInterfaceIDs []uint, resellerID *uint) error {
	if resellerID != nil {
		for _, ifaceID := range requestedInterfaceIDs {
			if err := s.ensureResellerCanUseInterface(*resellerID, ifaceID); err != nil {
				return err
			}
		}
	}

	var existing []model.ApplicationInterface
	if err := s.db.Where("application_id = ?", app.ID).Find(&existing).Error; err != nil {
		s.logger.Error("failed to list application interfaces for reconcile", zap.Uint("application_id", app.ID), zap.Error(err))
		return fmt.Errorf("failed to update application interfaces: %w", err)
	}
	existingByIface := make(map[uint]model.ApplicationInterface, len(existing))
	for _, link := range existing {
		existingByIface[link.InterfaceID] = link
	}
	wanted := make(map[uint]bool, len(requestedInterfaceIDs))
	for _, id := range requestedInterfaceIDs {
		wanted[id] = true
	}

	var server model.Server
	hasServer := s.db.First(&server).Error == nil

	var wg sync.WaitGroup
	for _, ifaceID := range requestedInterfaceIDs {
		if _, already := existingByIface[ifaceID]; already {
			continue
		}
		wg.Add(1)
		go func(ifaceID uint) {
			defer wg.Done()
			if err := s.provisionInterface(app, ifaceID, hasServer, server); err != nil {
				s.logger.Error("failed to provision application peer during edit, continuing with remaining resources", zap.Uint("application_id", app.ID), zap.Uint("interface_id", ifaceID), zap.Error(err))
			}
		}(ifaceID)
	}
	for _, link := range existing {
		if wanted[link.InterfaceID] {
			continue
		}
		wg.Add(1)
		go func(link model.ApplicationInterface) {
			defer wg.Done()
			if link.PeerID != nil {
				if err := s.peers.DeletePeer(*link.PeerID, app.ResellerID); err != nil {
					s.logger.Warn("failed to delete application peer during edit, continuing", zap.Uint("peer_id", *link.PeerID), zap.Error(err))
				}
			}
			if err := s.db.Unscoped().Delete(&link).Error; err != nil {
				s.logger.Error("failed to delete application interface link during edit", zap.Uint("application_id", app.ID), zap.Uint("interface_id", link.InterfaceID), zap.Error(err))
			}
		}(link)
	}
	wg.Wait()

	return nil
}

// reconcileApplicationUserManagerGroups mirrors
// reconcileApplicationInterfaces exactly, swapping in User Manager
// group+profile pairs -- matched by GroupName (an Application has at
// most one account per group, matching CreateApplication's own
// provisionUserManagerGroup, which never allows more than one). Changing
// ONLY the ProfileName for a group that's already present is treated as
// remove-then-add (a fresh account under the new profile) rather than an
// in-place profile swap, since ApplicationUserManagerGroup carries no
// "update profile" operation of its own -- this matches
// UserManagerService.UpdateAccount's own group/profile change (which
// also removes the old RouterOS profile link and creates a new one, not
// a true in-place mutation).
func (s *ApplicationService) reconcileApplicationUserManagerGroups(app *model.Application, requestedGroups []schema.ApplicationGroupProfile, resellerID *uint) error {
	if resellerID != nil {
		for _, g := range requestedGroups {
			if err := s.userMgr.ensureResellerCanUseGroup(*resellerID, g.GroupName); err != nil {
				return err
			}
			if err := s.userMgr.ensureResellerCanUseProfile(*resellerID, g.ProfileName); err != nil {
				return err
			}
		}
	}

	var existing []model.ApplicationUserManagerGroup
	if err := s.db.Where("application_id = ?", app.ID).Find(&existing).Error; err != nil {
		s.logger.Error("failed to list application user manager groups for reconcile", zap.Uint("application_id", app.ID), zap.Error(err))
		return fmt.Errorf("failed to update application user manager groups: %w", err)
	}
	existingByGroup := make(map[string]model.ApplicationUserManagerGroup, len(existing))
	for _, link := range existing {
		existingByGroup[link.GroupName] = link
	}
	wanted := make(map[string]bool, len(requestedGroups))
	for _, g := range requestedGroups {
		wanted[g.GroupName] = true
	}

	var wg sync.WaitGroup
	for _, g := range requestedGroups {
		if _, already := existingByGroup[g.GroupName]; already {
			continue
		}
		wg.Add(1)
		go func(g schema.ApplicationGroupProfile) {
			defer wg.Done()
			if err := s.provisionUserManagerGroup(app, g); err != nil {
				s.logger.Error("failed to provision application user manager account during edit, continuing with remaining resources", zap.Uint("application_id", app.ID), zap.String("group", g.GroupName), zap.Error(err))
			}
		}(g)
	}
	for _, link := range existing {
		if wanted[link.GroupName] {
			continue
		}
		wg.Add(1)
		go func(link model.ApplicationUserManagerGroup) {
			defer wg.Done()
			if link.AccountID != nil {
				if err := s.userMgr.DeleteAccount(*link.AccountID, app.ResellerID); err != nil {
					s.logger.Warn("failed to delete application user manager account during edit, continuing", zap.Uint("account_id", *link.AccountID), zap.Error(err))
				}
			}
			if err := s.db.Unscoped().Delete(&link).Error; err != nil {
				s.logger.Error("failed to delete application user manager group link during edit", zap.Uint("application_id", app.ID), zap.String("group", link.GroupName), zap.Error(err))
			}
		}(link)
	}
	wg.Wait()

	return nil
}

// reconcileApplicationXuiPanels mirrors reconcileApplicationInterfaces
// exactly, swapping in V2Ray x-ui panels.
func (s *ApplicationService) reconcileApplicationXuiPanels(app *model.Application, requestedPanelIDs []uint, resellerID *uint) error {
	if resellerID != nil {
		for _, panelID := range requestedPanelIDs {
			if err := s.ensureResellerCanUsePanel(*resellerID, panelID); err != nil {
				return err
			}
		}
	}

	var existing []model.ApplicationXuiPanel
	if err := s.db.Where("application_id = ?", app.ID).Find(&existing).Error; err != nil {
		s.logger.Error("failed to list application xui panels for reconcile", zap.Uint("application_id", app.ID), zap.Error(err))
		return fmt.Errorf("failed to update application v2ray panels: %w", err)
	}
	existingByPanel := make(map[uint]model.ApplicationXuiPanel, len(existing))
	for _, link := range existing {
		existingByPanel[link.PanelID] = link
	}
	wanted := make(map[uint]bool, len(requestedPanelIDs))
	for _, id := range requestedPanelIDs {
		wanted[id] = true
	}

	var wg sync.WaitGroup
	for _, panelID := range requestedPanelIDs {
		if _, already := existingByPanel[panelID]; already {
			continue
		}
		wg.Add(1)
		go func(panelID uint) {
			defer wg.Done()
			if err := s.provisionXuiPanel(app, panelID); err != nil {
				s.logger.Error("failed to provision application v2ray package during edit, continuing with remaining resources", zap.Uint("application_id", app.ID), zap.Uint("panel_id", panelID), zap.Error(err))
			}
		}(panelID)
	}
	for _, link := range existing {
		if wanted[link.PanelID] {
			continue
		}
		wg.Add(1)
		go func(link model.ApplicationXuiPanel) {
			defer wg.Done()
			if link.PackageID != nil {
				if err := s.v2ray.DeletePackage(*link.PackageID, app.ResellerID); err != nil {
					s.logger.Warn("failed to delete application v2ray package during edit, continuing", zap.Uint("package_id", *link.PackageID), zap.Error(err))
				}
			}
			if err := s.db.Unscoped().Delete(&link).Error; err != nil {
				s.logger.Error("failed to delete application xui panel link during edit", zap.Uint("application_id", app.ID), zap.Uint("panel_id", link.PanelID), zap.Error(err))
			}
		}(link)
	}
	wg.Wait()

	return nil
}

func (s *ApplicationService) buildResponse(app model.Application) (*schema.ApplicationResponse, error) {
	resp, err := s.transformApplicationToResponse(app)
	return &resp, err
}

// applicationExpireTimeString formats app.ExpireAt as "YYYY-MM-DD" --
// Peer/UserManagerAccount's own ExpireTime field expects that string
// format, not a time.Time, matching Peer.ExpireTime's existing
// convention throughout this codebase.
func applicationExpireTimeString(app *model.Application) *string {
	if app.ExpireAt == nil {
		return nil
	}
	return utils.Ptr(app.ExpireAt.Format("2006-01-02"))
}

// transformApplicationToResponse resolves this Application's three
// resource-join tables into display-ready rows (looking up each peer's/
// account's/package's own current name+enabled state), plus any admin-set
// ApplicationResourceLocation label. A missing/deleted underlying resource
// (PeerID set but the peer row itself gone) is skipped rather than erroring
// -- the same "a stale reference is not a request failure" stance this
// codebase takes throughout (see V2RayPackageLocation's own tolerance of
// a dead panel).
func (s *ApplicationService) transformApplicationToResponse(app model.Application) (schema.ApplicationResponse, error) {
	locationLabels := s.locationLabelsByKey()

	var wgLinks []model.ApplicationInterface
	s.db.Where("application_id = ?", app.ID).Find(&wgLinks)
	wireguardPeers := make([]schema.ApplicationResourceResponse, 0, len(wgLinks))
	for _, link := range wgLinks {
		if link.PeerID == nil {
			continue
		}
		var peer model.Peer
		if err := s.db.First(&peer, *link.PeerID).Error; err != nil {
			continue
		}
		var iface model.Interface
		ifaceName := ""
		if err := s.db.First(&iface, link.InterfaceID).Error; err == nil {
			ifaceName = iface.Name
		}
		wireguardPeers = append(wireguardPeers, schema.ApplicationResourceResponse{
			ResourceID:   link.InterfaceID,
			ResourceName: ifaceName,
			Label:        lookupLabel(locationLabels, model.ResourceTypeWireGuardInterface, fmt.Sprintf("%d", link.InterfaceID)),
			Enabled:      !peer.Disabled,
		})
	}

	var umLinks []model.ApplicationUserManagerGroup
	s.db.Where("application_id = ?", app.ID).Find(&umLinks)
	userManagerAccounts := make([]schema.ApplicationResourceResponse, 0, len(umLinks))
	for _, link := range umLinks {
		if link.AccountID == nil {
			continue
		}
		var account model.UserManagerAccount
		if err := s.db.First(&account, *link.AccountID).Error; err != nil {
			continue
		}
		userManagerAccounts = append(userManagerAccounts, schema.ApplicationResourceResponse{
			// ResourceID must be the account's own ID (matching
			// GetConnectConfigs' identical AppConnectUserManagerAccount.ResourceID
			// convention) -- confirmed, reported bug: this was previously
			// hardcoded to 0 for every account, which the mobile app's own
			// OpenVPN location picker relies on to join this summary
			// against connectConfigs' own per-account protocol/
			// openvpn_config data by ResourceID. WireGuard/V2Ray's
			// equivalent loops below already set a real ID; only this one
			// was left at the zero value.
			ResourceID:   *link.AccountID,
			ResourceName: link.GroupName,
			ProfileName:  utils.Ptr(account.Profile),
			Label:        lookupLabel(locationLabels, model.ResourceTypeUserManagerGroup, link.GroupName),
			Enabled:      !account.Disabled,
		})
	}

	var xuiLinks []model.ApplicationXuiPanel
	s.db.Where("application_id = ?", app.ID).Find(&xuiLinks)
	v2rayPackages := make([]schema.ApplicationResourceResponse, 0, len(xuiLinks))
	for _, link := range xuiLinks {
		if link.PackageID == nil {
			continue
		}
		var pkg model.V2RayPackage
		if err := s.db.First(&pkg, *link.PackageID).Error; err != nil {
			continue
		}
		var panel model.XuiPanel
		panelName := ""
		if err := s.db.First(&panel, link.PanelID).Error; err == nil {
			panelName = panel.Name
		}
		v2rayPackages = append(v2rayPackages, schema.ApplicationResourceResponse{
			ResourceID:   link.PanelID,
			ResourceName: panelName,
			Label:        lookupLabel(locationLabels, model.ResourceTypeXuiPanel, fmt.Sprintf("%d", link.PanelID)),
			Enabled:      pkg.Status == "active",
		})
	}

	var startAt, expireAt *string
	if app.StartAt != nil {
		startAt = utils.Ptr(timehelper.TehranDateOnly(*app.StartAt))
	}
	if app.ExpireAt != nil {
		expireAt = utils.Ptr(timehelper.TehranDateOnly(*app.ExpireAt))
	}

	// UsedBytes is recomputed live here rather than trusting app.UsedBytes
	// alone -- that column is only refreshed by EnforceApplicationQuotas'
	// periodic tick, so a freshly-provisioned or just-connected Application
	// would otherwise read back 0 until the next tick lands, confirmed
	// reported as "حجم مصرفی همیشه صفر است" by the mobile app's own Usage
	// tab. sumApplicationUsage is a cheap indexed SUM query, safe to run on
	// every GET /api/app/me.
	usedBytes := s.sumApplicationUsage(app.ID)

	return schema.ApplicationResponse{
		Id:                     app.ID,
		ResellerID:             app.ResellerID,
		Name:                   app.Name,
		AppUsername:            app.AppUsername,
		AppPassword:            app.AppPassword,
		TotalVolumeBytes:       app.TotalVolumeBytes,
		UsedBytes:              usedBytes,
		DurationDays:           app.DurationDays,
		StartAt:                startAt,
		ExpireAt:               expireAt,
		MaxOnlineUsers:         app.MaxOnlineUsers,
		Status:                 app.Status,
		Disabled:               app.Disabled,
		DownloadSpeedLimitMbps: app.DownloadSpeedLimitMbps,
		UploadSpeedLimitMbps:   app.UploadSpeedLimitMbps,
		WireGuardPeers:         wireguardPeers,
		UserManagerAccounts:    userManagerAccounts,
		V2RayPackages:          v2rayPackages,
	}, nil
}

// listApplicationsScoped mirrors V2RayPackageService's own
// listPackagesScoped convention exactly: resellerID nil (admin) sees only
// admin-owned rows (reseller_id IS NULL), a non-nil resellerID sees only
// that reseller's own rows -- an admin never implicitly sees every
// reseller's Applications through this path (GetApplicationsByReseller
// below is the deliberate, explicit "view one reseller's Applications"
// path, mirroring V2RayPackage's own reseller-scoped listing endpoint).
func (s *ApplicationService) listApplicationsScoped(resellerID *uint) ([]model.Application, error) {
	query := s.db.Model(&model.Application{})
	if resellerID != nil {
		query = query.Where("reseller_id = ?", *resellerID)
	} else {
		query = query.Where("reseller_id IS NULL")
	}

	var apps []model.Application
	if err := query.Order("id desc").Find(&apps).Error; err != nil {
		return nil, err
	}
	return apps, nil
}

func (s *ApplicationService) ListApplications(resellerID *uint) ([]schema.ApplicationResponse, error) {
	apps, err := s.listApplicationsScoped(resellerID)
	if err != nil {
		return nil, err
	}
	result := make([]schema.ApplicationResponse, 0, len(apps))
	for _, app := range apps {
		resp, err := s.transformApplicationToResponse(app)
		if err != nil {
			continue
		}
		result = append(result, resp)
	}
	return result, nil
}

func (s *ApplicationService) GetApplicationsByReseller(resellerID uint) ([]schema.ApplicationResponse, error) {
	var apps []model.Application
	if err := s.db.Where("reseller_id = ?", resellerID).Order("id desc").Find(&apps).Error; err != nil {
		return nil, err
	}
	result := make([]schema.ApplicationResponse, 0, len(apps))
	for _, app := range apps {
		resp, err := s.transformApplicationToResponse(app)
		if err != nil {
			continue
		}
		result = append(result, resp)
	}
	return result, nil
}

// GetApplicationByID is an UNSCOPED lookup -- used only by the mobile
// app's own endpoints (AppAuthController.Me/OnlineCount), where the
// application_id JWT claim IS the authorization (it can only have been
// minted by AuthenticateApp for that exact Application), unlike every
// admin/reseller-facing lookup in this file which scopes by resellerID.
func (s *ApplicationService) GetApplicationByID(id uint) (*schema.ApplicationResponse, error) {
	var app model.Application
	if err := s.db.First(&app, id).Error; err != nil {
		return nil, err
	}
	return s.buildResponse(app)
}

func (s *ApplicationService) getApplicationByIDScoped(id uint, resellerID *uint) (model.Application, error) {
	var app model.Application
	query := s.db.Where("id = ?", id)
	if resellerID != nil {
		query = query.Where("reseller_id = ?", *resellerID)
	}
	if err := query.First(&app).Error; err != nil {
		return model.Application{}, err
	}
	return app, nil
}

// UpdateApplication changes name/quota/duration/max-online-users/manual
// disable state. Raising TotalVolumeBytes or DurationDays past what
// caused a quota suspension automatically resumes every underlying
// resource, mirroring Reseller.UpdateReseller's own
// quotaRaised-triggers-resume pattern exactly.
func (s *ApplicationService) UpdateApplication(id uint, req *schema.UpdateApplicationRequest, resellerID *uint) (*schema.ApplicationResponse, error) {
	app, err := s.getApplicationByIDScoped(id, resellerID)
	if err != nil {
		return nil, fmt.Errorf("application not found: %w", err)
	}

	// A confirmed, reported gap: there was previously NO way to edit an
	// existing Application's protocols/interfaces/locations at all -- an
	// admin had to delete and recreate the whole Application (losing its
	// AppUsername/AppPassword and usage history) just to add or remove
	// one WireGuard interface, User Manager group, or V2Ray panel. Each
	// resource type is reconciled independently and only when the
	// caller actually sent that field -- see UpdateApplicationRequest's
	// own doc comment on the *[]T "omit means don't touch" convention.
	if req.InterfaceIDs != nil {
		if err := s.reconcileApplicationInterfaces(&app, *req.InterfaceIDs, resellerID); err != nil {
			return nil, err
		}
	}
	if req.UserManagerGroups != nil {
		if err := s.reconcileApplicationUserManagerGroups(&app, *req.UserManagerGroups, resellerID); err != nil {
			return nil, err
		}
	}
	if req.XuiPanelIDs != nil {
		if err := s.reconcileApplicationXuiPanels(&app, *req.XuiPanelIDs, resellerID); err != nil {
			return nil, err
		}
	}

	if req.Name != nil {
		app.Name = *req.Name
	}

	quotaRaised := false
	if req.TotalVolumeBytes != nil && *req.TotalVolumeBytes > app.TotalVolumeBytes {
		quotaRaised = true
	}
	if req.TotalVolumeBytes != nil {
		app.TotalVolumeBytes = *req.TotalVolumeBytes
	}
	if req.DurationDays != nil {
		app.DurationDays = *req.DurationDays
		startAt := time.Now()
		if app.StartAt != nil {
			startAt = *app.StartAt
		}
		expireAt := startAt.AddDate(0, 0, app.DurationDays)
		if app.ExpireAt == nil || expireAt.After(*app.ExpireAt) {
			quotaRaised = true
		}
		app.ExpireAt = &expireAt
	}
	if req.MaxOnlineUsers != nil {
		app.MaxOnlineUsers = *req.MaxOnlineUsers
	}
	if req.DownloadSpeedLimitMbps != nil {
		app.DownloadSpeedLimitMbps = req.DownloadSpeedLimitMbps
	}
	if req.UploadSpeedLimitMbps != nil {
		app.UploadSpeedLimitMbps = req.UploadSpeedLimitMbps
	}
	if req.Disabled != nil {
		app.Disabled = *req.Disabled
		if !*req.Disabled {
			quotaRaised = true // a manual re-enable should also resume any quota-suspended resources
		}
	}

	if err := s.db.Save(&app).Error; err != nil {
		s.logger.Error("failed to update application", zap.Error(err))
		return nil, fmt.Errorf("failed to update application: %w", err)
	}

	if quotaRaised && app.SuspendedByQuota && app.UsedBytes < app.TotalVolumeBytes {
		if err := s.resumeApplication(&app); err != nil {
			s.logger.Error("failed to resume quota-suspended application", zap.Uint("application_id", app.ID), zap.Error(err))
		}
	}

	return s.buildResponse(app)
}

func (s *ApplicationService) DeleteApplication(id uint, resellerID *uint) error {
	app, err := s.getApplicationByIDScoped(id, resellerID)
	if err != nil {
		return fmt.Errorf("application not found: %w", err)
	}

	var wgLinks []model.ApplicationInterface
	s.db.Where("application_id = ?", app.ID).Find(&wgLinks)
	for _, link := range wgLinks {
		if link.PeerID != nil {
			if err := s.peers.DeletePeer(*link.PeerID, app.ResellerID); err != nil {
				s.logger.Warn("failed to delete application peer, continuing cleanup", zap.Uint("peer_id", *link.PeerID), zap.Error(err))
			}
		}
	}

	var umLinks []model.ApplicationUserManagerGroup
	s.db.Where("application_id = ?", app.ID).Find(&umLinks)
	for _, link := range umLinks {
		if link.AccountID != nil {
			if err := s.userMgr.DeleteAccount(*link.AccountID, app.ResellerID); err != nil {
				s.logger.Warn("failed to delete application user manager account, continuing cleanup", zap.Uint("account_id", *link.AccountID), zap.Error(err))
			}
		}
	}

	var xuiLinks []model.ApplicationXuiPanel
	s.db.Where("application_id = ?", app.ID).Find(&xuiLinks)
	for _, link := range xuiLinks {
		if link.PackageID != nil {
			if err := s.v2ray.DeletePackage(*link.PackageID, app.ResellerID); err != nil {
				s.logger.Warn("failed to delete application v2ray package, continuing cleanup", zap.Uint("package_id", *link.PackageID), zap.Error(err))
			}
		}
	}

	s.db.Unscoped().Where("application_id = ?", app.ID).Delete(&model.ApplicationInterface{})
	s.db.Unscoped().Where("application_id = ?", app.ID).Delete(&model.ApplicationUserManagerGroup{})
	s.db.Unscoped().Where("application_id = ?", app.ID).Delete(&model.ApplicationXuiPanel{})

	if err := s.db.Delete(&app).Error; err != nil {
		return fmt.Errorf("failed to delete application: %w", err)
	}

	s.logResellerAction(app.ResellerID, AuditActionApplicationDeleted, fmt.Sprintf("حذف اپلیکیشن «%s» (یوزرنیم %s)", app.Name, app.AppUsername))
	return nil
}

// EnforceApplicationQuotas is the periodic job entrypoint (see
// cmd/main.go's job registration) -- for every active Application, sums
// current usage across every owned peer/account/package (their own
// existing running-total counters, NOT UsageSnapshot -- see this
// service's own top-level doc comment for why) and suspends the whole
// bundle the moment the combined total reaches TotalVolumeBytes or
// ExpireAt has passed. Mirrors the WireGuard/User Manager/V2Ray traffic
// job's own suspend bookkeeping (SuspendedByQuota/WasActiveBeforeSuspend)
// at the Application level, cascading down to every owned resource.
func (s *ApplicationService) EnforceApplicationQuotas() {
	var apps []model.Application
	if err := s.db.Where("disabled = ?", false).Find(&apps).Error; err != nil {
		s.logger.Error("failed to fetch applications for quota enforcement", zap.Error(err))
		return
	}

	for _, app := range apps {
		used := s.sumApplicationUsage(app.ID)
		expired := app.ExpireAt != nil && time.Now().After(*app.ExpireAt)

		// Confirmed, reported bug (Payment-mode/پرداختی resellers: every
		// user got force-disabled within moments of connecting, despite the
		// reseller's wallet having balance): this overQuota check ran
		// unconditionally against TotalVolumeBytes regardless of the owning
		// reseller's BillingMode, exactly the same class of bug already
		// fixed for the other three products (see applyResellerQuota/
		// applyUserManagerResellerQuota in cmd/jobs/traffic.go and
		// applyResellerV2RayQuota/enforcePackageQuota in v2ray_sync.go, all
		// of which guard their own leftover volume-quota field the same
		// way). A Payment-based reseller's Application keeps its old/
		// leftover TotalVolumeBytes (never cleared, same rationale as
		// V2RayQuotaBytes/UserManagerQuotaBytes -- so switching back to
		// Volume-based needs no migration), but that stale number must
		// never itself trigger suspension once billing is Payment-based:
		// only ChargeUsage's own billing-rejection (a debit actually
		// failing against the wallet) may disable a Payment reseller's
		// resources. Without this guard, a reseller switched to Payment
		// billing would have every peer/account/package under every one of
		// their Applications disabled the moment real usage (which starts
		// accruing as soon as users connect) crossed whatever
		// TotalVolumeBytes happened to be left over from before the
		// switch -- often small, so it could trip almost immediately.
		var paymentBased bool
		if app.ResellerID != nil {
			var reseller model.Reseller
			if err := s.db.Select("billing_mode").Where("id = ?", *app.ResellerID).First(&reseller).Error; err == nil {
				paymentBased = reseller.BillingMode == model.ResellerBillingModePayment
			}
		}
		overQuota := !paymentBased && used >= app.TotalVolumeBytes

		// Payment-based billing (پرداختی -- see model.Reseller.BillingMode's
		// own doc comment) charges only the newly-accrued delta since the
		// last tick, mirroring cmd/jobs/traffic.go's applyResellerQuota/
		// applyUserManagerResellerQuota and v2ray_sync.go's identical
		// pattern for the other three products. A no-op (billing is nil, or
		// the owning reseller isn't Payment-based, or delta<=0 on an
		// Application's very first tick / after a quota reset) is cheap and
		// safe -- see ResellerBillingService.ChargeUsage's own doc comment.
		// Applications have no per-location pricing concept (unlike
		// WireGuard interfaces/User Manager groups/V2Ray panels), so
		// locationKey is always "".
		if s.billing != nil && app.ResellerID != nil {
			delta := used - app.UsedBytes
			if err := s.billing.ChargeUsage(*app.ResellerID, model.ResellerBillingProductApplication, "", delta); err != nil {
				s.logger.Warn("failed to charge application usage to reseller wallet", zap.Uint("application_id", app.ID), zap.Uint("reseller_id", *app.ResellerID), zap.Error(err))
			}
		}

		if err := s.db.Model(&model.Application{}).Where("id = ?", app.ID).Update("used_bytes", used).Error; err != nil {
			s.logger.Warn("failed to update application used_bytes", zap.Uint("application_id", app.ID), zap.Error(err))
		}
		app.UsedBytes = used

		if (overQuota || expired) && !app.SuspendedByQuota {
			if err := s.suspendApplication(&app); err != nil {
				s.logger.Error("failed to suspend application over quota", zap.Uint("application_id", app.ID), zap.Error(err))
			}
		}
	}

	// Reseller-level ApplicationQuotaBytes pool -- run AFTER the per-
	// Application loop above so every Application.UsedBytes this tick
	// already reflects the freshest sumApplicationUsage, exactly like
	// DNSSyncService.SyncAccounts running per-account usage sync before its
	// own applyResellerDNSQuota pass. One reseller ID appears at most once
	// per tick even if it owns many Applications.
	seenResellers := make(map[uint]bool)
	for _, app := range apps {
		if app.ResellerID == nil || seenResellers[*app.ResellerID] {
			continue
		}
		seenResellers[*app.ResellerID] = true
		s.applyResellerApplicationQuota(*app.ResellerID)
	}
}

// applyResellerApplicationQuota mirrors DNSSyncService.applyResellerDNSQuota
// exactly, adapted for Application's own suspend/resume mechanism
// (suspendApplication/resumeApplication, which fan out to the underlying
// peer/account/package resources) instead of a single remote doctor-dns
// push. No Payment-mode billing branch here either -- ChargeUsage already
// runs per-Application inside EnforceApplicationQuotas above, independent
// of this reseller-pool check.
func (s *ApplicationService) applyResellerApplicationQuota(resellerID uint) {
	var appsToSuspend []model.Application
	var appsToResume []model.Application

	err := s.db.Transaction(func(tx *gorm.DB) error {
		var reseller model.Reseller
		if err := tx.Clauses(clause.Locking{Strength: "UPDATE"}).Where("id = ?", resellerID).First(&reseller).Error; err != nil {
			s.logger.Error("failed to fetch reseller for application quota", zap.Uint("reseller_id", resellerID), zap.Error(err))
			return nil
		}

		var liveUsed int64
		if err := tx.Model(&model.Application{}).Where("reseller_id = ?", resellerID).
			Select("COALESCE(SUM(used_bytes), 0)").Scan(&liveUsed).Error; err != nil {
			s.logger.Error("failed to sum reseller application usage", zap.Uint("reseller_id", resellerID), zap.Error(err))
			return nil
		}
		totalUsed := liveUsed + reseller.ApplicationDeletedUsageBytes

		if totalUsed != reseller.ApplicationUsedBytes {
			if err := tx.Model(&model.Reseller{}).Where("id = ?", resellerID).Update("application_used_bytes", totalUsed).Error; err != nil {
				s.logger.Error("failed to update reseller application usage", zap.Uint("reseller_id", resellerID), zap.Error(err))
				return err
			}
		}

		if reseller.ApplicationQuotaBytes == nil {
			return nil // unlimited
		}

		if totalUsed > *reseller.ApplicationQuotaBytes {
			if err := tx.Where("reseller_id = ? AND status != ?", resellerID, "suspended").
				Find(&appsToSuspend).Error; err != nil {
				s.logger.Error("failed to list applications to suspend for reseller quota", zap.Uint("reseller_id", resellerID), zap.Error(err))
				return err
			}
			if len(appsToSuspend) > 0 {
				ids := make([]uint, len(appsToSuspend))
				for i, a := range appsToSuspend {
					ids[i] = a.ID
				}
				if err := tx.Model(&model.Application{}).Where("id IN ?", ids).Updates(map[string]interface{}{
					"status":                      "suspended",
					"suspended_by_reseller_quota": true,
				}).Error; err != nil {
					return err
				}
			}
		} else {
			if err := tx.Where("reseller_id = ? AND suspended_by_reseller_quota = ?", resellerID, true).
				Find(&appsToResume).Error; err != nil {
				s.logger.Error("failed to list reseller-quota-suspended applications to resume", zap.Uint("reseller_id", resellerID), zap.Error(err))
				return err
			}
			if len(appsToResume) > 0 {
				ids := make([]uint, len(appsToResume))
				for i, a := range appsToResume {
					ids[i] = a.ID
				}
				if err := tx.Model(&model.Application{}).Where("id IN ?", ids).Updates(map[string]interface{}{
					"status":                      "active",
					"suspended_by_reseller_quota": false,
				}).Error; err != nil {
					s.logger.Error("failed to resume reseller-quota-suspended applications", zap.Uint("reseller_id", resellerID), zap.Error(err))
					return err
				}
			}
		}
		return nil
	})
	if err != nil {
		return
	}

	// Real (RouterOS/x-ui) calls happen AFTER the transaction commits --
	// mirrors applyResellerDNSQuota/applyResellerV2RayQuota's identical
	// ordering to avoid holding the reseller row lock across a network
	// round trip. Both suspend AND resume push to the underlying resources.
	for _, app := range appsToSuspend {
		if err := s.suspendApplication(&app); err != nil {
			s.logger.Error("failed to suspend application for reseller quota", zap.Uint("application_id", app.ID), zap.Error(err))
		}
	}
	for _, app := range appsToResume {
		if err := s.resumeApplication(&app); err != nil {
			s.logger.Error("failed to resume application for reseller quota", zap.Uint("application_id", app.ID), zap.Error(err))
		}
	}
}

// ResumeApplicationsForResellerQuota re-enables every one of resellerID's
// Applications currently suspended for the RESELLER's overall pool quota
// (SuspendedByResellerQuota), immediately -- the on-demand counterpart to
// applyResellerApplicationQuota's own resume branch, mirroring
// DNSSyncService.ResumeAccountsForResellerQuota's identical "instant result
// instead of a silent multi-minute wait" role. Called from
// Reseller.UpdateReseller the moment an admin raises ApplicationQuotaBytes
// back above the reseller's current usage.
func (s *ApplicationService) ResumeApplicationsForResellerQuota(resellerID uint) {
	var apps []model.Application
	if err := s.db.Where(
		"reseller_id = ? AND suspended_by_reseller_quota = ?", resellerID, true,
	).Find(&apps).Error; err != nil {
		s.logger.Error("failed to fetch reseller-quota-suspended applications", zap.Uint("reseller_id", resellerID), zap.Error(err))
		return
	}

	ids := make([]uint, len(apps))
	for i, a := range apps {
		ids[i] = a.ID
	}
	if len(ids) > 0 {
		if err := s.db.Model(&model.Application{}).Where("id IN ?", ids).Updates(map[string]interface{}{
			"status":                      "active",
			"suspended_by_reseller_quota": false,
		}).Error; err != nil {
			s.logger.Error("failed to clear reseller-quota-suspended flag for applications", zap.Uint("reseller_id", resellerID), zap.Error(err))
			return
		}
	}

	for _, app := range apps {
		if err := s.resumeApplication(&app); err != nil {
			s.logger.Error("failed to resume application for reseller quota", zap.Uint("application_id", app.ID), zap.Error(err))
		}
	}
}

// sumApplicationUsage adds up this Application's combined usage across
// every WireGuard peer / User Manager account / V2Ray package it owns.
// Two confirmed, reported bugs fixed here:
//  1. The V2Ray sum queried V2RayPackageLocation.download_usage/
//     upload_usage, columns that DO NOT EXIST on that model at all (it
//     tracks usage as a single UsedBytesCached field per location, see
//     that model's own doc comment) -- every tick this ran, it failed
//     outright with "SQL logic error: no such column: download_usage",
//     silently leaving V2Ray usage out of every Application's quota
//     total entirely (Scan into sum simply left it at its zero value on
//     error, so this wasn't just noisy logs, it was actively wrong).
//  2. The User Manager sum read the account's RAW download_usage/
//     upload_usage columns directly, ignoring UsageOffsetDownload/
//     UsageOffsetUpload (see that field's own doc comment -- the
//     "Reset Usage" feature's offset mechanism) -- an Application whose
//     User Manager account had its usage reset would still count the
//     pre-reset raw total here, silently undoing the reset from this
//     Application's own point of view.
func (s *ApplicationService) sumApplicationUsage(applicationID uint) int64 {
	var total int64

	var peerIDs []uint
	s.db.Model(&model.ApplicationInterface{}).Where("application_id = ? AND peer_id IS NOT NULL", applicationID).Pluck("peer_id", &peerIDs)
	if len(peerIDs) > 0 {
		var sum int64
		s.db.Model(&model.Peer{}).Where("id IN ?", peerIDs).
			Select("COALESCE(SUM(download_usage + upload_usage), 0)").Scan(&sum)
		total += sum
	}

	var accounts []model.UserManagerAccount
	var accountIDs []uint
	s.db.Model(&model.ApplicationUserManagerGroup{}).Where("application_id = ? AND account_id IS NOT NULL", applicationID).Pluck("account_id", &accountIDs)
	if len(accountIDs) > 0 {
		s.db.Where("id IN ?", accountIDs).Find(&accounts)
		for _, acct := range accounts {
			displayedDownload := acct.DownloadUsage - acct.UsageOffsetDownload
			if displayedDownload < 0 {
				displayedDownload = 0
			}
			displayedUpload := acct.UploadUsage - acct.UsageOffsetUpload
			if displayedUpload < 0 {
				displayedUpload = 0
			}
			total += displayedDownload + displayedUpload
		}
	}

	var packages []model.V2RayPackage
	var packageIDs []uint
	s.db.Model(&model.ApplicationXuiPanel{}).Where("application_id = ? AND package_id IS NOT NULL", applicationID).Pluck("package_id", &packageIDs)
	if len(packageIDs) > 0 {
		s.db.Where("id IN ?", packageIDs).Find(&packages)
		for _, pkg := range packages {
			var rawUsedBytes int64
			s.db.Model(&model.V2RayPackageLocation{}).Where("package_id = ?", pkg.ID).
				Select("COALESCE(SUM(used_bytes_cached), 0)").Scan(&rawUsedBytes)
			displayedUsedBytes := rawUsedBytes - pkg.UsageOffsetBytes
			if displayedUsedBytes < 0 {
				displayedUsedBytes = 0
			}
			total += displayedUsedBytes
		}
	}

	return total
}

// suspendApplication disables every owned peer/account/package (via each
// protocol's own real disable path -- UpdatePeer/UpdateAccount push to
// Mikrotik, UpdatePackage's Status change pushes to x-ui via
// applyPackageStateToLocations) and marks the Application itself
// suspended, mirroring Peer/UserManagerAccount/V2RayPackage's identical
// SuspendedByQuota/WasActiveBeforeSuspend bookkeeping.
func (s *ApplicationService) suspendApplication(app *model.Application) error {
	wasActive := app.Status == "active" && !app.Disabled

	var wgLinks []model.ApplicationInterface
	s.db.Where("application_id = ? AND peer_id IS NOT NULL", app.ID).Find(&wgLinks)
	for _, link := range wgLinks {
		disabled := true
		if _, err := s.peers.UpdatePeer(*link.PeerID, &schema.UpdatePeerRequest{Disabled: &disabled}, app.ResellerID); err != nil {
			s.logger.Warn("failed to disable application peer during quota suspension", zap.Uint("peer_id", *link.PeerID), zap.Error(err))
		}
	}

	var umLinks []model.ApplicationUserManagerGroup
	s.db.Where("application_id = ? AND account_id IS NOT NULL", app.ID).Find(&umLinks)
	for _, link := range umLinks {
		disabled := true
		if _, err := s.userMgr.UpdateAccount(*link.AccountID, &schema.UpdateUserManagerAccountRequest{Disabled: &disabled}, app.ResellerID); err != nil {
			s.logger.Warn("failed to disable application user manager account during quota suspension", zap.Uint("account_id", *link.AccountID), zap.Error(err))
		}
	}

	var xuiLinks []model.ApplicationXuiPanel
	s.db.Where("application_id = ? AND package_id IS NOT NULL", app.ID).Find(&xuiLinks)
	for _, link := range xuiLinks {
		status := "suspended"
		if _, err := s.v2ray.UpdatePackage(*link.PackageID, &schema.UpdateV2RayPackageRequest{Status: &status}, app.ResellerID); err != nil {
			s.logger.Warn("failed to suspend application v2ray package during quota suspension", zap.Uint("package_id", *link.PackageID), zap.Error(err))
		}
	}

	return s.db.Model(&model.Application{}).Where("id = ?", app.ID).Updates(map[string]interface{}{
		"status":                    "suspended",
		"suspended_by_quota":        true,
		"was_active_before_suspend": wasActive,
	}).Error
}

// resumeApplication mirrors suspendApplication exactly in reverse, only
// ever called when SuspendedByQuota && WasActiveBeforeSuspend, matching
// resumeQuotaSuspendedPeers' own "never resurrect something suspended on
// purpose" guarantee.
func (s *ApplicationService) resumeApplication(app *model.Application) error {
	if !app.WasActiveBeforeSuspend {
		return nil
	}

	var wgLinks []model.ApplicationInterface
	s.db.Where("application_id = ? AND peer_id IS NOT NULL", app.ID).Find(&wgLinks)
	for _, link := range wgLinks {
		disabled := false
		if _, err := s.peers.UpdatePeer(*link.PeerID, &schema.UpdatePeerRequest{Disabled: &disabled}, app.ResellerID); err != nil {
			s.logger.Warn("failed to re-enable application peer", zap.Uint("peer_id", *link.PeerID), zap.Error(err))
		}
	}

	var umLinks []model.ApplicationUserManagerGroup
	s.db.Where("application_id = ? AND account_id IS NOT NULL", app.ID).Find(&umLinks)
	for _, link := range umLinks {
		disabled := false
		if _, err := s.userMgr.UpdateAccount(*link.AccountID, &schema.UpdateUserManagerAccountRequest{Disabled: &disabled}, app.ResellerID); err != nil {
			s.logger.Warn("failed to re-enable application user manager account", zap.Uint("account_id", *link.AccountID), zap.Error(err))
		}
	}

	var xuiLinks []model.ApplicationXuiPanel
	s.db.Where("application_id = ? AND package_id IS NOT NULL", app.ID).Find(&xuiLinks)
	for _, link := range xuiLinks {
		status := "active"
		if _, err := s.v2ray.UpdatePackage(*link.PackageID, &schema.UpdateV2RayPackageRequest{Status: &status}, app.ResellerID); err != nil {
			s.logger.Warn("failed to re-activate application v2ray package", zap.Uint("package_id", *link.PackageID), zap.Error(err))
		}
	}

	return s.db.Model(&model.Application{}).Where("id = ?", app.ID).Updates(map[string]interface{}{
		"status":                    "active",
		"suspended_by_quota":        false,
		"was_active_before_suspend": false,
	}).Error
}

// --- Resource location labels (admin-only cosmetic settings page) ---

func locationKey(resourceType, resourceKey string) string {
	return resourceType + ":" + resourceKey
}

func (s *ApplicationService) locationLabelsByKey() map[string]string {
	var rows []model.ApplicationResourceLocation
	s.db.Find(&rows)
	result := make(map[string]string, len(rows))
	for _, r := range rows {
		result[locationKey(r.ResourceType, r.ResourceKey)] = r.Label
	}
	return result
}

// lookupLabel returns nil (not an empty-string pointer) when no admin
// label has been set for this resource -- ApplicationResourceResponse.Label
// is `omitempty` on the wire, so the frontend can distinguish "no label
// configured" from "labeled with an empty string."
func lookupLabel(labels map[string]string, resourceType, resourceKey string) *string {
	if label, ok := labels[locationKey(resourceType, resourceKey)]; ok {
		return utils.Ptr(label)
	}
	return nil
}

// SetResourceLocation upserts one (ResourceType, ResourceKey) -> Label
// mapping -- admin only, purely cosmetic (see
// model.ApplicationResourceLocation's own doc comment).
func (s *ApplicationService) SetResourceLocation(req *schema.SetApplicationResourceLocationRequest) error {
	var existing model.ApplicationResourceLocation
	err := s.db.Where("resource_type = ? AND resource_key = ?", req.ResourceType, req.ResourceKey).First(&existing).Error
	if err == nil {
		existing.Label = req.Label
		return s.db.Save(&existing).Error
	}
	if !errors.Is(err, gorm.ErrRecordNotFound) {
		return err
	}
	return s.db.Create(&model.ApplicationResourceLocation{
		ResourceType: req.ResourceType,
		ResourceKey:  req.ResourceKey,
		Label:        req.Label,
	}).Error
}

func (s *ApplicationService) ListResourceLocations() ([]schema.ApplicationResourceLocationResponse, error) {
	var rows []model.ApplicationResourceLocation
	if err := s.db.Order("resource_type, resource_key").Find(&rows).Error; err != nil {
		return nil, err
	}
	result := make([]schema.ApplicationResourceLocationResponse, 0, len(rows))
	for _, r := range rows {
		result = append(result, schema.ApplicationResourceLocationResponse{
			ResourceType: r.ResourceType,
			ResourceKey:  r.ResourceKey,
			Label:        r.Label,
		})
	}
	return result, nil
}

// --- Mobile app's own login/session ---

// AuthenticateApp checks AppUsername/AppPassword and returns the matching
// Application row -- deliberately a PLAIN password comparison (mirrors
// UserManagerAccount.Password/Peer.PrivateKey's own plaintext-storage
// precedent throughout this codebase), never bcrypt, since AppPassword
// must also be re-displayable verbatim to the admin/reseller who created
// it (the same reason UserManagerAccount.Password is plaintext).
func (s *ApplicationService) AuthenticateApp(username, password string) (*model.Application, error) {
	var app model.Application
	if err := s.db.Where("app_username = ?", username).First(&app).Error; err != nil {
		return nil, fmt.Errorf("invalid username or password")
	}
	if app.AppPassword != password {
		return nil, fmt.Errorf("invalid username or password")
	}
	return &app, nil
}

// UpsertDeviceSession records (or refreshes) one ApplicationDeviceSession
// row on login -- see that model's own doc comment. A no-op if deviceID
// is empty (an older app build that doesn't send one yet, or a manual
// API caller). Called from Login, not from every /me request, since
// FirstSeenAt should only ever be set once; LastSeenAt refreshes happen
// separately via TouchDeviceSession.
func (s *ApplicationService) UpsertDeviceSession(applicationID uint, deviceID, deviceLabel string) {
	if deviceID == "" {
		return
	}
	now := time.Now()
	var existing model.ApplicationDeviceSession
	err := s.db.Where("application_id = ? AND device_id = ?", applicationID, deviceID).First(&existing).Error
	if err == nil {
		s.db.Model(&existing).Updates(map[string]interface{}{"device_label": deviceLabel, "last_seen_at": now})
		return
	}
	s.db.Create(&model.ApplicationDeviceSession{
		ApplicationID: applicationID,
		DeviceID:      deviceID,
		DeviceLabel:   deviceLabel,
		FirstSeenAt:   now,
		LastSeenAt:    now,
	})
}

// TouchDeviceSession refreshes LastSeenAt for an already-known device on
// every authenticated /me call -- see AppAuthController.Me's own call
// site. Silently does nothing if the (applicationID, deviceID) pair has
// no existing row (an older app build's token that predates device-ID
// support, or a device that was already revoked -- either way, not an
// error condition worth surfacing to the caller).
func (s *ApplicationService) TouchDeviceSession(applicationID uint, deviceID string) {
	if deviceID == "" {
		return
	}
	s.db.Model(&model.ApplicationDeviceSession{}).
		Where("application_id = ? AND device_id = ?", applicationID, deviceID).
		Update("last_seen_at", time.Now())
}

// ListDeviceSessions answers GET /api/app/devices -- every device that
// has ever logged into this Application, most-recently-seen first.
func (s *ApplicationService) ListDeviceSessions(applicationID uint, currentDeviceID string) (*schema.AppDeviceSessionsResponse, error) {
	var rows []model.ApplicationDeviceSession
	if err := s.db.Where("application_id = ?", applicationID).Order("last_seen_at DESC").Find(&rows).Error; err != nil {
		return nil, err
	}
	devices := make([]schema.AppDeviceSessionResponse, 0, len(rows))
	for _, row := range rows {
		isOnline := false
		if row.ResourceType != "" && row.ResourceID != nil {
			isOnline = s.isResourceOnline(row.ResourceType, *row.ResourceID)
		}
		devices = append(devices, schema.AppDeviceSessionResponse{
			DeviceID:    row.DeviceID,
			DeviceLabel: row.DeviceLabel,
			FirstSeenAt: timehelper.TehranDateOnly(row.FirstSeenAt),
			LastSeenAt:  row.LastSeenAt.UTC().Format(time.RFC3339),
			IsCurrent:   row.DeviceID == currentDeviceID,
			IsOnline:    isOnline,
		})
	}
	return &schema.AppDeviceSessionsResponse{Devices: devices}, nil
}

// RevokeDeviceSession backs POST /api/app/devices/revoke -- deletes the
// session row for the given device. This does NOT invalidate that
// device's existing JWT (app-user tokens are stateless, matching
// AppAuthController's own doc comment on why -- there is no server-side
// token blocklist anywhere in this codebase), so a revoked device's app
// keeps working until its 30-day token naturally expires; what revoke
// actually accomplishes today is removing stale/no-longer-used devices
// from the list and no longer counting them once online-session tracking
// gains a similar real "currently active" signal, mirroring the scoped
// limitation already documented on GetOnlineCount.
func (s *ApplicationService) RevokeDeviceSession(applicationID uint, deviceID string) error {
	return s.db.Where("application_id = ? AND device_id = ?", applicationID, deviceID).
		Delete(&model.ApplicationDeviceSession{}).Error
}

// GetOnlineCount answers GET /api/app/online-count -- see
// schema.AppOnlineCountResponse's own doc comment for why the mobile app
// (not the panel) is responsible for actually refusing a new connection
// once at MaxOnlineUsers. "Online" means CURRENTLY CONNECTED RIGHT NOW,
// not merely "provisioned and enabled" -- a fixed bundle of N granted
// resources would otherwise permanently read as N/N online before anyone
// ever connects. WireGuard/User Manager identities are considered online
// when they have an open (DisconnectedAt IS NULL) IPConnectionLog row,
// the same live-session signal the Security feature's own connection
// history already relies on (populated by
// cmd/jobs/security_ip_collector.go, so this lags that job's own poll
// interval, not instantaneous). V2Ray packages reuse
// V2RayPackageLocation.IsOnline, which V2RaySyncService.pollOnlineStatus
// already maintains for the exact same reason.
func (s *ApplicationService) GetOnlineCount(applicationID uint) (*schema.AppOnlineCountResponse, error) {
	var app model.Application
	if err := s.db.First(&app, applicationID).Error; err != nil {
		return nil, err
	}

	var peerIDs []uint
	s.db.Model(&model.ApplicationInterface{}).Where("application_id = ? AND peer_id IS NOT NULL", applicationID).Pluck("peer_id", &peerIDs)
	var onlinePeers int64
	if len(peerIDs) > 0 {
		s.db.Model(&model.IPConnectionLog{}).
			Where("protocol = ? AND peer_id IN ? AND disconnected_at IS NULL", "wireguard", peerIDs).
			Count(&onlinePeers)
	}

	var accountIDs []uint
	s.db.Model(&model.ApplicationUserManagerGroup{}).Where("application_id = ? AND account_id IS NOT NULL", applicationID).Pluck("account_id", &accountIDs)
	var onlineAccounts int64
	if len(accountIDs) > 0 {
		s.db.Model(&model.IPConnectionLog{}).
			Where("protocol = ? AND account_id IN ? AND disconnected_at IS NULL", "user_manager", accountIDs).
			Count(&onlineAccounts)
	}

	var packageIDs []uint
	s.db.Model(&model.ApplicationXuiPanel{}).Where("application_id = ? AND package_id IS NOT NULL", applicationID).Pluck("package_id", &packageIDs)
	var onlinePackages int64
	if len(packageIDs) > 0 {
		s.db.Model(&model.V2RayPackageLocation{}).
			Where("package_id IN ? AND is_online = ?", packageIDs, true).
			Count(&onlinePackages)
	}

	return &schema.AppOnlineCountResponse{
		OnlineCount:    int(onlinePeers + onlineAccounts + onlinePackages),
		MaxOnlineUsers: app.MaxOnlineUsers,
	}, nil
}

// isResourceOnline answers "is this ONE specific resource currently
// connected" -- the same three live-session signals GetOnlineCount already
// aggregates (IPConnectionLog for wireguard/user_manager,
// V2RayPackageLocation.IsOnline for v2ray), just scoped to a single
// resource instead of summed across every resource an Application owns.
// resourceID here is the join-table's own PeerID/AccountID/PackageID (NOT
// the connect-config ResourceID the app reports back -- see
// ReportDeviceResource, which resolves one into the other before calling
// this).
func (s *ApplicationService) isResourceOnline(resourceType string, resourceID uint) bool {
	switch resourceType {
	case model.ResourceTypeWireGuardInterface:
		var count int64
		s.db.Model(&model.IPConnectionLog{}).
			Where("protocol = ? AND peer_id = ? AND disconnected_at IS NULL", "wireguard", resourceID).
			Count(&count)
		return count > 0
	case model.ResourceTypeUserManagerGroup:
		var count int64
		s.db.Model(&model.IPConnectionLog{}).
			Where("protocol = ? AND account_id = ? AND disconnected_at IS NULL", "user_manager", resourceID).
			Count(&count)
		return count > 0
	case model.ResourceTypeXuiPanel:
		var count int64
		s.db.Model(&model.V2RayPackageLocation{}).
			Where("package_id = ? AND is_online = ?", resourceID, true).
			Count(&count)
		return count > 0
	default:
		return false
	}
}

// ReportDeviceResource backs POST /api/app/devices/report-resource -- see
// schema.AppReportDeviceResourceRequest's own doc comment for when the
// mobile app calls this. req.ResourceID is the connect-config ResourceID
// (InterfaceID/AccountID/PanelID, matching AppConnect*Config.ResourceID
// exactly), resolved here to the underlying PeerID/AccountID/PackageID
// that IPConnectionLog/V2RayPackageLocation are actually keyed on, mirroring
// GetConnectConfigs' own join. Silently does nothing if the (applicationID,
// deviceID) session row doesn't exist yet (mirrors TouchDeviceSession's
// own "not an error condition worth surfacing" precedent) or if the
// reported resource isn't actually one this Application owns (a stale/
// forged resource_id must never let a device masquerade as another
// resource's online status).
func (s *ApplicationService) ReportDeviceResource(applicationID uint, deviceID string, resourceType string, resourceID uint) error {
	if deviceID == "" {
		return nil
	}

	var underlyingID uint
	switch resourceType {
	case model.ResourceTypeWireGuardInterface:
		var link model.ApplicationInterface
		if err := s.db.Where("application_id = ? AND interface_id = ? AND peer_id IS NOT NULL", applicationID, resourceID).First(&link).Error; err != nil {
			return nil
		}
		underlyingID = *link.PeerID
	case model.ResourceTypeUserManagerGroup:
		var link model.ApplicationUserManagerGroup
		if err := s.db.Where("application_id = ? AND account_id = ?", applicationID, resourceID).First(&link).Error; err != nil {
			return nil
		}
		underlyingID = *link.AccountID
	case model.ResourceTypeXuiPanel:
		var link model.ApplicationXuiPanel
		if err := s.db.Where("application_id = ? AND panel_id = ? AND package_id IS NOT NULL", applicationID, resourceID).First(&link).Error; err != nil {
			return nil
		}
		underlyingID = *link.PackageID
	default:
		return nil
	}

	return s.db.Model(&model.ApplicationDeviceSession{}).
		Where("application_id = ? AND device_id = ?", applicationID, deviceID).
		Updates(map[string]interface{}{
			"resource_type": resourceType,
			"resource_id":   underlyingID,
		}).Error
}

// GetConnectConfigs is the mobile app's own connect-configs endpoint --
// unlike GetApplicationByID/transformApplicationToResponse (a display-only
// summary), this returns EVERY field each protocol's native VPN engine
// needs to actually establish a tunnel: WireGuard private key + server
// public key + endpoint, User Manager username/password/server/port per
// granted protocol, and the V2Ray subscription/config link. Every value
// already exists on the Peer/Interface/UserManagerAccount/V2RayPackage
// rows created by provisionInterface/provisionUserManagerGroup/
// provisionXuiPanel -- this method only joins the three ownership tables
// (ApplicationInterface/ApplicationUserManagerGroup/ApplicationXuiPanel)
// to those rows and re-shapes what's already there.
//
// Deliberately bypasses the IsShared/ShareExpireTime gate every existing
// share endpoint (GetUserConfig/GetAccountShareDetails/
// GetPackageShareDetails) enforces -- app-JWT authentication already
// proves the caller owns this exact Application, so there is nothing left
// for that gate to protect against here.
//
// Every read here is a plain DB query, never a live MikroTik/x-ui call
// (mirrors GetPackageShareDetails' own "CombinedConfigCached is a cache,
// never computed live" contract) -- a single unreachable panel/router
// must never make this whole endpoint fail; a resource whose lookup
// fails is simply skipped (logged), matching ListApplications'/
// GetOnlineCount's own "skip on per-row error, don't fail the batch"
// convention elsewhere in this file.
func (s *ApplicationService) GetConnectConfigs(applicationID uint) (*schema.AppConnectConfigsResponse, error) {
	// DownloadSpeedLimitMbps/UploadSpeedLimitMbps live on the parent
	// Application row itself (a single client-side-enforced cap applying
	// to whichever protocol below the app connects with -- see
	// model.Application's own doc comment), not on any of the three
	// per-protocol resource tables this method otherwise only ever
	// joins against -- so this is the one query here that reads the
	// Application row directly rather than just its ownership links.
	var app model.Application
	if err := s.db.First(&app, applicationID).Error; err != nil {
		return nil, fmt.Errorf("application not found: %w", err)
	}

	locationLabels := s.locationLabelsByKey()

	var wgLinks []model.ApplicationInterface
	s.db.Where("application_id = ? AND peer_id IS NOT NULL", applicationID).Find(&wgLinks)
	wireguardPeers := make([]schema.AppConnectWireGuardConfig, 0, len(wgLinks))
	for _, link := range wgLinks {
		var peer model.Peer
		if err := s.db.First(&peer, *link.PeerID).Error; err != nil || peer.Disabled {
			continue
		}
		var iface model.Interface
		if err := s.db.Where("name = ?", peer.Interface).First(&iface).Error; err != nil {
			s.logger.Warn("failed to look up interface for application connect-config, skipping peer", zap.Uint("peer_id", *link.PeerID), zap.String("interface_name", peer.Interface), zap.Error(err))
			continue
		}

		dns := common.DefaultDns
		if peer.DNSServers != nil && *peer.DNSServers != "" {
			dns = *peer.DNSServers
		}

		wireguardPeers = append(wireguardPeers, schema.AppConnectWireGuardConfig{
			ResourceID:          link.InterfaceID,
			Label:               lookupLabel(locationLabels, model.ResourceTypeWireGuardInterface, fmt.Sprintf("%d", link.InterfaceID)),
			PrivateKey:          peer.PrivateKey,
			Address:             peer.AllowedAddress,
			DNS:                 dns,
			PeerPublicKey:       iface.PublicKey,
			Endpoint:            peer.Endpoint,
			EndpointPort:        peer.EndpointPort,
			AllowedIPs:          common.AllowedIpsIncludeLocal,
			PersistentKeepalive: peer.PersistentKeepalive,
		})
	}

	var umLinks []model.ApplicationUserManagerGroup
	s.db.Where("application_id = ? AND account_id IS NOT NULL", applicationID).Find(&umLinks)
	userManagerAccounts := make([]schema.AppConnectUserManagerAccount, 0, len(umLinks))
	for _, link := range umLinks {
		var account model.UserManagerAccount
		if err := s.db.First(&account, *link.AccountID).Error; err != nil || account.Disabled {
			continue
		}
		protocolInfos, err := buildUserManagerProtocolInfos(s.db, s.logger, account)
		if err != nil {
			s.logger.Warn("failed to build protocol infos for application connect-config, skipping account", zap.Uint("account_id", *link.AccountID), zap.Error(err))
			continue
		}
		// nil (no template uploaded yet) is a normal, expected state --
		// never blocks the rest of this account's connect-config, mirrors
		// this whole method's own "one missing piece never fails the
		// batch" convention.
		openVpnConfig, err := s.openVpnTmpl.BuildConfigForAccount(account.Username, account.Password)
		if err != nil {
			s.logger.Warn("failed to build openvpn config for application connect-config", zap.Uint("account_id", *link.AccountID), zap.Error(err))
		}
		userManagerAccounts = append(userManagerAccounts, schema.AppConnectUserManagerAccount{
			ResourceID:    *link.AccountID,
			Label:         lookupLabel(locationLabels, model.ResourceTypeUserManagerGroup, link.GroupName),
			Username:      account.Username,
			Password:      account.Password,
			Protocols:     protocolInfos,
			OpenVpnConfig: openVpnConfig,
		})
	}

	var xuiLinks []model.ApplicationXuiPanel
	s.db.Where("application_id = ? AND package_id IS NOT NULL", applicationID).Find(&xuiLinks)
	v2rayPackages := make([]schema.AppConnectV2RayPackage, 0, len(xuiLinks))
	for _, link := range xuiLinks {
		var pkg model.V2RayPackage
		if err := s.db.First(&pkg, *link.PackageID).Error; err != nil || pkg.Status != "active" {
			continue
		}
		var location model.V2RayPackageLocation
		if err := s.db.Where("package_id = ? AND panel_id = ?", pkg.ID, link.PanelID).First(&location).Error; err != nil {
			s.logger.Warn("failed to find v2ray package location for application connect-config, skipping package", zap.Uint("package_id", *link.PackageID), zap.Error(err))
			continue
		}

		v2rayPackages = append(v2rayPackages, schema.AppConnectV2RayPackage{
			ResourceID:       link.PanelID,
			Label:            lookupLabel(locationLabels, model.ResourceTypeXuiPanel, fmt.Sprintf("%d", link.PanelID)),
			ConfigLink:       location.ConfigLinkCached,
			SubscriptionBody: pkg.CombinedConfigCached,
		})
	}

	return &schema.AppConnectConfigsResponse{
		DownloadSpeedLimitMbps: app.DownloadSpeedLimitMbps,
		UploadSpeedLimitMbps:   app.UploadSpeedLimitMbps,
		WireGuardPeers:         wireguardPeers,
		UserManagerAccounts:    userManagerAccounts,
		V2RayPackages:          v2rayPackages,
	}, nil
}

// UploadOpenVpnTemplate/GetOpenVpnTemplateStatus/GetOpenVpnTemplatePath
// are thin pass-throughs to openVpnTmpl -- ApplicationController goes
// through ApplicationService rather than holding a second service
// reference directly, matching how every other Application admin action
// is already reached through this one façade.
func (s *ApplicationService) UploadOpenVpnTemplate(content []byte) error {
	return s.openVpnTmpl.UploadTemplate(content)
}

func (s *ApplicationService) GetOpenVpnTemplateStatus() (bool, *time.Time, error) {
	return s.openVpnTmpl.GetTemplateStatus()
}

func (s *ApplicationService) GetOpenVpnTemplatePath() (string, error) {
	return s.openVpnTmpl.GetTemplatePath()
}

// GetWeeklyUsage answers GET /api/app/me/weekly-usage -- replaces the
// mobile app's own MOCK_WEEKLY placeholder bar chart data (a confirmed,
// reported gap) with a real per-day sum of UsageSnapshot.TotalBytes
// across every peer/account/package this Application owns, grouped by
// Iran-local calendar day. UsageSnapshot.Timestamp is Unix seconds (see
// its own doc comment on why it's a separate column from Model.CreatedAt),
// so day-bucketing happens in Go against Asia/Tehran rather than in SQL,
// keeping this portable across this codebase's supported DB backends
// without a DB-specific timezone-aware DATE() function.
func (s *ApplicationService) GetWeeklyUsage(applicationID uint) (*schema.AppWeeklyUsageResponse, error) {
	var peerIDs []uint
	s.db.Model(&model.ApplicationInterface{}).Where("application_id = ? AND peer_id IS NOT NULL", applicationID).Pluck("peer_id", &peerIDs)

	var accountIDs []uint
	s.db.Model(&model.ApplicationUserManagerGroup{}).Where("application_id = ? AND account_id IS NOT NULL", applicationID).Pluck("account_id", &accountIDs)

	var packageIDs []uint
	s.db.Model(&model.ApplicationXuiPanel{}).Where("application_id = ? AND package_id IS NOT NULL", applicationID).Pluck("package_id", &packageIDs)

	tehran := timehelper.TehranLocation()
	now := time.Now().In(tehran)
	todayStart := time.Date(now.Year(), now.Month(), now.Day(), 0, 0, 0, 0, tehran)
	windowStart := todayStart.AddDate(0, 0, -6)

	totalsByDay := make(map[string]int64, 7)
	days := make([]schema.AppWeeklyUsageDay, 7)
	for i := 0; i < 7; i++ {
		dateKey := windowStart.AddDate(0, 0, i).Format("2006-01-02")
		days[i] = schema.AppWeeklyUsageDay{Date: dateKey, TotalBytes: 0}
		totalsByDay[dateKey] = 0
	}

	type snapshotRow struct {
		Timestamp  int64
		TotalBytes int64
	}
	var rows []snapshotRow
	windowStartUnix := windowStart.Unix()

	if len(peerIDs) > 0 {
		var peerRows []snapshotRow
		s.db.Model(&model.UsageSnapshot{}).
			Where("protocol = ? AND peer_id IN ? AND timestamp >= ?", model.UsageProtocolWireGuard, peerIDs, windowStartUnix).
			Select("timestamp, total_bytes").Scan(&peerRows)
		rows = append(rows, peerRows...)
	}
	if len(accountIDs) > 0 {
		var accountRows []snapshotRow
		s.db.Model(&model.UsageSnapshot{}).
			Where("protocol = ? AND account_id IN ? AND timestamp >= ?", model.UsageProtocolUserManager, accountIDs, windowStartUnix).
			Select("timestamp, total_bytes").Scan(&accountRows)
		rows = append(rows, accountRows...)
	}
	if len(packageIDs) > 0 {
		var packageRows []snapshotRow
		s.db.Model(&model.UsageSnapshot{}).
			Where("protocol = ? AND package_id IN ? AND timestamp >= ?", model.UsageProtocolV2Ray, packageIDs, windowStartUnix).
			Select("timestamp, total_bytes").Scan(&packageRows)
		rows = append(rows, packageRows...)
	}

	for _, row := range rows {
		dateKey := time.Unix(row.Timestamp, 0).In(tehran).Format("2006-01-02")
		if _, ok := totalsByDay[dateKey]; ok {
			totalsByDay[dateKey] += row.TotalBytes
		}
	}
	for i := range days {
		days[i].TotalBytes = totalsByDay[days[i].Date]
	}

	return &schema.AppWeeklyUsageResponse{Days: days}, nil
}
