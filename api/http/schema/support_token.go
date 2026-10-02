package schema

// GenerateSupportTokenResponse returns the raw token exactly once -- see
// service.SupportTokenService.Generate's doc comment; it is never
// retrievable again after this response, only its expiry/active state.
type GenerateSupportTokenResponse struct {
	Token     string `json:"token"`
	ExpiresAt string `json:"expires_at"`
}

// SupportTokenStatusResponse never includes the token value itself.
type SupportTokenStatusResponse struct {
	Active    bool   `json:"active"`
	ExpiresAt string `json:"expires_at,omitempty"`
}
