package model

import "time"

// DNSPanel is an admin-registered doctor-dns installation (one relay-inside-
// Iran + one exit-outside-Iran pair, tunnelled together and administered as
// a single unit -- see the doctor-dns project's own architecture) that Apex
// provisions Smart DNS accounts on via that installation's smartdns-panel
// API (the /apex/* routes added to it for this integration). Mirrors
// XuiPanel's role for V2Ray exactly, with one deliberate difference: a
// DNSAccount is NEVER fanned out across multiple DNSPanel rows the way a
// V2RayPackage fans out across every registered XuiPanel -- each doctor-dns
// installation is its own independent SQLite-backed service with no shared
// state across installations, so a customer's one DNS purchase lives on
// exactly ONE DNSPanel (picked at purchase time), not all of them. Multiple
// DNSPanel rows exist to support multiple independent doctor-dns
// installations (e.g. one per exit country), with several resellers
// commonly sharing access to the SAME one -- see ResellerDNSPanelAccess.
type DNSPanel struct {
	Model
	Name      string `gorm:"type:varchar(255);not null;uniqueIndex"` // internal label, e.g. "IR-Exit-01"
	SaleTitle string `gorm:"type:varchar(255);not null"`             // default customer-facing title, overridable per reseller later if needed (mirrors XuiPanel.SaleTitle)

	// APIBaseURL points at the doctor-dns exit node's smartdns-panel HTTPS
	// port (e.g. https://exit.example.com:8443) -- the same process and
	// port the relay(s) sync to, just a different path prefix (/apex/*
	// instead of /sync).
	APIBaseURL string `gorm:"type:varchar(255);not null"`

	// APIKey is the APEX_API_KEY bearer secret configured in that
	// installation's /etc/smart-dns/panel.env -- stored in plaintext, same
	// precedent as XuiPanel.Password/Server.Password/Peer.PrivateKey. No
	// at-rest encryption exists anywhere in this codebase; this column
	// deliberately doesn't introduce the first one.
	APIKey string `gorm:"type:varchar(255);not null"`

	// Status/LastError/LastSyncedAt are written by the background health/
	// sync job, never by the admin CRUD handlers -- mirrors XuiPanel's
	// identical fields exactly.
	Status       string  `gorm:"type:varchar(16);not null;default:'active'"` // "active" | "error"
	LastError    *string `gorm:"type:text"`
	LastSyncedAt *time.Time
}
