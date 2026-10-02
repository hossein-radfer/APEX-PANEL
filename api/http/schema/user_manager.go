package schema

type UserManagerAccountStatus string

var (
	ActiveUserManagerAccount    UserManagerAccountStatus = "active"
	InactiveUserManagerAccount  UserManagerAccountStatus = "inactive"
	ExpiredUserManagerAccount   UserManagerAccountStatus = "expired"
	SuspendedUserManagerAccount UserManagerAccountStatus = "suspended"
)

// Protocols is a multi-select: one RouterOS User Manager account is created
// regardless of how many protocols are chosen (RouterOS itself doesn't
// restrict a user to one tunnel type -- see model.UserManagerAccount's doc
// comment), this list only controls which protocols' connection info the
// panel reveals to the customer on their share page.
// BulkCreateUserManagerAccountRequest creates Count accounts sharing the
// same group/profile/protocol/traffic-limit/duration selection, each with
// its own freshly-generated random username+password -- mirrors
// BulkCreateV2RayPackageRequest/BulkCreatePeerRequest's exact shape,
// adapted for User Manager's credential-based (rather than keypair-based)
// per-account identity. DurationDays is converted to an ExpireTime date
// (today+DurationDays) once per batch, matching CreatePeerRequest.
// ExpireTime's own "fixed date, not a duration" convention on the
// single-create path.
type BulkCreateUserManagerAccountRequest struct {
	Count        int      `json:"count" validate:"required,min=1,max=500"`
	Group        string   `json:"group" validate:"required"`
	Profile      string   `json:"profile" validate:"required"`
	Protocols    []string `json:"protocols" validate:"required,min=1,dive,oneof=l2tp pptp sstp openvpn ikev2"`
	SharedUsers  *int     `json:"shared_users,omitempty"`
	TrafficLimit *string  `json:"traffic_limit,omitempty"`
	DurationDays int      `json:"duration_days" validate:"required,min=1"`
}

// BulkCreatedUserManagerAccount pairs a created account's normal response
// shape with its plaintext Password -- UserManagerAccountResponse itself
// deliberately never carries a password (see its own fields), but a
// bulk-created account's randomly-generated password is only ever known in
// plaintext at this exact moment, so it must ride along in this one
// response or be permanently lost (there being no separate "show me the
// password I just generated" endpoint, by design -- ChangeAccountPassword
// only ever sets a NEW one).
type BulkCreatedUserManagerAccount struct {
	UserManagerAccountResponse
	Password string `json:"password"`
}

type BulkCreateUserManagerAccountResponse struct {
	Accounts []BulkCreatedUserManagerAccount `json:"accounts"`
}

type CreateUserManagerAccountRequest struct {
	Username     string   `json:"username" validate:"required"`
	Password     string   `json:"password" validate:"required"`
	Group        string   `json:"group" validate:"required"`
	Profile      string   `json:"profile" validate:"required"`
	Protocols    []string `json:"protocols" validate:"required,min=1,dive,oneof=l2tp pptp sstp openvpn ikev2"`
	SharedUsers  *int     `json:"shared_users,omitempty"`
	Comment      *string  `json:"comment,omitempty"`
	TrafficLimit *string  `json:"traffic_limit,omitempty"` // GB, same convention as CreatePeerRequest
	ExpireTime   *string  `json:"expire_time,omitempty"`
}

type UpdateUserManagerAccountRequest struct {
	Disabled     *bool    `json:"disabled,omitempty"`
	Group        string   `json:"group,omitempty"`
	Profile      string   `json:"profile,omitempty"`
	Protocols    []string `json:"protocols,omitempty" validate:"omitempty,min=1,dive,oneof=l2tp pptp sstp openvpn ikev2"`
	Comment      *string  `json:"comment,omitempty"`
	SharedUsers  *int     `json:"shared_users,omitempty"`
	TrafficLimit *string  `json:"traffic_limit,omitempty"`
	ExpireTime   *string  `json:"expire_time,omitempty"`
}

// ChangeUserManagerAccountPasswordRequest is submitted from the dedicated
// "Change Password" dialog (mirrors the Share dialog's separate-action
// pattern rather than being folded into UpdateUserManagerAccountRequest).
type ChangeUserManagerAccountPasswordRequest struct {
	Password string `json:"password" validate:"required,min=1"`
}

type UpdateUserManagerAccountShareExpireRequest struct {
	ExpireTime *string `json:"expire_time"`
}

// BulkImportAccountsResult summarizes the outcome of a bulk-import run --
// see UserManagerService.BulkImportAccounts's doc comment for the full
// sync algorithm this reports on.
type BulkImportAccountsResult struct {
	Imported         int      `json:"imported"`          // synced into the panel's own database
	Skipped          int      `json:"skipped"`           // username not found on RouterOS
	AlreadyImported  int      `json:"already_imported"`  // username already exists in the panel's database
	Malformed        []string `json:"malformed"`         // raw lines that didn't match the expected format
	SkippedUsernames []string `json:"skipped_usernames"` // usernames not found on RouterOS, for the admin to double check
}

type UserManagerAccountShareStatusResponse struct {
	IsShared   bool    `json:"is_shared"`
	UUID       *string `json:"uuid"`
	ExpireTime *string `json:"expire_time"`
}

type UserManagerAccountResponse struct {
	Id            uint                       `json:"id"`
	UUID          string                     `json:"uuid"`
	Username      string                     `json:"username"`
	Group         string                     `json:"group"`
	Profile       string                     `json:"profile"`
	Protocols     []string                   `json:"protocols"`
	Disabled      bool                       `json:"disabled"`
	Comment       *string                    `json:"comment"`
	SharedUsers   int                        `json:"shared_users"`
	TrafficLimit  *string                    `json:"traffic_limit"`
	ExpireTime    *string                    `json:"expire_time"`
	TotalUsage    string                     `json:"total_usage"`
	Status        []UserManagerAccountStatus `json:"status"`
	IsOnline      bool                       `json:"is_online"`
	IsShared      bool                       `json:"is_shared"`
	HasConfigFile bool                       `json:"has_config_file"`
	ResellerID    *uint                      `json:"reseller_id,omitempty"`
	ResellerName  *string                    `json:"reseller_name,omitempty"`
}

type UserManagerGroupResponse struct {
	Name string `json:"name"`
}

type UserManagerProfileResponse struct {
	Name string `json:"name"`
}

type UpsertUserManagerProtocolConfigRequest struct {
	Protocol        string  `json:"protocol" validate:"required,oneof=l2tp pptp sstp openvpn ikev2"`
	Port            int     `json:"port" validate:"required,min=1,max=65535"`
	ServerAddress   *string `json:"server_address,omitempty"`
	CertificateName *string `json:"certificate_name,omitempty"`
	Enabled         bool    `json:"enabled"`
	Notes           *string `json:"notes,omitempty"`
}

type UserManagerProtocolConfigResponse struct {
	Id                 uint    `json:"id"`
	Protocol           string  `json:"protocol"`
	Port               int     `json:"port"`
	ServerAddress      *string `json:"server_address"`
	CertificateName    *string `json:"certificate_name"`
	Enabled            bool    `json:"enabled"`
	Notes              *string `json:"notes"`
	HasCertificateFile bool    `json:"has_certificate_file"`
	HasClientAppFile   bool    `json:"has_client_app_file"`
}

// UserManagerShareProtocolInfo is one protocol's connection info block on
// the share page -- a customer only ever sees the protocols their account
// was granted (UserManagerAccount.Protocols), never protocols they weren't
// sold, even though the same username/password would technically also work
// there (see model.UserManagerAccount's doc comment on why RouterOS itself
// can't enforce this restriction).
type UserManagerShareProtocolInfo struct {
	Protocol           string  `json:"protocol"`
	ServerAddress      string  `json:"server_address"`
	Port               int     `json:"port"`
	HasCertificateFile bool    `json:"has_certificate_file"`
	HasClientAppFile   bool    `json:"has_client_app_file"`
	Notes              *string `json:"notes,omitempty"`
}

// UserManagerAccountShareDetailsResponse is the public, unauthenticated
// connection-info payload shown on the share page -- username/password
// shared across protocols, plus one connection-info block per protocol the
// account was granted. No config file at the top level, no QR code (per the
// "simple HTML info page" decision).
type UserManagerAccountShareDetailsResponse struct {
	Username      string                         `json:"username"`
	Password      string                         `json:"password"`
	Protocols     []UserManagerShareProtocolInfo `json:"protocols"`
	ExpireTime    *string                        `json:"expire_time"`
	HasConfigFile bool                           `json:"has_config_file"`

	// Usage/quota, mirroring PeerDetailsResponse's shape so the share page
	// can show the same "used / limit (percent%)" info WireGuard peers get.
	TrafficLimit  *string `json:"traffic_limit"`  // GB, nil = unlimited
	DownloadUsage string  `json:"download_usage"` // GB
	UploadUsage   string  `json:"upload_usage"`   // GB
	TotalUsage    string  `json:"total_usage"`    // GB
	UsagePercent  *string `json:"usage_percent"`  // nil when unlimited
	IsOnline      bool    `json:"is_online"`
}

// UserManagerSelfSummaryResponse is the reseller-dashboard aggregate --
// mirrors SelfActivityResponse's shape (peer.go) for WireGuard, but scoped
// to a reseller's own User Manager accounts. Deliberately does not require
// canCreateUserManagerAccounts to be true to view (a reseller who used to
// have UM access but was revoked should still see their existing, now
// frozen, usage rather than the section vanishing).
type UserManagerSelfSummaryResponse struct {
	OnlineAccounts int    `json:"online_accounts"`
	TotalAccounts  int    `json:"total_accounts"`
	QuotaBytes     *int64 `json:"quota_bytes"` // nil = unlimited
	UsedBytes      int64  `json:"used_bytes"`
	RemainingBytes *int64 `json:"remaining_bytes"` // nil = unlimited
	MaxAccounts    *int   `json:"max_accounts"`    // nil = unlimited
}
