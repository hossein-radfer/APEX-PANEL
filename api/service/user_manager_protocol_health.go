package service

import (
	"context"
	"fmt"
	"net"
	"time"

	"go.uber.org/zap"
	"gorm.io/gorm"

	"github.com/maahdima/mwp/api/adaptor/mikrotik"
	"github.com/maahdima/mwp/api/dataservice/model"
)

// UserManagerProtocolHealthService implements spec section ب-7's own
// dedicated row for L2TP/PPTP/OpenVPN/SSTP: "پروب فعال به سرویس User
// Manager... فقط هشدار تلگرام، نه اقدام خودکار" (active probe to the
// User Manager service, alert-only, never an automatic remediation
// action -- restarting one of these servers could disrupt active
// customer sessions, so this is deliberately excluded from
// TunnelHealthService's own Level 1/2 engine).
//
// Confirmed hybrid check with the admin (this codebase's real setup has
// L2TP/PPTP/SSTP/OpenVPN configured directly on RouterOS by the admin,
// one GLOBAL port per protocol -- not per-location, see
// model.UserManagerProtocolConfig's own doc comment for why): (1) read
// RouterOS's own l2tp-server/pptp-server/sstp-server/ovpn-server
// "enabled" flag directly, (2) only if enabled, TCP-connect-probe the
// configured port to confirm something is genuinely listening. Both
// signals are recorded independently (model.UserManagerProtocolHealthStatus.
// RouterEnabled/PortReachable) so an alert names exactly which one
// failed.
type UserManagerProtocolHealthService struct {
	db              *gorm.DB
	mikrotikAdaptor *mikrotik.Adaptor
	botNotifier     *BotNotifier
	settings        *BotSettingsService
	logger          *zap.Logger
}

func NewUserManagerProtocolHealthService(db *gorm.DB, mikrotikAdaptor *mikrotik.Adaptor) *UserManagerProtocolHealthService {
	return &UserManagerProtocolHealthService{
		db:              db,
		mikrotikAdaptor: mikrotikAdaptor,
		settings:        NewBotSettingsService(db),
		logger:          zap.L().Named("UserManagerProtocolHealthService"),
	}
}

func (s *UserManagerProtocolHealthService) SetBotNotifier(notifier *BotNotifier) {
	s.botNotifier = notifier
}

// tcpProbeTimeout is deliberately short -- this runs once per protocol
// per poll tick, and a hung TCP handshake attempt must never stack up
// against the next tick.
const tcpProbeTimeout = 5 * time.Second

// protocolHealthFailureThreshold (confirmed, reported bug fix): a single
// transient probe failure (a brief network hiccup, a momentary port
// stall) used to immediately flip this protocol to unhealthy and fire a
// Telegram alert -- with zero debounce, unlike TunnelHealthService's own
// infrastructure-tunnel engine (3 consecutive bad samples required before
// declaring confirmed_down). Requiring this many consecutive failed polls
// before transitioning closes that gap; a single successful poll resets
// the counter to 0 immediately (fast recovery, slow failure).
const protocolHealthFailureThreshold = 2

// Poll checks all four User Manager protocols this poll tick. Each
// protocol's own admin-recorded UserManagerProtocolConfig row supplies
// the port/server-address to probe (the panel's own descriptive record,
// see that model's doc comment) -- a protocol with NO config row at all
// is treated as "the admin hasn't set this up yet," not a failure, so it
// is skipped entirely rather than alerting on something never claimed to
// exist.
func (s *UserManagerProtocolHealthService) Poll() {
	var configs []model.UserManagerProtocolConfig
	if err := s.db.Find(&configs).Error; err != nil {
		s.logger.Error("failed to load user manager protocol configs", zap.Error(err))
		return
	}

	now := time.Now()
	for _, cfg := range configs {
		if !cfg.Enabled {
			// The admin's own record says this protocol is intentionally
			// off -- not a failure, nothing to probe.
			continue
		}
		s.pollOneProtocol(cfg, now)
	}
}

func (s *UserManagerProtocolHealthService) pollOneProtocol(cfg model.UserManagerProtocolConfig, now time.Time) {
	ctx := context.Background()

	routerEnabled, skipPortProbe, err := s.readRouterEnabled(ctx, cfg.Protocol)
	if err != nil {
		s.logger.Warn("failed to read router-side protocol enabled state", zap.String("protocol", string(cfg.Protocol)), zap.Error(err))
		// Router unreachable this tick -- skip rather than recording a
		// false failure; the next tick will try again.
		return
	}

	// portReachable defaults to true when the port probe is skipped
	// (L2TP-over-IPsec, see readRouterEnabled's own doc comment) -- a TCP
	// connect can never validate a UDP-based IKE/L2TP handshake, so
	// treating an un-probed protocol as "reachable" avoids reporting a
	// false, structurally-guaranteed failure every single tick;
	// RouterEnabled remains the real signal for this protocol.
	portReachable := true
	if routerEnabled && !skipPortProbe {
		portReachable = s.probeTCPPort(cfg)
	}

	probePassed := routerEnabled && portReachable

	var status model.UserManagerProtocolHealthStatus
	dbErr := s.db.Where("protocol = ?", cfg.Protocol).First(&status).Error
	isNew := dbErr != nil
	wasHealthy := true
	if !isNew {
		wasHealthy = status.Healthy
	}

	// Debounce (see protocolHealthFailureThreshold's doc comment): a
	// passing probe always resets the streak and is healthy immediately;
	// a failing probe only flips Healthy to false once the consecutive
	// count reaches the threshold, so a single transient hiccup no longer
	// alerts on its own. A brand-new row (isNew) starts from the same
	// "previously healthy" assumption as wasHealthy's own default above,
	// so a first-ever failing poll is debounced exactly like any other.
	consecutiveFailures := 0
	healthy := true
	if !probePassed {
		consecutiveFailures = status.ConsecutiveFailures + 1
		healthy = wasHealthy && consecutiveFailures < protocolHealthFailureThreshold
	}

	status.Protocol = cfg.Protocol
	status.Healthy = healthy
	status.RouterEnabled = routerEnabled
	status.PortReachable = portReachable
	status.ConsecutiveFailures = consecutiveFailures
	status.LastCheckedAt = now

	if isNew {
		if err := s.db.Create(&status).Error; err != nil {
			s.logger.Error("failed to create user manager protocol health status", zap.String("protocol", string(cfg.Protocol)), zap.Error(err))
			return
		}
	} else if err := s.db.Save(&status).Error; err != nil {
		s.logger.Error("failed to update user manager protocol health status", zap.String("protocol", string(cfg.Protocol)), zap.Error(err))
		return
	}

	if healthy != wasHealthy {
		s.recordTransition(cfg.Protocol, wasHealthy, healthy, routerEnabled, portReachable, now)
	}
}

// readRouterEnabled reads the real, live "enabled" flag directly from
// RouterOS for the given protocol -- deliberately NOT trusting
// UserManagerProtocolConfig.Enabled alone (that field only records what
// the admin SAID they configured, see that field's own doc comment; this
// check confirms it against the router's actual current state, which
// could have drifted since). The second return value reports whether
// the TCP port-reachability probe should be SKIPPED for this protocol --
// true only for L2TP when the router itself reports use-ipsec="yes"
// (confirmed on a real production router): L2TP-over-IPsec's actual
// traffic is UDP (IKE on 500/4500, L2TP on 1701), so a TCP connect
// attempt against the admin-configured port can never succeed
// regardless of whether the service is actually healthy -- before this
// check existed, L2TP was reported "port not reachable" on every single
// poll tick even while genuinely serving traffic.
func (s *UserManagerProtocolHealthService) readRouterEnabled(ctx context.Context, protocol model.UserManagerAccountProtocol) (enabled bool, skipPortProbe bool, err error) {
	switch protocol {
	case model.ProtocolL2TP:
		cfg, err := s.mikrotikAdaptor.FetchL2TPServerConfig(ctx)
		if err != nil {
			return false, false, err
		}
		usesIpsec := cfg.UseIpsec != nil && *cfg.UseIpsec == "yes"
		return cfg.Enabled == "true", usesIpsec, nil
	case model.ProtocolPPTP:
		cfg, err := s.mikrotikAdaptor.FetchPPTPServerConfig(ctx)
		if err != nil {
			return false, false, err
		}
		return cfg.Enabled == "true", false, nil
	case model.ProtocolSSTP:
		cfg, err := s.mikrotikAdaptor.FetchSSTPServerConfig(ctx)
		if err != nil {
			return false, false, err
		}
		return cfg.Enabled == "true", false, nil
	case model.ProtocolOpenVPN:
		cfg, err := s.mikrotikAdaptor.FetchOvpnServerConfig(ctx)
		if err != nil {
			return false, false, err
		}
		return cfg.Enabled == "true", false, nil
	default:
		// ProtocolIKEv2 has no RouterOS User Manager counterpart at all
		// (see that constant's own doc comment) -- nothing to probe.
		return false, false, fmt.Errorf("protocol %q has no router-side health check", protocol)
	}
}

// probeTCPPort attempts a real TCP connection to the protocol's own
// configured port -- confirms something is actually listening, not just
// that RouterOS's config says it should be. ServerAddress falls back to
// the panel's own active MikroTik server address when unset, matching
// UserManagerProtocolConfig.ServerAddress's own documented fallback
// convention.
func (s *UserManagerProtocolHealthService) probeTCPPort(cfg model.UserManagerProtocolConfig) bool {
	host := ""
	if cfg.ServerAddress != nil && *cfg.ServerAddress != "" {
		host = *cfg.ServerAddress
	} else {
		var server model.Server
		if err := s.db.Where("is_active = ?", true).First(&server).Error; err != nil {
			return false
		}
		host = server.IPAddress
	}

	address := fmt.Sprintf("%s:%d", host, cfg.Port)
	conn, err := net.DialTimeout("tcp", address, tcpProbeTimeout)
	if err != nil {
		return false
	}
	_ = conn.Close()
	return true
}

func (s *UserManagerProtocolHealthService) recordTransition(protocol model.UserManagerAccountProtocol, wasHealthy, isHealthy, routerEnabled, portReachable bool, now time.Time) {
	from, to := "healthy", "unhealthy"
	if !wasHealthy {
		from = "unhealthy"
	}
	if isHealthy {
		to = "healthy"
	}

	evidence := s.buildEvidence(routerEnabled, portReachable)
	event := model.UserManagerProtocolHealthEvent{
		Protocol:   protocol,
		FromStatus: from,
		ToStatus:   to,
		Evidence:   evidence,
		DetectedAt: now,
	}
	if err := s.db.Create(&event).Error; err != nil {
		s.logger.Error("failed to record user manager protocol health event", zap.String("protocol", string(protocol)), zap.Error(err))
		return
	}

	// Alert only on a transition INTO unhealthy -- same policy as
	// TunnelHealthService's own recordTransition, per the admin's
	// standing "only alert on the transition, never every tick" rule.
	if to == "healthy" {
		return
	}
	if s.botNotifier == nil || s.settings == nil {
		return
	}
	settings, err := s.settings.GetOrCreate()
	if err != nil || settings.AdminChatID == "" {
		return
	}

	s.botNotifier.NotifyUserManagerProtocolDown(settings, string(protocol), evidence)

	sentAt := time.Now()
	s.db.Model(&event).Update("notified_at", sentAt)
}

func (s *UserManagerProtocolHealthService) buildEvidence(routerEnabled, portReachable bool) string {
	if !routerEnabled {
		return "سرویس در روتر غیرفعال شده است (enabled=false)"
	}
	if !portReachable {
		return "سرویس در روتر فعال است ولی پورت پاسخ نمی‌دهد (احتمال کرش یا مسدود شدن پورت)"
	}
	return "سرویس به حالت سالم بازگشت"
}

// ListStatuses returns every tracked User Manager protocol's current
// health -- the admin panel's own read endpoint.
func (s *UserManagerProtocolHealthService) ListStatuses() ([]model.UserManagerProtocolHealthStatus, error) {
	var statuses []model.UserManagerProtocolHealthStatus
	if err := s.db.Order("protocol asc").Find(&statuses).Error; err != nil {
		return nil, err
	}
	return statuses, nil
}

// ListEvents returns the most recent transition events, newest first.
func (s *UserManagerProtocolHealthService) ListEvents(limit int) ([]model.UserManagerProtocolHealthEvent, error) {
	if limit <= 0 || limit > 500 {
		limit = 100
	}
	var events []model.UserManagerProtocolHealthEvent
	if err := s.db.Order("detected_at desc").Limit(limit).Find(&events).Error; err != nil {
		return nil, err
	}
	return events, nil
}
