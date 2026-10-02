package service

import (
	"errors"
	"fmt"
	"testing"

	"github.com/maahdima/mwp/api/dataservice/model"
	"github.com/maahdima/mwp/api/http/schema"
)

func newTestApplicationRequest(name string) *schema.CreateApplicationRequest {
	return &schema.CreateApplicationRequest{
		Name:             name,
		TotalVolumeBytes: 1024,
		DurationDays:     30,
		MaxOnlineUsers:   1,
	}
}

// TestCreateApplication_NoLicenseLimiterIsUnaffected mirrors
// TestCreateReseller_NoLicenseLimiterIsUnaffected -- no SetLicenseLimiter
// call at all never blocks application creation.
func TestCreateApplication_NoLicenseLimiterIsUnaffected(t *testing.T) {
	svc, _ := newTestApplicationServiceForPermission(t)

	if _, err := svc.CreateApplication(newTestApplicationRequest("app-a"), nil); err != nil {
		t.Fatalf("expected application creation to succeed with no license limiter wired, got: %v", err)
	}
}

// TestCreateApplication_NotRestrictedIsUnaffected mirrors
// TestCreateReseller_NotRestrictedIsUnaffected.
func TestCreateApplication_NotRestrictedIsUnaffected(t *testing.T) {
	svc, _ := newTestApplicationServiceForPermission(t)
	svc.SetLicenseLimiter(&fakeFreeTierLimiter{limits: map[string]int64{}})

	for i := 0; i < 3; i++ {
		req := newTestApplicationRequest(fmt.Sprintf("app-%d", i))
		if _, err := svc.CreateApplication(req, nil); err != nil {
			t.Fatalf("expected application %d to succeed with no active cap, got: %v", i, err)
		}
	}
}

// TestCreateApplication_RestrictedUnderCapSucceeds confirms creation is
// allowed while the current application count is strictly below the
// configured cap.
func TestCreateApplication_RestrictedUnderCapSucceeds(t *testing.T) {
	svc, _ := newTestApplicationServiceForPermission(t)
	svc.SetLicenseLimiter(&fakeFreeTierLimiter{limits: map[string]int64{"max_applications": 2}})

	if _, err := svc.CreateApplication(newTestApplicationRequest("app-a"), nil); err != nil {
		t.Fatalf("expected the 1st application (under cap of 2) to succeed, got: %v", err)
	}
	if _, err := svc.CreateApplication(newTestApplicationRequest("app-b"), nil); err != nil {
		t.Fatalf("expected the 2nd application (at cap of 2, but check runs before insert) to succeed, got: %v", err)
	}
}

// TestCreateApplication_RestrictedAtCapIsRejected is the core regression
// test: once the application count reaches the configured cap, the NEXT
// creation attempt must be rejected with ErrFreeTierApplicationLimitReached,
// and must NOT have inserted a row.
func TestCreateApplication_RestrictedAtCapIsRejected(t *testing.T) {
	svc, db := newTestApplicationServiceForPermission(t)
	svc.SetLicenseLimiter(&fakeFreeTierLimiter{limits: map[string]int64{"max_applications": 1}})

	if _, err := svc.CreateApplication(newTestApplicationRequest("app-a"), nil); err != nil {
		t.Fatalf("expected the 1st application (under cap of 1) to succeed, got: %v", err)
	}

	_, err := svc.CreateApplication(newTestApplicationRequest("app-b"), nil)
	if !errors.Is(err, ErrFreeTierApplicationLimitReached) {
		t.Fatalf("expected ErrFreeTierApplicationLimitReached once the cap is reached, got: %v", err)
	}

	var count int64
	db.Model(&model.Application{}).Count(&count)
	if count != 1 {
		t.Fatalf("expected exactly 1 application row to exist (the rejected attempt must not have inserted), got %d", count)
	}
}
