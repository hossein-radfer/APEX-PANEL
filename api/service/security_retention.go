package service

import (
	"time"

	"go.uber.org/zap"
	"gorm.io/gorm"

	"github.com/maahdima/mwp/api/dataservice/model"
)

// defaultRetentionDays is the automatic-cleanup cutoff RunScheduledCleanup
// applies every night -- 14 days, the upper end of this project's own
// planning document recommendation ("۷ یا ۱۴ روز") for ether_traffic
// samples, chosen to keep more troubleshooting history than the minimum
// while still bounding growth (ether_traffic samples are written every 10
// seconds per interface, so even 14 days is a hard ceiling compared to
// the unbounded growth this fixes).
const defaultRetentionDays = 14

// SecurityRetentionService implements the admin's explicit cleanup
// requirements for the Security page's own tables, so they don't grow
// unbounded: delete IPConnectionLog/EtherTrafficSample/IPGeoCache rows
// older than a cutoff, and delete connection history belonging to users
// who no longer exist (expired volume/time, or deleted outright).
//
// Two trigger paths now exist for the age-based cleanup: the admin's own
// on-demand "پاکسازی" buttons on the Security page (any cutoff they
// choose, via RunRetentionCleanup), AND an automatic nightly pass
// (RunScheduledCleanup, wired into cmd/main.go) at defaultRetentionDays --
// added per this project's own planning doc (فاز پنجم-۳: "افزودن Job
// شبانه‌ی خودکار برای retention... نه فقط دکمه‌ی دستی") after
// ether_traffic_samples' 10-second write cadence was confirmed to grow
// unbounded with only a manual button and no default ceiling. The manual
// buttons remain unchanged for an admin who wants a different one-off
// cutoff or the inactive-users criteria (which the nightly job does NOT
// run, since "no longer active" isn't a time-based rule this job can
// safely default without admin judgment).
type SecurityRetentionService struct {
	db     *gorm.DB
	logger *zap.Logger
}

func NewSecurityRetentionService(db *gorm.DB) *SecurityRetentionService {
	return &SecurityRetentionService{db: db, logger: zap.L().Named("SecurityRetentionService")}
}

// RunScheduledCleanup is the automatic nightly counterpart to the admin's
// manual retention buttons -- see this service's own doc comment for why
// both paths exist. Failures are logged and swallowed rather than
// returned, matching LogRetentionService.VacuumJournal's identical
// "unattended job, no caller to report to" convention: one failed tick
// (e.g. a locked table) just means the next scheduled tick gets another
// chance, and this must never be allowed to block/crash the scheduler.
func (s *SecurityRetentionService) RunScheduledCleanup() {
	cutoff := time.Now().AddDate(0, 0, -defaultRetentionDays)

	if _, err := s.DeleteConnectionLogOlderThan(cutoff); err != nil {
		s.logger.Warn("scheduled connection log cleanup failed -- will retry on the next scheduled tick", zap.Error(err))
	}
	if _, err := s.DeleteEtherTrafficOlderThan(cutoff); err != nil {
		s.logger.Warn("scheduled ether traffic cleanup failed -- will retry on the next scheduled tick", zap.Error(err))
	}
}

// DeleteConnectionLogOlderThan removes every IPConnectionLog row whose
// ConnectedAt is before cutoff -- both a currently-open and a closed
// session are eligible; deleting an open session simply means the next
// collector tick opens a fresh one, it doesn't disrupt live tracking.
//
// Unscoped() is required here -- a confirmed, reported bug mirroring
// TunnelGraphService.pruneOldSnapshots' own identical mistake:
// IPConnectionLog/EtherTrafficSample both embed model.Model (carrying
// gorm.DeletedAt), so a plain .Delete() call only ever soft-deleted these
// "retention cleanup" rows, defeating the entire point of this service
// (freeing real disk space) -- every row this service had ever "deleted"
// was actually still present in the database file.
func (s *SecurityRetentionService) DeleteConnectionLogOlderThan(cutoff time.Time) (int64, error) {
	result := s.db.Unscoped().Where("connected_at < ?", cutoff).Delete(&model.IPConnectionLog{})
	if result.Error != nil {
		s.logger.Error("failed to delete old ip connection log rows", zap.Error(result.Error))
	}
	return result.RowsAffected, result.Error
}

// DeleteEtherTrafficOlderThan removes every EtherTrafficSample row sampled
// before cutoff. See DeleteConnectionLogOlderThan's own doc comment for why
// Unscoped() is required.
func (s *SecurityRetentionService) DeleteEtherTrafficOlderThan(cutoff time.Time) (int64, error) {
	result := s.db.Unscoped().Where("sampled_at < ?", cutoff).Delete(&model.EtherTrafficSample{})
	if result.Error != nil {
		s.logger.Error("failed to delete old ether traffic samples", zap.Error(result.Error))
	}
	return result.RowsAffected, result.Error
}

// DeleteConnectionLogForInactiveUsers removes every IPConnectionLog row
// whose owning WireGuard peer or User Manager account either no longer
// exists (deleted outright) or is now expired/volume-exhausted --
// implements the admin's "یوزر هایی که حجم‌شان تموم شده / تاریخ‌شان تموم
// شده / حذف شده" cleanup criteria in one pass. A row with neither PeerID
// nor AccountID set (shouldn't normally happen, see IPConnectionLog's own
// doc comment) is left untouched rather than guessed at.
func (s *SecurityRetentionService) DeleteConnectionLogForInactiveUsers() (int64, error) {
	var totalDeleted int64

	// WireGuard: every currently-live, still-enabled peer's ID -- any
	// IPConnectionLog row whose PeerID is NOT in this set belongs to a
	// peer that's either been deleted outright or disabled (this
	// codebase's own "expired/exhausted" signal, see Peer.Disabled's use
	// throughout the traffic job for quota/expiry enforcement), so it
	// qualifies for cleanup either way.
	var activePeerIDs []uint
	if err := s.db.Model(&model.Peer{}).Where("disabled = ?", false).Pluck("id", &activePeerIDs).Error; err != nil {
		s.logger.Error("failed to list active peers for retention cleanup", zap.Error(err))
		return totalDeleted, err
	}
	wgQuery := s.db.Unscoped().Where("protocol = ?", model.UsageProtocolWireGuard)
	if len(activePeerIDs) > 0 {
		wgQuery = wgQuery.Where("peer_id NOT IN ?", activePeerIDs)
	}
	result := wgQuery.Delete(&model.IPConnectionLog{})
	if result.Error != nil {
		s.logger.Error("failed to delete ip connection log for inactive wireguard peers", zap.Error(result.Error))
		return totalDeleted, result.Error
	}
	totalDeleted += result.RowsAffected

	// User Manager: same "not currently active" rule.
	var activeAccountIDs []uint
	if err := s.db.Model(&model.UserManagerAccount{}).Where("disabled = ?", false).Pluck("id", &activeAccountIDs).Error; err != nil {
		s.logger.Error("failed to list active user manager accounts for retention cleanup", zap.Error(err))
		return totalDeleted, err
	}
	umQuery := s.db.Unscoped().Where("protocol = ?", model.UsageProtocolUserManager)
	if len(activeAccountIDs) > 0 {
		umQuery = umQuery.Where("account_id NOT IN ?", activeAccountIDs)
	}
	result = umQuery.Delete(&model.IPConnectionLog{})
	if result.Error != nil {
		s.logger.Error("failed to delete ip connection log for inactive user manager accounts", zap.Error(result.Error))
		return totalDeleted, result.Error
	}
	totalDeleted += result.RowsAffected

	return totalDeleted, nil
}
