package service

import (
	"testing"
	"time"

	"github.com/maahdima/mwp/api/dataservice/model"
)

// TestPruneStaleTunnels_NeverTouchesAnotherServersRows is the core
// regression test for a confirmed, dangerous multi-server bug caught
// before it ever shipped: pruneStaleTunnels used to delete
// TunnelHealthStatus/TunnelPolicy/TunnelHealthScore/TunnelBackupProbeResult
// rows by interface_name alone, with no server_id condition -- so polling
// SERVER A would delete SERVER B's entire tunnel history for any
// interface name that doesn't happen to also be one of server A's own
// CURRENT tunnels (which, for two different physical routers, is nearly
// every interface name server B has). This seeds two servers each with
// their own "gre-TR" tunnel (same name, different servers -- the
// admin's own explicit requirement that same-named interfaces on
// different servers must never collide), runs pruneStaleTunnels for
// server A with an EMPTY current-tunnel list (the worst case: server A's
// discovery found nothing this tick), and confirms server B's rows for
// "gre-TR" survive completely untouched.
func TestPruneStaleTunnels_NeverTouchesAnotherServersRows(t *testing.T) {
	db := openTunnelHealthTestDB(t)
	fake := newFakeRouterOS(nil)
	srv := fake.server()
	defer srv.Close()

	svcA := newTestTunnelHealthService(t, db, srv)
	serverA := svcA.activeServerID

	// A second, independent server row + service instance sharing the
	// same db -- mirrors how ForEachActiveServer's sequential loop gives
	// every server its own pollOneServer pass against one shared database.
	// IPAddress must be distinct and non-empty: newTestTunnelHealthService's
	// own server row uses "" (unique index collision otherwise).
	serverBRow := model.Server{Name: "test-server-b", IPAddress: "10.99.0.2", IsActive: true}
	if err := db.Create(&serverBRow).Error; err != nil {
		t.Fatalf("failed to seed second server: %v", err)
	}
	serverB := serverBRow.ID

	const sharedName = "gre-TR"

	if err := db.Create(&model.TunnelHealthStatus{ServerID: serverA, InterfaceName: sharedName, Severity: "healthy", LastPolledAt: time.Now()}).Error; err != nil {
		t.Fatalf("failed to seed server A status: %v", err)
	}
	if err := db.Create(&model.TunnelHealthStatus{ServerID: serverB, InterfaceName: sharedName, Severity: "confirmed_down", LastPolledAt: time.Now()}).Error; err != nil {
		t.Fatalf("failed to seed server B status: %v", err)
	}
	if err := db.Create(&model.TunnelPolicy{ServerID: serverA, InterfaceName: sharedName, PolicyJSON: "{}"}).Error; err != nil {
		t.Fatalf("failed to seed server A policy: %v", err)
	}
	if err := db.Create(&model.TunnelPolicy{ServerID: serverB, InterfaceName: sharedName, PolicyJSON: "{}"}).Error; err != nil {
		t.Fatalf("failed to seed server B policy: %v", err)
	}
	if err := db.Create(&model.TunnelHealthScore{ServerID: serverB, InterfaceName: sharedName, Score: 80, SampledAt: time.Now()}).Error; err != nil {
		t.Fatalf("failed to seed server B score: %v", err)
	}
	if err := db.Create(&model.TunnelBackupProbeResult{ServerID: serverB, InterfaceName: sharedName, BackupGatewayIP: "10.0.0.1", Reachable: true, Evidence: "ok", ProbedAt: time.Now()}).Error; err != nil {
		t.Fatalf("failed to seed server B backup probe: %v", err)
	}

	// Server A's own discovery currently only sees a DIFFERENT tunnel
	// ("wg0") -- so "gre-TR" is stale from server A's point of view this
	// tick. This is the realistic trigger for the bug: without server_id
	// scoping, server B's identically-named "gre-TR" would also look
	// stale to server A's prune pass and get deleted.
	svcA.pruneStaleTunnels(serverA, []model.GraphNode{{Name: "wg0"}})

	var serverBStatusCount int64
	db.Model(&model.TunnelHealthStatus{}).Where("server_id = ? AND interface_name = ?", serverB, sharedName).Count(&serverBStatusCount)
	if serverBStatusCount != 1 {
		t.Errorf("expected server B's TunnelHealthStatus for %q to survive server A's prune, got count=%d", sharedName, serverBStatusCount)
	}

	var serverBPolicyCount int64
	db.Model(&model.TunnelPolicy{}).Where("server_id = ? AND interface_name = ?", serverB, sharedName).Count(&serverBPolicyCount)
	if serverBPolicyCount != 1 {
		t.Errorf("expected server B's TunnelPolicy for %q to survive server A's prune, got count=%d", sharedName, serverBPolicyCount)
	}

	var serverBScoreCount int64
	db.Model(&model.TunnelHealthScore{}).Where("server_id = ? AND interface_name = ?", serverB, sharedName).Count(&serverBScoreCount)
	if serverBScoreCount != 1 {
		t.Errorf("expected server B's TunnelHealthScore for %q to survive server A's prune, got count=%d", sharedName, serverBScoreCount)
	}

	var serverBProbeCount int64
	db.Model(&model.TunnelBackupProbeResult{}).Where("server_id = ? AND interface_name = ?", serverB, sharedName).Count(&serverBProbeCount)
	if serverBProbeCount != 1 {
		t.Errorf("expected server B's TunnelBackupProbeResult for %q to survive server A's prune, got count=%d", sharedName, serverBProbeCount)
	}

	// And server A's OWN now-stale row (not in its empty current list)
	// must actually have been pruned -- confirms this isn't just a
	// trivially-passing no-op.
	var serverAStatusCount int64
	db.Model(&model.TunnelHealthStatus{}).Where("server_id = ? AND interface_name = ?", serverA, sharedName).Count(&serverAStatusCount)
	if serverAStatusCount != 0 {
		t.Errorf("expected server A's own stale TunnelHealthStatus for %q to be pruned, got count=%d", sharedName, serverAStatusCount)
	}
}

// TestGetOrCreateDefault_SameInterfaceNameIsolatedPerServer confirms
// TunnelPolicyService.GetOrCreateDefault treats the same interface name
// on two different servers as two completely independent policies -- a
// confirmed, reported multi-server gap: before ServerID was threaded
// through, the exact same interface name on two different servers
// collided into ONE shared policy row, so an admin's remediation
// settings for one server's tunnel could silently apply to (or be
// overwritten by) a same-named tunnel on a different server.
func TestGetOrCreateDefault_SameInterfaceNameIsolatedPerServer(t *testing.T) {
	db := openTunnelHealthTestDB(t)
	policyService := NewTunnelPolicyService(db)

	const sharedName = "wg0"
	var serverA uint = 1
	var serverB uint = 2

	policyA, err := policyService.GetOrCreateDefault(serverA, sharedName)
	if err != nil {
		t.Fatalf("failed to create policy A: %v", err)
	}
	policyB, err := policyService.GetOrCreateDefault(serverB, sharedName)
	if err != nil {
		t.Fatalf("failed to create policy B: %v", err)
	}

	if policyA.ID == policyB.ID {
		t.Fatalf("expected two DISTINCT policy rows for the same interface name on different servers, got the same row (id=%d) for both", policyA.ID)
	}

	// Editing server A's policy must never affect server B's.
	var configA TunnelPolicyConfig
	configA.Level2.BackupTarget = "gre-A-backup"
	if _, err := policyService.Update(policyA.ID, configA); err != nil {
		t.Fatalf("failed to update policy A: %v", err)
	}

	reloadedB, err := policyService.GetOrCreateDefault(serverB, sharedName)
	if err != nil {
		t.Fatalf("failed to reload policy B: %v", err)
	}
	configB := policyService.Parse(reloadedB)
	if configB.Level2.BackupTarget == "gre-A-backup" {
		t.Errorf("expected server B's policy to be unaffected by server A's edit, but BackupTarget leaked across servers")
	}

	// List scoped to one server must return exactly that server's policy,
	// never the other server's same-named row.
	listA, err := policyService.List(&serverA)
	if err != nil {
		t.Fatalf("failed to list server A's policies: %v", err)
	}
	if len(listA) != 1 || listA[0].ServerID != serverA {
		t.Fatalf("expected List(serverA) to return exactly server A's 1 policy, got %d rows", len(listA))
	}

	// List with no server filter must return BOTH servers' rows, never
	// merged into one.
	listAll, err := policyService.List(nil)
	if err != nil {
		t.Fatalf("failed to list all policies: %v", err)
	}
	if len(listAll) != 2 {
		t.Fatalf("expected List(nil) to return both servers' policies (2 total), got %d", len(listAll))
	}
}

// TestLastActionSinceUnhealthy_NeverCrossesServerBoundary confirms the
// remediation escalation engine's own history lookup
// (lastActionSinceUnhealthy, which considerRemediation uses to decide
// "has Level 1 already been tried for this incident") never mistakes one
// server's remediation history for another server's -- a confirmed risk:
// without server_id scoping, a Level 1 already attempted on server A's
// "gre-TR" could make the engine skip straight to Level 2 for server B's
// own, completely unrelated "gre-TR" incident, or vice versa.
func TestLastActionSinceUnhealthy_NeverCrossesServerBoundary(t *testing.T) {
	db := openTunnelHealthTestDB(t)
	fake := newFakeRouterOS(nil)
	srv := fake.server()
	defer srv.Close()

	svcA := newTestTunnelHealthService(t, db, srv)
	serverA := svcA.activeServerID

	serverBRow := model.Server{Name: "test-server-b", IPAddress: "10.99.0.3", IsActive: true}
	if err := db.Create(&serverBRow).Error; err != nil {
		t.Fatalf("failed to seed second server: %v", err)
	}
	serverB := serverBRow.ID

	const sharedName = "gre-TR"

	// Server A already attempted (and completed) a Level 1 remediation
	// for this incident.
	if err := db.Create(&model.TunnelActionLog{
		ServerID: serverA, InterfaceName: sharedName, Level: "1",
		CommandDescription: "test", CommandDetail: "-", Simulated: false,
		Result: "موفق", ExecutedAt: time.Now(),
	}).Error; err != nil {
		t.Fatalf("failed to seed server A action log: %v", err)
	}

	// From server B's own perspective (activeServerID switched, exactly
	// like pollOneServer does between servers in Poll's loop), this same
	// interface name must look like it has NO remediation history yet --
	// server A's Level 1 must not leak across the boundary.
	svcA.activeServerID = serverB
	lastAction, err := svcA.lastActionSinceUnhealthy(sharedName)
	if err != nil {
		t.Fatalf("lastActionSinceUnhealthy failed: %v", err)
	}
	if lastAction != nil {
		t.Errorf("expected server B to see NO remediation history for %q, but found server A's own Level %q action leaking across the server boundary", sharedName, lastAction.Level)
	}
}
