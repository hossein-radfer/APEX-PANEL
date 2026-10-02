package service

import (
	"fmt"
	"testing"
	"time"

	"github.com/glebarez/sqlite"
	"golang.org/x/crypto/bcrypt"
	"gorm.io/gorm"
	"go.uber.org/zap"

	"github.com/maahdima/mwp/api/dataservice/model"
)

func newTestAuthenticationForResellerOtp(t *testing.T) (*Authentication, *gorm.DB) {
	t.Helper()

	dsn := fmt.Sprintf("file:auth_reseller_otp_test_%d?mode=memory&cache=shared", time.Now().UnixNano())
	db, err := gorm.Open(sqlite.Open(dsn), &gorm.Config{})
	if err != nil {
		t.Fatalf("failed to open test db: %v", err)
	}
	if err := db.AutoMigrate(&model.Admin{}, &model.Reseller{}, &model.OtpChallenge{}, &model.OtpIpBan{}); err != nil {
		t.Fatalf("failed to migrate test db: %v", err)
	}

	botService := &BotService{logger: zap.NewNop()}
	botNotifier := NewBotNotifier(botService, db, NewBotSettingsService(db))

	return &Authentication{
		db:          db,
		botNotifier: botNotifier,
		settings:    NewBotSettingsService(db),
		logger:      zap.NewNop(),
	}, db
}

func hashPassword(t *testing.T, password string) string {
	t.Helper()
	hash, err := bcrypt.GenerateFromPassword([]byte(password), bcrypt.DefaultCost)
	if err != nil {
		t.Fatalf("failed to hash password: %v", err)
	}
	return string(hash)
}

// TestLogin_ResellerOtpDisabled_LogsInNormally confirms the baseline,
// unchanged behavior: a reseller with OtpEnabled=false (the default) logs
// in with a full token pair immediately, exactly as before this feature
// existed.
func TestLogin_ResellerOtpDisabled_LogsInNormally(t *testing.T) {
	auth, db := newTestAuthenticationForResellerOtp(t)

	chatID := "123456"
	reseller := model.Reseller{
		Name: "no-otp", Username: "no-otp-reseller", PasswordHash: hashPassword(t, "password123"),
		IsActive: true, OtpEnabled: false, TelegramChatID: &chatID,
	}
	if err := db.Create(&reseller).Error; err != nil {
		t.Fatalf("failed to create reseller: %v", err)
	}

	resp, err := auth.Login("no-otp-reseller", "password123", "1.2.3.4")
	if err != nil {
		t.Fatalf("Login failed: %v", err)
	}
	if resp.OtpRequired {
		t.Fatal("expected no OTP challenge for a reseller with OtpEnabled=false")
	}
	if resp.AccessToken == "" {
		t.Fatal("expected a real access token to be issued immediately")
	}
	if resp.Role != "reseller" {
		t.Fatalf("expected role 'reseller', got %q", resp.Role)
	}
}

// TestLogin_ResellerOtpEnabledWithChatID_StartsChallenge confirms the new
// feature: a reseller with OtpEnabled=true AND a TelegramChatID set gets an
// OTP challenge instead of an immediate token pair.
func TestLogin_ResellerOtpEnabledWithChatID_StartsChallenge(t *testing.T) {
	auth, db := newTestAuthenticationForResellerOtp(t)

	chatID := "123456"
	reseller := model.Reseller{
		Name: "with-otp", Username: "with-otp-reseller", PasswordHash: hashPassword(t, "password123"),
		IsActive: true, OtpEnabled: true, TelegramChatID: &chatID,
	}
	if err := db.Create(&reseller).Error; err != nil {
		t.Fatalf("failed to create reseller: %v", err)
	}

	resp, err := auth.Login("with-otp-reseller", "password123", "1.2.3.4")
	if err != nil {
		t.Fatalf("Login failed: %v", err)
	}
	if !resp.OtpRequired {
		t.Fatal("expected an OTP challenge for a reseller with OtpEnabled=true and a TelegramChatID")
	}
	if resp.OtpToken == "" {
		t.Fatal("expected a non-empty otp_token")
	}
	if resp.AccessToken != "" {
		t.Fatal("expected no access token yet -- OTP is pending")
	}

	var challenge model.OtpChallenge
	if err := db.Where("token = ?", resp.OtpToken).First(&challenge).Error; err != nil {
		t.Fatalf("expected a persisted OtpChallenge row: %v", err)
	}
	if challenge.Username != "with-otp-reseller" {
		t.Fatalf("expected challenge username 'with-otp-reseller', got %q", challenge.Username)
	}
}

// TestLogin_ResellerOtpEnabledWithoutChatID_SkipsChallenge confirms the
// safety fallback: OtpEnabled=true but no TelegramChatID means OTP
// degrades to "off" rather than issuing an undeliverable challenge that
// would lock the reseller out.
func TestLogin_ResellerOtpEnabledWithoutChatID_SkipsChallenge(t *testing.T) {
	auth, db := newTestAuthenticationForResellerOtp(t)

	reseller := model.Reseller{
		Name: "no-chat", Username: "no-chat-reseller", PasswordHash: hashPassword(t, "password123"),
		IsActive: true, OtpEnabled: true, TelegramChatID: nil,
	}
	if err := db.Create(&reseller).Error; err != nil {
		t.Fatalf("failed to create reseller: %v", err)
	}

	resp, err := auth.Login("no-chat-reseller", "password123", "1.2.3.4")
	if err != nil {
		t.Fatalf("Login failed: %v", err)
	}
	if resp.OtpRequired {
		t.Fatal("expected OTP to be skipped when TelegramChatID is nil, even with OtpEnabled=true")
	}
	if resp.AccessToken == "" {
		t.Fatal("expected a real access token since OTP degraded to off")
	}
}

// TestVerifyOtp_ResellerChallenge_IssuesResellerTokens confirms the full
// round trip: a reseller OTP challenge, once correctly verified, issues a
// token pair with role="reseller" and the correct reseller_id -- not an
// admin token, and VerifyOtp's own admin-then-reseller fallback correctly
// resolves a reseller-owned challenge.
func TestVerifyOtp_ResellerChallenge_IssuesResellerTokens(t *testing.T) {
	auth, db := newTestAuthenticationForResellerOtp(t)

	chatID := "123456"
	reseller := model.Reseller{
		Name: "verify-me", Username: "verify-me-reseller", PasswordHash: hashPassword(t, "password123"),
		IsActive: true, OtpEnabled: true, TelegramChatID: &chatID,
	}
	if err := db.Create(&reseller).Error; err != nil {
		t.Fatalf("failed to create reseller: %v", err)
	}

	loginResp, err := auth.Login("verify-me-reseller", "password123", "1.2.3.4")
	if err != nil {
		t.Fatalf("Login failed: %v", err)
	}
	if !loginResp.OtpRequired {
		t.Fatal("expected an OTP challenge")
	}

	var challenge model.OtpChallenge
	if err := db.Where("token = ?", loginResp.OtpToken).First(&challenge).Error; err != nil {
		t.Fatalf("failed to load challenge: %v", err)
	}

	// Recover the real code the same way maybeStartResellerOtpChallenge
	// generated it -- the code itself was only bcrypt-hashed and sent via
	// Telegram (a no-op here since api is nil), so extract it by brute
	// force isn't viable; instead, verify the bcrypt hash accepts the
	// known plaintext by re-deriving through a fresh hash comparison is
	// not possible without the plaintext. Overwrite the stored hash with a
	// known code instead, matching how this codebase's other OTP tests
	// would need to work around the same one-way hash.
	knownCode := "1234"
	newHash, err := bcrypt.GenerateFromPassword([]byte(knownCode), bcrypt.DefaultCost)
	if err != nil {
		t.Fatalf("failed to hash known code: %v", err)
	}
	if err := db.Model(&challenge).Update("code_hash", string(newHash)).Error; err != nil {
		t.Fatalf("failed to overwrite code hash: %v", err)
	}

	verifyResp, err := auth.VerifyOtp(loginResp.OtpToken, knownCode, "1.2.3.4")
	if err != nil {
		t.Fatalf("VerifyOtp failed: %v", err)
	}
	if verifyResp.Role != "reseller" {
		t.Fatalf("expected role 'reseller', got %q", verifyResp.Role)
	}
	if verifyResp.ResellerID == nil || *verifyResp.ResellerID != reseller.ID {
		t.Fatalf("expected reseller_id %d, got %v", reseller.ID, verifyResp.ResellerID)
	}
	if verifyResp.AccessToken == "" {
		t.Fatal("expected a real access token after successful verification")
	}
}
