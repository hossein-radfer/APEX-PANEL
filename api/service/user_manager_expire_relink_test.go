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

// fakeExpireRelinkRouterOS covers the three RouterOS calls a profile
// re-link makes: DELETE the old link, PUT (create) a new one, POST
// (activate) it. Records how many times each happens and hands out
// incrementing link ids so a test can tell a fresh link from the old one.
type fakeExpireRelinkRouterOS struct {
	mu            sync.Mutex
	deletedLinks  []string
	createdLinks  []string
	activatedLink []string
	nextLinkID    int
}

func newFakeExpireRelinkRouterOS() *fakeExpireRelinkRouterOS {
	return &fakeExpireRelinkRouterOS{nextLinkID: 100}
}

func (f *fakeExpireRelinkRouterOS) server() *httptest.Server {
	return httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		f.mu.Lock()
		defer f.mu.Unlock()

		const profilePath = "/rest/user-manager/user-profile"
		switch {
		case r.Method == http.MethodDelete && len(r.URL.Path) > len(profilePath)+1:
			id := r.URL.Path[len(profilePath)+1:]
			f.deletedLinks = append(f.deletedLinks, id)
			w.WriteHeader(http.StatusOK)
			_, _ = w.Write([]byte(`{}`))
			return
		case r.Method == http.MethodPut && r.URL.Path == profilePath:
			f.nextLinkID++
			id := fmt.Sprintf("*%d", f.nextLinkID)
			f.createdLinks = append(f.createdLinks, id)
			w.Header().Set("Content-Type", "application/json")
			_ = json.NewEncoder(w).Encode(mikrotik.UserManagerUserProfile{ID: id})
			return
		case r.Method == http.MethodPost && r.URL.Path == profilePath+"/activate-user-profile":
			var body map[string]string
			_ = json.NewDecoder(r.Body).Decode(&body)
			f.activatedLink = append(f.activatedLink, body["numbers"])
			w.WriteHeader(http.StatusOK)
			_, _ = w.Write([]byte(`{}`))
			return
		}
		w.WriteHeader(http.StatusNotFound)
	}))
}

func openExpireRelinkTestDB(t *testing.T) *gorm.DB {
	t.Helper()
	dsn := fmt.Sprintf("file:user_manager_expire_relink_test_%d?mode=memory&cache=shared", time.Now().UnixNano())
	db, err := gorm.Open(sqlite.Open(dsn), &gorm.Config{})
	if err != nil {
		t.Fatalf("failed to open test db: %v", err)
	}
	if err := db.AutoMigrate(&model.Reseller{}, &model.UserManagerAccount{}); err != nil {
		t.Fatalf("failed to migrate test db: %v", err)
	}
	return db
}

func newTestUserManagerServiceWithFakeExpireRouter(t *testing.T, db *gorm.DB, srv *httptest.Server) *UserManagerService {
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

// TestUpdateAccount_ExtendingExpireTimeReLinksProfile is the regression
// test for a confirmed, reported bug: RouterOS User Manager profiles carry
// their OWN validity window (e.g. a fixed 30-day profile), started at the
// original ActivateUserManagerUserProfile call -- entirely separate from
// ExpireTime, this panel's own display-only bookkeeping. Extending
// ExpireTime alone (with no RouterOS call) left RouterOS's own clock
// running unaffected, so a customer could still be locked out at the
// ORIGINAL profile's expiry even though the panel showed more time
// remaining. Extending the expiry must re-provision the SAME profile
// (delete the old link, create+activate a fresh one) so RouterOS's own
// validity window restarts too.
func TestUpdateAccount_ExtendingExpireTimeReLinksProfile(t *testing.T) {
	db := openExpireRelinkTestDB(t)
	fake := newFakeExpireRelinkRouterOS()
	srv := fake.server()
	defer srv.Close()

	svc := newTestUserManagerServiceWithFakeExpireRouter(t, db, srv)

	oldLinkID := "*50"
	oldExpire := "2026-01-01"
	account := model.UserManagerAccount{
		UUID:                  "uuid-expire",
		Username:              "user-expire",
		Password:              "pass",
		RouterOSUserID:        "*U1",
		RouterOSProfileLinkID: &oldLinkID,
		Group:                 "default",
		Profile:               "30day",
		Protocols:             model.JoinProtocols([]model.UserManagerAccountProtocol{model.ProtocolL2TP}),
		ExpireTime:            &oldExpire,
	}
	if err := db.Create(&account).Error; err != nil {
		t.Fatalf("failed to create account: %v", err)
	}

	newExpire := "2026-06-01"
	_, err := svc.UpdateAccount(account.ID, &schema.UpdateUserManagerAccountRequest{
		ExpireTime: &newExpire,
	}, nil)
	if err != nil {
		t.Fatalf("UpdateAccount failed: %v", err)
	}

	fake.mu.Lock()
	defer fake.mu.Unlock()

	if len(fake.deletedLinks) != 1 || fake.deletedLinks[0] != oldLinkID {
		t.Fatalf("expected the OLD profile link (%q) to be deleted exactly once, got deletedLinks=%v", oldLinkID, fake.deletedLinks)
	}
	if len(fake.createdLinks) != 1 {
		t.Fatalf("expected exactly one new profile link to be created, got %v", fake.createdLinks)
	}
	if len(fake.activatedLink) != 1 || fake.activatedLink[0] != fake.createdLinks[0] {
		t.Fatalf("expected the NEW link to be activated, got activated=%v created=%v", fake.activatedLink, fake.createdLinks)
	}

	var got model.UserManagerAccount
	if err := db.First(&got, account.ID).Error; err != nil {
		t.Fatalf("failed to reload account: %v", err)
	}
	if got.RouterOSProfileLinkID == nil || *got.RouterOSProfileLinkID != fake.createdLinks[0] {
		t.Fatalf("expected account.RouterOSProfileLinkID to be updated to the new link id %q, got %v", fake.createdLinks[0], got.RouterOSProfileLinkID)
	}
	if got.ExpireTime == nil || *got.ExpireTime != newExpire {
		t.Fatalf("expected ExpireTime to be updated to %q, got %v", newExpire, got.ExpireTime)
	}
	if got.Profile != "30day" {
		t.Fatalf("expected Profile to stay unchanged at \"30day\", got %q", got.Profile)
	}
}

// TestUpdateAccount_UnchangedExpireTimeSkipsRelink confirms the fix is
// scoped to a genuine extension: re-sending the SAME ExpireTime value
// (e.g. a form re-submit that doesn't actually change anything) must not
// pay the cost of a needless RouterOS re-link.
func TestUpdateAccount_UnchangedExpireTimeSkipsRelink(t *testing.T) {
	db := openExpireRelinkTestDB(t)
	fake := newFakeExpireRelinkRouterOS()
	srv := fake.server()
	defer srv.Close()

	svc := newTestUserManagerServiceWithFakeExpireRouter(t, db, srv)

	linkID := "*50"
	expire := "2026-01-01"
	account := model.UserManagerAccount{
		UUID:                  "uuid-unchanged",
		Username:              "user-unchanged",
		Password:              "pass",
		RouterOSUserID:        "*U1",
		RouterOSProfileLinkID: &linkID,
		Group:                 "default",
		Profile:               "30day",
		Protocols:             model.JoinProtocols([]model.UserManagerAccountProtocol{model.ProtocolL2TP}),
		ExpireTime:            &expire,
	}
	if err := db.Create(&account).Error; err != nil {
		t.Fatalf("failed to create account: %v", err)
	}

	sameExpire := "2026-01-01"
	if _, err := svc.UpdateAccount(account.ID, &schema.UpdateUserManagerAccountRequest{
		ExpireTime: &sameExpire,
	}, nil); err != nil {
		t.Fatalf("UpdateAccount failed: %v", err)
	}

	fake.mu.Lock()
	defer fake.mu.Unlock()
	if len(fake.deletedLinks) != 0 || len(fake.createdLinks) != 0 {
		t.Fatalf("expected no profile re-link when ExpireTime is unchanged, got deleted=%v created=%v", fake.deletedLinks, fake.createdLinks)
	}
}
