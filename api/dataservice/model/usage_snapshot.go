package model

// UsageSnapshot is one traffic-delta event, recorded by every existing
// per-protocol sync/traffic job (WireGuard's CalculatePeerTraffic, User
// Manager's CalculateUserManagerUsage, V2Ray's SyncPackageUsage) at the
// exact same moment each already computes a delta for its own quota
// bookkeeping -- this table adds NOTHING to what those jobs already
// compute, it only additionally persists that same delta as its own row
// instead of only folding it into a running total. This is the single
// data source every report in the Reports section reads from; without it,
// historical/time-series reporting (daily usage, peak usage, rankings
// over a date range) would be impossible to reconstruct, since every
// other usage figure in this codebase is a plain running total with no
// history.
//
// Deliberately NOT backfilled for time before this table's introduction
// -- there is no historical delta data to reconstruct (every existing
// usage figure is a cumulative counter, not a time series), so reports
// covering a range that predates this table simply show sparse/partial
// data for the earliest days, filling in naturally as the sync jobs
// continue to run going forward.
type UsageSnapshot struct {
	Model
	// Timestamp is when this delta was recorded -- indexed for range
	// queries (every report groups/filters by a date range). Deliberately
	// a separate column from Model.CreatedAt (which is a Unix uint64, not
	// a time.Time) so report queries can use normal SQL date functions.
	Timestamp int64 `gorm:"not null;index"`

	// Protocol discriminates which VPN system this snapshot came from --
	// "wireguard", "user_manager", or "v2ray", matching this codebase's
	// existing convention of fully separate systems per product rather
	// than a shared table with a type column carrying different semantics
	// per type (see V2RayPackage's own doc comment on why V2Ray/WireGuard/
	// User Manager are never merged into one table).
	Protocol string `gorm:"type:varchar(32);not null;index"`

	// PackageID/PeerID/AccountID: exactly ONE of these is set, matching
	// Protocol -- V2Ray snapshots set PackageID, WireGuard snapshots set
	// PeerID, User Manager snapshots set AccountID. Nullable rather than a
	// polymorphic single "entity_id" column so each stays a real, typed
	// foreign key an admin/report query can join against directly.
	PackageID *uint `gorm:"index"`
	PeerID    *uint `gorm:"index"`
	AccountID *uint `gorm:"index"`

	// ResellerID is nil for an admin-direct entity, matching every other
	// model's own IS NULL convention in this codebase -- denormalized
	// here (also derivable via a join) purely so every reseller-scoped
	// report can filter/group directly on this table without a join to
	// three different parent tables depending on Protocol.
	ResellerID *uint `gorm:"index"`

	// PanelID is set only for V2Ray snapshots (which panel this location's
	// delta came from) -- nil for WireGuard/User Manager, which have no
	// per-panel concept.
	PanelID *uint `gorm:"index"`

	UploadBytes   int64 `gorm:"type:bigint;not null;default:0"`
	DownloadBytes int64 `gorm:"type:bigint;not null;default:0"`
	// TotalBytes is UploadBytes+DownloadBytes, stored redundantly (not
	// computed at query time) purely so every report's SUM() query can
	// read one column instead of an addition in every single query.
	TotalBytes int64 `gorm:"type:bigint;not null;default:0"`
}

// UsageProtocolWireGuard/UserManager/V2Ray are UsageSnapshot.Protocol's
// only valid values -- named constants so every write/read site uses the
// exact same literal string, never a typo'd copy.
const (
	UsageProtocolWireGuard   = "wireguard"
	UsageProtocolUserManager = "user_manager"
	UsageProtocolV2Ray       = "v2ray"
)
