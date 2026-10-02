package service

import (
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"time"

	"go.uber.org/zap"
	"gorm.io/gorm"

	"github.com/maahdima/mwp/api/config"
	"github.com/maahdima/mwp/api/dataservice/model"
)

var applicationOpenVpnTemplatePath string

func init() {
	appCfg := config.GetAppConfig()
	dir := filepath.Join(appCfg.PeerFilesDir, "application-openvpn-template")
	if err := os.MkdirAll(dir, os.ModePerm); err != nil {
		panic(fmt.Sprintf("failed to create application openvpn template directory: %v", err))
	}
	applicationOpenVpnTemplatePath = filepath.Join(dir, "template.ovpn")
}

// ApplicationOpenVpnTemplateService manages the ONE global .ovpn template
// every Application's OpenVPN connections are built from -- see
// model.ApplicationOpenVpnTemplate's own doc comment for why this is a
// singleton (one physical OpenVPN server, so server/port/CA-certificate
// are identical for every Application; only the per-group username/
// password differ, and those already come from provisionUserManagerGroup).
type ApplicationOpenVpnTemplateService struct {
	db     *gorm.DB
	logger *zap.Logger
}

func NewApplicationOpenVpnTemplateService(db *gorm.DB) *ApplicationOpenVpnTemplateService {
	return &ApplicationOpenVpnTemplateService{
		db:     db,
		logger: zap.L().Named("ApplicationOpenVpnTemplateService"),
	}
}

// UploadTemplate stores the admin-supplied .ovpn template content,
// overwriting any previously uploaded one -- admin only (enforced at the
// HTTP layer). The template must NOT itself already contain an
// <auth-user-pass> block or a bare "auth-user-pass" directive with an
// inline username/password (those would take priority over the
// per-Application block CombineWithCredentials appends below); a
// template using "auth-user-pass" with no inline value (prompting the
// client for creds) is exactly the expected shape and is left untouched.
func (s *ApplicationOpenVpnTemplateService) UploadTemplate(content []byte) error {
	if err := os.WriteFile(applicationOpenVpnTemplatePath, content, 0o600); err != nil {
		s.logger.Error("failed to write application openvpn template file", zap.Error(err))
		return fmt.Errorf("failed to write template file: %w", err)
	}

	var existing model.ApplicationOpenVpnTemplate
	err := s.db.First(&existing).Error
	if err == nil {
		return s.db.Model(&existing).Update("uploaded_at", time.Now()).Error
	}
	if err != gorm.ErrRecordNotFound {
		return err
	}
	return s.db.Create(&model.ApplicationOpenVpnTemplate{UploadedAt: time.Now()}).Error
}

// GetTemplateStatus reports whether a template exists and when it was
// last uploaded, for the admin settings page -- never returns the raw
// content (that's DownloadTemplate's job, admin-only, for verifying what
// was actually uploaded).
func (s *ApplicationOpenVpnTemplateService) GetTemplateStatus() (exists bool, uploadedAt *time.Time, err error) {
	var row model.ApplicationOpenVpnTemplate
	dbErr := s.db.First(&row).Error
	if dbErr == gorm.ErrRecordNotFound {
		return false, nil, nil
	}
	if dbErr != nil {
		return false, nil, dbErr
	}
	return true, &row.UploadedAt, nil
}

// GetTemplatePath returns the on-disk path of the uploaded template, for
// the admin-authenticated download endpoint.
func (s *ApplicationOpenVpnTemplateService) GetTemplatePath() (string, error) {
	if _, err := os.Stat(applicationOpenVpnTemplatePath); err != nil {
		return "", fmt.Errorf("no openvpn template has been uploaded")
	}
	return applicationOpenVpnTemplatePath, nil
}

// BuildConfigForAccount combines the global template with one
// UserManagerAccount's own username/password into a complete,
// self-contained .ovpn file the mobile app can hand directly to
// ics-openvpn -- returns nil (not an error) if no template has been
// uploaded yet, matching AppConnectUserManagerAccount.OpenVpnConfig's
// own documented "nil means not available yet" contract, since a
// missing template is an expected, normal state before the admin's
// first upload, not a failure.
func (s *ApplicationOpenVpnTemplateService) BuildConfigForAccount(username, password string) (*string, error) {
	content, err := os.ReadFile(applicationOpenVpnTemplatePath)
	if err != nil {
		if os.IsNotExist(err) {
			return nil, nil
		}
		s.logger.Error("failed to read application openvpn template file", zap.Error(err))
		return nil, fmt.Errorf("failed to read template file: %w", err)
	}

	// Inline <auth-user-pass> block: the standard OpenVPN client-config
	// mechanism for embedding credentials directly in a single .ovpn
	// file rather than a separate auth file -- ics-openvpn (and every
	// mainstream OpenVPN client) reads this block automatically, no
	// separate username/password prompt or file needed.
	combined := strings.TrimRight(string(content), "\n") + fmt.Sprintf(
		"\n<auth-user-pass>\n%s\n%s\n</auth-user-pass>\n",
		username, password,
	)
	return &combined, nil
}
