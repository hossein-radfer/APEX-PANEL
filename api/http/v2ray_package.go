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

// V2RayPackageController manages V2Ray packages -- structurally mirrors
// UserManagerAccountController, reusing the package-level
// errAlreadyHandled/getRoleAndResellerFromContext/peerScopeFromContext/
// adminResellerIDFromParam helpers directly since they're role/claims-only,
// not peer-specific.
type V2RayPackageController struct {
	packageService *service.V2RayPackageService
	logger         *zap.Logger
}

func NewV2RayPackageController(packageService *service.V2RayPackageService) *V2RayPackageController {
	return &V2RayPackageController{
		packageService: packageService,
		logger:         zap.L().Named("V2RayPackageController"),
	}
}

// GetSelfSummary returns the caller reseller's own V2Ray package
// count/usage, for the reseller dashboard. Reseller only, mirroring
// UserManagerAccountController.GetSelfSummary's exact guard.
func (c *V2RayPackageController) GetSelfSummary(ctx echo.Context) error {
	role, resellerID := getRoleAndResellerFromContext(ctx)
	if role != "reseller" || resellerID == nil {
		return ctx.JSON(http.StatusForbidden, schema.ErrorResponse{StatusCode: http.StatusForbidden, Status: "error", Message: "forbidden"})
	}

	summary, err := c.packageService.GetSelfSummary(*resellerID)
	if err != nil {
		c.logger.Error("failed to get v2ray self summary", zap.Error(err))
		return ctx.JSON(http.StatusInternalServerError, schema.ErrorResponse{
			StatusCode: http.StatusInternalServerError,
			Status:     "error",
			Message:    "failed to retrieve summary: " + err.Error(),
		})
	}

	return ctx.JSON(http.StatusOK, schema.BasicResponseData[schema.V2RaySelfSummaryResponse]{
		BasicResponse: schema.OkBasicResponse,
		Data:          *summary,
	})
}

// GetAdminSummary returns the panel-wide V2Ray rollup for the admin
// dashboard. Admin only.
func (c *V2RayPackageController) GetAdminSummary(ctx echo.Context) error {
	if role, _ := getRoleAndResellerFromContext(ctx); role == "reseller" {
		return ctx.JSON(http.StatusForbidden, schema.ErrorResponse{StatusCode: http.StatusForbidden, Status: "error", Message: "forbidden"})
	}

	summary, err := c.packageService.GetAdminSummary()
	if err != nil {
		c.logger.Error("failed to get v2ray admin summary", zap.Error(err))
		return ctx.JSON(http.StatusInternalServerError, schema.ErrorResponse{
			StatusCode: http.StatusInternalServerError,
			Status:     "error",
			Message:    "failed to retrieve summary: " + err.Error(),
		})
	}

	return ctx.JSON(http.StatusOK, schema.BasicResponseData[schema.V2RayAdminSummaryResponse]{
		BasicResponse: schema.OkBasicResponse,
		Data:          *summary,
	})
}

func (c *V2RayPackageController) ListPackages(ctx echo.Context) error {
	resellerID, err := peerScopeFromContext(ctx)
	if err != nil {
		return err
	}

	packages, err := c.packageService.ListPackages(resellerID)
	if err != nil {
		c.logger.Error("failed to list v2ray packages", zap.Error(err))
		return ctx.JSON(http.StatusInternalServerError, schema.ErrorResponse{
			StatusCode: http.StatusInternalServerError,
			Status:     "error",
			Message:    "failed to retrieve v2ray packages: " + err.Error(),
		})
	}

	return ctx.JSON(http.StatusOK, schema.BasicResponseData[[]schema.V2RayPackageResponse]{
		BasicResponse: schema.OkBasicResponse,
		Data:          packages,
	})
}

// ListPackagesByReseller returns all packages owned by a specific reseller. Admin only.
func (c *V2RayPackageController) ListPackagesByReseller(ctx echo.Context) error {
	if role, _ := getRoleAndResellerFromContext(ctx); role == "reseller" {
		return ctx.JSON(http.StatusForbidden, schema.ErrorResponse{StatusCode: http.StatusForbidden, Status: "error", Message: "forbidden"})
	}

	resellerIDStr := ctx.Param("reseller_id")
	resellerID, err := strconv.ParseUint(resellerIDStr, 10, 32)
	if err != nil {
		return ctx.JSON(http.StatusBadRequest, schema.BadParamsErrorResponse)
	}

	packages, err := c.packageService.ListPackagesByReseller(uint(resellerID))
	if err != nil {
		c.logger.Error("failed to list reseller v2ray packages", zap.Error(err))
		return ctx.JSON(http.StatusInternalServerError, schema.ErrorResponse{
			StatusCode: http.StatusInternalServerError,
			Status:     "error",
			Message:    "failed to retrieve reseller v2ray packages: " + err.Error(),
		})
	}

	return ctx.JSON(http.StatusOK, schema.BasicResponseData[[]schema.V2RayPackageResponse]{
		BasicResponse: schema.OkBasicResponse,
		Data:          packages,
	})
}

func (c *V2RayPackageController) CreatePackage(ctx echo.Context) error {
	var req schema.CreateV2RayPackageRequest
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

	pkg, err := c.packageService.CreatePackage(&req, resellerID)
	if err != nil {
		c.logger.Error("failed to create v2ray package", zap.Error(err))
		return ctx.JSON(http.StatusInternalServerError, schema.ErrorResponse{
			StatusCode: http.StatusInternalServerError,
			Status:     "error",
			Message:    "failed to create v2ray package: " + err.Error(),
		})
	}

	return ctx.JSON(http.StatusCreated, schema.BasicResponseData[schema.V2RayPackageResponse]{
		BasicResponse: schema.OkBasicResponse,
		Data:          *pkg,
	})
}

// BulkCreatePackages creates several packages at once, sharing volume/
// duration/panel selection, each with its own randomly-generated
// CustomerLabel -- see V2RayPackageService.BulkCreatePackages's own doc
// comment. Uses peerScopeFromContext exactly like CreatePackage above (the
// plain, un-suffixed endpoint auto-scopes for both an admin and a
// reseller caller), so this single route serves both, matching how the
// package-mutation endpoints already work.
func (c *V2RayPackageController) BulkCreatePackages(ctx echo.Context) error {
	var req schema.BulkCreateV2RayPackageRequest
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

	packages, err := c.packageService.BulkCreatePackages(&req, resellerID)
	if err != nil {
		c.logger.Error("failed to bulk create v2ray packages", zap.Error(err))
		return ctx.JSON(http.StatusInternalServerError, schema.ErrorResponse{
			StatusCode: http.StatusInternalServerError,
			Status:     "error",
			Message:    "failed to bulk create v2ray packages: " + err.Error(),
		})
	}

	return ctx.JSON(http.StatusCreated, schema.BasicResponseData[schema.BulkCreateV2RayPackageResponse]{
		BasicResponse: schema.OkBasicResponse,
		Data:          schema.BulkCreateV2RayPackageResponse{Packages: packages},
	})
}

// ExportPackages streams an .xlsx or .txt file listing the requested
// packages' CustomerLabel and selected link(s) -- see
// V2RayPackageService.ExportPackages's own doc comment. Every package ID
// is individually access-checked, so a reseller can never export a
// package outside their own scope.
func (c *V2RayPackageController) ExportPackages(ctx echo.Context) error {
	var req schema.ExportV2RayPackagesRequest
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

	path, cleanup, err := c.packageService.ExportPackages(&req, resellerID)
	if err != nil {
		c.logger.Warn("failed to export v2ray packages", zap.Error(err))
		return ctx.JSON(http.StatusBadRequest, schema.ErrorResponse{
			StatusCode: http.StatusBadRequest,
			Status:     "error",
			Message:    err.Error(),
		})
	}
	defer cleanup()

	filename := "v2ray-packages.xlsx"
	if req.Format == "txt" {
		filename = "v2ray-packages.txt"
	}
	return ctx.Attachment(path, filename)
}

// CreatePackageForReseller lets an admin create a package on behalf of a specific reseller.
func (c *V2RayPackageController) CreatePackageForReseller(ctx echo.Context) error {
	resellerID, err := adminResellerIDFromParam(ctx)
	if err != nil {
		return err
	}

	var req schema.CreateV2RayPackageRequest
	if err := ctx.Bind(&req); err != nil {
		c.logger.Warn("failed to bind request", zap.Error(err))
		return ctx.JSON(http.StatusBadRequest, schema.BadParamsErrorResponse)
	}
	if err := ctx.Validate(&req); err != nil {
		c.logger.Warn("failed to validate request", zap.Error(err))
		return ctx.JSON(http.StatusBadRequest, schema.BadParamsErrorResponse)
	}

	pkg, err := c.packageService.CreatePackage(&req, &resellerID)
	if err != nil {
		c.logger.Error("failed to create v2ray package for reseller", zap.Error(err))
		return ctx.JSON(http.StatusInternalServerError, schema.ErrorResponse{
			StatusCode: http.StatusInternalServerError,
			Status:     "error",
			Message:    "failed to create v2ray package: " + err.Error(),
		})
	}

	return ctx.JSON(http.StatusCreated, schema.BasicResponseData[schema.V2RayPackageResponse]{
		BasicResponse: schema.OkBasicResponse,
		Data:          *pkg,
	})
}

func (c *V2RayPackageController) UpdatePackage(ctx echo.Context) error {
	id, err := strconv.Atoi(ctx.Param("id"))
	if err != nil {
		return ctx.JSON(http.StatusBadRequest, schema.BadParamsErrorResponse)
	}

	var req schema.UpdateV2RayPackageRequest
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

	pkg, err := c.packageService.UpdatePackage(uint(id), &req, resellerID)
	if err != nil {
		if errors.Is(err, gorm.ErrRecordNotFound) {
			return ctx.JSON(http.StatusNotFound, schema.ErrorResponse{StatusCode: http.StatusNotFound, Status: "error", Message: "v2ray package not found"})
		}
		c.logger.Error("failed to update v2ray package", zap.Error(err))
		return ctx.JSON(http.StatusInternalServerError, schema.ErrorResponse{
			StatusCode: http.StatusInternalServerError,
			Status:     "error",
			Message:    "failed to update v2ray package: " + err.Error(),
		})
	}

	return ctx.JSON(http.StatusOK, schema.BasicResponseData[schema.V2RayPackageResponse]{
		BasicResponse: schema.OkBasicResponse,
		Data:          *pkg,
	})
}

func (c *V2RayPackageController) UpdatePackageForReseller(ctx echo.Context) error {
	resellerID, err := adminResellerIDFromParam(ctx)
	if err != nil {
		return err
	}

	id, err := strconv.Atoi(ctx.Param("id"))
	if err != nil {
		return ctx.JSON(http.StatusBadRequest, schema.BadParamsErrorResponse)
	}

	var req schema.UpdateV2RayPackageRequest
	if err := ctx.Bind(&req); err != nil {
		c.logger.Warn("failed to bind request", zap.Error(err))
		return ctx.JSON(http.StatusBadRequest, schema.BadParamsErrorResponse)
	}
	if err := ctx.Validate(&req); err != nil {
		c.logger.Warn("failed to validate request", zap.Error(err))
		return ctx.JSON(http.StatusBadRequest, schema.BadParamsErrorResponse)
	}

	pkg, err := c.packageService.UpdatePackage(uint(id), &req, &resellerID)
	if err != nil {
		if errors.Is(err, gorm.ErrRecordNotFound) {
			return ctx.JSON(http.StatusNotFound, schema.ErrorResponse{StatusCode: http.StatusNotFound, Status: "error", Message: "v2ray package not found"})
		}
		c.logger.Error("failed to update reseller's v2ray package", zap.Error(err))
		return ctx.JSON(http.StatusInternalServerError, schema.ErrorResponse{
			StatusCode: http.StatusInternalServerError,
			Status:     "error",
			Message:    "failed to update v2ray package: " + err.Error(),
		})
	}

	return ctx.JSON(http.StatusOK, schema.BasicResponseData[schema.V2RayPackageResponse]{
		BasicResponse: schema.OkBasicResponse,
		Data:          *pkg,
	})
}

// ResetPackageUsage mirrors WgPeerController.ResetPeerUsage/
// UserManagerAccountController.ResetAccountUsage -- the admin's own
// explicit "V2Ray needs a Reset Usage button just like WireGuard" request.
// See V2RayPackageService.ResetUsage's own doc comment for why this is
// offset-based rather than a direct UsedBytesCached reset.
func (c *V2RayPackageController) ResetPackageUsage(ctx echo.Context) error {
	id, err := strconv.Atoi(ctx.Param("id"))
	if err != nil {
		return ctx.JSON(http.StatusBadRequest, schema.BadParamsErrorResponse)
	}

	// Admin-only, per the admin's own explicit requirement -- see
	// requireAdminScope's own doc comment (wg_peer.go) for why a reseller
	// must never be able to zero their own usage.
	if _, scopeErr := requireAdminScope(ctx); scopeErr != nil {
		return scopeErr
	}

	if err := c.packageService.ResetUsage(uint(id), nil); err != nil {
		if errors.Is(err, gorm.ErrRecordNotFound) {
			return ctx.JSON(http.StatusNotFound, schema.ErrorResponse{StatusCode: http.StatusNotFound, Status: "error", Message: "v2ray package not found"})
		}
		c.logger.Error("failed to reset v2ray package usage", zap.Error(err))
		return ctx.JSON(http.StatusInternalServerError, schema.ErrorResponse{
			StatusCode: http.StatusInternalServerError,
			Status:     "error",
			Message:    "failed to reset package usage: " + err.Error(),
		})
	}

	return ctx.JSON(http.StatusOK, schema.OkBasicResponse)
}

func (c *V2RayPackageController) DeletePackage(ctx echo.Context) error {
	id, err := strconv.Atoi(ctx.Param("id"))
	if err != nil {
		return ctx.JSON(http.StatusBadRequest, schema.BadParamsErrorResponse)
	}

	resellerID, scopeErr := peerScopeFromContext(ctx)
	if scopeErr != nil {
		return scopeErr
	}

	if err := c.packageService.DeletePackage(uint(id), resellerID); err != nil {
		if errors.Is(err, gorm.ErrRecordNotFound) {
			return ctx.JSON(http.StatusNotFound, schema.ErrorResponse{StatusCode: http.StatusNotFound, Status: "error", Message: "v2ray package not found"})
		}
		c.logger.Error("failed to delete v2ray package", zap.Error(err))
		return ctx.JSON(http.StatusInternalServerError, schema.ErrorResponse{
			StatusCode: http.StatusInternalServerError,
			Status:     "error",
			Message:    "failed to delete v2ray package: " + err.Error(),
		})
	}

	// NoContent, not JSON: a 204 response must not carry a body (RFC 9110
	// §15.3.5) -- ctx.JSON here would write one anyway and Echo logs a
	// spurious warning for it on every call.
	return ctx.NoContent(http.StatusNoContent)
}

// BulkDeletePackages mirrors WgPeerController.BulkDeletePeers exactly --
// see its doc comment for the "delete expired/quota-exhausted" action this
// powers.
func (c *V2RayPackageController) BulkDeletePackages(ctx echo.Context) error {
	var req schema.BulkDeleteRequest
	if err := ctx.Bind(&req); err != nil || len(req.Ids) == 0 {
		return ctx.JSON(http.StatusBadRequest, schema.BadParamsErrorResponse)
	}

	resellerID, scopeErr := peerScopeFromContext(ctx)
	if scopeErr != nil {
		return scopeErr
	}

	deleted, failed := c.packageService.BulkDeletePackages(req.Ids, resellerID)

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

func (c *V2RayPackageController) DeletePackageForReseller(ctx echo.Context) error {
	resellerID, err := adminResellerIDFromParam(ctx)
	if err != nil {
		return err
	}

	id, err := strconv.Atoi(ctx.Param("id"))
	if err != nil {
		return ctx.JSON(http.StatusBadRequest, schema.BadParamsErrorResponse)
	}

	if err := c.packageService.DeletePackage(uint(id), &resellerID); err != nil {
		if errors.Is(err, gorm.ErrRecordNotFound) {
			return ctx.JSON(http.StatusNotFound, schema.ErrorResponse{StatusCode: http.StatusNotFound, Status: "error", Message: "v2ray package not found"})
		}
		c.logger.Error("failed to delete reseller's v2ray package", zap.Error(err))
		return ctx.JSON(http.StatusInternalServerError, schema.ErrorResponse{
			StatusCode: http.StatusInternalServerError,
			Status:     "error",
			Message:    "failed to delete v2ray package: " + err.Error(),
		})
	}

	// NoContent, not JSON: a 204 response must not carry a body (RFC 9110
	// §15.3.5) -- ctx.JSON here would write one anyway and Echo logs a
	// spurious warning for it on every call.
	return ctx.NoContent(http.StatusNoContent)
}

// GetLiveUsage handles the on-demand "view live usage" action -- unlike
// every other read in this controller, the underlying service call makes
// LIVE xui.GetClientTrafficsSplit requests right now (see
// V2RayPackageService.GetLiveUsage's own doc comment), so this is
// deliberately not folded into the ordinary ListPackages response.
func (c *V2RayPackageController) GetLiveUsage(ctx echo.Context) error {
	id, err := strconv.Atoi(ctx.Param("id"))
	if err != nil {
		return ctx.JSON(http.StatusBadRequest, schema.BadParamsErrorResponse)
	}

	resellerID, scopeErr := peerScopeFromContext(ctx)
	if scopeErr != nil {
		return scopeErr
	}

	usage, err := c.packageService.GetLiveUsage(uint(id), resellerID)
	if err != nil {
		if errors.Is(err, gorm.ErrRecordNotFound) {
			return ctx.JSON(http.StatusNotFound, schema.ErrorResponse{StatusCode: http.StatusNotFound, Status: "error", Message: "v2ray package not found"})
		}
		c.logger.Error("failed to fetch v2ray package live usage", zap.Error(err))
		return ctx.JSON(http.StatusInternalServerError, schema.ErrorResponse{
			StatusCode: http.StatusInternalServerError,
			Status:     "error",
			Message:    "failed to fetch live usage: " + err.Error(),
		})
	}

	return ctx.JSON(http.StatusOK, schema.BasicResponseData[schema.V2RayLiveUsageResponse]{
		BasicResponse: schema.OkBasicResponse,
		Data:          *usage,
	})
}

func (c *V2RayPackageController) GetPackageShareStatus(ctx echo.Context) error {
	id, err := strconv.Atoi(ctx.Param("id"))
	if err != nil {
		return ctx.JSON(http.StatusBadRequest, schema.BadParamsErrorResponse)
	}

	resellerID, scopeErr := peerScopeFromContext(ctx)
	if scopeErr != nil {
		return scopeErr
	}

	status, err := c.packageService.GetPackageShareStatus(uint(id), resellerID)
	if err != nil {
		if errors.Is(err, gorm.ErrRecordNotFound) {
			return ctx.JSON(http.StatusNotFound, schema.ErrorResponse{StatusCode: http.StatusNotFound, Status: "error", Message: "v2ray package not found"})
		}
		return ctx.JSON(http.StatusInternalServerError, schema.ErrorResponse{
			StatusCode: http.StatusInternalServerError,
			Status:     "error",
			Message:    "failed to retrieve share status: " + err.Error(),
		})
	}

	return ctx.JSON(http.StatusOK, schema.BasicResponseData[schema.V2RayPackageShareStatusResponse]{
		BasicResponse: schema.OkBasicResponse,
		Data:          *status,
	})
}

func (c *V2RayPackageController) UpdatePackageShareStatus(ctx echo.Context) error {
	id, err := strconv.Atoi(ctx.Param("id"))
	if err != nil {
		return ctx.JSON(http.StatusBadRequest, schema.BadParamsErrorResponse)
	}

	resellerID, scopeErr := peerScopeFromContext(ctx)
	if scopeErr != nil {
		return scopeErr
	}

	if err := c.packageService.UpdatePackageShareStatus(uint(id), resellerID); err != nil {
		if errors.Is(err, gorm.ErrRecordNotFound) {
			return ctx.JSON(http.StatusNotFound, schema.ErrorResponse{StatusCode: http.StatusNotFound, Status: "error", Message: "v2ray package not found"})
		}
		c.logger.Error("failed to toggle v2ray package share status", zap.Error(err))
		return ctx.JSON(http.StatusInternalServerError, schema.ErrorResponse{
			StatusCode: http.StatusInternalServerError,
			Status:     "error",
			Message:    "failed to toggle share status: " + err.Error(),
		})
	}

	return ctx.JSON(http.StatusOK, schema.OkBasicResponse)
}

func (c *V2RayPackageController) UpdatePackageShareExpire(ctx echo.Context) error {
	id, err := strconv.Atoi(ctx.Param("id"))
	if err != nil {
		return ctx.JSON(http.StatusBadRequest, schema.BadParamsErrorResponse)
	}

	var body struct {
		ExpireTime *string `json:"expire_time"`
	}
	if err := ctx.Bind(&body); err != nil {
		c.logger.Warn("failed to bind request", zap.Error(err))
		return ctx.JSON(http.StatusBadRequest, schema.BadParamsErrorResponse)
	}

	resellerID, scopeErr := peerScopeFromContext(ctx)
	if scopeErr != nil {
		return scopeErr
	}

	if err := c.packageService.UpdatePackageShareExpire(uint(id), body.ExpireTime, resellerID); err != nil {
		if errors.Is(err, gorm.ErrRecordNotFound) {
			return ctx.JSON(http.StatusNotFound, schema.ErrorResponse{StatusCode: http.StatusNotFound, Status: "error", Message: "v2ray package not found"})
		}
		c.logger.Error("failed to update v2ray package share expire time", zap.Error(err))
		return ctx.JSON(http.StatusInternalServerError, schema.ErrorResponse{
			StatusCode: http.StatusInternalServerError,
			Status:     "error",
			Message:    "failed to update share expire time: " + err.Error(),
		})
	}

	return ctx.JSON(http.StatusOK, schema.OkBasicResponse)
}

// SetSaleTitle upserts the caller reseller's custom sale title for one
// panel. Reseller only -- an admin sets titles on a reseller's behalf via
// SetSaleTitleForReseller instead.
func (c *V2RayPackageController) SetSaleTitle(ctx echo.Context) error {
	role, resellerClaim := getRoleAndResellerFromContext(ctx)
	if role != "reseller" || resellerClaim == nil {
		return ctx.JSON(http.StatusForbidden, schema.ErrorResponse{StatusCode: http.StatusForbidden, Status: "error", Message: "forbidden"})
	}

	var req schema.V2RaySaleTitleRequest
	if err := ctx.Bind(&req); err != nil {
		c.logger.Warn("failed to bind request", zap.Error(err))
		return ctx.JSON(http.StatusBadRequest, schema.BadParamsErrorResponse)
	}
	if err := ctx.Validate(&req); err != nil {
		c.logger.Warn("failed to validate request", zap.Error(err))
		return ctx.JSON(http.StatusBadRequest, schema.BadParamsErrorResponse)
	}

	if err := c.packageService.SetSaleTitle(*resellerClaim, &req); err != nil {
		c.logger.Error("failed to set v2ray sale title", zap.Error(err))
		return ctx.JSON(http.StatusInternalServerError, schema.InternalServerErrorResponse)
	}

	return ctx.JSON(http.StatusOK, schema.OkBasicResponse)
}

func (c *V2RayPackageController) SetSaleTitleForReseller(ctx echo.Context) error {
	resellerID, err := adminResellerIDFromParam(ctx)
	if err != nil {
		return err
	}

	var req schema.V2RaySaleTitleRequest
	if err := ctx.Bind(&req); err != nil {
		c.logger.Warn("failed to bind request", zap.Error(err))
		return ctx.JSON(http.StatusBadRequest, schema.BadParamsErrorResponse)
	}
	if err := ctx.Validate(&req); err != nil {
		c.logger.Warn("failed to validate request", zap.Error(err))
		return ctx.JSON(http.StatusBadRequest, schema.BadParamsErrorResponse)
	}

	if err := c.packageService.SetSaleTitle(resellerID, &req); err != nil {
		c.logger.Error("failed to set v2ray sale title for reseller", zap.Error(err))
		return ctx.JSON(http.StatusInternalServerError, schema.InternalServerErrorResponse)
	}

	return ctx.JSON(http.StatusOK, schema.OkBasicResponse)
}

// GetAssignedXuiPanels returns the x-ui panel IDs a reseller is allowed to
// use. Admin, or the reseller owner (mirrors ResellerController.
// GetAssignedUserManagerGroups's exact access rule).
func (c *V2RayPackageController) GetAssignedXuiPanels(ctx echo.Context) error {
	resellerIDStr := ctx.Param("reseller_id")
	nid, err := strconv.ParseUint(resellerIDStr, 10, 32)
	if err != nil {
		return ctx.JSON(http.StatusBadRequest, schema.BadParamsErrorResponse)
	}

	role, resellerClaim := getRoleAndResellerFromContext(ctx)
	if role == "reseller" && (resellerClaim == nil || *resellerClaim != uint(nid)) {
		return ctx.JSON(http.StatusForbidden, schema.ErrorResponse{StatusCode: http.StatusForbidden, Status: "error", Message: "forbidden"})
	}

	panelIDs, err := c.packageService.GetAssignedXuiPanels(uint(nid))
	if err != nil {
		c.logger.Error("failed to get assigned xui panels", zap.Uint64("reseller_id", nid), zap.Error(err))
		return ctx.JSON(http.StatusInternalServerError, schema.InternalServerErrorResponse)
	}

	return ctx.JSON(http.StatusOK, schema.BasicResponseData[[]uint]{
		BasicResponse: schema.OkBasicResponse,
		Data:          panelIDs,
	})
}

// GetAssignedXuiPanelSummaries is GetAssignedXuiPanels' reseller-safe
// counterpart, returning id+name instead of bare IDs -- see
// V2RayPackageService.GetAssignedXuiPanelSummaries' doc comment for why
// this exists. Same access rule as GetAssignedXuiPanels (admin or the
// reseller owner).
func (c *V2RayPackageController) GetAssignedXuiPanelSummaries(ctx echo.Context) error {
	resellerIDStr := ctx.Param("reseller_id")
	nid, err := strconv.ParseUint(resellerIDStr, 10, 32)
	if err != nil {
		return ctx.JSON(http.StatusBadRequest, schema.BadParamsErrorResponse)
	}

	role, resellerClaim := getRoleAndResellerFromContext(ctx)
	if role == "reseller" && (resellerClaim == nil || *resellerClaim != uint(nid)) {
		return ctx.JSON(http.StatusForbidden, schema.ErrorResponse{StatusCode: http.StatusForbidden, Status: "error", Message: "forbidden"})
	}

	panels, err := c.packageService.GetAssignedXuiPanelSummaries(uint(nid))
	if err != nil {
		c.logger.Error("failed to get assigned xui panel summaries", zap.Uint64("reseller_id", nid), zap.Error(err))
		return ctx.JSON(http.StatusInternalServerError, schema.InternalServerErrorResponse)
	}

	summaries := make([]schema.XuiPanelSummary, 0, len(panels))
	for _, panel := range panels {
		summaries = append(summaries, schema.XuiPanelSummary{Id: panel.ID, Name: panel.Name})
	}

	return ctx.JSON(http.StatusOK, schema.BasicResponseData[[]schema.XuiPanelSummary]{
		BasicResponse: schema.OkBasicResponse,
		Data:          summaries,
	})
}

// SetAssignedXuiPanels replaces the full set of x-ui panels a reseller is
// allowed to use. Admin only (mirrors ResellerController.
// SetAssignedUserManagerGroups's exact access rule).
func (c *V2RayPackageController) SetAssignedXuiPanels(ctx echo.Context) error {
	if role, _ := getRoleFromContext(ctx); role != "admin" {
		return ctx.JSON(http.StatusForbidden, schema.ErrorResponse{StatusCode: http.StatusForbidden, Status: "error", Message: "forbidden"})
	}

	resellerIDStr := ctx.Param("reseller_id")
	nid, err := strconv.ParseUint(resellerIDStr, 10, 32)
	if err != nil {
		return ctx.JSON(http.StatusBadRequest, schema.BadParamsErrorResponse)
	}

	var req schema.AssignResellerXuiPanelsRequest
	if err := ctx.Bind(&req); err != nil {
		c.logger.Warn("failed to bind request", zap.Error(err))
		return ctx.JSON(http.StatusBadRequest, schema.BadParamsErrorResponse)
	}

	if err := c.packageService.SetAssignedXuiPanels(uint(nid), req.PanelIDs); err != nil {
		c.logger.Error("failed to set assigned xui panels", zap.Uint64("reseller_id", nid), zap.Error(err))
		return ctx.JSON(http.StatusInternalServerError, schema.ErrorResponse{
			StatusCode: http.StatusInternalServerError,
			Status:     "error",
			Message:    err.Error(),
		})
	}

	return ctx.JSON(http.StatusOK, schema.OkBasicResponse)
}

func (c *V2RayPackageController) ListSaleTitles(ctx echo.Context) error {
	role, resellerClaim := getRoleAndResellerFromContext(ctx)
	if role != "reseller" || resellerClaim == nil {
		return ctx.JSON(http.StatusForbidden, schema.ErrorResponse{StatusCode: http.StatusForbidden, Status: "error", Message: "forbidden"})
	}

	titles, err := c.packageService.ListSaleTitles(*resellerClaim)
	if err != nil {
		c.logger.Error("failed to list v2ray sale titles", zap.Error(err))
		return ctx.JSON(http.StatusInternalServerError, schema.InternalServerErrorResponse)
	}

	return ctx.JSON(http.StatusOK, schema.BasicResponseData[[]schema.V2RaySaleTitleResponse]{
		BasicResponse: schema.OkBasicResponse,
		Data:          titles,
	})
}
