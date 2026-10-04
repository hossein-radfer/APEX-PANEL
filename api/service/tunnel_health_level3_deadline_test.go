package service

import (
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"testing"
	"time"

	"github.com/maahdima/mwp/api/adaptor/mikrotik"
	"github.com/maahdima/mwp/api/dataservice/model"
)

// newFakeRouterForBootGrace serves exactly what isInBootGrace needs
// (GET /rest/system/resource with a long uptime, so considerRemediation's
// own boot-grace gate never blocks the test) -- nothing else, since these
// tests exercise the forced-Level-3-deadline branch with Level3.Enabled
// left false, which attemptLevel3 rejects before touching routes/
// interfaces at all.
func newFakeRouterForBootGrace(t *testing.T) *httptest.Server {
	t.Helper()
	return httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Path == "/rest/system/resource" {
			w.Header().Set("Content-Type", "application/json")
			_ = json.NewEncoder(w).Encode(map[string]string{"uptime": "10h0m0s"})
			return
		}
		w.WriteHeader(http.StatusNotFound)
	}))
}

// TestConsiderRemediation_ForcesLevel3AfterDeadlineRegardlessOfLastLevel
// is the regression test for the confirmed, reported gap: the admin
// expects a tunnel confirmed-down for Level3.Enabled tunnels to fail over
// within 3 minutes, but the old sequential L1->L2->L3 escalation had no
// enforced wall-clock deadline. With ConfirmedDownSince set more than
// forceLevel3Deadline in the past and no remediation attempted yet
// (lastAction == nil), considerRemediation must jump straight to Level 3
// instead of starting at Level 1.
func TestConsiderRemediation_ForcesLevel3AfterDeadlineRegardlessOfLastLevel(t *testing.T) {
	srv := newFakeRouterForBootGrace(t)
	defer srv.Close()
	db := openTunnelHealthTestDB(t)
	svc := newTestTunnelHealthService(t, db, srv)

	iface := "gre-test"
	confirmedSince := time.Now().Add(-5 * time.Minute) // older than forceLevel3Deadline (3m)
	status := model.TunnelHealthStatus{
		InterfaceName:      iface,
		Severity:           "confirmed_down",
		ConfirmedDownSince: &confirmedSince,
		LastPolledAt:       time.Now(),
	}
	if err := db.Create(&status).Error; err != nil {
		t.Fatalf("failed to seed tunnel health status: %v", err)
	}

	policy, err := svc.policyService.GetOrCreateDefault(iface)
	if err != nil {
		t.Fatalf("failed to create default policy: %v", err)
	}
	config := svc.policyService.Parse(policy)
	// Level3.Enabled stays false (default) -- attemptLevel3 will reject
	// with a logged "disabled in policy" action rather than attempting
	// any network action, which is exactly what we want to observe here:
	// proof that Level 3 was the FIRST thing attempted, not Level 1.

	svc.considerRemediation(iface, policy, time.Now())

	var actions []model.TunnelActionLog
	if err := db.Where("interface_name = ?", iface).Order("created_at asc").Find(&actions).Error; err != nil {
		t.Fatalf("failed to load action log: %v", err)
	}
	if len(actions) != 1 {
		t.Fatalf("expected exactly one remediation action logged, got %d", len(actions))
	}
	if actions[0].Level != "3" {
		t.Fatalf("expected the FIRST action to be Level 3 (forced by the exceeded deadline), got Level %q", actions[0].Level)
	}
	_ = config
}

// TestConsiderRemediation_DoesNotForceLevel3BeforeDeadline confirms the
// deadline branch does NOT fire prematurely -- a tunnel confirmed-down
// for less than forceLevel3Deadline must still start at Level 1, exactly
// like before this feature existed.
func TestConsiderRemediation_DoesNotForceLevel3BeforeDeadline(t *testing.T) {
	srv := newFakeRouterForBootGrace(t)
	defer srv.Close()
	db := openTunnelHealthTestDB(t)
	svc := newTestTunnelHealthService(t, db, srv)

	iface := "gre-test2"
	confirmedSince := time.Now().Add(-30 * time.Second) // well inside the 3-minute deadline
	status := model.TunnelHealthStatus{
		InterfaceName:      iface,
		Severity:           "confirmed_down",
		ConfirmedDownSince: &confirmedSince,
		LastPolledAt:       time.Now(),
	}
	if err := db.Create(&status).Error; err != nil {
		t.Fatalf("failed to seed tunnel health status: %v", err)
	}

	policy, err := svc.policyService.GetOrCreateDefault(iface)
	if err != nil {
		t.Fatalf("failed to create default policy: %v", err)
	}

	svc.considerRemediation(iface, policy, time.Now())

	var actions []model.TunnelActionLog
	if err := db.Where("interface_name = ?", iface).Order("created_at asc").Find(&actions).Error; err != nil {
		t.Fatalf("failed to load action log: %v", err)
	}
	if len(actions) != 1 {
		t.Fatalf("expected exactly one remediation action logged, got %d", len(actions))
	}
	if actions[0].Level != "1" {
		t.Fatalf("expected the first action to be Level 1 (deadline not yet exceeded), got Level %q", actions[0].Level)
	}
}

// TestPollOneTunnel_SkipsEntirelyWhenPolicyDisabled is the regression
// test for the admin's explicit feature request: the ability to exclude
// one specific tunnel from detection/alerting/remediation entirely.
// With TunnelPolicyConfig.Enabled=false, pollOneTunnel must not create or
// update ANY TunnelHealthStatus row for that interface.
func TestPollOneTunnel_SkipsEntirelyWhenPolicyDisabled(t *testing.T) {
	srv := newFakeRouterForBootGrace(t)
	defer srv.Close()
	db := openTunnelHealthTestDB(t)
	svc := newTestTunnelHealthService(t, db, srv)

	iface := "gre-disabled"

	// Seed a policy with Enabled=false for this interface BEFORE polling
	// -- mirrors how an admin would configure this via the policy update
	// endpoint.
	disabledConfig := DefaultTunnelPolicyConfig()
	disabledConfig.Enabled = false
	raw, err := json.Marshal(disabledConfig)
	if err != nil {
		t.Fatalf("failed to marshal disabled policy: %v", err)
	}
	if err := db.Create(&model.TunnelPolicy{InterfaceName: iface, PolicyJSON: string(raw)}).Error; err != nil {
		t.Fatalf("failed to seed disabled policy: %v", err)
	}

	node := model.GraphNode{Name: iface, Type: "gre", PropertiesJSON: `{"interface_type":"gre"}`}
	// Deliberately a "would-be-confirmed_down" interface state (disabled,
	// not running) -- if Enabled=false did NOT short-circuit detection,
	// this would otherwise produce a TunnelHealthStatus row and likely a
	// remediation attempt, which the assertions below confirm never
	// happens.
	mtIface := mikrotik.Interface{Name: iface, Running: "false", Disabled: "true"}

	svc.pollOneTunnel(node, mtIface, nil, time.Now())

	var count int64
	if err := db.Model(&model.TunnelHealthStatus{}).Where("interface_name = ?", iface).Count(&count).Error; err != nil {
		t.Fatalf("failed to count tunnel health status rows: %v", err)
	}
	if count != 0 {
		t.Fatalf("expected zero TunnelHealthStatus rows for a disabled tunnel, got %d -- Enabled=false must skip detection entirely", count)
	}

	var actionCount int64
	if err := db.Model(&model.TunnelActionLog{}).Where("interface_name = ?", iface).Count(&actionCount).Error; err != nil {
		t.Fatalf("failed to count action log rows: %v", err)
	}
	if actionCount != 0 {
		t.Fatalf("expected zero remediation actions for a disabled tunnel, got %d", actionCount)
	}
}
