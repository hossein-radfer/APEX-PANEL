package service

import (
	"fmt"
	"testing"
	"time"

	"github.com/glebarez/sqlite"
	"gorm.io/gorm"

	"github.com/maahdima/mwp/api/dataservice/model"
)

func newTestV2RayTrafficPackageService(t *testing.T) (*V2RayTrafficPackageService, *gorm.DB) {
	t.Helper()

	dsn := fmt.Sprintf("file:v2ray_traffic_package_test_%d?mode=memory&cache=shared", time.Now().UnixNano())
	db, err := gorm.Open(sqlite.Open(dsn), &gorm.Config{})
	if err != nil {
		t.Fatalf("failed to open test db: %v", err)
	}
	if err := db.AutoMigrate(&model.V2RayTrafficPackage{}); err != nil {
		t.Fatalf("failed to migrate test db: %v", err)
	}
	return NewV2RayTrafficPackageService(db), db
}

// TestCreateTrafficPackage_ZeroPriceIsAllowed is a regression test for a
// confirmed, reported bug: creating a free (price = 0) V2Ray traffic
// package failed with "Failed to create package" / "bad parameters". The
// service layer here always correctly allowed priceAmount >= 0 -- the bug
// was in the HTTP request struct's validate tag (`required,min=0` on
// PriceAmount), since go-playground/validator's `required` treats a
// numeric field's zero value as "missing". Fixed by dropping `required`
// (see schema.CreateV2RayTrafficPackageRequest). This test guards the
// service-layer contract the HTTP layer must not violate again.
func TestCreateTrafficPackage_ZeroPriceIsAllowed(t *testing.T) {
	svc, _ := newTestV2RayTrafficPackageService(t)

	pkg, err := svc.CreateTrafficPackage("Free Promo", nil, 5*1024*1024*1024, 0)
	if err != nil {
		t.Fatalf("expected a free (price=0) package to be created successfully, got error: %v", err)
	}
	if pkg.PriceAmount != 0 {
		t.Fatalf("expected PriceAmount to be persisted as 0, got %d", pkg.PriceAmount)
	}
}

func TestCreateTrafficPackage_NegativePriceRejected(t *testing.T) {
	svc, _ := newTestV2RayTrafficPackageService(t)

	if _, err := svc.CreateTrafficPackage("Bad Price", nil, 1024, -1); err == nil {
		t.Fatal("expected an error for a negative price")
	}
}

func TestCreateTrafficPackage_DuplicateNameRejected(t *testing.T) {
	svc, _ := newTestV2RayTrafficPackageService(t)

	if _, err := svc.CreateTrafficPackage("10GB Boost", nil, 1024, 500); err != nil {
		t.Fatalf("failed to create first package: %v", err)
	}
	if _, err := svc.CreateTrafficPackage("10GB Boost", nil, 2048, 900); err == nil {
		t.Fatal("expected an error creating a second package with a duplicate name")
	}
}
