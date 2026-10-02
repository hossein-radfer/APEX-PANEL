package service

import (
	"testing"
	"time"

	"github.com/maahdima/mwp/api/dataservice/model"
)

func createTestV2RayPackageForDownsize(t *testing.T, svc *V2RayPackageService, suffix string, createdAt uint64) model.V2RayPackage {
	t.Helper()
	pkg := model.V2RayPackage{UUID: "pkg-" + suffix, TotalVolumeBytes: 1024, DurationDays: 30, Status: "active"}
	if err := svc.db.Create(&pkg).Error; err != nil {
		t.Fatalf("failed to create test v2ray package: %v", err)
	}
	if createdAt > 0 {
		if err := svc.db.Model(&model.V2RayPackage{}).Where("id = ?", pkg.ID).Update("created_at", createdAt).Error; err != nil {
			t.Fatalf("failed to backdate test package: %v", err)
		}
	}
	return pkg
}

// TestSuspendOldestForFreeTier_V2RayPackage_DisablesOnlyOldestExcess mirrors
// TestSuspendOldestForFreeTier_Peer_DisablesOnlyOldestExcess exactly.
func TestSuspendOldestForFreeTier_V2RayPackage_DisablesOnlyOldestExcess(t *testing.T) {
	svc, db := newTestV2RayPackageService(t)

	base := uint64(time.Now().Unix())
	oldest := createTestV2RayPackageForDownsize(t, svc, "1", base)
	middle := createTestV2RayPackageForDownsize(t, svc, "2", base+10)
	newest := createTestV2RayPackageForDownsize(t, svc, "3", base+20)

	suspended, err := svc.SuspendOldestForFreeTier(2)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if suspended != 1 {
		t.Fatalf("expected exactly 1 package suspended (3 active - cap 2), got %d", suspended)
	}

	var reloadedOldest, reloadedMiddle, reloadedNewest model.V2RayPackage
	db.First(&reloadedOldest, oldest.ID)
	db.First(&reloadedMiddle, middle.ID)
	db.First(&reloadedNewest, newest.ID)

	if reloadedOldest.Status != "suspended" || !reloadedOldest.SuspendedByQuota || !reloadedOldest.WasActiveBeforeSuspend {
		t.Fatalf("expected the OLDEST package to be suspended, got Status=%q SuspendedByQuota=%v WasActiveBeforeSuspend=%v",
			reloadedOldest.Status, reloadedOldest.SuspendedByQuota, reloadedOldest.WasActiveBeforeSuspend)
	}
	if reloadedMiddle.Status != "active" || reloadedNewest.Status != "active" {
		t.Fatalf("expected the two newest packages to remain active, got middle.Status=%q newest.Status=%q",
			reloadedMiddle.Status, reloadedNewest.Status)
	}
}

// TestSuspendOldestForFreeTier_V2RayPackage_UnderCapIsNoOp mirrors the peer
// equivalent.
func TestSuspendOldestForFreeTier_V2RayPackage_UnderCapIsNoOp(t *testing.T) {
	svc, _ := newTestV2RayPackageService(t)
	createTestV2RayPackageForDownsize(t, svc, "1", 0)

	suspended, err := svc.SuspendOldestForFreeTier(5)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if suspended != 0 {
		t.Fatalf("expected 0 packages suspended under cap, got %d", suspended)
	}
}

// TestSuspendOldestForFreeTier_V2RayPackage_IsIdempotent mirrors the peer
// equivalent.
func TestSuspendOldestForFreeTier_V2RayPackage_IsIdempotent(t *testing.T) {
	svc, _ := newTestV2RayPackageService(t)
	base := uint64(time.Now().Unix())
	createTestV2RayPackageForDownsize(t, svc, "1", base)
	createTestV2RayPackageForDownsize(t, svc, "2", base+10)

	first, err := svc.SuspendOldestForFreeTier(1)
	if err != nil {
		t.Fatalf("unexpected error on first call: %v", err)
	}
	if first != 1 {
		t.Fatalf("expected 1 package suspended on first call, got %d", first)
	}

	second, err := svc.SuspendOldestForFreeTier(1)
	if err != nil {
		t.Fatalf("unexpected error on second call: %v", err)
	}
	if second != 0 {
		t.Fatalf("expected 0 additional packages suspended on repeat call, got %d", second)
	}
}
