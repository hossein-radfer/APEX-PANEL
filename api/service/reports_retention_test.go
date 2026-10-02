package service

import (
	"fmt"
	"testing"
	"time"

	"github.com/glebarez/sqlite"
	"gorm.io/gorm"

	"github.com/maahdima/mwp/api/dataservice/model"
)

func openReportsRetentionTestDB(t *testing.T) *gorm.DB {
	t.Helper()

	dsn := fmt.Sprintf("file:reports_retention_%d?mode=memory&cache=shared", time.Now().UnixNano())
	db, err := gorm.Open(sqlite.Open(dsn), &gorm.Config{})
	if err != nil {
		t.Fatalf("failed to open test db: %v", err)
	}

	if err := db.AutoMigrate(&model.UsageSnapshot{}, &model.ResourceSample{}); err != nil {
		t.Fatalf("failed to migrate test db: %v", err)
	}

	return db
}

// TestDeleteUsageSnapshotsOlderThan_DeletesOldKeepsRecent is the core
// regression test for a confirmed, reported gap: UsageSnapshot had NO
// pruning at all (manual or scheduled) before this fix, and was found live
// at 4.85 million rows during a disk-full production incident.
func TestDeleteUsageSnapshotsOlderThan_DeletesOldKeepsRecent(t *testing.T) {
	db := openReportsRetentionTestDB(t)

	old := model.UsageSnapshot{Timestamp: time.Now().AddDate(0, 0, -40).Unix(), Protocol: model.UsageProtocolWireGuard, TotalBytes: 100}
	recent := model.UsageSnapshot{Timestamp: time.Now().AddDate(0, 0, -5).Unix(), Protocol: model.UsageProtocolWireGuard, TotalBytes: 200}
	if err := db.Create(&old).Error; err != nil {
		t.Fatalf("failed to create old snapshot: %v", err)
	}
	if err := db.Create(&recent).Error; err != nil {
		t.Fatalf("failed to create recent snapshot: %v", err)
	}

	svc := NewReportsRetentionService(db)
	deleted, err := svc.DeleteUsageSnapshotsOlderThan(time.Now().AddDate(0, 0, -reportsRetentionDays))
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if deleted != 1 {
		t.Fatalf("expected exactly 1 row deleted, got %d", deleted)
	}

	var remaining model.UsageSnapshot
	if err := db.First(&remaining).Error; err != nil {
		t.Fatalf("expected the recent row to remain: %v", err)
	}
	if remaining.TotalBytes != 200 {
		t.Errorf("expected the surviving row to be the recent one (TotalBytes=200), got %d", remaining.TotalBytes)
	}
}

// TestDeleteUsageSnapshotsOlderThan_IsHardDelete confirms Unscoped() is
// actually applied -- mirrors
// TestDeleteConnectionLogOlderThan_IsHardDelete's identical rationale:
// UsageSnapshot embeds model.Model (carrying gorm.DeletedAt), so a plain
// .Delete() would only soft-delete rows here, which is exactly the class
// of bug already found (and fixed) in TunnelGraphService.pruneOldSnapshots,
// SecurityRetentionService, and TunnelHealthService's own rolling-window
// prune.
func TestDeleteUsageSnapshotsOlderThan_IsHardDelete(t *testing.T) {
	db := openReportsRetentionTestDB(t)

	old := model.UsageSnapshot{Timestamp: time.Now().AddDate(0, 0, -40).Unix(), Protocol: model.UsageProtocolV2Ray, TotalBytes: 50}
	if err := db.Create(&old).Error; err != nil {
		t.Fatalf("failed to create old snapshot: %v", err)
	}

	svc := NewReportsRetentionService(db)
	if _, err := svc.DeleteUsageSnapshotsOlderThan(time.Now().AddDate(0, 0, -reportsRetentionDays)); err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	var unscopedCount int64
	if err := db.Unscoped().Model(&model.UsageSnapshot{}).Where("id = ?", old.ID).Count(&unscopedCount).Error; err != nil {
		t.Fatalf("failed to count unscoped: %v", err)
	}
	if unscopedCount != 0 {
		t.Fatalf("expected the old row to be genuinely (hard) deleted, but it's still present when queried Unscoped()")
	}
}

// TestDeleteResourceSamplesOlderThan_DeletesOldKeepsRecent mirrors the
// UsageSnapshot test above for ResourceSample, the second table found with
// no pruning at all.
func TestDeleteResourceSamplesOlderThan_DeletesOldKeepsRecent(t *testing.T) {
	db := openReportsRetentionTestDB(t)

	old := model.ResourceSample{Timestamp: time.Now().AddDate(0, 0, -40).Unix(), CPULoadPercent: 10}
	recent := model.ResourceSample{Timestamp: time.Now().AddDate(0, 0, -1).Unix(), CPULoadPercent: 20}
	if err := db.Create(&old).Error; err != nil {
		t.Fatalf("failed to create old sample: %v", err)
	}
	if err := db.Create(&recent).Error; err != nil {
		t.Fatalf("failed to create recent sample: %v", err)
	}

	svc := NewReportsRetentionService(db)
	deleted, err := svc.DeleteResourceSamplesOlderThan(time.Now().AddDate(0, 0, -reportsRetentionDays))
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if deleted != 1 {
		t.Fatalf("expected exactly 1 row deleted, got %d", deleted)
	}

	var remaining model.ResourceSample
	if err := db.First(&remaining).Error; err != nil {
		t.Fatalf("expected the recent row to remain: %v", err)
	}
	if remaining.CPULoadPercent != 20 {
		t.Errorf("expected the surviving row to be the recent one (CPULoadPercent=20), got %v", remaining.CPULoadPercent)
	}
}

// TestReportsRunScheduledCleanup_AppliesRetentionToBothTables confirms the
// nightly automatic job applies reportsRetentionDays to BOTH UsageSnapshot
// and ResourceSample in one call, mirroring
// TestRunScheduledCleanup_DeletesOnlyRowsOlderThanDefaultRetention's
// identical shape in security_retention_test.go.
func TestReportsRunScheduledCleanup_AppliesRetentionToBothTables(t *testing.T) {
	db := openReportsRetentionTestDB(t)

	oldSnapshot := model.UsageSnapshot{Timestamp: time.Now().AddDate(0, 0, -40).Unix(), Protocol: model.UsageProtocolWireGuard}
	recentSnapshot := model.UsageSnapshot{Timestamp: time.Now().AddDate(0, 0, -1).Unix(), Protocol: model.UsageProtocolWireGuard}
	oldSample := model.ResourceSample{Timestamp: time.Now().AddDate(0, 0, -40).Unix()}
	recentSample := model.ResourceSample{Timestamp: time.Now().AddDate(0, 0, -1).Unix()}
	for _, row := range []interface{}{&oldSnapshot, &recentSnapshot, &oldSample, &recentSample} {
		if err := db.Create(row).Error; err != nil {
			t.Fatalf("failed to seed row: %v", err)
		}
	}

	svc := NewReportsRetentionService(db)
	svc.RunScheduledCleanup()

	var snapshotCount, sampleCount int64
	db.Model(&model.UsageSnapshot{}).Count(&snapshotCount)
	db.Model(&model.ResourceSample{}).Count(&sampleCount)
	if snapshotCount != 1 {
		t.Errorf("expected exactly 1 usage snapshot to survive, got %d", snapshotCount)
	}
	if sampleCount != 1 {
		t.Errorf("expected exactly 1 resource sample to survive, got %d", sampleCount)
	}
}
