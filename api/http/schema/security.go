package schema

type GeoIPStatusResponse struct {
	CityLoaded bool `json:"city_loaded"`
	ASNLoaded  bool `json:"asn_loaded"`
}

// GeoIPModeResponse/SetGeoIPModeRequest back the Security page's online/
// offline IP-lookup switch (see GeoIPService.Lookup's own doc comment on
// what each mode actually does).
type GeoIPModeResponse struct {
	OnlineEnabled bool `json:"online_enabled"`
}

type SetGeoIPModeRequest struct {
	OnlineEnabled bool `json:"online_enabled"`
}

type SecurityIdentityResponse struct {
	Protocol        string   `json:"protocol"`
	Identity        string   `json:"identity"`
	LastIPAddress   string   `json:"last_ip_address"`
	LastCountry     *string  `json:"last_country,omitempty"`
	LastCity        *string  `json:"last_city,omitempty"`
	LastLat         *float64 `json:"last_lat,omitempty"`
	LastLon         *float64 `json:"last_lon,omitempty"`
	LastASN         *uint    `json:"last_asn,omitempty"`
	LastISP         *string  `json:"last_isp,omitempty"`
	LastConnectedAt string   `json:"last_connected_at"`
	IsCurrentlyOpen bool     `json:"is_currently_open"`
}

type SecurityIdentitiesResponse struct {
	Identities []SecurityIdentityResponse `json:"identities"`
}

type SecuritySessionResponse struct {
	IPAddress      string   `json:"ip_address"`
	ConnectedAt    string   `json:"connected_at"`
	DisconnectedAt *string  `json:"disconnected_at,omitempty"`
	Country        *string  `json:"country,omitempty"`
	Region         *string  `json:"region,omitempty"`
	City           *string  `json:"city,omitempty"`
	Lat            *float64 `json:"lat,omitempty"`
	Lon            *float64 `json:"lon,omitempty"`
	Timezone       *string  `json:"timezone,omitempty"`
	ASN            *uint    `json:"asn,omitempty"`
	ISP            *string  `json:"isp,omitempty"`
	UsedBytes      int64    `json:"used_bytes"`
}

type SecurityHistoryResponse struct {
	Protocol string                    `json:"protocol"`
	Identity string                    `json:"identity"`
	Sessions []SecuritySessionResponse `json:"sessions"`
}

type EtherTrafficRowResponse struct {
	SrcAddress       string   `json:"src_address"`
	IPProtocol       string   `json:"ip_protocol"`
	SrcPort          *int     `json:"src_port,omitempty"`
	TxBytesPerSecond int64    `json:"tx_bytes_per_second"`
	RxBytesPerSecond int64    `json:"rx_bytes_per_second"`
	TxPacketsRate    int64    `json:"tx_packets_rate"`
	RxPacketsRate    int64    `json:"rx_packets_rate"`
	Country          *string  `json:"country,omitempty"`
	City             *string  `json:"city,omitempty"`
	Lat              *float64 `json:"lat,omitempty"`
	Lon              *float64 `json:"lon,omitempty"`
	ISP              *string  `json:"isp,omitempty"`
	SampledAt        string   `json:"sampled_at"`
}

type EtherTrafficResponse struct {
	Flows []EtherTrafficRowResponse `json:"flows"`
}

// SecurityThreatResponse is one detected anomaly for the Security page's
// "تهدیدات" (Threats) panel -- see service.SecurityThreat's own doc
// comment for exactly what each field means and how it's computed.
type SecurityThreatResponse struct {
	Kind            string  `json:"kind"`
	Subject         string  `json:"subject"`
	Detail          string  `json:"detail"`
	Severity        string  `json:"severity"`
	Country         *string `json:"country,omitempty"`
	City            *string `json:"city,omitempty"`
	LastSeenAt      string  `json:"last_seen_at"`
	OccurrenceCount int     `json:"occurrence_count"`
}

type SecurityThreatsResponse struct {
	Threats []SecurityThreatResponse `json:"threats"`
}

// RetentionCleanupRequest mirrors the admin's own explicit retention
// criteria list: a fixed set of named windows plus the two entity-status
// criteria, rather than an arbitrary free-form date picker -- matches
// "هفت روز پیش / یک روز پیش / سه روز پیش / ماه پیش" plus
// "یوزرهایی که حجم‌شان تموم شده / تاریخ‌شان تموم شده / حذف شده".
type RetentionCleanupRequest struct {
	// Target is "connection_log" or "ether_traffic" -- which table this
	// cleanup applies to.
	Target string `json:"target" validate:"required,oneof=connection_log ether_traffic"`
	// OlderThanDays triggers the age-based delete when set (e.g. 1, 3, 7,
	// 30). Mutually exclusive with InactiveUsersOnly in practice, but both
	// may be combined in one request if the caller wants.
	OlderThanDays *int `json:"older_than_days,omitempty"`
	// InactiveUsersOnly triggers
	// SecurityRetentionService.DeleteConnectionLogForInactiveUsers --
	// only meaningful when Target is "connection_log".
	InactiveUsersOnly bool `json:"inactive_users_only,omitempty"`
}

type RetentionCleanupResponse struct {
	DeletedCount int64 `json:"deleted_count"`
}
