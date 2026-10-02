package service

import (
	"time"

	"go.uber.org/zap"
	"gorm.io/gorm"

	"github.com/maahdima/mwp/api/dataservice/model"
)

// UsageSnapshotWriter is the single write path every per-protocol traffic/
// sync job calls into at the exact moment it already computes a delta for
// its own quota bookkeeping -- see model.UsageSnapshot's own doc comment
// for why this table exists and why it's populated from three separate
// call sites rather than one central job. A write failure here is
// deliberately non-fatal to the caller (logged, not returned) -- the
// Reports section losing one data point is much less severe than a
// reporting-table write blocking or breaking the actual quota/usage
// enforcement path that every other job's own correctness depends on.
type UsageSnapshotWriter struct {
	db     *gorm.DB
	logger *zap.Logger
}

func NewUsageSnapshotWriter(db *gorm.DB) *UsageSnapshotWriter {
	return &UsageSnapshotWriter{
		db:     db,
		logger: zap.L().Named("UsageSnapshotWriter"),
	}
}

// RecordV2Ray writes one delta for a V2RayPackageLocation -- called from
// V2RaySyncService.syncOneLocation at the same point UsedBytesCached's
// delta is computed. V2Ray's GetClientTraffics only reports a combined
// up+down total (see xui.GetClientTraffics's own doc comment), so upload
// is always recorded as 0 and the full delta goes to download -- this
// mirrors how V2RayPackageLocation.UsedBytesCached itself already has no
// separate upload/download split.
func (w *UsageSnapshotWriter) RecordV2Ray(packageID, panelID uint, resellerID *uint, deltaBytes int64) {
	if deltaBytes <= 0 {
		return
	}
	w.write(model.UsageSnapshot{
		Timestamp:     time.Now().Unix(),
		Protocol:      model.UsageProtocolV2Ray,
		PackageID:     &packageID,
		PanelID:       &panelID,
		ResellerID:    resellerID,
		DownloadBytes: deltaBytes,
		TotalBytes:    deltaBytes,
	})
}

// RecordWireGuard writes one delta for a Peer -- called from
// cmd/jobs.Calculator.accumulatePeerDailyUsage at the same point
// PeerDailyUsage's own delta is accumulated.
func (w *UsageSnapshotWriter) RecordWireGuard(peerID uint, resellerID *uint, deltaUpload, deltaDownload int64) {
	total := deltaUpload + deltaDownload
	if total <= 0 {
		return
	}
	w.write(model.UsageSnapshot{
		Timestamp:     time.Now().Unix(),
		Protocol:      model.UsageProtocolWireGuard,
		PeerID:        &peerID,
		ResellerID:    resellerID,
		UploadBytes:   deltaUpload,
		DownloadBytes: deltaDownload,
		TotalBytes:    total,
	})
}

// RecordUserManager writes one delta for a UserManagerAccount -- called
// from cmd/jobs.Calculator.processUserManagerAccountUsage at the same
// point the reseller-level quota pool's delta is computed.
func (w *UsageSnapshotWriter) RecordUserManager(accountID uint, resellerID *uint, deltaUpload, deltaDownload int64) {
	total := deltaUpload + deltaDownload
	if total <= 0 {
		return
	}
	w.write(model.UsageSnapshot{
		Timestamp:     time.Now().Unix(),
		Protocol:      model.UsageProtocolUserManager,
		AccountID:     &accountID,
		ResellerID:    resellerID,
		UploadBytes:   deltaUpload,
		DownloadBytes: deltaDownload,
		TotalBytes:    total,
	})
}

func (w *UsageSnapshotWriter) write(snapshot model.UsageSnapshot) {
	if err := w.db.Create(&snapshot).Error; err != nil {
		w.logger.Warn("failed to record usage snapshot", zap.String("protocol", snapshot.Protocol), zap.Error(err))
	}
}
