package dataservice

import (
	"fmt"
	"testing"
	"time"

	"github.com/glebarez/sqlite"
	"gorm.io/gorm"

	"github.com/maahdima/mwp/api/dataservice/model"
)

// oldUserManagerAccount reproduces the PRE-FIX shape: every column the
// CURRENT model.UserManagerAccount already has, PLUS a leftover `Protocol`
// NOT NULL column (from before the field was renamed/pluralized to
// Protocols) -- simulating an already-deployed production database that
// has been through every migration up to, but not including, this fix.
type oldUserManagerAccount struct {
	model.Model
	UUID                  string  `gorm:"type:varchar(36);uniqueIndex;not null"`
	Username              string  `gorm:"type:varchar(255);uniqueIndex;not null"`
	Password              string  `gorm:"type:varchar(255);not null"`
	RouterOSUserID        string  `gorm:"type:varchar(64);not null"`
	RouterOSProfileLinkID *string `gorm:"type:varchar(64)"`
	Group                 string  `gorm:"type:varchar(255);not null"`
	Profile               string  `gorm:"type:varchar(255);not null"`
	Protocols             string  `gorm:"type:varchar(64);not null;default:''"`
	Protocol              string  `gorm:"type:varchar(32);not null"` // the stale column
	SharedUsers           int     `gorm:"type:int;not null;default:1"`
}

func (oldUserManagerAccount) TableName() string { return "user_manager_accounts" }

// TestAutoMigrate_DropsStaleUserManagerAccountProtocolColumn is a
// regression test for a confirmed, reported bug: creating a User Manager
// account (both admin-direct and reseller-created share the same code
// path) failed with "failed to store user manager account: constraint
// failed: NOT NULL constraint failed: user_manager_accounts.protocol" --
// a leftover column from an older schema revision that current code has
// no field for. AutoMigrate alone never drops a column a struct no longer
// declares, so an already-deployed database needed an explicit one-time
// cleanup step (dropStaleUserManagerAccountProtocolColumn) to self-heal.
//
// This simulates exactly that: builds a DB on the OLD schema (with the
// stale NOT NULL column), confirms the bug reproduces via a real INSERT
// through the CURRENT model (which has no value for that column), runs
// the real AutoMigrate (with the fix), and confirms both that existing
// data survived and that account creation now succeeds.
func TestAutoMigrate_DropsStaleUserManagerAccountProtocolColumn(t *testing.T) {
	dsn := fmt.Sprintf("file:um_protocol_migration_test_%d?mode=memory&cache=shared", time.Now().UnixNano())
	db, err := gorm.Open(sqlite.Open(dsn), &gorm.Config{})
	if err != nil {
		t.Fatalf("failed to open db: %v", err)
	}

	// Simulate an old, already-deployed DB with the stale NOT NULL
	// `protocol` column already in place, seeded with one pre-existing,
	// fully-populated account row (must survive the fix).
	if err := db.AutoMigrate(&oldUserManagerAccount{}); err != nil {
		t.Fatalf("failed to migrate old shape: %v", err)
	}
	if err := db.Create(&oldUserManagerAccount{
		UUID: "uuid-existing", Username: "existing-user", Password: "x",
		RouterOSUserID: "*existing", Group: "g", Profile: "p",
		Protocols: "l2tp", Protocol: "l2tp", SharedUsers: 1,
	}).Error; err != nil {
		t.Fatalf("failed to seed old row: %v", err)
	}

	// Confirm the bug reproduces: inserting through the CURRENT model
	// (which has no value to give the old NOT NULL `protocol` column)
	// must fail on the old schema.
	newAccountAttempt := model.UserManagerAccount{
		UUID: "uuid-new", Username: "new-user", Password: "x",
		RouterOSUserID: "*1", Group: "g", Profile: "p", Protocols: "l2tp",
	}
	if err := db.Create(&newAccountAttempt).Error; err == nil {
		t.Fatal("expected the OLD schema to reject an insert with no value for the stale NOT NULL protocol column (this proves the bug reproduces)")
	}

	// Now run the real fix's migration path against this same DB.
	if err := AutoMigrate(db); err != nil {
		t.Fatalf("AutoMigrate (with the fix) failed: %v", err)
	}

	// The original row must have survived (column drop must never lose
	// other data in the row/table).
	var count int64
	if err := db.Model(&model.UserManagerAccount{}).Where("username = ?", "existing-user").Count(&count).Error; err != nil {
		t.Fatalf("failed to count existing rows: %v", err)
	}
	if count != 1 {
		t.Fatalf("expected the original row to survive migration, got count=%d", count)
	}

	// The actual bug: creating a new account (the exact operation that
	// was failing) must now succeed.
	if err := db.Create(&model.UserManagerAccount{
		UUID: "uuid-new-2", Username: "new-user-2", Password: "x",
		RouterOSUserID: "*2", Group: "g", Profile: "p", Protocols: "l2tp,pptp",
	}).Error; err != nil {
		t.Fatalf("expected account creation to succeed after the fix, got error: %v", err)
	}
}

// TestAutoMigrate_FreshDatabaseNoOpsCleanly confirms the stale-column-drop
// step is safe to run against a database that never had the old bug (a
// brand-new install) -- it must simply no-op, not error, and running
// AutoMigrate twice (simulating a restart) must also be safe.
func TestAutoMigrate_FreshDatabaseNoOpsCleanly(t *testing.T) {
	dsn := fmt.Sprintf("file:um_protocol_fresh_migration_test_%d?mode=memory&cache=shared", time.Now().UnixNano())
	db, err := gorm.Open(sqlite.Open(dsn), &gorm.Config{})
	if err != nil {
		t.Fatalf("failed to open db: %v", err)
	}

	if err := AutoMigrate(db); err != nil {
		t.Fatalf("expected AutoMigrate to succeed cleanly on a fresh database, got: %v", err)
	}
	if err := AutoMigrate(db); err != nil {
		t.Fatalf("expected a second AutoMigrate run to succeed cleanly, got: %v", err)
	}

	if err := db.Create(&model.UserManagerAccount{
		UUID: "uuid-fresh", Username: "fresh-user", Password: "x",
		RouterOSUserID: "*1", Group: "g", Profile: "p", Protocols: "l2tp",
	}).Error; err != nil {
		t.Fatalf("expected a normal insert to succeed on a fresh, migrated database: %v", err)
	}
}

// oldResellerBillingPrice reproduces the PRE-FIX shape: no LocationKey
// column, and idx_reseller_billing_price scoped to just (reseller_id,
// product) -- simulating an already-deployed production database from
// before per-location pricing existed.
type oldResellerBillingPrice struct {
	model.Model
	ResellerID       uint  `gorm:"uniqueIndex:idx_reseller_billing_price;not null"`
	Product          string `gorm:"type:varchar(16);uniqueIndex:idx_reseller_billing_price;not null"`
	PricePerGBAmount int64  `gorm:"type:bigint;not null;default:0"`
}

func (oldResellerBillingPrice) TableName() string { return "reseller_billing_prices" }

// TestAutoMigrate_WidensResellerBillingPriceIndexForLocationKey is a
// regression test for the admin's own reported requirement: per-GB pricing
// needed to vary by location (WireGuard interface / User Manager group /
// V2Ray panel), not just by product. On an already-deployed database, the
// old 2-column unique index would reject two different locations' price
// rows for the same reseller+product as duplicates unless the migration
// widens it first. This simulates exactly that: builds a DB on the OLD
// schema, confirms two location-scoped rows for the same reseller+product
// collide (reproducing what would otherwise silently break this feature),
// runs the real AutoMigrate (with the fix), and confirms both that
// existing data survived and that two distinct locations can now each
// have their own price.
func TestAutoMigrate_WidensResellerBillingPriceIndexForLocationKey(t *testing.T) {
	dsn := fmt.Sprintf("file:billing_price_location_migration_test_%d?mode=memory&cache=shared", time.Now().UnixNano())
	db, err := gorm.Open(sqlite.Open(dsn), &gorm.Config{})
	if err != nil {
		t.Fatalf("failed to open db: %v", err)
	}

	if err := db.AutoMigrate(&oldResellerBillingPrice{}); err != nil {
		t.Fatalf("failed to migrate old shape: %v", err)
	}
	if err := db.Create(&oldResellerBillingPrice{
		ResellerID: 1, Product: "WIREGUARD", PricePerGBAmount: 1000,
	}).Error; err != nil {
		t.Fatalf("failed to seed old row: %v", err)
	}

	// Confirm the bug reproduces: on the OLD schema, a second row for the
	// same reseller+product (representing a different location, once the
	// column exists) collides with the old 2-column unique index.
	if err := db.Exec(
		"INSERT INTO reseller_billing_prices (reseller_id, product, location_key, price_per_gb_amount, created_at, updated_at) VALUES (1, 'WIREGUARD', 'wg-europe', 500, ?, ?)",
		time.Now(), time.Now(),
	).Error; err == nil {
		t.Fatal("expected the OLD 2-column index to reject a second row before location_key even exists as a real column")
	}

	// Now run the real fix's migration path against this same DB.
	if err := AutoMigrate(db); err != nil {
		t.Fatalf("AutoMigrate (with the fix) failed: %v", err)
	}

	// The original row must have survived (index rebuild must never lose
	// existing data).
	var count int64
	if err := db.Model(&model.ResellerBillingPrice{}).Where("reseller_id = ? AND product = ?", 1, "WIREGUARD").Count(&count).Error; err != nil {
		t.Fatalf("failed to count existing rows: %v", err)
	}
	if count != 1 {
		t.Fatalf("expected the original row to survive migration, got count=%d", count)
	}

	// The actual bug: two location-scoped rows for the same reseller+
	// product must now both succeed.
	if err := db.Create(&model.ResellerBillingPrice{
		ResellerID: 1, Product: "WIREGUARD", LocationKey: "wg-europe", PricePerGBAmount: 500,
	}).Error; err != nil {
		t.Fatalf("expected a location-scoped price row to succeed after the fix, got error: %v", err)
	}
	if err := db.Create(&model.ResellerBillingPrice{
		ResellerID: 1, Product: "WIREGUARD", LocationKey: "wg-middle-east", PricePerGBAmount: 1500,
	}).Error; err != nil {
		t.Fatalf("expected a second, different location-scoped price row to succeed after the fix, got error: %v", err)
	}

	// A duplicate (same reseller+product+location) must still correctly
	// be rejected -- the fix must widen the index, not remove it.
	if err := db.Create(&model.ResellerBillingPrice{
		ResellerID: 1, Product: "WIREGUARD", LocationKey: "wg-europe", PricePerGBAmount: 999,
	}).Error; err == nil {
		t.Fatal("expected a true duplicate (same reseller+product+location) to still be rejected by the widened unique index")
	}
}
