package service

import (
	"fmt"
	"testing"
	"time"

	"github.com/glebarez/sqlite"
	"go.uber.org/zap"
	"gorm.io/gorm"

	"github.com/maahdima/mwp/api/dataservice/model"
	"github.com/maahdima/mwp/api/http/schema"
)

// newTestApplicationServiceForPermission mirrors
// newTestApplicationServiceForDeviceResource -- the permission-gate methods
// under test (ensureResellerCanCreateApplications/
// ensureResellerUnderApplicationLimit) only touch the db field.
func newTestApplicationServiceForPermission(t *testing.T) (*ApplicationService, *gorm.DB) {
	t.Helper()

	dsn := fmt.Sprintf("file:app_permission_test_%d?mode=memory&cache=shared", time.Now().UnixNano())
	db, err := gorm.Open(sqlite.Open(dsn), &gorm.Config{})
	if err != nil {
		t.Fatalf("failed to open test db: %v", err)
	}

	if err := db.AutoMigrate(
		&model.Reseller{},
		&model.Application{},
	); err != nil {
		t.Fatalf("failed to migrate test db: %v", err)
	}

	return &ApplicationService{db: db, logger: zap.NewNop()}, db
}

// TestEnsureResellerCanCreateApplications verifies the permission gate is a
// plain boolean check, mirroring TestEnsureResellerCanResellV2Ray /
// TestEnsureResellerCanCreateUserManagerAccounts exactly.
func TestEnsureResellerCanCreateApplications(t *testing.T) {
	svc, db := newTestApplicationServiceForPermission(t)

	notAllowed := model.Reseller{Name: "no-apps", Username: "no-apps", PasswordHash: "x", CanCreateApplications: false}
	if err := db.Create(&notAllowed).Error; err != nil {
		t.Fatalf("failed to create reseller: %v", err)
	}

	allowed := model.Reseller{Name: "yes-apps", Username: "yes-apps", PasswordHash: "x", CanCreateApplications: true}
	if err := db.Create(&allowed).Error; err != nil {
		t.Fatalf("failed to create reseller: %v", err)
	}

	if err := svc.ensureResellerCanCreateApplications(notAllowed.ID); err == nil {
		t.Fatal("expected error for reseller without application permission")
	}
	if err := svc.ensureResellerCanCreateApplications(allowed.ID); err != nil {
		t.Fatalf("expected no error for reseller with application permission, got %v", err)
	}
}

// TestCreateApplication_RejectsUnauthorizedReseller confirms CreateApplication
// stops at the permission check for a reseller lacking CanCreateApplications,
// before ever fanning out to WireGuard/UserManager/V2Ray.
func TestCreateApplication_RejectsUnauthorizedReseller(t *testing.T) {
	svc, db := newTestApplicationServiceForPermission(t)

	reseller := model.Reseller{Name: "no-apps", Username: "no-apps-2", PasswordHash: "x", CanCreateApplications: false}
	if err := db.Create(&reseller).Error; err != nil {
		t.Fatalf("failed to create reseller: %v", err)
	}

	req := &schema.CreateApplicationRequest{
		Name:             "app-unauthorized",
		TotalVolumeBytes: 1024,
		DurationDays:     30,
		MaxOnlineUsers:   1,
	}
	_, err := svc.CreateApplication(req, &reseller.ID)
	if err == nil {
		t.Fatal("expected error creating application for unauthorized reseller")
	}
}

// TestEnsureResellerUnderApplicationLimit mirrors
// TestEnsureResellerUnderV2RayPackageLimit's own contract: nil MaxCount is
// unlimited, and the count check is inclusive of the configured ceiling.
func TestEnsureResellerUnderApplicationLimit(t *testing.T) {
	svc, db := newTestApplicationServiceForPermission(t)

	unlimited := model.Reseller{Name: "unlimited", Username: "unlimited-apps", PasswordHash: "x"}
	if err := db.Create(&unlimited).Error; err != nil {
		t.Fatalf("failed to create reseller: %v", err)
	}
	if err := svc.ensureResellerUnderApplicationLimit(unlimited.ID); err != nil {
		t.Fatalf("expected nil ApplicationMaxCount to mean unlimited, got error: %v", err)
	}

	maxCount := 1
	limited := model.Reseller{Name: "limited", Username: "limited-apps", PasswordHash: "x", ApplicationMaxCount: &maxCount}
	if err := db.Create(&limited).Error; err != nil {
		t.Fatalf("failed to create reseller: %v", err)
	}
	if err := svc.ensureResellerUnderApplicationLimit(limited.ID); err != nil {
		t.Fatalf("expected reseller with 0/1 applications to be under limit, got error: %v", err)
	}

	if err := db.Create(&model.Application{ResellerID: &limited.ID, Name: "app-1", AppUsername: "app-1"}).Error; err != nil {
		t.Fatalf("failed to create application: %v", err)
	}
	if err := svc.ensureResellerUnderApplicationLimit(limited.ID); err == nil {
		t.Fatal("expected error once reseller has reached its ApplicationMaxCount")
	}
}
