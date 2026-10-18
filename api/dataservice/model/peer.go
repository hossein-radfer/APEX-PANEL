package model

type Peer struct {
	Model
	UUID     string `gorm:"type:varchar(36);uniqueIndex;not null"`
	// ServerID identifies which physical Mikrotik router this peer lives
	// on -- see model.Interface.ServerID's own doc comment for the same
	// reasoning. PeerID and AllowedAddress used to be globally unique on
	// their own, implicitly assuming a single-server install; both are now
	// scoped to (ServerID, value) so the same RouterOS .id or address range
	// can independently exist on two different servers. Interface (below)
	// deliberately stays a plain name, not a foreign key -- see
	// Interface's own field doc comment just below for why.
	ServerID   uint    `gorm:"uniqueIndex:idx_peer_server_peer_id;uniqueIndex:idx_peer_server_allowed_address;not null"`
	PeerID     string  `gorm:"type:varchar(255);uniqueIndex:idx_peer_server_peer_id;not null"`
	Disabled   bool    `gorm:"type:boolean;not null;default:false"`
	Comment    *string `gorm:"type:text"`
	Name       string  `gorm:"type:varchar(255);not null"`
	PrivateKey string  `gorm:"type:varchar(255);not null"`
	PublicKey  string  `gorm:"type:varchar(255);not null"`
	// Interface is the RouterOS interface NAME this peer belongs to, not a
	// foreign key into model.Interface -- matches this codebase's existing
	// convention (config generation and RouterOS calls already address
	// interfaces by name, never by model.Interface.ID), so introducing an
	// InterfaceID FK here would be a larger, out-of-scope redesign. Always
	// resolve the owning Interface row by (ServerID, Interface) together,
	// never by Interface name alone, now that names are only unique per
	// server.
	Interface      string `gorm:"type:varchar(255);not null"`
	AllowedAddress string `gorm:"type:varchar(255);uniqueIndex:idx_peer_server_allowed_address;not null"`
	// Optional link to a reseller (owner) of this peer
	ResellerID          *uint   `gorm:"index"`
	DNSServers          *string `gorm:"type:varchar(255)"`
	Endpoint            string  `gorm:"type:varchar(255);not null"`
	EndpointPort        string  `gorm:"type:varchar(10);not null"`
	PersistentKeepalive string  `gorm:"type:varchar(10)"`
	SchedulerID         *string `gorm:"type:varchar(255)"`
	QueueID             *string `gorm:"type:varchar(255)"`
	ExpireTime          *string `gorm:"type:varchar(255)"`
	TrafficLimit        *int64  `gorm:"type:bigint"`
	TelegramUsername    *string `gorm:"type:varchar(255)"`
	FirstNotify         bool    `gorm:"type:boolean;not null;default:false;column:first_notify"`
	SecondNotify        bool    `gorm:"type:boolean;not null;default:false;column:second_notify"`
	ThirdNotify         bool    `gorm:"type:boolean;not null;default:false;column:third_notify"`
	DownloadBandwidth   *string `gorm:"type:varchar(255)"`
	UploadBandwidth     *string `gorm:"type:varchar(255)"`
	DownloadUsage       int64   `gorm:"type:bigint;not null;default:0"` // in bytes
	UploadUsage         int64   `gorm:"type:bigint;not null;default:0"` // in bytes
	LastTx              int64   `gorm:"type:bigint;not null;default:0"` // in bytes
	LastRx              int64   `gorm:"type:bigint;not null;default:0"` // in bytes
	IsShared            bool    `gorm:"type:boolean;not null;default:false"`
	ShareExpireTime     *string `gorm:"type:varchar(255)"`

	// SuspendedByQuota/WasActiveBeforeSuspend track whether this peer was
	// force-disabled by the reseller-quota job (see cmd/jobs/traffic.go)
	// rather than manually by an admin/reseller -- resumeQuotaSuspendedPeers
	// (service/reseller.go) only ever re-enables peers where both are true,
	// so a peer someone disabled on purpose is never silently turned back
	// on just because the reseller's quota was later topped up. Mirrors
	// UserManagerAccount.SuspendedByQuota/WasActiveBeforeSuspend exactly.
	SuspendedByQuota       bool `gorm:"type:boolean;not null;default:false"`
	WasActiveBeforeSuspend bool `gorm:"type:boolean;not null;default:false"`

	// SuspendedByTrafficLimit tracks whether THIS peer was force-disabled
	// for exceeding its own TrafficLimit (applyPeerTrafficLimit,
	// cmd/jobs/traffic.go) -- a confirmed, reported bug: "ریست حجم کار
	// نمی‌کند" (usage reset doesn't work), root-caused to the reset action
	// never clearing Disabled at all, leaving a peer stuck disabled even
	// after its usage was zeroed. Deliberately a SEPARATE flag from
	// SuspendedByQuota above -- that one is the RESELLER-level overall
	// quota pool (shared across this reseller's every peer/account), a
	// completely different concept from one peer's own individual limit;
	// reusing SuspendedByQuota here would make topping up a reseller's
	// overall quota incorrectly re-enable a peer whose own separate
	// TrafficLimit is still exceeded. ResetPeerUsage/ResetPeerUsages
	// (cmd/jobs/traffic.go) clear this (and re-enable on RouterOS) only
	// when it was THIS flag that caused the disable, never touching a
	// peer an admin disabled manually for an unrelated reason.
	SuspendedByTrafficLimit bool `gorm:"type:boolean;not null;default:false"`
}
