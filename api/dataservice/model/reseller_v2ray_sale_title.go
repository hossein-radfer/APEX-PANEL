package model

// ResellerV2RaySaleTitle records a reseller's custom customer-facing title
// for one specific XuiPanel, overriding that panel's own XuiPanel.SaleTitle
// on every config link generated for that reseller's packages on that
// panel. A row only exists when a reseller has actually set a custom title
// -- absence means "fall back to XuiPanel.SaleTitle" (resolved in
// V2RayPackageService.resolveSaleTitle). Deliberately not a full
// access-control table: V2Ray panels are a shared external resource with no
// per-reseller sub-resource to assign (see Reseller.CanResellV2Ray's doc
// comment), so this table exists ONLY to serve the branding need, not to
// gate which panels a reseller may use.
type ResellerV2RaySaleTitle struct {
	Model
	ResellerID uint   `gorm:"uniqueIndex:idx_reseller_v2ray_sale_title;not null"`
	PanelID    uint   `gorm:"uniqueIndex:idx_reseller_v2ray_sale_title;not null"`
	Title      string `gorm:"type:varchar(255);not null"`
}
