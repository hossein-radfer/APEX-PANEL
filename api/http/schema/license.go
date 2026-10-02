package schema

// LicenseStatusResponse mirrors service.LicenseStatus for the admin UI's
// license warning banner.
type LicenseStatusResponse struct {
	Activated     bool   `json:"activated"`
	Valid         bool   `json:"valid"`
	Reason        string `json:"reason,omitempty"`
	PlanName      string `json:"plan_name,omitempty"`
	ExpiresAt     string `json:"expires_at,omitempty"`
	ServerCount   int    `json:"server_count"`
	MaxServers    int    `json:"max_servers"`
	LastCheckedAt string `json:"last_checked_at,omitempty"`
	InGracePeriod bool   `json:"in_grace_period"`
}

// PublicLicenseStatusResponse is the minimal, unauthenticated subset of
// LicenseStatusResponse the login/activation screen needs to decide
// whether to render the lockdown UI -- deliberately excludes PlanName,
// ServerCount, MaxServers, and LastCheckedAt (no reason to expose licensing
// business details to an unauthenticated caller who merely knows the
// panel's URL). See http/license.go's PublicStatus handler.
type PublicLicenseStatusResponse struct {
	Activated     bool   `json:"activated"`
	Valid         bool   `json:"valid"`
	InGracePeriod bool   `json:"in_grace_period"`
	Reason        string `json:"reason,omitempty"`
}

// ActivateLicenseRequest is submitted from the web UI's license-activation
// page (see http/license.go's Activate handler). Username/Password are
// required because this endpoint is reachable without a JWT session (see
// that handler's doc comment for why) -- they authenticate the caller as a
// real admin so a license binding can't be hijacked by anyone who merely
// knows the panel's URL.
type ActivateLicenseRequest struct {
	Username   string `json:"username" validate:"required"`
	Password   string `json:"password" validate:"required"`
	LicenseKey string `json:"license_key" validate:"required"`
}

// StartTrialRequest is submitted from the web UI's "Free Trial" tab (see
// http/license.go's Trial handler). Username/Password are required for
// the same reason as ActivateLicenseRequest (this endpoint is also
// reachable without a JWT session, and must authenticate the caller as a
// real admin itself). Email is additionally required so the trial's
// auto-created Customer record on license-panel has a way to be followed
// up with -- see license-panel's LicenseService.IssueSelfTrial.
type StartTrialRequest struct {
	Username string `json:"username" validate:"required"`
	Password string `json:"password" validate:"required"`
	Email    string `json:"email" validate:"required,email"`
}

// UpdateCheckResponse is a trimmed view of license.UpdateCheckResponse --
// deliberately does NOT expose DownloadURL/SHA256/Signature to the
// frontend directly; those are only ever used server-side when the admin
// clicks "Install Update" (see a future UpdateService.Install call),
// never sent to the browser. ForceUpdate is purely informational (whether
// the release publisher marked this version mandatory) and carries no
// sensitive material, so it's safe to expose directly.
type UpdateCheckResponse struct {
	UpdateAvailable bool   `json:"update_available"`
	Version         string `json:"version,omitempty"`
	ChangeLog       string `json:"change_log,omitempty"`
	ForceUpdate     bool   `json:"force_update,omitempty"`
	CurrentVersion  string `json:"current_version"`
}
