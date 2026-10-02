package xui

import (
	"context"
	"encoding/json"
	"fmt"
	"strings"

	"github.com/maahdima/mwp/api/common"
	"github.com/maahdima/mwp/api/dataservice/model"
)

// XuiInbound mirrors GET /xui/API/inbounds/'s per-row shape, trimmed to the
// fields this adaptor actually needs (the admin's "pick an inbound"
// dropdown on panel registration).
type XuiInbound struct {
	ID       int    `json:"id"`
	Remark   string `json:"remark"`
	Protocol string `json:"protocol"`
	Enable   bool   `json:"enable"`
}

type xuiInboundsResponse struct {
	Success bool         `json:"success"`
	Msg     string       `json:"msg"`
	Obj     []XuiInbound `json:"obj"`
}

// xuiInboundDetail mirrors GET /xui/API/inbounds/get/:id's obj payload,
// trimmed to what's needed to resolve the correct client `flow` value and
// to read back an existing client's CURRENT flow (for the one-time repair
// pass, see V2RaySyncService.repairClientFlowIfNeeded) -- both
// StreamSettings and Settings arrive as JSON strings (x-ui's same "nested
// JSON as a string" convention), parsed in a second pass rather than
// typed fields here.
type xuiInboundDetail struct {
	StreamSettings string `json:"streamSettings"`
	Settings       string `json:"settings"`
}

type xuiInboundDetailResponse struct {
	Success bool             `json:"success"`
	Msg     string           `json:"msg"`
	Obj     xuiInboundDetail `json:"obj"`
}

// xuiStreamSettings is the minimal shape of a decoded streamSettings JSON
// string -- just the one field that determines whether `flow` is valid.
type xuiStreamSettings struct {
	Security string `json:"security"`
}

// XuiClient mirrors the "clients" entry nested inside an inbound's
// settings= JSON string, matching the shape documented by the operator's
// own tested addClient payload. flow/tgId/subId are VLESS-shaped; Trojan/
// Shadowsocks accounts (which key off password/email instead of id) are
// out of scope for this adaptor -- see V2RayPackageLocation's doc comment.
type XuiClient struct {
	ID         string `json:"id"`
	Flow       string `json:"flow"`
	Email      string `json:"email"`
	LimitIP    int    `json:"limitIp"`
	TotalGB    int64  `json:"totalGB"`    // bytes, NOT GB despite the field name -- see BytesFromGB
	ExpiryTime int64  `json:"expiryTime"` // epoch milliseconds
	Enable     bool   `json:"enable"`
	TgID       string `json:"tgId"`
	SubID      string `json:"subId"`
}

type xuiClientSettings struct {
	Clients []XuiClient `json:"clients"`
}

type addClientRequest struct {
	ID       int    `json:"id"`
	Settings string `json:"settings"`
}

type xuiActionResponse struct {
	Success bool   `json:"success"`
	Msg     string `json:"msg"`
}

// xuiTrafficStat mirrors GET .../getClientTraffics/:email's obj payload.
type xuiTrafficStat struct {
	Up    int64 `json:"up"`
	Down  int64 `json:"down"`
	Total int64 `json:"total"` // configured totalGB limit, not used-so-far
}

type xuiTrafficResponse struct {
	Success bool           `json:"success"`
	Msg     string         `json:"msg"`
	Obj     xuiTrafficStat `json:"obj"`
}

// xuiOnlinesResponse mirrors POST /xui/API/inbounds/onlines's envelope --
// obj's own shape is fork-dependent (see xuiOnlinesObjShapes' doc comment
// below), so it's captured here as raw JSON and parsed separately rather
// than a fixed typed field.
type xuiOnlinesResponse struct {
	Success bool            `json:"success"`
	Msg     string          `json:"msg"`
	Obj     json.RawMessage `json:"obj"`
}

// xuiOnlinesClientInfo is alireza0/x-ui's own richer per-client shape for
// this endpoint (confirmed against alireza0/x-ui's xray/api.go
// OnlineUserInfo struct) -- IPs is unused here (this codebase only needs
// the email to match against ClientEmail), kept only so json.Unmarshal
// doesn't need a second, narrower struct.
type xuiOnlinesClientInfo struct {
	Email string `json:"email"`
}

// parseXuiOnlinesObj handles BOTH confirmed real-world shapes this
// endpoint returns, which differ by x-ui fork -- a live, reported bug: a
// customer's second registered panel failed every online-status poll with
// "cannot unmarshal object into Go struct field ... of type string",
// because that panel runs a fork (alireza0/x-ui) whose /onlines endpoint
// returns `"obj": [{"email": "...", "ips": {...}}, ...]` (an array of
// per-client objects carrying connection IPs, sourced live from Xray-
// core's own gRPC stats service), rather than MHSanaei/3x-ui's simpler
// `"obj": ["email1", "email2", ...]` (a flat array of email strings) this
// adaptor originally only supported. Both shapes are tried in order (flat
// strings first, since it's the more common/simpler case); an obj that
// matches neither (e.g. genuinely malformed) returns an error, same as
// before.
func parseXuiOnlinesObj(raw json.RawMessage) ([]string, error) {
	if len(raw) == 0 || string(raw) == "null" {
		return nil, nil
	}

	var emails []string
	if err := json.Unmarshal(raw, &emails); err == nil {
		return emails, nil
	}

	var clients []xuiOnlinesClientInfo
	if err := json.Unmarshal(raw, &clients); err != nil {
		return nil, fmt.Errorf("obj matched neither the flat-email-array nor the per-client-object-array shape: %w", err)
	}

	emails = make([]string, 0, len(clients))
	for _, c := range clients {
		if c.Email != "" {
			emails = append(emails, c.Email)
		}
	}
	return emails, nil
}

// BytesFromGB converts a whole-GB size into the raw byte count x-ui's
// totalGB field actually expects (despite its name, it is bytes, not GB --
// e.g. 20GB must be sent as 21474836480). Named explicitly because this
// unit mismatch is easy to get backwards by accident.
func BytesFromGB(gb int64) int64 {
	return gb * 1024 * 1024 * 1024
}

// EpochMillis converts a Unix seconds timestamp into the epoch-millisecond
// value x-ui's expiryTime field expects.
func EpochMillis(unixSeconds int64) int64 {
	return unixSeconds * 1000
}

// xtlsRprxVisionFlow is the one flow value this codebase ever sets --
// valid ONLY when the inbound's own streamSettings.security is "tls" or
// "reality" (VLESS XTLS flow control requires the TLS/Reality handshake
// underneath it). Confirmed as a live, reproduced bug: hardcoding this
// value regardless of the inbound's actual security setting silently
// created clients that would never connect on any security:"none"
// inbound -- the client exists in x-ui and looks identical to a working
// one except for this one field.
const xtlsRprxVisionFlow = "xtls-rprx-vision"

// ResolveClientFlow fetches inboundID's own streamSettings and returns the
// correct `flow` value for a client on it: xtlsRprxVisionFlow if security
// is "tls" or "reality", or the empty string for anything else (including
// "none", "" unset, or an unrecognized value -- flow must never be set on
// a non-TLS/Reality inbound, so the safe default on any ambiguity is
// empty, matching how a client created manually through the x-ui web UI
// on such an inbound has flow=""). Called fresh before every AddClient/
// UpdateClient -- never cached -- since an admin can change an inbound's
// security setting in x-ui's own UI at any time, independent of this
// panel.
func ResolveClientFlow(ctx context.Context, panel model.XuiPanel, inboundID int) (string, error) {
	client, err := Login(ctx, panel)
	if err != nil {
		return "", err
	}

	var resp xuiInboundDetailResponse
	path := fmt.Sprintf(common.XuiInboundGetPathFmt, inboundID)
	if err := client.doJSON(ctx, "GET", path, nil, &resp); err != nil {
		return "", err
	}
	if !resp.Success {
		return "", fmt.Errorf("x-ui rejected inbound detail request: %s", resp.Msg)
	}

	var streamSettings xuiStreamSettings
	if err := json.Unmarshal([]byte(resp.Obj.StreamSettings), &streamSettings); err != nil {
		return "", fmt.Errorf("parsing inbound streamSettings: %w", err)
	}

	security := strings.ToLower(streamSettings.Security)
	if security == "tls" || security == "reality" {
		return xtlsRprxVisionFlow, nil
	}
	return "", nil
}

// GetInboundClientFlow reads inboundID's current client list and returns
// the CURRENT, live flow value x-ui has stored for clientEmail -- used
// only by the one-time-per-location repair pass (see
// V2RaySyncService.repairClientFlowIfNeeded) to detect whether a location
// created before the flow fix actually has the wrong value, without
// blindly re-issuing an UpdateClient for every location regardless of
// whether it needs it. Returns (flow, found, error) -- found=false means
// no client with that email exists on this inbound (e.g. it was deleted
// on the x-ui side directly), which the caller should treat as "nothing
// to repair," not an error.
func GetInboundClientFlow(ctx context.Context, panel model.XuiPanel, inboundID int, clientEmail string) (flow string, found bool, err error) {
	client, err := Login(ctx, panel)
	if err != nil {
		return "", false, err
	}

	var resp xuiInboundDetailResponse
	path := fmt.Sprintf(common.XuiInboundGetPathFmt, inboundID)
	if err := client.doJSON(ctx, "GET", path, nil, &resp); err != nil {
		return "", false, err
	}
	if !resp.Success {
		return "", false, fmt.Errorf("x-ui rejected inbound detail request: %s", resp.Msg)
	}

	var settings xuiClientSettings
	if err := json.Unmarshal([]byte(resp.Obj.Settings), &settings); err != nil {
		return "", false, fmt.Errorf("parsing inbound settings: %w", err)
	}

	for _, c := range settings.Clients {
		if c.Email == clientEmail {
			return c.Flow, true, nil
		}
	}
	return "", false, nil
}

// TestConnection logs in and fetches the inbound list in one call --
// used by the admin's "Test Connection" button, which also populates the
// inbound-id dropdown from the same round-trip.
func TestConnection(ctx context.Context, panel model.XuiPanel) ([]XuiInbound, error) {
	return ListInbounds(ctx, panel)
}

func ListInbounds(ctx context.Context, panel model.XuiPanel) ([]XuiInbound, error) {
	client, err := Login(ctx, panel)
	if err != nil {
		return nil, err
	}

	var resp xuiInboundsResponse
	if err := client.doJSON(ctx, "GET", common.XuiInboundsPath, nil, &resp); err != nil {
		return nil, err
	}
	if !resp.Success {
		return nil, fmt.Errorf("x-ui rejected inbounds request: %s", resp.Msg)
	}

	return resp.Obj, nil
}

// AddClient creates clientToAdd on panel's DefaultInboundID inbound.
// settings is a JSON-string-within-JSON (x-ui's own API quirk) -- marshal
// the clients list once for the inner string, then let the outer request
// marshal normally.
func AddClient(ctx context.Context, panel model.XuiPanel, clientToAdd XuiClient) error {
	client, err := Login(ctx, panel)
	if err != nil {
		return err
	}

	settingsJSON, err := json.Marshal(xuiClientSettings{Clients: []XuiClient{clientToAdd}})
	if err != nil {
		return fmt.Errorf("marshal client settings: %w", err)
	}

	req := addClientRequest{
		ID:       panel.DefaultInboundID,
		Settings: string(settingsJSON),
	}

	var resp xuiActionResponse
	if err := client.doJSON(ctx, "POST", common.XuiAddClientPath, req, &resp); err != nil {
		return err
	}
	if !resp.Success {
		return fmt.Errorf("x-ui rejected addClient: %s", resp.Msg)
	}

	return nil
}

// UpdateClient replaces clientUUID's settings on panel -- used both to
// disable a client on quota exhaustion (updated.Enable=false) and to reset
// usage after a top-up (paired with a fresh ExpiryTime).
func UpdateClient(ctx context.Context, panel model.XuiPanel, clientUUID string, updated XuiClient) error {
	client, err := Login(ctx, panel)
	if err != nil {
		return err
	}

	settingsJSON, err := json.Marshal(xuiClientSettings{Clients: []XuiClient{updated}})
	if err != nil {
		return fmt.Errorf("marshal client settings: %w", err)
	}

	req := addClientRequest{
		ID:       panel.DefaultInboundID,
		Settings: string(settingsJSON),
	}

	path := fmt.Sprintf(common.XuiUpdateClientPathFmt, clientUUID)

	var resp xuiActionResponse
	if err := client.doJSON(ctx, "POST", path, req, &resp); err != nil {
		return err
	}
	if !resp.Success {
		return fmt.Errorf("x-ui rejected updateClient: %s", resp.Msg)
	}

	return nil
}

// GetClientTraffics returns clientEmail's cumulative up+down byte counts
// on panel. Callers are responsible for delta-accumulating against a
// previously cached total (see service.SyncPackageUsage), matching how
// RouterOS's own counters are handled elsewhere in this codebase.
func GetClientTraffics(ctx context.Context, panel model.XuiPanel, clientEmail string) (totalBytes int64, err error) {
	up, down, err := GetClientTrafficsSplit(ctx, panel, clientEmail)
	if err != nil {
		return 0, err
	}
	return up + down, nil
}

// GetClientTrafficsSplit is GetClientTraffics with the up/down breakdown
// preserved -- used by the on-demand "view live usage" action
// (V2RayPackageService.GetLiveUsage), which shows both directions
// separately rather than only their sum.
func GetClientTrafficsSplit(ctx context.Context, panel model.XuiPanel, clientEmail string) (up, down int64, err error) {
	client, err := Login(ctx, panel)
	if err != nil {
		return 0, 0, err
	}

	path := fmt.Sprintf(common.XuiGetClientTrafficsFmt, clientEmail)

	var resp xuiTrafficResponse
	if err := client.doJSON(ctx, "GET", path, nil, &resp); err != nil {
		return 0, 0, err
	}
	if !resp.Success {
		return 0, 0, fmt.Errorf("x-ui rejected getClientTraffics: %s", resp.Msg)
	}

	return resp.Obj.Up, resp.Obj.Down, nil
}

// GetOnlineClients returns the set of client emails currently holding an
// open connection on panel, across every inbound -- used to compute each
// V2RayPackageLocation's is_online indicator (see V2RaySyncService's
// online-status poll). x-ui itself only tracks "currently connected," not
// a per-client history, so this is inherently a point-in-time snapshot:
// a client that connects and disconnects between two polls is never
// observed online, matching the same caching/staleness tradeoff already
// accepted for UsedBytesCached elsewhere in this package.
func GetOnlineClients(ctx context.Context, panel model.XuiPanel) (map[string]bool, error) {
	client, err := Login(ctx, panel)
	if err != nil {
		return nil, err
	}

	var resp xuiOnlinesResponse
	if err := client.doJSON(ctx, "POST", common.XuiOnlinesPath, nil, &resp); err != nil {
		return nil, err
	}
	if !resp.Success {
		return nil, fmt.Errorf("x-ui rejected onlines request: %s", resp.Msg)
	}

	emails, err := parseXuiOnlinesObj(resp.Obj)
	if err != nil {
		return nil, err
	}

	online := make(map[string]bool, len(emails))
	for _, email := range emails {
		online[email] = true
	}
	return online, nil
}

// GetSubscription fetches panel's raw base64 subscription content for
// subID -- the un-decoded blob x-ui itself serves at sub_base_url/{subId}.
// Decoding/rewriting/re-encoding happens in the sync service, not here.
//
// Deliberately does NOT call Login -- a real x-ui deployment serves the
// subscription endpoint on its OWN public, unauthenticated HTTP(S) server
// (sub_base_url), completely separate from the admin panel API
// (api_base_url): different port, often a different reverse proxy/TLS
// cert, and no session/cookie check at all. This is also exactly how every
// real V2Ray client app (v2rayNG, v2box, Shadowrocket, ...) fetches a
// subscription URL: a bare GET, no login step of any kind.
//
// A prior version of this function called Login(ctx, panel) first and
// reused that authenticated *Client's cookie-jar-bearing httpClient here.
// That was a confirmed, reproduced bug: Go's cookiejar (net/http/
// cookiejar, RFC 6265-compliant) scopes cookies by HOST only, not port, so
// when api_base_url and sub_base_url share a hostname on different ports
// (the common real-world layout, e.g. host:2053 admin / host:2096 sub) the
// admin panel's session cookie was replayed onto every subscription
// request. Any subscription-side reverse proxy/WAF that treats a foreign,
// unrecognized cookie as suspicious (a realistic hardened-deployment
// posture) then rejected the request outright. Because syncOneLocation
// treats a GetSubscription failure as non-fatal and explicitly keeps
// whatever ConfigLinkCached already held (see its own doc comment), this
// silently and PERMANENTLY froze the cached subscription link -- including
// x-ui's own raw, never-rewritten default content, which titles every
// client link with the raw client email (e.g. "pkg_<shortid>_<panelid>")
// instead of the resolved sale title. That is the exact shape of a
// previously reported live bug. Fetching the subscription unauthenticated,
// with its own plain client, removes this entire class of failure and
// matches how the subscription endpoint is actually meant to be consumed.
func GetSubscription(ctx context.Context, panel model.XuiPanel, subID string) (string, error) {
	// TrimSuffix guards against a double slash (e.g. "https://host/sub//"
	// + subID) when the admin's stored SubBaseURL already ends in "/" --
	// this produced a literal "sub//<uuid>" path that most x-ui deployments
	// (and any reverse proxy in front of them) 404 on, since it doesn't
	// match the registered "sub/:subId" route.
	baseURL := strings.TrimSuffix(panel.SubBaseURL, "/")
	return getSubscriptionRaw(ctx, baseURL+"/"+subID)
}
