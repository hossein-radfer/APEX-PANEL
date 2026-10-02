package service

import (
	"encoding/base64"
	"net/http"
	"net/http/httptest"
	"net/url"
	"strings"
	"testing"

	"github.com/maahdima/mwp/api/dataservice/model"
)

// This file exercises the FIX for a live, reproduced production bug: a
// real customer's V2Ray subscription URL (GET /api/v2ray-sub/{uuid}) served
// a blank page and failed in real V2Ray client apps (v2rayNG, v2box)
// because their config link's title was permanently stuck at the raw x-ui
// client email (e.g. "pkg_c2a63fc7_1") instead of the resolved sale title.
//
// Root cause, confirmed with two separate httptest servers standing in for
// a real x-ui deployment's two genuinely different servers (admin panel API
// vs. public subscription endpoint, on different ports): xui.GetSubscription
// used to call Login(ctx, panel) first and reuse that authenticated
// *Client's cookie-jar-bearing httpClient to fetch the subscription URL.
// Go's net/http/cookiejar (RFC 6265) scopes cookies by HOST only, not port,
// so when api_base_url and sub_base_url shared a hostname on different
// ports (a very common real x-ui layout, e.g. host:2053 admin / host:2096
// sub), the admin session cookie leaked onto every subscription request.
// A subscription-side reverse proxy/WAF that rejects unrecognized foreign
// cookies then made GetSubscription fail on every single sync tick,
// forever -- and since syncOneLocation treats that failure as non-fatal and
// explicitly preserves whatever ConfigLinkCached already held, the broken
// raw-email-titled link (x-ui's own default subscription content, which
// titles links with the client's `email` field) never got corrected.
//
// The fix: xui.GetSubscription no longer calls Login at all -- it issues a
// plain, unauthenticated GET via its own client with no cookie jar,
// matching both how a real x-ui subscription server is meant to be used and
// how every real V2Ray client app fetches a subscription URL.
func TestLiveRepro_SubscriptionFetch_NoAuthedCookieLeak(t *testing.T) {
	var subServerReceivedCookie string
	var subServerHit bool

	// ---- Stub 1: admin panel API (separate origin, sets a session cookie
	// on login, requires it on every other admin-panel call). ----
	adminMux := http.NewServeMux()
	adminMux.HandleFunc("/login", func(w http.ResponseWriter, r *http.Request) {
		http.SetCookie(w, &http.Cookie{Name: "3x-ui", Value: "admin-session-abc123", Path: "/"})
		w.Header().Set("Content-Type", "application/json")
		w.Write([]byte(`{"success":true,"msg":"ok"}`))
	})
	adminMux.HandleFunc("/xui/API/inbounds/getClientTraffics/pkg_client", func(w http.ResponseWriter, r *http.Request) {
		if _, err := r.Cookie("3x-ui"); err != nil {
			t.Errorf("getClientTraffics: expected session cookie on admin panel call, got none")
		}
		w.Header().Set("Content-Type", "application/json")
		w.Write([]byte(`{"success":true,"msg":"","obj":{"up":1000,"down":2000,"total":0}}`))
	})
	adminServer := httptest.NewServer(adminMux)
	defer adminServer.Close()

	// ---- Stub 2: the PUBLIC subscription server -- a STRICT stand-in for a
	// real hardened deployment: rejects any request carrying a foreign
	// cookie it doesn't recognize (401), exactly the behavior that
	// permanently broke GetSubscription before the fix. Also records
	// whether it ever received the admin cookie at all, so the test can
	// assert on the leak directly, not just its downstream effect.
	subMux := http.NewServeMux()
	subMux.HandleFunc("/sub/real-sub-id", func(w http.ResponseWriter, r *http.Request) {
		subServerHit = true
		if c, err := r.Cookie("3x-ui"); err == nil {
			subServerReceivedCookie = c.Value
			w.WriteHeader(http.StatusUnauthorized)
			w.Write([]byte("unauthorized: unexpected cookie"))
			return
		}
		// Realistic x-ui subscription body: base64 of the link, titled with
		// the client's raw email by x-ui's OWN default remark behavior --
		// MWPanel's rewrite step is responsible for replacing this.
		raw := "vless://cf26e28e-1180-400b-b282-5a25b92359ca@db.horanet.ir:27399?type=tcp&encryption=none&security=none#pkg_c2a63fc7_1"
		w.Write([]byte(base64.StdEncoding.EncodeToString([]byte(raw))))
	})
	subServer := httptest.NewServer(subMux)
	defer subServer.Close()

	// Register a panel whose api_base_url and sub_base_url point at the TWO
	// DIFFERENT stub servers/ports, exactly like a real x-ui deployment.
	sync, db := newTestV2RaySyncService(t)

	panel := model.XuiPanel{
		Name: "prod-like-panel", SaleTitle: "Premium VPN - Germany",
		APIBaseURL: adminServer.URL, Username: "admin", Password: "admin",
		DefaultInboundID: 1, Protocol: "vless",
		SubBaseURL: subServer.URL + "/sub", Status: "active",
	}
	if err := db.Create(&panel).Error; err != nil {
		t.Fatalf("failed to create panel: %v", err)
	}

	pkg := model.V2RayPackage{UUID: "live-repro-uuid", TotalVolumeBytes: 0, DurationDays: 30, Status: "active"}
	if err := db.Create(&pkg).Error; err != nil {
		t.Fatalf("failed to create package: %v", err)
	}

	// Location's ConfigLinkCached is PRE-SEEDED with the raw email-titled
	// link, simulating exactly the stuck state the user reported.
	staleRawLink := "vless://cf26e28e-1180-400b-b282-5a25b92359ca@db.horanet.ir:27399?type=tcp&encryption=none&security=none#pkg_c2a63fc7_1"
	loc := model.V2RayPackageLocation{
		PackageID: pkg.ID, PanelID: panel.ID,
		ClientUUID:       "cf26e28e-1180-400b-b282-5a25b92359ca",
		ClientEmail:      "pkg_client",
		SubID:            "real-sub-id",
		Enabled:          true,
		ConfigLinkCached: staleRawLink,
	}
	if err := db.Create(&loc).Error; err != nil {
		t.Fatalf("failed to create location: %v", err)
	}

	// Run the REAL sync path, multiple ticks, exactly as SyncPackageUsage's
	// scheduled job does in production.
	for i := 0; i < 3; i++ {
		sync.SyncPackageUsage()
	}

	if !subServerHit {
		t.Fatal("subscription stub server was never hit -- GetSubscription failed before reaching it")
	}
	if subServerReceivedCookie != "" {
		t.Fatalf("FIX REGRESSION: subscription server received the admin panel's session cookie (%q) -- GetSubscription is still reusing an authenticated client against the separate subscription server", subServerReceivedCookie)
	}

	var reloadedLoc model.V2RayPackageLocation
	if err := db.First(&reloadedLoc, loc.ID).Error; err != nil {
		t.Fatalf("failed to reload location: %v", err)
	}

	t.Logf("LastSyncError: %v", reloadedLoc.LastSyncError)
	t.Logf("ConfigLinkCached: %q", reloadedLoc.ConfigLinkCached)

	if reloadedLoc.LastSyncError != nil {
		t.Fatalf("sync recorded an error: %s", *reloadedLoc.LastSyncError)
	}

	// The critical assertion: the previously-stuck raw-email title must now
	// be HEALED by a normal sync tick, not stuck forever.
	if strings.Contains(reloadedLoc.ConfigLinkCached, "pkg_c2a63fc7_1") || strings.Contains(reloadedLoc.ConfigLinkCached, "pkg_client") {
		t.Fatalf("BUG STILL PRESENT: ConfigLinkCached still contains the raw client email as title instead of the resolved sale title %q -- got %q", panel.SaleTitle, reloadedLoc.ConfigLinkCached)
	}
	if !strings.Contains(reloadedLoc.ConfigLinkCached, url.QueryEscape(panel.SaleTitle)) && !strings.Contains(reloadedLoc.ConfigLinkCached, strings.ReplaceAll(panel.SaleTitle, " ", "%20")) {
		t.Fatalf("expected ConfigLinkCached to contain the url-encoded sale title, got %q", reloadedLoc.ConfigLinkCached)
	}

	// Confirm the fix all the way through to what /api/v2ray-sub/{uuid}
	// actually serves: CombinedConfigCached must be valid base64 containing
	// the corrected, non-empty content.
	var reloadedPkg model.V2RayPackage
	if err := db.First(&reloadedPkg, pkg.ID).Error; err != nil {
		t.Fatalf("failed to reload package: %v", err)
	}
	if reloadedPkg.CombinedConfigCached == "" {
		t.Fatal("expected CombinedConfigCached to be populated")
	}

	decoded, err := base64.StdEncoding.DecodeString(reloadedPkg.CombinedConfigCached)
	if err != nil {
		t.Fatalf("CombinedConfigCached is not valid base64: %v", err)
	}
	t.Logf("CombinedConfigCached (decoded): %q", string(decoded))

	if strings.Contains(string(decoded), "pkg_c2a63fc7_1") {
		t.Fatalf("combined subscription content still contains the raw client email title: %q", string(decoded))
	}
	if !strings.Contains(string(decoded), "vless://cf26e28e-1180-400b-b282-5a25b92359ca@db.horanet.ir:27399") {
		t.Fatalf("expected the combined config to contain the real client link, got %q", string(decoded))
	}
}

// TestLiveRepro_SubscriptionFetch_WorksAcrossDifferentPortsNoLogin confirms
// GetSubscription succeeds against a subscription server that ONLY exists
// on a different port than the admin API and NEVER implements /login at
// all -- i.e. proves the fetch genuinely requires no authentication step
// whatsoever, matching a real x-ui deployment's public subscription server.
func TestLiveRepro_SubscriptionFetch_WorksAcrossDifferentPortsNoLogin(t *testing.T) {
	adminMux := http.NewServeMux()
	adminMux.HandleFunc("/login", func(w http.ResponseWriter, r *http.Request) {
		http.SetCookie(w, &http.Cookie{Name: "3x-ui", Value: "admin-session-abc123", Path: "/"})
		w.Header().Set("Content-Type", "application/json")
		w.Write([]byte(`{"success":true,"msg":"ok"}`))
	})
	adminMux.HandleFunc("/xui/API/inbounds/getClientTraffics/pkg_client", func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		w.Write([]byte(`{"success":true,"msg":"","obj":{"up":0,"down":0,"total":0}}`))
	})
	adminServer := httptest.NewServer(adminMux)
	defer adminServer.Close()

	// This mux has NO /login handler at all -- if GetSubscription ever
	// tried to log in against it (it shouldn't -- it's a different server
	// entirely in production), or if it needed any cookie, this would fail.
	subMux := http.NewServeMux()
	subMux.HandleFunc("/sub/real-sub-id", func(w http.ResponseWriter, r *http.Request) {
		if len(r.Cookies()) != 0 {
			t.Errorf("expected no cookies on subscription request, got %v", r.Cookies())
		}
		raw := "vless://cf26e28e-1180-400b-b282-5a25b92359ca@db.horanet.ir:27399?type=tcp#pkg_c2a63fc7_1"
		w.Write([]byte(base64.StdEncoding.EncodeToString([]byte(raw))))
	})
	subServer := httptest.NewServer(subMux)
	defer subServer.Close()

	sync, db := newTestV2RaySyncService(t)

	panel := model.XuiPanel{
		Name: "no-login-sub-panel", SaleTitle: "Premium VPN - Germany",
		APIBaseURL: adminServer.URL, Username: "admin", Password: "admin",
		DefaultInboundID: 1, Protocol: "vless",
		SubBaseURL: subServer.URL + "/sub", Status: "active",
	}
	if err := db.Create(&panel).Error; err != nil {
		t.Fatalf("failed to create panel: %v", err)
	}

	pkg := model.V2RayPackage{UUID: "live-repro-uuid-4", TotalVolumeBytes: 0, DurationDays: 30, Status: "active"}
	if err := db.Create(&pkg).Error; err != nil {
		t.Fatalf("failed to create package: %v", err)
	}

	loc := model.V2RayPackageLocation{
		PackageID: pkg.ID, PanelID: panel.ID,
		ClientUUID: "cf26e28e-1180-400b-b282-5a25b92359ca", ClientEmail: "pkg_client",
		SubID: "real-sub-id", Enabled: true,
	}
	if err := db.Create(&loc).Error; err != nil {
		t.Fatalf("failed to create location: %v", err)
	}

	sync.SyncPackageUsage()

	var reloadedLoc model.V2RayPackageLocation
	if err := db.First(&reloadedLoc, loc.ID).Error; err != nil {
		t.Fatalf("failed to reload location: %v", err)
	}
	if reloadedLoc.LastSyncError != nil {
		t.Fatalf("sync recorded an error: %s", *reloadedLoc.LastSyncError)
	}
	if reloadedLoc.ConfigLinkCached == "" {
		t.Fatal("expected ConfigLinkCached to be populated by an unauthenticated subscription fetch")
	}
	if !strings.Contains(reloadedLoc.ConfigLinkCached, url.QueryEscape(panel.SaleTitle)) && !strings.Contains(reloadedLoc.ConfigLinkCached, strings.ReplaceAll(panel.SaleTitle, " ", "%20")) {
		t.Fatalf("expected sale title in rewritten link, got %q", reloadedLoc.ConfigLinkCached)
	}
}
