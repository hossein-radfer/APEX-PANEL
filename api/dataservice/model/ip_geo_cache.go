package model

import "time"

// IPGeoCache is a per-IP-address cache of GeoLite2 City/ASN lookups --
// looked up once per unique IP address seen (by the Security page's own
// job, see IPConnectionLog), never re-queried on every sync tick, per the
// admin's own explicit requirement ("دوباره گرفته نشه"). Cleared entirely
// (a full DELETE, not per-row TTL expiry) only when the admin either
// re-uploads a newer .mmdb pair or explicitly asks to clear it -- both via
// GeoIPService -- since a stale cached lookup is otherwise indistinguishable
// from a fresh one and there is no natural per-IP expiry rule that fits an
// mmdb file being swapped at an unpredictable time chosen by the admin.
type IPGeoCache struct {
	Model
	IPAddress string `gorm:"type:varchar(64);uniqueIndex;not null"`

	// City .mmdb fields -- all nullable since a lookup can partially miss
	// (e.g. ASN found but City not, or the IP is private/reserved and
	// neither database has an entry at all).
	Country  *string `gorm:"type:varchar(100)"`
	Region   *string `gorm:"type:varchar(100)"`
	City     *string `gorm:"type:varchar(100)"`
	Lat      *float64
	Lon      *float64
	Timezone *string `gorm:"type:varchar(64)"`

	// ASN .mmdb fields.
	ASN         *uint
	ISP         *string `gorm:"type:varchar(255)"`

	ResolvedAt time.Time
}
