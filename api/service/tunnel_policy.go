package service

import (
	"encoding/json"
	"errors"

	"gorm.io/gorm"

	"github.com/maahdima/mwp/api/dataservice/model"
)

// TunnelPolicyConfig is the parsed form of TunnelPolicy.PolicyJSON --
// field names/shape match the spec's own JSON example (section ب-7)
// verbatim, so an admin's exported/hand-edited policy JSON round-trips
// without translation.
type TunnelPolicyConfig struct {
	// Enabled (confirmed, reported feature request): lets the admin
	// exclude one specific tunnel from this entire system -- detection,
	// alerting, and remediation alike -- for cases where the admin
	// doesn't want it interfering with that tunnel at all. Defaults to
	// true (see DefaultTunnelPolicyConfig) so every existing tunnel's
	// behavior is unchanged unless the admin explicitly opts it out.
	Enabled bool `json:"enabled"`

	Detection struct {
		PingFailThreshold          int     `json:"ping_fail_threshold"`
		TxRxAsymmetryWindowMinutes int     `json:"tx_rx_asymmetry_window_minutes"`
		TxRxAsymmetryRatio         float64 `json:"tx_rx_asymmetry_ratio"`
	} `json:"detection"`
	Level1 struct {
		Action      string `json:"action"` // "toggle_interface"
		WaitSeconds int    `json:"wait_seconds"`
	} `json:"level1"`
	Level2 struct {
		BackupTarget string `json:"backup_target"` // another tunnel's Name -- empty means no L2 configured
		// BackupGatewayIP is the backup tunnel's own real next-hop IP, as
		// the admin's tunnel sees it in its own routing table (e.g.
		// "100.100.77.5") -- NOT the backup interface's name. A confirmed,
		// investigated production fact: on this fleet's real routers,
		// routes reach a tunnel through a next-hop IP that RouterOS then
		// resolves to the actual outgoing interface via ImmediateGw (see
		// mikrotik.RouteEntry.ImmediateGw's own doc comment) -- a route's
		// gateway field is NEVER just the bare interface name. This means
		// there is no way to derive "the right gateway IP for the backup
		// tunnel" purely from what's already discovered (the backup
		// tunnel might not be carrying any traffic of its own yet, so it
		// may have no route to read a gateway IP from at all) -- the
		// admin's own explicit choice, confirmed directly, was to enter
		// this value by hand per tunnel rather than have the engine guess
		// at it. Required for attemptLevel2 to actually move traffic;
		// empty means Level 2 stays simulation-only for this tunnel even
		// with BackupTarget set, exactly like an empty BackupTarget
		// already does.
		BackupGatewayIP string `json:"backup_gateway_ip"`
	} `json:"level2"`
	Level3 struct {
		Enabled       bool   `json:"enabled"`
		Type          string `json:"type"` // "gre" | "ipip" | "eoip" -- WireGuard is NOT supported here, see attemptLevel3's own doc comment
		RemoteAddress string `json:"remote_address"`
		LocalAddress  string `json:"local_address"` // "auto" or an explicit IP
		Masquerade    bool   `json:"masquerade"`
		// GatewayIP is the NEW tunnel's own next-hop IP, once created --
		// mirrors Level2.BackupGatewayIP's exact reasoning (a route's
		// gateway on this fleet is always a next-hop IP, never a bare
		// interface name, confirmed against real production routers).
		// Required for attemptLevel3 to actually create a route pointing
		// at the new tunnel; without it, the tunnel interface itself is
		// still created (if Enabled), but no traffic is routed onto it.
		GatewayIP string `json:"gateway_ip"`
		// NewInterfaceName is what the created tunnel interface is named
		// on the router -- kept distinct from the down interface's own
		// name so both can coexist during/after the incident for manual
		// review, rather than silently reusing (or colliding with) an
		// existing name.
		NewInterfaceName string `json:"new_interface_name"`
	} `json:"level3"`
	Fallback struct {
		StableDurationSeconds int `json:"stable_duration_seconds"`
	} `json:"fallback"`
	AntiFlapping struct {
		MaxFlaps      int `json:"max_flaps"`
		WindowMinutes int `json:"window_minutes"`
		CooldownHours int `json:"cooldown_hours"`
	} `json:"anti_flapping"`
}

// DefaultTunnelPolicyConfig mirrors the spec's own recommended defaults
// (already used as this session's detection thresholds in
// tunnel_health.go's handshakeStaleThreshold/txRxAsymmetryWindow/
// txRxAsymmetryRxRatio constants) -- used to seed a policy row the first
// time a tunnel is discovered, so every tunnel always has a usable policy
// without the admin having to configure one before the engine can reason
// about it at all.
func DefaultTunnelPolicyConfig() TunnelPolicyConfig {
	var c TunnelPolicyConfig
	c.Enabled = true
	c.Detection.PingFailThreshold = 3
	c.Detection.TxRxAsymmetryWindowMinutes = 3
	c.Detection.TxRxAsymmetryRatio = 0.05
	c.Level1.Action = "toggle_interface"
	c.Level1.WaitSeconds = 30
	c.Level3.Enabled = false
	c.Fallback.StableDurationSeconds = 180
	c.AntiFlapping.MaxFlaps = 3
	c.AntiFlapping.WindowMinutes = 60
	c.AntiFlapping.CooldownHours = 24
	return c
}

// TunnelPolicyService manages the "سیاست هر تانل" subpage's data --
// separate from TunnelHealthService itself so the policy CRUD surface
// (admin-facing forms) stays independent of the polling/decision engine
// that reads it.
type TunnelPolicyService struct {
	db *gorm.DB
}

func NewTunnelPolicyService(db *gorm.DB) *TunnelPolicyService {
	return &TunnelPolicyService{db: db}
}

// GetOrCreateDefault returns interfaceName's policy, creating one seeded
// with DefaultTunnelPolicyConfig if none exists yet.
func (s *TunnelPolicyService) GetOrCreateDefault(interfaceName string) (*model.TunnelPolicy, error) {
	var policy model.TunnelPolicy
	err := s.db.Where("interface_name = ?", interfaceName).First(&policy).Error
	if err == nil {
		return &policy, nil
	}
	if !errors.Is(err, gorm.ErrRecordNotFound) {
		return nil, err
	}

	raw, err := json.Marshal(DefaultTunnelPolicyConfig())
	if err != nil {
		return nil, err
	}
	policy = model.TunnelPolicy{InterfaceName: interfaceName, PolicyJSON: string(raw)}
	if err := s.db.Create(&policy).Error; err != nil {
		return nil, err
	}
	return &policy, nil
}

// Parse decodes a TunnelPolicy's PolicyJSON -- callers (the decision
// engine) always go through this rather than unmarshalling PolicyJSON
// themselves, so a malformed/partial JSON degrades to
// DefaultTunnelPolicyConfig's zero-risk values (Level3.Enabled=false,
// etc.) instead of failing the whole poll cycle for one bad policy row.
func (s *TunnelPolicyService) Parse(policy *model.TunnelPolicy) TunnelPolicyConfig {
	config := DefaultTunnelPolicyConfig()
	if policy == nil || policy.PolicyJSON == "" {
		return config
	}
	_ = json.Unmarshal([]byte(policy.PolicyJSON), &config)
	return config
}

func (s *TunnelPolicyService) List() ([]model.TunnelPolicy, error) {
	var policies []model.TunnelPolicy
	if err := s.db.Order("interface_name asc").Find(&policies).Error; err != nil {
		return nil, err
	}
	return policies, nil
}

func (s *TunnelPolicyService) Update(id uint, config TunnelPolicyConfig) (*model.TunnelPolicy, error) {
	raw, err := json.Marshal(config)
	if err != nil {
		return nil, err
	}
	var policy model.TunnelPolicy
	if err := s.db.First(&policy, id).Error; err != nil {
		return nil, err
	}
	policy.PolicyJSON = string(raw)
	if err := s.db.Save(&policy).Error; err != nil {
		return nil, err
	}
	return &policy, nil
}
