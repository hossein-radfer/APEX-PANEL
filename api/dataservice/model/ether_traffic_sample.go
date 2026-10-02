package model

import "time"

// EtherTrafficSample is one flow row from one `/tool/torch` poll against a
// single ether interface -- the collector job (see
// cmd/jobs/security_ether_torch.go) polls torch on a fixed interval (the
// admin explicitly chose a 10s poll of a 1s sample window over a
// continuously-streamed connection, since RouterOS's REST /tool/torch
// endpoint is a bounded-duration snapshot call, not a long-lived stream --
// this codebase's shared Mikrotik HTTP client is built for one-shot
// request/response only) and writes one row per (src address, protocol,
// port) flow torch reports that tick.
type EtherTrafficSample struct {
	Model
	Interface string `gorm:"type:varchar(64);not null;index"` // e.g. "ether1"

	SrcAddress string `gorm:"type:varchar(64);not null;index"`
	IPProtocol string `gorm:"type:varchar(32);not null"` // "tcp" | "udp" | "icmp" | ...
	SrcPort    *int

	TxBytesPerSecond int64
	RxBytesPerSecond int64
	TxPacketsRate    int64
	RxPacketsRate    int64

	// Geo/ASN fields, denormalized from IPGeoCache exactly like
	// IPConnectionLog's own equivalent fields -- see that model's doc
	// comment for why this is a copy, not a live join.
	Country  *string `gorm:"type:varchar(100)"`
	Region   *string `gorm:"type:varchar(100)"`
	City     *string `gorm:"type:varchar(100)"`
	Lat      *float64
	Lon      *float64
	Timezone *string `gorm:"type:varchar(64)"`
	ASN      *uint
	ISP      *string `gorm:"type:varchar(255)"`

	SampledAt time.Time `gorm:"not null;index"`
}
