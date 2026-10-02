package service

import (
	"encoding/json"
	"fmt"
	"net/http"
	"net/http/httptest"
	"sync"
	"testing"
	"time"

	"github.com/glebarez/sqlite"
	"gorm.io/gorm"

	"github.com/maahdima/mwp/api/adaptor/mikrotik"
	"github.com/maahdima/mwp/api/common"
	"github.com/maahdima/mwp/api/dataservice/model"
	"github.com/maahdima/mwp/api/http/schema"
)

// fakeRouterOS is a minimal /rest-shaped test server covering exactly
// what attemptLevel2 touches: GET /ip/route (list) and PATCH /ip/route/
// <id> (gateway switch). Records every PATCH body received, keyed by
// route ID, so tests can assert precisely which routes were touched and
// with what value -- and, just as importantly, which routes were NOT
// touched (routes through an unrelated interface must survive untouched).
type fakeRouterOS struct {
	mu       sync.Mutex
	routes   []mikrotik.RouteEntry
	patches  map[string]string // route ID -> gateway value received
	patchErr map[string]bool   // route ID -> force this PATCH to fail
}

func newFakeRouterOS(routes []mikrotik.RouteEntry) *fakeRouterOS {
	return &fakeRouterOS{
		routes:   routes,
		patches:  make(map[string]string),
		patchErr: make(map[string]bool),
	}
}

func (f *fakeRouterOS) server() *httptest.Server {
	return httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		f.mu.Lock()
		defer f.mu.Unlock()

		switch {
		case r.Method == http.MethodGet && r.URL.Path == "/rest/ip/route":
			w.Header().Set("Content-Type", "application/json")
			_ = json.NewEncoder(w).Encode(f.routes)
		case r.Method == http.MethodPatch && len(r.URL.Path) > len("/rest/ip/route/"):
			routeID := r.URL.Path[len("/rest/ip/route/"):]
			if f.patchErr[routeID] {
				w.WriteHeader(http.StatusInternalServerError)
				_, _ = w.Write([]byte(`{"error":500,"message":"forced test failure"}`))
				return
			}
			var body mikrotik.RouteEntry
			_ = json.NewDecoder(r.Body).Decode(&body)
			if body.Gateway != nil {
				f.patches[routeID] = *body.Gateway
			}
			w.Header().Set("Content-Type", "application/json")
			_ = json.NewEncoder(w).Encode(mikrotik.RouteEntry{ID: routeID})
		default:
			w.WriteHeader(http.StatusNotFound)
		}
	}))
}

func openTunnelHealthTestDB(t *testing.T) *gorm.DB {
	t.Helper()
	// A unique DSN per test -- "file::memory:?cache=shared" alone is
	// shared across every gorm.Open call in the whole test binary (any
	// two tests using the bare DSN corrupt each other's SystemConfig/
	// TunnelActionLog rows), exactly the isolation bug db_test.go's own
	// tests avoid via a unique nanosecond-suffixed name per test.
	dsn := fmt.Sprintf("file:tunnel_health_level2_test_%d?mode=memory&cache=shared", time.Now().UnixNano())
	db, err := gorm.Open(sqlite.Open(dsn), &gorm.Config{})
	if err != nil {
		t.Fatalf("failed to open test db: %v", err)
	}
	if err := db.AutoMigrate(
		&model.SystemConfig{},
		&model.TunnelPolicy{},
		&model.TunnelActionLog{},
		&model.ManagementRedlineEntry{},
		&model.BotSettings{},
		&model.TunnelHealthStatus{},
		&model.TunnelHealthEvent{},
		&model.TunnelHealthScore{},
		&model.TunnelIncidentDiagnosis{},
		&model.TunnelBackupProbeResult{},
	); err != nil {
		t.Fatalf("failed to migrate test db: %v", err)
	}
	return db
}

func newTestTunnelHealthService(t *testing.T, db *gorm.DB, srv *httptest.Server) *TunnelHealthService {
	t.Helper()
	mwpClients := common.NewMwpClients(db)
	isSSL := false
	// httptest.Server URLs are "http://127.0.0.1:PORT" -- split host:port
	// back out since SetClient rebuilds the URL from IPAddress+APIPort.
	addr := srv.Listener.Addr().String() // "127.0.0.1:PORT"
	host, port := addr[:len(addr)-len(":")-len(portOf(addr))], portOf(addr)
	mwpClients.SetClient(&schema.CreateServerRequest{
		Name:      "test-server",
		IPAddress: host,
		APIPort:   port,
		IsSSL:     &isSSL,
		Username:  "admin",
		Password:  "admin",
	})
	adaptor := mikrotik.NewAdaptor(mwpClients)
	return NewTunnelHealthService(db, adaptor, nil)
}

func portOf(addr string) string {
	for i := len(addr) - 1; i >= 0; i-- {
		if addr[i] == ':' {
			return addr[i+1:]
		}
	}
	return ""
}

// TestAttemptLevel2_SwitchesOnlyMatchingRoutes is the core regression
// test for the admin's own explicit requirement (real Level 2 failover):
// among several routes, only the ones actually going out the DOWN
// interface (per routeGatewayInterface's real-topology resolution --
// confirmed on this fleet's actual routers, a route's gateway is a
// next-hop IP resolved via ImmediateGw, never the bare interface name)
// must be switched to the admin's own configured BackupGatewayIP. A
// route through an unrelated interface, a disabled route, and RouterOS's
// own auto-generated Connect="true" route must all survive completely
// untouched.
func TestAttemptLevel2_SwitchesOnlyMatchingRoutes(t *testing.T) {
	db := openTunnelHealthTestDB(t)

	routes := []mikrotik.RouteEntry{
		// Matches gre-TR via ImmediateGw -- MUST be switched.
		{ID: "*1", DstAddress: "10.0.0.0/24", Gateway: strPtr("100.100.77.1"), ImmediateGw: strPtr("100.100.77.1%gre-TR")},
		// A second route also through gre-TR -- MUST also be switched.
		{ID: "*2", DstAddress: "10.0.1.0/24", Gateway: strPtr("100.100.77.1"), ImmediateGw: strPtr("100.100.77.1%gre-TR")},
		// Through a DIFFERENT interface -- must NOT be touched.
		{ID: "*3", DstAddress: "10.0.2.0/24", Gateway: strPtr("100.100.88.1"), ImmediateGw: strPtr("100.100.88.1%gre-EU")},
		// Disabled route through gre-TR -- must NOT be touched (forwards no real traffic).
		{ID: "*4", DstAddress: "10.0.3.0/24", Gateway: strPtr("100.100.77.1"), ImmediateGw: strPtr("100.100.77.1%gre-TR"), Disabled: strPtr("true")},
		// RouterOS's own auto-generated connected route for gre-TR -- must NOT be touched.
		{ID: "*5", DstAddress: "100.100.77.0/30", Gateway: strPtr("gre-TR"), Connect: strPtr("true")},
	}
	fake := newFakeRouterOS(routes)
	srv := fake.server()
	defer srv.Close()

	svc := newTestTunnelHealthService(t, db, srv)
	if err := svc.SetDryRun(false); err != nil {
		t.Fatalf("failed to disable dry-run: %v", err)
	}

	var config TunnelPolicyConfig
	config.Level2.BackupTarget = "gre-EU"
	config.Level2.BackupGatewayIP = "100.100.88.1"

	svc.attemptLevel2("gre-TR", config, time.Now())

	fake.mu.Lock()
	defer fake.mu.Unlock()

	if got, want := fake.patches["*1"], "100.100.88.1"; got != want {
		t.Errorf("route *1 (through gre-TR): expected gateway switched to %q, got %q", want, got)
	}
	if got, want := fake.patches["*2"], "100.100.88.1"; got != want {
		t.Errorf("route *2 (through gre-TR): expected gateway switched to %q, got %q", want, got)
	}
	if _, touched := fake.patches["*3"]; touched {
		t.Errorf("route *3 (through unrelated gre-EU) must NOT have been touched, but it was")
	}
	if _, touched := fake.patches["*4"]; touched {
		t.Errorf("route *4 (disabled) must NOT have been touched, but it was")
	}
	if _, touched := fake.patches["*5"]; touched {
		t.Errorf("route *5 (Connect=true, auto-generated) must NOT have been touched, but it was")
	}

	// Confirm this was logged as a REAL (non-simulated) success.
	var action model.TunnelActionLog
	if err := db.Where("interface_name = ? AND level = ?", "gre-TR", "2").Order("id desc").First(&action).Error; err != nil {
		t.Fatalf("expected a TunnelActionLog row for the level 2 action: %v", err)
	}
	if action.Simulated {
		t.Errorf("expected a REAL (non-simulated) action since dry-run was disabled, got Simulated=true")
	}
}

// TestAttemptLevel2_MissingBackupGatewayIPStaysSimulated confirms the
// admin's own explicit safety requirement: even with BackupTarget set,
// Level 2 must stay simulation-only (never touch a real route) until
// BackupGatewayIP is also explicitly configured.
func TestAttemptLevel2_MissingBackupGatewayIPStaysSimulated(t *testing.T) {
	db := openTunnelHealthTestDB(t)

	routes := []mikrotik.RouteEntry{
		{ID: "*1", DstAddress: "10.0.0.0/24", Gateway: strPtr("100.100.77.1"), ImmediateGw: strPtr("100.100.77.1%gre-TR")},
	}
	fake := newFakeRouterOS(routes)
	srv := fake.server()
	defer srv.Close()

	svc := newTestTunnelHealthService(t, db, srv)
	if err := svc.SetDryRun(false); err != nil {
		t.Fatalf("failed to disable dry-run: %v", err)
	}

	var config TunnelPolicyConfig
	config.Level2.BackupTarget = "gre-EU"
	// BackupGatewayIP intentionally left empty.

	svc.attemptLevel2("gre-TR", config, time.Now())

	fake.mu.Lock()
	defer fake.mu.Unlock()
	if len(fake.patches) != 0 {
		t.Errorf("expected zero routes touched with no BackupGatewayIP configured, got %d", len(fake.patches))
	}

	var action model.TunnelActionLog
	if err := db.Where("interface_name = ? AND level = ?", "gre-TR", "2").Order("id desc").First(&action).Error; err != nil {
		t.Fatalf("expected a TunnelActionLog row: %v", err)
	}
	if !action.Simulated {
		t.Errorf("expected Simulated=true when BackupGatewayIP is unset, got false")
	}
}

// TestAttemptLevel2_DryRunNeverTouchesRealRoutes confirms dry-run mode
// (the default, safe-by-default state) computes and logs the intended
// action but never sends a real PATCH, even with both BackupTarget and
// BackupGatewayIP fully configured.
func TestAttemptLevel2_DryRunNeverTouchesRealRoutes(t *testing.T) {
	db := openTunnelHealthTestDB(t)

	routes := []mikrotik.RouteEntry{
		{ID: "*1", DstAddress: "10.0.0.0/24", Gateway: strPtr("100.100.77.1"), ImmediateGw: strPtr("100.100.77.1%gre-TR")},
	}
	fake := newFakeRouterOS(routes)
	srv := fake.server()
	defer srv.Close()

	svc := newTestTunnelHealthService(t, db, srv)
	// Dry-run left at its default (true) -- deliberately NOT calling SetDryRun.

	var config TunnelPolicyConfig
	config.Level2.BackupTarget = "gre-EU"
	config.Level2.BackupGatewayIP = "100.100.88.1"

	svc.attemptLevel2("gre-TR", config, time.Now())

	fake.mu.Lock()
	defer fake.mu.Unlock()
	if len(fake.patches) != 0 {
		t.Errorf("expected zero routes touched in dry-run mode, got %d", len(fake.patches))
	}

	var action model.TunnelActionLog
	if err := db.Where("interface_name = ? AND level = ?", "gre-TR", "2").Order("id desc").First(&action).Error; err != nil {
		t.Fatalf("expected a TunnelActionLog row: %v", err)
	}
	if !action.Simulated {
		t.Errorf("expected Simulated=true in dry-run mode, got false")
	}
}

// TestAttemptLevel2_FallsBackToLastKnownGatewayWhenImmediateGwBlank is the
// regression test for a confirmed, reported production incident: RouterOS
// blanks a route's own ImmediateGw the instant its gateway interface
// actually goes down (nothing left to resolve to), which makes
// routeGatewayInterface fall back to the route's bare Gateway field --
// just the next-hop IP with no "%interface" suffix -- so it can never
// match the down interface's own name. Before this fix, attemptLevel2
// found ZERO matching routes for a genuinely-down tunnel with a real,
// correctly-configured route still pointed at it, and logged "nothing to
// do" instead of failing traffic over. TunnelHealthStatus.LastKnownGatewayIP
// (populated on a prior healthy poll) is the fallback that must still
// match this route by its bare Gateway IP.
func TestAttemptLevel2_FallsBackToLastKnownGatewayWhenImmediateGwBlank(t *testing.T) {
	db := openTunnelHealthTestDB(t)

	// ImmediateGw is blank here -- exactly what a real down gre-TR looks
	// like over the REST API, confirmed against a live incident. Gateway
	// is still the bare next-hop IP, unchanged from when the tunnel was
	// healthy.
	routes := []mikrotik.RouteEntry{
		{ID: "*1", DstAddress: "0.0.0.0/0", Gateway: strPtr("100.100.77.1"), ImmediateGw: strPtr("")},
		// Unrelated route through a different, unrelated gateway IP -- must NOT be touched.
		{ID: "*2", DstAddress: "10.0.2.0/24", Gateway: strPtr("100.100.88.1"), ImmediateGw: strPtr("100.100.88.1%gre-EU")},
	}
	fake := newFakeRouterOS(routes)
	srv := fake.server()
	defer srv.Close()

	svc := newTestTunnelHealthService(t, db, srv)
	if err := svc.SetDryRun(false); err != nil {
		t.Fatalf("failed to disable dry-run: %v", err)
	}

	// Seed the cache as if a prior HEALTHY poll had already observed
	// ImmediateGw="100.100.77.1%gre-TR" for this interface.
	if err := db.Create(&model.TunnelHealthStatus{
		InterfaceName:      "gre-TR",
		Severity:           "confirmed_down",
		LastKnownGatewayIP: "100.100.77.1",
		LastPolledAt:       time.Now(),
	}).Error; err != nil {
		t.Fatalf("failed to seed tunnel health status: %v", err)
	}

	var config TunnelPolicyConfig
	config.Level2.BackupTarget = "gre-EU"
	config.Level2.BackupGatewayIP = "100.100.88.1"

	svc.attemptLevel2("gre-TR", config, time.Now())

	fake.mu.Lock()
	defer fake.mu.Unlock()

	if got, want := fake.patches["*1"], "100.100.88.1"; got != want {
		t.Errorf("route *1 (bare gateway matching LastKnownGatewayIP): expected gateway switched to %q, got %q", want, got)
	}
	if _, touched := fake.patches["*2"]; touched {
		t.Errorf("route *2 (unrelated gateway) must NOT have been touched, but it was")
	}

	var action model.TunnelActionLog
	if err := db.Where("interface_name = ? AND level = ?", "gre-TR", "2").Order("id desc").First(&action).Error; err != nil {
		t.Fatalf("expected a TunnelActionLog row: %v", err)
	}
	if action.Simulated {
		t.Errorf("expected a REAL (non-simulated) failover once the fallback match found the route, got Simulated=true")
	}
	if action.Result == "" || action.Result[:3] == "هیچ" {
		t.Errorf("expected a real success result, not the 'no matching routes found' message, got: %q", action.Result)
	}
}
