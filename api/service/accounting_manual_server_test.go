package service

import (
	"fmt"
	"testing"
	"time"

	"github.com/glebarez/sqlite"
	"gorm.io/gorm"

	"github.com/maahdima/mwp/api/dataservice/model"
)

func newTestAccountingService(t *testing.T) (*AccountingService, *gorm.DB) {
	t.Helper()
	dsn := fmt.Sprintf("file:accounting_manual_server_test_%d?mode=memory&cache=shared", time.Now().UnixNano())
	db, err := gorm.Open(sqlite.Open(dsn), &gorm.Config{})
	if err != nil {
		t.Fatalf("failed to open test db: %v", err)
	}
	if err := db.AutoMigrate(&model.AccountingPartner{}, &model.AccountingCost{}, &model.Server{}); err != nil {
		t.Fatalf("failed to migrate test db: %v", err)
	}
	return NewAccountingService(db), db
}

// TestCreateCost_AcceptsManualServerNameWhenNoServerIDSet is the
// regression test for the confirmed, reported gap: a cost for
// infrastructure not present in the managed Server table (e.g. a foreign
// VPS with no RouterOS API) previously had no way to record which server
// it belonged to at all.
func TestCreateCost_AcceptsManualServerNameWhenNoServerIDSet(t *testing.T) {
	svc, db := newTestAccountingService(t)
	partner := model.AccountingPartner{Name: "Test Partner"}
	if err := db.Create(&partner).Error; err != nil {
		t.Fatalf("failed to create partner: %v", err)
	}

	label := "VPS آلمان (بدون RouterOS)"
	cost, err := svc.CreateCost(CreateCostInput{
		PartnerID:        partner.ID,
		ManualServerName: &label,
		AmountToman:      100000,
	})
	if err != nil {
		t.Fatalf("CreateCost failed: %v", err)
	}
	if cost.ManualServerName == nil || *cost.ManualServerName != label {
		t.Fatalf("expected ManualServerName %q, got %v", label, cost.ManualServerName)
	}
	if cost.ServerID != nil {
		t.Errorf("expected ServerID to stay nil, got %v", cost.ServerID)
	}
}

// TestCreateCost_ServerIDWinsOverManualServerName confirms the
// server-side mutual-exclusivity enforcement: even if a client sends
// both fields (a stale label left over from a previous NONE selection,
// plus a freshly picked ServerID), ServerID always wins and
// ManualServerName is discarded -- never trusting the client to have
// cleared one when setting the other.
func TestCreateCost_ServerIDWinsOverManualServerName(t *testing.T) {
	svc, db := newTestAccountingService(t)
	partner := model.AccountingPartner{Name: "Test Partner"}
	if err := db.Create(&partner).Error; err != nil {
		t.Fatalf("failed to create partner: %v", err)
	}
	server := model.Server{Name: "Managed Router", IPAddress: "127.0.0.1", APIPort: 80, Username: "admin", Password: "admin"}
	if err := db.Create(&server).Error; err != nil {
		t.Fatalf("failed to create server: %v", err)
	}

	label := "should be discarded"
	cost, err := svc.CreateCost(CreateCostInput{
		PartnerID:        partner.ID,
		ServerID:         &server.ID,
		ManualServerName: &label,
		AmountToman:      100000,
	})
	if err != nil {
		t.Fatalf("CreateCost failed: %v", err)
	}
	if cost.ManualServerName != nil {
		t.Errorf("expected ManualServerName to be discarded when ServerID is set, got %v", *cost.ManualServerName)
	}
	if cost.ServerID == nil || *cost.ServerID != server.ID {
		t.Errorf("expected ServerID %d, got %v", server.ID, cost.ServerID)
	}
}

// TestUpdateCost_SwitchingToManagedServerClearsManualServerName confirms
// the same mutual-exclusivity rule holds on update: a cost that
// previously had a manual label, now edited to point at a managed
// server, must not keep the stale label around.
func TestUpdateCost_SwitchingToManagedServerClearsManualServerName(t *testing.T) {
	svc, db := newTestAccountingService(t)
	partner := model.AccountingPartner{Name: "Test Partner"}
	if err := db.Create(&partner).Error; err != nil {
		t.Fatalf("failed to create partner: %v", err)
	}
	server := model.Server{Name: "Managed Router", IPAddress: "127.0.0.1", APIPort: 80, Username: "admin", Password: "admin"}
	if err := db.Create(&server).Error; err != nil {
		t.Fatalf("failed to create server: %v", err)
	}

	label := "VPS آلمان"
	cost, err := svc.CreateCost(CreateCostInput{
		PartnerID:        partner.ID,
		ManualServerName: &label,
		AmountToman:      50000,
	})
	if err != nil {
		t.Fatalf("CreateCost failed: %v", err)
	}

	updated, err := svc.UpdateCost(cost.ID, CreateCostInput{
		PartnerID:   partner.ID,
		ServerID:    &server.ID,
		AmountToman: 50000,
	})
	if err != nil {
		t.Fatalf("UpdateCost failed: %v", err)
	}
	if updated.ManualServerName != nil {
		t.Errorf("expected ManualServerName cleared after switching to a managed server, got %v", *updated.ManualServerName)
	}
}
