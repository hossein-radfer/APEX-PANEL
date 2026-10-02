package service

import (
	"fmt"
	"testing"
	"time"

	"github.com/glebarez/sqlite"
	"github.com/maahdima/mwp/api/dataservice/model"
	"gorm.io/gorm"
)

// newTestV2RayPackagePurchaseService uses a uniquely-named in-memory DB per
// call -- see newTestV2RayPackageService's doc comment
// (v2ray_permission_test.go) for why the bare "file::memory:?cache=shared"
// DSN is unsafe to reuse across test functions.
func newTestV2RayPackagePurchaseService(t *testing.T) (*V2RayPackagePurchaseService, *Wallet, *gorm.DB) {
	t.Helper()

	dsn := fmt.Sprintf("file:v2ray_purchase_test_%d?mode=memory&cache=shared", time.Now().UnixNano())
	db, err := gorm.Open(sqlite.Open(dsn), &gorm.Config{})
	if err != nil {
		t.Fatalf("failed to open test db: %v", err)
	}

	if err := db.AutoMigrate(
		&model.Reseller{},
		&model.Wallet{},
		&model.LedgerEntry{},
		&model.V2RayTrafficPackage{},
		&model.V2RayPackagePurchase{},
		// V2RayPackage/V2RayPackageLocation: grantPurchase's own
		// quota-raise detection now sums live usage over these tables
		// (see its own doc comment on why the STORED V2RayUsedBytes can't
		// be used directly), so this fixture needs them even though no
		// test here creates any package/location rows itself.
		&model.V2RayPackage{},
		&model.V2RayPackageLocation{},
	); err != nil {
		t.Fatalf("failed to migrate test db: %v", err)
	}

	walletService := NewWallet(db)
	trafficPackageService := NewV2RayTrafficPackageService(db)
	purchaseService := NewV2RayPackagePurchaseService(db, walletService, trafficPackageService)

	return purchaseService, walletService, db
}

// TestPurchasePackage_HappyPath confirms debit -> grant -> receipt mirrors
// UserManagerPackagePurchaseService's behavior: wallet debited, reseller's
// SEPARATE V2RayQuotaBytes pool raised (not QuotaBytes/UserManagerQuotaBytes),
// and a receipt row created.
func TestPurchasePackage_HappyPath(t *testing.T) {
	purchaseService, walletService, db := newTestV2RayPackagePurchaseService(t)

	initialQuota := int64(1000)
	reseller := model.Reseller{Name: "buyer", Username: "buyer", PasswordHash: "x", V2RayQuotaBytes: &initialQuota}
	if err := db.Create(&reseller).Error; err != nil {
		t.Fatalf("failed to create reseller: %v", err)
	}

	if _, err := walletService.Credit(reseller.ID, 10000, "test top-up", nil, nil); err != nil {
		t.Fatalf("failed to fund wallet: %v", err)
	}

	pkg := model.V2RayTrafficPackage{
		Name: "10GB Pack", TrafficBytes: 10 * 1024 * 1024 * 1024, PriceAmount: 500, IsActive: true,
	}
	if err := db.Create(&pkg).Error; err != nil {
		t.Fatalf("failed to create traffic package: %v", err)
	}

	purchase, err := purchaseService.PurchasePackage(reseller.ID, pkg.ID)
	if err != nil {
		t.Fatalf("expected purchase to succeed, got %v", err)
	}
	if purchase.TrafficBytes != pkg.TrafficBytes {
		t.Fatalf("expected receipt traffic bytes to match package, got %d", purchase.TrafficBytes)
	}

	wallet, err := walletService.GetOrCreateWallet(reseller.ID)
	if err != nil {
		t.Fatalf("failed to fetch wallet: %v", err)
	}
	if wallet.BalanceAmount != 10000-500 {
		t.Fatalf("expected wallet balance 9500 after debit, got %d", wallet.BalanceAmount)
	}

	var reloaded model.Reseller
	if err := db.First(&reloaded, reseller.ID).Error; err != nil {
		t.Fatalf("failed to reload reseller: %v", err)
	}
	expectedQuota := initialQuota + pkg.TrafficBytes
	if reloaded.V2RayQuotaBytes == nil || *reloaded.V2RayQuotaBytes != expectedQuota {
		t.Fatalf("expected V2RayQuotaBytes to be raised to %d, got %v", expectedQuota, reloaded.V2RayQuotaBytes)
	}

	// Confirm this never touched the OTHER two quota pools.
	if reloaded.QuotaBytes != nil {
		t.Fatalf("expected WireGuard QuotaBytes to remain untouched (nil), got %v", *reloaded.QuotaBytes)
	}
	if reloaded.UserManagerQuotaBytes != nil {
		t.Fatalf("expected UserManagerQuotaBytes to remain untouched (nil), got %v", *reloaded.UserManagerQuotaBytes)
	}
}

// TestPurchasePackage_InsufficientFunds confirms a reseller without enough
// wallet balance never reaches the grant step -- no receipt, no quota
// change.
func TestPurchasePackage_InsufficientFunds(t *testing.T) {
	purchaseService, _, db := newTestV2RayPackagePurchaseService(t)

	reseller := model.Reseller{Name: "poor", Username: "poor"}
	if err := db.Create(&reseller).Error; err != nil {
		t.Fatalf("failed to create reseller: %v", err)
	}
	// No wallet credit -- balance is zero.

	pkg := model.V2RayTrafficPackage{
		Name: "Expensive Pack", TrafficBytes: 1024, PriceAmount: 999999, IsActive: true,
	}
	if err := db.Create(&pkg).Error; err != nil {
		t.Fatalf("failed to create traffic package: %v", err)
	}

	if _, err := purchaseService.PurchasePackage(reseller.ID, pkg.ID); err == nil {
		t.Fatal("expected purchase to fail for insufficient funds")
	}

	var purchases []model.V2RayPackagePurchase
	if err := db.Find(&purchases).Error; err != nil {
		t.Fatalf("failed to list purchases: %v", err)
	}
	if len(purchases) != 0 {
		t.Fatalf("expected no purchase receipt to be created, got %d", len(purchases))
	}

	var reloaded model.Reseller
	if err := db.First(&reloaded, reseller.ID).Error; err != nil {
		t.Fatalf("failed to reload reseller: %v", err)
	}
	if reloaded.V2RayQuotaBytes != nil {
		t.Fatalf("expected V2RayQuotaBytes to remain untouched, got %v", *reloaded.V2RayQuotaBytes)
	}
}

// TestPurchasePackage_InactivePackageRejected confirms GetTrafficPackage's
// is_active filter (mirroring UserManagerTrafficPackageService) stops a
// purchase of a deactivated catalog entry before any wallet debit happens.
func TestPurchasePackage_InactivePackageRejected(t *testing.T) {
	purchaseService, walletService, db := newTestV2RayPackagePurchaseService(t)

	reseller := model.Reseller{Name: "buyer2", Username: "buyer2"}
	if err := db.Create(&reseller).Error; err != nil {
		t.Fatalf("failed to create reseller: %v", err)
	}
	if _, err := walletService.Credit(reseller.ID, 10000, "test top-up", nil, nil); err != nil {
		t.Fatalf("failed to fund wallet: %v", err)
	}

	// Created active, then explicitly deactivated via an Updates() call --
	// creating directly with IsActive: false doesn't work here because
	// false is bool's Go zero-value, and the column's `default:true` gorm
	// tag makes GORM's Create silently apply that default to any
	// zero-valued field, exactly like a real admin deactivating an
	// existing package through UpdateTrafficPackage would.
	pkg := model.V2RayTrafficPackage{
		Name: "Retired Pack", TrafficBytes: 1024, PriceAmount: 100, IsActive: true,
	}
	if err := db.Create(&pkg).Error; err != nil {
		t.Fatalf("failed to create traffic package: %v", err)
	}
	inactive := false
	if err := db.Model(&pkg).Update("is_active", inactive).Error; err != nil {
		t.Fatalf("failed to deactivate traffic package: %v", err)
	}

	if _, err := purchaseService.PurchasePackage(reseller.ID, pkg.ID); err == nil {
		t.Fatal("expected purchase to fail for an inactive package")
	}

	wallet, err := walletService.GetOrCreateWallet(reseller.ID)
	if err != nil {
		t.Fatalf("failed to fetch wallet: %v", err)
	}
	if wallet.BalanceAmount != 10000 {
		t.Fatalf("expected wallet balance to remain untouched at 10000, got %d", wallet.BalanceAmount)
	}
}

// fakeV2RayResellerQuotaResumer records which reseller IDs
// ResumePackagesForResellerQuota was called for, standing in for
// *V2RaySyncService in tests that only need to confirm the resume call
// happened (or didn't), not its actual x-ui side effects.
type fakeV2RayResellerQuotaResumer struct {
	calledFor []uint
}

func (f *fakeV2RayResellerQuotaResumer) ResumePackagesForResellerQuota(resellerID uint) {
	f.calledFor = append(f.calledFor, resellerID)
}

// TestPurchasePackage_TopUpFromOverQuota_ResumesSuspendedPackages verifies
// that a reseller whose V2Ray usage had exceeded quota (so
// applyResellerV2RayQuota had already suspended their packages) and who
// then tops up via THIS purchase flow gets their packages resumed. Before
// this fix, grantPurchase raised V2RayQuotaBytes with a raw GORM update and
// never called the resumer at all, leaving every package stuck
// suspended/disabled on x-ui even though the reseller was now back under
// quota.
func TestPurchasePackage_TopUpFromOverQuota_ResumesSuspendedPackages(t *testing.T) {
	purchaseService, walletService, db := newTestV2RayPackagePurchaseService(t)
	resumer := &fakeV2RayResellerQuotaResumer{}
	purchaseService.SetV2RayResumer(resumer)

	// Over quota: LIVE usage (1500, via a real package/location -- see
	// grantPurchase's own doc comment on why the raise-detection now reads
	// live usage instead of the stale V2RayUsedBytes field) > quota (1000).
	quota := int64(1000)
	reseller := model.Reseller{
		Name: "over-quota", Username: "over-quota", PasswordHash: "x",
		V2RayQuotaBytes: &quota, V2RayUsedBytes: 1500,
	}
	if err := db.Create(&reseller).Error; err != nil {
		t.Fatalf("failed to create reseller: %v", err)
	}
	pkg2ray := model.V2RayPackage{UUID: "pkg-over-quota", ResellerID: &reseller.ID, TotalVolumeBytes: 100000, DurationDays: 30, Status: "suspended"}
	if err := db.Create(&pkg2ray).Error; err != nil {
		t.Fatalf("failed to create v2ray package: %v", err)
	}
	loc := model.V2RayPackageLocation{PackageID: pkg2ray.ID, PanelID: 1, ClientUUID: "client-over-quota", ClientEmail: "over-quota@test", SubID: "sub-over-quota", UsedBytesCached: 1500}
	if err := db.Create(&loc).Error; err != nil {
		t.Fatalf("failed to create v2ray package location: %v", err)
	}
	if _, err := walletService.Credit(reseller.ID, 10000, "test top-up", nil, nil); err != nil {
		t.Fatalf("failed to fund wallet: %v", err)
	}

	// Top-up of 1000 bytes brings quota to 2000, now >= the 1500 used --
	// back under quota.
	pkg := model.V2RayTrafficPackage{Name: "Top-up Pack", TrafficBytes: 1000, PriceAmount: 100, IsActive: true}
	if err := db.Create(&pkg).Error; err != nil {
		t.Fatalf("failed to create traffic package: %v", err)
	}

	if _, err := purchaseService.PurchasePackage(reseller.ID, pkg.ID); err != nil {
		t.Fatalf("expected purchase to succeed, got %v", err)
	}

	if len(resumer.calledFor) != 1 || resumer.calledFor[0] != reseller.ID {
		t.Fatalf("expected ResumePackagesForResellerQuota to be called once for reseller %d, got calls: %v", reseller.ID, resumer.calledFor)
	}
}

// TestPurchasePackage_TopUpIgnoresStaleDeletedUsageCredit is the regression
// test for a confirmed, reported production incident (reseller
// "Mohammadreza", VOLUME-mode): a reseller whose LIVE usage is comfortably
// under quota, but whose STORED V2RayUsedBytes carries a large permanent
// V2RayDeletedUsageBytes credit from an earlier package deletion, must
// still have their quota-raise correctly detected -- the raise-detection
// must never be defeated by a deleted package's usage that no longer
// reflects anything actually provisioned.
func TestPurchasePackage_TopUpIgnoresStaleDeletedUsageCredit(t *testing.T) {
	purchaseService, walletService, db := newTestV2RayPackagePurchaseService(t)
	resumer := &fakeV2RayResellerQuotaResumer{}
	purchaseService.SetV2RayResumer(resumer)

	quota := int64(1000)
	reseller := model.Reseller{
		Name: "deleted-usage-credit", Username: "deleted-usage-credit", PasswordHash: "x",
		V2RayQuotaBytes: &quota,
		// Stored total (900 live + 1200 deleted credit = 2100) makes it
		// LOOK like this reseller is at 2100, but their real (live) usage
		// is only 900 -- comfortably under the 1000 quota already.
		V2RayUsedBytes:         2100,
		V2RayDeletedUsageBytes: 1200,
	}
	if err := db.Create(&reseller).Error; err != nil {
		t.Fatalf("failed to create reseller: %v", err)
	}
	pkg2ray := model.V2RayPackage{UUID: "pkg-deleted-credit", ResellerID: &reseller.ID, TotalVolumeBytes: 100000, DurationDays: 30, Status: "active"}
	if err := db.Create(&pkg2ray).Error; err != nil {
		t.Fatalf("failed to create v2ray package: %v", err)
	}
	loc := model.V2RayPackageLocation{PackageID: pkg2ray.ID, PanelID: 1, ClientUUID: "client-deleted-credit", ClientEmail: "deleted-credit@test", SubID: "sub-deleted-credit", UsedBytesCached: 900}
	if err := db.Create(&loc).Error; err != nil {
		t.Fatalf("failed to create v2ray package location: %v", err)
	}
	if _, err := walletService.Credit(reseller.ID, 10000, "test top-up", nil, nil); err != nil {
		t.Fatalf("failed to fund wallet: %v", err)
	}

	pkg := model.V2RayTrafficPackage{Name: "Small Pack", TrafficBytes: 50, PriceAmount: 50, IsActive: true}
	if err := db.Create(&pkg).Error; err != nil {
		t.Fatalf("failed to create traffic package: %v", err)
	}

	if _, err := purchaseService.PurchasePackage(reseller.ID, pkg.ID); err != nil {
		t.Fatalf("expected purchase to succeed, got %v", err)
	}

	// This reseller was never actually "over quota" by live usage (900 <
	// 1000), so wasOverQuota is false and no resume call should fire --
	// but critically, the purchase itself must succeed and the quota raise
	// (to 1050) must be recorded regardless.
	if len(resumer.calledFor) != 0 {
		t.Fatalf("expected no resume call (never over quota by live usage), got calls: %v", resumer.calledFor)
	}

	var reloaded model.Reseller
	if err := db.First(&reloaded, reseller.ID).Error; err != nil {
		t.Fatalf("failed to reload reseller: %v", err)
	}
	if reloaded.V2RayQuotaBytes == nil || *reloaded.V2RayQuotaBytes != 1050 {
		t.Fatalf("expected quota to still be raised to 1050 (1000+50), got %v", reloaded.V2RayQuotaBytes)
	}
}

// TestPurchasePackage_TopUpStillOverQuota_DoesNotResume confirms a partial
// top-up that does NOT bring the reseller back under quota correctly
// skips the resume call -- packages should stay suspended since they're
// still genuinely over their limit.
func TestPurchasePackage_TopUpStillOverQuota_DoesNotResume(t *testing.T) {
	purchaseService, walletService, db := newTestV2RayPackagePurchaseService(t)
	resumer := &fakeV2RayResellerQuotaResumer{}
	purchaseService.SetV2RayResumer(resumer)

	quota := int64(1000)
	reseller := model.Reseller{
		Name: "still-over", Username: "still-over", PasswordHash: "x",
		V2RayQuotaBytes: &quota, V2RayUsedBytes: 5000,
	}
	if err := db.Create(&reseller).Error; err != nil {
		t.Fatalf("failed to create reseller: %v", err)
	}
	if _, err := walletService.Credit(reseller.ID, 10000, "test top-up", nil, nil); err != nil {
		t.Fatalf("failed to fund wallet: %v", err)
	}

	// Tiny top-up: quota goes to 1100, still far below the 5000 used.
	pkg := model.V2RayTrafficPackage{Name: "Small Pack", TrafficBytes: 100, PriceAmount: 50, IsActive: true}
	if err := db.Create(&pkg).Error; err != nil {
		t.Fatalf("failed to create traffic package: %v", err)
	}

	if _, err := purchaseService.PurchasePackage(reseller.ID, pkg.ID); err != nil {
		t.Fatalf("expected purchase to succeed, got %v", err)
	}

	if len(resumer.calledFor) != 0 {
		t.Fatalf("expected ResumePackagesForResellerQuota NOT to be called (still over quota), got calls: %v", resumer.calledFor)
	}
}

// TestPurchasePackage_TopUpWhileUnderQuota_DoesNotResume confirms a
// reseller who was never over quota in the first place never triggers a
// spurious resume call -- ResumePackagesForResellerQuota should only ever
// fire on an actual over-quota -> under-quota transition.
func TestPurchasePackage_TopUpWhileUnderQuota_DoesNotResume(t *testing.T) {
	purchaseService, walletService, db := newTestV2RayPackagePurchaseService(t)
	resumer := &fakeV2RayResellerQuotaResumer{}
	purchaseService.SetV2RayResumer(resumer)

	quota := int64(1000)
	reseller := model.Reseller{
		Name: "healthy", Username: "healthy", PasswordHash: "x",
		V2RayQuotaBytes: &quota, V2RayUsedBytes: 200,
	}
	if err := db.Create(&reseller).Error; err != nil {
		t.Fatalf("failed to create reseller: %v", err)
	}
	if _, err := walletService.Credit(reseller.ID, 10000, "test top-up", nil, nil); err != nil {
		t.Fatalf("failed to fund wallet: %v", err)
	}

	pkg := model.V2RayTrafficPackage{Name: "Routine Pack", TrafficBytes: 500, PriceAmount: 50, IsActive: true}
	if err := db.Create(&pkg).Error; err != nil {
		t.Fatalf("failed to create traffic package: %v", err)
	}

	if _, err := purchaseService.PurchasePackage(reseller.ID, pkg.ID); err != nil {
		t.Fatalf("expected purchase to succeed, got %v", err)
	}

	if len(resumer.calledFor) != 0 {
		t.Fatalf("expected ResumePackagesForResellerQuota NOT to be called (never over quota), got calls: %v", resumer.calledFor)
	}
}
