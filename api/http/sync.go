package http

import (
	"net/http"
	"strings"

	"github.com/labstack/echo/v4"
	"go.uber.org/zap"
	"gorm.io/gorm"

	"github.com/maahdima/mwp/api/http/schema"
	"github.com/maahdima/mwp/api/service"
)

// SyncController's actions all operate panel-wide, across every registered
// server, not one at a time -- there is no per-server picker on the sync
// preview/manual-sync page yet. Each action builds one service.SyncService
// per server via syncServiceFactory and runs it independently, so a
// problem on one server's sync never blocks another's (see
// service.SyncPeersAllServers and its siblings for the shared
// "one dead source never blocks the rest" convention this follows).
type SyncController struct {
	db                 *gorm.DB
	syncServiceFactory service.SyncServiceFactory
	logger             *zap.Logger
}

func NewSyncController(db *gorm.DB, syncServiceFactory service.SyncServiceFactory) *SyncController {
	return &SyncController{
		db:                 db,
		syncServiceFactory: syncServiceFactory,
		logger:             zap.L().Named("SyncController"),
	}
}

func (c *SyncController) GetSyncInterfaces(ctx echo.Context) error {
	if forbidden, err := forbidUnlessAdmin(ctx); forbidden {
		return err
	}

	ifaces, err := service.GetSyncInterfacesAllServers(c.db, c.syncServiceFactory)
	if err != nil {
		c.logger.Error("failed to fetch sync interfaces", zap.Error(err))
		return ctx.JSON(http.StatusInternalServerError, schema.ErrorResponse{StatusCode: http.StatusInternalServerError, Status: "error", Message: "failed to fetch interfaces for sync: " + err.Error()})
	}

	return ctx.JSON(http.StatusOK, schema.BasicResponseData[[]schema.SyncInterfacePreviewResponse]{BasicResponse: schema.OkBasicResponse, Data: ifaces})
}

func (c *SyncController) GetSyncPeers(ctx echo.Context) error {
	if forbidden, err := forbidUnlessAdmin(ctx); forbidden {
		return err
	}

	interfaceName := strings.TrimSpace(ctx.QueryParam("interface"))
	peers, err := service.GetSyncPeersAllServers(c.db, c.syncServiceFactory, interfaceName)
	if err != nil {
		c.logger.Error("failed to fetch sync peers", zap.Error(err))
		return ctx.JSON(http.StatusInternalServerError, schema.ErrorResponse{StatusCode: http.StatusInternalServerError, Status: "error", Message: "failed to fetch peers for sync: " + err.Error()})
	}

	return ctx.JSON(http.StatusOK, schema.BasicResponseData[[]schema.SyncPeerPreviewResponse]{BasicResponse: schema.OkBasicResponse, Data: peers})
}

func (c *SyncController) SyncSelectedInterfaces(ctx echo.Context) error {
	if forbidden, err := forbidUnlessAdmin(ctx); forbidden {
		return err
	}

	var req schema.SyncInterfacesRequest
	if err := ctx.Bind(&req); err != nil {
		return ctx.JSON(http.StatusBadRequest, schema.BadParamsErrorResponse)
	}
	if err := ctx.Validate(&req); err != nil {
		return ctx.JSON(http.StatusBadRequest, schema.BadParamsErrorResponse)
	}
	if err := service.SyncSelectedInterfacesForServer(c.syncServiceFactory, req.ServerID, req.InterfaceIDs); err != nil {
		c.logger.Error("failed to sync selected interfaces", zap.Error(err))
		return ctx.JSON(http.StatusInternalServerError, schema.ErrorResponse{StatusCode: http.StatusInternalServerError, Status: "error", Message: "failed to sync selected interfaces: " + err.Error()})
	}
	return ctx.JSON(http.StatusOK, schema.OkBasicResponse)
}

func (c *SyncController) SyncSelectedPeers(ctx echo.Context) error {
	if forbidden, err := forbidUnlessAdmin(ctx); forbidden {
		return err
	}

	var req schema.SyncPeersRequest
	if err := ctx.Bind(&req); err != nil {
		return ctx.JSON(http.StatusBadRequest, schema.BadParamsErrorResponse)
	}
	if err := ctx.Validate(&req); err != nil {
		return ctx.JSON(http.StatusBadRequest, schema.BadParamsErrorResponse)
	}
	if err := service.SyncSelectedPeersForServer(c.syncServiceFactory, req.ServerID, req.PeerIDs); err != nil {
		c.logger.Error("failed to sync selected peers", zap.Error(err))
		return ctx.JSON(http.StatusInternalServerError, schema.ErrorResponse{StatusCode: http.StatusInternalServerError, Status: "error", Message: "failed to sync selected peers: " + err.Error()})
	}
	return ctx.JSON(http.StatusOK, schema.OkBasicResponse)
}

func (c *SyncController) SyncPeers(ctx echo.Context) error {
	if forbidden, err := forbidUnlessAdmin(ctx); forbidden {
		return err
	}

	if err := service.SyncPeersAllServers(c.db, c.syncServiceFactory); err != nil {
		c.logger.Error("failed to sync peers", zap.Error(err))
		return ctx.JSON(http.StatusInternalServerError, schema.ErrorResponse{
			StatusCode: http.StatusInternalServerError,
			Status:     "error",
			Message:    "failed to sync peers: " + err.Error(),
		})
	}

	return ctx.JSON(http.StatusOK, schema.OkBasicResponse)
}

func (c *SyncController) SyncInterfaces(ctx echo.Context) error {
	if forbidden, err := forbidUnlessAdmin(ctx); forbidden {
		return err
	}

	if err := service.SyncInterfacesAllServers(c.db, c.syncServiceFactory); err != nil {
		c.logger.Error("failed to sync interfaces", zap.Error(err))
		return ctx.JSON(http.StatusInternalServerError, schema.ErrorResponse{
			StatusCode: http.StatusInternalServerError,
			Status:     "error",
			Message:    "failed to sync interfaces: " + err.Error(),
		})
	}

	return ctx.JSON(http.StatusOK, schema.OkBasicResponse)
}
