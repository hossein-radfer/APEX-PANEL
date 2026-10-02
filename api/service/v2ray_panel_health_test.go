package service

import (
	"encoding/json"
	"fmt"
	"net"
	"net/http"
	"net/http/httptest"
	"net/url"
	"strconv"
	"testing"
	"time"

	"github.com/glebarez/sqlite"
	"github.com/maahdima/mwp/api/adaptor/mikrotik"
	"github.com/maahdima/mwp/api/common"
	"github.com/maahdima/mwp/api/dataservice/model"
	"github.com/maahdima/mwp/api/http/schema"
	"gorm.io/gorm"
)

// newTestV2RayPanelHealthService mirrors newTestV2RaySyncService's own
// uniquely-named in-memory DB convention.
func newTestV2RayPanelHealthService(t *testing.T) (*V2RayPanelHealthService, *gorm.DB) {
	t.Helper()

	dsn := fmt.Sprintf("file:v2ray_panel_health_test_%d?mode=memory&cache=shared", time.Now().UnixNano())
	db, err := gorm.Open(sqlite.Open(dsn), &gorm.Config{})
	if err != nil {
		t.Fatalf("failed to open test db: %v", err)
	}

	if err := db.AutoMigrate(&model.XuiPanel{}, &model.Server{}); err != nil {
		t.Fatalf("failed to migrate test db: %v", err)
	}

	return NewV2RayPanelHealthService(db), db
}

// registerFakeRouterOSServer registers a "test-router" named mikrotik
// client pointed at a local httptest server, and creates the matching
// model.Server row (diagnoseContainer looks the server up by ID to resolve
// its Name, mirroring MwpClients' own name-based client registry) --
// mirrors container_test.go's newTestAdaptorWithFakeRouter, adapted to
// also create the DB-side model.Server row this service needs.
func registerFakeRouterOSServer(t *testing.T, db *gorm.DB, handler http.HandlerFunc) (*mikrotik.Adaptor, model.Server) {
	t.Helper()
	testServer := httptest.NewServer(handler)
	t.Cleanup(testServer.Close)

	parsed, err := url.Parse(testServer.URL)
	if err != nil {
		t.Fatalf("failed to parse test server URL: %v", err)
	}
	_, port, err := net.SplitHostPort(parsed.Host)
	if err != nil {
		t.Fatalf("failed to split host/port: %v", err)
	}
	portNum, err := strconv.Atoi(port)
	if err != nil {
		t.Fatalf("failed to parse port: %v", err)
	}

	server := model.Server{Name: "test-router", IPAddress: "127.0.0.1", APIPort: portNum, Username: "admin", Password: "admin"}
	if err := db.Create(&server).Error; err != nil {
		t.Fatalf("failed to create server row: %v", err)
	}

	mwpClients := common.NewMwpClients(db)
	isSSL := false
	mwpClients.SetClient(&schema.CreateServerRequest{
		Name:      server.Name,
		IPAddress: server.IPAddress,
		APIPort:   port,
		IsSSL:     &isSSL,
		Username:  server.Username,
		Password:  server.Password,
	})

	return mikrotik.NewAdaptor(mwpClients), server
}

func TestDiagnoseContainer_ReportsStoppedWhenContainerIsStopped(t *testing.T) {
	svc, db := newTestV2RayPanelHealthService(t)

	adaptor, server := registerFakeRouterOSServer(t, db, func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		_ = json.NewEncoder(w).Encode([]mikrotik.Container{
			{Name: "xui-panel-a", Status: "stopped"},
		})
	})
	svc.SetMikrotikAdaptor(adaptor)

	containerName := "xui-panel-a"
	panel := model.XuiPanel{
		Name: "Panel A", SaleTitle: "Panel A", APIBaseURL: "http://127.0.0.1:1", Username: "u", Password: "p",
		DefaultInboundID: 1, Protocol: "vless", SubBaseURL: "http://127.0.0.1:1/sub",
		ContainerServerID: &server.ID, ContainerName: &containerName,
	}
	if err := db.Create(&panel).Error; err != nil {
		t.Fatalf("failed to create panel: %v", err)
	}

	stopped, status, serverName := svc.diagnoseContainer(panel)
	if !stopped {
		t.Fatal("expected diagnoseContainer to report stopped=true")
	}
	if status != "stopped" {
		t.Errorf("expected status 'stopped', got %q", status)
	}
	if serverName != "test-router" {
		t.Errorf("expected server name 'test-router', got %q", serverName)
	}
}

func TestDiagnoseContainer_FalseWhenContainerIsRunning(t *testing.T) {
	svc, db := newTestV2RayPanelHealthService(t)

	adaptor, server := registerFakeRouterOSServer(t, db, func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		_ = json.NewEncoder(w).Encode([]mikrotik.Container{
			{Name: "xui-panel-b", Status: "running"},
		})
	})
	svc.SetMikrotikAdaptor(adaptor)

	containerName := "xui-panel-b"
	panel := model.XuiPanel{
		Name: "Panel B", SaleTitle: "Panel B", APIBaseURL: "http://127.0.0.1:1", Username: "u", Password: "p",
		DefaultInboundID: 1, Protocol: "vless", SubBaseURL: "http://127.0.0.1:1/sub",
		ContainerServerID: &server.ID, ContainerName: &containerName,
	}
	if err := db.Create(&panel).Error; err != nil {
		t.Fatalf("failed to create panel: %v", err)
	}

	stopped, _, _ := svc.diagnoseContainer(panel)
	if stopped {
		t.Fatal("expected diagnoseContainer to report stopped=false when the container is actually running -- the panel being unreachable here must have a different cause")
	}
}

// TestDiagnoseContainer_NoMappingConfiguredFallsBackToFalse confirms the
// backward-compatibility guarantee: a panel created before item 9 existed
// (ContainerServerID/ContainerName both nil) must never attempt a
// container lookup at all, exactly matching its behavior before this
// feature was added.
func TestDiagnoseContainer_NoMappingConfiguredFallsBackToFalse(t *testing.T) {
	svc, db := newTestV2RayPanelHealthService(t)

	// Deliberately do NOT call SetMikrotikAdaptor -- a panel with no
	// mapping must not need one.
	panel := model.XuiPanel{
		Name: "Panel C (no mapping)", SaleTitle: "Panel C", APIBaseURL: "http://127.0.0.1:1", Username: "u", Password: "p",
		DefaultInboundID: 1, Protocol: "vless", SubBaseURL: "http://127.0.0.1:1/sub",
	}
	if err := db.Create(&panel).Error; err != nil {
		t.Fatalf("failed to create panel: %v", err)
	}

	stopped, status, serverName := svc.diagnoseContainer(panel)
	if stopped || status != "" || serverName != "" {
		t.Errorf("expected a no-op (false, \"\", \"\") for a panel with no container mapping, got (%v, %q, %q)", stopped, status, serverName)
	}
}

// TestDiagnoseContainer_NoAdaptorWiredFallsBackToFalse confirms
// SetMikrotikAdaptor is genuinely optional -- a service that never had it
// called (e.g. a lighter-weight test/embedding elsewhere) must not panic
// on a nil adaptor.
func TestDiagnoseContainer_NoAdaptorWiredFallsBackToFalse(t *testing.T) {
	svc, db := newTestV2RayPanelHealthService(t)

	server := model.Server{Name: "unused-router", IPAddress: "127.0.0.1", APIPort: 80, Username: "admin", Password: "admin"}
	if err := db.Create(&server).Error; err != nil {
		t.Fatalf("failed to create server: %v", err)
	}

	containerName := "xui-panel-d"
	panel := model.XuiPanel{
		Name: "Panel D", SaleTitle: "Panel D", APIBaseURL: "http://127.0.0.1:1", Username: "u", Password: "p",
		DefaultInboundID: 1, Protocol: "vless", SubBaseURL: "http://127.0.0.1:1/sub",
		ContainerServerID: &server.ID, ContainerName: &containerName,
	}
	if err := db.Create(&panel).Error; err != nil {
		t.Fatalf("failed to create panel: %v", err)
	}

	stopped, _, _ := svc.diagnoseContainer(panel)
	if stopped {
		t.Fatal("expected false when no mikrotik adaptor has been wired up")
	}
}

// TestDiagnoseContainer_ContainerNotFoundFallsBackToFalse confirms a
// container that's been renamed/removed on the router (a genuine
// misconfiguration between our mapping and the router's real state)
// degrades to the generic "unreachable" alert rather than crashing or
// falsely reporting "stopped."
func TestDiagnoseContainer_ContainerNotFoundFallsBackToFalse(t *testing.T) {
	svc, db := newTestV2RayPanelHealthService(t)

	adaptor, server := registerFakeRouterOSServer(t, db, func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		_ = json.NewEncoder(w).Encode([]mikrotik.Container{
			{Name: "some-other-container", Status: "running"},
		})
	})
	svc.SetMikrotikAdaptor(adaptor)

	containerName := "xui-panel-missing"
	panel := model.XuiPanel{
		Name: "Panel E", SaleTitle: "Panel E", APIBaseURL: "http://127.0.0.1:1", Username: "u", Password: "p",
		DefaultInboundID: 1, Protocol: "vless", SubBaseURL: "http://127.0.0.1:1/sub",
		ContainerServerID: &server.ID, ContainerName: &containerName,
	}
	if err := db.Create(&panel).Error; err != nil {
		t.Fatalf("failed to create panel: %v", err)
	}

	stopped, _, _ := svc.diagnoseContainer(panel)
	if stopped {
		t.Fatal("expected false when the mapped container name doesn't exist on the router")
	}
}
