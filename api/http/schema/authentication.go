package schema

type LoginRequest struct {
	Username string `json:"username" validate:"required"`
	Password string `json:"password" validate:"required"`
}

// LoginResponse covers two distinct outcomes of POST /auth/login, told
// apart by OtpRequired: a normal login (OtpRequired false/omitted) carries
// the real UserID/Username/AccessToken/RefreshToken/ExpiresIn/Role/
// ResellerID fields exactly as before; an admin login with
// BotSettings.OtpEnabled=true instead returns ONLY OtpRequired=true plus
// OtpToken -- every other field is its Go zero value and must be ignored by
// the client, which should immediately redirect to the OTP-verify screen
// and exchange OtpToken (+ the 4-digit code the admin receives via
// Telegram) for the real token pair via POST /auth/otp/verify.
type LoginResponse struct {
	UserID       uint   `json:"user_id"`
	Username     string `json:"username"`
	AccessToken  string `json:"access_token"`
	RefreshToken string `json:"refresh_token"`
	ExpiresIn    int64  `json:"expires_in"`
	Role         string `json:"role,omitempty"`
	ResellerID   *uint  `json:"reseller_id,omitempty"`
	OtpRequired  bool   `json:"otp_required,omitempty"`
	OtpToken     string `json:"otp_token,omitempty"`
}

type VerifyOtpRequest struct {
	OtpToken string `json:"otp_token" validate:"required"`
	Code     string `json:"code" validate:"required"`
}

type UpdateProfileRequest struct {
	OldPassword string  `json:"old_password" validate:"required"`
	NewUsername *string `json:"new_username"`
	NewPassword *string `json:"new_password"`
}
