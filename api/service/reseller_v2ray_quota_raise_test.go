package service

import (
	"fmt"
	"testing"
	"time"

	"github.com/glebarez/sqlite"
	"github.com/maahdima/mwp/api/dataservice/model"
	"github.com/maahdima/mwp/api/http/schema"
	"gorm.io/gorm"
)

func openResellerV2RayQuotaRaiseTestDB(t *testing.T) *gorm.DB {
	t.Helper()

	dsn := fmt.Sprintf("file:reseller_v2ray_quota_raise_%d?mode=memory&cache=shared", time.Now().UnixNano())
	db, err := gorm.Open(sqlite.Open(dsn), &gorm.Config{})
	if err != nil {
		t.Fatalf("failed to open test db: %v", err)
	}

	if err := db.AutoMigrate(
		&model.Reseller{},
		&model.Peer{},
		&model.UserManagerAccount{},
		&model.V2RayPackage{},
		&model.V2RayPackageLocation{},
		&model.Application{},
	); err != nil {
		t.Fatalf("failed to migrate test db: %v", err)
	}

	return db
}

// TestUpdateReseller_V2RayQuotaRaise_IgnoresStaleDeletedUsageCredit is the
// regression test for a confirmed, reported production incident (reseller
// "Mohammadreza", VOLUME-mode): UpdateReseller's own v2rayQuotaRaised
// detection compared the requested new quota against res.V2RayUsedBytes,
// the STORED total that permanently includes V2RayDeletedUsageBytes (a
// credit folded in at package-delete time so a reseller can't dodge
// Payment-mode tiered pricing or erase usage history by deleting a
// package -- see that field's own doc comment). For a VOLUME-mode
// reseller, that credit has no billing meaning at all, so comparing
// against it meant an admin raising V2RayQuotaBytes to comfortably cover
// the reseller's REAL current (live) usage could still fail to register
// as an actual raise -- and therefore never call ResumePackagesForResellerQuota
// -- whenever the reseller had ever deleted a package. Confirms the fix:
// the raise is detected correctly against the LIVE sum over
// currently-existing packages, even with a large stale deleted-usage
// credit sitting on the stored total.
func TestUpdateReseller_V2RayQuotaRaise_IgnoresStaleDeletedUsageCredit(t *testing.T) {
	db := openResellerV2RayQuotaRaiseTestDB(t)
	svc := NewReseller(db, nil)
	resumer := &fakeV2RayResellerQuotaResumer{}
	svc.SetV2RayResumer(resumer)

	// Live usage (215) is genuinely over the current 200 quota -- mirrors
	// Mohammadreza's real ~215GB-used/200GB-quota state -- while the
	// STORED total carries an extra 130-unit V2RayDeletedUsageBytes credit
	// from an earlier package deletion on top of that, so it reads 345.
	quota := int64(200)
	reseller := model.Reseller{
		Name: "mohammadreza-like", Username: "mohammadreza-like",
		V2RayQuotaBytes:        &quota,
		V2RayUsedBytes:         345, // 215 live + 130 deleted credit
		V2RayDeletedUsageBytes: 130,
	}
	if err := db.Create(&reseller).Error; err != nil {
		t.Fatalf("failed to create reseller: %v", err)
	}

	pkg := model.V2RayPackage{
		UUID: "pkg-live-215", ResellerID: &reseller.ID, TotalVolumeBytes: 100000, DurationDays: 30,
		Status: "suspended", SuspendedByResellerQuota: true, WasActiveBeforeSuspend: true,
	}
	if err := db.Create(&pkg).Error; err != nil {
		t.Fatalf("failed to create v2ray package: %v", err)
	}
	loc := model.V2RayPackageLocation{PackageID: pkg.ID, PanelID: 1, ClientUUID: "client-live-215", ClientEmail: "live-215@test", SubID: "sub-live-215", UsedBytesCached: 215}
	if err := db.Create(&loc).Error; err != nil {
		t.Fatalf("failed to create v2ray package location: %v", err)
	}

	// A modest raise to 220 -- comfortably covers the real 215 live usage,
	// but is still far short of the inflated 345 stored total. The OLD
	// code (comparing against the stored total) would never treat this as
	// "back under quota," leaving the reseller stuck suspended no matter
	// how much the admin raised the quota by, short of covering the
	// permanently-stuck deleted-usage credit too.
	newQuota := int64(220)
	if _, err := svc.UpdateReseller(reseller.ID, &schema.UpdateResellerRequest{
		V2RayQuotaBytes: &newQuota,
	}); err != nil {
		t.Fatalf("UpdateReseller failed: %v", err)
	}

	if len(resumer.calledFor) != 1 || resumer.calledFor[0] != reseller.ID {
		t.Fatalf("expected ResumePackagesForResellerQuota to be called once for reseller %d, got calls: %v", reseller.ID, resumer.calledFor)
	}
}

// TestUpdateReseller_V2RayQuotaRaise_StillOverLiveUsageDoesNotResume confirms
// the fix didn't overcorrect -- a raise that doesn't cover the reseller's
// REAL live usage must still not trigger a resume call.
func TestUpdateReseller_V2RayQuotaRaise_StillOverLiveUsageDoesNotResume(t *testing.T) {
	db := openResellerV2RayQuotaRaiseTestDB(t)
	svc := NewReseller(db, nil)
	resumer := &fakeV2RayResellerQuotaResumer{}
	svc.SetV2RayResumer(resumer)

	quota := int64(200)
	reseller := model.Reseller{
		Name: "still-over-live", Username: "still-over-live",
		V2RayQuotaBytes: &quota, V2RayUsedBytes: 500,
	}
	if err := db.Create(&reseller).Error; err != nil {
		t.Fatalf("failed to create reseller: %v", err)
	}

	pkg := model.V2RayPackage{UUID: "pkg-live-500", ResellerID: &reseller.ID, TotalVolumeBytes: 100000, DurationDays: 30, Status: "suspended"}
	if err := db.Create(&pkg).Error; err != nil {
		t.Fatalf("failed to create v2ray package: %v", err)
	}
	loc := model.V2RayPackageLocation{PackageID: pkg.ID, PanelID: 1, ClientUUID: "client-live-500", ClientEmail: "live-500@test", SubID: "sub-live-500", UsedBytesCached: 500}
	if err := db.Create(&loc).Error; err != nil {
		t.Fatalf("failed to create v2ray package location: %v", err)
	}

	// Raise to 250 -- still well under the real 500 live usage.
	newQuota := int64(250)
	if _, err := svc.UpdateReseller(reseller.ID, &schema.UpdateResellerRequest{
		V2RayQuotaBytes: &newQuota,
	}); err != nil {
		t.Fatalf("UpdateReseller failed: %v", err)
	}

	if len(resumer.calledFor) != 0 {
		t.Fatalf("expected no resume call (still over live usage), got calls: %v", resumer.calledFor)
	}
}
