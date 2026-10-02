package schema

// BotSettingsResponse never includes BotToken in full -- see
// BotSettingsController.GetSettings for why it's masked.
type BotSettingsResponse struct {
	BotTokenSet            bool   `json:"bot_token_set"`
	BotTokenMasked         string `json:"bot_token_masked"`
	AdminChatID            string `json:"admin_chat_id"`
	Enabled                bool   `json:"enabled"`
	NotifyLoginAlerts      bool   `json:"notify_login_alerts"`
	NotifyPurchaseReceipts bool   `json:"notify_purchase_receipts"`
	NotifyLiveLog          bool   `json:"notify_live_log"`
	NotifyQuotaWarnings    bool   `json:"notify_quota_warnings"`
	NotifyCriticalAlerts   bool   `json:"notify_critical_alerts"`
	NotifyAccountStatus    bool   `json:"notify_account_status"`
	OtpEnabled             bool   `json:"otp_enabled"`
	AutoBackupEnabled      bool   `json:"auto_backup_enabled"`
	AutoBackupHour         int    `json:"auto_backup_hour"`
	AutoBackupMinute       int    `json:"auto_backup_minute"`
	// AutoReportEnabled/Hour/Minute are read-only here -- the schedule is
	// set exclusively via the bot's /setreport command, not this endpoint,
	// so the web settings page can show "next report at HH:MM" without
	// offering a (redundant, easy-to-desync) second way to change it.
	AutoReportEnabled bool `json:"auto_report_enabled"`
	AutoReportHour    int  `json:"auto_report_hour"`
	AutoReportMinute  int  `json:"auto_report_minute"`

	// Socks5* (item 7): lets the panel's own bot AND every reseller's
	// bot-as-a-service instance (see ResellerBotManager) reach Telegram's
	// API through a proxy, for a panel installed on an Iran-hosted server
	// where Telegram is network-filtered. Socks5PasswordSet mirrors
	// BotTokenSet's own "never echo the secret back" convention --
	// Socks5Password itself is write-only from the client's perspective.
	Socks5Enabled     bool   `json:"socks5_enabled"`
	Socks5Address     string `json:"socks5_address"`
	Socks5Username    string `json:"socks5_username"`
	Socks5PasswordSet bool   `json:"socks5_password_set"`

	// FaoximaDomain is this install's own public domain each Faoxima
	// ("Bot X") instance's webhook/mini-app links are built under -- see
	// model.BotSettings.FaoximaDomain's own doc comment for why every
	// operator running this panel must set their own value here rather
	// than it being a fixed build-time default.
	FaoximaDomain string `json:"faoxima_domain"`
}

type UpdateBotSettingsRequest struct {
	BotToken               *string `json:"bot_token"`
	AdminChatID            *string `json:"admin_chat_id"`
	Enabled                *bool   `json:"enabled"`
	NotifyLoginAlerts      *bool   `json:"notify_login_alerts"`
	NotifyPurchaseReceipts *bool   `json:"notify_purchase_receipts"`
	NotifyLiveLog          *bool   `json:"notify_live_log"`
	NotifyQuotaWarnings    *bool   `json:"notify_quota_warnings"`
	NotifyCriticalAlerts   *bool   `json:"notify_critical_alerts"`
	NotifyAccountStatus    *bool   `json:"notify_account_status"`
	OtpEnabled             *bool   `json:"otp_enabled"`

	Socks5Enabled  *bool   `json:"socks5_enabled"`
	Socks5Address  *string `json:"socks5_address"`
	Socks5Username *string `json:"socks5_username"`
	Socks5Password *string `json:"socks5_password"`

	FaoximaDomain *string `json:"faoxima_domain"`
}

// ExtraAdminChatIDResponse/AddExtraAdminChatIDRequest expose
// model.BotExtraAdminChatID (previously manageable only via the bot's own
// /addadmin, /removeadmin, /listadmins commands) on the web settings page
// too -- the admin's own explicit request: a way to add more OTP/
// notification recipients without needing to know the Telegram bot
// command syntax.
type ExtraAdminChatIDResponse struct {
	Id     uint    `json:"id"`
	ChatID string  `json:"chat_id"`
	Label  *string `json:"label,omitempty"`
}

type AddExtraAdminChatIDRequest struct {
	ChatID string  `json:"chat_id" validate:"required"`
	Label  *string `json:"label,omitempty"`
}

// TestSocks5Request/Response back the settings page's "Test Connection"
// button (item 5) -- Address/Username/Password are sent explicitly (rather
// than read from the saved BotSettings row) so the admin can test a proxy
// before saving it. An empty Password means "use the currently-saved
// password" (mirrors UpdateBotSettingsRequest's own write-only convention)
// ONLY when PasswordUnchanged is true; otherwise an empty string is a real
// (attempted) empty password.
type TestSocks5Request struct {
	Address           string `json:"address" validate:"required"`
	Username          string `json:"username"`
	Password          string `json:"password"`
	PasswordUnchanged bool   `json:"password_unchanged"`
}

type TestSocks5Response struct {
	Connected bool   `json:"connected"`
	PingMs    int64  `json:"ping_ms,omitempty"`
	Error     string `json:"error,omitempty"`
}
