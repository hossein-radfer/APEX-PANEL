// Package license implements MWPanel's side of the license/update
// protocol -- the counterpart to license-panel's internal/licenseproto
// and internal/licensecrypto packages (see D:\license-panel). MWPanel
// intentionally does NOT import that module directly (the two are
// separate products with separate release cycles); these structs are a
// hand-maintained copy of the wire format. If you change one side's
// field names/tags, update the other to match, or signature verification
// will fail for every install.
package license

// ActivateRequest is sent once, the first time MWPanel starts with a
// license key it has not activated before.
type ActivateRequest struct {
	LicenseKey  string `json:"license_key"`
	Fingerprint string `json:"fingerprint"`
	Hostname    string `json:"hostname"`
	MWPVersion  string `json:"mwp_version"`
}

// HeartbeatRequest is sent periodically to keep proving MWPanel is still
// entitled to run.
type HeartbeatRequest struct {
	LicenseKey  string `json:"license_key"`
	Fingerprint string `json:"fingerprint"`
	MWPVersion  string `json:"mwp_version"`
}

// TrialRequest self-service claims a free trial license, instead of
// activating with a purchased key -- see service.LicenseService.StartTrial.
// Unlike Activate, this can fail with a real HTTP error (trials disabled,
// this fingerprint already claimed one) rather than a signed "invalid"
// response, since there is no pre-existing license to attach a rejection
// to on the server side.
type TrialRequest struct {
	Email       string `json:"email"`
	Fingerprint string `json:"fingerprint"`
	Hostname    string `json:"hostname"`
	MWPVersion  string `json:"mwp_version"`
}

// ValidateResponse is the signed payload license-panel returns for both
// Activate and Heartbeat. Field order/tags must exactly match
// license-panel's internal/licenseproto.ValidateResponse -- the signature
// is computed over the literal JSON bytes of this struct.
type ValidateResponse struct {
	Valid       bool   `json:"valid"`
	Reason      string `json:"reason,omitempty"`
	LicenseKey  string `json:"license_key"`
	PlanName    string `json:"plan_name"`
	ExpiresAt   string `json:"expires_at,omitempty"`
	IssuedAt    string `json:"issued_at"`
	ServerCount int    `json:"server_count"`
	MaxServers  int    `json:"max_servers"`
	CheckedAt   string `json:"checked_at"`

	// Restricted/FreeTierLimits implement phase 4-12's granular free-tier
	// system -- MUST stay byte-identical (field order, json tags) to
	// license-panel's own licenseproto.ValidateResponse, since the
	// signature is computed over the literal JSON bytes. See that repo's
	// own copy of this struct for the full doc comment on what these mean
	// and why Restricted is a separate flag from Valid (a Restricted
	// license is still Valid=true -- the panel keeps running, just under
	// reduced caps, per the admin's own explicit "افت به سطح پایین‌تر
	// به‌جای قفل کامل" requirement).
	Restricted     bool             `json:"restricted,omitempty"`
	FreeTierLimits map[string]int64 `json:"free_tier_limits,omitempty"`
}

type SignedValidateResponse struct {
	Payload   ValidateResponse `json:"payload"`
	Signature string           `json:"signature"`
}

// NonceRequest asks license-panel for a fresh, single-use challenge value
// -- step one of a two-step challenge/response flow used for capability
// checks that must not be satisfiable by a replayed cached response.
type NonceRequest struct {
	Fingerprint string `json:"fingerprint"`
}

// NonceResponse carries the freshly issued challenge token.
type NonceResponse struct {
	Nonce     string `json:"nonce"`
	ExpiresAt string `json:"expires_at"`
}

// CapabilityValidateRequest redeems a nonce previously obtained via
// NonceRequest -- step two of the challenge/response flow.
type CapabilityValidateRequest struct {
	LicenseKey    string `json:"license_key"`
	Fingerprint   string `json:"fingerprint"`
	Nonce         string `json:"nonce"`
	CapabilityKey string `json:"capability_key"`
}

// CapabilityValidateResponse is the signed payload ValidateCapability
// returns. Field order/tags must exactly match license-panel's
// internal/licenseproto.CapabilityValidateResponse (same hand-sync
// requirement as ValidateResponse above) -- Nonce is echoed back INSIDE
// the signed payload so ValidateCapability (this package's own function,
// see verify.go) can confirm the signature it just checked is over the
// EXACT nonce this specific call asked for, not a replayed signature
// from an earlier, different challenge.
type CapabilityValidateResponse struct {
	Valid         bool   `json:"valid"`
	Reason        string `json:"reason,omitempty"`
	LicenseKey    string `json:"license_key"`
	CapabilityKey string `json:"capability_key"`
	Nonce         string `json:"nonce"`
	CheckedAt     string `json:"checked_at"`
}

type SignedCapabilityValidateResponse struct {
	Payload   CapabilityValidateResponse `json:"payload"`
	Signature string                     `json:"signature"`
}

// UpdateCheckRequest asks license-panel whether a newer MWPanel release
// exists.
type UpdateCheckRequest struct {
	LicenseKey     string `json:"license_key"`
	Fingerprint    string `json:"fingerprint"`
	CurrentVersion string `json:"current_version"`
	Channel        string `json:"channel"`
}

// UpdateCheckResponse describes an available update, if any.
type UpdateCheckResponse struct {
	UpdateAvailable bool   `json:"update_available"`
	Version         string `json:"version,omitempty"`
	ChangeLog       string `json:"change_log,omitempty"`
	DownloadURL     string `json:"download_url,omitempty"`
	SHA256          string `json:"sha256,omitempty"`
	FileSizeBytes   int64  `json:"file_size_bytes,omitempty"`
	Signature       string `json:"signature,omitempty"`
	ForceUpdate     bool   `json:"force_update,omitempty"`
}
