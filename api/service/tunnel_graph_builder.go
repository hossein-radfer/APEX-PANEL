package service

import (
	"encoding/json"
	"net"
	"strings"

	"go.uber.org/zap"

	"github.com/maahdima/mwp/api/adaptor/mikrotik"
	"github.com/maahdima/mwp/api/dataservice/model"
)

// This file implements spec section ب-3's dependency-graph construction
// algorithm as ONE shared recursive walk, per the spec's own explicit
// instruction: "این دو الگوریتم را به‌صورت یک تابع بازگشتی عمومی بنویس"
// (write these two algorithms as one shared recursive function) --
// "example 1" (NAT -> tunnel) and "example 2" (User Manager Pool ->
// tunnel) both enter at a different starting node type, but from the
// mangle step onward they walk the IDENTICAL chain: mangle rule ->
// routing-table (via new-routing-mark) -> route (filtered to that
// table) -> gateway -> (tunnel interface, OR another IP to resolve
// recursively). buildGraph below builds every node first, then wires
// every edge type using that one shared "resolve routing-mark's
// downstream chain" step (wireMangleDownstream), called once per entry
// point (NAT rules AND, when User Manager pools are wired in during a
// later pass, pool-derived addresses) instead of being duplicated.

// buildGraph is the single entry point called once per Discover() tick.
// It creates every GraphNode for this snapshot's resources, then wires
// every GraphEdge by walking each documented relation in the spec's own
// order (ب-2's relation vocabulary).
func (s *TunnelGraphService) buildGraph(snapshotID uint, data *discoveryData) error {
	// --- Step 1: create one GraphNode per discovered entity. ---
	//
	// tunnelGatewayNames is a REAL infrastructure tunnel's own defining
	// property, per the admin's explicit correction: a name/type alone
	// (WireGuard vs GRE vs IPIP) proves NOTHING about whether an
	// interface is inter-server infrastructure or a customer-facing VPN
	// endpoint -- both are configured as ordinary RouterOS interfaces.
	// The only reliable signal is that something in the real routing
	// table actually treats this interface as a GATEWAY (i.e. traffic is
	// routed THROUGH it to reach somewhere else) -- a customer's
	// WireGuard peer interface never appears as a route's own gateway,
	// only as a route's own destination network. This is computed fresh
	// from data.Routes every tick, independent of interface type/name,
	// so a GRE tunnel today and some other tunnel type tomorrow are
	// detected identically without any hardcoded type list.
	tunnelGatewayNames := data.tunnelGatewayNames()
	interfaceTypeByName := data.interfaceTypeByName()

	natNodes := s.createNodesForRules(snapshotID, nodeTypeNatRule, data.NatRules)
	mangleNodes := s.createNodesForRules(snapshotID, nodeTypeMangleRule, data.MangleRules)
	addressNodeByInterface := s.createAddressNodes(snapshotID, data.Addresses)
	bridgeNodeByName := s.createBridgeNodes(snapshotID, data.Bridges)
	routingTableNodeByName := s.createRoutingTableNodes(snapshotID, data.RoutingTables)
	routeNodesByTable := s.createRouteNodes(snapshotID, data.Routes)
	tunnelNodeByName := s.createTunnelInterfaceNodes(snapshotID, tunnelGatewayNames, interfaceTypeByName)
	poolNodeByName := s.createPoolNodes(snapshotID, data.Pools)

	// --- Step 2: member_of_bridge (interface -> bridge, spec ب-1's own
	// /interface/bridge/port table). ---
	interfaceToBridge := make(map[string]string, len(data.BridgePorts))
	for _, port := range data.BridgePorts {
		interfaceToBridge[port.Interface] = port.Bridge
	}
	for ifaceName, addrNode := range addressNodeByInterface {
		if bridgeName, onBridge := interfaceToBridge[ifaceName]; onBridge {
			if bridgeNode, ok := bridgeNodeByName[bridgeName]; ok {
				s.createEdge(snapshotID, addrNode.ID, bridgeNode.ID, relationMemberOfBridge)
			}
		}
	}

	// --- Step 3: routed_via_table (mangle rule -> routing table, via
	// new-routing-mark) + table_routes_to_gateway (routing table -> every
	// route row filtered to that table) + gateway_is_tunnel (route ->
	// tunnel interface node, when the gateway names one directly). This
	// is the ONE shared chain both spec examples converge on from here
	// onward (see this file's own top doc comment). ---
	for _, mangleNode := range mangleNodes {
		s.wireMangleDownstream(snapshotID, mangleNode, routingTableNodeByName, routeNodesByTable, tunnelNodeByName)
	}

	// --- Step 4: nat_targets (NAT rule -> address it targets, spec ب-3
	// example 1 steps 1-2: dst-address/dst-port -> matching /ip/address's
	// own interface). A NAT rule's OWN mangle linkage (steps 3-4: which
	// mangle rule matches this same bridge/container) is resolved by
	// finding a mangle rule whose src/dst-address NETWORK contains this
	// NAT rule's target IP -- see mangleRuleContainsIP's own doc comment
	// for the confirmed production bug this replaces (an exact string
	// match between a NAT rule's single host IP and a mangle rule's own
	// /24 network never matches, since "172.17.0.2" != "172.17.0.0/24"
	// even though the mangle rule plainly governs that host). ---
	for i, natNode := range natNodes {
		rule := data.NatRules[i]
		// The real DNAT target lives in ToAddresses (confirmed against
		// production: e.g. a rule with comment "S-UI PANEL" has
		// to-addresses="172.17.0.2" and no dst-address at all -- the
		// rule's own dst-address, when present, is typically the
		// router's OWN public IP the client connected to, not the
		// container/tunnel-side address this graph cares about).
		// DstAddress is still tried as a fallback for a srcnat-style
		// rule that has no ToAddresses.
		target := rule.ToAddresses
		if target == nil {
			target = rule.DstAddress
		}
		if target == nil || *target == "" {
			continue
		}

		// nat_targets: only created when the target resolves to a REAL
		// locally-owned address (i.e. this NAT rule targets the router's
		// own interface). A dst-nat rule whose target is a
		// virtual/private address behind the router (the common
		// port-forward case, confirmed against a real CHR test rule)
		// legitimately has no such address to link to -- that's not a
		// missing edge, it's simply not the shape this relation
		// describes.
		if addrNode, ok := s.findAddressNodeForIP(*target, data.Addresses, addressNodeByInterface); ok {
			s.createEdge(snapshotID, natNode.ID, addrNode.ID, relationNatTargets)
		}
		// mangled_by: independent of whether the address step above
		// resolved -- a mangle rule whose own src/dst-address network
		// contains this NAT rule's target IP is itself sufficient
		// evidence they act on the same traffic (spec ب-3 step 4's own
		// "search mangle for a rule matching the same bridge/container").
		// This must NOT be gated on nat_targets succeeding, or the most
		// common real case (a DNAT rule whose target is a private
		// address with no matching local /ip/address entry -- confirmed
		// against a real CHR test rule) would silently never link to its
		// own mangle rule at all.
		for j, mangleRule := range data.MangleRules {
			if mangleRuleContainsIP(mangleRule, *target) {
				s.createEdge(snapshotID, natNode.ID, mangleNodes[j].ID, relationMangledBy)
			}
		}
	}

	// --- Step 5: pool_used_by_profile is deferred -- it requires User
	// Manager profile/limitation data (spec ب-3 example 2 step 2), which
	// this discovery pass does not yet read (User Manager's own
	// version-sensitive API is a separate, not-yet-scoped piece of this
	// system -- see this service's own top-level doc comment on why
	// remediation for User Manager protocols stays manual for now). Pool
	// nodes are still created above (poolNodeByName) so they're visible
	// on the graph inspector even before this edge type is wired. ---
	_ = poolNodeByName

	return nil
}

// wireMangleDownstream implements the SHARED tail of both spec examples:
// mangle rule's own new-routing-mark -> matching routing_table node ->
// every route row whose routing-table matches -> that route's own
// gateway, resolved either directly to a tunnel interface node
// (gateway_is_tunnel) or, if the gateway is itself an IP, left for a
// caller to resolve recursively via ResolveTunnelForNatRule's own
// walkToTunnel (spec ب-3 step 7's "if the gateway is an IP, look that IP
// up in the route table again, recursively").
func (s *TunnelGraphService) wireMangleDownstream(
	snapshotID uint,
	mangleNode *model.GraphNode,
	routingTableNodeByName map[string]*model.GraphNode,
	routeNodesByTable map[string][]*routeNodeEntry,
	tunnelNodeByName map[string]*model.GraphNode,
) {
	var props struct {
		NewRoutingMark *string `json:"new_routing_mark"`
	}
	_ = json.Unmarshal([]byte(mangleNode.PropertiesJSON), &props)
	if props.NewRoutingMark == nil || *props.NewRoutingMark == "" {
		return
	}

	tableNode, ok := routingTableNodeByName[*props.NewRoutingMark]
	if !ok {
		return
	}
	s.createEdge(snapshotID, mangleNode.ID, tableNode.ID, relationRoutedViaTable)

	for _, routeEntry := range routeNodesByTable[*props.NewRoutingMark] {
		s.createEdge(snapshotID, tableNode.ID, routeEntry.node.ID, relationTableRoutesToGateway)

		if routeEntry.gateway == "" {
			continue
		}
		if tunnelNode, isTunnel := tunnelNodeByName[routeEntry.gateway]; isTunnel {
			s.createEdge(snapshotID, routeEntry.node.ID, tunnelNode.ID, relationGatewayIsTunnel)
		}
		// A gateway that is itself an IP (not a named tunnel interface)
		// is intentionally NOT expanded further here -- ResolveTunnelForNatRule's
		// walkToTunnel already performs that recursive lookup on demand
		// per the spec's own step 7, rather than this build pass eagerly
		// materializing every possible recursive hop as edges up front.
	}
}

type routeNodeEntry struct {
	node    *model.GraphNode
	gateway string
}

func (s *TunnelGraphService) createNode(snapshotID uint, nodeType, refID, name string, properties any) *model.GraphNode {
	propsJSON, err := json.Marshal(properties)
	if err != nil {
		propsJSON = []byte("{}")
	}
	node := model.GraphNode{
		SnapshotID:     snapshotID,
		Type:           nodeType,
		MikrotikRefID:  refID,
		Name:           name,
		PropertiesJSON: string(propsJSON),
	}
	if err := s.db.Create(&node).Error; err != nil {
		s.logger.Error("failed to create graph node", zap.String("type", nodeType), zap.Error(err))
		return &node
	}
	return &node
}

func (s *TunnelGraphService) createEdge(snapshotID, fromNodeID, toNodeID uint, relation string) {
	edge := model.GraphEdge{SnapshotID: snapshotID, FromNodeID: fromNodeID, ToNodeID: toNodeID, Relation: relation}
	if err := s.db.Create(&edge).Error; err != nil {
		s.logger.Error("failed to create graph edge", zap.String("relation", relation), zap.Error(err))
	}
}

func firewallRuleName(chain, action string) string {
	return chain + "/" + action
}

func (s *TunnelGraphService) createNodesForRules(snapshotID uint, nodeType string, rules []mikrotik.FirewallRule) []*model.GraphNode {
	nodes := make([]*model.GraphNode, len(rules))
	for i, rule := range rules {
		nodes[i] = s.createNode(snapshotID, nodeType, rule.ID, firewallRuleName(rule.Chain, rule.Action), struct {
			SrcAddress        *string `json:"src_address"`
			DstAddress        *string `json:"dst_address"`
			DstPort           *string `json:"dst_port"`
			NewRoutingMark    *string `json:"new_routing_mark"`
			NewConnectionMark *string `json:"new_connection_mark"`
			ToAddresses       *string `json:"to_addresses"`
			ToPorts           *string `json:"to_ports"`
			Comment           *string `json:"comment"`
			Disabled          string  `json:"disabled"`
		}{rule.SrcAddress, rule.DstAddress, rule.DstPort, rule.NewRoutingMark, rule.NewConnectionMark, rule.ToAddresses, rule.ToPorts, rule.Comment, rule.Disabled})
	}
	return nodes
}

func (s *TunnelGraphService) createAddressNodes(snapshotID uint, addresses []mikrotik.IPAddress) map[string]*model.GraphNode {
	byInterface := make(map[string]*model.GraphNode, len(addresses))
	for _, addr := range addresses {
		node := s.createNode(snapshotID, nodeTypeAddress, addr.ID, addr.Address, struct {
			Interface string `json:"interface"`
			Network   string `json:"network"`
		}{addr.Interface, addr.Network})
		byInterface[addr.Interface] = node
	}
	return byInterface
}

func (s *TunnelGraphService) createBridgeNodes(snapshotID uint, bridges []mikrotik.BridgeEntry) map[string]*model.GraphNode {
	byName := make(map[string]*model.GraphNode, len(bridges))
	for _, b := range bridges {
		byName[b.Name] = s.createNode(snapshotID, nodeTypeBridge, b.ID, b.Name, struct{}{})
	}
	return byName
}

func (s *TunnelGraphService) createRoutingTableNodes(snapshotID uint, tables []mikrotik.RoutingTableEntry) map[string]*model.GraphNode {
	byName := make(map[string]*model.GraphNode, len(tables)+1)
	for _, t := range tables {
		byName[t.Name] = s.createNode(snapshotID, nodeTypeRoutingTable, t.ID, t.Name, struct {
			Fib string `json:"fib"`
		}{t.Fib})
	}
	// RouterOS's default "main" table is often not returned by
	// /routing/table (it's implicit) -- ensure it always has a node so
	// routes whose routing-table is empty/"main" still have somewhere to
	// attach, matching how RouterOS itself treats an unset routing-table
	// as "main".
	if _, exists := byName["main"]; !exists {
		byName["main"] = s.createNode(snapshotID, nodeTypeRoutingTable, "main", "main", struct{}{})
	}
	return byName
}

func (s *TunnelGraphService) createRouteNodes(snapshotID uint, routes []mikrotik.RouteEntry) map[string][]*routeNodeEntry {
	byTable := make(map[string][]*routeNodeEntry, len(routes))
	for _, r := range routes {
		table := "main"
		if r.RoutingTable != nil && *r.RoutingTable != "" {
			table = *r.RoutingTable
		}
		// routeGatewayInterface resolves via ImmediateGw first (RouterOS's
		// own "<ip>%<interface>" resolution of a recursive/IP gateway) --
		// using the bare Gateway field here misses every tunnel reached via
		// an IP next-hop (e.g. a route to a Google range with
		// gateway="100.100.77.1" only reveals gre-tunnel_DB as its real
		// outgoing interface through ImmediateGw). See
		// discoveryData.tunnelGatewayNames' own doc comment for the
		// confirmed production case this fixes.
		gateway := routeGatewayInterface(r)
		node := s.createNode(snapshotID, nodeTypeRoute, r.ID, r.DstAddress, struct {
			Gateway      string `json:"gateway"`
			RoutingTable string `json:"routing_table"`
		}{gateway, table})
		byTable[table] = append(byTable[table], &routeNodeEntry{node: node, gateway: gateway})
	}
	return byTable
}

// createTunnelInterfaceNodes records each detected infrastructure
// tunnel's own real interface type (wireguard/gre/ipip/eoip) in its
// PropertiesJSON -- TunnelHealthService reads this back to decide which
// detection method applies (a WireGuard-based tunnel gets last-handshake
// staleness checked, a GRE/IPIP/EoIP tunnel does not, since those
// protocols have no handshake concept at all).
func (s *TunnelGraphService) createTunnelInterfaceNodes(snapshotID uint, tunnelNames map[string]bool, ifaceTypeByName map[string]string) map[string]*model.GraphNode {
	byName := make(map[string]*model.GraphNode, len(tunnelNames))
	for name := range tunnelNames {
		byName[name] = s.createNode(snapshotID, nodeTypeTunnelInterface, name, name, struct {
			InterfaceType string `json:"interface_type"`
		}{ifaceTypeByName[name]})
	}
	return byName
}

func (s *TunnelGraphService) createPoolNodes(snapshotID uint, pools []mikrotik.IPPoolEntry) map[string]*model.GraphNode {
	byName := make(map[string]*model.GraphNode, len(pools))
	for _, p := range pools {
		byName[p.Name] = s.createNode(snapshotID, nodeTypePool, p.ID, p.Name, struct {
			Ranges string `json:"ranges"`
		}{p.Ranges})
	}
	return byName
}

// findAddressNodeForIP resolves a NAT rule's dst-address (which may be a
// bare IP, or an IP/CIDR, or even a comma-joined list on some RouterOS
// configs) to the /ip/address entry whose own network it falls inside --
// spec ب-3 step 2: "find that dst-address in /ip/address -> see which
// bridge/interface it belongs to". Exact-address match is tried first
// (the common case for a single-host DNAT target); a real CIDR
// containment check (net.IPNet.Contains, NOT a string-prefix compare --
// "192.168.1.5" textually prefix-matches "192.168.1" but so does the
// unrelated "192.168.10.5", a real bug a naive strings.HasPrefix would
// have) is the fallback for a dst-address that names a whole subnet.
// mangleRuleContainsIP reports whether mangleRule's own src-address or
// dst-address NETWORK contains ip. Confirmed against production data:
// a NAT rule's DNAT target is always a single host IP (e.g.
// "172.17.0.2"), while the mangle rule that actually governs that same
// host's traffic is written against the containing /24 network (e.g.
// "172.17.0.0/24") -- comparing these two strings for exact equality
// (the previous version of this matching step) can never succeed, which
// is why the graph inspector reported "no NAT rule found" for every
// single tunnel despite NAT and mangle rules plainly covering the same
// address space. A leading "!" negates the rule in RouterOS (matches
// traffic OUTSIDE that network) -- such a rule does not positively
// identify this IP's own traffic, so negated addresses are skipped
// rather than treated as a match.
func mangleRuleContainsIP(mangleRule mikrotik.FirewallRule, ip string) bool {
	targetIP := net.ParseIP(strings.SplitN(ip, "/", 2)[0])
	if targetIP == nil {
		return false
	}
	candidates := []*string{mangleRule.SrcAddress, mangleRule.DstAddress}
	for _, candidate := range candidates {
		if candidate == nil || *candidate == "" || strings.HasPrefix(*candidate, "!") {
			continue
		}
		network := *candidate
		if !strings.Contains(network, "/") {
			network += "/32"
		}
		_, ipNet, err := net.ParseCIDR(network)
		if err != nil {
			continue
		}
		if ipNet.Contains(targetIP) {
			return true
		}
	}
	return false
}

func (s *TunnelGraphService) findAddressNodeForIP(dstAddress string, addresses []mikrotik.IPAddress, byInterface map[string]*model.GraphNode) (*model.GraphNode, bool) {
	ip := strings.SplitN(dstAddress, "/", 2)[0]
	for _, addr := range addresses {
		addrIP := strings.SplitN(addr.Address, "/", 2)[0]
		if addrIP == ip {
			node, ok := byInterface[addr.Interface]
			return node, ok
		}
	}

	target := net.ParseIP(ip)
	if target == nil {
		return nil, false
	}
	for _, addr := range addresses {
		if addr.Network == "" {
			continue
		}
		// addr.Network is the bare network address (e.g. "192.168.1.0"),
		// not a CIDR string -- pair it with addr.Address's own prefix
		// length to reconstruct a real *net.IPNet.
		_, ipNet, err := net.ParseCIDR(addr.Network + "/" + cidrSuffix(addr.Address))
		if err != nil || ipNet == nil {
			continue
		}
		if ipNet.Contains(target) {
			node, ok := byInterface[addr.Interface]
			return node, ok
		}
	}
	return nil, false
}

// cidrSuffix extracts the "/NN" prefix length from a RouterOS address
// string like "10.10.10.1/24", defaulting to "32" (a single host) when
// the address carries no explicit prefix.
func cidrSuffix(address string) string {
	parts := strings.SplitN(address, "/", 2)
	if len(parts) == 2 && parts[1] != "" {
		return parts[1]
	}
	return "32"
}
