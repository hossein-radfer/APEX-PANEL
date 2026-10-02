package service

import (
	"testing"

	"github.com/maahdima/mwp/api/dataservice/model"
)

// TestGetAdminSummary_IgnoresStaleErrorOnDisabledLocation is the core
// regression test for the confirmed reported bug: "داخل وضعیت پنل‌ها
// پنل‌ها متصل هست ولی سه تا پنل با خطا Health: Sync error هست" (panels
// show connected but 3 panels show "Health: Sync error" in the admin
// dashboard). Root cause, confirmed on the live production database: a
// DISABLED location's LastSyncError is set once (from whenever it was
// last actually synced, before being disabled) and can never be
// cleared, since SyncPackageUsage (v2ray_sync.go) only ever polls
// Enabled locations -- the sync loop that would clear the error on a
// subsequent success never runs again for a disabled location. Before
// this fix, GetAdminSummary counted LastSyncError on EVERY location
// regardless of Enabled, so a panel with hundreds of perfectly healthy
// enabled locations still showed "Sync error" forever because of a
// handful of long-disabled locations. This confirms a panel whose only
// error is on a DISABLED location reports healthy.
func TestGetAdminSummary_IgnoresStaleErrorOnDisabledLocation(t *testing.T) {
	svc, db := newTestV2RayPackageService(t)

	panel := model.XuiPanel{
		Name: "panel-with-stale-error", SaleTitle: "Panel", APIBaseURL: "http://127.0.0.1:1",
		Username: "admin", Password: "admin", DefaultInboundID: 1, Protocol: "vless",
		SubBaseURL: "http://127.0.0.1:1/sub", Status: "active",
	}
	if err := db.Create(&panel).Error; err != nil {
		t.Fatalf("failed to create panel: %v", err)
	}

	// Two DIFFERENT packages, both provisioned on the SAME panel -- a
	// package+panel pair is unique per model.V2RayPackageLocation's own
	// index, so two locations on one panel must belong to two different
	// packages (mirroring how a real panel serves many customers' own
	// separate packages).
	disabledPkg := model.V2RayPackage{UUID: "pkg-summary-test-disabled", TotalVolumeBytes: 1024, DurationDays: 30, Status: "suspended"}
	if err := db.Create(&disabledPkg).Error; err != nil {
		t.Fatalf("failed to create disabled-owning package: %v", err)
	}
	healthyPkg := model.V2RayPackage{UUID: "pkg-summary-test-healthy", TotalVolumeBytes: 1024, DurationDays: 30, Status: "active"}
	if err := db.Create(&healthyPkg).Error; err != nil {
		t.Fatalf("failed to create healthy-owning package: %v", err)
	}

	staleErr := "connecting to https://example.com:443: dial tcp: no route to host"
	disabledWithStaleError := model.V2RayPackageLocation{
		PackageID: disabledPkg.ID, PanelID: panel.ID,
		ClientUUID: "client-disabled", ClientEmail: "disabled@test", SubID: "sub-disabled",
		Enabled: false, LastSyncError: &staleErr,
	}
	if err := db.Create(&disabledWithStaleError).Error; err != nil {
		t.Fatalf("failed to create disabled location: %v", err)
	}

	healthyEnabled := model.V2RayPackageLocation{
		PackageID: healthyPkg.ID, PanelID: panel.ID,
		ClientUUID: "client-healthy", ClientEmail: "healthy@test", SubID: "sub-healthy",
		Enabled: true, LastSyncError: nil,
	}
	if err := db.Create(&healthyEnabled).Error; err != nil {
		t.Fatalf("failed to create healthy location: %v", err)
	}

	summary, err := svc.GetAdminSummary()
	if err != nil {
		t.Fatalf("GetAdminSummary failed: %v", err)
	}

	if len(summary.Panels) != 1 {
		t.Fatalf("expected 1 panel in summary, got %d", len(summary.Panels))
	}
	if summary.Panels[0].HasRecentError {
		t.Error("expected panel to report healthy (no recent error) since its only error is on a DISABLED location, got HasRecentError=true")
	}
}

// TestGetAdminSummary_ReportsErrorOnEnabledLocation confirms the
// counterpart: a genuine error on a CURRENTLY-ENABLED location (one
// SyncPackageUsage is actually still polling) must still be reported --
// this fix must not silence real, currently-relevant errors.
func TestGetAdminSummary_ReportsErrorOnEnabledLocation(t *testing.T) {
	svc, db := newTestV2RayPackageService(t)

	panel := model.XuiPanel{
		Name: "panel-with-real-error", SaleTitle: "Panel", APIBaseURL: "http://127.0.0.1:1",
		Username: "admin", Password: "admin", DefaultInboundID: 1, Protocol: "vless",
		SubBaseURL: "http://127.0.0.1:1/sub", Status: "active",
	}
	if err := db.Create(&panel).Error; err != nil {
		t.Fatalf("failed to create panel: %v", err)
	}

	pkg := model.V2RayPackage{UUID: "pkg-summary-test-2", TotalVolumeBytes: 1024, DurationDays: 30, Status: "active"}
	if err := db.Create(&pkg).Error; err != nil {
		t.Fatalf("failed to create package: %v", err)
	}

	realErr := "connecting to https://example.com:443: context deadline exceeded"
	enabledWithRealError := model.V2RayPackageLocation{
		PackageID: pkg.ID, PanelID: panel.ID,
		ClientUUID: "client-broken", ClientEmail: "broken@test", SubID: "sub-broken",
		Enabled: true, LastSyncError: &realErr,
	}
	if err := db.Create(&enabledWithRealError).Error; err != nil {
		t.Fatalf("failed to create broken location: %v", err)
	}

	summary, err := svc.GetAdminSummary()
	if err != nil {
		t.Fatalf("GetAdminSummary failed: %v", err)
	}

	if len(summary.Panels) != 1 {
		t.Fatalf("expected 1 panel in summary, got %d", len(summary.Panels))
	}
	if !summary.Panels[0].HasRecentError {
		t.Error("expected panel to report an error since an ENABLED location currently has one, got HasRecentError=false")
	}
}
