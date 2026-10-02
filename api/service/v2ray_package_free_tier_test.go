package service

import (
	"errors"
	"testing"

	"github.com/maahdima/mwp/api/dataservice/model"
	"github.com/maahdima/mwp/api/http/schema"
)

func newTestV2RayPackageRequest() *schema.CreateV2RayPackageRequest {
	return &schema.CreateV2RayPackageRequest{TotalVolumeBytes: 1024, DurationDays: 30}
}

// TestCreatePackage_NoLicenseLimiterIsUnaffected mirrors
// TestCreateReseller_NoLicenseLimiterIsUnaffected -- a build where phase
// 4-12 isn't wired (no SetLicenseLimiter call at all) never blocks package
// creation.
func TestCreatePackage_NoLicenseLimiterIsUnaffected(t *testing.T) {
	svc, _ := newTestV2RayPackageService(t)

	if _, err := svc.CreatePackage(newTestV2RayPackageRequest(), nil); err != nil {
		t.Fatalf("expected package creation to succeed with no license limiter wired, got: %v", err)
	}
}

// TestCreatePackage_NotRestrictedIsUnaffected mirrors
// TestCreateReseller_NotRestrictedIsUnaffected -- GetFreeTierLimit
// reporting ok=false (the normal non-Restricted case) never blocks
// creation.
func TestCreatePackage_NotRestrictedIsUnaffected(t *testing.T) {
	svc, _ := newTestV2RayPackageService(t)
	svc.SetLicenseLimiter(&fakeFreeTierLimiter{limits: map[string]int64{}})

	for i := 0; i < 3; i++ {
		if _, err := svc.CreatePackage(newTestV2RayPackageRequest(), nil); err != nil {
			t.Fatalf("expected package %d to succeed with no active cap, got: %v", i, err)
		}
	}
}

// TestCreatePackage_RestrictedUnderCapSucceeds confirms creation is allowed
// while the current package count is strictly below the configured cap.
func TestCreatePackage_RestrictedUnderCapSucceeds(t *testing.T) {
	svc, _ := newTestV2RayPackageService(t)
	svc.SetLicenseLimiter(&fakeFreeTierLimiter{limits: map[string]int64{"max_v2ray_packages": 2}})

	if _, err := svc.CreatePackage(newTestV2RayPackageRequest(), nil); err != nil {
		t.Fatalf("expected the 1st package (under cap of 2) to succeed, got: %v", err)
	}
	if _, err := svc.CreatePackage(newTestV2RayPackageRequest(), nil); err != nil {
		t.Fatalf("expected the 2nd package (at cap of 2, but check runs before insert) to succeed, got: %v", err)
	}
}

// TestCreatePackage_RestrictedAtCapIsRejected is the core regression test:
// once the package count reaches the configured cap, the NEXT creation
// attempt must be rejected with ErrFreeTierV2RayPackageLimitReached, and
// must NOT have inserted a row.
func TestCreatePackage_RestrictedAtCapIsRejected(t *testing.T) {
	svc, db := newTestV2RayPackageService(t)
	svc.SetLicenseLimiter(&fakeFreeTierLimiter{limits: map[string]int64{"max_v2ray_packages": 1}})

	if _, err := svc.CreatePackage(newTestV2RayPackageRequest(), nil); err != nil {
		t.Fatalf("expected the 1st package (under cap of 1) to succeed, got: %v", err)
	}

	_, err := svc.CreatePackage(newTestV2RayPackageRequest(), nil)
	if !errors.Is(err, ErrFreeTierV2RayPackageLimitReached) {
		t.Fatalf("expected ErrFreeTierV2RayPackageLimitReached once the cap is reached, got: %v", err)
	}

	var count int64
	db.Model(&model.V2RayPackage{}).Count(&count)
	if count != 1 {
		t.Fatalf("expected exactly 1 package row to exist (the rejected attempt must not have inserted), got %d", count)
	}
}
