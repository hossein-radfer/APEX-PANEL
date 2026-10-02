package schema

import "time"

// ProvisionFaoximaRequest starts provisioning a brand-new, full Faoxima
// instance for the calling reseller -- see
// service.FaoximaProvisionerService's own doc comment for what
// "provision" actually does mechanically (database, files, webhook).
type ProvisionFaoximaRequest struct {
	BotToken    string `json:"bot_token" validate:"required"`
	AdminChatID string `json:"admin_chat_id"`
}

type UpdateFaoximaTokenRequest struct {
	BotToken string `json:"bot_token" validate:"required"`
}

// FaoximaInstanceResponse never includes DBPassword/WebhookSecret --
// those are internal provisioning details the reseller never needs to
// see or manage directly, mirroring BotSettingsResponse's own convention
// of never echoing the raw bot token back either.
type FaoximaInstanceResponse struct {
	Status       string    `json:"status"`
	AdminChatID  string    `json:"admin_chat_id"`
	ErrorMessage *string   `json:"error_message,omitempty"`
	CreatedAt    time.Time `json:"created_at"`
}

// FaoximaAdminInstanceResponse is the admin-facing "مدیریت ربات فاکسیمای
// نمایندگان" list/detail view -- unlike FaoximaInstanceResponse (a
// reseller's own self-service status view), this carries the reseller's
// own identity (since an admin browses across every reseller) and the
// billing-period fields the admin's own explicit request introduced:
// running a reseller's Faoxima instance costs the panel real resources,
// so it needs a per-reseller enable/disable control tied to a paid
// period, not a free-for-everyone toggle.
type FaoximaAdminInstanceResponse struct {
	ResellerID        uint       `json:"reseller_id"`
	ResellerName      string     `json:"reseller_name"`
	Status            string     `json:"status"`
	ErrorMessage      *string    `json:"error_message,omitempty"`
	BillingPeriodDays *int       `json:"billing_period_days,omitempty"`
	PeriodActivatedAt *time.Time `json:"period_activated_at,omitempty"`
	// PeriodExpiresAt is computed server-side (PeriodActivatedAt +
	// BillingPeriodDays) purely for the admin UI's own convenience --
	// never stored, always derived fresh from the two fields above.
	PeriodExpiresAt *time.Time `json:"period_expires_at,omitempty"`
	CreatedAt       time.Time  `json:"created_at"`
}

// AdminEnableFaoximaRequest is the admin's own "فعال‌سازی با دوره"
// action -- PeriodDays <= 0 (or omitted) means "enable with no billing
// period" (runs indefinitely, exactly like the reseller's own Enable),
// matching FaoximaProvisionerService.AdminEnableWithPeriod's own
// documented convention.
type AdminEnableFaoximaRequest struct {
	PeriodDays int `json:"period_days"`
}
