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
	"github.com/maahdima/mwp/api/dataservice/model"
)

// fakeRouterOSv3 is a minimal /rest-shaped test server covering exactly
// what attemptLevel3 touches: GET /ip/route (to discover which
// destinations the down interface was serving), PUT /interface/<type>
// (create the new tunnel interface), and PUT /ip/route (create the new
// route for each discovered destination). Kept as its own type (rather
// than extending fakeRouterOS from tunnel_health_level2_test.go) since
// Level 3's PUT-based create flow is a genuinely different request shape
// than Level 2's PATCH-based in-place switch, and conflating them would
// make either test harder to read than two small, focused fakes.
type fakeRouterOSv3 struct {
	mu sync.Mutex

	routes []mikrotik.RouteEntry

	createdInterfaces   []mikrotik.TunnelInterfaceEntry // every PUT /interface/<type> body received, in order
	createInterfacePath string                          // which /interface/<type> path was hit (e.g. "/rest/interface/gre")
	failInterfaceCreate bool

	createdRoutes  []mikrotik.RouteEntry // every PUT /ip/route body received, in order
	failRouteDsts  map[string]bool       // DstAddress -> force this route creation to fail
}

func newFakeRouterOSv3(routes []mikrotik.RouteEntry) *fakeRouterOSv3 {
	return &fakeRouterOSv3{
		routes:        routes,
		failRouteDsts: make(map[string]bool),
	}
}

func (f *fakeRouterOSv3) server() *httptest.Server {
	return httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		f.mu.Lock()
		defer f.mu.Unlock()

		switch {
		case r.Method == http.MethodGet && r.URL.Path == "/rest/ip/route":
			w.Header().Set("Content-Type", "application/json")
			_ = json.NewEncoder(w).Encode(f.routes)

		case r.Method == http.MethodPut && r.URL.Path == "/rest/ip/route":
			var body mikrotik.RouteEntry
			_ = json.NewDecoder(r.Body).Decode(&body)
			if f.failRouteDsts[body.DstAddress] {
				w.WriteHeader(http.StatusInternalServerError)
				_, _ = w.Write([]byte(`{"error":500,"message":"forced test failure"}`))
				return
			}
			body.ID = fmt.Sprintf("*r%d", len(f.createdRoutes)+1)
			f.createdRoutes = append(f.createdRoutes, body)
			w.Header().Set("Content-Type", "application/json")
			_ = json.NewEncoder(w).Encode(body)

		case r.Method == http.MethodPut &&
			(r.URL.Path == "/rest/interface/gre" || r.URL.Path == "/rest/interface/ipip" || r.URL.Path == "/rest/interface/eoip"):
			if f.failInterfaceCreate {
				w.WriteHeader(http.StatusInternalServerError)
				_, _ = w.Write([]byte(`{"error":500,"message":"forced test failure"}`))
				return
			}
			var body mikrotik.TunnelInterfaceEntry
			_ = json.NewDecoder(r.Body).Decode(&body)
			f.createInterfacePath = r.URL.Path
			body.ID = fmt.Sprintf("*i%d", len(f.createdInterfaces)+1)
			f.createdInterfaces = append(f.createdInterfaces, body)
			w.Header().Set("Content-Type", "application/json")
			_ = json.NewEncoder(w).Encode(body)

		default:
			w.WriteHeader(http.StatusNotFound)
		}
	}))
}

func openTunnelHealthLevel3TestDB(t *testing.T) *gorm.DB {
	t.Helper()
	dsn := fmt.Sprintf("file:tunnel_health_level3_test_%d?mode=memory&cache=shared", time.Now().UnixNano())
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
	); err != nil {
		t.Fatalf("failed to migrate test db: %v", err)
	}
	return db
}

// TestAttemptLevel3_CreatesInterfaceAndRoutesForMatchingDestinations is
// the core regression test for real Level 3 provisioning: only
// destinations actually reached through the down interface (per
// routeGatewayInterface's real-topology resolution, same rule as Level
// 2) get a new route created toward the new tunnel's GatewayIP, and the
// new tunnel interface itself is created with the configured type/
// remote-address.
func TestAttemptLevel3_CreatesInterfaceAndRoutesForMatchingDestinations(t *testing.T) {
	db := openTunnelHealthLevel3TestDB(t)

	routes := []mikrotik.RouteEntry{
		// Two destinations through the down interface -- MUST get new routes.
		{ID: "*1", DstAddress: "10.0.0.0/24", Gateway: strPtr("100.100.77.1"), ImmediateGw: strPtr("100.100.77.1%gre-TR")},
		{ID: "*2", DstAddress: "10.0.1.0/24", Gateway: strPtr("100.100.77.1"), ImmediateGw: strPtr("100.100.77.1%gre-TR")},
		// Through a different interface -- must NOT get a new route.
		{ID: "*3", DstAddress: "10.0.2.0/24", Gateway: strPtr("100.100.88.1"), ImmediateGw: strPtr("100.100.88.1%gre-EU")},
		// Disabled route through gre-TR -- must NOT get a new route.
		{ID: "*4", DstAddress: "10.0.3.0/24", Gateway: strPtr("100.100.77.1"), ImmediateGw: strPtr("100.100.77.1%gre-TR"), Disabled: strPtr("true")},
		// RouterOS's own auto-generated connected route -- must NOT get a new route.
		{ID: "*5", DstAddress: "100.100.77.0/30", Gateway: strPtr("gre-TR"), Connect: strPtr("true")},
	}
	fake := newFakeRouterOSv3(routes)
	srv := fake.server()
	defer srv.Close()

	svc := newTestTunnelHealthService(t, db, srv)
	if err := svc.SetDryRun(false); err != nil {
		t.Fatalf("failed to disable dry-run: %v", err)
	}

	var config TunnelPolicyConfig
	config.Level3.Enabled = true
	config.Level3.Type = "gre"
	config.Level3.RemoteAddress = "203.0.113.5"
	config.Level3.LocalAddress = "auto"
	config.Level3.GatewayIP = "203.0.113.5"
	config.Level3.NewInterfaceName = "gre-TR-new"

	svc.attemptLevel3("gre-TR", config, time.Now())

	fake.mu.Lock()
	defer fake.mu.Unlock()

	if got, want := fake.createInterfacePath, "/rest/interface/gre"; got != want {
		t.Errorf("expected interface created at %q, got %q", want, got)
	}
	if len(fake.createdInterfaces) != 1 {
		t.Fatalf("expected exactly 1 interface created, got %d", len(fake.createdInterfaces))
	}
	iface := fake.createdInterfaces[0]
	if iface.Name != "gre-TR-new" {
		t.Errorf("expected new interface name %q, got %q", "gre-TR-new", iface.Name)
	}
	if iface.RemoteAddr == nil || *iface.RemoteAddr != "203.0.113.5" {
		t.Errorf("expected remote-address 203.0.113.5, got %v", iface.RemoteAddr)
	}
	if iface.LocalAddr != nil {
		t.Errorf("expected local-address nil for \"auto\", got %v", *iface.LocalAddr)
	}

	if len(fake.createdRoutes) != 2 {
		t.Fatalf("expected exactly 2 routes created, got %d", len(fake.createdRoutes))
	}
	gotDsts := map[string]bool{}
	for _, r := range fake.createdRoutes {
		gotDsts[r.DstAddress] = true
		if r.Gateway == nil || *r.Gateway != "203.0.113.5" {
			t.Errorf("expected route to %s to have gateway 203.0.113.5, got %v", r.DstAddress, r.Gateway)
		}
	}
	if !gotDsts["10.0.0.0/24"] || !gotDsts["10.0.1.0/24"] {
		t.Errorf("expected routes created for 10.0.0.0/24 and 10.0.1.0/24, got %v", gotDsts)
	}
	if gotDsts["10.0.2.0/24"] || gotDsts["10.0.3.0/24"] || gotDsts["100.100.77.0/30"] {
		t.Errorf("expected no route created for unrelated/disabled/connect routes, got %v", gotDsts)
	}

	var action model.TunnelActionLog
	if err := db.Where("interface_name = ? AND level = ?", "gre-TR", "3").Order("id desc").First(&action).Error; err != nil {
		t.Fatalf("expected a TunnelActionLog row for the level 3 action: %v", err)
	}
	if action.Simulated {
		t.Errorf("expected a REAL (non-simulated) action since dry-run was disabled, got Simulated=true")
	}
}

// TestAttemptLevel3_DisabledStaysSimulatedAndTouchesNothing confirms the
// admin's own explicit config gate: Level3.Enabled=false must never
// create a real interface or route, regardless of what else is configured.
func TestAttemptLevel3_DisabledStaysSimulatedAndTouchesNothing(t *testing.T) {
	db := openTunnelHealthLevel3TestDB(t)

	routes := []mikrotik.RouteEntry{
		{ID: "*1", DstAddress: "10.0.0.0/24", Gateway: strPtr("100.100.77.1"), ImmediateGw: strPtr("100.100.77.1%gre-TR")},
	}
	fake := newFakeRouterOSv3(routes)
	srv := fake.server()
	defer srv.Close()

	svc := newTestTunnelHealthService(t, db, srv)
	if err := svc.SetDryRun(false); err != nil {
		t.Fatalf("failed to disable dry-run: %v", err)
	}

	var config TunnelPolicyConfig
	config.Level3.Enabled = false
	config.Level3.Type = "gre"
	config.Level3.RemoteAddress = "203.0.113.5"
	config.Level3.GatewayIP = "203.0.113.5"
	config.Level3.NewInterfaceName = "gre-TR-new"

	svc.attemptLevel3("gre-TR", config, time.Now())

	fake.mu.Lock()
	defer fake.mu.Unlock()
	if len(fake.createdInterfaces) != 0 || len(fake.createdRoutes) != 0 {
		t.Errorf("expected nothing created when Level3.Enabled=false, got %d interfaces, %d routes",
			len(fake.createdInterfaces), len(fake.createdRoutes))
	}

	var action model.TunnelActionLog
	if err := db.Where("interface_name = ? AND level = ?", "gre-TR", "3").Order("id desc").First(&action).Error; err != nil {
		t.Fatalf("expected a TunnelActionLog row: %v", err)
	}
	if !action.Simulated {
		t.Errorf("expected Simulated=true when Level3.Enabled=false, got false")
	}
}

// TestAttemptLevel3_MissingGatewayIPStaysSimulated confirms the same
// required-field safety gate as Level 2's own BackupGatewayIP check:
// even with Enabled/Type/RemoteAddress/NewInterfaceName all set, a
// missing GatewayIP must keep Level 3 simulation-only.
func TestAttemptLevel3_MissingGatewayIPStaysSimulated(t *testing.T) {
	db := openTunnelHealthLevel3TestDB(t)

	routes := []mikrotik.RouteEntry{
		{ID: "*1", DstAddress: "10.0.0.0/24", Gateway: strPtr("100.100.77.1"), ImmediateGw: strPtr("100.100.77.1%gre-TR")},
	}
	fake := newFakeRouterOSv3(routes)
	srv := fake.server()
	defer srv.Close()

	svc := newTestTunnelHealthService(t, db, srv)
	if err := svc.SetDryRun(false); err != nil {
		t.Fatalf("failed to disable dry-run: %v", err)
	}

	var config TunnelPolicyConfig
	config.Level3.Enabled = true
	config.Level3.Type = "gre"
	config.Level3.RemoteAddress = "203.0.113.5"
	config.Level3.NewInterfaceName = "gre-TR-new"
	// GatewayIP intentionally left empty.

	svc.attemptLevel3("gre-TR", config, time.Now())

	fake.mu.Lock()
	defer fake.mu.Unlock()
	if len(fake.createdInterfaces) != 0 || len(fake.createdRoutes) != 0 {
		t.Errorf("expected nothing created with no GatewayIP configured, got %d interfaces, %d routes",
			len(fake.createdInterfaces), len(fake.createdRoutes))
	}

	var action model.TunnelActionLog
	if err := db.Where("interface_name = ? AND level = ?", "gre-TR", "3").Order("id desc").First(&action).Error; err != nil {
		t.Fatalf("expected a TunnelActionLog row: %v", err)
	}
	if !action.Simulated {
		t.Errorf("expected Simulated=true when GatewayIP is unset, got false")
	}
}

// TestAttemptLevel3_UnsupportedTypeStaysSimulated confirms WireGuard (or
// any other type) is rejected -- see attemptLevel3's own doc comment on
// why WireGuard specifically cannot be safely auto-provisioned here.
func TestAttemptLevel3_UnsupportedTypeStaysSimulated(t *testing.T) {
	db := openTunnelHealthLevel3TestDB(t)

	fake := newFakeRouterOSv3(nil)
	srv := fake.server()
	defer srv.Close()

	svc := newTestTunnelHealthService(t, db, srv)
	if err := svc.SetDryRun(false); err != nil {
		t.Fatalf("failed to disable dry-run: %v", err)
	}

	var config TunnelPolicyConfig
	config.Level3.Enabled = true
	config.Level3.Type = "wireguard"
	config.Level3.RemoteAddress = "203.0.113.5"
	config.Level3.GatewayIP = "203.0.113.5"
	config.Level3.NewInterfaceName = "wg-new"

	svc.attemptLevel3("gre-TR", config, time.Now())

	fake.mu.Lock()
	defer fake.mu.Unlock()
	if len(fake.createdInterfaces) != 0 {
		t.Errorf("expected no interface created for unsupported type, got %d", len(fake.createdInterfaces))
	}

	var action model.TunnelActionLog
	if err := db.Where("interface_name = ? AND level = ?", "gre-TR", "3").Order("id desc").First(&action).Error; err != nil {
		t.Fatalf("expected a TunnelActionLog row: %v", err)
	}
	if !action.Simulated {
		t.Errorf("expected Simulated=true for unsupported tunnel type, got false")
	}
}

// TestAttemptLevel3_DryRunNeverTouchesRealRouter confirms dry-run mode
// (the default, safe-by-default state) computes and logs the intended
// action but never sends a real PUT, even with the full config set.
func TestAttemptLevel3_DryRunNeverTouchesRealRouter(t *testing.T) {
	db := openTunnelHealthLevel3TestDB(t)

	routes := []mikrotik.RouteEntry{
		{ID: "*1", DstAddress: "10.0.0.0/24", Gateway: strPtr("100.100.77.1"), ImmediateGw: strPtr("100.100.77.1%gre-TR")},
	}
	fake := newFakeRouterOSv3(routes)
	srv := fake.server()
	defer srv.Close()

	svc := newTestTunnelHealthService(t, db, srv)
	// Dry-run left at its default (true) -- deliberately NOT calling SetDryRun.

	var config TunnelPolicyConfig
	config.Level3.Enabled = true
	config.Level3.Type = "gre"
	config.Level3.RemoteAddress = "203.0.113.5"
	config.Level3.GatewayIP = "203.0.113.5"
	config.Level3.NewInterfaceName = "gre-TR-new"

	svc.attemptLevel3("gre-TR", config, time.Now())

	fake.mu.Lock()
	defer fake.mu.Unlock()
	if len(fake.createdInterfaces) != 0 || len(fake.createdRoutes) != 0 {
		t.Errorf("expected nothing created in dry-run mode, got %d interfaces, %d routes",
			len(fake.createdInterfaces), len(fake.createdRoutes))
	}

	var action model.TunnelActionLog
	if err := db.Where("interface_name = ? AND level = ?", "gre-TR", "3").Order("id desc").First(&action).Error; err != nil {
		t.Fatalf("expected a TunnelActionLog row: %v", err)
	}
	if !action.Simulated {
		t.Errorf("expected Simulated=true in dry-run mode, got false")
	}
}

// TestAttemptLevel3_PartialRouteFailureIsReportedNotSwallowed confirms
// that when the interface is created successfully but only some routes
// succeed, the outcome is logged as a real (non-simulated), non-silent
// partial failure rather than reported as a clean success.
func TestAttemptLevel3_PartialRouteFailureIsReportedNotSwallowed(t *testing.T) {
	db := openTunnelHealthLevel3TestDB(t)

	routes := []mikrotik.RouteEntry{
		{ID: "*1", DstAddress: "10.0.0.0/24", Gateway: strPtr("100.100.77.1"), ImmediateGw: strPtr("100.100.77.1%gre-TR")},
		{ID: "*2", DstAddress: "10.0.1.0/24", Gateway: strPtr("100.100.77.1"), ImmediateGw: strPtr("100.100.77.1%gre-TR")},
	}
	fake := newFakeRouterOSv3(routes)
	fake.failRouteDsts["10.0.1.0/24"] = true
	srv := fake.server()
	defer srv.Close()

	svc := newTestTunnelHealthService(t, db, srv)
	if err := svc.SetDryRun(false); err != nil {
		t.Fatalf("failed to disable dry-run: %v", err)
	}

	var config TunnelPolicyConfig
	config.Level3.Enabled = true
	config.Level3.Type = "gre"
	config.Level3.RemoteAddress = "203.0.113.5"
	config.Level3.GatewayIP = "203.0.113.5"
	config.Level3.NewInterfaceName = "gre-TR-new"

	svc.attemptLevel3("gre-TR", config, time.Now())

	fake.mu.Lock()
	defer fake.mu.Unlock()
	if len(fake.createdInterfaces) != 1 {
		t.Fatalf("expected the interface to still be created, got %d", len(fake.createdInterfaces))
	}
	if len(fake.createdRoutes) != 1 {
		t.Fatalf("expected exactly 1 of 2 routes to succeed, got %d", len(fake.createdRoutes))
	}

	var action model.TunnelActionLog
	if err := db.Where("interface_name = ? AND level = ?", "gre-TR", "3").Order("id desc").First(&action).Error; err != nil {
		t.Fatalf("expected a TunnelActionLog row: %v", err)
	}
	if action.Simulated {
		t.Errorf("a partial-failure real attempt must not be logged as Simulated=true")
	}
	if action.Result == "" {
		t.Errorf("expected a non-empty result describing the partial failure")
	}
}
