package model

import "strings"

// UserManagerAccountProtocol is a closed set matching the four RouterOS PPP
// tunnel types this feature manages. Stored as a plain string column (not a
// DB enum, for sqlite/postgres portability) but validated against this set
// at the schema/service layer. This is panel-only metadata: RouterOS User
// Manager itself has no protocol concept -- protocol association in
// RouterOS happens purely via which Group/Profile an account is bound to.
// This field exists so the panel knows which UserManagerProtocolConfig
// row's port to show on the account's share/connection-info page.
type UserManagerAccountProtocol string

const (
	ProtocolL2TP    UserManagerAccountProtocol = "l2tp"
	ProtocolPPTP    UserManagerAccountProtocol = "pptp"
	ProtocolSSTP    UserManagerAccountProtocol = "sstp"
	ProtocolOpenVPN UserManagerAccountProtocol = "openvpn"

	// ProtocolIKEv2 is a display-only tag: choosing it on account creation
	// records the customer's intended protocol for their share/connection-
	// info page (and picks up the IKEv2 row's port from
	// UserManagerProtocolConfig), but does NOT create or configure any
	// IKEv2-specific object on RouterOS. RouterOS IKEv2 is architecturally
	// IPsec + mode-config, not a User Manager PPP tunnel type -- User
	// Manager itself has no concept of IKEv2 at all, so this panel cannot
	// (and does not attempt to) provision a real IKEv2 credential through
	// it. The admin must still separately configure IKEv2 access on the
	// router themselves (Winbox) for this tag to correspond to anything
	// real. This is the same "protocol is panel-only metadata" model this
	// type's own doc comment describes for the other four values, just
	// with an extra caveat since IKEv2 has no RouterOS User Manager
	// counterpart at all (the other four at least map onto a real PPP
	// tunnel type the same account can authenticate over).
	ProtocolIKEv2 UserManagerAccountProtocol = "ikev2"
)

// UserManagerAccount is a RouterOS User Manager account (L2TP/PPTP/SSTP/
// OpenVPN) -- a fully independent VPN-account system from WireGuard's Peer
// model, deliberately not sharing a table or reusing Peer fields, so the
// two account types never get mixed in a listing or a query.
type UserManagerAccount struct {
	Model
	UUID     string `gorm:"type:varchar(36);uniqueIndex;not null"` // for the public share link, mirrors Peer.UUID
	Username string `gorm:"type:varchar(255);uniqueIndex;not null"`
	// Password is stored in plaintext -- the same precedent as
	// Peer.PrivateKey. RouterOS User Manager requires the plaintext
	// password to create the account, and the panel must be able to
	// redisplay it on the admin edit dialog and the public share page.
	Password string `gorm:"type:varchar(255);not null"`

	// RouterOSUserID is the RouterOS-internal `.id` returned by
	// /user-manager/user/add (e.g. "*7"). Needed for every subsequent
	// set/monitor/remove call against this account.
	RouterOSUserID string `gorm:"type:varchar(64);not null"`
	// RouterOSProfileLinkID is the `.id` of the user-profile relation row
	// created by /user-manager/user-profile/add, needed to target
	// activate-user-profile and for cleanup on delete.
	RouterOSProfileLinkID *string `gorm:"type:varchar(64)"`

	Group   string `gorm:"type:varchar(255);not null"` // RouterOS user-manager/user/group name
	Profile string `gorm:"type:varchar(255);not null"` // RouterOS user-manager/profile name

	// Protocols is panel-side metadata only -- RouterOS User Manager itself
	// has no protocol restriction: a single account's username/password
	// already authenticates against L2TP, PPTP, SSTP, and OpenVPN alike
	// (they all check the same User Manager user table), so there is
	// deliberately only ever ONE RouterOS account per UserManagerAccount
	// row, never one-per-protocol. This field exists purely to control what
	// the panel DISPLAYS: which UserManagerProtocolConfig rows' connection
	// info (server/port/certificate) appear on this account's share page,
	// so a customer sold "OpenVPN only" can't see L2TP/PPTP/SSTP details
	// even though the underlying credential would technically also work
	// there. Stored as a comma-separated list of UserManagerAccountProtocol
	// values (see ParseProtocols/JoinProtocols) rather than a join table,
	// mirroring this codebase's plain-string-column-for-enums convention.
	Protocols string `gorm:"type:varchar(64);not null;default:''"`

	Disabled bool    `gorm:"type:boolean;not null;default:false"`
	Comment  *string `gorm:"type:text"`

	// SharedUsers mirrors RouterOS's shared-users= field on user/add --
	// how many simultaneous sessions this single account may have.
	SharedUsers int `gorm:"type:int;not null;default:1"`

	// Optional link to a reseller (owner) of this account, exactly like
	// Peer.ResellerID -- nil means admin-owned.
	ResellerID *uint `gorm:"index"`

	// Quota/usage tracking, mirroring Peer.TrafficLimit/DownloadUsage/UploadUsage.
	TrafficLimit  *int64 `gorm:"type:bigint"` // nil = unlimited, bytes
	DownloadUsage int64  `gorm:"type:bigint;not null;default:0"`
	UploadUsage   int64  `gorm:"type:bigint;not null;default:0"`
	// LastTotalDownload/LastTotalUpload cache RouterOS's own cumulative
	// counters from the last /user-manager/user/monitor poll, mirroring
	// Peer.LastTx/LastRx's role: RouterOS counters can reset (e.g. the
	// account was re-added), so the traffic job needs the previous
	// absolute reading to compute a correct delta rather than assuming
	// monotonic growth.
	LastTotalDownload int64 `gorm:"type:bigint;not null;default:0"`
	LastTotalUpload   int64 `gorm:"type:bigint;not null;default:0"`

	// UsageOffsetDownload/UsageOffsetUpload implement the admin's own
	// "Reset Usage" action for this account -- unlike Peer (a WireGuard
	// counter this panel can zero directly on RouterOS), RouterOS User
	// Manager exposes no confirmed "reset this user's counter" API call,
	// and its /user-manager/user/monitor total is permanently cumulative
	// on the router itself. So DisplayedUsage = RawRouterOSTotal -
	// UsageOffset* (clamped at 0): "reset" just bumps the offset up to the
	// current raw total, making displayed usage read 0 again, without
	// ever touching RouterOS or LastTotalDownload/LastTotalUpload (which
	// stay raw-counter-based specifically so the reseller-level
	// UserManagerQuotaBytes delta pool in cmd/jobs/traffic.go is
	// completely unaffected by a reset -- the admin's own explicit
	// requirement that resetting one account's display must never move
	// reseller/global quota totals). See UserManagerService.ResetUsage.
	UsageOffsetDownload int64 `gorm:"type:bigint;not null;default:0"`
	UsageOffsetUpload   int64 `gorm:"type:bigint;not null;default:0"`

	// SuspendedByQuota/WasActiveBeforeSuspend mirror Peer's quota-suspend
	// bookkeeping exactly, but scoped to this account only -- exhausting a
	// reseller's User Manager quota disables only their UserManagerAccount
	// rows, never the reseller itself or their WireGuard peers (the two
	// quota pools are fully independent).
	SuspendedByQuota       bool `gorm:"type:boolean;not null;default:false"`
	WasActiveBeforeSuspend bool `gorm:"type:boolean;not null;default:false"`

	// SuspendedByTrafficLimit tracks whether THIS account was
	// force-disabled for exceeding its own TrafficLimit
	// (processUserManagerAccountUsage, cmd/jobs/traffic.go) -- mirrors
	// Peer.SuspendedByTrafficLimit's own doc comment exactly, including
	// the confirmed bug it fixes ("ریست حجم کار نمی‌کند") and why this
	// must be a flag distinct from SuspendedByQuota above (that one is
	// the reseller-level pool; conflating the two previously meant this
	// job's own comment ("only an admin manually re-enabling... can undo
	// that") was not actually enforced -- resumeQuotaSuspendedUserManagerAccounts
	// would have incorrectly cleared an account's own-limit suspension too
	// whenever the reseller's overall quota was topped up).
	SuspendedByTrafficLimit bool `gorm:"type:boolean;not null;default:false"`

	ExpireTime *string `gorm:"type:varchar(255)"` // "YYYY-MM-DD", same convention as Peer.ExpireTime

	// Share/connection-info page, mirroring Peer.IsShared/ShareExpireTime.
	IsShared        bool    `gorm:"type:boolean;not null;default:false"`
	ShareExpireTime *string `gorm:"type:varchar(255)"`

	// Notify flags mirroring Peer.FirstNotify/SecondNotify/ThirdNotify, for
	// parity if a future traffic-limit-warning job is added for this
	// resource too.
	FirstNotify  bool `gorm:"type:boolean;not null;default:false;column:first_notify"`
	SecondNotify bool `gorm:"type:boolean;not null;default:false;column:second_notify"`
	ThirdNotify  bool `gorm:"type:boolean;not null;default:false;column:third_notify"`

	// HasConfigFile records whether the admin has uploaded a config file
	// (typically an OpenVPN .ovpn profile) for this account -- RouterOS
	// User Manager has no concept of an OpenVPN client profile to generate
	// one from, so this is a plain admin-supplied file, stored on disk by
	// UserManagerConfigFile (see that file's doc comment), not in this row.
	HasConfigFile bool `gorm:"type:boolean;not null;default:false"`
}

// JoinProtocols/ParseProtocols convert between the []UserManagerAccountProtocol
// the service/schema layer works with and the comma-separated string stored
// in UserManagerAccount.Protocols.
func JoinProtocols(protocols []UserManagerAccountProtocol) string {
	parts := make([]string, len(protocols))
	for i, p := range protocols {
		parts[i] = string(p)
	}
	return strings.Join(parts, ",")
}

func ParseProtocols(stored string) []UserManagerAccountProtocol {
	if stored == "" {
		return nil
	}
	parts := strings.Split(stored, ",")
	protocols := make([]UserManagerAccountProtocol, 0, len(parts))
	for _, p := range parts {
		p = strings.TrimSpace(p)
		if p != "" {
			protocols = append(protocols, UserManagerAccountProtocol(p))
		}
	}
	return protocols
}
