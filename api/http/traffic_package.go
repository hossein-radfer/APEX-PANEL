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

type TrafficPackageController struct {
	packageService  *service.TrafficPackageService
	purchaseService *service.PackagePurchaseService
	logger          *zap.Logger
}

func NewTrafficPackageController(packageService *service.TrafficPackageService, purchaseService *service.PackagePurchaseService) *TrafficPackageController {
	return &TrafficPackageController{
		packageService:  packageService,
		purchaseService: purchaseService,
		logger:          zap.L().Named("TrafficPackageController"),
	}
}

func toTrafficPackageResponse(pkg *model.TrafficPackage) schema.TrafficPackageResponse {
	return schema.TrafficPackageResponse{
		ID:           pkg.ID,
		Name:         pkg.Name,
		Description:  pkg.Description,
		TrafficBytes: pkg.TrafficBytes,
		PriceAmount:  pkg.PriceAmount,
		IsActive:     pkg.IsActive,
	}
}

// ListTrafficPackages returns every active package. Both admins and
// resellers can read this (resellers need it to pick a package to buy); pass
// ?all=true (admin only) to include deactivated packages for management.
func (c *TrafficPackageController) ListTrafficPackages(ctx echo.Context) error {
	if ctx.QueryParam("all") == "true" {
		if forbidden, err := forbidUnlessAdmin(ctx); forbidden {
			return err
		}

		pkgs, err := c.packageService.ListAllTrafficPackages()
		if err != nil {
			c.logger.Error("failed to list all traffic packages", zap.Error(err))
			return ctx.JSON(http.StatusInternalServerError, schema.InternalServerErrorResponse)
		}

		resp := make([]schema.TrafficPackageResponse, 0, len(pkgs))
		for i := range pkgs {
			resp = append(resp, toTrafficPackageResponse(&pkgs[i]))
		}
		return ctx.JSON(http.StatusOK, schema.BasicResponseData[[]schema.TrafficPackageResponse]{
			BasicResponse: schema.OkBasicResponse,
			Data:          resp,
		})
	}

	pkgs, err := c.packageService.ListActiveTrafficPackages()
	if err != nil {
		c.logger.Error("failed to list traffic packages", zap.Error(err))
		return ctx.JSON(http.StatusInternalServerError, schema.InternalServerErrorResponse)
	}

	resp := make([]schema.TrafficPackageResponse, 0, len(pkgs))
	for i := range pkgs {
		resp = append(resp, toTrafficPackageResponse(&pkgs[i]))
	}
	return ctx.JSON(http.StatusOK, schema.BasicResponseData[[]schema.TrafficPackageResponse]{
		BasicResponse: schema.OkBasicResponse,
		Data:          resp,
	})
}

func (c *TrafficPackageController) CreateTrafficPackage(ctx echo.Context) error {
	if forbidden, err := forbidUnlessAdmin(ctx); forbidden {
		return err
	}

	var req schema.CreateTrafficPackageRequest
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
		c.logger.Error("failed to create traffic package", zap.Error(err))
		return ctx.JSON(http.StatusInternalServerError, schema.ErrorResponse{
			StatusCode: http.StatusInternalServerError,
			Status:     "error",
			Message:    err.Error(),
		})
	}

	return ctx.JSON(http.StatusOK, schema.BasicResponseData[schema.TrafficPackageResponse]{
		BasicResponse: schema.OkBasicResponse,
		Data:          toTrafficPackageResponse(pkg),
	})
}

func (c *TrafficPackageController) UpdateTrafficPackage(ctx echo.Context) error {
	if forbidden, err := forbidUnlessAdmin(ctx); forbidden {
		return err
	}

	id, err := strconv.ParseUint(ctx.Param("id"), 10, 32)
	if err != nil {
		return ctx.JSON(http.StatusBadRequest, schema.BadParamsErrorResponse)
	}

	var req schema.UpdateTrafficPackageRequest
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
		c.logger.Error("failed to update traffic package", zap.Error(err))
		return ctx.JSON(http.StatusInternalServerError, schema.ErrorResponse{
			StatusCode: http.StatusInternalServerError,
			Status:     "error",
			Message:    err.Error(),
		})
	}

	return ctx.JSON(http.StatusOK, schema.BasicResponseData[schema.TrafficPackageResponse]{
		BasicResponse: schema.OkBasicResponse,
		Data:          toTrafficPackageResponse(pkg),
	})
}

func (c *TrafficPackageController) DeleteTrafficPackage(ctx echo.Context) error {
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
		c.logger.Error("failed to delete traffic package", zap.Error(err))
		return ctx.JSON(http.StatusInternalServerError, schema.InternalServerErrorResponse)
	}

	return ctx.JSON(http.StatusOK, schema.OkBasicResponse)
}

func toPackagePurchaseResponse(p *model.PackagePurchase) schema.PackagePurchaseResponse {
	return schema.PackagePurchaseResponse{
		ID:                 p.ID,
		ResellerID:         p.ResellerID,
		TrafficPackageID:   p.TrafficPackageID,
		TrafficPackageName: p.TrafficPackageName,
		TrafficBytes:       p.TrafficBytes,
		PriceAmount:        p.PriceAmount,
		CreatedAt:          p.CreatedAt.Unix(),
	}
}

// PurchasePackage lets a reseller buy a package for themselves.
func (c *TrafficPackageController) PurchasePackage(ctx echo.Context) error {
	role, resellerClaim := getRoleAndResellerFromContext(ctx)
	if role != "reseller" || resellerClaim == nil {
		return ctx.JSON(http.StatusForbidden, schema.ErrorResponse{StatusCode: http.StatusForbidden, Status: "error", Message: "forbidden"})
	}

	var req schema.PurchasePackageRequest
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
		c.logger.Warn("failed to purchase traffic package", zap.Uint("reseller_id", *resellerClaim), zap.Error(err))
		return ctx.JSON(http.StatusBadRequest, schema.ErrorResponse{
			StatusCode: http.StatusBadRequest,
			Status:     "error",
			Message:    err.Error(),
		})
	}

	return ctx.JSON(http.StatusOK, schema.BasicResponseData[schema.PackagePurchaseResponse]{
		BasicResponse: schema.OkBasicResponse,
		Data:          toPackagePurchaseResponse(purchase),
	})
}

// ListMyPurchases returns the caller reseller's own purchase history.
func (c *TrafficPackageController) ListMyPurchases(ctx echo.Context) error {
	role, resellerClaim := getRoleAndResellerFromContext(ctx)
	if role != "reseller" || resellerClaim == nil {
		return ctx.JSON(http.StatusForbidden, schema.ErrorResponse{StatusCode: http.StatusForbidden, Status: "error", Message: "forbidden"})
	}

	purchases, err := c.purchaseService.ListPurchases(*resellerClaim, 50)
	if err != nil {
		c.logger.Error("failed to list purchases", zap.Error(err))
		return ctx.JSON(http.StatusInternalServerError, schema.InternalServerErrorResponse)
	}

	resp := make([]schema.PackagePurchaseResponse, 0, len(purchases))
	for i := range purchases {
		resp = append(resp, toPackagePurchaseResponse(&purchases[i]))
	}
	return ctx.JSON(http.StatusOK, schema.BasicResponseData[[]schema.PackagePurchaseResponse]{
		BasicResponse: schema.OkBasicResponse,
		Data:          resp,
	})
}
