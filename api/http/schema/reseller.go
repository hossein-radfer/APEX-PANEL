package schema

import "time"

type CreateResellerRequest struct {
	Name           string  `json:"name" validate:"required"`
	Username       string  `json:"username" validate:"required,min=3,max=64"`
	Password       string  `json:"password" validate:"required,min=8,max=128"`
	Email          *string `json:"email"`
	QuotaBytes     *int64  `json:"quotaBytes"`
	MaxPeers       *int    `json:"maxPeers" validate:"omitempty,min=1"`
	TelegramChatID *string `json:"telegramChatId"`
	OtpEnabled     *bool   `json:"otpEnabled"`

	// User Manager (L2TP/PPTP/SSTP/OpenVPN) permission + quota -- a
	// SEPARATE pool from QuotaBytes/MaxPeers above, never shared.
	CanCreateUserManagerAccounts *bool  `json:"canCreateUserManagerAccounts"`
	UserManagerQuotaBytes        *int64 `json:"userManagerQuotaBytes"`
	UserManagerMaxAccounts       *int   `json:"userManagerMaxAccounts" validate:"omitempty,min=1"`

	// V2Ray permission + quota -- a third, SEPARATE pool from QuotaBytes/
	// MaxPeers and UserManagerQuotaBytes/UserManagerMaxAccounts above,
	// never shared with either.
	CanResellV2Ray   *bool  `json:"canResellV2Ray"`
	V2RayQuotaBytes  *int64 `json:"v2rayQuotaBytes"`
	V2RayMaxPackages *int   `json:"v2rayMaxPackages" validate:"omitempty,min=1"`

	// DNS permission + quota -- a fourth, SEPARATE pool from every other
	// product above, never shared. Mirrors CanResellV2Ray/V2RayQuotaBytes/
	// V2RayMaxPackages exactly (model.Reseller.CanResellDNS/DNSQuotaBytes/
	// DNSMaxAccounts already existed on the model before this field was
	// added here -- this was a genuine gap: no admin API/UI path could
	// ever grant DNS-reselling permission to a reseller until now).
	CanResellDNS   *bool  `json:"canResellDns"`
	DNSQuotaBytes  *int64 `json:"dnsQuotaBytes"`
	DNSMaxAccounts *int   `json:"dnsMaxAccounts" validate:"omitempty,min=1"`

	// Applications permission + quota/count cap -- a fifth, SEPARATE pool
	// from every other product above, never shared. Mirrors
	// CanResellDNS/DNSQuotaBytes/DNSMaxAccounts exactly.
	CanCreateApplications *bool  `json:"canCreateApplications"`
	ApplicationQuotaBytes *int64 `json:"applicationQuotaBytes"`
	ApplicationMaxCount   *int   `json:"applicationMaxCount" validate:"omitempty,min=1"`

	// Payment-based billing (item 5), settable at creation time too --
	// see UpdateResellerRequest's own identical fields for the full
	// rationale. Omitted/nil BillingMode defaults to VOLUME via
	// model.Reseller's own gorm default, matching every other
	// not-yet-configured-at-creation field on this request.
	BillingMode     *string `json:"billingMode" validate:"omitempty,oneof=VOLUME PAYMENT"`
	PaymentSubMode  *string `json:"paymentSubMode" validate:"omitempty,oneof=PREPAID POSTPAID"`
	DebtLimitAmount *int64  `json:"debtLimitAmount" validate:"omitempty,min=0"`
}

type UpdateResellerRequest struct {
	Name           *string `json:"name"`
	Username       *string `json:"username" validate:"omitempty,min=3,max=64"`
	Password       *string `json:"password" validate:"omitempty,min=8,max=128"`
	Email          *string `json:"email"`
	IsActive       *bool   `json:"isActive"`
	QuotaBytes     *int64  `json:"quotaBytes"`
	MaxPeers       *int    `json:"maxPeers" validate:"omitempty,min=1"`
	ClearMaxPeers  *bool   `json:"clearMaxPeers"`
	TelegramChatID *string `json:"telegramChatId"`
	OtpEnabled     *bool   `json:"otpEnabled"`

	CanCreateUserManagerAccounts *bool  `json:"canCreateUserManagerAccounts"`
	UserManagerQuotaBytes        *int64 `json:"userManagerQuotaBytes"`
	UserManagerMaxAccounts       *int   `json:"userManagerMaxAccounts" validate:"omitempty,min=1"`
	ClearUserManagerMaxAccounts  *bool  `json:"clearUserManagerMaxAccounts"`

	CanResellV2Ray        *bool  `json:"canResellV2Ray"`
	V2RayQuotaBytes       *int64 `json:"v2rayQuotaBytes"`
	V2RayMaxPackages      *int   `json:"v2rayMaxPackages" validate:"omitempty,min=1"`
	ClearV2RayMaxPackages *bool  `json:"clearV2RayMaxPackages"`

	// DNS permission + quota -- mirrors CanResellV2Ray/V2RayQuotaBytes/
	// V2RayMaxPackages/ClearV2RayMaxPackages exactly, fourth independent
	// pool. See CreateResellerRequest's identical fields for why this was
	// added (a pre-existing model-level gap with no API path).
	CanResellDNS        *bool  `json:"canResellDns"`
	DNSQuotaBytes       *int64 `json:"dnsQuotaBytes"`
	DNSMaxAccounts      *int   `json:"dnsMaxAccounts" validate:"omitempty,min=1"`
	ClearDNSMaxAccounts *bool  `json:"clearDnsMaxAccounts"`

	// Applications permission + quota/count cap -- mirrors
	// CanResellDNS/DNSQuotaBytes/DNSMaxAccounts/ClearDNSMaxAccounts exactly,
	// fifth independent pool.
	CanCreateApplications    *bool  `json:"canCreateApplications"`
	ApplicationQuotaBytes    *int64 `json:"applicationQuotaBytes"`
	ApplicationMaxCount      *int   `json:"applicationMaxCount" validate:"omitempty,min=1"`
	ClearApplicationMaxCount *bool  `json:"clearApplicationMaxCount"`

	// Payment-based billing (item 5): BillingMode picks VOLUME (default,
	// unchanged behavior) or PAYMENT for this reseller -- see
	// model.Reseller.BillingMode's own doc comment. PaymentSubMode
	// (PREPAID/POSTPAID) and DebtLimitAmount are only meaningful when
	// BillingMode=PAYMENT; ClearDebtLimitAmount mirrors ClearMaxPeers'
	// existing "nil pointer means don't touch, explicit clear flag means
	// set to nil" convention for the one field here that's nil-means-
	// unlimited rather than nil-means-omitted.
	BillingMode          *string `json:"billingMode" validate:"omitempty,oneof=VOLUME PAYMENT"`
	PaymentSubMode       *string `json:"paymentSubMode" validate:"omitempty,oneof=PREPAID POSTPAID"`
	DebtLimitAmount      *int64  `json:"debtLimitAmount" validate:"omitempty,min=0"`
	ClearDebtLimitAmount *bool   `json:"clearDebtLimitAmount"`
}

// ResellerBillingPriceEntry is one (Product, LocationKey) -> Toman-per-GB
// price pair -- LocationKey == "" is the product-wide fallback price;
// LocationKey a WireGuard interface name / User Manager group name /
// V2Ray PanelID (as a decimal string) scopes the price to just that one
// location within the product (see model.ResellerBillingPrice's own doc
// comment). Each entry independently upserts, so a single request can set
// the product-wide WIREGUARD price and override just one interface's
// price at the same time.
type ResellerBillingPriceEntry struct {
	Product          string `json:"product" validate:"required,oneof=WIREGUARD USER_MANAGER V2RAY APPLICATION"`
	LocationKey      string `json:"locationKey"`
	PricePerGBAmount int64  `json:"pricePerGbAmount" validate:"min=0"`
}

// UpdateResellerBillingPricesRequest upserts this reseller's per-product
// (optionally per-location) Toman-per-GB prices in one call -- an entry
// not present in Prices is left completely untouched (not reset to zero)
// -- omitting WIREGUARD here because this reseller doesn't resell
// WireGuard is not the same request as explicitly wanting to charge 0/GB
// for it, and omitting one interface's override leaves that interface on
// the product-wide fallback price rather than clearing it to zero.
type UpdateResellerBillingPricesRequest struct {
	Prices []ResellerBillingPriceEntry `json:"prices" validate:"required,dive"`
}

type ResellerBillingPriceResponse struct {
	Product          string `json:"product"`
	LocationKey      string `json:"locationKey"`
	PricePerGBAmount int64  `json:"pricePerGbAmount"`
}

// ResellerBillingTierEntry is one manually-configured volume-discount
// breakpoint (see model.ResellerBillingTier's own doc comment) -- MinGB
// inclusive, MaxGB exclusive-or-nil (nil means "and above," exactly one
// tier per product should leave this nil).
type ResellerBillingTierEntry struct {
	MinGB            int64  `json:"minGb" validate:"min=0"`
	MaxGB            *int64 `json:"maxGb" validate:"omitempty,min=1"`
	PricePerGBAmount int64  `json:"pricePerGbAmount" validate:"min=0"`
}

// UpdateResellerBillingTiersRequest REPLACES every tier for one product
// with the given list (see ResellerBillingService.SetTiers's own doc
// comment on why this is a full replace, not a partial upsert) -- an
// empty Tiers list removes tiering for that product entirely, reverting
// to flat/location pricing.
type UpdateResellerBillingTiersRequest struct {
	Product string                     `json:"product" validate:"required,oneof=WIREGUARD USER_MANAGER V2RAY APPLICATION"`
	Tiers   []ResellerBillingTierEntry `json:"tiers" validate:"dive"`
}

type ResellerBillingTierResponse struct {
	Product          string `json:"product"`
	MinGB            int64  `json:"minGb"`
	MaxGB            *int64 `json:"maxGb"`
	PricePerGBAmount int64  `json:"pricePerGbAmount"`
}

type ResellerResponse struct {
	ID             uint      `json:"id"`
	Name           string    `json:"name"`
	Username       string    `json:"username"`
	Email          *string   `json:"email"`
	IsActive       bool      `json:"isActive"`
	QuotaBytes     *int64    `json:"quotaBytes"`
	UsedBytes      int64     `json:"usedBytes"`
	MaxPeers       *int      `json:"maxPeers"`
	PeerCount      int       `json:"peerCount"`
	TelegramChatID *string   `json:"telegramChatId"`
	OtpEnabled     bool      `json:"otpEnabled"`
	CreatedAt      time.Time `json:"createdAt,omitempty"`
	UpdatedAt      time.Time `json:"updatedAt,omitempty"`

	CanCreateUserManagerAccounts bool   `json:"canCreateUserManagerAccounts"`
	UserManagerQuotaBytes        *int64 `json:"userManagerQuotaBytes"`
	UserManagerUsedBytes         int64  `json:"userManagerUsedBytes"`
	UserManagerMaxAccounts       *int   `json:"userManagerMaxAccounts"`
	UserManagerAccountCount      int    `json:"userManagerAccountCount"`

	CanResellV2Ray    bool   `json:"canResellV2Ray"`
	V2RayQuotaBytes   *int64 `json:"v2rayQuotaBytes"`
	V2RayUsedBytes    int64  `json:"v2rayUsedBytes"`
	V2RayMaxPackages  *int   `json:"v2rayMaxPackages"`
	V2RayPackageCount int    `json:"v2rayPackageCount"`

	CanResellDNS    bool   `json:"canResellDns"`
	DNSQuotaBytes   *int64 `json:"dnsQuotaBytes"`
	DNSUsedBytes    int64  `json:"dnsUsedBytes"`
	DNSMaxAccounts  *int   `json:"dnsMaxAccounts"`
	DNSAccountCount int    `json:"dnsAccountCount"`

	CanCreateApplications bool   `json:"canCreateApplications"`
	ApplicationQuotaBytes *int64 `json:"applicationQuotaBytes"`
	ApplicationUsedBytes  int64  `json:"applicationUsedBytes"`
	ApplicationMaxCount   *int   `json:"applicationMaxCount"`
	ApplicationCount      int    `json:"applicationCount"`

	BillingMode      string `json:"billingMode"`
	PaymentSubMode   string `json:"paymentSubMode"`
	DebtLimitAmount  *int64 `json:"debtLimitAmount"`
	BillingSuspended bool   `json:"billingSuspended"`

	HasCompletedOnboarding bool `json:"hasCompletedOnboarding"`
}

// InterfaceIDs deliberately has NO "required" validation tag: an empty
// slice is a legitimate request meaning "clear all assigned interfaces"
// (e.g. for a reseller who should only create User Manager accounts, not
// WireGuard peers). go-playground/validator's "required" rule treats an
// empty slice as its zero value and rejects it, which previously made it
// impossible to ever unassign a reseller's last interface -- forcing at
// least one WireGuard interface to stay assigned even for a reseller who
// only needs User Manager access.
type AssignResellerInterfacesRequest struct {
	InterfaceIDs []uint `json:"interfaceIds"`
}

// AssignResellerUserManagerGroupsRequest/AssignResellerUserManagerProfilesRequest
// mirror AssignResellerInterfacesRequest exactly (including the deliberate
// absence of "required" -- see that struct's doc comment), restricting
// which RouterOS User Manager groups/profiles a reseller may assign when
// creating accounts.
type AssignResellerUserManagerGroupsRequest struct {
	GroupNames []string `json:"groupNames"`
}

type AssignResellerUserManagerProfilesRequest struct {
	ProfileNames []string `json:"profileNames"`
}
