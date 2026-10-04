package service

import (
	"testing"
	"time"

	"github.com/maahdima/mwp/api/dataservice/model"
)

// TestRenewPackage_ResetsExpiryToFullDurationFromNow is the core test for
// the admin's "V2Ray needs a Reset Days button too" request: a package
// created 20 of its 30 days ago (10 days remaining) must read a full 30
// days remaining again after renewal, anchored on today -- not on its
// original StartAt, which is UpdatePackage's own DurationDays behavior and
// deliberately NOT what a renewal should do (see RenewPackage's own doc
// comment for why a dedicated action exists instead of reusing it).
func TestRenewPackage_ResetsExpiryToFullDurationFromNow(t *testing.T) {
	svc, db := newTestV2RayPackageService(t)

	oldStart := time.Now().AddDate(0, 0, -20)
	oldExpire := oldStart.AddDate(0, 0, 30)
	pkg := model.V2RayPackage{
		UUID: "pkg-renew-1", TotalVolumeBytes: 1024, DurationDays: 30,
		StartAt: &oldStart, ExpireAt: &oldExpire, Status: "active",
	}
	if err := db.Create(&pkg).Error; err != nil {
		t.Fatalf("failed to create package: %v", err)
	}

	renewed, err := svc.RenewPackage(pkg.ID, nil)
	if err != nil {
		t.Fatalf("RenewPackage failed: %v", err)
	}
	if renewed.ExpireAt == nil {
		t.Fatal("expected ExpireAt to be set after renewal")
	}

	newExpireAt, err := time.Parse("2006-01-02", *renewed.ExpireAt)
	if err != nil {
		t.Fatalf("failed to parse renewed ExpireAt: %v", err)
	}

	daysRemaining := int(time.Until(newExpireAt).Hours() / 24)
	if daysRemaining < 29 || daysRemaining > 30 {
		t.Fatalf("expected ~30 days remaining after renewal (was 10 before), got %d days (new expiry=%v)", daysRemaining, newExpireAt)
	}

	var reloaded model.V2RayPackage
	if err := db.First(&reloaded, pkg.ID).Error; err != nil {
		t.Fatalf("failed to reload package: %v", err)
	}
	if reloaded.StartAt == nil || reloaded.StartAt.Before(time.Now().Add(-time.Minute)) {
		t.Fatalf("expected StartAt to move to approximately now, got %v", reloaded.StartAt)
	}
}

// TestRenewPackage_ReactivatesQuotaSuspendedPackage confirms a renewal
// also lifts a quota-triggered suspension -- mirrors ResetUsage's own
// rationale exactly (an admin granting more time, like granting more
// volume, is the kind of action that should bring a package back online).
func TestRenewPackage_ReactivatesQuotaSuspendedPackage(t *testing.T) {
	svc, db := newTestV2RayPackageService(t)

	pkg := model.V2RayPackage{
		UUID: "pkg-renew-2", TotalVolumeBytes: 1024, DurationDays: 30,
		Status: "suspended", SuspendedByQuota: true, WasActiveBeforeSuspend: true,
	}
	if err := db.Create(&pkg).Error; err != nil {
		t.Fatalf("failed to create package: %v", err)
	}

	renewed, err := svc.RenewPackage(pkg.ID, nil)
	if err != nil {
		t.Fatalf("RenewPackage failed: %v", err)
	}
	if renewed.Status != "active" {
		t.Fatalf("expected package status to be active after renewal, got %q", renewed.Status)
	}

	var reloaded model.V2RayPackage
	if err := db.First(&reloaded, pkg.ID).Error; err != nil {
		t.Fatalf("failed to reload package: %v", err)
	}
	if reloaded.SuspendedByQuota || reloaded.WasActiveBeforeSuspend {
		t.Fatalf("expected SuspendedByQuota/WasActiveBeforeSuspend cleared after renewal, got suspended_by_quota=%v was_active_before_suspend=%v",
			reloaded.SuspendedByQuota, reloaded.WasActiveBeforeSuspend)
	}
}

// TestRenewPackage_DoesNotReactivateManuallySuspendedPackage confirms a
// package an admin suspended on purpose (no quota flags set) is left
// suspended by a renewal -- only a quota-caused suspension should be
// auto-lifted, exactly like ResetUsage's own guarantee.
func TestRenewPackage_DoesNotReactivateManuallySuspendedPackage(t *testing.T) {
	svc, db := newTestV2RayPackageService(t)

	pkg := model.V2RayPackage{
		UUID: "pkg-renew-3", TotalVolumeBytes: 1024, DurationDays: 30,
		Status: "suspended",
	}
	if err := db.Create(&pkg).Error; err != nil {
		t.Fatalf("failed to create package: %v", err)
	}

	renewed, err := svc.RenewPackage(pkg.ID, nil)
	if err != nil {
		t.Fatalf("RenewPackage failed: %v", err)
	}
	if renewed.Status != "suspended" {
		t.Fatalf("expected a manually-suspended package to remain suspended after renewal, got %q", renewed.Status)
	}
}

// TestRenewPackage_RespectsResellerScope confirms a reseller can only
// renew their own package, mirroring every other scoped action
// (getPackageByIDScoped).
func TestRenewPackage_RespectsResellerScope(t *testing.T) {
	svc, db := newTestV2RayPackageService(t)

	otherResellerID := uint(999)
	pkg := model.V2RayPackage{
		UUID: "pkg-renew-4", TotalVolumeBytes: 1024, DurationDays: 30,
		Status: "active", ResellerID: &otherResellerID,
	}
	if err := db.Create(&pkg).Error; err != nil {
		t.Fatalf("failed to create package: %v", err)
	}

	callerResellerID := uint(1)
	if _, err := svc.RenewPackage(pkg.ID, &callerResellerID); err == nil {
		t.Fatal("expected RenewPackage to fail for a package owned by a different reseller, got nil error")
	}

	if _, err := svc.RenewPackage(pkg.ID, &otherResellerID); err != nil {
		t.Fatalf("expected RenewPackage to succeed for the owning reseller, got error: %v", err)
	}
}
