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

// oldUserManagerProtocolConfig reproduces the PRE-FIX shape: no ServerID
// column, and a single-column unique index on Protocol alone -- simulating
// an already-deployed production database from before this table supported
// per-server protocol settings (see model.UserManagerProtocolConfig's own
// doc comment for the customer-facing bug this fixes).
type oldUserManagerProtocolConfig struct {
	model.Model
	Protocol string `gorm:"type:varchar(16);uniqueIndex;not null"`
	Port     int    `gorm:"type:int;not null"`
}

func (oldUserManagerProtocolConfig) TableName() string { return "user_manager_protocol_configs" }

// TestAutoMigrate_AddsServerIDToUserManagerProtocolConfig is a regression
// test for a confirmed, reported bug: UserManagerProtocolConfig had exactly
// one row per protocol SYSTEM-WIDE, so a customer whose account lived on a
// server other than whichever one the admin last edited settings for was
// silently shown the WRONG port/address on their own connection-info share
// page. This simulates an already-deployed single-server database (old
// schema, one L2TP config row, old single-column unique index), runs the
// real AutoMigrate (with the fix), and confirms: the existing row survived
// with the correct backfilled ServerID, a second server can now configure
// the SAME protocol independently (the actual bug), and a true duplicate
// (same server+protocol) is still correctly rejected.
func TestAutoMigrate_AddsServerIDToUserManagerProtocolConfig(t *testing.T) {
	dsn := fmt.Sprintf("file:um_protocol_config_server_id_test_%d?mode=memory&cache=shared", time.Now().UnixNano())
	db, err := gorm.Open(sqlite.Open(dsn), &gorm.Config{})
	if err != nil {
		t.Fatalf("failed to open db: %v", err)
	}

	if err := db.AutoMigrate(&model.Server{}); err != nil {
		t.Fatalf("failed to migrate Server table: %v", err)
	}
	server1 := model.Server{Name: "server-one", IPAddress: "10.0.0.1", Username: "admin", Password: "pw"}
	if err := db.Create(&server1).Error; err != nil {
		t.Fatalf("failed to create the single pre-existing server: %v", err)
	}

	if err := db.AutoMigrate(&oldUserManagerProtocolConfig{}); err != nil {
		t.Fatalf("failed to migrate old shape: %v", err)
	}
	if err := db.Create(&oldUserManagerProtocolConfig{Protocol: "l2tp", Port: 1701}).Error; err != nil {
		t.Fatalf("failed to seed old row: %v", err)
	}

	// Confirm the bug reproduces: on the OLD schema, a second row for the
	// same protocol (representing a different server, once the column
	// exists) collides with the old single-column unique index.
	if err := db.Exec(
		"INSERT INTO user_manager_protocol_configs (protocol, port, created_at, updated_at) VALUES ('l2tp', 1702, ?, ?)",
		time.Now(), time.Now(),
	).Error; err == nil {
		t.Fatal("expected the OLD single-column index to reject a second row before server_id even exists as a real column")
	}

	// Mark this database as being on schema 29 (the version immediately
	// before this fix), matching a real already-upgraded single-server
	// install, then run the real fix's migration path.
	if err := db.AutoMigrate(&model.SystemConfig{}); err != nil {
		t.Fatalf("failed to migrate SystemConfig table: %v", err)
	}
	if err := db.Create(&model.SystemConfig{Key: "schema_version", Value: "29"}).Error; err != nil {
		t.Fatalf("failed to seed schema_version: %v", err)
	}

	if err := AutoMigrate(db); err != nil {
		t.Fatalf("AutoMigrate (with the fix) failed: %v", err)
	}

	// The original row must have survived, correctly backfilled to the
	// single pre-existing server (index rebuild must never lose existing
	// data or misattribute it).
	var existing model.UserManagerProtocolConfig
	if err := db.Where("protocol = ?", "l2tp").First(&existing).Error; err != nil {
		t.Fatalf("expected the original row to survive migration: %v", err)
	}
	if existing.ServerID != server1.ID {
		t.Fatalf("expected the original row to be backfilled to server %d, got %d", server1.ID, existing.ServerID)
	}
	if existing.Port != 1701 {
		t.Fatalf("expected the original row's port to be preserved, got %d", existing.Port)
	}

	// The actual bug: a second server can now configure the SAME protocol
	// independently, with its own port.
	server2 := model.Server{Name: "server-two", IPAddress: "10.0.0.2", Username: "admin", Password: "pw"}
	if err := db.Create(&server2).Error; err != nil {
		t.Fatalf("failed to create second server: %v", err)
	}
	if err := db.Create(&model.UserManagerProtocolConfig{
		ServerID: server2.ID, Protocol: "l2tp", Port: 1702,
	}).Error; err != nil {
		t.Fatalf("expected a second server's config for the same protocol to succeed after the fix, got error: %v", err)
	}

	// A true duplicate (same server+protocol) must still correctly be
	// rejected -- the fix must widen the index, not remove it.
	if err := db.Create(&model.UserManagerProtocolConfig{
		ServerID: server1.ID, Protocol: "l2tp", Port: 9999,
	}).Error; err == nil {
		t.Fatal("expected a true duplicate (same server+protocol) to still be rejected by the widened unique index")
	}
}

// TestAutoMigrate_UserManagerProtocolConfigMigrationOnAlreadyMultiServerInstall
// is a regression test for a confirmed PRODUCTION INCIDENT caused by an
// earlier version of addServerIDToUserManagerProtocolConfig: that version
// copied backfillServerIDForSingleServerInstall's "refuse if more than one
// server exists" guard verbatim, which is correct for a migration that
// ships AT THE SAME TIME multi-server support itself is introduced (more
// than one server at that exact moment is a genuine inconsistency), but
// WRONG for a migration introduced well after multi-server support already
// shipped and is in active real-world use -- there, 2+ servers is the
// normal, expected case. That earlier version made this table's migration
// panic (and permanently crash-loop, since AutoMigrate runs on every
// process start) on every already-multi-server production install, even
// though the table itself had zero existing rows to attribute to anyone.
//
// This simulates exactly the real incident: a database already upgraded
// past schema 29 (multi-server support already live) with TWO servers
// already registered, and the UserManagerProtocolConfig table not yet
// created at all (a protocol has never been configured yet) -- confirms
// AutoMigrate succeeds (does NOT panic/error) and the table ends up with
// the current, server-scoped schema ready to use.
func TestAutoMigrate_UserManagerProtocolConfigMigrationOnAlreadyMultiServerInstall(t *testing.T) {
	dsn := fmt.Sprintf("file:um_protocol_config_already_multiserver_test_%d?mode=memory&cache=shared", time.Now().UnixNano())
	db, err := gorm.Open(sqlite.Open(dsn), &gorm.Config{})
	if err != nil {
		t.Fatalf("failed to open db: %v", err)
	}

	// First boot: a single-server install upgrading through 28->29, exactly
	// like TestAutoMigrate_SecondServerAddedAfterMigrationDoesNotCrashOnRestart's
	// own setup.
	if err := AutoMigrate(db); err != nil {
		t.Fatalf("expected the initial AutoMigrate (fresh install) to succeed, got: %v", err)
	}
	server1 := model.Server{Name: "server-one", IPAddress: "10.0.0.1", Username: "admin", Password: "pw"}
	if err := db.Create(&server1).Error; err != nil {
		t.Fatalf("failed to create first server: %v", err)
	}
	server2 := model.Server{Name: "server-two", IPAddress: "10.0.0.2", Username: "admin", Password: "pw"}
	if err := db.Create(&server2).Error; err != nil {
		t.Fatalf("failed to create second server: %v", err)
	}

	// The admin never configured any User Manager protocol settings before
	// now -- this table has zero rows, exactly like the real incident.
	var count int64
	if err := db.Model(&model.UserManagerProtocolConfig{}).Count(&count).Error; err != nil {
		t.Fatalf("failed to count existing rows: %v", err)
	}
	if count != 0 {
		t.Fatalf("expected zero pre-existing protocol config rows, got %d", count)
	}

	// THE INCIDENT: this version's protocol-config migration runs for the
	// first time with 2 servers already registered. It must NOT panic/error
	// just because more than one server exists -- there is nothing
	// ambiguous to attribute since the table is empty.
	if err := AutoMigrate(db); err != nil {
		t.Fatalf("expected AutoMigrate to succeed on an already-multi-server install with no existing protocol config rows (this is the confirmed production incident), got: %v", err)
	}

	// The table must now be fully usable in its current, server-scoped
	// shape: both servers can configure the same protocol independently.
	if err := db.Create(&model.UserManagerProtocolConfig{
		ServerID: server1.ID, Protocol: "l2tp", Port: 1701,
	}).Error; err != nil {
		t.Fatalf("expected server one's config to succeed after migration, got error: %v", err)
	}
	if err := db.Create(&model.UserManagerProtocolConfig{
		ServerID: server2.ID, Protocol: "l2tp", Port: 1702,
	}).Error; err != nil {
		t.Fatalf("expected server two's config for the same protocol to succeed after migration, got error: %v", err)
	}

	// A further restart must stay idempotent.
	if err := AutoMigrate(db); err != nil {
		t.Fatalf("expected a further AutoMigrate run to keep succeeding, got: %v", err)
	}
}

// TestAutoMigrate_UserManagerProtocolConfigRefusesAmbiguousMultiServerBackfill
// confirms the genuinely ambiguous case is still correctly refused rather
// than guessed: pre-existing, UNSCOPED protocol config rows (old schema)
// with more than one server already registered has no single unambiguous
// server to attribute them to, and must error out for manual resolution --
// exactly what happened on the real production database this migration was
// fixed against (4 pre-existing rows from before multi-server adoption, 2
// servers registered by the time this migration finally ran).
func TestAutoMigrate_UserManagerProtocolConfigRefusesAmbiguousMultiServerBackfill(t *testing.T) {
	dsn := fmt.Sprintf("file:um_protocol_config_ambiguous_test_%d?mode=memory&cache=shared", time.Now().UnixNano())
	db, err := gorm.Open(sqlite.Open(dsn), &gorm.Config{})
	if err != nil {
		t.Fatalf("failed to open db: %v", err)
	}

	if err := db.AutoMigrate(&model.Server{}); err != nil {
		t.Fatalf("failed to migrate Server table: %v", err)
	}
	if err := db.Create(&model.Server{Name: "server-one", IPAddress: "10.0.0.1", Username: "admin", Password: "pw"}).Error; err != nil {
		t.Fatalf("failed to create first server: %v", err)
	}
	if err := db.Create(&model.Server{Name: "server-two", IPAddress: "10.0.0.2", Username: "admin", Password: "pw"}).Error; err != nil {
		t.Fatalf("failed to create second server: %v", err)
	}

	// Pre-existing, unscoped rows from before multi-server adoption -- the
	// exact real-world shape: configured once, long before a second server
	// was ever added.
	if err := db.AutoMigrate(&oldUserManagerProtocolConfig{}); err != nil {
		t.Fatalf("failed to migrate old shape: %v", err)
	}
	if err := db.Create(&oldUserManagerProtocolConfig{Protocol: "l2tp", Port: 500}).Error; err != nil {
		t.Fatalf("failed to seed old l2tp row: %v", err)
	}
	if err := db.Create(&oldUserManagerProtocolConfig{Protocol: "sstp", Port: 3679}).Error; err != nil {
		t.Fatalf("failed to seed old sstp row: %v", err)
	}

	if err := db.AutoMigrate(&model.SystemConfig{}); err != nil {
		t.Fatalf("failed to migrate SystemConfig table: %v", err)
	}
	if err := db.Create(&model.SystemConfig{Key: "schema_version", Value: "29"}).Error; err != nil {
		t.Fatalf("failed to seed schema_version: %v", err)
	}

	// AutoMigrate panics (via log.Panic) on this refusal, by design --
	// exactly like every other guard in this file (see
	// backfillServerIDForSingleServerInstall's own identical convention) --
	// so the real binary refuses to start with ambiguous data rather than
	// silently guessing. Recover here to assert on it instead of crashing
	// the test process.
	defer func() {
		r := recover()
		if r == nil {
			t.Fatal("expected AutoMigrate to panic/refuse (not silently guess) when existing unscoped rows meet more than one already-registered server")
		}
	}()
	_ = AutoMigrate(db)
}

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

// TestAutoMigrate_SecondServerAddedAfterMigrationDoesNotCrashOnRestart is a
// regression test for a confirmed production incident: the panel entered a
// permanent crash loop the moment an admin added a second server AFTER
// already upgrading past schema 29 on a single-server install.
// backfillServerIDForSingleServerInstall/backfillBillingLocationKeyWithServerID
// both refuse (return an error, which panics AutoMigrate) when they see
// more than one Server row -- a guard that is correct and necessary DURING
// the 28->29 upgrade itself (more than one server at that point means there
// is no reliable single server to attribute existing rows to), but which
// must NEVER fire again once that upgrade has already completed once,
// since every server added afterward is normal, expected multi-server
// growth, not leftover unattributed data. Confirms AutoMigrate runs clean
// on first boot with one server (the real 28->29 upgrade), a second server
// is then added exactly the way CreateServer does it, and a SUBSEQUENT
// AutoMigrate run (every future process restart) still succeeds instead of
// erroring.
func TestAutoMigrate_SecondServerAddedAfterMigrationDoesNotCrashOnRestart(t *testing.T) {
	dsn := fmt.Sprintf("file:second_server_restart_test_%d?mode=memory&cache=shared", time.Now().UnixNano())
	db, err := gorm.Open(sqlite.Open(dsn), &gorm.Config{})
	if err != nil {
		t.Fatalf("failed to open db: %v", err)
	}

	// First boot: a single-server install upgrading through 28->29.
	if err := AutoMigrate(db); err != nil {
		t.Fatalf("expected the initial AutoMigrate (fresh install) to succeed, got: %v", err)
	}

	server1 := model.Server{Name: "server-one", IPAddress: "10.0.0.1", Username: "admin", Password: "pw"}
	if err := db.Create(&server1).Error; err != nil {
		t.Fatalf("failed to create first server: %v", err)
	}

	// A restart with exactly one server present must still succeed (this
	// already worked before the fix, confirming the gate doesn't change
	// behavior for the still-single-server case).
	if err := AutoMigrate(db); err != nil {
		t.Fatalf("expected AutoMigrate to succeed with one server present, got: %v", err)
	}

	// The admin now adds a second server through the panel -- completely
	// normal multi-server growth, not a migration artifact.
	server2 := model.Server{Name: "server-two", IPAddress: "10.0.0.2", Username: "admin", Password: "pw"}
	if err := db.Create(&server2).Error; err != nil {
		t.Fatalf("failed to create second server: %v", err)
	}

	// THE BUG: every subsequent process restart re-runs AutoMigrate, which
	// must NOT error now that two servers legitimately exist.
	if err := AutoMigrate(db); err != nil {
		t.Fatalf("expected AutoMigrate to succeed after a second server was added post-migration (this is the confirmed crash-loop bug), got: %v", err)
	}

	// One more restart for good measure -- must stay idempotent forever,
	// not just survive exactly one extra run.
	if err := AutoMigrate(db); err != nil {
		t.Fatalf("expected a further AutoMigrate run to keep succeeding, got: %v", err)
	}
}
