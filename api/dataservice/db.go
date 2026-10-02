package dataservice

import (
	"errors"
	"fmt"
	"log"
	"os"
	"strconv"
	"time"

	"github.com/maahdima/mwp/api/config"
	"github.com/maahdima/mwp/api/dataservice/model"

	"github.com/glebarez/sqlite"
	"gorm.io/driver/postgres"
	"gorm.io/gorm"
	"gorm.io/gorm/logger"
)

func ConnectDB(config config.DBConfig) (db *gorm.DB, err error) {
	conf := new(gorm.Config)
	// A confirmed, reported production incident this fixes: this branch
	// only ever installed a custom logger for MODE=development, so a
	// production run left conf.Logger nil and fell through to GORM's OWN
	// internal default (logger.Config{SlowThreshold: 200*time.Millisecond,
	// LogLevel: logger.Warn}, set in gorm.Open itself) -- the loudest
	// setting GORM ships, not a quieter production default. At real
	// production scale (thousands of WireGuard peers/User Manager
	// accounts/V2Ray locations, RouterOS/x-ui round-trips routinely well
	// past 200ms under load) that meant EVERY such query -- full SQL text,
	// bound parameters, row counts -- was logged to stdout/journald on
	// every sync tick, exactly backwards from what "production mode"
	// should mean. This is also a large share of the syslog/journal disk
	// bloat documented separately (see this codebase's own retention/
	// cleanup notes) and adds real CPU/memory pressure formatting and
	// writing that volume of log lines under the same load that's already
	// slow. Fixed by giving BOTH modes an explicit logger.Error-level,
	// 1s-threshold logger (quiet unless something is genuinely wrong or
	// pathologically slow) -- development differs only in also printing to
	// stdout with colorized output for a human watching a local run;
	// production's is otherwise identical.
	conf.Logger = logger.New(
		log.New(os.Stdout, "\r\n", log.LstdFlags),
		logger.Config{
			SlowThreshold: time.Second,
			LogLevel:      logger.Error,
			// gorm.ErrRecordNotFound is routine, expected control flow
			// throughout this codebase (e.g. resolveV2RaySaleTitle's own
			// "no override configured, fall back to the panel's default"
			// check, called once per V2Ray location on every sync tick) --
			// not a real error worth a log line. Confirmed, reported
			// contributor to the same log-volume/CPU pressure this whole
			// logger config fixes: at fleet scale this alone produced one
			// full-SQL error line per location per tick for the extremely
			// common "reseller has no custom sale title" case.
			IgnoreRecordNotFoundError: true,
			ParameterizedQueries:      false,
			Colorful:                  os.Getenv("MODE") == "development",
		},
	)

	var dialect gorm.Dialector
	if config.Dialect == "sqlite" {
		// journal_mode=WAL lets readers proceed concurrently with the one
		// writer SQLite ever allows at a time (the default rollback-journal
		// mode blocks ALL readers during a write); busy_timeout makes a
		// connection that loses the race for the write lock BLOCK AND RETRY
		// for up to busy_timeout milliseconds instead of failing
		// immediately. Confirmed, reported bug this fixes: with neither
		// pragma set and no cap on how many connections database/sql could
		// open (see SetMaxOpenConns below), concurrent background jobs/
		// requests hitting the DB at the same moment surfaced as "SQL
		// logic error: cannot start a transaction within a transaction"
		// and "database is locked (SQLITE_BUSY)" across completely
		// unrelated services (traffic jobs, security polling, wallet
		// writes, geoip caching) -- this was always a latent risk of this
		// connection setup, not something introduced by any single
		// feature; it simply hadn't been hit hard enough by concurrent
		// load to surface until now.
		//
		// Raised from 5000 to 20000: confirmed, reported follow-up
		// incident at fleet scale (~2500 User Manager accounts, ~1400
		// WireGuard peers, ~700 V2Ray packages) -- CalculateUserManagerUsage,
		// CalculatePeerTraffic, and V2RaySyncService.SyncPackageUsage are
		// three fully independent jobs sharing this same DB, each gated
		// on RouterOS/x-ui network round-trips that can individually take
		// tens of seconds under load (confirmed via journalctl: ~37s
		// average per MonitorUserManagerUser call during a live incident),
		// so a single pass of CalculateUserManagerUsage alone can run for
		// HOURS, not seconds -- these jobs are effectively always
		// overlapping, not just colliding briefly at shared tick
		// boundaries. A 5s busy_timeout is negligible next to a
		// multi-second-to-tens-of-seconds external call, so a writer that
		// loses the race should simply wait rather than error -- 20s
		// gives real headroom for the writer ahead of it (bounded by its
		// own external call's latency, not by this timeout) to finish
		// without the caller ever needing to see SQLITE_BUSY at all in
		// the common case.
		dialect = sqlite.Open(
			fmt.Sprintf("file:%s?_pragma=foreign_keys(1)&_pragma=journal_mode(WAL)&_pragma=busy_timeout(20000)", config.Database),
		)
	} else if config.Dialect == "postgres" {
		var dsn string

		dsn = fmt.Sprintf("host=%s user=%s password=%s dbname=%s port=%s",
			config.Host, config.Username, config.Password, config.Database, config.Port)

		dialect = postgres.New(postgres.Config{DSN: dsn})
	} else {
		err = errors.New("invalid db dialect")
		return
	}

	db, err = gorm.Open(dialect, conf)
	if err != nil {
		return
	}

	if config.Dialect == "sqlite" {
		// SQLite allows exactly one WRITER at a time no matter what, but a
		// pool capped at exactly 1 connection total (write AND read) turned
		// out to be a confirmed, reported production incident of its own:
		// database/sql's connection acquisition blocks with no timeout, so
		// if the single connection is held by a request that's ALSO
		// waiting on a slow/unreachable external call (MikroTik, Telegram
		// -- see newTelegramBotAPI's own doc comment for exactly this),
		// every other DB-touching request in the entire process -- login
		// included -- queues up behind it indefinitely. A small pool (4)
		// gives concurrent reads/short writes somewhere else to go instead
		// of serializing behind one stuck holder, while the busy_timeout
		// pragma above still absorbs the genuine SQLite single-writer
		// conflicts this introduces (a second writer now blocks up to
		// busy_timeout waiting for the first to finish, instead of never
		// being handed a connection to try in the first place).
		//
		// Deliberately NOT raised further -- briefly tried 20 (reasoning:
		// "let userManagerUsagePollConcurrency=20 goroutines each hold
		// their own connection"), but that reasoning was wrong:
		// MonitorUserManagerUser's RouterOS HTTP call
		// (jobs/traffic.go's processUserManagerAccountUsage) happens
		// BEFORE any DB connection is acquired, not while holding one --
		// so a larger pool didn't give slow-external-call goroutines
		// anywhere useful to go, it just let more goroutines pile onto
		// SQLite's one real writer AT THE SAME INSTANT, which measurably
		// increased "database is locked" frequency in production instead
		// of reducing it (confirmed via journalctl: lock-error rate rose
		// after raising this, reverted same day). More connections cannot
		// parallelize SQLite writes; they can only parallelize the wait
		// queue in front of the one writer.
		sqlDB, sqlErr := db.DB()
		if sqlErr != nil {
			err = sqlErr
			return
		}
		sqlDB.SetMaxOpenConns(4)
	}

	return
}

// CurrentSchemaVersion identifies the panel's data-model version. It is
// written to the SystemConfig table after every successful AutoMigrate so
// future versions can detect what state the database is in. This value must
// only ever be bumped forward when new, additive model changes are
// introduced — it is metadata, not a migration trigger, since GORM's
// AutoMigrate is already safe/additive on its own.
const CurrentSchemaVersion = "28"

func AutoMigrate(db *gorm.DB) error {
	if err := dropStaleUserManagerAccountProtocolColumn(db); err != nil {
		log.Panic("failed to drop stale user_manager_accounts.protocol column: ", err)
		return err
	}
	if err := dropStaleTunnelHealthTables(db); err != nil {
		log.Panic("failed to drop stale tunnel-health tables: ", err)
		return err
	}
	if err := dropStaleCurrencyColumns(db); err != nil {
		log.Panic("failed to drop stale currency columns: ", err)
		return err
	}
	if err := addLocationKeyToBillingPrice(db); err != nil {
		log.Panic("failed to add location_key to reseller_billing_prices: ", err)
		return err
	}
	if err := backfillFaoximaDomain(db); err != nil {
		log.Panic("failed to backfill bot_settings.faoxima_domain: ", err)
		return err
	}

	// IMPORTANT: db.Migrator().AutoMigrate only ever creates missing tables
	// and adds missing columns/indexes for the structs listed below — it
	// never drops or truncates existing tables/columns, so existing reseller,
	// wallet, invoice, and peer data is always preserved across upgrades.
	// New model fields must be added to their existing struct (see
	// model.Reseller.MaxPeers for an example) rather than introduced via a
	// separate destructive migration path.
	err := db.Migrator().AutoMigrate(
		&model.Interface{},
		&model.IPPool{},
		&model.Reseller{},
		&model.ResellerInterface{},
		&model.Peer{},
		&model.Traffic{},
		&model.TotalTrafficUsage{},
		&model.PeerDailyUsage{},
		&model.Server{},
		&model.Admin{},
		&model.Wallet{},
		&model.LedgerEntry{},
		&model.Invoice{},
		&model.PricePlan{},
		&model.SystemConfig{},
		&model.AuditLog{},
		&model.BotSettings{},
		&model.BotExtraAdminChatID{},
		&model.SSLSettings{},
		&model.TrafficPackage{},
		&model.PackagePurchase{},
		&model.UserManagerAccount{},
		&model.UserManagerProtocolConfig{},
		&model.ResellerUserManagerGroup{},
		&model.ResellerUserManagerProfile{},
		&model.UserManagerTrafficPackage{},
		&model.UserManagerPackagePurchase{},
		&model.SupportToken{},
		&model.ApiKey{},
		&model.XuiPanel{},
		&model.ResellerV2RaySaleTitle{},
		&model.ResellerXuiPanelAccess{},
		&model.V2RayPackage{},
		&model.V2RayPackageLocation{},
		&model.V2RayTrafficPackage{},
		&model.V2RayPackagePurchase{},
		&model.UsageSnapshot{},
		&model.ResourceSample{},
		&model.IPGeoCache{},
		&model.IPConnectionLog{},
		&model.EtherTrafficSample{},
		&model.ApplicationPlan{},
		&model.Application{},
		&model.ApplicationInterface{},
		&model.ApplicationUserManagerGroup{},
		&model.ApplicationXuiPanel{},
		&model.ApplicationResourceLocation{},
		&model.ApplicationDeviceSession{},
		&model.ApplicationOpenVpnTemplate{},
		&model.OtpChallenge{},
		&model.OtpIpBan{},
		&model.AccountingPartner{},
		&model.AccountingCost{},
		&model.AccountingCustomerPayment{},
		&model.TunnelHealthStatus{},
		&model.TunnelHealthSample{},
		&model.TunnelHealthEvent{},
		&model.TunnelHealthScore{},
		&model.TunnelIncidentDiagnosis{},
		&model.TunnelBackupProbeResult{},
		&model.TunnelPolicy{},
		&model.ManagementRedlineEntry{},
		&model.TunnelActionLog{},
		&model.DiscoverySnapshot{},
		&model.GraphNode{},
		&model.GraphEdge{},
		&model.UserManagerProtocolHealthStatus{},
		&model.UserManagerProtocolHealthEvent{},
		&model.AppVersion{},
		&model.ResellerBillingPrice{},
		&model.ResellerBillingTier{},
		&model.FaoximaInstance{},
		&model.DNSPanel{},
		&model.ResellerDNSPanelAccess{},
		&model.DNSPlan{},
		&model.DNSAccount{},
		&model.DNSAccountAllowedCountry{},
		&model.DNSIPRegistrationLog{},
	)
	if err != nil {
		log.Panic("failed to auto migrate db: ", err)
		return err
	}

	if err := recordSchemaVersion(db); err != nil {
		log.Panic("failed to record schema version: ", err)
		return err
	}

	return nil
}

// dropStaleUserManagerAccountProtocolColumn is a one-time, idempotent
// cleanup for a confirmed production bug: an older revision of
// model.UserManagerAccount had a singular `Protocol` column (`not null`,
// no default), later replaced by the current plural `Protocols` column
// (see that field's own doc comment). AutoMigrate only ever ADDS missing
// columns -- it never drops or renames one that a struct no longer
// declares -- so any database that ever ran that older revision still has
// the stale `protocol` column sitting in place with its NOT NULL
// constraint, and current code has no field to write into it. Every
// INSERT into user_manager_accounts (both admin-direct and reseller
// creation share the same CreateAccount code path, plus the bulk-import
// path) then fails at the database layer with "NOT NULL constraint
// failed: user_manager_accounts.protocol" -- which RouterOS's own side
// then compounds: the RouterOS user was already created successfully
// before this DB write was attempted, so a retry after this failure hits
// "username already exists" against RouterOS itself, even though no
// panel-side account row exists to show for it.
//
// Runs before AutoMigrate, drops the column only if it's still present (a
// database that never had the old schema, or one already cleaned up by a
// prior run, simply no-ops here), and is safe to run on every startup
// indefinitely.
// addLocationKeyToBillingPrice is a one-time, idempotent migration for the
// admin's own reported requirement: per-GB pricing needed to vary by
// location (WireGuard interface / User Manager group / V2Ray panel), not
// just by product, so ResellerBillingPrice gained a LocationKey column
// folded into its uniqueness constraint (see that field's own doc
// comment). Plain AutoMigrate below only ever ADDS a missing column; it
// cannot widen an EXISTING unique index to include a new column, and
// SQLite itself has no ALTER TABLE support for that either -- an
// already-deployed database's idx_reseller_billing_price index would stay
// scoped to (reseller_id, product) forever, silently rejecting the
// location-scoped rows this feature needs (two different locations for
// the same reseller+product would collide as "duplicate"). This function
// detects that stale 2-column index by name and drops it; AutoMigrate,
// which always runs immediately after every one of these dropStale*
// helpers, then recreates idx_reseller_billing_price correctly scoped to
// all three columns from the current model definition. A fresh install
// (or a database already migrated by a prior run) has no such index to
// find, so this simply no-ops.
func addLocationKeyToBillingPrice(db *gorm.DB) error {
	migrator := db.Migrator()
	if !migrator.HasTable(&model.ResellerBillingPrice{}) {
		return nil
	}
	if migrator.HasColumn(&model.ResellerBillingPrice{}, "location_key") {
		return nil
	}
	if migrator.HasIndex(&model.ResellerBillingPrice{}, "idx_reseller_billing_price") {
		if err := migrator.DropIndex(&model.ResellerBillingPrice{}, "idx_reseller_billing_price"); err != nil {
			return err
		}
	}
	return nil
}

// backfillFaoximaDomain is a one-time, idempotent migration for the admin's
// own explicit requirement: the Faoxima ("Bot X") webhook domain used to be
// a single hardcoded constant baked into every install's binary, which is
// wrong for anyone besides the original operator -- each install running
// this panel has its own domain, not the original operator's. BotSettings
// gained a FaoximaDomain column (editable from Settings) to replace it.
//
// Since this column didn't exist before, AutoMigrate would otherwise add it
// as an empty string for every existing install, silently breaking every
// already-provisioned Faoxima instance (their webhook/mini-app URLs would
// suddenly be built against an empty domain). This runs BEFORE AutoMigrate
// so it can tell "the column doesn't exist yet -- this is an upgrade from
// before this feature existed" apart from "the column already exists" (a
// fresh install, or a database already migrated by a prior run, where an
// admin may have deliberately cleared the field) and backfills only the
// former case with the panel's previous hardcoded default, so existing
// installs keep working completely unchanged across the upgrade. A fresh
// install never hits this function's backfill at all -- GetOrCreate's own
// default applies there instead (see BotSettingsService).
func backfillFaoximaDomain(db *gorm.DB) error {
	migrator := db.Migrator()
	if !migrator.HasTable(&model.BotSettings{}) {
		return nil
	}
	if migrator.HasColumn(&model.BotSettings{}, "faoxima_domain") {
		return nil
	}
	if err := migrator.AddColumn(&model.BotSettings{}, "FaoximaDomain"); err != nil {
		return err
	}
	return db.Model(&model.BotSettings{}).
		Where("faoxima_domain = ? OR faoxima_domain IS NULL", "").
		Update("faoxima_domain", previousHardcodedFaoximaDomain).Error
}

// previousHardcodedFaoximaDomain is this panel's own historical hardcoded
// Faoxima webhook domain, before it became an admin-editable BotSettings
// field -- used ONLY by backfillFaoximaDomain, to keep already-provisioned
// Faoxima instances on installs that predate this change working unchanged
// across the upgrade. New installs set their own domain from Settings and
// never see this value.
const previousHardcodedFaoximaDomain = "bot.horanet.ir"

func dropStaleUserManagerAccountProtocolColumn(db *gorm.DB) error {
	migrator := db.Migrator()
	const staleColumnName = "protocol"

	if !migrator.HasTable(&model.UserManagerAccount{}) {
		return nil
	}
	if !migrator.HasColumn(&model.UserManagerAccount{}, staleColumnName) {
		return nil
	}
	return migrator.DropColumn(&model.UserManagerAccount{}, staleColumnName)
}

// dropStaleCurrencyColumns is a one-time-per-table, idempotent cleanup for
// the wallet/billing subsystem (Wallet, Invoice, PricePlan, TrafficPackage,
// PackagePurchase, V2RayTrafficPackage, V2RayPackagePurchase,
// UserManagerTrafficPackage, UserManagerPackagePurchase): these models were
// originally built assuming a USD-with-cents currency, with a `Currency`
// column (`varchar(3)`, defaulting to 'USD') on each one. This panel bills
// exclusively in Toman (Iran's currency, no meaningful sub-unit in
// practical use), so every one of these models has had its Currency field
// removed entirely -- amounts are now plain whole-Toman integers, matching
// the AmountToman convention already established in model/accounting.go.
//
// AutoMigrate only ever ADDS missing columns/tables -- it never drops a
// column a struct no longer declares -- so an already-deployed database
// still has these now-unused `currency` columns sitting around with a
// NOT NULL constraint and a stale 'USD' default. They're harmless to keep
// from a correctness standpoint (nothing reads them anymore), but leaving
// orphaned nullable-in-spirit-but-NOT-NULL columns around silently
// un-migrated is exactly the kind of drift dropStaleUserManagerAccount
// ProtocolColumn above exists to prevent, so they're dropped explicitly
// here instead.
//
// Runs before AutoMigrate, drops each column only if its table and column
// are still present (a fresh install, or a database already cleaned up by
// a prior run, simply no-ops per table), and is safe to run on every
// startup indefinitely -- exactly like dropStaleUserManagerAccount
// ProtocolColumn, this is not gated on a stored schema version because
// checking column existence first already makes repeat runs a no-op.
func dropStaleCurrencyColumns(db *gorm.DB) error {
	migrator := db.Migrator()
	const staleColumnName = "currency"

	tables := []interface{}{
		&model.Wallet{},
		&model.Invoice{},
		&model.PricePlan{},
		&model.TrafficPackage{},
		&model.PackagePurchase{},
		&model.V2RayTrafficPackage{},
		&model.V2RayPackagePurchase{},
		&model.UserManagerTrafficPackage{},
		&model.UserManagerPackagePurchase{},
	}
	for _, table := range tables {
		if !migrator.HasTable(table) {
			continue
		}
		if !migrator.HasColumn(table, staleColumnName) {
			continue
		}
		if err := migrator.DropColumn(table, staleColumnName); err != nil {
			return err
		}
	}
	return nil
}

// dropStaleTunnelHealthTables is a one-time, idempotent cleanup for a
// confirmed conceptual bug in the "tunnel-ai" feature's first shipped
// revision (schema versions 13-15): it monitored customer-facing
// WireGuard PEER interfaces (keyed by a FK into model.Interface) as if
// they were infrastructure "tunnels" -- an admin-reported bug, since a
// genuine infrastructure tunnel (GRE/IPIP/EoIP/etc between servers) has
// no row in model.Interface at all, and an ordinary WireGuard interface
// customers connect to is NOT infrastructure. The corrected model keys
// every one of these tables by the plain RouterOS InterfaceName instead
// (see model/tunnel_health.go's own top-level doc comment) -- a
// genuinely different, incompatible shape, not an additive column
// change AutoMigrate could handle on its own. Since this feature has
// only ever been live for a few days and the admin explicitly confirmed
// discarding this specific data is fine (it was measuring the wrong
// thing in the first place), the simplest and safest fix is to drop and
// let AutoMigrate recreate these four tables fresh, rather than
// attempting a column-rename migration for data that was never correct.
//
// Gated on the stored schema_version being below "16" (this migration's
// own version) so it runs exactly once, not on every future startup --
// unlike dropStaleUserManagerAccountProtocolColumn above, this one is
// NOT safe to repeat indefinitely, since it would otherwise also erase
// legitimate history accumulated after this fix ships.
func dropStaleTunnelHealthTables(db *gorm.DB) error {
	// A confirmed test/startup-ordering bug: on a database that doesn't
	// have the system_configs TABLE yet at all (a genuinely fresh
	// install, or -- as this function's own tests discovered -- any
	// AutoMigrate run against a partially-seeded test database), the
	// query below fails with "no such table" rather than
	// gorm.ErrRecordNotFound, since a query against a missing table is a
	// SQL error, not an empty result set. The RecordNotFound branch right
	// below already treats "nothing stored yet" as "fresh install,
	// nothing stale to drop" -- a missing table means exactly the same
	// thing (there is even less to migrate than an empty table), so it
	// takes that same no-op path instead of panicking the whole
	// AutoMigrate chain.
	if !db.Migrator().HasTable(&model.SystemConfig{}) {
		return nil
	}
	var cfg model.SystemConfig
	err := db.Where("key = ?", "schema_version").First(&cfg).Error
	if err != nil {
		if errors.Is(err, gorm.ErrRecordNotFound) {
			// Fresh install, nothing stale to drop.
			return nil
		}
		return err
	}
	storedVersion, convErr := strconv.Atoi(cfg.Value)
	if convErr != nil || storedVersion >= 16 {
		return nil
	}

	migrator := db.Migrator()
	staleTables := []interface{}{
		&model.TunnelHealthStatus{},
		&model.TunnelHealthSample{},
		&model.TunnelHealthEvent{},
		&model.TunnelPolicy{},
		&model.TunnelActionLog{},
	}
	for _, table := range staleTables {
		if migrator.HasTable(table) {
			if err := migrator.DropTable(table); err != nil {
				return err
			}
		}
	}
	return nil
}

// recordSchemaVersion upserts the current schema version into SystemConfig.
// This is a metadata write only — it never touches any other table.
func recordSchemaVersion(db *gorm.DB) error {
	var existing model.SystemConfig
	err := db.Where("key = ?", "schema_version").First(&existing).Error
	if err == nil {
		existing.Value = CurrentSchemaVersion
		return db.Save(&existing).Error
	}
	if !errors.Is(err, gorm.ErrRecordNotFound) {
		return err
	}

	return db.Create(&model.SystemConfig{Key: "schema_version", Value: CurrentSchemaVersion}).Error
}
