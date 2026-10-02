package service

import (
	"crypto/rand"
	"errors"
	"fmt"
	"math/big"
	"strconv"
	"time"

	"github.com/golang-jwt/jwt/v5"
	"go.uber.org/zap"
	"golang.org/x/crypto/bcrypt"
	"gorm.io/gorm"

	"github.com/maahdima/mwp/api/config"
	"github.com/maahdima/mwp/api/dataservice/model"
	"github.com/maahdima/mwp/api/http/schema"
	"github.com/maahdima/mwp/api/utils"
)

// OTP tuning constants.
const (
	otpCodeLength  = 4
	otpValidity    = 99 * time.Second
	otpMaxAttempts = 3
	otpBanDuration = 3 * time.Minute
)

var errOtpBanned = errors.New("too many failed attempts, try again later")
var errOtpInvalid = errors.New("invalid or expired code")

// ErrOtpBanned/ErrOtpInvalid are exported so the HTTP layer can tell these
// two specific failure reasons apart (429 vs 401) from any other error
// VerifyOtp might return.
var (
	ErrOtpBanned  = errOtpBanned
	ErrOtpInvalid = errOtpInvalid
)

var (
	accessTokenTTL  time.Duration
	refreshTokenTTL time.Duration
)

func init() {
	authCfg := config.GetAuthConfig()

	accessTTL, _ := strconv.Atoi(authCfg.AccessTokenTTL)
	refreshTTL, _ := strconv.Atoi(authCfg.RefreshTokenTTL)

	accessTokenTTL = time.Duration(accessTTL) * time.Second
	refreshTokenTTL = time.Duration(refreshTTL) * time.Second
}

type Authentication struct {
	db            *gorm.DB
	AccessSecret  []byte
	RefreshSecret []byte
	botNotifier   *BotNotifier
	settings      *BotSettingsService
	logger        *zap.Logger
}

// SetBotNotifier wires the OTP-delivery channel in after construction --
// mirrors WgPeer/ResellerService/UserManagerService/V2RayPackageService's
// identical SetBotNotifier(botNotifier) pattern (see http-server.go's
// construction order: botNotifier is built once and threaded into every
// service that needs to push a Telegram message).
func (a *Authentication) SetBotNotifier(notifier *BotNotifier) {
	a.botNotifier = notifier
}

func NewAuthentication(db *gorm.DB) *Authentication {
	systemConfig := NewSystemConfigService(db)
	logger := zap.L().Named("AuthenticationService")

	// Persisted, not regenerated per process start -- see
	// SystemConfigService.GetOrCreateJwtSecret's own doc comment for the
	// confirmed, reported bug this fixes (every restart used to silently
	// invalidate every admin/reseller session). A generation failure
	// falls back to a random, unpersisted secret rather than panicking
	// the whole process over an auth-secret write hiccup -- degrades to
	// the OLD (still-correct, just restart-unstable) behavior instead of
	// refusing to start.
	accessSecret, err := systemConfig.GetOrCreateJwtSecret("admin_access")
	if err != nil {
		logger.Error("failed to load/persist admin access secret, falling back to a per-process random one", zap.Error(err))
		accessSecret = utils.RandomString(24)
	}
	refreshSecret, err := systemConfig.GetOrCreateJwtSecret("admin_refresh")
	if err != nil {
		logger.Error("failed to load/persist admin refresh secret, falling back to a per-process random one", zap.Error(err))
		refreshSecret = utils.RandomString(24)
	}

	return &Authentication{
		db:            db,
		AccessSecret:  []byte(accessSecret),
		RefreshSecret: []byte(refreshSecret),
		settings:      NewBotSettingsService(db),
		logger:        logger,
	}
}

// Login authenticates by username/password, checking model.Admin first
// then model.Reseller. ipAddress is only used for the admin-only OTP gate
// below (see model.OtpChallenge's doc comment) -- a reseller login never
// triggers or is affected by it, matching the admin's own explicit
// "OTP is a login gate for the admin account" scope.
func (a *Authentication) Login(username, password, ipAddress string) (*schema.LoginResponse, error) {
	var admin model.Admin

	if err := a.db.First(&admin, "username = ?", username).Error; err == nil {
		if !admin.IsActive {
			return nil, errors.New("account disabled")
		}

		if err := bcrypt.CompareHashAndPassword([]byte(admin.Password), []byte(password)); err != nil {
			a.logger.Error("password mismatch", zap.String("username", username), zap.Error(err))
			return nil, errors.New("password mismatch")
		}

		if otpResp, err := a.maybeStartOtpChallenge(admin.Username, ipAddress); err != nil {
			return nil, err
		} else if otpResp != nil {
			return otpResp, nil
		}

		accessToken, refreshToken, err := a.generateTokenPair(username, admin.Role, admin.ResellerID)
		if err != nil {
			a.logger.Error("failed to generate token pair", zap.Error(err))
			return nil, err
		}

		expiresIn := int64(accessTokenTTL.Seconds())
		return &schema.LoginResponse{
			UserID:       admin.ID,
			Username:     admin.Username,
			AccessToken:  accessToken,
			RefreshToken: refreshToken,
			ExpiresIn:    expiresIn,
			Role:         admin.Role,
			ResellerID:   admin.ResellerID,
		}, nil
	} else if !errors.Is(err, gorm.ErrRecordNotFound) {
		a.logger.Error("failed to query admin from database", zap.Error(err))
		return nil, err
	}

	var reseller model.Reseller
	if err := a.db.First(&reseller, "username = ?", username).Error; err != nil {
		if errors.Is(err, gorm.ErrRecordNotFound) {
			a.logger.Error("user not found", zap.String("username", username))
			return nil, gorm.ErrRecordNotFound
		}
		a.logger.Error("failed to query reseller from database", zap.Error(err))
		return nil, err
	}

	if !reseller.IsActive {
		return nil, errors.New("account disabled")
	}

	if err := bcrypt.CompareHashAndPassword([]byte(reseller.PasswordHash), []byte(password)); err != nil {
		a.logger.Error("reseller password mismatch", zap.String("username", username), zap.Error(err))
		return nil, errors.New("password mismatch")
	}

	if otpResp, err := a.maybeStartResellerOtpChallenge(&reseller, ipAddress); err != nil {
		return nil, err
	} else if otpResp != nil {
		return otpResp, nil
	}

	resellerID := reseller.ID
	accessToken, refreshToken, err := a.generateTokenPair(reseller.Username, "reseller", &resellerID)
	if err != nil {
		a.logger.Error("failed to generate reseller token pair", zap.Error(err))
		return nil, err
	}

	expiresIn := int64(accessTokenTTL.Seconds())
	return &schema.LoginResponse{
		UserID:       reseller.ID,
		Username:     reseller.Username,
		AccessToken:  accessToken,
		RefreshToken: refreshToken,
		ExpiresIn:    expiresIn,
		Role:         "reseller",
		ResellerID:   &resellerID,
	}, nil
}

// maybeStartOtpChallenge is called right after an admin's password check
// succeeds. It returns (nil, nil) when OTP is disabled (BotSettings.
// OtpEnabled false) or misconfigured (no bot notifier wired, or no
// AdminChatID set -- issuing an unrecoverable challenge that can never be
// delivered would lock the admin out of their own panel, so this
// degrades to "OTP effectively off" rather than failing the login), in
// which case Login proceeds to issue real tokens exactly as before. When
// OTP is active, it generates a code, persists the challenge, sends the
// code via Telegram, and returns a non-nil *schema.LoginResponse carrying
// ONLY OtpRequired+OtpToken for Login to return immediately.
func (a *Authentication) maybeStartOtpChallenge(username, ipAddress string) (*schema.LoginResponse, error) {
	if a.botNotifier == nil || a.settings == nil {
		return nil, nil
	}
	settings, err := a.settings.GetOrCreate()
	if err != nil || !settings.OtpEnabled || settings.AdminChatID == "" {
		return nil, nil
	}

	if banned, err := a.isIPBanned(ipAddress); err != nil {
		a.logger.Error("failed to check otp ip ban", zap.Error(err))
	} else if banned {
		return nil, errOtpBanned
	}

	code, err := generateOtpCode()
	if err != nil {
		a.logger.Error("failed to generate otp code", zap.Error(err))
		return nil, err
	}
	codeHash, err := bcrypt.GenerateFromPassword([]byte(code), bcrypt.DefaultCost)
	if err != nil {
		a.logger.Error("failed to hash otp code", zap.Error(err))
		return nil, err
	}
	token := utils.RandomString(32)

	challenge := model.OtpChallenge{
		Token:     token,
		Username:  username,
		CodeHash:  string(codeHash),
		IPAddress: ipAddress,
		ExpiresAt: time.Now().Add(otpValidity),
	}
	if err := a.db.Create(&challenge).Error; err != nil {
		a.logger.Error("failed to persist otp challenge", zap.Error(err))
		return nil, err
	}

	a.botNotifier.NotifyOtpCode(settings, username, ipAddress, code, int(otpValidity.Seconds()))

	return &schema.LoginResponse{OtpRequired: true, OtpToken: token}, nil
}

// maybeStartResellerOtpChallenge mirrors maybeStartOtpChallenge exactly
// (same OtpChallenge row shape, same IP-ban check, same code generation),
// but is gated by THIS reseller's own Reseller.OtpEnabled field instead of
// the admin's global BotSettings.OtpEnabled, and delivers via the
// reseller's own TelegramChatID instead of the admin bot broadcast. Both
// OtpEnabled=false and a nil TelegramChatID degrade to "OTP effectively
// off" (same reasoning as maybeStartOtpChallenge's identical AdminChatID
// fallback) -- issuing a challenge that can never be delivered would lock
// the reseller out of their own account with no way to complete it.
func (a *Authentication) maybeStartResellerOtpChallenge(reseller *model.Reseller, ipAddress string) (*schema.LoginResponse, error) {
	if a.botNotifier == nil || !reseller.OtpEnabled || reseller.TelegramChatID == nil || *reseller.TelegramChatID == "" {
		return nil, nil
	}

	if banned, err := a.isIPBanned(ipAddress); err != nil {
		a.logger.Error("failed to check otp ip ban", zap.Error(err))
	} else if banned {
		return nil, errOtpBanned
	}

	code, err := generateOtpCode()
	if err != nil {
		a.logger.Error("failed to generate otp code", zap.Error(err))
		return nil, err
	}
	codeHash, err := bcrypt.GenerateFromPassword([]byte(code), bcrypt.DefaultCost)
	if err != nil {
		a.logger.Error("failed to hash otp code", zap.Error(err))
		return nil, err
	}
	token := utils.RandomString(32)

	challenge := model.OtpChallenge{
		Token:     token,
		Username:  reseller.Username,
		CodeHash:  string(codeHash),
		IPAddress: ipAddress,
		ExpiresAt: time.Now().Add(otpValidity),
	}
	if err := a.db.Create(&challenge).Error; err != nil {
		a.logger.Error("failed to persist reseller otp challenge", zap.Error(err))
		return nil, err
	}

	a.botNotifier.NotifyResellerOtpCode(*reseller.TelegramChatID, reseller.Username, ipAddress, code, int(otpValidity.Seconds()))

	return &schema.LoginResponse{OtpRequired: true, OtpToken: token}, nil
}

// generateOtpCode returns a random otpCodeLength-digit string, left-padded
// with zeros (e.g. "0042") -- crypto/rand rather than math/rand since this
// gates access to the panel, matching how session tokens (utils.
// RandomString) are already generated in this codebase.
func generateOtpCode() (string, error) {
	max := big.NewInt(1)
	for i := 0; i < otpCodeLength; i++ {
		max.Mul(max, big.NewInt(10))
	}
	n, err := rand.Int(rand.Reader, max)
	if err != nil {
		return "", err
	}
	return fmt.Sprintf("%0*d", otpCodeLength, n.Int64()), nil
}

// isIPBanned reports whether ipAddress currently has an active
// model.OtpIpBan (BannedUntil in the future) -- an expired ban row is
// treated as not-banned but deliberately left in place rather than deleted
// here (VerifyOtp's own failure path is what creates/overwrites bans; a
// stale expired row is harmless and gets overwritten the next time this
// same IP earns a fresh ban).
func (a *Authentication) isIPBanned(ipAddress string) (bool, error) {
	var ban model.OtpIpBan
	err := a.db.Where("ip_address = ?", ipAddress).First(&ban).Error
	if errors.Is(err, gorm.ErrRecordNotFound) {
		return false, nil
	}
	if err != nil {
		return false, err
	}
	return time.Now().Before(ban.BannedUntil), nil
}

// VerifyOtp consumes a pending challenge (created by maybeStartOtpChallenge
// above) and, on a correct code, issues the real token pair -- the second
// and final step of the admin OTP login gate. Every failure path is
// deliberately generic (ErrOtpInvalid) to a caller who doesn't already know
// which admin this challenge belongs to, EXCEPT the ban check, which is
// checked first and returns the more specific ErrOtpBanned so the frontend
// can show a distinct "wait N minutes" message.
func (a *Authentication) VerifyOtp(otpToken, code, ipAddress string) (*schema.LoginResponse, error) {
	if banned, err := a.isIPBanned(ipAddress); err != nil {
		a.logger.Error("failed to check otp ip ban", zap.Error(err))
	} else if banned {
		return nil, errOtpBanned
	}

	var challenge model.OtpChallenge
	if err := a.db.Where("token = ?", otpToken).First(&challenge).Error; err != nil {
		if errors.Is(err, gorm.ErrRecordNotFound) {
			return nil, errOtpInvalid
		}
		a.logger.Error("failed to load otp challenge", zap.Error(err))
		return nil, err
	}

	if challenge.ConsumedAt != nil || time.Now().After(challenge.ExpiresAt) {
		return nil, errOtpInvalid
	}

	if err := bcrypt.CompareHashAndPassword([]byte(challenge.CodeHash), []byte(code)); err != nil {
		// Wrong code: bump this challenge's attempt count and, once this IP
		// has struck out across whatever challenge(s) it has been trying,
		// ban it for otpBanDuration -- banning happens here (on failure),
		// not on challenge creation, so an admin who just mistypes their
		// own code once is never banned, only genuine repeated failures.
		newCount := challenge.AttemptCount + 1
		if updateErr := a.db.Model(&challenge).Update("attempt_count", newCount).Error; updateErr != nil {
			a.logger.Warn("failed to persist otp attempt count", zap.Error(updateErr))
		}
		if newCount >= otpMaxAttempts {
			a.banIP(ipAddress)
			return nil, errOtpBanned
		}
		return nil, errOtpInvalid
	}

	now := time.Now()
	if err := a.db.Model(&challenge).Update("consumed_at", now).Error; err != nil {
		a.logger.Error("failed to mark otp challenge consumed", zap.Error(err))
		return nil, err
	}

	var admin model.Admin
	if err := a.db.First(&admin, "username = ?", challenge.Username).Error; err == nil {
		if !admin.IsActive {
			return nil, errors.New("account disabled")
		}

		accessToken, refreshToken, err := a.generateTokenPair(admin.Username, admin.Role, admin.ResellerID)
		if err != nil {
			a.logger.Error("failed to generate token pair after otp verification", zap.Error(err))
			return nil, err
		}

		return &schema.LoginResponse{
			UserID:       admin.ID,
			Username:     admin.Username,
			AccessToken:  accessToken,
			RefreshToken: refreshToken,
			ExpiresIn:    int64(accessTokenTTL.Seconds()),
			Role:         admin.Role,
			ResellerID:   admin.ResellerID,
		}, nil
	} else if !errors.Is(err, gorm.ErrRecordNotFound) {
		a.logger.Error("failed to load admin for otp verification", zap.Error(err))
		return nil, err
	}

	// Not an admin username -- this challenge belongs to a reseller OTP
	// login instead (see maybeStartResellerOtpChallenge), mirroring Login's
	// own admin-then-reseller fallback order exactly.
	var reseller model.Reseller
	if err := a.db.First(&reseller, "username = ?", challenge.Username).Error; err != nil {
		a.logger.Error("failed to load reseller for otp verification", zap.Error(err))
		return nil, err
	}
	if !reseller.IsActive {
		return nil, errors.New("account disabled")
	}

	resellerID := reseller.ID
	accessToken, refreshToken, err := a.generateTokenPair(reseller.Username, "reseller", &resellerID)
	if err != nil {
		a.logger.Error("failed to generate reseller token pair after otp verification", zap.Error(err))
		return nil, err
	}

	return &schema.LoginResponse{
		UserID:       reseller.ID,
		Username:     reseller.Username,
		AccessToken:  accessToken,
		RefreshToken: refreshToken,
		ExpiresIn:    int64(accessTokenTTL.Seconds()),
		Role:         "reseller",
		ResellerID:   &resellerID,
	}, nil
}

// banIP upserts a model.OtpIpBan for ipAddress, extending its lockout to
// otpBanDuration from now -- best-effort: a DB error here is logged but
// never returned, since the caller (VerifyOtp) has already decided to
// treat this attempt as failed/banned regardless of whether the ban row
// itself persists.
func (a *Authentication) banIP(ipAddress string) {
	bannedUntil := time.Now().Add(otpBanDuration)
	var ban model.OtpIpBan
	err := a.db.Where("ip_address = ?", ipAddress).First(&ban).Error
	if errors.Is(err, gorm.ErrRecordNotFound) {
		if createErr := a.db.Create(&model.OtpIpBan{IPAddress: ipAddress, BannedUntil: bannedUntil}).Error; createErr != nil {
			a.logger.Warn("failed to create otp ip ban", zap.String("ip", ipAddress), zap.Error(createErr))
		}
		return
	}
	if err != nil {
		a.logger.Warn("failed to look up otp ip ban", zap.String("ip", ipAddress), zap.Error(err))
		return
	}
	if updateErr := a.db.Model(&ban).Update("banned_until", bannedUntil).Error; updateErr != nil {
		a.logger.Warn("failed to update otp ip ban", zap.String("ip", ipAddress), zap.Error(updateErr))
	}
}

func (a *Authentication) UpdateProfile(oldUsername, oldPassword string, newUsername, newPassword *string) error {
	var admin model.Admin

	if err := a.db.First(&admin, "username = ?", oldUsername).Error; err == nil {
		if err := bcrypt.CompareHashAndPassword([]byte(admin.Password), []byte(oldPassword)); err != nil {
			a.logger.Error("password mismatch", zap.String("username", oldUsername), zap.Error(err))
			return errors.New("password mismatch")
		}

		if newUsername != nil {
			admin.Username = *newUsername
		}

		if newPassword != nil {
			hashedPassword, err := bcrypt.GenerateFromPassword([]byte(*newPassword), bcrypt.DefaultCost)
			if err != nil {
				a.logger.Error("failed to hash new password", zap.Error(err))
				return err
			}
			admin.Password = string(hashedPassword)
		}

		if err := a.db.Save(&admin).Error; err != nil {
			a.logger.Error("failed to update user profile", zap.Error(err))
			return err
		}

		return nil
	} else if !errors.Is(err, gorm.ErrRecordNotFound) {
		a.logger.Error("failed to query user from database", zap.Error(err))
		return err
	}

	var reseller model.Reseller
	if err := a.db.First(&reseller, "username = ?", oldUsername).Error; err != nil {
		if errors.Is(err, gorm.ErrRecordNotFound) {
			a.logger.Error("user not found", zap.String("username", oldUsername))
			return gorm.ErrRecordNotFound
		}
		a.logger.Error("failed to query reseller from database", zap.Error(err))
		return err
	}

	if err := bcrypt.CompareHashAndPassword([]byte(reseller.PasswordHash), []byte(oldPassword)); err != nil {
		a.logger.Error("password mismatch", zap.String("username", oldUsername), zap.Error(err))
		return errors.New("password mismatch")
	}

	if newUsername != nil {
		reseller.Username = *newUsername
	}
	if newPassword != nil {
		hashedPassword, err := bcrypt.GenerateFromPassword([]byte(*newPassword), bcrypt.DefaultCost)
		if err != nil {
			a.logger.Error("failed to hash new password", zap.Error(err))
			return err
		}
		reseller.PasswordHash = string(hashedPassword)
	}

	if err := a.db.Save(&reseller).Error; err != nil {
		a.logger.Error("failed to update user profile", zap.Error(err))
		return err
	}

	return nil
}

func (a *Authentication) generateAccessToken(subject, role string, resellerID *uint) (string, error) {
	claims := jwt.MapClaims{
		"sub":  subject,
		"exp":  time.Now().Add(accessTokenTTL).Unix(),
		"role": role,
	}
	if resellerID != nil {
		claims["reseller_id"] = *resellerID
	}
	token := jwt.NewWithClaims(jwt.SigningMethodHS256, claims)
	return token.SignedString(a.AccessSecret)
}

func (a *Authentication) generateRefreshToken(subject, role string, resellerID *uint) (string, error) {
	claims := jwt.MapClaims{
		"sub":  subject,
		"exp":  time.Now().Add(refreshTokenTTL).Unix(),
		"role": role,
	}
	if resellerID != nil {
		claims["reseller_id"] = *resellerID
	}
	token := jwt.NewWithClaims(jwt.SigningMethodHS256, claims)
	return token.SignedString(a.RefreshSecret)
}

func (a *Authentication) generateTokenPair(subject, role string, resellerID *uint) (string, string, error) {
	accessToken, err := a.generateAccessToken(subject, role, resellerID)
	if err != nil {
		return "", "", err
	}

	refreshToken, err := a.generateRefreshToken(subject, role, resellerID)
	if err != nil {
		return "", "", err
	}

	return accessToken, refreshToken, nil
}
