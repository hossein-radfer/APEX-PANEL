package schema

type PeerStatus string

var (
	ActivePeer    PeerStatus = "active"
	InactivePeer  PeerStatus = "inactive"
	ExpiredPeer   PeerStatus = "expired"
	SuspendedPeer PeerStatus = "suspended"
)

type NewPeerAllowedAddressRequest struct {
	InterfaceId uint `json:"interface_id" validate:"required"`
}

// BulkDeleteRequest/BulkDeleteResponse are shared across the Peer, User
// Manager account, and V2Ray package bulk-delete endpoints (the "delete
// expired/quota-exhausted users" action) -- one generic shape rather than
// three near-identical ones, since all three services now expose the exact
// same BulkDelete*(ids, resellerID) (deleted []uint, failed map[uint]string)
// signature.
type BulkDeleteRequest struct {
	Ids []uint `json:"ids" validate:"required,min=1"`
}

type BulkDeleteFailure struct {
	Id    uint   `json:"id"`
	Error string `json:"error"`
}

type BulkDeleteResponse struct {
	Deleted []uint              `json:"deleted"`
	Failed  []BulkDeleteFailure `json:"failed"`
}

type NewPeerAllowedAddressResponse struct {
	AllowedAddress string `json:"allowed_address"`
}

// BulkCreatePeerRequest creates Count peers on the same interface/endpoint,
// each with its own freshly-generated keypair (WgPeer.GetPeerCredentials)
// and freshly-resolved next-free allowed_address (WgPeer.
// GetNewPeerAllowedAddress) -- mirrors BulkCreateV2RayPackageRequest's
// shape exactly, adapted for WireGuard's extra per-peer identity fields
// that V2Ray packages don't have (a package has no keypair or address of
// its own). DurationDays is converted to an ExpireTime date
// (today+DurationDays) once per batch, matching every peer's ExpireTime to
// the same day -- CreatePeerRequest.ExpireTime itself stays a fixed date
// rather than a duration since that field is shared with the single-create
// form, which lets an admin pick an arbitrary date directly.
type BulkCreatePeerRequest struct {
	Count               int     `json:"count" validate:"required,min=1,max=500"`
	InterfaceId         uint    `json:"interface_id" validate:"required"`
	Endpoint            string  `json:"endpoint" validate:"required"`
	DurationDays        int     `json:"duration_days" validate:"required,min=1"`
	TrafficLimit        *string `json:"traffic_limit,omitempty"`
	DownloadBandwidth   *string `json:"download_bandwidth,omitempty"`
	UploadBandwidth     *string `json:"upload_bandwidth,omitempty"`
	DNSServers          *string `json:"dns_servers,omitempty"`
	PersistentKeepAlive *string `json:"persistent_keepalive,omitempty"`
}

type BulkCreatePeerResponse struct {
	Peers []PeerResponse `json:"peers"`
}

type CreatePeerRequest struct {
	Comment             *string `json:"comment,omitempty"`
	TelegramUsername    *string `json:"telegram_username,omitempty"`
	Name                string  `json:"name" validate:"required"`
	InterfaceId         uint    `json:"interface_id" validate:"required"`
	PrivateKey          string  `json:"private_key" validate:"required"`
	PublicKey           string  `json:"public_key" validate:"required"`
	AllowedAddress      string  `json:"allowed_address" validate:"required"`
	DNSServers          *string `json:"dns_servers,omitempty"`
	PresharedKey        *string `json:"preshared_key,omitempty"`
	PersistentKeepAlive *string `json:"persistent_keepalive"`
	Endpoint            string  `json:"endpoint" validate:"required"`
	ExpireTime          *string `json:"expire_time,omitempty"`
	TrafficLimit        *string `json:"traffic_limit,omitempty"`
	DownloadBandwidth   *string `json:"download_bandwidth,omitempty"`
	UploadBandwidth     *string `json:"upload_bandwidth,omitempty"`
}

type UpdatePeerRequest struct {
	Disabled            *bool   `json:"disabled,omitempty"`
	Comment             *string `json:"comment,omitempty"`
	TelegramUsername    *string `json:"telegram_username,omitempty"`
	Name                string  `json:"name,omitempty"`
	AllowedAddress      string  `json:"allowed_address,omitempty"`
	DNSServers          *string `json:"dns_servers,omitempty"`
	PresharedKey        *string `json:"preshared_key,omitempty"`
	PersistentKeepAlive *string `json:"persistent_keepalive,omitempty"`
	ExpireTime          *string `json:"expire_time,omitempty"`
	TrafficLimit        *string `json:"traffic_limit,omitempty"`
	DownloadBandwidth   *string `json:"download_bandwidth,omitempty"`
	UploadBandwidth     *string `json:"upload_bandwidth,omitempty"`
}

type UpdatePeerShareExpireRequest struct {
	ExpireTime *string `json:"expire_time"`
}

type PeerCredentialsResponse struct {
	PublicKey  string `json:"public_key"`
	PrivateKey string `json:"private_key"`
}

type PeerDetailsResponse struct {
	Name          string  `json:"name"`
	TrafficLimit  *string `json:"traffic_limit"`
	ExpireTime    *string `json:"expire_time"`
	DownloadUsage string  `json:"download_usage"`
	UploadUsage   string  `json:"upload_usage"`
	TotalUsage    string  `json:"total_usage"`
	UsagePercent  *string `json:"usage_percent"`
	IsOnline      bool    `json:"is_online"`
	// Location is the admin-set "لوکیشن" label for this peer's WireGuard
	// interface (e.g. "آلمان", "ترکیه") -- nil when no label has been set.
	// See PeerDetailsResponse's own caller (WgPeer.GetPeerDetails) for
	// exactly how it's resolved.
	Location *string `json:"location,omitempty"`
}

type PeerShareStatusResponse struct {
	IsShared   bool    `json:"is_shared"`
	UUID       *string `json:"uuid"`
	ExpireTime *string `json:"expire_time"`
}

type PeerResponse struct {
	Id                uint         `json:"id"`
	UUID              string       `json:"uuid"`
	Disabled          bool         `json:"disabled"`
	Comment           *string      `json:"comment"`
	TelegramUsername  *string      `json:"telegram_username"`
	Name              string       `json:"name"`
	Interface         string       `json:"interface"`
	AllowedAddress    string       `json:"allowed_address"`
	DNSServers        *string      `json:"dns_servers"`
	TrafficLimit      *string      `json:"traffic_limit"`
	ExpireTime        *string      `json:"expire_time"`
	DownloadBandwidth *string      `json:"download_bandwidth"`
	UploadBandwidth   *string      `json:"upload_bandwidth"`
	TotalUsage        string       `json:"total_usage"`
	Status            []PeerStatus `json:"status"`
	IsOnline          bool         `json:"is_online"`
	IsShared          bool         `json:"is_shared"`
}

type PeerStatsResponse struct {
	RecentOnlinePeers *[]RecentOnlinePeers `json:"recent_online_peers"`
	TotalPeers        int                  `json:"total_peers"`
	OnlinePeers       int                  `json:"online_peers"`
	OfflinePeers      int                  `json:"offline_peers"`
	DisabledPeers     int                  `json:"disabled_peers"`
}

type RecentOnlinePeers struct {
	Name     string `json:"name"`
	LastSeen string `json:"last_seen"`
}

// ResellerActivityResponse reports per-reseller online-peer counts and
// today's traffic usage, for the admin dashboard's reseller breakdown.
type ResellerActivityResponse struct {
	ResellerID   uint   `json:"reseller_id"`
	ResellerName string `json:"reseller_name"`
	OnlinePeers  int    `json:"online_peers"`
	TotalPeers   int    `json:"total_peers"`
	TodayUsageGB string `json:"today_usage_gb"`
}

// ResellerActivitySummaryResponse is the admin-wide rollup: a page of the
// per-reseller breakdown (sorted by name) plus grand totals computed across
// ALL resellers, not just the current page.
type ResellerActivitySummaryResponse struct {
	Resellers         []ResellerActivityResponse `json:"resellers"`
	TotalOnlinePeers  int                         `json:"total_online_peers"`
	TotalTodayUsageGB string                      `json:"total_today_usage_gb"`
	Page              int                         `json:"page"`
	PageSize          int                         `json:"page_size"`
	TotalCount        int                         `json:"total_count"`
	TotalPages        int                         `json:"total_pages"`
}

// SelfActivityResponse is a reseller's own activity snapshot for their
// dashboard: how many of their own peers are online right now, and how much
// traffic they've used today.
type SelfActivityResponse struct {
	OnlinePeers  int    `json:"online_peers"`
	TotalPeers   int    `json:"total_peers"`
	TodayUsageGB string `json:"today_usage_gb"`
}
