package service

import (
	"errors"
	"fmt"
	"testing"
	"time"

	"github.com/glebarez/sqlite"
	"gorm.io/gorm"

	"github.com/maahdima/mwp/api/dataservice/model"
	"github.com/maahdima/mwp/api/http/schema"
)

func openResellerFreeTierTestDB(t *testing.T) *gorm.DB {
	t.Helper()

	dsn := fmt.Sprintf("file:reseller_free_tier_%d?mode=memory&cache=shared", time.Now().UnixNano())
	db, err := gorm.Open(sqlite.Open(dsn), &gorm.Config{})
	if err != nil {
		t.Fatalf("failed to open test db: %v", err)
	}
	if err := db.AutoMigrate(&model.Reseller{}, &model.Admin{}); err != nil {
		t.Fatalf("failed to migrate test db: %v", err)
	}
	return db
}

// fakeFreeTierLimiter is a minimal test double for the freeTierLimiter
// interface -- lets each test pin an exact key/value/ok combination
// without needing a real LicenseService or a signed heartbeat round trip.
type fakeFreeTierLimiter struct {
	limits map[string]int64
}

func (f *fakeFreeTierLimiter) GetFreeTierLimit(key string) (int64, bool) {
	v, ok := f.limits[key]
	if !ok || v <= 0 {
		return 0, false
	}
	return v, true
}

func newTestResellerRequest(username string) *schema.CreateResellerRequest {
	return &schema.CreateResellerRequest{
		Name:     username,
		Username: username,
		Password: "password123",
	}
}

// TestCreateReseller_NoLicenseLimiterIsUnaffected confirms the default,
// pre-existing behavior (no SetLicenseLimiter call at all, e.g. a build
// where phase 4-12 isn't wired yet) never blocks reseller creation --
// mirrors every other "safe to leave unset" collaborator in this file.
func TestCreateReseller_NoLicenseLimiterIsUnaffected(t *testing.T) {
	db := openResellerFreeTierTestDB(t)
	svc := NewReseller(db, nil)

	if _, err := svc.CreateReseller(newTestResellerRequest("reseller-a")); err != nil {
		t.Fatalf("expected reseller creation to succeed with no license limiter wired, got: %v", err)
	}
}

// TestCreateReseller_NotRestrictedIsUnaffected confirms a limiter that
// reports "no cap" (GetFreeTierLimit returns ok=false, the normal
// non-Restricted case) never blocks creation, however many resellers
// already exist.
func TestCreateReseller_NotRestrictedIsUnaffected(t *testing.T) {
	db := openResellerFreeTierTestDB(t)
	svc := NewReseller(db, nil)
	svc.SetLicenseLimiter(&fakeFreeTierLimiter{limits: map[string]int64{}})

	for i := 0; i < 3; i++ {
		if _, err := svc.CreateReseller(newTestResellerRequest(fmt.Sprintf("reseller-%d", i))); err != nil {
			t.Fatalf("expected reseller %d to succeed with no active cap, got: %v", i, err)
		}
	}
}

// TestCreateReseller_RestrictedUnderCapSucceeds confirms creation is
// allowed as long as the current reseller count is strictly below the
// configured free-tier cap.
func TestCreateReseller_RestrictedUnderCapSucceeds(t *testing.T) {
	db := openResellerFreeTierTestDB(t)
	svc := NewReseller(db, nil)
	svc.SetLicenseLimiter(&fakeFreeTierLimiter{limits: map[string]int64{"max_resellers": 2}})

	if _, err := svc.CreateReseller(newTestResellerRequest("reseller-a")); err != nil {
		t.Fatalf("expected the 1st reseller (under cap of 2) to succeed, got: %v", err)
	}
	if _, err := svc.CreateReseller(newTestResellerRequest("reseller-b")); err != nil {
		t.Fatalf("expected the 2nd reseller (at cap of 2, but check runs before insert) to succeed, got: %v", err)
	}
}

// TestCreateReseller_RestrictedAtCapIsRejected is the core regression test:
// once the reseller count reaches the configured free-tier cap, the NEXT
// creation attempt must be rejected with ErrFreeTierResellerLimitReached,
// and must NOT have inserted a row.
func TestCreateReseller_RestrictedAtCapIsRejected(t *testing.T) {
	db := openResellerFreeTierTestDB(t)
	svc := NewReseller(db, nil)
	svc.SetLicenseLimiter(&fakeFreeTierLimiter{limits: map[string]int64{"max_resellers": 1}})

	if _, err := svc.CreateReseller(newTestResellerRequest("reseller-a")); err != nil {
		t.Fatalf("expected the 1st reseller (under cap of 1) to succeed, got: %v", err)
	}

	_, err := svc.CreateReseller(newTestResellerRequest("reseller-b"))
	if !errors.Is(err, ErrFreeTierResellerLimitReached) {
		t.Fatalf("expected ErrFreeTierResellerLimitReached once the cap is reached, got: %v", err)
	}

	var count int64
	db.Model(&model.Reseller{}).Count(&count)
	if count != 1 {
		t.Fatalf("expected exactly 1 reseller row to exist (the rejected attempt must not have inserted), got %d", count)
	}
}
