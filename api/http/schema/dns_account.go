package schema

// CreateDNSAccountRequest is the admin/reseller "sell a DNS account" form --
// mirrors CreateV2RayPackageRequest's shape, minus panel fan-out (a
// DNSAccount lives on exactly one PanelID, see model.DNSPanel's own doc
// comment for why), plus the three DNS-specific limit fields V2Ray has no
// equivalent of.
type CreateDNSAccountRequest struct {
	CustomerLabel *string `json:"customer_label,omitempty"`
	Comment       *string `json:"comment,omitempty"`

	PanelID uint `json:"panel_id" validate:"required"`

	// PlanID, when set, copies model.DNSPlan's bundle (TotalVolumeBytes,
	// SpeedKbps, DurationDays, MaxConcurrentIPs, DailyIPRegistrationLimit)
	// into this request's own fields below BEFORE account creation -- see
	// DNSAccountService.CreateAccount's own doc comment. Any of those fields
	// also explicitly set in this same request OVERRIDE the plan's value
	// (manual override, per phase 4-3's explicit requirement), since they
	// are applied first and the plan only fills in what's still zero.
	PlanID *uint `json:"plan_id,omitempty"`

	TotalVolumeBytes int64 `json:"total_volume_bytes"` // 0 = unlimited
	SpeedKbps        int   `json:"speed_kbps"`         // 0 = unlimited
	DurationDays     int   `json:"duration_days"`      // 0 = never expires
	TemplateID       *int  `json:"template_id,omitempty"`

	MaxConcurrentIPs         int      `json:"max_concurrent_ips" validate:"min=1"`
	DailyIPRegistrationLimit int      `json:"daily_ip_registration_limit"` // 0 = unlimited
	AllowedCountries         []string `json:"allowed_countries,omitempty"` // English names; empty = unrestricted
}

type UpdateDNSAccountRequest struct {
	CustomerLabel *string `json:"customer_label,omitempty"`
	Comment       *string `json:"comment,omitempty"`

	// PlanID, when set, copies model.DNSPlan's bundle into whichever of the
	// fields below are left nil in this SAME request -- mirrors
	// CreateDNSAccountRequest.PlanID's identical semantics. A field also
	// explicitly set here always wins over the plan's value.
	PlanID *uint `json:"plan_id,omitempty"`

	TotalVolumeBytes *int64  `json:"total_volume_bytes,omitempty"`
	SpeedKbps        *int    `json:"speed_kbps,omitempty"`
	DurationDays     *int    `json:"duration_days,omitempty"`
	TemplateID       *int    `json:"template_id,omitempty"`
	Status           *string `json:"status,omitempty"`

	MaxConcurrentIPs         *int     `json:"max_concurrent_ips,omitempty"`
	DailyIPRegistrationLimit *int     `json:"daily_ip_registration_limit,omitempty"`
	AllowedCountries         []string `json:"allowed_countries,omitempty"` // when non-nil, REPLACES the full current set (empty slice = clear all)
}

type DNSAccountResponse struct {
	Id            uint    `json:"id"`
	UUID          string  `json:"uuid"`
	CustomerLabel *string `json:"customer_label,omitempty"`
	Comment       *string `json:"comment,omitempty"`

	PanelID   uint   `json:"panel_id"`
	PanelName string `json:"panel_name"`

	PlanID   *uint   `json:"plan_id,omitempty"`
	PlanName *string `json:"plan_name,omitempty"`

	TotalVolumeBytes int64   `json:"total_volume_bytes"`
	UsedBytes        int64   `json:"used_bytes"`
	SpeedKbps        int     `json:"speed_kbps"`
	DurationDays     int     `json:"duration_days"`
	StartAt          *string `json:"start_at,omitempty"`
	ExpireAt         *string `json:"expire_at,omitempty"`
	TemplateID       *int    `json:"template_id,omitempty"`

	Status                   string   `json:"status"`
	IsShared                 bool     `json:"is_shared"`
	MaxConcurrentIPs         int      `json:"max_concurrent_ips"`
	DailyIPRegistrationLimit int      `json:"daily_ip_registration_limit"`
	AllowedCountries         []string `json:"allowed_countries"`

	CurrentIP     *string `json:"current_ip,omitempty"`
	IsOnline      bool    `json:"is_online"` // proxy: active + has a registered IP + not expired -- NOT a real-time signal, see DNSAccountService.GetConnectionStatus
	LastSyncError *string `json:"last_sync_error,omitempty"`

	ResellerID   *uint   `json:"reseller_id,omitempty"`
	ResellerName *string `json:"reseller_name,omitempty"`
}

// DNSAccountShareDetailsResponse is the public sub-page's data source --
// mirrors V2RayPackageShareDetailsResponse's role, adapted for DNS's own
// customer actions (register-IP, remaining-today counter) instead of a
// subscription link.
type DNSAccountShareDetailsResponse struct {
	CustomerLabel *string `json:"customer_label,omitempty"`
	PlanTitle     string  `json:"plan_title"`
	// PriceAmount is the DNSPlan's whole-Toman price this account was last
	// set from (nil if the account has no plan reference) -- phase 4-3's
	// "smart subscription link showing volume, speed, start/end date,
	// price": the other three were already shown here before this field
	// was added; this closes the gap.
	PriceAmount      *int64  `json:"price_amount,omitempty"`
	TotalVolumeBytes int64   `json:"total_volume_bytes"`
	SpeedKbps        int     `json:"speed_kbps"` // 0 = unlimited, same convention as DNSAccount.SpeedKbps
	UsedBytes        int64   `json:"used_bytes"`
	UsagePercent     *string `json:"usage_percent,omitempty"`
	ExpireAt         *string `json:"expire_at,omitempty"`
	DaysRemaining    *int    `json:"days_remaining,omitempty"`
	Status           string  `json:"status"`
	IsOnline         bool    `json:"is_online"`
	CurrentIP        *string `json:"current_ip,omitempty"`

	MaxConcurrentIPs         int      `json:"max_concurrent_ips"`
	DailyIPRegistrationLimit int      `json:"daily_ip_registration_limit"`
	RegistrationsUsedToday   int      `json:"registrations_used_today"`
	AllowedCountries         []string `json:"allowed_countries,omitempty"`
}

// RegisterDNSIPResponse is the public "register my IP" action's result --
// Ok=false is a routine, expected outcome (daily cap reached, geo-blocked,
// or doctor-dns itself rejected the IP as claimed by another account), not
// an HTTP-level error, so the caller always gets 200 with this body and
// reads Ok/Message.
type RegisterDNSIPResponse struct {
	Ok      bool   `json:"ok"`
	Message string `json:"message"`
	IP      string `json:"ip,omitempty"`
}

type AssignResellerDNSPanelsRequest struct {
	PanelIDs []uint `json:"panel_ids"`
}

// DNSAccountShareStatusResponse mirrors V2RayPackageShareStatusResponse
// exactly.
type DNSAccountShareStatusResponse struct {
	IsShared   bool    `json:"is_shared"`
	UUID       *string `json:"uuid,omitempty"`
	ExpireTime *string `json:"expire_time,omitempty"`
}

// DNSSelfSummaryResponse mirrors V2RaySelfSummaryResponse's identical shape
// for a reseller's own dashboard.
type DNSSelfSummaryResponse struct {
	OnlineAccounts int    `json:"online_accounts"`
	TotalAccounts  int    `json:"total_accounts"`
	QuotaBytes     *int64 `json:"quota_bytes,omitempty"`
	UsedBytes      int64  `json:"used_bytes"`
	RemainingBytes *int64 `json:"remaining_bytes,omitempty"`
	MaxAccounts    *int   `json:"max_accounts,omitempty"`
}

// DNSAdminSummaryResponse is the admin dashboard's panel-wide DNS rollup --
// mirrors V2RayAdminSummaryResponse's shape, simplified for DNS's
// one-account-per-panel model (no per-location fan-out).
type DNSAdminSummaryResponse struct {
	TotalAccounts    int                     `json:"total_accounts"`
	OnlineAccounts   int                     `json:"online_accounts"`
	TotalVolumeBytes int64                   `json:"total_volume_bytes"`
	TotalUsedBytes   int64                   `json:"total_used_bytes"`
	Panels           []DNSAdminSummaryPanel `json:"panels"`
}

// DNSAdminSummaryPanel is one registered DNSPanel's own row in the admin
// dashboard rollup -- mirrors V2RayAdminSummaryPanel's shape.
type DNSAdminSummaryPanel struct {
	PanelID        uint   `json:"panel_id"`
	PanelName      string `json:"panel_name"`
	AccountCount   int    `json:"account_count"`
	OnlineCount    int    `json:"online_count"`
	HasRecentError bool   `json:"has_recent_error"`
}
