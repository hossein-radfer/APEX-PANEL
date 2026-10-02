package service

import (
	"encoding/base64"
	"encoding/json"
	"fmt"
	"net/http"
	"net/http/httptest"
	"strings"
	"sync/atomic"
	"testing"
	"time"

	"github.com/glebarez/sqlite"
	"github.com/maahdima/mwp/api/dataservice/model"
	"gorm.io/gorm"
)

// newTestV2RaySyncService uses a uniquely-named in-memory DB per call --
// see newTestV2RayPackageService's doc comment (v2ray_permission_test.go)
// for why the bare "file::memory:?cache=shared" DSN is unsafe to reuse
// across test functions.
func newTestV2RaySyncService(t *testing.T) (*V2RaySyncService, *gorm.DB) {
	t.Helper()

	dsn := fmt.Sprintf("file:v2ray_sync_test_%d?mode=memory&cache=shared", time.Now().UnixNano())
	db, err := gorm.Open(sqlite.Open(dsn), &gorm.Config{})
	if err != nil {
		t.Fatalf("failed to open test db: %v", err)
	}

	if err := db.AutoMigrate(
		&model.Reseller{},
		&model.XuiPanel{},
		&model.ResellerV2RaySaleTitle{},
		&model.V2RayPackage{},
		&model.V2RayPackageLocation{},
	); err != nil {
		t.Fatalf("failed to migrate test db: %v", err)
	}

	panels := NewXuiPanelService(db)
	return NewV2RaySyncService(db, panels), db
}

// TestCalculateV2RayDelta_Monotonic confirms the ordinary case: current >=
// prev is a plain subtraction, no reset flagged.
func TestCalculateV2RayDelta_Monotonic(t *testing.T) {
	delta, wasReset := calculateV2RayDelta(1000, 1500, v2rayByteCounterMax)
	if delta != 500 {
		t.Fatalf("expected delta 500, got %d", delta)
	}
	if wasReset {
		t.Fatal("expected no reset for monotonic increase")
	}
}

// TestCalculateV2RayDelta_Reset confirms a counter drop is treated as a
// reset (the entire current reading counted as new usage), mirroring
// calculateDelta's behavior in cmd/jobs/traffic.go.
func TestCalculateV2RayDelta_Reset(t *testing.T) {
	delta, wasReset := calculateV2RayDelta(5000, 100, v2rayByteCounterMax)
	if !wasReset {
		t.Fatal("expected reset to be detected on counter drop")
	}
	if delta != 100 {
		t.Fatalf("expected delta to equal the new reading (100) on reset, got %d", delta)
	}
}

// TestCalculateV2RayDelta_Unchanged confirms a repeated identical reading
// (the common case between two ticks with no new traffic) produces a zero
// delta, not a false reset.
func TestCalculateV2RayDelta_Unchanged(t *testing.T) {
	delta, wasReset := calculateV2RayDelta(2000, 2000, v2rayByteCounterMax)
	if delta != 0 {
		t.Fatalf("expected zero delta for unchanged counter, got %d", delta)
	}
	if wasReset {
		t.Fatal("expected no reset for an unchanged counter")
	}
}

// TestRecordLocationError_IsolatedToOneLocation confirms a location whose
// panel no longer exists gets its own LastSyncError recorded without
// touching any other location -- the concrete implementation of "one dead
// panel never breaks the whole package" for the case where the panel row
// itself was deleted out from under an existing location.
func TestRecordLocationError_IsolatedToOneLocation(t *testing.T) {
	sync, db := newTestV2RaySyncService(t)

	pkg := model.V2RayPackage{UUID: "pkg-uuid", TotalVolumeBytes: 1024, DurationDays: 30, Status: "active"}
	if err := db.Create(&pkg).Error; err != nil {
		t.Fatalf("failed to create package: %v", err)
	}

	deadLocation := model.V2RayPackageLocation{
		PackageID: pkg.ID, PanelID: 9999, // no such panel
		ClientUUID: "client-a", ClientEmail: "a@test", SubID: "sub-a", Enabled: true,
	}
	if err := db.Create(&deadLocation).Error; err != nil {
		t.Fatalf("failed to create dead location: %v", err)
	}

	healthyLocation := model.V2RayPackageLocation{
		PackageID: pkg.ID, PanelID: 8888, // also no such panel, but untouched by the call below
		ClientUUID: "client-b", ClientEmail: "b@test", SubID: "sub-b", Enabled: true,
		UsedBytesCached: 500,
	}
	if err := db.Create(&healthyLocation).Error; err != nil {
		t.Fatalf("failed to create healthy location: %v", err)
	}

	sync.recordLocationError(deadLocation.ID, gorm.ErrRecordNotFound)

	var reloadedDead model.V2RayPackageLocation
	if err := db.First(&reloadedDead, deadLocation.ID).Error; err != nil {
		t.Fatalf("failed to reload dead location: %v", err)
	}
	if reloadedDead.LastSyncError == nil {
		t.Fatal("expected LastSyncError to be set on the dead location")
	}

	var reloadedHealthy model.V2RayPackageLocation
	if err := db.First(&reloadedHealthy, healthyLocation.ID).Error; err != nil {
		t.Fatalf("failed to reload healthy location: %v", err)
	}
	if reloadedHealthy.LastSyncError != nil {
		t.Fatalf("expected healthy location's LastSyncError to remain nil, got %v", *reloadedHealthy.LastSyncError)
	}
	if reloadedHealthy.UsedBytesCached != 500 {
		t.Fatalf("expected healthy location's usage to be untouched, got %d", reloadedHealthy.UsedBytesCached)
	}
}

// TestRecombineConfig_SkipsEmptyAndStaleLocations confirms recombineConfig
// only folds in locations with a non-empty ConfigLinkCached -- a location
// that has never successfully synced (empty cache) is silently omitted
// from the combined subscription rather than corrupting it, the concrete
// mechanism behind "one dead panel never breaks the whole package" applied
// to the public subscription endpoint.
func TestRecombineConfig_SkipsEmptyAndStaleLocations(t *testing.T) {
	sync, db := newTestV2RaySyncService(t)

	pkg := model.V2RayPackage{UUID: "pkg-uuid-2", TotalVolumeBytes: 1024, DurationDays: 30, Status: "active"}
	if err := db.Create(&pkg).Error; err != nil {
		t.Fatalf("failed to create package: %v", err)
	}

	// ConfigLinkCached is plaintext, not base64 -- see the field's own doc
	// comment (model.V2RayPackageLocation) and rewriteSubscriptionTitle's.
	working := model.V2RayPackageLocation{
		PackageID: pkg.ID, PanelID: 1,
		ClientUUID: "c1", ClientEmail: "c1@test", SubID: "s1", Enabled: true,
		ConfigLinkCached: "vless://good-link#Working",
	}
	if err := db.Create(&working).Error; err != nil {
		t.Fatalf("failed to create working location: %v", err)
	}

	neverSynced := model.V2RayPackageLocation{
		PackageID: pkg.ID, PanelID: 2,
		ClientUUID: "c2", ClientEmail: "c2@test", SubID: "s2", Enabled: true,
		ConfigLinkCached: "", // never successfully synced
	}
	if err := db.Create(&neverSynced).Error; err != nil {
		t.Fatalf("failed to create never-synced location: %v", err)
	}

	sync.recombineConfig(pkg.ID)

	var reloaded model.V2RayPackage
	if err := db.First(&reloaded, pkg.ID).Error; err != nil {
		t.Fatalf("failed to reload package: %v", err)
	}
	if reloaded.CombinedConfigCached == "" {
		t.Fatal("expected CombinedConfigCached to be populated from the one working location")
	}

	decoded, err := base64.StdEncoding.DecodeString(reloaded.CombinedConfigCached)
	if err != nil {
		t.Fatalf("expected CombinedConfigCached to be valid base64, got error: %v", err)
	}
	// The first line is always the synthetic info entry (see
	// buildInfoConfigLine) -- only the SECOND line is the one real working
	// location's link; the never-synced location must still be omitted.
	lines := strings.Split(string(decoded), "\n")
	if len(lines) != 2 {
		t.Fatalf("expected exactly 2 lines (synthetic info + one working location), got %d: %q", len(lines), string(decoded))
	}
	if !strings.HasPrefix(lines[0], "vless://00000000-0000-0000-0000-000000000000@") {
		t.Fatalf("expected the first line to be the synthetic info entry, got %q", lines[0])
	}
	if lines[1] != "vless://good-link#Working" {
		t.Fatalf("expected the second line to be exactly the working location's link, got %q", lines[1])
	}
}

// TestSyncPackageUsage_OnlineStatusPolledOncePerPanel is a regression test
// for the online-status feature (a customer's V2Ray engineer explicitly
// requested per-location online indicators): two locations on the SAME
// panel -- one per PACKAGE, since V2RayPackageLocation's own
// package_id+panel_id uniqueness constraint forbids two locations for the
// same package on the same panel -- must trigger exactly one
// /xui/API/inbounds/onlines call for that panel's tick, not one per
// location, and each location's is_online must reflect whether ITS OWN
// client email appeared in that one shared response, not the other
// location's.
func TestSyncPackageUsage_OnlineStatusPolledOncePerPanel(t *testing.T) {
	sync, db := newTestV2RaySyncService(t)

	var onlinesCallCount int32
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		switch {
		case r.URL.Path == "/login":
			http.SetCookie(w, &http.Cookie{Name: "3x-ui", Value: "tok", Path: "/"})
			w.Header().Set("Content-Type", "application/json")
			_ = json.NewEncoder(w).Encode(map[string]interface{}{"success": true, "msg": ""})
		case r.URL.Path == "/xui/API/inbounds/onlines":
			atomic.AddInt32(&onlinesCallCount, 1)
			w.Header().Set("Content-Type", "application/json")
			_ = json.NewEncoder(w).Encode(map[string]interface{}{
				"success": true, "msg": "", "obj": []string{"online@test"},
			})
		case strings.HasPrefix(r.URL.Path, "/xui/API/inbounds/getClientTraffics/"):
			w.Header().Set("Content-Type", "application/json")
			_ = json.NewEncoder(w).Encode(map[string]interface{}{
				"success": true, "msg": "", "obj": map[string]interface{}{"up": 0, "down": 0, "total": 0},
			})
		default:
			// GetSubscription hits a separate, unauthenticated server (see
			// xui.GetSubscription's own doc comment) -- anything else here
			// (including a subscription-path guess) can safely 404, since
			// syncOneLocation treats that as non-fatal and keeps whatever
			// ConfigLinkCached already held.
			w.WriteHeader(http.StatusNotFound)
		}
	}))
	defer server.Close()

	panel := model.XuiPanel{
		Name: "shared-panel", SaleTitle: "Shared", APIBaseURL: server.URL,
		Username: "admin", Password: "admin", DefaultInboundID: 1, Protocol: "vless",
		SubBaseURL: server.URL + "/sub", Status: "active",
	}
	if err := db.Create(&panel).Error; err != nil {
		t.Fatalf("failed to create panel: %v", err)
	}

	pkgA := model.V2RayPackage{UUID: "pkg-online-test-a", TotalVolumeBytes: 1024 * 1024 * 1024, DurationDays: 30, Status: "active"}
	if err := db.Create(&pkgA).Error; err != nil {
		t.Fatalf("failed to create package A: %v", err)
	}
	pkgB := model.V2RayPackage{UUID: "pkg-online-test-b", TotalVolumeBytes: 1024 * 1024 * 1024, DurationDays: 30, Status: "active"}
	if err := db.Create(&pkgB).Error; err != nil {
		t.Fatalf("failed to create package B: %v", err)
	}

	onlineLoc := model.V2RayPackageLocation{
		PackageID: pkgA.ID, PanelID: panel.ID,
		ClientUUID: "c-online", ClientEmail: "online@test", SubID: "s-online", Enabled: true,
		FlowRepairedAt: timeNow(), // skip the flow-repair pass, not under test here
	}
	if err := db.Create(&onlineLoc).Error; err != nil {
		t.Fatalf("failed to create online location: %v", err)
	}

	offlineLoc := model.V2RayPackageLocation{
		PackageID: pkgB.ID, PanelID: panel.ID,
		ClientUUID: "c-offline", ClientEmail: "offline@test", SubID: "s-offline", Enabled: true,
		FlowRepairedAt: timeNow(),
	}
	if err := db.Create(&offlineLoc).Error; err != nil {
		t.Fatalf("failed to create offline location: %v", err)
	}

	sync.SyncPackageUsage()

	if got := atomic.LoadInt32(&onlinesCallCount); got != 1 {
		t.Fatalf("expected exactly 1 onlines call for 2 locations sharing one panel, got %d", got)
	}

	var reloadedOnline model.V2RayPackageLocation
	if err := db.First(&reloadedOnline, onlineLoc.ID).Error; err != nil {
		t.Fatalf("failed to reload online location: %v", err)
	}
	if !reloadedOnline.IsOnline {
		t.Fatal("expected the location whose email was in the onlines response to be marked online")
	}
	if reloadedOnline.OnlineCheckedAt == nil {
		t.Fatal("expected OnlineCheckedAt to be set after a successful poll")
	}

	var reloadedOffline model.V2RayPackageLocation
	if err := db.First(&reloadedOffline, offlineLoc.ID).Error; err != nil {
		t.Fatalf("failed to reload offline location: %v", err)
	}
	if reloadedOffline.IsOnline {
		t.Fatal("expected the location whose email was NOT in the onlines response to be marked offline")
	}
}

// TestBuildInfoConfigLine_ReflectsRemainingVolumeDaysAndStatus is a
// regression/contract test for the synthetic "info" subscription entry
// (explicitly requested so a customer's V2Ray app shows their remaining
// volume/days/status at a glance in the server list, without needing the
// separate share page): confirms the title format and that it correctly
// reflects both consumed volume and a real expiry date.
func TestBuildInfoConfigLine_ReflectsRemainingVolumeDaysAndStatus(t *testing.T) {
	expireAt := time.Now().AddDate(0, 0, 10)
	pkg := model.V2RayPackage{
		TotalVolumeBytes: 10 * 1024 * 1024 * 1024, // 10GB
		Status:           "active",
		ExpireAt:         &expireAt,
	}

	line := buildInfoConfigLine(pkg, 4*1024*1024*1024) // 4GB used -> 6GB remaining

	if !strings.HasPrefix(line, "vless://") {
		t.Fatalf("expected a vless:// synthetic entry, got %q", line)
	}
	if !strings.Contains(line, "6.0GB") {
		t.Fatalf("expected the title to show 6.0GB remaining (10GB total - 4GB used), got %q", line)
	}
	// Persian "روز" ("days"), percent-encoded -- see urlEncodeTitle.
	if !strings.Contains(line, "9%20%D8%B1%D9%88%D8%B2") && !strings.Contains(line, "10%20%D8%B1%D9%88%D8%B2") {
		// time.Until's exact hour count depends on when the test runs
		// relative to expireAt, so either 9 or 10 days remaining is
		// correct here -- both are still "close to 10", never negative or
		// wildly off.
		t.Fatalf("expected the title to show ~9-10 days remaining, got %q", line)
	}
	// Persian "فعال" ("active"), percent-encoded.
	if !strings.Contains(line, "%D9%81%D8%B9%D8%A7%D9%84") {
		t.Fatalf("expected the title to include the package status, got %q", line)
	}
	// 🟢, percent-encoded (its UTF-8 bytes each fall outside urlEncodeTitle's
	// unreserved-character set) -- 40% usage is well under the 70% yellow
	// threshold.
	if !strings.Contains(line, "%F0%9F%9F%A2") {
		t.Fatalf("expected the green traffic-light emoji for 40%% usage (well under the 70%% threshold), got %q", line)
	}
}

// TestBuildInfoConfigLine_UnlimitedWhenNoExpiry confirms a package with no
// ExpireAt (unlimited duration) shows "نامحدود" (Persian "unlimited")
// rather than a nonsensical negative/zero day count.
func TestBuildInfoConfigLine_UnlimitedWhenNoExpiry(t *testing.T) {
	pkg := model.V2RayPackage{TotalVolumeBytes: 1024 * 1024 * 1024, Status: "active", ExpireAt: nil}

	line := buildInfoConfigLine(pkg, 0)

	// Persian "نامحدود" ("unlimited"), percent-encoded.
	if !strings.Contains(line, "%D9%86%D8%A7%D9%85%D8%AD%D8%AF%D9%88%D8%AF") {
		t.Fatalf("expected 'نامحدود' in the title for a package with no expiry, got %q", line)
	}
}

// TestRewriteSubscriptionTitle_OutputFeedsRecombineConfigCorrectly is a
// regression test for a real bug a live smoke test caught: an earlier
// version of rewriteSubscriptionTitle returned its result RE-ENCODED as
// base64, but recombineConfig also base64-decoded every location's
// ConfigLinkCached before combining -- a double-encoding mismatch. Since
// rewriteSubscriptionTitle's output is plaintext (not valid base64 in
// general -- it contains "://" and other characters outside the base64
// alphabet), every single location failed recombineConfig's decode step
// and was silently skipped (recombineConfig's own "one bad location never
// breaks the whole package" continue), leaving CombinedConfigCached
// permanently empty for every package regardless of sync success. This
// test exercises both functions together, exactly as SyncPackageUsage
// actually calls them, to catch any regression of this specific
// integration bug (unlike the tests above, which exercise recombineConfig
// alone with a hand-built plaintext fixture).
func TestRewriteSubscriptionTitle_OutputFeedsRecombineConfigCorrectly(t *testing.T) {
	sync, db := newTestV2RaySyncService(t)

	panel := model.XuiPanel{
		Name: "panel-x", SaleTitle: "Sale Title X", APIBaseURL: "http://127.0.0.1:1",
		Username: "admin", Password: "admin", DefaultInboundID: 1, Protocol: "vless",
		SubBaseURL: "http://127.0.0.1:1/sub", Status: "active",
	}
	if err := db.Create(&panel).Error; err != nil {
		t.Fatalf("failed to create panel: %v", err)
	}

	pkg := model.V2RayPackage{UUID: "pkg-uuid-3", TotalVolumeBytes: 1024, DurationDays: 30, Status: "active"}
	if err := db.Create(&pkg).Error; err != nil {
		t.Fatalf("failed to create package: %v", err)
	}

	loc := model.V2RayPackageLocation{
		PackageID: pkg.ID, PanelID: panel.ID,
		ClientUUID: "c1", ClientEmail: "c1@test", SubID: "s1", Enabled: true,
	}
	if err := db.Create(&loc).Error; err != nil {
		t.Fatalf("failed to create location: %v", err)
	}

	// Simulates what xui.GetSubscription actually returns: base64 of a
	// newline-separated link list, exactly as x-ui itself serves it.
	rawSubBase64 := base64.StdEncoding.EncodeToString([]byte("vless://test-uuid@host:443?type=tcp#OldTitle"))

	rewritten, err := sync.rewriteSubscriptionTitle(rawSubBase64, loc, panel)
	if err != nil {
		t.Fatalf("rewriteSubscriptionTitle failed: %v", err)
	}

	// The critical assertion: rewriteSubscriptionTitle's output must NOT
	// be base64 -- it must be the plaintext link, directly storable in
	// ConfigLinkCached and directly consumable by recombineConfig with no
	// decode step. If this regresses back to returning base64, this
	// assertion catches it immediately without needing recombineConfig at
	// all.
	if !strings.Contains(rewritten, "vless://") {
		t.Fatalf("expected plaintext output containing the rewritten link, got %q", rewritten)
	}

	if err := db.Model(&model.V2RayPackageLocation{}).Where("id = ?", loc.ID).Update("config_link_cached", rewritten).Error; err != nil {
		t.Fatalf("failed to persist rewritten config: %v", err)
	}

	sync.recombineConfig(pkg.ID)

	var reloaded model.V2RayPackage
	if err := db.First(&reloaded, pkg.ID).Error; err != nil {
		t.Fatalf("failed to reload package: %v", err)
	}
	if reloaded.CombinedConfigCached == "" {
		t.Fatal("expected CombinedConfigCached to be populated -- this is exactly the bug the smoke test caught: rewriteSubscriptionTitle's output silently failed recombineConfig's base64 decode step, leaving this permanently empty")
	}

	decoded, err := base64.StdEncoding.DecodeString(reloaded.CombinedConfigCached)
	if err != nil {
		t.Fatalf("expected CombinedConfigCached to be valid base64, got error: %v", err)
	}
	if !strings.Contains(string(decoded), "vless://test-uuid@host:443") {
		t.Fatalf("expected the combined config to contain the rewritten link, got %q", string(decoded))
	}
	if !strings.Contains(string(decoded), "Sale%20Title%20X") {
		t.Fatalf("expected the combined config's title to be rewritten to the panel's sale title, got %q", string(decoded))
	}
}

// TestRewriteSubscriptionTitle_PlaintextResponseWhenEncodingDisabled is a
// regression test for a real, reported bug: a panel with its own
// "Subscription Encode" (base64) setting turned OFF returns /sub/{subId}
// content as a RAW vless://... link, not base64 -- rewriteSubscriptionTitle
// used to unconditionally base64-decode its input, which failed with
// "illegal base64 data at input byte 5" (the ':' right after "vless") on
// exactly this response shape. syncOneLocation treats that as non-fatal
// and silently keeps ConfigLinkCached at its previous (empty, for a
// location that never synced successfully before) value -- which is what
// produced both a missing config link AND a missing QR code (the frontend
// only renders a QR for a non-empty config_link) on the share page for any
// panel configured with encoding off.
func TestRewriteSubscriptionTitle_PlaintextResponseWhenEncodingDisabled(t *testing.T) {
	sync, db := newTestV2RaySyncService(t)

	panel := model.XuiPanel{
		Name: "panel-plaintext", SaleTitle: "Plaintext Panel", APIBaseURL: "http://127.0.0.1:1",
		Username: "admin", Password: "admin", DefaultInboundID: 1, Protocol: "vless",
		SubBaseURL: "http://127.0.0.1:1/sub", Status: "active",
	}
	if err := db.Create(&panel).Error; err != nil {
		t.Fatalf("failed to create panel: %v", err)
	}

	pkg := model.V2RayPackage{UUID: "pkg-uuid-plaintext", TotalVolumeBytes: 1024, DurationDays: 30, Status: "active"}
	if err := db.Create(&pkg).Error; err != nil {
		t.Fatalf("failed to create package: %v", err)
	}

	loc := model.V2RayPackageLocation{
		PackageID: pkg.ID, PanelID: panel.ID,
		ClientUUID: "c1", ClientEmail: "c1@test", SubID: "s1", Enabled: true,
	}
	if err := db.Create(&loc).Error; err != nil {
		t.Fatalf("failed to create location: %v", err)
	}

	// NOT base64-encoded -- exactly what a panel with Subscription Encode
	// off returns: the raw link text, starting directly with "vless://".
	rawSubPlaintext := "vless://test-uuid@host:443?type=tcp#OldTitle"

	rewritten, err := sync.rewriteSubscriptionTitle(rawSubPlaintext, loc, panel)
	if err != nil {
		t.Fatalf("expected rewriteSubscriptionTitle to handle plaintext (non-base64) input without error, got: %v", err)
	}
	if !strings.Contains(rewritten, "vless://test-uuid@host:443") {
		t.Fatalf("expected the plaintext link to be preserved, got %q", rewritten)
	}
	if !strings.Contains(rewritten, "Plaintext%20Panel") {
		t.Fatalf("expected the title to be rewritten even on the plaintext path, got %q", rewritten)
	}
}

// TestRewriteSubscriptionTitle_MultiLinePlaintextResponse confirms the
// plaintext path handles multiple links (one per line), each rewritten
// independently -- not just the single-link case above.
func TestRewriteSubscriptionTitle_MultiLinePlaintextResponse(t *testing.T) {
	sync, db := newTestV2RaySyncService(t)

	panel := model.XuiPanel{
		Name: "panel-multiline", SaleTitle: "Multi Line", APIBaseURL: "http://127.0.0.1:1",
		Username: "admin", Password: "admin", DefaultInboundID: 1, Protocol: "vless",
		SubBaseURL: "http://127.0.0.1:1/sub", Status: "active",
	}
	if err := db.Create(&panel).Error; err != nil {
		t.Fatalf("failed to create panel: %v", err)
	}

	pkg := model.V2RayPackage{UUID: "pkg-uuid-multiline", TotalVolumeBytes: 1024, DurationDays: 30, Status: "active"}
	if err := db.Create(&pkg).Error; err != nil {
		t.Fatalf("failed to create package: %v", err)
	}

	loc := model.V2RayPackageLocation{
		PackageID: pkg.ID, PanelID: panel.ID,
		ClientUUID: "c1", ClientEmail: "c1@test", SubID: "s1", Enabled: true,
	}
	if err := db.Create(&loc).Error; err != nil {
		t.Fatalf("failed to create location: %v", err)
	}

	rawSubPlaintext := "vless://uuid-a@host:443?type=tcp#TitleA\nvmess://eyJ2IjoiMiJ9#TitleB"

	rewritten, err := sync.rewriteSubscriptionTitle(rawSubPlaintext, loc, panel)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	lines := strings.Split(rewritten, "\n")
	if len(lines) != 2 {
		t.Fatalf("expected 2 rewritten lines, got %d: %q", len(lines), rewritten)
	}
	if !strings.HasPrefix(lines[0], "vless://uuid-a@host:443") || !strings.HasSuffix(lines[0], "Multi%20Line") {
		t.Fatalf("expected the first line to keep its link and get the rewritten title, got %q", lines[0])
	}
	if !strings.HasPrefix(lines[1], "vmess://eyJ2IjoiMiJ9") || !strings.HasSuffix(lines[1], "Multi%20Line") {
		t.Fatalf("expected the second line to keep its link and get the rewritten title, got %q", lines[1])
	}
}

// TestIsRawLinkContent covers every scheme prefix this codebase must
// recognize as plaintext (not base64), plus the negative case (real base64
// content that happens to not decode to a link, which must still go
// through the base64 path).
// TestEnforcePackageQuota_PaymentModeIgnoresStaleTotalVolumeBytes is the
// regression test for a confirmed, reported production incident: reseller
// "Sha" (Payment-based/payment-mode) complained their V2Ray configs kept
// auto-disabling themselves. Root cause: enforcePackageQuota's own
// TotalVolumeBytes check had no BillingMode guard (applyResellerV2RayQuota,
// called from this same function just above, already had the equivalent
// guard for the reseller-wide pool) -- so a Payment-based reseller's
// package, still carrying a leftover TotalVolumeBytes figure from before
// they were switched to Payment billing (never cleared, by design, so
// switching back to Volume-based needs no migration), kept getting
// suspended by that stale byte ceiling on every sync tick even though
// ChargeUsage (the actual Payment-mode enforcement path) had nothing to do
// with it and the reseller was fully solvent.
func TestEnforcePackageQuota_PaymentModeIgnoresStaleTotalVolumeBytes(t *testing.T) {
	sync, db := newTestV2RaySyncService(t)

	reseller := model.Reseller{
		Name:            "sha-payment-reseller",
		BillingMode:     model.ResellerBillingModePayment,
		BillingSuspended: false,
	}
	if err := db.Create(&reseller).Error; err != nil {
		t.Fatalf("failed to create reseller: %v", err)
	}

	pkg := model.V2RayPackage{
		ResellerID:       &reseller.ID,
		Status:           "active",
		TotalVolumeBytes: 10 * 1024 * 1024 * 1024, // stale 10GB ceiling from before Payment mode
	}
	if err := db.Create(&pkg).Error; err != nil {
		t.Fatalf("failed to create package: %v", err)
	}

	// Usage well past the stale TotalVolumeBytes ceiling -- under the old
	// (buggy) logic this alone would suspend the package for a Payment-mode
	// reseller; it must now be ignored entirely.
	loc := model.V2RayPackageLocation{
		PackageID:       pkg.ID,
		UsedBytesCached: 50 * 1024 * 1024 * 1024,
	}
	if err := db.Create(&loc).Error; err != nil {
		t.Fatalf("failed to create location: %v", err)
	}

	sync.enforcePackageQuota(t.Context(), pkg.ID)

	var got model.V2RayPackage
	if err := db.First(&got, pkg.ID).Error; err != nil {
		t.Fatalf("failed to reload package: %v", err)
	}
	if got.SuspendedByQuota {
		t.Fatalf("expected Payment-mode package to NOT be suspended by the stale TotalVolumeBytes ceiling, but SuspendedByQuota=true")
	}
	if got.Status != "active" {
		t.Fatalf("expected package to remain active, got status=%q", got.Status)
	}
}

// TestEnforcePackageQuota_PaymentModeSuspendsOnBillingSuspended confirms
// the OTHER half of the same branch still works: a Payment-mode reseller
// whose wallet debit was rejected (BillingSuspended=true) must still have
// this package suspended, via the BillingSuspended surface rather than
// TotalVolumeBytes.
func TestEnforcePackageQuota_PaymentModeSuspendsOnBillingSuspended(t *testing.T) {
	sync, db := newTestV2RaySyncService(t)

	reseller := model.Reseller{
		Name:             "sha-suspended-reseller",
		BillingMode:      model.ResellerBillingModePayment,
		BillingSuspended: true,
	}
	if err := db.Create(&reseller).Error; err != nil {
		t.Fatalf("failed to create reseller: %v", err)
	}

	pkg := model.V2RayPackage{
		ResellerID: &reseller.ID,
		Status:     "active",
		// No TotalVolumeBytes at all -- suspension here must come purely
		// from BillingSuspended, proving the two signals are independent.
	}
	if err := db.Create(&pkg).Error; err != nil {
		t.Fatalf("failed to create package: %v", err)
	}

	sync.enforcePackageQuota(t.Context(), pkg.ID)

	var got model.V2RayPackage
	if err := db.First(&got, pkg.ID).Error; err != nil {
		t.Fatalf("failed to reload package: %v", err)
	}
	if !got.SuspendedByQuota {
		t.Fatalf("expected package to be suspended due to reseller.BillingSuspended, but SuspendedByQuota=false")
	}
	if got.Status != "suspended" {
		t.Fatalf("expected status=suspended, got %q", got.Status)
	}
}

func TestIsRawLinkContent(t *testing.T) {
	cases := []struct {
		name  string
		input string
		want  bool
	}{
		{"vless", "vless://uuid@host:443#title", true},
		{"vmess", "vmess://eyJ2IjoiMiJ9", true},
		{"trojan", "trojan://password@host:443#title", true},
		{"shadowsocks", "ss://base64stuff@host:443#title", true},
		{"multiline starts with vless", "vless://a@host:443#x\nvmess://b", true},
		{"leading whitespace before scheme", "  vless://uuid@host:443#title", true},
		{"base64 content", base64.StdEncoding.EncodeToString([]byte("vless://uuid@host:443#title")), false},
		{"empty", "", false},
	}

	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			got := isRawLinkContent(strings.TrimSpace(tc.input))
			if got != tc.want {
				t.Fatalf("isRawLinkContent(%q) = %v, want %v", tc.input, got, tc.want)
			}
		})
	}
}
