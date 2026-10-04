package service

import (
	"context"
	"encoding/base64"
	"fmt"
	"strconv"
	"strings"
	"time"

	"go.uber.org/zap"
	"gorm.io/gorm"
	"gorm.io/gorm/clause"

	"github.com/maahdima/mwp/api/adaptor/xui"
	"github.com/maahdima/mwp/api/dataservice/model"
)

// v2rayByteCounterMax bounds calculateV2RayDelta's reset/wrap detection --
// x-ui reports plain 64-bit byte counters (unlike RouterOS's unit-suffixed
// strings), so this is a generous sentinel rather than a real hardware
// limit: any counter drop this large is treated as a reset, never a wrap.
const v2rayByteCounterMax = 1 << 62

// V2RaySyncService is the background job body implementing every x-ui
// network call this feature makes -- CreatePackage/DeletePackage in
// V2RayPackageService also call the adaptor directly for their own
// immediate create/delete needs, but ALL usage polling and subscription
// content fetching happens here, on its own scheduled tick, never at
// request time (see V2RayPackage.CombinedConfigCached's doc comment for
// why the public share endpoint never calls x-ui live).
type V2RaySyncService struct {
	db            *gorm.DB
	panels        *XuiPanelService
	logger        *zap.Logger
	usageRecorder *UsageSnapshotWriter
	billing       *ResellerBillingService
}

// SetResellerBiller wires the Payment-based billing engine after
// construction, mirroring SetUsageRecorder's own "optional, set once after
// construction" pattern. Safe to leave unset -- enforcePackageQuota simply
// skips the payment-based charge (Volume-based enforcement is entirely
// unaffected either way) if nil.
func (s *V2RaySyncService) SetResellerBiller(billing *ResellerBillingService) {
	s.billing = billing
}

func NewV2RaySyncService(db *gorm.DB, panels *XuiPanelService) *V2RaySyncService {
	return &V2RaySyncService{
		db:     db,
		panels: panels,
		logger: zap.L().Named("V2RaySyncService"),
	}
}

// SetUsageRecorder wires the Reports section's usage-history writer after
// construction, mirroring V2RayPackageService.SetBotNotifier's own
// "optional, set once after construction" pattern. Safe to leave unset --
// syncOneLocation simply skips recording a snapshot if nil.
func (s *V2RaySyncService) SetUsageRecorder(recorder *UsageSnapshotWriter) {
	s.usageRecorder = recorder
}

// SyncPackageUsage is the scheduled job entrypoint. For every enabled
// location it polls usage and refreshes the cached subscription link; a
// single location's failure (a panel being down, in particular) is caught
// and recorded on that location alone via an explicit `continue`, never a
// `return` -- one dead panel must never stop every other location (on the
// same package or a different one) from syncing on this tick. After all
// locations are attempted, every package's CombinedConfigCached is
// recomputed from whatever ConfigLinkCached values are currently non-empty,
// and reseller V2Ray quota pools are updated/enforced.
func (s *V2RaySyncService) SyncPackageUsage() {
	var locations []model.V2RayPackageLocation
	if err := s.db.Where("enabled = ?", true).Find(&locations).Error; err != nil {
		s.logger.Error("failed to list v2ray package locations for sync", zap.Error(err))
		return
	}

	ctx := context.Background()
	touchedPackageIDs := make(map[uint]struct{})

	// A confirmed, reported bug: enforcePackageQuota (which alone knows how
	// to re-check BillingSuspended and call reenablePackageLocations) only
	// ever runs for a package reached via the loop below -- and that loop
	// is driven ENTIRELY by currently enabled=true locations. Once every
	// one of a package's locations was disabled (whatever the original
	// cause), the package could never be reached again by this tick, so it
	// could never self-heal even after ChargeUsage/ResumeBillingSuspension
	// cleared the reseller's own BillingSuspended flag -- only a manual
	// admin/API resume could bring it back. Explicitly adding every
	// currently suspended-by-quota package's ID here (regardless of its
	// locations' enabled state) closes that gap: enforcePackageQuota
	// itself already re-derives the correct BillingMode/BillingSuspended/
	// TotalVolumeBytes decision from scratch, so re-running it for an
	// already-suspended package is always safe, including the "still
	// genuinely over quota" case, which just re-confirms suspension.
	var suspendedPackageIDs []uint
	if err := s.db.Model(&model.V2RayPackage{}).
		Where("suspended_by_quota = ?", true).
		Pluck("id", &suspendedPackageIDs).Error; err != nil {
		s.logger.Error("failed to list suspended v2ray packages for resume check", zap.Error(err))
	} else {
		for _, id := range suspendedPackageIDs {
			touchedPackageIDs[id] = struct{}{}
		}
	}

	// GetOnlineClients is a panel-wide call (every online client across
	// every inbound), not scoped to one location -- fetched at most once
	// per distinct panel per tick and reused for every location on that
	// panel, rather than once per location, so a package with many
	// locations on the same panel doesn't multiply this call unnecessarily.
	onlineByPanel := make(map[uint]map[string]bool)

	for _, loc := range locations {
		touchedPackageIDs[loc.PackageID] = struct{}{}

		panel, err := s.panels.GetPanel(loc.PanelID)
		if err != nil {
			s.recordLocationError(loc.ID, fmt.Errorf("panel no longer exists: %w", err))
			continue
		}

		online, polled := onlineByPanel[panel.ID]
		if !polled {
			result, onlineErr := xui.GetOnlineClients(ctx, panel)
			if onlineErr != nil {
				s.logger.Warn("failed to poll v2ray panel online status, leaving locations' is_online unchanged this tick",
					zap.Uint("panel_id", panel.ID), zap.Error(onlineErr))
				result = nil
			}
			onlineByPanel[panel.ID] = result
			online = result
		}
		if online != nil {
			s.updateLocationOnlineStatus(loc.ID, online[loc.ClientEmail])
		}

		s.syncOneLocation(ctx, loc, panel)
	}

	for packageID := range touchedPackageIDs {
		s.recombineConfig(packageID)
		s.enforcePackageQuota(ctx, packageID)
	}
}

// updateLocationOnlineStatus persists this tick's online/offline reading
// for one location -- a separate, small update from syncOneLocation's own
// Updates call since the online poll is panel-scoped and happens before
// per-location traffic/subscription sync, not after.
func (s *V2RaySyncService) updateLocationOnlineStatus(locationID uint, isOnline bool) {
	if err := s.db.Model(&model.V2RayPackageLocation{}).Where("id = ?", locationID).Updates(map[string]interface{}{
		"is_online":         isOnline,
		"online_checked_at": timeNow(),
	}).Error; err != nil {
		s.logger.Warn("failed to persist v2ray location online status", zap.Uint("location_id", locationID), zap.Error(err))
	}
}

func (s *V2RaySyncService) syncOneLocation(ctx context.Context, loc model.V2RayPackageLocation, panel model.XuiPanel) {
	if loc.FlowRepairedAt == nil {
		s.repairClientFlowIfNeeded(ctx, loc, panel)
	}

	totalBytes, err := xui.GetClientTraffics(ctx, panel, loc.ClientEmail)
	if err != nil {
		s.recordLocationError(loc.ID, err)
		return
	}

	delta, wasReset := calculateV2RayDelta(loc.LastTotalUsedBytes, totalBytes, v2rayByteCounterMax)
	if wasReset {
		s.logger.Info("v2ray location counter reset detected", zap.Uint("location_id", loc.ID), zap.Int64("prev", loc.LastTotalUsedBytes), zap.Int64("current", totalBytes))
	}

	newUsedBytes := loc.UsedBytesCached + delta
	if delta < 0 {
		// A negative delta should be structurally impossible given
		// calculateV2RayDelta's contract, but guard against ever
		// decreasing the cached total from a single bad reading.
		newUsedBytes = loc.UsedBytesCached
	}

	if delta > 0 {
		var pkg model.V2RayPackage
		if err := s.db.Select("reseller_id").First(&pkg, loc.PackageID).Error; err == nil {
			if s.usageRecorder != nil {
				s.usageRecorder.RecordV2Ray(loc.PackageID, loc.PanelID, pkg.ResellerID, delta)
			}
			// Payment-based billing: charged here (per-location, on the
			// fresh delta) rather than in enforcePackageQuota (which runs
			// once per PACKAGE, summing across locations) since a no-op for
			// non-Payment resellers is cheap and this is the one place that
			// actually has the fresh per-tick delta; enforcePackageQuota
			// below re-checks BillingSuspended once per package afterward
			// to actually disable/re-enable.
			if s.billing != nil && pkg.ResellerID != nil {
				locationKey := strconv.FormatUint(uint64(loc.PanelID), 10)
				_ = s.billing.ChargeUsage(*pkg.ResellerID, model.ResellerBillingProductV2Ray, locationKey, delta)
			}
		}
	}

	rawSub, subErr := xui.GetSubscription(ctx, panel, loc.SubID)

	updates := map[string]interface{}{
		"used_bytes_cached":     newUsedBytes,
		"last_total_used_bytes": totalBytes,
		"last_synced_at":        timeNow(),
		"last_sync_error":       nil,
	}

	if subErr != nil {
		s.logger.Warn("failed to fetch v2ray subscription content, keeping previous cached link",
			zap.Uint("location_id", loc.ID), zap.Error(subErr))
	} else if rewritten, rewriteErr := s.rewriteSubscriptionTitle(rawSub, loc, panel); rewriteErr != nil {
		s.logger.Warn("failed to rewrite v2ray subscription title, keeping previous cached link",
			zap.Uint("location_id", loc.ID), zap.Error(rewriteErr))
	} else {
		updates["config_link_cached"] = rewritten
	}

	if err := s.db.Model(&model.V2RayPackageLocation{}).Where("id = ?", loc.ID).Updates(updates).Error; err != nil {
		s.logger.Error("failed to persist v2ray location sync results", zap.Uint("location_id", loc.ID), zap.Error(err))
	}
}

// repairClientFlowIfNeeded is a one-time-per-location migration step (see
// V2RayPackageLocation.FlowRepairedAt's own doc comment for the full
// rationale): reads back this location's CURRENT flow value directly from
// x-ui, compares it against what the inbound's real security setting
// requires, and issues a single corrective UpdateClient only if they
// actually differ -- a location created before the flow fix on a
// security:"tls"/"reality" inbound is untouched (it was already correct),
// while one created on a security:"none" inbound with the old hardcoded
// "xtls-rprx-vision" gets its flow cleared to "" so it can finally
// connect. FlowRepairedAt is set regardless of whether a repair was
// actually needed, so this check runs exactly once per location, ever --
// not on every sync tick.
func (s *V2RaySyncService) repairClientFlowIfNeeded(ctx context.Context, loc model.V2RayPackageLocation, panel model.XuiPanel) {
	markChecked := func(hadWrongFlow bool) {
		if err := s.db.Model(&model.V2RayPackageLocation{}).Where("id = ?", loc.ID).Updates(map[string]interface{}{
			"flow_repaired_at": timeNow(),
			"had_wrong_flow":   hadWrongFlow,
		}).Error; err != nil {
			s.logger.Error("failed to record v2ray flow repair check", zap.Uint("location_id", loc.ID), zap.Error(err))
		}
	}

	correctFlow, err := xui.ResolveClientFlow(ctx, panel, panel.DefaultInboundID)
	if err != nil {
		// Don't mark this checked on failure -- a panel that's briefly
		// unreachable should get another chance on a future tick, not be
		// silently skipped forever because of a transient error.
		s.logger.Warn("failed to resolve inbound security settings during flow repair check, will retry on a future tick",
			zap.Uint("location_id", loc.ID), zap.Error(err))
		return
	}

	currentFlow, found, err := xui.GetInboundClientFlow(ctx, panel, panel.DefaultInboundID, loc.ClientEmail)
	if err != nil {
		s.logger.Warn("failed to read current client flow during flow repair check, will retry on a future tick",
			zap.Uint("location_id", loc.ID), zap.Error(err))
		return
	}
	if !found {
		// The client no longer exists on x-ui (deleted directly on the
		// panel, outside this codebase) -- nothing to repair, and no
		// point re-checking a client that isn't there.
		markChecked(false)
		return
	}

	if currentFlow == correctFlow {
		markChecked(false)
		return
	}

	s.logger.Warn("found v2ray client with incorrect flow, repairing",
		zap.Uint("location_id", loc.ID), zap.String("client_email", loc.ClientEmail),
		zap.String("had_flow", currentFlow), zap.String("correct_flow", correctFlow))

	if err := xui.UpdateClient(ctx, panel, loc.ClientUUID, xui.XuiClient{
		ID:     loc.ClientUUID,
		Flow:   correctFlow,
		Email:  loc.ClientEmail,
		SubID:  loc.SubID,
		Enable: loc.Enabled,
	}); err != nil {
		s.logger.Error("failed to repair v2ray client flow, will retry on a future tick",
			zap.Uint("location_id", loc.ID), zap.Error(err))
		return
	}

	markChecked(true)
}

// rewriteSubscriptionTitle decodes panel's raw subscription content -- a
// newline-separated list of vless/vmess/... links, either base64-encoded
// (x-ui's "Subscription Encode" setting on, the default) OR plaintext
// (that same setting turned off, in which case /sub/{subId} returns the
// raw link text directly -- see isRawLinkContent) -- replaces every link's
// title (the part after '#') with the resolved sale title for this
// location's package, and returns PLAINTEXT (not re-encoded) -- the value
// stored in V2RayPackageLocation.ConfigLinkCached is plaintext by design,
// only ever read back by recombineConfig (never served directly to any
// client), which base64-encodes the FINAL cross-panel-combined result
// exactly once. An earlier version of this function returned base64 here,
// which recombineConfig then tried to base64-DECODE again -- a real bug a
// live smoke test caught: every location's rewritten content was valid
// plaintext, not valid base64, so that decode failed silently on every
// single location (recombineConfig's own "one failed location never
// breaks the whole package" `continue`), leaving CombinedConfigCached
// permanently empty for every package regardless of sync success.
//
// A second, separately reported bug: this function used to unconditionally
// base64-decode the input, which fails with "illegal base64 data at input
// byte 5" (the ':' right after "vless") whenever a panel has its own
// "Subscription Encode" turned off. syncOneLocation treats that error as
// non-fatal and keeps ConfigLinkCached at its previous value, which for a
// location that had never synced successfully is empty -- explaining both
// a missing config link AND a missing QR code on the share page (the
// frontend only renders a QR for a non-empty config_link) for any panel
// configured with encoding off.
func (s *V2RaySyncService) rewriteSubscriptionTitle(rawSub string, loc model.V2RayPackageLocation, panel model.XuiPanel) (string, error) {
	return rewriteV2RaySubscriptionTitle(s.db, rawSub, loc.PackageID, panel)
}

// v2raySchemePrefixes are every link scheme x-ui's subscription endpoint
// can return in plaintext when a panel's own "Subscription Encode"
// (base64) setting is turned off -- checked in rewriteSubscriptionTitle
// before attempting a base64 decode, so this codebase works correctly
// against panels configured either way, not just the base64-on default.
var v2raySchemePrefixes = []string{"vless://", "vmess://", "trojan://", "ss://"}

// isRawLinkContent reports whether trimmed is already a plain link list
// (one or more lines each starting with a known V2Ray scheme prefix)
// rather than base64-encoded content -- only the first non-empty line is
// checked, since a subscription response is either entirely base64 or
// entirely plaintext, never a mix.
func isRawLinkContent(trimmed string) bool {
	firstLine := trimmed
	if idx := strings.IndexByte(trimmed, '\n'); idx != -1 {
		firstLine = trimmed[:idx]
	}
	firstLine = strings.TrimSpace(firstLine)

	for _, prefix := range v2raySchemePrefixes {
		if strings.HasPrefix(firstLine, prefix) {
			return true
		}
	}
	return false
}

// recombineConfig rebuilds packageID's CombinedConfigCached from every
// location's current ConfigLinkCached -- an empty/stale location (one that
// failed this tick, or every prior tick) is simply omitted, never causes
// the whole package's subscription to error (see V2RayPackage.
// CombinedConfigCached's doc comment). A synthetic, non-connectable "info"
// entry is always prepended first (see buildInfoConfigLine) so the
// customer's own V2Ray app's server list shows their remaining volume/
// days/status at a glance, refreshed on every tick alongside the real
// entries below it.
// RecombineConfig is the exported entry point satisfying
// v2rayConfigRecombiner (see V2RayPackageService.SetConfigRecombiner) so
// package creation/location-add can trigger an immediate rebuild instead
// of waiting for this service's own periodic tick.
func (s *V2RaySyncService) RecombineConfig(packageID uint) {
	s.recombineConfig(packageID)
}

func (s *V2RaySyncService) recombineConfig(packageID uint) {
	var pkg model.V2RayPackage
	if err := s.db.First(&pkg, packageID).Error; err != nil {
		s.logger.Error("failed to fetch v2ray package for config recombine", zap.Uint("package_id", packageID), zap.Error(err))
		return
	}

	var locations []model.V2RayPackageLocation
	if err := s.db.Where("package_id = ? AND config_link_cached != ''", packageID).Find(&locations).Error; err != nil {
		s.logger.Error("failed to fetch locations for v2ray config recombine", zap.Uint("package_id", packageID), zap.Error(err))
		return
	}

	var usedBytes int64
	for _, loc := range locations {
		usedBytes += loc.UsedBytesCached
	}

	lines := make([]string, 0, len(locations)+1)
	if infoLine := buildInfoConfigLine(pkg, usedBytes); infoLine != "" {
		lines = append(lines, infoLine)
	}
	for _, loc := range locations {
		// ConfigLinkCached is plaintext (see rewriteSubscriptionTitle's doc
		// comment) -- no decode step here, this table's WHERE clause
		// (config_link_cached != '') already filters out anything empty.
		lines = append(lines, strings.TrimSpace(loc.ConfigLinkCached))
	}

	combined := base64.StdEncoding.EncodeToString([]byte(strings.Join(lines, "\n")))

	if err := s.db.Model(&model.V2RayPackage{}).Where("id = ?", packageID).Updates(map[string]interface{}{
		"combined_config_cached":    combined,
		"combined_config_synced_at": timeNow(),
	}).Error; err != nil {
		s.logger.Error("failed to persist v2ray combined config", zap.Uint("package_id", packageID), zap.Error(err))
	}
}

// enforcePackageQuota sums packageID's locations' UsedBytesCached and, on
// exceeding TotalVolumeBytes, disables every location's client on its
// panel plus updates the owning reseller's V2Ray quota pool -- mirrors
// applyUserManagerResellerQuota's reseller-pool-exhaustion structure
// (cmd/jobs/traffic.go), counting packages instead of accounts.
func (s *V2RaySyncService) enforcePackageQuota(ctx context.Context, packageID uint) {
	var pkg model.V2RayPackage
	if err := s.db.First(&pkg, packageID).Error; err != nil {
		s.logger.Error("failed to fetch v2ray package for quota enforcement", zap.Uint("package_id", packageID), zap.Error(err))
		return
	}

	var usedBytes int64
	if err := s.db.Model(&model.V2RayPackageLocation{}).
		Where("package_id = ?", packageID).
		Select("COALESCE(SUM(used_bytes_cached), 0)").
		Scan(&usedBytes).Error; err != nil {
		s.logger.Error("failed to sum v2ray package usage", zap.Uint("package_id", packageID), zap.Error(err))
		return
	}

	if pkg.ResellerID != nil {
		s.applyResellerV2RayQuota(*pkg.ResellerID, usedBytes)
	}

	// Quota enforcement uses the OFFSET-ADJUSTED usage (see
	// model.V2RayPackage.UsageOffsetBytes' own doc comment) -- the reseller
	// pool call just above intentionally used the raw usedBytes instead, so
	// a "Reset Usage" action never shrinks the reseller's own quota total.
	displayedUsedBytes := usedBytes - pkg.UsageOffsetBytes
	if displayedUsedBytes < 0 {
		displayedUsedBytes = 0
	}

	// Confirmed, reported production incident this fixes (reseller "Sha",
	// Payment-based/payment-mode, complained their V2Ray configs kept
	// auto-disabling themselves): this TotalVolumeBytes check is the
	// per-PACKAGE sibling of applyResellerV2RayQuota's own reseller-wide
	// BillingMode guard just above -- a Payment-based reseller's package
	// keeps its old/leftover TotalVolumeBytes set (never cleared, same
	// rationale as V2RayQuotaBytes there) purely so switching back to
	// Volume-based needs no migration, but that leftover value must never
	// itself trigger suspension for a Payment-based reseller; only
	// ResellerBillingService.ChargeUsage's own billing-rejection path
	// (surfaced below via BillingSuspended) may disable a Payment
	// reseller's packages. Missing this guard here (while
	// applyResellerV2RayQuota already had it) is exactly why Sha's
	// packages kept re-suspending: their real per-package usage
	// legitimately exceeded the old TotalVolumeBytes figure left over from
	// before they were switched to Payment billing.
	var reseller model.Reseller
	var hasReseller bool
	if pkg.ResellerID != nil {
		if err := s.db.Select("billing_mode", "billing_suspended").Where("id = ?", *pkg.ResellerID).First(&reseller).Error; err == nil {
			hasReseller = true
		}
	}

	var overQuota bool
	if hasReseller && reseller.BillingMode == model.ResellerBillingModePayment {
		// Payment-based billing: a reseller whose wallet debit was rejected
		// (see ChargeUsage in syncOneLocation above) is disabled here
		// exactly like a Volume-based reseller going over TotalVolumeBytes
		// -- reusing the identical overQuota branch below rather than a
		// separate disable path, since "disable this package's locations"
		// is the same action regardless of which billing mode triggered it.
		overQuota = reseller.BillingSuspended
	} else {
		overQuota = pkg.TotalVolumeBytes > 0 && displayedUsedBytes >= pkg.TotalVolumeBytes
	}

	// A confirmed, reported bug: once a package went over quota and got
	// disabled below, nothing ever reversed it -- not even after the admin
	// raised TotalVolumeBytes back above usedBytes (e.g. granting more
	// volume) and/or set Status back to "active" via UpdatePackage. This
	// branch is the counterpart to the disable branch below: usage now
	// under the (possibly just-raised) limit, and this package really was
	// auto-suspended by quota (not manually suspended/expired by an admin,
	// tracked via WasActiveBeforeSuspend exactly like the reseller-level
	// resumeQuotaSuspendedPeers/resumeQuotaSuspendedUserManagerAccounts in
	// reseller.go) -- so it's safe to re-enable on this package's own next
	// sync tick, not just when a reseller-wide quota top-up runs.
	if !overQuota && pkg.SuspendedByQuota && pkg.WasActiveBeforeSuspend {
		s.reenablePackageLocations(ctx, pkg)
		return
	}

	if !overQuota || pkg.SuspendedByQuota {
		return
	}

	s.logger.Warn("v2ray package traffic limit exceeded", zap.Uint("package_id", packageID), zap.Int64("used", displayedUsedBytes), zap.Int64("limit", pkg.TotalVolumeBytes))

	var locations []model.V2RayPackageLocation
	if err := s.db.Where("package_id = ?", packageID).Find(&locations).Error; err != nil {
		s.logger.Error("failed to fetch locations to disable on quota exceed", zap.Uint("package_id", packageID), zap.Error(err))
		return
	}

	for _, loc := range locations {
		panel, err := s.panels.GetPanel(loc.PanelID)
		if err != nil {
			continue
		}

		// Resolved fresh, same as CreatePackage's own AddClient call --
		// UpdateClient replaces the WHOLE client object on x-ui, so an
		// empty/wrong flow here would silently break a currently-working
		// TLS/Reality client the next time it's re-enabled, even though
		// this specific call's only real intent is to flip Enable=false.
		flow, flowErr := xui.ResolveClientFlow(ctx, panel, panel.DefaultInboundID)
		if flowErr != nil {
			s.logger.Warn("failed to resolve inbound security settings before disabling client, defaulting to no flow",
				zap.Uint("panel_id", panel.ID), zap.Error(flowErr))
		}

		if err := xui.UpdateClient(ctx, panel, loc.ClientUUID, xui.XuiClient{
			ID:      loc.ClientUUID,
			Flow:    flow,
			Email:   loc.ClientEmail,
			SubID:   loc.SubID,
			TotalGB: pkg.TotalVolumeBytes,
			Enable:  false,
		}); err != nil {
			s.logger.Error("failed to disable v2ray client on quota exceed", zap.Uint("location_id", loc.ID), zap.Error(err))
			continue
		}

		if updErr := s.db.Model(&model.V2RayPackageLocation{}).Where("id = ?", loc.ID).Update("enabled", false).Error; updErr != nil {
			s.logger.Error("failed to persist v2ray location disabled state", zap.Uint("location_id", loc.ID), zap.Error(updErr))
		}
	}

	s.db.Model(&model.V2RayPackage{}).Where("id = ?", packageID).Updates(map[string]interface{}{
		"status":                    "suspended",
		"suspended_by_quota":        true,
		"was_active_before_suspend": pkg.Status == "active",
	})
}

// ResumePackagesForReseller re-enables every one of resellerID's V2Ray
// packages that is currently suspended-by-quota and was active
// beforehand -- the V2Ray-specific piece of Reseller.ResumeBillingSuspension
// above (injected into it as a narrow interface to avoid a constructor
// cycle), called once a Payment-based reseller's wallet is solvent again.
// Mirrors resumeQuotaSuspendedPeers/resumeQuotaSuspendedUserManagerAccounts'
// own "SuspendedByQuota AND WasActiveBeforeSuspend" targeting exactly,
// just reusing this file's own existing per-package reenablePackageLocations
// rather than duplicating its x-ui client-update logic.
func (s *V2RaySyncService) ResumePackagesForReseller(resellerID uint) {
	var packages []model.V2RayPackage
	if err := s.db.Where(
		"reseller_id = ? AND suspended_by_quota = ? AND was_active_before_suspend = ?",
		resellerID, true, true,
	).Find(&packages).Error; err != nil {
		s.logger.Error("failed to fetch billing-suspended v2ray packages", zap.Uint("reseller_id", resellerID), zap.Error(err))
		return
	}

	ctx := context.Background()
	for _, pkg := range packages {
		s.reenablePackageLocations(ctx, pkg)
	}
}

// reenablePackageLocations pushes Enable=true (and the current
// TotalVolumeBytes) to every one of pkg's x-ui clients and clears the
// quota-suspend tracking flags plus restores Status to "active" -- the
// per-package mirror of resumeQuotaSuspendedPeers/
// resumeQuotaSuspendedUserManagerAccounts in reseller.go, run automatically
// by enforcePackageQuota rather than only on a reseller-wide top-up event.
func (s *V2RaySyncService) reenablePackageLocations(ctx context.Context, pkg model.V2RayPackage) {
	var locations []model.V2RayPackageLocation
	if err := s.db.Where("package_id = ?", pkg.ID).Find(&locations).Error; err != nil {
		s.logger.Error("failed to fetch locations to re-enable after quota resume", zap.Uint("package_id", pkg.ID), zap.Error(err))
		return
	}

	for _, loc := range locations {
		panel, err := s.panels.GetPanel(loc.PanelID)
		if err != nil {
			continue
		}

		flow, flowErr := xui.ResolveClientFlow(ctx, panel, panel.DefaultInboundID)
		if flowErr != nil {
			s.logger.Warn("failed to resolve inbound security settings before re-enabling client, defaulting to no flow",
				zap.Uint("panel_id", panel.ID), zap.Error(flowErr))
		}

		client := xui.XuiClient{
			ID:      loc.ClientUUID,
			Flow:    flow,
			Email:   loc.ClientEmail,
			SubID:   loc.SubID,
			TotalGB: pkg.TotalVolumeBytes,
			Enable:  true,
		}

		updateErr := xui.UpdateClient(ctx, panel, loc.ClientUUID, client)
		if updateErr != nil {
			// UpdateClient can fail if the client no longer exists in the
			// inbound's current settings (e.g. removed directly on the
			// x-ui panel, or the inbound was recreated independently).
			// Fall back to AddClient, recreating the client with the same
			// UUID/email/subID this row already has, so the customer's
			// existing config link/subscription URL keeps working
			// unchanged. A panel that's merely slow/unreachable will fail
			// AddClient too (leaving this location disabled for the next
			// retry, same as before) -- this only helps the
			// genuinely-missing-client case, it doesn't mask real
			// connectivity errors.
			if addErr := xui.AddClient(ctx, panel, client); addErr != nil {
				s.logger.Error("failed to re-enable v2ray client after quota resume (update and recreate both failed)",
					zap.Uint("location_id", loc.ID), zap.Error(updateErr), zap.NamedError("add_error", addErr))
				continue
			}
			s.logger.Warn("v2ray client was missing on panel during quota resume, recreated it with the same identity",
				zap.Uint("location_id", loc.ID), zap.Error(updateErr))
		}

		if updErr := s.db.Model(&model.V2RayPackageLocation{}).Where("id = ?", loc.ID).Update("enabled", true).Error; updErr != nil {
			s.logger.Error("failed to persist v2ray location re-enabled state", zap.Uint("location_id", loc.ID), zap.Error(updErr))
		}
	}

	s.db.Model(&model.V2RayPackage{}).Where("id = ?", pkg.ID).Updates(map[string]interface{}{
		"status":                    "active",
		"suspended_by_quota":        false,
		"was_active_before_suspend": false,
	})
}

// applyResellerV2RayQuota mirrors applyUserManagerResellerQuota's structure
// (cmd/jobs/traffic.go), operating on the separate V2RayQuotaBytes/
// V2RayUsedBytes pool. Per-package enforcement (enforcePackageQuota
// above) only checks a single package's own TotalVolumeBytes, so a
// reseller with many small packages could otherwise exceed their overall
// quota pool indefinitely as long as no single package individually
// crosses its own limit.
//
// This enforces the pool exactly like applyUserManagerResellerQuota
// enforces UserManagerQuotaBytes: once totalUsed exceeds V2RayQuotaBytes,
// every one of this reseller's not-already-disabled packages is disabled
// (all its x-ui clients across every location) and marked
// SuspendedByResellerQuota -- a flag deliberately separate from
// SuspendedByQuota (a package's own TotalVolumeBytes limit, a different
// concept -- see V2RayPackage.SuspendedByResellerQuota's own doc comment).
func (s *V2RaySyncService) applyResellerV2RayQuota(resellerID uint, packageUsedBytes int64) {
	// packagesToSuspend is populated INSIDE the transaction below (a
	// pure DB read/write, no network I/O) and acted on AFTER it commits
	// -- a confirmed bug caught while writing this function's own test:
	// the original version called disablePackageLocations (real x-ui
	// HTTP requests) from INSIDE this same transaction, while
	// tx.Clauses(clause.Locking{Strength: "UPDATE"}) was still holding a
	// lock on the reseller row. On SQLite (this codebase's own database),
	// that real network call can itself trigger further DB access on a
	// SEPARATE connection while the first transaction is still open and
	// uncommitted, deadlocking the whole call for the busy_timeout
	// duration (or longer, if the network call itself never returns) --
	// exactly the kind of "one slow external call blocks everything else"
	// bug this session already root-caused and fixed once before for the
	// panel-hang incidents (see dataservice/db.go's own SetMaxOpenConns
	// doc comment). Mirrors this file's OWN reenablePackageLocations/
	// ResumePackagesForResellerQuota below, which likewise never wrap
	// their x-ui calls in a DB transaction.
	var packagesToSuspend []model.V2RayPackage

	err := s.db.Transaction(func(tx *gorm.DB) error {
		var reseller model.Reseller
		if err := tx.Clauses(clause.Locking{Strength: "UPDATE"}).Where("id = ?", resellerID).First(&reseller).Error; err != nil {
			s.logger.Error("failed to fetch reseller for v2ray quota", zap.Uint("reseller_id", resellerID), zap.Error(err))
			return nil
		}

		var liveUsed int64
		if err := tx.Model(&model.V2RayPackageLocation{}).
			Joins("JOIN v2_ray_packages ON v2_ray_packages.id = v2_ray_package_locations.package_id").
			Where("v2_ray_packages.reseller_id = ?", resellerID).
			Select("COALESCE(SUM(v2_ray_package_locations.used_bytes_cached), 0)").
			Scan(&liveUsed).Error; err != nil {
			s.logger.Error("failed to sum reseller v2ray usage", zap.Uint("reseller_id", resellerID), zap.Error(err))
			return nil
		}
		// + V2RayDeletedUsageBytes: a confirmed, reported bug's fix -- see
		// that field's own doc comment. Without this, deleting a package
		// would make this STORED total (reseller.V2RayUsedBytes, read by
		// Payment-mode billing's tiered pricing via cumulativeUsedBytes, and
		// by every usage report) silently shrink, letting a reseller dodge
		// a higher pricing tier or hide usage history by deleting packages.
		totalUsed := liveUsed + reseller.V2RayDeletedUsageBytes

		if totalUsed != reseller.V2RayUsedBytes {
			if err := tx.Model(&model.Reseller{}).Where("id = ?", resellerID).Update("v2_ray_used_bytes", totalUsed).Error; err != nil {
				s.logger.Error("failed to update reseller v2ray usage", zap.Uint("reseller_id", resellerID), zap.Error(err))
				return err
			}
		}

		// Same billing-mode guard as applyResellerQuota/
		// applyUserManagerResellerQuota's own identical fix (cmd/jobs/
		// traffic.go) -- a Payment-based reseller's leftover
		// V2RayQuotaBytes (kept, not cleared, on purpose so switching back
		// to Volume-based needs no migration) must never itself trigger
		// package suspension; only ResellerBillingService.ChargeUsage's
		// own billing-rejection path may disable a Payment reseller's
		// packages.
		if reseller.BillingMode == model.ResellerBillingModePayment {
			return nil
		}
		// Confirmed, reported production incident this fixes (reseller
		// "Mohammadreza", VOLUME-mode, whose packages kept staying
		// suspended -- deleting an old/expired package was even tried as a
		// workaround and made it WORSE): this quota-EXCEEDED check must
		// compare against liveUsed (the sum over currently-existing
		// packages only), never totalUsed (which permanently includes
		// V2RayDeletedUsageBytes -- see just above). V2RayDeletedUsageBytes
		// exists so a reseller can't dodge Payment-mode tiered pricing or
		// erase usage history by deleting a package, which is exactly why
		// it belongs in the STORED total; but a Volume-mode reseller's
		// quota is a live capacity limit on resources that currently
		// exist, not a lifetime billing ledger -- a deleted package's
		// historical usage should free up its quota headroom the same way
		// deleting a WireGuard peer or User Manager account already does
		// for their own quota pools (neither has an equivalent "deleted
		// usage" credit at all). Using totalUsed here meant a reseller's
		// quota could never actually be freed by deleting packages, and
		// worse, deleting one made an ALREADY-over-quota reseller's
		// situation permanently unrecoverable without an admin manually
		// raising V2RayQuotaBytes -- confirmed live: deleting a single
		// package added its entire historical usage into
		// V2RayDeletedUsageBytes, which stayed in totalUsed forever even
		// though liveUsed (and therefore the reseller's REAL current
		// footprint) dropped by the exact same amount.
		if reseller.V2RayQuotaBytes == nil || liveUsed <= *reseller.V2RayQuotaBytes {
			return nil
		}

		s.logger.Warn("reseller v2ray quota exceeded, disabling all packages",
			zap.Uint("reseller_id", resellerID), zap.Int64("used", liveUsed), zap.Int64("quota", *reseller.V2RayQuotaBytes))

		if err := tx.Where("reseller_id = ? AND status != ?", resellerID, "suspended").Find(&packagesToSuspend).Error; err != nil {
			s.logger.Error("failed to fetch reseller v2ray packages for quota suspension", zap.Uint("reseller_id", resellerID), zap.Error(err))
			packagesToSuspend = nil
			return nil
		}

		for _, pkg := range packagesToSuspend {
			if err := tx.Model(&model.V2RayPackage{}).Where("id = ?", pkg.ID).Updates(map[string]interface{}{
				"status":                      "suspended",
				"suspended_by_reseller_quota": true,
				"was_active_before_suspend":   pkg.Status == "active",
			}).Error; err != nil {
				s.logger.Error("failed to mark v2ray package suspended by reseller quota", zap.Uint("package_id", pkg.ID), zap.Error(err))
			}
		}

		return nil
	})
	if err != nil {
		return
	}

	// Real x-ui network calls, now safely outside any open DB transaction.
	ctx := context.Background()
	for _, pkg := range packagesToSuspend {
		s.disablePackageLocations(ctx, pkg)
	}
}

// ResumePackagesForResellerQuota re-enables every one of resellerID's
// V2Ray packages that was disabled specifically because the RESELLER's
// overall quota was exceeded (SuspendedByResellerQuota, set by
// applyResellerV2RayQuota above) and was actually active beforehand --
// the counterpart to that function, called from UpdateReseller once an
// admin raises V2RayQuotaBytes back above the reseller's current usage.
// Deliberately separate from ResumePackagesForReseller (which targets
// SuspendedByQuota, a package's own TotalVolumeBytes limit, for the
// unrelated Payment-billing-resume case) -- resuming one must never
// resume the other.
func (s *V2RaySyncService) ResumePackagesForResellerQuota(resellerID uint) {
	var packages []model.V2RayPackage
	if err := s.db.Where(
		"reseller_id = ? AND suspended_by_reseller_quota = ? AND was_active_before_suspend = ?",
		resellerID, true, true,
	).Find(&packages).Error; err != nil {
		s.logger.Error("failed to fetch reseller-quota-suspended v2ray packages", zap.Uint("reseller_id", resellerID), zap.Error(err))
		return
	}

	ctx := context.Background()
	for _, pkg := range packages {
		s.reenablePackageLocations(ctx, pkg)
		if err := s.db.Model(&model.V2RayPackage{}).Where("id = ?", pkg.ID).Update("suspended_by_reseller_quota", false).Error; err != nil {
			s.logger.Error("failed to clear suspended_by_reseller_quota flag", zap.Uint("package_id", pkg.ID), zap.Error(err))
		}
	}
}

// RepairDisabledLocationsForPackage re-pushes Enable=true to every one of
// pkg's x-ui locations currently marked Enabled=false in our own DB,
// regardless of the package's own status/suspend flags -- an operational
// escape hatch for a package whose suspend flags were already cleared
// (status="active") while one or more of its locations stayed disabled
// because an earlier x-ui push failed, since
// ResumePackagesForResellerQuota/ResumePackagesForReseller only query by
// the package's suspend flags, which such a package no longer has set.
// Safe to call on an already-healthy package (no-op: nothing has
// Enabled=false to touch).
func (s *V2RaySyncService) RepairDisabledLocationsForPackage(packageID uint) error {
	var pkg model.V2RayPackage
	if err := s.db.First(&pkg, packageID).Error; err != nil {
		return fmt.Errorf("failed to load package %d: %w", packageID, err)
	}

	var locations []model.V2RayPackageLocation
	if err := s.db.Where("package_id = ? AND enabled = ?", packageID, false).Find(&locations).Error; err != nil {
		return fmt.Errorf("failed to fetch disabled locations for package %d: %w", packageID, err)
	}

	ctx := context.Background()
	for _, loc := range locations {
		panel, err := s.panels.GetPanel(loc.PanelID)
		if err != nil {
			s.logger.Error("failed to load panel while repairing disabled location", zap.Uint("location_id", loc.ID), zap.Error(err))
			continue
		}

		flow, flowErr := xui.ResolveClientFlow(ctx, panel, panel.DefaultInboundID)
		if flowErr != nil {
			s.logger.Warn("failed to resolve inbound security settings while repairing disabled location, defaulting to no flow",
				zap.Uint("panel_id", panel.ID), zap.Error(flowErr))
		}

		client := xui.XuiClient{
			ID:      loc.ClientUUID,
			Flow:    flow,
			Email:   loc.ClientEmail,
			SubID:   loc.SubID,
			TotalGB: pkg.TotalVolumeBytes,
			Enable:  true,
		}

		updateErr := xui.UpdateClient(ctx, panel, loc.ClientUUID, client)
		if updateErr != nil {
			if addErr := xui.AddClient(ctx, panel, client); addErr != nil {
				s.logger.Error("failed to repair disabled v2ray location (update and recreate both failed)",
					zap.Uint("location_id", loc.ID), zap.Error(updateErr), zap.NamedError("add_error", addErr))
				continue
			}
			s.logger.Warn("v2ray client was missing on panel during manual repair, recreated it with the same identity",
				zap.Uint("location_id", loc.ID), zap.Error(updateErr))
		}

		if updErr := s.db.Model(&model.V2RayPackageLocation{}).Where("id = ?", loc.ID).Update("enabled", true).Error; updErr != nil {
			s.logger.Error("failed to persist v2ray location repaired state", zap.Uint("location_id", loc.ID), zap.Error(updErr))
		}
	}

	return nil
}

// disablePackageLocations pushes Enable=false to every one of pkg's x-ui
// clients -- the shared disable half of enforcePackageQuota's own inline
// loop, extracted so applyResellerV2RayQuota can reuse it without
// duplicating the x-ui client-update logic. Does NOT touch pkg's own DB
// row (status/suspended flags) -- callers own that, since the two
// callers (enforcePackageQuota, applyResellerV2RayQuota) set different
// suspend flags for different reasons.
func (s *V2RaySyncService) disablePackageLocations(ctx context.Context, pkg model.V2RayPackage) {
	var locations []model.V2RayPackageLocation
	if err := s.db.Where("package_id = ?", pkg.ID).Find(&locations).Error; err != nil {
		s.logger.Error("failed to fetch locations to disable for reseller quota", zap.Uint("package_id", pkg.ID), zap.Error(err))
		return
	}

	for _, loc := range locations {
		panel, err := s.panels.GetPanel(loc.PanelID)
		if err != nil {
			continue
		}

		flow, flowErr := xui.ResolveClientFlow(ctx, panel, panel.DefaultInboundID)
		if flowErr != nil {
			s.logger.Warn("failed to resolve inbound security settings before disabling client, defaulting to no flow",
				zap.Uint("panel_id", panel.ID), zap.Error(flowErr))
		}

		if err := xui.UpdateClient(ctx, panel, loc.ClientUUID, xui.XuiClient{
			ID:      loc.ClientUUID,
			Flow:    flow,
			Email:   loc.ClientEmail,
			SubID:   loc.SubID,
			TotalGB: pkg.TotalVolumeBytes,
			Enable:  false,
		}); err != nil {
			s.logger.Error("failed to disable v2ray client for reseller quota", zap.Uint("location_id", loc.ID), zap.Error(err))
			continue
		}

		if updErr := s.db.Model(&model.V2RayPackageLocation{}).Where("id = ?", loc.ID).Update("enabled", false).Error; updErr != nil {
			s.logger.Error("failed to persist v2ray location disabled state", zap.Uint("location_id", loc.ID), zap.Error(updErr))
		}
	}
}

func (s *V2RaySyncService) recordLocationError(locationID uint, syncErr error) {
	errStr := syncErr.Error()
	if err := s.db.Model(&model.V2RayPackageLocation{}).Where("id = ?", locationID).Updates(map[string]interface{}{
		"last_sync_error": errStr,
		"last_synced_at":  timeNow(),
	}).Error; err != nil {
		s.logger.Warn("failed to record v2ray location sync error", zap.Uint("location_id", locationID), zap.Error(err))
	}
}

// calculateV2RayDelta mirrors cmd/jobs/traffic.go's calculateDelta exactly
// -- duplicated rather than exported/shared since that function is
// unexported in the jobs package and this is a small, self-contained
// piece of arithmetic not worth introducing a cross-package dependency
// for.
func calculateV2RayDelta(prev, current, maxCounter int64) (int64, bool) {
	if current >= prev {
		return current - prev, false
	}
	if maxCounter <= 0 || prev > maxCounter || current > maxCounter {
		return current, true
	}
	if (prev - current) > (maxCounter / 2) {
		return (maxCounter - prev) + current, false
	}
	return current, true
}

// infoConfigUUID is a fixed, meaningless UUID used only to keep
// buildInfoConfigLine's synthetic entry syntactically valid as a vless://
// URI (V2Ray client apps parse the whole line before displaying it in
// their server list, so it must still look like a real link) -- it points
// at 127.0.0.1 on an unused port, so on the rare chance a customer taps it
// directly instead of one of the real entries below it, it simply fails
// to connect rather than doing anything else.
const infoConfigUUID = "00000000-0000-0000-0000-000000000000"

// usagePercentEmoji returns the traffic-light emoji matching the admin's
// own confirmed thresholds for this synthetic status line: 0-69% green,
// 70-90% yellow, 91-100% (and anything past it, e.g. a not-yet-enforced
// overage) red.
func usagePercentEmoji(usedBytes, totalBytes int64) string {
	if totalBytes <= 0 {
		return "🟢"
	}
	percent := float64(usedBytes) / float64(totalBytes) * 100
	switch {
	case percent >= 91:
		return "🔴"
	case percent >= 70:
		return "🟡"
	default:
		return "🟢"
	}
}

// persianPackageStatus translates the package's raw Status column into the
// Persian word a customer sees inside their V2Ray/V2Box app -- the raw
// value ("active"/"suspended"/"expired") is an internal enum, not
// customer-facing text.
func persianPackageStatus(status string) string {
	switch status {
	case "suspended":
		return "معلق"
	case "expired":
		return "منقضی"
	default:
		return "فعال"
	}
}

// buildInfoConfigLine returns a synthetic, non-functional vless:// entry
// whose TITLE (the part after '#', exactly like every real entry's own
// title) is the customer's own current remaining-volume/remaining-days/
// status summary -- requested so a customer opens their V2Ray app and
// sees this at a glance in their server list, without needing to visit
// the share page separately. Regenerated fresh on every sync tick
// alongside the real entries (see recombineConfig), so it's never more
// stale than the rest of the subscription. Returns "" if pkg has no
// usable total volume to report against (defensive; every package created
// through this codebase's own CreatePackage always has one).
//
// Formatted in Persian with a traffic-light emoji reflecting usagePercent
// (0-69% 🟢, 70-90% 🟡, 91-100% 🔴, admin-confirmed thresholds) -- a
// confirmed, reported complaint: the previous plain-English
// "0.4GB | 8 Days | active" text was described as "dry" and gave no
// at-a-glance visual signal of how close to the limit a package was.
func buildInfoConfigLine(pkg model.V2RayPackage, usedBytes int64) string {
	remainingGB := "?"
	if pkg.TotalVolumeBytes > 0 {
		remaining := pkg.TotalVolumeBytes - usedBytes
		if remaining < 0 {
			remaining = 0
		}
		remainingGB = fmt.Sprintf("%.1fGB", float64(remaining)/(1024*1024*1024))
	}

	daysRemaining := "?"
	if pkg.ExpireAt != nil {
		days := int(time.Until(*pkg.ExpireAt).Hours() / 24)
		if days < 0 {
			days = 0
		}
		daysRemaining = fmt.Sprintf("%d روز", days)
	} else {
		daysRemaining = "نامحدود"
	}

	emoji := usagePercentEmoji(usedBytes, pkg.TotalVolumeBytes)
	status := persianPackageStatus(pkg.Status)

	title := fmt.Sprintf("%s %s | %s | %s", emoji, remainingGB, daysRemaining, status)

	return fmt.Sprintf("vless://%s@127.0.0.1:1?type=tcp&security=none#%s", infoConfigUUID, urlEncodeTitle(title))
}

// urlEncodeTitle percent-encodes title for safe placement after a link's
// '#' fragment marker -- config link titles are user-facing display names
// (e.g. a reseller's custom branding) that may contain spaces/non-ASCII
// characters, which must be percent-encoded to keep the link itself valid.
func urlEncodeTitle(title string) string {
	var b strings.Builder
	for _, r := range title {
		if (r >= 'a' && r <= 'z') || (r >= 'A' && r <= 'Z') || (r >= '0' && r <= '9') || r == '-' || r == '_' || r == '.' || r == '~' {
			b.WriteRune(r)
		} else {
			for _, c := range []byte(string(r)) {
				fmt.Fprintf(&b, "%%%02X", c)
			}
		}
	}
	return b.String()
}

func timeNow() *time.Time {
	now := time.Now()
	return &now
}
