package service

import (
	"fmt"
	"net/http"
	"net/http/httptest"
	"testing"
	"time"

	"github.com/glebarez/sqlite"
	"gorm.io/gorm"

	"github.com/maahdima/mwp/api/adaptor/mikrotik"
	"github.com/maahdima/mwp/api/common"
	"github.com/maahdima/mwp/api/dataservice/model"
	"github.com/maahdima/mwp/api/http/schema"
)

// fakeDeleteUserManagerRouterOS is a minimal /rest-shaped test server
// covering DELETE /user-manager/user/<id> and DELETE
// /user-manager/user-profile/<id> -- lets a test choose, per RouterOS id,
// whether the router answers 404 (already gone -- the scenario this file's
// regression test reproduces) or 200 (a real, successful delete).
type fakeDeleteUserManagerRouterOS struct {
	notFoundIDs map[string]bool
}

func newFakeDeleteUserManagerRouterOS(notFoundIDs ...string) *fakeDeleteUserManagerRouterOS {
	set := make(map[string]bool, len(notFoundIDs))
	for _, id := range notFoundIDs {
		set[id] = true
	}
	return &fakeDeleteUserManagerRouterOS{notFoundIDs: set}
}

func (f *fakeDeleteUserManagerRouterOS) server() *httptest.Server {
	return httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.Method != http.MethodDelete {
			w.WriteHeader(http.StatusNotFound)
			return
		}
		const userPrefix = "/rest/user-manager/user/"
		const profilePrefix = "/rest/user-manager/user-profile/"
		var id string
		switch {
		case len(r.URL.Path) > len(userPrefix) && r.URL.Path[:len(userPrefix)] == userPrefix:
			id = r.URL.Path[len(userPrefix):]
		case len(r.URL.Path) > len(profilePrefix) && r.URL.Path[:len(profilePrefix)] == profilePrefix:
			id = r.URL.Path[len(profilePrefix):]
		default:
			w.WriteHeader(http.StatusNotFound)
			return
		}
		if f.notFoundIDs[id] {
			w.WriteHeader(http.StatusNotFound)
			_, _ = w.Write([]byte(`{"error":404,"message":"Not Found","detail":"no such item"}`))
			return
		}
		w.WriteHeader(http.StatusOK)
		_, _ = w.Write([]byte(`{}`))
	}))
}

func openUserManagerDeleteTestDB(t *testing.T) *gorm.DB {
	t.Helper()
	dsn := fmt.Sprintf("file:user_manager_delete_test_%d?mode=memory&cache=shared", time.Now().UnixNano())
	db, err := gorm.Open(sqlite.Open(dsn), &gorm.Config{})
	if err != nil {
		t.Fatalf("failed to open test db: %v", err)
	}
	if err := db.AutoMigrate(&model.Reseller{}, &model.UserManagerAccount{}); err != nil {
		t.Fatalf("failed to migrate test db: %v", err)
	}
	return db
}

func newTestUserManagerServiceWithFakeDeleteRouter(t *testing.T, db *gorm.DB, srv *httptest.Server) *UserManagerService {
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

// TestDeleteAccount_TolerateMikrotikNotFound is the regression test for a
// confirmed, reported bug: deleting a User Manager account from the panel
// AFTER it was already removed directly on RouterOS (outside the panel)
// used to fail with RouterOS's own 404, leaving the admin permanently
// stuck with a phantom DB row they could never delete. A 404 on either the
// profile-link cleanup or the user delete itself must not block the
// account being removed from the panel's own database.
func TestDeleteAccount_TolerateMikrotikNotFound(t *testing.T) {
	db := openUserManagerDeleteTestDB(t)
	fake := newFakeDeleteUserManagerRouterOS("*U1", "*P1")
	srv := fake.server()
	defer srv.Close()

	svc := newTestUserManagerServiceWithFakeDeleteRouter(t, db, srv)

	linkID := "*P1"
	account := model.UserManagerAccount{
		UUID:                  "uuid-gone",
		Username:              "user-gone",
		Password:              "pass",
		RouterOSUserID:        "*U1",
		RouterOSProfileLinkID: &linkID,
		Group:                 "default",
		Profile:               "default",
		Protocols:             model.JoinProtocols([]model.UserManagerAccountProtocol{model.ProtocolL2TP}),
	}
	if err := db.Create(&account).Error; err != nil {
		t.Fatalf("failed to create account: %v", err)
	}

	if err := svc.DeleteAccount(account.ID, nil); err != nil {
		t.Fatalf("expected DeleteAccount to tolerate a 404 from an already-gone RouterOS user/profile, got error: %v", err)
	}

	var count int64
	if err := db.Unscoped().Model(&model.UserManagerAccount{}).Where("id = ?", account.ID).Count(&count).Error; err != nil {
		t.Fatalf("failed to count accounts: %v", err)
	}
	if count != 0 {
		t.Fatalf("expected the account row to be gone from the database, but it still exists")
	}
}

// TestDeleteAccount_RealMikrotikErrorStillBlocksDelete confirms the fix is
// narrowly scoped to 404s: a genuine connectivity/permission failure (here
// simulated as a 500) must still block the delete, leaving the DB row
// intact for a retry.
func TestDeleteAccount_RealMikrotikErrorStillBlocksDelete(t *testing.T) {
	db := openUserManagerDeleteTestDB(t)
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusInternalServerError)
		_, _ = w.Write([]byte(`{"error":500,"message":"Internal Server Error"}`))
	}))
	defer srv.Close()

	svc := newTestUserManagerServiceWithFakeDeleteRouter(t, db, srv)

	account := model.UserManagerAccount{
		UUID:           "uuid-real-error",
		Username:       "user-real-error",
		Password:       "pass",
		RouterOSUserID: "*U1",
		Group:          "default",
		Profile:        "default",
		Protocols:      model.JoinProtocols([]model.UserManagerAccountProtocol{model.ProtocolL2TP}),
	}
	if err := db.Create(&account).Error; err != nil {
		t.Fatalf("failed to create account: %v", err)
	}

	if err := svc.DeleteAccount(account.ID, nil); err == nil {
		t.Fatalf("expected DeleteAccount to fail on a genuine (non-404) mikrotik error, but it succeeded")
	}

	var count int64
	if err := db.Model(&model.UserManagerAccount{}).Where("id = ?", account.ID).Count(&count).Error; err != nil {
		t.Fatalf("failed to count accounts: %v", err)
	}
	if count != 1 {
		t.Fatalf("expected the account row to survive a blocked delete, got count=%d", count)
	}
}

// TestDeleteAccount_PreservesResellerUsageTotal is the regression test for
// a confirmed, reported bug: deleting a User Manager account used to
// silently erase its historical usage from the owning reseller's
// UserManagerUsedBytes total (a live SUM over currently-existing rows),
// which would corrupt billing/reports built on that total. Deleting the
// account must fold its usage into UserManagerDeletedUsageBytes so the
// reseller's total is preserved.
func TestDeleteAccount_PreservesResellerUsageTotal(t *testing.T) {
	db := openUserManagerDeleteTestDB(t)
	fake := newFakeDeleteUserManagerRouterOS()
	srv := fake.server()
	defer srv.Close()

	svc := newTestUserManagerServiceWithFakeDeleteRouter(t, db, srv)

	reseller := model.Reseller{Name: "usage-preserve", Username: "usage-preserve", PasswordHash: "x"}
	if err := db.Create(&reseller).Error; err != nil {
		t.Fatalf("failed to create reseller: %v", err)
	}

	account := model.UserManagerAccount{
		UUID:           "uuid-usage",
		Username:       "user-usage",
		Password:       "pass",
		RouterOSUserID: "*U1",
		Group:          "default",
		Profile:        "default",
		Protocols:      model.JoinProtocols([]model.UserManagerAccountProtocol{model.ProtocolL2TP}),
		ResellerID:     &reseller.ID,
		DownloadUsage:  3 * 1024 * 1024 * 1024,
		UploadUsage:    1 * 1024 * 1024 * 1024,
	}
	if err := db.Create(&account).Error; err != nil {
		t.Fatalf("failed to create account: %v", err)
	}

	if err := svc.DeleteAccount(account.ID, nil); err != nil {
		t.Fatalf("DeleteAccount failed: %v", err)
	}

	var got model.Reseller
	if err := db.First(&got, reseller.ID).Error; err != nil {
		t.Fatalf("failed to reload reseller: %v", err)
	}
	wantCredit := int64(4 * 1024 * 1024 * 1024)
	if got.UserManagerDeletedUsageBytes != wantCredit {
		t.Fatalf("expected UserManagerDeletedUsageBytes=%d after deleting a 4GB-used account, got %d", wantCredit, got.UserManagerDeletedUsageBytes)
	}
}
