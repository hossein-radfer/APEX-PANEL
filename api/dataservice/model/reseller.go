package model

import "time"

type Reseller struct {
	Model
	Name           string  `gorm:"type:varchar(255);not null"`
	Username       string  `gorm:"type:varchar(64);index"`
	PasswordHash   string  `gorm:"type:varchar(255);not null;default:''"`
	Email          *string `gorm:"type:varchar(255);uniqueIndex"`
	IsActive       bool    `gorm:"not null;default:true"`
	QuotaBytes     *int64  `gorm:"type:bigint"` // nil means unlimited
	UsedBytes      int64   `gorm:"type:bigint;not null;default:0"`
	MaxPeers       *int    `gorm:"type:int"` // nil means unlimited number of peers
	TelegramChatID *string `gorm:"type:varchar(64)"`

	// OtpEnabled gates this SPECIFIC reseller's login behind the same
	// Telegram-delivered 4-digit two-factor code the admin account already
	// uses (see model.OtpChallenge / Authentication.maybeStartOtpChallenge)
	// -- deliberately a PER-RESELLER field, unlike the admin's own
	// single global BotSettings.OtpEnabled toggle, since an admin
	// reasonably wants to require this for some resellers (e.g. ones with
	// wallet/billing access) without forcing it on every reseller account.
	// Requires TelegramChatID to also be set (the delivery channel); if
	// TelegramChatID is nil, OtpEnabled is silently treated as off rather
	// than issuing an unrecoverable challenge that could never be
	// delivered (mirrors maybeStartOtpChallenge's identical admin-side
	// safety fallback for a missing AdminChatID).
	OtpEnabled bool `gorm:"not null;default:false"`

	// QuotaWarningSent latches once a low-quota (<=10% remaining) Telegram
	// warning has been sent for the CURRENT quota period, so the traffic job
	// doesn't spam it on every tick -- cleared back to false whenever the
	// admin raises QuotaBytes enough to give the reseller >10% headroom
	// again (see Reseller.UpdateReseller).
	QuotaWarningSent bool `gorm:"not null;default:false"`

	// CanCreateUserManagerAccounts gates whether this reseller may create
	// L2TP/PPTP/SSTP/OpenVPN accounts via RouterOS User Manager -- separate
	// from WireGuard peer creation (which is gated by ResellerInterface
	// assignment instead). A simple boolean, not a join table, since User
	// Manager is one shared RouterOS resource with no per-reseller
	// sub-resource to assign.
	CanCreateUserManagerAccounts bool `gorm:"not null;default:false"`

	// UserManagerQuotaBytes/UserManagerUsedBytes/UserManagerMaxAccounts
	// mirror QuotaBytes/UsedBytes/MaxPeers exactly, but as a SEPARATE quota
	// pool for User Manager accounts -- a reseller's WireGuard and User
	// Manager limits are independently configured and independently
	// enforced (exhausting one never disables the other).
	UserManagerQuotaBytes  *int64 `gorm:"type:bigint"` // nil means unlimited
	UserManagerUsedBytes   int64  `gorm:"type:bigint;not null;default:0"`
	UserManagerMaxAccounts *int   `gorm:"type:int"` // nil means unlimited number of accounts

	// UserManagerDeletedUsageBytes is a confirmed, reported bug's fix: since
	// UserManagerUsedBytes is a LIVE SUM recomputed every tick over
	// currently-existing UserManagerAccount rows (see
	// applyUserManagerResellerQuota's own doc comment for why it's a live
	// sum rather than an accumulator), hard-deleting an account silently
	// erased its historical usage from this reseller's total and every
	// report/invoice built on it -- billing and usage history must survive
	// the underlying account being deleted. Fixed by folding the deleted
	// account's own usage into this running credit at delete time
	// (UserManagerService.DeleteAccount), which the live-sum calculation
	// then adds back on top of the SUM over still-existing rows. Mirrors
	// V2RayDeletedUsageBytes below for the identical bug in that product.
	UserManagerDeletedUsageBytes int64 `gorm:"type:bigint;not null;default:0"`

	// CanResellV2Ray/V2RayQuotaBytes/V2RayUsedBytes/V2RayMaxPackages mirror
	// the User Manager block above, for the third independent VPN product
	// (V2Ray, provisioned across one or more x-ui panels). CanResellV2Ray
	// remains the top-level on/off gate; WHICH specific panels a reseller
	// may use is a separate, explicit assignment via ResellerXuiPanelAccess
	// (mirrors ResellerInterface's role for WireGuard) -- added after an
	// initial boolean-only design turned out not to match a real
	// requirement: admins need to restrict specific resellers to specific
	// x-ui panels, not just gate V2Ray access on/off entirely.
	CanResellV2Ray   bool   `gorm:"not null;default:false"`
	V2RayQuotaBytes  *int64 `gorm:"type:bigint"` // nil means unlimited
	V2RayUsedBytes   int64  `gorm:"type:bigint;not null;default:0"`
	V2RayMaxPackages *int   `gorm:"type:int"` // nil means unlimited number of packages

	// V2RayDeletedUsageBytes mirrors UserManagerDeletedUsageBytes above --
	// same confirmed bug (usage disappearing from the reseller's total when
	// its owning package/location is deleted), same fix (fold the deleted
	// usage into this running credit before the row is removed, added back
	// on top of applyResellerV2RayQuota's own live SUM).
	V2RayDeletedUsageBytes int64 `gorm:"type:bigint;not null;default:0"`

	// CanResellDNS/DNSQuotaBytes/DNSUsedBytes/DNSMaxAccounts mirror the
	// V2Ray block above exactly, for the fourth independent product (Smart
	// DNS, provisioned on one or more registered doctor-dns exit-node
	// panels -- see model.DNSPanel/model.DNSAccount). CanResellDNS is the
	// top-level on/off gate; WHICH specific DNS panel(s) a reseller may use
	// is a separate explicit assignment via ResellerDNSPanelAccess (mirrors
	// ResellerXuiPanelAccess's role for V2Ray) -- several resellers commonly
	// share the SAME doctor-dns panel, and DNSAccount.ApexRef (not this
	// permission table) is what keeps their end-users from colliding on
	// that shared panel.
	CanResellDNS   bool   `gorm:"not null;default:false"`
	DNSQuotaBytes  *int64 `gorm:"type:bigint"` // nil means unlimited
	DNSUsedBytes   int64  `gorm:"type:bigint;not null;default:0"`
	DNSMaxAccounts *int   `gorm:"type:int"` // nil means unlimited number of accounts

	// DNSDeletedUsageBytes mirrors V2RayDeletedUsageBytes/
	// UserManagerDeletedUsageBytes -- preserves a deleted DNSAccount's
	// historical usage in this reseller's running total (see
	// DNSAccountService.DeleteAccount).
	DNSDeletedUsageBytes int64 `gorm:"type:bigint;not null;default:0"`

	// CanCreateApplications gates whether this reseller may create
	// Application bundles (the mobile-app end-user identity that fans out
	// to a peer/account/package across WireGuard/UserManager/V2Ray --
	// see ApplicationService.CreateApplication) at all, mirroring
	// CanCreateUserManagerAccounts/CanResellV2Ray's role as the top-level
	// on/off switch for their own product.
	CanCreateApplications bool `gorm:"not null;default:false"`

	// ApplicationMaxCount mirrors MaxPeers/UserManagerMaxAccounts/
	// V2RayMaxPackages -- nil means unlimited number of Applications for
	// this reseller (still bounded indirectly by their WireGuard/
	// UserManager/V2Ray sub-resource limits).
	ApplicationMaxCount *int `gorm:"type:int"`

	// ApplicationQuotaBytes/ApplicationUsedBytes mirror
	// DNSQuotaBytes/DNSUsedBytes above -- phase 2's explicit requirement
	// ("قابلیت انتخاب پلن و محدودیت حجم و اکانت" for Applications, same as
	// every other product). Originally Applications had deliberately NO
	// quota-bytes pool of its own (an Application's cost was considered
	// already capped by whichever underlying WireGuard/UserManager/V2Ray
	// limits the reseller has), but the admin explicitly asked for a
	// dedicated Application-level cap too, matching DNS/V2Ray/UserManager's
	// own pattern for consistency. ApplicationUsedBytes is a cached
	// aggregate written by the same background job that maintains
	// DNSUsedBytes/V2RayUsedBytes (SUM of Application.UsedBytes across this
	// reseller's own Applications, since Application.UsedBytes is itself
	// already the live per-Application aggregate across its
	// peer/account/package fan-out -- see Application.UsedBytes' own doc
	// comment), not a live per-request recompute.
	ApplicationQuotaBytes *int64 `gorm:"type:bigint"` // nil means unlimited
	ApplicationUsedBytes  int64  `gorm:"type:bigint;not null;default:0"`

	// ApplicationDeletedUsageBytes mirrors DNSDeletedUsageBytes/
	// V2RayDeletedUsageBytes/UserManagerDeletedUsageBytes -- preserves a
	// deleted Application's historical usage in this reseller's running
	// total (see ApplicationService.DeleteApplication).
	ApplicationDeletedUsageBytes int64 `gorm:"type:bigint;not null;default:0"`

	// BillingMode picks which of the two mutually-exclusive enforcement
	// paths applies to this reseller: ResellerBillingModeVolume (the
	// original, still-default behavior above -- QuotaBytes/UsedBytes per
	// product, reseller disabled outright at 100%) or
	// ResellerBillingModePayment (new -- every GB of usage across all three
	// products is priced in Toman via ResellerBillingPrice rows and debited
	// from this reseller's Wallet as it accrues; QuotaBytes/UsedBytes above
	// are simply unused/ignored for a Payment-based reseller, kept rather
	// than removed so switching a reseller back to Volume-based later needs
	// no data migration). See PaymentSubMode for the two ways a
	// Payment-based reseller's wallet is allowed to behave.
	BillingMode string `gorm:"type:varchar(16);not null;default:'VOLUME'"`

	// PaymentSubMode only applies when BillingMode is Payment-based:
	// ResellerPaymentSubModePrepaid (پیش‌پرداخت) reuses Wallet.Debit's
	// existing enforceFunds=true behavior unchanged -- a debit that would
	// take the balance below zero is rejected outright, so this reseller's
	// configs are disabled the moment their balance can no longer cover the
	// next accrued charge. ResellerPaymentSubModePostpaid (پس‌پرداخت) uses
	// the new Wallet.DebitAllowNegative instead -- the balance is allowed to
	// go negative (the reseller "owes" the panel), up to DebtLimitAmount;
	// only exceeding that debt ceiling triggers disablement. Meaningless
	// (and ignored) when BillingMode is Volume-based.
	PaymentSubMode string `gorm:"type:varchar(16);not null;default:'PREPAID'"`

	// DebtLimitAmount is the maximum negative balance (whole Toman, stored
	// as a positive ceiling on how far into debt this reseller may go) a
	// Postpaid reseller may carry before their configs are disabled for
	// non-payment. nil means this Postpaid reseller may carry unlimited
	// debt (admin fully trusts them) -- mirrors QuotaBytes' own
	// nil-means-unlimited convention. Ignored for Prepaid/Volume-based
	// resellers.
	DebtLimitAmount *int64 `gorm:"type:bigint"`

	// BillingSuspended latches once a Payment-based reseller has been
	// disabled for insufficient funds / exceeded debt ceiling, mirroring
	// QuotaWarningSent/the peer-level WasActiveBeforeSuspend pattern used
	// throughout this file for volume-based suspension -- lets the billing
	// job distinguish "already suspended, don't re-log/re-disable every
	// tick" from a fresh crossing of the threshold, and lets a wallet
	// top-up (or admin raising DebtLimitAmount) trigger an automatic
	// resume exactly like resumeQuotaSuspendedPeers does for volume-based
	// resellers.
	BillingSuspended bool `gorm:"not null;default:false"`

	// HasCompletedOnboarding latches true the first time this reseller
	// dismisses (or finishes) the first-login step-by-step wizard shown on
	// the reseller dashboard -- mirrors QuotaWarningSent/BillingSuspended's
	// shape exactly (a plain not-null-default-false boolean flipped by one
	// specific code path), but unlike those two has no "flip back" path:
	// once shown/dismissed it never needs to reappear for this reseller.
	// Set to true only by ResellerController.CompleteOnboarding (the
	// reseller's own self-service call, ownership-checked against their
	// JWT reseller_id claim exactly like UpdateReseller's existing
	// reseller-self-update guard) -- never by an admin, since this is
	// purely a per-reseller UI latch with no admin-facing meaning.
	HasCompletedOnboarding bool `gorm:"not null;default:false"`

	CreatedAt time.Time
	UpdatedAt time.Time
}

const (
	ResellerBillingModeVolume  = "VOLUME"
	ResellerBillingModePayment = "PAYMENT"

	ResellerPaymentSubModePrepaid  = "PREPAID"
	ResellerPaymentSubModePostpaid = "POSTPAID"
)

// ResellerBillingPrice holds one product's Toman-per-gigabyte price for one
// reseller under Payment-based billing, optionally scoped to a single
// location within that product -- WireGuard's LocationKey is an interface
// name, User Manager's is a RouterOS User Manager group name, V2Ray's is a
// V2RayPackageLocation's PanelID (formatted as a decimal string). LocationKey
// = "" is the product-wide fallback price, unscoped to any one location --
// it is what every row created before per-location pricing existed already
// means, so no migration/backfill was needed to introduce this column: an
// old row simply keeps meaning "this reseller's flat WIREGUARD price" and
// ChargeUsage's resolution order (see its own doc comment) tries the
// location-specific row first, falling back to this one. A reseller who
// resells both WireGuard and V2Ray may reasonably be charged different
// per-GB rates for each, and now also different rates per interface/group/
// panel within either -- e.g. cheaper for European WireGuard interfaces
// than Middle-Eastern ones, since bandwidth cost genuinely differs by
// location, which flat per-product pricing had no way to express. No row
// for a given (product, location) means that combination is not
// price-configured yet; the billing job treats a missing row the same as
// PricePerGBAmount=0 (usage accrues but nothing is charged) rather than
// blocking usage outright.
type ResellerBillingPrice struct {
	Model
	ResellerID       uint   `gorm:"uniqueIndex:idx_reseller_billing_price;not null"`
	Product          string `gorm:"type:varchar(16);uniqueIndex:idx_reseller_billing_price;not null"`
	LocationKey      string `gorm:"type:varchar(255);uniqueIndex:idx_reseller_billing_price;not null;default:''"`
	PricePerGBAmount int64  `gorm:"type:bigint;not null;default:0"` // whole Toman per GiB
	CreatedAt        time.Time
	UpdatedAt        time.Time
}

const (
	ResellerBillingProductWireGuard   = "WIREGUARD"
	ResellerBillingProductUserManager = "USER_MANAGER"
	ResellerBillingProductV2Ray       = "V2RAY"
	ResellerBillingProductApplication = "APPLICATION"
	ResellerBillingProductDNS         = "DNS"
)

// ResellerBillingTier defines one manually-configured volume discount step
// for a (reseller, product) pair under Payment-based billing -- the admin's
// own explicit request: a reseller using 4000GB/month should pay less per
// GB than one using 200GB, via hand-set breakpoints rather than an
// automatic formula (e.g. "0-500GB costs X, 500-2000GB costs Y, above that
// Z"). MinGB is inclusive, MaxGB is exclusive-or-nil (nil means "and
// above", exactly one tier per product should leave this nil to cover the
// top end). Applies on top of whatever price ChargeUsage resolves via
// Product+LocationKey (see ResellerBillingPrice's own doc comment) -- when
// tier rows exist for a product, they REPLACE that flat price for usage
// falling in their range rather than stacking with it, since "the same GB
// costs two different amounts" would be incoherent; ChargeUsage's own doc
// comment on tier resolution explains the precedence exactly. No tier rows
// for a product means flat (possibly per-location) pricing applies
// unchanged, so tiering is purely opt-in per product.
type ResellerBillingTier struct {
	Model
	ResellerID       uint   `gorm:"uniqueIndex:idx_reseller_billing_tier;not null"`
	Product          string `gorm:"type:varchar(16);uniqueIndex:idx_reseller_billing_tier;not null"`
	MinGB            int64  `gorm:"type:bigint;uniqueIndex:idx_reseller_billing_tier;not null;default:0"`
	MaxGB            *int64 `gorm:"type:bigint"` // nil means unbounded (this is the top tier)
	PricePerGBAmount int64  `gorm:"type:bigint;not null;default:0"`
	CreatedAt        time.Time
	UpdatedAt        time.Time
}

// ResellerInterface grants a reseller permission to create peers on a specific
// Mikrotik WireGuard interface. A reseller may only use interfaces it has been
// explicitly assigned to.
type ResellerInterface struct {
	Model
	ResellerID  uint `gorm:"uniqueIndex:idx_reseller_interface;not null"`
	InterfaceID uint `gorm:"uniqueIndex:idx_reseller_interface;not null"`
}

// ResellerXuiPanelAccess grants a reseller permission to create V2Ray
// packages on a specific x-ui panel -- mirrors ResellerInterface's role
// exactly, but for x-ui panels instead of WireGuard interfaces. A reseller
// with CanResellV2Ray=true but zero rows here has no panels to fan out to
// (mirrors ResellerInterface's own "must be explicitly assigned, no
// empty-means-all fallback" contract). Admin-owned packages (ResellerID
// nil on V2RayPackage) are never scoped by this table -- the admin can
// always use every registered panel.
type ResellerXuiPanelAccess struct {
	Model
	ResellerID uint `gorm:"uniqueIndex:idx_reseller_xui_panel;not null"`
	PanelID    uint `gorm:"uniqueIndex:idx_reseller_xui_panel;not null"`
}

// ResellerDNSPanelAccess grants a reseller permission to create DNS accounts
// on a specific registered DNSPanel (one doctor-dns exit-node installation)
// -- mirrors ResellerXuiPanelAccess exactly. Several resellers are expected
// to share the SAME DNSPanel (per the admin's own explicit requirement: one
// doctor-dns installation, multiple resellers building on it, each seeing
// only their own end-users) -- this table only grants ACCESS to the shared
// panel; keeping each reseller's accounts from colliding on it is
// DNSAccount.ApexRef's job, not this table's. A reseller with
// CanResellDNS=true but zero rows here has no panel to create accounts on
// (mirrors ResellerXuiPanelAccess's own "must be explicitly assigned, no
// empty-means-all fallback" contract). Admin-owned accounts (ResellerID nil
// on DNSAccount) are never scoped by this table -- the admin can always use
// every registered panel.
type ResellerDNSPanelAccess struct {
	Model
	ResellerID uint `gorm:"uniqueIndex:idx_reseller_dns_panel;not null"`
	PanelID    uint `gorm:"uniqueIndex:idx_reseller_dns_panel;not null"`
}
