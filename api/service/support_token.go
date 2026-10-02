package service

import (
	"errors"
	"time"

	"golang.org/x/crypto/bcrypt"
	"gorm.io/gorm"

	"github.com/maahdima/mwp/api/dataservice/model"
	"github.com/maahdima/mwp/api/utils"
)

// supportTokenTTL is fixed (not admin-configurable) per this feature's
// design: a support token must always auto-expire, so a customer who
// generates one and forgets about it never leaves a standing, unbounded
// window of remote log access open.
const supportTokenTTL = 24 * time.Hour

var (
	ErrSupportTokenNotConfigured = errors.New("no support token has been generated")
	ErrSupportTokenExpired       = errors.New("support token has expired")
	ErrSupportTokenInvalid       = errors.New("invalid support token")
)

// SupportTokenService backs the "Generate Support Token" button (customer-
// facing, in the panel's own settings) and the token verification the
// public, unauthenticated log-access endpoint performs on every request.
// See model.SupportToken's doc comment for the full privacy rationale --
// this is deliberately the ONLY thing a support token can ever grant
// access to (read/filter this install's own log file), with no path to
// a shell, the database, or any other panel data.
type SupportTokenService struct {
	db *gorm.DB
}

func NewSupportTokenService(db *gorm.DB) *SupportTokenService {
	return &SupportTokenService{db: db}
}

// Generate creates a brand-new token, overwriting (and immediately
// invalidating) any previous one -- see model.SupportToken's single-row
// semantics. Returns the raw token exactly once; only its bcrypt hash is
// ever persisted, so this is the only moment the plaintext value exists
// outside the customer's own clipboard.
func (s *SupportTokenService) Generate() (token string, expiresAt time.Time, err error) {
	raw := utils.RandomString(40)
	hashed, err := bcrypt.GenerateFromPassword([]byte(raw), bcrypt.DefaultCost)
	if err != nil {
		return "", time.Time{}, err
	}

	expiresAt = time.Now().Add(supportTokenTTL)

	var existing model.SupportToken
	err = s.db.First(&existing).Error
	switch {
	case err == nil:
		existing.TokenHash = string(hashed)
		existing.ExpiresAt = expiresAt
		if saveErr := s.db.Save(&existing).Error; saveErr != nil {
			return "", time.Time{}, saveErr
		}
	case errors.Is(err, gorm.ErrRecordNotFound):
		record := model.SupportToken{TokenHash: string(hashed), ExpiresAt: expiresAt}
		if createErr := s.db.Create(&record).Error; createErr != nil {
			return "", time.Time{}, createErr
		}
	default:
		return "", time.Time{}, err
	}

	return raw, expiresAt, nil
}

// Revoke deletes the current token immediately, if one exists -- lets a
// customer close the access window early instead of waiting out the 24h
// expiry.
func (s *SupportTokenService) Revoke() error {
	return s.db.Where("1 = 1").Delete(&model.SupportToken{}).Error
}

// Status reports whether a token is currently active and, if so, when it
// expires -- powers the settings page's "a token is active, expires at…"
// display without ever re-exposing the token value itself (which is
// unrecoverable once generated, matching a password's one-way hash).
func (s *SupportTokenService) Status() (active bool, expiresAt time.Time, err error) {
	var record model.SupportToken
	err = s.db.First(&record).Error
	if errors.Is(err, gorm.ErrRecordNotFound) {
		return false, time.Time{}, nil
	}
	if err != nil {
		return false, time.Time{}, err
	}
	if time.Now().After(record.ExpiresAt) {
		return false, record.ExpiresAt, nil
	}
	return true, record.ExpiresAt, nil
}

// Verify checks a candidate token against the currently stored one,
// enforcing expiry -- called on every request to the public log-access
// endpoint. An expired token is treated identically to "no token
// configured" from the caller's perspective (both return
// ErrSupportTokenExpired/ErrSupportTokenNotConfigured, both simply deny
// access) but the distinct errors let the caller log which case occurred.
func (s *SupportTokenService) Verify(candidate string) error {
	var record model.SupportToken
	err := s.db.First(&record).Error
	if errors.Is(err, gorm.ErrRecordNotFound) {
		return ErrSupportTokenNotConfigured
	}
	if err != nil {
		return err
	}

	if time.Now().After(record.ExpiresAt) {
		return ErrSupportTokenExpired
	}

	if bcrypt.CompareHashAndPassword([]byte(record.TokenHash), []byte(candidate)) != nil {
		return ErrSupportTokenInvalid
	}

	return nil
}
