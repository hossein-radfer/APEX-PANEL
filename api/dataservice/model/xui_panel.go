package model

import "time"

// XuiPanel is an admin-registered x-ui (alireza0) server MWPanel provisions
// V2Ray clients on. Unlike Server (the single Mikrotik connection), there
// can be many XuiPanel rows -- every registered panel participates in every
// V2RayPackage's fan-out (see V2RayPackageLocation), so a customer's one
// package becomes one client on every panel simultaneously.
type XuiPanel struct {
	Model
	Name       string `gorm:"type:varchar(255);not null;uniqueIndex"` // internal label, e.g. "DXB-01"
	SaleTitle  string `gorm:"type:varchar(255);not null"`             // default customer-facing title, overridable per reseller (see ResellerV2RaySaleTitle)
	APIBaseURL string `gorm:"type:varchar(255);not null"`
	Username   string `gorm:"type:varchar(128);not null"`
	// Password is stored in plaintext -- same precedent as Server.Password/
	// Peer.PrivateKey/UserManagerAccount.Password. No at-rest encryption
	// exists anywhere in this codebase; this column deliberately doesn't
	// introduce the first one.
	Password         string `gorm:"type:varchar(255);not null"`
	DefaultInboundID int    `gorm:"type:int;not null"`
	// Protocol is informational only (mirrors the selected inbound's own
	// protocol) -- shown on the admin panel list, never sent to x-ui.
	Protocol   string `gorm:"type:varchar(32);not null"`
	SubBaseURL string `gorm:"type:varchar(255);not null"`

	// Status/LastError/LastSyncedAt are written by the sync job
	// (service.SyncPackageUsage), never by the admin CRUD handlers --
	// they reflect the panel's live reachability, distinct from the admin's
	// own "Test Connection" button which checks synchronously on demand.
	Status       string  `gorm:"type:varchar(16);not null;default:'active'"` // "active" | "error"
	LastError    *string `gorm:"type:text"`
	LastSyncedAt *time.Time

	// ContainerServerID/ContainerName (item 9): alireza0's x-ui panels are
	// commonly hosted INSIDE a RouterOS 7 native Docker container
	// (confirmed with the admin -- "پنل های v2ray که علیرضا هستن برروی
	// میکروتیک هستند که با استفاده از کانتینر ایجاد شدن"), so a container
	// stopping on the router is the single most common cause of a panel
	// going unreachable. Both nil (the previous, and still fully
	// supported, default) means "this panel isn't known to run inside a
	// RouterOS container" -- V2RayPanelHealthService falls back to its
	// original x-ui-login-only check with no container diagnosis for such
	// a panel, exactly as it always has. Deliberately TWO separate
	// nullable fields rather than one combined "location" string: the
	// mapping needs a real FK into model.Server (to resolve which
	// router's REST API/credentials to query) plus RouterOS's own
	// container Name on that specific router, and gorm can enforce
	// referential integrity on the former but not on an embedded string.
	ContainerServerID *uint   `gorm:"index"`
	ContainerName     *string `gorm:"type:varchar(128)"`
}
