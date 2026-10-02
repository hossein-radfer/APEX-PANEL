package common

var (
	DeviceInfoPath     = "/system/resource"
	DeviceIdentityPath = "/system/identity"
	DeviceDnsPath      = "/ip/dns"
	DeviceIPv4Path     = "/ip/address"
	InterfacePath      = "/interface"
	WGInterfacePath    = "/interface/wireguard"
	WGPeerPath         = "/interface/wireguard/peers"
	QueuePath          = "/queue/simple"
	SchedulerPath      = "/system/scheduler"

	UserManagerUserPath        = "/user-manager/user"
	UserManagerUserGroupPath   = "/user-manager/user/group"
	UserManagerProfilePath     = "/user-manager/profile"
	UserManagerUserProfilePath = "/user-manager/user-profile"
	PPPActivePath              = "/ppp/active"

	// ToolTorchPath is RouterOS's REST equivalent of the CLI's
	// `/tool torch` monitor command -- unlike every other path in this
	// file, this is a bounded-DURATION snapshot call (POST with a
	// "duration" field), not a plain resource GET, and it must never be
	// called with a long duration: see SecurityEtherTorchService's own doc
	// comment for why this codebase polls it on a short, fixed interval
	// with a short sample window instead of treating it like a live
	// stream.
	ToolTorchPath = "/tool/torch"

	// ToolPingPath is RouterOS's REST equivalent of the CLI's `/ping`
	// command -- POST with "address" and "count", blocks until `count`
	// echoes have been sent (or timed out) then returns one JSON array,
	// one element per echo, same bounded-snapshot shape as ToolTorchPath
	// above (never a live stream). Used for active reachability probes:
	// the tunnel self-healing engine's own root-cause diagnosis (pinging
	// other destinations from the SAME router to tell "this one tunnel is
	// dead" apart from "the whole router/uplink is dead") and its nightly
	// backup-path pre-check (confirming a configured Level 2 backup
	// gateway is actually reachable before an incident ever needs it).
	ToolPingPath = "/ping"

	// ContainerPath is RouterOS 7's native Docker-container feature
	// (/container) -- used by the V2Ray panel-container mapping (item 9):
	// alireza0's x-ui panels are hosted INSIDE a RouterOS container on
	// some registered Server, so when x-ui itself becomes unreachable,
	// this path lets the health check distinguish "the container actually
	// stopped" (a router-level fact, fixable by restarting the container)
	// from "the container is running but x-ui inside it is unresponsive"
	// (a different failure mode entirely). A plain resource GET, same
	// convention as every other path in this file.
	ContainerPath = "/container"
)

// tunnel-ai discovery paths (spec section ب-1) -- read-only GETs used to
// build the dependency graph (NAT -> address -> bridge -> mangle ->
// routing-table -> route -> tunnel interface, spec section ب-3). Every
// path here is a plain resource GET, same convention as every other path
// in this file -- no new HTTP verb/behavior introduced.
var (
	IPPoolPath              = "/ip/pool"
	FirewallFilterPath      = "/ip/firewall/filter"
	FirewallNatPath         = "/ip/firewall/nat"
	FirewallManglePath      = "/ip/firewall/mangle"
	FirewallAddressListPath = "/ip/firewall/address-list"
	IPRoutePath             = "/ip/route"
	RoutingRoutePath        = "/routing/route" // v7 -- see RoutingRoutePath's own caller for the v6 fallback note
	RoutingTablePath        = "/routing/table"
	BridgePath              = "/interface/bridge"
	BridgePortPath          = "/interface/bridge/port"
	GREInterfacePath        = "/interface/gre"
	IPIPInterfacePath       = "/interface/ipip"
	EoIPInterfacePath       = "/interface/eoip"
)

// User Manager protocol server-enabled config paths (spec section ب-7's
// own hybrid probe: check Enabled here first, then TCP-connect to Port
// only when Enabled=true) -- each is a RouterOS SINGLETON config object
// (one row, no .id), confirmed against a real CHR: GET returns a single
// JSON object with its own "enabled"/"port" fields, not an array.
var (
	L2TPServerPath = "/interface/l2tp-server/server"
	PPTPServerPath = "/interface/pptp-server/server"
	SSTPServerPath = "/interface/sstp-server/server"
	OvpnServerPath = "/interface/ovpn-server/server"
)

var (
	XuiLoginPath            = "/login"
	XuiInboundsPath         = "/xui/API/inbounds/"
	XuiInboundGetPathFmt    = "/xui/API/inbounds/get/%d"
	XuiAddClientPath        = "/xui/API/inbounds/addClient"
	XuiUpdateClientPathFmt  = "/xui/API/inbounds/updateClient/%s"
	XuiGetClientTrafficsFmt = "/xui/API/inbounds/getClientTraffics/%s"
	XuiOnlinesPath          = "/xui/API/inbounds/onlines"
)

var (
	IPv4DefaultInterface = "ether1"
)

var (
	SchedulerComment   = "Expire WireGuard Peer: "
	SchedulerName      = "Schedule: "
	SchedulerStartTime = "12:00:00"
	SchedulerInterval  = "00:00:00"
	SchedulerPolicy    = "read,write"
	SchedulerEvent     = "/interface/wireguard/peers/disable"
)

var (
	QueueComment     = "Wireguard Bandwidth Queue: "
	QueueName        = "Bandwidth Limit: "
	DefaultKeepalive = "25"
)

// TODO : Make these configurable
var (
	AllowedIpsExcludeLocal = ""
	AllowedIpsIncludeLocal = "0.0.0.0/0, ::/0"
	DefaultDns             = "65.6.65.6, 64.6.64.6"
)
