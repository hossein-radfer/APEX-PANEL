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

func openBillingModeSwitchTestDB(t *testing.T) *gorm.DB {
	t.Helper()

	dsn := fmt.Sprintf("file:reseller_billing_mode_switch_%d?mode=memory&cache=shared", time.Now().UnixNano())
	db, err := gorm.Open(sqlite.Open(dsn), &gorm.Config{})
	if err != nil {
		t.Fatalf("failed to open test db: %v", err)
	}

	if err := db.AutoMigrate(
		&model.Reseller{},
		&model.Peer{},
		&model.UserManagerAccount{},
		&model.Wallet{},
	); err != nil {
		t.Fatalf("failed to migrate test db: %v", err)
	}

	return db
}

// TestUpdateReseller_VolumeToPayment_ResumesIsActiveAndQuotaSuspendedResources
// is the regression test for the confirmed gap: a reseller hard-disabled by
// the Volume-based byte-quota system (is_active=false, peers/accounts
// suspended_by_quota) must come back online when switched to Payment-based
// billing, since quota is documented as "simply unused/ignored" under that
// mode -- previously nothing ever cleared is_active or resumed those
// resources on a mode switch, leaving the reseller permanently stuck even
// though they're now billed per-GB instead.
func TestUpdateReseller_VolumeToPayment_ResumesIsActiveAndQuotaSuspendedResources(t *testing.T) {
	db := openBillingModeSwitchTestDB(t)
	svc := NewReseller(db, nil)
	svc.SetWallet(NewWallet(db))

	quota := int64(1000)
	reseller := model.Reseller{
		Name: "vol-to-pay", Username: "vol-to-pay",
		BillingMode: model.ResellerBillingModeVolume,
		QuotaBytes:  &quota,
		UsedBytes:   5000, // way over quota
		IsActive:    false,
	}
	if err := db.Create(&reseller).Error; err != nil {
		t.Fatalf("failed to create reseller: %v", err)
	}

	peer := model.Peer{
		UUID: "v2p-peer", PeerID: "v2p-peer", Name: "p",
		PrivateKey: "pk", PublicKey: "pub", Interface: "wg0",
		AllowedAddress: "10.20.10.1/32", Endpoint: "example.com", EndpointPort: "51820",
		ResellerID:             &reseller.ID,
		Disabled:               true,
		SuspendedByQuota:       true,
		WasActiveBeforeSuspend: true,
	}
	if err := db.Create(&peer).Error; err != nil {
		t.Fatalf("failed to create peer: %v", err)
	}

	account := model.UserManagerAccount{
		Username: "v2p-account", Password: "pw", RouterOSUserID: "*1",
		ResellerID:             &reseller.ID,
		Disabled:               true,
		SuspendedByQuota:       true,
		WasActiveBeforeSuspend: true,
	}
	if err := db.Create(&account).Error; err != nil {
		t.Fatalf("failed to create user manager account: %v", err)
	}

	paymentMode := model.ResellerBillingModePayment
	if _, err := svc.UpdateReseller(reseller.ID, &schema.UpdateResellerRequest{
		BillingMode: &paymentMode,
	}); err != nil {
		t.Fatalf("UpdateReseller failed: %v", err)
	}

	var gotReseller model.Reseller
	if err := db.First(&gotReseller, reseller.ID).Error; err != nil {
		t.Fatalf("failed to reload reseller: %v", err)
	}
	if !gotReseller.IsActive {
		t.Error("expected reseller IsActive=true after switching to Payment-based billing, still false")
	}
	if gotReseller.BillingMode != model.ResellerBillingModePayment {
		t.Errorf("expected BillingMode=PAYMENT, got %q", gotReseller.BillingMode)
	}

	var gotPeer model.Peer
	if err := db.First(&gotPeer, peer.ID).Error; err != nil {
		t.Fatalf("failed to reload peer: %v", err)
	}
	if gotPeer.Disabled {
		t.Error("expected peer to be resumed (Disabled=false) after switching to Payment-based billing")
	}

	var gotAccount model.UserManagerAccount
	if err := db.First(&gotAccount, account.ID).Error; err != nil {
		t.Fatalf("failed to reload user manager account: %v", err)
	}
	if gotAccount.Disabled {
		t.Error("expected user manager account to be resumed (Disabled=false) after switching to Payment-based billing")
	}

	// Wallet should have been bootstrapped immediately, not left to lazy
	// creation on the first charge.
	var wallet model.Wallet
	if err := db.Where("reseller_id = ?", reseller.ID).First(&wallet).Error; err != nil {
		t.Errorf("expected a wallet row to exist after switching to Payment-based billing, got error: %v", err)
	}
}

// TestUpdateReseller_PaymentToVolume_ClearsBillingSuspendedAndResumes is the
// regression test for the other confirmed gap: a reseller disabled under
// Payment-based billing (BillingSuspended=true, resources suspended_by_quota
// reusing the same flags) must be resumed and have BillingSuspended cleared
// when switched back to Volume-based -- otherwise the stale flag could
// spuriously resume unrelated peers if the reseller is ever switched back to
// Payment mode later.
func TestUpdateReseller_PaymentToVolume_ClearsBillingSuspendedAndResumes(t *testing.T) {
	db := openBillingModeSwitchTestDB(t)
	svc := NewReseller(db, nil)
	svc.SetWallet(NewWallet(db))

	reseller := model.Reseller{
		Name: "pay-to-vol", Username: "pay-to-vol",
		BillingMode:      model.ResellerBillingModePayment,
		BillingSuspended: true,
		IsActive:         true,
	}
	if err := db.Create(&reseller).Error; err != nil {
		t.Fatalf("failed to create reseller: %v", err)
	}

	peer := model.Peer{
		UUID: "p2v-peer", PeerID: "p2v-peer", Name: "p",
		PrivateKey: "pk", PublicKey: "pub", Interface: "wg0",
		AllowedAddress: "10.20.10.2/32", Endpoint: "example.com", EndpointPort: "51820",
		ResellerID:             &reseller.ID,
		Disabled:               true,
		SuspendedByQuota:       true,
		WasActiveBeforeSuspend: true,
	}
	if err := db.Create(&peer).Error; err != nil {
		t.Fatalf("failed to create peer: %v", err)
	}

	volumeMode := model.ResellerBillingModeVolume
	if _, err := svc.UpdateReseller(reseller.ID, &schema.UpdateResellerRequest{
		BillingMode: &volumeMode,
	}); err != nil {
		t.Fatalf("UpdateReseller failed: %v", err)
	}

	var gotReseller model.Reseller
	if err := db.First(&gotReseller, reseller.ID).Error; err != nil {
		t.Fatalf("failed to reload reseller: %v", err)
	}
	if gotReseller.BillingSuspended {
		t.Error("expected BillingSuspended to be cleared after switching to Volume-based billing")
	}

	var gotPeer model.Peer
	if err := db.First(&gotPeer, peer.ID).Error; err != nil {
		t.Fatalf("failed to reload peer: %v", err)
	}
	if gotPeer.Disabled {
		t.Error("expected peer to be resumed (Disabled=false) after switching to Volume-based billing")
	}
}

// TestUpdateReseller_SameBillingMode_DoesNotForceResumeManuallyDisabledPeer
// confirms the reconciliation logic only fires on an ACTUAL mode change --
// an UpdateReseller call that sends the same BillingMode the reseller
// already has (or omits it) must not resume a peer the admin disabled for
// unrelated reasons.
func TestUpdateReseller_SameBillingMode_DoesNotForceResumeManuallyDisabledPeer(t *testing.T) {
	db := openBillingModeSwitchTestDB(t)
	svc := NewReseller(db, nil)

	reseller := model.Reseller{
		Name: "no-switch", Username: "no-switch",
		BillingMode: model.ResellerBillingModeVolume,
		IsActive:    true,
	}
	if err := db.Create(&reseller).Error; err != nil {
		t.Fatalf("failed to create reseller: %v", err)
	}

	peer := model.Peer{
		UUID: "no-switch-peer", PeerID: "no-switch-peer", Name: "p",
		PrivateKey: "pk", PublicKey: "pub", Interface: "wg0",
		AllowedAddress: "10.20.10.3/32", Endpoint: "example.com", EndpointPort: "51820",
		ResellerID: &reseller.ID,
		Disabled:   true, // manually disabled, not quota-related
	}
	if err := db.Create(&peer).Error; err != nil {
		t.Fatalf("failed to create peer: %v", err)
	}

	sameMode := model.ResellerBillingModeVolume
	newName := "no-switch-renamed"
	if _, err := svc.UpdateReseller(reseller.ID, &schema.UpdateResellerRequest{
		Name:        &newName,
		BillingMode: &sameMode,
	}); err != nil {
		t.Fatalf("UpdateReseller failed: %v", err)
	}

	var gotPeer model.Peer
	if err := db.First(&gotPeer, peer.ID).Error; err != nil {
		t.Fatalf("failed to reload peer: %v", err)
	}
	if !gotPeer.Disabled {
		t.Error("expected manually-disabled peer to remain disabled when BillingMode is unchanged")
	}
}

// TestUpdateReseller_PaymentToVolume_LeavesOwnUsageQuotaSuspensionAlone
// confirms the reconciliation is scoped to resuming resources suspended
// by the OLD mode's own enforcement path -- it must not accidentally
// resume a peer suspended for an entirely different, still-valid reason
// after the switch (in this codebase, SuspendedByQuota/
// WasActiveBeforeSuspend is the single shared flag pair both Volume-quota
// and Payment-billing-suspension reuse, so this test's real job is
// confirming the resume path itself is unconditional on the flags alone,
// matching resumeQuotaSuspendedPeers' own pre-existing contract -- the
// point here is that this behavior is unchanged, not newly introduced).
func TestUpdateReseller_PaymentToVolume_LeavesUnrelatedDisabledPeerAlone(t *testing.T) {
	db := openBillingModeSwitchTestDB(t)
	svc := NewReseller(db, nil)
	svc.SetWallet(NewWallet(db))

	reseller := model.Reseller{
		Name: "pay-to-vol-2", Username: "pay-to-vol-2",
		BillingMode:      model.ResellerBillingModePayment,
		BillingSuspended: true,
	}
	if err := db.Create(&reseller).Error; err != nil {
		t.Fatalf("failed to create reseller: %v", err)
	}

	// Disabled by the admin directly, unrelated to billing suspension.
	unrelatedPeer := model.Peer{
		UUID: "p2v-unrelated", PeerID: "p2v-unrelated", Name: "p",
		PrivateKey: "pk", PublicKey: "pub", Interface: "wg0",
		AllowedAddress: "10.20.10.4/32", Endpoint: "example.com", EndpointPort: "51820",
		ResellerID: &reseller.ID,
		Disabled:   true,
	}
	if err := db.Create(&unrelatedPeer).Error; err != nil {
		t.Fatalf("failed to create peer: %v", err)
	}

	volumeMode := model.ResellerBillingModeVolume
	if _, err := svc.UpdateReseller(reseller.ID, &schema.UpdateResellerRequest{
		BillingMode: &volumeMode,
	}); err != nil {
		t.Fatalf("UpdateReseller failed: %v", err)
	}

	var got model.Peer
	if err := db.First(&got, unrelatedPeer.ID).Error; err != nil {
		t.Fatalf("failed to reload peer: %v", err)
	}
	if !got.Disabled {
		t.Error("expected a peer disabled for an unrelated reason (not suspended_by_quota) to remain disabled after the billing-mode switch")
	}
}
