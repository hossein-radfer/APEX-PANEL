package model

import "time"

// ApiKey is an admin-issued credential for THIRD-PARTY/EXTERNAL clients
// (another panel instance, an automation script) to call the panel's
// external API surface (see setupExternalApiRoutes) WITHOUT an admin JWT --
// phase 4-3's "public API key separate from the admin JWT for the whole
// panel". Deliberately admin-scoped (not per-reseller): the external API
// surface this guards is a whole-panel integration point, not a reseller
// self-service credential.
//
// Multiple concurrent keys are supported (unlike SupportToken's single-row
// design) so an admin can issue a separately labeled, separately revocable
// key per integration (e.g. "Iran panel sync", "billing automation script")
// without one integration's compromise forcing every other integration to
// be re-keyed too.
type ApiKey struct {
	Model
	// Label is the admin-chosen name identifying what this key is for
	// (shown in the key list instead of the key itself, which is never
	// displayed again after creation).
	Label string `gorm:"type:varchar(100);not null"`
	// KeyHash is bcrypt(rawKey), never the raw key -- mirrors
	// SupportToken.TokenHash so a database dump alone never leaks a live
	// credential.
	KeyHash string `gorm:"type:varchar(128);not null"`
	// KeyPrefix is the first 8 characters of the raw key, stored in the
	// clear so the admin can visually distinguish keys in the list (e.g.
	// confirm which key a client is actually sending) without either
	// storing the full raw key or needing to re-hash-and-compare against
	// every row just to render a helpful label.
	KeyPrefix string `gorm:"type:varchar(16);not null"`
	// LastUsedAt is nil until the key authenticates its first request --
	// surfaced in the admin UI so a key that's never been used (or has
	// gone unexpectedly quiet) is visible at a glance, per phase 4-3's
	// "generate/revoke/view usage" requirement.
	LastUsedAt *time.Time
	// Revoked keys are never deleted (unlike SupportToken's overwrite
	// semantics) so the admin retains an audit trail of what integrations
	// existed historically; APIKeyMiddleware rejects any key with
	// Revoked=true exactly like it would a nonexistent one.
	Revoked bool `gorm:"not null;default:false"`
}
