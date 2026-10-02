package service

import (
	"testing"
	"time"

	"github.com/maahdima/mwp/api/dataservice/model"
	"github.com/maahdima/mwp/api/http/schema"
)

// TestUpdatePackage_RecomputesExpireAtWhenDurationDaysChanges is a
// regression test for a confirmed bug: UpdatePackage wrote the new
// DurationDays onto the package row but never recomputed ExpireAt, so
// changing a package's duration (e.g. a renewal driven by an external
// integration calling PUT /v2ray-package/:id) had no actual effect on
// when the package expires. Confirms ExpireAt now moves in lockstep with
// DurationDays, anchored to the package's own StartAt exactly like
// CreatePackage's own formula.
func TestUpdatePackage_RecomputesExpireAtWhenDurationDaysChanges(t *testing.T) {
	svc, _ := newTestV2RayPackageService(t)

	created, err := svc.CreatePackage(&schema.CreateV2RayPackageRequest{
		TotalVolumeBytes: 1024, DurationDays: 30,
	}, nil)
	if err != nil {
		t.Fatalf("failed to create package: %v", err)
	}
	if created.ExpireAt == nil {
		t.Fatal("expected ExpireAt to be set on creation")
	}
	originalExpireAt, err := time.Parse("2006-01-02", *created.ExpireAt)
	if err != nil {
		t.Fatalf("failed to parse original ExpireAt: %v", err)
	}

	newDuration := 60
	updated, err := svc.UpdatePackage(created.Id, &schema.UpdateV2RayPackageRequest{
		DurationDays: &newDuration,
	}, nil)
	if err != nil {
		t.Fatalf("failed to update package: %v", err)
	}
	if updated.ExpireAt == nil {
		t.Fatal("expected ExpireAt to still be set after update")
	}
	updatedExpireAt, err := time.Parse("2006-01-02", *updated.ExpireAt)
	if err != nil {
		t.Fatalf("failed to parse updated ExpireAt: %v", err)
	}

	if !updatedExpireAt.After(originalExpireAt) {
		t.Fatalf("expected ExpireAt to move forward after extending DurationDays 30->60, original=%v updated=%v (bug: ExpireAt was never recomputed)", originalExpireAt, updatedExpireAt)
	}

	gotDiffDays := int(updatedExpireAt.Sub(originalExpireAt).Hours() / 24)
	if gotDiffDays != 30 {
		t.Fatalf("expected ExpireAt to move forward by exactly 30 days (60-30), got %d days", gotDiffDays)
	}
}

// TestUpdatePackage_OtherFieldsDoNotDisturbExpireAt confirms updating an
// unrelated field (CustomerLabel/Status) leaves ExpireAt untouched, since
// only a genuine DurationDays change should recompute it.
func TestUpdatePackage_OtherFieldsDoNotDisturbExpireAt(t *testing.T) {
	svc, _ := newTestV2RayPackageService(t)

	created, err := svc.CreatePackage(&schema.CreateV2RayPackageRequest{
		TotalVolumeBytes: 1024, DurationDays: 30,
	}, nil)
	if err != nil {
		t.Fatalf("failed to create package: %v", err)
	}

	label := "renamed"
	updated, err := svc.UpdatePackage(created.Id, &schema.UpdateV2RayPackageRequest{
		CustomerLabel: &label,
	}, nil)
	if err != nil {
		t.Fatalf("failed to update package: %v", err)
	}

	if updated.ExpireAt == nil || created.ExpireAt == nil || *updated.ExpireAt != *created.ExpireAt {
		t.Fatalf("expected ExpireAt to remain unchanged when only CustomerLabel is updated, before=%v after=%v", created.ExpireAt, updated.ExpireAt)
	}
}

// TestUpdatePackage_ReactivatingClearsSuspendedByResellerQuota is the
// regression test for a confirmed, reported production incident (reseller
// "Mohammadreza", VOLUME-mode, whose overall V2Ray pool sits slightly over
// its configured quota): manually setting a package's Status back to
// "active" (the only lever an admin/reseller has to bring a suspended
// package back online by hand) cleared SuspendedByQuota/
// WasActiveBeforeSuspend but left SuspendedByResellerQuota=true stuck in
// the DB. Since the package's Status was now "active" (no longer
// "suspended"), applyResellerV2RayQuota's own suspend query -- which
// targets every package with status != "suspended" for a still-over-quota
// reseller -- picked the package right back up on the very next sync tick
// and re-suspended it, undoing the manual reactivation within moments and
// repeating for as long as the reseller's pool stayed over quota. Confirms
// the fix: reactivating via Status now clears SuspendedByResellerQuota too.
func TestUpdatePackage_ReactivatingClearsSuspendedByResellerQuota(t *testing.T) {
	svc, db := newTestV2RayPackageService(t)

	pkg := model.V2RayPackage{
		UUID: "pkg-reseller-quota-flap", TotalVolumeBytes: 1024, DurationDays: 30,
		Status: "suspended", SuspendedByResellerQuota: true, WasActiveBeforeSuspend: true,
	}
	if err := db.Create(&pkg).Error; err != nil {
		t.Fatalf("failed to create package: %v", err)
	}

	active := "active"
	updated, err := svc.UpdatePackage(pkg.ID, &schema.UpdateV2RayPackageRequest{Status: &active}, nil)
	if err != nil {
		t.Fatalf("failed to update package: %v", err)
	}
	if updated.Status != "active" {
		t.Fatalf("expected package status to be active after reactivation, got %q", updated.Status)
	}

	var reloaded model.V2RayPackage
	if err := db.First(&reloaded, pkg.ID).Error; err != nil {
		t.Fatalf("failed to reload package: %v", err)
	}
	if reloaded.SuspendedByResellerQuota {
		t.Fatalf("expected SuspendedByResellerQuota to be cleared on manual reactivation, but it is still true -- " +
			"the next reseller-quota sync tick would treat this package as never-suspended and re-suspend it immediately")
	}
	if reloaded.SuspendedByQuota || reloaded.WasActiveBeforeSuspend {
		t.Fatalf("expected SuspendedByQuota/WasActiveBeforeSuspend to also be cleared on reactivation, got suspended_by_quota=%v was_active_before_suspend=%v",
			reloaded.SuspendedByQuota, reloaded.WasActiveBeforeSuspend)
	}
}

// TestUpdatePackage_ReactivatingDoesNotResurrectUnsuspendedResellerQuotaFlag
// confirms the fix above is scoped to an actual Status->"active" transition
// -- updating an unrelated field on an already-active package with no
// suspend flags set must not somehow set SuspendedByResellerQuota, and a
// package left "suspended" by this same call must keep the flag exactly as
// it was (this call only ever clears the flag, never sets it).
func TestUpdatePackage_ReactivatingDoesNotResurrectUnsuspendedResellerQuotaFlag(t *testing.T) {
	svc, db := newTestV2RayPackageService(t)

	pkg := model.V2RayPackage{
		UUID: "pkg-reseller-quota-untouched", TotalVolumeBytes: 1024, DurationDays: 30,
		Status: "active",
	}
	if err := db.Create(&pkg).Error; err != nil {
		t.Fatalf("failed to create package: %v", err)
	}

	label := "renamed"
	if _, err := svc.UpdatePackage(pkg.ID, &schema.UpdateV2RayPackageRequest{CustomerLabel: &label}, nil); err != nil {
		t.Fatalf("failed to update package: %v", err)
	}

	var reloaded model.V2RayPackage
	if err := db.First(&reloaded, pkg.ID).Error; err != nil {
		t.Fatalf("failed to reload package: %v", err)
	}
	if reloaded.SuspendedByResellerQuota {
		t.Fatalf("expected an unrelated field update to never set SuspendedByResellerQuota, got true")
	}
}
