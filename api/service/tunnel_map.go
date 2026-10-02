package service

import (
	"encoding/json"
	"sort"

	"github.com/maahdima/mwp/api/dataservice/model"
)

// TunnelMapNode/TunnelMapGroup mirror the HTTP layer's own
// GraphTunnelResponse/GraphLocationGroupResponse/GraphProtocolGroupResponse
// shapes (see http/schema/tunnel_health.go) -- kept as plain service-layer
// structs here so this file has no dependency on the http package,
// matching this codebase's own layering convention (service never
// imports http).
// NatRuleSummary is one NAT rule's own IDENTIFYING fields -- not its
// graph-node "name", which is only ever chain+"/"+action (e.g.
// "dstnat/dst-nat") and is IDENTICAL across nearly every DNAT rule on a
// real router (confirmed: 26 rules on one production tunnel all shared
// this exact same name). Displaying that repeated string 26 times told
// an admin nothing about which of their actual forwarded services
// (which port, which comment, which target) maps to this tunnel -- see
// BuildTunnelMap's own doc comment for the reported complaint this
// fixes. Comment is preferred as the primary label when the admin set
// one (it is their own name for the rule); Port/Target always render
// alongside it since two rules can share a comment.
type NatRuleSummary struct {
	Comment string // admin's own label, "" when unset
	Port    string // dst-port, "" when the rule matches all ports
	Target  string // to-addresses[:to-ports], the real forwarded destination
	// MangleRoutingMark/RoutingTable are the SAME admin-facing request as
	// the NAT/port/target fields above: "باید منگل‌ها اینا هم باشن و
	// روتینگ که مسیر چه پنل‌هایی داخل پنل به کدوم تانل‌ها ختم میشه" (the
	// mangle rules and routing need to be there too, showing which
	// panels route to which tunnels) -- an admin looking at a forwarded
	// service needs to see WHY it ends up on this tunnel (which mangle
	// rule tags it with which routing-mark, which routing table that
	// mark selects), not just the end result. Empty when this NAT rule's
	// own chain to the tunnel could not be resolved (e.g. its target
	// address doesn't fall under any mangle rule's own address range --
	// see mangleRuleContainsIP's own doc comment), matching this
	// section's existing "show what's real, nothing fabricated"
	// convention.
	MangleRoutingMark string
	RoutingTable      string
}

type TunnelMapTunnel struct {
	InterfaceName string
	InterfaceType string
	NatRules      []NatRuleSummary
}

type TunnelMapLocationGroup struct {
	Location string
	Tunnels  []TunnelMapTunnel
}

type TunnelMapProtocolGroup struct {
	Protocol  string
	Locations []TunnelMapLocationGroup
}

// BuildTunnelMap is the admin's own explicit request answered directly:
// "خودش باید نقشه‌سازی کنه بر اساس هر پروتکل و لوکیشن و تانل... نه اینکه
// بیاد صرفا یک لیست ساده بسازه" (it should build the map itself, grouped
// by protocol/location/tunnel -- not just produce a flat list). Reads the
// most recent discovery snapshot's tunnel_interface nodes, groups them by
// their own real RouterOS interface type (gre/ipip/eoip/wireguard --
// discovered dynamically, see TunnelGraphService.DiscoveredTunnelInterfaces'
// own doc comment for why this is never a hardcoded type list), and for
// each tunnel resolves every NAT rule whose own dependency chain reaches
// it (via the same recursive walkToTunnel this service already uses for
// a single lookup, run here once per discovered NAT rule against this
// snapshot).
//
// Infrastructure tunnels (this function's own subject) have no
// admin-assigned "location" the way a customer-facing resource does --
// see this function's own grouping choice: it groups by the tunnel's
// OWN remote identity (its interface name) rather than inventing a fake
// location label, since the admin has not asked for per-tunnel location
// naming and none exists in RouterOS's own config to read. Grouping by
// protocol type alone (one location bucket per protocol, named after the
// protocol itself when no finer distinction exists) keeps the hierarchy
// real rather than fabricated.
func (s *TunnelGraphService) BuildTunnelMap(snapshotID uint) ([]TunnelMapProtocolGroup, error) {
	var tunnelNodes []model.GraphNode
	if err := s.db.Where("snapshot_id = ? AND type = ?", snapshotID, nodeTypeTunnelInterface).Find(&tunnelNodes).Error; err != nil {
		return nil, err
	}
	if len(tunnelNodes) == 0 {
		return nil, nil
	}

	var natNodes []model.GraphNode
	if err := s.db.Where("snapshot_id = ? AND type = ?", snapshotID, nodeTypeNatRule).Find(&natNodes).Error; err != nil {
		return nil, err
	}

	// Load every node/edge for this snapshot ONCE and walk in memory --
	// a real router can have dozens to hundreds of NAT rules, and this
	// endpoint is polled every 60s by the graph tab's own UI refresh, so
	// calling ResolveTunnelForNatRule (2+ DB queries per recursion step)
	// once per NAT rule would mean hundreds of SQLite round-trips per
	// minute purely for a UI refresh, on a router already known to be
	// under load. allNodes/edgesByFromNode are built once here instead.
	allNodes, allEdges, err := s.NodesAndEdges(snapshotID)
	if err != nil {
		return nil, err
	}
	nodeByID := make(map[uint]model.GraphNode, len(allNodes))
	for _, n := range allNodes {
		nodeByID[n.ID] = n
	}
	edgesByFromNode := make(map[uint][]model.GraphEdge, len(allEdges))
	for _, e := range allEdges {
		edgesByFromNode[e.FromNodeID] = append(edgesByFromNode[e.FromNodeID], e)
	}

	// natRulesByTunnel: for every NAT rule, walk its own dependency chain
	// (identical logic to walkToTunnel, but against the in-memory maps
	// above instead of re-querying the DB at every step) and record which
	// tunnel it reaches, if any -- summarized via natRuleSummary rather
	// than the node's own bare Name (see NatRuleSummary's own doc comment
	// for why: chain+"/"+action is identical across nearly every DNAT
	// rule on a real router, so the old code's flat name list rendered as
	// the same string repeated 26 times with zero distinguishing
	// information). walkToTunnelWithPath additionally returns the mangle
	// rule and routing table encountered along the way (the admin's own
	// explicit follow-up request -- see NatRuleSummary's own doc comment).
	natRulesByTunnel := make(map[string][]NatRuleSummary)
	for _, natNode := range natNodes {
		tunnelName, path := walkToTunnelWithPath(natNode.ID, nodeByID, edgesByFromNode, make(map[uint]bool))
		if tunnelName == "" {
			continue
		}
		summary := natRuleSummary(natNode)
		summary.MangleRoutingMark = path.mangleRoutingMark
		summary.RoutingTable = path.routingTable
		natRulesByTunnel[tunnelName] = append(natRulesByTunnel[tunnelName], summary)
	}

	tunnelsByType := make(map[string][]TunnelMapTunnel)
	for _, node := range tunnelNodes {
		ifaceType := tunnelInterfaceType(node)
		if ifaceType == "" {
			ifaceType = "نامشخص"
		}
		tunnelsByType[ifaceType] = append(tunnelsByType[ifaceType], TunnelMapTunnel{
			InterfaceName: node.Name,
			InterfaceType: ifaceType,
			NatRules:      dedupeNatRules(natRulesByTunnel[node.Name]),
		})
	}

	protocolTypes := make([]string, 0, len(tunnelsByType))
	for t := range tunnelsByType {
		protocolTypes = append(protocolTypes, t)
	}
	sort.Strings(protocolTypes)

	groups := make([]TunnelMapProtocolGroup, 0, len(protocolTypes))
	for _, protocolType := range protocolTypes {
		tunnels := tunnelsByType[protocolType]
		sort.Slice(tunnels, func(i, j int) bool { return tunnels[i].InterfaceName < tunnels[j].InterfaceName })
		groups = append(groups, TunnelMapProtocolGroup{
			Protocol: protocolType,
			Locations: []TunnelMapLocationGroup{
				{Location: protocolTypeLabelFa(protocolType), Tunnels: tunnels},
			},
		})
	}
	return groups, nil
}

// natRuleSummary extracts a NAT rule graph node's own real identifying
// fields (stashed in PropertiesJSON by createNodesForRules) into the
// admin-facing NatRuleSummary -- see that type's own doc comment for why
// the node's bare Name (chain+"/"+action) is never used for display.
func natRuleSummary(natNode model.GraphNode) NatRuleSummary {
	var props struct {
		DstPort     *string `json:"dst_port"`
		ToAddresses *string `json:"to_addresses"`
		ToPorts     *string `json:"to_ports"`
		Comment     *string `json:"comment"`
	}
	_ = json.Unmarshal([]byte(natNode.PropertiesJSON), &props)

	summary := NatRuleSummary{}
	if props.Comment != nil {
		summary.Comment = *props.Comment
	}
	if props.DstPort != nil {
		summary.Port = *props.DstPort
	}
	if props.ToAddresses != nil {
		summary.Target = *props.ToAddresses
		if props.ToPorts != nil && *props.ToPorts != "" {
			summary.Target += ":" + *props.ToPorts
		}
	}
	return summary
}

// dedupeNatRules collapses NatRuleSummary entries that are IDENTICAL
// across every displayed field -- RouterOS commonly has several NAT
// rules (e.g. one per protocol/port variant) that share the exact same
// comment/port/target from this graph's own point of view; showing each
// one separately would just repeat the same line, the same problem this
// whole rework exists to fix.
func dedupeNatRules(rules []NatRuleSummary) []NatRuleSummary {
	seen := make(map[NatRuleSummary]bool, len(rules))
	out := make([]NatRuleSummary, 0, len(rules))
	for _, r := range rules {
		if seen[r] {
			continue
		}
		seen[r] = true
		out = append(out, r)
	}
	return out
}

// walkToTunnelInMemory mirrors TunnelGraphService.walkToTunnel's exact
// recursive logic (same cycle guard, same "stop at the first
// tunnel_interface node reached" rule) but against pre-loaded maps
// instead of issuing a DB query at every step -- see BuildTunnelMap's
// own doc comment for why this matters when walking many NAT rules in
// one request.
func walkToTunnelInMemory(nodeID uint, nodeByID map[uint]model.GraphNode, edgesByFromNode map[uint][]model.GraphEdge, visited map[uint]bool) string {
	if visited[nodeID] {
		return ""
	}
	visited[nodeID] = true

	node, ok := nodeByID[nodeID]
	if !ok {
		return ""
	}
	if node.Type == nodeTypeTunnelInterface {
		return node.Name
	}

	for _, edge := range edgesByFromNode[nodeID] {
		if result := walkToTunnelInMemory(edge.ToNodeID, nodeByID, edgesByFromNode, visited); result != "" {
			return result
		}
	}
	return ""
}

// pathToTunnel is the mangle/routing-table nodes walkToTunnelWithPath
// passed through on its way to a tunnel -- see NatRuleSummary's own doc
// comment for why the admin wants these surfaced, not just the end
// result.
type pathToTunnel struct {
	mangleRoutingMark string
	routingTable      string
}

// walkToTunnelWithPath mirrors walkToTunnelInMemory's exact recursive
// logic (same cycle guard, same "stop at the first tunnel_interface node
// reached" rule) but additionally records the mangle rule's own
// new-routing-mark and the routing table name encountered along the
// path -- the admin's own explicit follow-up request to see WHICH
// mangle rule and routing table carry a given forwarded service to its
// tunnel, not just the final tunnel name (see NatRuleSummary's own doc
// comment). mangleRoutingMark/routingTable are threaded through the
// recursion by value (not accumulated in a shared slice) since a NAT
// rule's own chain only ever passes through one mangle rule and one
// routing table before reaching a route/tunnel (see wireMangleDownstream's
// own doc comment for that shared chain shape).
func walkToTunnelWithPath(nodeID uint, nodeByID map[uint]model.GraphNode, edgesByFromNode map[uint][]model.GraphEdge, visited map[uint]bool) (string, pathToTunnel) {
	return walkToTunnelWithPathAcc(nodeID, nodeByID, edgesByFromNode, visited, pathToTunnel{})
}

func walkToTunnelWithPathAcc(nodeID uint, nodeByID map[uint]model.GraphNode, edgesByFromNode map[uint][]model.GraphEdge, visited map[uint]bool, acc pathToTunnel) (string, pathToTunnel) {
	if visited[nodeID] {
		return "", acc
	}
	visited[nodeID] = true

	node, ok := nodeByID[nodeID]
	if !ok {
		return "", acc
	}
	if node.Type == nodeTypeTunnelInterface {
		return node.Name, acc
	}
	switch node.Type {
	case nodeTypeMangleRule:
		var props struct {
			NewRoutingMark *string `json:"new_routing_mark"`
		}
		_ = json.Unmarshal([]byte(node.PropertiesJSON), &props)
		if props.NewRoutingMark != nil {
			acc.mangleRoutingMark = *props.NewRoutingMark
		}
	case nodeTypeRoutingTable:
		acc.routingTable = node.Name
	}

	for _, edge := range edgesByFromNode[nodeID] {
		if result, resultAcc := walkToTunnelWithPathAcc(edge.ToNodeID, nodeByID, edgesByFromNode, visited, acc); result != "" {
			return result, resultAcc
		}
	}
	return "", acc
}

// protocolTypeLabelFa renders a tunnel interface type as a Persian label
// for the "location" bucket header -- infrastructure tunnels of the same
// type are grouped together since RouterOS gives no finer per-tunnel
// location concept to read (see BuildTunnelMap's own doc comment).
func protocolTypeLabelFa(interfaceType string) string {
	switch interfaceType {
	case "gre":
		return "تانل‌های GRE"
	case "ipip":
		return "تانل‌های IPIP"
	case "eoip":
		return "تانل‌های EoIP"
	case "wireguard":
		return "تانل‌های WireGuard (زیرساختی)"
	default:
		return "نامشخص"
	}
}
