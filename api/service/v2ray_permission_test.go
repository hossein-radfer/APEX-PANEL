package service

import (
	"encoding/json"
	"fmt"
	"net/http"
	"net/http/httptest"
	"testing"
	"time"

	"github.com/glebarez/sqlite"
	"github.com/maahdima/mwp/api/dataservice/model"
	"github.com/maahdima/mwp/api/http/schema"
	"gorm.io/gorm"
)

// newTestV2RayPackageService gives each call its own uniquely-named
// in-memory sqlite DB (mirrors reseller_test.go's openTestDB pattern)
// rather than the bare "file::memory:?cache=shared" DSN some older test
// files in this package use -- that bare DSN is actually a SINGLE
// process-wide shared database across every test in the package (sqlite's
// cache=shared mode keys purely on the DSN string, and an empty path means
// every caller connects to the exact same database), which silently makes
// tests order-dependent/collision-prone unless every test carefully uses
// disjoint IDs. A unique name per call sidesteps that class of flakiness
// entirely.
func newTestV2RayPackageService(t *testing.T) (*V2RayPackageService, *gorm.DB) {
	t.Helper()

	dsn := fmt.Sprintf("file:v2ray_package_test_%d?mode=memory&cache=shared", time.Now().UnixNano())
	db, err := gorm.Open(sqlite.Open(dsn), &gorm.Config{})
	if err != nil {
		t.Fatalf("failed to open test db: %v", err)
	}

	if err := db.AutoMigrate(
		&model.Reseller{},
		&model.XuiPanel{},
		&model.ResellerV2RaySaleTitle{},
		&model.ResellerXuiPanelAccess{},
		&model.V2RayPackage{},
		&model.V2RayPackageLocation{},
	); err != nil {
		t.Fatalf("failed to migrate test db: %v", err)
	}

	panels := NewXuiPanelService(db)
	return NewV2RayPackageService(db, panels, nil), db
}

// TestEnsureResellerCanResellV2Ray verifies the permission gate is a plain
// boolean check, mirroring
// TestEnsureResellerCanCreateUserManagerAccounts exactly.
func TestEnsureResellerCanResellV2Ray(t *testing.T) {
	svc, db := newTestV2RayPackageService(t)

	notAllowed := model.Reseller{Name: "no-v2ray", Username: "no-v2ray", PasswordHash: "x", CanResellV2Ray: false}
	if err := db.Create(&notAllowed).Error; err != nil {
		t.Fatalf("failed to create reseller: %v", err)
	}

	allowed := model.Reseller{Name: "yes-v2ray", Username: "yes-v2ray", PasswordHash: "x", CanResellV2Ray: true}
	if err := db.Create(&allowed).Error; err != nil {
		t.Fatalf("failed to create reseller: %v", err)
	}

	if err := svc.ensureResellerCanResellV2Ray(notAllowed.ID); err == nil {
		t.Fatal("expected error for reseller without v2ray permission")
	}

	if err := svc.ensureResellerCanResellV2Ray(allowed.ID); err != nil {
		t.Fatalf("expected no error for reseller with v2ray permission, got %v", err)
	}
}

// TestCreatePackage_RejectsUnauthorizedReseller confirms CreatePackage stops
// at the permission check for a reseller lacking CanResellV2Ray, before ever
// listing panels or attempting a fan-out.
func TestCreatePackage_RejectsUnauthorizedReseller(t *testing.T) {
	svc, db := newTestV2RayPackageService(t)

	reseller := model.Reseller{Name: "no-v2ray", Username: "no-v2ray-2", PasswordHash: "x", CanResellV2Ray: false}
	if err := db.Create(&reseller).Error; err != nil {
		t.Fatalf("failed to create reseller: %v", err)
	}

	req := &schema.CreateV2RayPackageRequest{
		TotalVolumeBytes: 1024,
		DurationDays:     30,
	}

	_, err := svc.CreatePackage(req, &reseller.ID)
	if err == nil {
		t.Fatal("expected error creating package for unauthorized reseller")
	}
}

// TestCreatePackage_AdminScopeSkipsPermissionCheck confirms an admin-direct
// package (resellerID nil) is created successfully even with zero
// registered panels -- CreatePackage's fan-out loop is safe to run with an
// empty panel list, matching the doc comment's claim that this is a valid,
// visible state rather than an error.
func TestCreatePackage_AdminScopeSkipsPermissionCheck(t *testing.T) {
	svc, _ := newTestV2RayPackageService(t)

	req := &schema.CreateV2RayPackageRequest{
		TotalVolumeBytes: 2048,
		DurationDays:     30,
	}

	resp, err := svc.CreatePackage(req, nil)
	if err != nil {
		t.Fatalf("expected admin-direct package creation to succeed, got %v", err)
	}
	if resp.ResellerID != nil {
		t.Fatalf("expected admin-direct package to have nil ResellerID, got %v", *resp.ResellerID)
	}
	if len(resp.Locations) != 0 {
		t.Fatalf("expected zero locations with zero registered panels, got %d", len(resp.Locations))
	}
}

// TestCreatePackage_FailedLocationIsPersistedDisabled is a regression test
// for a real bug a live smoke test caught: CreatePackage set
// location.Enabled = false on the in-memory struct before calling
// db.Create(&location), but GORM's Create() silently substitutes the
// column's `gorm:"default:true"` value for any explicitly-false bool field
// (false being Go's zero value, indistinguishable from "never set" to
// GORM) -- so every failed-AddClient location was actually being persisted
// as Enabled=true despite LastSyncError being correctly recorded. Fixed by
// creating the row first, then issuing a separate Update("enabled", false)
// call, which this test guards against regressing.
func TestCreatePackage_FailedLocationIsPersistedDisabled(t *testing.T) {
	svc, db := newTestV2RayPackageService(t)

	// Port 1 on loopback: nothing listens there, and it fails near-
	// instantly (connection refused) rather than waiting out a real
	// network timeout, keeping this test fast.
	panel := model.XuiPanel{
		Name: "unreachable", SaleTitle: "Unreachable", APIBaseURL: "http://127.0.0.1:1",
		Username: "admin", Password: "admin", DefaultInboundID: 1, Protocol: "vless",
		SubBaseURL: "http://127.0.0.1:1/sub", Status: "active",
	}
	if err := db.Create(&panel).Error; err != nil {
		t.Fatalf("failed to create panel: %v", err)
	}

	req := &schema.CreateV2RayPackageRequest{
		TotalVolumeBytes: 1024,
		DurationDays:     30,
	}

	resp, err := svc.CreatePackage(req, nil)
	if err != nil {
		t.Fatalf("expected package creation to succeed despite an unreachable panel, got %v", err)
	}
	if len(resp.Locations) != 1 {
		t.Fatalf("expected exactly one location for the one registered panel, got %d", len(resp.Locations))
	}

	loc := resp.Locations[0]
	if loc.LastSyncError == nil {
		t.Fatal("expected LastSyncError to be set for the unreachable panel")
	}
	if loc.Enabled {
		t.Fatal("expected a location whose AddClient call failed to be persisted as Enabled=false, got true")
	}

	// Confirm directly against the DB too, not just the transformed
	// response -- guards against a bug that only manifests in one layer.
	var dbLocation model.V2RayPackageLocation
	if err := db.Where("package_id = ?", resp.Id).First(&dbLocation).Error; err != nil {
		t.Fatalf("failed to reload location from db: %v", err)
	}
	if dbLocation.Enabled {
		t.Fatal("expected the persisted db row's Enabled column to be false, got true")
	}
}

// makeUnreachablePanel creates a panel row pointed at loopback port 1 (a
// well-known-to-fail-instantly address, no real network timeout) -- used
// throughout the panel-scoping tests below, which only care about WHICH
// panels get a V2RayPackageLocation row, not whether AddClient itself
// succeeds.
func makeUnreachablePanel(t *testing.T, db *gorm.DB, name string) model.XuiPanel {
	t.Helper()
	panel := model.XuiPanel{
		Name: name, SaleTitle: name, APIBaseURL: "http://127.0.0.1:1",
		Username: "admin", Password: "admin", DefaultInboundID: 1, Protocol: "vless",
		SubBaseURL: "http://127.0.0.1:1/sub", Status: "active",
	}
	if err := db.Create(&panel).Error; err != nil {
		t.Fatalf("failed to create panel %q: %v", name, err)
	}
	return panel
}

// TestCreatePackage_AdminDefaultUsesEveryRegisteredPanel confirms an
// admin-direct package with no PanelIDs specified fans out to EVERY
// registered panel -- the "multiple panels default to all" rule.
func TestCreatePackage_AdminDefaultUsesEveryRegisteredPanel(t *testing.T) {
	svc, db := newTestV2RayPackageService(t)
	makeUnreachablePanel(t, db, "panel-a")
	makeUnreachablePanel(t, db, "panel-b")
	makeUnreachablePanel(t, db, "panel-c")

	resp, err := svc.CreatePackage(&schema.CreateV2RayPackageRequest{TotalVolumeBytes: 1024, DurationDays: 30}, nil)
	if err != nil {
		t.Fatalf("expected package creation to succeed, got %v", err)
	}
	if len(resp.Locations) != 3 {
		t.Fatalf("expected a location on all 3 registered panels, got %d", len(resp.Locations))
	}
}

// TestCreatePackage_AdminNarrowsToRequestedPanels confirms an admin-direct
// package with an explicit PanelIDs subset only fans out to those panels,
// not every registered one.
func TestCreatePackage_AdminNarrowsToRequestedPanels(t *testing.T) {
	svc, db := newTestV2RayPackageService(t)
	panelA := makeUnreachablePanel(t, db, "panel-a")
	makeUnreachablePanel(t, db, "panel-b")
	makeUnreachablePanel(t, db, "panel-c")

	resp, err := svc.CreatePackage(&schema.CreateV2RayPackageRequest{
		TotalVolumeBytes: 1024, DurationDays: 30, PanelIDs: []uint{panelA.ID},
	}, nil)
	if err != nil {
		t.Fatalf("expected package creation to succeed, got %v", err)
	}
	if len(resp.Locations) != 1 {
		t.Fatalf("expected exactly 1 location (the requested panel), got %d", len(resp.Locations))
	}
	if resp.Locations[0].PanelID != panelA.ID {
		t.Fatalf("expected the requested panel %d, got %d", panelA.ID, resp.Locations[0].PanelID)
	}
}

// TestCreatePackage_ResellerDefaultUsesOnlyGrantedPanels confirms a
// reseller package with no PanelIDs specified defaults to every panel THAT
// RESELLER was granted -- not every registered panel (the key difference
// from the admin-direct default, and the entire point of
// ResellerXuiPanelAccess existing).
func TestCreatePackage_ResellerDefaultUsesOnlyGrantedPanels(t *testing.T) {
	svc, db := newTestV2RayPackageService(t)
	panelA := makeUnreachablePanel(t, db, "panel-a")
	makeUnreachablePanel(t, db, "panel-b") // never granted to the reseller below

	reseller := model.Reseller{Name: "r1", Username: "r1", PasswordHash: "x", CanResellV2Ray: true}
	if err := db.Create(&reseller).Error; err != nil {
		t.Fatalf("failed to create reseller: %v", err)
	}
	if err := svc.SetAssignedXuiPanels(reseller.ID, []uint{panelA.ID}); err != nil {
		t.Fatalf("failed to grant panel access: %v", err)
	}

	resp, err := svc.CreatePackage(&schema.CreateV2RayPackageRequest{TotalVolumeBytes: 1024, DurationDays: 30}, &reseller.ID)
	if err != nil {
		t.Fatalf("expected package creation to succeed, got %v", err)
	}
	if len(resp.Locations) != 1 {
		t.Fatalf("expected exactly 1 location (only the granted panel), got %d", len(resp.Locations))
	}
	if resp.Locations[0].PanelID != panelA.ID {
		t.Fatalf("expected the granted panel %d, got %d", panelA.ID, resp.Locations[0].PanelID)
	}
}

// TestCreatePackage_ResellerWithNoGrantedPanelsGetsZeroLocations confirms
// CanResellV2Ray=true alone is not enough -- a reseller granted zero panels
// (or never assigned any) gets a package with zero locations, mirroring
// ResellerInterface's own "must be explicitly assigned, no implicit
// access" contract.
func TestCreatePackage_ResellerWithNoGrantedPanelsGetsZeroLocations(t *testing.T) {
	svc, db := newTestV2RayPackageService(t)
	makeUnreachablePanel(t, db, "panel-a")

	reseller := model.Reseller{Name: "r2", Username: "r2", PasswordHash: "x", CanResellV2Ray: true}
	if err := db.Create(&reseller).Error; err != nil {
		t.Fatalf("failed to create reseller: %v", err)
	}
	// Deliberately no SetAssignedXuiPanels call.

	resp, err := svc.CreatePackage(&schema.CreateV2RayPackageRequest{TotalVolumeBytes: 1024, DurationDays: 30}, &reseller.ID)
	if err != nil {
		t.Fatalf("expected package creation to succeed (zero locations is valid, not an error), got %v", err)
	}
	if len(resp.Locations) != 0 {
		t.Fatalf("expected zero locations for a reseller with no granted panels, got %d", len(resp.Locations))
	}
}

// TestCreatePackage_ResellerCannotRequestUngrantedPanel confirms a
// reseller explicitly requesting a panel they were NOT granted is
// rejected with a clear error, rather than silently ignored or silently
// granting access.
func TestCreatePackage_ResellerCannotRequestUngrantedPanel(t *testing.T) {
	svc, db := newTestV2RayPackageService(t)
	panelA := makeUnreachablePanel(t, db, "panel-a")
	panelB := makeUnreachablePanel(t, db, "panel-b")

	reseller := model.Reseller{Name: "r3", Username: "r3", PasswordHash: "x", CanResellV2Ray: true}
	if err := db.Create(&reseller).Error; err != nil {
		t.Fatalf("failed to create reseller: %v", err)
	}
	if err := svc.SetAssignedXuiPanels(reseller.ID, []uint{panelA.ID}); err != nil {
		t.Fatalf("failed to grant panel access: %v", err)
	}

	_, err := svc.CreatePackage(&schema.CreateV2RayPackageRequest{
		TotalVolumeBytes: 1024, DurationDays: 30, PanelIDs: []uint{panelB.ID},
	}, &reseller.ID)
	if err == nil {
		t.Fatal("expected an error requesting a panel the reseller was not granted")
	}
}

// TestSetAssignedXuiPanels_ReplacesFullSet confirms a second call fully
// replaces the previous grant set rather than merging with it, mirroring
// SetAssignedUserManagerGroups's delete-all-then-recreate contract.
func TestSetAssignedXuiPanels_ReplacesFullSet(t *testing.T) {
	svc, db := newTestV2RayPackageService(t)
	panelA := makeUnreachablePanel(t, db, "panel-a")
	panelB := makeUnreachablePanel(t, db, "panel-b")

	reseller := model.Reseller{Name: "r4", Username: "r4", PasswordHash: "x"}
	if err := db.Create(&reseller).Error; err != nil {
		t.Fatalf("failed to create reseller: %v", err)
	}

	if err := svc.SetAssignedXuiPanels(reseller.ID, []uint{panelA.ID, panelB.ID}); err != nil {
		t.Fatalf("failed to grant both panels: %v", err)
	}
	if err := svc.SetAssignedXuiPanels(reseller.ID, []uint{panelB.ID}); err != nil {
		t.Fatalf("failed to narrow grant to one panel: %v", err)
	}

	got, err := svc.GetAssignedXuiPanels(reseller.ID)
	if err != nil {
		t.Fatalf("failed to read back assigned panels: %v", err)
	}
	if len(got) != 1 || got[0] != panelB.ID {
		t.Fatalf("expected exactly [%d] after replacement, got %v", panelB.ID, got)
	}
}

// makeSlowStubPanel starts an httptest.Server that mimics enough of x-ui's
// real /login + addClient behavior to satisfy xui.AddClient, but sleeps
// slowDelay before responding to every request -- used to prove
// CreatePackage/DeletePackage call every panel CONCURRENTLY, not
// sequentially. t.Cleanup closes the server automatically.
func makeSlowStubPanel(t *testing.T, db *gorm.DB, name string, slowDelay time.Duration) model.XuiPanel {
	t.Helper()

	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		time.Sleep(slowDelay)
		switch r.URL.Path {
		case "/login":
			http.SetCookie(w, &http.Cookie{Name: "session", Value: "tok", Path: "/"})
			w.Header().Set("Content-Type", "application/json")
			_ = json.NewEncoder(w).Encode(map[string]interface{}{"success": true, "msg": ""})
		default:
			w.Header().Set("Content-Type", "application/json")
			_ = json.NewEncoder(w).Encode(map[string]interface{}{"success": true, "msg": ""})
		}
	}))
	t.Cleanup(server.Close)

	panel := model.XuiPanel{
		Name: name, SaleTitle: name, APIBaseURL: server.URL,
		Username: "admin", Password: "admin", DefaultInboundID: 1, Protocol: "vless",
		SubBaseURL: server.URL + "/sub", Status: "active",
	}
	if err := db.Create(&panel).Error; err != nil {
		t.Fatalf("failed to create panel %q: %v", name, err)
	}
	return panel
}

// TestCreatePackage_CallsPanelsConcurrentlyNotSequentially is a regression
// test for a real, severe bug a live profiling run found: CreatePackage
// called xui.AddClient on each registered panel inside a single-threaded
// for loop, so N slow/unreachable panels added their delays TOGETHER to
// the total request time (measured live: 3 unreachable panels made a
// single create-package HTTP request take ~45 seconds -- 3 x the 15s
// per-panel xui.Login timeout). Fixed by running each panel's AddClient
// call on its own goroutine. This test uses 3 stub panels that each sleep
// 300ms per request: the sequential-bug version would take >=900ms
// (3 x 300ms, likely more with the multiple requests each panel goroutine
// makes), the fixed concurrent version should complete in well under that,
// close to the single slowest panel's own delay.
func TestCreatePackage_CallsPanelsConcurrentlyNotSequentially(t *testing.T) {
	svc, db := newTestV2RayPackageService(t)

	const perPanelDelay = 300 * time.Millisecond
	makeSlowStubPanel(t, db, "slow-a", perPanelDelay)
	makeSlowStubPanel(t, db, "slow-b", perPanelDelay)
	makeSlowStubPanel(t, db, "slow-c", perPanelDelay)

	start := time.Now()
	resp, err := svc.CreatePackage(&schema.CreateV2RayPackageRequest{TotalVolumeBytes: 1024, DurationDays: 30}, nil)
	elapsed := time.Since(start)

	if err != nil {
		t.Fatalf("expected package creation to succeed, got %v", err)
	}
	if len(resp.Locations) != 3 {
		t.Fatalf("expected 3 locations (one per panel), got %d", len(resp.Locations))
	}

	// Each panel goroutine now makes 4 sequential round-trips against the
	// stub (login+get for xui.ResolveClientFlow's inbound-security check,
	// then login+addClient), each hitting the stub's perPanelDelay sleep
	// (the stub sleeps on every path, not just /login) -- so even a FULLY
	// concurrent implementation needs >= 4 x perPanelDelay (1200ms) for the
	// single slowest panel's own four round-trips. A sequential
	// implementation needs 3 panels x 4 round-trips x perPanelDelay
	// (>= 3600ms). The threshold sits well between those two numbers --
	// comfortably above the concurrent floor (leaving headroom for
	// scheduling/CI jitter) and comfortably below the sequential floor.
	const roundTripsPerPanel = 4
	concurrentFloor := roundTripsPerPanel * perPanelDelay
	sequentialFloor := time.Duration(len(resp.Locations)) * roundTripsPerPanel * perPanelDelay
	threshold := (concurrentFloor + sequentialFloor) / 2
	if elapsed >= threshold {
		t.Fatalf("CreatePackage took %v across 3 panels with %v delay each -- expected well under %v (sequential floor) if panels are called concurrently, this looks sequential again", elapsed, perPanelDelay, threshold)
	}
}

// makeTrafficStubPanel starts an httptest.Server implementing /login and
// GET .../getClientTraffics/:email, returning the given up/down pair for
// every email -- used by the GetLiveUsage tests below.
func makeTrafficStubPanel(t *testing.T, db *gorm.DB, name string, up, down int64) model.XuiPanel {
	t.Helper()

	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		switch {
		case r.URL.Path == "/login":
			http.SetCookie(w, &http.Cookie{Name: "3x-ui", Value: "tok", Path: "/"})
			w.Header().Set("Content-Type", "application/json")
			_ = json.NewEncoder(w).Encode(map[string]interface{}{"success": true, "msg": ""})
		case len(r.URL.Path) > len("/xui/API/inbounds/getClientTraffics/"):
			w.Header().Set("Content-Type", "application/json")
			_ = json.NewEncoder(w).Encode(map[string]interface{}{
				"success": true, "msg": "",
				"obj": map[string]interface{}{"up": up, "down": down, "total": 0},
			})
		default:
			w.WriteHeader(http.StatusNotFound)
		}
	}))
	t.Cleanup(server.Close)

	panel := model.XuiPanel{
		Name: name, SaleTitle: name, APIBaseURL: server.URL,
		Username: "admin", Password: "admin", DefaultInboundID: 1, Protocol: "vless",
		SubBaseURL: server.URL + "/sub", Status: "active",
	}
	if err := db.Create(&panel).Error; err != nil {
		t.Fatalf("failed to create panel %q: %v", name, err)
	}
	return panel
}

// TestGetLiveUsage_RefreshesCacheFromLiveReading is a regression/contract
// test for the "view live usage" on-demand action (explicitly requested by
// a customer's V2Ray engineer): confirms GetLiveUsage calls x-ui LIVE
// (not the cache), returns the up/down split per location, and -- as its
// documented side effect -- overwrites UsedBytesCached with the fresh
// reading so the next ordinary (cached) list view reflects it too.
func TestGetLiveUsage_RefreshesCacheFromLiveReading(t *testing.T) {
	svc, db := newTestV2RayPackageService(t)

	panel := makeTrafficStubPanel(t, db, "live-panel", 300, 700)

	pkg := model.V2RayPackage{UUID: "pkg-live-usage", TotalVolumeBytes: 10000, DurationDays: 30, Status: "active"}
	if err := db.Create(&pkg).Error; err != nil {
		t.Fatalf("failed to create package: %v", err)
	}

	loc := model.V2RayPackageLocation{
		PackageID: pkg.ID, PanelID: panel.ID,
		ClientUUID: "c1", ClientEmail: "c1@test", SubID: "s1", Enabled: true,
		UsedBytesCached: 1, // deliberately stale, to prove it gets overwritten
	}
	if err := db.Create(&loc).Error; err != nil {
		t.Fatalf("failed to create location: %v", err)
	}

	usage, err := svc.GetLiveUsage(pkg.ID, nil)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	if len(usage.Locations) != 1 {
		t.Fatalf("expected 1 location row, got %d", len(usage.Locations))
	}
	row := usage.Locations[0]
	if row.UpBytes != 300 || row.DownBytes != 700 || row.TotalBytes != 1000 {
		t.Fatalf("expected up=300 down=700 total=1000, got up=%d down=%d total=%d", row.UpBytes, row.DownBytes, row.TotalBytes)
	}
	if usage.TotalUsedBytes != 1000 {
		t.Fatalf("expected response-level TotalUsedBytes 1000, got %d", usage.TotalUsedBytes)
	}
	if usage.TotalVolumeBytes != 10000 {
		t.Fatalf("expected TotalVolumeBytes to mirror the package's own limit (10000), got %d", usage.TotalVolumeBytes)
	}

	var reloaded model.V2RayPackageLocation
	if err := db.First(&reloaded, loc.ID).Error; err != nil {
		t.Fatalf("failed to reload location: %v", err)
	}
	if reloaded.UsedBytesCached != 1000 {
		t.Fatalf("expected GetLiveUsage to refresh UsedBytesCached to 1000, got %d (stale cache was not overwritten)", reloaded.UsedBytesCached)
	}
}

// TestGetLiveUsage_OnePanelFailureDoesNotBlockOthers confirms the same
// "one dead panel never breaks the rest" tolerance already established for
// the periodic sync job (see V2RaySyncService.SyncPackageUsage's own doc
// comment) also applies to this on-demand action: a location on an
// unreachable panel gets an Error string, not a failed whole-request.
func TestGetLiveUsage_OnePanelFailureDoesNotBlockOthers(t *testing.T) {
	svc, db := newTestV2RayPackageService(t)

	workingPanel := makeTrafficStubPanel(t, db, "working-panel", 100, 200)
	deadPanel := model.XuiPanel{
		Name: "dead-panel", SaleTitle: "dead", APIBaseURL: "http://127.0.0.1:1",
		Username: "admin", Password: "admin", DefaultInboundID: 1, Protocol: "vless",
		SubBaseURL: "http://127.0.0.1:1/sub", Status: "active",
	}
	if err := db.Create(&deadPanel).Error; err != nil {
		t.Fatalf("failed to create dead panel: %v", err)
	}

	pkg := model.V2RayPackage{UUID: "pkg-live-usage-partial", TotalVolumeBytes: 10000, DurationDays: 30, Status: "active"}
	if err := db.Create(&pkg).Error; err != nil {
		t.Fatalf("failed to create package: %v", err)
	}

	workingLoc := model.V2RayPackageLocation{
		PackageID: pkg.ID, PanelID: workingPanel.ID,
		ClientUUID: "c-working", ClientEmail: "working@test", SubID: "s-working", Enabled: true,
	}
	if err := db.Create(&workingLoc).Error; err != nil {
		t.Fatalf("failed to create working location: %v", err)
	}

	deadLoc := model.V2RayPackageLocation{
		PackageID: pkg.ID, PanelID: deadPanel.ID,
		ClientUUID: "c-dead", ClientEmail: "dead@test", SubID: "s-dead", Enabled: true,
	}
	if err := db.Create(&deadLoc).Error; err != nil {
		t.Fatalf("failed to create dead location: %v", err)
	}

	usage, err := svc.GetLiveUsage(pkg.ID, nil)
	if err != nil {
		t.Fatalf("expected GetLiveUsage to succeed despite one dead panel, got error: %v", err)
	}
	if len(usage.Locations) != 2 {
		t.Fatalf("expected 2 location rows, got %d", len(usage.Locations))
	}

	var workingRow, deadRow *schema.V2RayLiveUsageLocation
	for i := range usage.Locations {
		if usage.Locations[i].PanelID == workingPanel.ID {
			workingRow = &usage.Locations[i]
		}
		if usage.Locations[i].PanelID == deadPanel.ID {
			deadRow = &usage.Locations[i]
		}
	}
	if workingRow == nil || workingRow.Error != nil || workingRow.TotalBytes != 300 {
		t.Fatalf("expected the working panel's row to succeed with total=300, got %+v", workingRow)
	}
	if deadRow == nil || deadRow.Error == nil {
		t.Fatalf("expected the dead panel's row to carry an Error, got %+v", deadRow)
	}
}
