package service

import (
	"errors"
	"fmt"
	"os"
	"path/filepath"

	"go.uber.org/zap"
	"gorm.io/gorm"

	"github.com/maahdima/mwp/api/common"
	"github.com/maahdima/mwp/api/config"
	"github.com/maahdima/mwp/api/dataservice/model"
	"github.com/maahdima/mwp/api/utils"
)

var userManagerConfigsPath string

func init() {
	appCfg := config.GetAppConfig()
	userManagerConfigsPath = filepath.Join(appCfg.PeerFilesDir, "user-manager-config")
	if err := os.MkdirAll(userManagerConfigsPath, os.ModePerm); err != nil {
		panic(fmt.Sprintf("failed to create user manager config directory: %v", err))
	}
}

// UserManagerConfigFile manages an admin-uploaded config file (typically an
// OpenVPN .ovpn profile) attached to a UserManagerAccount -- mirrors
// ConfigGenerator's on-disk storage convention exactly (<uuid>.<ext> under
// a dedicated directory), but for an admin-supplied file rather than a
// panel-generated one, since RouterOS User Manager itself has no concept of
// an OpenVPN client profile to generate from.
type UserManagerConfigFile struct {
	db     *gorm.DB
	logger *zap.Logger
}

func NewUserManagerConfigFile(db *gorm.DB) *UserManagerConfigFile {
	return &UserManagerConfigFile{
		db:     db,
		logger: zap.L().Named("UserManagerConfigFile"),
	}
}

func configFilePath(uuid string) string {
	return fmt.Sprintf("%s/%s.ovpn", userManagerConfigsPath, uuid)
}

// UploadConfig stores the given config file content for an account and
// records that it now has one. Overwrites any previously uploaded file for
// the same account. Admin only (enforced at the HTTP layer).
func (f *UserManagerConfigFile) UploadConfig(accountID uint, resellerID *uint, content []byte) error {
	account, err := f.getAccountScoped(accountID, resellerID)
	if err != nil {
		return err
	}

	if err := os.WriteFile(configFilePath(account.UUID), content, 0o600); err != nil {
		f.logger.Error("failed to write user manager config file", zap.String("uuid", account.UUID), zap.Error(err))
		return fmt.Errorf("failed to write config file: %w", err)
	}

	if err := f.db.Model(&model.UserManagerAccount{}).Where("id = ?", account.ID).Update("has_config_file", true).Error; err != nil {
		f.logger.Error("failed to record config file upload", zap.Uint("id", account.ID), zap.Error(err))
		return err
	}

	return nil
}

// GetConfigPath returns the on-disk path for an account's uploaded config
// file, for the admin/reseller-owner authenticated download endpoint.
func (f *UserManagerConfigFile) GetConfigPath(accountID uint, resellerID *uint) (string, error) {
	account, err := f.getAccountScoped(accountID, resellerID)
	if err != nil {
		return "", err
	}
	if !account.HasConfigFile {
		return "", fmt.Errorf("no config file has been uploaded for this account")
	}
	return configFilePath(account.UUID), nil
}

// GetPublicConfigPath is the public, unauthenticated variant used by the
// share page's "Download Config" button -- mirrors
// ConfigGenerator.GetUserConfig's IsShared/ShareExpireTime gate exactly.
func (f *UserManagerConfigFile) GetPublicConfigPath(uuid string) (string, error) {
	var account model.UserManagerAccount
	if err := f.db.First(&account, "uuid = ?", uuid).Error; err != nil {
		if errors.Is(err, gorm.ErrRecordNotFound) {
			f.logger.Error("user manager account not found in database", zap.String("uuid", uuid))
			return "", err
		}
		f.logger.Error("failed to find user manager account in database", zap.Error(err))
		return "", err
	}

	if !utils.IsPeerSharable(account.IsShared, account.ShareExpireTime) {
		return "", common.ErrUserManagerAccountNotShared
	}

	if !account.HasConfigFile {
		return "", fmt.Errorf("no config file has been uploaded for this account")
	}

	return configFilePath(account.UUID), nil
}

// RemoveConfig deletes an account's uploaded config file, if any -- called
// from UserManagerService.DeleteAccount's cleanup sequence. Not finding a
// file on disk is not treated as an error (nothing to clean up).
func (f *UserManagerConfigFile) RemoveConfig(uuid string) {
	if err := os.Remove(configFilePath(uuid)); err != nil && !os.IsNotExist(err) {
		f.logger.Warn("failed to remove user manager config file", zap.String("uuid", uuid), zap.Error(err))
	}
}

func (f *UserManagerConfigFile) getAccountScoped(id uint, resellerID *uint) (model.UserManagerAccount, error) {
	var account model.UserManagerAccount
	query := f.db.Where("id = ?", id)
	if resellerID != nil {
		query = query.Where("reseller_id = ?", *resellerID)
	}
	if err := query.First(&account).Error; err != nil {
		return model.UserManagerAccount{}, err
	}
	return account, nil
}
