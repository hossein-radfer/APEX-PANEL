package service

import (
	"fmt"
	"strings"
	"testing"
	"time"

	"github.com/glebarez/sqlite"
	"github.com/maahdima/mwp/api/dataservice/model"
	"gorm.io/gorm"
)

func openPackagePurchaseTestDB(t *testing.T) *gorm.DB {
	t.Helper()

	dsn := fmt.Sprintf("file:package_purchase_%d?mode=memory&cache=shared", time.Now().UnixNano())
	db, err := gorm.Open(sqlite.Open(dsn), &gorm.Config{})
	if err != nil {
		t.Fatalf("failed to open test db: %v", err)
	}

	if err := db.AutoMigrate(
		&model.Reseller{}, &model.Wallet{}, &model.LedgerEntry{},
		&model.TrafficPackage{}, &model.PackagePurchase{},
	); err != nil {
		t.Fatalf("failed to migrate test db: %v", err)
	}

	return db
}

func TestPurchasePackageDebitsWalletAndRaisesQuota(t *testing.T) {
	db := openPackagePurchaseTestDB(t)
	walletSvc := NewWallet(db)
	pkgSvc := NewTrafficPackageService(db)
	purchaseSvc := NewPackagePurchaseService(db, walletSvc, pkgSvc)

	quota := int64(1000)
	reseller := model.Reseller{Name: "buyer", Username: "buyer", QuotaBytes: &quota}
	if err := db.Create(&reseller).Error; err != nil {
		t.Fatalf("failed to create reseller: %v", err)
	}

	if _, err := walletSvc.Credit(reseller.ID, 5000, "seed", nil, nil); err != nil {
		t.Fatalf("seed credit failed: %v", err)
	}

	pkg, err := pkgSvc.CreateTrafficPackage("10GB Boost", nil, 500, 2000)
	if err != nil {
		t.Fatalf("failed to create package: %v", err)
	}

	purchase, err := purchaseSvc.PurchasePackage(reseller.ID, pkg.ID)
	if err != nil {
		t.Fatalf("PurchasePackage failed: %v", err)
	}

	if purchase.TrafficPackageName != pkg.Name || purchase.PriceAmount != pkg.PriceAmount || purchase.TrafficBytes != pkg.TrafficBytes {
		t.Fatalf("purchase receipt does not match package snapshot: %+v vs %+v", purchase, pkg)
	}

	balance, err := walletSvc.GetWalletBalance(reseller.ID)
	if err != nil {
		t.Fatalf("failed to get balance: %v", err)
	}
	if balance != 5000-2000 {
		t.Fatalf("expected balance 3000 after purchase, got %d", balance)
	}

	var updatedReseller model.Reseller
	if err := db.First(&updatedReseller, reseller.ID).Error; err != nil {
		t.Fatalf("failed to reload reseller: %v", err)
	}
	if updatedReseller.QuotaBytes == nil || *updatedReseller.QuotaBytes != quota+pkg.TrafficBytes {
		t.Fatalf("expected quota raised to %d, got %v", quota+pkg.TrafficBytes, updatedReseller.QuotaBytes)
	}
}

func TestPurchasePackageFailsOnInsufficientFunds(t *testing.T) {
	db := openPackagePurchaseTestDB(t)
	walletSvc := NewWallet(db)
	pkgSvc := NewTrafficPackageService(db)
	purchaseSvc := NewPackagePurchaseService(db, walletSvc, pkgSvc)

	quota := int64(1000)
	reseller := model.Reseller{Name: "poor-buyer", Username: "poor-buyer", QuotaBytes: &quota}
	if err := db.Create(&reseller).Error; err != nil {
		t.Fatalf("failed to create reseller: %v", err)
	}
	// No wallet credit at all -- balance starts at 0.

	pkg, err := pkgSvc.CreateTrafficPackage("Expensive Boost", nil, 500, 2000)
	if err != nil {
		t.Fatalf("failed to create package: %v", err)
	}

	if _, err := purchaseSvc.PurchasePackage(reseller.ID, pkg.ID); err == nil {
		t.Fatalf("expected insufficient-funds error, got nil")
	} else if !strings.Contains(err.Error(), "insufficient funds") {
		t.Fatalf("expected insufficient funds error, got: %v", err)
	}

	// Confirm nothing was charged and quota is untouched.
	balance, err := walletSvc.GetWalletBalance(reseller.ID)
	if err != nil {
		t.Fatalf("failed to get balance: %v", err)
	}
	if balance != 0 {
		t.Fatalf("expected balance to remain 0 after failed purchase, got %d", balance)
	}

	var unchangedReseller model.Reseller
	if err := db.First(&unchangedReseller, reseller.ID).Error; err != nil {
		t.Fatalf("failed to reload reseller: %v", err)
	}
	if unchangedReseller.QuotaBytes == nil || *unchangedReseller.QuotaBytes != quota {
		t.Fatalf("expected quota to remain %d after failed purchase, got %v", quota, unchangedReseller.QuotaBytes)
	}

	var purchaseCount int64
	db.Model(&model.PackagePurchase{}).Where("reseller_id = ?", reseller.ID).Count(&purchaseCount)
	if purchaseCount != 0 {
		t.Fatalf("expected no purchase record after failed purchase, got %d", purchaseCount)
	}
}

func TestPurchasePackageFailsOnUnknownPackage(t *testing.T) {
	db := openPackagePurchaseTestDB(t)
	walletSvc := NewWallet(db)
	pkgSvc := NewTrafficPackageService(db)
	purchaseSvc := NewPackagePurchaseService(db, walletSvc, pkgSvc)

	reseller := model.Reseller{Name: "no-such-package", Username: "no-such-package"}
	if err := db.Create(&reseller).Error; err != nil {
		t.Fatalf("failed to create reseller: %v", err)
	}
	if _, err := walletSvc.Credit(reseller.ID, 100000, "seed", nil, nil); err != nil {
		t.Fatalf("seed credit failed: %v", err)
	}

	if _, err := purchaseSvc.PurchasePackage(reseller.ID, 999999); err == nil {
		t.Fatalf("expected error for unknown package id, got nil")
	}

	balance, err := walletSvc.GetWalletBalance(reseller.ID)
	if err != nil {
		t.Fatalf("failed to get balance: %v", err)
	}
	if balance != 100000 {
		t.Fatalf("expected balance untouched at 100000, got %d", balance)
	}
}

// TestPurchasePackageUnlimitedQuotaStillGrantsReceipt confirms a reseller
// with QuotaBytes == nil (unlimited) can still buy a package: the wallet is
// still debited and a receipt is still recorded, there's just no numeric
// cap to raise.
func TestPurchasePackageUnlimitedQuotaStillGrantsReceipt(t *testing.T) {
	db := openPackagePurchaseTestDB(t)
	walletSvc := NewWallet(db)
	pkgSvc := NewTrafficPackageService(db)
	purchaseSvc := NewPackagePurchaseService(db, walletSvc, pkgSvc)

	reseller := model.Reseller{Name: "unlimited-buyer", Username: "unlimited-buyer", QuotaBytes: nil}
	if err := db.Create(&reseller).Error; err != nil {
		t.Fatalf("failed to create reseller: %v", err)
	}
	if _, err := walletSvc.Credit(reseller.ID, 5000, "seed", nil, nil); err != nil {
		t.Fatalf("seed credit failed: %v", err)
	}

	pkg, err := pkgSvc.CreateTrafficPackage("Unlimited-owner Boost", nil, 500, 1000)
	if err != nil {
		t.Fatalf("failed to create package: %v", err)
	}

	if _, err := purchaseSvc.PurchasePackage(reseller.ID, pkg.ID); err != nil {
		t.Fatalf("PurchasePackage failed: %v", err)
	}

	var updatedReseller model.Reseller
	if err := db.First(&updatedReseller, reseller.ID).Error; err != nil {
		t.Fatalf("failed to reload reseller: %v", err)
	}
	if updatedReseller.QuotaBytes != nil {
		t.Fatalf("expected quota to remain unlimited (nil), got %v", *updatedReseller.QuotaBytes)
	}

	balance, err := walletSvc.GetWalletBalance(reseller.ID)
	if err != nil {
		t.Fatalf("failed to get balance: %v", err)
	}
	if balance != 5000-1000 {
		t.Fatalf("expected balance 4000, got %d", balance)
	}
}

func TestListPurchasesReturnsMostRecentFirst(t *testing.T) {
	db := openPackagePurchaseTestDB(t)
	walletSvc := NewWallet(db)
	pkgSvc := NewTrafficPackageService(db)
	purchaseSvc := NewPackagePurchaseService(db, walletSvc, pkgSvc)

	reseller := model.Reseller{Name: "history-buyer", Username: "history-buyer"}
	if err := db.Create(&reseller).Error; err != nil {
		t.Fatalf("failed to create reseller: %v", err)
	}
	if _, err := walletSvc.Credit(reseller.ID, 100000, "seed", nil, nil); err != nil {
		t.Fatalf("seed credit failed: %v", err)
	}

	pkgA, _ := pkgSvc.CreateTrafficPackage("Pack A", nil, 100, 100)
	pkgB, _ := pkgSvc.CreateTrafficPackage("Pack B", nil, 200, 200)

	if _, err := purchaseSvc.PurchasePackage(reseller.ID, pkgA.ID); err != nil {
		t.Fatalf("purchase A failed: %v", err)
	}
	if _, err := purchaseSvc.PurchasePackage(reseller.ID, pkgB.ID); err != nil {
		t.Fatalf("purchase B failed: %v", err)
	}

	history, err := purchaseSvc.ListPurchases(reseller.ID, 10)
	if err != nil {
		t.Fatalf("ListPurchases failed: %v", err)
	}
	if len(history) != 2 {
		t.Fatalf("expected 2 purchases, got %d", len(history))
	}
	if history[0].TrafficPackageName != "Pack B" {
		t.Fatalf("expected most recent purchase (Pack B) first, got %s", history[0].TrafficPackageName)
	}
}
