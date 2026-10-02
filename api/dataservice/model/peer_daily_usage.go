package model

// PeerDailyUsage accumulates traffic for a single peer within a single
// calendar day (UTC), so daily consumption can be reported per peer and
// aggregated per reseller without recomputing from raw counters.
type PeerDailyUsage struct {
	Model
	PeerID        uint   `gorm:"index:idx_peer_daily_usage_peer_date,unique;not null"`
	ResellerID    *uint  `gorm:"index"`
	Date          string `gorm:"type:varchar(10);index:idx_peer_daily_usage_peer_date,unique;not null"` // YYYY-MM-DD (UTC)
	DownloadUsage int64  `gorm:"type:bigint;not null;default:0"`                                        // in bytes
	UploadUsage   int64  `gorm:"type:bigint;not null;default:0"`                                        // in bytes
}
