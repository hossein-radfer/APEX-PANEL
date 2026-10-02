package service

import (
	"encoding/json"
	"fmt"
	"net/http"
	"net/http/httptest"
	"sync"
	"testing"
	"time"

	"github.com/glebarez/sqlite"
	"gorm.io/gorm"

	"github.com/maahdima/mwp/api/adaptor/mikrotik"
	"github.com/maahdima/mwp/api/common"
	"github.com/maahdima/mwp/api/dataservice/model"
	"github.com/maahdima/mwp/api/http/schema"
)

// fakeUserManagerRouterOS is a minimal /rest-shaped test server covering
// exactly what ResetUsage's re-enable path touches: PATCH
// /user-manager/user/<id> (setting disabled=false). Records every PATCH
// body received, keyed by user id, mirroring fakeRouterOS's own pattern
// in tunnel_health_level2_test.go.
type fakeUserManagerRouterOS struct {
	mu      sync.Mutex
	patches map[string]string // routeros user id -> disabled value received
}

func newFakeUserManagerRouterOS() *fakeUserManagerRouterOS {
	return &fakeUserManagerRouterOS{patches: make(map[string]string)}
}

func (f *fakeUserManagerRouterOS) server() *httptest.Server {
	return httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		f.mu.Lock()
		defer f.mu.Unlock()

		const prefix = "/rest/user-manager/user/"
		if r.Method == http.MethodPatch && len(r.URL.Path) > len(prefix) {
			userID := r.URL.Path[len(prefix):]
			var body mikrotik.UserManagerUser
			_ = json.NewDecoder(r.Body).Decode(&body)
			if body.Disabled != nil {
				f.patches[userID] = *body.Disabled
			}
			w.Header().Set("Content-Type", "application/json")
			_ = json.NewEncoder(w).Encode(mikrotik.UserManagerUser{ID: userID})
			return
		}
		w.WriteHeader(http.StatusNotFound)
	}))
}

func openUserManagerResetTestDB(t *testing.T) *gorm.DB {
	t.Helper()
	dsn := fmt.Sprintf("file:user_manager_reset_test_%d?mode=memory&cache=shared", time.Now().UnixNano())
	db, err := gorm.Open(sqlite.Open(dsn), &gorm.Config{})
	if err != nil {
		t.Fatalf("failed to open test db: %v", err)
	}
	if err := db.AutoMigrate(&model.Reseller{}, &model.UserManagerAccount{}); err != nil {
		t.Fatalf("failed to migrate test db: %v", err)
	}
	return db
}

func newTestUserManagerServiceWithFakeRouter(t *testing.T, db *gorm.DB, srv *httptest.Server) *UserManagerService {
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
	return NewUserManagerService(db, adaptor, nil)
}

// TestResetUsage_ReenablesAccountSuspendedByOwnTrafficLimit is the core
// regression test for the confirmed reported bug "ریست حجم کار نمی‌کند"
// (usage reset doesn't work): an account force-disabled by
// processUserManagerAccountUsage for exceeding its OWN TrafficLimit must
// come back online (both in the DB and on RouterOS) when an admin resets
// its usage -- previously ResetUsage only zeroed the counters and never
// touched Disabled at all, so the account stayed cut off from the
// customer's point of view even though the panel reported 0 usage.
func TestResetUsage_ReenablesAccountSuspendedByOwnTrafficLimit(t *testing.T) {
	db := openUserManagerResetTestDB(t)
	fake := newFakeUserManagerRouterOS()
	srv := fake.server()
	defer srv.Close()
	svc := newTestUserManagerServiceWithFakeRouter(t, db, srv)

	limit := int64(1000)
	account := model.UserManagerAccount{
		UUID: "uuid-1", Username: "user-1", Password: "pass",
		RouterOSUserID: "*7", Group: "default", Profile: "default",
		Protocols:               model.JoinProtocols([]model.UserManagerAccountProtocol{model.ProtocolL2TP}),
		TrafficLimit:            &limit,
		DownloadUsage:           1500,
		LastTotalDownload:       1500,
		Disabled:                true,
		SuspendedByTrafficLimit: true,
	}
	if err := db.Create(&account).Error; err != nil {
		t.Fatalf("failed to create account: %v", err)
	}

	if err := svc.ResetUsage(account.ID, nil); err != nil {
		t.Fatalf("ResetUsage failed: %v", err)
	}

	var updated model.UserManagerAccount
	if err := db.First(&updated, account.ID).Error; err != nil {
		t.Fatalf("failed to reload account: %v", err)
	}
	if updated.Disabled {
		t.Errorf("expected account to be re-enabled (Disabled=false) after reset, got Disabled=true")
	}
	if updated.SuspendedByTrafficLimit {
		t.Errorf("expected SuspendedByTrafficLimit cleared after reset, got true")
	}
	if updated.DownloadUsage != 0 || updated.UploadUsage != 0 {
		t.Errorf("expected usage zeroed after reset, got download=%d upload=%d", updated.DownloadUsage, updated.UploadUsage)
	}

	fake.mu.Lock()
	defer fake.mu.Unlock()
	if got, want := fake.patches["*7"], "false"; got != want {
		t.Errorf("expected RouterOS PATCH disabled=%q for account *7, got %q", want, got)
	}
}

// TestResetUsage_DoesNotReenableManuallyDisabledAccount confirms an
// account an admin disabled manually (Disabled=true,
// SuspendedByTrafficLimit=false) is left untouched by a usage reset --
// only a disable this job itself caused should ever be auto-cleared.
func TestResetUsage_DoesNotReenableManuallyDisabledAccount(t *testing.T) {
	db := openUserManagerResetTestDB(t)

	account := model.UserManagerAccount{
		UUID: "uuid-2", Username: "user-2", Password: "pass",
		RouterOSUserID:          "*8",
		Group:                   "default",
		Profile:                 "default",
		Protocols:               model.JoinProtocols([]model.UserManagerAccountProtocol{model.ProtocolL2TP}),
		DownloadUsage:           500,
		Disabled:                true,
		SuspendedByTrafficLimit: false,
	}
	if err := db.Create(&account).Error; err != nil {
		t.Fatalf("failed to create account: %v", err)
	}

	// nil adaptor is safe here: SuspendedByTrafficLimit is false, so
	// ResetUsage must never reach the adaptor at all for this account.
	svc := NewUserManagerService(db, nil, nil)

	if err := svc.ResetUsage(account.ID, nil); err != nil {
		t.Fatalf("ResetUsage failed: %v", err)
	}

	var updated model.UserManagerAccount
	if err := db.First(&updated, account.ID).Error; err != nil {
		t.Fatalf("failed to reload account: %v", err)
	}
	if !updated.Disabled {
		t.Errorf("expected a manually-disabled account to remain disabled after reset, got Disabled=false")
	}
	if updated.DownloadUsage != 0 {
		t.Errorf("expected usage zeroed after reset regardless of disabled state, got download=%d", updated.DownloadUsage)
	}
}

// TestResetUsage_ClearingDoesNotAffectResellerQuotaSuspension confirms
// SuspendedByQuota (the RESELLER-level pool) is completely untouched by
// ResetUsage -- the cross-contamination bug this whole fix avoids: an
// account suspended for the RESELLER's overall quota being exhausted
// must NOT be re-enabled just because an admin reset this one account's
// own displayed usage.
func TestResetUsage_ClearingDoesNotAffectResellerQuotaSuspension(t *testing.T) {
	db := openUserManagerResetTestDB(t)

	account := model.UserManagerAccount{
		UUID: "uuid-3", Username: "user-3", Password: "pass",
		RouterOSUserID:         "*9",
		Group:                  "default",
		Profile:                "default",
		Protocols:              model.JoinProtocols([]model.UserManagerAccountProtocol{model.ProtocolL2TP}),
		DownloadUsage:          500,
		Disabled:               true,
		SuspendedByQuota:       true,
		WasActiveBeforeSuspend: true,
	}
	if err := db.Create(&account).Error; err != nil {
		t.Fatalf("failed to create account: %v", err)
	}

	svc := NewUserManagerService(db, nil, nil)
	if err := svc.ResetUsage(account.ID, nil); err != nil {
		t.Fatalf("ResetUsage failed: %v", err)
	}

	var updated model.UserManagerAccount
	if err := db.First(&updated, account.ID).Error; err != nil {
		t.Fatalf("failed to reload account: %v", err)
	}
	if !updated.Disabled {
		t.Errorf("expected reseller-quota-suspended account to remain disabled after an unrelated usage reset")
	}
	if !updated.SuspendedByQuota || !updated.WasActiveBeforeSuspend {
		t.Errorf("expected SuspendedByQuota/WasActiveBeforeSuspend to remain untouched, got SuspendedByQuota=%v WasActiveBeforeSuspend=%v",
			updated.SuspendedByQuota, updated.WasActiveBeforeSuspend)
	}
}
