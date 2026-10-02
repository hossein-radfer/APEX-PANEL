package service

import (
	"testing"

	"github.com/glebarez/sqlite"
	"github.com/maahdima/mwp/api/dataservice/model"
	"gorm.io/gorm"
)

func openResellerInterfaceTestDB(t *testing.T) *gorm.DB {
	t.Helper()

	db, err := gorm.Open(sqlite.Open("file::memory:?cache=shared"), &gorm.Config{})
	if err != nil {
		t.Fatalf("failed to open test db: %v", err)
	}

	if err := db.AutoMigrate(&model.Reseller{}, &model.Interface{}, &model.ResellerInterface{}); err != nil {
		t.Fatalf("failed to migrate test db: %v", err)
	}

	return db
}

func TestResellerInterfaceAssignment(t *testing.T) {
	db := openResellerInterfaceTestDB(t)
	svc := NewReseller(db, nil)

	reseller := model.Reseller{Name: "iface-owner", Username: "iface-owner"}
	if err := db.Create(&reseller).Error; err != nil {
		t.Fatalf("failed to create reseller: %v", err)
	}

	ifaceA := model.Interface{InterfaceID: "wg0", Name: "wg0", PrivateKey: "priv-a", PublicKey: "pub-a", ListenPort: "51820"}
	ifaceB := model.Interface{InterfaceID: "wg1", Name: "wg1", PrivateKey: "priv-b", PublicKey: "pub-b", ListenPort: "51821"}
	if err := db.Create(&ifaceA).Error; err != nil {
		t.Fatalf("failed to create interface A: %v", err)
	}
	if err := db.Create(&ifaceB).Error; err != nil {
		t.Fatalf("failed to create interface B: %v", err)
	}

	// Initially no interfaces are assigned.
	ids, err := svc.GetAssignedInterfaceIDs(reseller.ID)
	if err != nil {
		t.Fatalf("failed to get assigned interfaces: %v", err)
	}
	if len(ids) != 0 {
		t.Fatalf("expected no assigned interfaces, got %v", ids)
	}

	if assigned, err := svc.IsInterfaceAssigned(reseller.ID, ifaceA.ID); err != nil || assigned {
		t.Fatalf("expected interface A not assigned, got assigned=%v err=%v", assigned, err)
	}

	// Assign only interface A.
	if err := svc.SetAssignedInterfaces(reseller.ID, []uint{ifaceA.ID}); err != nil {
		t.Fatalf("failed to assign interfaces: %v", err)
	}

	if assigned, err := svc.IsInterfaceAssigned(reseller.ID, ifaceA.ID); err != nil || !assigned {
		t.Fatalf("expected interface A assigned, got assigned=%v err=%v", assigned, err)
	}
	if assigned, err := svc.IsInterfaceAssigned(reseller.ID, ifaceB.ID); err != nil || assigned {
		t.Fatalf("expected interface B not assigned, got assigned=%v err=%v", assigned, err)
	}

	// Reassign to interface B only; A should be dropped (replace semantics, not additive).
	if err := svc.SetAssignedInterfaces(reseller.ID, []uint{ifaceB.ID}); err != nil {
		t.Fatalf("failed to reassign interfaces: %v", err)
	}

	ids, err = svc.GetAssignedInterfaceIDs(reseller.ID)
	if err != nil {
		t.Fatalf("failed to get assigned interfaces after reassignment: %v", err)
	}
	if len(ids) != 1 || ids[0] != ifaceB.ID {
		t.Fatalf("expected only interface B assigned, got %v", ids)
	}

	// Assigning a non-existent interface id must fail and not partially apply.
	if err := svc.SetAssignedInterfaces(reseller.ID, []uint{ifaceA.ID, 9999}); err == nil {
		t.Fatalf("expected error assigning non-existent interface id")
	}

	ids, err = svc.GetAssignedInterfaceIDs(reseller.ID)
	if err != nil {
		t.Fatalf("failed to get assigned interfaces after failed reassignment: %v", err)
	}
	if len(ids) != 1 || ids[0] != ifaceB.ID {
		t.Fatalf("expected assignment to remain unchanged after failed update, got %v", ids)
	}

	// Clearing assignment.
	if err := svc.SetAssignedInterfaces(reseller.ID, []uint{}); err != nil {
		t.Fatalf("failed to clear assignment: %v", err)
	}
	ids, err = svc.GetAssignedInterfaceIDs(reseller.ID)
	if err != nil {
		t.Fatalf("failed to get assigned interfaces after clearing: %v", err)
	}
	if len(ids) != 0 {
		t.Fatalf("expected no assigned interfaces after clearing, got %v", ids)
	}
}

// TestResellerInterfaceReassignSamePairDoesNotHitUniqueConstraint is a
// regression test for a bug where SetAssignedInterfaces soft-deleted the
// old (reseller_id, interface_id) link rows before re-creating them; the
// soft-deleted row still occupied the unique index, so re-saving the SAME
// assignment (e.g. an admin opening "Edit Assigned Interfaces" and hitting
// Save without changing anything, or toggling a checkbox off and back on)
// failed with "UNIQUE constraint failed: reseller_interfaces.reseller_id,
// reseller_interfaces.interface_id". The fix uses Unscoped().Delete so the
// old row is actually removed, not just marked deleted.
func TestResellerInterfaceReassignSamePairDoesNotHitUniqueConstraint(t *testing.T) {
	db := openResellerInterfaceTestDB(t)
	svc := NewReseller(db, nil)

	reseller := model.Reseller{Name: "repeat-saver", Username: "repeat-saver"}
	if err := db.Create(&reseller).Error; err != nil {
		t.Fatalf("failed to create reseller: %v", err)
	}

	iface := model.Interface{InterfaceID: "wg-repeat", Name: "wg-repeat", PrivateKey: "priv", PublicKey: "pub", ListenPort: "51830"}
	if err := db.Create(&iface).Error; err != nil {
		t.Fatalf("failed to create interface: %v", err)
	}

	// Save the same assignment several times in a row -- exactly what
	// happens when an admin repeatedly opens the edit dialog and hits Save.
	for i := 0; i < 3; i++ {
		if err := svc.SetAssignedInterfaces(reseller.ID, []uint{iface.ID}); err != nil {
			t.Fatalf("save #%d: failed to (re)assign the same interface: %v", i+1, err)
		}

		ids, err := svc.GetAssignedInterfaceIDs(reseller.ID)
		if err != nil {
			t.Fatalf("save #%d: failed to get assigned interfaces: %v", i+1, err)
		}
		if len(ids) != 1 || ids[0] != iface.ID {
			t.Fatalf("save #%d: expected [%d], got %v", i+1, iface.ID, ids)
		}
	}

	// Confirm no stray soft-deleted rows are left occupying the unique
	// index (the underlying cause of the original bug).
	var staleCount int64
	if err := db.Unscoped().Model(&model.ResellerInterface{}).
		Where("reseller_id = ? AND interface_id = ? AND deleted_at IS NOT NULL", reseller.ID, iface.ID).
		Count(&staleCount).Error; err != nil {
		t.Fatalf("failed to count stale rows: %v", err)
	}
	if staleCount != 0 {
		t.Fatalf("expected no soft-deleted reseller_interfaces rows for this pair, found %d", staleCount)
	}
}

func TestCreatePeerRejectsUnassignedInterface(t *testing.T) {
	db := openResellerInterfaceTestDB(t)
	if err := db.AutoMigrate(&model.Peer{}); err != nil {
		t.Fatalf("failed to migrate peer table: %v", err)
	}

	resellerSvc := NewReseller(db, nil)
	peerSvc := NewWGPeer(db, nil, nil, nil, nil, nil, nil)

	reseller := model.Reseller{Name: "peer-owner", Username: "peer-owner"}
	if err := db.Create(&reseller).Error; err != nil {
		t.Fatalf("failed to create reseller: %v", err)
	}

	assignedIface := model.Interface{InterfaceID: "peer-test-wg0", Name: "peer-test-wg0", PrivateKey: "priv-a", PublicKey: "pub-a", ListenPort: "51822"}
	unassignedIface := model.Interface{InterfaceID: "peer-test-wg1", Name: "peer-test-wg1", PrivateKey: "priv-b", PublicKey: "pub-b", ListenPort: "51823"}
	if err := db.Create(&assignedIface).Error; err != nil {
		t.Fatalf("failed to create assigned interface: %v", err)
	}
	if err := db.Create(&unassignedIface).Error; err != nil {
		t.Fatalf("failed to create unassigned interface: %v", err)
	}

	if err := resellerSvc.SetAssignedInterfaces(reseller.ID, []uint{assignedIface.ID}); err != nil {
		t.Fatalf("failed to assign interface: %v", err)
	}

	assigned, err := peerSvc.isInterfaceAssignedToReseller(reseller.ID, assignedIface.ID)
	if err != nil {
		t.Fatalf("failed to check assigned interface: %v", err)
	}
	if !assigned {
		t.Fatalf("expected reseller to be assigned to its own interface")
	}

	notAssigned, err := peerSvc.isInterfaceAssignedToReseller(reseller.ID, unassignedIface.ID)
	if err != nil {
		t.Fatalf("failed to check unassigned interface: %v", err)
	}
	if notAssigned {
		t.Fatalf("expected reseller to not be assigned to interface it was never granted")
	}
}
