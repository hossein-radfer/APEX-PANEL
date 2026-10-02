package model

import "time"

// SupportToken is the "privacy-first remote debugging" credential: a
// customer-generated, time-limited token that grants a read-only,
// logs-only view of THIS install to whoever holds it (the license-panel
// operator, entering it into their "Panel Logs" tool). Deliberately
// scoped to nothing beyond log read/filter -- no server status, no
// database access, no remote command execution -- and deliberately NOT
// an SSH credential of any kind, per this feature's core privacy
// requirement: the master admin must never be able to reach a shell on a
// customer's server.
//
// Single-row semantics: only one token is ever "current" for an install.
// Generating a new one overwrites this row, immediately invalidating
// whatever token existed before -- there is no history of past tokens.
type SupportToken struct {
	Model
	// TokenHash is bcrypt(token), never the raw token -- mirrors how
	// admin/reseller passwords are stored, so a database dump alone never
	// leaks a live credential.
	TokenHash string `gorm:"type:varchar(128);not null"`
	ExpiresAt time.Time
}
