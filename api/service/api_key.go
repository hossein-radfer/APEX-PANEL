package service

import (
	"errors"
	"time"

	"golang.org/x/crypto/bcrypt"
	"gorm.io/gorm"

	"github.com/maahdima/mwp/api/dataservice/model"
	"github.com/maahdima/mwp/api/utils"
)

// apiKeyRawLength is deliberately longer than SupportToken's 40 chars --
// this credential, unlike a support token, has no expiry to bound a brute
// force window, so it needs a larger keyspace to stay safe indefinitely.
const apiKeyRawLength = 48

// apiKeyPrefixLength is how many characters of the raw key are kept in the
// clear (ApiKey.KeyPrefix) purely for the admin UI to visually distinguish
// keys in a list -- see that field's own doc comment. Short enough to
// reveal negligible entropy about the full key, long enough to be visually
// distinct between keys.
const apiKeyPrefixLength = 8

var (
	ErrApiKeyInvalid = errors.New("invalid or revoked api key")
)

// ApiKeyService backs phase 4-3's admin-issued, JWT-independent credential
// for external panel/automation clients -- see model.ApiKey's own doc
// comment for the full rationale on why this is a separate, multi-key
// system rather than an extension of SupportToken's single-row design.
type ApiKeyService struct {
	db *gorm.DB
}

func NewApiKeyService(db *gorm.DB) *ApiKeyService {
	return &ApiKeyService{db: db}
}

// Generate creates a new, independently revocable key under the given
// label. Returns the raw key exactly once -- only its bcrypt hash and an
// 8-character prefix are ever persisted, matching SupportToken.Generate's
// same one-time-reveal contract.
func (s *ApiKeyService) Generate(label string) (rawKey string, keyID uint, err error) {
	raw := utils.RandomString(apiKeyRawLength)
	hashed, err := bcrypt.GenerateFromPassword([]byte(raw), bcrypt.DefaultCost)
	if err != nil {
		return "", 0, err
	}

	record := model.ApiKey{
		Label:     label,
		KeyHash:   string(hashed),
		KeyPrefix: raw[:apiKeyPrefixLength],
	}
	if err := s.db.Create(&record).Error; err != nil {
		return "", 0, err
	}

	return raw, record.ID, nil
}

// List returns every key, revoked or not, newest first -- the admin UI
// itself decides how to visually distinguish a revoked key rather than
// this service hiding history from it (mirrors ApiKey.Revoked's own doc
// comment on why revoked keys are kept, not deleted).
func (s *ApiKeyService) List() ([]model.ApiKey, error) {
	var keys []model.ApiKey
	if err := s.db.Order("created_at DESC").Find(&keys).Error; err != nil {
		return nil, err
	}
	return keys, nil
}

// Revoke marks a key permanently unusable -- does not delete the row (see
// model.ApiKey.Revoked's own doc comment), so it keeps showing in List for
// audit purposes.
func (s *ApiKeyService) Revoke(id uint) error {
	return s.db.Model(&model.ApiKey{}).Where("id = ?", id).Update("revoked", true).Error
}

// Verify checks a candidate raw key against every non-revoked key sharing
// its prefix, bumping LastUsedAt on success -- called by APIKeyMiddleware
// on every request to the external API surface. Filtering by KeyPrefix
// first (a plain, indexable equality check) before falling back to bcrypt's
// deliberately-slow comparison keeps this from doing a full bcrypt compare
// against every live key on every single external-API request; a prefix
// collision between two independently-generated 48-char random keys is
// astronomically unlikely, but even if one occurred, exact-match still
// re-derives correctly since every candidate sharing the prefix is compared.
func (s *ApiKeyService) Verify(candidate string) error {
	if len(candidate) < apiKeyPrefixLength {
		return ErrApiKeyInvalid
	}

	var candidates []model.ApiKey
	if err := s.db.Where("key_prefix = ? AND revoked = ?", candidate[:apiKeyPrefixLength], false).Find(&candidates).Error; err != nil {
		return err
	}

	for _, key := range candidates {
		if bcrypt.CompareHashAndPassword([]byte(key.KeyHash), []byte(candidate)) == nil {
			now := time.Now()
			// Best-effort -- a failed usage-timestamp update must never
			// block the request this key just legitimately authenticated.
			s.db.Model(&model.ApiKey{}).Where("id = ?", key.ID).Update("last_used_at", now)
			return nil
		}
	}

	return ErrApiKeyInvalid
}
