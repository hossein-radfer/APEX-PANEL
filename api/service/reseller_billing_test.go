package service

import (
	"testing"

	"github.com/glebarez/sqlite"
	"github.com/maahdima/mwp/api/dataservice/model"
	"gorm.io/gorm"
)

func openBillingTestDB(t *testing.T) *gorm.DB {
	t.Helper()

	db, err := gorm.Open(sqlite.Open("file::memory:?cache=shared"), &gorm.Config{})
	if err != nil {
		t.Fatalf("failed to open test db: %v", err)
	}

	if err := db.AutoMigrate(
		&model.Reseller{},
		&model.Wallet{},
		&model.LedgerEntry{},
		&model.ResellerBillingPrice{},
		&model.ResellerBillingTier{},
		&model.Peer{},
		&model.UserManagerAccount{},
	); err != nil {
		t.Fatalf("failed to migrate test db: %v", err)
	}

	return db
}

func newPaymentReseller(t *testing.T, db *gorm.DB, name string) model.Reseller {
	t.Helper()
	reseller := model.Reseller{
		Name:            name,
		BillingMode:     model.ResellerBillingModePayment,
		PaymentSubMode:  model.ResellerPaymentSubModePostpaid,
		DebtLimitAmount: nil, // unlimited debt, so a charge is never rejected in these tests
	}
	if err := db.Create(&reseller).Error; err != nil {
		t.Fatalf("failed to create reseller: %v", err)
	}
	// No wallet row needs seeding -- Wallet.recordTransaction
	// auto-creates one with a 0 balance on first use.
	return reseller
}

func walletBalance(t *testing.T, db *gorm.DB, resellerID uint) int64 {
	t.Helper()
	balance, err := NewWallet(db).GetWalletBalance(resellerID)
	if err != nil {
		t.Fatalf("failed to read wallet balance: %v", err)
	}
	return balance
}

// TestChargeUsage_FlatProductWidePrice confirms the baseline behavior:
// with only a product-wide (LocationKey == "") price row, a 2GB delta at
// 1000 Toman/GB debits exactly 2000 Toman regardless of locationKey.
func TestChargeUsage_FlatProductWidePrice(t *testing.T) {
	db := openBillingTestDB(t)
	billing := NewResellerBillingService(db, NewWallet(db))
	reseller := newPaymentReseller(t, db, "flat-price-reseller")

	if err := db.Create(&model.ResellerBillingPrice{
		ResellerID: reseller.ID, Product: "WIREGUARD", LocationKey: "", PricePerGBAmount: 1000,
	}).Error; err != nil {
		t.Fatalf("failed to seed price: %v", err)
	}

	if err := billing.ChargeUsage(reseller.ID, "WIREGUARD", "wg-any-interface", 2*bytesPerGB); err != nil {
		t.Fatalf("ChargeUsage failed: %v", err)
	}

	if got := walletBalance(t, db, reseller.ID); got != -2000 {
		t.Fatalf("expected wallet balance -2000 (2GB * 1000 Toman/GB), got %d", got)
	}
}

// TestChargeUsage_LocationSpecificPriceOverridesFlat confirms the admin's
// own explicit requirement: a WireGuard interface with its own price row
// is charged at THAT rate, not the product-wide fallback, while a
// different interface with no override still uses the fallback.
func TestChargeUsage_LocationSpecificPriceOverridesFlat(t *testing.T) {
	db := openBillingTestDB(t)
	billing := NewResellerBillingService(db, NewWallet(db))
	reseller := newPaymentReseller(t, db, "location-price-reseller")

	if err := db.Create(&model.ResellerBillingPrice{
		ResellerID: reseller.ID, Product: "WIREGUARD", LocationKey: "", PricePerGBAmount: 2000,
	}).Error; err != nil {
		t.Fatalf("failed to seed fallback price: %v", err)
	}
	if err := db.Create(&model.ResellerBillingPrice{
		ResellerID: reseller.ID, Product: "WIREGUARD", LocationKey: "wg-europe", PricePerGBAmount: 500,
	}).Error; err != nil {
		t.Fatalf("failed to seed location price: %v", err)
	}

	// 1GB on the cheaper European interface -- must use 500, not 2000.
	if err := billing.ChargeUsage(reseller.ID, "WIREGUARD", "wg-europe", bytesPerGB); err != nil {
		t.Fatalf("ChargeUsage (europe) failed: %v", err)
	}
	if got := walletBalance(t, db, reseller.ID); got != -500 {
		t.Fatalf("expected -500 after 1GB on wg-europe at its own 500/GB price, got %d", got)
	}

	// 1GB on an interface with no override -- must fall back to 2000.
	if err := billing.ChargeUsage(reseller.ID, "WIREGUARD", "wg-middle-east", bytesPerGB); err != nil {
		t.Fatalf("ChargeUsage (middle-east) failed: %v", err)
	}
	if got := walletBalance(t, db, reseller.ID); got != -2500 {
		t.Fatalf("expected -2500 after the fallback 2000/GB charge on top, got %d", got)
	}
}

// TestChargeUsage_TiersReplaceFlatPricing confirms tiers, once configured
// for a product, are used INSTEAD of any flat/location price row for that
// product (even if one exists) -- see model.ResellerBillingTier's own doc
// comment on why the two never stack.
func TestChargeUsage_TiersReplaceFlatPricing(t *testing.T) {
	db := openBillingTestDB(t)
	billing := NewResellerBillingService(db, NewWallet(db))
	reseller := newPaymentReseller(t, db, "tiered-reseller")

	// A flat price that must be IGNORED once tiers exist for this product.
	if err := db.Create(&model.ResellerBillingPrice{
		ResellerID: reseller.ID, Product: "WIREGUARD", LocationKey: "", PricePerGBAmount: 999999,
	}).Error; err != nil {
		t.Fatalf("failed to seed flat price: %v", err)
	}

	maxGB0to500 := int64(500)
	if err := db.Create(&model.ResellerBillingTier{
		ResellerID: reseller.ID, Product: "WIREGUARD", MinGB: 0, MaxGB: &maxGB0to500, PricePerGBAmount: 1000,
	}).Error; err != nil {
		t.Fatalf("failed to seed tier 1: %v", err)
	}
	if err := db.Create(&model.ResellerBillingTier{
		ResellerID: reseller.ID, Product: "WIREGUARD", MinGB: 500, MaxGB: nil, PricePerGBAmount: 300,
	}).Error; err != nil {
		t.Fatalf("failed to seed tier 2: %v", err)
	}

	// 10GB delta, starting from 0 cumulative usage -- entirely within
	// tier 1 (0-500GB @ 1000/GB) -- must be 10 * 1000 = 10000, NOT the
	// flat 999999/GB price.
	if err := billing.ChargeUsage(reseller.ID, "WIREGUARD", "", 10*bytesPerGB); err != nil {
		t.Fatalf("ChargeUsage failed: %v", err)
	}
	if got := walletBalance(t, db, reseller.ID); got != -10000 {
		t.Fatalf("expected -10000 (10GB @ tier-1 rate 1000/GB), got %d -- tiers did not replace the flat price", got)
	}
}

// TestChargeUsage_TierCrossingSplitsDeltaAcrossBothTiers is the sharpest
// edge case in the whole tiering feature: a single tick's delta that
// pushes the reseller's cumulative usage PAST a tier boundary must be
// priced as two separate portions (the part under the old tier at the old
// rate, the part over at the new rate), not entirely at whichever tier
// the start or end point happens to land in.
func TestChargeUsage_TierCrossingSplitsDeltaAcrossBothTiers(t *testing.T) {
	db := openBillingTestDB(t)
	billing := NewResellerBillingService(db, NewWallet(db))
	reseller := newPaymentReseller(t, db, "boundary-crossing-reseller")

	maxGB0to500 := int64(500)
	if err := db.Create(&model.ResellerBillingTier{
		ResellerID: reseller.ID, Product: "WIREGUARD", MinGB: 0, MaxGB: &maxGB0to500, PricePerGBAmount: 1000,
	}).Error; err != nil {
		t.Fatalf("failed to seed tier 1: %v", err)
	}
	if err := db.Create(&model.ResellerBillingTier{
		ResellerID: reseller.ID, Product: "WIREGUARD", MinGB: 500, MaxGB: nil, PricePerGBAmount: 300,
	}).Error; err != nil {
		t.Fatalf("failed to seed tier 2: %v", err)
	}

	// Put the reseller at exactly 490GB of cumulative WireGuard usage
	// already (simulating prior ticks), matching what chargeAcrossTiers
	// reads via cumulativeUsedBytes -- Reseller.UsedBytes for WIREGUARD.
	if err := db.Model(&model.Reseller{}).Where("id = ?", reseller.ID).
		Update("used_bytes", 490*bytesPerGB).Error; err != nil {
		t.Fatalf("failed to seed prior usage: %v", err)
	}

	// This tick's delta is 20GB: 490 -> 510GB cumulative. The first 10GB
	// (490->500) must be priced at tier 1's 1000/GB; the remaining 10GB
	// (500->510) at tier 2's 300/GB. Expected: 10*1000 + 10*300 = 13000.
	if err := billing.ChargeUsage(reseller.ID, "WIREGUARD", "", 20*bytesPerGB); err != nil {
		t.Fatalf("ChargeUsage failed: %v", err)
	}
	if got := walletBalance(t, db, reseller.ID); got != -13000 {
		t.Fatalf("expected -13000 (10GB@1000 + 10GB@300 split across the tier boundary), got %d", got)
	}
}

// TestChargeUsage_NoConfigChargesNothing confirms the existing, unchanged
// safety behavior: with no price row and no tiers at all for a product,
// usage is not charged (but ChargeUsage itself must not error).
func TestChargeUsage_NoConfigChargesNothing(t *testing.T) {
	db := openBillingTestDB(t)
	billing := NewResellerBillingService(db, NewWallet(db))
	reseller := newPaymentReseller(t, db, "unconfigured-reseller")

	if err := billing.ChargeUsage(reseller.ID, "WIREGUARD", "wg-any", 5*bytesPerGB); err != nil {
		t.Fatalf("ChargeUsage should no-op cleanly with nothing configured, got error: %v", err)
	}
	if got := walletBalance(t, db, reseller.ID); got != 0 {
		t.Fatalf("expected wallet balance unchanged at 0, got %d", got)
	}
}

// TestChargeUsage_SuccessfulDebitResumesStuckBillingSuspended is the
// regression test for a confirmed, reported production incident (reseller
// "Sha", Payment-based/payment-mode, whose V2Ray configs kept auto-disabling
// themselves): BillingSuspended previously had no self-healing path at all
// once a rejected debit set it true -- the ONLY way to clear it was the
// admin's manual wallet-credit HTTP handler explicitly calling
// ResumeBillingSuspension. A reseller who became solvent again through ANY
// other path (here: a later debit that simply succeeds, e.g. after their
// balance was topped up by a different route) needed ChargeUsage itself to
// notice and self-heal the flag via the injected resumer.
func TestChargeUsage_SuccessfulDebitResumesStuckBillingSuspended(t *testing.T) {
	db := openBillingTestDB(t)
	wallet := NewWallet(db)
	billing := NewResellerBillingService(db, wallet)
	resellerSvc := NewReseller(db, nil)
	billing.SetResumer(resellerSvc)

	reseller := newPaymentReseller(t, db, "stuck-suspended-reseller")

	if err := db.Create(&model.ResellerBillingPrice{
		ResellerID: reseller.ID, Product: "V2RAY", LocationKey: "", PricePerGBAmount: 1000,
	}).Error; err != nil {
		t.Fatalf("failed to seed price: %v", err)
	}

	// Simulate the stuck state directly: BillingSuspended=true with no
	// matching debit-rejection having just happened (mirrors a reseller
	// topped up through a path that never called ResumeBillingSuspension).
	if err := db.Model(&model.Reseller{}).Where("id = ?", reseller.ID).
		Update("billing_suspended", true).Error; err != nil {
		t.Fatalf("failed to seed stuck billing_suspended: %v", err)
	}

	// Fund the wallet so the next charge's debit succeeds (Postpaid with a
	// nil DebtLimitAmount would also always succeed, but crediting first
	// makes the "solvent again" scenario explicit and realistic).
	if _, err := wallet.Credit(reseller.ID, 10000, "top-up", nil, nil); err != nil {
		t.Fatalf("failed to credit wallet: %v", err)
	}

	if err := billing.ChargeUsage(reseller.ID, "V2RAY", "", 1*bytesPerGB); err != nil {
		t.Fatalf("ChargeUsage failed: %v", err)
	}

	var got model.Reseller
	if err := db.First(&got, reseller.ID).Error; err != nil {
		t.Fatalf("failed to reload reseller: %v", err)
	}
	if got.BillingSuspended {
		t.Fatalf("expected BillingSuspended to self-heal to false after a successful debit, still true")
	}
}

// TestChargeUsage_NoResumerConfiguredStillSucceeds confirms SetResumer is a
// genuinely optional collaborator: leaving it unset must never break a
// successful charge, even for a reseller stuck BillingSuspended=true.
func TestChargeUsage_NoResumerConfiguredStillSucceeds(t *testing.T) {
	db := openBillingTestDB(t)
	billing := NewResellerBillingService(db, NewWallet(db))
	reseller := newPaymentReseller(t, db, "no-resumer-reseller")

	if err := db.Create(&model.ResellerBillingPrice{
		ResellerID: reseller.ID, Product: "V2RAY", LocationKey: "", PricePerGBAmount: 1000,
	}).Error; err != nil {
		t.Fatalf("failed to seed price: %v", err)
	}
	if err := db.Model(&model.Reseller{}).Where("id = ?", reseller.ID).
		Update("billing_suspended", true).Error; err != nil {
		t.Fatalf("failed to seed stuck billing_suspended: %v", err)
	}

	if err := billing.ChargeUsage(reseller.ID, "V2RAY", "", 1*bytesPerGB); err != nil {
		t.Fatalf("ChargeUsage should still succeed with no resumer configured, got error: %v", err)
	}
}
