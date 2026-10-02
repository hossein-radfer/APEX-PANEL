package service

import (
	"fmt"
	"testing"
	"time"

	"github.com/glebarez/sqlite"
	"github.com/maahdima/mwp/api/dataservice/model"
	"github.com/maahdima/mwp/api/http/schema"
	"gorm.io/gorm"
)

func newTestReportsService(t *testing.T) (*ReportsService, *gorm.DB) {
	t.Helper()

	dsn := fmt.Sprintf("file:reports_test_%d?mode=memory&cache=shared", time.Now().UnixNano())
	db, err := gorm.Open(sqlite.Open(dsn), &gorm.Config{})
	if err != nil {
		t.Fatalf("failed to open test db: %v", err)
	}

	if err := db.AutoMigrate(
		&model.Reseller{},
		&model.Peer{},
		&model.UserManagerAccount{},
		&model.V2RayPackage{},
		&model.V2RayPackageLocation{},
		&model.XuiPanel{},
		&model.UsageSnapshot{},
		&model.ResourceSample{},
		&model.LedgerEntry{},
		&model.Wallet{},
	); err != nil {
		t.Fatalf("failed to migrate test db: %v", err)
	}

	return NewReportsService(db), db
}

// TestResolveDateRange_Today confirms the "today" bucket starts at UTC
// midnight and ends at "now" -- the smallest of the three ranges.
func TestResolveDateRange_Today(t *testing.T) {
	start, end := resolveDateRange(schema.ReportsRangeToday)
	if end-start > 24*3600 {
		t.Fatalf("expected 'today' range to span at most 24h, got %ds", end-start)
	}
	if start >= end {
		t.Fatalf("expected start < end, got start=%d end=%d", start, end)
	}
}

// TestResolveDateRange_DefaultsTo7Days confirms an unrecognized range
// value falls back to 7 days rather than erroring or producing an
// unbounded range.
func TestResolveDateRange_DefaultsTo7Days(t *testing.T) {
	start, end := resolveDateRange("garbage-value")
	gotDays := (end - start) / 86400
	if gotDays != 7 {
		t.Fatalf("expected unrecognized range to default to 7 days, got %d days", gotDays)
	}
}

// TestResolveDateRange_30Days confirms the 30d range is actually ~30
// days, not accidentally reusing the 7-day default.
func TestResolveDateRange_30Days(t *testing.T) {
	start, end := resolveDateRange(schema.ReportsRange30Days)
	gotDays := (end - start) / 86400
	if gotDays < 29 || gotDays > 30 {
		t.Fatalf("expected '30d' range to span ~30 days, got %d days", gotDays)
	}
}

// TestGetDailyUsage_SplitsByProtocolAndSumsCorrectly is a regression test
// for the core aggregation this entire Reports section depends on:
// snapshots from all three protocols on the same day must be correctly
// bucketed by day AND by protocol, with TotalBytes always equal to the
// sum of all three.
func TestGetDailyUsage_SplitsByProtocolAndSumsCorrectly(t *testing.T) {
	svc, db := newTestReportsService(t)

	now := time.Now().Unix()
	peerID, accountID, packageID := uint(1), uint(2), uint(3)

	snapshots := []model.UsageSnapshot{
		{Timestamp: now, Protocol: model.UsageProtocolWireGuard, PeerID: &peerID, UploadBytes: 100, DownloadBytes: 200, TotalBytes: 300},
		{Timestamp: now, Protocol: model.UsageProtocolUserManager, AccountID: &accountID, UploadBytes: 50, DownloadBytes: 50, TotalBytes: 100},
		{Timestamp: now, Protocol: model.UsageProtocolV2Ray, PackageID: &packageID, DownloadBytes: 500, TotalBytes: 500},
	}
	for _, s := range snapshots {
		if err := db.Create(&s).Error; err != nil {
			t.Fatalf("failed to create snapshot: %v", err)
		}
	}

	resp, err := svc.GetDailyUsage(schema.ReportsRangeToday, "")
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if len(resp.Points) != 1 {
		t.Fatalf("expected 1 day of points, got %d", len(resp.Points))
	}

	point := resp.Points[0]
	if point.WireGuardBytes != 300 {
		t.Fatalf("expected WireGuardBytes=300, got %d", point.WireGuardBytes)
	}
	if point.UserManagerBytes != 100 {
		t.Fatalf("expected UserManagerBytes=100, got %d", point.UserManagerBytes)
	}
	if point.V2RayBytes != 500 {
		t.Fatalf("expected V2RayBytes=500, got %d", point.V2RayBytes)
	}
	if point.TotalBytes != 900 {
		t.Fatalf("expected TotalBytes=900 (sum of all three), got %d", point.TotalBytes)
	}
}

// TestGetDailyUsage_ProtocolFilter confirms passing a specific protocol
// filter excludes the other two protocols' snapshots entirely, not just
// from their own field but from TotalBytes too.
func TestGetDailyUsage_ProtocolFilter(t *testing.T) {
	svc, db := newTestReportsService(t)

	now := time.Now().Unix()
	peerID, packageID := uint(1), uint(2)

	db.Create(&model.UsageSnapshot{Timestamp: now, Protocol: model.UsageProtocolWireGuard, PeerID: &peerID, TotalBytes: 300})
	db.Create(&model.UsageSnapshot{Timestamp: now, Protocol: model.UsageProtocolV2Ray, PackageID: &packageID, TotalBytes: 500})

	resp, err := svc.GetDailyUsage(schema.ReportsRangeToday, model.UsageProtocolWireGuard)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if len(resp.Points) != 1 {
		t.Fatalf("expected 1 day of points, got %d", len(resp.Points))
	}
	if resp.Points[0].TotalBytes != 300 {
		t.Fatalf("expected only the wireguard snapshot's 300 bytes, got %d", resp.Points[0].TotalBytes)
	}
	if resp.Points[0].V2RayBytes != 0 {
		t.Fatalf("expected v2ray bytes to be excluded by the protocol filter, got %d", resp.Points[0].V2RayBytes)
	}
}

// TestGetResellerUsageRanking_ExcludesAdminDirectUsage confirms snapshots
// with a nil ResellerID (admin-direct entities) never appear in the
// reseller ranking -- there is no "reseller" to attribute them to.
func TestGetResellerUsageRanking_ExcludesAdminDirectUsage(t *testing.T) {
	svc, db := newTestReportsService(t)

	reseller := model.Reseller{Name: "Test Reseller", Username: "test-reseller", PasswordHash: "x"}
	if err := db.Create(&reseller).Error; err != nil {
		t.Fatalf("failed to create reseller: %v", err)
	}

	now := time.Now().Unix()
	peerID1, peerID2 := uint(1), uint(2)

	db.Create(&model.UsageSnapshot{Timestamp: now, Protocol: model.UsageProtocolWireGuard, PeerID: &peerID1, ResellerID: &reseller.ID, TotalBytes: 1000})
	db.Create(&model.UsageSnapshot{Timestamp: now, Protocol: model.UsageProtocolWireGuard, PeerID: &peerID2, ResellerID: nil, TotalBytes: 5000})

	resp, err := svc.GetResellerUsageRanking(schema.ReportsRangeToday)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if len(resp.Rows) != 1 {
		t.Fatalf("expected exactly 1 reseller row (admin-direct excluded), got %d", len(resp.Rows))
	}
	if resp.Rows[0].Bytes != 1000 {
		t.Fatalf("expected reseller's own 1000 bytes only, got %d", resp.Rows[0].Bytes)
	}
}

// TestGetProtocolShare_SumsAcrossFullRange confirms the pie-chart data
// sums every snapshot per protocol across the whole range, not just the
// most recent one.
func TestGetProtocolShare_SumsAcrossFullRange(t *testing.T) {
	svc, db := newTestReportsService(t)

	now := time.Now().Unix()
	peerID := uint(1)

	db.Create(&model.UsageSnapshot{Timestamp: now, Protocol: model.UsageProtocolWireGuard, PeerID: &peerID, TotalBytes: 300})
	db.Create(&model.UsageSnapshot{Timestamp: now - 60, Protocol: model.UsageProtocolWireGuard, PeerID: &peerID, TotalBytes: 200})

	// "today" (not a fixed offset like -3600s, which could fall before
	// UTC midnight depending on what time of day this test happens to
	// run) -- resolveDateRange's own truncation makes -60s always safe.
	resp, err := svc.GetProtocolShare(schema.ReportsRangeToday)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if resp.WireGuardBytes != 500 {
		t.Fatalf("expected WireGuardBytes=500 (sum of both snapshots), got %d", resp.WireGuardBytes)
	}
}
