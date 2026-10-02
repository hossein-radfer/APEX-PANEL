package mikrotik

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"net/url"

	"go.uber.org/zap"

	"github.com/maahdima/mwp/api/common"
)

// UserManagerUser mirrors RouterOS's /user-manager/user resource.
//
// KNOWN VERSION-SENSITIVITY RISK: the Group field is sent as "group" (the
// modern RouterOS field name, matching the command the operator confirmed
// working against their own router). Some older RouterOS builds reportedly
// expect "groups" instead. This adaptor deliberately does NOT implement a
// dual-field retry -- that would be speculative code against an unconfirmed
// API surface. If group assignment silently fails on a given router, this
// is the first thing to check.
type UserManagerUser struct {
	ID          string  `json:".id,omitempty"`
	Name        string  `json:"name,omitempty"`
	Password    *string `json:"password,omitempty"`
	SharedUsers *string `json:"shared-users,omitempty"`
	Group       *string `json:"group,omitempty"`
	Disabled    *string `json:"disabled,omitempty"`
	Comment     *string `json:"comment,omitempty"`
}

type UserManagerUserGroup struct {
	Name        string `json:"name,omitempty"`
	DefaultName string `json:"default-name,omitempty"`
}

type UserManagerProfile struct {
	Name string `json:"name,omitempty"`
}

type UserManagerUserProfile struct {
	ID      string `json:".id,omitempty"`
	User    string `json:"user,omitempty"`
	Profile string `json:"profile,omitempty"`
}

// UserManagerMonitorResult models the response shape of
// /user-manager/user/monitor numbers=<id> once="". total-download/
// total-upload arrive as RouterOS's human-readable unit-suffixed strings
// (e.g. "8.6GiB"), NOT raw byte counts -- see utils.ParseRouterOSByteSize.
//
// UNCONFIRMED: this struct shape is inferred from the console command the
// operator tested; the exact REST JSON envelope was not captured. Verify
// against a live response before trusting this blindly in production.
type UserManagerMonitorResult struct {
	TotalDownload string `json:"total-download,omitempty"`
	TotalUpload   string `json:"total-upload,omitempty"`
}

// PPPActiveSession models one row of /ppp/active/print, used to determine
// whether a given User Manager account currently has a live tunnel session.
//
// UNCONFIRMED: matching an account to a session by Name == account.Username
// is a reasonable assumption from RouterOS's documented schema (PPTP/L2TP/
// SSTP/OpenVPN sessions are all PPP-profile-based and surface here), but was
// not explicitly verified by the operator's own testing notes.
type PPPActiveSession struct {
	ID      string `json:".id,omitempty"`
	Name    string `json:"name,omitempty"`
	Service string `json:"service,omitempty"` // "l2tp" | "pptp" | "sstp" | "ovpn"
	Address string `json:"address,omitempty"`
	Uptime  string `json:"uptime,omitempty"`
	// CallerID is the client's own real-world source IP (RouterOS's
	// documented field name for this on /ppp/active/print) -- unlike
	// Address above, which is the INTERNAL tunnel address RouterOS assigned
	// this session, not where the client is actually connecting from. The
	// Security page's IP-tracking feature (see GeoIPService/
	// IPConnectionLog) needs CallerID specifically, matching the sample
	// ppp-online-users.txt the admin provided (caller-id="5.116.132.205").
	CallerID string `json:"caller-id,omitempty"`
}

// CreateUserManagerUser implements confirmed command group (1):
// /user-manager/user/add name=<username> password=<password> shared-users=<count>
func (a *Adaptor) CreateUserManagerUser(c context.Context, user UserManagerUser) (*UserManagerUser, error) {
	var created UserManagerUser

	httpClient := a.mwpClients.GetClient(nil)

	err := httpClient.Put(
		c,
		common.UserManagerUserPath,
		user,
		&created,
	)
	if err != nil {
		a.logger.Error("failed to create user manager user", zap.Error(err))
		return nil, err
	}

	return &created, nil
}

// SetUserManagerUserGroup implements confirmed command group (2):
// /user-manager/user/set numbers=<id> group=<group name>
func (a *Adaptor) SetUserManagerUserGroup(c context.Context, userID string, group string) (*UserManagerUser, error) {
	var updated UserManagerUser

	httpClient := a.mwpClients.GetClient(nil)

	err := httpClient.Patch(
		c,
		common.UserManagerUserPath+"/"+userID,
		UserManagerUser{Group: &group},
		&updated,
	)
	if err != nil {
		a.logger.Error("failed to set user manager user group", zap.Error(err))
		return nil, err
	}

	return &updated, nil
}

// SetUserManagerUserPassword changes an existing account's password on
// RouterOS. Same REST shape as SetUserManagerUserGroup -- a PATCH with only
// the one field being changed.
func (a *Adaptor) SetUserManagerUserPassword(c context.Context, userID string, password string) (*UserManagerUser, error) {
	var updated UserManagerUser

	httpClient := a.mwpClients.GetClient(nil)

	err := httpClient.Patch(
		c,
		common.UserManagerUserPath+"/"+userID,
		UserManagerUser{Password: &password},
		&updated,
	)
	if err != nil {
		a.logger.Error("failed to set user manager user password", zap.Error(err))
		return nil, err
	}

	return &updated, nil
}

// SetUserManagerUserDisabled toggles an account's disabled state, used by
// the panel's own enable/disable/suspend-by-quota actions. Same REST shape
// as SetUserManagerUserGroup.
func (a *Adaptor) SetUserManagerUserDisabled(c context.Context, userID string, disabled string) (*UserManagerUser, error) {
	var updated UserManagerUser

	httpClient := a.mwpClients.GetClient(nil)

	err := httpClient.Patch(
		c,
		common.UserManagerUserPath+"/"+userID,
		UserManagerUser{Disabled: &disabled},
		&updated,
	)
	if err != nil {
		a.logger.Error("failed to set user manager user disabled state", zap.Error(err))
		return nil, err
	}

	return &updated, nil
}

// FetchUserManagerUsers implements the /user-manager/user/print half of
// confirmed command groups relying on a full listing (used to resolve a
// username to its RouterOS .id).
func (a *Adaptor) FetchUserManagerUsers(c context.Context) ([]UserManagerUser, error) {
	var users []UserManagerUser

	httpClient := a.mwpClients.GetClient(nil)

	err := httpClient.Get(
		c,
		common.UserManagerUserPath,
		&users,
	)
	if err != nil {
		a.logger.Error("failed to fetch user manager users", zap.Error(err))
		return nil, err
	}

	return users, nil
}

// FetchUserManagerUserGroups implements confirmed command group (4):
// /user-manager/user/group/print .proplist=name,default-name
func (a *Adaptor) FetchUserManagerUserGroups(c context.Context) ([]UserManagerUserGroup, error) {
	var groups []UserManagerUserGroup

	httpClient := a.mwpClients.GetClient(nil)

	err := httpClient.Get(
		c,
		common.UserManagerUserGroupPath,
		&groups,
	)
	if err != nil {
		a.logger.Error("failed to fetch user manager user groups", zap.Error(err))
		return nil, err
	}

	return groups, nil
}

// FetchUserManagerProfiles implements confirmed command group (5):
// /user-manager/profile/print .proplist=name
func (a *Adaptor) FetchUserManagerProfiles(c context.Context) ([]UserManagerProfile, error) {
	var profiles []UserManagerProfile

	httpClient := a.mwpClients.GetClient(nil)

	err := httpClient.Get(
		c,
		common.UserManagerProfilePath,
		&profiles,
	)
	if err != nil {
		a.logger.Error("failed to fetch user manager profiles", zap.Error(err))
		return nil, err
	}

	return profiles, nil
}

// FetchUserManagerUserProfileLinks lists every user<->profile relation row
// via /user-manager/user-profile/print. Unlike the confirmed command
// groups this adaptor otherwise implements, this specific GET was not
// explicitly tested by the operator against live hardware -- it mirrors
// FetchUserManagerUsers/FetchUserManagerProfiles's same GET-the-collection
// shape, which IS confirmed, so the risk here is narrowly about whether
// this particular resource path supports print the same way. Used by
// UserManagerService.BulkImportAccounts to discover each RouterOS-created
// account's Profile in a single bulk call (Group is already available
// directly on UserManagerUser -- see that struct), instead of one request
// per imported account.
func (a *Adaptor) FetchUserManagerUserProfileLinks(c context.Context) ([]UserManagerUserProfile, error) {
	var links []UserManagerUserProfile

	httpClient := a.mwpClients.GetClient(nil)

	err := httpClient.Get(
		c,
		common.UserManagerUserProfilePath,
		&links,
	)
	if err != nil {
		a.logger.Error("failed to fetch user manager user-profile links", zap.Error(err))
		return nil, err
	}

	return links, nil
}

// CreateUserManagerUserProfile implements the first half of confirmed
// command group (3): /user-manager/user-profile/add user=<username> profile=<profile>
func (a *Adaptor) CreateUserManagerUserProfile(c context.Context, link UserManagerUserProfile) (*UserManagerUserProfile, error) {
	var created UserManagerUserProfile

	httpClient := a.mwpClients.GetClient(nil)

	err := httpClient.Put(
		c,
		common.UserManagerUserProfilePath,
		link,
		&created,
	)
	if err != nil {
		a.logger.Error("failed to create user manager user-profile link", zap.Error(err))
		return nil, err
	}

	// A confirmed, reported bug: some RouterOS firmware/builds' response to
	// this PUT does not echo back a `.id` field, which silently unmarshals
	// to an empty string rather than erroring. That empty ID then gets
	// stored as the account's RouterOSProfileLinkID and, months later, fed
	// straight into DeleteUserManagerUserProfile's URL path -- producing
	// RouterOS's own "missing or invalid resource identifier" 400 on every
	// future delete attempt for that account, since an empty path segment
	// is never a valid resource id. Failing loudly here, at creation time,
	// surfaces the problem immediately (a clear create-time error) instead
	// of silently deferring it to an unrelated delete call much later.
	if created.ID == "" {
		err := fmt.Errorf("router did not return a profile-link id for %q -- the account may need to be created manually or the RouterOS firmware/API may not support this operation as expected", link.Profile)
		a.logger.Error("user manager user-profile link created with an empty id", zap.String("profile", link.Profile))
		return nil, err
	}

	return &created, nil
}

// ActivateUserManagerUserProfile implements the second half of confirmed
// command group (3): /user-manager/user-profile/activate-user-profile numbers=<link id>
//
// This is a RouterOS "run a command" style endpoint, not a plain CRUD verb
// -- implemented via POST (already supported by httphelper.Client, simply
// unused by any other adaptor file so far).
func (a *Adaptor) ActivateUserManagerUserProfile(c context.Context, linkID string) error {
	httpClient := a.mwpClients.GetClient(nil)

	err := httpClient.Post(
		c,
		common.UserManagerUserProfilePath+"/activate-user-profile",
		map[string]string{"numbers": linkID},
		nil,
	)
	if err != nil {
		a.logger.Error("failed to activate user manager user-profile", zap.Error(err))
		return err
	}

	return nil
}

// MonitorUserManagerUser implements confirmed command group (6):
// /user-manager/user/monitor numbers=<id> once=""
//
// Another "run a command" style endpoint (POST, same reasoning as
// ActivateUserManagerUserProfile above).
func (a *Adaptor) MonitorUserManagerUser(c context.Context, userID string) (*UserManagerMonitorResult, error) {
	httpClient := a.mwpClients.GetClient(nil)

	// The REST envelope for /user-manager/user/monitor was never confirmed
	// against a live response (see the UNCONFIRMED note on
	// UserManagerMonitorResult above) -- unmarshal into a raw json.RawMessage
	// first so a shape mismatch (e.g. RouterOS returning a single-element
	// array like /interface/monitor-traffic does, instead of a flat object)
	// is logged with the ACTUAL bytes RouterOS sent, rather than silently
	// producing a zero-valued struct that looks like "genuinely zero usage".
	var raw json.RawMessage

	err := httpClient.Post(
		c,
		common.UserManagerUserPath+"/monitor",
		map[string]string{"numbers": userID, "once": ""},
		&raw,
	)
	if err != nil {
		a.logger.Error("failed to monitor user manager user",
			zap.String("routeros_user_id", userID),
			zap.Error(err))
		return nil, err
	}

	a.logger.Debug("user manager monitor raw response",
		zap.String("routeros_user_id", userID),
		zap.ByteString("raw_body", raw))

	result, err := parseUserManagerMonitorResult(raw)
	if err != nil {
		a.logger.Error("failed to parse user manager monitor response -- see raw_body for the actual shape RouterOS returned",
			zap.String("routeros_user_id", userID),
			zap.ByteString("raw_body", raw),
			zap.Error(err))
		return nil, err
	}

	return result, nil
}

// parseUserManagerMonitorResult accepts either of the two shapes RouterOS's
// REST layer is known to use for "monitor"-style endpoints: a flat object
// (as the console command implies), or a single-element array (as several
// other RouterOS *-monitor REST endpoints, e.g. /interface/monitor-traffic,
// are documented to return). Trying both here -- rather than picking one
// and letting the other silently zero out -- is what makes a genuine shape
// mismatch show up as a loud parse error in the logs instead of a quiet
// "0 bytes used" that's indistinguishable from real zero usage.
func parseUserManagerMonitorResult(raw json.RawMessage) (*UserManagerMonitorResult, error) {
	trimmed := bytes.TrimSpace(raw)
	if len(trimmed) == 0 {
		return nil, fmt.Errorf("empty response body")
	}

	if trimmed[0] == '[' {
		var results []UserManagerMonitorResult
		if err := json.Unmarshal(trimmed, &results); err != nil {
			return nil, fmt.Errorf("failed to unmarshal array-shaped response: %w", err)
		}
		if len(results) == 0 {
			return nil, fmt.Errorf("monitor returned an empty array -- the RouterOS .id used (numbers=) likely does not match any user-manager user")
		}
		return &results[0], nil
	}

	var result UserManagerMonitorResult
	if err := json.Unmarshal(trimmed, &result); err != nil {
		return nil, fmt.Errorf("failed to unmarshal object-shaped response: %w", err)
	}
	return &result, nil
}

// DeleteUserManagerUserProfile implements confirmed command group (7)'s
// profile-relation cleanup step.
//
// CONFIRMED FIX (live-hardware tested): the console command's
// "?user=<user id>" query-filter syntax does NOT translate to the REST
// API -- RouterOS's REST layer rejected it with
// {"error":400,"message":"Bad Request","detail":"missing or invalid
// resource identifier"}. The REST API instead needs the user-profile
// RELATION's own ".id" (returned by CreateUserManagerUserProfile and
// stored as UserManagerAccount.RouterOSProfileLinkID), addressed as a
// path segment, matching every other RouterOS REST delete in this
// codebase (see DeleteUserManagerUser below).
func (a *Adaptor) DeleteUserManagerUserProfile(c context.Context, profileLinkID string) error {
	httpClient := a.mwpClients.GetClient(nil)

	err := httpClient.Delete(
		c,
		common.UserManagerUserProfilePath+"/"+url.PathEscape(profileLinkID),
		nil,
	)
	if err != nil {
		a.logger.Error("failed to delete user manager user-profile link", zap.Error(err))
		return err
	}

	return nil
}

// DeleteUserManagerUser implements the second half of confirmed command
// group (7): /user-manager/user/remove numbers=<user id>
//
// Must be called AFTER DeleteUserManagerUserProfile, matching the
// operator-confirmed order exactly (profile relation first, then the user
// itself).
func (a *Adaptor) DeleteUserManagerUser(c context.Context, userID string) error {
	httpClient := a.mwpClients.GetClient(nil)

	err := httpClient.Delete(
		c,
		common.UserManagerUserPath+"/"+userID,
		nil,
	)
	if err != nil {
		a.logger.Error("failed to delete user manager user", zap.Error(err))
		return err
	}

	return nil
}

// FetchPPPActiveSessions implements the /ppp/active/print half of the
// confirmed usage-check sequence, used to determine online status.
func (a *Adaptor) FetchPPPActiveSessions(c context.Context) ([]PPPActiveSession, error) {
	var sessions []PPPActiveSession

	httpClient := a.mwpClients.GetClient(nil)

	err := httpClient.Get(
		c,
		common.PPPActivePath,
		&sessions,
	)
	if err != nil {
		a.logger.Error("failed to fetch ppp active sessions", zap.Error(err))
		return nil, err
	}

	return sessions, nil
}
