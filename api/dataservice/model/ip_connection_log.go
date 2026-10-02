package model

import "time"

// IPConnectionLog is one continuous session of a single client identity
// (a WireGuard peer name, a User Manager username -- V2Ray already has
// its own dedicated online/usage tracking via V2RayPackageLocation, so it
// is deliberately NOT covered by this table, only WireGuard/User Manager
// per the admin's own explicit request) being seen connected from one
// specific IP address. A NEW row is opened whenever the collector job
// (see cmd/jobs/security_ip_collector.go) observes an identity connected
// from an IP that doesn't match its currently-open row (or has none open);
// the previously-open row (if any) is closed (DisconnectedAt set) at that
// same moment. This means "how long was IP X connected" is always
// [ConnectedAt, DisconnectedAt) on one row, not something that has to be
// reconstructed from a series of point-in-time snapshots.
type IPConnectionLog struct {
	Model
	// Protocol is "wireguard" or "user_manager" -- mirrors
	// UsageSnapshot.Protocol's own two-of-three-systems convention (see
	// UsageProtocolWireGuard/UsageProtocolUserManager).
	Protocol string `gorm:"type:varchar(32);not null;index"`
	// Identity is the WireGuard peer's Name or the User Manager account's
	// Username, for display -- PeerID/AccountID below are the actual join
	// keys (exactly one of the two is set, matching Protocol, mirroring
	// UsageSnapshot's own identical "exactly ONE of PeerID/AccountID is set"
	// convention), needed so this session's own usage figure can be
	// computed on demand as SUM(UsageSnapshot.TotalBytes) for that ID within
	// [ConnectedAt, DisconnectedAt) -- reusing UsageSnapshotWriter's already-
	// correct, already-reset-aware delta accounting rather than duplicating
	// that logic here. A plain Identity string alone would risk attributing
	// usage to the wrong entity if a peer/account is ever renamed to match
	// a different one's old name.
	Identity string `gorm:"type:varchar(255);not null;index"`
	PeerID   *uint  `gorm:"index"`
	AccountID *uint `gorm:"index"`

	IPAddress string `gorm:"type:varchar(64);not null;index"`

	ConnectedAt time.Time `gorm:"not null;index"`
	// DisconnectedAt is nil while this IP is still the identity's current
	// connection -- exactly one row per (Protocol, Identity) may have a nil
	// DisconnectedAt at any time; the collector job enforces this by
	// closing the previous open row before opening a new one.
	DisconnectedAt *time.Time

	// Geo/ASN fields are DENORMALIZED onto this row (copied from
	// IPGeoCache at the moment this row is created), not looked up via a
	// join every time connection history is displayed -- so a later
	// IPGeoCache update/clear (e.g. after uploading a newer mmdb) does not
	// retroactively rewrite what a past session's location was recorded
	// as, matching how this feature's own history view is meant to show
	// "what we resolved this session to AT THE TIME," not a live re-lookup.
	Country  *string `gorm:"type:varchar(100)"`
	Region   *string `gorm:"type:varchar(100)"`
	City     *string `gorm:"type:varchar(100)"`
	Lat      *float64
	Lon      *float64
	Timezone *string `gorm:"type:varchar(64)"`
	ASN      *uint
	ISP      *string `gorm:"type:varchar(255)"`
}
