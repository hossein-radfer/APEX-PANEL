package service

import (
	"errors"
	"testing"

	"github.com/maahdima/mwp/api/dataservice/model"
	"gorm.io/gorm"
)

// TestEnsurePackageAccess mirrors TestEnsureAccountAccess exactly: admin
// scope (nil resellerID) can access any package, the owning reseller can
// access their own, and a different reseller gets gorm.ErrRecordNotFound.
func TestEnsurePackageAccess(t *testing.T) {
	svc, db := newTestV2RayPackageService(t)

	ownerResellerID := uint(21)
	otherResellerID := uint(22)
	pkg := model.V2RayPackage{
		UUID:             "scope-uuid",
		ResellerID:       &ownerResellerID,
		TotalVolumeBytes: 1024,
		DurationDays:     30,
		Status:           "active",
	}

	if err := db.Create(&pkg).Error; err != nil {
		t.Fatalf("failed to create package: %v", err)
	}

	if err := svc.EnsurePackageAccess(pkg.ID, nil); err != nil {
		t.Fatalf("expected admin scope to access package, got %v", err)
	}

	if err := svc.EnsurePackageAccess(pkg.ID, &ownerResellerID); err != nil {
		t.Fatalf("expected owner reseller to access package, got %v", err)
	}

	err := svc.EnsurePackageAccess(pkg.ID, &otherResellerID)
	if !errors.Is(err, gorm.ErrRecordNotFound) {
		t.Fatalf("expected record not found for non-owner reseller, got %v", err)
	}
}

// TestListPackagesScoped confirms the reseller_id IS NULL vs = ? scoping
// split mirrors TestListAccountsScoped exactly -- the "V2Ray Packages"
// (admin-direct) view and "Resellers V2Ray" (filtered-to-one-reseller)
// view are two different queries over the same table.
func TestListPackagesScoped(t *testing.T) {
	svc, db := newTestV2RayPackageService(t)

	resellerID := uint(31)

	adminPackage := model.V2RayPackage{
		UUID: "admin-uuid", ResellerID: nil,
		TotalVolumeBytes: 1024, DurationDays: 30, Status: "active",
	}
	if err := db.Create(&adminPackage).Error; err != nil {
		t.Fatalf("failed to create admin package: %v", err)
	}

	resellerPackage := model.V2RayPackage{
		UUID: "reseller-uuid", ResellerID: &resellerID,
		TotalVolumeBytes: 2048, DurationDays: 30, Status: "active",
	}
	if err := db.Create(&resellerPackage).Error; err != nil {
		t.Fatalf("failed to create reseller package: %v", err)
	}

	adminList, err := svc.ListPackages(nil)
	if err != nil {
		t.Fatalf("failed to list admin packages: %v", err)
	}
	if len(adminList) != 1 || adminList[0].UUID != "admin-uuid" {
		t.Fatalf("expected admin scope to return only the admin-owned package, got %+v", adminList)
	}

	resellerList, err := svc.ListPackages(&resellerID)
	if err != nil {
		t.Fatalf("failed to list reseller packages: %v", err)
	}
	if len(resellerList) != 1 || resellerList[0].UUID != "reseller-uuid" {
		t.Fatalf("expected reseller scope to return only their own package, got %+v", resellerList)
	}

	byReseller, err := svc.ListPackagesByReseller(resellerID)
	if err != nil {
		t.Fatalf("failed to list packages by reseller: %v", err)
	}
	if len(byReseller) != 1 || byReseller[0].UUID != "reseller-uuid" {
		t.Fatalf("expected ListPackagesByReseller to return the reseller's package, got %+v", byReseller)
	}
}
