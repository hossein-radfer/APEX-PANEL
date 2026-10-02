package service

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"sort"
	"strings"
	"time"

	"go.uber.org/zap"
	"gorm.io/gorm"

	"github.com/maahdima/mwp/api/adaptor/mikrotik"
	"github.com/maahdima/mwp/api/dataservice/model"
	"github.com/maahdima/mwp/api/utils"
)

// TunnelHealthService implements the "tunnel-ai" spec's detection +
// self-healing engine for INFRASTRUCTURE tunnels only (GRE/IPIP/EoIP/
// WireGuard links between servers) -- discovery of WHICH interfaces are
// genuine tunnels comes from TunnelGraphService (see its own
// DiscoveredTunnelInterfaces, keyed on real routing-table participation,
// not interface type/name). This is a corrected rewrite of this
// service's first shipped revision, which wrongly monitored
// customer-facing WireGuard peer interfaces as if they were
// infrastructure -- see model/tunnel_health.go's own top-level doc
// comment for the full explanation of that reported bug and the fix.
//
// Detection method per tunnel TYPE follows spec section ب-7's own table,
// confirmed with the admin:
//   - WireGuard-based tunnels: interface state + last-handshake staleness
//   - Tx/Rx asymmetry (all three, same as before).
//   - GRE/IPIP/EoIP tunnels: interface state + Tx/Rx asymmetry only (no
//     handshake concept exists for these protocols).
//
// The self-healing decision engine (spec section ب-5) -- Level 1 (toggle
// interface), Level 2 (failover to a policy-defined backup tunnel),
// anti-flapping cooldown, and fallback-to-primary -- applies identically
// regardless of tunnel type, since toggling an interface's disabled flag
// works the same way for GRE/IPIP/EoIP/WireGuard alike. Level 3
// (autonomous new-tunnel creation) is intentionally NOT implemented yet.
//
// Every remediation action is gated by IsDryRun (default true, safe) and
// by TunnelRedlineValidator (spec section ب-6), exactly as before.
type TunnelHealthService struct {
	db              *gorm.DB
	mikrotikAdaptor *mikrotik.Adaptor
	graphService    *TunnelGraphService
	botNotifier     *BotNotifier
	settings        *BotSettingsService
	policyService   *TunnelPolicyService
	redline         *TunnelRedlineValidator
	logger          *zap.Logger
}

func NewTunnelHealthService(db *gorm.DB, mikrotikAdaptor *mikrotik.Adaptor, graphService *TunnelGraphService) *TunnelHealthService {
	return &TunnelHealthService{
		db:              db,
		mikrotikAdaptor: mikrotikAdaptor,
		graphService:    graphService,
		settings:        NewBotSettingsService(db),
		policyService:   NewTunnelPolicyService(db),
		redline:         NewTunnelRedlineValidator(db),
		logger:          zap.L().Named("TunnelHealthService"),
	}
}

// SetBotNotifier wires the Telegram alert channel in -- mirrors every
// other service's identical SetBotNotifier(botNotifier) pattern.
func (s *TunnelHealthService) SetBotNotifier(notifier *BotNotifier) {
	s.botNotifier = notifier
}

// dryRunConfigKey/emergencyStopConfigKey are SystemConfig rows (see
// model.SystemConfig's own key/value convention) -- plain booleans don't
// warrant a dedicated table, and this mirrors SystemConfigService's own
// GetPortOverride pattern of storing a single scalar under a well-known
// key.
const (
	dryRunConfigKey        = "tunnel_ai_dry_run"
	emergencyStopConfigKey = "tunnel_ai_emergency_stop"
)

// IsDryRun reports the current dry-run toggle -- defaults to true
// (simulate-only) whenever the row doesn't exist yet, so the engine is
// SAFE BY DEFAULT on a fresh install; an admin must explicitly opt into
// live remediation via SetDryRun(false).
func (s *TunnelHealthService) IsDryRun() bool {
	var cfg model.SystemConfig
	if err := s.db.Where("key = ?", dryRunConfigKey).First(&cfg).Error; err != nil {
		return true
	}
	return cfg.Value != "false"
}

func (s *TunnelHealthService) SetDryRun(enabled bool) error {
	return s.setBoolConfig(dryRunConfigKey, enabled)
}

// IsEmergencyStopped reports the global kill-switch (spec's own "دکمه
// توقف اضطراری", section ب-6) -- checked at the very top of every
// remediation attempt; when true the engine only observes, exactly like
// dry-run, but is a SEPARATE flag so an admin can leave dry-run
// permanently on for testing while still having an independent panic
// button once live mode is eventually enabled.
func (s *TunnelHealthService) IsEmergencyStopped() bool {
	var cfg model.SystemConfig
	if err := s.db.Where("key = ?", emergencyStopConfigKey).First(&cfg).Error; err != nil {
		return false
	}
	return cfg.Value == "true"
}

func (s *TunnelHealthService) SetEmergencyStop(enabled bool) error {
	return s.setBoolConfig(emergencyStopConfigKey, enabled)
}

func (s *TunnelHealthService) setBoolConfig(key string, enabled bool) error {
	value := "false"
	if enabled {
		value = "true"
	}
	var cfg model.SystemConfig
	err := s.db.Where("key = ?", key).First(&cfg).Error
	if err == nil {
		cfg.Value = value
		return s.db.Save(&cfg).Error
	}
	if !errors.Is(err, gorm.ErrRecordNotFound) {
		return err
	}
	return s.db.Create(&model.SystemConfig{Key: key, Value: value}).Error
}

// Detection thresholds -- the spec's own recommended defaults (section
// b-4). handshakeStaleThreshold: WireGuard's own industry-standard
// staleness marker (a peer that hasn't rekeyed in ~3 minutes is presumed
// to have lost its session). txRxAsymmetryWindow/txRxAsymmetryRxRatio:
// "tx keeps climbing, rx stays ~flat" over 3 consecutive 1-minute samples
// means packets are being sent into a tunnel that isn't answering.
const (
	handshakeStaleThreshold = 180 * time.Second
	txRxAsymmetryWindow     = 3 // consecutive samples, ~1/minute
	txRxAsymmetryRxRatio    = 0.05
	// txRxAsymmetryMinTxBytes is a confirmed, reported false-positive fix:
	// an infrastructure tunnel with NO real customer traffic on it right
	// now still carries a small amount of background/keepalive chatter
	// (routing protocol hellos, GRE keepalives), typically tens of KB
	// over a few minutes -- at that volume, ordinary noise in the byte
	// counters can easily push the rx/tx ratio below
	// txRxAsymmetryRxRatio on some tick even though nothing is actually
	// failing, since there is no real traffic pattern to judge in the
	// first place (see the admin's own point: "شاید اصلا کاربری از اون
	// تانل استفاده نمی‌کنه" -- maybe no one is even using that tunnel
	// right now). Below this floor, asymmetry is simply not evaluated --
	// an idle tunnel is reported on its OTHER signals (running/disabled/
	// handshake) only, not flagged down for a traffic pattern too small
	// to mean anything. 200KB of real outbound traffic over the 3-sample
	// window is comfortably above idle keepalive noise while still being
	// small enough to catch a genuinely-used tunnel failing quickly.
	txRxAsymmetryMinTxBytes = 200_000
	// txRxSampleRetention keeps only enough TunnelHealthSample rows to
	// feed the asymmetry window above -- pruned every tick so this table
	// never grows into a second traffic-history table (see
	// TunnelHealthSample's own doc comment).
	txRxSampleRetention = 10
)

// Early-warning health score. computeHealthScore folds the SAME signals
// pollOneTunnel already has on hand into one 0-100 number, so a tunnel's
// own trajectory (not just its current binary severity) can be tracked
// and alerted on. healthScoreHistoryRetention/healthScoreWarnLookback
// mirror txRxSampleRetention's own rolling-window approach -- computed
// and stored on the SAME 60s cycle as every other poll signal (matching
// the admin's own explicit choice not to run this on a separate
// schedule), pruned so this never grows into a full time-series store.
const (
	healthScoreHistoryRetention = 20
	// healthScoreWarnLookback is how many samples back "previous" means
	// when deciding whether a tunnel is degrading -- 5 samples at the
	// engine's own 60s poll interval is ~5 minutes, long enough to smooth
	// over a single noisy tick without being so long that a genuinely
	// fast failure is missed before Poll's own binary severity check
	// (which reacts on the very next tick) gets there first anyway.
	healthScoreWarnLookback = 5
	// healthScoreDropThreshold is how many points a score must fall over
	// healthScoreWarnLookback samples to count as "degrading" -- chosen
	// so a small amount of score jitter (e.g. one slightly-late
	// handshake) never fires a warning on its own; it takes a real,
	// sustained decline to cross this.
	healthScoreDropThreshold = 20
	// healthScoreWarnCooldown avoids re-alerting every single tick while
	// a tunnel sits in a degraded-but-not-yet-unhealthy state -- one
	// early warning per interface per cooldown window is enough to have
	// already told the admin; repeating it every 60s would just be noise
	// on top of the ACTUAL detection alert that will fire once severity
	// itself changes.
	healthScoreWarnCooldown = 30 * time.Minute
)

// computeHealthScore turns the same running/disabled/handshake/asymmetry
// signals computeSeverity already uses into a smooth 0-100 score instead
// of a 3-value enum, so a tunnel's trend (not just its current bucket)
// can be tracked. Deliberately simple and explainable (fixed point
// deductions per signal, not a learned/weighted model) -- matching this
// engine's overall "deterministic, no hidden state" design (see this
// file's own decision-engine doc comment).
func computeHealthScore(running, disabled bool, worstHandshakeAge *int, asymmetry bool) int {
	score := 100
	if disabled {
		score -= 60
	} else if !running {
		score -= 50
	}
	if worstHandshakeAge != nil {
		// Linearly deduct up to 30 points as the worst handshake age
		// approaches (and passes) handshakeStaleThreshold -- a handshake
		// that's merely getting old (say, half the stale threshold) already
		// pulls the score down noticeably before computeSeverity itself
		// would call this tunnel "suspect".
		staleSeconds := int(handshakeStaleThreshold.Seconds())
		ratio := float64(*worstHandshakeAge) / float64(staleSeconds)
		if ratio > 1 {
			ratio = 1
		}
		score -= int(ratio * 30)
	}
	if asymmetry {
		score -= 40
	}
	if score < 0 {
		score = 0
	}
	if score > 100 {
		score = 100
	}
	return score
}

// recordHealthScoreAndWarn stores this tick's health score, prunes old
// rows, then checks whether the score has dropped substantially over
// healthScoreWarnLookback samples -- if so, and the tunnel is not
// ALREADY confirmed_down/suspect (those already get their own, more
// specific alert via recordTransition) and no warning was already sent
// within healthScoreWarnCooldown, sends the early-warning Telegram
// alert. This is deliberately independent of computeSeverity's own
// binary state machine: a tunnel can be trending badly for many ticks
// before (or even without ever) crossing into suspect/confirmed_down,
// and the whole point of this capability is to surface that trend
// early rather than waiting for the binary threshold to trip.
func (s *TunnelHealthService) recordHealthScoreAndWarn(interfaceName string, running, disabled bool, worstHandshakeAge *int, asymmetry bool, severity string, now time.Time) {
	score := computeHealthScore(running, disabled, worstHandshakeAge, asymmetry)

	if err := s.db.Create(&model.TunnelHealthScore{InterfaceName: interfaceName, Score: score, SampledAt: now}).Error; err != nil {
		s.logger.Warn("failed to record tunnel health score", zap.String("interface", interfaceName), zap.Error(err))
	}

	var toDelete []uint
	s.db.Model(&model.TunnelHealthScore{}).
		Where("interface_name = ?", interfaceName).
		Order("sampled_at desc").
		Offset(healthScoreHistoryRetention).
		Pluck("id", &toDelete)
	if len(toDelete) > 0 {
		s.db.Where("id IN ?", toDelete).Delete(&model.TunnelHealthScore{})
	}

	if severity != "healthy" {
		// Already suspect/confirmed_down -- recordTransition's own alert
		// (or, if this severity persists, silence by design) already
		// covers it; an early "warning" about a tunnel that's already
		// known-bad would be redundant noise, not useful lead time.
		return
	}

	var samples []model.TunnelHealthScore
	if err := s.db.Where("interface_name = ?", interfaceName).Order("sampled_at desc").Limit(healthScoreWarnLookback + 1).Find(&samples).Error; err != nil {
		return
	}
	if len(samples) < healthScoreWarnLookback+1 {
		return
	}
	previousScore := samples[len(samples)-1].Score
	currentScore := samples[0].Score
	if previousScore-currentScore < healthScoreDropThreshold {
		return
	}

	// The cooldown check and the event write below both happen
	// regardless of whether a Telegram notifier is currently configured
	// -- this TunnelHealthEvent row IS the durable, restart-safe "was
	// this already warned about recently" record (matching
	// antiFlappingCooldownActive's own "no hidden in-memory state"
	// convention elsewhere in this file), and it also powers the panel's
	// OWN history view of degrading-warning incidents independent of
	// whether Telegram happens to be wired up. Only the actual outbound
	// message send below is skipped without a notifier.
	var recentWarning model.TunnelHealthEvent
	err := s.db.Where("interface_name = ? AND to_status = ? AND detected_at >= ?",
		interfaceName, "degrading_warning", now.Add(-healthScoreWarnCooldown)).
		Order("detected_at desc").First(&recentWarning).Error
	if err == nil {
		return
	}
	if !errors.Is(err, gorm.ErrRecordNotFound) {
		s.logger.Error("failed to check early-warning cooldown", zap.String("interface", interfaceName), zap.Error(err))
		return
	}

	// Recorded as a TunnelHealthEvent with a dedicated pseudo-status
	// ("degrading_warning", never a real Severity value computeSeverity
	// itself produces) so it shares the same table/cooldown-query shape
	// as every other transition event, rather than adding a whole new
	// table for what is fundamentally the same kind of record.
	event := model.TunnelHealthEvent{
		InterfaceName: interfaceName,
		FromStatus:    "healthy",
		ToStatus:      "degrading_warning",
		Evidence:      fmt.Sprintf("امتیاز سلامت از %d به %d کاهش یافت", previousScore, currentScore),
		DetectedAt:    now,
	}
	if err := s.db.Create(&event).Error; err != nil {
		s.logger.Error("failed to record early-warning event", zap.String("interface", interfaceName), zap.Error(err))
		return
	}

	if s.botNotifier == nil || s.settings == nil {
		return
	}
	settings, err := s.settings.GetOrCreate()
	if err != nil || settings.AdminChatID == "" {
		return
	}
	s.botNotifier.NotifyTunnelHealthDegrading(settings, interfaceName, previousScore, currentScore, now)
	sentAt := time.Now()
	s.db.Model(&event).Update("notified_at", sentAt)
}

// Poll is the scheduled job entry point -- one full detection cycle
// across every interface TunnelGraphService's latest snapshot flagged as
// a genuine infrastructure tunnel. Deliberately does NOT poll every
// WireGuard interface on the router (see this service's own top-level
// doc comment) -- if no discovery snapshot exists yet, this is a no-op,
// not an error, until the graph-discovery job (running on its own,
// separate 5-minute schedule) completes its first tick.
func (s *TunnelHealthService) Poll() {
	tunnelNodes, err := s.graphService.DiscoveredTunnelInterfaces()
	if err != nil {
		s.logger.Error("failed to load discovered tunnel interfaces", zap.Error(err))
		return
	}
	if len(tunnelNodes) == 0 {
		return
	}

	s.pruneStaleTunnels(tunnelNodes)

	ctx := context.Background()
	mtInterfaces, err := s.mikrotikAdaptor.FetchAllInterfaces(ctx)
	if err != nil {
		s.logger.Error("failed to fetch interfaces from mikrotik", zap.Error(err))
		return
	}
	mtIfaceByName := make(map[string]mikrotik.Interface, len(mtInterfaces))
	for _, i := range mtInterfaces {
		mtIfaceByName[i.Name] = i
	}

	mtPeers, err := s.mikrotikAdaptor.FetchWgPeers(ctx)
	if err != nil {
		s.logger.Error("failed to fetch wireguard peers from mikrotik", zap.Error(err))
		mtPeers = nil // WireGuard-based tunnels simply skip handshake detection this tick
	}
	peersByInterface := make(map[string][]mikrotik.WireGuardPeer, len(mtPeers))
	for _, p := range mtPeers {
		peersByInterface[p.Interface] = append(peersByInterface[p.Interface], p)
	}

	now := time.Now()
	for _, node := range tunnelNodes {
		mtIface, known := mtIfaceByName[node.Name]
		if !known {
			// The graph saw this interface as a route's gateway in the
			// last discovery tick, but it no longer exists on the router
			// right now (deleted between ticks) -- nothing to poll.
			continue
		}
		s.pollOneTunnel(node, mtIface, peersByInterface[node.Name], now)
	}
}

// pruneStaleTunnels deletes TunnelHealthStatus/TunnelPolicy rows for any
// interface name that is no longer among the freshly discovered tunnels.
// Confirmed as a real reported bug: TunnelHealthStatus is documented as
// "one row per interface name" (current state, not history -- see that
// model's own doc comment), yet nothing ever deleted a row once its
// interface stopped being detected as a tunnel -- so every interface the
// FIRST, buggy version of tunnelGatewayNames() ever wrongly flagged
// (ordinary numbered WireGuard customer peers) is still sitting in this
// table today, alongside the real tunnels the corrected detection now
// also finds, making both the tunnel-status list and the per-tunnel
// policy list (TunnelPolicyService.List, which reads every TunnelPolicy
// row with no such filter either) show a mix of real tunnels and long-
// dead false positives. TunnelHealthEvent/TunnelActionLog are
// deliberately NOT touched here -- those are this system's own
// append-only audit history (see their own doc comments), and a past
// event genuinely happened at the time it was recorded regardless of
// what today's detection says.
func (s *TunnelHealthService) pruneStaleTunnels(tunnelNodes []model.GraphNode) {
	currentNames := make([]string, len(tunnelNodes))
	for i, node := range tunnelNodes {
		currentNames[i] = node.Name
	}
	// Unscoped (hard delete), not a soft delete: both tables carry a
	// plain UNIQUE index on interface_name that is NOT filtered on
	// deleted_at, so a soft-deleted row would still occupy that name and
	// make pollOneTunnel's/GetOrCreateDefault's own Create() calls fail
	// with a unique-constraint violation the moment a pruned interface
	// (e.g. one that drops out of detection for one tick and comes back)
	// needs its row recreated.
	if err := s.db.Unscoped().Where("interface_name NOT IN ?", currentNames).Delete(&model.TunnelHealthStatus{}).Error; err != nil {
		s.logger.Error("failed to prune stale tunnel health statuses", zap.Error(err))
	}
	if err := s.db.Unscoped().Where("interface_name NOT IN ?", currentNames).Delete(&model.TunnelPolicy{}).Error; err != nil {
		s.logger.Error("failed to prune stale tunnel policies", zap.Error(err))
	}
	// TunnelHealthScore/TunnelBackupProbeResult are per-interface ROLLING
	// windows (see their own doc comments), pruned down to a fixed row
	// count only on the next poll/probe FOR THAT SAME INTERFACE -- an
	// interface that stops being detected as a tunnel entirely would
	// otherwise never poll/probe again and its old rows would linger
	// forever instead of aging out, so they're hard-deleted here too,
	// same as TunnelHealthStatus/TunnelPolicy above.
	if err := s.db.Unscoped().Where("interface_name NOT IN ?", currentNames).Delete(&model.TunnelHealthScore{}).Error; err != nil {
		s.logger.Error("failed to prune stale tunnel health scores", zap.Error(err))
	}
	if err := s.db.Unscoped().Where("interface_name NOT IN ?", currentNames).Delete(&model.TunnelBackupProbeResult{}).Error; err != nil {
		s.logger.Error("failed to prune stale tunnel backup probe results", zap.Error(err))
	}
}

// tunnelInterfaceType reads back the real interface type
// (TunnelGraphService.createTunnelInterfaceNodes stashed it in
// PropertiesJSON) -- drives which detection signals apply.
func tunnelInterfaceType(node model.GraphNode) string {
	var props struct {
		InterfaceType string `json:"interface_type"`
	}
	_ = json.Unmarshal([]byte(node.PropertiesJSON), &props)
	return props.InterfaceType
}

func (s *TunnelHealthService) pollOneTunnel(node model.GraphNode, mtIface mikrotik.Interface, peers []mikrotik.WireGuardPeer, now time.Time) {
	interfaceName := node.Name
	interfaceType := tunnelInterfaceType(node)
	running := mtIface.Running == "true"
	disabled := mtIface.Disabled == "true"

	// Handshake staleness is only meaningful for a WireGuard-based
	// tunnel -- GRE/IPIP/EoIP have no handshake concept at all (spec
	// section ب-7's own per-protocol table).
	var worstHandshakeAge *int
	if interfaceType == "wireguard" {
		worstHandshakeAge = s.worstHandshakeAge(peers, now)
	}
	asymmetry := s.recordTxRxSample(interfaceName, mtIface, peers, interfaceType, now)

	severity := s.computeSeverity(running, disabled, worstHandshakeAge, asymmetry)

	var status model.TunnelHealthStatus
	err := s.db.Where("interface_name = ?", interfaceName).First(&status).Error
	isNew := err != nil
	previousSeverity := "healthy"
	if !isNew {
		previousSeverity = status.Severity
	}

	status.InterfaceName = interfaceName
	status.Severity = severity
	status.InterfaceRunning = running
	status.InterfaceDisabled = disabled
	status.WorstPeerLastHandshakeAgeSeconds = worstHandshakeAge
	status.TxRxAsymmetryDetected = asymmetry
	status.LastPolledAt = now

	// Cache this tick's resolved gateway IP ONLY while the tunnel is
	// actually healthy -- see LastKnownGatewayIP's own doc comment for
	// the exact incident this closes. Deliberately does NOT overwrite
	// the cached value on a down/suspect tick: RouterOS's own
	// ImmediateGw resolution is exactly what goes missing during an
	// outage, so refreshing from a now-empty live value here would erase
	// the one piece of data attemptLevel2 needs to still find the
	// affected routes.
	if severity == "healthy" {
		if gw := s.currentGatewayIP(interfaceName); gw != "" {
			status.LastKnownGatewayIP = gw
		}
	}

	if isNew {
		if err := s.db.Create(&status).Error; err != nil {
			s.logger.Error("failed to create tunnel health status", zap.String("interface", interfaceName), zap.Error(err))
			return
		}
	} else {
		if err := s.db.Save(&status).Error; err != nil {
			s.logger.Error("failed to update tunnel health status", zap.String("interface", interfaceName), zap.Error(err))
			return
		}
	}

	if severity != previousSeverity {
		s.recordTransition(interfaceName, previousSeverity, severity, running, disabled, worstHandshakeAge, asymmetry, now)
	}

	// Early-warning health score -- computed and checked on every poll
	// tick regardless of severity, since the whole point is to catch a
	// tunnel that is degrading BEFORE severity itself ever changes (see
	// recordHealthScoreAndWarn's own doc comment).
	s.recordHealthScoreAndWarn(interfaceName, running, disabled, worstHandshakeAge, asymmetry, severity, now)

	// Every discovered tunnel always has a usable policy (seeded with
	// safe defaults, Level3 disabled) so the decision engine below never
	// has to special-case "no policy yet" -- see
	// TunnelPolicyService.GetOrCreateDefault.
	policy, err := s.policyService.GetOrCreateDefault(interfaceName)
	if err != nil {
		s.logger.Error("failed to load tunnel policy", zap.String("interface", interfaceName), zap.Error(err))
		return
	}

	if severity == "confirmed_down" || severity == "suspect" {
		// Root-cause diagnosis runs ONCE per incident, the moment a tunnel
		// FIRST becomes confirmed_down (not on every tick it stays down,
		// and not for a mere "suspect") -- see diagnoseIncidentOnce's own
		// doc comment.
		if severity == "confirmed_down" && previousSeverity != "confirmed_down" {
			s.diagnoseIncidentOnce(interfaceName, now)
		}
		s.considerRemediation(interfaceName, policy, now)
	} else if severity == "healthy" && previousSeverity != "healthy" {
		s.considerFallbackRecovery(interfaceName, now)
	}
}

// computeSeverity combines all applicable signals -- CONFIRMED_DOWN
// takes priority over SUSPECT, matching the spec's own severity ordering
// (section b-4, method 3's own note: Tx/Rx asymmetry alone is enough for
// CONFIRMED_DOWN even if the interface still reports running=true).
func (s *TunnelHealthService) computeSeverity(running, disabled bool, worstHandshakeAge *int, asymmetry bool) string {
	if asymmetry {
		return "confirmed_down"
	}
	if disabled || !running {
		return "suspect"
	}
	if worstHandshakeAge != nil && *worstHandshakeAge > int(handshakeStaleThreshold.Seconds()) {
		return "suspect"
	}
	return "healthy"
}

// worstHandshakeAge returns the largest last-handshake age (in seconds)
// across every peer that has EVER handshaked on this interface -- nil
// when there are no peers at all, or when every peer has never
// handshaked. A peer with no last-handshake field at all (RouterOS omits
// it entirely for a peer that has never connected) is deliberately
// EXCLUDED from this computation, not treated as maximally stale -- see
// model.TunnelHealthStatus.WorstPeerLastHandshakeAgeSeconds's own doc
// comment for the confirmed, reported bug this avoids repeating.
func (s *TunnelHealthService) worstHandshakeAge(peers []mikrotik.WireGuardPeer, now time.Time) *int {
	var worst *int
	for _, p := range peers {
		if p.LastHandshake == nil {
			continue
		}
		age, err := utils.ParseCustomDuration(*p.LastHandshake)
		if err != nil {
			continue
		}
		ageSeconds := int(age.Seconds())
		if worst == nil || ageSeconds > *worst {
			worst = &ageSeconds
		}
	}
	return worst
}

// recordTxRxSample implements spec section b-4 method 3: record this
// tick's Tx/Rx counters, prune old samples, then check whether Tx has
// grown substantially over the window while Rx has stayed nearly flat --
// packets going out with (almost) nothing coming back is the strongest
// single signal a tunnel is dead even while RouterOS still reports the
// interface itself as running. For a WireGuard-based tunnel, counters
// are summed across its peers (RouterOS reports Tx/Rx per-peer there);
// for GRE/IPIP/EoIP, RouterOS reports Tx/Rx directly on the interface
// itself.
func (s *TunnelHealthService) recordTxRxSample(interfaceName string, mtIface mikrotik.Interface, peers []mikrotik.WireGuardPeer, interfaceType string, now time.Time) bool {
	var totalTx, totalRx int64
	if interfaceType == "wireguard" {
		for _, p := range peers {
			totalTx += utils.ParseStringToInt(p.TransferTx)
			totalRx += utils.ParseStringToInt(p.TransferRx)
		}
	} else {
		totalTx = utils.ParseStringToInt(mtIface.TxByte)
		totalRx = utils.ParseStringToInt(mtIface.RxByte)
	}

	sample := model.TunnelHealthSample{InterfaceName: interfaceName, TxBytes: totalTx, RxBytes: totalRx, SampledAt: now}
	if err := s.db.Create(&sample).Error; err != nil {
		s.logger.Warn("failed to record tunnel health sample", zap.String("interface", interfaceName), zap.Error(err))
	}

	// Prune down to txRxSampleRetention rows for this interface -- a
	// small, fixed-size rolling window, not a growing history table.
	//
	// Unscoped() is required here -- a confirmed, reported bug mirroring
	// TunnelGraphService.pruneOldSnapshots/SecurityRetentionService's own
	// identical mistake: TunnelHealthSample embeds model.Model (carrying
	// gorm.DeletedAt), so this plain .Delete() call only ever soft-deleted
	// rows, defeating the entire point of this rolling-window prune --
	// every row "pruned" since this code was written is still present in
	// the database file (223K+ rows found live against a rolling window
	// meant to stay small per interface).
	var toDelete []uint
	s.db.Model(&model.TunnelHealthSample{}).
		Where("interface_name = ?", interfaceName).
		Order("sampled_at desc").
		Offset(txRxSampleRetention).
		Pluck("id", &toDelete)
	if len(toDelete) > 0 {
		s.db.Unscoped().Where("id IN ?", toDelete).Delete(&model.TunnelHealthSample{})
	}

	var samples []model.TunnelHealthSample
	if err := s.db.Where("interface_name = ?", interfaceName).Order("sampled_at desc").Limit(txRxAsymmetryWindow + 1).Find(&samples).Error; err != nil {
		return false
	}
	if len(samples) < txRxAsymmetryWindow+1 {
		return false
	}

	oldest := samples[len(samples)-1]
	newest := samples[0]
	txDelta := newest.TxBytes - oldest.TxBytes
	rxDelta := newest.RxBytes - oldest.RxBytes

	// Below txRxAsymmetryMinTxBytes there simply isn't enough real
	// outbound traffic to judge asymmetry from at all -- see that
	// constant's own doc comment for the confirmed false-positive this
	// avoids on a tunnel nobody happens to be using right now.
	if txDelta < txRxAsymmetryMinTxBytes {
		return false
	}
	return float64(rxDelta) < float64(txDelta)*txRxAsymmetryRxRatio
}

// recordTransition writes a TunnelHealthEvent for a severity change and
// sends a Telegram alert -- the admin's own explicit requirement that
// alerts fire ONLY on a healthy->(suspect|confirmed_down) transition,
// never repeatedly while a tunnel remains down and never on a recovery
// (silent recovery, matching the spec's own "fallback" philosophy that
// recovery is the expected/unremarkable case, not a lack of the failure
// that WAS remarkable).
func (s *TunnelHealthService) recordTransition(interfaceName, from, to string, running, disabled bool, worstHandshakeAge *int, asymmetry bool, now time.Time) {
	evidence := s.buildEvidence(running, disabled, worstHandshakeAge, asymmetry)

	event := model.TunnelHealthEvent{
		InterfaceName: interfaceName,
		FromStatus:    from,
		ToStatus:      to,
		Evidence:      evidence,
		DetectedAt:    now,
	}
	if err := s.db.Create(&event).Error; err != nil {
		s.logger.Error("failed to record tunnel health event", zap.String("interface", interfaceName), zap.Error(err))
		return
	}

	// Alert only on a transition INTO suspect/confirmed_down, matching
	// the admin's own explicit choice -- a transition back to healthy is
	// recorded (for the history page) but never notified.
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

	s.botNotifier.NotifyTunnelHealthChange(settings, interfaceName, to, evidence, now)

	sentAt := time.Now()
	s.db.Model(&event).Update("notified_at", sentAt)
}

// buildEvidence renders which signal(s) fired into one Persian sentence --
// matching the spec's own "message_fa" / human-readable-Persian-log
// requirement (section b-8).
func (s *TunnelHealthService) buildEvidence(running, disabled bool, worstHandshakeAge *int, asymmetry bool) string {
	var reasons []string
	if disabled {
		reasons = append(reasons, "اینترفیس غیرفعال شده است")
	} else if !running {
		reasons = append(reasons, "اینترفیس در حالت running نیست")
	}
	if worstHandshakeAge != nil && *worstHandshakeAge > int(handshakeStaleThreshold.Seconds()) {
		reasons = append(reasons, fmt.Sprintf("آخرین handshake بیش از %d ثانیه پیش بوده", *worstHandshakeAge))
	}
	if asymmetry {
		reasons = append(reasons, "ترافیک خروجی افزایش دارد ولی ترافیک ورودی تقریباً صفر است (نشانه‌ی افتادن تانل)")
	}
	if len(reasons) == 0 {
		return "وضعیت به حالت سالم بازگشت"
	}
	result := reasons[0]
	for _, r := range reasons[1:] {
		result += "؛ " + r
	}
	return result
}

// Root-cause diagnosis. A confirmed_down alert on its own only tells the
// admin THAT a tunnel is down, not WHY -- three operationally very
// different situations (the tunnel's own remote end is unreachable, the
// whole router has lost its uplink, or the router itself is overloaded)
// all look identical from that alert alone. diagnoseIncidentOnce runs a
// small, fast set of active probes FROM THE SAME ROUTER the instant a
// tunnel is first confirmed down, to tell those apart.
const (
	// diagnosisPingCount is deliberately small -- this runs synchronously
	// inside the poll tick (which itself runs every 60s), so the total
	// added latency across a handful of probe pings must stay well under
	// that budget. 2 echoes is enough to tell "responds at all" from
	// "completely unreachable" without turning this into a slow, thorough
	// network diagnostic.
	diagnosisPingCount = 2
	// diagnosisCPULoadHighPercent/diagnosisFreeMemoryLowPercent are the
	// thresholds for calling the router itself "resource_exhausted" --
	// intentionally conservative (a router legitimately busy running many
	// tunnels can sit well below these) so this cause is only reported
	// when the router is genuinely under unusual pressure, not just busy.
	diagnosisCPULoadHighPercent   = 90
	diagnosisFreeMemoryLowPercent = 5
)

// diagnoseIncidentOnce is called exactly once per incident (guarded by
// its own caller only invoking this on the healthy/suspect ->
// confirmed_down transition edge, never on every tick a tunnel stays
// down) -- pings the down tunnel's own gateway IP, pings a small sample
// of OTHER currently-healthy tunnels' gateway IPs from the same router,
// and reads the router's own current CPU/memory, then classifies the
// result into TunnelIncidentDiagnosis.Cause. Best-effort throughout: any
// probe that fails to even run (adaptor error) is treated as
// inconclusive for that one signal rather than aborting the whole
// diagnosis, since a partial diagnosis is still more useful to the admin
// than none at all.
func (s *TunnelHealthService) diagnoseIncidentOnce(interfaceName string, now time.Time) {
	ctx := context.Background()

	downGatewayIP := s.tunnelGatewayIP(ctx, interfaceName)
	downReachable, downEvidence := s.probeReachable(ctx, downGatewayIP)

	otherReachableCount, otherTotal, otherEvidence := s.probeOtherTunnels(ctx, interfaceName)

	cpuHigh, memLow, resourceEvidence := s.probeRouterResources(ctx)

	var cause string
	var evidenceParts []string
	switch {
	case cpuHigh || memLow:
		cause = "router_resource_exhausted"
		evidenceParts = append(evidenceParts, resourceEvidence)
	case otherTotal > 0 && otherReachableCount == 0:
		cause = "router_uplink_down"
		evidenceParts = append(evidenceParts, fmt.Sprintf("هیچ‌کدام از %d تانل سالم دیگر هم از این روتر پاسخ ندادند", otherTotal))
	case downGatewayIP != "" && !downReachable && (otherTotal == 0 || otherReachableCount > 0):
		cause = "remote_unreachable"
		evidenceParts = append(evidenceParts, downEvidence)
		if otherTotal > 0 {
			evidenceParts = append(evidenceParts, fmt.Sprintf("%d از %d تانل دیگر همچنان پاسخ می‌دهند", otherReachableCount, otherTotal))
		}
	default:
		cause = "unknown"
		if downGatewayIP == "" {
			evidenceParts = append(evidenceParts, "آی‌پی واسطه‌ی این تانل برای تست پینگ در دسترس نبود")
		} else {
			evidenceParts = append(evidenceParts, downEvidence)
		}
	}
	if otherEvidence != "" {
		evidenceParts = append(evidenceParts, otherEvidence)
	}
	evidence := strings.Join(evidenceParts, "؛ ")

	diagnosis := model.TunnelIncidentDiagnosis{
		InterfaceName: interfaceName,
		Cause:         cause,
		Evidence:      evidence,
		DiagnosedAt:   now,
	}
	if err := s.db.Create(&diagnosis).Error; err != nil {
		s.logger.Error("failed to record tunnel incident diagnosis", zap.String("interface", interfaceName), zap.Error(err))
		return
	}

	if s.botNotifier == nil || s.settings == nil {
		return
	}
	settings, err := s.settings.GetOrCreate()
	if err != nil || settings.AdminChatID == "" {
		return
	}
	s.botNotifier.NotifyTunnelIncidentDiagnosis(settings, interfaceName, cause, evidence, now)
}

// tunnelGatewayIP looks up interfaceName's own real next-hop IP from the
// current routing table (same routeGatewayInterface resolution Level 2/3
// already rely on) -- reads whichever matching route's OWN gateway/
// immediate-gw value first, since that IS the address a ping should
// target to test reachability of this specific tunnel's remote end.
func (s *TunnelHealthService) tunnelGatewayIP(ctx context.Context, interfaceName string) string {
	routes, err := s.mikrotikAdaptor.FetchIPRoutes(ctx)
	if err != nil {
		return ""
	}
	for _, route := range routes {
		if route.Connect != nil && *route.Connect == "true" {
			continue
		}
		if routeGatewayInterface(route) != interfaceName {
			continue
		}
		if route.Gateway != nil && *route.Gateway != "" {
			// Gateway may itself carry a "%interface" suffix (see
			// routeGatewayInterface's own parsing) -- strip it back off
			// since a ping target must be a bare address.
			if idx := strings.LastIndex(*route.Gateway, "%"); idx != -1 {
				return (*route.Gateway)[:idx]
			}
			return *route.Gateway
		}
	}
	return ""
}

// probeReachable pings one address (empty address is treated as
// "nothing to probe", not an error) and reports whether at least one
// echo got a reply.
func (s *TunnelHealthService) probeReachable(ctx context.Context, address string) (bool, string) {
	if address == "" {
		return false, "آی‌پی مقصد برای تست در دسترس نبود"
	}
	results, err := s.mikrotikAdaptor.Ping(ctx, address, diagnosisPingCount)
	if err != nil {
		return false, fmt.Sprintf("تست پینگ به %s ناموفق بود: %s", address, err.Error())
	}
	for _, r := range results {
		if r.Time != "" {
			return true, fmt.Sprintf("%s پاسخ داد", address)
		}
	}
	return false, fmt.Sprintf("%s به هیچ‌کدام از %d پینگ پاسخ نداد", address, diagnosisPingCount)
}

// probeOtherTunnels samples up to 3 OTHER currently-healthy tunnels'
// gateway IPs and pings each -- the signal that tells "just this one
// tunnel is dead" (others still respond) apart from "the whole router
// lost its uplink" (nothing responds, including tunnels Poll itself
// still considers healthy as of its last tick).
func (s *TunnelHealthService) probeOtherTunnels(ctx context.Context, downInterfaceName string) (reachableCount, total int, evidence string) {
	const maxOtherProbes = 3

	var others []model.TunnelHealthStatus
	if err := s.db.Where("interface_name != ? AND severity = ?", downInterfaceName, "healthy").
		Limit(maxOtherProbes).Find(&others).Error; err != nil {
		return 0, 0, ""
	}
	if len(others) == 0 {
		return 0, 0, ""
	}

	var names []string
	for _, o := range others {
		gatewayIP := s.tunnelGatewayIP(ctx, o.InterfaceName)
		if gatewayIP == "" {
			continue
		}
		total++
		reachable, _ := s.probeReachable(ctx, gatewayIP)
		if reachable {
			reachableCount++
			names = append(names, o.InterfaceName)
		}
	}
	if total == 0 {
		return 0, 0, ""
	}
	return reachableCount, total, fmt.Sprintf("تانل‌های دیگر بررسی‌شده: %d از %d پاسخ دادند", reachableCount, total)
}

// probeRouterResources reads the router's own current CPU load / free
// memory and reports whether either crosses this diagnosis's own
// conservative overload thresholds.
func (s *TunnelHealthService) probeRouterResources(ctx context.Context) (cpuHigh, memLow bool, evidence string) {
	info, err := s.mikrotikAdaptor.FetchDeviceInfo(ctx)
	if err != nil {
		return false, false, "خواندن وضعیت منابع روتر ممکن نشد"
	}
	cpuLoad := utils.ParseStringToInt(info.CPULoad)
	if cpuLoad >= diagnosisCPULoadHighPercent {
		cpuHigh = true
	}
	freeMem := utils.ParseStringToInt(info.FreeMemory)
	totalMem := utils.ParseStringToInt(info.TotalMemory)
	if totalMem > 0 {
		freeMemPercent := freeMem * 100 / totalMem
		if freeMemPercent <= diagnosisFreeMemoryLowPercent {
			memLow = true
		}
	}
	if !cpuHigh && !memLow {
		return false, false, ""
	}
	return cpuHigh, memLow, fmt.Sprintf("بار CPU روتر: %d%%، حافظه‌ی آزاد: %s از %s", cpuLoad, info.FreeMemory, info.TotalMemory)
}

// ListStatuses returns every tracked infrastructure tunnel's current
// health -- the admin panel's own "read the current state" endpoint.
func (s *TunnelHealthService) ListStatuses() ([]model.TunnelHealthStatus, error) {
	var statuses []model.TunnelHealthStatus
	if err := s.db.Order("severity desc, interface_name asc").Find(&statuses).Error; err != nil {
		s.logger.Error("failed to list tunnel health statuses", zap.Error(err))
		return nil, err
	}
	return statuses, nil
}

// ListEvents returns the most recent detection events (history/alerting
// log page, spec section b-8), newest first.
func (s *TunnelHealthService) ListEvents(limit int) ([]model.TunnelHealthEvent, error) {
	if limit <= 0 || limit > 500 {
		limit = 100
	}
	var events []model.TunnelHealthEvent
	if err := s.db.Order("detected_at desc").Limit(limit).Find(&events).Error; err != nil {
		s.logger.Error("failed to list tunnel health events", zap.Error(err))
		return nil, err
	}
	return events, nil
}

// ListActions returns the most recent remediation decisions (real or
// simulated) -- the "تاریخچه و گزارش‌ها" page's own dry-run-aware view.
func (s *TunnelHealthService) ListActions(limit int) ([]model.TunnelActionLog, error) {
	if limit <= 0 || limit > 500 {
		limit = 100
	}
	var actions []model.TunnelActionLog
	if err := s.db.Order("executed_at desc").Limit(limit).Find(&actions).Error; err != nil {
		s.logger.Error("failed to list tunnel action log", zap.Error(err))
		return nil, err
	}
	return actions, nil
}

// ---------------------------------------------------------------------
// Decision engine (spec section ب-5). Deliberately simple/deterministic:
// no hidden state beyond what's readable from TunnelActionLog/
// TunnelHealthEvent -- anti-flapping and cooldown are both computed
// fresh from these tables on every call rather than cached in memory, so
// the engine's behavior survives a process restart without amnesia.
// ---------------------------------------------------------------------

// antiFlappingCooldownActive implements the spec's "جلوگیری از نوسان":
// if this tunnel has flapped (healthy->suspect/confirmed_down
// transitions) more than policy.AntiFlapping.MaxFlaps times within the
// configured window, remediation is suppressed for CooldownHours --
// during cooldown the tunnel stays on whatever it last fell back to,
// only a notification fires, matching the spec's own "روی بکاپ می‌ماند،
// هیچ اقدام خودکار بیشتری انجام نمی‌شود" (stays on backup, no further
// automatic action).
func (s *TunnelHealthService) antiFlappingCooldownActive(interfaceName string, policy TunnelPolicyConfig, now time.Time) bool {
	if policy.AntiFlapping.MaxFlaps <= 0 || policy.AntiFlapping.WindowMinutes <= 0 {
		return false
	}
	windowStart := now.Add(-time.Duration(policy.AntiFlapping.WindowMinutes) * time.Minute)
	var flapCount int64
	s.db.Model(&model.TunnelHealthEvent{}).
		Where("interface_name = ? AND from_status = ? AND to_status != ? AND detected_at >= ?", interfaceName, "healthy", "healthy", windowStart).
		Count(&flapCount)
	if flapCount <= int64(policy.AntiFlapping.MaxFlaps) {
		return false
	}

	// Over the flap limit -- but only actually IN cooldown if the most
	// recent flap was within CooldownHours of now (so cooldown expires
	// naturally rather than being permanent).
	cooldownStart := now.Add(-time.Duration(policy.AntiFlapping.CooldownHours) * time.Hour)
	var recentFlap model.TunnelHealthEvent
	err := s.db.Where("interface_name = ? AND from_status = ? AND to_status != ? AND detected_at >= ?", interfaceName, "healthy", "healthy", cooldownStart).
		Order("detected_at desc").First(&recentFlap).Error
	return err == nil
}

// logSkipOnce records (and notifies) a remediation gate being hit, but
// only the first time for a given incident -- guarded by
// lastActionSinceUnhealthy already returning nil, i.e. nothing (skip or
// real action) has been logged since the tunnel's last healthy->
// unhealthy transition. Uses level "0" so it never satisfies
// considerRemediation's own lastAction.Level=="1"/"2" escalation checks
// -- a logged skip must never be mistaken for an attempted Level 1.
func (s *TunnelHealthService) logSkipOnce(interfaceName string, now time.Time, reason string) {
	lastAction, err := s.lastActionSinceUnhealthy(interfaceName)
	if err != nil {
		s.logger.Error("failed to check prior remediation log before skip", zap.String("interface", interfaceName), zap.Error(err))
		return
	}
	if lastAction != nil {
		return
	}
	s.logAction(interfaceName, "0", fmt.Sprintf("اقدام درمانی برای %s انجام نشد", interfaceName), "-", true, reason, now)
}

// lastActionSinceUnhealthy returns the highest remediation level already
// attempted for this interface since its most recent healthy->unhealthy
// transition -- so considerRemediation only escalates (L1 then L2),
// never re-attempts L1 forever or jumps straight to L2 without trying L1
// first.
func (s *TunnelHealthService) lastActionSinceUnhealthy(interfaceName string) (*model.TunnelActionLog, error) {
	var lastHealthyEvent model.TunnelHealthEvent
	err := s.db.Where("interface_name = ? AND to_status = ?", interfaceName, "healthy").
		Order("detected_at desc").First(&lastHealthyEvent).Error
	sinceTime := time.Time{}
	if err == nil {
		sinceTime = lastHealthyEvent.DetectedAt
	}

	var action model.TunnelActionLog
	err = s.db.Where("interface_name = ? AND executed_at >= ?", interfaceName, sinceTime).
		Order("executed_at desc").First(&action).Error
	if err != nil {
		if errors.Is(err, gorm.ErrRecordNotFound) {
			return nil, nil
		}
		return nil, err
	}
	return &action, nil
}

// considerRemediation is the spec's decision-engine entry point for an
// unhealthy tunnel: BOOT_GRACE check, anti-flapping cooldown check, then
// escalate from whatever level was last attempted (or start at Level 1).
func (s *TunnelHealthService) considerRemediation(interfaceName string, policy *model.TunnelPolicy, now time.Time) {
	// Each of these three gates previously returned completely silently --
	// a confirmed, reported bug: the admin got the initial "tunnel became
	// suspect" alert and then nothing else ever, with no way to tell
	// whether the engine had decided not to act (and why) or simply
	// hadn't gotten to it yet. Each gate now logs its skip exactly ONCE
	// per incident (guarded by lastActionSinceUnhealthy already having no
	// entry yet), not on every 60s poll tick, so a tunnel stuck in e.g.
	// boot-grace for its full 15 minutes produces one explanatory message
	// instead of up to 15.
	if s.IsEmergencyStopped() {
		s.logSkipOnce(interfaceName, now, "توقف اضطراری فعال است؛ سیستم درمانی خودکار غیرفعال است")
		return
	}
	if s.isInBootGrace(now) {
		// Spec section ب-4 method 6: the router itself rebooted less than
		// 15 minutes ago -- observe only, no action, until services have
		// had time to come fully back up on their own.
		s.logSkipOnce(interfaceName, now, "روتر اخیراً ری‌استارت شده (کمتر از ۱۵ دقیقه)؛ منتظر پایداری سرویس‌ها هستیم")
		return
	}

	config := s.policyService.Parse(policy)

	if s.antiFlappingCooldownActive(interfaceName, config, now) {
		s.logSkipOnce(interfaceName, now, "این تانل اخیراً بارها قطع/وصل شده؛ در دوره‌ی خنک‌سازی ضد نوسان است")
		return
	}

	lastAction, err := s.lastActionSinceUnhealthy(interfaceName)
	if err != nil {
		s.logger.Error("failed to determine last remediation level", zap.String("interface", interfaceName), zap.Error(err))
		return
	}

	if lastAction == nil {
		s.attemptLevel1(interfaceName, config, now)
		return
	}

	// Level 1 was already attempted for this incident and the tunnel is
	// STILL unhealthy on this poll tick -- escalate to Level 2. Level 1
	// is never retried a second time within the same incident (the spec
	// itself only prescribes one L1 attempt with a wait, then escalate).
	if lastAction.Level == "1" {
		s.attemptLevel2(interfaceName, config, now)
		return
	}
	// Level 2 already ran and the tunnel is STILL unhealthy -- escalate
	// to Level 3 (build an entirely new tunnel + route). Only reachable
	// once per incident, same one-shot-per-level escalation discipline as
	// Level 1 -> Level 2 above.
	if lastAction.Level == "2" {
		s.attemptLevel3(interfaceName, config, now)
	}
	// lastAction.Level == "3": already built a new tunnel; nothing
	// further to escalate to.
}

// isInBootGrace implements spec section ب-4 method 6: reads RouterOS's
// own system-resource uptime (already exposed via FetchDeviceInfo, used
// elsewhere in the panel for the Reports section) -- if the router has
// been up for less than 15 minutes, the whole engine holds off on
// remediation so services have time to come back up on their own after
// a reboot before anything is toggled.
func (s *TunnelHealthService) isInBootGrace(now time.Time) bool {
	info, err := s.mikrotikAdaptor.FetchDeviceInfo(context.Background())
	if err != nil {
		// Uptime unreadable -- fail open toward caution: treat as boot
		// grace so a transient API error never causes an unintended
		// remediation attempt during what might actually be a reboot.
		return true
	}
	uptime, err := utils.ParseCustomDuration(info.Uptime)
	if err != nil {
		return true
	}
	return uptime < 15*time.Minute
}

// attemptLevel1 implements spec section ب-5 Level 1 ("احیای نرم"):
// disable then re-enable the interface. Works identically for GRE/IPIP/
// EoIP/WireGuard-based tunnels alike, since RouterOS's own
// /interface/set disabled=yes|no applies uniformly across interface
// types. In dry-run mode, this computes and logs the exact command that
// would be sent, then stops.
func (s *TunnelHealthService) attemptLevel1(interfaceName string, config TunnelPolicyConfig, now time.Time) {
	description := fmt.Sprintf("غیرفعال و مجدداً فعال کردن اینترفیس %s (سطح ۱)", interfaceName)
	detail := fmt.Sprintf("PATCH /interface/%s {disabled: true} سپس {disabled: false}", interfaceName)

	if s.redline.IsProtectedInterface(interfaceName) {
		s.logAction(interfaceName, "1", description, detail, true, "رد شد: این اینترفیس در «منطقه امن مدیریتی» قرار دارد", now)
		return
	}

	if s.IsDryRun() {
		s.logAction(interfaceName, "1", description, detail, true, "شبیه‌سازی شد (حالت آزمایشی فعال است)", now)
		return
	}

	ctx := context.Background()
	if err := s.mikrotikAdaptor.SetInterfaceDisabledByName(ctx, interfaceName, true); err != nil {
		s.logAction(interfaceName, "1", description, detail, false, "ناموفق: "+err.Error(), now)
		return
	}
	// Previously hardcoded to a fixed 2s regardless of the admin's own
	// configured wait_seconds -- a confirmed, reported bug (the policy
	// form's "Wait Seconds" field had no effect on the actual remediation
	// timing). A zero/unset value falls back to the same 2s this always
	// used, so an existing policy saved before this field was wired up
	// keeps behaving exactly as it did.
	wait := time.Duration(config.Level1.WaitSeconds) * time.Second
	if wait <= 0 {
		wait = 2 * time.Second
	}
	time.Sleep(wait)
	if err := s.mikrotikAdaptor.SetInterfaceDisabledByName(ctx, interfaceName, false); err != nil {
		s.logAction(interfaceName, "1", description, detail, false, "ناموفق در فعال‌سازی مجدد: "+err.Error(), now)
		return
	}
	s.logAction(interfaceName, "1", description, detail, false, "موفق", now)
}

// currentGatewayIP fetches the live routing table and returns the first
// resolved next-hop IP (the "<ip>" half of ImmediateGw's own "<ip>%
// <interface>" pair) for a route currently going out interfaceName -- the
// same live signal attemptLevel2 itself relies on, but called from
// pollOneTunnel on a HEALTHY tick specifically to populate
// TunnelHealthStatus.LastKnownGatewayIP before the tunnel can ever go
// down and make this same live lookup return nothing. Returns "" on any
// fetch error or when no matching route/gateway IP is found; the caller
// treats that as "nothing to cache this tick," never as a reason to fail
// the whole poll.
func (s *TunnelHealthService) currentGatewayIP(interfaceName string) string {
	routes, err := s.mikrotikAdaptor.FetchIPRoutes(context.Background())
	if err != nil {
		return ""
	}
	for _, route := range routes {
		if route.Disabled != nil && *route.Disabled == "true" {
			continue
		}
		if route.Connect != nil && *route.Connect == "true" {
			continue
		}
		if routeGatewayInterface(route) != interfaceName {
			continue
		}
		if route.ImmediateGw != nil && *route.ImmediateGw != "" {
			if idx := strings.LastIndex(*route.ImmediateGw, "%"); idx != -1 {
				return (*route.ImmediateGw)[:idx]
			}
		}
	}
	return ""
}

// attemptLevel2 implements spec section ب-5 Level 2 ("سوییچ بکاپ"): for
// every route currently going out the down interface (per
// routeGatewayInterface's own real-topology resolution -- routes on this
// fleet reach a tunnel through a next-hop IP, not the bare interface
// name, see mikrotik.RouteEntry.ImmediateGw's own doc comment), PATCH
// just that route's own gateway field to the admin's own manually-entered
// BackupGatewayIP (see TunnelPolicyConfig.Level2's own doc comment on why
// this can't be derived automatically). Every other property of each
// route (dst-address, routing-table, distance) is left untouched -- this
// only ever changes WHERE already-matched traffic goes next, never WHAT
// traffic a route matches. Requires BOTH BackupTarget (for the log
// message/audit trail) and BackupGatewayIP (the actual value written) to
// be configured; missing either keeps this simulation-only, exactly like
// an unconfigured Level 2 always has.
func (s *TunnelHealthService) attemptLevel2(interfaceName string, config TunnelPolicyConfig, now time.Time) {
	target := config.Level2.BackupTarget
	if target == "" {
		s.logAction(interfaceName, "2", fmt.Sprintf("سطح ۲ برای %s تعریف نشده است", interfaceName), "-", true, "رد شد: backup_target در سیاست این تانل تنظیم نشده", now)
		return
	}

	description := fmt.Sprintf("انتقال ترافیک از %s به تانل بکاپ %s (سطح ۲)", interfaceName, target)

	gatewayIP := strings.TrimSpace(config.Level2.BackupGatewayIP)
	if gatewayIP == "" {
		detail := fmt.Sprintf("جابجایی gateway در routing-table مرتبط با %s به سمت %s", interfaceName, target)
		s.logAction(interfaceName, "2", description, detail, true, "رد شد: آی‌پی واسطه‌ی تانل بکاپ (backup_gateway_ip) در سیاست این تانل تنظیم نشده", now)
		return
	}

	if s.redline.IsProtectedInterface(interfaceName) {
		s.logAction(interfaceName, "2", description, "-", true, "رد شد: این اینترفیس در «منطقه امن مدیریتی» قرار دارد", now)
		return
	}

	ctx := context.Background()
	routes, err := s.mikrotikAdaptor.FetchIPRoutes(ctx)
	if err != nil {
		s.logAction(interfaceName, "2", description, "-", false, "ناموفق: خواندن جدول روتینگ ممکن نشد: "+err.Error(), now)
		return
	}

	// lastKnownGatewayIP is the confirmed, reported fallback fix: RouterOS
	// blanks a route's own ImmediateGw the instant its gateway interface
	// actually goes down (nothing left to resolve), which makes
	// routeGatewayInterface fall back to the route's bare Gateway field
	// -- just the next-hop IP with no "%interface" suffix -- so it can
	// never match interfaceName by name alone during a real outage. This
	// was confirmed directly against a live incident: attemptLevel2 found
	// zero matching routes for a genuinely-down tunnel with an existing,
	// correctly-configured traffic route, purely because ImmediateGw had
	// already gone blank by the time this ran. See
	// TunnelHealthStatus.LastKnownGatewayIP's own doc comment for where
	// this cached value is populated (only ever written on a HEALTHY
	// poll tick, so it always holds the last resolution seen before this
	// exact failure mode could occur).
	var lastKnownGatewayIP string
	var status model.TunnelHealthStatus
	if err := s.db.Where("interface_name = ?", interfaceName).First(&status).Error; err == nil {
		lastKnownGatewayIP = status.LastKnownGatewayIP
	}

	var matchingRouteIDs []string
	for _, route := range routes {
		if route.Disabled != nil && *route.Disabled == "true" {
			continue
		}
		if route.Connect != nil && *route.Connect == "true" {
			continue
		}
		if routeGatewayInterface(route) == interfaceName {
			matchingRouteIDs = append(matchingRouteIDs, route.ID)
			continue
		}
		if lastKnownGatewayIP != "" && route.Gateway != nil && *route.Gateway == lastKnownGatewayIP {
			matchingRouteIDs = append(matchingRouteIDs, route.ID)
		}
	}

	if len(matchingRouteIDs) == 0 {
		s.logAction(interfaceName, "2", description, "-", true, "هیچ مسیر فعالی در جدول روتینگ از طریق این اینترفیس یافت نشد؛ اقدامی لازم نبود", now)
		return
	}

	detail := fmt.Sprintf("PATCH gateway=%s روی %d مسیر که از طریق %s عبور می‌کردند", gatewayIP, len(matchingRouteIDs), interfaceName)

	if s.IsDryRun() {
		s.logAction(interfaceName, "2", description, detail, true, "شبیه‌سازی شد (حالت آزمایشی فعال است)", now)
		return
	}

	var failedCount int
	for _, routeID := range matchingRouteIDs {
		if err := s.mikrotikAdaptor.SetRouteGateway(ctx, routeID, gatewayIP); err != nil {
			s.logger.Error("failed to switch route gateway during level 2 failover",
				zap.String("interface", interfaceName), zap.String("route_id", routeID), zap.Error(err))
			failedCount++
		}
	}

	if failedCount == len(matchingRouteIDs) {
		s.logAction(interfaceName, "2", description, detail, false, fmt.Sprintf("ناموفق: هیچ‌کدام از %d مسیر جابجا نشد", len(matchingRouteIDs)), now)
		return
	}
	if failedCount > 0 {
		s.logAction(interfaceName, "2", description, detail, false, fmt.Sprintf("جزئی: %d از %d مسیر جابجا شد، %d مورد ناموفق", len(matchingRouteIDs)-failedCount, len(matchingRouteIDs), failedCount), now)
		return
	}
	s.logAction(interfaceName, "2", description, detail, false, fmt.Sprintf("موفق: %d مسیر به تانل بکاپ منتقل شد", len(matchingRouteIDs)), now)
}

// attemptLevel3 implements spec section ب-5 Level 3 ("ساخت مسیر جدید"):
// creates a brand-new GRE/IPIP/EoIP tunnel interface per the admin's own
// policy config, then a fresh route for each destination network the
// down interface was previously serving, pointing at the new tunnel's
// own GatewayIP. WireGuard is deliberately NOT supported as a Level3.Type
// here -- unlike GRE/IPIP/EoIP (true point-to-point tunnels needing only
// remote-address/local-address), a usable WireGuard interface also needs
// at least one peer configured with its own public key/allowed-IPs,
// which this policy shape has no field for and which the engine cannot
// safely invent on its own (the far end's public key is not something
// this router can discover automatically) -- attempting it would create
// an interface that can never actually pass traffic. The existing routes
// through the down interface are deliberately left untouched (unlike
// Level 2's own in-place gateway switch) -- Level 3 builds NEW
// infrastructure alongside the broken tunnel for the admin to review,
// rather than modifying already-broken routing state further.
func (s *TunnelHealthService) attemptLevel3(interfaceName string, config TunnelPolicyConfig, now time.Time) {
	if !config.Level3.Enabled {
		s.logAction(interfaceName, "3", fmt.Sprintf("سطح ۳ برای %s فعال نشده است", interfaceName), "-", true, "رد شد: سطح ۳ در سیاست این تانل غیرفعال است", now)
		return
	}

	tunnelType := strings.ToLower(strings.TrimSpace(config.Level3.Type))
	if tunnelType != "gre" && tunnelType != "ipip" && tunnelType != "eoip" {
		s.logAction(interfaceName, "3", fmt.Sprintf("سطح ۳ برای %s", interfaceName), "-", true, fmt.Sprintf("رد شد: نوع تانل %q پشتیبانی نمی‌شود (فقط gre/ipip/eoip)", config.Level3.Type), now)
		return
	}

	remoteAddr := strings.TrimSpace(config.Level3.RemoteAddress)
	gatewayIP := strings.TrimSpace(config.Level3.GatewayIP)
	newName := strings.TrimSpace(config.Level3.NewInterfaceName)
	if remoteAddr == "" || gatewayIP == "" || newName == "" {
		s.logAction(interfaceName, "3", fmt.Sprintf("سطح ۳ برای %s", interfaceName), "-", true, "رد شد: remote_address، gateway_ip یا new_interface_name در سیاست تنظیم نشده", now)
		return
	}

	description := fmt.Sprintf("ساخت تانل جدید %s (%s) برای جایگزینی %s (سطح ۳)", newName, tunnelType, interfaceName)

	if s.redline.IsProtectedInterface(interfaceName) {
		s.logAction(interfaceName, "3", description, "-", true, "رد شد: این اینترفیس در «منطقه امن مدیریتی» قرار دارد", now)
		return
	}

	ctx := context.Background()
	routes, err := s.mikrotikAdaptor.FetchIPRoutes(ctx)
	if err != nil {
		s.logAction(interfaceName, "3", description, "-", false, "ناموفق: خواندن جدول روتینگ ممکن نشد: "+err.Error(), now)
		return
	}
	var destinations []string
	for _, route := range routes {
		if route.Disabled != nil && *route.Disabled == "true" {
			continue
		}
		if route.Connect != nil && *route.Connect == "true" {
			continue
		}
		if routeGatewayInterface(route) == interfaceName && route.DstAddress != "" {
			destinations = append(destinations, route.DstAddress)
		}
	}

	var localAddr *string
	if la := strings.TrimSpace(config.Level3.LocalAddress); la != "" && la != "auto" {
		localAddr = &la
	}

	detail := fmt.Sprintf("PUT /interface/%s {name:%s, remote-address:%s} سپس ایجاد مسیر برای %d مقصد به سمت %s", tunnelType, newName, remoteAddr, len(destinations), gatewayIP)

	if s.IsDryRun() {
		s.logAction(interfaceName, "3", description, detail, true, "شبیه‌سازی شد (حالت آزمایشی فعال است)", now)
		return
	}

	var createErr error
	switch tunnelType {
	case "gre":
		_, createErr = s.mikrotikAdaptor.CreateGREInterface(ctx, newName, remoteAddr, localAddr)
	case "ipip":
		_, createErr = s.mikrotikAdaptor.CreateIPIPInterface(ctx, newName, remoteAddr, localAddr)
	case "eoip":
		_, createErr = s.mikrotikAdaptor.CreateEoIPInterface(ctx, newName, remoteAddr, localAddr)
	}
	if createErr != nil {
		s.logAction(interfaceName, "3", description, detail, false, "ناموفق: ساخت اینترفیس تانل جدید: "+createErr.Error(), now)
		return
	}

	if len(destinations) == 0 {
		s.logAction(interfaceName, "3", description, detail, false, "موفق: اینترفیس تانل جدید ساخته شد؛ هیچ مسیر فعالی برای این اینترفیس یافت نشد که مسیر جدیدی برایش ساخته شود", now)
		return
	}

	var failedRoutes int
	for _, dst := range destinations {
		if _, err := s.mikrotikAdaptor.CreateRoute(ctx, dst, gatewayIP); err != nil {
			s.logger.Error("failed to create route during level 3 new-tunnel provisioning",
				zap.String("interface", interfaceName), zap.String("destination", dst), zap.Error(err))
			failedRoutes++
		}
	}

	if failedRoutes == len(destinations) {
		s.logAction(interfaceName, "3", description, detail, false, fmt.Sprintf("جزئی: اینترفیس ساخته شد اما هیچ‌کدام از %d مسیر ایجاد نشد", len(destinations)), now)
		return
	}
	if failedRoutes > 0 {
		s.logAction(interfaceName, "3", description, detail, false, fmt.Sprintf("جزئی: اینترفیس و %d از %d مسیر ساخته شد، %d مورد ناموفق", len(destinations)-failedRoutes, len(destinations), failedRoutes), now)
		return
	}
	s.logAction(interfaceName, "3", description, detail, false, fmt.Sprintf("موفق: تانل جدید %s و %d مسیر ساخته شد", newName, len(destinations)), now)
}

// considerFallbackRecovery implements the spec's "بازگشت خودکار": once a
// tunnel that had an active remediation returns to healthy, log a
// recovery note so the action history shows the incident's full
// lifecycle. Level 1's own recovery needs no further action (toggling
// the interface back on was the whole remediation). Level 2, now that it
// actually moves routes onto the backup tunnel's gateway (see
// attemptLevel2), is deliberately NOT auto-reverted here: doing so
// safely would require precisely reconstructing every affected route's
// exact original gateway value (including a recursive next-hop IP, not
// just an interface name), and a bug in that reconstruction risks
// leaving live routes worse off than simply staying on the backup tunnel
// until an admin reviews and reverts by hand. The log entry below makes
// this explicit so the admin knows a manual check is expected, rather
// than assuming traffic silently moved back on its own.
func (s *TunnelHealthService) considerFallbackRecovery(interfaceName string, now time.Time) {
	lastAction, err := s.lastActionSinceUnhealthy(interfaceName)
	if err != nil || lastAction == nil {
		return
	}
	if lastAction.Level == "2" {
		s.logAction(interfaceName, lastAction.Level, fmt.Sprintf("بازگشت %s به حالت سالم", interfaceName), "-", true, "بازیابی شد؛ توجه: مسیرها همچنان روی تانل بکاپ هستند و باید در صورت نیاز دستی بازگردانده شوند", now)
		return
	}
	s.logAction(interfaceName, lastAction.Level, fmt.Sprintf("بازگشت %s به حالت سالم", interfaceName), "-", true, "بازیابی شد؛ اقدام درمانی دیگری لازم نیست", now)
}

// ---------------------------------------------------------------------
// Active backup-path probe (spec-adjacent capability, run on its own
// nightly schedule -- see ProbeBackupPaths's own doc comment for why
// this is separate from Poll's own 60s cycle). Confirms a tunnel's
// configured Level 2 backup gateway is ACTUALLY reachable before a real
// incident ever needs it, rather than discovering a dead backup only
// during a live double-failure.
// ---------------------------------------------------------------------

const (
	// backupProbeRetention keeps a short rolling history per interface
	// (same rolling-window convention as TunnelHealthSample/
	// TunnelHealthScore) purely so the panel can show "last few nights"
	// rather than only the very latest result.
	backupProbeRetention = 14
	backupProbePingCount = 2
)

// ProbeBackupPaths is the scheduled job entry point for the nightly
// active backup-path check -- run on its OWN schedule (once daily, see
// this project's cmd/main.go wiring) rather than on Poll's 60s cycle,
// since pinging every configured backup gateway on every single poll
// tick would be wasted load for a value (confidence that a rarely-used
// standby path still works) that does not need minute-by-minute
// freshness. Iterates every tunnel that currently has a policy with a
// non-empty Level2.BackupGatewayIP configured -- a tunnel with no Level 2
// configured yet has nothing to probe and is silently skipped.
func (s *TunnelHealthService) ProbeBackupPaths() {
	policies, err := s.policyService.List()
	if err != nil {
		s.logger.Error("failed to list tunnel policies for backup probe", zap.Error(err))
		return
	}

	ctx := context.Background()
	now := time.Now()
	for _, policy := range policies {
		config := s.policyService.Parse(&policy)
		gatewayIP := strings.TrimSpace(config.Level2.BackupGatewayIP)
		if gatewayIP == "" {
			continue
		}
		s.probeOneBackupPath(ctx, policy.InterfaceName, gatewayIP, now)
	}
}

func (s *TunnelHealthService) probeOneBackupPath(ctx context.Context, interfaceName, gatewayIP string, now time.Time) {
	var reachable bool
	var evidence string
	results, err := s.mikrotikAdaptor.Ping(ctx, gatewayIP, backupProbePingCount)
	if err != nil {
		evidence = "تست پینگ ناموفق بود: " + err.Error()
	} else {
		for _, r := range results {
			if r.Time != "" {
				reachable = true
				break
			}
		}
		if reachable {
			evidence = fmt.Sprintf("%s به پینگ پاسخ داد", gatewayIP)
		} else {
			evidence = fmt.Sprintf("%s به هیچ‌کدام از %d پینگ پاسخ نداد", gatewayIP, backupProbePingCount)
		}
	}

	probe := model.TunnelBackupProbeResult{
		InterfaceName:   interfaceName,
		BackupGatewayIP: gatewayIP,
		Reachable:       reachable,
		Evidence:        evidence,
		ProbedAt:        now,
	}
	if err := s.db.Create(&probe).Error; err != nil {
		s.logger.Error("failed to record backup probe result", zap.String("interface", interfaceName), zap.Error(err))
		return
	}

	var toDelete []uint
	s.db.Model(&model.TunnelBackupProbeResult{}).
		Where("interface_name = ?", interfaceName).
		Order("probed_at desc").
		Offset(backupProbeRetention).
		Pluck("id", &toDelete)
	if len(toDelete) > 0 {
		s.db.Where("id IN ?", toDelete).Delete(&model.TunnelBackupProbeResult{})
	}

	if reachable {
		return
	}
	if s.botNotifier == nil || s.settings == nil {
		return
	}
	settings, err := s.settings.GetOrCreate()
	if err != nil || settings.AdminChatID == "" {
		return
	}
	s.botNotifier.NotifyBackupPathUnreachable(settings, interfaceName, gatewayIP, evidence, now)
}

// ListBackupProbeResults returns the most recent nightly backup-probe
// results (newest first) -- the panel's own read endpoint for this
// capability.
func (s *TunnelHealthService) ListBackupProbeResults(limit int) ([]model.TunnelBackupProbeResult, error) {
	if limit <= 0 || limit > 500 {
		limit = 100
	}
	var results []model.TunnelBackupProbeResult
	if err := s.db.Order("probed_at desc").Limit(limit).Find(&results).Error; err != nil {
		s.logger.Error("failed to list backup probe results", zap.Error(err))
		return nil, err
	}
	return results, nil
}

// ListIncidentDiagnoses returns the most recent root-cause diagnoses
// (newest first) -- the panel's own read endpoint for
// diagnoseIncidentOnce's output.
func (s *TunnelHealthService) ListIncidentDiagnoses(limit int) ([]model.TunnelIncidentDiagnosis, error) {
	if limit <= 0 || limit > 500 {
		limit = 100
	}
	var diagnoses []model.TunnelIncidentDiagnosis
	if err := s.db.Order("diagnosed_at desc").Limit(limit).Find(&diagnoses).Error; err != nil {
		s.logger.Error("failed to list tunnel incident diagnoses", zap.Error(err))
		return nil, err
	}
	return diagnoses, nil
}

// ListHealthScoreHistory returns interfaceName's own recent health-score
// samples (oldest first, so the panel can plot them directly as a trend
// line without having to reverse the slice itself).
func (s *TunnelHealthService) ListHealthScoreHistory(interfaceName string, limit int) ([]model.TunnelHealthScore, error) {
	if limit <= 0 || limit > 500 {
		limit = healthScoreHistoryRetention
	}
	var scores []model.TunnelHealthScore
	if err := s.db.Where("interface_name = ?", interfaceName).
		Order("sampled_at desc").Limit(limit).Find(&scores).Error; err != nil {
		s.logger.Error("failed to list tunnel health score history", zap.String("interface", interfaceName), zap.Error(err))
		return nil, err
	}
	for i, j := 0, len(scores)-1; i < j; i, j = i+1, j-1 {
		scores[i], scores[j] = scores[j], scores[i]
	}
	return scores, nil
}

// ---------------------------------------------------------------------
// Historical incident-pattern report (weekly digest, its own schedule --
// see cmd/main.go wiring). Everything above reacts to ONE tunnel's OWN
// current/recent state; this instead looks across ALL tunnels' full
// TunnelHealthEvent history to surface patterns no single incident alert
// would ever reveal: a chronically-flapping tunnel, or two tunnels that
// keep going down together (a strong hint they share an underlying
// cause -- e.g. the same upstream link -- even though the engine treats
// them as fully independent interfaces).
// ---------------------------------------------------------------------

const (
	// incidentPatternLookback is how far back the weekly report looks --
	// matches its own weekly cadence (see cmd/main.go) with a little
	// slack so a report that's a few hours late for any reason still
	// covers the full intended week.
	incidentPatternLookback = 8 * 24 * time.Hour
	// incidentPatternMinFlaps is the minimum healthy->unhealthy
	// transition count within the lookback window for a tunnel to be
	// called out as "chronically flapping" in the report -- below this,
	// one or two incidents in a week is normal operational noise, not a
	// pattern worth a dedicated callout.
	incidentPatternMinFlaps = 3
	// incidentPatternCorrelationWindow is how close together two
	// DIFFERENT tunnels' own healthy->unhealthy transitions must land to
	// be called out as "failing together" -- short enough that it's
	// implausible to be coincidence, long enough to allow for Poll's own
	// 60s cadence meaning two genuinely-simultaneous failures may be
	// detected a tick or two apart.
	incidentPatternCorrelationWindow = 5 * time.Minute
	// incidentPatternMinCorrelatedCount is the minimum number of times
	// two tunnels must have failed together within the window above
	// before being called out -- one shared coincidence proves nothing;
	// a repeated pattern across the week does.
	incidentPatternMinCorrelatedCount = 2
)

// BuildIncidentPatternReport queries TunnelHealthEvent for the lookback
// window and returns a ready-to-send Persian report body, or "" if
// nothing in the window meets either pattern's own minimum threshold --
// callers should simply not send a notification at all in that case (see
// NotifyTunnelIncidentPatternReport's own doc comment), rather than
// spamming an admin with an empty "nothing interesting happened" message
// every single week.
func (s *TunnelHealthService) BuildIncidentPatternReport(now time.Time) string {
	since := now.Add(-incidentPatternLookback)
	var events []model.TunnelHealthEvent
	if err := s.db.Where("from_status = ? AND to_status != ? AND to_status != ? AND detected_at >= ?",
		"healthy", "healthy", "degrading_warning", since).
		Order("detected_at asc").Find(&events).Error; err != nil {
		s.logger.Error("failed to load tunnel health events for pattern report", zap.Error(err))
		return ""
	}
	if len(events) == 0 {
		return ""
	}

	flapCounts := make(map[string]int)
	for _, e := range events {
		flapCounts[e.InterfaceName]++
	}

	type flapReport struct {
		name  string
		count int
	}
	var chronicFlappers []flapReport
	for name, count := range flapCounts {
		if count >= incidentPatternMinFlaps {
			chronicFlappers = append(chronicFlappers, flapReport{name, count})
		}
	}
	sort.Slice(chronicFlappers, func(i, j int) bool { return chronicFlappers[i].count > chronicFlappers[j].count })

	// Correlation: for every pair of DIFFERENT interfaces, count how many
	// times their own transitions landed within incidentPatternCorrelationWindow
	// of each other. O(n^2) over one week's worth of incidents, which for
	// any real fleet's actual incident volume is a tiny list -- this is a
	// once-a-week background job, not a hot path.
	type pairKey struct{ a, b string }
	pairCounts := make(map[pairKey]int)
	for i := 0; i < len(events); i++ {
		for j := i + 1; j < len(events); j++ {
			if events[i].InterfaceName == events[j].InterfaceName {
				continue
			}
			delta := events[j].DetectedAt.Sub(events[i].DetectedAt)
			if delta < 0 {
				delta = -delta
			}
			if delta > incidentPatternCorrelationWindow {
				continue
			}
			a, b := events[i].InterfaceName, events[j].InterfaceName
			if a > b {
				a, b = b, a
			}
			pairCounts[pairKey{a, b}]++
		}
	}
	type correlationReport struct {
		a, b  string
		count int
	}
	var correlated []correlationReport
	for pair, count := range pairCounts {
		if count >= incidentPatternMinCorrelatedCount {
			correlated = append(correlated, correlationReport{pair.a, pair.b, count})
		}
	}
	sort.Slice(correlated, func(i, j int) bool { return correlated[i].count > correlated[j].count })

	if len(chronicFlappers) == 0 && len(correlated) == 0 {
		return ""
	}

	var b strings.Builder
	if len(chronicFlappers) > 0 {
		b.WriteString("🔁 <b>تانل‌های پرنوسان این هفته:</b>\n")
		for _, f := range chronicFlappers {
			b.WriteString(fmt.Sprintf("• <code>%s</code>: %d بار قطع/مشکوک شد\n", f.name, f.count))
		}
	}
	if len(correlated) > 0 {
		if b.Len() > 0 {
			b.WriteString("\n")
		}
		b.WriteString("🔗 <b>تانل‌هایی که معمولاً با هم قطع می‌شوند:</b>\n")
		for _, c := range correlated {
			b.WriteString(fmt.Sprintf("• <code>%s</code> و <code>%s</code>: %d بار در بازه‌ی کوتاه با هم قطع شدند\n", c.a, c.b, c.count))
		}
	}
	return b.String()
}

// SendIncidentPatternReport is the scheduled job entry point for the
// weekly digest -- builds the report and only actually notifies the
// admin when there is something worth reporting.
func (s *TunnelHealthService) SendIncidentPatternReport() {
	if s.botNotifier == nil || s.settings == nil {
		return
	}
	settings, err := s.settings.GetOrCreate()
	if err != nil || settings.AdminChatID == "" {
		return
	}
	now := time.Now()
	report := s.BuildIncidentPatternReport(now)
	if report == "" {
		return
	}
	s.botNotifier.NotifyTunnelIncidentPatternReport(settings, report, now)
}

// logAction is the single write path for TunnelActionLog -- every
// remediation decision, simulated or real, goes through this so the
// dry-run/live distinction is always recorded consistently.
func (s *TunnelHealthService) logAction(interfaceName, level, description, detail string, simulated bool, result string, now time.Time) {
	action := model.TunnelActionLog{
		InterfaceName:      interfaceName,
		Level:              level,
		CommandDescription: description,
		CommandDetail:      detail,
		Simulated:          simulated,
		Result:             result,
		ExecutedAt:         now,
	}
	if err := s.db.Create(&action).Error; err != nil {
		s.logger.Error("failed to record tunnel action log", zap.String("interface", interfaceName), zap.Error(err))
	}

	// A confirmed, reported gap: the admin only ever got told a tunnel
	// WENT bad, never what the panel did about it or whether that action
	// worked -- see NotifyTunnelActionTaken's own doc comment for why
	// this is safe to send on every logAction call without spam risk.
	if s.botNotifier == nil || s.settings == nil {
		return
	}
	settings, err := s.settings.GetOrCreate()
	if err != nil || settings.AdminChatID == "" {
		return
	}
	s.botNotifier.NotifyTunnelActionTaken(settings, interfaceName, level, description, simulated, result, now)
}
