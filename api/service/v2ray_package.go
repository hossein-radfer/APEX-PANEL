package service

import (
	"context"
	"errors"
	"fmt"
	mathrand "math/rand"
	"sync"
	"time"

	"github.com/google/uuid"
	"go.uber.org/zap"
	"gorm.io/gorm"

	"github.com/maahdima/mwp/api/adaptor/xui"
	"github.com/maahdima/mwp/api/common"
	"github.com/maahdima/mwp/api/dataservice/model"
	"github.com/maahdima/mwp/api/http/schema"
	"github.com/maahdima/mwp/api/utils"
)

// V2RayPackageService manages V2Ray packages provisioned across every
// registered XuiPanel -- a fully independent VPN product from Peer
// (WireGuard) and UserManagerAccount, mirroring their permission/quota/
// scoping conventions but never sharing a table, a quota pool, or a
// reseller-suspension path with either. Unlike those two, a single package
// fans out to a client on EVERY registered panel (see CreatePackage) --
// usage is the sum across a package's locations, computed and cached by
// the background sync job (v2ray_sync.go), never live at request time.
type V2RayPackageService struct {
	db             *gorm.DB
	panels         *XuiPanelService
	auditLog       *AuditLog
	botNotifier    *BotNotifier
	licenseLimiter freeTierLimiter
	logger         *zap.Logger
}

func NewV2RayPackageService(db *gorm.DB, panels *XuiPanelService, auditLog *AuditLog) *V2RayPackageService {
	return &V2RayPackageService{
		db:       db,
		panels:   panels,
		auditLog: auditLog,
		logger:   zap.L().Named("V2RayPackageService"),
	}
}

// SetBotNotifier wires the Telegram notifier after construction, mirroring
// UserManagerService.SetBotNotifier -- safe to leave unset.
func (s *V2RayPackageService) SetBotNotifier(notifier *BotNotifier) {
	s.botNotifier = notifier
}

// SetLicenseLimiter wires the free-tier cap source after construction --
// mirrors Reseller.SetLicenseLimiter / WgPeer.SetLicenseLimiter exactly.
// Safe to leave unset (nil licenseLimiter disables enforcement entirely).
func (s *V2RayPackageService) SetLicenseLimiter(limiter freeTierLimiter) {
	s.licenseLimiter = limiter
}

// ErrFreeTierV2RayPackageLimitReached is returned by CreatePackage when the
// license is Restricted and the configured "max_v2ray_packages" cap has
// already been reached.
var ErrFreeTierV2RayPackageLimitReached = errors.New("free tier limit reached: maximum v2ray packages")

// SuspendOldestForFreeTier mirrors WgPeer.SuspendOldestForFreeTier exactly
// (see its own doc comment for the full DB-only, external-call-deferred
// rationale) -- the oldest excess V2RayPackage rows (by CreatedAt) beyond
// cap are force-suspended at the DB level via the same
// Status/SuspendedByQuota/WasActiveBeforeSuspend columns
// applyResellerV2RayQuota's own suspend branch already writes, so the
// periodic v2ray-sync job disables the actual x-ui client(s) on its own
// next tick, exactly like it already does for byte-quota exhaustion.
func (s *V2RayPackageService) SuspendOldestForFreeTier(cap int64) (int, error) {
	if cap <= 0 {
		return 0, nil
	}

	var activeCount int64
	if err := s.db.Model(&model.V2RayPackage{}).Where("status = ?", "active").Count(&activeCount).Error; err != nil {
		return 0, err
	}

	excess := activeCount - cap
	if excess <= 0 {
		return 0, nil
	}

	var toSuspend []model.V2RayPackage
	if err := s.db.Where("status = ?", "active").Order("created_at ASC").Limit(int(excess)).Find(&toSuspend).Error; err != nil {
		return 0, err
	}

	suspended := 0
	for _, pkg := range toSuspend {
		if err := s.db.Model(&model.V2RayPackage{}).Where("id = ?", pkg.ID).Updates(map[string]interface{}{
			"status":                    "suspended",
			"suspended_by_quota":        true,
			"was_active_before_suspend": true,
		}).Error; err != nil {
			s.logger.Error("failed to suspend v2ray package for free-tier cap", zap.Uint("package_id", pkg.ID), zap.Error(err))
			continue
		}
		suspended++
	}

	return suspended, nil
}

func (s *V2RayPackageService) logResellerAction(resellerID *uint, action, description string) {
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

	if action == AuditActionV2RayPackageCreated && s.botNotifier != nil {
		s.botNotifier.NotifyLiveLog(fmt.Sprintf("%s: %s", name, description))
	}
}

// ensureResellerCanResellV2Ray mirrors
// UserManagerService.ensureResellerCanCreateUserManagerAccounts exactly --
// a plain boolean gate, since x-ui panels are a shared external resource
// with no per-reseller sub-resource to assign.
func (s *V2RayPackageService) ensureResellerCanResellV2Ray(resellerID uint) error {
	var reseller model.Reseller
	if err := s.db.First(&reseller, resellerID).Error; err != nil {
		return err
	}
	if !reseller.CanResellV2Ray {
		return fmt.Errorf("reseller is not permitted to resell v2ray packages")
	}
	return nil
}

// ensureResellerUnderV2RayPackageLimit mirrors
// UserManagerService.ensureResellerUnderUserManagerAccountLimit exactly.
func (s *V2RayPackageService) ensureResellerUnderV2RayPackageLimit(resellerID uint) error {
	var reseller model.Reseller
	if err := s.db.First(&reseller, resellerID).Error; err != nil {
		return err
	}
	if reseller.V2RayMaxPackages == nil {
		return nil
	}

	var count int64
	if err := s.db.Model(&model.V2RayPackage{}).Where("reseller_id = ?", resellerID).Count(&count).Error; err != nil {
		return err
	}
	if int(count) >= *reseller.V2RayMaxPackages {
		return fmt.Errorf("reseller has reached its maximum allowed v2ray packages (%d)", *reseller.V2RayMaxPackages)
	}
	return nil
}

// SetAssignedXuiPanels replaces the full set of x-ui panels resellerID may
// use -- mirrors Reseller.SetAssignedUserManagerGroups exactly (delete-all-
// then-recreate inside one transaction).
func (s *V2RayPackageService) SetAssignedXuiPanels(resellerID uint, panelIDs []uint) error {
	var reseller model.Reseller
	if err := s.db.First(&reseller, resellerID).Error; err != nil {
		return err
	}

	return s.db.Transaction(func(tx *gorm.DB) error {
		if err := tx.Unscoped().Where("reseller_id = ?", resellerID).Delete(&model.ResellerXuiPanelAccess{}).Error; err != nil {
			return err
		}

		if len(panelIDs) == 0 {
			return nil
		}

		links := make([]model.ResellerXuiPanelAccess, 0, len(panelIDs))
		for _, panelID := range panelIDs {
			links = append(links, model.ResellerXuiPanelAccess{ResellerID: resellerID, PanelID: panelID})
		}

		return tx.Create(&links).Error
	})
}

// GetAssignedXuiPanels returns the panel IDs resellerID is allowed to use.
func (s *V2RayPackageService) GetAssignedXuiPanels(resellerID uint) ([]uint, error) {
	var links []model.ResellerXuiPanelAccess
	if err := s.db.Where("reseller_id = ?", resellerID).Find(&links).Error; err != nil {
		return nil, err
	}

	panelIDs := make([]uint, 0, len(links))
	for _, link := range links {
		panelIDs = append(panelIDs, link.PanelID)
	}
	return panelIDs, nil
}

// GetAssignedXuiPanelSummaries is GetAssignedXuiPanels' reseller-safe
// counterpart: it returns id+name for each assigned panel instead of bare
// IDs. Added because V2RayForm (used by both admin and reseller sessions)
// needs panel names to render its "which server(s)" checkbox list, but
// GET /xui-panel (the full list, with connection credentials) is
// admin-only -- a reseller has no other permitted way to resolve their
// assigned panel IDs into display names. See GetAssignedXuiPanels' own
// access rule (admin or the reseller owner), which this reuses unchanged.
func (s *V2RayPackageService) GetAssignedXuiPanelSummaries(resellerID uint) ([]model.XuiPanel, error) {
	var links []model.ResellerXuiPanelAccess
	if err := s.db.Where("reseller_id = ?", resellerID).Find(&links).Error; err != nil {
		return nil, err
	}

	panelIDs := make([]uint, 0, len(links))
	for _, link := range links {
		panelIDs = append(panelIDs, link.PanelID)
	}
	if len(panelIDs) == 0 {
		return []model.XuiPanel{}, nil
	}

	var panels []model.XuiPanel
	if err := s.db.Where("id IN ?", panelIDs).Find(&panels).Error; err != nil {
		return nil, err
	}
	return panels, nil
}

// resolveEffectivePanels implements CreatePackage's panel-selection rule:
//   - Admin-direct package (resellerID nil): candidatePanels is every
//     registered panel; requestedIDs (if non-empty) narrows that set.
//   - Reseller package: candidatePanels is only the panels this reseller
//     has been explicitly granted via ResellerXuiPanelAccess (mirrors
//     ResellerInterface's "must be assigned, no implicit access" rule);
//     requestedIDs (if non-empty) must be a subset of that grant, checked
//     explicitly rather than silently filtered, so a reseller attempting
//     to use a panel they weren't granted gets a clear rejection instead
//     of a silently-smaller package.
//
// An empty/omitted requestedIDs means "use every candidate panel" -- the
// smart default the UI applies automatically when only one panel exists
// and pre-selects (but still shows as picked) when multiple exist.
func (s *V2RayPackageService) resolveEffectivePanels(resellerID *uint, requestedIDs []uint) ([]model.XuiPanel, error) {
	var candidates []model.XuiPanel

	if resellerID == nil {
		all, err := s.panels.ListAllPanelModels()
		if err != nil {
			return nil, fmt.Errorf("failed to list registered panels: %w", err)
		}
		candidates = all
	} else {
		assignedIDs, err := s.GetAssignedXuiPanels(*resellerID)
		if err != nil {
			return nil, fmt.Errorf("failed to look up assigned panels: %w", err)
		}
		if len(assignedIDs) == 0 {
			return nil, nil
		}
		all, err := s.panels.ListAllPanelModels()
		if err != nil {
			return nil, fmt.Errorf("failed to list registered panels: %w", err)
		}
		assignedSet := make(map[uint]struct{}, len(assignedIDs))
		for _, id := range assignedIDs {
			assignedSet[id] = struct{}{}
		}
		for _, p := range all {
			if _, ok := assignedSet[p.ID]; ok {
				candidates = append(candidates, p)
			}
		}
	}

	if len(requestedIDs) == 0 {
		return candidates, nil
	}

	candidateByID := make(map[uint]model.XuiPanel, len(candidates))
	for _, p := range candidates {
		candidateByID[p.ID] = p
	}

	selected := make([]model.XuiPanel, 0, len(requestedIDs))
	for _, id := range requestedIDs {
		p, ok := candidateByID[id]
		if !ok {
			if resellerID != nil {
				return nil, fmt.Errorf("reseller is not permitted to use panel id %d", id)
			}
			return nil, fmt.Errorf("panel id %d does not exist", id)
		}
		selected = append(selected, p)
	}

	return selected, nil
}

// getPackageByIDScoped mirrors UserManagerService.getAccountByIDScoped
// exactly.
func (s *V2RayPackageService) getPackageByIDScoped(id uint, resellerID *uint) (model.V2RayPackage, error) {
	var pkg model.V2RayPackage
	query := s.db.Where("id = ?", id)
	if resellerID != nil {
		query = query.Where("reseller_id = ?", *resellerID)
	}
	if err := query.First(&pkg).Error; err != nil {
		return model.V2RayPackage{}, err
	}
	return pkg, nil
}

// EnsurePackageAccess mirrors UserManagerService.EnsureAccountAccess.
func (s *V2RayPackageService) EnsurePackageAccess(id uint, resellerID *uint) error {
	_, err := s.getPackageByIDScoped(id, resellerID)
	return err
}

// CreatePackage creates a V2Ray package and fans it out to the effective
// panel set resolved by resolveEffectivePanels -- req.PanelIDs (if
// non-empty) narrows that set; empty/omitted means every panel the caller
// is allowed to use (every registered panel for an admin-direct package,
// or every panel this reseller has been explicitly granted). A single
// location's AddClient call failing does NOT fail the whole package
// creation -- it's recorded with LastSyncError set and Enabled=false,
// surfaced in the admin UI as a per-location status badge, and retried by
// the next sync job tick. This mirrors the "one dead panel never breaks
// the whole package" principle (see V2RayPackageLocation's doc comment)
// applied to creation, not just serving. If EVERY selected panel fails
// (including the case of zero candidate panels), the package itself is
// still created -- an empty/all-failed set of locations is a valid,
// visible state for the admin to fix, not a reason to reject the whole
// operation.
func (s *V2RayPackageService) CreatePackage(req *schema.CreateV2RayPackageRequest, resellerID *uint) (*schema.V2RayPackageResponse, error) {
	if s.licenseLimiter != nil {
		if maxPackages, ok := s.licenseLimiter.GetFreeTierLimit("max_v2ray_packages"); ok {
			var count int64
			if err := s.db.Model(&model.V2RayPackage{}).Count(&count).Error; err != nil {
				return nil, err
			}
			if count >= maxPackages {
				return nil, ErrFreeTierV2RayPackageLimitReached
			}
		}
	}

	if resellerID != nil {
		if err := s.ensureResellerCanResellV2Ray(*resellerID); err != nil {
			return nil, err
		}
		if err := s.ensureResellerUnderV2RayPackageLimit(*resellerID); err != nil {
			return nil, err
		}
	}

	panels, err := s.resolveEffectivePanels(resellerID, req.PanelIDs)
	if err != nil {
		return nil, err
	}

	// StartAt/ExpireAt are computed once, here, at creation time -- a real
	// gap this replaces: the fields existed on the model and were already
	// read/formatted by transformPackageToResponse, but nothing ever WROTE
	// them, so every package's share page showed "Expire Time: Never"
	// regardless of DurationDays.
	now := time.Now()
	expireAt := now.AddDate(0, 0, req.DurationDays)

	pkg := model.V2RayPackage{
		UUID:             uuid.New().String(),
		CustomerLabel:    req.CustomerLabel,
		Comment:          req.Comment,
		ResellerID:       resellerID,
		TotalVolumeBytes: req.TotalVolumeBytes,
		DurationDays:     req.DurationDays,
		StartAt:          &now,
		ExpireAt:         &expireAt,
		Status:           "active",
	}

	if err := s.db.Create(&pkg).Error; err != nil {
		s.logger.Error("failed to store v2ray package", zap.Error(err))
		return nil, fmt.Errorf("failed to create v2ray package: %w", err)
	}

	shortID := pkg.UUID
	if len(shortID) > 8 {
		shortID = shortID[:8]
	}

	// Fan out to every panel CONCURRENTLY, not sequentially -- a live
	// profiling run found this was a real, severe bug: with N registered
	// panels, calling xui.AddClient one at a time in a for loop meant a
	// single slow/unreachable panel added up to its own 15s xui.Login
	// timeout to the TOTAL request time, so N unreachable panels made this
	// HTTP request (and the admin waiting on it) hang for N*15s -- 45s
	// confirmed for 3 panels. Running every panel's AddClient call on its
	// own goroutine bounds the whole operation to whatever the SLOWEST
	// single panel takes (~15s worst case), not their sum, matching
	// V2RayPackageLocation's own doc comment that one dead panel must
	// never degrade the rest of the system.
	type locationResult struct {
		location model.V2RayPackageLocation
	}
	results := make([]locationResult, len(panels))
	var wg sync.WaitGroup
	for i, panel := range panels {
		wg.Add(1)
		go func(i int, panel model.XuiPanel) {
			defer wg.Done()

			clientUUID := uuid.New().String()
			clientEmail := fmt.Sprintf("pkg_%s_%d", shortID, panel.ID)
			subID := uuid.New().String()

			location := model.V2RayPackageLocation{
				PackageID:   pkg.ID,
				PanelID:     panel.ID,
				ClientUUID:  clientUUID,
				ClientEmail: clientEmail,
				SubID:       subID,
				Enabled:     true,
			}

			ctx, cancel := context.WithTimeout(context.Background(), 20*time.Second)
			defer cancel()

			// flow="xtls-rprx-vision" is only valid when the inbound's own
			// streamSettings.security is "tls" or "reality" -- a confirmed,
			// reproduced bug (reported by a live customer's own V2Ray
			// engineer, with side-by-side evidence from a real panel) had
			// this hardcoded regardless of the inbound's actual security
			// setting, silently creating clients that would NEVER connect
			// on any security:"none" inbound (VLESS flow control requires
			// the TLS/Reality handshake underneath it). Resolved fresh
			// against the inbound's real, current settings on every call,
			// never assumed.
			flow, flowErr := xui.ResolveClientFlow(ctx, panel, panel.DefaultInboundID)
			if flowErr != nil {
				s.logger.Warn("failed to resolve inbound security settings, defaulting to no flow (safe on non-TLS/Reality inbounds)",
					zap.Uint("panel_id", panel.ID), zap.Error(flowErr))
			}

			err := xui.AddClient(ctx, panel, xui.XuiClient{
				ID:      clientUUID,
				Flow:    flow,
				Email:   clientEmail,
				TotalGB: req.TotalVolumeBytes,
				Enable:  true,
				SubID:   subID,
			})
			if err != nil {
				s.logger.Warn("failed to add client on panel during package creation, location marked for retry",
					zap.Uint("panel_id", panel.ID), zap.String("panel_name", panel.Name), zap.Error(err))
				errStr := err.Error()
				location.LastSyncError = &errStr
			} else {
				// Fetch this location's own config link RIGHT NOW, rather
				// than leaving ConfigLinkCached empty until the next
				// scheduled sync tick (up to TRAFFIC_JOB_INTERVAL seconds
				// away) -- a confirmed, reported bug: a freshly-created
				// package's share page showed blank config links/QR codes
				// until the background job happened to run, which reads as
				// "this panel is broken" to a customer who has no idea a
				// periodic job even exists. Best-effort: a failure here is
				// silently left for the next sync tick to fill in, exactly
				// like every other transient x-ui failure in this codebase
				// -- it does not fail package creation itself.
				if rawSub, subErr := xui.GetSubscription(ctx, panel, subID); subErr != nil {
					s.logger.Warn("failed to fetch v2ray subscription content immediately after client creation, will be filled in by the next sync tick",
						zap.Uint("panel_id", panel.ID), zap.Error(subErr))
				} else if rewritten, rewriteErr := rewriteV2RaySubscriptionTitle(s.db, rawSub, pkg.ID, panel); rewriteErr != nil {
					s.logger.Warn("failed to rewrite v2ray subscription title immediately after client creation, will be filled in by the next sync tick",
						zap.Uint("panel_id", panel.ID), zap.Error(rewriteErr))
				} else {
					location.ConfigLinkCached = rewritten
					now := time.Now()
					location.LastSyncedAt = &now
				}
			}

			results[i] = locationResult{location: location}
		}(i, panel)
	}
	wg.Wait()

	for _, r := range results {
		addClientFailed := r.location.LastSyncError != nil

		if createErr := s.db.Create(&r.location).Error; createErr != nil {
			s.logger.Error("failed to store v2ray package location", zap.Uint("panel_id", r.location.PanelID), zap.Error(createErr))
			continue
		}

		// Enabled=false must be applied via a separate Update, not set on
		// the struct before Create -- GORM's Create() treats an explicit
		// false on a field with a `gorm:"default:true"` tag as "unset" and
		// silently substitutes the column default (true) instead, since
		// false is bool's Go zero value. This is the exact same class of
		// bug as V2RayTrafficPackage.IsActive's create-time default
		// override; the fix here is the same: create first, then Update
		// the boolean explicitly so GORM emits a real UPDATE statement
		// instead of relying on the zero-value-suppressed INSERT.
		if addClientFailed {
			if updateErr := s.db.Model(&r.location).Update("enabled", false).Error; updateErr != nil {
				s.logger.Error("failed to mark failed v2ray location as disabled", zap.Uint("panel_id", r.location.PanelID), zap.Error(updateErr))
			}
		}
	}

	customerLabel := "بدون‌نام"
	if pkg.CustomerLabel != nil && *pkg.CustomerLabel != "" {
		customerLabel = *pkg.CustomerLabel
	}
	volumeGB := float64(pkg.TotalVolumeBytes) / (1024 * 1024 * 1024)
	s.logResellerAction(resellerID, AuditActionV2RayPackageCreated, fmt.Sprintf(
		"ساخت پکیج V2Ray برای %s | %.1f گیگابایت | %d روز (uuid=%s)",
		customerLabel, volumeGB, pkg.DurationDays, pkg.UUID,
	))

	resp, err := s.transformPackageToResponse(pkg)
	if err != nil {
		return nil, err
	}
	return &resp, nil
}

// bulkLabelAdjectives/bulkLabelNouns seed a small, pronounceable random
// name generator for BulkCreatePackages -- deliberately not a UUID or raw
// random hex, since CustomerLabel is customer-facing (it appears as the
// package's own title inside a V2Ray client app, see
// rewriteV2RaySubscriptionTitle), so a readable-if-arbitrary name reads
// better there than an opaque string would.
var bulkLabelAdjectives = []string{"Swift", "Silver", "Golden", "Crimson", "Azure", "Rapid", "Cosmic", "Lunar", "Solar", "Amber"}
var bulkLabelNouns = []string{"Falcon", "Tiger", "Comet", "Phoenix", "Wolf", "Eagle", "Panther", "Dragon", "Hawk", "Fox"}

// generateBulkPackageLabel returns a random "<Adjective><Noun><n>" base
// name suffixed with "_<volume>GB_<days>d" -- the admin's own explicit
// format: "اسم رندوم + حجم + روز". index disambiguates identical
// adjective/noun draws within the same batch (astronomically unlikely to
// collide given the small word lists times a batch of up to 500, but free
// to guarantee outright rather than leave to chance). Uses math/rand
// (mirrors utils.RandomString's own existing convention in this codebase)
// -- this is a display label, not a security token, so no need for
// crypto/rand here.
func generateBulkPackageLabel(index int, totalVolumeBytes int64, durationDays int) string {
	adjective := bulkLabelAdjectives[mathrand.Intn(len(bulkLabelAdjectives))]
	noun := bulkLabelNouns[mathrand.Intn(len(bulkLabelNouns))]
	volumeGB := float64(totalVolumeBytes) / (1024 * 1024 * 1024)
	return fmt.Sprintf("%s%s%d_%.0fGB_%dd", adjective, noun, index+1, volumeGB, durationDays)
}

// BulkCreatePackages creates req.Count packages sharing the same volume/
// duration/panel selection, each via a plain call to CreatePackage above
// (reusing its existing validation, x-ray-panel fan-out, and audit-log
// behavior verbatim, rather than duplicating any of that logic here) --
// only the CustomerLabel differs per package, via generateBulkPackageLabel.
// The reseller package-limit check happens UP FRONT, against the full
// req.Count, so this either creates the whole batch or fails before
// creating any of it -- avoiding the confusing partial-batch state a
// mid-loop quota failure would otherwise leave behind.
func (s *V2RayPackageService) BulkCreatePackages(req *schema.BulkCreateV2RayPackageRequest, resellerID *uint) ([]schema.V2RayPackageResponse, error) {
	if resellerID != nil {
		if err := s.ensureResellerCanResellV2Ray(*resellerID); err != nil {
			return nil, err
		}
		var reseller model.Reseller
		if err := s.db.First(&reseller, *resellerID).Error; err != nil {
			return nil, err
		}
		if reseller.V2RayMaxPackages != nil {
			var existingCount int64
			if err := s.db.Model(&model.V2RayPackage{}).Where("reseller_id = ?", *resellerID).Count(&existingCount).Error; err != nil {
				return nil, err
			}
			if int(existingCount)+req.Count > *reseller.V2RayMaxPackages {
				return nil, fmt.Errorf("creating %d packages would exceed the reseller's maximum allowed v2ray packages (%d, %d already exist)", req.Count, *reseller.V2RayMaxPackages, existingCount)
			}
		}
	}

	results := make([]schema.V2RayPackageResponse, 0, req.Count)
	for i := 0; i < req.Count; i++ {
		label := generateBulkPackageLabel(i, req.TotalVolumeBytes, req.DurationDays)
		pkg, err := s.CreatePackage(&schema.CreateV2RayPackageRequest{
			CustomerLabel:    &label,
			TotalVolumeBytes: req.TotalVolumeBytes,
			DurationDays:     req.DurationDays,
			PanelIDs:         req.PanelIDs,
		}, resellerID)
		if err != nil {
			s.logger.Error("bulk create failed partway through -- returning packages created so far", zap.Int("created", len(results)), zap.Int("requested", req.Count), zap.Error(err))
			return results, fmt.Errorf("failed after creating %d of %d packages: %w", len(results), req.Count, err)
		}

		// Sharing is off by default on every package (see CreatePackage --
		// IsShared is never set there, so it's Go's bool zero value, false).
		// A bulk-created batch exists specifically to be exported with its
		// share/subscription links (the admin's whole point in using this
		// flow), so turning sharing on here -- once, right after creation --
		// is what makes ExportPackages' ShareLink/SubscriptionLink columns
		// resolve to real, working links instead of ones that 404 against
		// GetPackageShareDetails' own IsShared check. An ordinary single
		// CreatePackage is untouched: this only runs on the bulk path.
		if updateErr := s.db.Model(&model.V2RayPackage{}).Where("id = ?", pkg.Id).Update("is_shared", true).Error; updateErr != nil {
			s.logger.Warn("failed to enable sharing on bulk-created package, its share/subscription links will not resolve until sharing is turned on manually", zap.Uint("package_id", pkg.Id), zap.Error(updateErr))
		} else {
			pkg.IsShared = true
		}

		results = append(results, *pkg)
	}

	return results, nil
}

// UpdatePackage updates a package's fields and, when Status or
// TotalVolumeBytes actually change, pushes Enable/TotalGB to every one of
// this package's x-ui clients SYNCHRONOUSLY, in this same request --
// mirroring Peer.UpdatePeer's own "push to the real device before touching
// the DB" convention (updateMikrotikPeer), rather than PeerService's
// no-op-on-mismatch design this function used to have.
//
// A confirmed, reported bug: this function previously only wrote pkg.Status/
// TotalVolumeBytes to the local DB row and relied on a doc comment's claim
// that "the next sync tick" would apply it. That claim was false --
// SyncPackageUsage's enforcePackageQuota only ever calls xui.UpdateClient
// when USAGE crosses TotalVolumeBytes; it never reads pkg.Status at all, and
// it only ever DISABLES a client, never re-enables one. So manually setting
// Status to "expired"/"suspended" left every one of the package's real x-ui
// clients enabled indefinitely, and increasing TotalVolumeBytes plus setting
// Status back to "active" never re-enabled an already-disabled client nor
// pushed the new limit to x-ui -- both exactly as reported. Fixed here by
// resolving the package's desired enabled state (active/not quota-exceeded)
// and applying it, plus the current TotalVolumeBytes, to every location the
// moment either input changes -- no more waiting on a background tick for
// an admin-initiated change.
func (s *V2RayPackageService) UpdatePackage(id uint, req *schema.UpdateV2RayPackageRequest, resellerID *uint) (*schema.V2RayPackageResponse, error) {
	pkg, err := s.getPackageByIDScoped(id, resellerID)
	if err != nil {
		return nil, fmt.Errorf("package not found: %w", err)
	}

	// A confirmed, reported gap: there was previously NO way to edit an
	// existing package's set of x-ui panel locations at all -- an admin
	// had to delete and recreate the whole package (losing its usage
	// history, share links, and customer-facing config) just to add or
	// remove one panel. req.PanelIDs, when the caller sends it (even as
	// an empty list), fully REPLACES the current location set: resolved
	// through the exact same reseller-scoped resolveEffectivePanels
	// CreatePackage itself uses (so a reseller can never add a panel
	// they're not assigned to via this path either), then reconciled
	// against the package's current locations -- newly-selected panels
	// get a real x-ui client created (addPackageLocation, CreatePackage's
	// own per-panel logic factored out), de-selected panels get their
	// x-ui client disabled and their location row removed
	// (removePackageLocation, DeletePackage's own per-location logic
	// factored out), and panels present in both sets are left completely
	// untouched (no pointless disable+recreate churn on a location that
	// isn't actually changing).
	if req.PanelIDs != nil {
		if err := s.reconcilePackageLocations(pkg, *req.PanelIDs, resellerID); err != nil {
			return nil, err
		}
	}

	if req.CustomerLabel != nil {
		pkg.CustomerLabel = req.CustomerLabel
	}
	if req.Comment != nil {
		pkg.Comment = req.Comment
	}

	volumeChanged := req.TotalVolumeBytes != nil && *req.TotalVolumeBytes != pkg.TotalVolumeBytes
	if req.TotalVolumeBytes != nil {
		pkg.TotalVolumeBytes = *req.TotalVolumeBytes
	}
	if req.DurationDays != nil {
		pkg.DurationDays = *req.DurationDays

		// A confirmed gap: this field was being updated with no
		// corresponding recompute of ExpireAt, so changing DurationDays
		// (e.g. a renewal call from an external integration) silently had
		// no effect on when the package actually expires. Mirrors
		// CreatePackage's own StartAt+DurationDays -> ExpireAt formula
		// exactly, anchored to the package's existing StartAt (falling
		// back to now if it was never set) so a plain duration correction
		// behaves predictably -- callers that want "add N more days from
		// today" (a renewal) should compute the resulting total
		// DurationDays themselves before calling this, same as any other
		// field here.
		startAt := time.Now()
		if pkg.StartAt != nil {
			startAt = *pkg.StartAt
		}
		expireAt := startAt.AddDate(0, 0, pkg.DurationDays)
		pkg.ExpireAt = &expireAt
	}

	statusChanged := req.Status != nil && *req.Status != pkg.Status
	if req.Status != nil {
		pkg.Status = *req.Status
		if pkg.Status == "active" {
			// An admin reactivating a package that was previously disabled
			// by the quota job (see enforcePackageQuota) is exactly the
			// "resume" case Peer/UserManagerAccount already track via
			// these two flags -- clear them so a future quota check
			// doesn't treat this as still-suspended.
			pkg.SuspendedByQuota = false
			pkg.WasActiveBeforeSuspend = false

			// Confirmed, reported production incident this fixes (reseller
			// "Mohammadreza", VOLUME-mode, whose overall V2Ray pool sits
			// slightly over its 200GB quota): SuspendedByResellerQuota was
			// never cleared here, only SuspendedByQuota above -- so an admin
			// (or the reseller) manually flipping a reseller-quota-suspended
			// package back to "active" (via this same Status field) brought
			// the x-ui client back online for a moment, but left
			// SuspendedByResellerQuota=true stuck in the DB. Since this
			// package's Status is now "active" (no longer "suspended"),
			// applyResellerV2RayQuota's own suspend query -- which selects
			// every package with status != "suspended" for a still-over-
			// quota reseller (see that function's own doc comment) -- picked
			// this package right back up on its very next tick and
			// re-suspended it, undoing the manual reactivation within
			// moments and repeating indefinitely for as long as the
			// reseller's pool stays over quota. Clearing this flag here too
			// (mirroring SuspendedByQuota just above) makes a manual
			// reactivation stick exactly like it visibly does, instead of
			// silently reverting on the next sync tick; if the reseller's
			// pool is still genuinely over quota, applyResellerV2RayQuota
			// will still (correctly, and now only once) re-suspend it with a
			// fresh SuspendedByResellerQuota=true, rather than flapping on a
			// stale flag that was never actually cleared.
			pkg.SuspendedByResellerQuota = false
		}
	}

	if err := s.db.Save(&pkg).Error; err != nil {
		s.logger.Error("failed to update v2ray package", zap.Error(err))
		return nil, fmt.Errorf("failed to update v2ray package: %w", err)
	}

	if statusChanged || volumeChanged {
		s.applyPackageStateToLocations(pkg)
	}

	resp, err := s.transformPackageToResponse(pkg)
	if err != nil {
		return nil, err
	}
	return &resp, nil
}

// reconcilePackageLocations replaces pkg's full set of x-ui panel
// locations with requestedPanelIDs -- see UpdatePackage's own call site
// doc comment for the reported gap this fixes. requestedPanelIDs is
// resolved through resolveEffectivePanels (the SAME reseller-scoping
// CreatePackage uses), so a reseller can never smuggle in a panel they
// aren't assigned to via an edit any more than they could via creation.
func (s *V2RayPackageService) reconcilePackageLocations(pkg model.V2RayPackage, requestedPanelIDs []uint, resellerID *uint) error {
	panels, err := s.resolveEffectivePanels(resellerID, requestedPanelIDs)
	if err != nil {
		return fmt.Errorf("failed to resolve requested panels: %w", err)
	}

	var existing []model.V2RayPackageLocation
	if err := s.db.Where("package_id = ?", pkg.ID).Find(&existing).Error; err != nil {
		s.logger.Error("failed to list v2ray package locations for reconcile", zap.Uint("package_id", pkg.ID), zap.Error(err))
		return fmt.Errorf("failed to update package locations: %w", err)
	}
	existingByPanel := make(map[uint]model.V2RayPackageLocation, len(existing))
	for _, loc := range existing {
		existingByPanel[loc.PanelID] = loc
	}
	wantedByPanel := make(map[uint]model.XuiPanel, len(panels))
	for _, p := range panels {
		wantedByPanel[p.ID] = p
	}

	var toAdd []model.XuiPanel
	for _, p := range panels {
		if _, already := existingByPanel[p.ID]; !already {
			toAdd = append(toAdd, p)
		}
	}
	var toRemove []model.V2RayPackageLocation
	for _, loc := range existing {
		if _, stillWanted := wantedByPanel[loc.PanelID]; !stillWanted {
			toRemove = append(toRemove, loc)
		}
	}

	// Concurrent, not sequential -- same fix and same rationale as
	// CreatePackage/DeletePackage's own fan-out (see their doc comments):
	// a for loop here would let N unreachable panels add up to N*15s to
	// this one request.
	var wg sync.WaitGroup
	for _, panel := range toAdd {
		wg.Add(1)
		go func(panel model.XuiPanel) {
			defer wg.Done()
			s.addPackageLocation(pkg, panel)
		}(panel)
	}
	for _, loc := range toRemove {
		panel, err := s.panels.GetPanel(loc.PanelID)
		if err != nil {
			s.logger.Warn("panel no longer exists, skipping remote disable for removed location",
				zap.Uint("panel_id", loc.PanelID), zap.Error(err))
			continue
		}
		wg.Add(1)
		go func(panel model.XuiPanel, loc model.V2RayPackageLocation) {
			defer wg.Done()
			s.removePackageLocation(panel, loc)
		}(panel, loc)
	}
	wg.Wait()

	if len(toRemove) > 0 {
		ids := make([]uint, len(toRemove))
		for i, loc := range toRemove {
			ids[i] = loc.ID
		}
		if err := s.db.Where("id IN ?", ids).Delete(&model.V2RayPackageLocation{}).Error; err != nil {
			s.logger.Error("failed to delete removed v2ray package locations from database", zap.Uint("package_id", pkg.ID), zap.Error(err))
			return fmt.Errorf("failed to update package locations: %w", err)
		}
	}

	return nil
}

// addPackageLocation creates one new x-ui client for pkg on panel and
// persists its V2RayPackageLocation row -- CreatePackage's own per-panel
// client-creation logic (flow resolution, AddClient, immediate
// subscription-link fetch), factored out so UpdatePackage's
// reconcilePackageLocations can add a location to an EXISTING package
// the exact same way a brand-new package's locations are created.
func (s *V2RayPackageService) addPackageLocation(pkg model.V2RayPackage, panel model.XuiPanel) {
	shortID := pkg.UUID
	if len(shortID) > 8 {
		shortID = shortID[:8]
	}

	clientUUID := uuid.New().String()
	clientEmail := fmt.Sprintf("pkg_%s_%d", shortID, panel.ID)
	subID := uuid.New().String()

	location := model.V2RayPackageLocation{
		PackageID:   pkg.ID,
		PanelID:     panel.ID,
		ClientUUID:  clientUUID,
		ClientEmail: clientEmail,
		SubID:       subID,
		Enabled:     pkg.Status == "active",
	}

	ctx, cancel := context.WithTimeout(context.Background(), 20*time.Second)
	defer cancel()

	flow, flowErr := xui.ResolveClientFlow(ctx, panel, panel.DefaultInboundID)
	if flowErr != nil {
		s.logger.Warn("failed to resolve inbound security settings, defaulting to no flow (safe on non-TLS/Reality inbounds)",
			zap.Uint("panel_id", panel.ID), zap.Error(flowErr))
	}

	addClientFailed := false
	if err := xui.AddClient(ctx, panel, xui.XuiClient{
		ID:      clientUUID,
		Flow:    flow,
		Email:   clientEmail,
		TotalGB: pkg.TotalVolumeBytes,
		Enable:  location.Enabled,
		SubID:   subID,
	}); err != nil {
		s.logger.Warn("failed to add client on panel while adding a location to an existing package, location marked for retry",
			zap.Uint("panel_id", panel.ID), zap.String("panel_name", panel.Name), zap.Error(err))
		errStr := err.Error()
		location.LastSyncError = &errStr
		addClientFailed = true
	} else if rawSub, subErr := xui.GetSubscription(ctx, panel, subID); subErr != nil {
		s.logger.Warn("failed to fetch v2ray subscription content immediately after adding a location, will be filled in by the next sync tick",
			zap.Uint("panel_id", panel.ID), zap.Error(subErr))
	} else if rewritten, rewriteErr := rewriteV2RaySubscriptionTitle(s.db, rawSub, pkg.ID, panel); rewriteErr != nil {
		s.logger.Warn("failed to rewrite v2ray subscription title immediately after adding a location, will be filled in by the next sync tick",
			zap.Uint("panel_id", panel.ID), zap.Error(rewriteErr))
	} else {
		location.ConfigLinkCached = rewritten
		now := time.Now()
		location.LastSyncedAt = &now
	}

	if err := s.db.Create(&location).Error; err != nil {
		s.logger.Error("failed to store v2ray package location", zap.Uint("panel_id", location.PanelID), zap.Error(err))
		return
	}
	// See CreatePackage's own identical doc comment: Enabled=false must
	// be applied via a separate Update, not set on the struct before
	// Create -- GORM's Create() treats an explicit false on a
	// gorm:"default:true" field as "unset" and substitutes the column
	// default instead.
	if addClientFailed && location.Enabled {
		if updateErr := s.db.Model(&location).Update("enabled", false).Error; updateErr != nil {
			s.logger.Error("failed to mark failed v2ray location as disabled", zap.Uint("panel_id", location.PanelID), zap.Error(updateErr))
		}
	}
}

// removePackageLocation disables loc's x-ui client on panel -- best
// effort, matching DeletePackage's own identical step; the caller is
// responsible for deleting loc's own database row afterward (this only
// handles the remote side, mirroring DeletePackage's separation of "one
// disable call per location" from "one bulk DB delete after they all
// finish").
func (s *V2RayPackageService) removePackageLocation(panel model.XuiPanel, loc model.V2RayPackageLocation) {
	ctx, cancel := context.WithTimeout(context.Background(), 20*time.Second)
	defer cancel()

	if err := xui.UpdateClient(ctx, panel, loc.ClientUUID, xui.XuiClient{
		ID:     loc.ClientUUID,
		Email:  loc.ClientEmail,
		Enable: false,
	}); err != nil {
		s.logger.Warn("failed to disable client on panel while removing a location from an existing package, continuing",
			zap.Uint("panel_id", loc.PanelID), zap.Error(err))
	}
}

// applyPackageStateToLocations pushes pkg's current Enable state (derived
// from Status -- only "active" is enabled, matching Peer/UserManagerAccount's
// own status-to-enabled convention) and TotalVolumeBytes to every one of
// this package's x-ui clients, concurrently, mirroring CreatePackage's own
// fan-out (same rationale: one unreachable panel must not block the rest).
// Best-effort per location -- a failure here is logged and left for the next
// sync tick to retry, same as every other x-ui call in this service.
func (s *V2RayPackageService) applyPackageStateToLocations(pkg model.V2RayPackage) {
	var locations []model.V2RayPackageLocation
	if err := s.db.Where("package_id = ?", pkg.ID).Find(&locations).Error; err != nil {
		s.logger.Error("failed to fetch locations to apply package state", zap.Uint("package_id", pkg.ID), zap.Error(err))
		return
	}

	enable := pkg.Status == "active"

	ctx := context.Background()
	var wg sync.WaitGroup
	for _, loc := range locations {
		panel, err := s.panels.GetPanel(loc.PanelID)
		if err != nil {
			continue
		}

		wg.Add(1)
		go func(panel model.XuiPanel, loc model.V2RayPackageLocation) {
			defer wg.Done()

			callCtx, cancel := context.WithTimeout(ctx, 20*time.Second)
			defer cancel()

			flow, flowErr := xui.ResolveClientFlow(callCtx, panel, panel.DefaultInboundID)
			if flowErr != nil {
				s.logger.Warn("failed to resolve inbound security settings before applying package state, defaulting to no flow",
					zap.Uint("panel_id", panel.ID), zap.Error(flowErr))
			}

			if err := xui.UpdateClient(callCtx, panel, loc.ClientUUID, xui.XuiClient{
				ID:      loc.ClientUUID,
				Flow:    flow,
				Email:   loc.ClientEmail,
				SubID:   loc.SubID,
				TotalGB: pkg.TotalVolumeBytes,
				Enable:  enable,
			}); err != nil {
				s.logger.Error("failed to apply v2ray package state to panel", zap.Uint("location_id", loc.ID), zap.Error(err))
				return
			}

			if updErr := s.db.Model(&model.V2RayPackageLocation{}).Where("id = ?", loc.ID).Update("enabled", enable).Error; updErr != nil {
				s.logger.Error("failed to persist v2ray location enabled state", zap.Uint("location_id", loc.ID), zap.Error(updErr))
			}
		}(panel, loc)
	}
	wg.Wait()
}

// DeletePackage removes a package and every one of its locations, both on
// each x-ui panel (best-effort -- a panel that's unreachable simply logs a
// warning, matching the "partial failure never blocks the operation"
// principle applied throughout this service) and in the local database.
// ResetUsage zeroes this package's DISPLAYED UsedBytes -- mirrors
// WgPeer/UserManagerService's "Reset Usage" action, adapted for V2Ray's
// own delta-accumulated UsedBytesCached (see model.V2RayPackage.
// UsageOffsetBytes' own doc comment for the full mechanism: this bumps the
// offset up to the package's CURRENT raw SUM(UsedBytesCached), which
// every read path then subtracts back out, without ever touching
// UsedBytesCached itself or the reseller-level V2RayUsedBytes pool --
// exactly the admin's own explicit requirement that a reset must never
// move reseller/global quota totals).
//
// A confirmed, reported bug this also fixes: a package the quota job had
// already suspended (SuspendedByQuota=true, Status="suspended", x-ui
// client disabled) stayed exactly that way after a usage reset -- zeroing
// UsageOffsetBytes alone never re-ran UpdatePackage's own "Status==active"
// reactivation branch, so the admin's reset appeared to succeed (the
// package now reads 0 used) while the customer's client remained offline
// until a separate, manual Status edit. Mirrors UpdatePackage's own
// Status="active" branch exactly (same three flags, same reason for
// clearing SuspendedByResellerQuota too), then re-applies package state to
// every x-ui location -- but ONLY when the package was actually suspended
// by quota; a package an admin deliberately suspended for an unrelated
// reason (no traffic-limit trigger) must not be silently re-enabled by a
// usage reset.
func (s *V2RayPackageService) ResetUsage(id uint, resellerID *uint) error {
	pkg, err := s.getPackageByIDScoped(id, resellerID)
	if err != nil {
		return fmt.Errorf("package not found: %w", err)
	}

	var rawUsedBytes int64
	if err := s.db.Model(&model.V2RayPackageLocation{}).
		Where("package_id = ?", pkg.ID).
		Select("COALESCE(SUM(used_bytes_cached), 0)").
		Scan(&rawUsedBytes).Error; err != nil {
		s.logger.Error("failed to sum v2ray package usage for reset", zap.Uint("package_id", pkg.ID), zap.Error(err))
		return fmt.Errorf("failed to reset package usage: %w", err)
	}

	updates := map[string]interface{}{
		"usage_offset_bytes": rawUsedBytes,
	}
	wasSuspendedByQuota := pkg.SuspendedByQuota
	if wasSuspendedByQuota {
		updates["status"] = "active"
		updates["suspended_by_quota"] = false
		updates["was_active_before_suspend"] = false
		updates["suspended_by_reseller_quota"] = false
	}

	if err := s.db.Model(&model.V2RayPackage{}).Where("id = ?", pkg.ID).Updates(updates).Error; err != nil {
		s.logger.Error("failed to reset v2ray package usage", zap.Uint("package_id", pkg.ID), zap.Error(err))
		return fmt.Errorf("failed to reset package usage: %w", err)
	}

	if wasSuspendedByQuota {
		pkg.Status = "active"
		pkg.SuspendedByQuota = false
		pkg.WasActiveBeforeSuspend = false
		pkg.SuspendedByResellerQuota = false
		s.applyPackageStateToLocations(pkg)
	}

	return nil
}

func (s *V2RayPackageService) DeletePackage(id uint, resellerID *uint) error {
	pkg, err := s.getPackageByIDScoped(id, resellerID)
	if err != nil {
		return fmt.Errorf("package not found: %w", err)
	}

	var locations []model.V2RayPackageLocation
	if err := s.db.Where("package_id = ?", pkg.ID).Find(&locations).Error; err != nil {
		s.logger.Error("failed to list v2ray package locations for delete", zap.Error(err))
		return fmt.Errorf("failed to delete v2ray package: %w", err)
	}

	// Concurrent, not sequential -- same fix and same rationale as
	// CreatePackage's fan-out (see its doc comment): a for loop here
	// would let N unreachable panels add up to N*15s to this request.
	var wg sync.WaitGroup
	for _, loc := range locations {
		panel, err := s.panels.GetPanel(loc.PanelID)
		if err != nil {
			s.logger.Warn("panel no longer exists, skipping remote delete for this location",
				zap.Uint("panel_id", loc.PanelID), zap.Error(err))
			continue
		}

		wg.Add(1)
		go func(panel model.XuiPanel, loc model.V2RayPackageLocation) {
			defer wg.Done()

			ctx, cancel := context.WithTimeout(context.Background(), 20*time.Second)
			defer cancel()

			if err := xui.UpdateClient(ctx, panel, loc.ClientUUID, xui.XuiClient{
				ID:     loc.ClientUUID,
				Email:  loc.ClientEmail,
				Enable: false,
			}); err != nil {
				s.logger.Warn("failed to disable client on panel during package delete, continuing",
					zap.Uint("panel_id", loc.PanelID), zap.Error(err))
			}
		}(panel, loc)
	}
	wg.Wait()

	// A confirmed, reported bug: V2RayUsedBytes is a LIVE SUM recomputed
	// every sync tick over currently-existing V2RayPackageLocation rows
	// (see applyResellerV2RayQuota), scoped by a JOIN to v2_ray_packages --
	// GORM's default soft-delete scope excludes both this package's own
	// row and its locations' rows from that SUM the instant they're
	// deleted, silently erasing this package's historical usage from the
	// owning reseller's total. Folding it into
	// Reseller.V2RayDeletedUsageBytes BEFORE the delete preserves it,
	// mirroring UserManagerService.DeleteAccount's identical fix (see that
	// field's own doc comment).
	if pkg.ResellerID != nil {
		var deletedUsage int64
		for _, loc := range locations {
			deletedUsage += loc.UsedBytesCached
		}
		if deletedUsage > 0 {
			if err := s.db.Model(&model.Reseller{}).Where("id = ?", *pkg.ResellerID).
				Update("v2_ray_deleted_usage_bytes", gorm.Expr("v2_ray_deleted_usage_bytes + ?", deletedUsage)).Error; err != nil {
				s.logger.Error("failed to credit deleted package's usage to reseller total", zap.Uint("reseller_id", *pkg.ResellerID), zap.Error(err))
				return fmt.Errorf("failed to preserve deleted package's usage: %w", err)
			}
		}
	}

	if err := s.db.Where("package_id = ?", pkg.ID).Delete(&model.V2RayPackageLocation{}).Error; err != nil {
		s.logger.Error("failed to delete v2ray package locations from database", zap.Error(err))
		return fmt.Errorf("failed to delete v2ray package: %w", err)
	}

	if err := s.db.Delete(&pkg).Error; err != nil {
		s.logger.Error("failed to delete v2ray package from database", zap.Error(err))
		return fmt.Errorf("failed to delete v2ray package: %w", err)
	}

	s.logResellerAction(resellerID, AuditActionV2RayPackageDeleted, fmt.Sprintf("Deleted v2ray package (uuid=%s)", pkg.UUID))

	return nil
}

// BulkDeletePackages deletes each given package via the exact same
// DeletePackage path one at a time -- mirrors WgPeer.BulkDeletePeers: no
// batch x-ui call exists, and one package's failure (e.g. an unreachable
// panel) must never block deleting the rest. Used by the
// "expired/quota-exhausted" bulk cleanup action.
func (s *V2RayPackageService) BulkDeletePackages(ids []uint, resellerID *uint) (deleted []uint, failed map[uint]string) {
	failed = make(map[uint]string)
	for _, id := range ids {
		if err := s.DeletePackage(id, resellerID); err != nil {
			failed[id] = err.Error()
			continue
		}
		deleted = append(deleted, id)
	}
	return deleted, failed
}

// ListPackages returns packages scoped to the caller -- nil resellerID
// (admin) returns only admin-owned packages (reseller_id IS NULL), matching
// UserManagerService.ListAccounts's exact scoping convention.
func (s *V2RayPackageService) ListPackages(resellerID *uint) ([]schema.V2RayPackageResponse, error) {
	return s.listPackagesScoped(resellerID)
}

// ListPackagesByReseller returns all packages owned by a specific reseller
// -- used by the admin-facing "Resellers V2Ray" oversight page.
func (s *V2RayPackageService) ListPackagesByReseller(resellerID uint) ([]schema.V2RayPackageResponse, error) {
	return s.listPackagesScoped(&resellerID)
}

func (s *V2RayPackageService) listPackagesScoped(resellerID *uint) ([]schema.V2RayPackageResponse, error) {
	var packages []model.V2RayPackage
	query := s.db.Model(&model.V2RayPackage{})
	if resellerID != nil {
		query = query.Where("reseller_id = ?", *resellerID)
	} else {
		query = query.Where("reseller_id IS NULL")
	}
	// Newest-first -- a confirmed, reported complaint: with no explicit
	// order, a freshly-created package landed wherever the DB engine
	// happened to return it (typically physical/insertion order without an
	// ORDER BY), forcing the admin to page to the end of a long list to
	// find the package they just made.
	if err := query.Order("id desc").Find(&packages).Error; err != nil {
		s.logger.Error("failed to fetch v2ray packages", zap.Error(err))
		return nil, fmt.Errorf("failed to fetch v2ray packages: %w", err)
	}

	resp := make([]schema.V2RayPackageResponse, 0, len(packages))
	for _, pkg := range packages {
		r, err := s.transformPackageToResponse(pkg)
		if err != nil {
			s.logger.Warn("failed to transform v2ray package to response, skipping", zap.Uint("id", pkg.ID), zap.Error(err))
			continue
		}
		resp = append(resp, r)
	}
	return resp, nil
}

// resolveSaleTitle implements the fallback rule: a reseller's custom title
// for panelID, or panel.SaleTitle if none was set. resellerID nil (an
// admin-direct package) always uses panel.SaleTitle -- custom titles are a
// reseller branding feature only.
func (s *V2RayPackageService) resolveSaleTitle(resellerID *uint, panel model.XuiPanel) string {
	return resolveV2RaySaleTitle(s.db, resellerID, panel)
}

// SetSaleTitle upserts a reseller's custom title for one panel -- admin-
// only (enforced at the HTTP layer), since only the admin assigns branding
// on a reseller's behalf per the plan's original design.
func (s *V2RayPackageService) SetSaleTitle(resellerID uint, req *schema.V2RaySaleTitleRequest) error {
	var existing model.ResellerV2RaySaleTitle
	err := s.db.Where("reseller_id = ? AND panel_id = ?", resellerID, req.PanelID).First(&existing).Error
	if err == nil {
		existing.Title = req.Title
		return s.db.Save(&existing).Error
	}
	if !errors.Is(err, gorm.ErrRecordNotFound) {
		return err
	}

	return s.db.Create(&model.ResellerV2RaySaleTitle{
		ResellerID: resellerID,
		PanelID:    req.PanelID,
		Title:      req.Title,
	}).Error
}

func (s *V2RayPackageService) ListSaleTitles(resellerID uint) ([]schema.V2RaySaleTitleResponse, error) {
	var titles []model.ResellerV2RaySaleTitle
	if err := s.db.Where("reseller_id = ?", resellerID).Find(&titles).Error; err != nil {
		return nil, err
	}

	resp := make([]schema.V2RaySaleTitleResponse, 0, len(titles))
	for _, t := range titles {
		var panel model.XuiPanel
		panelName := "unknown"
		if err := s.db.Select("name").First(&panel, t.PanelID).Error; err == nil {
			panelName = panel.Name
		}
		resp = append(resp, schema.V2RaySaleTitleResponse{
			PanelID:   t.PanelID,
			PanelName: panelName,
			Title:     t.Title,
		})
	}
	return resp, nil
}

// GetPackageShareDetails is the public, unauthenticated lookup behind the
// share page -- a pure DB read of CombinedConfigCached, never a live x-ui
// call (see V2RayPackage.CombinedConfigCached's doc comment).
func (s *V2RayPackageService) GetPackageShareDetails(uuidStr, publicBaseURL string) (*schema.V2RayPackageShareDetailsResponse, error) {
	var pkg model.V2RayPackage
	if err := s.db.First(&pkg, "uuid = ?", uuidStr).Error; err != nil {
		if errors.Is(err, gorm.ErrRecordNotFound) {
			return nil, err
		}
		s.logger.Error("failed to find v2ray package in database", zap.Error(err))
		return nil, err
	}

	if !utils.IsPeerSharable(pkg.IsShared, pkg.ShareExpireTime) {
		return nil, common.ErrV2RayPackageNotShared
	}

	var locations []model.V2RayPackageLocation
	if err := s.db.Where("package_id = ?", pkg.ID).Find(&locations).Error; err != nil {
		s.logger.Error("failed to fetch v2ray package locations for share details", zap.Error(err))
		return nil, err
	}

	locationUsages := make([]schema.V2RayShareLocationUsage, 0, len(locations))
	var usedBytes int64
	isOnline := false
	for _, loc := range locations {
		usedBytes += loc.UsedBytesCached

		var panel model.XuiPanel
		title := "Unknown"
		if err := s.db.First(&panel, loc.PanelID).Error; err == nil {
			title = s.resolveSaleTitle(pkg.ResellerID, panel)
		}

		// IsOnline is the real per-location value cached by the sync job's
		// panel-wide onlines poll (see V2RaySyncService.SyncPackageUsage) --
		// this endpoint itself never calls x-ui live (see this method's own
		// doc comment), it only reads back what the last tick observed.
		if loc.IsOnline {
			isOnline = true
		}

		locationUsages = append(locationUsages, schema.V2RayShareLocationUsage{
			Title:      title,
			ConfigLink: loc.ConfigLinkCached,
			UsedBytes:  loc.UsedBytesCached,
			IsOnline:   loc.IsOnline,
			Protocol:   panel.Protocol,
		})
	}

	// Offset-adjusted, same as transformPackageToResponse -- see
	// model.V2RayPackage.UsageOffsetBytes' own doc comment.
	displayedUsedBytes := usedBytes - pkg.UsageOffsetBytes
	if displayedUsedBytes < 0 {
		displayedUsedBytes = 0
	}

	var usagePercent *string
	if pkg.TotalVolumeBytes > 0 {
		percent := float64(displayedUsedBytes) / float64(pkg.TotalVolumeBytes) * 100
		usagePercent = utils.Ptr(fmt.Sprintf("%.1f", percent))
	}

	var expireAt *string
	var daysRemaining *int
	if pkg.ExpireAt != nil {
		formatted := pkg.ExpireAt.Format("2006-01-02")
		expireAt = &formatted

		days := int(time.Until(*pkg.ExpireAt).Hours() / 24)
		if days < 0 {
			days = 0
		}
		daysRemaining = &days
	}

	return &schema.V2RayPackageShareDetailsResponse{
		SubscriptionURL:  fmt.Sprintf("%s/api/v2ray-sub/%s", publicBaseURL, pkg.UUID),
		CustomerLabel:    pkg.CustomerLabel,
		Status:           pkg.Status,
		TotalVolumeBytes: pkg.TotalVolumeBytes,
		UsedBytes:        displayedUsedBytes,
		UsagePercent:     usagePercent,
		ExpireAt:         expireAt,
		DaysRemaining:    daysRemaining,
		IsOnline:         isOnline,
		Locations:        locationUsages,
	}, nil
}

// GetLiveUsage is the on-demand "view live usage" action -- unlike every
// other read path in this service, it calls xui.GetClientTraffics LIVE,
// for every one of packageID's locations, right now, at request time. This
// is a deliberate, narrow exception to CombinedConfigCached/
// UsedBytesCached's own "never call x-ui at request time" rule (see
// V2RayPackage.CombinedConfigCached's doc comment) -- added specifically
// so an admin/reseller can confirm a customer's CURRENT usage on demand
// (e.g. while on a support call) without waiting for the next scheduled
// sync tick. The normal package list view must keep reading the periodic
// job's cached UsedBytesCached, exactly as before; this action refreshes
// that cache as a side effect (so the next list view reflects this read
// too) but is never itself the list view's data source.
func (s *V2RayPackageService) GetLiveUsage(id uint, resellerID *uint) (*schema.V2RayLiveUsageResponse, error) {
	pkg, err := s.getPackageByIDScoped(id, resellerID)
	if err != nil {
		return nil, err
	}

	var locations []model.V2RayPackageLocation
	if err := s.db.Where("package_id = ?", pkg.ID).Find(&locations).Error; err != nil {
		return nil, fmt.Errorf("failed to fetch v2ray package locations: %w", err)
	}

	ctx := context.Background()
	rows := make([]schema.V2RayLiveUsageLocation, 0, len(locations))
	var totalUsedBytes int64

	for _, loc := range locations {
		panel, panelErr := s.panels.GetPanel(loc.PanelID)
		if panelErr != nil {
			errStr := "panel no longer exists"
			rows = append(rows, schema.V2RayLiveUsageLocation{PanelID: loc.PanelID, Enabled: loc.Enabled, Error: &errStr})
			continue
		}

		up, down, trafficErr := xui.GetClientTrafficsSplit(ctx, panel, loc.ClientEmail)
		if trafficErr != nil {
			errStr := trafficErr.Error()
			rows = append(rows, schema.V2RayLiveUsageLocation{
				PanelID: loc.PanelID, PanelName: panel.Name, Enabled: loc.Enabled, Error: &errStr,
			})
			continue
		}

		total := up + down
		totalUsedBytes += total

		rows = append(rows, schema.V2RayLiveUsageLocation{
			PanelID: loc.PanelID, PanelName: panel.Name, Enabled: loc.Enabled,
			UpBytes: up, DownBytes: down, TotalBytes: total,
		})

		// Refresh the cache as a side effect -- LastTotalUsedBytes tracks
		// x-ui's own absolute counter (matching syncOneLocation's own
		// delta-accumulation contract), UsedBytesCached is set directly to
		// this fresh total rather than delta-accumulated a second time,
		// since this total already IS the full up-to-date absolute reading.
		if err := s.db.Model(&model.V2RayPackageLocation{}).Where("id = ?", loc.ID).Updates(map[string]interface{}{
			"used_bytes_cached":     total,
			"last_total_used_bytes": total,
		}).Error; err != nil {
			s.logger.Warn("failed to persist live usage refresh", zap.Uint("location_id", loc.ID), zap.Error(err))
		}
	}

	return &schema.V2RayLiveUsageResponse{
		TotalVolumeBytes: pkg.TotalVolumeBytes,
		TotalUsedBytes:   totalUsedBytes,
		Locations:        rows,
	}, nil
}

// GetSelfSummary is a reseller's own V2Ray dashboard aggregate -- mirrors
// UserManagerService.GetSelfSummary's exact shape/pattern, a pure DB read
// (no live x-ui calls -- reuses whatever the periodic sync job has already
// cached, matching every other dashboard-summary read in this codebase).
func (s *V2RayPackageService) GetSelfSummary(resellerID uint) (*schema.V2RaySelfSummaryResponse, error) {
	var reseller model.Reseller
	if err := s.db.First(&reseller, resellerID).Error; err != nil {
		s.logger.Error("failed to fetch reseller for v2ray summary", zap.Uint("resellerID", resellerID), zap.Error(err))
		return nil, err
	}

	var packages []model.V2RayPackage
	if err := s.db.Where("reseller_id = ?", resellerID).Find(&packages).Error; err != nil {
		s.logger.Error("failed to fetch v2ray packages for summary", zap.Uint("resellerID", resellerID), zap.Error(err))
		return nil, err
	}

	packageIDs := make([]uint, len(packages))
	for i, p := range packages {
		packageIDs[i] = p.ID
	}

	online := 0
	if len(packageIDs) > 0 {
		// "Online" here means at least one of the package's locations was
		// observed connected on the sync job's last onlines poll (see
		// V2RayPackageLocation.IsOnline's own doc comment) -- one query
		// across every package at once, not one per package.
		var onlinePackageIDs []uint
		if err := s.db.Model(&model.V2RayPackageLocation{}).
			Where("package_id IN ? AND is_online = ?", packageIDs, true).
			Distinct().Pluck("package_id", &onlinePackageIDs).Error; err != nil {
			s.logger.Warn("failed to count online v2ray packages for summary", zap.Uint("resellerID", resellerID), zap.Error(err))
		} else {
			online = len(onlinePackageIDs)
		}
	}

	var remaining *int64
	if reseller.V2RayQuotaBytes != nil {
		r := *reseller.V2RayQuotaBytes - reseller.V2RayUsedBytes
		if r < 0 {
			r = 0
		}
		remaining = &r
	}

	return &schema.V2RaySelfSummaryResponse{
		OnlinePackages: online,
		TotalPackages:  len(packages),
		QuotaBytes:     reseller.V2RayQuotaBytes,
		UsedBytes:      reseller.V2RayUsedBytes,
		RemainingBytes: remaining,
		MaxPackages:    reseller.V2RayMaxPackages,
	}, nil
}

// GetAdminSummary is the admin dashboard's panel-wide V2Ray rollup -- every
// registered panel's own location/online count plus grand totals across
// every package system-wide (admin-direct and every reseller's combined).
// A pure DB read, same as GetSelfSummary above.
func (s *V2RayPackageService) GetAdminSummary() (*schema.V2RayAdminSummaryResponse, error) {
	var panels []model.XuiPanel
	if err := s.db.Find(&panels).Error; err != nil {
		s.logger.Error("failed to fetch xui panels for v2ray admin summary", zap.Error(err))
		return nil, err
	}

	var locations []model.V2RayPackageLocation
	if err := s.db.Find(&locations).Error; err != nil {
		s.logger.Error("failed to fetch v2ray locations for admin summary", zap.Error(err))
		return nil, err
	}

	type panelAgg struct {
		locationCount  int
		onlineCount    int
		hasRecentError bool
	}
	aggByPanel := make(map[uint]*panelAgg, len(panels))
	var totalUsedBytes int64
	for _, loc := range locations {
		agg, ok := aggByPanel[loc.PanelID]
		if !ok {
			agg = &panelAgg{}
			aggByPanel[loc.PanelID] = agg
		}
		agg.locationCount++
		if loc.IsOnline {
			agg.onlineCount++
		}
		// Confirmed, reported bug: this previously counted LastSyncError
		// on EVERY location regardless of Enabled, with no recency bound
		// at all -- SyncPackageUsage (v2ray_sync.go) only ever polls
		// Enabled locations, so a DISABLED location's LastSyncError (set
		// back when it was last synced, before being disabled by quota/
		// expiry/an admin) can never be cleared, since the sync loop that
		// would clear it never runs for a disabled location again. This
		// permanently pinned "Sync error" on a panel's admin-dashboard row
		// even when every one of its currently-active locations was
		// syncing perfectly -- confirmed on the live database: 5 panels
		// each had a small number of long-disabled locations still
		// carrying a stale error (one with a NULL LastSyncedAt, meaning
		// it was disabled before ever syncing even once) while every
		// enabled location on those same panels synced with zero errors.
		// Scoping this to Enabled locations only means a panel's health
		// flag reflects what SyncPackageUsage is ACTUALLY still checking
		// on it, matching the location-dot UI's own identical
		// Enabled-gates-the-error-color convention (V2RayLocationStatus).
		if loc.Enabled && loc.LastSyncError != nil {
			agg.hasRecentError = true
		}
		totalUsedBytes += loc.UsedBytesCached
	}

	panelRows := make([]schema.V2RayAdminSummaryPanel, 0, len(panels))
	onlineLocations := 0
	for _, p := range panels {
		agg := aggByPanel[p.ID]
		if agg == nil {
			agg = &panelAgg{}
		}
		onlineLocations += agg.onlineCount
		panelRows = append(panelRows, schema.V2RayAdminSummaryPanel{
			PanelID:        p.ID,
			PanelName:      p.Name,
			LocationCount:  agg.locationCount,
			OnlineCount:    agg.onlineCount,
			HasRecentError: agg.hasRecentError,
		})
	}

	var totalPackages int64
	if err := s.db.Model(&model.V2RayPackage{}).Count(&totalPackages).Error; err != nil {
		s.logger.Error("failed to count v2ray packages for admin summary", zap.Error(err))
		return nil, err
	}

	var totalVolumeBytes int64
	if err := s.db.Model(&model.V2RayPackage{}).
		Select("COALESCE(SUM(total_volume_bytes), 0)").Scan(&totalVolumeBytes).Error; err != nil {
		s.logger.Error("failed to sum v2ray package volume for admin summary", zap.Error(err))
		return nil, err
	}

	return &schema.V2RayAdminSummaryResponse{
		TotalPackages:    int(totalPackages),
		TotalLocations:   len(locations),
		OnlineLocations:  onlineLocations,
		TotalVolumeBytes: totalVolumeBytes,
		TotalUsedBytes:   totalUsedBytes,
		Panels:           panelRows,
	}, nil
}

// V2RaySubscriptionContent is GetCombinedSubscription's full result --
// besides the raw base64 body, it carries the numbers needed to build the
// `Subscription-Userinfo` header most V2Ray client apps (v2rayNG, v2box,
// Shadowrocket, etc.) expect on a subscription response; several clients
// treat its absence as an invalid/unparsable subscription and show an
// error even though the base64 body itself is perfectly valid.
type V2RaySubscriptionContent struct {
	Body       string
	UsedBytes  int64
	TotalBytes int64
	ExpireAt   *time.Time
}

// GetCombinedSubscription is the raw text/plain body served at
// /api/public/v2ray-sub/{uuid} -- a pure DB read, matching
// GetPackageShareDetails's "never call x-ui live" rule.
func (s *V2RayPackageService) GetCombinedSubscription(uuidStr string) (*V2RaySubscriptionContent, error) {
	var pkg model.V2RayPackage
	if err := s.db.First(&pkg, "uuid = ?", uuidStr).Error; err != nil {
		return nil, err
	}
	if !utils.IsPeerSharable(pkg.IsShared, pkg.ShareExpireTime) {
		return nil, common.ErrV2RayPackageNotShared
	}

	var usedBytes int64
	s.db.Model(&model.V2RayPackageLocation{}).
		Where("package_id = ?", pkg.ID).
		Select("COALESCE(SUM(used_bytes_cached), 0)").
		Scan(&usedBytes)

	// Offset-adjusted, same as transformPackageToResponse/
	// GetPackageShareDetails -- see model.V2RayPackage.UsageOffsetBytes'
	// own doc comment. The client app reading this Subscription-Userinfo
	// value must see the same post-reset usage the panel itself shows.
	displayedUsedBytes := usedBytes - pkg.UsageOffsetBytes
	if displayedUsedBytes < 0 {
		displayedUsedBytes = 0
	}

	return &V2RaySubscriptionContent{
		Body:       pkg.CombinedConfigCached,
		UsedBytes:  displayedUsedBytes,
		TotalBytes: pkg.TotalVolumeBytes,
		ExpireAt:   pkg.ExpireAt,
	}, nil
}

// GetPackageShareStatus/UpdatePackageShareStatus/UpdatePackageShareExpire
// mirror UserManagerService's equivalent share endpoints exactly, scoped
// to a V2RayPackage.
func (s *V2RayPackageService) GetPackageShareStatus(id uint, resellerID *uint) (*schema.V2RayPackageShareStatusResponse, error) {
	pkg, err := s.getPackageByIDScoped(id, resellerID)
	if err != nil {
		return nil, err
	}

	var uuidPtr *string
	if pkg.IsShared {
		uuidPtr = &pkg.UUID
	}

	return &schema.V2RayPackageShareStatusResponse{
		IsShared:   pkg.IsShared,
		UUID:       uuidPtr,
		ExpireTime: pkg.ShareExpireTime,
	}, nil
}

func (s *V2RayPackageService) UpdatePackageShareStatus(id uint, resellerID *uint) error {
	pkg, err := s.getPackageByIDScoped(id, resellerID)
	if err != nil {
		return fmt.Errorf("package not found: %w", err)
	}

	if err := s.db.Model(&pkg).Update("is_shared", !pkg.IsShared).Error; err != nil {
		s.logger.Error("failed to update v2ray package share status", zap.Error(err))
		return fmt.Errorf("failed to update share status: %w", err)
	}
	return nil
}

func (s *V2RayPackageService) UpdatePackageShareExpire(id uint, expireTime *string, resellerID *uint) error {
	pkg, err := s.getPackageByIDScoped(id, resellerID)
	if err != nil {
		return fmt.Errorf("package not found: %w", err)
	}
	if !pkg.IsShared {
		return fmt.Errorf("v2ray package is not shared, cannot set expire time")
	}

	if err := s.db.Model(&pkg).Update("share_expire_time", expireTime).Error; err != nil {
		s.logger.Error("failed to update v2ray package share expire time", zap.Error(err))
		return fmt.Errorf("failed to update share expire time: %w", err)
	}
	return nil
}

func (s *V2RayPackageService) transformPackageToResponse(pkg model.V2RayPackage) (schema.V2RayPackageResponse, error) {
	var locations []model.V2RayPackageLocation
	if err := s.db.Where("package_id = ?", pkg.ID).Find(&locations).Error; err != nil {
		return schema.V2RayPackageResponse{}, fmt.Errorf("failed to fetch v2ray package locations: %w", err)
	}

	var usedBytes int64
	locationStatuses := make([]schema.V2RayPackageLocationStatus, 0, len(locations))
	for _, loc := range locations {
		usedBytes += loc.UsedBytesCached

		panelName := "unknown"
		var panel model.XuiPanel
		if err := s.db.Select("name").First(&panel, loc.PanelID).Error; err == nil {
			panelName = panel.Name
		}

		var lastSyncedAt *string
		if loc.LastSyncedAt != nil {
			formatted := loc.LastSyncedAt.Format("2006-01-02T15:04:05Z07:00")
			lastSyncedAt = &formatted
		}

		locationStatuses = append(locationStatuses, schema.V2RayPackageLocationStatus{
			PanelID:       loc.PanelID,
			PanelName:     panelName,
			Enabled:       loc.Enabled,
			LastSyncedAt:  lastSyncedAt,
			LastSyncError: loc.LastSyncError,
			HadWrongFlow:  loc.HadWrongFlow,
			FlowRepaired:  loc.FlowRepairedAt != nil,
			IsOnline:      loc.IsOnline,
		})
	}

	var startAt, expireAt *string
	if pkg.StartAt != nil {
		formatted := pkg.StartAt.Format("2006-01-02")
		startAt = &formatted
	}
	if pkg.ExpireAt != nil {
		formatted := pkg.ExpireAt.Format("2006-01-02")
		expireAt = &formatted
	}

	// Displayed usage is offset-adjusted -- see model.V2RayPackage.
	// UsageOffsetBytes' own doc comment for why this is subtracted here
	// rather than from the underlying V2RayPackageLocation.UsedBytesCached
	// rows themselves (those must stay raw so the reseller-level quota sum
	// is never affected by a "Reset Usage" action).
	displayedUsedBytes := usedBytes - pkg.UsageOffsetBytes
	if displayedUsedBytes < 0 {
		displayedUsedBytes = 0
	}

	return schema.V2RayPackageResponse{
		Id:               pkg.ID,
		UUID:             pkg.UUID,
		CustomerLabel:    pkg.CustomerLabel,
		Comment:          pkg.Comment,
		TotalVolumeBytes: pkg.TotalVolumeBytes,
		UsedBytes:        displayedUsedBytes,
		DurationDays:     pkg.DurationDays,
		StartAt:          startAt,
		ExpireAt:         expireAt,
		Status:           pkg.Status,
		IsShared:         pkg.IsShared,
		Locations:        locationStatuses,
		ResellerID:       pkg.ResellerID,
	}, nil
}
