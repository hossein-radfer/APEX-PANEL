package model

import "time"

// V2RayPackagePurchase mirrors UserManagerPackagePurchase exactly -- the
// order/receipt row tying a wallet debit to a V2RayTrafficPackage and the
// V2Ray quota bump it granted. Fields are denormalized snapshots at
// purchase time, same rationale as PackagePurchase/UserManagerPackagePurchase.
type V2RayPackagePurchase struct {
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
