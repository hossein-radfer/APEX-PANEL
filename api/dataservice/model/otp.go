package model

import "time"

// OtpChallenge is one pending admin-login two-factor verification -- issued
// by Authentication.Login the moment an admin's username/password check
// succeeds while BotSettings.OtpEnabled is true, and consumed exactly once
// by Authentication.VerifyOtp. The real access/refresh token pair is never
// generated until this challenge is verified, so a stolen password alone is
// not enough to reach the panel -- the attacker would also need to receive
// the code via the admin's own Telegram bot chat(s).
//
// Token is a random, unguessable string returned to the client in place of
// LoginResponse's tokens (see schema.LoginResponse.OtpToken) -- the client
// must present it back on /auth/otp/verify alongside the 4-digit code, so a
// verify request can never be replayed against a DIFFERENT pending
// challenge than the one the login response actually returned.
//
// CodeHash is bcrypt, never the plaintext code -- mirrors every other
// credential in this codebase (Admin.Password, Reseller.PasswordHash)
// never being stored in plaintext, even though this code is short-lived.
type OtpChallenge struct {
	Model
	Token        string    `gorm:"type:varchar(64);uniqueIndex;not null"`
	Username     string    `gorm:"type:varchar(64);not null"`
	CodeHash     string    `gorm:"type:varchar(128);not null"`
	IPAddress    string    `gorm:"type:varchar(64);not null"`
	AttemptCount int       `gorm:"not null;default:0"`
	ExpiresAt    time.Time `gorm:"not null;index"`
	// ConsumedAt is set the moment a challenge is successfully verified --
	// a non-nil value makes the token permanently unusable even if it
	// hasn't expired yet, closing the window where a code could otherwise
	// be replayed a second time before ExpiresAt.
	ConsumedAt *time.Time
}

// OtpIpBan is a temporary lockout on further OTP attempts from one IP
// address, created once that IP accumulates too many wrong codes in a
// row (see Authentication.VerifyOtp). Scoped to the IP rather than the
// username, since there is normally only one admin account.
type OtpIpBan struct {
	Model
	IPAddress   string    `gorm:"type:varchar(64);uniqueIndex;not null"`
	BannedUntil time.Time `gorm:"not null;index"`
}
