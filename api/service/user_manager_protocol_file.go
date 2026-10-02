package service

import (
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"strings"

	"go.uber.org/zap"
	"gorm.io/gorm"

	"github.com/maahdima/mwp/api/config"
	"github.com/maahdima/mwp/api/dataservice/model"
)

var userManagerProtocolFilesPath string

func init() {
	appCfg := config.GetAppConfig()
	userManagerProtocolFilesPath = filepath.Join(appCfg.PeerFilesDir, "user-manager-protocol-files")
	if err := os.MkdirAll(userManagerProtocolFilesPath, os.ModePerm); err != nil {
		panic(fmt.Sprintf("failed to create user manager protocol files directory: %v", err))
	}
}

// UserManagerProtocolFileKind distinguishes the two admin-uploadable files
// per protocol: a reference/quality certificate file, and a client app
// installer end-users download to actually connect.
type UserManagerProtocolFileKind string

const (
	ProtocolFileKindCertificate UserManagerProtocolFileKind = "certificate"
	ProtocolFileKindClientApp   UserManagerProtocolFileKind = "client-app"
)

// UserManagerProtocolFile manages the two admin-uploaded, per-protocol
// files (not per-account, unlike UserManagerConfigFile): a
// certificate/quality reference file, and a client app installer. Both are
// shared across every account of that protocol -- one upload serves every
// customer, mirroring how UserManagerProtocolConfig's port/server settings
// are also one row per protocol, not per account.
//
// Filenames are stored on disk as "<protocol>-<kind><ext>", where <ext> is
// preserved from the admin's original upload (client apps can be .exe,
// .apk, .zip, etc. -- unlike UserManagerConfigFile's fixed .ovpn
// extension, this file's type isn't known ahead of time).
type UserManagerProtocolFile struct {
	db     *gorm.DB
	logger *zap.Logger
}

func NewUserManagerProtocolFile(db *gorm.DB) *UserManagerProtocolFile {
	return &UserManagerProtocolFile{
		db:     db,
		logger: zap.L().Named("UserManagerProtocolFile"),
	}
}

func protocolFilePathPattern(protocol model.UserManagerAccountProtocol, kind UserManagerProtocolFileKind) string {
	return filepath.Join(userManagerProtocolFilesPath, fmt.Sprintf("%s-%s.*", protocol, kind))
}

func protocolFilePath(protocol model.UserManagerAccountProtocol, kind UserManagerProtocolFileKind, ext string) string {
	return filepath.Join(userManagerProtocolFilesPath, fmt.Sprintf("%s-%s%s", protocol, kind, ext))
}

// findExistingProtocolFile globs for whatever extension the previously
// uploaded file has, since UploadFile below doesn't know it ahead of time.
func findExistingProtocolFile(protocol model.UserManagerAccountProtocol, kind UserManagerProtocolFileKind) (string, error) {
	matches, err := filepath.Glob(protocolFilePathPattern(protocol, kind))
	if err != nil {
		return "", err
	}
	if len(matches) == 0 {
		return "", os.ErrNotExist
	}
	return matches[0], nil
}

func (f *UserManagerProtocolFile) columnFor(kind UserManagerProtocolFileKind) string {
	if kind == ProtocolFileKindCertificate {
		return "has_certificate_file"
	}
	return "has_client_app_file"
}

// UploadFile stores the given file content for a protocol+kind pair and
// records that it now has one, replacing any previous upload (including
// one with a different extension). Admin only (enforced at the HTTP
// layer).
func (f *UserManagerProtocolFile) UploadFile(protocol model.UserManagerAccountProtocol, kind UserManagerProtocolFileKind, originalFilename string, content []byte) error {
	// Remove any previously uploaded file for this protocol+kind first --
	// the extension may differ from this upload's, so a plain overwrite
	// by path isn't enough to avoid leaving a stale file behind.
	if existing, err := findExistingProtocolFile(protocol, kind); err == nil {
		_ = os.Remove(existing)
	}

	ext := filepath.Ext(originalFilename)
	if ext == "" {
		ext = ".bin"
	}

	if err := os.WriteFile(protocolFilePath(protocol, kind, ext), content, 0o600); err != nil {
		f.logger.Error("failed to write user manager protocol file", zap.String("protocol", string(protocol)), zap.String("kind", string(kind)), zap.Error(err))
		return fmt.Errorf("failed to write file: %w", err)
	}

	var config model.UserManagerProtocolConfig
	err := f.db.Where("protocol = ?", protocol).First(&config).Error
	if errors.Is(err, gorm.ErrRecordNotFound) {
		config = model.UserManagerProtocolConfig{Protocol: protocol, Port: 0}
		if kind == ProtocolFileKindCertificate {
			config.HasCertificateFile = true
		} else {
			config.HasClientAppFile = true
		}
		if err := f.db.Create(&config).Error; err != nil {
			f.logger.Error("failed to create protocol config row for file upload", zap.Error(err))
			return err
		}
		return nil
	}
	if err != nil {
		f.logger.Error("failed to look up protocol config for file upload", zap.Error(err))
		return err
	}

	if err := f.db.Model(&model.UserManagerProtocolConfig{}).Where("id = ?", config.ID).Update(f.columnFor(kind), true).Error; err != nil {
		f.logger.Error("failed to record protocol file upload", zap.Uint("id", config.ID), zap.Error(err))
		return err
	}

	return nil
}

// GetFilePath returns the on-disk path for a protocol+kind pair, for the
// admin's authenticated download endpoint.
func (f *UserManagerProtocolFile) GetFilePath(protocol model.UserManagerAccountProtocol, kind UserManagerProtocolFileKind) (string, error) {
	path, err := findExistingProtocolFile(protocol, kind)
	if err != nil {
		return "", fmt.Errorf("no %s file has been uploaded for %s", kind, protocol)
	}
	return path, nil
}

// GetPublicFilePath is the public, unauthenticated variant used by the
// share page's protocol-app/certificate download buttons -- unlike account
// config files, these carry no per-account share gate: they're the same
// file for every customer of that protocol, so anyone who already has a
// valid share link (i.e. already knows their own account's connection
// info) may also fetch the protocol-wide file.
func (f *UserManagerProtocolFile) GetPublicFilePath(protocol model.UserManagerAccountProtocol, kind UserManagerProtocolFileKind) (string, error) {
	return f.GetFilePath(protocol, kind)
}

// DeleteFile removes a previously uploaded protocol file and clears its
// has_*_file flag. A confirmed, reported gap: the admin UI could upload
// and download these files but had no way to remove one (e.g. to replace
// it with nothing, or after uploading the wrong file) -- the only escape
// hatch was uploading a new file to overwrite it, or editing the DB/disk
// by hand. Admin only (enforced at the HTTP layer).
func (f *UserManagerProtocolFile) DeleteFile(protocol model.UserManagerAccountProtocol, kind UserManagerProtocolFileKind) error {
	existing, err := findExistingProtocolFile(protocol, kind)
	if err != nil {
		return fmt.Errorf("no %s file has been uploaded for %s", kind, protocol)
	}

	if err := os.Remove(existing); err != nil {
		f.logger.Error("failed to remove user manager protocol file", zap.String("protocol", string(protocol)), zap.String("kind", string(kind)), zap.Error(err))
		return fmt.Errorf("failed to delete file: %w", err)
	}

	var config model.UserManagerProtocolConfig
	if err := f.db.Where("protocol = ?", protocol).First(&config).Error; err != nil {
		if errors.Is(err, gorm.ErrRecordNotFound) {
			// The file existed on disk but the config row doesn't -- the
			// file itself is already gone at this point, nothing left to
			// clear a flag on.
			return nil
		}
		f.logger.Error("failed to look up protocol config for file delete", zap.Error(err))
		return err
	}

	if err := f.db.Model(&model.UserManagerProtocolConfig{}).Where("id = ?", config.ID).Update(f.columnFor(kind), false).Error; err != nil {
		f.logger.Error("failed to record protocol file delete", zap.Uint("id", config.ID), zap.Error(err))
		return err
	}

	return nil
}

// DownloadFilename returns a customer-facing filename for the Content-
// Disposition header, e.g. "openvpn-client-app.exe".
func DownloadFilename(protocol model.UserManagerAccountProtocol, kind UserManagerProtocolFileKind, path string) string {
	ext := filepath.Ext(path)
	label := strings.ReplaceAll(string(kind), "-", "-")
	return fmt.Sprintf("%s-%s%s", protocol, label, ext)
}
