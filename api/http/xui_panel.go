package http

import (
	"errors"
	"net/http"
	"strconv"

	"github.com/labstack/echo/v4"
	"go.uber.org/zap"
	"gorm.io/gorm"

	"github.com/maahdima/mwp/api/http/schema"
	"github.com/maahdima/mwp/api/service"
)

// XuiPanelController manages admin-registered x-ui panels -- every endpoint
// is admin-only (reuses forbidUnlessAdmin, same guard ResellerController
// and UserManagerTrafficPackageController's admin-only actions use).
type XuiPanelController struct {
	panelService *service.XuiPanelService
	logger       *zap.Logger
}

func NewXuiPanelController(panelService *service.XuiPanelService) *XuiPanelController {
	return &XuiPanelController{
		panelService: panelService,
		logger:       zap.L().Named("XuiPanelController"),
	}
}

func (c *XuiPanelController) ListPanels(ctx echo.Context) error {
	if forbidden, err := forbidUnlessAdmin(ctx); forbidden {
		return err
	}

	panels, err := c.panelService.ListPanels()
	if err != nil {
		c.logger.Error("failed to list x-ui panels", zap.Error(err))
		return ctx.JSON(http.StatusInternalServerError, schema.InternalServerErrorResponse)
	}

	return ctx.JSON(http.StatusOK, schema.BasicResponseData[[]schema.XuiPanelResponse]{
		BasicResponse: schema.OkBasicResponse,
		Data:          panels,
	})
}

func (c *XuiPanelController) CreatePanel(ctx echo.Context) error {
	if forbidden, err := forbidUnlessAdmin(ctx); forbidden {
		return err
	}

	var req schema.CreateXuiPanelRequest
	if err := ctx.Bind(&req); err != nil {
		c.logger.Warn("failed to bind request", zap.Error(err))
		return ctx.JSON(http.StatusBadRequest, schema.BadParamsErrorResponse)
	}
	if err := ctx.Validate(&req); err != nil {
		c.logger.Warn("failed to validate request", zap.Error(err))
		return ctx.JSON(http.StatusBadRequest, schema.BadParamsErrorResponse)
	}

	panel, err := c.panelService.CreatePanel(&req)
	if err != nil {
		c.logger.Error("failed to create x-ui panel", zap.Error(err))
		return ctx.JSON(http.StatusInternalServerError, schema.ErrorResponse{
			StatusCode: http.StatusInternalServerError,
			Status:     "error",
			Message:    "failed to create panel: " + err.Error(),
		})
	}

	return ctx.JSON(http.StatusCreated, schema.BasicResponseData[schema.XuiPanelResponse]{
		BasicResponse: schema.OkBasicResponse,
		Data:          *panel,
	})
}

func (c *XuiPanelController) UpdatePanel(ctx echo.Context) error {
	if forbidden, err := forbidUnlessAdmin(ctx); forbidden {
		return err
	}

	id, err := strconv.Atoi(ctx.Param("id"))
	if err != nil {
		return ctx.JSON(http.StatusBadRequest, schema.BadParamsErrorResponse)
	}

	var req schema.UpdateXuiPanelRequest
	if err := ctx.Bind(&req); err != nil {
		c.logger.Warn("failed to bind request", zap.Error(err))
		return ctx.JSON(http.StatusBadRequest, schema.BadParamsErrorResponse)
	}

	panel, err := c.panelService.UpdatePanel(uint(id), &req)
	if err != nil {
		if errors.Is(err, gorm.ErrRecordNotFound) {
			return ctx.JSON(http.StatusNotFound, schema.ErrorResponse{StatusCode: http.StatusNotFound, Status: "error", Message: "panel not found"})
		}
		c.logger.Error("failed to update x-ui panel", zap.Error(err))
		return ctx.JSON(http.StatusInternalServerError, schema.ErrorResponse{
			StatusCode: http.StatusInternalServerError,
			Status:     "error",
			Message:    "failed to update panel: " + err.Error(),
		})
	}

	return ctx.JSON(http.StatusOK, schema.BasicResponseData[schema.XuiPanelResponse]{
		BasicResponse: schema.OkBasicResponse,
		Data:          *panel,
	})
}

func (c *XuiPanelController) DeletePanel(ctx echo.Context) error {
	if forbidden, err := forbidUnlessAdmin(ctx); forbidden {
		return err
	}

	id, err := strconv.Atoi(ctx.Param("id"))
	if err != nil {
		return ctx.JSON(http.StatusBadRequest, schema.BadParamsErrorResponse)
	}

	if err := c.panelService.DeletePanel(uint(id)); err != nil {
		c.logger.Error("failed to delete x-ui panel", zap.Error(err))
		return ctx.JSON(http.StatusInternalServerError, schema.ErrorResponse{
			StatusCode: http.StatusInternalServerError,
			Status:     "error",
			Message:    "failed to delete panel: " + err.Error(),
		})
	}

	// NoContent, not JSON: a 204 response must not carry a body (RFC 9110
	// §15.3.5) -- ctx.JSON here would write one anyway and Echo logs a
	// spurious warning for it on every call.
	return ctx.NoContent(http.StatusNoContent)
}

// TestConnection tests an already-saved panel's credentials/connectivity.
func (c *XuiPanelController) TestConnection(ctx echo.Context) error {
	if forbidden, err := forbidUnlessAdmin(ctx); forbidden {
		return err
	}

	id, err := strconv.Atoi(ctx.Param("id"))
	if err != nil {
		return ctx.JSON(http.StatusBadRequest, schema.BadParamsErrorResponse)
	}

	result, err := c.panelService.TestConnection(uint(id))
	if err != nil {
		c.logger.Warn("x-ui connection test failed", zap.Uint("panel_id", uint(id)), zap.Error(err))
		return ctx.JSON(http.StatusBadGateway, schema.ErrorResponse{
			StatusCode: http.StatusBadGateway,
			Status:     "error",
			Message:    "connection test failed: " + err.Error(),
		})
	}

	return ctx.JSON(http.StatusOK, schema.BasicResponseData[schema.TestXuiConnectionResponse]{
		BasicResponse: schema.OkBasicResponse,
		Data:          *result,
	})
}

// TestConnectionUnsaved tests connectivity using the create-form's
// in-progress field values, before the panel has been persisted -- lets
// the admin populate the inbound-id dropdown while still filling out the
// create dialog.
func (c *XuiPanelController) TestConnectionUnsaved(ctx echo.Context) error {
	if forbidden, err := forbidUnlessAdmin(ctx); forbidden {
		return err
	}

	var req schema.TestXuiPanelConnectionRequest
	if err := ctx.Bind(&req); err != nil {
		c.logger.Warn("failed to bind request", zap.Error(err))
		return ctx.JSON(http.StatusBadRequest, schema.BadParamsErrorResponse)
	}
	// Required so a request whose Content-Type causes Echo's binder to
	// silently produce an all-zero-value struct (e.g. a non-JSON media
	// type on a JSON body, which Echo's form-binding path does not treat
	// as an error) is rejected here with a clear 400, instead of reaching
	// xui.Login with an empty APIBaseURL and surfacing as the far more
	// confusing "unsupported protocol scheme" error from Go's own HTTP
	// transport.
	if err := ctx.Validate(&req); err != nil {
		c.logger.Warn("failed to validate request", zap.Error(err))
		return ctx.JSON(http.StatusBadRequest, schema.BadParamsErrorResponse)
	}

	result, err := c.panelService.TestConnectionUnsaved(&req)
	if err != nil {
		c.logger.Warn("x-ui connection test (unsaved) failed", zap.String("api_base_url", req.APIBaseURL), zap.Error(err))
		return ctx.JSON(http.StatusBadGateway, schema.ErrorResponse{
			StatusCode: http.StatusBadGateway,
			Status:     "error",
			Message:    "connection test failed: " + err.Error(),
		})
	}

	return ctx.JSON(http.StatusOK, schema.BasicResponseData[schema.TestXuiConnectionResponse]{
		BasicResponse: schema.OkBasicResponse,
		Data:          *result,
	})
}
