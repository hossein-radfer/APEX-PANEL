package service

import (
	"time"

	"go.uber.org/zap"
	"gorm.io/gorm"

	"github.com/maahdima/mwp/api/dataservice/model"
)

// reportsRetentionDays bounds how long UsageSnapshot/ResourceSample history
// is kept -- 30 days, matching resolveDateRange's own longest report window
// (reports.go) and quotaPredictionLookbackDays' 7-day window; nothing in
// the Reports section ever queries further back than 30 days, so any row
// older than that is pure dead weight.
const reportsRetentionDays = 30

// ReportsRetentionService implements automatic nightly cleanup for the
// Reports section's own two unbounded history tables -- a confirmed,
// reported gap: unlike SecurityRetentionService's identical role for
// IPConnectionLog/EtherTrafficSample, UsageSnapshot/ResourceSample had NO
// pruning of any kind (manual or scheduled), so both grew forever. Found
// live at 4.85 million rows (UsageSnapshot) and 36K+ rows (ResourceSample)
// during an incident where the production server's disk filled to 100% and
// crashed MySQL -- UsageSnapshot alone was the single largest table in the
// database by a wide margin.
type ReportsRetentionService struct {
	db     *gorm.DB
	logger *zap.Logger
}

func NewReportsRetentionService(db *gorm.DB) *ReportsRetentionService {
	return &ReportsRetentionService{db: db, logger: zap.L().Named("ReportsRetentionService")}
}

// RunScheduledCleanup is the automatic nightly job -- see this service's
// own doc comment for why it exists. Failures are logged and swallowed
// rather than returned, matching SecurityRetentionService.RunScheduledCleanup's
// identical "unattended job, no caller to report to" convention.
func (s *ReportsRetentionService) RunScheduledCleanup() {
	cutoff := time.Now().AddDate(0, 0, -reportsRetentionDays)

	if _, err := s.DeleteUsageSnapshotsOlderThan(cutoff); err != nil {
		s.logger.Warn("scheduled usage snapshot cleanup failed -- will retry on the next scheduled tick", zap.Error(err))
	}
	if _, err := s.DeleteResourceSamplesOlderThan(cutoff); err != nil {
		s.logger.Warn("scheduled resource sample cleanup failed -- will retry on the next scheduled tick", zap.Error(err))
	}
}

// DeleteUsageSnapshotsOlderThan removes every UsageSnapshot row whose
// Timestamp (a Unix epoch, not Model.CreatedAt) is before cutoff.
//
// Unscoped() is required here -- UsageSnapshot embeds model.Model (carrying
// gorm.DeletedAt), so a plain .Delete() call would only soft-delete rows,
// repeating the exact same mistake already found and fixed in
// TunnelGraphService.pruneOldSnapshots/SecurityRetentionService/
// TunnelHealthService's own rolling-window prune.
func (s *ReportsRetentionService) DeleteUsageSnapshotsOlderThan(cutoff time.Time) (int64, error) {
	result := s.db.Unscoped().Where("timestamp < ?", cutoff.Unix()).Delete(&model.UsageSnapshot{})
	if result.Error != nil {
		s.logger.Error("failed to delete old usage snapshot rows", zap.Error(result.Error))
	}
	return result.RowsAffected, result.Error
}

// DeleteResourceSamplesOlderThan removes every ResourceSample row whose
// Timestamp is before cutoff. See DeleteUsageSnapshotsOlderThan's own doc
// comment for why Unscoped() is required.
func (s *ReportsRetentionService) DeleteResourceSamplesOlderThan(cutoff time.Time) (int64, error) {
	result := s.db.Unscoped().Where("timestamp < ?", cutoff.Unix()).Delete(&model.ResourceSample{})
	if result.Error != nil {
		s.logger.Error("failed to delete old resource sample rows", zap.Error(result.Error))
	}
	return result.RowsAffected, result.Error
}
