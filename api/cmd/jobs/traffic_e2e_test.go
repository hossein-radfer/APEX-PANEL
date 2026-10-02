package traffic

import (
	"context"
	"testing"

	"github.com/maahdima/mwp/api/adaptor/mikrotik"
	"github.com/maahdima/mwp/api/dataservice/model"
)

// fakeAdaptor implements minimal methods used by Calculator in tests
type fakeAdaptor struct{}

func (f *fakeAdaptor) FetchWgPeer(ctx context.Context, peerID string) (*mikrotik.WireGuardPeer, error) {
	// Return a peer with large TransferTx to simulate traffic
	wg := &mikrotik.WireGuardPeer{
		ID:         peerID,
		TransferTx: "200",
		TransferRx: "0",
	}
	return wg, nil
}

func (f *fakeAdaptor) FetchWgPeers(ctx context.Context) ([]mikrotik.WireGuardPeer, error) {
	return []mikrotik.WireGuardPeer{}, nil
}

func (f *fakeAdaptor) FetchInterface(ctx context.Context, interfaceID string) (*mikrotik.Interface, error) {
	// return empty interface counters
	return &mikrotik.Interface{TxByte: "0", RxByte: "0"}, nil
}

func (f *fakeAdaptor) UpdateWgPeer(ctx context.Context, peerID string, wgPeer mikrotik.WireGuardPeer) (*mikrotik.WireGuardPeer, error) {
	return &wgPeer, nil
}

// implement other methods as no-ops to satisfy calls
func (f *fakeAdaptor) UpdateScheduler(ctx context.Context, id string, s mikrotik.Scheduler) (*mikrotik.Scheduler, error) {
	return &s, nil
}
func (f *fakeAdaptor) UpdateSimpleQueue(ctx context.Context, id string, q mikrotik.Queue) (*mikrotik.Queue, error) {
	return &q, nil
}

// MonitorUserManagerUser/SetUserManagerUserDisabled were added to the
// MikrotikAdaptor interface for User Manager traffic polling/quota
// enforcement after this fake was first written -- zero-value results are
// fine since none of the traffic-calculator tests in this file exercise
// the User Manager path.
func (f *fakeAdaptor) MonitorUserManagerUser(ctx context.Context, userID string) (*mikrotik.UserManagerMonitorResult, error) {
	return &mikrotik.UserManagerMonitorResult{}, nil
}

func (f *fakeAdaptor) SetUserManagerUserDisabled(ctx context.Context, userID string, disabled string) (*mikrotik.UserManagerUser, error) {
	return &mikrotik.UserManagerUser{}, nil
}

// openTestDB helper is defined in traffic_test.go and reused here.

func TestTrafficEndToEnd_DisablesResellerOnQuota(t *testing.T) {
	db := openTestDB(t)

	// create reseller with quota 100 bytes
	quota := int64(100)
	res := model.Reseller{Name: "e2e-reseller", QuotaBytes: &quota, UsedBytes: 0, IsActive: true}
	if err := db.Create(&res).Error; err != nil {
		t.Fatalf("failed to create reseller: %v", err)
	}

	// create peer linked to reseller
	peer := model.Peer{
		UUID:           "e2e-uuid",
		PeerID:         "e2e-peer",
		Name:           "peer-e2e",
		PrivateKey:     "priv",
		PublicKey:      "pub",
		Interface:      "wg0",
		AllowedAddress: "10.0.0.5/32",
		ResellerID:     &res.ID,
		Disabled:       false,
		LastTx:         0,
		LastRx:         0,
	}
	if err := db.Create(&peer).Error; err != nil {
		t.Fatalf("failed to create peer: %v", err)
	}

	// create calculator with fake adaptor
	calc := NewTrafficCalculator(db, &fakeAdaptor{}, nil)

	// run calculation
	calc.CalculatePeerTraffic()

	// fetch reseller and peer
	var updatedRes model.Reseller
	if err := db.First(&updatedRes, "id = ?", res.ID).Error; err != nil {
		t.Fatalf("failed to fetch reseller: %v", err)
	}
	if updatedRes.IsActive {
		t.Fatalf("expected reseller to be disabled after quota exceeded")
	}

	var updatedPeer model.Peer
	if err := db.First(&updatedPeer, "id = ?", peer.ID).Error; err != nil {
		t.Fatalf("failed to fetch peer: %v", err)
	}
	if !updatedPeer.Disabled {
		t.Fatalf("expected peer to be disabled after quota exceeded")
	}
}
