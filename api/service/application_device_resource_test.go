package service

import (
	"fmt"
	"testing"
	"time"

	"github.com/glebarez/sqlite"
	"go.uber.org/zap"
	"gorm.io/gorm"

	"github.com/maahdima/mwp/api/dataservice/model"
)

// newTestApplicationServiceForDeviceResource builds an ApplicationService
// with only the DB dependency wired -- ReportDeviceResource/
// isResourceOnline/ListDeviceSessions never touch peers/userMgr/v2ray/
// openVpnTmpl/auditLog, so constructing the full NewApplicationService
// graph (which needs a live WgPeer/UserManagerService/V2RayPackageService)
// would be pure unrelated setup weight for this test.
func newTestApplicationServiceForDeviceResource(t *testing.T) (*ApplicationService, *gorm.DB) {
	t.Helper()

	dsn := fmt.Sprintf("file:app_device_resource_test_%d?mode=memory&cache=shared", time.Now().UnixNano())
	db, err := gorm.Open(sqlite.Open(dsn), &gorm.Config{})
	if err != nil {
		t.Fatalf("failed to open test db: %v", err)
	}

	if err := db.AutoMigrate(
		&model.Application{},
		&model.ApplicationInterface{},
		&model.ApplicationUserManagerGroup{},
		&model.ApplicationXuiPanel{},
		&model.ApplicationDeviceSession{},
		&model.IPConnectionLog{},
		&model.V2RayPackageLocation{},
	); err != nil {
		t.Fatalf("failed to migrate test db: %v", err)
	}

	return &ApplicationService{db: db, logger: zap.NewNop()}, db
}

// TestReportDeviceResource_WireGuard_ResolvesInterfaceIDToPeerID confirms
// the core resolution step: the app reports the connect-config ResourceID
// (InterfaceID), and ReportDeviceResource must store the underlying PeerID
// on the device session row -- NOT the InterfaceID itself -- since that's
// what IPConnectionLog.PeerID is actually keyed on.
func TestReportDeviceResource_WireGuard_ResolvesInterfaceIDToPeerID(t *testing.T) {
	svc, db := newTestApplicationServiceForDeviceResource(t)

	app := model.Application{Name: "app-1", AppUsername: "app-1"}
	if err := db.Create(&app).Error; err != nil {
		t.Fatalf("failed to create application: %v", err)
	}

	peerID := uint(42)
	link := model.ApplicationInterface{ApplicationID: app.ID, InterfaceID: 7, PeerID: &peerID}
	if err := db.Create(&link).Error; err != nil {
		t.Fatalf("failed to create application interface link: %v", err)
	}

	session := model.ApplicationDeviceSession{ApplicationID: app.ID, DeviceID: "device-1", FirstSeenAt: time.Now(), LastSeenAt: time.Now()}
	if err := db.Create(&session).Error; err != nil {
		t.Fatalf("failed to create device session: %v", err)
	}

	if err := svc.ReportDeviceResource(app.ID, "device-1", model.ResourceTypeWireGuardInterface, 7); err != nil {
		t.Fatalf("ReportDeviceResource failed: %v", err)
	}

	var updated model.ApplicationDeviceSession
	if err := db.Where("application_id = ? AND device_id = ?", app.ID, "device-1").First(&updated).Error; err != nil {
		t.Fatalf("failed to reload device session: %v", err)
	}
	if updated.ResourceType != model.ResourceTypeWireGuardInterface {
		t.Errorf("expected resource_type=%q, got %q", model.ResourceTypeWireGuardInterface, updated.ResourceType)
	}
	if updated.ResourceID == nil || *updated.ResourceID != peerID {
		t.Errorf("expected resource_id=%d (the underlying PeerID, not the InterfaceID), got %v", peerID, updated.ResourceID)
	}
}

// TestReportDeviceResource_UnknownResource_DoesNothing confirms a
// forged/stale resource_id that this Application does not actually own is
// silently ignored, never lets a device claim an arbitrary resource's
// online status as its own.
func TestReportDeviceResource_UnknownResource_DoesNothing(t *testing.T) {
	svc, db := newTestApplicationServiceForDeviceResource(t)

	app := model.Application{Name: "app-1", AppUsername: "app-1"}
	if err := db.Create(&app).Error; err != nil {
		t.Fatalf("failed to create application: %v", err)
	}
	session := model.ApplicationDeviceSession{ApplicationID: app.ID, DeviceID: "device-1", FirstSeenAt: time.Now(), LastSeenAt: time.Now()}
	if err := db.Create(&session).Error; err != nil {
		t.Fatalf("failed to create device session: %v", err)
	}

	if err := svc.ReportDeviceResource(app.ID, "device-1", model.ResourceTypeWireGuardInterface, 999); err != nil {
		t.Fatalf("ReportDeviceResource should not error on an unowned resource, got: %v", err)
	}

	var updated model.ApplicationDeviceSession
	if err := db.Where("application_id = ? AND device_id = ?", app.ID, "device-1").First(&updated).Error; err != nil {
		t.Fatalf("failed to reload device session: %v", err)
	}
	if updated.ResourceType != "" || updated.ResourceID != nil {
		t.Errorf("expected resource_type/resource_id to remain unset, got type=%q id=%v", updated.ResourceType, updated.ResourceID)
	}
}

// TestListDeviceSessions_IsOnlineReflectsLiveIPConnectionLog confirms
// GetOnlineCount's live-session signal now also drives the PER-DEVICE
// IsOnline flag: a device whose reported peer has an open (DisconnectedAt
// nil) IPConnectionLog row must read as online, and a device with no
// reported resource at all must read as offline (never a fabricated
// online).
func TestListDeviceSessions_IsOnlineReflectsLiveIPConnectionLog(t *testing.T) {
	svc, db := newTestApplicationServiceForDeviceResource(t)

	app := model.Application{Name: "app-1", AppUsername: "app-1"}
	if err := db.Create(&app).Error; err != nil {
		t.Fatalf("failed to create application: %v", err)
	}

	onlinePeerID := uint(10)
	offlinePeerID := uint(20)
	sessions := []model.ApplicationDeviceSession{
		{ApplicationID: app.ID, DeviceID: "device-online", ResourceType: model.ResourceTypeWireGuardInterface, ResourceID: &onlinePeerID, FirstSeenAt: time.Now(), LastSeenAt: time.Now()},
		{ApplicationID: app.ID, DeviceID: "device-offline", ResourceType: model.ResourceTypeWireGuardInterface, ResourceID: &offlinePeerID, FirstSeenAt: time.Now(), LastSeenAt: time.Now()},
		{ApplicationID: app.ID, DeviceID: "device-unreported", FirstSeenAt: time.Now(), LastSeenAt: time.Now()},
	}
	for i := range sessions {
		if err := db.Create(&sessions[i]).Error; err != nil {
			t.Fatalf("failed to create device session: %v", err)
		}
	}

	// Only the online peer has an open connection log row.
	if err := db.Create(&model.IPConnectionLog{Protocol: "wireguard", PeerID: &onlinePeerID}).Error; err != nil {
		t.Fatalf("failed to create ip connection log: %v", err)
	}
	closedAt := time.Now()
	if err := db.Create(&model.IPConnectionLog{Protocol: "wireguard", PeerID: &offlinePeerID, DisconnectedAt: &closedAt}).Error; err != nil {
		t.Fatalf("failed to create closed ip connection log: %v", err)
	}

	result, err := svc.ListDeviceSessions(app.ID, "")
	if err != nil {
		t.Fatalf("ListDeviceSessions failed: %v", err)
	}

	byDeviceID := map[string]bool{}
	for _, d := range result.Devices {
		byDeviceID[d.DeviceID] = d.IsOnline
	}

	if !byDeviceID["device-online"] {
		t.Error("expected device-online to be IsOnline=true (open IPConnectionLog row)")
	}
	if byDeviceID["device-offline"] {
		t.Error("expected device-offline to be IsOnline=false (its IPConnectionLog row is closed)")
	}
	if byDeviceID["device-unreported"] {
		t.Error("expected device-unreported to be IsOnline=false (no resource ever reported)")
	}
}
