package service

import (
	"errors"
	"fmt"
	"time"

	"go.uber.org/zap"
	"gorm.io/gorm"

	"github.com/maahdima/mwp/api/dataservice/model"
)

// appMaintenanceEnabledConfigKey/appMaintenanceMessageConfigKey are
// SystemConfig rows -- mirrors TunnelHealthService's own dryRunConfigKey/
// emergencyStopConfigKey convention exactly (a plain boolean/string
// singleton doesn't warrant a dedicated table). Independent of any
// specific AppVersion row: maintenance mode can block every version,
// including the latest one, e.g. during a server migration.
const (
	appMaintenanceEnabledConfigKey = "app_maintenance_enabled"
	appMaintenanceMessageConfigKey = "app_maintenance_message"
)

// ApplicationVersionService implements the admin's own explicit "مدیریت
// اپلیکیشن" requirement: publish a new mobile-app release, optionally
// mark it mandatory (blocking every older build until the user updates),
// and independently put the whole app into maintenance mode. Read-only
// from the mobile app's own point of view (CheckVersion) -- every write
// path here is admin-only, enforced at the HTTP layer exactly like
// ApplicationOpenVpnTemplateService's own admin-only upload.
type ApplicationVersionService struct {
	db     *gorm.DB
	logger *zap.Logger
}

func NewApplicationVersionService(db *gorm.DB) *ApplicationVersionService {
	return &ApplicationVersionService{
		db:     db,
		logger: zap.L().Named("ApplicationVersionService"),
	}
}

// PublishVersion records a new app release. VersionCode is unique (see
// model.AppVersion.VersionCode's own doc comment) -- publishing the same
// code twice is treated as an update to that release's own metadata
// (e.g. correcting a typo in ReleaseNotes) rather than a duplicate-key
// error, since re-uploading the exact same build number is a normal
// admin correction, not a mistake to reject.
func (s *ApplicationVersionService) PublishVersion(versionCode int, versionName string, releaseNotes *string, downloadURL string, isMandatory bool) (*model.AppVersion, error) {
	var existing model.AppVersion
	err := s.db.Where("version_code = ?", versionCode).First(&existing).Error
	if err == nil {
		existing.VersionName = versionName
		existing.ReleaseNotes = releaseNotes
		existing.DownloadURL = downloadURL
		existing.IsMandatory = isMandatory
		existing.PublishedAt = time.Now()
		if err := s.db.Save(&existing).Error; err != nil {
			s.logger.Error("failed to update existing app version", zap.Int("version_code", versionCode), zap.Error(err))
			return nil, fmt.Errorf("failed to update app version: %w", err)
		}
		return &existing, nil
	}
	if !errors.Is(err, gorm.ErrRecordNotFound) {
		return nil, err
	}

	version := model.AppVersion{
		VersionCode:  versionCode,
		VersionName:  versionName,
		ReleaseNotes: releaseNotes,
		DownloadURL:  downloadURL,
		IsMandatory:  isMandatory,
		PublishedAt:  time.Now(),
	}
	if err := s.db.Create(&version).Error; err != nil {
		s.logger.Error("failed to create app version", zap.Int("version_code", versionCode), zap.Error(err))
		return nil, fmt.Errorf("failed to publish app version: %w", err)
	}
	return &version, nil
}

// ListVersions returns every published version, newest first -- the
// admin's own "مدیریت اپلیکیشن" page listing.
func (s *ApplicationVersionService) ListVersions() ([]model.AppVersion, error) {
	var versions []model.AppVersion
	if err := s.db.Order("version_code desc").Find(&versions).Error; err != nil {
		s.logger.Error("failed to list app versions", zap.Error(err))
		return nil, err
	}
	return versions, nil
}

// DeleteVersion removes one published version -- e.g. an admin correcting
// a mistaken publish. Deleting a version never affects an app instance
// already running it; it only stops that version from being offered as
// "the latest" or contributing to the mandatory-update chain going
// forward.
func (s *ApplicationVersionService) DeleteVersion(id uint) error {
	if err := s.db.Delete(&model.AppVersion{}, id).Error; err != nil {
		s.logger.Error("failed to delete app version", zap.Uint("id", id), zap.Error(err))
		return fmt.Errorf("failed to delete app version: %w", err)
	}
	return nil
}

// SetMaintenanceMode enables/disables the global maintenance gate --
// mirrors TunnelHealthService.SetDryRun's own setBoolConfig-backed
// pattern. message is shown to the user alongside the maintenance
// notice; nil/empty clears it back to no custom message.
func (s *ApplicationVersionService) SetMaintenanceMode(enabled bool, message *string) error {
	value := "false"
	if enabled {
		value = "true"
	}
	if err := s.upsertConfig(appMaintenanceEnabledConfigKey, value); err != nil {
		return err
	}
	messageValue := ""
	if message != nil {
		messageValue = *message
	}
	return s.upsertConfig(appMaintenanceMessageConfigKey, messageValue)
}

// GetMaintenanceMode reports the current global maintenance gate state.
func (s *ApplicationVersionService) GetMaintenanceMode() (enabled bool, message *string) {
	var enabledCfg model.SystemConfig
	if err := s.db.Where("key = ?", appMaintenanceEnabledConfigKey).First(&enabledCfg).Error; err == nil {
		enabled = enabledCfg.Value == "true"
	}
	var messageCfg model.SystemConfig
	if err := s.db.Where("key = ?", appMaintenanceMessageConfigKey).First(&messageCfg).Error; err == nil && messageCfg.Value != "" {
		message = &messageCfg.Value
	}
	return enabled, message
}

func (s *ApplicationVersionService) upsertConfig(key, value string) error {
	var cfg model.SystemConfig
	err := s.db.Where("key = ?", key).First(&cfg).Error
	if err == nil {
		cfg.Value = value
		return s.db.Save(&cfg).Error
	}
	if !errors.Is(err, gorm.ErrRecordNotFound) {
		return err
	}
	return s.db.Create(&model.SystemConfig{Key: key, Value: value}).Error
}

// CheckVersion answers the mobile app's own public GET
// /api/app/version-check?current_version=N call -- the app's very first
// network call on every launch, before login, so even a user who cannot
// currently authenticate still finds out they must update or that the
// service is under maintenance.
//
// UpdateRequired is true when EITHER: (a) no version has ever been
// published at all is never mandatory (nothing to update TO), or more
// precisely (b) currentVersionCode is lower than the version code of any
// published, mandatory release -- not just the single latest release.
// This is a deliberate choice: an admin may publish v3 as mandatory,
// then later publish v4 as NOT mandatory (a minor, optional
// improvement) -- a user still on v2 must still be forced through the
// v3 update along the way, even though the very latest release no
// longer itself carries the mandatory flag. Simplified to "is
// currentVersionCode below the highest VersionCode among all mandatory
// rows" -- exactly captures that chain without needing to walk version
// history in order.
func (s *ApplicationVersionService) CheckVersion(currentVersionCode int) (*model.AppVersion, bool, error) {
	var latest model.AppVersion
	err := s.db.Order("version_code desc").First(&latest).Error
	if err != nil {
		if errors.Is(err, gorm.ErrRecordNotFound) {
			return nil, false, nil
		}
		s.logger.Error("failed to load latest app version", zap.Error(err))
		return nil, false, err
	}

	var highestMandatoryCode int
	if err := s.db.Model(&model.AppVersion{}).
		Where("is_mandatory = ?", true).
		Select("COALESCE(MAX(version_code), 0)").
		Scan(&highestMandatoryCode).Error; err != nil {
		s.logger.Error("failed to compute highest mandatory app version", zap.Error(err))
		return nil, false, err
	}

	updateRequired := currentVersionCode < highestMandatoryCode
	return &latest, updateRequired, nil
}
