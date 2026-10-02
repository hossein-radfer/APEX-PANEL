package service

import (
	"context"
	"encoding/json"
	"strings"
	"time"

	"go.uber.org/zap"
	"gorm.io/gorm"

	"github.com/maahdima/mwp/api/adaptor/mikrotik"
	"github.com/maahdima/mwp/api/dataservice/model"
)

// Graph node/edge relation vocabulary -- fixed, per spec section ب-2.
const (
	nodeTypeNatRule         = "nat_rule"
	nodeTypeAddress         = "address"
	nodeTypeBridge          = "bridge"
	nodeTypeMangleRule      = "mangle_rule"
	nodeTypeRoutingTable    = "routing_table"
	nodeTypeRoute           = "route"
	nodeTypeTunnelInterface = "tunnel_interface"
	nodeTypePool            = "pool"

	relationNatTargets           = "nat_targets"
	relationMangledBy            = "mangled_by"
	relationMemberOfBridge       = "member_of_bridge"
	relationRoutedViaTable       = "routed_via_table"
	relationTableRoutesToGateway = "table_routes_to_gateway"
	relationGatewayIsTunnel      = "gateway_is_tunnel"

	// snapshotRetentionCount keeps only the last N snapshots (and their
	// graph rows) per server -- a rolling window for Config Drift
	// comparison (spec ب-4 method 5, "compare against the last known-good
	// snapshot"), not an ever-growing history table.
	snapshotRetentionCount = 20
)

// TunnelGraphService implements spec sections ب-1 (discovery) through
// ب-3 (dependency graph). Read-only: it only ever GETs from RouterOS and
// writes to its own snapshot/graph tables -- never mutates router state.
type TunnelGraphService struct {
	db              *gorm.DB
	mikrotikAdaptor *mikrotik.Adaptor
	logger          *zap.Logger
}

func NewTunnelGraphService(db *gorm.DB, mikrotikAdaptor *mikrotik.Adaptor) *TunnelGraphService {
	return &TunnelGraphService{
		db:              db,
		mikrotikAdaptor: mikrotikAdaptor,
		logger:          zap.L().Named("TunnelGraphService"),
	}
}

// discoveryData bundles every resource one poll tick reads -- built once
// per server, per tick, then fed to both the raw-snapshot JSON and the
// graph builder so the two are always consistent with each other (same
// single read, not two separate fetches that could race against a
// config change in between).
type discoveryData struct {
	Pools          []mikrotik.IPPoolEntry              `json:"pools"`
	Addresses      []mikrotik.IPAddress                `json:"addresses"`
	NatRules       []mikrotik.FirewallRule             `json:"nat_rules"`
	MangleRules    []mikrotik.FirewallRule             `json:"mangle_rules"`
	FilterRules    []mikrotik.FirewallRule             `json:"filter_rules"`
	AddressList    []mikrotik.FirewallAddressListEntry `json:"address_list"`
	Routes         []mikrotik.RouteEntry               `json:"routes"`
	RoutingTables  []mikrotik.RoutingTableEntry        `json:"routing_tables"`
	Bridges        []mikrotik.BridgeEntry              `json:"bridges"`
	BridgePorts    []mikrotik.BridgePortEntry          `json:"bridge_ports"`
	WgInterfaces   []mikrotik.WireGuardInterface       `json:"wg_interfaces"`
	GreInterfaces  []mikrotik.TunnelInterfaceEntry     `json:"gre_interfaces"`
	IpipInterfaces []mikrotik.TunnelInterfaceEntry     `json:"ipip_interfaces"`
	EoipInterfaces []mikrotik.TunnelInterfaceEntry     `json:"eoip_interfaces"`
	UsedV7Routes   bool                                `json:"used_v7_routes"`
}

// allInterfaceNames is every interface name this snapshot knows about,
// across every type read -- used only to tell "this route's Gateway
// field names a real interface" apart from "this route's Gateway field
// is an IP address to resolve recursively" (spec ب-3 step 7). It is
// deliberately NOT the tunnel-detection signal itself -- see
// tunnelGatewayNames below for that.
func (d *discoveryData) allInterfaceNames() map[string]bool {
	names := make(map[string]bool)
	for _, i := range d.WgInterfaces {
		names[i.Name] = true
	}
	for _, i := range d.GreInterfaces {
		names[i.Name] = true
	}
	for _, i := range d.IpipInterfaces {
		names[i.Name] = true
	}
	for _, i := range d.EoipInterfaces {
		names[i.Name] = true
	}
	return names
}

// interfaceTypeByName maps every known interface name to its real
// RouterOS type ("wireguard" | "gre" | "ipip" | "eoip") -- used to record
// which detection method applies once an interface is confirmed to be a
// genuine infrastructure tunnel (see tunnelGatewayNames below).
func (d *discoveryData) interfaceTypeByName() map[string]string {
	types := make(map[string]string)
	for _, i := range d.WgInterfaces {
		types[i.Name] = "wireguard"
	}
	for _, i := range d.GreInterfaces {
		types[i.Name] = "gre"
	}
	for _, i := range d.IpipInterfaces {
		types[i.Name] = "ipip"
	}
	for _, i := range d.EoipInterfaces {
		types[i.Name] = "eoip"
	}
	return types
}

// routeGatewayInterface resolves the real outgoing interface for a route.
// ImmediateGw ("<next-hop-ip>%<interface>", confirmed against a real
// production router) is authoritative when present -- it is RouterOS's
// own resolution of a recursive/IP gateway down to the interface that
// actually carries the traffic, which a route's bare Gateway field does
// not give you when Gateway is an IP address rather than an interface
// name (see RouteEntry.ImmediateGw's own doc comment). Falls back to
// Gateway itself (also handling its own rarer "ip%interface" form) only
// when ImmediateGw is absent.
func routeGatewayInterface(r mikrotik.RouteEntry) string {
	if r.ImmediateGw != nil && *r.ImmediateGw != "" {
		if idx := strings.LastIndex(*r.ImmediateGw, "%"); idx != -1 {
			return (*r.ImmediateGw)[idx+1:]
		}
		return *r.ImmediateGw
	}
	if r.Gateway == nil || *r.Gateway == "" {
		return ""
	}
	if idx := strings.LastIndex(*r.Gateway, "%"); idx != -1 {
		return (*r.Gateway)[idx+1:]
	}
	return *r.Gateway
}

// tunnelGatewayNames is the ACTUAL infrastructure-tunnel detection rule,
// per the admin's own explicit correction to an earlier, wrong version
// of this code that flagged every WireGuard/GRE/IPIP/EoIP interface as a
// "tunnel" purely by type/name -- that wrongly caught ordinary
// customer-facing WireGuard peer interfaces (which are also, trivially,
// "a WireGuard interface") alongside genuine inter-server links. The
// only property that actually distinguishes infrastructure from a
// customer endpoint is: something in the real routing table uses this
// interface as a GATEWAY (traffic is routed THROUGH it to reach some
// other network) -- a customer's own WireGuard interface is always a
// route's DESTINATION network (e.g. 10.10.10.0/24), never a route's
// gateway. This is recomputed fresh from data.Routes every tick and
// works identically regardless of interface type, so a GRE tunnel today
// and a different tunnel technology tomorrow are both detected the same
// way with no hardcoded type list.
//
// Verified directly against a real production router's own REST output
// (not just the CHR): the earlier version of this rule both missed real
// tunnels (gre-tunnel_DB, gre-TR, gre-ho, gre-AR, gre-GR-h all route
// traffic via a next-hop IP, e.g. gateway="100.100.77.1", and only reveal
// their real outgoing interface via ImmediateGw -- see
// routeGatewayInterface above) and wrongly flagged ordinary numbered
// WireGuard peers (wg3/wg5/wg7/wg8) as tunnels purely because of stale
// Disabled="true" routes left over in non-main routing tables.
func (d *discoveryData) tunnelGatewayNames() map[string]bool {
	interfaceNames := d.allInterfaceNames()
	gateways := make(map[string]bool)
	for _, r := range d.Routes {
		// A disabled route forwards no real traffic -- see
		// RouteEntry.Disabled's own doc comment for the reported false
		// positive this excludes.
		if r.Disabled != nil && *r.Disabled == "true" {
			continue
		}
		// Exclude RouterOS's own auto-generated directly-connected routes
		// (Connect="true") -- see RouteEntry.Connect's own doc comment.
		// Every interface with an IP gets one of these; counting it here
		// would flag every such interface as a "tunnel," including
		// ordinary customer WireGuard peer interfaces.
		if r.Connect != nil && *r.Connect == "true" {
			continue
		}
		iface := routeGatewayInterface(r)
		if iface == "" {
			continue
		}
		if interfaceNames[iface] {
			gateways[iface] = true
		}
	}
	return gateways
}

// Poll is the scheduled job entry point -- resolves the single active
// server (this codebase's own established "one active server" convention,
// see model.Server.IsActive) and runs one discovery+graph-build tick
// against it. A no-active-server state (fresh install, or every server
// disabled) is a silent no-op, not an error -- same convention as this
// codebase's other server-scoped background jobs.
func (s *TunnelGraphService) Poll() {
	var server model.Server
	if err := s.db.Where("is_active = ?", true).First(&server).Error; err != nil {
		return
	}
	if err := s.Discover(server.ID); err != nil {
		s.logger.Error("tunnel graph discovery tick failed", zap.Uint("server_id", server.ID), zap.Error(err))
	}
}

// Discover runs one full read of serverID's NAT/mangle/route/bridge/
// tunnel-interface configuration, stores a versioned DiscoverySnapshot,
// and builds this tick's GraphNode/GraphEdge rows -- spec sections
// ب-1/ب-2/ب-3's own single entry point (this service does not currently
// support multiple concurrently-configured servers beyond the one the
// mikrotik adaptor's default client points at, matching every other
// service in this codebase's own "one active server" convention).
func (s *TunnelGraphService) Discover(serverID uint) error {
	ctx := context.Background()
	data, err := s.fetchAll(ctx)
	if err != nil {
		s.logger.Error("failed to fetch discovery data", zap.Error(err))
		return err
	}

	rawJSON, err := json.Marshal(data)
	if err != nil {
		return err
	}

	snapshot := model.DiscoverySnapshot{ServerID: serverID, TakenAt: time.Now(), RawJSON: string(rawJSON)}
	if err := s.db.Create(&snapshot).Error; err != nil {
		s.logger.Error("failed to store discovery snapshot", zap.Error(err))
		return err
	}

	if err := s.buildGraph(snapshot.ID, data); err != nil {
		s.logger.Error("failed to build dependency graph", zap.Uint("snapshot_id", snapshot.ID), zap.Error(err))
		return err
	}

	s.pruneOldSnapshots(serverID)
	return nil
}

// fetchAll reads every discovery resource. /routing/route (v7) is tried
// first; if it fails (spec ب-1's own note that this path is v7-only),
// this falls back to /ip/route (present on both v6 and v7) rather than
// treating the whole discovery tick as failed -- a v6 server can still
// build a graph, just via the older route-table view.
func (s *TunnelGraphService) fetchAll(ctx context.Context) (*discoveryData, error) {
	data := &discoveryData{}
	var err error

	if data.Pools, err = s.mikrotikAdaptor.FetchIPPools(ctx); err != nil {
		return nil, err
	}
	if data.Addresses, err = s.mikrotikAdaptor.FetchIPv4Addresses(ctx); err != nil {
		return nil, err
	}
	if data.NatRules, err = s.mikrotikAdaptor.FetchFirewallNatRules(ctx); err != nil {
		return nil, err
	}
	if data.MangleRules, err = s.mikrotikAdaptor.FetchFirewallMangleRules(ctx); err != nil {
		return nil, err
	}
	if data.FilterRules, err = s.mikrotikAdaptor.FetchFirewallFilterRules(ctx); err != nil {
		return nil, err
	}
	if data.AddressList, err = s.mikrotikAdaptor.FetchFirewallAddressList(ctx); err != nil {
		return nil, err
	}
	if data.RoutingTables, err = s.mikrotikAdaptor.FetchRoutingTables(ctx); err != nil {
		return nil, err
	}
	if data.Bridges, err = s.mikrotikAdaptor.FetchBridges(ctx); err != nil {
		return nil, err
	}
	if data.BridgePorts, err = s.mikrotikAdaptor.FetchBridgePorts(ctx); err != nil {
		return nil, err
	}
	if data.WgInterfaces, err = s.mikrotikAdaptor.FetchWgInterfaces(ctx); err != nil {
		return nil, err
	}
	if data.GreInterfaces, err = s.mikrotikAdaptor.FetchGREInterfaces(ctx); err != nil {
		return nil, err
	}
	if data.IpipInterfaces, err = s.mikrotikAdaptor.FetchIPIPInterfaces(ctx); err != nil {
		return nil, err
	}
	if data.EoipInterfaces, err = s.mikrotikAdaptor.FetchEoIPInterfaces(ctx); err != nil {
		return nil, err
	}

	if routes, rErr := s.mikrotikAdaptor.FetchRoutingRoutes(ctx); rErr == nil {
		data.Routes = routes
		data.UsedV7Routes = true
	} else {
		s.logger.Warn("routing/route unavailable (expected on RouterOS v6), falling back to ip/route", zap.Error(rErr))
		if data.Routes, err = s.mikrotikAdaptor.FetchIPRoutes(ctx); err != nil {
			return nil, err
		}
	}

	return data, nil
}

func (s *TunnelGraphService) pruneOldSnapshots(serverID uint) {
	var keepIDs []uint
	s.db.Model(&model.DiscoverySnapshot{}).
		Where("server_id = ?", serverID).
		Order("taken_at desc").
		Limit(snapshotRetentionCount).
		Pluck("id", &keepIDs)
	if len(keepIDs) == 0 {
		return
	}

	var staleIDs []uint
	s.db.Model(&model.DiscoverySnapshot{}).
		Where("server_id = ? AND id NOT IN ?", serverID, keepIDs).
		Pluck("id", &staleIDs)
	if len(staleIDs) == 0 {
		return
	}

	// Unscoped() is the actual fix here -- a confirmed, reported bug: plain
	// .Delete() on these three models (all embedding Model, which carries
	// gorm.DeletedAt) only sets deleted_at, leaving every "pruned" row
	// permanently on disk. On a live production database this left 99.7%
	// (5,690,578 of 5,708,260) of graph_nodes rows soft-deleted-but-present,
	// contributing well over 1GB of otherwise-recoverable space. Unscoped()
	// makes these genuine hard deletes, matching this function's own name
	// and doc comment ("prune") -- there is no product need to ever recover
	// a stale snapshot's graph data once a newer snapshot has superseded it.
	s.db.Unscoped().Where("snapshot_id IN ?", staleIDs).Delete(&model.GraphEdge{})
	s.db.Unscoped().Where("snapshot_id IN ?", staleIDs).Delete(&model.GraphNode{})
	s.db.Unscoped().Where("id IN ?", staleIDs).Delete(&model.DiscoverySnapshot{})
}

// LatestSnapshot returns serverID's most recent DiscoverySnapshot, or
// nil if none exists yet.
func (s *TunnelGraphService) LatestSnapshot(serverID uint) (*model.DiscoverySnapshot, error) {
	var snapshot model.DiscoverySnapshot
	err := s.db.Where("server_id = ?", serverID).Order("taken_at desc").First(&snapshot).Error
	if err != nil {
		if err == gorm.ErrRecordNotFound {
			return nil, nil
		}
		return nil, err
	}
	return &snapshot, nil
}

// LatestGraphForActiveServer resolves the single active server (same
// convention as Poll) and returns its most recent snapshot plus that
// snapshot's nodes/edges -- the HTTP controller's own single read
// endpoint for the graph inspector page, so it never needs to know a
// server ID itself.
func (s *TunnelGraphService) LatestGraphForActiveServer() (*model.DiscoverySnapshot, []model.GraphNode, []model.GraphEdge, error) {
	var server model.Server
	if err := s.db.Where("is_active = ?", true).First(&server).Error; err != nil {
		return nil, nil, nil, nil
	}
	snapshot, err := s.LatestSnapshot(server.ID)
	if err != nil || snapshot == nil {
		return snapshot, nil, nil, err
	}
	nodes, edges, err := s.NodesAndEdges(snapshot.ID)
	if err != nil {
		return snapshot, nil, nil, err
	}
	return snapshot, nodes, edges, nil
}

// DiscoveredTunnelInterfaces returns every interface name the MOST
// RECENT snapshot flagged as a genuine infrastructure tunnel (a
// tunnel_interface graph node -- see discoveryData.tunnelGatewayNames'
// own doc comment for the real detection rule: something in the routing
// table actually uses this interface as a gateway, independent of its
// type/name). This is TunnelHealthService's own entry point for deciding
// WHICH interfaces to poll for health -- it deliberately never falls
// back to scanning model.Interface (customer WireGuard peer interfaces),
// per the admin's own explicit correction that those are NOT
// infrastructure tunnels. Returns an empty slice (not an error) when no
// snapshot exists yet, so a fresh install simply has nothing to poll
// until the first discovery tick completes, rather than failing.
func (s *TunnelGraphService) DiscoveredTunnelInterfaces() ([]model.GraphNode, error) {
	var server model.Server
	if err := s.db.Where("is_active = ?", true).First(&server).Error; err != nil {
		return nil, nil
	}
	snapshot, err := s.LatestSnapshot(server.ID)
	if err != nil {
		return nil, err
	}
	if snapshot == nil {
		return nil, nil
	}
	var nodes []model.GraphNode
	if err := s.db.Where("snapshot_id = ? AND type = ?", snapshot.ID, nodeTypeTunnelInterface).Find(&nodes).Error; err != nil {
		return nil, err
	}
	return nodes, nil
}

// NodesAndEdges returns every GraphNode/GraphEdge for one snapshot -- the
// "دیده‌بان گراف" read endpoint that lets an admin inspect exactly what
// this tick's dependency chain looks like.
func (s *TunnelGraphService) NodesAndEdges(snapshotID uint) ([]model.GraphNode, []model.GraphEdge, error) {
	var nodes []model.GraphNode
	if err := s.db.Where("snapshot_id = ?", snapshotID).Find(&nodes).Error; err != nil {
		return nil, nil, err
	}
	var edges []model.GraphEdge
	if err := s.db.Where("snapshot_id = ?", snapshotID).Find(&edges).Error; err != nil {
		return nil, nil, err
	}
	return nodes, edges, nil
}

// ResolveTunnelForNatRule walks the SAME recursive chain buildGraph
// already computed edges for for a given NAT rule's graph node, and
// returns the name of the tunnel interface it ultimately resolves to (or
// "" if the chain doesn't reach one within this snapshot) -- this is the
// spec's own worked example (ب-3, "example 1: NAT to tunnel") exposed as
// a direct query rather than requiring a caller to walk the edge table
// themselves.
func (s *TunnelGraphService) ResolveTunnelForNatRule(snapshotID uint, natRuleNodeID uint) (string, error) {
	visited := make(map[uint]bool)
	return s.walkToTunnel(snapshotID, natRuleNodeID, visited)
}

func (s *TunnelGraphService) walkToTunnel(snapshotID uint, nodeID uint, visited map[uint]bool) (string, error) {
	if visited[nodeID] {
		// A cycle in the graph (shouldn't happen with well-formed
		// RouterOS config, but a defensive stop is cheap insurance
		// against ever looping forever on a pathological one).
		return "", nil
	}
	visited[nodeID] = true

	var node model.GraphNode
	if err := s.db.First(&node, nodeID).Error; err != nil {
		return "", err
	}
	if node.Type == nodeTypeTunnelInterface {
		return node.Name, nil
	}

	var edges []model.GraphEdge
	if err := s.db.Where("snapshot_id = ? AND from_node_id = ?", snapshotID, nodeID).Find(&edges).Error; err != nil {
		return "", err
	}
	for _, edge := range edges {
		if result, err := s.walkToTunnel(snapshotID, edge.ToNodeID, visited); err != nil {
			return "", err
		} else if result != "" {
			return result, nil
		}
	}
	return "", nil
}
