package model

import "time"

// UserManagerTrafficPackage mirrors TrafficPackage exactly, but is the
// catalog of one-time extra-traffic add-ons a reseller can purchase against
// their SEPARATE User Manager (L2TP/PPTP/SSTP/OpenVPN) quota, instead of
// their WireGuard quota. Deliberately not shared with TrafficPackage as a
// single table with a "pool" discriminator, matching this codebase's
// established convention of a fully separate model/service/route/UI per
// VPN system (see Reseller.UserManagerQuotaBytes's own doc comment for the
// same rationale on the quota columns this credits).
type UserManagerTrafficPackage struct {
	Model
	Name         string  `gorm:"type:varchar(128);not null;uniqueIndex"`
	Description  *string `gorm:"type:text"`
	TrafficBytes int64   `gorm:"type:bigint;not null"` // size of the package, in bytes
	PriceAmount  int64   `gorm:"type:bigint;not null"` // whole Toman, matching the wallet's balance unit
	IsActive     bool    `gorm:"not null;default:true"`
	CreatedAt    time.Time
	UpdatedAt    time.Time
}

// UserManagerPackagePurchase mirrors PackagePurchase exactly -- the
// order/receipt row tying a wallet debit to a UserManagerTrafficPackage and
// the User Manager quota bump it granted. Fields are denormalized snapshots
// at purchase time, same rationale as PackagePurchase.
type UserManagerPackagePurchase struct {
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
