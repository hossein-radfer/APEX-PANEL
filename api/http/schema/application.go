package schema

// CreateApplicationRequest builds a mobile-app end-user bundle spanning
// WireGuard/User Manager/V2Ray in one call. Empty/omitted InterfaceIDs/
// UserManagerGroups/XuiPanelIDs means "none of that protocol" (unlike
// V2RayPackage's own "empty PanelIDs means every allowed panel" default --
// an Application's resource set must be explicit, since silently
// provisioning a protocol nobody asked for is a much bigger mistake here
// than under-provisioning one).
type CreateApplicationRequest struct {
	Name              string                    `json:"name" validate:"required"`
	InterfaceIDs      []uint                    `json:"interface_ids,omitempty"`
	UserManagerGroups []ApplicationGroupProfile `json:"user_manager_groups,omitempty"`
	XuiPanelIDs       []uint                    `json:"xui_panel_ids,omitempty"`

	// ApplicationPlanID, when set, copies model.ApplicationPlan's bundle
	// (TotalVolumeBytes, DurationDays, MaxOnlineUsers, speed limits) into
	// this request's own fields below BEFORE creation -- see
	// ApplicationService.applyPlanIfSet's own doc comment. Any of those
	// fields also explicitly set in this same request OVERRIDE the plan's
	// value (manual override), since they are applied first and the plan
	// only fills in what's still zero.
	ApplicationPlanID *uint `json:"application_plan_id,omitempty"`

	// required_without=ApplicationPlanID: these three stay mandatory for a
	// plan-less (manual) creation exactly as before, but a request that
	// references a plan may omit them and let the plan fill them in --
	// mirrors CreateDNSAccountRequest's "0 = fill from plan" convention,
	// adapted with an explicit validator tag instead of a bare zero-value
	// check, since 0 is not itself a valid volume/duration/online-user
	// count for Application (unlike DNS, which treats 0 as "unlimited").
	TotalVolumeBytes int64 `json:"total_volume_bytes" validate:"required_without=ApplicationPlanID,omitempty,min=1"`
	DurationDays     int   `json:"duration_days" validate:"required_without=ApplicationPlanID,omitempty,min=1"`
	MaxOnlineUsers   int   `json:"max_online_users" validate:"required_without=ApplicationPlanID,omitempty,min=1"`
	// AppUsername/AppPassword are optional -- omitted means the service
	// generates a random 5-character value for each (see
	// ApplicationService.CreateApplication's own doc comment).
	AppUsername *string `json:"app_username,omitempty"`
	AppPassword *string `json:"app_password,omitempty"`
	// DownloadSpeedLimitMbps/UploadSpeedLimitMbps are optional -- nil (or
	// omitted) means unlimited. See model.Application's own doc comment
	// on why this is enforced client-side by the mobile app, not via a
	// server-side MikroTik queue.
	DownloadSpeedLimitMbps *int `json:"download_speed_limit_mbps,omitempty" validate:"omitempty,min=1"`
	UploadSpeedLimitMbps   *int `json:"upload_speed_limit_mbps,omitempty" validate:"omitempty,min=1"`
}

// ApplicationGroupProfile pairs a RouterOS User Manager group with the
// profile to create the account under -- CreateUserManagerAccountRequest
// requires both (a group alone is not enough to create a working
// account), so an Application's group selection must carry a profile
// choice per group, not just a bare list of group names.
type ApplicationGroupProfile struct {
	GroupName   string `json:"group_name" validate:"required"`
	ProfileName string `json:"profile_name" validate:"required"`
}

// DownloadSpeedLimitMbps/UploadSpeedLimitMbps follow the same convention
// as Comment elsewhere in this codebase: omitting the field leaves the
// stored value untouched, sending a number sets it -- there is
// currently no way to explicitly clear an already-set limit back to
// unlimited through this endpoint (a pre-existing limitation shared
// with every other *T update field here, not unique to this one).
type UpdateApplicationRequest struct {
	Name                   *string `json:"name,omitempty"`
	TotalVolumeBytes       *int64  `json:"total_volume_bytes,omitempty"`
	DurationDays           *int    `json:"duration_days,omitempty"`
	MaxOnlineUsers         *int    `json:"max_online_users,omitempty"`
	Disabled               *bool   `json:"disabled,omitempty"`
	DownloadSpeedLimitMbps *int    `json:"download_speed_limit_mbps,omitempty" validate:"omitempty,min=1"`
	UploadSpeedLimitMbps   *int    `json:"upload_speed_limit_mbps,omitempty" validate:"omitempty,min=1"`
	// InterfaceIDs/UserManagerGroups/XuiPanelIDs, when present (even as
	// an empty, non-nil list), REPLACE this Application's full set of
	// WireGuard interfaces / User Manager group+profile pairs / V2Ray
	// panels, respectively -- a confirmed, reported gap: there was
	// previously no way to edit which protocols/interfaces/locations an
	// existing Application uses at all after creation, forcing an admin
	// to delete and recreate the whole Application (losing its
	// AppUsername/AppPassword/usage history) just to add or remove one.
	// Pointers (not bare slices), matching UpdateV2RayPackageRequest.
	// PanelIDs' identical convention, so "omit this field" (leave that
	// resource type untouched) is distinguishable from "replace with an
	// empty list" (remove every resource of that type). Each of the
	// three resource types is reconciled completely independently --
	// e.g. sending only InterfaceIDs never touches UserManagerGroups or
	// XuiPanelIDs at all.
	InterfaceIDs      *[]uint                    `json:"interface_ids,omitempty"`
	UserManagerGroups *[]ApplicationGroupProfile `json:"user_manager_groups,omitempty"`
	XuiPanelIDs       *[]uint                    `json:"xui_panel_ids,omitempty"`
}

type ApplicationResourceResponse struct {
	ResourceID   uint    `json:"resource_id"`   // InterfaceID / PanelID
	ResourceName string  `json:"resource_name"` // Interface.Name / XuiPanel.Name, or GroupName for User Manager
	ProfileName  *string `json:"profile_name,omitempty"`
	Label        *string `json:"label,omitempty"` // from ApplicationResourceLocation, if set
	Enabled      bool    `json:"enabled"`
}

type ApplicationResponse struct {
	Id                  uint                          `json:"id"`
	ResellerID          *uint                         `json:"reseller_id,omitempty"`
	Name                string                        `json:"name"`
	AppUsername         string                        `json:"app_username"`
	AppPassword         string                        `json:"app_password"`
	TotalVolumeBytes    int64                         `json:"total_volume_bytes"`
	UsedBytes           int64                         `json:"used_bytes"`
	DurationDays        int                           `json:"duration_days"`
	StartAt             *string                       `json:"start_at,omitempty"`
	ExpireAt            *string                       `json:"expire_at,omitempty"`
	MaxOnlineUsers      int                           `json:"max_online_users"`
	Status              string                        `json:"status"`
	Disabled            bool                          `json:"disabled"`
	DownloadSpeedLimitMbps *int                       `json:"download_speed_limit_mbps,omitempty"`
	UploadSpeedLimitMbps   *int                       `json:"upload_speed_limit_mbps,omitempty"`
	WireGuardPeers      []ApplicationResourceResponse `json:"wireguard_peers"`
	UserManagerAccounts []ApplicationResourceResponse `json:"user_manager_accounts"`
	V2RayPackages       []ApplicationResourceResponse `json:"v2ray_packages"`
	// ProvisioningErrors is only ever populated by CreateApplication's
	// own response (never by a later GET) -- one entry per requested
	// resource that failed to provision (e.g. an IP-pool collision on
	// one specific WireGuard interface), so the admin can see a partial
	// failure immediately instead of assuming every checked
	// protocol/resource succeeded just because the request itself
	// returned 201. See ApplicationService.CreateApplication's own doc
	// comment on why one resource's failure no longer silently skips
	// every resource requested after it.
	ProvisioningErrors []string `json:"provisioning_errors,omitempty"`
}

type ApplicationsResponse struct {
	Applications []ApplicationResponse `json:"applications"`
}

// SetApplicationResourceLocationRequest drives the admin-only "location
// label" settings page (three lists: interfaces / user manager groups /
// xui panels, each with an editable Label) -- purely cosmetic, see
// model.ApplicationResourceLocation's own doc comment.
type SetApplicationResourceLocationRequest struct {
	ResourceType string `json:"resource_type" validate:"required,oneof=wireguard_interface user_manager_group xui_panel"`
	ResourceKey  string `json:"resource_key" validate:"required"`
	Label        string `json:"label" validate:"required"`
}

type ApplicationResourceLocationResponse struct {
	ResourceType string `json:"resource_type"`
	ResourceKey  string `json:"resource_key"`
	Label        string `json:"label"`
}

type ApplicationResourceLocationsResponse struct {
	Locations []ApplicationResourceLocationResponse `json:"locations"`
}

// --- Mobile app's own login/session (unauthenticated by admin/reseller JWT) ---

type AppLoginRequest struct {
	Username string `json:"username" validate:"required"`
	Password string `json:"password" validate:"required"`
	// DeviceID/DeviceLabel identify the device logging in -- see
	// model.ApplicationDeviceSession's own doc comment for why DeviceID
	// is a client-generated UUID rather than an OS-level identifier.
	// Both optional so an older app build (or a non-device caller like a
	// manual API test) never fails login just for omitting them -- a
	// login with no DeviceID simply doesn't get a device-session row.
	DeviceID    string `json:"device_id,omitempty"`
	DeviceLabel string `json:"device_label,omitempty"`
}

type AppLoginResponse struct {
	AccessToken string `json:"access_token"`
}

// AppOnlineCountResponse backs GET /api/app/online-count -- the mobile
// app calls this BEFORE attempting a WireGuard/V2Ray connection, since
// the panel cannot pre-block a WireGuard UDP handshake or an x-ui
// subscription pull the way RouterOS natively enforces User Manager's
// own shared-users= limit. The app compares OnlineCount against
// MaxOnlineUsers itself and refuses to connect locally if already at the
// limit, showing the admin's configured error message.
type AppOnlineCountResponse struct {
	OnlineCount    int `json:"online_count"`
	MaxOnlineUsers int `json:"max_online_users"`
}

// --- Connectable configs (GET /api/app/me/connect-configs) ---------------

// AppConnectWireGuardConfig carries EVERY field the mobile app's
// com.wireguard.android.backend Config.Builder needs to build a working
// tunnel directly -- structured fields, not a pre-rendered .conf string,
// so the native WireGuard engine never has to parse text. Mirrors exactly
// what buildPeerConfigString (config_file.go) already assembles for the
// desktop .conf download, just shaped as JSON instead of the wireguard.Template string.
type AppConnectWireGuardConfig struct {
	ResourceID          uint    `json:"resource_id"` // matches the resource_id already returned by ApplicationResourceResponse for this same peer
	Label               *string `json:"label,omitempty"`
	PrivateKey          string  `json:"private_key"`
	Address             string  `json:"address"` // Peer.AllowedAddress -- the client-side interface address
	DNS                 string  `json:"dns"`
	PeerPublicKey       string  `json:"peer_public_key"` // the WireGuard SERVER interface's public key
	Endpoint            string  `json:"endpoint"`
	EndpointPort        string  `json:"endpoint_port"`
	AllowedIPs          string  `json:"allowed_ips"`
	PersistentKeepalive string  `json:"persistent_keepalive"`
}

// AppConnectUserManagerAccount carries one account's username/password
// plus one AppConnectUserManagerAccount.Protocols entry per protocol the
// account was actually granted -- mirrors GetAccountShareDetails'
// own per-protocol loop (user_manager.go), reused via
// buildUserManagerProtocolInfos rather than duplicated.
type AppConnectUserManagerAccount struct {
	ResourceID uint                           `json:"resource_id"`
	Label      *string                        `json:"label,omitempty"`
	Username   string                         `json:"username"`
	Password   string                         `json:"password"`
	Protocols  []UserManagerShareProtocolInfo `json:"protocols"`
	// OpenVpnConfig is a complete, ready-to-import .ovpn file -- the
	// admin's one global ApplicationOpenVpnTemplate with this account's
	// own <auth-user-pass> username/password block inlined, or nil if no
	// template has been uploaded yet. The app never needs to know about
	// the template/combination step; it just gets a file it can hand
	// directly to ics-openvpn.
	OpenVpnConfig *string `json:"openvpn_config,omitempty"`
}

// AppConnectV2RayPackage carries both the per-panel raw config link (a
// single vless://... URI, ready to feed directly into the V2Ray core) and
// the package's combined multi-panel subscription body, so the app can
// use whichever shape its V2Ray engine wants.
type AppConnectV2RayPackage struct {
	ResourceID       uint    `json:"resource_id"`
	Label            *string `json:"label,omitempty"`
	ConfigLink       string  `json:"config_link"`       // this specific panel's V2RayPackageLocation.ConfigLinkCached
	SubscriptionBody string  `json:"subscription_body"` // V2RayPackage.CombinedConfigCached (base64)
}

// AppConnectConfigsResponse is the full connectable-credentials payload
// behind GET /api/app/me/connect-configs -- deliberately bypasses every
// IsShared/ShareExpireTime gate the admin-facing share endpoints
// (GetUserConfig/GetAccountShareDetails/GetPackageShareDetails) enforce,
// since app-JWT authentication already proves this caller owns the
// Application these resources belong to. Only includes resources whose
// provisioning has actually completed (PeerID/AccountID/PackageID
// non-nil on the join table row) and that are currently Enabled, mirroring
// ApplicationResourceResponse.Enabled's own meaning. Every field here
// comes from a plain DB read -- see ApplicationService.GetConnectConfigs's
// own doc comment for why this never calls MikroTik/x-ui live.
type AppConnectConfigsResponse struct {
	// DownloadSpeedLimitMbps/UploadSpeedLimitMbps are this Application's
	// own client-side throughput cap (see model.Application's own doc
	// comment for why this is enforced by the app itself rather than
	// server-side) -- a single pair of values applying uniformly to
	// WHICHEVER protocol below the app actually connects with, not a
	// per-resource setting, so they live here at the top level rather
	// than duplicated onto each WireGuard/UserManager/V2Ray entry. Nil
	// means unlimited.
	DownloadSpeedLimitMbps *int `json:"download_speed_limit_mbps,omitempty"`
	UploadSpeedLimitMbps   *int `json:"upload_speed_limit_mbps,omitempty"`

	WireGuardPeers      []AppConnectWireGuardConfig    `json:"wireguard_peers"`
	UserManagerAccounts []AppConnectUserManagerAccount `json:"user_manager_accounts"`
	V2RayPackages       []AppConnectV2RayPackage       `json:"v2ray_packages"`
}

// AppWeeklyUsageDay is one Iran-local calendar day's combined
// upload+download total (bytes) across every peer/account/package this
// Application owns -- Date is "YYYY-MM-DD" (Asia/Tehran), not a
// timestamp, matching how the mobile app's own Usage tab bar chart
// labels each bar.
type AppWeeklyUsageDay struct {
	Date       string `json:"date"`
	TotalBytes int64  `json:"total_bytes"`
}

// AppWeeklyUsageResponse backs GET /api/app/me/weekly-usage -- always
// exactly 7 entries (today plus the preceding 6 Iran-local days, oldest
// first), zero-filled for any day with no UsageSnapshot rows, so the
// mobile app's bar chart never has to backfill missing days itself.
type AppWeeklyUsageResponse struct {
	Days []AppWeeklyUsageDay `json:"days"`
}

// AppDeviceSessionResponse is one row in the mobile app's own Devices
// tab -- see model.ApplicationDeviceSession's own doc comment. IsCurrent
// lets the app grey out/disable the "log out" action on the device the
// caller is currently using (revoking your own current session from
// inside itself is a confusing action to expose). FirstSeenAt is a
// Tehran-local "YYYY-MM-DD" display date (matches every other date shown
// in this app), while LastSeenAt is a raw RFC3339 timestamp (UTC) --
// deliberately NOT pre-formatted, so the app can compute "seen X minutes
// ago" / "online now" freshness client-side, which a date-only string
// can't support.
// IsOnline is true when this device's last-reported resource (see
// model.ApplicationDeviceSession.ResourceType/ResourceID) currently has a
// live connection -- an open IPConnectionLog row for WireGuard/User
// Manager, or V2RayPackageLocation.IsOnline for V2Ray, the exact same
// live-session signal ApplicationService.GetOnlineCount already reuses.
// False (never null) whenever the device has no reported resource yet, so
// the app can render "آفلاین" without a separate null-state.
type AppDeviceSessionResponse struct {
	DeviceID    string `json:"device_id"`
	DeviceLabel string `json:"device_label"`
	FirstSeenAt string `json:"first_seen_at"`
	LastSeenAt  string `json:"last_seen_at"`
	IsCurrent   bool   `json:"is_current"`
	IsOnline    bool   `json:"is_online"`
}

type AppDeviceSessionsResponse struct {
	Devices []AppDeviceSessionResponse `json:"devices"`
}

type AppRevokeDeviceRequest struct {
	DeviceID string `json:"device_id" validate:"required"`
}

// AppReportDeviceResourceRequest backs POST /api/app/devices/report-resource
// -- called once by the mobile app right when it actually establishes a VPN
// tunnel (never on a recurring heartbeat), telling the panel which single
// resource this device is now using so GET /api/app/devices can answer
// "is this specific device online" rather than only a system-wide
// aggregate (see AppOnlineCountResponse's own scoped-limitation doc
// comment, which this endpoint closes on the per-device Devices list).
// ResourceID must be the exact value the app already received on the
// matching AppConnect*Config.ResourceID field from GET /me/connect-configs
// (InterfaceID for wireguard, AccountID for user_manager, PanelID for
// v2ray -- deliberately NOT re-derived here, so the app never needs a
// second lookup).
type AppReportDeviceResourceRequest struct {
	DeviceID     string `json:"device_id" validate:"required"`
	ResourceType string `json:"resource_type" validate:"required,oneof=wireguard_interface user_manager_group xui_panel"`
	ResourceID   uint   `json:"resource_id" validate:"required"`
}

// ApplicationOpenVpnTemplateStatusResponse backs the admin settings
// page's "is a template uploaded, and when" display -- never carries the
// template's own content (see ApplicationController.DownloadOpenVpnTemplate
// for that, a separate admin-only endpoint).
type ApplicationOpenVpnTemplateStatusResponse struct {
	Exists     bool    `json:"exists"`
	UploadedAt *string `json:"uploaded_at,omitempty"`
}

// --- Mobile app version management (admin-only "مدیریت اپلیکیشن") -------

type PublishAppVersionRequest struct {
	VersionCode  int     `json:"version_code" validate:"required,min=1"`
	VersionName  string  `json:"version_name" validate:"required"`
	ReleaseNotes *string `json:"release_notes,omitempty"`
	DownloadURL  string  `json:"download_url" validate:"required,url"`
	IsMandatory  bool    `json:"is_mandatory"`
}

type AppVersionResponse struct {
	Id           uint    `json:"id"`
	VersionCode  int     `json:"version_code"`
	VersionName  string  `json:"version_name"`
	ReleaseNotes *string `json:"release_notes,omitempty"`
	DownloadURL  string  `json:"download_url"`
	IsMandatory  bool    `json:"is_mandatory"`
	PublishedAt  string  `json:"published_at"`
}

type AppVersionsResponse struct {
	Versions []AppVersionResponse `json:"versions"`
}

type SetAppMaintenanceModeRequest struct {
	Enabled bool    `json:"enabled"`
	Message *string `json:"message,omitempty"`
}

type AppMaintenanceModeResponse struct {
	Enabled bool    `json:"enabled"`
	Message *string `json:"message,omitempty"`
}

// AppVersionCheckResponse backs the public GET /api/app/version-check
// endpoint -- called by the mobile app on every launch (before login, so
// even a user who cannot currently log in still finds out they must
// update or that the service is under maintenance). UpdateRequired is
// deliberately computed server-side (comparing the caller's own
// ?current_version against the latest published AppVersion.IsMandatory
// chain -- see ApplicationVersionService.CheckVersion's own doc comment
// for the exact rule) rather than leaving the app to compare version
// codes itself, so a policy change (e.g. retroactively marking an old
// version mandatory-blocked) takes effect for every installed app
// immediately, without an app-side update. MaintenanceMode is
// independent of any specific version -- it can block EVERY version,
// including the latest one, e.g. during a server migration.
type AppVersionCheckResponse struct {
	LatestVersionCode int     `json:"latest_version_code"`
	LatestVersionName string  `json:"latest_version_name"`
	ReleaseNotes      *string `json:"release_notes,omitempty"`
	DownloadURL       string  `json:"download_url"`
	UpdateRequired    bool    `json:"update_required"`
	MaintenanceMode   bool    `json:"maintenance_mode"`
	MaintenanceMessage *string `json:"maintenance_message,omitempty"`
}
