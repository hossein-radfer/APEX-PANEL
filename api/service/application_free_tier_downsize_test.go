package service

import (
	"testing"
	"time"

	"github.com/maahdima/mwp/api/dataservice/model"
)

func createTestApplicationForDownsize(t *testing.T, svc *ApplicationService, suffix string, createdAt uint64) model.Application {
	t.Helper()
	app := model.Application{Name: "app-" + suffix, AppUsername: "app-user-" + suffix, Status: "active"}
	if err := svc.db.Create(&app).Error; err != nil {
		t.Fatalf("failed to create test application: %v", err)
	}
	if createdAt > 0 {
		if err := svc.db.Model(&model.Application{}).Where("id = ?", app.ID).Update("created_at", createdAt).Error; err != nil {
			t.Fatalf("failed to backdate test application: %v", err)
		}
	}
	return app
}

// TestSuspendOldestForFreeTier_Application_DisablesOnlyOldestExcess mirrors
// TestSuspendOldestForFreeTier_Peer_DisablesOnlyOldestExcess exactly.
func TestSuspendOldestForFreeTier_Application_DisablesOnlyOldestExcess(t *testing.T) {
	svc, db := newTestApplicationServiceForPermission(t)

	base := uint64(time.Now().Unix())
	oldest := createTestApplicationForDownsize(t, svc, "1", base)
	middle := createTestApplicationForDownsize(t, svc, "2", base+10)
	newest := createTestApplicationForDownsize(t, svc, "3", base+20)

	suspended, err := svc.SuspendOldestForFreeTier(2)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if suspended != 1 {
		t.Fatalf("expected exactly 1 application suspended (3 active - cap 2), got %d", suspended)
	}

	var reloadedOldest, reloadedMiddle, reloadedNewest model.Application
	db.First(&reloadedOldest, oldest.ID)
	db.First(&reloadedMiddle, middle.ID)
	db.First(&reloadedNewest, newest.ID)

	if reloadedOldest.Status != "suspended" || !reloadedOldest.SuspendedByQuota || !reloadedOldest.WasActiveBeforeSuspend {
		t.Fatalf("expected the OLDEST application to be suspended, got Status=%q SuspendedByQuota=%v WasActiveBeforeSuspend=%v",
			reloadedOldest.Status, reloadedOldest.SuspendedByQuota, reloadedOldest.WasActiveBeforeSuspend)
	}
	if reloadedMiddle.Status != "active" || reloadedNewest.Status != "active" {
		t.Fatalf("expected the two newest applications to remain active, got middle.Status=%q newest.Status=%q",
			reloadedMiddle.Status, reloadedNewest.Status)
	}
}

// TestSuspendOldestForFreeTier_Application_UnderCapIsNoOp mirrors the peer
// equivalent.
func TestSuspendOldestForFreeTier_Application_UnderCapIsNoOp(t *testing.T) {
	svc, _ := newTestApplicationServiceForPermission(t)
	createTestApplicationForDownsize(t, svc, "1", 0)

	suspended, err := svc.SuspendOldestForFreeTier(5)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if suspended != 0 {
		t.Fatalf("expected 0 applications suspended under cap, got %d", suspended)
	}
}

// TestSuspendOldestForFreeTier_Application_IsIdempotent mirrors the peer
// equivalent.
func TestSuspendOldestForFreeTier_Application_IsIdempotent(t *testing.T) {
	svc, _ := newTestApplicationServiceForPermission(t)
	base := uint64(time.Now().Unix())
	createTestApplicationForDownsize(t, svc, "1", base)
	createTestApplicationForDownsize(t, svc, "2", base+10)

	first, err := svc.SuspendOldestForFreeTier(1)
	if err != nil {
		t.Fatalf("unexpected error on first call: %v", err)
	}
	if first != 1 {
		t.Fatalf("expected 1 application suspended on first call, got %d", first)
	}

	second, err := svc.SuspendOldestForFreeTier(1)
	if err != nil {
		t.Fatalf("unexpected error on second call: %v", err)
	}
	if second != 0 {
		t.Fatalf("expected 0 additional applications suspended on repeat call, got %d", second)
	}
}
