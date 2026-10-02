package service

import (
	"testing"
	"time"

	"github.com/maahdima/mwp/api/dataservice/model"
)

func createTestDNSAccountForDownsize(t *testing.T, svc *DNSAccountService, panelID uint, suffix string, createdAt uint64) model.DNSAccount {
	t.Helper()
	acct := model.DNSAccount{
		UUID: "dns-uuid-" + suffix, PanelID: panelID, ApexRef: "apex:test:" + suffix,
		Status: "active", IsShared: true, MaxConcurrentIPs: 1,
	}
	if err := svc.db.Create(&acct).Error; err != nil {
		t.Fatalf("failed to create test dns account: %v", err)
	}
	if createdAt > 0 {
		if err := svc.db.Model(&model.DNSAccount{}).Where("id = ?", acct.ID).Update("created_at", createdAt).Error; err != nil {
			t.Fatalf("failed to backdate test account: %v", err)
		}
	}
	return acct
}

// TestSuspendOldestForFreeTier_DNSAccount_DisablesOnlyOldestExcess mirrors
// TestSuspendOldestForFreeTier_Peer_DisablesOnlyOldestExcess exactly.
func TestSuspendOldestForFreeTier_DNSAccount_DisablesOnlyOldestExcess(t *testing.T) {
	svc, db, _ := newTestDNSAccountService(t, alwaysAcceptClaimHandler)

	var panel model.DNSPanel
	if err := db.First(&panel).Error; err != nil {
		t.Fatalf("failed to load test panel: %v", err)
	}

	base := uint64(time.Now().Unix())
	oldest := createTestDNSAccountForDownsize(t, svc, panel.ID, "1", base)
	middle := createTestDNSAccountForDownsize(t, svc, panel.ID, "2", base+10)
	newest := createTestDNSAccountForDownsize(t, svc, panel.ID, "3", base+20)

	suspended, err := svc.SuspendOldestForFreeTier(2)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if suspended != 1 {
		t.Fatalf("expected exactly 1 account suspended (3 active - cap 2), got %d", suspended)
	}

	var reloadedOldest, reloadedMiddle, reloadedNewest model.DNSAccount
	db.First(&reloadedOldest, oldest.ID)
	db.First(&reloadedMiddle, middle.ID)
	db.First(&reloadedNewest, newest.ID)

	if reloadedOldest.Status != "suspended" || !reloadedOldest.SuspendedByQuota || !reloadedOldest.WasActiveBeforeSuspend {
		t.Fatalf("expected the OLDEST account to be suspended, got Status=%q SuspendedByQuota=%v WasActiveBeforeSuspend=%v",
			reloadedOldest.Status, reloadedOldest.SuspendedByQuota, reloadedOldest.WasActiveBeforeSuspend)
	}
	if reloadedMiddle.Status != "active" || reloadedNewest.Status != "active" {
		t.Fatalf("expected the two newest accounts to remain active, got middle.Status=%q newest.Status=%q",
			reloadedMiddle.Status, reloadedNewest.Status)
	}
}

// TestSuspendOldestForFreeTier_DNSAccount_UnderCapIsNoOp mirrors the peer
// equivalent.
func TestSuspendOldestForFreeTier_DNSAccount_UnderCapIsNoOp(t *testing.T) {
	svc, db, _ := newTestDNSAccountService(t, alwaysAcceptClaimHandler)
	var panel model.DNSPanel
	db.First(&panel)
	createTestDNSAccountForDownsize(t, svc, panel.ID, "1", 0)

	suspended, err := svc.SuspendOldestForFreeTier(5)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if suspended != 0 {
		t.Fatalf("expected 0 accounts suspended under cap, got %d", suspended)
	}
}

// TestSuspendOldestForFreeTier_DNSAccount_IsIdempotent mirrors the peer
// equivalent.
func TestSuspendOldestForFreeTier_DNSAccount_IsIdempotent(t *testing.T) {
	svc, db, _ := newTestDNSAccountService(t, alwaysAcceptClaimHandler)
	var panel model.DNSPanel
	db.First(&panel)

	base := uint64(time.Now().Unix())
	createTestDNSAccountForDownsize(t, svc, panel.ID, "1", base)
	createTestDNSAccountForDownsize(t, svc, panel.ID, "2", base+10)

	first, err := svc.SuspendOldestForFreeTier(1)
	if err != nil {
		t.Fatalf("unexpected error on first call: %v", err)
	}
	if first != 1 {
		t.Fatalf("expected 1 account suspended on first call, got %d", first)
	}

	second, err := svc.SuspendOldestForFreeTier(1)
	if err != nil {
		t.Fatalf("unexpected error on second call: %v", err)
	}
	if second != 0 {
		t.Fatalf("expected 0 additional accounts suspended on repeat call, got %d", second)
	}
}
