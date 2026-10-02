package model

import "time"

// DNSAccount is one customer's Smart DNS service, provisioned on exactly one
// registered DNSPanel (see DNSPanel's own doc comment for why there is no
// V2Ray-style multi-panel fan-out here). Deliberately its own table, not
// sharing one with Peer/UserManagerAccount/V2RayPackage, mirroring how each
// of those three existing VPN products already has its own independent
// table despite similar shapes.
type DNSAccount struct {
	Model
	UUID string `gorm:"type:varchar(36);uniqueIndex;not null"` // public sub-page link, mirrors V2RayPackage.UUID/Peer.UUID

	PanelID uint `gorm:"index;not null"` // which DNSPanel this account was provisioned on

	// ApexRef is the opaque key sent to doctor-dns as apex_ref on every
	// /apex/* call -- generated once at creation and never changed. It is
	// what keeps two resellers' end-users from colliding on a DNSPanel they
	// both have access to (see ResellerDNSPanelAccess's own doc comment):
	// distinct resellers always get distinct ApexRef values even when
	// pointed at the very same doctor-dns installation. Never shown to the
	// customer and carries no login meaning on the doctor-dns side.
	ApexRef string `gorm:"type:varchar(120);uniqueIndex;not null"`

	// CustomerLabel/Comment mirror V2RayPackage's identical two fields --
	// CustomerLabel is customer-facing (shown on the public sub page),
	// Comment is admin-only internal bookkeeping, never read by anything
	// that builds sub-page content.
	CustomerLabel *string `gorm:"type:varchar(255)"`
	Comment       *string `gorm:"type:text"`

	// ResellerID nil (IS NULL) means an admin-direct account, matching
	// V2RayPackage/UserManagerAccount's convention.
	ResellerID *uint `gorm:"index"`

	TotalVolumeBytes int64 `gorm:"type:bigint;not null;default:0"` // 0 = unlimited, matches doctor-dns's own quota_bytes convention
	SpeedKbps        int   `gorm:"type:int;not null;default:0"`    // 0 = unlimited, sent straight through as doctor-dns's speed_kbps
	DurationDays     int   `gorm:"type:int;not null;default:0"`    // 0 = never expires, matches doctor-dns's expire_days semantics
	StartAt          *time.Time
	ExpireAt         *time.Time

	// TemplateID is the doctor-dns "plan" (a named routing bundle configured
	// on that installation's own admin panel, see DoctorDnsTemplate) this
	// account is on. nil means that DNSPanel's own default template.
	TemplateID *int `gorm:"type:int"`

	// DNSPlanID is the local Apex-side tier (see DNSPlan's own doc comment
	// for why this is a completely separate concept from TemplateID above,
	// despite both being called "plan"-ish) this account was created/last
	// set from. Purely a display/bookkeeping reference -- selecting a plan
	// copies its bundle into this account's own fields below, which remain
	// the single source of truth for provisioning; nil means this account
	// was configured manually with no plan, or has since been edited enough
	// that a plan reference no longer applies.
	DNSPlanID *uint `gorm:"index"`

	Status string `gorm:"type:varchar(16);not null;default:'active'"` // "active" | "suspended" | "expired"

	// SuspendedByQuota/WasActiveBeforeSuspend/SuspendedByResellerQuota
	// mirror V2RayPackage's identical trio exactly -- see that model's own
	// doc comments for the full reasoning (two genuinely different reasons
	// an account can be suspended, tracked separately so resuming one never
	// wrongly resumes/blocks the other).
	SuspendedByQuota         bool `gorm:"type:boolean;not null;default:false"`
	WasActiveBeforeSuspend   bool `gorm:"type:boolean;not null;default:false"`
	SuspendedByResellerQuota bool `gorm:"type:boolean;not null;default:false"`

	// UsageOffsetBytes implements the admin's "Reset Usage" action, mirroring
	// V2RayPackage.UsageOffsetBytes exactly: DisplayedUsedBytes =
	// UsedBytesCached - UsageOffsetBytes (clamped at 0). doctor-dns's own
	// used_bytes is itself a server-side running total (not something this
	// panel accumulates deltas into), so "reset" here can never touch the
	// remote counter -- only this local offset.
	UsageOffsetBytes int64 `gorm:"type:bigint;not null;default:0"`

	// --- DNS-specific limits (no equivalent on the other three products) ---

	// MaxConcurrentIPs is this account's concurrent-device/IP ceiling --
	// sent to doctor-dns as its max_ips field (registering past this count
	// there replaces the oldest registered IP, doctor-dns's own existing
	// behavior, unchanged by this integration).
	MaxConcurrentIPs int `gorm:"type:int;not null;default:1"`

	// DailyIPRegistrationLimit caps how many TIMES PER DAY the public sub
	// page's "register my IP" action may succeed for this account --
	// enforced entirely on the Apex side (see DNSIPRegistrationLog), since
	// doctor-dns itself has and needs no such concept: it only tracks how
	// many DISTINCT IPs may be registered at once (MaxConcurrentIPs above),
	// never how often registration may be attempted. 0 means unlimited
	// (still bounded by MaxConcurrentIPs on the doctor-dns side).
	DailyIPRegistrationLimit int `gorm:"type:int;not null;default:0"`

	// Share page, mirrors V2RayPackage.IsShared/ShareExpireTime.
	IsShared        bool    `gorm:"type:boolean;not null;default:false"`
	ShareExpireTime *string `gorm:"type:varchar(255)"`

	// --- Cached remote state, written only by the background sync job ---

	UsedBytesCached int64   `gorm:"type:bigint;not null;default:0"`       // doctor-dns's own users.used_bytes, copied verbatim (already an absolute running total server-side, not a delta this panel accumulates)
	RemoteStatus    string  `gorm:"type:varchar(16);not null;default:''"` // doctor-dns's own status column: pending/active/suspended/expired/over_quota
	CurrentIP       *string `gorm:"type:varchar(64)"`                     // most recently confirmed registered IP, from doctor-dns's own reported ip field
	LastSyncedAt    *time.Time
	LastSyncError   *string `gorm:"type:text"`
}

// DNSAccountAllowedCountry is one allowed country for a DNSAccount's
// geo-fenced IP registration -- mirrors ResellerInterface/
// ResellerXuiPanelAccess/ResellerDNSPanelAccess's own join-table convention
// for a per-row "many" relationship in this codebase, rather than a packed
// comma-separated column.
//
// An account with ZERO rows here has NO geo-restriction at all (any
// country's IP may register) -- this is the OPPOSITE of
// ResellerXuiPanelAccess/ResellerDNSPanelAccess's own "zero rows = no
// access" convention for those permission tables, so it must stay
// documented clearly here: for THIS table specifically, empty means
// unrestricted, not blocked from everywhere.
type DNSAccountAllowedCountry struct {
	Model
	DNSAccountID uint   `gorm:"uniqueIndex:idx_dns_account_country;not null"`
	CountryName  string `gorm:"type:varchar(100);uniqueIndex:idx_dns_account_country;not null"` // English name, matches GeoIPLookupResult.Country (MaxMind's Names["en"])
}

// DNSIPRegistrationLog is one IP-registration ATTEMPT for a DNSAccount,
// written by every call to the public sub page's "register my IP" action --
// both accepted and rejected attempts are logged (Accepted distinguishes
// them), so DailyIPRegistrationLimit can be enforced by counting today's
// Accepted rows, and so an admin/customer can see WHY a given attempt was
// refused (daily cap vs. geo-block vs. doctor-dns-side conflict) rather than
// just a bare failure. Mirrors IPConnectionLog's role as a lightweight audit
// trail, but deliberately simpler: a write-once event log, not an open/close
// session table -- doctor-dns's own /apex/ surface has no session concept
// for DNS registrations to track.
type DNSIPRegistrationLog struct {
	Model
	// AttemptedAt is an explicit time.Time (NOT Model.CreatedAt, which is a
	// Unix-epoch uint64 per Model's own convention and awkward to run
	// day-window comparisons against) -- mirrors IPConnectionLog.ConnectedAt's
	// identical reasoning for the same problem. This is the basis for the
	// daily-window count in DNSAccountService.RegisterCustomerIP.
	AttemptedAt  time.Time `gorm:"not null;index"`
	DNSAccountID uint      `gorm:"index;not null"`
	IPAddress    string    `gorm:"type:varchar(64);not null"`
	Country      *string   `gorm:"type:varchar(100)"` // denormalized from GeoIPService.Lookup at registration time
	Accepted     bool      `gorm:"not null"`
	RejectReason *string   `gorm:"type:varchar(255)"`
}
