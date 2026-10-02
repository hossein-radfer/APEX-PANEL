package model

import "time"

// BotSettings is a singleton row (always ID 1) holding the panel's Telegram
// bot configuration and per-feature notification toggles. A singleton table
// (rather than a key-value SystemConfig-style store) keeps every field
// strongly typed and matches how admin-configurable behavior toggles are
// modeled elsewhere in this codebase (e.g. Reseller's flat boolean columns).
type BotSettings struct {
	Model
	BotToken    string `gorm:"type:varchar(255)"`
	AdminChatID string `gorm:"type:varchar(64)"` // Telegram chat_id (numeric, as a string) of the super-admin
	Enabled     bool   `gorm:"not null;default:false"`

	// Per-feature toggles. All default false so enabling the bot at all
	// (Enabled=true) does not implicitly start sending every notification
	// type -- an admin opts into each one explicitly.
	NotifyLoginAlerts      bool `gorm:"not null;default:false"`
	NotifyPurchaseReceipts bool `gorm:"not null;default:false"`
	NotifyLiveLog          bool `gorm:"not null;default:false"`
	NotifyQuotaWarnings    bool `gorm:"not null;default:false"`
	NotifyCriticalAlerts   bool `gorm:"not null;default:false"`
	NotifyAccountStatus    bool `gorm:"not null;default:false"`

	// OtpEnabled gates the admin-login two-factor gate (see model.
	// OtpChallenge's own doc comment): when true, a successful admin
	// username/password check does NOT issue tokens directly -- it instead
	// sends a 4-digit code to AdminChatID (and every extra admin recipient)
	// and requires POST /auth/otp/verify before tokens are issued.
	// Deliberately admin-only, never applied to reseller logins (matches
	// the admin's own explicit "OTP is a login gate for the admin account"
	// request) and defaults to false so existing installs are never
	// suddenly locked out of their own panel by an upgrade.
	OtpEnabled bool `gorm:"not null;default:false"`

	// AutoBackupHour/AutoBackupMinute (0-23 / 0-59, in UTC) configure when
	// the bot sends a daily automatic database backup to AdminChatID.
	// AutoBackupEnabled gates whether this scheduled job actually runs.
	// These are set via a bot command (see the panel's Telegram integration
	// docs), not the web settings page.
	AutoBackupEnabled bool `gorm:"not null;default:false"`
	AutoBackupHour    int  `gorm:"not null;default:3"`
	AutoBackupMinute  int  `gorm:"not null;default:0"`

	// AutoReportHour/AutoReportMinute (0-23 / 0-59, in UTC) configure when
	// the bot sends the daily all-resellers usage summary to AdminChatID.
	// AutoReportEnabled gates whether this scheduled job actually runs. Set
	// via the bot's /setreport command, not the web settings page -- same
	// reasoning as the backup schedule: this is something the admin controls
	// from their phone, not a toggle buried in a web settings form.
	AutoReportEnabled bool `gorm:"not null;default:false"`
	AutoReportHour    int  `gorm:"not null;default:9"`
	AutoReportMinute  int  `gorm:"not null;default:0"`

	// LastBackupSentDate/LastReportSentDate (UTC "2006-01-02") record the
	// last calendar day a scheduled backup/report was actually sent, so the
	// once-a-minute scheduler tick can tell "already sent today, skip" apart
	// from "haven't reached today's scheduled time yet" without needing a
	// separate cron library with day-level granularity.
	LastBackupSentDate string `gorm:"type:varchar(10)"`
	LastReportSentDate string `gorm:"type:varchar(10)"`

	// LastUpdateID is the Telegram getUpdates offset checkpoint: the ID of
	// the last update this panel has already processed, so a restart
	// resumes polling from where it left off instead of re-processing (or
	// permanently skipping) updates that arrived while the process was down.
	LastUpdateID int `gorm:"not null;default:0"`

	// Socks5Enabled/Socks5Address/Socks5Username/Socks5Password (item 7):
	// routes ALL outbound Telegram API calls (both this panel's own admin
	// bot AND every reseller's bot-as-a-service instance, see
	// ResellerBotManager) through a SOCKS5 proxy instead of a direct
	// connection -- for a panel installed on an Iran-hosted server, where
	// Telegram's own API is filtered/blocked at the network level, so the
	// bot itself has no way to reach api.telegram.org without one. A
	// single, panel-wide proxy (not per-reseller) since this is a server-
	// network-level constraint of WHERE the panel itself is hosted, not a
	// per-reseller preference. Socks5Address is host:port (e.g.
	// "127.0.0.1:1080"); Socks5Username/Socks5Password are optional (many
	// SOCKS5 proxies require no auth).
	Socks5Enabled  bool   `gorm:"not null;default:false"`
	Socks5Address  string `gorm:"type:varchar(255)"`
	Socks5Username string `gorm:"type:varchar(128)"`
	Socks5Password string `gorm:"type:varchar(255)"`

	// FaoximaDomain is the public domain each Faoxima ("Bot X") instance's
	// Telegram webhook and mini-app links are built under -- this is the
	// operator's own domain, not something that should ever ship hardcoded
	// for every install, since two different operators running this panel
	// must each point their own instances at their own domain. Must be
	// proxied through a CDN/edge network (e.g. Cloudflare) for reliable
	// inbound webhook delivery. Backfilled to the panel's previous
	// hardcoded default on upgrade (see backfillFaoximaDomain in db.go) so
	// existing installs keep working unchanged; new installs must set this
	// from Settings before enabling any Faoxima instance.
	FaoximaDomain string `gorm:"type:varchar(255)"`

	CreatedAt time.Time
	UpdatedAt time.Time
}
