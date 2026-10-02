package model

// ResourceSample is one periodic snapshot of the Mikrotik router's own
// CPU/memory load, sampled by the same scheduler cadence as the traffic
// jobs (see cmd/jobs's resource sampler) -- FetchDeviceInfo already
// exposes these figures LIVE (api/adaptor/mikrotik/device_data.go), but
// nothing previously stored a history of them, so there was no way to
// answer "what was the router's peak CPU load this week" -- only "what is
// it right now." Feeds the Reports section's "server resource usage and
// peak" report exclusively; no other part of this codebase reads this
// table.
type ResourceSample struct {
	Model
	Timestamp int64 `gorm:"not null;index"`
	// CPULoadPercent/MemoryUsedPercent are parsed from FetchDeviceInfo's
	// own string fields (CPULoad is already a plain percentage number as
	// a string; MemoryUsedPercent is derived here as
	// (TotalMemory-FreeMemory)/TotalMemory*100, since RouterOS reports
	// free/total, not used-percent, directly).
	CPULoadPercent    float64 `gorm:"not null;default:0"`
	MemoryUsedPercent float64 `gorm:"not null;default:0"`
}
