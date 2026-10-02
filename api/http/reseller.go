package http

import (
	"errors"
	"net/http"
	"strconv"

	"github.com/golang-jwt/jwt/v5"
	"github.com/maahdima/mwp/api/http/schema"
	"github.com/maahdima/mwp/api/service"

	"github.com/labstack/echo/v4"
	"go.uber.org/zap"
	"gorm.io/gorm"
)

type ResellerController struct {
	resellerService        *service.Reseller
	resellerBillingService *service.ResellerBillingService
	logger                 *zap.Logger
}

func NewResellerController(resellerService *service.Reseller, resellerBillingService *service.ResellerBillingService) *ResellerController {
	return &ResellerController{
		resellerService:        resellerService,
		resellerBillingService: resellerBillingService,
		logger:                 zap.L().Named("ResellerController"),
	}
}

func (c *ResellerController) CreateReseller(ctx echo.Context) error {
	// only admin users can create resellers
	if role, _ := getRoleFromContext(ctx); role != "admin" {
		return ctx.JSON(http.StatusForbidden, schema.ErrorResponse{StatusCode: http.StatusForbidden, Status: "error", Message: "forbidden"})
	}
	var req schema.CreateResellerRequest
	if err := ctx.Bind(&req); err != nil {
		c.logger.Error("failed to bind request", zap.Error(err))
		return ctx.JSON(http.StatusBadRequest, schema.BadParamsErrorResponse)
	}
	if err := ctx.Validate(&req); err != nil {
		c.logger.Warn("failed to validate request", zap.Error(err))
		return ctx.JSON(http.StatusBadRequest, schema.BadParamsErrorResponse)
	}

	res, err := c.resellerService.CreateReseller(&req)
	if err != nil {
		c.logger.Error("failed to create reseller", zap.Error(err))
		// Surfaces the real reason (e.g. "username already exists") instead
		// of the generic InternalServerErrorResponse -- a confirmed,
		// reported bug: the frontend's save dialog always showed the same
		// opaque "Failed to save reseller" toast no matter what actually
		// went wrong, because this handler discarded err's own message.
		// Matches how SetAssignedInterfaces/SetAssignedUserManagerGroups/
		// SetAssignedUserManagerProfiles/SetAssignedXuiPanels in this same
		// file already correctly report err.Error() below.
		return ctx.JSON(http.StatusBadRequest, schema.ErrorResponse{
			StatusCode: http.StatusBadRequest,
			Status:     "error",
			Message:    err.Error(),
		})
	}

	return ctx.JSON(http.StatusOK, schema.BasicResponseData[schema.ResellerResponse]{
		BasicResponse: schema.OkBasicResponse,
		Data:          *res,
	})
}

func (c *ResellerController) GetReseller(ctx echo.Context) error {
	id := ctx.Param("id")
	if id == "" {
		return ctx.JSON(http.StatusBadRequest, schema.BadParamsErrorResponse)
	}
	nid, err := strconv.Atoi(id)
	if err != nil {
		return ctx.JSON(http.StatusBadRequest, schema.BadParamsErrorResponse)
	}

	// allow admin or the reseller owner
	role, resellerClaim := getRoleAndResellerFromContext(ctx)
	if role == "reseller" {
		if resellerClaim == nil || *resellerClaim != uint(nid) {
			return ctx.JSON(http.StatusForbidden, schema.ErrorResponse{StatusCode: http.StatusForbidden, Status: "error", Message: "forbidden"})
		}
	}

	res, err := c.resellerService.GetReseller(uint(nid))
	if err != nil {
		if errors.Is(err, gorm.ErrRecordNotFound) {
			return ctx.JSON(http.StatusNotFound, schema.ErrorResponse{StatusCode: http.StatusNotFound, Status: "error", Message: "reseller not found"})
		}
		c.logger.Error("failed to fetch reseller", zap.Error(err))
		return ctx.JSON(http.StatusInternalServerError, schema.InternalServerErrorResponse)
	}

	return ctx.JSON(http.StatusOK, schema.BasicResponseData[schema.ResellerResponse]{
		BasicResponse: schema.OkBasicResponse,
		Data:          *res,
	})
}

func (c *ResellerController) UpdateReseller(ctx echo.Context) error {
	id := ctx.Param("id")
	if id == "" {
		return ctx.JSON(http.StatusBadRequest, schema.BadParamsErrorResponse)
	}
	nid, err := strconv.Atoi(id)
	if err != nil {
		return ctx.JSON(http.StatusBadRequest, schema.BadParamsErrorResponse)
	}

	var req schema.UpdateResellerRequest
	if err := ctx.Bind(&req); err != nil {
		return ctx.JSON(http.StatusBadRequest, schema.BadParamsErrorResponse)
	}
	if err := ctx.Validate(&req); err != nil {
		c.logger.Warn("failed to validate update reseller request", zap.Error(err))
		return ctx.JSON(http.StatusBadRequest, schema.BadParamsErrorResponse)
	}

	// allow admin or the reseller owner
	role, resellerClaim := getRoleAndResellerFromContext(ctx)
	if role == "reseller" {
		if resellerClaim == nil || *resellerClaim != uint(nid) {
			return ctx.JSON(http.StatusForbidden, schema.ErrorResponse{StatusCode: http.StatusForbidden, Status: "error", Message: "forbidden"})
		}
		// reseller users can update their own profile fields but not account state or quota.
		req.IsActive = nil
		req.QuotaBytes = nil
	}

	res, err := c.resellerService.UpdateReseller(uint(nid), &req)
	if err != nil {
		if errors.Is(err, gorm.ErrRecordNotFound) {
			return ctx.JSON(http.StatusNotFound, schema.ErrorResponse{StatusCode: http.StatusNotFound, Status: "error", Message: "reseller not found"})
		}
		c.logger.Error("failed to update reseller", zap.Error(err))
		// See CreateReseller's identical fix above -- surfaces the real
		// reason (e.g. "username already exists") instead of the generic
		// InternalServerErrorResponse.
		return ctx.JSON(http.StatusBadRequest, schema.ErrorResponse{
			StatusCode: http.StatusBadRequest,
			Status:     "error",
			Message:    err.Error(),
		})
	}

	return ctx.JSON(http.StatusOK, schema.BasicResponseData[schema.ResellerResponse]{
		BasicResponse: schema.OkBasicResponse,
		Data:          *res,
	})
}

// CompleteOnboarding lets a reseller latch their own
// HasCompletedOnboarding flag to true after dismissing/finishing the
// first-login wizard -- mirrors UpdateReseller's ownership guard (a
// reseller may only complete their OWN onboarding), but admins are also
// allowed through (e.g. support marking it done on a reseller's behalf).
func (c *ResellerController) CompleteOnboarding(ctx echo.Context) error {
	id := ctx.Param("id")
	if id == "" {
		return ctx.JSON(http.StatusBadRequest, schema.BadParamsErrorResponse)
	}
	nid, err := strconv.Atoi(id)
	if err != nil {
		return ctx.JSON(http.StatusBadRequest, schema.BadParamsErrorResponse)
	}

	role, resellerClaim := getRoleAndResellerFromContext(ctx)
	if role == "reseller" {
		if resellerClaim == nil || *resellerClaim != uint(nid) {
			return ctx.JSON(http.StatusForbidden, schema.ErrorResponse{StatusCode: http.StatusForbidden, Status: "error", Message: "forbidden"})
		}
	}

	res, err := c.resellerService.CompleteOnboarding(uint(nid))
	if err != nil {
		if errors.Is(err, gorm.ErrRecordNotFound) {
			return ctx.JSON(http.StatusNotFound, schema.ErrorResponse{StatusCode: http.StatusNotFound, Status: "error", Message: "reseller not found"})
		}
		c.logger.Error("failed to complete reseller onboarding", zap.Error(err))
		return ctx.JSON(http.StatusInternalServerError, schema.InternalServerErrorResponse)
	}

	return ctx.JSON(http.StatusOK, schema.BasicResponseData[schema.ResellerResponse]{
		BasicResponse: schema.OkBasicResponse,
		Data:          *res,
	})
}

func (c *ResellerController) ListResellers(ctx echo.Context) error {
	// only admin can list all resellers
	if role, _ := getRoleFromContext(ctx); role != "admin" {
		return ctx.JSON(http.StatusForbidden, schema.ErrorResponse{StatusCode: http.StatusForbidden, Status: "error", Message: "forbidden"})
	}

	list, err := c.resellerService.ListResellers()
	if err != nil {
		c.logger.Error("failed to list resellers", zap.Error(err))
		return ctx.JSON(http.StatusInternalServerError, schema.InternalServerErrorResponse)
	}

	return ctx.JSON(http.StatusOK, schema.BasicResponseData[[]schema.ResellerResponse]{
		BasicResponse: schema.OkBasicResponse,
		Data:          list,
	})
}

func (c *ResellerController) DeleteReseller(ctx echo.Context) error {
	id := ctx.Param("id")
	if id == "" {
		return ctx.JSON(http.StatusBadRequest, schema.BadParamsErrorResponse)
	}
	nid, err := strconv.Atoi(id)
	if err != nil {
		return ctx.JSON(http.StatusBadRequest, schema.BadParamsErrorResponse)
	}

	if role, _ := getRoleFromContext(ctx); role != "admin" {
		return ctx.JSON(http.StatusForbidden, schema.ErrorResponse{StatusCode: http.StatusForbidden, Status: "error", Message: "forbidden"})
	}

	if err := c.resellerService.DeleteReseller(uint(nid)); err != nil {
		if errors.Is(err, gorm.ErrRecordNotFound) {
			return ctx.JSON(http.StatusNotFound, schema.ErrorResponse{
				StatusCode: http.StatusNotFound,
				Status:     "error",
				Message:    "reseller not found",
			})
		}
		c.logger.Error("failed to delete reseller", zap.Uint("id", uint(nid)), zap.Error(err))
		return ctx.JSON(http.StatusInternalServerError, schema.InternalServerErrorResponse)
	}

	return ctx.JSON(http.StatusOK, schema.OkBasicResponse)
}

// GetAssignedInterfaces returns the interface IDs a reseller may use.
// Admins can inspect any reseller; resellers can only inspect their own assignment.
func (c *ResellerController) GetAssignedInterfaces(ctx echo.Context) error {
	id := ctx.Param("id")
	if id == "" {
		return ctx.JSON(http.StatusBadRequest, schema.BadParamsErrorResponse)
	}
	nid, err := strconv.Atoi(id)
	if err != nil {
		return ctx.JSON(http.StatusBadRequest, schema.BadParamsErrorResponse)
	}

	role, resellerClaim := getRoleAndResellerFromContext(ctx)
	if role == "reseller" && (resellerClaim == nil || *resellerClaim != uint(nid)) {
		return ctx.JSON(http.StatusForbidden, schema.ErrorResponse{StatusCode: http.StatusForbidden, Status: "error", Message: "forbidden"})
	}

	ids, err := c.resellerService.GetAssignedInterfaceIDs(uint(nid))
	if err != nil {
		c.logger.Error("failed to get assigned interfaces", zap.Uint("id", uint(nid)), zap.Error(err))
		return ctx.JSON(http.StatusInternalServerError, schema.InternalServerErrorResponse)
	}

	return ctx.JSON(http.StatusOK, schema.BasicResponseData[[]uint]{
		BasicResponse: schema.OkBasicResponse,
		Data:          ids,
	})
}

// SetAssignedInterfaces replaces the interfaces a reseller is allowed to use. Admin only.
func (c *ResellerController) SetAssignedInterfaces(ctx echo.Context) error {
	id := ctx.Param("id")
	if id == "" {
		return ctx.JSON(http.StatusBadRequest, schema.BadParamsErrorResponse)
	}
	nid, err := strconv.Atoi(id)
	if err != nil {
		return ctx.JSON(http.StatusBadRequest, schema.BadParamsErrorResponse)
	}

	if role, _ := getRoleFromContext(ctx); role != "admin" {
		return ctx.JSON(http.StatusForbidden, schema.ErrorResponse{StatusCode: http.StatusForbidden, Status: "error", Message: "forbidden"})
	}

	var req schema.AssignResellerInterfacesRequest
	if err := ctx.Bind(&req); err != nil {
		c.logger.Error("failed to bind request", zap.Error(err))
		return ctx.JSON(http.StatusBadRequest, schema.BadParamsErrorResponse)
	}

	if err := c.resellerService.SetAssignedInterfaces(uint(nid), req.InterfaceIDs); err != nil {
		c.logger.Error("failed to set assigned interfaces", zap.Uint("id", uint(nid)), zap.Error(err))
		return ctx.JSON(http.StatusInternalServerError, schema.ErrorResponse{
			StatusCode: http.StatusInternalServerError,
			Status:     "error",
			Message:    err.Error(),
		})
	}

	return ctx.JSON(http.StatusOK, schema.OkBasicResponse)
}

// GetBillingPrices returns this reseller's configured Payment-based
// per-product Toman-per-GB prices. Admin only, same as every other
// reseller-configuration endpoint in this controller.
func (c *ResellerController) GetBillingPrices(ctx echo.Context) error {
	id := ctx.Param("id")
	nid, err := strconv.Atoi(id)
	if err != nil {
		return ctx.JSON(http.StatusBadRequest, schema.BadParamsErrorResponse)
	}

	if role, _ := getRoleFromContext(ctx); role != "admin" {
		return ctx.JSON(http.StatusForbidden, schema.ErrorResponse{StatusCode: http.StatusForbidden, Status: "error", Message: "forbidden"})
	}

	prices, err := c.resellerBillingService.GetPrices(uint(nid))
	if err != nil {
		c.logger.Error("failed to fetch reseller billing prices", zap.Uint("id", uint(nid)), zap.Error(err))
		return ctx.JSON(http.StatusInternalServerError, schema.ErrorResponse{
			StatusCode: http.StatusInternalServerError,
			Status:     "error",
			Message:    err.Error(),
		})
	}

	return ctx.JSON(http.StatusOK, schema.BasicResponseData[[]schema.ResellerBillingPriceResponse]{
		BasicResponse: schema.OkBasicResponse,
		Data:          prices,
	})
}

// SetBillingPrices upserts this reseller's Payment-based per-product
// Toman-per-GB prices. Admin only.
func (c *ResellerController) SetBillingPrices(ctx echo.Context) error {
	id := ctx.Param("id")
	nid, err := strconv.Atoi(id)
	if err != nil {
		return ctx.JSON(http.StatusBadRequest, schema.BadParamsErrorResponse)
	}

	if role, _ := getRoleFromContext(ctx); role != "admin" {
		return ctx.JSON(http.StatusForbidden, schema.ErrorResponse{StatusCode: http.StatusForbidden, Status: "error", Message: "forbidden"})
	}

	var req schema.UpdateResellerBillingPricesRequest
	if err := ctx.Bind(&req); err != nil {
		c.logger.Error("failed to bind request", zap.Error(err))
		return ctx.JSON(http.StatusBadRequest, schema.BadParamsErrorResponse)
	}
	if err := ctx.Validate(&req); err != nil {
		c.logger.Warn("failed to validate request", zap.Error(err))
		return ctx.JSON(http.StatusBadRequest, schema.BadParamsErrorResponse)
	}

	if err := c.resellerBillingService.SetPrices(uint(nid), req.Prices); err != nil {
		c.logger.Error("failed to set reseller billing prices", zap.Uint("id", uint(nid)), zap.Error(err))
		return ctx.JSON(http.StatusInternalServerError, schema.ErrorResponse{
			StatusCode: http.StatusInternalServerError,
			Status:     "error",
			Message:    err.Error(),
		})
	}

	return ctx.JSON(http.StatusOK, schema.OkBasicResponse)
}

// GetBillingTiers returns this reseller's configured volume-discount
// tiers for every product -- see model.ResellerBillingTier's own doc
// comment. Admin only, same as every other reseller-configuration
// endpoint in this controller.
func (c *ResellerController) GetBillingTiers(ctx echo.Context) error {
	id := ctx.Param("id")
	nid, err := strconv.Atoi(id)
	if err != nil {
		return ctx.JSON(http.StatusBadRequest, schema.BadParamsErrorResponse)
	}

	if role, _ := getRoleFromContext(ctx); role != "admin" {
		return ctx.JSON(http.StatusForbidden, schema.ErrorResponse{StatusCode: http.StatusForbidden, Status: "error", Message: "forbidden"})
	}

	tiers, err := c.resellerBillingService.GetTiers(uint(nid))
	if err != nil {
		c.logger.Error("failed to fetch reseller billing tiers", zap.Uint("id", uint(nid)), zap.Error(err))
		return ctx.JSON(http.StatusInternalServerError, schema.ErrorResponse{
			StatusCode: http.StatusInternalServerError,
			Status:     "error",
			Message:    err.Error(),
		})
	}

	return ctx.JSON(http.StatusOK, schema.BasicResponseData[[]schema.ResellerBillingTierResponse]{
		BasicResponse: schema.OkBasicResponse,
		Data:          tiers,
	})
}

// SetBillingTiers replaces this reseller's volume-discount tiers for one
// product (see ResellerBillingService.SetTiers's own doc comment on why
// this is a full replace per product). Admin only.
func (c *ResellerController) SetBillingTiers(ctx echo.Context) error {
	id := ctx.Param("id")
	nid, err := strconv.Atoi(id)
	if err != nil {
		return ctx.JSON(http.StatusBadRequest, schema.BadParamsErrorResponse)
	}

	if role, _ := getRoleFromContext(ctx); role != "admin" {
		return ctx.JSON(http.StatusForbidden, schema.ErrorResponse{StatusCode: http.StatusForbidden, Status: "error", Message: "forbidden"})
	}

	var req schema.UpdateResellerBillingTiersRequest
	if err := ctx.Bind(&req); err != nil {
		c.logger.Error("failed to bind request", zap.Error(err))
		return ctx.JSON(http.StatusBadRequest, schema.BadParamsErrorResponse)
	}
	if err := ctx.Validate(&req); err != nil {
		c.logger.Warn("failed to validate request", zap.Error(err))
		return ctx.JSON(http.StatusBadRequest, schema.BadParamsErrorResponse)
	}

	if err := c.resellerBillingService.SetTiers(uint(nid), req.Product, req.Tiers); err != nil {
		c.logger.Error("failed to set reseller billing tiers", zap.Uint("id", uint(nid)), zap.String("product", req.Product), zap.Error(err))
		return ctx.JSON(http.StatusInternalServerError, schema.ErrorResponse{
			StatusCode: http.StatusInternalServerError,
			Status:     "error",
			Message:    err.Error(),
		})
	}

	return ctx.JSON(http.StatusOK, schema.OkBasicResponse)
}

// GetAssignedUserManagerGroups returns the RouterOS User Manager group
// names a reseller is allowed to use. Admin or the reseller owner.
func (c *ResellerController) GetAssignedUserManagerGroups(ctx echo.Context) error {
	id := ctx.Param("id")
	if id == "" {
		return ctx.JSON(http.StatusBadRequest, schema.BadParamsErrorResponse)
	}
	nid, err := strconv.Atoi(id)
	if err != nil {
		return ctx.JSON(http.StatusBadRequest, schema.BadParamsErrorResponse)
	}

	role, resellerClaim := getRoleAndResellerFromContext(ctx)
	if role == "reseller" && (resellerClaim == nil || *resellerClaim != uint(nid)) {
		return ctx.JSON(http.StatusForbidden, schema.ErrorResponse{StatusCode: http.StatusForbidden, Status: "error", Message: "forbidden"})
	}

	names, err := c.resellerService.GetAssignedUserManagerGroups(uint(nid))
	if err != nil {
		c.logger.Error("failed to get assigned user manager groups", zap.Uint("id", uint(nid)), zap.Error(err))
		return ctx.JSON(http.StatusInternalServerError, schema.InternalServerErrorResponse)
	}

	return ctx.JSON(http.StatusOK, schema.BasicResponseData[[]string]{
		BasicResponse: schema.OkBasicResponse,
		Data:          names,
	})
}

// SetAssignedUserManagerGroups replaces the User Manager groups a reseller
// is allowed to use. Admin only.
func (c *ResellerController) SetAssignedUserManagerGroups(ctx echo.Context) error {
	id := ctx.Param("id")
	if id == "" {
		return ctx.JSON(http.StatusBadRequest, schema.BadParamsErrorResponse)
	}
	nid, err := strconv.Atoi(id)
	if err != nil {
		return ctx.JSON(http.StatusBadRequest, schema.BadParamsErrorResponse)
	}

	if role, _ := getRoleFromContext(ctx); role != "admin" {
		return ctx.JSON(http.StatusForbidden, schema.ErrorResponse{StatusCode: http.StatusForbidden, Status: "error", Message: "forbidden"})
	}

	var req schema.AssignResellerUserManagerGroupsRequest
	if err := ctx.Bind(&req); err != nil {
		c.logger.Error("failed to bind request", zap.Error(err))
		return ctx.JSON(http.StatusBadRequest, schema.BadParamsErrorResponse)
	}

	if err := c.resellerService.SetAssignedUserManagerGroups(uint(nid), req.GroupNames); err != nil {
		c.logger.Error("failed to set assigned user manager groups", zap.Uint("id", uint(nid)), zap.Error(err))
		return ctx.JSON(http.StatusInternalServerError, schema.ErrorResponse{
			StatusCode: http.StatusInternalServerError,
			Status:     "error",
			Message:    err.Error(),
		})
	}

	return ctx.JSON(http.StatusOK, schema.OkBasicResponse)
}

// GetAssignedUserManagerProfiles mirrors GetAssignedUserManagerGroups
// exactly, for RouterOS User Manager profiles.
func (c *ResellerController) GetAssignedUserManagerProfiles(ctx echo.Context) error {
	id := ctx.Param("id")
	if id == "" {
		return ctx.JSON(http.StatusBadRequest, schema.BadParamsErrorResponse)
	}
	nid, err := strconv.Atoi(id)
	if err != nil {
		return ctx.JSON(http.StatusBadRequest, schema.BadParamsErrorResponse)
	}

	role, resellerClaim := getRoleAndResellerFromContext(ctx)
	if role == "reseller" && (resellerClaim == nil || *resellerClaim != uint(nid)) {
		return ctx.JSON(http.StatusForbidden, schema.ErrorResponse{StatusCode: http.StatusForbidden, Status: "error", Message: "forbidden"})
	}

	names, err := c.resellerService.GetAssignedUserManagerProfiles(uint(nid))
	if err != nil {
		c.logger.Error("failed to get assigned user manager profiles", zap.Uint("id", uint(nid)), zap.Error(err))
		return ctx.JSON(http.StatusInternalServerError, schema.InternalServerErrorResponse)
	}

	return ctx.JSON(http.StatusOK, schema.BasicResponseData[[]string]{
		BasicResponse: schema.OkBasicResponse,
		Data:          names,
	})
}

// SetAssignedUserManagerProfiles mirrors SetAssignedUserManagerGroups
// exactly, for RouterOS User Manager profiles. Admin only.
func (c *ResellerController) SetAssignedUserManagerProfiles(ctx echo.Context) error {
	id := ctx.Param("id")
	if id == "" {
		return ctx.JSON(http.StatusBadRequest, schema.BadParamsErrorResponse)
	}
	nid, err := strconv.Atoi(id)
	if err != nil {
		return ctx.JSON(http.StatusBadRequest, schema.BadParamsErrorResponse)
	}

	if role, _ := getRoleFromContext(ctx); role != "admin" {
		return ctx.JSON(http.StatusForbidden, schema.ErrorResponse{StatusCode: http.StatusForbidden, Status: "error", Message: "forbidden"})
	}

	var req schema.AssignResellerUserManagerProfilesRequest
	if err := ctx.Bind(&req); err != nil {
		c.logger.Error("failed to bind request", zap.Error(err))
		return ctx.JSON(http.StatusBadRequest, schema.BadParamsErrorResponse)
	}

	if err := c.resellerService.SetAssignedUserManagerProfiles(uint(nid), req.ProfileNames); err != nil {
		c.logger.Error("failed to set assigned user manager profiles", zap.Uint("id", uint(nid)), zap.Error(err))
		return ctx.JSON(http.StatusInternalServerError, schema.ErrorResponse{
			StatusCode: http.StatusInternalServerError,
			Status:     "error",
			Message:    err.Error(),
		})
	}

	return ctx.JSON(http.StatusOK, schema.OkBasicResponse)
}

func getRoleFromContext(ctx echo.Context) (string, *uint) {
	user := ctx.Get("user")
	if user == nil {
		return "", nil
	}
	token, ok := user.(*jwt.Token)
	if !ok {
		return "", nil
	}
	claims, ok := token.Claims.(jwt.MapClaims)
	if !ok {
		return "", nil
	}
	role, _ := claims["role"].(string)
	var rid *uint
	if v, ok := claims["reseller_id"]; ok {
		switch x := v.(type) {
		case float64:
			id := uint(x)
			rid = &id
		case int:
			id := uint(x)
			rid = &id
		}
	}
	return role, rid
}

func getRoleAndResellerFromContext(ctx echo.Context) (string, *uint) {
	return getRoleFromContext(ctx)
}

// errAlreadyHandled is returned by guard helpers (adminResellerIDFromParam,
// peerScopeFromContext) after they have already written a JSON error
// response (403/400) to the client. ctx.JSON's own return value is NOT a
// usable "did this fail?" signal -- it reports whether the HTTP write
// itself succeeded, which is nil (no error) precisely in the success case
// where the 403/400 body WAS written correctly. Returning that nil
// straight to the caller silently defeats every `if err != nil { return
// err }` guard in this file: execution falls through into service code
// with a zero-value resellerID, on a possibly-nil service pointer. Every
// guard helper in this file must return errAlreadyHandled instead of
// ctx.JSON's own result, specifically so a non-nil error is guaranteed
// whenever a response was already written.
var errAlreadyHandled = errors.New("request already handled")

// adminResellerIDFromParam is the shared guard for every admin-on-behalf-
// of-reseller endpoint (e.g. "/peer/reseller/:reseller_id", "/user-manager/
// account/reseller/:reseller_id"): it forbids reseller-role callers outright
// (a reseller must only ever use their own-scoped route, never this one,
// even for their own reseller_id) and parses the :reseller_id path param.
// Callers use it as:
//
//	resellerID, err := adminResellerIDFromParam(ctx)
//	if err != nil {
//		return err
//	}
//
// where err is errAlreadyHandled once a 403/400 has already been written
// to ctx -- see errAlreadyHandled's own doc comment for why this must NOT
// be ctx.JSON's own return value.
func adminResellerIDFromParam(ctx echo.Context) (uint, error) {
	if role, _ := getRoleAndResellerFromContext(ctx); role == "reseller" {
		_ = ctx.JSON(http.StatusForbidden, schema.ErrorResponse{
			StatusCode: http.StatusForbidden,
			Status:     "error",
			Message:    "forbidden",
		})
		return 0, errAlreadyHandled
	}

	resellerID, err := strconv.ParseUint(ctx.Param("reseller_id"), 10, 32)
	if err != nil {
		_ = ctx.JSON(http.StatusBadRequest, schema.BadParamsErrorResponse)
		return 0, errAlreadyHandled
	}

	return uint(resellerID), nil
}
