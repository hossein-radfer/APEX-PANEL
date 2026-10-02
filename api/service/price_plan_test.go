package service

import (
	"errors"
	"testing"

	"github.com/glebarez/sqlite"
	"github.com/maahdima/mwp/api/dataservice/model"
	"gorm.io/gorm"
)

func openPricePlanTestDB(t *testing.T) *gorm.DB {
	t.Helper()

	db, err := gorm.Open(sqlite.Open("file::memory:?cache=shared"), &gorm.Config{})
	if err != nil {
		t.Fatalf("failed to open test db: %v", err)
	}

	if err := db.AutoMigrate(&model.PricePlan{}); err != nil {
		t.Fatalf("failed to migrate test db: %v", err)
	}

	return db
}

func TestPricePlanCreateUpdateAndDeactivate(t *testing.T) {
	db := openPricePlanTestDB(t)
	svc := NewPricePlan(db)

	desc := "starter"
	traffic := int64(10_000)
	maxPeers := int32(50)
	maxServers := int32(3)

	plan, err := svc.CreatePricePlan("Starter", &desc, 1299, "MONTHLY", &traffic, &maxPeers, &maxServers)
	if err != nil {
		t.Fatalf("create failed: %v", err)
	}

	if plan.Description == nil || *plan.Description != desc {
		t.Fatalf("expected description to be persisted")
	}

	newName := "Starter Plus"
	newDesc := "starter upgraded"
	newPrice := int64(1599)
	newInterval := "QUARTERLY"
	newTraffic := int64(20_000)
	newMaxPeers := int32(100)
	newMaxServers := int32(8)

	updated, err := svc.UpdatePricePlan(
		plan.ID,
		&newName,
		&newDesc,
		&newPrice,
		&newInterval,
		&newTraffic,
		&newMaxPeers,
		&newMaxServers,
	)
	if err != nil {
		t.Fatalf("update failed: %v", err)
	}

	if updated.Name != newName || updated.BasePriceAmount != newPrice || updated.BillingInterval != newInterval {
		t.Fatalf("unexpected updated core fields: %#v", updated)
	}
	if updated.Description == nil || *updated.Description != newDesc {
		t.Fatalf("description was not updated")
	}
	if updated.TrafficAllowance == nil || *updated.TrafficAllowance != newTraffic {
		t.Fatalf("traffic allowance was not updated")
	}

	if err := svc.DeactivatePricePlan(plan.ID); err != nil {
		t.Fatalf("deactivate failed: %v", err)
	}

	active, err := svc.ListActivePricePlans()
	if err != nil {
		t.Fatalf("list active failed: %v", err)
	}
	if len(active) != 0 {
		t.Fatalf("expected no active plans after deactivation, got %d", len(active))
	}

	_, err = svc.GetPricePlan(plan.ID)
	if !errors.Is(err, gorm.ErrRecordNotFound) {
		t.Fatalf("expected not found after deactivation, got %v", err)
	}
}
