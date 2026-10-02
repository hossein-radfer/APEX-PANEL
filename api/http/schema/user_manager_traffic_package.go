package schema

// UserManagerTrafficPackageResponse/CreateUserManagerTrafficPackageRequest/
// UpdateUserManagerTrafficPackageRequest/UserManagerPackagePurchaseResponse
// mirror TrafficPackageResponse/CreateTrafficPackageRequest/
// UpdateTrafficPackageRequest/PackagePurchaseResponse exactly, for the
// separate User Manager (L2TP/PPTP/SSTP/OpenVPN) traffic-package pool.

type UserManagerTrafficPackageResponse struct {
	ID           uint    `json:"id"`
	Name         string  `json:"name"`
	Description  *string `json:"description"`
	TrafficBytes int64   `json:"traffic_bytes"`
	PriceAmount  int64   `json:"price_amount"`
	IsActive     bool    `json:"is_active"`
}

type CreateUserManagerTrafficPackageRequest struct {
	Name         string  `json:"name" validate:"required,min=1"`
	Description  *string `json:"description"`
	TrafficBytes int64   `json:"traffic_bytes" validate:"required,gt=0"`
	// PriceAmount deliberately does NOT use `required` -- see the identical,
	// confirmed bug fixed in schema.CreateV2RayTrafficPackageRequest
	// (go-playground/validator's `required` wrongly rejects a valid
	// PriceAmount: 0 free/promotional package as "missing"). `min=0` alone
	// still enforces non-negative.
	PriceAmount int64 `json:"price_amount" validate:"min=0"`
}

type UpdateUserManagerTrafficPackageRequest struct {
	Name         *string `json:"name"`
	Description  *string `json:"description"`
	TrafficBytes *int64  `json:"traffic_bytes" validate:"omitempty,gt=0"`
	PriceAmount  *int64  `json:"price_amount" validate:"omitempty,min=0"`
	IsActive     *bool   `json:"is_active"`
}

// PurchaseUserManagerPackageRequest identifies which package a reseller
// wants to buy. The resellerID always comes from the caller's own JWT, not
// the body — same rationale as PurchasePackageRequest.
type PurchaseUserManagerPackageRequest struct {
	TrafficPackageID uint `json:"traffic_package_id" validate:"required"`
}

type UserManagerPackagePurchaseResponse struct {
	ID                 uint   `json:"id"`
	ResellerID         uint   `json:"reseller_id"`
	TrafficPackageID   uint   `json:"traffic_package_id"`
	TrafficPackageName string `json:"traffic_package_name"`
	TrafficBytes       int64  `json:"traffic_bytes"`
	PriceAmount        int64  `json:"price_amount"`
	CreatedAt          int64  `json:"created_at"`
}
