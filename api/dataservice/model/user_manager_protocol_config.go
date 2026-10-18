package model

// UserManagerProtocolConfig is an admin-only settings row -- exactly one per
// (protocol, server) pair, enforced by a composite unique index. This table
// is purely descriptive: the panel NEVER reads or writes these values to
// RouterOS's actual /interface l2tp-server server (and the pptp/sstp/ovpn
// equivalents) -- the admin turns each protocol on and configures its real
// port in Winbox themselves. This table only records what the admin says
// they configured, so the panel can build correct share/connection-info
// links for accounts of that protocol.
//
// ServerID exists because the same protocol can be configured differently
// (a different port, a different public address) on each of the admin's
// MikroTik servers -- a confirmed, reported bug: before this column
// existed, this table had exactly one row per protocol system-wide, so
// whichever server's settings were entered last silently became "the"
// port/address shown to every customer, including customers whose account
// actually lives on a different server with a different real port/address.
type UserManagerProtocolConfig struct {
	Model
	ServerID uint                       `gorm:"not null;uniqueIndex:idx_user_manager_protocol_configs_server_protocol"`
	Protocol UserManagerAccountProtocol `gorm:"type:varchar(16);not null;uniqueIndex:idx_user_manager_protocol_configs_server_protocol"`

	// Port is the admin-entered port that must match what was actually
	// configured on the router for this protocol's tunnel service. Never
	// synced from RouterOS -- purely descriptive, for share-link generation.
	Port int `gorm:"type:int;not null"`

	// ServerAddress is the public hostname/IP end-users should connect to
	// for this protocol. Optional: when unset, the panel falls back to the
	// existing Server list's address (the same one WireGuard peer endpoints
	// already use).
	ServerAddress *string `gorm:"type:varchar(255)"`

	// CertificateName is relevant for TLS-based protocols (SSTP, and
	// optionally OpenVPN) -- records which RouterOS certificate name the
	// admin bound to the relevant *-server's certificate= setting in
	// Winbox, purely for display/reference on the admin settings page.
	CertificateName *string `gorm:"type:varchar(255)"`

	// Enabled is the admin's own record of "I turned this protocol's
	// server on in Winbox" -- purely informational, drives a UI badge,
	// never sent to RouterOS.
	Enabled bool `gorm:"type:boolean;not null;default:true"`

	// Notes is shown to end users on the share/connection-info page for
	// every account of this protocol (e.g. "only UDP is open on this
	// port," "use TCP mode in your client") -- NOT admin-only, despite
	// living in an otherwise admin-only settings table. Optional; omitted
	// from the share page entirely when unset.
	Notes *string `gorm:"type:text"`

	// HasCertificateFile/HasClientAppFile record whether the admin has
	// uploaded a reference/quality file and a client app installer for
	// this protocol on this server -- stored on disk by
	// UserManagerProtocolFile (mirrors UserManagerAccount.HasConfigFile's
	// on-disk-file-plus-boolean-flag convention). Both are shown as
	// downloads on every share page for an account of this protocol on
	// this server, not per-account -- one file serves every customer on
	// that protocol+server pair.
	HasCertificateFile bool `gorm:"type:boolean;not null;default:false"`
	HasClientAppFile   bool `gorm:"type:boolean;not null;default:false"`
}
