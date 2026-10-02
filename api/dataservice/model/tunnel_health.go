package model

import "time"

// This file implements the "tunnel-ai" autonomous network monitoring
// system (see the admin's own Persian spec document) -- discovery +
// multi-signal detection + Telegram alerting + a self-healing decision
// engine for INFRASTRUCTURE tunnels (GRE/IPIP/EoIP/etc between servers).
//
// IMPORTANT, confirmed by the admin after a reported bug: "tunnel" here
// means genuine inter-server infrastructure links, detected by their
// real participation in the routing table as a route's own gateway (see
// TunnelGraphService's own discoveryData.tunnelGatewayNames) -- NOT
// customer-facing WireGuard peer interfaces (which is what an earlier,
// wrong version of this code monitored, purely because it happened to
// also be a "WireGuard interface" by type). Every table below is
// therefore keyed by InterfaceName (the plain RouterOS interface name,
// e.g. "gre-tunnel1"), never by a FK into model.Interface -- that table
// only holds customer WireGuard peer interfaces created through this
// panel and has no row at all for a GRE/IPIP/EoIP link between servers.
//
// Deliberately mostly READ-ONLY: nothing here modifies RouterOS state
// beyond the self-healing engine's own explicitly-scoped Level 1/2
// actions (see TunnelHealthService), always gated by dry-run.

// TunnelHealthStatus is one infrastructure tunnel's current computed
// health, upserted every poll tick (see TunnelHealthService.Poll) -- one
// row per interface name, not a history table (see TunnelHealthEvent
// below for the append-only history/alerting side).
type TunnelHealthStatus struct {
	Model
	InterfaceName string `gorm:"type:varchar(255);uniqueIndex;not null"`

	// Severity is "healthy" | "suspect" | "confirmed_down" -- mirrors the
	// spec's own SUSPECT/CONFIRMED_DOWN vocabulary. Never
	// "remediating"/"failed_over"/etc: those states belong to the
	// self-healing state machine (bond b-5 in the spec), not this
	// detection-only phase.
	Severity string `gorm:"type:varchar(32);not null;default:'healthy'"`

	// InterfaceRunning/InterfaceDisabled cache the last poll's raw
	// RouterOS interface state (method 1 in spec section b-4).
	InterfaceRunning  bool `gorm:"not null;default:true"`
	InterfaceDisabled bool `gorm:"not null;default:false"`

	// WorstPeerLastHandshakeAgeSeconds is only meaningful for a
	// WireGuard-based infrastructure tunnel (a WireGuard interface CAN
	// legitimately be used for a server-to-server link, not just
	// customer peers -- detected the same tunnelGatewayNames way as any
	// other type) -- nil for GRE/IPIP/EoIP, which have no handshake
	// concept, and nil when there is nothing to measure staleness
	// against.
	WorstPeerLastHandshakeAgeSeconds *int `gorm:""`

	// TxRxAsymmetryDetected is method 3 (spec's own "tx keeps climbing,
	// rx stays flat" heuristic) -- see TunnelHealthService.
	// checkTxRxAsymmetry for the exact window/ratio this uses.
	TxRxAsymmetryDetected bool `gorm:"not null;default:false"`

	// LastKnownGatewayIP caches the most recent HEALTHY poll's resolved
	// next-hop IP for this interface (the "<ip>" half of a route's own
	// ImmediateGw "<ip>%<interface>" pair -- see
	// mikrotik.RouteEntry.ImmediateGw's own doc comment). Confirmed,
	// reported production incident this fixes: attemptLevel2's own route
	// matching (routeGatewayInterface) relies entirely on RouterOS's
	// LIVE ImmediateGw resolution to identify which routes go out the
	// down interface -- but ImmediateGw goes BLANK the instant the
	// interface actually goes down (RouterOS has nothing left to
	// resolve), at which point every affected route's bare Gateway field
	// is just the same next-hop IP with no "%interface" suffix, and
	// routeGatewayInterface falls back to returning that bare IP --
	// which never equals the down interface's own name, so Level 2 finds
	// ZERO matching routes and logs "nothing to do" even though the
	// tunnel is genuinely down and traffic genuinely needs to move. This
	// field lets attemptLevel2 ALSO match a route whose bare Gateway
	// equals this cached IP, closing that gap without needing to touch
	// RouterOS again during the outage itself (the IP a route points at
	// doesn't change just because resolution to an interface name
	// broke). Empty until the first healthy poll observes a real
	// ImmediateGw for this interface.
	LastKnownGatewayIP string `gorm:"type:varchar(64);not null;default:''"`

	LastPolledAt time.Time `gorm:"not null"`
}

// TunnelHealthSample is one poll tick's raw Tx/Rx counters for one
// tunnel interface -- a short rolling window (see TunnelHealthService.
// txRxSampleRetention) purely to feed the Tx/Rx asymmetry detector's own
// delta-over-time computation; not a long-term traffic-history table (see
// EtherTrafficSample/UsageSnapshot for that, in the Security/Reports
// sections -- this is a narrower, purpose-built buffer that gets pruned
// aggressively so it never becomes a second copy of that data).
type TunnelHealthSample struct {
	Model
	InterfaceName string    `gorm:"type:varchar(255);index;not null"`
	TxBytes       int64     `gorm:"type:bigint;not null"`
	RxBytes       int64     `gorm:"type:bigint;not null"`
	SampledAt     time.Time `gorm:"not null;index"`
}

// TunnelHealthEvent is the append-only history the spec calls
// detection_events -- one row per state TRANSITION (healthy->suspect,
// suspect->confirmed_down, any->healthy), not one row per poll tick, so
// this table grows with actual incidents rather than with polling
// frequency. Evidence is a short Persian sentence naming which signal(s)
// fired, matching the spec's own "message_fa" requirement for
// human-readable, Persian log lines.
type TunnelHealthEvent struct {
	Model
	InterfaceName string    `gorm:"type:varchar(255);index;not null"`
	FromStatus    string    `gorm:"type:varchar(32);not null"`
	ToStatus      string    `gorm:"type:varchar(32);not null"`
	Evidence      string    `gorm:"type:text;not null"`
	DetectedAt    time.Time `gorm:"not null;index"`
	// NotifiedAt is set once the Telegram alert for this transition has
	// been sent -- the admin's own explicit requirement that alerts fire
	// ONLY on a healthy->suspect (or ->confirmed_down) transition, never
	// on every poll tick a tunnel remains down.
	NotifiedAt *time.Time
}

// ---------------------------------------------------------------------
// Discovery + dependency graph (spec sections ب-1/ب-2/ب-3). Each poll
// tick writes ONE new DiscoverySnapshot per server (versioned, never
// overwritten -- needed both to build the graph and, later, for Config
// Drift detection per spec ب-4 method 5) and a fresh set of GraphNode/
// GraphEdge rows tied to that snapshot. Older snapshots/graphs are
// pruned to a small retention window (see TunnelGraphService's own
// pruning constant) so this stays a rolling window, not an
// ever-growing history table.
// ---------------------------------------------------------------------

// DiscoverySnapshot is one full read of a server's NAT/mangle/route/
// bridge/tunnel-interface configuration -- RawJSON holds the complete
// raw discovery payload (every resource this poll tick read), so a
// human or a future Config Drift detector can diff two snapshots without
// re-deriving anything from the graph tables.
type DiscoverySnapshot struct {
	Model
	ServerID uint      `gorm:"index;not null"`
	TakenAt  time.Time `gorm:"not null;index"`
	RawJSON  string    `gorm:"type:text;not null"`
}

// GraphNode is one entity discovered on a server at a given snapshot --
// Type is one of "nat_rule" | "address" | "bridge" | "mangle_rule" |
// "routing_table" | "route" | "tunnel_interface" | "pool" | "profile"
// (spec ب-2's own node-type vocabulary). MikrotikRefID is that entity's
// own RouterOS ".id" (or, for types RouterOS doesn't ID directly, like a
// routing-table name, the natural key itself) -- kept as a string since
// RouterOS ids are themselves strings like "*1A".
type GraphNode struct {
	Model
	SnapshotID     uint   `gorm:"index;not null"`
	Type           string `gorm:"type:varchar(32);not null;index"`
	MikrotikRefID  string `gorm:"type:varchar(64);not null"`
	Name           string `gorm:"type:varchar(255)"`
	PropertiesJSON string `gorm:"type:text;not null;default:'{}'"`
}

// GraphEdge is one directed relationship between two GraphNode rows in
// the SAME snapshot -- Relation is one of the spec's own fixed
// vocabulary (ب-2): nat_targets, mangled_by, member_of_bridge,
// routed_via_table, table_routes_to_gateway, gateway_is_tunnel,
// pool_used_by_profile.
type GraphEdge struct {
	Model
	SnapshotID uint   `gorm:"index;not null"`
	FromNodeID uint   `gorm:"index;not null"`
	ToNodeID   uint   `gorm:"index;not null"`
	Relation   string `gorm:"type:varchar(32);not null"`
}

// ---------------------------------------------------------------------
// Phase 2: the self-healing decision engine (spec section ب-5 onward).
// Everything below is deliberately usable in two modes controlled by a
// single global toggle (SystemConfig key "tunnel_ai_dry_run", see
// TunnelHealthService.IsDryRun) -- the admin's own explicit current
// requirement: "دستوراتی که میخواد انجام بده رو فقط توی ui نشان بده، نه
// عملی" (only show the commands it WOULD run in the UI, don't actually
// run them). Dry-run is the default/safe state; every remediation
// attempt is recorded in TunnelActionLog with Simulated=true/false, and
// in dry-run mode ExecuteRemediation stops immediately after computing
// and logging the command, before ever calling the mikrotik adaptor.
// ---------------------------------------------------------------------

// TunnelPolicy is one infrastructure tunnel's per-tunnel remediation
// policy -- the JSON shape mirrors the spec's own tunnel_policies example
// (section ب-7) verbatim: detection thresholds, level1/level2/level3
// remediation recipes, fallback, anti-flapping. Stored as a single JSON
// blob (PolicyJSON) rather than normalized columns because the shape is
// admin-edited as a whole document in the "سیاست هر تانل" subpage, and
// different tunnels may only populate a subset of levels (e.g. level3
// disabled) -- see TunnelPolicyConfig in service/tunnel_policy.go for the
// parsed Go struct this JSON deserializes into. Keyed by InterfaceName,
// same convention as TunnelHealthStatus -- see this file's own top-level
// doc comment for why (a GRE tunnel has no model.Interface row to FK to).
type TunnelPolicy struct {
	Model
	InterfaceName string `gorm:"type:varchar(255);uniqueIndex;not null"`
	PolicyJSON    string `gorm:"type:text;not null"`
}

// ManagementRedlineEntry is one protected interface/IP/port the admin
// uses for their OWN management connection to a server -- spec section
// ب-6's "منطقه امن مدیریتی" (management safe zone). Deliberately a
// SEPARATE table from TunnelPolicy, checked by an independent validator
// (see service/tunnel_redline.go) before every single mutating command,
// so a bug in the decision engine can never accidentally touch it -- the
// spec's own explicit requirement that this check "باید مستقل از منطق
// تصمیم‌گیری باشد" (must be independent of the decision logic).
type ManagementRedlineEntry struct {
	Model
	// Kind is "interface" | "ip" | "port" -- which field below is
	// populated depends on this.
	Kind    string  `gorm:"type:varchar(16);not null"`
	Value   string  `gorm:"type:varchar(255);not null"`
	Comment *string `gorm:"type:varchar(255)"`
}

// TunnelActionLog is the append-only record of every remediation
// DECISION the engine made -- one row per attempted action, whether it
// was actually sent to RouterOS or only simulated. This is the spec's
// own actions_log (section ب-2): command_sent, result, executed_at.
// Simulated=true is what powers the admin's current dry-run request --
// the UI's "تاریخچه و گزارش‌ها" page reads this table directly to show
// "would have run X" without X ever having been sent.
type TunnelActionLog struct {
	Model
	InterfaceName string `gorm:"type:varchar(255);index;not null"`
	// Level is "1" | "2" | "3" -- matches the spec's own
	// REMEDIATING_L1/FAILED_OVER_L2/AUTO_ROUTED_L3 state names.
	Level string `gorm:"type:varchar(8);not null"`
	// CommandDescription is a short Persian sentence describing the
	// action (e.g. "غیرفعال و مجدداً فعال کردن اینترفیس"), and
	// CommandDetail is the literal RouterOS REST call that either was or
	// would have been made (method + path + body) -- both are always
	// populated, dry-run or not, since the whole point of dry-run is to
	// let the admin read CommandDetail with confidence it's exactly what
	// would be sent for real.
	CommandDescription string `gorm:"type:text;not null"`
	CommandDetail      string `gorm:"type:text;not null"`
	// Simulated=true means this action was computed and logged but NEVER
	// sent to the mikrotik adaptor -- see TunnelHealthService.IsDryRun.
	//
	// CONFIRMED PRODUCTION BUG (found while adding real Level 2 execution
	// and writing a real regression test against it): `gorm:"default:true"`
	// on a bool column makes GORM's Create() treat Go's own zero value for
	// bool (false) as "field not set" and silently substitute the schema
	// default instead -- every single logAction(..., simulated: false, ...)
	// call in this whole file (a REAL, successful Level 1 remediation, or
	// now a real Level 2 failover) was being persisted with Simulated=true
	// regardless, even though the real RouterOS PATCH genuinely executed.
	// The actual remediation was never affected (this only corrupted the
	// AUDIT TRAIL, not the action itself), but an admin reading
	// TunnelActionLog had no way to trust whether Simulated actually meant
	// what it says. Dropping the `default:` tag (the migration below
	// backfills existing rows to the same true-by-default value this tag
	// used to provide at the schema level, but from application code
	// instead, so GORM's Create() never overrides an explicit false again)
	// is the fix; the Go zero value staying `false` is fine since every
	// call site always sets this field explicitly (there is no code path
	// that creates a TunnelActionLog without deciding this value first).
	Simulated bool `gorm:"not null"`
	// Result is "" while pending (should never persist that way -- every
	// row is written after the outcome, real or simulated, is already
	// known), "شبیه‌سازی شد" for a simulated action, or the real outcome
	// ("موفق" / an error string) for a live action.
	Result     string    `gorm:"type:text;not null"`
	ExecutedAt time.Time `gorm:"not null;index"`
}

// ---------------------------------------------------------------------
// Health trend + early warning. Everything in Poll up to this point is
// binary (healthy/suspect/confirmed_down, decided fresh every tick with
// no memory of the tunnel's own recent trajectory). TunnelHealthScore
// adds a rolling 0-100 score computed from the SAME signals Poll already
// reads (handshake age, tx/rx ratio, running/disabled) so a tunnel that
// is visibly degrading -- but hasn't yet crossed the suspect/
// confirmed_down threshold -- can trigger an early Telegram warning
// instead of the admin only finding out once it's already down.
// ---------------------------------------------------------------------

// TunnelHealthScore is one poll tick's computed 0-100 health score for
// one tunnel -- 100 is perfectly healthy, 0 is confirmed_down. Kept as
// its own small history table (not folded into TunnelHealthStatus, which
// is current-state-only per its own doc comment) so the panel can chart
// a trend line and so the early-warning detector can compare "now" to
// "N samples ago" without re-deriving history from TunnelHealthSample
// (which only carries raw tx/rx counters, not a combined score).
// Retention mirrors TunnelHealthSample's own rolling-window approach --
// see TunnelHealthService's own pruning constant -- so this never grows
// into a full time-series store.
type TunnelHealthScore struct {
	Model
	InterfaceName string    `gorm:"type:varchar(255);index;not null"`
	Score         int       `gorm:"not null"`
	SampledAt     time.Time `gorm:"not null;index"`
}

// TunnelIncidentDiagnosis is the self-healing engine's own root-cause
// guess for one confirmed_down incident -- computed once, the first time
// a tunnel is confirmed down (not repeated every poll tick while it stays
// down), by actively pinging a small set of OTHER destinations from the
// same router (see TunnelHealthService.diagnoseIncident) and reading the
// router's own CPU/memory at that moment. This tells apart three
// operationally very different situations that a plain "tunnel X is
// down" alert cannot distinguish on its own: the tunnel's own remote end
// is unreachable while the rest of the router's connectivity is fine
// (Cause = "remote_unreachable"), the WHOLE router has lost upstream
// connectivity (every other tunnel/probe target is also unreachable,
// Cause = "router_uplink_down"), or the router itself is overloaded
// (Cause = "router_resource_exhausted", CPU/memory reads back
// abnormally high at the same moment). Cause = "unknown" when none of
// the probes gives a clear signal either way -- a deliberately honest
// "couldn't tell" rather than guessing.
type TunnelIncidentDiagnosis struct {
	Model
	InterfaceName string `gorm:"type:varchar(255);index;not null"`
	// Cause is "remote_unreachable" | "router_uplink_down" |
	// "router_resource_exhausted" | "unknown".
	Cause       string    `gorm:"type:varchar(32);not null"`
	Evidence    string    `gorm:"type:text;not null"`
	DiagnosedAt time.Time `gorm:"not null;index"`
}

// TunnelBackupProbeResult is one nightly active-probe result for a
// tunnel's OWN configured Level 2 backup gateway -- spec-adjacent
// capability the admin asked for: confirm the backup path a Level 2
// failover would actually switch traffic onto is itself reachable,
// BEFORE a real incident ever needs it, rather than discovering a dead
// backup only during a live double-failure. Deliberately its own small
// table (one row appended per tunnel per probe run, pruned to a rolling
// window) rather than folded into TunnelHealthStatus, since this probes
// a DIFFERENT address (the backup gateway) than the tunnel's own health
// signals do.
type TunnelBackupProbeResult struct {
	Model
	InterfaceName   string    `gorm:"type:varchar(255);index;not null"`
	BackupGatewayIP string    `gorm:"type:varchar(64);not null"`
	Reachable       bool      `gorm:"not null"`
	Evidence        string    `gorm:"type:text;not null"`
	ProbedAt        time.Time `gorm:"not null;index"`
}

// ---------------------------------------------------------------------
// User Manager protocol health (spec section ب-7's own dedicated row for
// L2TP/PPTP/OpenVPN/SSTP): "پروب فعال به سرویس User Manager... فقط
// هشدار تلگرام، نه اقدام خودکار" (active probe to the User Manager
// service, alert-only, no automatic action -- restarting could disrupt
// active sessions). Deliberately a SEPARATE table from
// TunnelHealthStatus/TunnelActionLog: these four protocols are
// GLOBAL/singleton config on this codebase's real RouterOS setup (one
// port per protocol, not per-location -- see
// UserManagerProtocolConfig's own doc comment), never fed into the
// Level 1/2 remediation engine, and use a two-signal hybrid check
// (confirmed with the admin) rather than the tunnel engine's three
// WireGuard/GRE signals: (1) Enabled read directly from RouterOS's own
// l2tp-server/pptp-server/sstp-server/ovpn-server config, (2) a
// TCP-connect probe to the configured port, only attempted when (1) is
// true. Both signals are recorded independently so an alert can say
// exactly which one failed.
type UserManagerProtocolHealthStatus struct {
	Model
	Protocol UserManagerAccountProtocol `gorm:"type:varchar(16);uniqueIndex;not null"`

	// Healthy is true only when BOTH signals pass (or when the admin has
	// not enabled this protocol at all -- an intentionally-off protocol
	// is not a failure, so Healthy stays true and no alert ever fires
	// for it; see TunnelHealthService's own doc comment on this).
	Healthy bool `gorm:"not null;default:true"`

	// RouterEnabled/PortReachable are the two independent signals -- both
	// stored so the admin can see exactly which one failed, matching the
	// admin's own explicit request that each be recorded as separate
	// evidence.
	RouterEnabled bool `gorm:"not null;default:false"`
	PortReachable bool `gorm:"not null;default:false"`

	LastCheckedAt time.Time `gorm:"not null"`
}

// UserManagerProtocolHealthEvent is the append-only transition history
// for UserManagerProtocolHealthStatus -- mirrors TunnelHealthEvent's own
// "one row per transition, not per poll tick" shape, kept as a separate
// table since these two health domains (infrastructure tunnels vs.
// User Manager protocol availability) are conceptually and structurally
// independent per the spec's own section ب-7 distinction.
type UserManagerProtocolHealthEvent struct {
	Model
	Protocol   UserManagerAccountProtocol `gorm:"type:varchar(16);index;not null"`
	FromStatus string                     `gorm:"type:varchar(16);not null"` // "healthy" | "unhealthy"
	ToStatus   string                     `gorm:"type:varchar(16);not null"`
	Evidence   string                     `gorm:"type:text;not null"`
	DetectedAt time.Time                  `gorm:"not null;index"`
	NotifiedAt *time.Time
}
