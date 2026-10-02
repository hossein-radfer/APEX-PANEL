package schema

// CreateApplicationPlanRequest/UpdateApplicationPlanRequest -- see
// model.ApplicationPlan's own doc comment for why this is a global catalog
// (no reseller/resource scoping), mirroring CreateDNSPlanRequest exactly.
type CreateApplicationPlanRequest struct {
	Name                   string  `json:"name" validate:"required"`
	Description            *string `json:"description,omitempty"`
	PriceAmount            int64   `json:"price_amount" validate:"min=0"`
	TotalVolumeBytes       int64   `json:"total_volume_bytes" validate:"min=0"`
	DurationDays           int     `json:"duration_days" validate:"min=0"`
	MaxOnlineUsers         int     `json:"max_online_users" validate:"min=1"`
	DownloadSpeedLimitMbps *int    `json:"download_speed_limit_mbps,omitempty"`
	UploadSpeedLimitMbps   *int    `json:"upload_speed_limit_mbps,omitempty"`
}

type UpdateApplicationPlanRequest struct {
	Name                   *string `json:"name,omitempty"`
	Description            *string `json:"description,omitempty"`
	PriceAmount            *int64  `json:"price_amount,omitempty"`
	TotalVolumeBytes       *int64  `json:"total_volume_bytes,omitempty"`
	DurationDays           *int    `json:"duration_days,omitempty"`
	MaxOnlineUsers         *int    `json:"max_online_users,omitempty"`
	DownloadSpeedLimitMbps *int    `json:"download_speed_limit_mbps,omitempty"`
	UploadSpeedLimitMbps   *int    `json:"upload_speed_limit_mbps,omitempty"`
	ClearSpeedLimits       *bool   `json:"clear_speed_limits,omitempty"`
	IsActive               *bool   `json:"is_active,omitempty"`
}

type ApplicationPlanResponse struct {
	Id                     uint    `json:"id"`
	Name                   string  `json:"name"`
	Description            *string `json:"description,omitempty"`
	PriceAmount            int64   `json:"price_amount"`
	TotalVolumeBytes       int64   `json:"total_volume_bytes"`
	DurationDays           int     `json:"duration_days"`
	MaxOnlineUsers         int     `json:"max_online_users"`
	DownloadSpeedLimitMbps *int    `json:"download_speed_limit_mbps,omitempty"`
	UploadSpeedLimitMbps   *int    `json:"upload_speed_limit_mbps,omitempty"`
	IsActive               bool    `json:"is_active"`
}
