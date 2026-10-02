package service

import (
	"fmt"
	"testing"
	"time"

	"github.com/glebarez/sqlite"
	"github.com/maahdima/mwp/api/dataservice/model"
	"github.com/maahdima/mwp/api/http/schema"
	"gorm.io/gorm"
)

func openQuotaResumeTestDB(t *testing.T) *gorm.DB {
	t.Helper()

	// A private, uniquely-named in-memory DB per call (rather than the
	// literal "file::memory:?cache=shared" DSN some sibling test files in
	// this package use) — that literal DSN is process-wide shared cache, so
	// every test file using it verbatim ends up on the SAME database when
	// the package's tests run together, and their unique-indexed fixture
	// data (e.g. AllowedAddress) can collide across files. See
	// peer_scope_test.go's "10.10.10.2/32" vs. this file for a real instance
	// of that collision.
	dsn := fmt.Sprintf("file:reseller_quota_resume_%d?mode=memory&cache=shared", time.Now().UnixNano())
	db, err := gorm.Open(sqlite.Open(dsn), &gorm.Config{})
	if err != nil {
		t.Fatalf("failed to open test db: %v", err)
	}

	if err := db.AutoMigrate(&model.Reseller{}, &model.Peer{}, &model.UserManagerAccount{}); err != nil {
		t.Fatalf("failed to migrate test db: %v", err)
	}

	return db
}

// TestUpdateResellerResumesOnlyQuotaSuspendedPeers pins down the full
// suspend/resume contract: raising QuotaBytes back above UsedBytes must
// re-enable only the peers that (a) were actually disabled by the
// reseller-quota job (SuspendedByQuota) and (b) were running beforehand
// (WasActiveBeforeSuspend). A peer the admin manually disabled, and one that
// was already disabled before the quota job ever ran, must be left alone.
func TestUpdateResellerResumesOnlyQuotaSuspendedPeers(t *testing.T) {
	db := openQuotaResumeTestDB(t)
	svc := NewReseller(db, nil)

	quota := int64(1000)
	reseller := model.Reseller{
		Name:       "quota-owner",
		Username:   "quota-owner",
		QuotaBytes: &quota,
		UsedBytes:  1500, // already over quota
	}
	if err := db.Create(&reseller).Error; err != nil {
		t.Fatalf("failed to create reseller: %v", err)
	}

	// Peer A: was active, then quota-suspended by the traffic job. Must resume.
	peerA := model.Peer{
		UUID: "quota-resume-a", PeerID: "quota-resume-a", Name: "a",
		PrivateKey: "pk", PublicKey: "pub", Interface: "wg0",
		AllowedAddress: "10.10.10.1/32", Endpoint: "example.com", EndpointPort: "51820",
		ResellerID:             &reseller.ID,
		Disabled:               true,
		SuspendedByQuota:       true,
		WasActiveBeforeSuspend: true,
	}
	// Peer B: was already disabled by the admin before quota ran out. Must
	// stay disabled and untouched.
	peerB := model.Peer{
		UUID: "quota-resume-b", PeerID: "quota-resume-b", Name: "b",
		PrivateKey: "pk", PublicKey: "pub", Interface: "wg0",
		AllowedAddress: "10.10.10.2/32", Endpoint: "example.com", EndpointPort: "51820",
		ResellerID:             &reseller.ID,
		Disabled:               true,
		SuspendedByQuota:       true,
		WasActiveBeforeSuspend: false,
	}
	// Peer C: disabled, but not by the quota job at all (e.g. manual admin
	// toggle unrelated to quota). Must stay disabled and untouched.
	peerC := model.Peer{
		UUID: "quota-resume-c", PeerID: "quota-resume-c", Name: "c",
		PrivateKey: "pk", PublicKey: "pub", Interface: "wg0",
		AllowedAddress: "10.10.10.3/32", Endpoint: "example.com", EndpointPort: "51820",
		ResellerID:             &reseller.ID,
		Disabled:               true,
		SuspendedByQuota:       false,
		WasActiveBeforeSuspend: false,
	}
	for _, p := range []*model.Peer{&peerA, &peerB, &peerC} {
		if err := db.Create(p).Error; err != nil {
			t.Fatalf("failed to create peer %s: %v", p.Name, err)
		}
	}

	newQuota := int64(2000) // now above UsedBytes (1500) -> should trigger resume
	if _, err := svc.UpdateReseller(reseller.ID, &schema.UpdateResellerRequest{
		QuotaBytes: &newQuota,
	}); err != nil {
		t.Fatalf("UpdateReseller failed: %v", err)
	}

	var gotA, gotB, gotC model.Peer
	if err := db.First(&gotA, peerA.ID).Error; err != nil {
		t.Fatalf("failed to reload peer A: %v", err)
	}
	if err := db.First(&gotB, peerB.ID).Error; err != nil {
		t.Fatalf("failed to reload peer B: %v", err)
	}
	if err := db.First(&gotC, peerC.ID).Error; err != nil {
		t.Fatalf("failed to reload peer C: %v", err)
	}

	if gotA.Disabled {
		t.Errorf("peer A: expected resumed (Disabled=false), got Disabled=true")
	}
	if gotA.SuspendedByQuota || gotA.WasActiveBeforeSuspend {
		t.Errorf("peer A: expected both tracking flags cleared after resume, got SuspendedByQuota=%v WasActiveBeforeSuspend=%v", gotA.SuspendedByQuota, gotA.WasActiveBeforeSuspend)
	}

	if !gotB.Disabled {
		t.Errorf("peer B: expected to remain disabled (was never active before suspend), got Disabled=false")
	}
	if !gotC.Disabled {
		t.Errorf("peer C: expected to remain disabled (not quota-suspended), got Disabled=false")
	}
}

// TestUpdateResellerDoesNotResumeWhenStillOverQuota confirms a quota increase
// that still leaves UsedBytes over the new cap does not resume anything.
func TestUpdateResellerDoesNotResumeWhenStillOverQuota(t *testing.T) {
	db := openQuotaResumeTestDB(t)
	svc := NewReseller(db, nil)

	quota := int64(1000)
	reseller := model.Reseller{
		Name: "still-over", Username: "still-over",
		QuotaBytes: &quota, UsedBytes: 5000,
	}
	if err := db.Create(&reseller).Error; err != nil {
		t.Fatalf("failed to create reseller: %v", err)
	}

	peer := model.Peer{
		UUID: "still-over-peer", PeerID: "still-over-peer", Name: "p",
		PrivateKey: "pk", PublicKey: "pub", Interface: "wg0",
		AllowedAddress: "10.10.10.4/32", Endpoint: "example.com", EndpointPort: "51820",
		ResellerID:             &reseller.ID,
		Disabled:               true,
		SuspendedByQuota:       true,
		WasActiveBeforeSuspend: true,
	}
	if err := db.Create(&peer).Error; err != nil {
		t.Fatalf("failed to create peer: %v", err)
	}

	// Raise the quota, but not enough to clear UsedBytes (5000).
	newQuota := int64(2000)
	if _, err := svc.UpdateReseller(reseller.ID, &schema.UpdateResellerRequest{
		QuotaBytes: &newQuota,
	}); err != nil {
		t.Fatalf("UpdateReseller failed: %v", err)
	}

	var got model.Peer
	if err := db.First(&got, peer.ID).Error; err != nil {
		t.Fatalf("failed to reload peer: %v", err)
	}
	if !got.Disabled {
		t.Errorf("expected peer to remain disabled since reseller is still over the new quota, got Disabled=false")
	}
}
