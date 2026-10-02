package service

import (
	"encoding/json"
	"fmt"
	"net/http"
	"net/http/httptest"
	"testing"
	"time"

	"github.com/glebarez/sqlite"
	"gorm.io/gorm"

	"github.com/maahdima/mwp/api/dataservice/model"
)

// newTestDNSAccountService builds a DNSAccountService against an in-memory
// SQLite db and a fake doctor-dns exit node (claimsHandler decides how
// /apex/user-claim-ip responds) -- mirrors this package's own
// newTestApplicationServiceForBilling construction convention.
func newTestDNSAccountService(t *testing.T, claimsHandler http.HandlerFunc) (*DNSAccountService, *gorm.DB, *httptest.Server) {
	t.Helper()

	dsn := fmt.Sprintf("file:dns_account_test_%d?mode=memory&cache=shared", time.Now().UnixNano())
	db, err := gorm.Open(sqlite.Open(dsn), &gorm.Config{})
	if err != nil {
		t.Fatalf("failed to open test db: %v", err)
	}
	if err := db.AutoMigrate(
		&model.DNSPanel{},
		&model.DNSAccount{},
		&model.DNSAccountAllowedCountry{},
		&model.DNSIPRegistrationLog{},
		&model.Reseller{},
		&model.IPGeoCache{},
		&model.SystemConfig{},
	); err != nil {
		t.Fatalf("failed to migrate test db: %v", err)
	}

	server := httptest.NewServer(claimsHandler)
	t.Cleanup(server.Close)

	panel := model.DNSPanel{Name: "test-panel", SaleTitle: "Smart DNS", APIBaseURL: server.URL, APIKey: "k", Status: "active"}
	if err := db.Create(&panel).Error; err != nil {
		t.Fatalf("failed to create test panel: %v", err)
	}

	panelService := NewDNSPanelService(db)
	geoIP := NewGeoIPService(db, t.TempDir(), NewSystemConfigService(db))
	return NewDNSAccountService(db, panelService, geoIP, nil), db, server
}

// alwaysAcceptClaimHandler is the fake doctor-dns response for a successful
// IP registration.
func alwaysAcceptClaimHandler(w http.ResponseWriter, r *http.Request) {
	w.Header().Set("Content-Type", "application/json")
	_ = json.NewEncoder(w).Encode(map[string]interface{}{"ok": true, "message": "ok"})
}

func createTestDNSAccount(t *testing.T, db *gorm.DB, panelID uint, opts func(*model.DNSAccount)) model.DNSAccount {
	t.Helper()
	acct := model.DNSAccount{
		UUID:             fmt.Sprintf("uuid-%d", time.Now().UnixNano()),
		PanelID:          panelID,
		ApexRef:          fmt.Sprintf("apex:test:%d", time.Now().UnixNano()),
		Status:           "active",
		IsShared:         true,
		MaxConcurrentIPs: 1,
	}
	if opts != nil {
		opts(&acct)
	}
	if err := db.Create(&acct).Error; err != nil {
		t.Fatalf("failed to create test DNS account: %v", err)
	}
	return acct
}

// TestRegisterCustomerIP_DailyLimitBlocksWithoutCallingPanel confirms the
// daily-registration cap is enforced BEFORE doctor-dns is ever called: once
// today's accepted-registration count reaches the limit, a further attempt
// is rejected locally and the fake panel never sees the request.
func TestRegisterCustomerIP_DailyLimitBlocksWithoutCallingPanel(t *testing.T) {
	panelCalled := false
	svc, db, _ := newTestDNSAccountService(t, func(w http.ResponseWriter, r *http.Request) {
		panelCalled = true
		alwaysAcceptClaimHandler(w, r)
	})

	var panel model.DNSPanel
	db.First(&panel)
	acct := createTestDNSAccount(t, db, panel.ID, func(a *model.DNSAccount) {
		a.DailyIPRegistrationLimit = 1
	})

	// First registration should succeed and call the panel.
	result, err := svc.RegisterCustomerIP(acct.UUID, "1.2.3.4")
	if err != nil {
		t.Fatalf("unexpected error on first registration: %v", err)
	}
	if !result.Ok {
		t.Fatalf("expected first registration to succeed, got: %+v", result)
	}
	if !panelCalled {
		t.Fatal("expected the panel to be called for the first registration")
	}

	panelCalled = false
	result, err = svc.RegisterCustomerIP(acct.UUID, "5.6.7.8")
	if err != nil {
		t.Fatalf("unexpected error on second registration: %v", err)
	}
	if result.Ok {
		t.Fatalf("expected second registration to be rejected by the daily cap, got: %+v", result)
	}
	if panelCalled {
		t.Fatal("daily-cap rejection must never reach the doctor-dns panel")
	}
}

// TestRegisterCustomerIP_GeoFenceFailClosedOnUnresolvedCountry confirms the
// admin's explicit choice: when a geo-fenced account's IP resolves to no
// country at all (private/unresolvable), the registration is REJECTED
// (fail-closed), not silently allowed through.
func TestRegisterCustomerIP_GeoFenceFailClosedOnUnresolvedCountry(t *testing.T) {
	panelCalled := false
	svc, db, _ := newTestDNSAccountService(t, func(w http.ResponseWriter, r *http.Request) {
		panelCalled = true
		alwaysAcceptClaimHandler(w, r)
	})

	var panel model.DNSPanel
	db.First(&panel)
	acct := createTestDNSAccount(t, db, panel.ID, nil)

	if err := db.Create(&model.DNSAccountAllowedCountry{DNSAccountID: acct.ID, CountryName: "Iran"}).Error; err != nil {
		t.Fatalf("failed to seed allowed country: %v", err)
	}

	// A private/reserved IP resolves to no country from an empty GeoIP
	// database (no .mmdb uploaded in this test), exercising the
	// fail-closed path.
	result, err := svc.RegisterCustomerIP(acct.UUID, "10.0.0.5")
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if result.Ok {
		t.Fatalf("expected geo-fenced registration with an unresolved country to be rejected, got: %+v", result)
	}
	if panelCalled {
		t.Fatal("a fail-closed geo rejection must never reach the doctor-dns panel")
	}

	var logs []model.DNSIPRegistrationLog
	db.Where("dns_account_id = ?", acct.ID).Find(&logs)
	if len(logs) != 1 || logs[0].Accepted {
		t.Fatalf("expected exactly one rejected log row, got: %+v", logs)
	}
}

// TestRegisterCustomerIP_NoCountryRestrictionAllowsThrough confirms an
// account with zero DNSAccountAllowedCountry rows has NO geo restriction at
// all (the documented "empty means unrestricted" convention, the opposite
// of the reseller-panel-access join tables' own "empty means no access").
func TestRegisterCustomerIP_NoCountryRestrictionAllowsThrough(t *testing.T) {
	svc, db, _ := newTestDNSAccountService(t, alwaysAcceptClaimHandler)

	var panel model.DNSPanel
	db.First(&panel)
	acct := createTestDNSAccount(t, db, panel.ID, nil)

	result, err := svc.RegisterCustomerIP(acct.UUID, "203.0.113.9")
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if !result.Ok {
		t.Fatalf("expected an unrestricted account to register successfully, got: %+v", result)
	}
}
