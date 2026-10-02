package http

import (
	"errors"
	"net/http"
	"strconv"

	"github.com/maahdima/mwp/api/common"
	"github.com/maahdima/mwp/api/http/schema"
	"github.com/maahdima/mwp/api/service"

	"github.com/labstack/echo/v4"
	"go.uber.org/zap"
	"gorm.io/gorm"
)

type ServerController struct {
	serverService *service.Server
	mwpClients    *common.MwpClients
	logger        *zap.Logger
}

func NewServerController(serverService *service.Server, mwpClients *common.MwpClients) *ServerController {
	return &ServerController{
		serverService: serverService,
		mwpClients:    mwpClients,
		logger:        zap.L().Named("ServerController"),
	}
}

func (c *ServerController) GetServers(ctx echo.Context) error {
	if forbidden, err := forbidUnlessAdmin(ctx); forbidden {
		return err
	}

	servers, err := c.serverService.GetServers()
	if err != nil {
		c.logger.Error("failed to get servers", zap.Error(err))
		return ctx.JSON(http.StatusInternalServerError, schema.ErrorResponse{
			StatusCode: http.StatusInternalServerError,
			Status:     "error",
			Message:    "failed to retrieve servers: " + err.Error(),
		})
	}

	return ctx.JSON(http.StatusOK, schema.BasicResponseData[[]schema.ServerResponse]{
		BasicResponse: schema.OkBasicResponse,
		Data:          *servers,
	})
}

// GetServerEndpoints returns minimal, credential-free server connection info.
// Available to any authenticated role (including resellers) since building a
// WireGuard peer config requires knowing the server's IP address.
func (c *ServerController) GetServerEndpoints(ctx echo.Context) error {
	endpoints, err := c.serverService.GetServerEndpoints()
	if err != nil {
		c.logger.Error("failed to get server endpoints", zap.Error(err))
		return ctx.JSON(http.StatusInternalServerError, schema.ErrorResponse{
			StatusCode: http.StatusInternalServerError,
			Status:     "error",
			Message:    "failed to retrieve server endpoints: " + err.Error(),
		})
	}

	return ctx.JSON(http.StatusOK, schema.BasicResponseData[[]schema.ServerEndpointResponse]{
		BasicResponse: schema.OkBasicResponse,
		Data:          *endpoints,
	})
}

// GetConnectionHealth runs a live, on-demand probe of this install's
// configured Mikrotik server (the same check ClientConnectionMiddleware
// runs before every gated request) and reports back exactly why it
// succeeded or failed -- latency on success; a specific reason code
// (timeout/connection_refused/unauthorized/bad_status/not_configured)
// plus a human-readable detail on failure -- instead of making the admin
// wait for an unrelated request to fail and guess from a generic "not
// connected" error.
func (c *ServerController) GetConnectionHealth(ctx echo.Context) error {
	if forbidden, err := forbidUnlessAdmin(ctx); forbidden {
		return err
	}

	connected, reason, detail := c.mwpClients.CheckConnection(nil)

	return ctx.JSON(http.StatusOK, schema.BasicResponseData[schema.ServerConnectionHealthResponse]{
		BasicResponse: schema.OkBasicResponse,
		Data: schema.ServerConnectionHealthResponse{
			Connected: connected,
			Reason:    reason,
			Detail:    detail,
		},
	})
}

func (c *ServerController) CreateServer(ctx echo.Context) error {
	if forbidden, err := forbidUnlessAdmin(ctx); forbidden {
		return err
	}

	var req schema.CreateServerRequest

	if err := ctx.Bind(&req); err != nil {
		c.logger.Error("failed to bind request", zap.Error(err))
		return ctx.JSON(http.StatusBadRequest, schema.BadParamsErrorResponse)
	}

	if err := ctx.Validate(&req); err != nil {
		c.logger.Warn("failed to validate request", zap.Error(err))
		return ctx.JSON(http.StatusBadRequest, schema.BadParamsErrorResponse)
	}

	server, err := c.serverService.CreateServer(&req)
	if err != nil {
		c.logger.Error("failed to create server", zap.Error(err))
		return ctx.JSON(http.StatusInternalServerError, schema.ErrorResponse{
			StatusCode: http.StatusInternalServerError,
			Status:     "error",
			Message:    "failed to create server: " + err.Error(),
		})
	}

	return ctx.JSON(http.StatusOK, schema.BasicResponseData[schema.ServerResponse]{
		BasicResponse: schema.OkBasicResponse,
		Data:          *server,
	})
}

func (c *ServerController) UpdateServerStatus(ctx echo.Context) error {
	if forbidden, err := forbidUnlessAdmin(ctx); forbidden {
		return err
	}

	id := ctx.Param("id")
	if id == "" {
		c.logger.Error("Server ID is required")
		return ctx.JSON(http.StatusBadRequest, schema.BadParamsErrorResponse)
	}

	serverId, err := strconv.Atoi(id)
	if err != nil {
		c.logger.Error("Invalid server ID", zap.Error(err))
		return ctx.JSON(http.StatusBadRequest, schema.BadParamsErrorResponse)
	}

	server, err := c.serverService.ToggleServerStatus(uint(serverId))
	if err != nil {
		if errors.Is(err, gorm.ErrRecordNotFound) {
			return ctx.JSON(http.StatusNotFound, schema.ErrorResponse{
				StatusCode: http.StatusNotFound,
				Status:     "error",
				Message:    "server not found",
			})
		}
		c.logger.Error("failed to update server status", zap.Error(err))
		return ctx.JSON(http.StatusInternalServerError, schema.ErrorResponse{
			StatusCode: http.StatusInternalServerError,
			Status:     "error",
			Message:    "failed to update server status: " + err.Error(),
		})
	}

	return ctx.JSON(http.StatusOK, schema.BasicResponseData[schema.ServerResponse]{
		BasicResponse: schema.OkBasicResponse,
		Data:          *server,
	})
}

func (c *ServerController) UpdateServer(ctx echo.Context) error {
	if forbidden, err := forbidUnlessAdmin(ctx); forbidden {
		return err
	}

	id := ctx.Param("id")
	if id == "" {
		c.logger.Error("Server ID is required")
		return ctx.JSON(http.StatusBadRequest, schema.BadParamsErrorResponse)
	}

	serverId, err := strconv.Atoi(id)
	if err != nil {
		c.logger.Error("Invalid server ID", zap.Error(err))
		return ctx.JSON(http.StatusBadRequest, schema.BadParamsErrorResponse)
	}

	var req schema.UpdateServerRequest
	if err := ctx.Bind(&req); err != nil {
		c.logger.Error("failed to bind request", zap.Error(err))
		return ctx.JSON(http.StatusBadRequest, schema.BadParamsErrorResponse)
	}

	if err := ctx.Validate(&req); err != nil {
		c.logger.Warn("failed to validate request", zap.Error(err))
		return ctx.JSON(http.StatusBadRequest, schema.BadParamsErrorResponse)
	}

	server, err := c.serverService.UpdateServer(uint(serverId), &req)
	if err != nil {
		if errors.Is(err, gorm.ErrRecordNotFound) {
			return ctx.JSON(http.StatusNotFound, schema.ErrorResponse{
				StatusCode: http.StatusNotFound,
				Status:     "error",
				Message:    "server not found",
			})
		}
		c.logger.Error("failed to update server", zap.Error(err))
		return ctx.JSON(http.StatusInternalServerError, schema.ErrorResponse{
			StatusCode: http.StatusInternalServerError,
			Status:     "error",
			Message:    "failed to update server: " + err.Error(),
		})
	}

	return ctx.JSON(http.StatusOK, schema.BasicResponseData[schema.ServerResponse]{
		BasicResponse: schema.OkBasicResponse,
		Data:          *server,
	})
}

func (c *ServerController) DeleteServer(ctx echo.Context) error {
	if forbidden, err := forbidUnlessAdmin(ctx); forbidden {
		return err
	}

	id := ctx.Param("id")
	if id == "" {
		c.logger.Error("Server ID is required")
		return ctx.JSON(http.StatusBadRequest, schema.BadParamsErrorResponse)
	}

	serverId, err := strconv.Atoi(id)
	if err != nil {
		c.logger.Error("Invalid server ID", zap.Error(err))
		return ctx.JSON(http.StatusBadRequest, schema.BadParamsErrorResponse)
	}

	if err := c.serverService.DeleteServer(uint(serverId)); err != nil {
		if errors.Is(err, gorm.ErrRecordNotFound) {
			return ctx.JSON(http.StatusNotFound, schema.ErrorResponse{
				StatusCode: http.StatusNotFound,
				Status:     "error",
				Message:    "server not found",
			})
		}
		c.logger.Error("failed to delete server", zap.Error(err))
		return ctx.JSON(http.StatusInternalServerError, schema.ErrorResponse{
			StatusCode: http.StatusInternalServerError,
			Status:     "error",
			Message:    "failed to delete server: " + err.Error(),
		})
	}

	return ctx.JSON(http.StatusOK, schema.OkBasicResponse)
}
