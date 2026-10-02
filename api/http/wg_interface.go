package http

import (
	"errors"
	"net/http"
	"strconv"

	"github.com/maahdima/mwp/api/http/schema"
	"github.com/maahdima/mwp/api/service"

	"github.com/labstack/echo/v4"
	"go.uber.org/zap"
	"gorm.io/gorm"
)

type WgInterfaceController struct {
	interfaceService *service.WgInterface
	resellerService  *service.Reseller
	logger           *zap.Logger
}

func NewWgInterfaceController(interfaceService *service.WgInterface, resellerService *service.Reseller) *WgInterfaceController {
	return &WgInterfaceController{
		interfaceService: interfaceService,
		resellerService:  resellerService,
		logger:           zap.L().Named("WgInterfaceController"),
	}
}

func (c *WgInterfaceController) GetInterfaces(ctx echo.Context) error {
	var allowedIDs []uint
	if role, resellerID := getRoleAndResellerFromContext(ctx); role == "reseller" {
		if resellerID == nil {
			return ctx.JSON(http.StatusForbidden, schema.ErrorResponse{StatusCode: http.StatusForbidden, Status: "error", Message: "forbidden"})
		}
		ids, err := c.resellerService.GetAssignedInterfaceIDs(*resellerID)
		if err != nil {
			c.logger.Error("failed to get assigned interfaces", zap.Error(err))
			return ctx.JSON(http.StatusInternalServerError, schema.InternalServerErrorResponse)
		}
		allowedIDs = ids
	}

	interfaces, err := c.interfaceService.GetInterfaces(allowedIDs)
	if err != nil {
		c.logger.Error("failed to get wireguard interfaces", zap.Error(err))
		return ctx.JSON(http.StatusInternalServerError, schema.ErrorResponse{
			StatusCode: http.StatusInternalServerError,
			Status:     "error",
			Message:    "failed to retrieve wireguard interfaces: " + err.Error(),
		})
	}

	return ctx.JSON(http.StatusOK, schema.BasicResponseData[[]schema.InterfaceResponse]{
		BasicResponse: schema.OkBasicResponse,
		Data:          *interfaces,
	})
}

// forbidUnlessAdmin writes a 403 response and returns true if the caller is not an admin.
func forbidUnlessAdmin(ctx echo.Context) (bool, error) {
	if role, _ := getRoleAndResellerFromContext(ctx); role != "admin" {
		return true, ctx.JSON(http.StatusForbidden, schema.ErrorResponse{StatusCode: http.StatusForbidden, Status: "error", Message: "forbidden"})
	}
	return false, nil
}

// attachFile is a drop-in replacement for ctx.Attachment(path, name) that
// forces Content-Type to application/octet-stream first -- a confirmed,
// reported bug ("دانلود سرتیفیکیت openvpn در ساب که با فرمت txt دانلود
// میشه"): Echo's Attachment -> http.ServeContent only derives
// Content-Type from the file's own on-disk extension when the header
// isn't ALREADY set (see net/http/fs.go's own "if Content-Type isn't
// set" comment). Config/certificate downloads here use extensions
// (.ovpn, .conf, .crt, .pem, ...) that mostly have no entry in Go's mime
// type registry, so ServeContent falls back to sniffing the (plain-text)
// content as text/plain -- and several browsers then save the file with
// a .txt suffix regardless of what name the Content-Disposition header
// actually gives. Forcing a generic octet-stream type makes every
// browser trust that filename verbatim instead. Every download
// call site in this package that serves a non-standard extension should
// go through this helper rather than calling ctx.Attachment directly.
func attachFile(ctx echo.Context, path string, name string) error {
	ctx.Response().Header().Set(echo.HeaderContentType, "application/octet-stream")
	return ctx.Attachment(path, name)
}

func (c *WgInterfaceController) CreateInterface(ctx echo.Context) error {
	if forbidden, err := forbidUnlessAdmin(ctx); forbidden {
		return err
	}

	var req schema.CreateInterfaceRequest
	if err := ctx.Bind(&req); err != nil {
		c.logger.Error("failed to bind request", zap.Error(err))
		return ctx.JSON(http.StatusBadRequest, schema.ErrorResponse{
			StatusCode: http.StatusBadRequest,
			Status:     "error",
			Message:    "invalid request data: " + err.Error(),
		})
	}

	iface, err := c.interfaceService.CreateInterface(&req)
	if err != nil {
		c.logger.Error("failed to create wireguard interface", zap.Error(err))
		return ctx.JSON(http.StatusInternalServerError, schema.ErrorResponse{
			StatusCode: http.StatusInternalServerError,
			Status:     "error",
			Message:    "failed to create wireguard interface: " + err.Error(),
		})
	}

	return ctx.JSON(http.StatusCreated, schema.BasicResponseData[schema.InterfaceResponse]{
		BasicResponse: schema.OkBasicResponse,
		Data:          *iface,
	})
}

func (c *WgInterfaceController) UpdateInterfaceStatus(ctx echo.Context) error {
	if forbidden, err := forbidUnlessAdmin(ctx); forbidden {
		return err
	}

	id := ctx.Param("id")
	if id == "" {
		c.logger.Error("interface ID is required")
		return ctx.JSON(http.StatusBadRequest, schema.BadParamsErrorResponse)
	}

	interfaceId, err := strconv.Atoi(id)
	if err != nil {
		c.logger.Error("Invalid interface ID", zap.Error(err))
		return ctx.JSON(http.StatusBadRequest, schema.BadParamsErrorResponse)
	}

	err = c.interfaceService.ToggleInterfaceStatus(uint(interfaceId))
	if err != nil {
		if errors.Is(err, gorm.ErrRecordNotFound) {
			return ctx.JSON(http.StatusNotFound, schema.ErrorResponse{
				StatusCode: http.StatusNotFound,
				Status:     "error",
				Message:    "interface not found",
			})
		}
		c.logger.Error("failed to update wireguard interface status", zap.Error(err))
		return ctx.JSON(http.StatusInternalServerError, schema.ErrorResponse{
			StatusCode: http.StatusInternalServerError,
			Status:     "error",
			Message:    "failed to update wireguard interface status: " + err.Error(),
		})
	}

	return ctx.JSON(http.StatusOK, schema.OkBasicResponse)
}

func (c *WgInterfaceController) UpdateInterface(ctx echo.Context) error {
	if forbidden, err := forbidUnlessAdmin(ctx); forbidden {
		return err
	}

	id := ctx.Param("id")
	if id == "" {
		c.logger.Error("interface ID is required")
		return ctx.JSON(http.StatusBadRequest, schema.BadParamsErrorResponse)
	}

	interfaceId, err := strconv.Atoi(ctx.Param("id"))
	if err != nil {
		c.logger.Error("Invalid interface ID", zap.Error(err))
		return ctx.JSON(http.StatusBadRequest, schema.BadParamsErrorResponse)
	}

	var req schema.UpdateInterfaceRequest
	if err := ctx.Bind(&req); err != nil {
		c.logger.Error("failed to bind request", zap.Error(err))
		return ctx.JSON(http.StatusBadRequest, schema.BadParamsErrorResponse)
	}

	if err := ctx.Validate(req); err != nil {
		c.logger.Warn("failed to validate request", zap.Error(err))
		return ctx.JSON(http.StatusBadRequest, schema.BadParamsErrorResponse)
	}

	iface, err := c.interfaceService.UpdateInterface(uint(interfaceId), &req)
	if err != nil {
		if errors.Is(err, gorm.ErrRecordNotFound) {
			return ctx.JSON(http.StatusNotFound, schema.ErrorResponse{
				StatusCode: http.StatusNotFound,
				Status:     "error",
				Message:    "interface not found",
			})
		}
		c.logger.Error("failed to update wireguard interface", zap.Error(err))
		return ctx.JSON(http.StatusInternalServerError, schema.ErrorResponse{
			StatusCode: http.StatusInternalServerError,
			Status:     "error",
			Message:    "failed to update wireguard interface: " + err.Error(),
		})
	}

	return ctx.JSON(http.StatusOK, schema.BasicResponseData[schema.InterfaceResponse]{
		BasicResponse: schema.OkBasicResponse,
		Data:          *iface,
	})
}

func (c *WgInterfaceController) DeleteInterface(ctx echo.Context) error {
	if forbidden, err := forbidUnlessAdmin(ctx); forbidden {
		return err
	}

	id := ctx.Param("id")
	if id == "" {
		c.logger.Error("Interface ID is required")
		return ctx.JSON(http.StatusBadRequest, schema.BadParamsErrorResponse)
	}

	interfaceId, err := strconv.Atoi(ctx.Param("id"))
	if err != nil {
		c.logger.Error("Invalid interface ID", zap.Error(err))
		return ctx.JSON(http.StatusBadRequest, schema.BadParamsErrorResponse)
	}

	err = c.interfaceService.DeleteInterface(uint(interfaceId))
	if err != nil {
		if errors.Is(err, gorm.ErrRecordNotFound) {
			return ctx.JSON(http.StatusNotFound, schema.ErrorResponse{
				StatusCode: http.StatusNotFound,
				Status:     "error",
				Message:    "interface not found",
			})
		}
		if errors.Is(err, service.ErrInterfaceHasIPPool) {
			return ctx.JSON(http.StatusConflict, schema.ErrorResponse{
				StatusCode: http.StatusConflict,
				Status:     "error",
				Message:    err.Error(),
			})
		}
		c.logger.Error("failed to delete wireguard interface", zap.Error(err))
		return ctx.JSON(http.StatusInternalServerError, schema.ErrorResponse{
			StatusCode: http.StatusInternalServerError,
			Status:     "error",
			Message:    "failed to delete wireguard interface: " + err.Error(),
		})
	}

	return ctx.JSON(http.StatusNoContent, schema.BasicResponse{
		StatusCode: http.StatusNoContent,
		Status:     "success",
	})
}
