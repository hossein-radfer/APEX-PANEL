package service

import (
	"errors"
	"fmt"
	"testing"
	"time"

	"github.com/glebarez/sqlite"
	"go.uber.org/zap"
	"gorm.io/gorm"

	"github.com/maahdima/mwp/api/adaptor/mikrotik"
	"github.com/maahdima/mwp/api/common"
	"github.com/maahdima/mwp/api/dataservice/model"
	"github.com/maahdima/mwp/api/http/schema"
)

// newTestUserManagerServiceForFreeTier builds a UserManagerService against
// an in-memory sqlite db and a real (but unconfigured) mikrotik.Adaptor --
// mirrors newTestWgPeerServiceWithFakeRouter's construction, minus the fake
// server: with zero registered clients, common.MwpClients.GetClient(nil)
// cleanly returns nil and httphelper.Client's methods are nil-receiver-safe
// (see Put/Patch's own `if c == nil` guard), so calls past the free-tier
// check fail cleanly with ErrNilClient rather than panicking -- exactly
// what these tests need, since the free-tier cap check runs BEFORE any
// MikroTik round trip.
func newTestUserManagerServiceForFreeTier(t *testing.T) (*UserManagerService, *gorm.DB) {
	t.Helper()

	dsn := fmt.Sprintf("file:user_manager_free_tier_test_%d?mode=memory&cache=shared", time.Now().UnixNano())
	db, err := gorm.Open(sqlite.Open(dsn), &gorm.Config{})
	if err != nil {
		t.Fatalf("failed to open test db: %v", err)
	}

	if err := db.AutoMigrate(&model.UserManagerAccount{}, &model.Reseller{}); err != nil {
		t.Fatalf("failed to migrate test db: %v", err)
	}

	mwpClients := common.NewMwpClients(db)
	adaptor := mikrotik.NewAdaptor(mwpClients)
	return &UserManagerService{db: db, mikrotikAdaptor: adaptor, logger: zap.NewNop()}, db
}

func newTestCreateUserManagerAccountRequest(username string) *schema.CreateUserManagerAccountRequest {
	return &schema.CreateUserManagerAccountRequest{Username: username, Password: "password123", Group: "g1", Profile: "p1"}
}

// TestUserManagerService_CreateAccount_NoLicenseLimiterIsUnaffected mirrors
// TestCreateReseller_NoLicenseLimiterIsUnaffected -- no SetLicenseLimiter
// call at all never blocks at the free-tier check (the request still fails
// afterward with ErrNilClient, proving the cap check itself was skipped
// rather than accidentally passing).
func TestUserManagerService_CreateAccount_NoLicenseLimiterIsUnaffected(t *testing.T) {
	svc, _ := newTestUserManagerServiceForFreeTier(t)

	_, err := svc.CreateAccount(newTestCreateUserManagerAccountRequest("um-a"), nil)
	if err == nil || errors.Is(err, ErrFreeTierUserManagerLimitReached) {
		t.Fatalf("expected failure past the free-tier check (nil mikrotik client), got: %v", err)
	}
}

// TestUserManagerService_CreateAccount_NotRestrictedIsUnaffected mirrors
// TestCreateReseller_NotRestrictedIsUnaffected.
func TestUserManagerService_CreateAccount_NotRestrictedIsUnaffected(t *testing.T) {
	svc, _ := newTestUserManagerServiceForFreeTier(t)
	svc.SetLicenseLimiter(&fakeFreeTierLimiter{limits: map[string]int64{}})

	_, err := svc.CreateAccount(newTestCreateUserManagerAccountRequest("um-b"), nil)
	if err == nil || errors.Is(err, ErrFreeTierUserManagerLimitReached) {
		t.Fatalf("expected failure past the free-tier check (nil mikrotik client), got: %v", err)
	}
}

// TestUserManagerService_CreateAccount_RestrictedUnderCapPassesFreeTierCheck
// confirms the cap check itself lets a request through when the current
// account count is strictly below the configured cap.
func TestUserManagerService_CreateAccount_RestrictedUnderCapPassesFreeTierCheck(t *testing.T) {
	svc, _ := newTestUserManagerServiceForFreeTier(t)
	svc.SetLicenseLimiter(&fakeFreeTierLimiter{limits: map[string]int64{"max_user_manager_accounts": 5}})

	_, err := svc.CreateAccount(newTestCreateUserManagerAccountRequest("um-c"), nil)
	if err == nil || errors.Is(err, ErrFreeTierUserManagerLimitReached) {
		t.Fatalf("expected failure past the free-tier check (nil mikrotik client), got: %v", err)
	}
}

// TestUserManagerService_CreateAccount_RestrictedAtCapIsRejected is the
// core regression test: once the account count reaches the configured
// cap, CreateAccount must reject with ErrFreeTierUserManagerLimitReached
// before ever attempting a MikroTik round trip.
func TestUserManagerService_CreateAccount_RestrictedAtCapIsRejected(t *testing.T) {
	svc, db := newTestUserManagerServiceForFreeTier(t)
	svc.SetLicenseLimiter(&fakeFreeTierLimiter{limits: map[string]int64{"max_user_manager_accounts": 1}})

	existing := model.UserManagerAccount{Username: "um-existing"}
	if err := db.Create(&existing).Error; err != nil {
		t.Fatalf("failed to seed existing account: %v", err)
	}

	_, err := svc.CreateAccount(newTestCreateUserManagerAccountRequest("um-d"), nil)
	if !errors.Is(err, ErrFreeTierUserManagerLimitReached) {
		t.Fatalf("expected ErrFreeTierUserManagerLimitReached once the cap is reached, got: %v", err)
	}

	var count int64
	db.Model(&model.UserManagerAccount{}).Count(&count)
	if count != 1 {
		t.Fatalf("expected exactly 1 account row to exist (the rejected attempt must not have inserted), got %d", count)
	}
}
