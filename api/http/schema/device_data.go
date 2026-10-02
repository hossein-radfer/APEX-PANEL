package schema

type DailyTrafficUsageResponse struct {
	InterfaceId   uint   `json:"interface_id"`
	Date          string `json:"date"`
	DownloadUsage string `json:"download"`
	UploadUsage   string `json:"upload"`
	TotalUsage    string `json:"total"`
}

type DeviceStatsResponse struct {
	ServerInfo        *ServerStatsResponse
	InterfaceInfo     *InterfaceStatsResponse
	PeerInfo          *PeerStatsResponse
	TrafficInfo       *TrafficInfo
	DeviceIdentity    *DeviceIdentity
	DeviceInfo        *DeviceInfo
	DeviceIPv4Address *DeviceIPv4Address
	DNSConfig         *DNSConfig
}

// TrafficInfo.TotalUsage is the unified figure: WireGuard's cumulative
// counter (WireGuardUsage, delta-accumulated since the last reset -- see
// model.TotalTrafficUsage) plus a live snapshot sum of every User Manager
// account's current usage (UserManagerUsage, direct-copy of RouterOS's own
// counters -- see cmd/jobs/traffic.go's processUserManagerAccountUsage).
// The two source figures have different semantics (one is a running total
// across resets, the other is a live snapshot), which is why both are
// exposed separately alongside the combined total -- "Reset" only zeroes
// the WireGuard side (see ResetTotalTrafficUsage), so UserManagerUsage can
// keep contributing to TotalUsage even right after a reset.
type TrafficInfo struct {
	TotalUsage       string `json:"total_usage"`
	WireGuardUsage   string `json:"wireguard_usage"`
	UserManagerUsage string `json:"user_manager_usage"`
}

type DeviceInfo struct {
	BoardName   string `json:"board_name"`
	OSVersion   string `json:"os_version"`
	CpuArch     string `json:"cpu_arch"`
	Uptime      string `json:"uptime"`
	CpuLoad     string `json:"cpu_load"`
	TotalMemory string `json:"total_memory"`
	FreeMemory  string `json:"free_memory"`
	TotalDisk   string `json:"total_disk"`
	FreeDisk    string `json:"free_disk"`
}

type DeviceResource struct {
	Uptime      string `json:"uptime"`
	CpuUsage    string `json:"cpu_usage"`
	MemoryUsage string `json:"memory_usage"`
	DiskUsage   string `json:"disk_usage"`
}

type DeviceIdentity struct {
	Identity string `json:"identity"`
}

type DeviceIPv4Address struct {
	IPv4 string `json:"ipv4,omitempty"`
	ISP  string `json:"isp,omitempty"`
}

type DNSConfig struct {
	DnsServer string `json:"dns_servers,omitempty"`
}
