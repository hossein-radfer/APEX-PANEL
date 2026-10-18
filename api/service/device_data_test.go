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
// opposed to no server being configured at all. Requires the caller to
// have already created exactly one model.Server row in db (MwpClients is
// keyed by server ID -- see its own doc comment -- so this looks that row
// up to register the client under its real ID).
func newTestDeviceDataServiceWithUnreachableRouter(t *testing.T, db *gorm.DB) *DeviceData {
	t.Helper()

	var server model.Server
	if err := db.First(&server).Error; err != nil {
		t.Fatalf("expected a model.Server row to already exist: %v", err)
	}

	unreachable := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusNotFound)
	}))
	t.Cleanup(unreachable.Close)

	mwpClients := common.NewMwpClients(db)
	isSSL := false
	addr := unreachable.Listener.Addr().String()
	host, port := addr[:len(addr)-len(":")-len(portOf(addr))], portOf(addr)
	mwpClients.SetClient(server.ID, &schema.CreateServerRequest{
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
// router-dependent sections (DeviceInfo/DeviceIdentity/DNSConfig) come back
// nil instead of aborting the whole response. PeerInfo is a partial
// exception since the multi-server rework of WgPeer.GetPeersData (see its
// own doc comment): its TotalPeers count is itself a pure DB read, so it
// stays populated even when every server's live online-status fetch fails;
// only the live portion (OnlinePeers) degrades.
func TestGetDeviceData_UnreachableRouterDoesNotBlankTheWholeDashboard(t *testing.T) {
	db := openDeviceDataTestDB(t)

	server := model.Server{Name: "server-1", IsActive: true}
	if err := db.Create(&server).Error; err != nil {
		t.Fatalf("failed to seed server: %v", err)
	}
	if err := db.Create(&model.Interface{ServerID: server.ID, InterfaceID: "wg0", Name: "wg0"}).Error; err != nil {
		t.Fatalf("failed to seed interface: %v", err)
	}
	// A peer row is required for GetPeersData's own live Mikrotik call to
	// actually happen -- it only fetches live state for servers that own
	// at least one DB peer row (see WgPeer.GetPeersData's own doc comment),
	// so PeerInfo's "degrades to nil on an unreachable router" behavior
	// this test verifies has nothing to exercise without one.
	if err := db.Create(&model.Peer{
		ServerID: server.ID, UUID: "peer-uuid-1", PeerID: "*1", Name: "peer-1",
		PrivateKey: "priv", PublicKey: "pub", Interface: "wg0",
		AllowedAddress: "10.0.0.2/32", Endpoint: "example.com", EndpointPort: "51820",
	}).Error; err != nil {
		t.Fatalf("failed to seed peer: %v", err)
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

	// PeerInfo, unlike DeviceInfo/DeviceIdentity/DNSConfig below, is now a
	// multi-server-safe aggregate (see WgPeer.GetPeersData's own doc
	// comment on fetching per-server rather than via one single-server
	// call) -- an unreachable server's own live fetch is skipped (logged,
	// not fatal) while every OTHER server's data (and the pure-DB total
	// peer count) is still returned, rather than the whole section going
	// nil the moment any one server is unreachable.
	if data.PeerInfo == nil {
		t.Error("expected PeerInfo to still be populated (TotalPeers is a pure DB read; only the live online-status portion degrades)")
	} else {
		if data.PeerInfo.TotalPeers != 1 {
			t.Errorf("expected 1 total peer from the DB read, got %d", data.PeerInfo.TotalPeers)
		}
		if data.PeerInfo.OnlinePeers != 0 {
			t.Errorf("expected 0 online peers since the only server is unreachable, got %d", data.PeerInfo.OnlinePeers)
		}
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
