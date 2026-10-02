package schema

type TunnelHealthStatusResponse struct {
	Id                               uint   `json:"id"`
	InterfaceName                    string `json:"interface_name"`
	InterfaceType                    string `json:"interface_type,omitempty"`
	Severity                         string `json:"severity"`
	InterfaceRunning                 bool   `json:"interface_running"`
	InterfaceDisabled                bool   `json:"interface_disabled"`
	WorstPeerLastHandshakeAgeSeconds *int   `json:"worst_peer_last_handshake_age_seconds,omitempty"`
	TxRxAsymmetryDetected            bool   `json:"tx_rx_asymmetry_detected"`
	LastPolledAt                     string `json:"last_polled_at"`
}

type TunnelHealthEventResponse struct {
	Id            uint    `json:"id"`
	InterfaceName string  `json:"interface_name"`
	FromStatus    string  `json:"from_status"`
	ToStatus      string  `json:"to_status"`
	Evidence      string  `json:"evidence"`
	DetectedAt    string  `json:"detected_at"`
	NotifiedAt    *string `json:"notified_at,omitempty"`
}

type TunnelActionLogResponse struct {
	Id                 uint   `json:"id"`
	InterfaceName      string `json:"interface_name"`
	Level              string `json:"level"`
	CommandDescription string `json:"command_description"`
	CommandDetail      string `json:"command_detail"`
	Simulated          bool   `json:"simulated"`
	Result             string `json:"result"`
	ExecutedAt         string `json:"executed_at"`
}

// TunnelHealthScoreResponse is one poll tick's computed early-warning
// score for a tunnel -- the panel's own trend-chart data point.
type TunnelHealthScoreResponse struct {
	Score     int    `json:"score"`
	SampledAt string `json:"sampled_at"`
}

// TunnelIncidentDiagnosisResponse is one confirmed_down incident's
// root-cause guess (see TunnelHealthService.diagnoseIncidentOnce).
type TunnelIncidentDiagnosisResponse struct {
	Id            uint   `json:"id"`
	InterfaceName string `json:"interface_name"`
	Cause         string `json:"cause"`
	Evidence      string `json:"evidence"`
	DiagnosedAt   string `json:"diagnosed_at"`
}

// TunnelBackupProbeResultResponse is one nightly active-probe result for
// a tunnel's own configured Level 2 backup gateway.
type TunnelBackupProbeResultResponse struct {
	Id              uint   `json:"id"`
	InterfaceName   string `json:"interface_name"`
	BackupGatewayIP string `json:"backup_gateway_ip"`
	Reachable       bool   `json:"reachable"`
	Evidence        string `json:"evidence"`
	ProbedAt        string `json:"probed_at"`
}

type TunnelAiSettingsResponse struct {
	DryRun        bool `json:"dry_run"`
	EmergencyStop bool `json:"emergency_stop"`
}

type UpdateTunnelAiSettingsRequest struct {
	DryRun        *bool `json:"dry_run,omitempty"`
	EmergencyStop *bool `json:"emergency_stop,omitempty"`
}

type TunnelPolicyDetectionResponse struct {
	PingFailThreshold          int     `json:"ping_fail_threshold"`
	TxRxAsymmetryWindowMinutes int     `json:"tx_rx_asymmetry_window_minutes"`
	TxRxAsymmetryRatio         float64 `json:"tx_rx_asymmetry_ratio"`
}

type TunnelPolicyLevel1Response struct {
	Action      string `json:"action"`
	WaitSeconds int    `json:"wait_seconds"`
}

type TunnelPolicyLevel2Response struct {
	BackupTarget string `json:"backup_target"`
	// BackupGatewayIP is the backup tunnel's own real next-hop IP,
	// entered by hand by the admin -- see TunnelPolicyConfig.Level2's own
	// doc comment on why this can't be derived automatically. Empty
	// means Level 2 stays simulation-only even with BackupTarget set.
	BackupGatewayIP string `json:"backup_gateway_ip"`
}

type TunnelPolicyLevel3Response struct {
	Enabled       bool   `json:"enabled"`
	Type          string `json:"type"`
	RemoteAddress string `json:"remote_address"`
	LocalAddress  string `json:"local_address"`
	Masquerade    bool   `json:"masquerade"`
	// GatewayIP and NewInterfaceName are required for Level 3 to actually
	// create a new tunnel + route -- see TunnelPolicyConfig.Level3's own
	// doc comment.
	GatewayIP        string `json:"gateway_ip"`
	NewInterfaceName string `json:"new_interface_name"`
}

type TunnelPolicyFallbackResponse struct {
	StableDurationSeconds int `json:"stable_duration_seconds"`
}

type TunnelPolicyAntiFlappingResponse struct {
	MaxFlaps      int `json:"max_flaps"`
	WindowMinutes int `json:"window_minutes"`
	CooldownHours int `json:"cooldown_hours"`
}

type TunnelPolicyResponse struct {
	Id            uint                             `json:"id"`
	InterfaceName string                           `json:"interface_name"`
	Detection     TunnelPolicyDetectionResponse    `json:"detection"`
	Level1        TunnelPolicyLevel1Response       `json:"level1"`
	Level2        TunnelPolicyLevel2Response       `json:"level2"`
	Level3        TunnelPolicyLevel3Response       `json:"level3"`
	Fallback      TunnelPolicyFallbackResponse     `json:"fallback"`
	AntiFlapping  TunnelPolicyAntiFlappingResponse `json:"anti_flapping"`
}

type UpdateTunnelPolicyRequest struct {
	Detection    TunnelPolicyDetectionResponse    `json:"detection" validate:"required"`
	Level1       TunnelPolicyLevel1Response       `json:"level1" validate:"required"`
	Level2       TunnelPolicyLevel2Response       `json:"level2"`
	Level3       TunnelPolicyLevel3Response       `json:"level3"`
	Fallback     TunnelPolicyFallbackResponse     `json:"fallback" validate:"required"`
	AntiFlapping TunnelPolicyAntiFlappingResponse `json:"anti_flapping" validate:"required"`
}

type ManagementRedlineEntryResponse struct {
	Id      uint    `json:"id"`
	Kind    string  `json:"kind"`
	Value   string  `json:"value"`
	Comment *string `json:"comment,omitempty"`
}

type CreateRedlineEntryRequest struct {
	Kind    string  `json:"kind" validate:"required,oneof=interface ip port"`
	Value   string  `json:"value" validate:"required"`
	Comment *string `json:"comment,omitempty"`
}

type GraphSnapshotResponse struct {
	Id      uint   `json:"id"`
	TakenAt string `json:"taken_at"`
}

type GraphNodeResponse struct {
	Id            uint   `json:"id"`
	Type          string `json:"type"`
	MikrotikRefID string `json:"mikrotik_ref_id"`
	Name          string `json:"name"`
	Properties    string `json:"properties"`
}

type GraphEdgeResponse struct {
	Id         uint   `json:"id"`
	FromNodeID uint   `json:"from_node_id"`
	ToNodeID   uint   `json:"to_node_id"`
	Relation   string `json:"relation"`
}

type GraphResponse struct {
	Snapshot GraphSnapshotResponse `json:"snapshot"`
	Nodes    []GraphNodeResponse   `json:"nodes"`
	Edges    []GraphEdgeResponse   `json:"edges"`
}

// NatRuleSummaryResponse is one NAT rule's own identifying fields --
// deliberately NOT the rule's bare chain+"/"+action name, which is
// identical across nearly every DNAT rule on a real router and told an
// admin nothing about which of their actual forwarded services this
// rule represents (a confirmed, reported complaint: 26 rules on one
// tunnel all rendered as the same repeated "dstnat/dst-nat" string).
type NatRuleSummaryResponse struct {
	Comment           string `json:"comment"`
	Port              string `json:"port"`
	Target            string `json:"target"`
	MangleRoutingMark string `json:"mangle_routing_mark"`
	RoutingTable      string `json:"routing_table"`
}

// GraphTunnelResponse is one discovered infrastructure tunnel, already
// resolved to its own chain of NAT rules that reach it -- the server-side
// "map-making" the admin explicitly asked for (grouping by protocol and
// location themselves, not a flat node/edge list the UI has to make
// sense of).
type GraphTunnelResponse struct {
	InterfaceName string                   `json:"interface_name"`
	InterfaceType string                   `json:"interface_type"`
	NatRules      []NatRuleSummaryResponse `json:"nat_rules"`
}

// GraphLocationGroupResponse is one location's tunnels -- Location is
// the admin-facing label from ApplicationResourceLocation when a
// matching resource is registered there, or the raw interface/panel
// name as a fallback when it isn't (so a tunnel never silently
// disappears just because nobody has labeled its location yet).
type GraphLocationGroupResponse struct {
	Location string                `json:"location"`
	Tunnels  []GraphTunnelResponse `json:"tunnels"`
}

// GraphProtocolGroupResponse is one protocol's locations -- Protocol is
// one of "wireguard" | "gre" | "ipip" | "eoip" (infrastructure tunnel
// types) or "user_manager" | "v2ray" (service-resource types, grouped
// the same way for a single consistent map).
type GraphProtocolGroupResponse struct {
	Protocol  string                       `json:"protocol"`
	Locations []GraphLocationGroupResponse `json:"locations"`
}

// TunnelMapResponse is the admin's own explicit request: "خودش باید
// نقشه‌سازی کنه... نه اینکه بیاد صرفا یک لیست ساده بسازه" (it should
// build the map itself, not just produce a flat list) -- server-computed
// grouping by protocol, then location, then tunnel, so the graph
// inspector page renders a real hierarchy directly rather than a flat
// node/edge table the admin has to mentally reassemble.
type TunnelMapResponse struct {
	SnapshotTakenAt string                       `json:"snapshot_taken_at"`
	Protocols       []GraphProtocolGroupResponse `json:"protocols"`
}

// UserManagerProtocolHealthStatusResponse is spec section ب-7's own
// alert-only health row for one of L2TP/PPTP/SSTP/OpenVPN.
type UserManagerProtocolHealthStatusResponse struct {
	Protocol      string `json:"protocol"`
	Healthy       bool   `json:"healthy"`
	RouterEnabled bool   `json:"router_enabled"`
	PortReachable bool   `json:"port_reachable"`
	LastCheckedAt string `json:"last_checked_at"`
}

type UserManagerProtocolHealthEventResponse struct {
	Id         uint    `json:"id"`
	Protocol   string  `json:"protocol"`
	FromStatus string  `json:"from_status"`
	ToStatus   string  `json:"to_status"`
	Evidence   string  `json:"evidence"`
	DetectedAt string  `json:"detected_at"`
	NotifiedAt *string `json:"notified_at,omitempty"`
}
