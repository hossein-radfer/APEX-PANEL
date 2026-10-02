package service

import (
	"testing"
	"time"

	"github.com/maahdima/mwp/api/dataservice/model"
)

func createTestUserManagerAccountForDownsize(t *testing.T, s *UserManagerService, suffix string, createdAt uint64) model.UserManagerAccount {
	t.Helper()
	acct := model.UserManagerAccount{UUID: "um-uuid-" + suffix, Username: "um-" + suffix, Password: "pw", RouterOSUserID: "*" + suffix}
	if err := s.db.Create(&acct).Error; err != nil {
		t.Fatalf("failed to create test user manager account: %v", err)
	}
	if createdAt > 0 {
		if err := s.db.Model(&model.UserManagerAccount{}).Where("id = ?", acct.ID).Update("created_at", createdAt).Error; err != nil {
			t.Fatalf("failed to backdate test account: %v", err)
		}
	}
	return acct
}

// TestSuspendOldestForFreeTier_UserManager_DisablesOnlyOldestExcess mirrors
// TestSuspendOldestForFreeTier_Peer_DisablesOnlyOldestExcess exactly.
func TestSuspendOldestForFreeTier_UserManager_DisablesOnlyOldestExcess(t *testing.T) {
	svc, db := newTestUserManagerServiceForFreeTier(t)

	base := uint64(time.Now().Unix())
	oldest := createTestUserManagerAccountForDownsize(t, svc, "1", base)
	middle := createTestUserManagerAccountForDownsize(t, svc, "2", base+10)
	newest := createTestUserManagerAccountForDownsize(t, svc, "3", base+20)

	suspended, err := svc.SuspendOldestForFreeTier(2)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if suspended != 1 {
		t.Fatalf("expected exactly 1 account suspended (3 active - cap 2), got %d", suspended)
	}

	var reloadedOldest, reloadedMiddle, reloadedNewest model.UserManagerAccount
	db.First(&reloadedOldest, oldest.ID)
	db.First(&reloadedMiddle, middle.ID)
	db.First(&reloadedNewest, newest.ID)

	if !reloadedOldest.Disabled || !reloadedOldest.SuspendedByQuota || !reloadedOldest.WasActiveBeforeSuspend {
		t.Fatalf("expected the OLDEST account to be suspended, got Disabled=%v SuspendedByQuota=%v WasActiveBeforeSuspend=%v",
			reloadedOldest.Disabled, reloadedOldest.SuspendedByQuota, reloadedOldest.WasActiveBeforeSuspend)
	}
	if reloadedMiddle.Disabled || reloadedNewest.Disabled {
		t.Fatalf("expected the two newest accounts to remain enabled, got middle.Disabled=%v newest.Disabled=%v",
			reloadedMiddle.Disabled, reloadedNewest.Disabled)
	}
}

// TestSuspendOldestForFreeTier_UserManager_UnderCapIsNoOp mirrors the peer
// equivalent.
func TestSuspendOldestForFreeTier_UserManager_UnderCapIsNoOp(t *testing.T) {
	svc, _ := newTestUserManagerServiceForFreeTier(t)
	createTestUserManagerAccountForDownsize(t, svc, "1", 0)

	suspended, err := svc.SuspendOldestForFreeTier(5)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if suspended != 0 {
		t.Fatalf("expected 0 accounts suspended under cap, got %d", suspended)
	}
}

// TestSuspendOldestForFreeTier_UserManager_IsIdempotent mirrors the peer
// equivalent.
func TestSuspendOldestForFreeTier_UserManager_IsIdempotent(t *testing.T) {
	svc, _ := newTestUserManagerServiceForFreeTier(t)
	base := uint64(time.Now().Unix())
	createTestUserManagerAccountForDownsize(t, svc, "1", base)
	createTestUserManagerAccountForDownsize(t, svc, "2", base+10)

	first, err := svc.SuspendOldestForFreeTier(1)
	if err != nil {
		t.Fatalf("unexpected error on first call: %v", err)
	}
	if first != 1 {
		t.Fatalf("expected 1 account suspended on first call, got %d", first)
	}

	second, err := svc.SuspendOldestForFreeTier(1)
	if err != nil {
		t.Fatalf("unexpected error on second call: %v", err)
	}
	if second != 0 {
		t.Fatalf("expected 0 additional accounts suspended on repeat call, got %d", second)
	}
}
