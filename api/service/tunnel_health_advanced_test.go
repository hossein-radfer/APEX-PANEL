package service

import (
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"sync"
	"testing"
	"time"

	"github.com/maahdima/mwp/api/adaptor/mikrotik"
	"github.com/maahdima/mwp/api/dataservice/model"
)

// TestComputeHealthScore_HealthyIsPerfect confirms a fully healthy
// tunnel (running, not disabled, no stale handshake, no asymmetry)
// always scores exactly 100 -- the ceiling the early-warning detector's
// own drop-from-baseline math depends on.
func TestComputeHealthScore_HealthyIsPerfect(t *testing.T) {
	if got := computeHealthScore(true, false, nil, false); got != 100 {
		t.Errorf("expected 100 for a fully healthy tunnel, got %d", got)
	}
}

// TestComputeHealthScore_DisabledScoresLowerThanNotRunning confirms the
// score reflects severity of signal, not just presence -- a disabled
// interface (an explicit admin/engine action) is scored worse than one
// that merely isn't running yet (which could be transient).
func TestComputeHealthScore_DisabledScoresLowerThanNotRunning(t *testing.T) {
	disabledScore := computeHealthScore(false, true, nil, false)
	notRunningScore := computeHealthScore(false, false, nil, false)
	if disabledScore >= notRunningScore {
		t.Errorf("expected disabled score (%d) to be lower than not-running score (%d)", disabledScore, notRunningScore)
	}
}

// TestComputeHealthScore_HandshakeAgeDegradesGradually confirms a
// worsening (but not yet stale-threshold-crossing) handshake age
// produces a WORSE score than a fresh one, even while both are still
// "healthy" per computeSeverity's own binary threshold -- this gradual
// degradation before the binary threshold trips is the entire point of
// the early-warning capability.
func TestComputeHealthScore_HandshakeAgeDegradesGradually(t *testing.T) {
	fresh := 5
	aging := int(handshakeStaleThreshold.Seconds()) - 10 // just under the "suspect" threshold
	freshScore := computeHealthScore(true, false, &fresh, false)
	agingScore := computeHealthScore(true, false, &aging, false)
	if agingScore >= freshScore {
		t.Errorf("expected an aging (but not yet stale) handshake to score worse than a fresh one: fresh=%d aging=%d", freshScore, agingScore)
	}
	if agingScore <= 0 {
		t.Errorf("expected an aging-but-not-yet-stale handshake to still score above 0, got %d", agingScore)
	}
}

// TestComputeHealthScore_AsymmetryScoresWorstOfAll confirms tx/rx
// asymmetry (the strongest single down-signal per this file's own
// detection doc comments) drags the score down more than a stale
// handshake alone.
func TestComputeHealthScore_AsymmetryScoresWorstOfAll(t *testing.T) {
	stale := int(handshakeStaleThreshold.Seconds()) + 100
	staleScore := computeHealthScore(true, false, &stale, false)
	asymmetryScore := computeHealthScore(true, false, nil, true)
	if asymmetryScore >= staleScore {
		t.Errorf("expected asymmetry-only score (%d) to be worse than stale-handshake-only score (%d)", asymmetryScore, staleScore)
	}
}

// TestRecordHealthScoreAndWarn_FiresEarlyWarningOnSustainedDrop builds up
// a run of healthy-but-degrading samples (fresh handshake ages getting
// steadily worse each tick, all still under the suspect threshold so
// severity itself never changes) and confirms an early-warning
// TunnelHealthEvent (to_status = "degrading_warning") gets recorded once
// the drop crosses healthScoreDropThreshold -- the core regression test
// for the whole early-warning capability.
func TestRecordHealthScoreAndWarn_FiresEarlyWarningOnSustainedDrop(t *testing.T) {
	db := openTunnelHealthTestDB(t)
	fake := newFakeRouterOS(nil)
	srv := fake.server()
	defer srv.Close()
	svc := newTestTunnelHealthService(t, db, srv)

	const iface = "gre-TR"
	base := time.Now()

	// Tick 0: perfectly fresh handshake -- score 100.
	fresh := 1
	svc.recordHealthScoreAndWarn(iface, true, false, &fresh, false, "healthy", base)

	// Ticks 1..healthScoreWarnLookback: handshake age climbs toward (but
	// stays under) the stale threshold, dragging the score down well past
	// healthScoreDropThreshold by the final tick.
	staleSeconds := int(handshakeStaleThreshold.Seconds())
	for i := 1; i <= healthScoreWarnLookback; i++ {
		age := (staleSeconds * i) / (healthScoreWarnLookback + 1)
		svc.recordHealthScoreAndWarn(iface, true, false, &age, false, "healthy", base.Add(time.Duration(i)*time.Minute))
	}

	var warning model.TunnelHealthEvent
	err := db.Where("interface_name = ? AND to_status = ?", iface, "degrading_warning").First(&warning).Error
	if err != nil {
		t.Fatalf("expected an early-warning event to be recorded, got error: %v", err)
	}
}

// TestRecordHealthScoreAndWarn_NoWarningWhenScoreStaysFlat confirms a
// tunnel that stays consistently healthy across every sample never gets
// a spurious early warning -- the counterpart to the test above.
func TestRecordHealthScoreAndWarn_NoWarningWhenScoreStaysFlat(t *testing.T) {
	db := openTunnelHealthTestDB(t)
	fake := newFakeRouterOS(nil)
	srv := fake.server()
	defer srv.Close()
	svc := newTestTunnelHealthService(t, db, srv)

	const iface = "gre-TR"
	base := time.Now()
	fresh := 1
	for i := 0; i <= healthScoreWarnLookback; i++ {
		svc.recordHealthScoreAndWarn(iface, true, false, &fresh, false, "healthy", base.Add(time.Duration(i)*time.Minute))
	}

	var count int64
	db.Model(&model.TunnelHealthEvent{}).Where("interface_name = ? AND to_status = ?", iface, "degrading_warning").Count(&count)
	if count != 0 {
		t.Errorf("expected no early-warning event for a consistently healthy tunnel, got %d", count)
	}
}

// TestRecordHealthScoreAndWarn_SkipsWhenAlreadyUnhealthy confirms the
// early-warning path is a no-op once severity itself is no longer
// "healthy" -- recordTransition's own alert already covers that case, so
// firing both would be redundant noise (see recordHealthScoreAndWarn's
// own doc comment).
func TestRecordHealthScoreAndWarn_SkipsWhenAlreadyUnhealthy(t *testing.T) {
	db := openTunnelHealthTestDB(t)
	fake := newFakeRouterOS(nil)
	srv := fake.server()
	defer srv.Close()
	svc := newTestTunnelHealthService(t, db, srv)

	const iface = "gre-TR"
	base := time.Now()
	fresh := 1
	svc.recordHealthScoreAndWarn(iface, true, false, &fresh, false, "healthy", base)
	stale := int(handshakeStaleThreshold.Seconds()) + 100
	for i := 1; i <= healthScoreWarnLookback; i++ {
		svc.recordHealthScoreAndWarn(iface, true, false, &stale, false, "suspect", base.Add(time.Duration(i)*time.Minute))
	}

	var count int64
	db.Model(&model.TunnelHealthEvent{}).Where("interface_name = ? AND to_status = ?", iface, "degrading_warning").Count(&count)
	if count != 0 {
		t.Errorf("expected no early-warning event once severity is already non-healthy, got %d", count)
	}
}

// ---------------------------------------------------------------------
// Root-cause diagnosis tests -- extend the fake RouterOS server with a
// /rest/ping handler.
// ---------------------------------------------------------------------

// fakeRouterOSWithPing wraps fakeRouterOS's own route-listing behavior
// and adds a /rest/ping handler, since diagnoseIncidentOnce/
// ProbeBackupPaths are the only callers in this package that need a
// working ping endpoint -- kept as a separate small type rather than
// bloating fakeRouterOS itself, matching this file's own established
// pattern of one small purpose-built fake per test concern (see
// fakeRouterOSv3 in tunnel_health_level3_test.go for precedent).
type fakeRouterOSWithPing struct {
	mu             sync.Mutex
	routes         []mikrotik.RouteEntry
	deviceInfo     mikrotik.SystemInfo
	reachableHosts map[string]bool // address -> whether /rest/ping should report a reply
}

func newFakeRouterOSWithPing(routes []mikrotik.RouteEntry) *fakeRouterOSWithPing {
	return &fakeRouterOSWithPing{
		routes:         routes,
		reachableHosts: make(map[string]bool),
		deviceInfo: mikrotik.SystemInfo{
			CPULoad:     "10",
			FreeMemory:  "500000000",
			TotalMemory: "1000000000",
		},
	}
}

func (f *fakeRouterOSWithPing) server() *httptest.Server {
	return httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		f.mu.Lock()
		defer f.mu.Unlock()

		switch {
		case r.Method == http.MethodGet && r.URL.Path == "/rest/ip/route":
			w.Header().Set("Content-Type", "application/json")
			_ = json.NewEncoder(w).Encode(f.routes)
		case r.Method == http.MethodGet && r.URL.Path == "/rest/system/resource":
			w.Header().Set("Content-Type", "application/json")
			_ = json.NewEncoder(w).Encode(f.deviceInfo)
		case r.Method == http.MethodPost && r.URL.Path == "/rest/ping":
			var body struct {
				Address string `json:"address"`
			}
			_ = json.NewDecoder(r.Body).Decode(&body)
			w.Header().Set("Content-Type", "application/json")
			if f.reachableHosts[body.Address] {
				_ = json.NewEncoder(w).Encode([]mikrotik.PingResult{{Host: body.Address, Time: "1ms", TTL: "64"}})
			} else {
				_ = json.NewEncoder(w).Encode([]mikrotik.PingResult{{Host: body.Address, Timeout: "1"}})
			}
		default:
			w.WriteHeader(http.StatusNotFound)
		}
	}))
}

// TestDiagnoseIncidentOnce_RemoteUnreachableWhenOnlyThisTunnelFails sets
// up one down tunnel (gre-TR, gateway unreachable) alongside another
// ALREADY-healthy tunnel (gre-EU) whose own gateway IS reachable --
// confirms the diagnosis correctly concludes "remote_unreachable" (this
// one tunnel's own problem), not "router_uplink_down".
func TestDiagnoseIncidentOnce_RemoteUnreachableWhenOnlyThisTunnelFails(t *testing.T) {
	db := openTunnelHealthTestDB(t)

	routes := []mikrotik.RouteEntry{
		{ID: "*1", DstAddress: "10.0.0.0/24", Gateway: strPtr("100.100.77.1"), ImmediateGw: strPtr("100.100.77.1%gre-TR")},
		{ID: "*2", DstAddress: "10.0.1.0/24", Gateway: strPtr("100.100.88.1"), ImmediateGw: strPtr("100.100.88.1%gre-EU")},
	}
	fake := newFakeRouterOSWithPing(routes)
	fake.reachableHosts["100.100.88.1"] = true // the OTHER tunnel's gateway responds
	// 100.100.77.1 (the down tunnel's own gateway) is left unreachable.
	srv := fake.server()
	defer srv.Close()

	// gre-EU must already be recorded as healthy for probeOtherTunnels to
	// consider it a candidate "other tunnel" to sample.
	if err := db.Create(&model.TunnelHealthStatus{InterfaceName: "gre-EU", Severity: "healthy", LastPolledAt: time.Now()}).Error; err != nil {
		t.Fatalf("failed to seed healthy tunnel status: %v", err)
	}

	svc := newTestTunnelHealthService(t, db, srv)
	svc.diagnoseIncidentOnce("gre-TR", time.Now())

	var diagnosis model.TunnelIncidentDiagnosis
	if err := db.Where("interface_name = ?", "gre-TR").First(&diagnosis).Error; err != nil {
		t.Fatalf("expected a diagnosis row: %v", err)
	}
	if diagnosis.Cause != "remote_unreachable" {
		t.Errorf("expected cause 'remote_unreachable', got %q (evidence: %s)", diagnosis.Cause, diagnosis.Evidence)
	}
}

// TestDiagnoseIncidentOnce_RouterUplinkDownWhenNothingResponds sets up a
// down tunnel AND another supposedly-healthy tunnel whose gateway is
// ALSO unreachable -- confirms the diagnosis concludes
// "router_uplink_down" rather than blaming just the one tunnel, since
// the evidence points at the whole router having lost connectivity.
func TestDiagnoseIncidentOnce_RouterUplinkDownWhenNothingResponds(t *testing.T) {
	db := openTunnelHealthTestDB(t)

	routes := []mikrotik.RouteEntry{
		{ID: "*1", DstAddress: "10.0.0.0/24", Gateway: strPtr("100.100.77.1"), ImmediateGw: strPtr("100.100.77.1%gre-TR")},
		{ID: "*2", DstAddress: "10.0.1.0/24", Gateway: strPtr("100.100.88.1"), ImmediateGw: strPtr("100.100.88.1%gre-EU")},
	}
	fake := newFakeRouterOSWithPing(routes)
	// Nothing is reachable -- both the down tunnel's own gateway AND the
	// other tunnel's gateway time out.
	srv := fake.server()
	defer srv.Close()

	if err := db.Create(&model.TunnelHealthStatus{InterfaceName: "gre-EU", Severity: "healthy", LastPolledAt: time.Now()}).Error; err != nil {
		t.Fatalf("failed to seed healthy tunnel status: %v", err)
	}

	svc := newTestTunnelHealthService(t, db, srv)
	svc.diagnoseIncidentOnce("gre-TR", time.Now())

	var diagnosis model.TunnelIncidentDiagnosis
	if err := db.Where("interface_name = ?", "gre-TR").First(&diagnosis).Error; err != nil {
		t.Fatalf("expected a diagnosis row: %v", err)
	}
	if diagnosis.Cause != "router_uplink_down" {
		t.Errorf("expected cause 'router_uplink_down', got %q (evidence: %s)", diagnosis.Cause, diagnosis.Evidence)
	}
}

// TestDiagnoseIncidentOnce_RouterResourceExhausted confirms an
// overloaded router (CPU above the diagnosis threshold) is reported as
// the cause even when the tunnel's own gateway would otherwise look like
// a plain remote-unreachable case -- resource exhaustion takes priority
// since it plausibly explains every OTHER signal at once.
func TestDiagnoseIncidentOnce_RouterResourceExhausted(t *testing.T) {
	db := openTunnelHealthTestDB(t)

	routes := []mikrotik.RouteEntry{
		{ID: "*1", DstAddress: "10.0.0.0/24", Gateway: strPtr("100.100.77.1"), ImmediateGw: strPtr("100.100.77.1%gre-TR")},
	}
	fake := newFakeRouterOSWithPing(routes)
	fake.deviceInfo.CPULoad = "97"
	srv := fake.server()
	defer srv.Close()

	svc := newTestTunnelHealthService(t, db, srv)
	svc.diagnoseIncidentOnce("gre-TR", time.Now())

	var diagnosis model.TunnelIncidentDiagnosis
	if err := db.Where("interface_name = ?", "gre-TR").First(&diagnosis).Error; err != nil {
		t.Fatalf("expected a diagnosis row: %v", err)
	}
	if diagnosis.Cause != "router_resource_exhausted" {
		t.Errorf("expected cause 'router_resource_exhausted', got %q (evidence: %s)", diagnosis.Cause, diagnosis.Evidence)
	}
}

// ---------------------------------------------------------------------
// Active backup-path probe tests.
// ---------------------------------------------------------------------

// TestProbeBackupPaths_RecordsReachableAndUnreachable confirms
// ProbeBackupPaths probes every policy with a configured
// Level2.BackupGatewayIP and records the correct Reachable value for
// each, skipping tunnels with no backup gateway configured at all.
func TestProbeBackupPaths_RecordsReachableAndUnreachable(t *testing.T) {
	db := openTunnelHealthTestDB(t)
	fake := newFakeRouterOSWithPing(nil)
	fake.reachableHosts["100.100.88.1"] = true
	srv := fake.server()
	defer srv.Close()
	svc := newTestTunnelHealthService(t, db, srv)

	reachablePolicy, err := svc.policyService.GetOrCreateDefault("gre-TR")
	if err != nil {
		t.Fatalf("failed to create policy for gre-TR: %v", err)
	}
	var reachableConfig TunnelPolicyConfig
	reachableConfig.Level2.BackupTarget = "gre-EU"
	reachableConfig.Level2.BackupGatewayIP = "100.100.88.1"
	if _, err := svc.policyService.Update(reachablePolicy.ID, reachableConfig); err != nil {
		t.Fatalf("failed to save reachable policy: %v", err)
	}

	unreachablePolicy, err := svc.policyService.GetOrCreateDefault("gre-EU")
	if err != nil {
		t.Fatalf("failed to create policy for gre-EU: %v", err)
	}
	var unreachableConfig TunnelPolicyConfig
	unreachableConfig.Level2.BackupTarget = "gre-ASIA"
	unreachableConfig.Level2.BackupGatewayIP = "100.100.99.1"
	if _, err := svc.policyService.Update(unreachablePolicy.ID, unreachableConfig); err != nil {
		t.Fatalf("failed to save unreachable policy: %v", err)
	}

	// No Level 2 configured at all -- must be silently skipped.
	if _, err := svc.policyService.GetOrCreateDefault("gre-NONE"); err != nil {
		t.Fatalf("failed to create policy for gre-NONE: %v", err)
	}

	svc.ProbeBackupPaths()

	var reachableProbe model.TunnelBackupProbeResult
	if err := db.Where("interface_name = ?", "gre-TR").First(&reachableProbe).Error; err != nil {
		t.Fatalf("expected a probe result for gre-TR: %v", err)
	}
	if !reachableProbe.Reachable {
		t.Errorf("expected gre-TR's backup gateway to be reported reachable")
	}

	var unreachableProbe model.TunnelBackupProbeResult
	if err := db.Where("interface_name = ?", "gre-EU").First(&unreachableProbe).Error; err != nil {
		t.Fatalf("expected a probe result for gre-EU: %v", err)
	}
	if unreachableProbe.Reachable {
		t.Errorf("expected gre-EU's backup gateway to be reported unreachable")
	}

	var noBackupCount int64
	db.Model(&model.TunnelBackupProbeResult{}).Where("interface_name = ?", "gre-NONE").Count(&noBackupCount)
	if noBackupCount != 0 {
		t.Errorf("expected gre-NONE (no backup configured) to be skipped entirely, got %d probe rows", noBackupCount)
	}
}

// ---------------------------------------------------------------------
// Weekly incident-pattern report tests.
// ---------------------------------------------------------------------

// TestBuildIncidentPatternReport_FlagsChronicFlapper confirms a tunnel
// that flapped past incidentPatternMinFlaps within the lookback window
// is called out by name in the report body.
func TestBuildIncidentPatternReport_FlagsChronicFlapper(t *testing.T) {
	db := openTunnelHealthTestDB(t)
	fake := newFakeRouterOS(nil)
	srv := fake.server()
	defer srv.Close()
	svc := newTestTunnelHealthService(t, db, srv)

	now := time.Now()
	for i := 0; i < incidentPatternMinFlaps; i++ {
		event := model.TunnelHealthEvent{
			InterfaceName: "gre-FLAPPY",
			FromStatus:    "healthy",
			ToStatus:      "confirmed_down",
			Evidence:      "test",
			DetectedAt:    now.Add(-time.Duration(i) * time.Hour),
		}
		if err := db.Create(&event).Error; err != nil {
			t.Fatalf("failed to seed flap event: %v", err)
		}
	}

	report := svc.BuildIncidentPatternReport(now)
	if report == "" {
		t.Fatal("expected a non-empty report for a chronically-flapping tunnel")
	}
	if !containsSubstring(report, "gre-FLAPPY") {
		t.Errorf("expected report to mention gre-FLAPPY, got: %s", report)
	}
}

// TestBuildIncidentPatternReport_EmptyWhenNothingNoteworthy confirms an
// otherwise-quiet week (a couple of isolated, non-repeating incidents)
// produces an empty report -- callers must not notify on this.
func TestBuildIncidentPatternReport_EmptyWhenNothingNoteworthy(t *testing.T) {
	db := openTunnelHealthTestDB(t)
	fake := newFakeRouterOS(nil)
	srv := fake.server()
	defer srv.Close()
	svc := newTestTunnelHealthService(t, db, srv)

	now := time.Now()
	event := model.TunnelHealthEvent{
		InterfaceName: "gre-QUIET",
		FromStatus:    "healthy",
		ToStatus:      "confirmed_down",
		Evidence:      "test",
		DetectedAt:    now.Add(-time.Hour),
	}
	if err := db.Create(&event).Error; err != nil {
		t.Fatalf("failed to seed event: %v", err)
	}

	report := svc.BuildIncidentPatternReport(now)
	if report != "" {
		t.Errorf("expected an empty report for a single isolated incident, got: %s", report)
	}
}

// TestBuildIncidentPatternReport_FlagsCorrelatedTunnels confirms two
// DIFFERENT tunnels that repeatedly fail within
// incidentPatternCorrelationWindow of each other are called out as a
// correlated pair.
func TestBuildIncidentPatternReport_FlagsCorrelatedTunnels(t *testing.T) {
	db := openTunnelHealthTestDB(t)
	fake := newFakeRouterOS(nil)
	srv := fake.server()
	defer srv.Close()
	svc := newTestTunnelHealthService(t, db, srv)

	now := time.Now()
	for i := 0; i < incidentPatternMinCorrelatedCount; i++ {
		base := now.Add(-time.Duration(i) * 24 * time.Hour)
		events := []model.TunnelHealthEvent{
			{InterfaceName: "gre-A", FromStatus: "healthy", ToStatus: "confirmed_down", Evidence: "test", DetectedAt: base},
			{InterfaceName: "gre-B", FromStatus: "healthy", ToStatus: "confirmed_down", Evidence: "test", DetectedAt: base.Add(30 * time.Second)},
		}
		for _, e := range events {
			if err := db.Create(&e).Error; err != nil {
				t.Fatalf("failed to seed correlated event: %v", err)
			}
		}
	}

	report := svc.BuildIncidentPatternReport(now)
	if report == "" {
		t.Fatal("expected a non-empty report for correlated tunnels")
	}
	if !containsSubstring(report, "gre-A") || !containsSubstring(report, "gre-B") {
		t.Errorf("expected report to mention both gre-A and gre-B, got: %s", report)
	}
}

func containsSubstring(haystack, needle string) bool {
	return len(haystack) >= len(needle) && (func() bool {
		for i := 0; i+len(needle) <= len(haystack); i++ {
			if haystack[i:i+len(needle)] == needle {
				return true
			}
		}
		return false
	})()
}
