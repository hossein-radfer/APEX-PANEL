package schema

// CreateV2RayPackageRequest.PanelIDs is optional -- an empty/omitted list
// means "every panel the caller is allowed to use" (every registered panel
// for an admin-direct package, or every panel a reseller has been granted
// access to for a reseller package), matching the smart-default behavior
// the admin UI presents: a single registered panel is used automatically
// with no picker shown, multiple panels default to "all" with an explicit
// per-panel checkbox list to narrow the selection.
type CreateV2RayPackageRequest struct {
	CustomerLabel *string `json:"customer_label,omitempty"`
	// Comment is an optional, admin-only note -- see model.V2RayPackage.Comment's
	// own doc comment for why this is never customer-facing.
	Comment          *string `json:"comment,omitempty"`
	TotalVolumeBytes int64   `json:"total_volume_bytes" validate:"required,min=1"`
	DurationDays     int     `json:"duration_days" validate:"required,min=1"`
	PanelIDs         []uint  `json:"panel_ids,omitempty"`
}

// BulkCreateV2RayPackageRequest creates Count packages in one call, each
// sharing the same volume/duration/panel selection but with its own
// randomly-generated CustomerLabel (see V2RayPackageService.
// generateBulkPackageLabel) -- the admin's own explicit requirement: a
// random base name with a "_<volume>GB_<days>d" suffix so the label
// itself communicates the package's plan when it shows up inside a V2Ray
// client app, matching how a manually-typed CustomerLabel already would.
type BulkCreateV2RayPackageRequest struct {
	Count            int    `json:"count" validate:"required,min=1,max=500"`
	TotalVolumeBytes int64  `json:"total_volume_bytes" validate:"required,min=1"`
	DurationDays     int    `json:"duration_days" validate:"required,min=1"`
	PanelIDs         []uint `json:"panel_ids,omitempty"`
}

type BulkCreateV2RayPackageResponse struct {
	Packages []V2RayPackageResponse `json:"packages"`
}

// ExportV2RayPackagesRequest drives the Excel/txt export of a bulk-created
// (or any) set of packages -- IncludeShareLink/IncludeSubscriptionLink are
// independent checkboxes (either, or both, per the admin's own explicit
// "هم بتونم تکی انتخاب کنم هم دوتاش" requirement). BaseURL is supplied by
// the frontend (window.location.origin) since the backend has no reliable
// notion of the public-facing URL outside of a live request -- mirrors
// GetPackageShareDetails's own publicBaseURL parameter for the exact same
// reason.
type ExportV2RayPackagesRequest struct {
	PackageIDs              []uint `json:"package_ids" validate:"required,min=1"`
	IncludeShareLink        bool   `json:"include_share_link"`
	IncludeSubscriptionLink bool   `json:"include_subscription_link"`
	BaseURL                 string `json:"base_url" validate:"required"`
	// Format is "xlsx" or "txt".
	Format string `json:"format" validate:"required,oneof=xlsx txt"`
}

type UpdateV2RayPackageRequest struct {
	CustomerLabel    *string `json:"customer_label,omitempty"`
	Comment          *string `json:"comment,omitempty"`
	TotalVolumeBytes *int64  `json:"total_volume_bytes,omitempty"`
	DurationDays     *int    `json:"duration_days,omitempty"`
	Status           *string `json:"status,omitempty" validate:"omitempty,oneof=active suspended expired"`
	// PanelIDs, when present (even as an empty, non-nil list), REPLACES
	// this package's full set of x-ui panel locations -- a confirmed,
	// reported gap: there was previously no way to edit which panels an
	// existing package's client exists on at all after creation, forcing
	// an admin to delete and recreate the whole package just to add or
	// remove one location. A pointer (not a bare slice) so "omit this
	// field entirely" (leave locations untouched) is distinguishable from
	// "replace with an empty list" (remove every location), matching this
	// request's own every-other-field "nil means don't touch" convention.
	PanelIDs *[]uint `json:"panel_ids,omitempty"`
}

// V2RayPackageLocationStatus surfaces per-panel health directly on the
// package row -- so a customer/admin can see at a glance that e.g. one of
// three registered panels is currently unreachable, without that failure
// ever affecting the package's overall usability (see
// V2RayPackageLocation's doc comment on why one dead panel never breaks
// the whole package).
type V2RayPackageLocationStatus struct {
	PanelID       uint    `json:"panel_id"`
	PanelName     string  `json:"panel_name"`
	Enabled       bool    `json:"enabled"`
	LastSyncedAt  *string `json:"last_synced_at,omitempty"`
	LastSyncError *string `json:"last_sync_error,omitempty"`
	// HadWrongFlow/FlowRepaired surface the one-time client-flow repair
	// pass's result (see V2RayPackageLocation.FlowRepairedAt's doc
	// comment) directly on the package row -- HadWrongFlow=true means this
	// specific customer's client was actually broken (created with the
	// old hardcoded xtls-rprx-vision flow on a non-TLS/Reality inbound)
	// and has now been auto-repaired; FlowRepaired=false means the check
	// hasn't run yet (a panel that was unreachable on every tick so far).
	HadWrongFlow bool `json:"had_wrong_flow"`
	FlowRepaired bool `json:"flow_repaired"`
	// IsOnline mirrors V2RayPackageLocation.IsOnline -- this location's
	// client's connection state as of the most recent sync tick's
	// panel-wide onlines poll (see V2RaySyncService.SyncPackageUsage),
	// shown as a separate indicator alongside Enabled/LastSyncError's
	// existing panel-health dot, not a replacement for it -- a location can
	// be perfectly healthy (Enabled, no LastSyncError) while IsOnline=false
	// simply because no client is connected right now.
	IsOnline bool `json:"is_online"`
}

type V2RayPackageResponse struct {
	Id               uint                         `json:"id"`
	UUID             string                       `json:"uuid"`
	CustomerLabel    *string                      `json:"customer_label,omitempty"`
	Comment          *string                      `json:"comment,omitempty"`
	TotalVolumeBytes int64                        `json:"total_volume_bytes"`
	UsedBytes        int64                        `json:"used_bytes"`
	DurationDays     int                          `json:"duration_days"`
	StartAt          *string                      `json:"start_at,omitempty"`
	ExpireAt         *string                      `json:"expire_at,omitempty"`
	Status           string                       `json:"status"`
	IsShared         bool                         `json:"is_shared"`
	Locations        []V2RayPackageLocationStatus `json:"locations"`
	ResellerID       *uint                        `json:"reseller_id,omitempty"`
	ResellerName     *string                      `json:"reseller_name,omitempty"`
}

// AssignResellerXuiPanelsRequest replaces the full set of x-ui panels a
// reseller may use -- mirrors AssignResellerInterfacesRequest's shape.
// An empty PanelIDs list means the reseller has been granted NO panels
// (mirrors ResellerInterface's "must be explicitly assigned" contract) --
// distinct from CreateV2RayPackageRequest.PanelIDs being empty, which
// instead means "use every panel I'm allowed to use."
type AssignResellerXuiPanelsRequest struct {
	PanelIDs []uint `json:"panel_ids"`
}

type V2RaySaleTitleRequest struct {
	PanelID uint   `json:"panel_id" validate:"required"`
	Title   string `json:"title" validate:"required"`
}

type V2RaySaleTitleResponse struct {
	PanelID   uint   `json:"panel_id"`
	PanelName string `json:"panel_name"`
	Title     string `json:"title"`
}

type V2RayPackageShareStatusResponse struct {
	IsShared   bool    `json:"is_shared"`
	UUID       *string `json:"uuid"`
	ExpireTime *string `json:"expire_time"`
}

// V2RayShareLocationUsage is one panel's own connection-info box on the
// share page -- title (the reseller's custom sale title, falling back to
// the panel's own default, matching the actual title baked into
// ConfigLink itself), that single panel's own raw config link (plain,
// NOT base64 -- the customer's app needs the literal vless:// URI to scan/
// paste, unlike SubscriptionURL below which points at the combined,
// base64-encoded multi-panel feed), and that location's own used-bytes
// figure -- one box per registered/authorized panel on the share page,
// see V2RayPackageShareDetailsResponse's own doc comment for the full
// per-panel-box + final-summary-box layout this backs.
type V2RayShareLocationUsage struct {
	Title      string `json:"title"`
	ConfigLink string `json:"config_link"`
	UsedBytes  int64  `json:"used_bytes"`
	// IsOnline mirrors V2RayPackageLocationStatus.IsOnline -- see its doc
	// comment. Shown per-location on the share page, alongside the
	// existing package-wide V2RayPackageShareDetailsResponse.IsOnline
	// summary flag.
	IsOnline bool `json:"is_online"`
	// Protocol mirrors XuiPanel.Protocol (informational, set by the admin
	// at panel registration) -- shown alongside the location's title on
	// the subscription landing page.
	Protocol string `json:"protocol,omitempty"`
}

// V2RayPackageShareDetailsResponse is the public, unauthenticated payload
// behind the share page. Layout (per the exact spec this backs): one box
// per entry in Locations (panel title, that panel's own ConfigLink, the
// shared ExpireAt, and that location's own UsedBytes -- the frontend
// renders a QR code of ConfigLink client-side, no separate QR field
// needed), plus one final summary box showing the package-wide
// TotalVolumeBytes/UsedBytes/UsagePercent, IsOnline, and SubscriptionURL
// (the combined, base64-encoded multi-panel feed for v2rayNG/v2box/etc,
// distinct from any single Locations[i].ConfigLink).
type V2RayPackageShareDetailsResponse struct {
	SubscriptionURL string `json:"subscription_url"`
	// CustomerLabel mirrors V2RayPackage.CustomerLabel -- shown as the
	// profile name on the subscription landing page (see item 14's
	// profile-card spec); falls back to a generic label client-side when
	// nil (this codebase has no separate customer/account name field
	// anywhere else either, see V2RayPackage's own doc comment).
	CustomerLabel    *string `json:"customer_label,omitempty"`
	Status           string  `json:"status"`
	TotalVolumeBytes int64   `json:"total_volume_bytes"`
	UsedBytes        int64   `json:"used_bytes"`
	UsagePercent     *string `json:"usage_percent"`
	ExpireAt         *string `json:"expire_at,omitempty"`
	// DaysRemaining is nil when ExpireAt is nil (never expires); 0 means
	// expired today or earlier (never negative -- clamped, matching
	// UsagePercent's own "never show a nonsensical negative number" rule).
	DaysRemaining *int                      `json:"days_remaining,omitempty"`
	IsOnline      bool                      `json:"is_online"`
	Locations     []V2RayShareLocationUsage `json:"locations"`
}

// V2RaySelfSummaryResponse is a reseller's own V2Ray dashboard aggregate --
// mirrors UserManagerSelfSummaryResponse's shape exactly, scoped to a
// reseller's own packages. Deliberately does not require CanResellV2Ray to
// be true to view (a reseller who used to have V2Ray access but was
// revoked should still see their existing, now frozen, usage rather than
// the section vanishing).
type V2RaySelfSummaryResponse struct {
	OnlinePackages int    `json:"online_packages"`
	TotalPackages  int    `json:"total_packages"`
	QuotaBytes     *int64 `json:"quota_bytes"` // nil = unlimited
	UsedBytes      int64  `json:"used_bytes"`
	RemainingBytes *int64 `json:"remaining_bytes"` // nil = unlimited
	MaxPackages    *int   `json:"max_packages"`    // nil = unlimited
}

// V2RayAdminSummaryResponse is the admin dashboard's panel-wide V2Ray
// rollup -- every registered XuiPanel's own health/location count (see
// V2RayPackageLocationStatus's own doc comment on why one dead panel is
// tracked per-location, never surfaced as a single whole-system failure),
// plus grand totals across every package (admin-direct and every
// reseller's) system-wide.
type V2RayAdminSummaryResponse struct {
	TotalPackages    int                      `json:"total_packages"`
	TotalLocations   int                      `json:"total_locations"`
	OnlineLocations  int                      `json:"online_locations"`
	TotalVolumeBytes int64                    `json:"total_volume_bytes"`
	TotalUsedBytes   int64                    `json:"total_used_bytes"`
	Panels           []V2RayAdminSummaryPanel `json:"panels"`
}

// V2RayAdminSummaryPanel is one registered XuiPanel's own row in the
// admin dashboard rollup -- LocationCount/OnlineCount let the admin see at
// a glance which panel is actually carrying load, HasRecentError surfaces
// whether ANY of that panel's locations recorded a sync error on their
// last tick (a coarse, panel-level health signal derived from
// per-location data, matching V2RayLocationStatus's own indicator dot).
type V2RayAdminSummaryPanel struct {
	PanelID        uint   `json:"panel_id"`
	PanelName      string `json:"panel_name"`
	LocationCount  int    `json:"location_count"`
	OnlineCount    int    `json:"online_count"`
	HasRecentError bool   `json:"has_recent_error"`
}

// V2RayLiveUsageLocation is one location's row in the on-demand "view live
// usage" table (V2RayPackageService.GetLiveUsage) -- unlike
// V2RayPackageLocationStatus (which always reflects the periodic sync
// job's last cached reading), every field here comes from a
// getClientTraffics call made AT REQUEST TIME, for this one action only.
// Error is set (Up/Down/Total left zero) when this specific location's
// live call failed -- mirrors the sync job's own "one dead panel never
// blocks the others" tolerance, applied here to a single on-demand check
// instead of a scheduled tick.
type V2RayLiveUsageLocation struct {
	PanelID   uint   `json:"panel_id"`
	PanelName string `json:"panel_name"`
	Enabled   bool   `json:"enabled"`
	UpBytes   int64  `json:"up_bytes"`
	DownBytes int64  `json:"down_bytes"`
	// TotalBytes is Up+Down for this on-demand reading -- NOT necessarily
	// equal to the cached UsedBytesCached the periodic job maintains, since
	// x-ui's own counter may have been reset/rolled over between the last
	// tick and this on-demand check; this table intentionally shows the
	// raw current reading, not a delta-reconciled figure.
	TotalBytes int64   `json:"total_bytes"`
	Error      *string `json:"error,omitempty"`
}

// V2RayLiveUsageResponse is GetLiveUsage's full result -- Locations is the
// per-panel breakdown table, TotalBytes/TotalVolumeBytes let the frontend
// render the same "used of total" summary row the package list already
// shows, computed from this SAME live read rather than the (now
// just-refreshed) cache, so the two numbers are guaranteed consistent
// within one response.
type V2RayLiveUsageResponse struct {
	TotalVolumeBytes int64                    `json:"total_volume_bytes"`
	TotalUsedBytes   int64                    `json:"total_used_bytes"`
	Locations        []V2RayLiveUsageLocation `json:"locations"`
}
