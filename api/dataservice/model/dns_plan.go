package model

// DNSPlan is an admin-defined, named preset bundle of DNS account limits
// (e.g. "برنزی"/"نقره‌ای"/"طلایی" -- Bronze/Silver/Gold) -- phase 4-3's
// "tiered plan system instead of manual per-account setup, with manual
// override still allowed in special cases." Mirrors TrafficPackage's own
// catalog shape (a standalone admin-managed table, not scoped to a specific
// reseller or DNSPanel) rather than ResellerBillingTier's shape, which is a
// pricing/volume-discount breakpoint concept -- a genuinely different
// feature this model must not be confused with.
//
// Deliberately GLOBAL (no PanelID FK): these limits (MaxConcurrentIPs,
// DailyIPRegistrationLimit, TotalVolumeBytes, SpeedKbps, DurationDays) are
// meaningful independent of which doctor-dns installation an account lands
// on, and multiple resellers already commonly share one DNSPanel (see
// ResellerDNSPanelAccess), which would make a per-panel catalog awkward to
// reconcile. The doctor-dns-side TemplateID concept (a routing bundle
// configured on one specific installation) stays completely separate and
// per-panel, exactly as before -- a DNSPlan and a doctor-dns TemplateID are
// two unrelated "plan"-shaped words that must not be conflated in the UI.
//
// Selecting a plan on DNSAccount creation/edit copies this bundle's values
// into the account's own columns (DNSAccount.TotalVolumeBytes etc. stay the
// single source of truth for provisioning/doctor-dns push -- no schema
// duplication or live-reference headache) and additionally sets
// DNSAccount.DNSPlanID purely for display/bookkeeping ("this account is on
// the Gold plan"); every field remains a normal editable input afterward,
// satisfying the explicit "allow manual override" requirement.
type DNSPlan struct {
	Model
	Name        string  `gorm:"type:varchar(128);not null;uniqueIndex"`
	Description *string `gorm:"type:text"`

	// PriceAmount is whole Toman, matching every other price field in this
	// codebase (TrafficPackage.PriceAmount, wallet balances) -- shown on the
	// DNS share page once an account is on a priced plan, closing out phase
	// 4-3's other open item ("smart subscription link showing volume,
	// speed, start/end date, price": volume/dates were already shown before
	// this change, price was the one missing piece).
	PriceAmount int64 `gorm:"type:bigint;not null;default:0"`

	// The bundle -- identical fields/semantics to their DNSAccount
	// namesakes (see that model's own doc comments for what 0 means on
	// each).
	TotalVolumeBytes         int64 `gorm:"type:bigint;not null;default:0"`
	SpeedKbps                int   `gorm:"type:int;not null;default:0"`
	DurationDays             int   `gorm:"type:int;not null;default:0"`
	MaxConcurrentIPs         int   `gorm:"type:int;not null;default:1"`
	DailyIPRegistrationLimit int   `gorm:"type:int;not null;default:0"`

	IsActive bool `gorm:"not null;default:true"`
}
