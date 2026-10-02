package model

// ApplicationPlan is an admin-defined, named preset bundle of Application
// limits (e.g. "برنزی"/"نقره‌ای"/"طلایی") -- phase 2's "tiered plan system
// instead of manual per-application setup, with manual override still
// allowed." Mirrors DNSPlan's own shape exactly: a standalone, GLOBAL
// admin-managed catalog (no ResellerID/panel FK), since these limits
// (TotalVolumeBytes, DurationDays, MaxOnlineUsers, speed caps) are
// meaningful independent of which reseller or underlying
// interface/group/panel an Application ultimately fans out to.
//
// Selecting a plan on Application creation copies this bundle's values into
// the Application's own columns (Application.TotalVolumeBytes etc. stay the
// single source of truth for quota enforcement -- no schema duplication)
// and additionally sets Application.ApplicationPlanID purely for display/
// bookkeeping; every field remains a normal editable input afterward,
// satisfying the same "allow manual override" requirement DNSPlan already
// established.
type ApplicationPlan struct {
	Model
	Name        string  `gorm:"type:varchar(128);not null;uniqueIndex"`
	Description *string `gorm:"type:text"`

	// PriceAmount is whole Toman, matching DNSPlan.PriceAmount/
	// TrafficPackage.PriceAmount's own convention.
	PriceAmount int64 `gorm:"type:bigint;not null;default:0"`

	// The bundle -- identical fields/semantics to their Application
	// namesakes (see that model's own doc comments for what each governs).
	TotalVolumeBytes int64 `gorm:"type:bigint;not null;default:0"`
	DurationDays     int   `gorm:"type:int;not null;default:0"`
	MaxOnlineUsers   int   `gorm:"type:int;not null;default:1"`

	// DownloadSpeedLimitMbps/UploadSpeedLimitMbps mirror Application's own
	// nil-means-unlimited convention -- a plan need not cap speed at all.
	DownloadSpeedLimitMbps *int `gorm:"type:int"`
	UploadSpeedLimitMbps   *int `gorm:"type:int"`

	IsActive bool `gorm:"not null;default:true"`
}
