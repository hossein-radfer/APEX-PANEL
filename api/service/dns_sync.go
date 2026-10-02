package service

import (
	"context"
	"time"

	"go.uber.org/zap"
	"gorm.io/gorm"
	"gorm.io/gorm/clause"

	"github.com/maahdima/mwp/api/adaptor/doctordns"
	"github.com/maahdima/mwp/api/dataservice/model"
)

// DNSSyncService is the background job body for every doctor-dns network
// call the periodic tick needs -- mirrors V2RaySyncService's role, scaled
// down for DNS's single-panel-per-account model (no per-location fan-out,
// so no concurrent goroutines are needed here: one account is one HTTP
// call).
type DNSSyncService struct {
	db     *gorm.DB
	panels *DNSPanelService
	logger *zap.Logger
}

func NewDNSSyncService(db *gorm.DB, panels *DNSPanelService) *DNSSyncService {
	return &DNSSyncService{
		db:     db,
		panels: panels,
		logger: zap.L().Named("DNSSyncService"),
	}
}

// SyncAccounts is the scheduled job entrypoint. For every active or
// suspended account (not already "expired" -- an expired account has
// nothing left to sync) it polls doctor-dns's read-only /apex/user-state,
// refreshes the local cache, and enforces quota/expiry. A single account's
// failure (its panel being unreachable, in particular) is caught and
// recorded on that account alone, never stopping the rest of the tick.
func (s *DNSSyncService) SyncAccounts() {
	var accounts []model.DNSAccount
	if err := s.db.Where("status != ?", "expired").Find(&accounts).Error; err != nil {
		s.logger.Error("failed to list DNS accounts for sync", zap.Error(err))
		return
	}

	ctx := context.Background()
	touchedResellerIDs := make(map[uint]struct{})

	for _, acct := range accounts {
		s.syncOneAccount(ctx, acct)
		if acct.ResellerID != nil {
			touchedResellerIDs[*acct.ResellerID] = struct{}{}
		}
	}

	for resellerID := range touchedResellerIDs {
		s.applyResellerDNSQuota(resellerID)
	}
}

func (s *DNSSyncService) syncOneAccount(ctx context.Context, acct model.DNSAccount) {
	panel, err := s.panels.GetPanel(acct.PanelID)
	if err != nil {
		s.logger.Warn("DNS panel no longer exists, skipping sync for this account", zap.Uint("account_id", acct.ID), zap.Uint("panel_id", acct.PanelID), zap.Error(err))
		return
	}

	callCtx, cancel := context.WithTimeout(ctx, 15*time.Second)
	defer cancel()

	remote, err := doctordns.GetUserState(callCtx, panel, acct.ApexRef)
	if err != nil {
		s.logger.Warn("failed to sync DNS account state", zap.Uint("account_id", acct.ID), zap.Error(err))
		errStr := err.Error()
		if updErr := s.db.Model(&model.DNSAccount{}).Where("id = ?", acct.ID).Update("last_sync_error", errStr).Error; updErr != nil {
			s.logger.Error("failed to persist DNS account sync error", zap.Uint("account_id", acct.ID), zap.Error(updErr))
		}
		return
	}

	updates := map[string]interface{}{
		"used_bytes_cached": remote.UsedBytes,
		"remote_status":     remote.Status,
		"last_sync_error":   nil,
	}
	if remote.IP != nil {
		updates["current_ip"] = *remote.IP
	}
	syncedAt := time.Now()
	updates["last_synced_at"] = &syncedAt
	if err := s.db.Model(&model.DNSAccount{}).Where("id = ?", acct.ID).Updates(updates).Error; err != nil {
		s.logger.Error("failed to persist DNS account sync result", zap.Uint("account_id", acct.ID), zap.Error(err))
		return
	}
	acct.UsedBytesCached = remote.UsedBytes

	s.enforceAccountQuota(acct)
}

// enforceAccountQuota compares this account's offset-adjusted usage against
// its own TotalVolumeBytes limit -- mirrors enforcePackageQuota's per-
// package check, scaled to DNS's single UsedBytesCached field (no
// per-location SUM needed).
func (s *DNSSyncService) enforceAccountQuota(acct model.DNSAccount) {
	if acct.TotalVolumeBytes <= 0 {
		return // 0 = unlimited
	}
	displayedUsedBytes := acct.UsedBytesCached - acct.UsageOffsetBytes
	if displayedUsedBytes < 0 {
		displayedUsedBytes = 0
	}

	overQuota := displayedUsedBytes >= acct.TotalVolumeBytes

	if overQuota && acct.Status == "active" {
		s.logger.Info("DNS account exceeded its quota, suspending", zap.Uint("account_id", acct.ID))
		if err := s.db.Model(&model.DNSAccount{}).Where("id = ?", acct.ID).Updates(map[string]interface{}{
			"status":                    "suspended",
			"suspended_by_quota":        true,
			"was_active_before_suspend": true,
		}).Error; err != nil {
			s.logger.Error("failed to mark DNS account suspended by quota", zap.Uint("account_id", acct.ID), zap.Error(err))
			return
		}
		panel, err := s.panels.GetPanel(acct.PanelID)
		if err != nil {
			return
		}
		ctx, cancel := context.WithTimeout(context.Background(), 15*time.Second)
		defer cancel()
		if _, err := doctordns.SetUserStatus(ctx, panel, acct.ApexRef, "suspended"); err != nil {
			s.logger.Warn("failed to push DNS quota suspension to panel", zap.Uint("account_id", acct.ID), zap.Error(err))
		}
		return
	}

	if !overQuota && acct.SuspendedByQuota && acct.WasActiveBeforeSuspend {
		s.logger.Info("DNS account back under quota, resuming", zap.Uint("account_id", acct.ID))
		if err := s.db.Model(&model.DNSAccount{}).Where("id = ?", acct.ID).Updates(map[string]interface{}{
			"status":             "active",
			"suspended_by_quota": false,
		}).Error; err != nil {
			s.logger.Error("failed to resume quota-suspended DNS account", zap.Uint("account_id", acct.ID), zap.Error(err))
			return
		}
		panel, err := s.panels.GetPanel(acct.PanelID)
		if err != nil {
			return
		}
		ctx, cancel := context.WithTimeout(context.Background(), 15*time.Second)
		defer cancel()
		if _, err := doctordns.SetUserStatus(ctx, panel, acct.ApexRef, "active"); err != nil {
			s.logger.Warn("failed to push DNS quota resume to panel", zap.Uint("account_id", acct.ID), zap.Error(err))
		}
	}
}

// applyResellerDNSQuota mirrors applyResellerV2RayQuota's core "recompute
// the live sum, add back deleted-account credit, suspend/resume every
// account under this reseller if the pool itself is over/under" logic,
// simplified: no Payment-mode billing branch (DNS is volume-mode only in
// this integration, see model.Reseller.CanResellDNS's own doc comment) and
// no per-location fan-out (one doctor-dns call per account, not per
// location).
func (s *DNSSyncService) applyResellerDNSQuota(resellerID uint) {
	var accountsToSuspend []model.DNSAccount
	var accountsToResume []model.DNSAccount

	err := s.db.Transaction(func(tx *gorm.DB) error {
		var reseller model.Reseller
		if err := tx.Clauses(clause.Locking{Strength: "UPDATE"}).Where("id = ?", resellerID).First(&reseller).Error; err != nil {
			s.logger.Error("failed to fetch reseller for DNS quota", zap.Uint("reseller_id", resellerID), zap.Error(err))
			return nil
		}

		var liveUsed int64
		if err := tx.Model(&model.DNSAccount{}).Where("reseller_id = ?", resellerID).
			Select("COALESCE(SUM(used_bytes_cached), 0)").Scan(&liveUsed).Error; err != nil {
			s.logger.Error("failed to sum reseller DNS usage", zap.Uint("reseller_id", resellerID), zap.Error(err))
			return nil
		}
		totalUsed := liveUsed + reseller.DNSDeletedUsageBytes

		if totalUsed != reseller.DNSUsedBytes {
			if err := tx.Model(&model.Reseller{}).Where("id = ?", resellerID).Update("dns_used_bytes", totalUsed).Error; err != nil {
				s.logger.Error("failed to update reseller DNS usage", zap.Uint("reseller_id", resellerID), zap.Error(err))
				return err
			}
		}

		if reseller.DNSQuotaBytes == nil {
			return nil // unlimited
		}

		if totalUsed > *reseller.DNSQuotaBytes {
			if err := tx.Where("reseller_id = ? AND status != ?", resellerID, "suspended").
				Find(&accountsToSuspend).Error; err != nil {
				s.logger.Error("failed to list DNS accounts to suspend for reseller quota", zap.Uint("reseller_id", resellerID), zap.Error(err))
				return err
			}
			if len(accountsToSuspend) > 0 {
				ids := make([]uint, len(accountsToSuspend))
				for i, a := range accountsToSuspend {
					ids[i] = a.ID
				}
				if err := tx.Model(&model.DNSAccount{}).Where("id IN ?", ids).Updates(map[string]interface{}{
					"status":                      "suspended",
					"suspended_by_reseller_quota": true,
				}).Error; err != nil {
					return err
				}
			}
		} else {
			if err := tx.Where("reseller_id = ? AND suspended_by_reseller_quota = ?", resellerID, true).
				Find(&accountsToResume).Error; err != nil {
				s.logger.Error("failed to list reseller-quota-suspended DNS accounts to resume", zap.Uint("reseller_id", resellerID), zap.Error(err))
				return err
			}
			if len(accountsToResume) > 0 {
				ids := make([]uint, len(accountsToResume))
				for i, a := range accountsToResume {
					ids[i] = a.ID
				}
				if err := tx.Model(&model.DNSAccount{}).Where("id IN ?", ids).Updates(map[string]interface{}{
					"status":                      "active",
					"suspended_by_reseller_quota": false,
				}).Error; err != nil {
					s.logger.Error("failed to resume reseller-quota-suspended DNS accounts", zap.Uint("reseller_id", resellerID), zap.Error(err))
					return err
				}
			}
		}
		return nil
	})
	if err != nil {
		return
	}

	// Real network calls happen AFTER the transaction commits -- mirrors
	// applyResellerV2RayQuota's own identical ordering and the deadlock it
	// was written to avoid (see that function's doc comment). Both the
	// suspend AND resume paths must push to doctor-dns here -- a confirmed
	// bug in an earlier version of this function only pushed on suspend,
	// leaving a resumed account's own remote status stuck on "suspended"
	// (accessible/quota-wise "active" locally, but doctor-dns itself never
	// told to let it back online) until the next unrelated per-account
	// sync tick happened to overwrite it.
	for _, acct := range accountsToSuspend {
		s.pushRemoteStatus(acct, "suspended")
	}
	for _, acct := range accountsToResume {
		s.pushRemoteStatus(acct, "active")
	}
}

// ResumeAccountsForResellerQuota re-enables every one of resellerID's DNS
// accounts currently suspended for the RESELLER's overall pool quota
// (SuspendedByResellerQuota), immediately -- the on-demand counterpart to
// applyResellerDNSQuota's own resume branch (which only runs on the next
// scheduled SyncAccounts tick, up to several minutes later). Called from
// Reseller.UpdateReseller the moment an admin raises DNSQuotaBytes back
// above the reseller's current usage, mirroring
// V2RaySyncService.ResumePackagesForResellerQuota's identical "give the
// admin an instant result instead of a silent multi-minute wait" role.
// Deliberately does NOT also require WasActiveBeforeSuspend=true (unlike
// V2Ray's resumer) -- the suspend branch in applyResellerDNSQuota above
// never sets that flag for the reseller-quota path (only the account's-
// own-quota path does, in enforceAccountQuota), so requiring it here would
// mean this resume path could never resume anything it itself suspended.
func (s *DNSSyncService) ResumeAccountsForResellerQuota(resellerID uint) {
	var accounts []model.DNSAccount
	if err := s.db.Where(
		"reseller_id = ? AND suspended_by_reseller_quota = ?", resellerID, true,
	).Find(&accounts).Error; err != nil {
		s.logger.Error("failed to fetch reseller-quota-suspended DNS accounts", zap.Uint("reseller_id", resellerID), zap.Error(err))
		return
	}

	ids := make([]uint, len(accounts))
	for i, a := range accounts {
		ids[i] = a.ID
	}
	if len(ids) > 0 {
		if err := s.db.Model(&model.DNSAccount{}).Where("id IN ?", ids).Updates(map[string]interface{}{
			"status":                      "active",
			"suspended_by_reseller_quota": false,
		}).Error; err != nil {
			s.logger.Error("failed to resume reseller-quota-suspended DNS accounts", zap.Uint("reseller_id", resellerID), zap.Error(err))
			return
		}
	}

	for _, acct := range accounts {
		s.pushRemoteStatus(acct, "active")
	}
}

// pushRemoteStatus is the shared best-effort doctor-dns status push used by
// both the suspend and resume branches of applyResellerDNSQuota -- a failed
// push is logged, never fatal (the local DB state, which callers already
// committed, remains the source of truth; the next per-account sync tick
// will reconcile the remote side if this push failed).
func (s *DNSSyncService) pushRemoteStatus(acct model.DNSAccount, status string) {
	panel, err := s.panels.GetPanel(acct.PanelID)
	if err != nil {
		return
	}
	ctx, cancel := context.WithTimeout(context.Background(), 15*time.Second)
	defer cancel()
	if _, err := doctordns.SetUserStatus(ctx, panel, acct.ApexRef, status); err != nil {
		s.logger.Warn("failed to push DNS reseller-quota status to panel", zap.Uint("account_id", acct.ID), zap.String("status", status), zap.Error(err))
	}
}
