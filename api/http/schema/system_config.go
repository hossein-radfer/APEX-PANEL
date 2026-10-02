package schema

// PortConfigResponse reports the currently active listen port (whatever the
// running process actually bound, regardless of source) and any pending
// admin-set override that hasn't taken effect yet (requires a restart).
type PortConfigResponse struct {
	CurrentPort int  `json:"current_port"`
	PendingPort *int `json:"pending_port"`
}

type UpdatePortConfigRequest struct {
	Port int `json:"port" validate:"required,min=1,max=65535"`
}

// DatabaseSizeTableEntry is one table's share of the database file, as
// measured by SQLite's own dbstat virtual table -- BytesUsed is the exact
// on-disk page usage (including that table's own indexes' pages, reported
// as separate entries with an "idx_"-prefixed Name, matching sqlite_master's
// own naming for auto/explicit indexes), not an estimate.
type DatabaseSizeTableEntry struct {
	Name      string `json:"name"`
	BytesUsed int64  `json:"bytes_used"`
}

// DatabaseSizeResponse reports the total on-disk size of the panel's own
// SQLite database file (TotalBytes -- from the filesystem, so it also
// reflects any pending free space SQLite hasn't reclaimed via VACUUM yet)
// alongside a per-table breakdown of what's actually using that space.
// Tables are always returned sorted largest-first so the admin sees the
// biggest contributors without needing to sort client-side.
type DatabaseSizeResponse struct {
	TotalBytes int64                    `json:"total_bytes"`
	Tables     []DatabaseSizeTableEntry `json:"tables"`
}

// SystemHealthResponse bundles every tile the admin's health dashboard
// shows -- see service.SystemHealth's own doc comment for the semantics
// of each field. CPUPercent is -1 when unavailable (e.g. the very first
// call after a process restart, before two /proc/stat samples exist).
type SystemHealthResponse struct {
	CPUPercent         float64 `json:"cpu_percent"`
	MemoryTotalBytes   int64   `json:"memory_total_bytes"`
	MemoryUsedBytes    int64   `json:"memory_used_bytes"`
	DiskTotalBytes     int64   `json:"disk_total_bytes"`
	DiskUsedBytes      int64   `json:"disk_used_bytes"`
	AutoBackupEnabled  bool    `json:"auto_backup_enabled"`
	LastBackupSentDate string  `json:"last_backup_sent_date"`
	BackupIsUpToDate   bool    `json:"backup_is_up_to_date"`
}
