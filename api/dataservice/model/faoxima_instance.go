package model

import "time"

// FaoximaInstance tracks one reseller's own full Faoxima Telegram bot
// deployment -- the "bot-as-a-service" feature, upgraded from an earlier,
// much simpler Go-native bot to a genuinely complete Faoxima instance per
// reseller's own explicit request: the reseller gets a fully-featured
// sales bot (protocol/volume/location/price flows, wallet, invoicing --
// everything Faoxima itself already does) by entering only their own
// Telegram bot token and chat ID, never receiving Faoxima's PHP source or
// needing their own separate hosting.
//
// Each instance is a FULL, INDEPENDENT COPY of the Faoxima codebase
// (copied from a fixed template directory at provision time, never a
// shared codebase multiplexed by config -- see
// FaoximaProvisionerService's own doc comment for why this was chosen
// over a shared-codebase approach) plus its own dedicated MySQL database
// and MySQL user, so one reseller's instance can never read or corrupt
// another's data purely by filesystem/DB permission boundaries, with no
// runtime tenant-switching logic to get wrong.
type FaoximaInstance struct {
	Model
	ResellerID uint `gorm:"uniqueIndex;not null"` // one instance per reseller, matches ResellerBillingPrice's own one-row-per-reseller convention

	// InstanceSlug is the filesystem-safe identifier used both for the
	// instance's own directory name (under the provisioner's configured
	// base path, e.g. /opt/faoxima-instances/reseller-42) and the
	// webhook URL subpath -- derived once at provision time from
	// ResellerID (never from the reseller's own name/username, which can
	// change), so it never needs to be regenerated later.
	InstanceSlug string `gorm:"type:varchar(64);uniqueIndex;not null"`

	// DBName/DBUser/DBPassword are this instance's own dedicated MySQL
	// database and user, stored in plain text -- matching this codebase's
	// own existing convention for secrets-at-rest (e.g.
	// BotSettings.Socks5Password, BotSettings.BotToken), which relies on
	// filesystem/DB-file permissions rather than field-level encryption.
	// A dedicated user (never the shared MySQL root/admin account) is
	// GRANTed privileges scoped to ONLY this one database, so a bug or
	// compromise in one reseller's Faoxima instance can never read or
	// write another instance's or the main panel's own data.
	DBName     string `gorm:"type:varchar(64);not null"`
	DBUser     string `gorm:"type:varchar(64);not null"`
	DBPassword string `gorm:"type:text;not null"`

	// BotToken/AdminChatID/WebhookSecret mirror Faoxima's own config.php
	// fields ($APIKEY/$adminnumber/$secrettoken). WebhookSecret is
	// generated once at provision time and passed to Telegram's
	// setWebhook as secret_token, exactly matching Faoxima's own
	// install.sh convention (see that file's install_additional_bot
	// function) of storing it as this instance's own `admin.password` row
	// for the webhook auth check.
	BotToken      string `gorm:"type:text;not null"`
	AdminChatID   string `gorm:"type:varchar(64)"`
	WebhookSecret string `gorm:"type:varchar(64);not null"`

	// Status tracks provisioning lifecycle -- see the FaoximaInstanceStatus
	// constants below. A reseller only ever sees Enabled/Disabled/Error
	// from the panel UI; Provisioning is a transient state while the
	// (potentially slow: DB creation + file copy + webhook registration)
	// provisioning sequence runs.
	Status       string  `gorm:"type:varchar(16);not null;default:'PROVISIONING'"`
	ErrorMessage *string `gorm:"type:text"` // set when Status=ERROR, cleared on next successful operation

	// BillingPeriodDays/PeriodActivatedAt implement the admin's own
	// explicit request: running a reseller's Faoxima instance costs the
	// PANEL real resources (bandwidth + memory for a full separate PHP
	// deployment), so it must not be free/indefinite for every reseller
	// by default -- "درست نیست که رایگان برای همه نمایندگان باشه." An
	// admin sets BillingPeriodDays (e.g. 30, matching a reseller's own
	// payment cycle) when enabling an instance; PeriodActivatedAt is
	// stamped at that same moment. FaoximaBillingService.CheckExpiredPeriods
	// (run on its own schedule) disables any ENABLED instance whose
	// period has elapsed, requiring the admin's own explicit "دوره را
	// ریست کن" action (AdminResetPeriod) -- which re-stamps
	// PeriodActivatedAt to now and re-enables -- before that reseller's
	// bot resumes, rather than silently auto-renewing forever. Nil
	// BillingPeriodDays means "no billing period configured" (the
	// instance stays enabled indefinitely, e.g. before an admin has
	// opted this reseller into the period-based model at all) --
	// existing instances created before this field existed default to
	// nil/unlimited so they are never unexpectedly disabled by a
	// deploy.
	BillingPeriodDays *int       `gorm:"type:int"`
	PeriodActivatedAt *time.Time `gorm:""`

	CreatedAt time.Time
	UpdatedAt time.Time
}

const (
	FaoximaInstanceStatusProvisioning = "PROVISIONING"
	FaoximaInstanceStatusEnabled      = "ENABLED"
	FaoximaInstanceStatusDisabled     = "DISABLED"
	FaoximaInstanceStatusError        = "ERROR"
)
