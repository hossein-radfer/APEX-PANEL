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

// DNSPanelController manages admin-registered doctor-dns installations --
// every endpoint is admin-only, mirroring XuiPanelController exactly.
type DNSPanelController struct {
	panelService *service.DNSPanelService
	logger       *zap.Logger
}

func NewDNSPanelController(panelService *service.DNSPanelService) *DNSPanelController {
	return &DNSPanelController{
		panelService: panelService,
		logger:       zap.L().Named("DNSPanelController"),
	}
}

func (c *DNSPanelController) ListPanels(ctx echo.Context) error {
	if forbidden, err := forbidUnlessAdmin(ctx); forbidden {
		return err
	}

	panels, err := c.panelService.ListPanels()
	if err != nil {
		c.logger.Error("failed to list DNS panels", zap.Error(err))
		return ctx.JSON(http.StatusInternalServerError, schema.InternalServerErrorResponse)
	}

	return ctx.JSON(http.StatusOK, schema.BasicResponseData[[]schema.DNSPanelResponse]{
		BasicResponse: schema.OkBasicResponse,
		Data:          panels,
	})
}

func (c *DNSPanelController) CreatePanel(ctx echo.Context) error {
	if forbidden, err := forbidUnlessAdmin(ctx); forbidden {
		return err
	}

	var req schema.CreateDNSPanelRequest
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
		c.logger.Error("failed to create DNS panel", zap.Error(err))
		return ctx.JSON(http.StatusInternalServerError, schema.ErrorResponse{
			StatusCode: http.StatusInternalServerError,
			Status:     "error",
			Message:    "failed to create panel: " + err.Error(),
		})
	}

	return ctx.JSON(http.StatusCreated, schema.BasicResponseData[schema.DNSPanelResponse]{
		BasicResponse: schema.OkBasicResponse,
		Data:          *panel,
	})
}

func (c *DNSPanelController) UpdatePanel(ctx echo.Context) error {
	if forbidden, err := forbidUnlessAdmin(ctx); forbidden {
		return err
	}

	id, err := strconv.Atoi(ctx.Param("id"))
	if err != nil {
		return ctx.JSON(http.StatusBadRequest, schema.BadParamsErrorResponse)
	}

	var req schema.UpdateDNSPanelRequest
	if err := ctx.Bind(&req); err != nil {
		c.logger.Warn("failed to bind request", zap.Error(err))
		return ctx.JSON(http.StatusBadRequest, schema.BadParamsErrorResponse)
	}

	panel, err := c.panelService.UpdatePanel(uint(id), &req)
	if err != nil {
		if errors.Is(err, gorm.ErrRecordNotFound) {
			return ctx.JSON(http.StatusNotFound, schema.ErrorResponse{StatusCode: http.StatusNotFound, Status: "error", Message: "panel not found"})
		}
		c.logger.Error("failed to update DNS panel", zap.Error(err))
		return ctx.JSON(http.StatusInternalServerError, schema.ErrorResponse{
			StatusCode: http.StatusInternalServerError,
			Status:     "error",
			Message:    "failed to update panel: " + err.Error(),
		})
	}

	return ctx.JSON(http.StatusOK, schema.BasicResponseData[schema.DNSPanelResponse]{
		BasicResponse: schema.OkBasicResponse,
		Data:          *panel,
	})
}

func (c *DNSPanelController) DeletePanel(ctx echo.Context) error {
	if forbidden, err := forbidUnlessAdmin(ctx); forbidden {
		return err
	}

	id, err := strconv.Atoi(ctx.Param("id"))
	if err != nil {
		return ctx.JSON(http.StatusBadRequest, schema.BadParamsErrorResponse)
	}

	if err := c.panelService.DeletePanel(uint(id)); err != nil {
		c.logger.Error("failed to delete DNS panel", zap.Error(err))
		return ctx.JSON(http.StatusInternalServerError, schema.ErrorResponse{
			StatusCode: http.StatusInternalServerError,
			Status:     "error",
			Message:    "failed to delete panel: " + err.Error(),
		})
	}

	return ctx.NoContent(http.StatusNoContent)
}

// TestConnection tests an already-saved panel's credentials/connectivity.
func (c *DNSPanelController) TestConnection(ctx echo.Context) error {
	if forbidden, err := forbidUnlessAdmin(ctx); forbidden {
		return err
	}

	id, err := strconv.Atoi(ctx.Param("id"))
	if err != nil {
		return ctx.JSON(http.StatusBadRequest, schema.BadParamsErrorResponse)
	}

	result, err := c.panelService.TestConnection(uint(id))
	if err != nil {
		c.logger.Warn("DNS panel connection test failed", zap.Uint("panel_id", uint(id)), zap.Error(err))
		return ctx.JSON(http.StatusBadGateway, schema.ErrorResponse{
			StatusCode: http.StatusBadGateway,
			Status:     "error",
			Message:    "connection test failed: " + err.Error(),
		})
	}

	return ctx.JSON(http.StatusOK, schema.BasicResponseData[schema.TestDNSConnectionResponse]{
		BasicResponse: schema.OkBasicResponse,
		Data:          *result,
	})
}

// TestConnectionUnsaved tests connectivity using the create-form's
// in-progress field values, before the panel has been persisted.
func (c *DNSPanelController) TestConnectionUnsaved(ctx echo.Context) error {
	if forbidden, err := forbidUnlessAdmin(ctx); forbidden {
		return err
	}

	var req schema.TestDNSPanelConnectionRequest
	if err := ctx.Bind(&req); err != nil {
		c.logger.Warn("failed to bind request", zap.Error(err))
		return ctx.JSON(http.StatusBadRequest, schema.BadParamsErrorResponse)
	}
	if err := ctx.Validate(&req); err != nil {
		c.logger.Warn("failed to validate request", zap.Error(err))
		return ctx.JSON(http.StatusBadRequest, schema.BadParamsErrorResponse)
	}

	result, err := c.panelService.TestConnectionUnsaved(&req)
	if err != nil {
		c.logger.Warn("DNS panel connection test (unsaved) failed", zap.String("api_base_url", req.APIBaseURL), zap.Error(err))
		return ctx.JSON(http.StatusBadGateway, schema.ErrorResponse{
			StatusCode: http.StatusBadGateway,
			Status:     "error",
			Message:    "connection test failed: " + err.Error(),
		})
	}

	return ctx.JSON(http.StatusOK, schema.BasicResponseData[schema.TestDNSConnectionResponse]{
		BasicResponse: schema.OkBasicResponse,
		Data:          *result,
	})
}
