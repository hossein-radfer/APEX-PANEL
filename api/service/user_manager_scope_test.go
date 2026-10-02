package service

import (
	"errors"
	"testing"

	"github.com/maahdima/mwp/api/dataservice/model"
	"gorm.io/gorm"
)

// TestEnsureAccountAccess mirrors TestEnsurePeerAccess exactly: admin scope
// (nil resellerID) can access any account, the owning reseller can access
// their own, and a different reseller gets gorm.ErrRecordNotFound (not a
// distinguishable "forbidden" error) -- same not-found-shaped 403 pattern
// WgPeer uses.
func TestEnsureAccountAccess(t *testing.T) {
	svc, db := newTestUserManagerService(t)

	ownerResellerID := uint(21)
	otherResellerID := uint(22)
	account := model.UserManagerAccount{
		UUID:           "scope-uuid",
		Username:       "scope-user",
		Password:       "scope-pass",
		RouterOSUserID: "*5",
		Group:          "default",
		Profile:        "default",
		Protocols:      model.JoinProtocols([]model.UserManagerAccountProtocol{model.ProtocolOpenVPN}),
		ResellerID:     &ownerResellerID,
	}

	if err := db.Create(&account).Error; err != nil {
		t.Fatalf("failed to create account: %v", err)
	}

	if err := svc.EnsureAccountAccess(account.ID, nil); err != nil {
		t.Fatalf("expected admin scope to access account, got %v", err)
	}

	if err := svc.EnsureAccountAccess(account.ID, &ownerResellerID); err != nil {
		t.Fatalf("expected owner reseller to access account, got %v", err)
	}

	err := svc.EnsureAccountAccess(account.ID, &otherResellerID)
	if !errors.Is(err, gorm.ErrRecordNotFound) {
		t.Fatalf("expected record not found for non-owner reseller, got %v", err)
	}
}

// TestListAccountsScoped confirms the reseller_id IS NULL vs = ? scoping
// split mirrors GetPeers/GetPeersByReseller exactly.
func TestListAccountsScoped(t *testing.T) {
	svc, db := newTestUserManagerService(t)

	resellerID := uint(31)

	adminAccount := model.UserManagerAccount{
		UUID: "admin-uuid", Username: "admin-user", Password: "pass",
		RouterOSUserID: "*1", Group: "default", Profile: "default",
		Protocols: model.JoinProtocols([]model.UserManagerAccountProtocol{model.ProtocolL2TP}), ResellerID: nil,
	}
	if err := db.Create(&adminAccount).Error; err != nil {
		t.Fatalf("failed to create admin account: %v", err)
	}

	resellerAccount := model.UserManagerAccount{
		UUID: "reseller-uuid", Username: "reseller-user", Password: "pass",
		RouterOSUserID: "*2", Group: "default", Profile: "default",
		Protocols: model.JoinProtocols([]model.UserManagerAccountProtocol{model.ProtocolPPTP}), ResellerID: &resellerID,
	}
	if err := db.Create(&resellerAccount).Error; err != nil {
		t.Fatalf("failed to create reseller account: %v", err)
	}

	adminList, err := svc.ListAccounts(nil)
	if err != nil {
		t.Fatalf("failed to list admin accounts: %v", err)
	}
	if len(*adminList) != 1 || (*adminList)[0].Username != "admin-user" {
		t.Fatalf("expected admin scope to return only the admin-owned account, got %+v", *adminList)
	}

	resellerList, err := svc.ListAccounts(&resellerID)
	if err != nil {
		t.Fatalf("failed to list reseller accounts: %v", err)
	}
	if len(*resellerList) != 1 || (*resellerList)[0].Username != "reseller-user" {
		t.Fatalf("expected reseller scope to return only their own account, got %+v", *resellerList)
	}

	byReseller, err := svc.ListAccountsByReseller(resellerID)
	if err != nil {
		t.Fatalf("failed to list accounts by reseller: %v", err)
	}
	if len(*byReseller) != 1 || (*byReseller)[0].Username != "reseller-user" {
		t.Fatalf("expected ListAccountsByReseller to return the reseller's account, got %+v", *byReseller)
	}
}
