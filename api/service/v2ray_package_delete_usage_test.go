package service

import (
	"testing"

	"github.com/maahdima/mwp/api/dataservice/model"
)

// TestDeletePackage_PreservesResellerUsageTotal is the regression test for
// a confirmed, reported bug: V2RayUsedBytes is a LIVE SUM recomputed every
// sync tick over currently-existing V2RayPackageLocation rows (see
// applyResellerV2RayQuota's own doc comment), scoped via a JOIN to
// v2_ray_packages -- so deleting a package (and its locations) used to
// silently erase its historical usage from the owning reseller's total,
// corrupting any billing/report built on it. Deleting the package must
// fold its locations' cached usage into V2RayDeletedUsageBytes so the
// reseller's total survives.
//
// The package's locations deliberately point at a PanelID with no
// matching XuiPanel row -- DeletePackage's own x-ui disable calls are
// best-effort (logged, non-fatal) when GetPanel fails, so this test
// exercises the pure DB-side credit-folding logic without needing a live
// x-ui server.
func TestDeletePackage_PreservesResellerUsageTotal(t *testing.T) {
	svc, db := newTestV2RayPackageService(t)

	reseller := model.Reseller{Name: "v2ray-usage-preserve", Username: "v2ray-usage-preserve", PasswordHash: "x", CanResellV2Ray: true}
	if err := db.Create(&reseller).Error; err != nil {
		t.Fatalf("failed to create reseller: %v", err)
	}

	pkg := model.V2RayPackage{
		ResellerID:       &reseller.ID,
		Status:           "active",
		TotalVolumeBytes: 100 * 1024 * 1024 * 1024,
	}
	if err := db.Create(&pkg).Error; err != nil {
		t.Fatalf("failed to create package: %v", err)
	}

	locations := []model.V2RayPackageLocation{
		{PackageID: pkg.ID, PanelID: 998, ClientUUID: "uuid-1", ClientEmail: "a@x", UsedBytesCached: 2 * 1024 * 1024 * 1024},
		{PackageID: pkg.ID, PanelID: 999, ClientUUID: "uuid-2", ClientEmail: "b@x", UsedBytesCached: 3 * 1024 * 1024 * 1024},
	}
	for i := range locations {
		if err := db.Create(&locations[i]).Error; err != nil {
			t.Fatalf("failed to create location: %v", err)
		}
	}

	if err := svc.DeletePackage(pkg.ID, nil); err != nil {
		t.Fatalf("DeletePackage failed: %v", err)
	}

	var got model.Reseller
	if err := db.First(&got, reseller.ID).Error; err != nil {
		t.Fatalf("failed to reload reseller: %v", err)
	}
	wantCredit := int64(5 * 1024 * 1024 * 1024)
	if got.V2RayDeletedUsageBytes != wantCredit {
		t.Fatalf("expected V2RayDeletedUsageBytes=%d after deleting a package with 5GB total cached usage, got %d", wantCredit, got.V2RayDeletedUsageBytes)
	}

	var count int64
	if err := db.Model(&model.V2RayPackage{}).Where("id = ?", pkg.ID).Count(&count).Error; err != nil {
		t.Fatalf("failed to count packages: %v", err)
	}
	if count != 0 {
		t.Fatalf("expected the package to be gone (soft-deleted, excluded from default queries), got count=%d", count)
	}
}
