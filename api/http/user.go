package http

import (
	"errors"
	"fmt"
	"net/http"

	"github.com/labstack/echo/v4"
	"go.uber.org/zap"
	"gorm.io/gorm"

	"github.com/maahdima/mwp/api/common"
	"github.com/maahdima/mwp/api/http/schema"
	"github.com/maahdima/mwp/api/service"
)

type UserController struct {
	peerService             *service.WgPeer
	peerConfigService       *service.ConfigGenerator
	peerQrCodeService       *service.QRCodeGenerator
	userManagerService      *service.UserManagerService
	userManagerConfigFile   *service.UserManagerConfigFile
	userManagerProtocolFile *service.UserManagerProtocolFile
	logger                  *zap.Logger
}

func NewUserController(peerService *service.WgPeer, peerConfigService *service.ConfigGenerator, qrCodeService *service.QRCodeGenerator, userManagerService *service.UserManagerService, userManagerConfigFile *service.UserManagerConfigFile, userManagerProtocolFile *service.UserManagerProtocolFile) *UserController {
	return &UserController{
		peerService:             peerService,
		peerConfigService:       peerConfigService,
		peerQrCodeService:       qrCodeService,
		userManagerService:      userManagerService,
		userManagerConfigFile:   userManagerConfigFile,
		userManagerProtocolFile: userManagerProtocolFile,
		logger:                  zap.L().Named("UserController"),
	}
}

func (u *UserController) GetUserDetails(ctx echo.Context) error {
	uuid := ctx.Param("uuid")
	if uuid == "" {
		u.logger.Error("Peer uuid is required")
		return ctx.JSON(http.StatusBadRequest, schema.BadParamsErrorResponse)
	}

	stats, err := u.peerService.GetPeerDetails(uuid)
	if err != nil {
		if errors.Is(err, gorm.ErrRecordNotFound) || errors.Is(err, common.ErrPeerNotShared) {
			return ctx.JSON(http.StatusNotFound, schema.ErrorResponse{
				StatusCode: http.StatusNotFound,
				Status:     "error",
				Message:    "peer not found",
			})
		}

		return ctx.JSON(http.StatusInternalServerError, schema.ErrorResponse{
			StatusCode: http.StatusInternalServerError,
			Status:     "error",
			Message:    "failed to retrieve peer stats: " + err.Error(),
		})
	}

	return ctx.JSON(http.StatusOK, schema.BasicResponseData[schema.PeerDetailsResponse]{
		BasicResponse: schema.OkBasicResponse,
		Data:          *stats,
	})
}

func (u *UserController) GetUserConfig(ctx echo.Context) error {
	uuid := ctx.Param("uuid")
	if uuid == "" {
		u.logger.Error("Peer uuid is required")
		return ctx.JSON(http.StatusBadRequest, schema.BadParamsErrorResponse)
	}

	config, err := u.peerConfigService.GetUserConfig(uuid)
	if err != nil {
		if errors.Is(err, gorm.ErrRecordNotFound) || errors.Is(err, common.ErrPeerNotShared) {
			return ctx.JSON(http.StatusNotFound, schema.ErrorResponse{
				StatusCode: http.StatusNotFound,
				Status:     "error",
				Message:    "peer not found",
			})
		}

		return ctx.JSON(http.StatusInternalServerError, schema.ErrorResponse{
			StatusCode: http.StatusInternalServerError,
			Status:     "error",
			Message:    "failed to retrieve peer config: " + err.Error(),
		})
	}

	return attachFile(ctx, config, fmt.Sprintf("peer-%s.conf", uuid))
}

func (u *UserController) GetUserQRCode(ctx echo.Context) error {
	uuid := ctx.Param("uuid")
	if uuid == "" {
		u.logger.Error("Peer uuid is required")
		return ctx.JSON(http.StatusBadRequest, schema.BadParamsErrorResponse)
	}

	qrCode, err := u.peerQrCodeService.GetUserQRCode(uuid)
	if err != nil {
		if errors.Is(err, gorm.ErrRecordNotFound) || errors.Is(err, common.ErrPeerNotShared) {
			return ctx.JSON(http.StatusNotFound, schema.ErrorResponse{
				StatusCode: http.StatusNotFound,
				Status:     "error",
				Message:    "peer not found",
			})
		}

		return ctx.JSON(http.StatusInternalServerError, schema.ErrorResponse{
			StatusCode: http.StatusInternalServerError,
			Status:     "error",
			Message:    "failed to retrieve peer QR code: " + err.Error(),
		})
	}

	return ctx.File(qrCode)
}

// GetUserManagerAccountShareDetails is the public, unauthenticated
// connection-info lookup for the User Manager share page -- reuses this
// same public surface the WireGuard share endpoints above already use,
// rather than a new parallel public route group.
func (u *UserController) GetUserManagerAccountShareDetails(ctx echo.Context) error {
	uuid := ctx.Param("uuid")
	if uuid == "" {
		u.logger.Error("User manager account uuid is required")
		return ctx.JSON(http.StatusBadRequest, schema.BadParamsErrorResponse)
	}

	details, err := u.userManagerService.GetAccountShareDetails(uuid)
	if err != nil {
		if errors.Is(err, gorm.ErrRecordNotFound) || errors.Is(err, common.ErrUserManagerAccountNotShared) {
			return ctx.JSON(http.StatusNotFound, schema.ErrorResponse{
				StatusCode: http.StatusNotFound,
				Status:     "error",
				Message:    "user manager account not found",
			})
		}

		return ctx.JSON(http.StatusInternalServerError, schema.ErrorResponse{
			StatusCode: http.StatusInternalServerError,
			Status:     "error",
			Message:    "failed to retrieve user manager account details: " + err.Error(),
		})
	}

	return ctx.JSON(http.StatusOK, schema.BasicResponseData[schema.UserManagerAccountShareDetailsResponse]{
		BasicResponse: schema.OkBasicResponse,
		Data:          *details,
	})
}

// GetUserManagerAccountConfig is the public, unauthenticated config-file
// download for the User Manager share page's "Download Config" button --
// same IsShared/ShareExpireTime gate as GetUserManagerAccountShareDetails,
// via UserManagerConfigFile.GetPublicConfigPath.
func (u *UserController) GetUserManagerAccountConfig(ctx echo.Context) error {
	uuid := ctx.Param("uuid")
	if uuid == "" {
		u.logger.Error("User manager account uuid is required")
		return ctx.JSON(http.StatusBadRequest, schema.BadParamsErrorResponse)
	}

	path, err := u.userManagerConfigFile.GetPublicConfigPath(uuid)
	if err != nil {
		if errors.Is(err, gorm.ErrRecordNotFound) || errors.Is(err, common.ErrUserManagerAccountNotShared) {
			return ctx.JSON(http.StatusNotFound, schema.ErrorResponse{
				StatusCode: http.StatusNotFound,
				Status:     "error",
				Message:    "user manager account not found",
			})
		}

		return ctx.JSON(http.StatusNotFound, schema.ErrorResponse{
			StatusCode: http.StatusNotFound,
			Status:     "error",
			Message:    err.Error(),
		})
	}

	return attachFile(ctx, path, fmt.Sprintf("%s.ovpn", uuid))
}

// GetUserManagerProtocolCertificateFile is the public, unauthenticated
// download for the admin-uploaded certificate/quality file of the
// account's protocol -- gated by the same "uuid must be a currently
// shared account" check as the account's own share endpoints, via
// GetProtocolForSharedAccount.
func (u *UserController) GetUserManagerProtocolCertificateFile(ctx echo.Context) error {
	return u.getUserManagerProtocolFile(ctx, service.ProtocolFileKindCertificate)
}

// GetUserManagerProtocolClientAppFile mirrors
// GetUserManagerProtocolCertificateFile exactly, for the client app
// installer.
func (u *UserController) GetUserManagerProtocolClientAppFile(ctx echo.Context) error {
	return u.getUserManagerProtocolFile(ctx, service.ProtocolFileKindClientApp)
}

func (u *UserController) getUserManagerProtocolFile(ctx echo.Context, kind service.UserManagerProtocolFileKind) error {
	uuid := ctx.Param("uuid")
	if uuid == "" {
		u.logger.Error("User manager account uuid is required")
		return ctx.JSON(http.StatusBadRequest, schema.BadParamsErrorResponse)
	}

	requestedProtocol := ctx.QueryParam("protocol")
	if requestedProtocol == "" {
		return ctx.JSON(http.StatusBadRequest, schema.BadParamsErrorResponse)
	}

	protocol, err := u.userManagerService.EnsureProtocolForSharedAccount(uuid, requestedProtocol)
	if err != nil {
		if errors.Is(err, gorm.ErrRecordNotFound) || errors.Is(err, common.ErrUserManagerAccountNotShared) {
			return ctx.JSON(http.StatusNotFound, schema.ErrorResponse{
				StatusCode: http.StatusNotFound,
				Status:     "error",
				Message:    "user manager account not found",
			})
		}
		return ctx.JSON(http.StatusInternalServerError, schema.InternalServerErrorResponse)
	}

	path, err := u.userManagerProtocolFile.GetPublicFilePath(protocol, kind)
	if err != nil {
		return ctx.JSON(http.StatusNotFound, schema.ErrorResponse{
			StatusCode: http.StatusNotFound,
			Status:     "error",
			Message:    err.Error(),
		})
	}

	return attachFile(ctx, path, service.DownloadFilename(protocol, kind, path))
}
