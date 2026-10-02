package model

import "time"

// V2RayPackageLocation is one V2RayPackage's client on one specific
// XuiPanel -- the fan-out row implementing "every package gets a client on
// every registered panel." A package with N registered panels has N
// location rows. This codebase only implements VLESS/VMess connection
// info (per the plan's own documented addClient payload); Trojan uses
// client.password and Shadowsocks uses client.email as their RouterOS-
// equivalent identifier instead of client.id -- that distinction is a
// known, explicitly out-of-scope limitation of ClientUUID below.
type V2RayPackageLocation struct {
	Model
	PackageID uint `gorm:"uniqueIndex:idx_v2ray_package_location;not null"`
	PanelID   uint `gorm:"uniqueIndex:idx_v2ray_package_location;not null"`

	// ClientUUID is x-ui's client.id (VLESS/VMess only -- see doc comment
	// above). Needed for updateClient/delClient calls against this location.
	ClientUUID string `gorm:"type:varchar(64);not null"`
	// ClientEmail is x-ui's required unique-per-inbound identifier,
	// generated as "pkg_<uuid short>_<panel_id>" at creation time. Also the
	// key getClientTraffics/resetClientTraffic are addressed by.
	ClientEmail string `gorm:"type:varchar(255);not null"`
	SubID       string `gorm:"type:varchar(64);not null"`

	UsedBytesCached int64 `gorm:"type:bigint;not null;default:0"`
	// LastTotalUsedBytes caches x-ui's own cumulative counter from the last
	// getClientTraffics poll, mirroring Peer.LastTx/UserManagerAccount.
	// LastTotalDownload's role: the sync job needs the previous absolute
	// reading to compute a correct delta rather than assume monotonic
	// growth (a client reset on the x-ui side would otherwise undercount).
	LastTotalUsedBytes int64 `gorm:"type:bigint;not null;default:0"`

	// ConfigLinkCached is this location's own single rewritten config link
	// (title already substituted per the sale-title fallback rule), stored
	// as PLAINTEXT -- not base64. Combined with every other location's
	// ConfigLinkCached at sync time into V2RayPackage.CombinedConfigCached,
	// which IS base64-encoded (exactly once, at that final combination
	// step) since that's the value actually served to V2Ray client apps.
	// Written only by the sync job.
	ConfigLinkCached string `gorm:"type:text;not null;default:''"`

	// Enabled deliberately carries NO `default:` tag -- a confirmed GORM
	// gotcha (see model.TunnelActionLog.Simulated's own doc comment for
	// the first occurrence of this exact bug class this session):
	// `gorm:"...;default:true"` on a bool makes GORM's Create() treat
	// Go's own zero value (false) as "field not set" and silently
	// substitute the schema default instead, so a caller that explicitly
	// wants to CREATE a location as already-disabled would silently get
	// Enabled=true regardless. Every current production call site
	// (CreatePackage, UpdatePackage's add-new-location path) already sets
	// Enabled: true explicitly at creation time, so this had no live
	// production impact -- caught instead by a test that tried to create
	// a location with Enabled: false and got Enabled: true back. Removing
	// the tag costs nothing (every real Create() call already sets this
	// field one way or the other) and closes the trap for any future
	// caller.
	Enabled bool `gorm:"type:boolean;not null"`

	// LastSyncedAt/LastSyncError: a failure talking to THIS location's
	// panel is captured here and never surfaces as a whole-package
	// failure -- the sync job continues to the next location on error,
	// and the public subscription endpoint simply omits a location whose
	// ConfigLinkCached is stale/empty rather than erroring.
	LastSyncedAt  *time.Time
	LastSyncError *string `gorm:"type:text"`

	// FlowRepairedAt/HadWrongFlow track a one-time-per-location repair
	// pass (see V2RaySyncService.repairClientFlowIfNeeded) added after a
	// confirmed, reproduced bug: flow was hardcoded to "xtls-rprx-vision"
	// regardless of the inbound's actual security setting, which silently
	// created clients that could never connect on any security:"none"
	// inbound. FlowRepairedAt nil means this location predates the fix and
	// has not yet been checked; once checked (whether or not a repair was
	// actually needed), it's set and never rechecked again -- this is a
	// migration concern, not an ongoing per-tick cost. HadWrongFlow records
	// whether THIS location was actually broken, so the admin can see
	// exactly how many real customers were affected, not just that the
	// check ran.
	FlowRepairedAt *time.Time
	HadWrongFlow   bool `gorm:"type:boolean;not null;default:false"`

	// IsOnline/OnlineCheckedAt cache the panel-wide onlines poll (see
	// V2RaySyncService.pollOnlineStatus) -- x-ui only exposes "currently
	// connected right now," so this is a point-in-time snapshot refreshed
	// once per sync tick, not a live value. IsOnline defaults false (never
	// been seen online / not yet polled), which is also the correct display
	// state for a location whose panel is unreachable.
	IsOnline        bool `gorm:"type:boolean;not null;default:false"`
	OnlineCheckedAt *time.Time
}
