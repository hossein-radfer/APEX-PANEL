package model

import "time"

// TrafficPackage is an admin-defined, one-time extra-traffic add-on a
// reseller can purchase when their main quota runs out (as opposed to
// PricePlan, which models a recurring subscription/billing plan). The same
// catalog row is read by both the web panel and the Telegram bot, so there
// is exactly one source of truth for package pricing/availability.
type TrafficPackage struct {
	Model
	Name         string  `gorm:"type:varchar(128);not null;uniqueIndex"`
	Description  *string `gorm:"type:text"`
	TrafficBytes int64   `gorm:"type:bigint;not null"` // size of the package, in bytes
	PriceAmount  int64   `gorm:"type:bigint;not null"` // whole Toman, matching the wallet's balance unit
	IsActive     bool    `gorm:"not null;default:true"`
	CreatedAt    time.Time
	UpdatedAt    time.Time
}

// PackagePurchase records a single reseller purchase of a TrafficPackage —
// the order/receipt row that ties a wallet debit (LedgerEntry) back to the
// package that was bought and the quota bump it granted. TrafficPackageName/
// TrafficBytes/PriceAmount are denormalized snapshots of the package at
// purchase time, so a later edit or deactivation of the catalog entry never
// changes the historical record of what a reseller actually paid for.
type PackagePurchase struct {
	Model
	ResellerID         uint   `gorm:"index;not null;foreignKey:ResellerID;references:ID"`
	TrafficPackageID   uint   `gorm:"index;not null;foreignKey:TrafficPackageID;references:ID"`
	TrafficPackageName string `gorm:"type:varchar(128);not null"`
	TrafficBytes       int64  `gorm:"type:bigint;not null"`
	PriceAmount        int64  `gorm:"type:bigint;not null"` // whole Toman
	LedgerEntryID      *uint  `gorm:"index"` // the wallet debit this purchase produced
	CreatedAt          time.Time
	UpdatedAt          time.Time
}
