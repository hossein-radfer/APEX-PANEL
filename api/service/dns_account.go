package service

import (
	"context"
	"errors"
	"fmt"
	"time"

	"github.com/google/uuid"
	"go.uber.org/zap"
	"gorm.io/gorm"

	"github.com/maahdima/mwp/api/adaptor/doctordns"
	"github.com/maahdima/mwp/api/common"
	"github.com/maahdima/mwp/api/dataservice/model"
	"github.com/maahdima/mwp/api/http/schema"
	"github.com/maahdima/mwp/api/utils"
)

// DNSAccountService manages Smart DNS accounts provisioned on a registered
// DNSPanel -- mirrors V2RayPackageService's role, permission/quota
// conventions and API shape, but WITHOUT V2Ray's multi-panel fan-out: a
// DNSAccount lives on exactly one DNSPanel, chosen at creation (see
// model.DNSPanel's own doc comment for why doctor-dns's architecture makes
// fan-out meaningless here). That single simplification is what removes the
// concurrency/reconciliation machinery V2RayPackageService needs (no
// per-location goroutines, no "add/remove one location" reconcile step).
type DNSAccountService struct {
	db             *gorm.DB
	panels         *DNSPanelService
	plans          *DNSPlanService
	geoIP          *GeoIPService
	auditLog       *AuditLog
	botNotifier    *BotNotifier
	licenseLimiter freeTierLimiter
	logger         *zap.Logger
}

func NewDNSAccountService(db *gorm.DB, panels *DNSPanelService, geoIP *GeoIPService, auditLog *AuditLog) *DNSAccountService {
	return &DNSAccountService{
		db:       db,
		panels:   panels,
		geoIP:    geoIP,
		auditLog: auditLog,
		logger:   zap.L().Named("DNSAccountService"),
	}
}

// SetBotNotifier wires the Telegram notifier after construction, mirroring
// V2RayPackageService.SetBotNotifier -- safe to leave unset.
func (s *DNSAccountService) SetBotNotifier(notifier *BotNotifier) {
	s.botNotifier = notifier
}

// SetLicenseLimiter wires the free-tier cap source after construction --
// mirrors V2RayPackageService.SetLicenseLimiter exactly. Safe to leave
// unset (nil licenseLimiter disables enforcement entirely).
func (s *DNSAccountService) SetLicenseLimiter(limiter freeTierLimiter) {
	s.licenseLimiter = limiter
}

// ErrFreeTierDNSAccountLimitReached is returned by CreateAccount when the
// license is Restricted and the configured "max_dns_accounts" cap has
// already been reached.
var ErrFreeTierDNSAccountLimitReached = errors.New("free tier limit reached: maximum dns accounts")

// SuspendOldestForFreeTier mirrors WgPeer.SuspendOldestForFreeTier exactly
// (see its own doc comment for the full DB-only, external-call-deferred
// rationale) -- the oldest excess DNSAccount rows (by CreatedAt) beyond cap
// are force-suspended at the DB level via the same
// Status/SuspendedByQuota/WasActiveBeforeSuspend columns this feature's
// existing quota-suspend paths already write, so the periodic DNS-sync job
// reconciles the actual doctor-dns account state on its own next tick.
func (s *DNSAccountService) SuspendOldestForFreeTier(cap int64) (int, error) {
	if cap <= 0 {
		return 0, nil
	}

	var activeCount int64
	if err := s.db.Model(&model.DNSAccount{}).Where("status = ?", "active").Count(&activeCount).Error; err != nil {
		return 0, err
	}

	excess := activeCount - cap
	if excess <= 0 {
		return 0, nil
	}

	var toSuspend []model.DNSAccount
	if err := s.db.Where("status = ?", "active").Order("created_at ASC").Limit(int(excess)).Find(&toSuspend).Error; err != nil {
		return 0, err
	}

	suspended := 0
	for _, acct := range toSuspend {
		if err := s.db.Model(&model.DNSAccount{}).Where("id = ?", acct.ID).Updates(map[string]interface{}{
			"status":                    "suspended",
			"suspended_by_quota":        true,
			"was_active_before_suspend": true,
		}).Error; err != nil {
			s.logger.Error("failed to suspend dns account for free-tier cap", zap.Uint("account_id", acct.ID), zap.Error(err))
			continue
		}
		suspended++
	}

	return suspended, nil
}

// SetPlanService wires DNSPlanService after construction, same
// after-the-fact-wiring convention as SetBotNotifier -- safe to leave
// unset (PlanID on create/update requests is then simply ignored rather
// than erroring, so an install that never defines any DNSPlan rows keeps
// working exactly as before this feature existed).
func (s *DNSAccountService) SetPlanService(plans *DNSPlanService) {
	s.plans = plans
}

// applyPlanIfSet, when planID is non-nil and s.plans is wired, fetches that
// DNSPlan and copies its bundle into the given request pointers -- but ONLY
// into fields the caller left at their zero value, so anything the admin
// explicitly typed in the same request always wins (phase 4-3's "allow
// manual override in special cases" requirement). Returns silently (no
// error) if the plan doesn't exist or the service isn't wired, since a
// missing/stale plan reference should never block account creation/update
// -- the request's own explicit field values are always sufficient on
// their own.
func (s *DNSAccountService) applyPlanIfSet(planID *uint, totalVolumeBytes *int64, speedKbps, durationDays, maxConcurrentIPs, dailyIPRegistrationLimit *int) {
	if planID == nil || s.plans == nil {
		return
	}
	plan, err := s.plans.GetPlan(*planID)
	if err != nil {
		s.logger.Warn("DNS plan referenced by account request not found, ignoring", zap.Uint("plan_id", *planID), zap.Error(err))
		return
	}
	if totalVolumeBytes != nil && *totalVolumeBytes == 0 {
		*totalVolumeBytes = plan.TotalVolumeBytes
	}
	if speedKbps != nil && *speedKbps == 0 {
		*speedKbps = plan.SpeedKbps
	}
	if durationDays != nil && *durationDays == 0 {
		*durationDays = plan.DurationDays
	}
	if maxConcurrentIPs != nil && *maxConcurrentIPs == 0 {
		*maxConcurrentIPs = plan.MaxConcurrentIPs
	}
	if dailyIPRegistrationLimit != nil && *dailyIPRegistrationLimit == 0 {
		*dailyIPRegistrationLimit = plan.DailyIPRegistrationLimit
	}
}

func (s *DNSAccountService) logResellerAction(resellerID *uint, action, description string) {
	if resellerID == nil {
		return
	}
	var reseller model.Reseller
	name := "unknown"
	if err := s.db.Select("name").First(&reseller, *resellerID).Error; err == nil {
		name = reseller.Name
	}
	if s.auditLog != nil {
		s.auditLog.Log(*resellerID, name, action, description)
	}
	if action == AuditActionDNSAccountCreated && s.botNotifier != nil {
		s.botNotifier.NotifyLiveLog(fmt.Sprintf("%s: %s", name, description))
	}
}

// ensureResellerCanResellDNS mirrors ensureResellerCanResellV2Ray exactly.
func (s *DNSAccountService) ensureResellerCanResellDNS(resellerID uint) error {
	var reseller model.Reseller
	if err := s.db.First(&reseller, resellerID).Error; err != nil {
		return err
	}
	if !reseller.CanResellDNS {
		return fmt.Errorf("reseller is not permitted to resell DNS accounts")
	}
	return nil
}

// ensureResellerUnderDNSAccountLimit mirrors
// ensureResellerUnderV2RayPackageLimit exactly.
func (s *DNSAccountService) ensureResellerUnderDNSAccountLimit(resellerID uint) error {
	var reseller model.Reseller
	if err := s.db.First(&reseller, resellerID).Error; err != nil {
		return err
	}
	if reseller.DNSMaxAccounts == nil {
		return nil
	}
	var count int64
	if err := s.db.Model(&model.DNSAccount{}).Where("reseller_id = ?", resellerID).Count(&count).Error; err != nil {
		return err
	}
	if int(count) >= *reseller.DNSMaxAccounts {
		return fmt.Errorf("reseller has reached its maximum allowed DNS accounts (%d)", *reseller.DNSMaxAccounts)
	}
	return nil
}

// SetAssignedDNSPanels replaces the full set of DNS panels resellerID may
// use -- mirrors V2RayPackageService.SetAssignedXuiPanels exactly.
func (s *DNSAccountService) SetAssignedDNSPanels(resellerID uint, panelIDs []uint) error {
	var reseller model.Reseller
	if err := s.db.First(&reseller, resellerID).Error; err != nil {
		return err
	}

	return s.db.Transaction(func(tx *gorm.DB) error {
		if err := tx.Unscoped().Where("reseller_id = ?", resellerID).Delete(&model.ResellerDNSPanelAccess{}).Error; err != nil {
			return err
		}
		if len(panelIDs) == 0 {
			return nil
		}
		links := make([]model.ResellerDNSPanelAccess, 0, len(panelIDs))
		for _, panelID := range panelIDs {
			links = append(links, model.ResellerDNSPanelAccess{ResellerID: resellerID, PanelID: panelID})
		}
		return tx.Create(&links).Error
	})
}

// GetAssignedDNSPanels returns the panel IDs resellerID is allowed to use.
func (s *DNSAccountService) GetAssignedDNSPanels(resellerID uint) ([]uint, error) {
	var links []model.ResellerDNSPanelAccess
	if err := s.db.Where("reseller_id = ?", resellerID).Find(&links).Error; err != nil {
		return nil, err
	}
	panelIDs := make([]uint, 0, len(links))
	for _, link := range links {
		panelIDs = append(panelIDs, link.PanelID)
	}
	return panelIDs, nil
}

// GetAssignedDNSPanelSummaries mirrors GetAssignedXuiPanelSummaries exactly
// -- id+name only, safe for a reseller session to receive.
func (s *DNSAccountService) GetAssignedDNSPanelSummaries(resellerID uint) ([]model.DNSPanel, error) {
	panelIDs, err := s.GetAssignedDNSPanels(resellerID)
	if err != nil {
		return nil, err
	}
	if len(panelIDs) == 0 {
		return []model.DNSPanel{}, nil
	}
	var panels []model.DNSPanel
	if err := s.db.Where("id IN ?", panelIDs).Find(&panels).Error; err != nil {
		return nil, err
	}
	return panels, nil
}

// resolveEffectivePanel implements CreateAccount's panel-selection rule --
// the single-panel counterpart to resolveEffectivePanels: an admin-direct
// account may use any registered panel; a reseller account's requested
// panel must be one they were explicitly granted via ResellerDNSPanelAccess
// (mirrors ResellerXuiPanelAccess's "must be assigned, no implicit access"
// rule).
func (s *DNSAccountService) resolveEffectivePanel(resellerID *uint, requestedPanelID uint) (model.DNSPanel, error) {
	panel, err := s.panels.GetPanel(requestedPanelID)
	if err != nil {
		return model.DNSPanel{}, fmt.Errorf("panel id %d does not exist", requestedPanelID)
	}
	if resellerID == nil {
		return panel, nil
	}
	assignedIDs, err := s.GetAssignedDNSPanels(*resellerID)
	if err != nil {
		return model.DNSPanel{}, fmt.Errorf("failed to look up assigned panels: %w", err)
	}
	for _, id := range assignedIDs {
		if id == requestedPanelID {
			return panel, nil
		}
	}
	return model.DNSPanel{}, fmt.Errorf("reseller is not permitted to use panel id %d", requestedPanelID)
}

// TestPanelForReseller is the reseller-scoped counterpart to
// DNSPanelService.TestConnection (which is admin-only server-side,
// mirroring XuiPanelController's identical convention) -- a confirmed,
// reported bug: the account-creation wizard's own "بارگذاری پلن‌ها" (load
// plans) button called that admin-only endpoint unconditionally for EVERY
// caller, so a reseller creating their own DNS account always got a 403
// there. This reuses resolveEffectivePanel's exact same "must be one of
// this reseller's ResellerDNSPanelAccess grants" check before delegating
// to the real test-connection call, so a reseller can only ever probe a
// panel they were actually assigned -- never an arbitrary panel ID.
func (s *DNSAccountService) TestPanelForReseller(resellerID uint, panelID uint) (*schema.TestDNSConnectionResponse, error) {
	if _, err := s.resolveEffectivePanel(&resellerID, panelID); err != nil {
		return nil, err
	}
	return s.panels.TestConnection(panelID)
}

func (s *DNSAccountService) getAccountByIDScoped(id uint, resellerID *uint) (model.DNSAccount, error) {
	var acct model.DNSAccount
	query := s.db.Where("id = ?", id)
	if resellerID != nil {
		query = query.Where("reseller_id = ?", *resellerID)
	}
	if err := query.First(&acct).Error; err != nil {
		return model.DNSAccount{}, err
	}
	return acct, nil
}

// EnsureAccountAccess mirrors V2RayPackageService.EnsurePackageAccess.
func (s *DNSAccountService) EnsureAccountAccess(id uint, resellerID *uint) error {
	_, err := s.getAccountByIDScoped(id, resellerID)
	return err
}

// expireDaysPointer converts a whole-day int into doctor-dns's own
// expire_days float shape -- 0 is sent as nil (never expires), matching
// smartdns-panel's do_apex_user_create: it only sets an expiry when
// days > 0.
func expireDaysPointer(durationDays int) *float64 {
	if durationDays <= 0 {
		return nil
	}
	f := float64(durationDays)
	return &f
}

// generateApexRef builds the opaque key sent to doctor-dns as apex_ref --
// scoped by reseller (or "admin") so two resellers sharing the same
// DNSPanel can never collide, per the admin's own explicit requirement (see
// DNSAccount.ApexRef's doc comment). Suffixed with a short random UUID
// fragment, not the account's own DB id, so this can be computed BEFORE the
// row is inserted (needed since ApexRef is itself a column on that row).
func generateApexRef(resellerID *uint) string {
	scope := "admin"
	if resellerID != nil {
		scope = fmt.Sprintf("r%d", *resellerID)
	}
	short := uuid.New().String()
	if len(short) > 8 {
		short = short[:8]
	}
	return fmt.Sprintf("apex:%s:dns:%s", scope, short)
}

// CreateAccount creates a DNSAccount and provisions it on the resolved
// panel synchronously. Unlike V2RayPackageService.CreatePackage's
// concurrent multi-panel fan-out, there is exactly one remote call here --
// if it fails, the local row is still created (mirroring the "one dead
// panel never blocks the record from existing" principle) with
// LastSyncError set, Status left as requested, and RemoteStatus empty; the
// background sync job's next tick retries it.
func (s *DNSAccountService) CreateAccount(req *schema.CreateDNSAccountRequest, resellerID *uint) (*schema.DNSAccountResponse, error) {
	if s.licenseLimiter != nil {
		if maxAccounts, ok := s.licenseLimiter.GetFreeTierLimit("max_dns_accounts"); ok {
			var count int64
			if err := s.db.Model(&model.DNSAccount{}).Count(&count).Error; err != nil {
				return nil, err
			}
			if count >= maxAccounts {
				return nil, ErrFreeTierDNSAccountLimitReached
			}
		}
	}

	if resellerID != nil {
		if err := s.ensureResellerCanResellDNS(*resellerID); err != nil {
			return nil, err
		}
		if err := s.ensureResellerUnderDNSAccountLimit(*resellerID); err != nil {
			return nil, err
		}
	}

	panel, err := s.resolveEffectivePanel(resellerID, req.PanelID)
	if err != nil {
		return nil, err
	}

	s.applyPlanIfSet(req.PlanID, &req.TotalVolumeBytes, &req.SpeedKbps, &req.DurationDays, &req.MaxConcurrentIPs, &req.DailyIPRegistrationLimit)

	maxIPs := req.MaxConcurrentIPs
	if maxIPs < 1 {
		maxIPs = 1
	}

	now := time.Now()
	var expireAt *time.Time
	if req.DurationDays > 0 {
		t := now.AddDate(0, 0, req.DurationDays)
		expireAt = &t
	}

	acct := model.DNSAccount{
		UUID:                     uuid.New().String(),
		PanelID:                  panel.ID,
		ApexRef:                  generateApexRef(resellerID),
		CustomerLabel:            req.CustomerLabel,
		Comment:                  req.Comment,
		ResellerID:               resellerID,
		DNSPlanID:                req.PlanID,
		TotalVolumeBytes:         req.TotalVolumeBytes,
		SpeedKbps:                req.SpeedKbps,
		DurationDays:             req.DurationDays,
		StartAt:                  &now,
		ExpireAt:                 expireAt,
		TemplateID:               req.TemplateID,
		Status:                   "active",
		MaxConcurrentIPs:         maxIPs,
		DailyIPRegistrationLimit: req.DailyIPRegistrationLimit,
	}

	if err := s.db.Create(&acct).Error; err != nil {
		s.logger.Error("failed to store DNS account", zap.Error(err))
		return nil, fmt.Errorf("failed to create DNS account: %w", err)
	}

	if err := s.replaceAllowedCountries(acct.ID, req.AllowedCountries); err != nil {
		s.logger.Warn("failed to store allowed countries for new DNS account", zap.Uint("account_id", acct.ID), zap.Error(err))
	}

	label := ""
	if acct.CustomerLabel != nil {
		label = *acct.CustomerLabel
	}
	ctx, cancel := context.WithTimeout(context.Background(), 20*time.Second)
	defer cancel()
	remoteUser, provisionErr := doctordns.CreateOrUpdateUser(ctx, panel, acct.ApexRef, label,
		req.TotalVolumeBytes, req.SpeedKbps, expireDaysPointer(req.DurationDays), req.TemplateID, maxIPs)
	if provisionErr != nil {
		s.logger.Warn("failed to provision DNS account on panel, will be retried by the next sync tick",
			zap.Uint("panel_id", panel.ID), zap.String("apex_ref", acct.ApexRef), zap.Error(provisionErr))
		errStr := provisionErr.Error()
		if updErr := s.db.Model(&acct).Update("last_sync_error", errStr).Error; updErr != nil {
			s.logger.Error("failed to persist DNS account provisioning error", zap.Error(updErr))
		}
	} else {
		updates := map[string]interface{}{"last_sync_error": nil, "remote_status": remoteUser.Status}
		if remoteUser.IP != nil {
			updates["current_ip"] = *remoteUser.IP
		}
		syncedAt := time.Now()
		updates["last_synced_at"] = &syncedAt
		if updErr := s.db.Model(&acct).Updates(updates).Error; updErr != nil {
			s.logger.Error("failed to persist DNS account provisioning result", zap.Error(updErr))
		}
	}

	customerLabel := "بدون‌نام"
	if acct.CustomerLabel != nil && *acct.CustomerLabel != "" {
		customerLabel = *acct.CustomerLabel
	}
	volumeGB := float64(acct.TotalVolumeBytes) / (1024 * 1024 * 1024)
	s.logResellerAction(resellerID, AuditActionDNSAccountCreated, fmt.Sprintf(
		"ساخت اکانت DNS برای %s | %.1f گیگابایت | %d روز (uuid=%s)",
		customerLabel, volumeGB, acct.DurationDays, acct.UUID,
	))

	return s.transformAccountToResponse(acct)
}

// replaceAllowedCountries fully replaces accountID's allowed-country set --
// an empty/nil slice clears every row (meaning "unrestricted", per
// DNSAccountAllowedCountry's own doc comment on that convention).
func (s *DNSAccountService) replaceAllowedCountries(accountID uint, countries []string) error {
	return s.db.Transaction(func(tx *gorm.DB) error {
		if err := tx.Unscoped().Where("dns_account_id = ?", accountID).Delete(&model.DNSAccountAllowedCountry{}).Error; err != nil {
			return err
		}
		if len(countries) == 0 {
			return nil
		}
		rows := make([]model.DNSAccountAllowedCountry, 0, len(countries))
		seen := make(map[string]struct{}, len(countries))
		for _, c := range countries {
			if c == "" {
				continue
			}
			if _, dup := seen[c]; dup {
				continue
			}
			seen[c] = struct{}{}
			rows = append(rows, model.DNSAccountAllowedCountry{DNSAccountID: accountID, CountryName: c})
		}
		if len(rows) == 0 {
			return nil
		}
		return tx.Create(&rows).Error
	})
}

func (s *DNSAccountService) getAllowedCountries(accountID uint) ([]string, error) {
	var rows []model.DNSAccountAllowedCountry
	if err := s.db.Where("dns_account_id = ?", accountID).Order("country_name").Find(&rows).Error; err != nil {
		return nil, err
	}
	countries := make([]string, 0, len(rows))
	for _, r := range rows {
		countries = append(countries, r.CountryName)
	}
	return countries, nil
}

// UpdateAccount updates an account's fields and, when any provisioning-
// relevant field changes, pushes the new state to doctor-dns synchronously
// -- mirrors V2RayPackageService.UpdatePackage's "push to the real device
// before returning" convention, simplified to one panel/one call instead of
// a location reconcile.
func (s *DNSAccountService) UpdateAccount(id uint, req *schema.UpdateDNSAccountRequest, resellerID *uint) (*schema.DNSAccountResponse, error) {
	acct, err := s.getAccountByIDScoped(id, resellerID)
	if err != nil {
		return nil, fmt.Errorf("DNS account not found: %w", err)
	}

	if req.CustomerLabel != nil {
		acct.CustomerLabel = req.CustomerLabel
	}
	if req.Comment != nil {
		acct.Comment = req.Comment
	}

	// PlanID fills in ONLY the fields this same request left nil (never
	// overriding a field the admin explicitly set in this request, and
	// never touching fields already on the account that this request
	// doesn't mention at all) -- mirrors CreateAccount's identical
	// "explicit request field always wins" rule, adapted for Update's
	// nil-means-unchanged (rather than Create's zero-means-unset)
	// convention.
	if req.PlanID != nil && s.plans != nil {
		if plan, planErr := s.plans.GetPlan(*req.PlanID); planErr != nil {
			s.logger.Warn("DNS plan referenced by account update not found, ignoring", zap.Uint("plan_id", *req.PlanID), zap.Error(planErr))
		} else {
			acct.DNSPlanID = req.PlanID
			if req.TotalVolumeBytes == nil {
				v := plan.TotalVolumeBytes
				req.TotalVolumeBytes = &v
			}
			if req.SpeedKbps == nil {
				v := plan.SpeedKbps
				req.SpeedKbps = &v
			}
			if req.DurationDays == nil {
				v := plan.DurationDays
				req.DurationDays = &v
			}
			if req.MaxConcurrentIPs == nil {
				v := plan.MaxConcurrentIPs
				req.MaxConcurrentIPs = &v
			}
			if req.DailyIPRegistrationLimit == nil {
				v := plan.DailyIPRegistrationLimit
				req.DailyIPRegistrationLimit = &v
			}
		}
	}

	provisioningChanged := false
	if req.TotalVolumeBytes != nil && *req.TotalVolumeBytes != acct.TotalVolumeBytes {
		acct.TotalVolumeBytes = *req.TotalVolumeBytes
		provisioningChanged = true
	}
	if req.SpeedKbps != nil && *req.SpeedKbps != acct.SpeedKbps {
		acct.SpeedKbps = *req.SpeedKbps
		provisioningChanged = true
	}
	if req.TemplateID != nil {
		acct.TemplateID = req.TemplateID
		provisioningChanged = true
	}
	if req.MaxConcurrentIPs != nil && *req.MaxConcurrentIPs >= 1 && *req.MaxConcurrentIPs != acct.MaxConcurrentIPs {
		acct.MaxConcurrentIPs = *req.MaxConcurrentIPs
		provisioningChanged = true
	}
	if req.DailyIPRegistrationLimit != nil {
		acct.DailyIPRegistrationLimit = *req.DailyIPRegistrationLimit
	}
	if req.DurationDays != nil {
		acct.DurationDays = *req.DurationDays
		startAt := time.Now()
		if acct.StartAt != nil {
			startAt = *acct.StartAt
		}
		if acct.DurationDays > 0 {
			t := startAt.AddDate(0, 0, acct.DurationDays)
			acct.ExpireAt = &t
		} else {
			acct.ExpireAt = nil
		}
		provisioningChanged = true
	}

	statusChanged := false
	if req.Status != nil && *req.Status != acct.Status {
		acct.Status = *req.Status
		statusChanged = true
		if acct.Status == "active" {
			acct.SuspendedByQuota = false
			acct.WasActiveBeforeSuspend = false
			acct.SuspendedByResellerQuota = false
		}
	}

	if req.AllowedCountries != nil {
		if err := s.replaceAllowedCountries(acct.ID, req.AllowedCountries); err != nil {
			s.logger.Warn("failed to update allowed countries", zap.Uint("account_id", acct.ID), zap.Error(err))
		}
	}

	if err := s.db.Save(&acct).Error; err != nil {
		s.logger.Error("failed to update DNS account", zap.Error(err))
		return nil, fmt.Errorf("failed to update DNS account: %w", err)
	}

	if provisioningChanged || statusChanged {
		s.pushAccountStateToPanel(acct)
	}

	return s.transformAccountToResponse(acct)
}

// pushAccountStateToPanel re-sends acct's full desired state to doctor-dns
// -- reused by both UpdateAccount (a provisioning-relevant field changed)
// and the sync job's quota-enforcement path. Best-effort: a failure is
// logged and left on LastSyncError for the next tick to retry, matching
// every other doctor-dns call in this service.
func (s *DNSAccountService) pushAccountStateToPanel(acct model.DNSAccount) {
	panel, err := s.panels.GetPanel(acct.PanelID)
	if err != nil {
		s.logger.Warn("DNS panel no longer exists, cannot push account state", zap.Uint("panel_id", acct.PanelID), zap.Error(err))
		return
	}

	label := ""
	if acct.CustomerLabel != nil {
		label = *acct.CustomerLabel
	}

	ctx, cancel := context.WithTimeout(context.Background(), 20*time.Second)
	defer cancel()

	var remoteErr error
	var remoteUser *doctordns.User
	if acct.Status == "active" {
		remoteUser, remoteErr = doctordns.CreateOrUpdateUser(ctx, panel, acct.ApexRef, label,
			acct.TotalVolumeBytes, acct.SpeedKbps, expireDaysPointer(acct.DurationDays), acct.TemplateID, acct.MaxConcurrentIPs)
		// CreateOrUpdateUser's upsert deliberately never clears a suspended
		// status on doctor-dns's side (see create_apex_user's doc comment) --
		// so reactivating an account that doctor-dns still has suspended
		// (from an earlier explicit suspend) needs this separate call, or
		// its IP never gets synced back into the relay's allowlist.
		if remoteErr == nil {
			remoteUser, remoteErr = doctordns.SetUserStatus(ctx, panel, acct.ApexRef, "active")
		}
	} else {
		remoteUser, remoteErr = doctordns.SetUserStatus(ctx, panel, acct.ApexRef, "suspended")
	}

	updates := map[string]interface{}{}
	if remoteErr != nil {
		s.logger.Warn("failed to push DNS account state to panel", zap.Uint("account_id", acct.ID), zap.Error(remoteErr))
		errStr := remoteErr.Error()
		updates["last_sync_error"] = errStr
	} else {
		updates["last_sync_error"] = nil
		updates["remote_status"] = remoteUser.Status
		if remoteUser.IP != nil {
			updates["current_ip"] = *remoteUser.IP
		}
		syncedAt := time.Now()
		updates["last_synced_at"] = &syncedAt
	}
	if err := s.db.Model(&model.DNSAccount{}).Where("id = ?", acct.ID).Updates(updates).Error; err != nil {
		s.logger.Error("failed to persist DNS account push result", zap.Uint("account_id", acct.ID), zap.Error(err))
	}
}

// ResetUsage mirrors V2RayPackageService.ResetUsage's offset mechanism for
// DNS's single UsedBytesCached field, but unlike V2Ray it also has to clear
// the counter doctor-dns enforces quota against on its own side: doctor-dns
// decides over_quota by comparing ITS used_bytes column, never anything this
// service sends, so an offset that only changes what this panel displays
// leaves an over-quota account exactly as over quota there, and the next
// sync tick re-suspends it even right after this call. Best-effort like
// every other doctor-dns push in this service: a failure here is logged and
// left for the operator to retry, since the local offset reset below still
// happened and is the only immediately-visible half of this operation.
//
// A confirmed, reported bug this also fixes: this only ever flipped Status
// back to "active" when doctor-dns's OWN response already reported
// "active" -- but SuspendedByQuota/WasActiveBeforeSuspend/
// SuspendedByResellerQuota (the local flags applyResellerDNSQuota's own
// suspend/resume logic actually reads) were never cleared here at all, on
// either branch. Left stuck true, they made this exact account look
// "still suspended by quota" to the next quota sync tick regardless of what
// doctor-dns just reported, silently re-suspending it moments after an
// admin's manual reset appeared to succeed -- mirrors the identical gap
// just fixed in V2RayPackageService.ResetUsage. Cleared unconditionally
// whenever this account was actually quota-suspended locally, independent
// of the best-effort remote call's own outcome above (the local reset is
// the operator's real intent; the remote push is a courtesy sync, not a
// precondition for it).
func (s *DNSAccountService) ResetUsage(id uint, resellerID *uint) error {
	acct, err := s.getAccountByIDScoped(id, resellerID)
	if err != nil {
		return fmt.Errorf("DNS account not found: %w", err)
	}

	updates := map[string]interface{}{"usage_offset_bytes": acct.UsedBytesCached}
	wasSuspendedByQuota := acct.SuspendedByQuota
	if wasSuspendedByQuota {
		updates["status"] = "active"
		updates["suspended_by_quota"] = false
		updates["was_active_before_suspend"] = false
		updates["suspended_by_reseller_quota"] = false
	}
	if err := s.db.Model(&model.DNSAccount{}).Where("id = ?", acct.ID).Updates(updates).Error; err != nil {
		s.logger.Error("failed to reset DNS account usage", zap.Uint("account_id", acct.ID), zap.Error(err))
		return fmt.Errorf("failed to reset DNS account usage: %w", err)
	}

	if panel, panelErr := s.panels.GetPanel(acct.PanelID); panelErr == nil {
		ctx, cancel := context.WithTimeout(context.Background(), 20*time.Second)
		remoteUser, remoteErr := doctordns.ResetUsage(ctx, panel, acct.ApexRef)
		cancel()
		if remoteErr != nil {
			s.logger.Warn("failed to reset DNS account usage on panel", zap.Uint("account_id", acct.ID), zap.Error(remoteErr))
		} else if remoteUser.Status == "active" && acct.Status != "active" && !wasSuspendedByQuota {
			if err := s.db.Model(&model.DNSAccount{}).Where("id = ?", acct.ID).
				Updates(map[string]interface{}{"status": "active", "remote_status": remoteUser.Status}).Error; err != nil {
				s.logger.Error("failed to persist DNS account status after usage reset", zap.Uint("account_id", acct.ID), zap.Error(err))
			}
		}
	} else {
		s.logger.Warn("DNS panel no longer exists, cannot reset usage on panel", zap.Uint("panel_id", acct.PanelID), zap.Error(panelErr))
	}

	return nil
}

// DeleteAccount removes acct on doctor-dns (best-effort) and locally,
// folding its usage into the owning reseller's DNSDeletedUsageBytes credit
// first -- mirrors V2RayPackageService.DeletePackage's identical fix for
// the same "usage disappears from the reseller's live-sum total" bug.
func (s *DNSAccountService) DeleteAccount(id uint, resellerID *uint) error {
	acct, err := s.getAccountByIDScoped(id, resellerID)
	if err != nil {
		return fmt.Errorf("DNS account not found: %w", err)
	}

	if panel, panelErr := s.panels.GetPanel(acct.PanelID); panelErr == nil {
		ctx, cancel := context.WithTimeout(context.Background(), 20*time.Second)
		defer cancel()
		if delErr := doctordns.DeleteUser(ctx, panel, acct.ApexRef); delErr != nil {
			s.logger.Warn("failed to delete DNS account on panel, continuing with local delete", zap.Uint("panel_id", panel.ID), zap.Error(delErr))
		}
	} else {
		s.logger.Warn("DNS panel no longer exists, skipping remote delete", zap.Uint("panel_id", acct.PanelID), zap.Error(panelErr))
	}

	if acct.ResellerID != nil && acct.UsedBytesCached > 0 {
		if err := s.db.Model(&model.Reseller{}).Where("id = ?", *acct.ResellerID).
			Update("dns_deleted_usage_bytes", gorm.Expr("dns_deleted_usage_bytes + ?", acct.UsedBytesCached)).Error; err != nil {
			s.logger.Error("failed to credit deleted DNS account's usage to reseller total", zap.Uint("reseller_id", *acct.ResellerID), zap.Error(err))
			return fmt.Errorf("failed to preserve deleted account's usage: %w", err)
		}
	}

	if err := s.db.Unscoped().Where("dns_account_id = ?", acct.ID).Delete(&model.DNSAccountAllowedCountry{}).Error; err != nil {
		s.logger.Warn("failed to delete allowed-country rows for DNS account", zap.Uint("account_id", acct.ID), zap.Error(err))
	}

	if err := s.db.Delete(&acct).Error; err != nil {
		s.logger.Error("failed to delete DNS account from database", zap.Error(err))
		return fmt.Errorf("failed to delete DNS account: %w", err)
	}

	s.logResellerAction(resellerID, AuditActionDNSAccountDeleted, fmt.Sprintf("Deleted DNS account (uuid=%s)", acct.UUID))
	return nil
}

// BulkDeleteAccounts mirrors V2RayPackageService.BulkDeletePackages exactly.
func (s *DNSAccountService) BulkDeleteAccounts(ids []uint, resellerID *uint) (deleted []uint, failed map[uint]string) {
	failed = make(map[uint]string)
	for _, id := range ids {
		if err := s.DeleteAccount(id, resellerID); err != nil {
			failed[id] = err.Error()
			continue
		}
		deleted = append(deleted, id)
	}
	return deleted, failed
}

// ListAccounts/ListAccountsByReseller mirror V2RayPackageService's identical
// scoping convention.
func (s *DNSAccountService) ListAccounts(resellerID *uint) ([]schema.DNSAccountResponse, error) {
	return s.listAccountsScoped(resellerID)
}

func (s *DNSAccountService) ListAccountsByReseller(resellerID uint) ([]schema.DNSAccountResponse, error) {
	return s.listAccountsScoped(&resellerID)
}

func (s *DNSAccountService) listAccountsScoped(resellerID *uint) ([]schema.DNSAccountResponse, error) {
	var accounts []model.DNSAccount
	query := s.db.Model(&model.DNSAccount{})
	if resellerID != nil {
		query = query.Where("reseller_id = ?", *resellerID)
	} else {
		query = query.Where("reseller_id IS NULL")
	}
	if err := query.Order("id desc").Find(&accounts).Error; err != nil {
		s.logger.Error("failed to fetch DNS accounts", zap.Error(err))
		return nil, fmt.Errorf("failed to fetch DNS accounts: %w", err)
	}

	resp := make([]schema.DNSAccountResponse, 0, len(accounts))
	for _, acct := range accounts {
		r, err := s.transformAccountToResponse(acct)
		if err != nil {
			s.logger.Warn("failed to transform DNS account to response, skipping", zap.Uint("id", acct.ID), zap.Error(err))
			continue
		}
		resp = append(resp, *r)
	}
	return resp, nil
}

// isAccountConnected implements the "online" proxy discussed with the
// admin: doctor-dns exposes no real-time connection signal at all (no
// equivalent of x-ui's /onlines poll), so this is deliberately a coarse
// stand-in -- "has a registered IP and is currently allowed to use it" --
// not "traffic is flowing right now". Surfaced to the UI as IsOnline but
// should be labeled in a way that doesn't overpromise a live signal that
// doesn't exist.
func isAccountConnected(acct model.DNSAccount) bool {
	if acct.Status != "active" || acct.CurrentIP == nil {
		return false
	}
	if acct.ExpireAt != nil && acct.ExpireAt.Before(time.Now()) {
		return false
	}
	return true
}

// GetAccountShareDetails is the public, unauthenticated sub-page's data
// source -- a pure DB read, mirrors GetPackageShareDetails's identical
// contract (never calls doctor-dns live).
func (s *DNSAccountService) GetAccountShareDetails(uuidStr string) (*schema.DNSAccountShareDetailsResponse, error) {
	var acct model.DNSAccount
	if err := s.db.First(&acct, "uuid = ?", uuidStr).Error; err != nil {
		return nil, err
	}
	if !utils.IsPeerSharable(acct.IsShared, acct.ShareExpireTime) {
		return nil, common.ErrDNSAccountNotShared
	}

	planTitle := "Smart DNS"
	if panel, err := s.panels.GetPanel(acct.PanelID); err == nil {
		planTitle = panel.SaleTitle
	}

	var priceAmount *int64
	if acct.DNSPlanID != nil && s.plans != nil {
		if plan, err := s.plans.GetPlan(*acct.DNSPlanID); err == nil {
			priceAmount = &plan.PriceAmount
		}
	}

	displayedUsedBytes := acct.UsedBytesCached - acct.UsageOffsetBytes
	if displayedUsedBytes < 0 {
		displayedUsedBytes = 0
	}

	var usagePercent *string
	if acct.TotalVolumeBytes > 0 {
		percent := float64(displayedUsedBytes) / float64(acct.TotalVolumeBytes) * 100
		usagePercent = utils.Ptr(fmt.Sprintf("%.1f", percent))
	}

	var expireAt *string
	var daysRemaining *int
	if acct.ExpireAt != nil {
		formatted := acct.ExpireAt.Format("2006-01-02")
		expireAt = &formatted
		days := int(time.Until(*acct.ExpireAt).Hours() / 24)
		if days < 0 {
			days = 0
		}
		daysRemaining = &days
	}

	registrationsToday, err := s.countRegistrationsToday(acct.ID)
	if err != nil {
		s.logger.Warn("failed to count today's DNS IP registrations", zap.Uint("account_id", acct.ID), zap.Error(err))
	}

	countries, err := s.getAllowedCountries(acct.ID)
	if err != nil {
		s.logger.Warn("failed to load allowed countries", zap.Uint("account_id", acct.ID), zap.Error(err))
	}

	return &schema.DNSAccountShareDetailsResponse{
		CustomerLabel:            acct.CustomerLabel,
		PlanTitle:                planTitle,
		PriceAmount:              priceAmount,
		TotalVolumeBytes:         acct.TotalVolumeBytes,
		SpeedKbps:                acct.SpeedKbps,
		UsedBytes:                displayedUsedBytes,
		UsagePercent:             usagePercent,
		ExpireAt:                 expireAt,
		DaysRemaining:            daysRemaining,
		Status:                   acct.Status,
		IsOnline:                 isAccountConnected(acct),
		CurrentIP:                acct.CurrentIP,
		MaxConcurrentIPs:         acct.MaxConcurrentIPs,
		DailyIPRegistrationLimit: acct.DailyIPRegistrationLimit,
		RegistrationsUsedToday:   registrationsToday,
		AllowedCountries:         countries,
	}, nil
}

// countRegistrationsToday counts ACCEPTED registration attempts since local
// midnight -- the daily-cap window is calendar-day/server-local-time, per
// the admin's own confirmed choice of "one registration = one slot off the
// limit" (a re-registration of the SAME ip is not special-cased: doctor-dns
// itself does not distinguish "new IP" from "same IP again" at the protocol
// level either, see do_claim_register's own INSERT OR REPLACE).
func (s *DNSAccountService) countRegistrationsToday(accountID uint) (int, error) {
	now := time.Now()
	startOfDay := time.Date(now.Year(), now.Month(), now.Day(), 0, 0, 0, 0, now.Location())
	var count int64
	err := s.db.Model(&model.DNSIPRegistrationLog{}).
		Where("dns_account_id = ? AND accepted = ? AND attempted_at >= ?", accountID, true, startOfDay).
		Count(&count).Error
	return int(count), err
}

// RegisterCustomerIP is the public sub-page's "register my IP" action --
// the daily-cap and geo-fence checks happen HERE, entirely on the Apex
// side, before doctor-dns is ever called: a rejected attempt never reaches
// the remote panel at all, matching the admin's explicit design ("doctor-dns
// itself needs no daily-limit or geo concept -- Apex is already in the loop
// for every registration coming from its own sub page").
func (s *DNSAccountService) RegisterCustomerIP(uuidStr, ip string) (*schema.RegisterDNSIPResponse, error) {
	var acct model.DNSAccount
	if err := s.db.First(&acct, "uuid = ?", uuidStr).Error; err != nil {
		return nil, err
	}
	if !utils.IsPeerSharable(acct.IsShared, acct.ShareExpireTime) {
		return nil, common.ErrDNSAccountNotShared
	}
	if acct.Status != "active" {
		return &schema.RegisterDNSIPResponse{Ok: false, Message: "این اکانت غیرفعال است"}, nil
	}

	if acct.DailyIPRegistrationLimit > 0 {
		usedToday, err := s.countRegistrationsToday(acct.ID)
		if err != nil {
			return nil, fmt.Errorf("failed to check daily registration count: %w", err)
		}
		if usedToday >= acct.DailyIPRegistrationLimit {
			s.writeRegistrationLog(acct.ID, ip, nil, false, "daily registration limit reached")
			return &schema.RegisterDNSIPResponse{Ok: false, Message: "سقف ثبت آی‌پی امروز پر شده است"}, nil
		}
	}

	var countryPtr *string
	allowedCountries, err := s.getAllowedCountries(acct.ID)
	if err != nil {
		return nil, fmt.Errorf("failed to load allowed countries: %w", err)
	}
	if len(allowedCountries) > 0 {
		if s.geoIP == nil {
			s.writeRegistrationLog(acct.ID, ip, nil, false, "geo lookup unavailable")
			return &schema.RegisterDNSIPResponse{Ok: false, Message: "بررسی موقعیت جغرافیایی در دسترس نیست"}, nil
		}
		result, lookupErr := s.geoIP.Lookup(ip)
		if lookupErr != nil || result.Country == nil {
			// Fail-closed: an unresolved country on a geo-fenced account is
			// rejected, per the admin's explicit choice -- see
			// DNSAccountAllowedCountry's own doc comment for why this is the
			// safer default for a security control.
			s.writeRegistrationLog(acct.ID, ip, nil, false, "could not determine IP's country")
			return &schema.RegisterDNSIPResponse{Ok: false, Message: "کشور این آی‌پی قابل تشخیص نیست و اجازه ثبت داده نشد"}, nil
		}
		countryPtr = result.Country
		allowed := false
		for _, c := range allowedCountries {
			if c == *result.Country {
				allowed = true
				break
			}
		}
		if !allowed {
			s.writeRegistrationLog(acct.ID, ip, countryPtr, false, fmt.Sprintf("country %s not allowed", *result.Country))
			return &schema.RegisterDNSIPResponse{Ok: false, Message: "این آی‌پی متعلق به کشور مجاز نیست"}, nil
		}
	}

	panel, err := s.panels.GetPanel(acct.PanelID)
	if err != nil {
		return nil, fmt.Errorf("DNS panel not found: %w", err)
	}

	ctx, cancel := context.WithTimeout(context.Background(), 15*time.Second)
	defer cancel()
	result, err := doctordns.ClaimIP(ctx, panel, acct.ApexRef, ip)
	if err != nil {
		return nil, fmt.Errorf("failed to register IP: %w", err)
	}

	if !result.Ok {
		s.writeRegistrationLog(acct.ID, ip, countryPtr, false, result.Message)
		return &schema.RegisterDNSIPResponse{Ok: false, Message: result.Message}, nil
	}

	s.writeRegistrationLog(acct.ID, ip, countryPtr, true, "")
	if err := s.db.Model(&acct).Update("current_ip", ip).Error; err != nil {
		s.logger.Warn("failed to persist newly-registered DNS current IP", zap.Uint("account_id", acct.ID), zap.Error(err))
	}

	return &schema.RegisterDNSIPResponse{Ok: true, Message: result.Message, IP: ip}, nil
}

func (s *DNSAccountService) writeRegistrationLog(accountID uint, ip string, country *string, accepted bool, rejectReason string) {
	log := model.DNSIPRegistrationLog{
		AttemptedAt:  time.Now(),
		DNSAccountID: accountID,
		IPAddress:    ip,
		Country:      country,
		Accepted:     accepted,
	}
	if rejectReason != "" {
		log.RejectReason = &rejectReason
	}
	if err := s.db.Create(&log).Error; err != nil {
		s.logger.Warn("failed to write DNS IP registration log", zap.Uint("account_id", accountID), zap.Error(err))
	}
}

// GetSelfSummary mirrors V2RayPackageService.GetSelfSummary's identical
// shape/pattern.
func (s *DNSAccountService) GetSelfSummary(resellerID uint) (*schema.DNSSelfSummaryResponse, error) {
	var reseller model.Reseller
	if err := s.db.First(&reseller, resellerID).Error; err != nil {
		return nil, err
	}

	var accounts []model.DNSAccount
	if err := s.db.Where("reseller_id = ?", resellerID).Find(&accounts).Error; err != nil {
		return nil, err
	}

	online := 0
	for _, a := range accounts {
		if isAccountConnected(a) {
			online++
		}
	}

	var remaining *int64
	if reseller.DNSQuotaBytes != nil {
		r := *reseller.DNSQuotaBytes - reseller.DNSUsedBytes
		if r < 0 {
			r = 0
		}
		remaining = &r
	}

	return &schema.DNSSelfSummaryResponse{
		OnlineAccounts: online,
		TotalAccounts:  len(accounts),
		QuotaBytes:     reseller.DNSQuotaBytes,
		UsedBytes:      reseller.DNSUsedBytes,
		RemainingBytes: remaining,
		MaxAccounts:    reseller.DNSMaxAccounts,
	}, nil
}

// GetAdminSummary is the admin dashboard's panel-wide DNS rollup -- every
// registered DNSPanel's own account/online count plus grand totals across
// every account system-wide (admin-direct and every reseller's combined).
// Mirrors V2RayPackageService.GetAdminSummary's shape, simplified for DNS's
// one-account-lives-on-exactly-one-panel model (no per-location fan-out to
// aggregate). A pure DB read.
func (s *DNSAccountService) GetAdminSummary() (*schema.DNSAdminSummaryResponse, error) {
	var panels []model.DNSPanel
	if err := s.db.Find(&panels).Error; err != nil {
		s.logger.Error("failed to fetch dns panels for admin summary", zap.Error(err))
		return nil, err
	}

	var accounts []model.DNSAccount
	if err := s.db.Find(&accounts).Error; err != nil {
		s.logger.Error("failed to fetch dns accounts for admin summary", zap.Error(err))
		return nil, err
	}

	type panelAgg struct {
		accountCount   int
		onlineCount    int
		hasRecentError bool
	}
	aggByPanel := make(map[uint]*panelAgg, len(panels))
	var totalUsedBytes int64
	onlineAccounts := 0
	for _, acct := range accounts {
		agg, ok := aggByPanel[acct.PanelID]
		if !ok {
			agg = &panelAgg{}
			aggByPanel[acct.PanelID] = agg
		}
		agg.accountCount++
		if isAccountConnected(acct) {
			agg.onlineCount++
			onlineAccounts++
		}
		if acct.LastSyncError != nil {
			agg.hasRecentError = true
		}
		totalUsedBytes += acct.UsedBytesCached
	}

	panelRows := make([]schema.DNSAdminSummaryPanel, 0, len(panels))
	for _, p := range panels {
		agg := aggByPanel[p.ID]
		if agg == nil {
			agg = &panelAgg{}
		}
		panelRows = append(panelRows, schema.DNSAdminSummaryPanel{
			PanelID:        p.ID,
			PanelName:      p.SaleTitle,
			AccountCount:   agg.accountCount,
			OnlineCount:    agg.onlineCount,
			HasRecentError: agg.hasRecentError,
		})
	}

	var totalVolumeBytes int64
	if err := s.db.Model(&model.DNSAccount{}).
		Select("COALESCE(SUM(total_volume_bytes), 0)").Scan(&totalVolumeBytes).Error; err != nil {
		s.logger.Error("failed to sum dns account volume for admin summary", zap.Error(err))
		return nil, err
	}

	return &schema.DNSAdminSummaryResponse{
		TotalAccounts:    len(accounts),
		OnlineAccounts:   onlineAccounts,
		TotalVolumeBytes: totalVolumeBytes,
		TotalUsedBytes:   totalUsedBytes,
		Panels:           panelRows,
	}, nil
}

// GetAccountShareStatus/UpdateAccountShareStatus/UpdateAccountShareExpire
// mirror V2RayPackageService's identical trio for the share-toggle UI.
func (s *DNSAccountService) GetAccountShareStatus(id uint, resellerID *uint) (*schema.DNSAccountShareStatusResponse, error) {
	acct, err := s.getAccountByIDScoped(id, resellerID)
	if err != nil {
		return nil, err
	}
	var uuidPtr *string
	if acct.IsShared {
		uuidPtr = &acct.UUID
	}
	return &schema.DNSAccountShareStatusResponse{IsShared: acct.IsShared, UUID: uuidPtr, ExpireTime: acct.ShareExpireTime}, nil
}

func (s *DNSAccountService) UpdateAccountShareStatus(id uint, resellerID *uint) error {
	acct, err := s.getAccountByIDScoped(id, resellerID)
	if err != nil {
		return fmt.Errorf("DNS account not found: %w", err)
	}
	if err := s.db.Model(&acct).Update("is_shared", !acct.IsShared).Error; err != nil {
		s.logger.Error("failed to update DNS account share status", zap.Error(err))
		return fmt.Errorf("failed to update share status: %w", err)
	}
	return nil
}

func (s *DNSAccountService) UpdateAccountShareExpire(id uint, expireTime *string, resellerID *uint) error {
	acct, err := s.getAccountByIDScoped(id, resellerID)
	if err != nil {
		return fmt.Errorf("DNS account not found: %w", err)
	}
	if !acct.IsShared {
		return fmt.Errorf("DNS account is not shared, cannot set expire time")
	}
	if err := s.db.Model(&acct).Update("share_expire_time", expireTime).Error; err != nil {
		s.logger.Error("failed to update DNS account share expire time", zap.Error(err))
		return fmt.Errorf("failed to update share expire time: %w", err)
	}
	return nil
}

func (s *DNSAccountService) transformAccountToResponse(acct model.DNSAccount) (*schema.DNSAccountResponse, error) {
	panelName := "unknown"
	if panel, err := s.panels.GetPanel(acct.PanelID); err == nil {
		panelName = panel.Name
	}

	var resellerName *string
	if acct.ResellerID != nil {
		var reseller model.Reseller
		if err := s.db.Select("name").First(&reseller, *acct.ResellerID).Error; err == nil {
			resellerName = &reseller.Name
		}
	}

	var planName *string
	if acct.DNSPlanID != nil && s.plans != nil {
		if plan, err := s.plans.GetPlan(*acct.DNSPlanID); err == nil {
			planName = &plan.Name
		}
	}

	countries, err := s.getAllowedCountries(acct.ID)
	if err != nil {
		s.logger.Warn("failed to load allowed countries for response", zap.Uint("account_id", acct.ID), zap.Error(err))
		countries = []string{}
	}

	var startAt, expireAt *string
	if acct.StartAt != nil {
		formatted := acct.StartAt.Format("2006-01-02")
		startAt = &formatted
	}
	if acct.ExpireAt != nil {
		formatted := acct.ExpireAt.Format("2006-01-02")
		expireAt = &formatted
	}

	displayedUsedBytes := acct.UsedBytesCached - acct.UsageOffsetBytes
	if displayedUsedBytes < 0 {
		displayedUsedBytes = 0
	}

	return &schema.DNSAccountResponse{
		Id:                       acct.ID,
		UUID:                     acct.UUID,
		CustomerLabel:            acct.CustomerLabel,
		Comment:                  acct.Comment,
		PanelID:                  acct.PanelID,
		PanelName:                panelName,
		PlanID:                   acct.DNSPlanID,
		PlanName:                 planName,
		TotalVolumeBytes:         acct.TotalVolumeBytes,
		UsedBytes:                displayedUsedBytes,
		SpeedKbps:                acct.SpeedKbps,
		DurationDays:             acct.DurationDays,
		StartAt:                  startAt,
		ExpireAt:                 expireAt,
		TemplateID:               acct.TemplateID,
		Status:                   acct.Status,
		IsShared:                 acct.IsShared,
		MaxConcurrentIPs:         acct.MaxConcurrentIPs,
		DailyIPRegistrationLimit: acct.DailyIPRegistrationLimit,
		AllowedCountries:         countries,
		CurrentIP:                acct.CurrentIP,
		IsOnline:                 isAccountConnected(acct),
		LastSyncError:            acct.LastSyncError,
		ResellerID:               acct.ResellerID,
		ResellerName:             resellerName,
	}, nil
}
