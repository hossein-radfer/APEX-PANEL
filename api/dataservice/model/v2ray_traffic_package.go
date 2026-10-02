package model

import "time"

// V2RayTrafficPackage mirrors UserManagerTrafficPackage exactly, but is the
// catalog of one-time traffic add-ons a reseller can purchase against their
// SEPARATE V2Ray quota, instead of their WireGuard or User Manager quota.
// Deliberately not shared with TrafficPackage/UserManagerTrafficPackage as
// a single table with a "pool" discriminator, matching this codebase's
// established convention of a fully separate model/service/route/UI per
// VPN system (see Reseller.V2RayQuotaBytes's own doc comment).
type V2RayTrafficPackage struct {
	Model
	Name         string  `gorm:"type:varchar(128);not null;uniqueIndex"`
	Description  *string `gorm:"type:text"`
	TrafficBytes int64   `gorm:"type:bigint;not null"` // size of the package, in bytes
	PriceAmount  int64   `gorm:"type:bigint;not null"` // whole Toman, matching the wallet's balance unit
	IsActive     bool    `gorm:"not null;default:true"`
	CreatedAt    time.Time
	UpdatedAt    time.Time
}
