package service

import (
	"fmt"
	"net/http"
	"net/http/httptest"
	"testing"
	"time"

	"github.com/glebarez/sqlite"
	"gorm.io/gorm"

	"github.com/maahdima/mwp/api/adaptor/mikrotik"
	"github.com/maahdima/mwp/api/common"
	"github.com/maahdima/mwp/api/dataservice/model"
	"github.com/maahdima/mwp/api/http/schema"
)

func openDeviceDataTestDB(t *testing.T) *gorm.DB {
	t.Helper()
	dsn := fmt.Sprintf("file:device_data_test_%d?mode=memory&cache=shared", time.Now().UnixNano())
	db, err := gorm.Open(sqlite.Open(dsn), &gorm.Config{})
	if err != nil {
		t.Fatalf("failed to open test db: %v", err)
	}
	if err := db.AutoMigrate(
		&model.Server{},
		&model.Interface{},
		&model.Peer{},
		&model.UserManagerAccount{},
		&model.TotalTrafficUsage{},
	); err != nil {
		t.Fatalf("failed to migrate test db: %v", err)
	}
	return db
}

// newTestDeviceDataServiceWithUnreachableRouter wires a DeviceData service
// whose mikrotikAdaptor points at a real (but always-erroring) test server
// -- mirrors newTestUserManagerServiceWithFakeRouter's own pattern
// (user_manager_reset_usage_test.go), simulating a Mikrotik router that is
// registered but unreachable/misconfigured (every request 404s), as
// opposed to no server being configured at all.
func newTestDeviceDataServiceWithUnreachableRouter(t *testing.T, db *gorm.DB) *DeviceData {
	t.Helper()

	unreachable := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusNotFound)
	}))
	t.Cleanup(unreachable.Close)

	mwpClients := common.NewMwpClients(db)
	isSSL := false
	addr := unreachable.Listener.Addr().String()
	host, port := addr[:len(addr)-len(":")-len(portOf(addr))], portOf(addr)
	mwpClients.SetClient(&schema.CreateServerRequest{
		Name:      "unreachable-test-server",
		IPAddress: host,
		APIPort:   port,
		IsSSL:     &isSSL,
		Username:  "admin",
		Password:  "admin",
	})
	adaptor := mikrotik.NewAdaptor(mwpClients)

	serverService := NewServerService(db, mwpClients, adaptor)
	interfaceService := NewWgInterface(db, adaptor)
	peerService := NewWGPeer(db, adaptor, nil, nil, nil, nil, nil)

	return NewDeviceData(db, adaptor, serverService, interfaceService, peerService)
}

// TestGetDeviceData_UnreachableRouterDoesNotBlankTheWholeDashboard is the
// regression test for a confirmed, reported production bug: the admin
// dashboard's stats cards (Total Servers/Interfaces/Users, Online Users,
// Total Traffic, plus Mikrotik Statistics/Hardware Statistics) all
// rendered completely blank -- not a loading skeleton, not an error
// message, just empty -- whenever ANY ONE of GetDeviceData's eight
// independent sub-fetches failed. Four of those eight make a LIVE network
// call to a Mikrotik router via mikrotikAdaptor, and on a multi-server
// deployment that adaptor's default client picks an ARBITRARY single
// server (Go map iteration order) -- so one unreachable/misconfigured
// router (completely unrelated to the OTHER configured servers, and to
// ServerInfo/InterfaceInfo/TrafficInfo, which are pure DB reads with no
// network dependency) blacked out the entire dashboard. Confirms the fix:
// GetDeviceData no longer returns an error at all just because the router
// is unreachable, and DB-only sections (ServerInfo/InterfaceInfo/
// TrafficInfo) are still populated correctly while the genuinely
// router-dependent sections (DeviceInfo/DeviceIdentity/DNSConfig/PeerInfo)
// come back nil instead of aborting the whole response.
func TestGetDeviceData_UnreachableRouterDoesNotBlankTheWholeDashboard(t *testing.T) {
	db := openDeviceDataTestDB(t)

	if err := db.Create(&model.Server{Name: "server-1", IsActive: true}).Error; err != nil {
		t.Fatalf("failed to seed server: %v", err)
	}
	if err := db.Create(&model.Interface{InterfaceID: "wg0", Name: "wg0"}).Error; err != nil {
		t.Fatalf("failed to seed interface: %v", err)
	}

	svc := newTestDeviceDataServiceWithUnreachableRouter(t, db)

	data, err := svc.GetDeviceData()
	if err != nil {
		t.Fatalf("expected GetDeviceData to never return an error just because the router is unreachable, got: %v", err)
	}
	if data == nil {
		t.Fatal("expected a non-nil response")
	}

	// DB-only sections must still be populated correctly -- they have
	// nothing to do with the unreachable router.
	if data.ServerInfo == nil {
		t.Error("expected ServerInfo to be populated (pure DB read, no network dependency)")
	} else if data.ServerInfo.TotalServers != 1 {
		t.Errorf("expected 1 total server, got %d", data.ServerInfo.TotalServers)
	}
	if data.InterfaceInfo == nil {
		t.Error("expected InterfaceInfo to be populated (pure DB read, no network dependency)")
	} else if data.InterfaceInfo.TotalInterfaces != 1 {
		t.Errorf("expected 1 total interface, got %d", data.InterfaceInfo.TotalInterfaces)
	}
	if data.TrafficInfo == nil {
		t.Error("expected TrafficInfo to be populated (pure DB read, no network dependency)")
	}

	// Router-dependent sections must degrade to nil, not abort everything.
	if data.PeerInfo != nil {
		t.Error("expected PeerInfo to be nil (its own fetch requires a live Mikrotik call, which fails here)")
	}
	if data.DeviceInfo != nil {
		t.Error("expected DeviceInfo to be nil (its own fetch requires a live Mikrotik call, which fails here)")
	}
	if data.DeviceIdentity != nil {
		t.Error("expected DeviceIdentity to be nil (its own fetch requires a live Mikrotik call, which fails here)")
	}
	if data.DNSConfig != nil {
		t.Error("expected DNSConfig to be nil (its own fetch requires a live Mikrotik call, which fails here)")
	}
}
