package schema

import (
	"testing"

	"github.com/go-playground/validator/v10"
)

// TestCreateV2RayTrafficPackageRequest_ZeroPriceIsValid is a regression
// test for a confirmed, reported bug: PriceAmount previously carried the
// `required,min=0` validate tag, and go-playground/validator's `required`
// treats a numeric field's zero value as "missing" -- so a free/
// promotional package (PriceAmount: 0) failed HTTP-layer validation with
// "bad parameters" even though the service layer always allowed it. Fixed
// by dropping `required` (min=0 alone still enforces non-negative).
func TestCreateV2RayTrafficPackageRequest_ZeroPriceIsValid(t *testing.T) {
	v := validator.New()
	req := CreateV2RayTrafficPackageRequest{
		Name:         "Free Promo",
		TrafficBytes: 5 * 1024 * 1024 * 1024,
		PriceAmount:  0,
	}
	if err := v.Struct(req); err != nil {
		t.Fatalf("expected PriceAmount: 0 to pass validation, got: %v", err)
	}
}

func TestCreateV2RayTrafficPackageRequest_NegativePriceRejected(t *testing.T) {
	v := validator.New()
	req := CreateV2RayTrafficPackageRequest{
		Name:         "Bad Price",
		TrafficBytes: 1024,
		PriceAmount:  -1,
	}
	if err := v.Struct(req); err == nil {
		t.Fatal("expected a negative PriceAmount to fail validation")
	}
}

func TestCreateV2RayTrafficPackageRequest_ZeroTrafficBytesRejected(t *testing.T) {
	v := validator.New()
	req := CreateV2RayTrafficPackageRequest{
		Name:         "No Traffic",
		TrafficBytes: 0,
		PriceAmount:  100,
	}
	if err := v.Struct(req); err == nil {
		t.Fatal("expected TrafficBytes: 0 to still fail validation (0 bytes is never a meaningful package size)")
	}
}
