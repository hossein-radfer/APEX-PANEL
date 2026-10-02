package doctordns

import (
	"context"
	"fmt"

	"github.com/maahdima/mwp/api/dataservice/model"
)

// User mirrors doctor-dns's _apex_user_view() response shape exactly (see
// smartdns-panel's own _apex_user_view method) -- returned by every
// /apex/user-* call that echoes back the account's current state.
type User struct {
	ApexRef    string  `json:"apex_ref"`
	Status     string  `json:"status"`
	QuotaBytes int64   `json:"quota_bytes"`
	UsedBytes  int64   `json:"used_bytes"`
	SpeedKbps  int     `json:"speed_kbps"`
	ExpiresAt  *string `json:"expires_at"`
	TemplateID *int    `json:"template_id"`
	MaxIPs     int     `json:"max_ips"`
	IP         *string `json:"ip"`
}

// Template mirrors one entry of /apex/templates's response.
type Template struct {
	ID        int    `json:"id"`
	Name      string `json:"name"`
	IsDefault bool   `json:"is_default"`
}

type userEnvelope struct {
	Ok   bool `json:"ok"`
	User User `json:"user"`
}

type createUserRequest struct {
	ApexRef    string   `json:"apex_ref"`
	Label      string   `json:"label"`
	QuotaBytes int64    `json:"quota_bytes"`
	SpeedKbps  int      `json:"speed_kbps"`
	ExpireDays *float64 `json:"expire_days,omitempty"`
	TemplateID *int     `json:"template_id,omitempty"`
	MaxIPs     int      `json:"max_ips"`
}

// CreateOrUpdateUser creates, or fully re-provisions (upsert keyed on
// ApexRef), one DNS account on panel. Safe to call again for a renewal or a
// plan change -- doctor-dns's own create_apex_user is itself an upsert (see
// that function's doc comment), so this never creates a duplicate account.
// expireDays of nil or 0 means "never expires", matching doctor-dns's own
// expire_days semantics exactly (a positive value is a whole/fractional day
// count added to the current time on the doctor-dns side, not a fixed
// calendar date, so re-provisioning always extends from "now").
func CreateOrUpdateUser(ctx context.Context, panel model.DNSPanel, apexRef, label string, quotaBytes int64, speedKbps int, expireDays *float64, templateID *int, maxIPs int) (*User, error) {
	if maxIPs < 1 {
		maxIPs = 1
	}
	req := createUserRequest{
		ApexRef:    apexRef,
		Label:      label,
		QuotaBytes: quotaBytes,
		SpeedKbps:  speedKbps,
		ExpireDays: expireDays,
		TemplateID: templateID,
		MaxIPs:     maxIPs,
	}
	var resp userEnvelope
	if err := doJSON(ctx, panel, "/apex/user-create", req, &resp); err != nil {
		return nil, err
	}
	return &resp.User, nil
}

// SetUserStatus suspends or resumes apexRef without touching quota, speed,
// expiry or template -- the mid-cycle toggle a reseller needs on a wallet/
// invoice-driven suspend, mirroring how V2Ray/WireGuard's own suspend paths
// avoid re-sending the full provisioning payload for a pure status flip.
func SetUserStatus(ctx context.Context, panel model.DNSPanel, apexRef, status string) (*User, error) {
	if status != "active" && status != "suspended" {
		return nil, fmt.Errorf("status must be \"active\" or \"suspended\", got %q", status)
	}
	req := map[string]string{"apex_ref": apexRef, "status": status}
	var resp userEnvelope
	if err := doJSON(ctx, panel, "/apex/user-status", req, &resp); err != nil {
		return nil, err
	}
	return &resp.User, nil
}

// ResetUsage zeros apexRef's byte counter on doctor-dns's own side.
//
// Quota enforcement on doctor-dns compares its own used_bytes column against
// quota_bytes on every sync tick (see enforce_quotas in smartdns-panel) --
// never anything this service sends -- so zeroing only DNSAccount's local
// UsedBytesCached/usage_offset_bytes leaves the account exactly as over
// quota as it was on doctor-dns's side, and the very next tick re-suspends
// it (status over_quota) even right after an explicit resume. This call is
// the only way to actually clear that.
func ResetUsage(ctx context.Context, panel model.DNSPanel, apexRef string) (*User, error) {
	req := map[string]string{"apex_ref": apexRef}
	var resp userEnvelope
	if err := doJSON(ctx, panel, "/apex/user-reset-usage", req, &resp); err != nil {
		return nil, err
	}
	return &resp.User, nil
}

// DeleteUser removes apexRef's account on panel entirely.
func DeleteUser(ctx context.Context, panel model.DNSPanel, apexRef string) error {
	req := map[string]string{"apex_ref": apexRef}
	return doJSON(ctx, panel, "/apex/user-delete", req, nil)
}

// GetUserState is a READ-ONLY poll of apexRef's current state -- deliberately
// separate from CreateOrUpdateUser, whose upsert semantics would silently
// reset quota/speed/expiry/template to whatever this call last sent. This is
// what the background sync job calls, never CreateOrUpdateUser.
func GetUserState(ctx context.Context, panel model.DNSPanel, apexRef string) (*User, error) {
	req := map[string]string{"apex_ref": apexRef}
	var resp userEnvelope
	if err := doJSON(ctx, panel, "/apex/user-state", req, &resp); err != nil {
		return nil, err
	}
	return &resp.User, nil
}

// ClaimIPResult mirrors do_claim_register's own {"ok", "message"} shape --
// Ok=false is a routine, expected outcome (e.g. the IP is already claimed by
// a different account on the same doctor-dns installation), not a
// transport-level error, so it is returned as a value, not an error.
type ClaimIPResult struct {
	Ok      bool   `json:"ok"`
	Message string `json:"message"`
}

// ClaimIP registers ip against apexRef's account, replacing the oldest
// registered IP once the account's own MaxIPs ceiling is reached (doctor-dns's
// existing, unchanged do_claim_register behavior).
func ClaimIP(ctx context.Context, panel model.DNSPanel, apexRef, ip string) (*ClaimIPResult, error) {
	req := map[string]string{"apex_ref": apexRef, "ip": ip}
	var resp ClaimIPResult
	if err := doJSON(ctx, panel, "/apex/user-claim-ip", req, &resp); err != nil {
		return nil, err
	}
	return &resp, nil
}

type templatesResponse struct {
	Templates []Template `json:"templates"`
}

// ListTemplates returns the DNS "plans" (routing bundles) configured on
// panel's own admin UI -- the picker Apex shows when creating/editing a
// DNSAccount.
func ListTemplates(ctx context.Context, panel model.DNSPanel) ([]Template, error) {
	var resp templatesResponse
	if err := doJSON(ctx, panel, "/apex/templates", map[string]string{}, &resp); err != nil {
		return nil, err
	}
	return resp.Templates, nil
}

// TestConnection is the admin's "Test Connection" button -- a cheap call
// (the template list) that also doubles as a connectivity/API-key check,
// mirroring xui.TestConnection's identical "login + list something cheap"
// pattern.
func TestConnection(ctx context.Context, panel model.DNSPanel) ([]Template, error) {
	return ListTemplates(ctx, panel)
}
