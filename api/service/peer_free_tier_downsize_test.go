package service

import (
	"testing"
	"time"

	"github.com/maahdima/mwp/api/dataservice/model"
)

func createTestPeerForDownsize(t *testing.T, w *WgPeer, uuidSuffix string, createdAt uint64) model.Peer {
	t.Helper()
	p := model.Peer{
		UUID: "peer-uuid-" + uuidSuffix, PeerID: "peer-" + uuidSuffix, Name: "peer-" + uuidSuffix,
		PrivateKey: "priv", PublicKey: "pub", Interface: "wg0",
		AllowedAddress: "10.0.0." + uuidSuffix + "/32", Endpoint: "example.com", EndpointPort: "51820",
	}
	if err := w.db.Create(&p).Error; err != nil {
		t.Fatalf("failed to create test peer: %v", err)
	}
	if createdAt > 0 {
		if err := w.db.Model(&model.Peer{}).Where("id = ?", p.ID).Update("created_at", createdAt).Error; err != nil {
			t.Fatalf("failed to backdate test peer: %v", err)
		}
	}
	return p
}

// TestSuspendOldestForFreeTier_Peer_DisablesOnlyOldestExcess is the core
// regression test for the retroactive side of phase 4-12: given more active
// peers than the cap allows, only the OLDEST excess ones (by CreatedAt) are
// disabled -- the newest ones (what a customer is actually using right now)
// are left untouched.
func TestSuspendOldestForFreeTier_Peer_DisablesOnlyOldestExcess(t *testing.T) {
	w, db := newTestWgPeerForFreeTier(t)

	base := uint64(time.Now().Unix())
	oldest := createTestPeerForDownsize(t, w, "1", base)
	middle := createTestPeerForDownsize(t, w, "2", base+10)
	newest := createTestPeerForDownsize(t, w, "3", base+20)

	suspended, err := w.SuspendOldestForFreeTier(2)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if suspended != 1 {
		t.Fatalf("expected exactly 1 peer suspended (3 active - cap 2), got %d", suspended)
	}

	var reloadedOldest, reloadedMiddle, reloadedNewest model.Peer
	db.First(&reloadedOldest, oldest.ID)
	db.First(&reloadedMiddle, middle.ID)
	db.First(&reloadedNewest, newest.ID)

	if !reloadedOldest.Disabled || !reloadedOldest.SuspendedByQuota || !reloadedOldest.WasActiveBeforeSuspend {
		t.Fatalf("expected the OLDEST peer to be suspended, got Disabled=%v SuspendedByQuota=%v WasActiveBeforeSuspend=%v",
			reloadedOldest.Disabled, reloadedOldest.SuspendedByQuota, reloadedOldest.WasActiveBeforeSuspend)
	}
	if reloadedMiddle.Disabled || reloadedNewest.Disabled {
		t.Fatalf("expected the two newest peers to remain enabled, got middle.Disabled=%v newest.Disabled=%v",
			reloadedMiddle.Disabled, reloadedNewest.Disabled)
	}
}

// TestSuspendOldestForFreeTier_Peer_UnderCapIsNoOp confirms a peer count at
// or below the cap suspends nothing.
func TestSuspendOldestForFreeTier_Peer_UnderCapIsNoOp(t *testing.T) {
	w, _ := newTestWgPeerForFreeTier(t)
	createTestPeerForDownsize(t, w, "1", 0)

	suspended, err := w.SuspendOldestForFreeTier(5)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if suspended != 0 {
		t.Fatalf("expected 0 peers suspended under cap, got %d", suspended)
	}
}

// TestSuspendOldestForFreeTier_Peer_IsIdempotent confirms a second call
// with the same cap, after the first already converged, suspends nothing
// further (already-disabled peers are excluded from the candidate query).
func TestSuspendOldestForFreeTier_Peer_IsIdempotent(t *testing.T) {
	w, _ := newTestWgPeerForFreeTier(t)
	base := uint64(time.Now().Unix())
	createTestPeerForDownsize(t, w, "1", base)
	createTestPeerForDownsize(t, w, "2", base+10)

	first, err := w.SuspendOldestForFreeTier(1)
	if err != nil {
		t.Fatalf("unexpected error on first call: %v", err)
	}
	if first != 1 {
		t.Fatalf("expected 1 peer suspended on first call, got %d", first)
	}

	second, err := w.SuspendOldestForFreeTier(1)
	if err != nil {
		t.Fatalf("unexpected error on second call: %v", err)
	}
	if second != 0 {
		t.Fatalf("expected 0 additional peers suspended on repeat call (already converged), got %d", second)
	}
}

// TestSuspendOldestForFreeTier_Peer_ZeroCapIsNoOp confirms cap<=0 (the
// GetFreeTierLimit "unlimited" sentinel) never suspends anything, matching
// enforceFreeTierDownsizing's own guard.
func TestSuspendOldestForFreeTier_Peer_ZeroCapIsNoOp(t *testing.T) {
	w, _ := newTestWgPeerForFreeTier(t)
	createTestPeerForDownsize(t, w, "1", 0)

	suspended, err := w.SuspendOldestForFreeTier(0)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if suspended != 0 {
		t.Fatalf("expected 0 peers suspended with cap<=0, got %d", suspended)
	}
}
