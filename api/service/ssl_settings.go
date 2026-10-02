package service

import (
	"crypto/tls"
	"errors"
	"fmt"
	"os"

	"go.uber.org/zap"
	"gorm.io/gorm"

	"github.com/maahdima/mwp/api/dataservice/model"
)

// sslSettingsSingletonID mirrors botSettingsSingletonID's own convention --
// exactly one manual-TLS configuration per panel instance.
const sslSettingsSingletonID = 1

type SSLSettingsService struct {
	db     *gorm.DB
	logger *zap.Logger
}

func NewSSLSettingsService(db *gorm.DB) *SSLSettingsService {
	return &SSLSettingsService{
		db:     db,
		logger: zap.L().Named("SSLSettingsService"),
	}
}

// GetOrCreate returns the singleton SSLSettings row, creating it disabled
// with empty paths the first time it's requested -- mirrors
// BotSettingsService.GetOrCreate exactly.
func (s *SSLSettingsService) GetOrCreate() (*model.SSLSettings, error) {
	var settings model.SSLSettings
	if err := s.db.First(&settings, sslSettingsSingletonID).Error; err != nil {
		if !errors.Is(err, gorm.ErrRecordNotFound) {
			s.logger.Error("failed to fetch ssl settings", zap.Error(err))
			return nil, err
		}

		settings = model.SSLSettings{Model: model.Model{ID: sslSettingsSingletonID}}
		if err := s.db.Create(&settings).Error; err != nil {
			s.logger.Error("failed to create default ssl settings", zap.Error(err))
			return nil, err
		}
	}

	return &settings, nil
}

// UpdateSettingsInput mirrors every admin-editable field on SSLSettings.
// Pointer fields are optional partial updates; nil means "leave unchanged" --
// same convention as BotSettingsService.UpdateSettingsInput.
type UpdateSSLSettingsInput struct {
	Domain          *string
	CertificatePath *string
	PrivateKeyPath  *string
	Enabled         *bool
}

// UpdateSettings applies a partial update. When the caller is turning
// Enabled on (either explicitly, or it was already on and either path
// changed), the resulting certificate/key pair is validated with
// tls.LoadX509KeyPair BEFORE anything is persisted -- an admin pasting a
// wrong path or mismatched cert/key finds out immediately from this call's
// error, rather than discovering it only after restarting the panel and
// finding the HTTPS listener refused to start (see
// LoadTLSConfig's own doc comment for where that second, load-bearing
// check happens at actual startup time).
func (s *SSLSettingsService) UpdateSettings(input UpdateSSLSettingsInput) (*model.SSLSettings, error) {
	current, err := s.GetOrCreate()
	if err != nil {
		return nil, err
	}

	updates := map[string]interface{}{}
	if input.Domain != nil {
		updates["domain"] = *input.Domain
	}
	if input.CertificatePath != nil {
		updates["certificate_path"] = *input.CertificatePath
	}
	if input.PrivateKeyPath != nil {
		updates["private_key_path"] = *input.PrivateKeyPath
	}
	if input.Enabled != nil {
		updates["enabled"] = *input.Enabled
	}

	willBeEnabled := current.Enabled
	if input.Enabled != nil {
		willBeEnabled = *input.Enabled
	}
	certPath := current.CertificatePath
	if input.CertificatePath != nil {
		certPath = *input.CertificatePath
	}
	keyPath := current.PrivateKeyPath
	if input.PrivateKeyPath != nil {
		keyPath = *input.PrivateKeyPath
	}

	if willBeEnabled {
		if certPath == "" || keyPath == "" {
			return nil, errors.New("برای فعال‌سازی SSL باید مسیر گواهی و کلید خصوصی هر دو مشخص شوند")
		}
		if _, err := tls.LoadX509KeyPair(certPath, keyPath); err != nil {
			return nil, fmt.Errorf("گواهی/کلید معتبر نیست یا در مسیر داده‌شده یافت نشد: %w", err)
		}
	}

	if len(updates) == 0 {
		return current, nil
	}

	if err := s.db.Model(&model.SSLSettings{}).Where("id = ?", sslSettingsSingletonID).Updates(updates).Error; err != nil {
		s.logger.Error("failed to update ssl settings", zap.Error(err))
		return nil, err
	}

	return s.GetOrCreate()
}

// LoadTLSConfig is called once at process startup (see
// cmd/http-server/http-server.go) -- the actual, load-bearing check that
// decides whether the HTTP server binds with TLS at all. Deliberately
// re-validates the files from disk rather than trusting
// UpdateSettings' earlier check: the files on disk can change independently
// of this panel (Certbot's own renewal timer overwrites them in place
// roughly every 60 days), so a startup that's happening long after the
// admin last touched this settings page must still confirm the CURRENT
// on-disk files are loadable, not rely on a check performed weeks earlier.
// Returns (nil, nil) if SSL is not enabled -- the caller falls back to
// plain HTTP in that case, exactly as before this feature existed.
func (s *SSLSettingsService) LoadTLSConfig() (*tls.Config, error) {
	settings, err := s.GetOrCreate()
	if err != nil {
		return nil, err
	}
	if !settings.Enabled {
		return nil, nil
	}

	if _, err := os.Stat(settings.CertificatePath); err != nil {
		return nil, fmt.Errorf("فایل گواهی SSL در مسیر %s یافت نشد: %w", settings.CertificatePath, err)
	}
	if _, err := os.Stat(settings.PrivateKeyPath); err != nil {
		return nil, fmt.Errorf("فایل کلید خصوصی SSL در مسیر %s یافت نشد: %w", settings.PrivateKeyPath, err)
	}

	cert, err := tls.LoadX509KeyPair(settings.CertificatePath, settings.PrivateKeyPath)
	if err != nil {
		return nil, fmt.Errorf("بارگذاری گواهی/کلید SSL ناموفق بود: %w", err)
	}

	return &tls.Config{
		Certificates: []tls.Certificate{cert},
	}, nil
}
