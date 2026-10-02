package schema

// CreateDNSPlanRequest/UpdateDNSPlanRequest -- see model.DNSPlan's own doc
// comment for why this is a global catalog (no panel scoping) and how it
// differs from doctor-dns's own per-panel TemplateID concept.
type CreateDNSPlanRequest struct {
	Name                     string  `json:"name" validate:"required"`
	Description              *string `json:"description,omitempty"`
	PriceAmount              int64   `json:"price_amount" validate:"min=0"`
	TotalVolumeBytes         int64   `json:"total_volume_bytes" validate:"min=0"`
	SpeedKbps                int     `json:"speed_kbps" validate:"min=0"`
	DurationDays             int     `json:"duration_days" validate:"min=0"`
	MaxConcurrentIPs         int     `json:"max_concurrent_ips" validate:"min=1"`
	DailyIPRegistrationLimit int     `json:"daily_ip_registration_limit" validate:"min=0"`
}

type UpdateDNSPlanRequest struct {
	Name                     *string `json:"name,omitempty"`
	Description              *string `json:"description,omitempty"`
	PriceAmount              *int64  `json:"price_amount,omitempty"`
	TotalVolumeBytes         *int64  `json:"total_volume_bytes,omitempty"`
	SpeedKbps                *int    `json:"speed_kbps,omitempty"`
	DurationDays             *int    `json:"duration_days,omitempty"`
	MaxConcurrentIPs         *int    `json:"max_concurrent_ips,omitempty"`
	DailyIPRegistrationLimit *int    `json:"daily_ip_registration_limit,omitempty"`
	IsActive                 *bool   `json:"is_active,omitempty"`
}

type DNSPlanResponse struct {
	Id                       uint    `json:"id"`
	Name                     string  `json:"name"`
	Description              *string `json:"description,omitempty"`
	PriceAmount              int64   `json:"price_amount"`
	TotalVolumeBytes         int64   `json:"total_volume_bytes"`
	SpeedKbps                int     `json:"speed_kbps"`
	DurationDays             int     `json:"duration_days"`
	MaxConcurrentIPs         int     `json:"max_concurrent_ips"`
	DailyIPRegistrationLimit int     `json:"daily_ip_registration_limit"`
	IsActive                 bool    `json:"is_active"`
}
