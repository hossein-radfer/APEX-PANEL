package service

import (
	"encoding/json"
	"fmt"
	"net/http"
	"net/http/httptest"
	"testing"
	"time"

	"github.com/glebarez/sqlite"
	"go.uber.org/zap"
	"gorm.io/gorm"

	"github.com/maahdima/mwp/api/adaptor/mikrotik"
	"github.com/maahdima/mwp/api/common"
	"github.com/maahdima/mwp/api/dataservice/model"
	"github.com/maahdima/mwp/api/http/schema"
)

// fakeWgPeerRouterOS is a minimal /rest-shaped test server covering exactly
// what suspendApplication's peer-disable path touches: PATCH
// /interface/wireguard/peers/<id> -- mirrors fakeUserManagerRouterOS's own
// pattern in user_manager_reset_usage_test.go, just for WireGuard peers
// instead of User Manager accounts.
type fakeWgPeerRouterOS struct{}

func (f *fakeWgPeerRouterOS) server() *httptest.Server {
	return httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		const prefix = "/rest/interface/wireguard/peers/"
		if r.Method == http.MethodPatch && len(r.URL.Path) > len(prefix) {
			peerID := r.URL.Path[len(prefix):]
			var body mikrotik.WireGuardPeer
			_ = json.NewDecoder(r.Body).Decode(&body)
			w.Header().Set("Content-Type", "application/json")
			_ = json.NewEncoder(w).Encode(mikrotik.WireGuardPeer{ID: peerID, Disabled: body.Disabled})
			return
		}
		w.WriteHeader(http.StatusNotFound)
	}))
}

// newTestWgPeerServiceWithFakeRouter mirrors
// newTestUserManagerServiceWithFakeRouter's own construction exactly, just
// for WgPeer -- used only by the test(s) that need suspendApplication to
// actually complete its peer-disable side effect against a real (faked)
// RouterOS endpoint, rather than newTestApplicationServiceForBilling's own
// peers:nil shortcut.
func newTestWgPeerServiceWithFakeRouter(t *testing.T, db *gorm.DB, srv *httptest.Server) *WgPeer {
	t.Helper()
	mwpClients := common.NewMwpClients(db)
	isSSL := false
	addr := srv.Listener.Addr().String()
	host, port := addr[:len(addr)-len(":")-len(portOf(addr))], portOf(addr)
	mwpClients.SetClient(&schema.CreateServerRequest{
		Name:      "test-server",
		IPAddress: host,
		APIPort:   port,
		IsSSL:     &isSSL,
		Username:  "admin",
		Password:  "admin",
	})
	adaptor := mikrotik.NewAdaptor(mwpClients)
	return NewWGPeer(db, adaptor, nil, nil, nil, nil, nil)
}

// newTestApplicationServiceForBilling builds an ApplicationService wired
// with only the DB and (optionally) a ResellerBillingService -- mirrors
// newTestApplicationServiceForDeviceResource's own "just what this test
// needs" convention. EnforceApplicationQuotas/sumApplicationUsage never
// touch peers/userMgr/v2ray/openVpnTmpl/auditLog for a WireGuard-only
// Application, so constructing the full NewApplicationService graph is
// unnecessary setup weight here.
func newTestApplicationServiceForBilling(t *testing.T) (*ApplicationService, *gorm.DB) {
	t.Helper()

	dsn := fmt.Sprintf("file:app_billing_test_%d?mode=memory&cache=shared", time.Now().UnixNano())
	db, err := gorm.Open(sqlite.Open(dsn), &gorm.Config{})
	if err != nil {
		t.Fatalf("failed to open test db: %v", err)
	}

	if err := db.AutoMigrate(
		&model.Application{},
		&model.ApplicationInterface{},
		&model.ApplicationUserManagerGroup{},
		&model.ApplicationXuiPanel{},
		&model.Peer{},
		&model.UserManagerAccount{},
		&model.V2RayPackage{},
		&model.V2RayPackageLocation{},
		&model.Reseller{},
		&model.Wallet{},
		&model.LedgerEntry{},
		&model.ResellerBillingPrice{},
		&model.ResellerBillingTier{},
	); err != nil {
		t.Fatalf("failed to migrate test db: %v", err)
	}

	return &ApplicationService{db: db, logger: zap.NewNop()}, db
}

// TestEnforceApplicationQuotas_ChargesPaymentResellerForUsageDelta confirms
// the core new wiring: an Application owned by a Payment-based (پرداختی)
// reseller with a configured Toman/GB price gets that reseller's wallet
// debited for exactly the newly-accrued usage delta on each tick -- not the
// full running total (which would double-charge every subsequent tick).
func TestEnforceApplicationQuotas_ChargesPaymentResellerForUsageDelta(t *testing.T) {
	svc, db := newTestApplicationServiceForBilling(t)
	svc.billing = NewResellerBillingService(db, NewWallet(db))

	reseller := newPaymentReseller(t, db, "app-billing-reseller")
	if err := db.Create(&model.ResellerBillingPrice{
		ResellerID: reseller.ID, Product: model.ResellerBillingProductApplication, LocationKey: "", PricePerGBAmount: 500,
	}).Error; err != nil {
		t.Fatalf("failed to seed price: %v", err)
	}

	app := model.Application{
		ResellerID: &reseller.ID, Name: "app-1", AppUsername: "app-billing-1",
		TotalVolumeBytes: 100 * bytesPerGB, UsedBytes: 0,
	}
	if err := db.Create(&app).Error; err != nil {
		t.Fatalf("failed to create application: %v", err)
	}

	peer := model.Peer{
		UUID: "peer-uuid-1", PeerID: "peer-1", Name: "peer-1",
		PrivateKey: "priv", PublicKey: "pub", Interface: "wg0",
		AllowedAddress: "10.0.0.2/32", Endpoint: "example.com", EndpointPort: "51820",
		DownloadUsage: 2 * bytesPerGB, UploadUsage: 0,
	}
	if err := db.Create(&peer).Error; err != nil {
		t.Fatalf("failed to create peer: %v", err)
	}
	if err := db.Create(&model.ApplicationInterface{ApplicationID: app.ID, InterfaceID: 1, PeerID: &peer.ID}).Error; err != nil {
		t.Fatalf("failed to link peer to application: %v", err)
	}

	svc.EnforceApplicationQuotas()

	// First tick: delta is 2GB - 0 = 2GB, at 500 Toman/GB = 1000 Toman.
	if got := walletBalance(t, db, reseller.ID); got != -1000 {
		t.Fatalf("expected wallet balance -1000 after first tick (2GB * 500 Toman/GB), got %d", got)
	}

	// Simulate more usage accruing before the next tick.
	if err := db.Model(&peer).Update("download_usage", 5*bytesPerGB).Error; err != nil {
		t.Fatalf("failed to bump peer usage: %v", err)
	}

	svc.EnforceApplicationQuotas()

	// Second tick: delta is 5GB - 2GB = 3GB, at 500 Toman/GB = 1500 Toman
	// MORE -- total should be -1000 + -1500 = -2500, never re-charging the
	// first 2GB again.
	if got := walletBalance(t, db, reseller.ID); got != -2500 {
		t.Fatalf("expected wallet balance -2500 after second tick (only the 3GB delta charged), got %d", got)
	}
}

// TestEnforceApplicationQuotas_VolumeBasedResellerIsNeverCharged confirms
// ChargeUsage's own no-op guarantee holds through this new call site too --
// an Application owned by the original, still-default Volume-based
// (حجمی) reseller must never have its wallet touched, even with billing
// wired and a price row present (e.g. left over from a prior mode switch).
func TestEnforceApplicationQuotas_VolumeBasedResellerIsNeverCharged(t *testing.T) {
	svc, db := newTestApplicationServiceForBilling(t)
	svc.billing = NewResellerBillingService(db, NewWallet(db))

	reseller := model.Reseller{Name: "volume-reseller", BillingMode: model.ResellerBillingModeVolume}
	if err := db.Create(&reseller).Error; err != nil {
		t.Fatalf("failed to create reseller: %v", err)
	}
	if err := db.Create(&model.ResellerBillingPrice{
		ResellerID: reseller.ID, Product: model.ResellerBillingProductApplication, LocationKey: "", PricePerGBAmount: 500,
	}).Error; err != nil {
		t.Fatalf("failed to seed price: %v", err)
	}

	app := model.Application{
		ResellerID: &reseller.ID, Name: "app-1", AppUsername: "app-billing-2",
		TotalVolumeBytes: 100 * bytesPerGB, UsedBytes: 0,
	}
	if err := db.Create(&app).Error; err != nil {
		t.Fatalf("failed to create application: %v", err)
	}

	peer := model.Peer{
		UUID: "peer-uuid-2", PeerID: "peer-2", Name: "peer-2",
		PrivateKey: "priv", PublicKey: "pub", Interface: "wg0",
		AllowedAddress: "10.0.0.3/32", Endpoint: "example.com", EndpointPort: "51820",
		DownloadUsage: 2 * bytesPerGB, UploadUsage: 0,
	}
	if err := db.Create(&peer).Error; err != nil {
		t.Fatalf("failed to create peer: %v", err)
	}
	if err := db.Create(&model.ApplicationInterface{ApplicationID: app.ID, InterfaceID: 1, PeerID: &peer.ID}).Error; err != nil {
		t.Fatalf("failed to link peer to application: %v", err)
	}

	svc.EnforceApplicationQuotas()

	if got := walletBalance(t, db, reseller.ID); got != 0 {
		t.Fatalf("expected a Volume-based reseller's wallet to never be touched, got balance %d", got)
	}
}

// TestEnforceApplicationQuotas_PaymentResellerNeverVolumeSuspended is the
// regression test for a confirmed, reported production bug: every
// Payment-mode (پرداختی) reseller's users got force-disabled within
// moments of connecting, even though the reseller's wallet had balance.
// Root cause was this same leftover-quota-field bug already fixed for
// WireGuard/User Manager/V2Ray (see applyResellerQuota/
// applyUserManagerResellerQuota/applyResellerV2RayQuota's own doc
// comments) but missed here: EnforceApplicationQuotas' overQuota check
// ran unconditionally against TotalVolumeBytes, so an Application whose
// TotalVolumeBytes was left over from before its reseller switched to
// Payment billing (here deliberately tiny, 1GB, to simulate that stale
// leftover) got suspended -- disabling every peer/account/package it
// owns -- the instant real usage crossed it, regardless of the wallet
// debit (which this test's unlimited-debt Postpaid reseller always
// succeeds) ever being rejected.
func TestEnforceApplicationQuotas_PaymentResellerNeverVolumeSuspended(t *testing.T) {
	svc, db := newTestApplicationServiceForBilling(t)
	svc.billing = NewResellerBillingService(db, NewWallet(db))

	reseller := newPaymentReseller(t, db, "app-payment-no-suspend-reseller")

	app := model.Application{
		ResellerID: &reseller.ID, Name: "app-1", AppUsername: "app-billing-no-suspend",
		TotalVolumeBytes: 1 * bytesPerGB, UsedBytes: 0, Status: "active",
	}
	if err := db.Create(&app).Error; err != nil {
		t.Fatalf("failed to create application: %v", err)
	}

	peer := model.Peer{
		UUID: "peer-uuid-no-suspend", PeerID: "peer-no-suspend", Name: "peer-no-suspend",
		PrivateKey: "priv", PublicKey: "pub", Interface: "wg0",
		AllowedAddress: "10.0.0.5/32", Endpoint: "example.com", EndpointPort: "51820",
		// 5GB, well past the 1GB TotalVolumeBytes above.
		DownloadUsage: 5 * bytesPerGB, UploadUsage: 0,
	}
	if err := db.Create(&peer).Error; err != nil {
		t.Fatalf("failed to create peer: %v", err)
	}
	if err := db.Create(&model.ApplicationInterface{ApplicationID: app.ID, InterfaceID: 1, PeerID: &peer.ID}).Error; err != nil {
		t.Fatalf("failed to link peer to application: %v", err)
	}

	svc.EnforceApplicationQuotas()

	var updatedApp model.Application
	if err := db.First(&updatedApp, app.ID).Error; err != nil {
		t.Fatalf("failed to reload application: %v", err)
	}
	if updatedApp.SuspendedByQuota {
		t.Fatalf("expected Payment-mode reseller's application to never be volume-suspended, but SuspendedByQuota=true")
	}
	if updatedApp.Status != "active" {
		t.Fatalf("expected application status to remain active, got %q", updatedApp.Status)
	}

	var updatedPeer model.Peer
	if err := db.First(&updatedPeer, peer.ID).Error; err != nil {
		t.Fatalf("failed to reload peer: %v", err)
	}
	if updatedPeer.Disabled {
		t.Fatalf("expected peer to remain enabled under Payment billing despite exceeding stale TotalVolumeBytes")
	}
}

// TestEnforceApplicationQuotas_VolumeResellerStillSuspendedOverQuota confirms
// the fix above didn't remove volume enforcement for the original,
// still-default Volume-based (حجمی) reseller -- TotalVolumeBytes must keep
// working exactly as before for anyone not on Payment billing.
func TestEnforceApplicationQuotas_VolumeResellerStillSuspendedOverQuota(t *testing.T) {
	svc, db := newTestApplicationServiceForBilling(t)
	svc.billing = NewResellerBillingService(db, NewWallet(db))

	fakeRouter := (&fakeWgPeerRouterOS{}).server()
	defer fakeRouter.Close()
	svc.peers = newTestWgPeerServiceWithFakeRouter(t, db, fakeRouter)

	reseller := model.Reseller{Name: "volume-reseller-suspend", BillingMode: model.ResellerBillingModeVolume}
	if err := db.Create(&reseller).Error; err != nil {
		t.Fatalf("failed to create reseller: %v", err)
	}

	app := model.Application{
		ResellerID: &reseller.ID, Name: "app-1", AppUsername: "app-billing-volume-suspend",
		TotalVolumeBytes: 1 * bytesPerGB, UsedBytes: 0, Status: "active",
	}
	if err := db.Create(&app).Error; err != nil {
		t.Fatalf("failed to create application: %v", err)
	}

	peer := model.Peer{
		ResellerID: &reseller.ID,
		UUID:       "peer-uuid-volume-suspend", PeerID: "peer-volume-suspend", Name: "peer-volume-suspend",
		PrivateKey: "priv", PublicKey: "pub", Interface: "wg0",
		AllowedAddress: "10.0.0.6/32", Endpoint: "example.com", EndpointPort: "51820",
		DownloadUsage: 5 * bytesPerGB, UploadUsage: 0,
	}
	if err := db.Create(&peer).Error; err != nil {
		t.Fatalf("failed to create peer: %v", err)
	}
	if err := db.Create(&model.ApplicationInterface{ApplicationID: app.ID, InterfaceID: 1, PeerID: &peer.ID}).Error; err != nil {
		t.Fatalf("failed to link peer to application: %v", err)
	}

	svc.EnforceApplicationQuotas()

	var updatedApp model.Application
	if err := db.First(&updatedApp, app.ID).Error; err != nil {
		t.Fatalf("failed to reload application: %v", err)
	}
	if !updatedApp.SuspendedByQuota {
		t.Fatalf("expected Volume-mode reseller's application to still be suspended over quota")
	}
}

// TestEnforceApplicationQuotas_NilBillingIsSafeNoOp confirms
// EnforceApplicationQuotas still runs correctly (usage bookkeeping/quota
// suspension unaffected) when billing was never wired via SetBilling --
// e.g. any deployment/test path that constructs ApplicationService without
// calling it, matching NewApplicationService's own nil-by-default field.
func TestEnforceApplicationQuotas_NilBillingIsSafeNoOp(t *testing.T) {
	svc, db := newTestApplicationServiceForBilling(t)
	// svc.billing intentionally left nil.

	resellerID := uint(1)
	app := model.Application{
		ResellerID: &resellerID, Name: "app-1", AppUsername: "app-billing-3",
		TotalVolumeBytes: 100 * bytesPerGB, UsedBytes: 0,
	}
	if err := db.Create(&app).Error; err != nil {
		t.Fatalf("failed to create application: %v", err)
	}

	peer := model.Peer{
		UUID: "peer-uuid-3", PeerID: "peer-3", Name: "peer-3",
		PrivateKey: "priv", PublicKey: "pub", Interface: "wg0",
		AllowedAddress: "10.0.0.4/32", Endpoint: "example.com", EndpointPort: "51820",
		DownloadUsage: 2 * bytesPerGB, UploadUsage: 0,
	}
	if err := db.Create(&peer).Error; err != nil {
		t.Fatalf("failed to create peer: %v", err)
	}
	if err := db.Create(&model.ApplicationInterface{ApplicationID: app.ID, InterfaceID: 1, PeerID: &peer.ID}).Error; err != nil {
		t.Fatalf("failed to link peer to application: %v", err)
	}

	svc.EnforceApplicationQuotas()

	var updated model.Application
	if err := db.First(&updated, app.ID).Error; err != nil {
		t.Fatalf("failed to reload application: %v", err)
	}
	if updated.UsedBytes != 2*bytesPerGB {
		t.Fatalf("expected used_bytes to still be tracked without billing wired, got %d", updated.UsedBytes)
	}
}
