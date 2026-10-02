package service

import (
	"testing"

	"github.com/maahdima/mwp/api/dataservice/model"
	"github.com/maahdima/mwp/api/http/schema"
)

// TestEnsureResellerUnderUserManagerAccountLimit_Unlimited confirms a nil
// UserManagerMaxAccounts means unlimited, mirroring ensureResellerUnderPeerLimit.
func TestEnsureResellerUnderUserManagerAccountLimit_Unlimited(t *testing.T) {
	svc, db := newTestUserManagerService(t)

	reseller := model.Reseller{Name: "unlimited", Username: "unlimited-um", PasswordHash: "x", CanCreateUserManagerAccounts: true}
	if err := db.Create(&reseller).Error; err != nil {
		t.Fatalf("failed to create reseller: %v", err)
	}

	for i := 0; i < 5; i++ {
		account := model.UserManagerAccount{
			UUID:           "uuid-unlimited",
			Username:       "user-unlimited",
			Password:       "pass",
			RouterOSUserID: "*1",
			Group:          "default",
			Profile:        "default",
			Protocols:      model.JoinProtocols([]model.UserManagerAccountProtocol{model.ProtocolL2TP}),
			ResellerID:     &reseller.ID,
		}
		account.UUID = account.UUID + string(rune('a'+i))
		account.Username = account.Username + string(rune('a'+i))
		if err := db.Create(&account).Error; err != nil {
			t.Fatalf("failed to create account: %v", err)
		}
	}

	if err := svc.ensureResellerUnderUserManagerAccountLimit(reseller.ID); err != nil {
		t.Fatalf("expected no error for unlimited reseller, got %v", err)
	}
}

// TestEnsureResellerUnderUserManagerAccountLimit_Enforced confirms the cap
// is enforced once MaxAccounts is set, and is a SEPARATE limit from
// MaxPeers (this test never touches model.Peer at all).
func TestEnsureResellerUnderUserManagerAccountLimit_Enforced(t *testing.T) {
	svc, db := newTestUserManagerService(t)

	maxAccounts := 2
	reseller := model.Reseller{
		Name: "limited", Username: "limited-um", PasswordHash: "x",
		CanCreateUserManagerAccounts: true,
		UserManagerMaxAccounts:       &maxAccounts,
	}
	if err := db.Create(&reseller).Error; err != nil {
		t.Fatalf("failed to create reseller: %v", err)
	}

	if err := svc.ensureResellerUnderUserManagerAccountLimit(reseller.ID); err != nil {
		t.Fatalf("expected no error under limit, got %v", err)
	}

	for i := 0; i < 2; i++ {
		account := model.UserManagerAccount{
			UUID:           "uuid-limited-" + string(rune('a'+i)),
			Username:       "user-limited-" + string(rune('a'+i)),
			Password:       "pass",
			RouterOSUserID: "*1",
			Group:          "default",
			Profile:        "default",
			Protocols:      model.JoinProtocols([]model.UserManagerAccountProtocol{model.ProtocolPPTP}),
			ResellerID:     &reseller.ID,
		}
		if err := db.Create(&account).Error; err != nil {
			t.Fatalf("failed to create account: %v", err)
		}
	}

	if err := svc.ensureResellerUnderUserManagerAccountLimit(reseller.ID); err == nil {
		t.Fatal("expected error once reseller reached max user manager accounts")
	}
}

// TestCreateAccount_RejectsOverQuotaAccountCount confirms CreateAccount
// stops at the quota check before touching the (nil, panic-prone) adaptor.
func TestCreateAccount_RejectsOverQuotaAccountCount(t *testing.T) {
	svc, db := newTestUserManagerService(t)

	maxAccounts := 1
	reseller := model.Reseller{
		Name: "at-limit", Username: "at-limit-um", PasswordHash: "x",
		CanCreateUserManagerAccounts: true,
		UserManagerMaxAccounts:       &maxAccounts,
	}
	if err := db.Create(&reseller).Error; err != nil {
		t.Fatalf("failed to create reseller: %v", err)
	}

	existing := model.UserManagerAccount{
		UUID: "uuid-existing", Username: "user-existing", Password: "pass",
		RouterOSUserID: "*1", Group: "default", Profile: "default",
		Protocols: model.JoinProtocols([]model.UserManagerAccountProtocol{model.ProtocolSSTP}), ResellerID: &reseller.ID,
	}
	if err := db.Create(&existing).Error; err != nil {
		t.Fatalf("failed to create existing account: %v", err)
	}

	req := &schema.CreateUserManagerAccountRequest{
		Username: "newuser", Password: "newpass", Group: "default", Profile: "default", Protocols: []string{"sstp"},
	}

	if _, err := svc.CreateAccount(req, &reseller.ID); err == nil {
		t.Fatal("expected error creating account beyond max account count")
	}
}
