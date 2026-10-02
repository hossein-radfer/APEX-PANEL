package http

import (
	"net/http"
	"strconv"

	"github.com/maahdima/mwp/api/config"
	"github.com/maahdima/mwp/api/http/schema"
	"github.com/maahdima/mwp/api/service"

	"github.com/labstack/echo/v4"
	"go.uber.org/zap"
)

type SystemConfigController struct {
	systemConfigService *service.SystemConfigService
	logger              *zap.Logger
}

func NewSystemConfigController(systemConfigService *service.SystemConfigService) *SystemConfigController {
	return &SystemConfigController{
		systemConfigService: systemConfigService,
		logger:              zap.L().Named("SystemConfigController"),
	}
}

// GetPortConfig reports the port the running process actually bound to
// (config.GetAppConfig().Port, i.e. SERVER_PORT as it stood at startup,
// already reflecting any prior override -- see cmd/main.go) alongside any
// admin-set override still pending a restart. Admin only.
func (c *SystemConfigController) GetPortConfig(ctx echo.Context) error {
	if forbidden, err := forbidUnlessAdmin(ctx); forbidden {
		return err
	}

	appCfg := config.GetAppConfig()
	currentPort, err := strconv.Atoi(appCfg.Port)
	if err != nil {
		c.logger.Error("failed to parse current SERVER_PORT as an integer", zap.String("port", appCfg.Port), zap.Error(err))
		return ctx.JSON(http.StatusInternalServerError, schema.InternalServerErrorResponse)
	}

	resp := schema.PortConfigResponse{CurrentPort: currentPort}

	overrideStr, err := c.systemConfigService.GetPortOverride()
	if err != nil {
		c.logger.Error("failed to read port override", zap.Error(err))
		return ctx.JSON(http.StatusInternalServerError, schema.InternalServerErrorResponse)
	}
	if overrideStr != "" {
		if overridePort, err := strconv.Atoi(overrideStr); err == nil && overridePort != currentPort {
			resp.PendingPort = &overridePort
		}
	}

	return ctx.JSON(http.StatusOK, schema.BasicResponseData[schema.PortConfigResponse]{
		BasicResponse: schema.OkBasicResponse,
		Data:          resp,
	})
}

// UpdatePortConfig persists a new desired listen port. This does NOT change
// the port the currently-running process is bound to -- Echo binds its
// listener once at startup and cannot be rebound live, so the change only
// takes effect the next time the panel process restarts (see
// SystemConfigService.SetPortOverride and cmd/main.go). Admin only.
func (c *SystemConfigController) UpdatePortConfig(ctx echo.Context) error {
	if forbidden, err := forbidUnlessAdmin(ctx); forbidden {
		return err
	}

	var req schema.UpdatePortConfigRequest
	if err := ctx.Bind(&req); err != nil {
		c.logger.Warn("failed to bind request", zap.Error(err))
		return ctx.JSON(http.StatusBadRequest, schema.BadParamsErrorResponse)
	}
	if err := ctx.Validate(&req); err != nil {
		c.logger.Warn("failed to validate request", zap.Error(err))
		return ctx.JSON(http.StatusBadRequest, schema.BadParamsErrorResponse)
	}

	if err := c.systemConfigService.SetPortOverride(req.Port); err != nil {
		c.logger.Error("failed to set port override", zap.Error(err))
		return ctx.JSON(http.StatusInternalServerError, schema.ErrorResponse{
			StatusCode: http.StatusInternalServerError,
			Status:     "error",
			Message:    err.Error(),
		})
	}

	appCfg := config.GetAppConfig()
	currentPort, _ := strconv.Atoi(appCfg.Port)
	pendingPort := req.Port

	return ctx.JSON(http.StatusOK, schema.BasicResponseData[schema.PortConfigResponse]{
		BasicResponse: schema.OkBasicResponse,
		Data: schema.PortConfigResponse{
			CurrentPort: currentPort,
			PendingPort: &pendingPort,
		},
	})
}

// GetDatabaseSize reports the panel's own SQLite database file's total
// size and a per-table breakdown of what's using that space (see
// SystemConfigService.GetDatabaseSizeBreakdown's own doc comment for the
// exact mechanism). Admin only -- this exposes internal table names/sizes
// that aren't relevant to a reseller's own view of the panel.
func (c *SystemConfigController) GetDatabaseSize(ctx echo.Context) error {
	if forbidden, err := forbidUnlessAdmin(ctx); forbidden {
		return err
	}

	breakdown, err := c.systemConfigService.GetDatabaseSizeBreakdown()
	if err != nil {
		c.logger.Error("failed to get database size breakdown", zap.Error(err))
		return ctx.JSON(http.StatusInternalServerError, schema.ErrorResponse{
			StatusCode: http.StatusInternalServerError,
			Status:     "error",
			Message:    err.Error(),
		})
	}

	tables := make([]schema.DatabaseSizeTableEntry, 0, len(breakdown.Tables))
	for _, t := range breakdown.Tables {
		tables = append(tables, schema.DatabaseSizeTableEntry{Name: t.Name, BytesUsed: t.BytesUsed})
	}

	return ctx.JSON(http.StatusOK, schema.BasicResponseData[schema.DatabaseSizeResponse]{
		BasicResponse: schema.OkBasicResponse,
		Data: schema.DatabaseSizeResponse{
			TotalBytes: breakdown.TotalBytes,
			Tables:     tables,
		},
	})
}
