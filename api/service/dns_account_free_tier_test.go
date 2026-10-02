package service

import (
	"errors"
	"testing"

	"github.com/maahdima/mwp/api/dataservice/model"
	"github.com/maahdima/mwp/api/http/schema"
)

func newTestDNSAccountRequest(panelID uint) *schema.CreateDNSAccountRequest {
	return &schema.CreateDNSAccountRequest{PanelID: panelID, MaxConcurrentIPs: 1}
}

// TestDNSAccountService_CreateAccount_NoLicenseLimiterIsUnaffected mirrors
// TestCreateReseller_NoLicenseLimiterIsUnaffected -- no SetLicenseLimiter
// call at all never blocks account creation.
func TestDNSAccountService_CreateAccount_NoLicenseLimiterIsUnaffected(t *testing.T) {
	svc, db, _ := newTestDNSAccountService(t, alwaysAcceptClaimHandler)

	var panel model.DNSPanel
	if err := db.First(&panel).Error; err != nil {
		t.Fatalf("failed to load test panel: %v", err)
	}

	if _, err := svc.CreateAccount(newTestDNSAccountRequest(panel.ID), nil); err != nil {
		t.Fatalf("expected account creation to succeed with no license limiter wired, got: %v", err)
	}
}

// TestDNSAccountService_CreateAccount_NotRestrictedIsUnaffected mirrors
// TestCreateReseller_NotRestrictedIsUnaffected.
func TestDNSAccountService_CreateAccount_NotRestrictedIsUnaffected(t *testing.T) {
	svc, db, _ := newTestDNSAccountService(t, alwaysAcceptClaimHandler)
	svc.SetLicenseLimiter(&fakeFreeTierLimiter{limits: map[string]int64{}})

	var panel model.DNSPanel
	if err := db.First(&panel).Error; err != nil {
		t.Fatalf("failed to load test panel: %v", err)
	}

	for i := 0; i < 3; i++ {
		if _, err := svc.CreateAccount(newTestDNSAccountRequest(panel.ID), nil); err != nil {
			t.Fatalf("expected account %d to succeed with no active cap, got: %v", i, err)
		}
	}
}

// TestDNSAccountService_CreateAccount_RestrictedUnderCapSucceeds confirms
// creation is allowed while the current account count is strictly below
// the configured cap.
func TestDNSAccountService_CreateAccount_RestrictedUnderCapSucceeds(t *testing.T) {
	svc, db, _ := newTestDNSAccountService(t, alwaysAcceptClaimHandler)
	svc.SetLicenseLimiter(&fakeFreeTierLimiter{limits: map[string]int64{"max_dns_accounts": 2}})

	var panel model.DNSPanel
	if err := db.First(&panel).Error; err != nil {
		t.Fatalf("failed to load test panel: %v", err)
	}

	if _, err := svc.CreateAccount(newTestDNSAccountRequest(panel.ID), nil); err != nil {
		t.Fatalf("expected the 1st account (under cap of 2) to succeed, got: %v", err)
	}
	if _, err := svc.CreateAccount(newTestDNSAccountRequest(panel.ID), nil); err != nil {
		t.Fatalf("expected the 2nd account (at cap of 2, but check runs before insert) to succeed, got: %v", err)
	}
}

// TestDNSAccountService_CreateAccount_RestrictedAtCapIsRejected is the core
// regression test: once the account count reaches the configured cap, the
// NEXT creation attempt must be rejected with
// ErrFreeTierDNSAccountLimitReached, and must NOT have inserted a row.
func TestDNSAccountService_CreateAccount_RestrictedAtCapIsRejected(t *testing.T) {
	svc, db, _ := newTestDNSAccountService(t, alwaysAcceptClaimHandler)
	svc.SetLicenseLimiter(&fakeFreeTierLimiter{limits: map[string]int64{"max_dns_accounts": 1}})

	var panel model.DNSPanel
	if err := db.First(&panel).Error; err != nil {
		t.Fatalf("failed to load test panel: %v", err)
	}

	if _, err := svc.CreateAccount(newTestDNSAccountRequest(panel.ID), nil); err != nil {
		t.Fatalf("expected the 1st account (under cap of 1) to succeed, got: %v", err)
	}

	_, err := svc.CreateAccount(newTestDNSAccountRequest(panel.ID), nil)
	if !errors.Is(err, ErrFreeTierDNSAccountLimitReached) {
		t.Fatalf("expected ErrFreeTierDNSAccountLimitReached once the cap is reached, got: %v", err)
	}

	var count int64
	db.Model(&model.DNSAccount{}).Count(&count)
	if count != 1 {
		t.Fatalf("expected exactly 1 account row to exist (the rejected attempt must not have inserted), got %d", count)
	}
}
