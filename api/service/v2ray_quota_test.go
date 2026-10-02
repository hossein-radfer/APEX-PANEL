package service

import (
	"testing"

	"github.com/maahdima/mwp/api/dataservice/model"
	"github.com/maahdima/mwp/api/http/schema"
)

// TestEnsureResellerUnderV2RayPackageLimit_Unlimited confirms a nil
// V2RayMaxPackages means unlimited, mirroring
// ensureResellerUnderUserManagerAccountLimit's own unlimited test.
func TestEnsureResellerUnderV2RayPackageLimit_Unlimited(t *testing.T) {
	svc, db := newTestV2RayPackageService(t)

	reseller := model.Reseller{Name: "unlimited", Username: "unlimited-v2ray", PasswordHash: "x", CanResellV2Ray: true}
	if err := db.Create(&reseller).Error; err != nil {
		t.Fatalf("failed to create reseller: %v", err)
	}

	for i := 0; i < 5; i++ {
		pkg := model.V2RayPackage{
			UUID:             "uuid-unlimited-" + string(rune('a'+i)),
			ResellerID:       &reseller.ID,
			TotalVolumeBytes: 1024,
			DurationDays:     30,
			Status:           "active",
		}
		if err := db.Create(&pkg).Error; err != nil {
			t.Fatalf("failed to create package: %v", err)
		}
	}

	if err := svc.ensureResellerUnderV2RayPackageLimit(reseller.ID); err != nil {
		t.Fatalf("expected no error for unlimited reseller, got %v", err)
	}
}

// TestEnsureResellerUnderV2RayPackageLimit_Enforced confirms the cap is
// enforced once V2RayMaxPackages is set, and is a SEPARATE limit from
// MaxPeers/UserManagerMaxAccounts (this test never touches those models).
func TestEnsureResellerUnderV2RayPackageLimit_Enforced(t *testing.T) {
	svc, db := newTestV2RayPackageService(t)

	maxPackages := 2
	reseller := model.Reseller{
		Name: "limited", Username: "limited-v2ray", PasswordHash: "x",
		CanResellV2Ray:   true,
		V2RayMaxPackages: &maxPackages,
	}
	if err := db.Create(&reseller).Error; err != nil {
		t.Fatalf("failed to create reseller: %v", err)
	}

	if err := svc.ensureResellerUnderV2RayPackageLimit(reseller.ID); err != nil {
		t.Fatalf("expected no error under limit, got %v", err)
	}

	for i := 0; i < 2; i++ {
		pkg := model.V2RayPackage{
			UUID:             "uuid-limited-" + string(rune('a'+i)),
			ResellerID:       &reseller.ID,
			TotalVolumeBytes: 1024,
			DurationDays:     30,
			Status:           "active",
		}
		if err := db.Create(&pkg).Error; err != nil {
			t.Fatalf("failed to create package: %v", err)
		}
	}

	if err := svc.ensureResellerUnderV2RayPackageLimit(reseller.ID); err == nil {
		t.Fatal("expected error once reseller reached max v2ray packages")
	}
}

// TestCreatePackage_RejectsOverQuotaPackageCount confirms CreatePackage
// stops at the quota check before attempting any panel fan-out.
func TestCreatePackage_RejectsOverQuotaPackageCount(t *testing.T) {
	svc, db := newTestV2RayPackageService(t)

	maxPackages := 1
	reseller := model.Reseller{
		Name: "at-limit", Username: "at-limit-v2ray", PasswordHash: "x",
		CanResellV2Ray:   true,
		V2RayMaxPackages: &maxPackages,
	}
	if err := db.Create(&reseller).Error; err != nil {
		t.Fatalf("failed to create reseller: %v", err)
	}

	existing := model.V2RayPackage{
		UUID: "uuid-existing", ResellerID: &reseller.ID,
		TotalVolumeBytes: 1024, DurationDays: 30, Status: "active",
	}
	if err := db.Create(&existing).Error; err != nil {
		t.Fatalf("failed to create existing package: %v", err)
	}

	req := &schema.CreateV2RayPackageRequest{
		TotalVolumeBytes: 2048,
		DurationDays:     30,
	}

	if _, err := svc.CreatePackage(req, &reseller.ID); err == nil {
		t.Fatal("expected error creating package beyond max package count")
	}
}
