package traffic

import (
	"context"
	"strconv"
	"sync"
	"testing"

	"github.com/maahdima/mwp/api/adaptor/mikrotik"
	"github.com/maahdima/mwp/api/dataservice/model"
)

// configurableFakeAdaptor is a MikrotikAdaptor fake whose FetchWgPeer(s)
// return values are configurable per test (unlike the fixed-value
// fakeAdaptor in traffic_e2e_test.go), and which records every
// UpdateWgPeer call's Disabled value -- needed to verify
// ResetPeerUsage(s)'s own re-enable behavior.
type configurableFakeAdaptor struct {
	mu resetTestMutex

	wgPeer  *mikrotik.WireGuardPeer  // returned by FetchWgPeer
	wgPeers []mikrotik.WireGuardPeer // returned by FetchWgPeers
	updates map[string]string        // peerID -> last Disabled value passed to UpdateWgPeer
}

// resetTestMutex is a tiny local alias so this file has no import cycle
// concerns with sync -- just sync.Mutex under a name that signals intent
// at each call site below.
type resetTestMutex = sync.Mutex

func newConfigurableFakeAdaptor() *configurableFakeAdaptor {
	return &configurableFakeAdaptor{updates: make(map[string]string)}
}

func (f *configurableFakeAdaptor) FetchWgPeer(ctx context.Context, peerID string) (*mikrotik.WireGuardPeer, error) {
	if f.wgPeer != nil {
		return f.wgPeer, nil
	}
	return &mikrotik.WireGuardPeer{ID: peerID, TransferTx: "0", TransferRx: "0"}, nil
}

func (f *configurableFakeAdaptor) FetchWgPeers(ctx context.Context) ([]mikrotik.WireGuardPeer, error) {
	return f.wgPeers, nil
}

func (f *configurableFakeAdaptor) FetchInterface(ctx context.Context, interfaceID string) (*mikrotik.Interface, error) {
	return &mikrotik.Interface{TxByte: "0", RxByte: "0"}, nil
}

func (f *configurableFakeAdaptor) UpdateWgPeer(ctx context.Context, peerID string, wgPeer mikrotik.WireGuardPeer) (*mikrotik.WireGuardPeer, error) {
	f.mu.Lock()
	defer f.mu.Unlock()
	f.updates[peerID] = wgPeer.Disabled
	return &wgPeer, nil
}

func (f *configurableFakeAdaptor) UpdateScheduler(ctx context.Context, id string, s mikrotik.Scheduler) (*mikrotik.Scheduler, error) {
	return &s, nil
}
func (f *configurableFakeAdaptor) UpdateSimpleQueue(ctx context.Context, id string, q mikrotik.Queue) (*mikrotik.Queue, error) {
	return &q, nil
}
func (f *configurableFakeAdaptor) MonitorUserManagerUser(ctx context.Context, userID string) (*mikrotik.UserManagerMonitorResult, error) {
	return &mikrotik.UserManagerMonitorResult{}, nil
}
func (f *configurableFakeAdaptor) SetUserManagerUserDisabled(ctx context.Context, userID string, disabled string) (*mikrotik.UserManagerUser, error) {
	return &mikrotik.UserManagerUser{}, nil
}

// TestResetPeerUsage_ReenablesPeerSuspendedByOwnTrafficLimit is the core
// regression test for the confirmed reported bug "ریست حجم کار نمی‌کند"
// (WireGuard usage reset doesn't work): a peer force-disabled by
// applyPeerTrafficLimit for exceeding its OWN TrafficLimit must come
// back online (both in the DB and on RouterOS) when an admin resets its
// usage -- previously ResetPeerUsage only zeroed the counters and never
// touched Disabled at all, so the peer stayed cut off even though the
// panel reported 0 usage after "reset".
func TestResetPeerUsage_ReenablesPeerSuspendedByOwnTrafficLimit(t *testing.T) {
	db := openTestDB(t)

	limit := int64(1000)
	peer := model.Peer{
		UUID: "uuid-reset-1", PeerID: "peer-reset-1", Name: "p1",
		PrivateKey: "priv", PublicKey: "pub", Interface: "wg0",
		AllowedAddress:          "10.0.0.10/32",
		TrafficLimit:            &limit,
		DownloadUsage:           1500,
		LastTx:                  1500,
		Disabled:                true,
		SuspendedByTrafficLimit: true,
	}
	if err := db.Create(&peer).Error; err != nil {
		t.Fatalf("failed to create peer: %v", err)
	}

	fake := newConfigurableFakeAdaptor()
	fake.wgPeer = &mikrotik.WireGuardPeer{ID: peer.PeerID, TransferTx: "1500", TransferRx: "0"}
	calc := NewTrafficCalculator(db, fake, nil)

	if err := calc.ResetPeerUsage(peer.ID); err != nil {
		t.Fatalf("ResetPeerUsage failed: %v", err)
	}

	var updated model.Peer
	if err := db.First(&updated, peer.ID).Error; err != nil {
		t.Fatalf("failed to reload peer: %v", err)
	}
	if updated.Disabled {
		t.Errorf("expected peer to be re-enabled (Disabled=false) after reset, got Disabled=true")
	}
	if updated.SuspendedByTrafficLimit {
		t.Errorf("expected SuspendedByTrafficLimit cleared after reset, got true")
	}
	if updated.DownloadUsage != 0 || updated.UploadUsage != 0 {
		t.Errorf("expected usage zeroed after reset, got download=%d upload=%d", updated.DownloadUsage, updated.UploadUsage)
	}

	fake.mu.Lock()
	defer fake.mu.Unlock()
	if got, want := fake.updates[peer.PeerID], strconv.FormatBool(false); got != want {
		t.Errorf("expected RouterOS UpdateWgPeer Disabled=%q for %s, got %q", want, peer.PeerID, got)
	}
}

// TestResetPeerUsage_DoesNotReenableManuallyDisabledPeer confirms a peer
// an admin disabled manually (Disabled=true, SuspendedByTrafficLimit=false)
// is left untouched by a usage reset -- only a disable this job itself
// caused should ever be auto-cleared.
func TestResetPeerUsage_DoesNotReenableManuallyDisabledPeer(t *testing.T) {
	db := openTestDB(t)

	peer := model.Peer{
		UUID: "uuid-reset-2", PeerID: "peer-reset-2", Name: "p2",
		PrivateKey: "priv", PublicKey: "pub", Interface: "wg0",
		AllowedAddress:          "10.0.0.11/32",
		DownloadUsage:           500,
		Disabled:                true,
		SuspendedByTrafficLimit: false,
	}
	if err := db.Create(&peer).Error; err != nil {
		t.Fatalf("failed to create peer: %v", err)
	}

	fake := newConfigurableFakeAdaptor()
	calc := NewTrafficCalculator(db, fake, nil)

	if err := calc.ResetPeerUsage(peer.ID); err != nil {
		t.Fatalf("ResetPeerUsage failed: %v", err)
	}

	var updated model.Peer
	if err := db.First(&updated, peer.ID).Error; err != nil {
		t.Fatalf("failed to reload peer: %v", err)
	}
	if !updated.Disabled {
		t.Errorf("expected a manually-disabled peer to remain disabled after reset, got Disabled=false")
	}
	if updated.DownloadUsage != 0 {
		t.Errorf("expected usage zeroed after reset regardless of disabled state, got download=%d", updated.DownloadUsage)
	}

	fake.mu.Lock()
	defer fake.mu.Unlock()
	if _, touched := fake.updates[peer.PeerID]; touched {
		t.Errorf("expected no RouterOS UpdateWgPeer call for a manually-disabled peer, but one was made")
	}
}

// TestResetPeerUsage_DoesNotAffectResellerQuotaSuspension confirms
// SuspendedByQuota (the RESELLER-level pool) is completely untouched by
// ResetPeerUsage -- the cross-contamination bug this fix avoids: a peer
// suspended for the RESELLER's overall quota being exhausted must NOT be
// re-enabled just because an admin reset this one peer's own usage.
func TestResetPeerUsage_DoesNotAffectResellerQuotaSuspension(t *testing.T) {
	db := openTestDB(t)

	peer := model.Peer{
		UUID: "uuid-reset-3", PeerID: "peer-reset-3", Name: "p3",
		PrivateKey: "priv", PublicKey: "pub", Interface: "wg0",
		AllowedAddress:         "10.0.0.12/32",
		DownloadUsage:          500,
		Disabled:               true,
		SuspendedByQuota:       true,
		WasActiveBeforeSuspend: true,
	}
	if err := db.Create(&peer).Error; err != nil {
		t.Fatalf("failed to create peer: %v", err)
	}

	fake := newConfigurableFakeAdaptor()
	calc := NewTrafficCalculator(db, fake, nil)

	if err := calc.ResetPeerUsage(peer.ID); err != nil {
		t.Fatalf("ResetPeerUsage failed: %v", err)
	}

	var updated model.Peer
	if err := db.First(&updated, peer.ID).Error; err != nil {
		t.Fatalf("failed to reload peer: %v", err)
	}
	if !updated.Disabled {
		t.Errorf("expected reseller-quota-suspended peer to remain disabled after an unrelated usage reset")
	}
	if !updated.SuspendedByQuota || !updated.WasActiveBeforeSuspend {
		t.Errorf("expected SuspendedByQuota/WasActiveBeforeSuspend to remain untouched, got SuspendedByQuota=%v WasActiveBeforeSuspend=%v",
			updated.SuspendedByQuota, updated.WasActiveBeforeSuspend)
	}
}

// TestResetPeerUsages_BulkReenablesOnlyTrafficLimitSuspended confirms
// the bulk reset (ResetPeerUsages) applies the exact same selective
// re-enable rule per-peer: a traffic-limit-suspended peer comes back,
// a manually-disabled peer does not.
func TestResetPeerUsages_BulkReenablesOnlyTrafficLimitSuspended(t *testing.T) {
	db := openTestDB(t)

	limit := int64(1000)
	trafficLimited := model.Peer{
		UUID: "uuid-bulk-1", PeerID: "peer-bulk-1", Name: "b1",
		PrivateKey: "priv", PublicKey: "pub", Interface: "wg0",
		AllowedAddress:          "10.0.0.20/32",
		TrafficLimit:            &limit,
		DownloadUsage:           1500,
		Disabled:                true,
		SuspendedByTrafficLimit: true,
	}
	manuallyDisabled := model.Peer{
		UUID: "uuid-bulk-2", PeerID: "peer-bulk-2", Name: "b2",
		PrivateKey: "priv", PublicKey: "pub", Interface: "wg0",
		AllowedAddress:          "10.0.0.21/32",
		DownloadUsage:           500,
		Disabled:                true,
		SuspendedByTrafficLimit: false,
	}
	if err := db.Create(&trafficLimited).Error; err != nil {
		t.Fatalf("failed to create peer: %v", err)
	}
	if err := db.Create(&manuallyDisabled).Error; err != nil {
		t.Fatalf("failed to create peer: %v", err)
	}

	fake := newConfigurableFakeAdaptor()
	fake.wgPeers = []mikrotik.WireGuardPeer{
		{ID: trafficLimited.PeerID, TransferTx: "1500", TransferRx: "0"},
		{ID: manuallyDisabled.PeerID, TransferTx: "500", TransferRx: "0"},
	}
	calc := NewTrafficCalculator(db, fake, nil)

	if err := calc.ResetPeerUsages(); err != nil {
		t.Fatalf("ResetPeerUsages failed: %v", err)
	}

	var updatedTrafficLimited model.Peer
	if err := db.First(&updatedTrafficLimited, trafficLimited.ID).Error; err != nil {
		t.Fatalf("failed to reload peer: %v", err)
	}
	if updatedTrafficLimited.Disabled || updatedTrafficLimited.SuspendedByTrafficLimit {
		t.Errorf("expected traffic-limit-suspended peer to be fully re-enabled, got Disabled=%v SuspendedByTrafficLimit=%v",
			updatedTrafficLimited.Disabled, updatedTrafficLimited.SuspendedByTrafficLimit)
	}

	var updatedManual model.Peer
	if err := db.First(&updatedManual, manuallyDisabled.ID).Error; err != nil {
		t.Fatalf("failed to reload peer: %v", err)
	}
	if !updatedManual.Disabled {
		t.Errorf("expected manually-disabled peer to remain disabled after bulk reset")
	}

	fake.mu.Lock()
	defer fake.mu.Unlock()
	if got, want := fake.updates[trafficLimited.PeerID], "false"; got != want {
		t.Errorf("expected RouterOS re-enable for %s, got %q", trafficLimited.PeerID, got)
	}
	if _, touched := fake.updates[manuallyDisabled.PeerID]; touched {
		t.Errorf("expected no RouterOS call for manually-disabled peer %s", manuallyDisabled.PeerID)
	}
}
