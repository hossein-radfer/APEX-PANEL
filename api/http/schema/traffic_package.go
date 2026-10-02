package schema

type TrafficPackageResponse struct {
	ID           uint    `json:"id"`
	Name         string  `json:"name"`
	Description  *string `json:"description"`
	TrafficBytes int64   `json:"traffic_bytes"`
	PriceAmount  int64   `json:"price_amount"`
	IsActive     bool    `json:"is_active"`
}

type CreateTrafficPackageRequest struct {
	Name         string  `json:"name" validate:"required,min=1"`
	Description  *string `json:"description"`
	TrafficBytes int64   `json:"traffic_bytes" validate:"required,gt=0"`
	// PriceAmount deliberately does NOT use `required`: go-playground/
	// validator's `required` tag rejects a numeric field's zero value as
	// "missing", which would wrongly reject a genuinely valid free/
	// promotional package (PriceAmount: 0). See the identical, confirmed
	// bug fixed in schema.CreateV2RayTrafficPackageRequest for the full
	// rationale. `min=0` alone still enforces non-negative.
	PriceAmount int64 `json:"price_amount" validate:"min=0"`
}

type UpdateTrafficPackageRequest struct {
	Name         *string `json:"name"`
	Description  *string `json:"description"`
	TrafficBytes *int64  `json:"traffic_bytes" validate:"omitempty,gt=0"`
	PriceAmount  *int64  `json:"price_amount" validate:"omitempty,min=0"`
	IsActive     *bool   `json:"is_active"`
}

// PurchasePackageRequest identifies which package a reseller wants to buy.
// The resellerID itself comes from the URL (admin buying on a reseller's
// behalf) or the caller's own JWT (a reseller buying for themselves), never
// from the request body — see peerScopeFromContext's pattern in wg_peer.go
// for why body-supplied ownership fields are unsafe.
type PurchasePackageRequest struct {
	TrafficPackageID uint `json:"traffic_package_id" validate:"required"`
}

type PackagePurchaseResponse struct {
	ID                 uint   `json:"id"`
	ResellerID         uint   `json:"reseller_id"`
	TrafficPackageID   uint   `json:"traffic_package_id"`
	TrafficPackageName string `json:"traffic_package_name"`
	TrafficBytes       int64  `json:"traffic_bytes"`
	PriceAmount        int64  `json:"price_amount"`
	CreatedAt          int64  `json:"created_at"`
}
