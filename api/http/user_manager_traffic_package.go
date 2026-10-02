package http

import (
	"errors"
	"net/http"
	"strconv"

	"github.com/maahdima/mwp/api/dataservice/model"
	"github.com/maahdima/mwp/api/http/schema"
	"github.com/maahdima/mwp/api/service"

	"github.com/labstack/echo/v4"
	"go.uber.org/zap"
	"gorm.io/gorm"
)

// UserManagerTrafficPackageController mirrors TrafficPackageController
// exactly, for the separate User Manager traffic-package pool.
type UserManagerTrafficPackageController struct {
	packageService  *service.UserManagerTrafficPackageService
	purchaseService *service.UserManagerPackagePurchaseService
	logger          *zap.Logger
}

func NewUserManagerTrafficPackageController(packageService *service.UserManagerTrafficPackageService, purchaseService *service.UserManagerPackagePurchaseService) *UserManagerTrafficPackageController {
	return &UserManagerTrafficPackageController{
		packageService:  packageService,
		purchaseService: purchaseService,
		logger:          zap.L().Named("UserManagerTrafficPackageController"),
	}
}

func toUserManagerTrafficPackageResponse(pkg *model.UserManagerTrafficPackage) schema.UserManagerTrafficPackageResponse {
	return schema.UserManagerTrafficPackageResponse{
		ID:           pkg.ID,
		Name:         pkg.Name,
		Description:  pkg.Description,
		TrafficBytes: pkg.TrafficBytes,
		PriceAmount:  pkg.PriceAmount,
		IsActive:     pkg.IsActive,
	}
}

func (c *UserManagerTrafficPackageController) ListTrafficPackages(ctx echo.Context) error {
	if ctx.QueryParam("all") == "true" {
		if forbidden, err := forbidUnlessAdmin(ctx); forbidden {
			return err
		}

		pkgs, err := c.packageService.ListAllTrafficPackages()
		if err != nil {
			c.logger.Error("failed to list all user manager traffic packages", zap.Error(err))
			return ctx.JSON(http.StatusInternalServerError, schema.InternalServerErrorResponse)
		}

		resp := make([]schema.UserManagerTrafficPackageResponse, 0, len(pkgs))
		for i := range pkgs {
			resp = append(resp, toUserManagerTrafficPackageResponse(&pkgs[i]))
		}
		return ctx.JSON(http.StatusOK, schema.BasicResponseData[[]schema.UserManagerTrafficPackageResponse]{
			BasicResponse: schema.OkBasicResponse,
			Data:          resp,
		})
	}

	pkgs, err := c.packageService.ListActiveTrafficPackages()
	if err != nil {
		c.logger.Error("failed to list user manager traffic packages", zap.Error(err))
		return ctx.JSON(http.StatusInternalServerError, schema.InternalServerErrorResponse)
	}

	resp := make([]schema.UserManagerTrafficPackageResponse, 0, len(pkgs))
	for i := range pkgs {
		resp = append(resp, toUserManagerTrafficPackageResponse(&pkgs[i]))
	}
	return ctx.JSON(http.StatusOK, schema.BasicResponseData[[]schema.UserManagerTrafficPackageResponse]{
		BasicResponse: schema.OkBasicResponse,
		Data:          resp,
	})
}

func (c *UserManagerTrafficPackageController) CreateTrafficPackage(ctx echo.Context) error {
	if forbidden, err := forbidUnlessAdmin(ctx); forbidden {
		return err
	}

	var req schema.CreateUserManagerTrafficPackageRequest
	if err := ctx.Bind(&req); err != nil {
		c.logger.Warn("failed to bind request", zap.Error(err))
		return ctx.JSON(http.StatusBadRequest, schema.BadParamsErrorResponse)
	}
	if err := ctx.Validate(&req); err != nil {
		c.logger.Warn("failed to validate request", zap.Error(err))
		return ctx.JSON(http.StatusBadRequest, schema.BadParamsErrorResponse)
	}

	pkg, err := c.packageService.CreateTrafficPackage(req.Name, req.Description, req.TrafficBytes, req.PriceAmount)
	if err != nil {
		c.logger.Error("failed to create user manager traffic package", zap.Error(err))
		return ctx.JSON(http.StatusInternalServerError, schema.ErrorResponse{
			StatusCode: http.StatusInternalServerError,
			Status:     "error",
			Message:    err.Error(),
		})
	}

	return ctx.JSON(http.StatusOK, schema.BasicResponseData[schema.UserManagerTrafficPackageResponse]{
		BasicResponse: schema.OkBasicResponse,
		Data:          toUserManagerTrafficPackageResponse(pkg),
	})
}

func (c *UserManagerTrafficPackageController) UpdateTrafficPackage(ctx echo.Context) error {
	if forbidden, err := forbidUnlessAdmin(ctx); forbidden {
		return err
	}

	id, err := strconv.ParseUint(ctx.Param("id"), 10, 32)
	if err != nil {
		return ctx.JSON(http.StatusBadRequest, schema.BadParamsErrorResponse)
	}

	var req schema.UpdateUserManagerTrafficPackageRequest
	if err := ctx.Bind(&req); err != nil {
		c.logger.Warn("failed to bind request", zap.Error(err))
		return ctx.JSON(http.StatusBadRequest, schema.BadParamsErrorResponse)
	}
	if err := ctx.Validate(&req); err != nil {
		c.logger.Warn("failed to validate request", zap.Error(err))
		return ctx.JSON(http.StatusBadRequest, schema.BadParamsErrorResponse)
	}

	pkg, err := c.packageService.UpdateTrafficPackage(uint(id), req.Name, req.Description, req.TrafficBytes, req.PriceAmount, req.IsActive)
	if err != nil {
		if errors.Is(err, gorm.ErrRecordNotFound) {
			return ctx.JSON(http.StatusNotFound, schema.ErrorResponse{StatusCode: http.StatusNotFound, Status: "error", Message: "traffic package not found"})
		}
		c.logger.Error("failed to update user manager traffic package", zap.Error(err))
		return ctx.JSON(http.StatusInternalServerError, schema.ErrorResponse{
			StatusCode: http.StatusInternalServerError,
			Status:     "error",
			Message:    err.Error(),
		})
	}

	return ctx.JSON(http.StatusOK, schema.BasicResponseData[schema.UserManagerTrafficPackageResponse]{
		BasicResponse: schema.OkBasicResponse,
		Data:          toUserManagerTrafficPackageResponse(pkg),
	})
}

func (c *UserManagerTrafficPackageController) DeleteTrafficPackage(ctx echo.Context) error {
	if forbidden, err := forbidUnlessAdmin(ctx); forbidden {
		return err
	}

	id, err := strconv.ParseUint(ctx.Param("id"), 10, 32)
	if err != nil {
		return ctx.JSON(http.StatusBadRequest, schema.BadParamsErrorResponse)
	}

	if err := c.packageService.DeleteTrafficPackage(uint(id)); err != nil {
		if errors.Is(err, gorm.ErrRecordNotFound) {
			return ctx.JSON(http.StatusNotFound, schema.ErrorResponse{
				StatusCode: http.StatusNotFound,
				Status:     "error",
				Message:    "traffic package not found",
			})
		}
		c.logger.Error("failed to delete user manager traffic package", zap.Error(err))
		return ctx.JSON(http.StatusInternalServerError, schema.InternalServerErrorResponse)
	}

	return ctx.JSON(http.StatusOK, schema.OkBasicResponse)
}

func toUserManagerPackagePurchaseResponse(p *model.UserManagerPackagePurchase) schema.UserManagerPackagePurchaseResponse {
	return schema.UserManagerPackagePurchaseResponse{
		ID:                 p.ID,
		ResellerID:         p.ResellerID,
		TrafficPackageID:   p.TrafficPackageID,
		TrafficPackageName: p.TrafficPackageName,
		TrafficBytes:       p.TrafficBytes,
		PriceAmount:        p.PriceAmount,
		CreatedAt:          p.CreatedAt.Unix(),
	}
}

// PurchasePackage lets a reseller buy a User Manager package for themselves.
func (c *UserManagerTrafficPackageController) PurchasePackage(ctx echo.Context) error {
	role, resellerClaim := getRoleAndResellerFromContext(ctx)
	if role != "reseller" || resellerClaim == nil {
		return ctx.JSON(http.StatusForbidden, schema.ErrorResponse{StatusCode: http.StatusForbidden, Status: "error", Message: "forbidden"})
	}

	var req schema.PurchaseUserManagerPackageRequest
	if err := ctx.Bind(&req); err != nil {
		c.logger.Warn("failed to bind request", zap.Error(err))
		return ctx.JSON(http.StatusBadRequest, schema.BadParamsErrorResponse)
	}
	if err := ctx.Validate(&req); err != nil {
		c.logger.Warn("failed to validate request", zap.Error(err))
		return ctx.JSON(http.StatusBadRequest, schema.BadParamsErrorResponse)
	}

	purchase, err := c.purchaseService.PurchasePackage(*resellerClaim, req.TrafficPackageID)
	if err != nil {
		if errors.Is(err, gorm.ErrRecordNotFound) {
			return ctx.JSON(http.StatusNotFound, schema.ErrorResponse{StatusCode: http.StatusNotFound, Status: "error", Message: "traffic package not found"})
		}
		c.logger.Warn("failed to purchase user manager traffic package", zap.Uint("reseller_id", *resellerClaim), zap.Error(err))
		return ctx.JSON(http.StatusBadRequest, schema.ErrorResponse{
			StatusCode: http.StatusBadRequest,
			Status:     "error",
			Message:    err.Error(),
		})
	}

	return ctx.JSON(http.StatusOK, schema.BasicResponseData[schema.UserManagerPackagePurchaseResponse]{
		BasicResponse: schema.OkBasicResponse,
		Data:          toUserManagerPackagePurchaseResponse(purchase),
	})
}

// ListMyPurchases returns the caller reseller's own User Manager package
// purchase history.
func (c *UserManagerTrafficPackageController) ListMyPurchases(ctx echo.Context) error {
	role, resellerClaim := getRoleAndResellerFromContext(ctx)
	if role != "reseller" || resellerClaim == nil {
		return ctx.JSON(http.StatusForbidden, schema.ErrorResponse{StatusCode: http.StatusForbidden, Status: "error", Message: "forbidden"})
	}

	purchases, err := c.purchaseService.ListPurchases(*resellerClaim, 50)
	if err != nil {
		c.logger.Error("failed to list user manager package purchases", zap.Error(err))
		return ctx.JSON(http.StatusInternalServerError, schema.InternalServerErrorResponse)
	}

	resp := make([]schema.UserManagerPackagePurchaseResponse, 0, len(purchases))
	for i := range purchases {
		resp = append(resp, toUserManagerPackagePurchaseResponse(&purchases[i]))
	}
	return ctx.JSON(http.StatusOK, schema.BasicResponseData[[]schema.UserManagerPackagePurchaseResponse]{
		BasicResponse: schema.OkBasicResponse,
		Data:          resp,
	})
}
