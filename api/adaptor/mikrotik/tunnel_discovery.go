package mikrotik

import (
	"context"
	"encoding/json"
	"strconv"

	"github.com/maahdima/mwp/api/common"
)

// This file implements spec section ب-1's discovery reads for the
// "tunnel-ai" dependency graph (ب-2/ب-3): NAT -> address -> bridge ->
// mangle -> routing-table -> route -> tunnel interface. Every type here
// is deliberately a loose, mostly-string mirror of RouterOS's own REST
// JSON shape (same convention as WireGuardInterface/WireGuardPeer) --
// fields are read-only inputs to the graph builder, never round-tripped
// back through an Update/Create call, so there is no need for the
// stricter typing a mutating adaptor would want.

type IPPoolEntry struct {
	ID     string `json:".id,omitempty"`
	Name   string `json:"name,omitempty"`
	Ranges string `json:"ranges,omitempty"`
}

func (a *Adaptor) FetchIPPools(c context.Context) ([]IPPoolEntry, error) {
	var pools []IPPoolEntry
	err := a.mwpClients.GetClient(nil).Get(c, common.IPPoolPath, &pools)
	if err != nil {
		return nil, err
	}
	return pools, nil
}

// FirewallRule mirrors one row from /ip/firewall/filter, /ip/firewall/nat,
// or /ip/firewall/mangle -- the three resources share almost identical
// field shapes on RouterOS's own REST API, differing mainly in which
// optional fields are populated for a given chain/action, so one shared
// struct (rather than three near-duplicates) covers all three call
// sites below.
type FirewallRule struct {
	ID                string  `json:".id,omitempty"`
	Chain             string  `json:"chain,omitempty"`
	Action            string  `json:"action,omitempty"`
	SrcAddress        *string `json:"src-address,omitempty"`
	DstAddress        *string `json:"dst-address,omitempty"`
	DstPort           *string `json:"dst-port,omitempty"`
	Protocol          *string `json:"protocol,omitempty"`
	InInterface       *string `json:"in-interface,omitempty"`
	OutInterface      *string `json:"out-interface,omitempty"`
	ToAddresses       *string `json:"to-addresses,omitempty"`
	ToPorts           *string `json:"to-ports,omitempty"`
	ConnectionMark    *string `json:"connection-mark,omitempty"`
	NewConnectionMark *string `json:"new-connection-mark,omitempty"`
	NewRoutingMark    *string `json:"new-routing-mark,omitempty"`
	Passthrough       *string `json:"passthrough,omitempty"`
	Disabled          string  `json:"disabled,omitempty"`
	Comment           *string `json:"comment,omitempty"`
}

func (a *Adaptor) FetchFirewallFilterRules(c context.Context) ([]FirewallRule, error) {
	var rules []FirewallRule
	if err := a.mwpClients.GetClient(nil).Get(c, common.FirewallFilterPath, &rules); err != nil {
		return nil, err
	}
	return rules, nil
}

func (a *Adaptor) FetchFirewallNatRules(c context.Context) ([]FirewallRule, error) {
	var rules []FirewallRule
	if err := a.mwpClients.GetClient(nil).Get(c, common.FirewallNatPath, &rules); err != nil {
		return nil, err
	}
	return rules, nil
}

func (a *Adaptor) FetchFirewallMangleRules(c context.Context) ([]FirewallRule, error) {
	var rules []FirewallRule
	if err := a.mwpClients.GetClient(nil).Get(c, common.FirewallManglePath, &rules); err != nil {
		return nil, err
	}
	return rules, nil
}

type FirewallAddressListEntry struct {
	ID      string  `json:".id,omitempty"`
	List    string  `json:"list,omitempty"`
	Address string  `json:"address,omitempty"`
	Timeout *string `json:"timeout,omitempty"`
}

func (a *Adaptor) FetchFirewallAddressList(c context.Context) ([]FirewallAddressListEntry, error) {
	var entries []FirewallAddressListEntry
	if err := a.mwpClients.GetClient(nil).Get(c, common.FirewallAddressListPath, &entries); err != nil {
		return nil, err
	}
	return entries, nil
}

// RouteEntry mirrors both /ip/route (v6-style, still present in v7 as a
// legacy/compat view) and /routing/route (v7's own richer shape) -- the
// fields this graph builder actually needs (dst-address, gateway,
// routing-table, active) exist under the same names on both, so one
// struct covers both call sites; RoutingTable falls back to reading
// "vrf-interface"-scoped rows as table "main" when routing-table itself
// is absent, matching RouterOS's own default.
type RouteEntry struct {
	ID           string  `json:".id,omitempty"`
	DstAddress   string  `json:"dst-address,omitempty"`
	Gateway      *string `json:"gateway,omitempty"`
	RoutingTable *string `json:"routing-table,omitempty"`
	Distance     *string `json:"distance,omitempty"`
	Active       *string `json:"active,omitempty"`
	// Connect="true" marks RouterOS's own AUTO-GENERATED directly-connected
	// route that exists for every interface carrying an IP address
	// (confirmed against a real CHR: ether1, wireguard1, and every GRE
	// interface each get one of these) -- it means "this network is
	// reachable because this interface's own IP sits on it," NOT "traffic
	// is being forwarded through this interface toward some other
	// destination." A tunnel-detection rule that doesn't exclude these
	// would wrongly flag EVERY interface with an address as a "tunnel,"
	// including ordinary customer-facing WireGuard peer interfaces --
	// see discoveryData.tunnelGatewayNames' own doc comment for the
	// reported bug this fixes. Static string, not bool, since it may be
	// entirely absent on a real forwarding route.
	Connect *string `json:"connect,omitempty"`
	// Disabled="true" is a route the admin turned off (or a leftover from
	// a past config change) -- confirmed on a real production router to
	// leave several stale "0.0.0.0/0 -> wg1"-style disabled routes sitting
	// in non-main routing tables (VPN/VPN1/NAMAHDOD) pointing at ordinary
	// numbered WireGuard peer interfaces. Without excluding these, the
	// detection rule wrongly flagged those peers as infrastructure tunnels
	// purely because of dead config that forwards no real traffic.
	Disabled *string `json:"disabled,omitempty"`
	// ImmediateGw is RouterOS's own resolved "<next-hop-ip>%<interface>"
	// pair for this route (present on /ip/route; confirmed on a real
	// production router). Gateway itself is very often a bare IP address
	// (the next hop to recursively resolve), NOT an interface name -- e.g.
	// a route to a Google IP range with gateway="100.100.77.1" is only
	// revealed to actually go out gre-tunnel_DB via ImmediateGw
	// ("100.100.77.1%gre-tunnel_DB"). A detection rule that only ever
	// checks Gateway against known interface names misses every tunnel
	// reached through a recursive/IP gateway like this -- see
	// discoveryData.tunnelGatewayNames' own doc comment.
	ImmediateGw *string `json:"immediate-gw,omitempty"`
}

func (a *Adaptor) FetchIPRoutes(c context.Context) ([]RouteEntry, error) {
	var routes []RouteEntry
	if err := a.mwpClients.GetClient(nil).Get(c, common.IPRoutePath, &routes); err != nil {
		return nil, err
	}
	return routes, nil
}

// SetRouteGateway PATCHes a single /ip/route entry's own gateway field by
// its ".id" -- the self-healing engine's Level 2 action (tunnel
// failover). Deliberately the ONLY field this touches: dst-address,
// routing-table, distance, and every other property of the route are
// left completely untouched, so a failover can never accidentally
// reshape what traffic the route matches or which routing table it
// belongs to -- only WHERE that already-matched traffic goes next.
func (a *Adaptor) SetRouteGateway(c context.Context, routeID string, gatewayIP string) error {
	httpClient := a.mwpClients.GetClient(nil)
	return httpClient.Patch(
		c,
		common.IPRoutePath+"/"+routeID,
		RouteEntry{Gateway: &gatewayIP},
		&RouteEntry{},
	)
}

// FetchRoutingRoutes reads /routing/route -- the v7-only richer route
// table. Callers should treat a failure here as "this router is v6, use
// FetchIPRoutes instead" (spec section ب-1's own note that /routing/route
// only exists on v7) rather than a fatal discovery error, since this
// codebase's fleet may not be 100% v7 forever even though it is today.
func (a *Adaptor) FetchRoutingRoutes(c context.Context) ([]RouteEntry, error) {
	var routes []RouteEntry
	if err := a.mwpClients.GetClient(nil).Get(c, common.RoutingRoutePath, &routes); err != nil {
		return nil, err
	}
	return routes, nil
}

type RoutingTableEntry struct {
	ID   string `json:".id,omitempty"`
	Name string `json:"name,omitempty"`
	Fib  string `json:"fib,omitempty"`
}

func (a *Adaptor) FetchRoutingTables(c context.Context) ([]RoutingTableEntry, error) {
	var tables []RoutingTableEntry
	if err := a.mwpClients.GetClient(nil).Get(c, common.RoutingTablePath, &tables); err != nil {
		return nil, err
	}
	return tables, nil
}

type BridgeEntry struct {
	ID   string `json:".id,omitempty"`
	Name string `json:"name,omitempty"`
}

func (a *Adaptor) FetchBridges(c context.Context) ([]BridgeEntry, error) {
	var bridges []BridgeEntry
	if err := a.mwpClients.GetClient(nil).Get(c, common.BridgePath, &bridges); err != nil {
		return nil, err
	}
	return bridges, nil
}

type BridgePortEntry struct {
	ID        string `json:".id,omitempty"`
	Interface string `json:"interface,omitempty"`
	Bridge    string `json:"bridge,omitempty"`
}

func (a *Adaptor) FetchBridgePorts(c context.Context) ([]BridgePortEntry, error) {
	var ports []BridgePortEntry
	if err := a.mwpClients.GetClient(nil).Get(c, common.BridgePortPath, &ports); err != nil {
		return nil, err
	}
	return ports, nil
}

// TunnelInterfaceEntry mirrors one row from /interface/gre,
// /interface/ipip, or /interface/eoip -- these three non-WireGuard
// tunnel types share the same minimal shape this graph builder needs
// (name/running/disabled), so one struct covers all three, matching
// FirewallRule's own "shared shape, three call sites" pattern above.
type TunnelInterfaceEntry struct {
	ID         string  `json:".id,omitempty"`
	Name       string  `json:"name,omitempty"`
	Disabled   string  `json:"disabled,omitempty"`
	Running    *string `json:"running,omitempty"`
	RemoteAddr *string `json:"remote-address,omitempty"`
	LocalAddr  *string `json:"local-address,omitempty"`
}

func (a *Adaptor) FetchGREInterfaces(c context.Context) ([]TunnelInterfaceEntry, error) {
	var ifaces []TunnelInterfaceEntry
	if err := a.mwpClients.GetClient(nil).Get(c, common.GREInterfacePath, &ifaces); err != nil {
		return nil, err
	}
	return ifaces, nil
}

func (a *Adaptor) FetchIPIPInterfaces(c context.Context) ([]TunnelInterfaceEntry, error) {
	var ifaces []TunnelInterfaceEntry
	if err := a.mwpClients.GetClient(nil).Get(c, common.IPIPInterfacePath, &ifaces); err != nil {
		return nil, err
	}
	return ifaces, nil
}

func (a *Adaptor) FetchEoIPInterfaces(c context.Context) ([]TunnelInterfaceEntry, error) {
	var ifaces []TunnelInterfaceEntry
	if err := a.mwpClients.GetClient(nil).Get(c, common.EoIPInterfacePath, &ifaces); err != nil {
		return nil, err
	}
	return ifaces, nil
}

// CreateGREInterface/CreateIPIPInterface/CreateEoIPInterface are the
// self-healing engine's Level 3 action (spec section ب-5, "ساخت مسیر
// جدید") -- each PUTs a new point-to-point tunnel interface to its own
// /interface/{gre,ipip,eoip} list, matching CreateWgInterface's own
// established PUT-to-the-list-path convention. Only Name/RemoteAddr/
// LocalAddr are ever set here (LocalAddr left nil sends no local-address
// at all, letting RouterOS auto-select one -- the config's own "auto"
// sentinel is translated to this by the caller); MTU/keepalive/etc. all
// keep RouterOS's own defaults, since Level 3's whole purpose is
// emergency connectivity restoration, not replicating every tuning knob
// of a hand-configured tunnel.
func (a *Adaptor) CreateGREInterface(c context.Context, name string, remoteAddr string, localAddr *string) (*TunnelInterfaceEntry, error) {
	return a.createTunnelInterface(c, common.GREInterfacePath, name, remoteAddr, localAddr)
}

func (a *Adaptor) CreateIPIPInterface(c context.Context, name string, remoteAddr string, localAddr *string) (*TunnelInterfaceEntry, error) {
	return a.createTunnelInterface(c, common.IPIPInterfacePath, name, remoteAddr, localAddr)
}

func (a *Adaptor) CreateEoIPInterface(c context.Context, name string, remoteAddr string, localAddr *string) (*TunnelInterfaceEntry, error) {
	return a.createTunnelInterface(c, common.EoIPInterfacePath, name, remoteAddr, localAddr)
}

func (a *Adaptor) createTunnelInterface(c context.Context, path string, name string, remoteAddr string, localAddr *string) (*TunnelInterfaceEntry, error) {
	payload := TunnelInterfaceEntry{
		Name:       name,
		RemoteAddr: &remoteAddr,
		LocalAddr:  localAddr,
	}
	var created TunnelInterfaceEntry
	if err := a.mwpClients.GetClient(nil).Put(c, path, payload, &created); err != nil {
		return nil, err
	}
	return &created, nil
}

// CreateRoute PUTs a new /ip/route entry -- the second half of Level 3's
// "ساخت مسیر جدید": once the new tunnel interface itself exists (see
// CreateGREInterface et al above), traffic still needs an actual route
// pointing at it. gatewayIP mirrors attemptLevel2's own BackupGatewayIP
// convention -- the admin-configured next-hop IP through the NEW tunnel,
// since (as established for Level 2) a route's gateway on this fleet is
// always resolved via a next-hop IP, never a bare interface name.
func (a *Adaptor) CreateRoute(c context.Context, dstAddress string, gatewayIP string) (*RouteEntry, error) {
	payload := RouteEntry{
		DstAddress: dstAddress,
		Gateway:    &gatewayIP,
	}
	var created RouteEntry
	if err := a.mwpClients.GetClient(nil).Put(c, common.IPRoutePath, payload, &created); err != nil {
		return nil, err
	}
	return &created, nil
}

// PingResult is one echo reply from RouterOS's own `/ping` REST call --
// field names mirror RouterOS's REST JSON verbatim. A reply that timed
// out omits "time"/"ttl" entirely and reports a non-zero "timeout"
// count instead (there is no single top-level "success" field), so
// callers should treat Time == "" as a lost echo.
type PingResult struct {
	Host    string `json:"host,omitempty"`
	Time    string `json:"time,omitempty"`
	TTL     string `json:"ttl,omitempty"`
	Seq     string `json:"seq,omitempty"`
	Timeout string `json:"timeout,omitempty"`
}

// Ping runs a bounded `/ping` sample against address -- count controls
// how many echoes RouterOS itself sends before returning (this blocks
// for roughly that many round-trips, same bounded-snapshot shape as
// FetchTorchFlows, never a live stream). Used by the tunnel self-healing
// engine for root-cause diagnosis (telling "this one tunnel is dead"
// apart from "the router/uplink itself is dead" by pinging other
// destinations from the same router) and for the nightly active
// backup-path probe (confirming a configured Level 2 gateway is
// reachable before an incident ever needs it).
func (a *Adaptor) Ping(c context.Context, address string, count int) ([]PingResult, error) {
	var results []PingResult

	reqBody := map[string]string{
		"address": address,
		"count":   strconv.Itoa(count),
	}

	if err := a.mwpClients.GetClient(nil).Post(c, common.ToolPingPath, reqBody, &results); err != nil {
		return nil, err
	}
	return results, nil
}

// PPPServerConfig mirrors the singleton "server enabled?" config object
// shared by RouterOS's l2tp-server/pptp-server/sstp-server/ovpn-server
// paths -- confirmed against a real CHR (each of the four returns this
// same {enabled, port} shape, though only sstp-server/ovpn-server carry
// a meaningful "port" field; l2tp/pptp use fixed protocol ports and omit
// it). Used by the User Manager protocol health check (spec section
// ب-7's own hybrid probe: read Enabled here first, only TCP-probe the
// port when Enabled=true).
type PPPServerConfig struct {
	Enabled string  `json:"enabled,omitempty"`
	Port    *string `json:"port,omitempty"`
	// UseIpsec is only ever meaningful on /interface/l2tp-server/server
	// (confirmed on a real production router: use-ipsec="yes"). When set,
	// the server's actual data-plane traffic (UDP 500/1701/4500 for the
	// IKE/L2TP/NAT-T handshake) is NOT a TCP service at all -- see
	// UserManagerProtocolHealthService.probeTCPPort's own doc comment for
	// why a TCP port-reachability probe is skipped whenever this is
	// "yes", instead of misreporting the server as unreachable every
	// single tick regardless of whether it is actually healthy.
	UseIpsec *string `json:"use-ipsec,omitempty"`
}

func (a *Adaptor) FetchL2TPServerConfig(c context.Context) (*PPPServerConfig, error) {
	var cfg PPPServerConfig
	if err := a.mwpClients.GetClient(nil).Get(c, common.L2TPServerPath, &cfg); err != nil {
		return nil, err
	}
	return &cfg, nil
}

func (a *Adaptor) FetchPPTPServerConfig(c context.Context) (*PPPServerConfig, error) {
	var cfg PPPServerConfig
	if err := a.mwpClients.GetClient(nil).Get(c, common.PPTPServerPath, &cfg); err != nil {
		return nil, err
	}
	return &cfg, nil
}

func (a *Adaptor) FetchSSTPServerConfig(c context.Context) (*PPPServerConfig, error) {
	var cfg PPPServerConfig
	if err := a.mwpClients.GetClient(nil).Get(c, common.SSTPServerPath, &cfg); err != nil {
		return nil, err
	}
	return &cfg, nil
}

// FetchOvpnServerConfig is more defensive than its l2tp/pptp/sstp
// siblings above: a confirmed, reported production finding is that
// GET /interface/ovpn-server/server can return a JSON ARRAY (`[]`) on
// at least one real router, unlike the other three protocols' own
// consistently-object singleton response (confirmed separately against
// a CHR test server) -- decoding straight into PPPServerConfig then
// fails with "cannot unmarshal array into Go value". Rather than assume
// either shape, this reads the raw body and tries both: an object first
// (the documented/expected singleton shape), then an array (falling
// back to its first element, or to Enabled=false with no error if the
// array is empty -- an empty list here most plausibly means "no OVPN
// server instance configured," which this health check should read as
// "not enabled" rather than fail on).
func (a *Adaptor) FetchOvpnServerConfig(c context.Context) (*PPPServerConfig, error) {
	var raw json.RawMessage
	if err := a.mwpClients.GetClient(nil).Get(c, common.OvpnServerPath, &raw); err != nil {
		return nil, err
	}

	var cfg PPPServerConfig
	if err := json.Unmarshal(raw, &cfg); err == nil {
		return &cfg, nil
	}

	var list []PPPServerConfig
	if err := json.Unmarshal(raw, &list); err != nil {
		return nil, err
	}
	if len(list) == 0 {
		return &PPPServerConfig{Enabled: "false"}, nil
	}
	return &list[0], nil
}
