package schema

// V2RayTrafficPackage request/response DTOs mirror
// UserManagerTrafficPackage's equivalents field-for-field -- see
// user_manager_traffic_package.go for the shared rationale.

type CreateV2RayTrafficPackageRequest struct {
	Name        string  `json:"name" validate:"required"`
	Description *string `json:"description,omitempty"`
	// TrafficBytes intentionally keeps `required` -- 0 bytes is never a
	// meaningful package size, so rejecting the zero value here is correct.
	TrafficBytes int64 `json:"traffic_bytes" validate:"required,min=1"`
	// PriceAmount deliberately does NOT use `required`: go-playground/
	// validator's `required` tag treats a numeric field's zero value as
	// "missing" -- so PriceAmount:0 (a genuinely valid, common case: a
	// free/promotional traffic package) was being rejected as "bad
	// parameters" even though the service layer (CreateTrafficPackage)
	// already explicitly allows priceAmount >= 0. This was a confirmed,
	// reported bug ("Failed to create package" whenever Price was left at
	// 0). `min=0` alone still enforces non-negative without excluding
	// zero itself.
	PriceAmount int64  `json:"price_amount" validate:"min=0"`
	IsActive    *bool  `json:"is_active,omitempty"`
}

type UpdateV2RayTrafficPackageRequest struct {
	Name         string  `json:"name,omitempty"`
	Description  *string `json:"description,omitempty"`
	TrafficBytes *int64  `json:"traffic_bytes,omitempty"`
	PriceAmount  *int64  `json:"price_amount,omitempty"`
	IsActive     *bool   `json:"is_active,omitempty"`
}

type V2RayTrafficPackageResponse struct {
	Id           uint    `json:"id"`
	Name         string  `json:"name"`
	Description  *string `json:"description,omitempty"`
	TrafficBytes int64   `json:"traffic_bytes"`
	PriceAmount  int64   `json:"price_amount"`
	IsActive     bool    `json:"is_active"`
}

type PurchaseV2RayPackageRequest struct {
	TrafficPackageID uint `json:"traffic_package_id" validate:"required"`
}

type V2RayPackagePurchaseResponse struct {
	Id                 uint   `json:"id"`
	TrafficPackageName string `json:"traffic_package_name"`
	TrafficBytes       int64  `json:"traffic_bytes"`
	PriceAmount        int64  `json:"price_amount"`
	CreatedAt          string `json:"created_at"`
}
