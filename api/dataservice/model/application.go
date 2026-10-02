package model

import "time"

// Application is a single mobile-app end-user bundle -- deliberately NOT an
// Admin/Reseller (a fully separate login identity, see AppUsername/
// AppPassword below), and deliberately NOT a Peer/UserManagerAccount/
// V2RayPackage itself. Creating an Application fans out to zero-or-more
// real resources on all three VPN products at once (see
// ApplicationInterface/ApplicationUserManagerGroup/ApplicationXuiPanel),
// all sharing ONE combined volume/duration quota -- mirrors how
// V2RayPackage fans out to one V2RayPackageLocation per panel, just one
// level higher (spanning three independent products instead of one).
type Application struct {
	Model

	// ResellerID nil means admin-owned, matching V2RayPackage/
	// UserManagerAccount's own "nil = admin-direct" convention.
	ResellerID *uint `gorm:"index"`

	// Name is an internal admin/reseller-facing label -- NOT the app
	// login username (AppUsername below), mirrors V2RayPackage.CustomerLabel's
	// "admin bookkeeping, never sent to the end system" role.
	Name string `gorm:"type:varchar(255)"`

	// AppUsername/AppPassword are this Application's OWN separate login
	// credential for the mobile app -- unrelated to any Admin/Reseller
	// username. Plaintext password, same precedent as
	// UserManagerAccount.Password/Peer.PrivateKey/XuiPanel.Password (no
	// at-rest encryption exists anywhere in this codebase). Globally
	// unique since the app's login endpoint has no reseller-scoping
	// concept to disambiguate a duplicate username.
	AppUsername string `gorm:"type:varchar(32);uniqueIndex;not null"`
	AppPassword string `gorm:"type:varchar(64);not null"`

	// ApplicationPlanID records which ApplicationPlan (if any) this
	// Application was created from -- purely for display/bookkeeping
	// ("this application is on the Gold plan"), mirrors
	// DNSAccount.DNSPlanID exactly. Nil means the fields below were set
	// manually with no plan involved. Selecting a plan copies its bundle
	// into the fields below at creation time; they remain independently
	// editable afterward (see ApplicationPlan's own doc comment).
	ApplicationPlanID *uint `gorm:"index"`

	TotalVolumeBytes int64 `gorm:"type:bigint;not null"`
	// UsedBytes is a cached aggregate across every peer/account/package
	// this Application owns, refreshed by EnforceApplicationQuotas --
	// NOT a live sum computed per-request, mirroring Reseller.UsedBytes'
	// own "cached running total, written by a job" convention.
	UsedBytes    int64 `gorm:"type:bigint;not null;default:0"`
	DurationDays int   `gorm:"type:int;not null"`
	StartAt      *time.Time
	ExpireAt     *time.Time

	// MaxOnlineUsers is the admin's "چند کاربر همزمان" limit -- pushed
	// directly onto every UserManagerAccount this Application owns as
	// SharedUsers (RouterOS enforces it natively there); for WireGuard/
	// V2Ray, RouterOS/x-ui have no equivalent native concept, so the
	// mobile app itself enforces this limit client-side by asking
	// GET /api/app/online-count before connecting (see that endpoint's
	// own doc comment -- the panel cannot pre-block a WireGuard UDP
	// handshake the way it can gate an HTTP request).
	MaxOnlineUsers int `gorm:"type:int;not null;default:1"`

	// Status mirrors V2RayPackage.Status's three-value convention
	// ("active" | "suspended" | "expired").
	Status string `gorm:"type:varchar(16);not null;default:'active'"`

	// SuspendedByQuota/WasActiveBeforeSuspend mirror Peer/
	// UserManagerAccount/V2RayPackage's identical quota-suspend
	// bookkeeping -- EnforceApplicationQuotas sets both true when
	// UsedBytes crosses TotalVolumeBytes (or ExpireAt passes), and
	// resumeQuotaSuspendedApplication (mirroring
	// resumeQuotaSuspendedPeers) only re-enables when both are true, so
	// something an admin disabled on purpose is never silently
	// resurrected by a later top-up.
	SuspendedByQuota       bool `gorm:"type:boolean;not null;default:false"`
	WasActiveBeforeSuspend bool `gorm:"type:boolean;not null;default:false"`

	// SuspendedByResellerQuota tracks whether THIS Application was
	// suspended because its OWNING RESELLER's overall ApplicationQuotaBytes
	// pool went over -- a separate flag from SuspendedByQuota above
	// (which fires on this Application's OWN TotalVolumeBytes), mirroring
	// DNSAccount/V2RayPackage's identical two-flag distinction so raising
	// one limit never wrongly resumes an Application still suspended by
	// the other.
	SuspendedByResellerQuota bool `gorm:"type:boolean;not null;default:false"`

	// Disabled is the admin/reseller's own manual on/off switch --
	// distinct from SuspendedByQuota (automatic). Per the admin's own
	// explicit requirement ("یوزر برنامه نباید غیرفعال بشه"), Disabled
	// (like SuspendedByQuota) only ever blocks VPN resource access, NEVER
	// the app login itself -- AppUsername/AppPassword keep working so
	// the app can still show the account's status/quota screen.
	Disabled bool `gorm:"type:boolean;not null;default:false"`

	// DownloadSpeedLimitMbps/UploadSpeedLimitMbps are an optional
	// per-Application throughput cap, nil meaning unlimited. Unlike
	// Peer.DownloadBandwidth/UploadBandwidth (which drive a real MikroTik
	// Simple Queue, server-side), these are enforced client-side by the
	// Android app itself, applying uniformly across all supported
	// protocol families from one place. The value is carried down to the
	// app via AppConnectConfigsResponse (see that struct's own doc
	// comment) and applied identically regardless of which protocol the
	// user connects with.
	DownloadSpeedLimitMbps *int `gorm:"type:int"`
	UploadSpeedLimitMbps   *int `gorm:"type:int"`
}

// ApplicationInterface grants one Application access to create/own a peer
// on a specific WireGuard interface -- mirrors ResellerInterface's shape
// exactly, one level down (per-Application instead of per-Reseller).
// PeerID is nil until CreateApplication actually provisions the real peer,
// then holds the FK for every subsequent usage/quota/teardown operation.
type ApplicationInterface struct {
	Model
	ApplicationID uint  `gorm:"uniqueIndex:idx_application_interface;not null"`
	InterfaceID   uint  `gorm:"uniqueIndex:idx_application_interface;not null"`
	PeerID        *uint `gorm:"index"`
}

// ApplicationUserManagerGroup grants one Application access to create/own
// an account in a specific RouterOS User Manager group -- mirrors
// ResellerUserManagerGroup's shape (string-keyed, since RouterOS groups
// aren't modeled as their own DB rows -- see ResellerUserManagerGroup's
// own doc comment for why).
type ApplicationUserManagerGroup struct {
	Model
	ApplicationID uint   `gorm:"uniqueIndex:idx_application_um_group;not null"`
	GroupName     string `gorm:"type:varchar(255);uniqueIndex:idx_application_um_group;not null"`
	AccountID     *uint  `gorm:"index"`
}

// ApplicationXuiPanel grants one Application access to create/own a V2Ray
// package on a specific x-ui panel -- mirrors ResellerXuiPanelAccess's
// shape exactly, one level down.
type ApplicationXuiPanel struct {
	Model
	ApplicationID uint  `gorm:"uniqueIndex:idx_application_xui_panel;not null"`
	PanelID       uint  `gorm:"uniqueIndex:idx_application_xui_panel;not null"`
	PackageID     *uint `gorm:"index"`
}

// ApplicationResourceLocation maps one underlying resource (a WireGuard
// interface, a User Manager group, or an x-ui panel) to an admin-defined
// display Label (e.g. "آلمان - فرانکفورت") shown in the mobile app's
// location picker -- purely cosmetic, never read by any provisioning
// logic. A small dedicated lookup table (matching this codebase's own
// "small join/lookup table per concern" convention, e.g.
// ResellerV2RaySaleTitle) rather than a new column on Interface, since
// User Manager groups have no backing DB row at all to attach a column to
// (RouterOS is the source of truth for group names -- see
// ResellerUserManagerGroup's own doc comment), so a resource-type-keyed
// lookup table is the only shape that works uniformly across all three
// resource kinds.
type ApplicationResourceLocation struct {
	Model
	// ResourceType is one of ResourceTypeWireGuardInterface/
	// ResourceTypeUserManagerGroup/ResourceTypeXuiPanel.
	ResourceType string `gorm:"type:varchar(32);uniqueIndex:idx_app_resource_location;not null"`
	// ResourceKey is the InterfaceID/PanelID (formatted as a decimal
	// string) or the GroupName, depending on ResourceType -- stored as a
	// plain string column (not a typed FK) since it must uniformly key
	// both numeric-ID and string-name resources across all three
	// products, mirroring UsageSnapshot's own polymorphic-but-typed
	// convention (just collapsed to one string column here since there's
	// only ever one lookup value, not three nullable FKs).
	ResourceKey string `gorm:"type:varchar(255);uniqueIndex:idx_app_resource_location;not null"`
	Label       string `gorm:"type:varchar(255);not null"`
}

const (
	ResourceTypeWireGuardInterface = "wireguard_interface"
	ResourceTypeUserManagerGroup   = "user_manager_group"
	ResourceTypeXuiPanel           = "xui_panel"
)

// ApplicationOpenVpnTemplate holds the ONE admin-uploaded .ovpn template
// shared by every Application's OpenVPN connections -- confirmed,
// explicit requirement: the server/port/CA-certificate block is the
// same for everyone (one physical OpenVPN server), only the
// username/password differ, and those already come from each group's
// own UserManagerAccount (see ApplicationService.provisionUserManagerGroup).
// This is a singleton table (at most one row ever exists, always
// fetched with no WHERE clause) rather than reusing SystemConfig's
// key-value table, since SystemConfig.Value is capped at varchar(255)
// and an .ovpn file (embedded CA cert, often several KB) would not fit.
// The file itself lives on disk (mirrors UserManagerConfigFile's own
// on-disk convention) -- this row only tracks that one exists.
type ApplicationOpenVpnTemplate struct {
	Model
	// UploadedAt records when the admin last replaced the template, shown
	// on the settings page so they can tell whether a re-upload is stale.
	UploadedAt time.Time `gorm:"not null"`
}

// AppVersion is one published release of the mobile app -- the admin's
// own explicit "مدیریت اپلیکیشن" requirement: publish a new version, mark
// it mandatory (blocking every older build from being used at all until
// the user updates), and put the whole app into maintenance mode
// (AppMaintenanceMode, a SystemConfig flag -- see that key's own doc
// comment in service/application_version.go) independent of any specific
// version. Every row here is an admin-published release; the mobile app
// itself never writes to this table, only reads the LATEST one (by
// VersionCode) via the public GET /api/app/version-check endpoint.
type AppVersion struct {
	Model
	// VersionCode is a plain increasing integer (matching Android's own
	// versionCode convention) -- the ONLY field ever compared to decide
	// "is the app I'm running on out of date", never VersionName (a
	// human-readable string like "2.4.1" is for display only, never
	// reliably orderable/comparable across arbitrary formatting).
	VersionCode int    `gorm:"not null;uniqueIndex"`
	VersionName string `gorm:"type:varchar(32);not null"`
	// ReleaseNotes is shown to the user alongside the update prompt --
	// optional, admin may simply not write any.
	ReleaseNotes *string `gorm:"type:text"`
	// DownloadURL is where the mobile app sends the user to actually get
	// the new build (e.g. a direct APK link, or a store listing) --
	// deliberately NOT a file upload/hosted-on-this-panel path: an APK is
	// commonly tens of megabytes, and this panel already has no CDN/
	// object-storage layer for large binary assets (see PeerFilesDir's
	// own small-text-file-only usage throughout this codebase). The admin
	// hosts the APK wherever they already distribute it and pastes the
	// link here.
	DownloadURL string `gorm:"type:text;not null"`
	// IsMandatory, when true, means every app instance running a version
	// OLDER than this one is blocked from normal use until it updates --
	// see AppVersionCheckResponse.UpdateRequired's own doc comment for
	// exactly how the mobile app is expected to enforce this (the panel
	// itself has no way to force-close a native app; this is a
	// client-enforced gate, matching AppOnlineCountResponse's own
	// established "server states the fact, app enforces it locally"
	// precedent).
	IsMandatory bool `gorm:"not null;default:false"`
	PublishedAt time.Time `gorm:"not null"`
}

// ApplicationDeviceSession is one (Application, DeviceID) pairing --
// DeviceID is a random UUID the mobile app generates once on first
// launch and persists locally (there is no OS-level stable device
// identifier this app can read without a native dependency this
// codebase doesn't have), sent as a header on every authenticated
// app-JWT request. A confirmed, reported gap this closes: the mobile
// app's own Devices tab previously showed a hardcoded mock list
// (MOCK_DEVICES) since there was no real per-device session record to
// show at all -- app-JWT tokens are stateless with nothing recorded
// server-side per issuance. One row per distinct device that has ever
// logged into this Application; LastSeenAt is touched on every
// authenticated request (see AppAuthController's own JWT middleware
// hook), so "connected"/"active" in the app's UI can be derived as
// "LastSeenAt within the last few minutes" rather than a separate
// explicit online/offline flag this table would otherwise need to keep
// in sync.
type ApplicationDeviceSession struct {
	Model
	ApplicationID uint   `gorm:"uniqueIndex:idx_app_device_session;not null"`
	DeviceID      string `gorm:"type:varchar(64);uniqueIndex:idx_app_device_session;not null"`
	// DeviceLabel is a best-effort display string (e.g. "Android 14") --
	// no native dependency for the real device model/manufacturer exists
	// in this app yet, so this is deliberately coarse rather than absent.
	DeviceLabel string `gorm:"type:varchar(255);not null;default:''"`
	FirstSeenAt time.Time
	LastSeenAt  time.Time
	// ResourceType/ResourceID record which single resource this device
	// last reported connecting with -- one of ResourceTypeWireGuardInterface/
	// ResourceTypeUserManagerGroup/ResourceTypeXuiPanel (same enum
	// ApplicationResourceLocation already uses), and the matching
	// AppConnect*Config.ResourceID value the mobile app already received
	// from GET /me/connect-configs (InterfaceID/AccountID/PanelID
	// depending on ResourceType -- NOT always a PeerID/PackageID, see
	// ApplicationService.GetConnectConfigs). Reported once via
	// POST /api/app/devices/report-resource when the app actually
	// establishes a VPN connection, not on every heartbeat. Empty/nil
	// until the app's first report -- a device that has only ever logged
	// in but never connected has no resource to be "online" on.
	ResourceType string `gorm:"type:varchar(32);not null;default:''"`
	ResourceID   *uint
}
