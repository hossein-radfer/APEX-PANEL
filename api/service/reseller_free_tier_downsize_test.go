package service

import (
	"fmt"
	"testing"
	"time"

	"github.com/maahdima/mwp/api/dataservice/model"
)

// TestSuspendOldestForFreeTier_Reseller_DisablesOnlyOldestExcess mirrors
// TestSuspendOldestForFreeTier_Peer_DisablesOnlyOldestExcess, but for
// Reseller.IsActive (resellers have no SuspendedByQuota/
// WasActiveBeforeSuspend pair -- IsActive=false is this codebase's own
// existing reseller-level suspend flag, reused here exactly as
// cmd/jobs/traffic.go's billing-suspension path already does).
func TestSuspendOldestForFreeTier_Reseller_DisablesOnlyOldestExcess(t *testing.T) {
	db := openResellerFreeTierTestDB(t)
	svc := NewReseller(db, nil)

	base := time.Now().Add(-1 * time.Hour)
	oldest := model.Reseller{Name: "r1", Username: "r1", PasswordHash: "x", IsActive: true}
	middle := model.Reseller{Name: "r2", Username: "r2", PasswordHash: "x", IsActive: true}
	newest := model.Reseller{Name: "r3", Username: "r3", PasswordHash: "x", IsActive: true}
	for _, r := range []*model.Reseller{&oldest, &middle, &newest} {
		if err := db.Create(r).Error; err != nil {
			t.Fatalf("failed to create test reseller: %v", err)
		}
	}
	db.Model(&model.Reseller{}).Where("id = ?", oldest.ID).Update("created_at", base)
	db.Model(&model.Reseller{}).Where("id = ?", middle.ID).Update("created_at", base.Add(10*time.Second))
	db.Model(&model.Reseller{}).Where("id = ?", newest.ID).Update("created_at", base.Add(20*time.Second))

	suspended, err := svc.SuspendOldestForFreeTier(2)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if suspended != 1 {
		t.Fatalf("expected exactly 1 reseller suspended (3 active - cap 2), got %d", suspended)
	}

	var reloadedOldest, reloadedMiddle, reloadedNewest model.Reseller
	db.First(&reloadedOldest, oldest.ID)
	db.First(&reloadedMiddle, middle.ID)
	db.First(&reloadedNewest, newest.ID)

	if reloadedOldest.IsActive {
		t.Fatalf("expected the OLDEST reseller to be deactivated, got IsActive=true")
	}
	if !reloadedMiddle.IsActive || !reloadedNewest.IsActive {
		t.Fatalf("expected the two newest resellers to remain active, got middle.IsActive=%v newest.IsActive=%v",
			reloadedMiddle.IsActive, reloadedNewest.IsActive)
	}
}

// TestSuspendOldestForFreeTier_Reseller_UnderCapIsNoOp mirrors the peer
// equivalent.
func TestSuspendOldestForFreeTier_Reseller_UnderCapIsNoOp(t *testing.T) {
	db := openResellerFreeTierTestDB(t)
	svc := NewReseller(db, nil)
	if err := db.Create(&model.Reseller{Name: "r1", Username: fmt.Sprintf("r1-%d", time.Now().UnixNano()), PasswordHash: "x", IsActive: true}).Error; err != nil {
		t.Fatalf("failed to create test reseller: %v", err)
	}

	suspended, err := svc.SuspendOldestForFreeTier(5)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if suspended != 0 {
		t.Fatalf("expected 0 resellers suspended under cap, got %d", suspended)
	}
}

// TestSuspendOldestForFreeTier_Reseller_IsIdempotent mirrors the peer
// equivalent.
func TestSuspendOldestForFreeTier_Reseller_IsIdempotent(t *testing.T) {
	db := openResellerFreeTierTestDB(t)
	svc := NewReseller(db, nil)

	base := time.Now().Add(-1 * time.Hour)
	r1 := model.Reseller{Name: "r1", Username: fmt.Sprintf("r1-%d", time.Now().UnixNano()), PasswordHash: "x", IsActive: true}
	r2 := model.Reseller{Name: "r2", Username: fmt.Sprintf("r2-%d", time.Now().UnixNano()), PasswordHash: "x", IsActive: true}
	if err := db.Create(&r1).Error; err != nil {
		t.Fatalf("failed to create r1: %v", err)
	}
	if err := db.Create(&r2).Error; err != nil {
		t.Fatalf("failed to create r2: %v", err)
	}
	db.Model(&model.Reseller{}).Where("id = ?", r1.ID).Update("created_at", base)
	db.Model(&model.Reseller{}).Where("id = ?", r2.ID).Update("created_at", base.Add(10*time.Second))

	first, err := svc.SuspendOldestForFreeTier(1)
	if err != nil {
		t.Fatalf("unexpected error on first call: %v", err)
	}
	if first != 1 {
		t.Fatalf("expected 1 reseller suspended on first call, got %d", first)
	}

	second, err := svc.SuspendOldestForFreeTier(1)
	if err != nil {
		t.Fatalf("unexpected error on second call: %v", err)
	}
	if second != 0 {
		t.Fatalf("expected 0 additional resellers suspended on repeat call, got %d", second)
	}
}
