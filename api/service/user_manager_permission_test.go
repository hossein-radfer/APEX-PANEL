package service

import (
	"testing"

	"github.com/glebarez/sqlite"
	"github.com/maahdima/mwp/api/dataservice/model"
	"github.com/maahdima/mwp/api/http/schema"
	"gorm.io/gorm"
)

func newTestUserManagerService(t *testing.T) (*UserManagerService, *gorm.DB) {
	t.Helper()

	db, err := gorm.Open(sqlite.Open("file::memory:?cache=shared"), &gorm.Config{})
	if err != nil {
		t.Fatalf("failed to open test db: %v", err)
	}

	if err := db.AutoMigrate(&model.Reseller{}, &model.UserManagerAccount{}); err != nil {
		t.Fatalf("failed to migrate test db: %v", err)
	}

	return NewUserManagerService(db, nil, nil), db
}

// TestEnsureResellerCanCreateUserManagerAccounts verifies the permission
// gate is a plain boolean check, entirely separate from WireGuard's
// interface-assignment gate.
func TestEnsureResellerCanCreateUserManagerAccounts(t *testing.T) {
	svc, db := newTestUserManagerService(t)

	notAllowed := model.Reseller{Name: "no-um", Username: "no-um", PasswordHash: "x", CanCreateUserManagerAccounts: false}
	if err := db.Create(&notAllowed).Error; err != nil {
		t.Fatalf("failed to create reseller: %v", err)
	}

	allowed := model.Reseller{Name: "yes-um", Username: "yes-um", PasswordHash: "x", CanCreateUserManagerAccounts: true}
	if err := db.Create(&allowed).Error; err != nil {
		t.Fatalf("failed to create reseller: %v", err)
	}

	if err := svc.ensureResellerCanCreateUserManagerAccounts(notAllowed.ID); err == nil {
		t.Fatal("expected error for reseller without user manager permission")
	}

	if err := svc.ensureResellerCanCreateUserManagerAccounts(allowed.ID); err != nil {
		t.Fatalf("expected no error for reseller with user manager permission, got %v", err)
	}
}

// TestCreateAccount_RejectsUnauthorizedReseller confirms CreateAccount stops
// at the permission check (before ever touching the Mikrotik adaptor, which
// is nil in this test and would panic if reached) for a reseller lacking
// CanCreateUserManagerAccounts.
func TestCreateAccount_RejectsUnauthorizedReseller(t *testing.T) {
	svc, db := newTestUserManagerService(t)

	reseller := model.Reseller{Name: "no-um", Username: "no-um-2", PasswordHash: "x", CanCreateUserManagerAccounts: false}
	if err := db.Create(&reseller).Error; err != nil {
		t.Fatalf("failed to create reseller: %v", err)
	}

	req := &schema.CreateUserManagerAccountRequest{
		Username:  "testuser",
		Password:  "testpass",
		Group:     "default",
		Profile:   "default",
		Protocols: []string{"l2tp"},
	}

	_, err := svc.CreateAccount(req, &reseller.ID)
	if err == nil {
		t.Fatal("expected error creating account for unauthorized reseller")
	}
}
