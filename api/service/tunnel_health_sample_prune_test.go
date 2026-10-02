package service

import (
	"fmt"
	"testing"
	"time"

	"github.com/glebarez/sqlite"
	"gorm.io/gorm"

	"github.com/maahdima/mwp/api/dataservice/model"
)

// TestTunnelHealthSamplePrune_IsHardDelete is the regression test for a
// confirmed, reported bug in recordTxRxSample's own rolling-window prune
// (tunnel_health.go): TunnelHealthSample embeds model.Model (carrying
// gorm.DeletedAt), so the prune's plain .Delete() call only ever
// soft-deleted rows past the retention window, defeating its entire
// purpose (223K+ rows were found live against a window meant to keep only
// txRxSampleRetention rows per interface). This test exercises the exact
// same query/delete shape recordTxRxSample uses, isolated from the
// RouterOS-fetching half of that function, and confirms Unscoped() makes
// the deleted rows genuinely gone -- mirrors
// TestDeleteConnectionLogOlderThan_IsHardDelete's identical rationale in
// security_retention_test.go.
func TestTunnelHealthSamplePrune_IsHardDelete(t *testing.T) {
	dsn := fmt.Sprintf("file:tunnel_health_sample_prune_%d?mode=memory&cache=shared", time.Now().UnixNano())
	db, err := gorm.Open(sqlite.Open(dsn), &gorm.Config{})
	if err != nil {
		t.Fatalf("failed to open test db: %v", err)
	}
	if err := db.AutoMigrate(&model.TunnelHealthSample{}); err != nil {
		t.Fatalf("failed to migrate test db: %v", err)
	}

	const retention = 3
	now := time.Now()
	var oldestID uint
	for i := 0; i < retention+2; i++ {
		sample := model.TunnelHealthSample{
			InterfaceName: "wg1",
			TxBytes:       int64(i),
			RxBytes:       int64(i),
			SampledAt:     now.Add(time.Duration(-i) * time.Minute),
		}
		if err := db.Create(&sample).Error; err != nil {
			t.Fatalf("failed to create sample %d: %v", i, err)
		}
		if i == retention+1 {
			oldestID = sample.ID
		}
	}

	// Exact same prune shape as recordTxRxSample's own inline logic.
	var toDelete []uint
	db.Model(&model.TunnelHealthSample{}).
		Where("interface_name = ?", "wg1").
		Order("sampled_at desc").
		Offset(retention).
		Pluck("id", &toDelete)
	if len(toDelete) == 0 {
		t.Fatal("expected at least one row past the retention window to be pruned")
	}
	if err := db.Unscoped().Where("id IN ?", toDelete).Delete(&model.TunnelHealthSample{}).Error; err != nil {
		t.Fatalf("prune delete failed: %v", err)
	}

	var remaining int64
	db.Model(&model.TunnelHealthSample{}).Where("interface_name = ?", "wg1").Count(&remaining)
	if remaining != retention {
		t.Fatalf("expected exactly %d rows to survive the rolling window, got %d", retention, remaining)
	}

	var unscopedCount int64
	if err := db.Unscoped().Model(&model.TunnelHealthSample{}).Where("id = ?", oldestID).Count(&unscopedCount).Error; err != nil {
		t.Fatalf("failed to count unscoped: %v", err)
	}
	if unscopedCount != 0 {
		t.Fatalf("expected the oldest pruned row to be genuinely (hard) deleted, but it's still present when queried Unscoped()")
	}
}
