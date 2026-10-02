package service

import (
	"errors"
	"fmt"
	"testing"
	"time"

	"github.com/glebarez/sqlite"
	"go.uber.org/zap"
	"gorm.io/gorm"

	"github.com/maahdima/mwp/api/dataservice/model"
	"github.com/maahdima/mwp/api/http/schema"
)

// newTestWgPeerForFreeTier builds a bare WgPeer{db, logger} against an
// in-memory sqlite db -- mirrors newTestApplicationServiceForPermission's
// own "just what this test needs" convention. The free-tier cap check in
// CreatePeer runs before getInterface/RouterOS provisioning, so these tests
// never need a real (or faked) MikroTik router.
func newTestWgPeerForFreeTier(t *testing.T) (*WgPeer, *gorm.DB) {
	t.Helper()

	dsn := fmt.Sprintf("file:peer_free_tier_test_%d?mode=memory&cache=shared", time.Now().UnixNano())
	db, err := gorm.Open(sqlite.Open(dsn), &gorm.Config{})
	if err != nil {
		t.Fatalf("failed to open test db: %v", err)
	}

	if err := db.AutoMigrate(&model.Peer{}, &model.Interface{}, &model.Reseller{}, &model.ResellerInterface{}); err != nil {
		t.Fatalf("failed to migrate test db: %v", err)
	}

	return &WgPeer{db: db, logger: zap.NewNop()}, db
}

func newTestCreatePeerRequest() *schema.CreatePeerRequest {
	return &schema.CreatePeerRequest{InterfaceId: 999, AllowedAddress: "10.0.0.2/32"}
}

// TestCreatePeer_NoLicenseLimiterIsUnaffected mirrors
// TestCreateReseller_NoLicenseLimiterIsUnaffected -- no SetLicenseLimiter
// call at all never blocks at the free-tier check (the request still fails
// afterward on the missing interface, proving the cap check itself was
// skipped rather than accidentally passing).
func TestCreatePeer_NoLicenseLimiterIsUnaffected(t *testing.T) {
	w, _ := newTestWgPeerForFreeTier(t)

	_, err := w.CreatePeer(newTestCreatePeerRequest(), nil)
	if err == nil || errors.Is(err, ErrFreeTierPeerLimitReached) {
		t.Fatalf("expected failure past the free-tier check (missing interface), got: %v", err)
	}
}

// TestCreatePeer_NotRestrictedIsUnaffected mirrors
// TestCreateReseller_NotRestrictedIsUnaffected -- GetFreeTierLimit
// reporting ok=false never blocks at the free-tier check.
func TestCreatePeer_NotRestrictedIsUnaffected(t *testing.T) {
	w, _ := newTestWgPeerForFreeTier(t)
	w.SetLicenseLimiter(&fakeFreeTierLimiter{limits: map[string]int64{}})

	_, err := w.CreatePeer(newTestCreatePeerRequest(), nil)
	if err == nil || errors.Is(err, ErrFreeTierPeerLimitReached) {
		t.Fatalf("expected failure past the free-tier check (missing interface), got: %v", err)
	}
}

// TestCreatePeer_RestrictedUnderCapPassesFreeTierCheck confirms the cap
// check itself lets a request through when the current peer count is
// strictly below the configured cap (the request still fails afterward on
// the missing interface -- this test only isolates the free-tier gate).
func TestCreatePeer_RestrictedUnderCapPassesFreeTierCheck(t *testing.T) {
	w, _ := newTestWgPeerForFreeTier(t)
	w.SetLicenseLimiter(&fakeFreeTierLimiter{limits: map[string]int64{"max_peers": 5}})

	_, err := w.CreatePeer(newTestCreatePeerRequest(), nil)
	if err == nil || errors.Is(err, ErrFreeTierPeerLimitReached) {
		t.Fatalf("expected failure past the free-tier check (missing interface), got: %v", err)
	}
}

// TestCreatePeer_RestrictedAtCapIsRejected is the core regression test:
// once the peer count reaches the configured cap, CreatePeer must reject
// with ErrFreeTierPeerLimitReached before ever calling getInterface.
func TestCreatePeer_RestrictedAtCapIsRejected(t *testing.T) {
	w, db := newTestWgPeerForFreeTier(t)
	w.SetLicenseLimiter(&fakeFreeTierLimiter{limits: map[string]int64{"max_peers": 1}})

	existing := model.Peer{
		UUID: "peer-uuid-cap", PeerID: "peer-cap", Name: "peer-cap",
		PrivateKey: "priv", PublicKey: "pub", Interface: "wg0",
		AllowedAddress: "10.0.0.5/32", Endpoint: "example.com", EndpointPort: "51820",
	}
	if err := db.Create(&existing).Error; err != nil {
		t.Fatalf("failed to seed existing peer: %v", err)
	}

	_, err := w.CreatePeer(newTestCreatePeerRequest(), nil)
	if !errors.Is(err, ErrFreeTierPeerLimitReached) {
		t.Fatalf("expected ErrFreeTierPeerLimitReached once the cap is reached, got: %v", err)
	}

	var count int64
	db.Model(&model.Peer{}).Count(&count)
	if count != 1 {
		t.Fatalf("expected exactly 1 peer row to exist (the rejected attempt must not have inserted), got %d", count)
	}
}
