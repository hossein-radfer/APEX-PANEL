package http

import (
	"errors"
	"fmt"
	"net/http"
	"strconv"

	traffic "github.com/maahdima/mwp/api/cmd/jobs"
	"github.com/maahdima/mwp/api/http/schema"
	"github.com/maahdima/mwp/api/service"

	"github.com/labstack/echo/v4"
	"go.uber.org/zap"
	"gorm.io/gorm"
)

type WgPeerController struct {
	peerService            *service.WgPeer
	configGeneratorService *service.ConfigGenerator
	qrCodeGeneratorService *service.QRCodeGenerator
	excelGeneratorService  *service.ExcelGenerator
	trafficCalculator      *traffic.Calculator
	logger                 *zap.Logger
}

func NewWgPeerController(PeerService *service.WgPeer, configGeneratorService *service.ConfigGenerator, qrCodeGeneratorService *service.QRCodeGenerator, excelGeneratorService *service.ExcelGenerator, trafficCalculator *traffic.Calculator) *WgPeerController {
	return &WgPeerController{
		peerService:            PeerService,
		configGeneratorService: configGeneratorService,
		qrCodeGeneratorService: qrCodeGeneratorService,
		excelGeneratorService:  excelGeneratorService,
		trafficCalculator:      trafficCalculator,
		logger:                 zap.L().Named("WgPeerController"),
	}
}

func (c *WgPeerController) GetPeerCredentials(ctx echo.Context) error {
	credentials, err := c.peerService.GetPeerCredentials()
	if err != nil {
		c.logger.Error("failed to get wireguard peer credentials", zap.Error(err))
		return ctx.JSON(http.StatusInternalServerError, schema.ErrorResponse{
			StatusCode: http.StatusInternalServerError,
			Status:     "error",
			Message:    "failed to retrieve wireguard peer credentials: " + err.Error(),
		})
	}

	return ctx.JSON(http.StatusOK, schema.BasicResponseData[schema.PeerCredentialsResponse]{
		BasicResponse: schema.OkBasicResponse,
		Data:          *credentials,
	})
}

func (c *WgPeerController) GetNewPeerAllowedAddress(ctx echo.Context) error {
	var req schema.NewPeerAllowedAddressRequest

	if err := ctx.Bind(&req); err != nil {
		c.logger.Warn("failed to bind request", zap.Error(err))
		return ctx.JSON(http.StatusBadRequest, schema.BadParamsErrorResponse)
	}

	if err := ctx.Validate(&req); err != nil {
		c.logger.Warn("failed to validate request", zap.Error(err))
		return ctx.JSON(http.StatusBadRequest, schema.BadParamsErrorResponse)
	}

	resellerID, scopeErr := peerScopeFromContext(ctx)
	if scopeErr != nil {
		return scopeErr
	}

	allowedAddress, err := c.peerService.GetNewPeerAllowedAddress(req.InterfaceId, resellerID)
	if err != nil {
		c.logger.Error("failed to get wireguard peer allowed addresses", zap.Error(err))
		return ctx.JSON(http.StatusInternalServerError, schema.ErrorResponse{
			StatusCode: http.StatusInternalServerError,
			Status:     "error",
			Message:    "failed to retrieve wireguard peer allowed addresses: " + err.Error(),
		})
	}

	return ctx.JSON(http.StatusOK, schema.BasicResponseData[schema.NewPeerAllowedAddressResponse]{
		BasicResponse: schema.OkBasicResponse,
		Data:          *allowedAddress,
	})
}

func (c *WgPeerController) GetPeers(ctx echo.Context) error {
	resellerID, err := peerScopeFromContext(ctx)
	if err != nil {
		return err
	}

	peers, err := c.peerService.GetPeers(resellerID)
	if err != nil {
		c.logger.Error("failed to get wireguard peers", zap.Error(err))
		return ctx.JSON(http.StatusInternalServerError, schema.ErrorResponse{
			StatusCode: http.StatusInternalServerError,
			Status:     "error",
			Message:    "failed to retrieve wireguard peers: " + err.Error(),
		})
	}

	return ctx.JSON(http.StatusOK, schema.BasicResponseData[[]schema.PeerResponse]{
		BasicResponse: schema.OkBasicResponse,
		Data:          *peers,
	})
}

// TODO: implement
func (c *WgPeerController) GetPeerByID(ctx echo.Context) error {
	return nil
}

// GetPeersByReseller is the admin-only "Reseller Peers" page's endpoint --
// unlike GetPeers (which scopes to the caller's OWN reseller_id from their
// JWT when role=="reseller", or shows everything unscoped for role==
// "admin"), this always scopes to the specific :reseller_id in the URL,
// regardless of who is asking, and is only reachable by an admin caller
// (see adminResellerIDFromParam).
func (c *WgPeerController) GetPeersByReseller(ctx echo.Context) error {
	resellerID, err := adminResellerIDFromParam(ctx)
	if err != nil {
		return err
	}

	peers, err := c.peerService.GetPeersByReseller(resellerID)
	if err != nil {
		c.logger.Error("failed to get wireguard peers for reseller", zap.Error(err))
		return ctx.JSON(http.StatusInternalServerError, schema.ErrorResponse{
			StatusCode: http.StatusInternalServerError,
			Status:     "error",
			Message:    "failed to retrieve wireguard peers: " + err.Error(),
		})
	}

	return ctx.JSON(http.StatusOK, schema.BasicResponseData[[]schema.PeerResponse]{
		BasicResponse: schema.OkBasicResponse,
		Data:          *peers,
	})
}

// CreatePeerForReseller lets an admin create a peer on behalf of a
// specific reseller (as opposed to CreatePeer, which creates on behalf of
// whichever reseller is making the call, or unowned when an admin calls
// it directly).
func (c *WgPeerController) CreatePeerForReseller(ctx echo.Context) error {
	resellerID, err := adminResellerIDFromParam(ctx)
	if err != nil {
		return err
	}

	var req schema.CreatePeerRequest
	if err := ctx.Bind(&req); err != nil {
		c.logger.Warn("failed to bind request", zap.Error(err))
		return ctx.JSON(http.StatusBadRequest, schema.BadParamsErrorResponse)
	}
	if err := ctx.Validate(&req); err != nil {
		c.logger.Warn("failed to validate request", zap.Error(err))
		return ctx.JSON(http.StatusBadRequest, schema.BadParamsErrorResponse)
	}

	peer, err := c.peerService.CreatePeer(&req, &resellerID)
	if err != nil {
		c.logger.Error("failed to create wireguard peer for reseller", zap.Error(err))
		return ctx.JSON(http.StatusInternalServerError, schema.ErrorResponse{
			StatusCode: http.StatusInternalServerError,
			Status:     "error",
			Message:    "failed to create wireguard peer: " + err.Error(),
		})
	}

	return ctx.JSON(http.StatusCreated, schema.BasicResponseData[schema.PeerResponse]{
		BasicResponse: schema.OkBasicResponse,
		Data:          *peer,
	})
}

// UpdatePeerForReseller lets an admin update a peer that belongs to a
// specific reseller. The :reseller_id path segment is used purely as the
// authorization scope (the peer must belong to that reseller, enforced by
// peerService.UpdatePeer's own resellerID-scoped lookup) -- it is not a
// separate lookup key from :id.
func (c *WgPeerController) UpdatePeerForReseller(ctx echo.Context) error {
	resellerID, err := adminResellerIDFromParam(ctx)
	if err != nil {
		return err
	}

	id := ctx.Param("id")
	if id == "" {
		c.logger.Error("Peer ID is required")
		return ctx.JSON(http.StatusBadRequest, schema.BadParamsErrorResponse)
	}
	peerId, err := strconv.Atoi(id)
	if err != nil {
		c.logger.Error("Invalid peer ID", zap.Error(err))
		return ctx.JSON(http.StatusBadRequest, schema.BadParamsErrorResponse)
	}

	var req schema.UpdatePeerRequest
	if err := ctx.Bind(&req); err != nil {
		c.logger.Warn("failed to bind request", zap.Error(err))
		return ctx.JSON(http.StatusBadRequest, schema.BadParamsErrorResponse)
	}
	if err := ctx.Validate(&req); err != nil {
		c.logger.Warn("failed to validate request", zap.Error(err))
		return ctx.JSON(http.StatusBadRequest, schema.BadParamsErrorResponse)
	}

	peer, err := c.peerService.UpdatePeer(uint(peerId), &req, &resellerID)
	if err != nil {
		if errors.Is(err, gorm.ErrRecordNotFound) {
			return ctx.JSON(http.StatusNotFound, schema.ErrorResponse{
				StatusCode: http.StatusNotFound,
				Status:     "error",
				Message:    "peer not found",
			})
		}
		c.logger.Error("failed to update wireguard peer for reseller", zap.Error(err))
		return ctx.JSON(http.StatusInternalServerError, schema.ErrorResponse{
			StatusCode: http.StatusInternalServerError,
			Status:     "error",
			Message:    "failed to update wireguard peer: " + err.Error(),
		})
	}

	return ctx.JSON(http.StatusOK, schema.BasicResponseData[schema.PeerResponse]{
		BasicResponse: schema.OkBasicResponse,
		Data:          *peer,
	})
}

// DeletePeerForReseller lets an admin delete a peer that belongs to a
// specific reseller.
func (c *WgPeerController) DeletePeerForReseller(ctx echo.Context) error {
	resellerID, err := adminResellerIDFromParam(ctx)
	if err != nil {
		return err
	}

	id := ctx.Param("id")
	if id == "" {
		c.logger.Error("Peer ID is required")
		return ctx.JSON(http.StatusBadRequest, schema.BadParamsErrorResponse)
	}
	peerId, err := strconv.Atoi(id)
	if err != nil {
		c.logger.Error("Invalid peer ID", zap.Error(err))
		return ctx.JSON(http.StatusBadRequest, schema.BadParamsErrorResponse)
	}

	if err := c.peerService.DeletePeer(uint(peerId), &resellerID); err != nil {
		if errors.Is(err, gorm.ErrRecordNotFound) {
			return ctx.JSON(http.StatusNotFound, schema.ErrorResponse{
				StatusCode: http.StatusNotFound,
				Status:     "error",
				Message:    "peer not found",
			})
		}
		c.logger.Error("failed to delete wireguard peer for reseller", zap.Error(err))
		return ctx.JSON(http.StatusInternalServerError, schema.ErrorResponse{
			StatusCode: http.StatusInternalServerError,
			Status:     "error",
			Message:    "failed to delete wireguard peer: " + err.Error(),
		})
	}

	return ctx.JSON(http.StatusOK, schema.OkBasicResponse)
}

// UpdatePeerStatusForReseller lets an admin toggle a peer's enabled state
// for a peer belonging to a specific reseller.
func (c *WgPeerController) UpdatePeerStatusForReseller(ctx echo.Context) error {
	resellerID, err := adminResellerIDFromParam(ctx)
	if err != nil {
		return err
	}

	id := ctx.Param("id")
	if id == "" {
		c.logger.Error("Peer ID is required")
		return ctx.JSON(http.StatusBadRequest, schema.BadParamsErrorResponse)
	}
	peerId, err := strconv.Atoi(id)
	if err != nil {
		c.logger.Error("Invalid peer ID", zap.Error(err))
		return ctx.JSON(http.StatusBadRequest, schema.BadParamsErrorResponse)
	}

	if err := c.peerService.TogglePeerStatus(uint(peerId), &resellerID); err != nil {
		if errors.Is(err, gorm.ErrRecordNotFound) {
			return ctx.JSON(http.StatusNotFound, schema.ErrorResponse{
				StatusCode: http.StatusNotFound,
				Status:     "error",
				Message:    "peer not found",
			})
		}
		c.logger.Error("failed to update wireguard peer status for reseller", zap.Error(err))
		return ctx.JSON(http.StatusInternalServerError, schema.ErrorResponse{
			StatusCode: http.StatusInternalServerError,
			Status:     "error",
			Message:    "failed to update wireguard peer status: " + err.Error(),
		})
	}

	return ctx.JSON(http.StatusOK, schema.OkBasicResponse)
}

func (c *WgPeerController) CreatePeer(ctx echo.Context) error {
	var req schema.CreatePeerRequest

	if err := ctx.Bind(&req); err != nil {
		c.logger.Warn("failed to bind request", zap.Error(err))
		return ctx.JSON(http.StatusBadRequest, schema.BadParamsErrorResponse)
	}

	if err := ctx.Validate(&req); err != nil {
		c.logger.Warn("failed to validate request", zap.Error(err))
		return ctx.JSON(http.StatusBadRequest, schema.BadParamsErrorResponse)
	}

	resellerID, err := peerScopeFromContext(ctx)
	if err != nil {
		return err
	}

	peer, err := c.peerService.CreatePeer(&req, resellerID)
	if err != nil {
		c.logger.Error("failed to create wireguard peer", zap.Error(err))
		return ctx.JSON(http.StatusInternalServerError, schema.ErrorResponse{
			StatusCode: http.StatusInternalServerError,
			Status:     "error",
			Message:    "failed to create wireguard peer: " + err.Error(),
		})
	}

	return ctx.JSON(http.StatusCreated, schema.BasicResponseData[schema.PeerResponse]{
		BasicResponse: schema.OkBasicResponse,
		Data:          *peer,
	})
}

// BulkCreatePeers mirrors V2RayPackageController.BulkCreatePackages exactly
// -- see WgPeer.BulkCreatePeers' own doc comment for the batch-creation
// behavior this powers.
func (c *WgPeerController) BulkCreatePeers(ctx echo.Context) error {
	var req schema.BulkCreatePeerRequest
	if err := ctx.Bind(&req); err != nil {
		c.logger.Warn("failed to bind request", zap.Error(err))
		return ctx.JSON(http.StatusBadRequest, schema.BadParamsErrorResponse)
	}
	if err := ctx.Validate(&req); err != nil {
		c.logger.Warn("failed to validate request", zap.Error(err))
		return ctx.JSON(http.StatusBadRequest, schema.BadParamsErrorResponse)
	}

	resellerID, err := peerScopeFromContext(ctx)
	if err != nil {
		return err
	}

	peers, err := c.peerService.BulkCreatePeers(&req, resellerID)
	if err != nil {
		c.logger.Error("failed to bulk create wireguard peers", zap.Error(err))
		return ctx.JSON(http.StatusInternalServerError, schema.ErrorResponse{
			StatusCode: http.StatusInternalServerError,
			Status:     "error",
			Message:    "failed to bulk create wireguard peers: " + err.Error(),
		})
	}

	return ctx.JSON(http.StatusCreated, schema.BasicResponseData[schema.BulkCreatePeerResponse]{
		BasicResponse: schema.OkBasicResponse,
		Data:          schema.BulkCreatePeerResponse{Peers: peers},
	})
}

func (c *WgPeerController) UpdatePeerStatus(ctx echo.Context) error {
	id := ctx.Param("id")
	if id == "" {
		c.logger.Error("Peer ID is required")
		return ctx.JSON(http.StatusBadRequest, schema.BadParamsErrorResponse)
	}

	peerId, err := strconv.Atoi(id)
	if err != nil {
		c.logger.Error("Invalid peer ID", zap.Error(err))
		return ctx.JSON(http.StatusBadRequest, schema.BadParamsErrorResponse)
	}

	resellerID, scopeErr := peerScopeFromContext(ctx)
	if scopeErr != nil {
		return scopeErr
	}

	err = c.peerService.TogglePeerStatus(uint(peerId), resellerID)
	if err != nil {
		if errors.Is(err, gorm.ErrRecordNotFound) {
			return ctx.JSON(http.StatusNotFound, schema.ErrorResponse{
				StatusCode: http.StatusNotFound,
				Status:     "error",
				Message:    "peer not found",
			})
		}
		c.logger.Error("failed to update wireguard peer status", zap.Error(err))
		return ctx.JSON(http.StatusInternalServerError, schema.ErrorResponse{
			StatusCode: http.StatusInternalServerError,
			Status:     "error",
			Message:    "failed to update wireguard peer status: " + err.Error(),
		})
	}

	return ctx.JSON(http.StatusOK, schema.OkBasicResponse)
}

func (c *WgPeerController) UpdatePeer(ctx echo.Context) error {
	id := ctx.Param("id")
	if id == "" {
		c.logger.Error("Peer ID is required")
		return ctx.JSON(http.StatusBadRequest, schema.BadParamsErrorResponse)
	}

	peerId, err := strconv.Atoi(id)
	if err != nil {
		c.logger.Error("Invalid peer ID", zap.Error(err))
		return ctx.JSON(http.StatusBadRequest, schema.BadParamsErrorResponse)
	}

	var req schema.UpdatePeerRequest
	if err := ctx.Bind(&req); err != nil {
		c.logger.Warn("failed to bind request", zap.Error(err))
		return ctx.JSON(http.StatusBadRequest, schema.BadParamsErrorResponse)
	}

	if err := ctx.Validate(&req); err != nil {
		c.logger.Warn("failed to validate request", zap.Error(err))
		return ctx.JSON(http.StatusBadRequest, schema.BadParamsErrorResponse)
	}

	resellerID, scopeErr := peerScopeFromContext(ctx)
	if scopeErr != nil {
		return scopeErr
	}

	peer, err := c.peerService.UpdatePeer(uint(peerId), &req, resellerID)
	if err != nil {
		if errors.Is(err, gorm.ErrRecordNotFound) {
			return ctx.JSON(http.StatusNotFound, schema.ErrorResponse{
				StatusCode: http.StatusNotFound,
				Status:     "error",
				Message:    "peer not found",
			})
		}
		c.logger.Error("failed to update wireguard peer", zap.Error(err))
		return ctx.JSON(http.StatusInternalServerError, schema.ErrorResponse{
			StatusCode: http.StatusInternalServerError,
			Status:     "error",
			Message:    "failed to update wireguard peer: " + err.Error(),
		})
	}

	return ctx.JSON(http.StatusOK, schema.BasicResponseData[schema.PeerResponse]{
		BasicResponse: schema.OkBasicResponse,
		Data:          *peer,
	})
}

func (c *WgPeerController) DeletePeer(ctx echo.Context) error {
	id := ctx.Param("id")
	if id == "" {
		c.logger.Error("Peer ID is required")
		return ctx.JSON(http.StatusBadRequest, schema.BadParamsErrorResponse)
	}

	peerId, err := strconv.Atoi(id)
	if err != nil {
		c.logger.Error("Invalid peer ID", zap.Error(err))
		return ctx.JSON(http.StatusBadRequest, schema.BadParamsErrorResponse)
	}

	resellerID, scopeErr := peerScopeFromContext(ctx)
	if scopeErr != nil {
		return scopeErr
	}

	err = c.peerService.DeletePeer(uint(peerId), resellerID)
	if err != nil {
		if errors.Is(err, gorm.ErrRecordNotFound) {
			return ctx.JSON(http.StatusNotFound, schema.ErrorResponse{
				StatusCode: http.StatusNotFound,
				Status:     "error",
				Message:    "peer not found",
			})
		}
		c.logger.Error("failed to delete wireguard peer", zap.Error(err))
		return ctx.JSON(http.StatusInternalServerError, schema.ErrorResponse{
			StatusCode: http.StatusInternalServerError,
			Status:     "error",
			Message:    "failed to delete wireguard peer: " + err.Error(),
		})
	}

	return ctx.JSON(http.StatusNoContent, schema.BasicResponse{
		StatusCode: http.StatusNoContent,
		Status:     "success",
	})
}

// BulkDeletePeers powers the "delete expired/quota-exhausted peers" cleanup
// action -- the frontend already knows which peers are expired/suspended
// from the status array GetPeers returns, so this endpoint just deletes a
// caller-supplied ID list scoped exactly like DeletePeer, returning a
// per-ID success/failure breakdown instead of failing the whole request if
// one peer can't be removed.
func (c *WgPeerController) BulkDeletePeers(ctx echo.Context) error {
	var req schema.BulkDeleteRequest
	if err := ctx.Bind(&req); err != nil || len(req.Ids) == 0 {
		return ctx.JSON(http.StatusBadRequest, schema.BadParamsErrorResponse)
	}

	resellerID, scopeErr := peerScopeFromContext(ctx)
	if scopeErr != nil {
		return scopeErr
	}

	deleted, failed := c.peerService.BulkDeletePeers(req.Ids, resellerID)

	failures := make([]schema.BulkDeleteFailure, 0, len(failed))
	for id, msg := range failed {
		failures = append(failures, schema.BulkDeleteFailure{Id: id, Error: msg})
	}

	return ctx.JSON(http.StatusOK, schema.BasicResponseData[schema.BulkDeleteResponse]{
		BasicResponse: schema.OkBasicResponse,
		Data: schema.BulkDeleteResponse{
			Deleted: deleted,
			Failed:  failures,
		},
	})
}

func (c *WgPeerController) GetPeerConfig(ctx echo.Context) error {
	id := ctx.Param("id")
	if id == "" {
		c.logger.Error("Peer ID is required")
		return ctx.JSON(http.StatusBadRequest, schema.BadParamsErrorResponse)
	}

	peerId, err := strconv.Atoi(id)
	if err != nil {
		c.logger.Error("Invalid peer ID", zap.Error(err))
		return ctx.JSON(http.StatusBadRequest, schema.BadParamsErrorResponse)
	}

	resellerID, scopeErr := peerScopeFromContext(ctx)
	if scopeErr != nil {
		return scopeErr
	}
	if err := c.peerService.EnsurePeerAccess(uint(peerId), resellerID); err != nil {
		if errors.Is(err, gorm.ErrRecordNotFound) {
			return ctx.JSON(http.StatusNotFound, schema.ErrorResponse{
				StatusCode: http.StatusNotFound,
				Status:     "error",
				Message:    "peer not found",
			})
		}
		return ctx.JSON(http.StatusInternalServerError, schema.InternalServerErrorResponse)
	}

	config, err := c.configGeneratorService.GetPeerConfig(uint(peerId))
	if err != nil {
		if errors.Is(err, gorm.ErrRecordNotFound) {
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

	return attachFile(ctx, config, fmt.Sprintf("peer-%d.conf", peerId))
}

func (c *WgPeerController) GetPeerQRCode(ctx echo.Context) error {
	id := ctx.Param("id")
	if id == "" {
		c.logger.Error("Peer ID is required")
		return ctx.JSON(http.StatusBadRequest, schema.BadParamsErrorResponse)
	}

	peerId, err := strconv.Atoi(id)
	if err != nil {
		c.logger.Error("Invalid peer ID", zap.Error(err))
		return ctx.JSON(http.StatusBadRequest, schema.BadParamsErrorResponse)
	}

	resellerID, scopeErr := peerScopeFromContext(ctx)
	if scopeErr != nil {
		return scopeErr
	}
	if err := c.peerService.EnsurePeerAccess(uint(peerId), resellerID); err != nil {
		if errors.Is(err, gorm.ErrRecordNotFound) {
			return ctx.JSON(http.StatusNotFound, schema.ErrorResponse{
				StatusCode: http.StatusNotFound,
				Status:     "error",
				Message:    "peer not found",
			})
		}
		return ctx.JSON(http.StatusInternalServerError, schema.InternalServerErrorResponse)
	}

	qrCode, err := c.qrCodeGeneratorService.GetPeerQRCode(uint(peerId))
	if err != nil {
		if errors.Is(err, gorm.ErrRecordNotFound) {
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

func (c *WgPeerController) GetPeerShareStatus(ctx echo.Context) error {
	id := ctx.Param("id")
	if id == "" {
		c.logger.Error("Peer ID is required")
		return ctx.JSON(http.StatusBadRequest, schema.BadParamsErrorResponse)
	}

	peerId, err := strconv.Atoi(id)
	if err != nil {
		c.logger.Error("Invalid peer ID", zap.Error(err))
		return ctx.JSON(http.StatusBadRequest, schema.BadParamsErrorResponse)
	}

	resellerID, scopeErr := peerScopeFromContext(ctx)
	if scopeErr != nil {
		return scopeErr
	}

	status, err := c.peerService.GetPeerShareStatus(uint(peerId), resellerID)
	if err != nil {
		if errors.Is(err, gorm.ErrRecordNotFound) {
			return ctx.JSON(http.StatusNotFound, schema.ErrorResponse{
				StatusCode: http.StatusNotFound,
				Status:     "error",
				Message:    "peer not found",
			})
		}

		return ctx.JSON(http.StatusInternalServerError, schema.ErrorResponse{
			StatusCode: http.StatusInternalServerError,
			Status:     "error",
			Message:    "failed to retrieve peer share status: " + err.Error(),
		})
	}

	return ctx.JSON(http.StatusOK, schema.BasicResponseData[schema.PeerShareStatusResponse]{
		BasicResponse: schema.OkBasicResponse,
		Data:          *status,
	})
}

func (c *WgPeerController) UpdatePeerShareStatus(ctx echo.Context) error {
	id := ctx.Param("id")
	if id == "" {
		c.logger.Error("Peer ID is required")
		return ctx.JSON(http.StatusBadRequest, schema.BadParamsErrorResponse)
	}

	peerId, err := strconv.Atoi(id)
	if err != nil {
		c.logger.Error("Invalid peer ID", zap.Error(err))
		return ctx.JSON(http.StatusBadRequest, schema.BadParamsErrorResponse)
	}

	resellerID, scopeErr := peerScopeFromContext(ctx)
	if scopeErr != nil {
		return scopeErr
	}

	err = c.peerService.TogglePeerShareStatus(uint(peerId), resellerID)
	if err != nil {
		if errors.Is(err, gorm.ErrRecordNotFound) {
			return ctx.JSON(http.StatusNotFound, schema.ErrorResponse{
				StatusCode: http.StatusNotFound,
				Status:     "error",
				Message:    "peer not found",
			})
		}
		c.logger.Error("failed to toggle wireguard peer share status", zap.Error(err))
		return ctx.JSON(http.StatusInternalServerError, schema.ErrorResponse{
			StatusCode: http.StatusInternalServerError,
			Status:     "error",
			Message:    "failed to toggle wireguard peer share status: " + err.Error(),
		})
	}

	return ctx.JSON(http.StatusOK, schema.OkBasicResponse)
}

func (c *WgPeerController) UpdatePeerShareExpire(ctx echo.Context) error {
	id := ctx.Param("id")
	if id == "" {
		c.logger.Error("Peer ID is required")
		return ctx.JSON(http.StatusBadRequest, schema.BadParamsErrorResponse)
	}

	peerId, err := strconv.Atoi(id)
	if err != nil {
		c.logger.Error("Invalid peer ID", zap.Error(err))
		return ctx.JSON(http.StatusBadRequest, schema.BadParamsErrorResponse)
	}

	var req schema.UpdatePeerShareExpireRequest
	if err := ctx.Bind(&req); err != nil {
		c.logger.Warn("failed to bind request", zap.Error(err))
		return ctx.JSON(http.StatusBadRequest, schema.BadParamsErrorResponse)
	}

	if err := ctx.Validate(&req); err != nil {
		c.logger.Warn("failed to validate request", zap.Error(err))
		return ctx.JSON(http.StatusBadRequest, schema.BadParamsErrorResponse)
	}

	resellerID, scopeErr := peerScopeFromContext(ctx)
	if scopeErr != nil {
		return scopeErr
	}

	err = c.peerService.UpdatePeerShareExpireTime(uint(peerId), req.ExpireTime, resellerID)
	if err != nil {
		if errors.Is(err, gorm.ErrRecordNotFound) {
			return ctx.JSON(http.StatusNotFound, schema.ErrorResponse{
				StatusCode: http.StatusNotFound,
				Status:     "error",
				Message:    "peer not found",
			})
		}
		c.logger.Error("failed to update peer share expire time", zap.Error(err))
		return ctx.JSON(http.StatusInternalServerError, schema.ErrorResponse{
			StatusCode: http.StatusInternalServerError,
			Status:     "error",
			Message:    "failed to update peer share expire time: " + err.Error(),
		})
	}

	return ctx.JSON(http.StatusOK, schema.OkBasicResponse)
}

func (c *WgPeerController) ResetPeerUsage(ctx echo.Context) error {
	id := ctx.Param("id")
	if id == "" {
		c.logger.Error("Peer ID is required")
		return ctx.JSON(http.StatusBadRequest, schema.BadParamsErrorResponse)
	}

	peerId, err := strconv.Atoi(id)
	if err != nil {
		c.logger.Error("Invalid peer ID", zap.Error(err))
		return ctx.JSON(http.StatusBadRequest, schema.BadParamsErrorResponse)
	}

	// Admin-only, per the admin's own explicit requirement: a reseller may
	// see their own usage but must never be able to zero it themselves
	// (mirrors ResetPeerUsages' identical bulk-reset gate just below, which
	// was already admin-only -- this single-peer counterpart had been
	// missed).
	if _, scopeErr := requireAdminScope(ctx); scopeErr != nil {
		return scopeErr
	}
	if err := c.peerService.EnsurePeerAccess(uint(peerId), nil); err != nil {
		if errors.Is(err, gorm.ErrRecordNotFound) {
			return ctx.JSON(http.StatusNotFound, schema.ErrorResponse{
				StatusCode: http.StatusNotFound,
				Status:     "error",
				Message:    "peer not found",
			})
		}
		return ctx.JSON(http.StatusInternalServerError, schema.InternalServerErrorResponse)
	}

	err = c.trafficCalculator.ResetPeerUsage(uint(peerId))
	if err != nil {
		c.logger.Error("failed to reset wireguard peer usage", zap.Error(err))
		return ctx.JSON(http.StatusInternalServerError, schema.ErrorResponse{
			StatusCode: http.StatusInternalServerError,
			Status:     "error",
			Message:    "failed to reset wireguard peer usage: " + err.Error(),
		})
	}

	return ctx.JSON(http.StatusOK, schema.OkBasicResponse)
}

func (c *WgPeerController) ResetPeerUsages(ctx echo.Context) error {
	if role, _ := getRoleAndResellerFromContext(ctx); role == "reseller" {
		return ctx.JSON(http.StatusForbidden, schema.ErrorResponse{StatusCode: http.StatusForbidden, Status: "error", Message: "forbidden"})
	}

	err := c.trafficCalculator.ResetPeerUsages()
	if err != nil {
		c.logger.Error("failed to reset wireguard peer usages", zap.Error(err))
		return ctx.JSON(http.StatusInternalServerError, schema.ErrorResponse{
			StatusCode: http.StatusInternalServerError,
			Status:     "error",
			Message:    "failed to reset wireguard peer usages: " + err.Error(),
		})
	}

	return ctx.JSON(http.StatusOK, schema.OkBasicResponse)
}

func (c *WgPeerController) ExportPeersTrafficData(ctx echo.Context) error {
	if role, _ := getRoleAndResellerFromContext(ctx); role == "reseller" {
		return ctx.JSON(http.StatusForbidden, schema.ErrorResponse{StatusCode: http.StatusForbidden, Status: "error", Message: "forbidden"})
	}

	filePath, err := c.excelGeneratorService.GetTrafficUsageReport()
	if err != nil {
		c.logger.Error("failed to export peers traffic data", zap.Error(err))
		return ctx.JSON(http.StatusInternalServerError, schema.ErrorResponse{
			StatusCode: http.StatusInternalServerError,
			Status:     "error",
			Message:    "failed to export peers traffic data: " + err.Error(),
		})
	}

	return ctx.File(filePath)
}

// GetResellerActivitySummary returns the admin-wide per-reseller online/usage
// breakdown. Admin only.
func (c *WgPeerController) GetResellerActivitySummary(ctx echo.Context) error {
	if role, _ := getRoleAndResellerFromContext(ctx); role == "reseller" {
		return ctx.JSON(http.StatusForbidden, schema.ErrorResponse{StatusCode: http.StatusForbidden, Status: "error", Message: "forbidden"})
	}

	page := 1
	if p, err := strconv.Atoi(ctx.QueryParam("page")); err == nil && p > 0 {
		page = p
	}
	pageSize := 20
	if ps, err := strconv.Atoi(ctx.QueryParam("page_size")); err == nil && ps > 0 {
		pageSize = ps
	}

	summary, err := c.peerService.GetResellerActivitySummary(page, pageSize)
	if err != nil {
		c.logger.Error("failed to get reseller activity summary", zap.Error(err))
		return ctx.JSON(http.StatusInternalServerError, schema.ErrorResponse{
			StatusCode: http.StatusInternalServerError,
			Status:     "error",
			Message:    "failed to retrieve reseller activity summary: " + err.Error(),
		})
	}

	return ctx.JSON(http.StatusOK, schema.BasicResponseData[schema.ResellerActivitySummaryResponse]{
		BasicResponse: schema.OkBasicResponse,
		Data:          *summary,
	})
}

// GetSelfActivity returns the caller reseller's own online-peer count and
// today's usage. Reseller only (admins have no single-reseller context).
func (c *WgPeerController) GetSelfActivity(ctx echo.Context) error {
	role, resellerID := getRoleAndResellerFromContext(ctx)
	if role != "reseller" || resellerID == nil {
		return ctx.JSON(http.StatusForbidden, schema.ErrorResponse{StatusCode: http.StatusForbidden, Status: "error", Message: "forbidden"})
	}

	activity, err := c.peerService.GetSelfActivity(*resellerID)
	if err != nil {
		c.logger.Error("failed to get self activity", zap.Error(err))
		return ctx.JSON(http.StatusInternalServerError, schema.ErrorResponse{
			StatusCode: http.StatusInternalServerError,
			Status:     "error",
			Message:    "failed to retrieve activity: " + err.Error(),
		})
	}

	return ctx.JSON(http.StatusOK, schema.BasicResponseData[schema.SelfActivityResponse]{
		BasicResponse: schema.OkBasicResponse,
		Data:          *activity,
	})
}

// peerScopeFromContext returns errAlreadyHandled (not ctx.JSON's own
// return value -- see errAlreadyHandled's doc comment in reseller.go for
// why) once it has written a 403 for a malformed reseller token.
func peerScopeFromContext(ctx echo.Context) (*uint, error) {
	role, resellerID := getRoleAndResellerFromContext(ctx)
	if role == "reseller" {
		if resellerID == nil {
			_ = ctx.JSON(http.StatusForbidden, schema.ErrorResponse{StatusCode: http.StatusForbidden, Status: "error", Message: "forbidden"})
			return nil, errAlreadyHandled
		}

		return resellerID, nil
	}

	return nil, nil
}

// requireAdminScope is the shared guard for an action the admin's own
// explicit requirement restricts to admin-only, even though the surrounding
// route group is otherwise reachable by both roles (e.g. WireGuard/
// UserManager/V2Ray/DNS's "Reset Usage" action -- a reseller must be able to
// see their own usage, but only the admin may zero it, since a reseller
// resetting their own package right before a billing cycle would otherwise
// let them erase their own overage). Mirrors peerScopeFromContext's
// errAlreadyHandled convention exactly. Returns nil (admin) on success --
// callers pass that straight through to the *Scoped service call the same
// way peerScopeFromContext's own nil result already does for every other
// admin-reachable action.
func requireAdminScope(ctx echo.Context) (*uint, error) {
	role, _ := getRoleAndResellerFromContext(ctx)
	if role == "reseller" {
		_ = ctx.JSON(http.StatusForbidden, schema.ErrorResponse{StatusCode: http.StatusForbidden, Status: "error", Message: "فقط ادمین مجاز به ریست مصرف است"})
		return nil, errAlreadyHandled
	}

	return nil, nil
}
