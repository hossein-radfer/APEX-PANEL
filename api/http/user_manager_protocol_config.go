package http

import (
	"io"
	"net/http"

	"github.com/maahdima/mwp/api/dataservice/model"
	"github.com/maahdima/mwp/api/http/schema"
	"github.com/maahdima/mwp/api/service"

	"github.com/labstack/echo/v4"
	"go.uber.org/zap"
)

// UserManagerProtocolConfigController manages the admin-only, per-protocol
// port/settings table used to build correct share links -- never touches
// RouterOS.
type UserManagerProtocolConfigController struct {
	accountService *service.UserManagerService
	protocolFile   *service.UserManagerProtocolFile
	logger         *zap.Logger
}

func NewUserManagerProtocolConfigController(accountService *service.UserManagerService, protocolFile *service.UserManagerProtocolFile) *UserManagerProtocolConfigController {
	return &UserManagerProtocolConfigController{
		accountService: accountService,
		protocolFile:   protocolFile,
		logger:         zap.L().Named("UserManagerProtocolConfigController"),
	}
}

// ListProtocolConfigs is readable by any authenticated role (admin or
// reseller), needed so the account-creation UI/share links can display the
// configured port.
func (c *UserManagerProtocolConfigController) ListProtocolConfigs(ctx echo.Context) error {
	configs, err := c.accountService.GetProtocolConfigs()
	if err != nil {
		c.logger.Error("failed to list user manager protocol configs", zap.Error(err))
		return ctx.JSON(http.StatusInternalServerError, schema.ErrorResponse{
			StatusCode: http.StatusInternalServerError,
			Status:     "error",
			Message:    "failed to retrieve protocol configs: " + err.Error(),
		})
	}

	return ctx.JSON(http.StatusOK, schema.BasicResponseData[[]schema.UserManagerProtocolConfigResponse]{
		BasicResponse: schema.OkBasicResponse,
		Data:          configs,
	})
}

// UpsertProtocolConfig is admin-only.
func (c *UserManagerProtocolConfigController) UpsertProtocolConfig(ctx echo.Context) error {
	if role, _ := getRoleAndResellerFromContext(ctx); role != "admin" {
		return ctx.JSON(http.StatusForbidden, schema.ErrorResponse{StatusCode: http.StatusForbidden, Status: "error", Message: "forbidden"})
	}

	var req schema.UpsertUserManagerProtocolConfigRequest
	if err := ctx.Bind(&req); err != nil {
		c.logger.Warn("failed to bind request", zap.Error(err))
		return ctx.JSON(http.StatusBadRequest, schema.BadParamsErrorResponse)
	}
	if err := ctx.Validate(&req); err != nil {
		c.logger.Warn("failed to validate request", zap.Error(err))
		return ctx.JSON(http.StatusBadRequest, schema.BadParamsErrorResponse)
	}

	config, err := c.accountService.UpsertProtocolConfig(&req)
	if err != nil {
		c.logger.Error("failed to upsert user manager protocol config", zap.Error(err))
		return ctx.JSON(http.StatusInternalServerError, schema.ErrorResponse{
			StatusCode: http.StatusInternalServerError,
			Status:     "error",
			Message:    "failed to save protocol config: " + err.Error(),
		})
	}

	return ctx.JSON(http.StatusOK, schema.BasicResponseData[schema.UserManagerProtocolConfigResponse]{
		BasicResponse: schema.OkBasicResponse,
		Data:          *config,
	})
}

// validateProtocolParam checks the ":protocol" path param against the
// closed set of supported protocols, returning it typed if valid.
func validateProtocolParam(ctx echo.Context) (model.UserManagerAccountProtocol, error) {
	protocol := model.UserManagerAccountProtocol(ctx.Param("protocol"))
	switch protocol {
	case model.ProtocolL2TP, model.ProtocolPPTP, model.ProtocolSSTP, model.ProtocolOpenVPN, model.ProtocolIKEv2:
		return protocol, nil
	default:
		return "", ctx.JSON(http.StatusBadRequest, schema.BadParamsErrorResponse)
	}
}

// UploadProtocolCertificateFile stores an admin-supplied reference/quality
// file (e.g. a certificate export) for a protocol. Admin only.
func (c *UserManagerProtocolConfigController) UploadProtocolCertificateFile(ctx echo.Context) error {
	return c.uploadProtocolFile(ctx, service.ProtocolFileKindCertificate)
}

// UploadProtocolClientAppFile stores an admin-supplied client app
// installer for a protocol, so end-users can download and install the
// software needed to connect. Admin only.
func (c *UserManagerProtocolConfigController) UploadProtocolClientAppFile(ctx echo.Context) error {
	return c.uploadProtocolFile(ctx, service.ProtocolFileKindClientApp)
}

func (c *UserManagerProtocolConfigController) uploadProtocolFile(ctx echo.Context, kind service.UserManagerProtocolFileKind) error {
	if role, _ := getRoleAndResellerFromContext(ctx); role != "admin" {
		return ctx.JSON(http.StatusForbidden, schema.ErrorResponse{StatusCode: http.StatusForbidden, Status: "error", Message: "forbidden"})
	}

	protocol, err := validateProtocolParam(ctx)
	if err != nil {
		return err
	}

	fileHeader, err := ctx.FormFile("file")
	if err != nil {
		return ctx.JSON(http.StatusBadRequest, schema.ErrorResponse{
			StatusCode: http.StatusBadRequest,
			Status:     "error",
			Message:    "missing 'file' in upload",
		})
	}

	// Client app installers can be large (tens of MB); a generous but
	// bounded cap guards against a runaway/malicious upload.
	const maxProtocolFileUploadSize = 200 << 20 // 200 MiB
	if fileHeader.Size > maxProtocolFileUploadSize {
		return ctx.JSON(http.StatusBadRequest, schema.ErrorResponse{
			StatusCode: http.StatusBadRequest,
			Status:     "error",
			Message:    "uploaded file exceeds the maximum allowed size",
		})
	}

	src, err := fileHeader.Open()
	if err != nil {
		c.logger.Error("failed to open uploaded protocol file", zap.Error(err))
		return ctx.JSON(http.StatusInternalServerError, schema.InternalServerErrorResponse)
	}
	defer src.Close()

	content, err := io.ReadAll(src)
	if err != nil {
		c.logger.Error("failed to read uploaded protocol file", zap.Error(err))
		return ctx.JSON(http.StatusInternalServerError, schema.InternalServerErrorResponse)
	}

	if err := c.protocolFile.UploadFile(protocol, kind, fileHeader.Filename, content); err != nil {
		c.logger.Error("failed to upload protocol file", zap.String("protocol", string(protocol)), zap.String("kind", string(kind)), zap.Error(err))
		return ctx.JSON(http.StatusInternalServerError, schema.ErrorResponse{
			StatusCode: http.StatusInternalServerError,
			Status:     "error",
			Message:    "failed to upload file: " + err.Error(),
		})
	}

	return ctx.JSON(http.StatusOK, schema.OkBasicResponse)
}

// DownloadProtocolCertificateFile lets an authenticated admin download the
// certificate/quality file previously uploaded for a protocol.
func (c *UserManagerProtocolConfigController) DownloadProtocolCertificateFile(ctx echo.Context) error {
	return c.downloadProtocolFile(ctx, service.ProtocolFileKindCertificate)
}

// DownloadProtocolClientAppFile lets an authenticated admin download the
// client app installer previously uploaded for a protocol.
func (c *UserManagerProtocolConfigController) DownloadProtocolClientAppFile(ctx echo.Context) error {
	return c.downloadProtocolFile(ctx, service.ProtocolFileKindClientApp)
}

func (c *UserManagerProtocolConfigController) downloadProtocolFile(ctx echo.Context, kind service.UserManagerProtocolFileKind) error {
	protocol, err := validateProtocolParam(ctx)
	if err != nil {
		return err
	}

	path, err := c.protocolFile.GetFilePath(protocol, kind)
	if err != nil {
		return ctx.JSON(http.StatusNotFound, schema.ErrorResponse{
			StatusCode: http.StatusNotFound,
			Status:     "error",
			Message:    err.Error(),
		})
	}

	return attachFile(ctx, path, service.DownloadFilename(protocol, kind, path))
}

// DeleteProtocolCertificateFile lets an authenticated admin remove the
// certificate/quality file previously uploaded for a protocol.
func (c *UserManagerProtocolConfigController) DeleteProtocolCertificateFile(ctx echo.Context) error {
	return c.deleteProtocolFile(ctx, service.ProtocolFileKindCertificate)
}

// DeleteProtocolClientAppFile lets an authenticated admin remove the
// client app installer previously uploaded for a protocol.
func (c *UserManagerProtocolConfigController) DeleteProtocolClientAppFile(ctx echo.Context) error {
	return c.deleteProtocolFile(ctx, service.ProtocolFileKindClientApp)
}

func (c *UserManagerProtocolConfigController) deleteProtocolFile(ctx echo.Context, kind service.UserManagerProtocolFileKind) error {
	if role, _ := getRoleAndResellerFromContext(ctx); role != "admin" {
		return ctx.JSON(http.StatusForbidden, schema.ErrorResponse{StatusCode: http.StatusForbidden, Status: "error", Message: "forbidden"})
	}

	protocol, err := validateProtocolParam(ctx)
	if err != nil {
		return err
	}

	if err := c.protocolFile.DeleteFile(protocol, kind); err != nil {
		c.logger.Error("failed to delete protocol file", zap.String("protocol", string(protocol)), zap.String("kind", string(kind)), zap.Error(err))
		return ctx.JSON(http.StatusNotFound, schema.ErrorResponse{
			StatusCode: http.StatusNotFound,
			Status:     "error",
			Message:    err.Error(),
		})
	}

	return ctx.JSON(http.StatusOK, schema.OkBasicResponse)
}
