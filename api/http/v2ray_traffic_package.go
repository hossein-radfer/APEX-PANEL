package http

import (
	"errors"
	"net/http"
	"strconv"

	"github.com/labstack/echo/v4"
	"go.uber.org/zap"
	"gorm.io/gorm"

	"github.com/maahdima/mwp/api/dataservice/model"
	"github.com/maahdima/mwp/api/http/schema"
	"github.com/maahdima/mwp/api/service"
)

// V2RayTrafficPackageController mirrors UserManagerTrafficPackageController
// exactly, for the separate V2Ray traffic-package pool.
type V2RayTrafficPackageController struct {
	packageService  *service.V2RayTrafficPackageService
	purchaseService *service.V2RayPackagePurchaseService
	logger          *zap.Logger
}

func NewV2RayTrafficPackageController(packageService *service.V2RayTrafficPackageService, purchaseService *service.V2RayPackagePurchaseService) *V2RayTrafficPackageController {
	return &V2RayTrafficPackageController{
		packageService:  packageService,
		purchaseService: purchaseService,
		logger:          zap.L().Named("V2RayTrafficPackageController"),
	}
}

func toV2RayTrafficPackageResponse(pkg *model.V2RayTrafficPackage) schema.V2RayTrafficPackageResponse {
	return schema.V2RayTrafficPackageResponse{
		Id:           pkg.ID,
		Name:         pkg.Name,
		Description:  pkg.Description,
		TrafficBytes: pkg.TrafficBytes,
		PriceAmount:  pkg.PriceAmount,
		IsActive:     pkg.IsActive,
	}
}

func (c *V2RayTrafficPackageController) ListTrafficPackages(ctx echo.Context) error {
	if ctx.QueryParam("all") == "true" {
		if forbidden, err := forbidUnlessAdmin(ctx); forbidden {
			return err
		}

		pkgs, err := c.packageService.ListAllTrafficPackages()
		if err != nil {
			c.logger.Error("failed to list all v2ray traffic packages", zap.Error(err))
			return ctx.JSON(http.StatusInternalServerError, schema.InternalServerErrorResponse)
		}

		resp := make([]schema.V2RayTrafficPackageResponse, 0, len(pkgs))
		for i := range pkgs {
			resp = append(resp, toV2RayTrafficPackageResponse(&pkgs[i]))
		}
		return ctx.JSON(http.StatusOK, schema.BasicResponseData[[]schema.V2RayTrafficPackageResponse]{
			BasicResponse: schema.OkBasicResponse,
			Data:          resp,
		})
	}

	pkgs, err := c.packageService.ListActiveTrafficPackages()
	if err != nil {
		c.logger.Error("failed to list v2ray traffic packages", zap.Error(err))
		return ctx.JSON(http.StatusInternalServerError, schema.InternalServerErrorResponse)
	}

	resp := make([]schema.V2RayTrafficPackageResponse, 0, len(pkgs))
	for i := range pkgs {
		resp = append(resp, toV2RayTrafficPackageResponse(&pkgs[i]))
	}
	return ctx.JSON(http.StatusOK, schema.BasicResponseData[[]schema.V2RayTrafficPackageResponse]{
		BasicResponse: schema.OkBasicResponse,
		Data:          resp,
	})
}

func (c *V2RayTrafficPackageController) CreateTrafficPackage(ctx echo.Context) error {
	if forbidden, err := forbidUnlessAdmin(ctx); forbidden {
		return err
	}

	var req schema.CreateV2RayTrafficPackageRequest
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
		c.logger.Error("failed to create v2ray traffic package", zap.Error(err))
		return ctx.JSON(http.StatusInternalServerError, schema.ErrorResponse{
			StatusCode: http.StatusInternalServerError,
			Status:     "error",
			Message:    err.Error(),
		})
	}

	return ctx.JSON(http.StatusOK, schema.BasicResponseData[schema.V2RayTrafficPackageResponse]{
		BasicResponse: schema.OkBasicResponse,
		Data:          toV2RayTrafficPackageResponse(pkg),
	})
}

func (c *V2RayTrafficPackageController) UpdateTrafficPackage(ctx echo.Context) error {
	if forbidden, err := forbidUnlessAdmin(ctx); forbidden {
		return err
	}

	id, err := strconv.ParseUint(ctx.Param("id"), 10, 32)
	if err != nil {
		return ctx.JSON(http.StatusBadRequest, schema.BadParamsErrorResponse)
	}

	var req schema.UpdateV2RayTrafficPackageRequest
	if err := ctx.Bind(&req); err != nil {
		c.logger.Warn("failed to bind request", zap.Error(err))
		return ctx.JSON(http.StatusBadRequest, schema.BadParamsErrorResponse)
	}
	if err := ctx.Validate(&req); err != nil {
		c.logger.Warn("failed to validate request", zap.Error(err))
		return ctx.JSON(http.StatusBadRequest, schema.BadParamsErrorResponse)
	}

	var name *string
	if req.Name != "" {
		name = &req.Name
	}

	pkg, err := c.packageService.UpdateTrafficPackage(uint(id), name, req.Description, req.TrafficBytes, req.PriceAmount, req.IsActive)
	if err != nil {
		if errors.Is(err, gorm.ErrRecordNotFound) {
			return ctx.JSON(http.StatusNotFound, schema.ErrorResponse{StatusCode: http.StatusNotFound, Status: "error", Message: "traffic package not found"})
		}
		c.logger.Error("failed to update v2ray traffic package", zap.Error(err))
		return ctx.JSON(http.StatusInternalServerError, schema.ErrorResponse{
			StatusCode: http.StatusInternalServerError,
			Status:     "error",
			Message:    err.Error(),
		})
	}

	return ctx.JSON(http.StatusOK, schema.BasicResponseData[schema.V2RayTrafficPackageResponse]{
		BasicResponse: schema.OkBasicResponse,
		Data:          toV2RayTrafficPackageResponse(pkg),
	})
}

func (c *V2RayTrafficPackageController) DeleteTrafficPackage(ctx echo.Context) error {
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
		c.logger.Error("failed to delete v2ray traffic package", zap.Error(err))
		return ctx.JSON(http.StatusInternalServerError, schema.InternalServerErrorResponse)
	}

	return ctx.JSON(http.StatusOK, schema.OkBasicResponse)
}

func toV2RayPackagePurchaseResponse(p *model.V2RayPackagePurchase) schema.V2RayPackagePurchaseResponse {
	return schema.V2RayPackagePurchaseResponse{
		Id:                 p.ID,
		TrafficPackageName: p.TrafficPackageName,
		TrafficBytes:       p.TrafficBytes,
		PriceAmount:        p.PriceAmount,
		CreatedAt:          p.CreatedAt.Format("2006-01-02T15:04:05Z07:00"),
	}
}

// PurchasePackage lets a reseller buy a V2Ray package for themselves.
func (c *V2RayTrafficPackageController) PurchasePackage(ctx echo.Context) error {
	role, resellerClaim := getRoleAndResellerFromContext(ctx)
	if role != "reseller" || resellerClaim == nil {
		return ctx.JSON(http.StatusForbidden, schema.ErrorResponse{StatusCode: http.StatusForbidden, Status: "error", Message: "forbidden"})
	}

	var req schema.PurchaseV2RayPackageRequest
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
		c.logger.Warn("failed to purchase v2ray traffic package", zap.Uint("reseller_id", *resellerClaim), zap.Error(err))
		return ctx.JSON(http.StatusBadRequest, schema.ErrorResponse{
			StatusCode: http.StatusBadRequest,
			Status:     "error",
			Message:    err.Error(),
		})
	}

	return ctx.JSON(http.StatusOK, schema.BasicResponseData[schema.V2RayPackagePurchaseResponse]{
		BasicResponse: schema.OkBasicResponse,
		Data:          toV2RayPackagePurchaseResponse(purchase),
	})
}

// ListMyPurchases returns the caller reseller's own V2Ray package purchase history.
func (c *V2RayTrafficPackageController) ListMyPurchases(ctx echo.Context) error {
	role, resellerClaim := getRoleAndResellerFromContext(ctx)
	if role != "reseller" || resellerClaim == nil {
		return ctx.JSON(http.StatusForbidden, schema.ErrorResponse{StatusCode: http.StatusForbidden, Status: "error", Message: "forbidden"})
	}

	purchases, err := c.purchaseService.ListPurchases(*resellerClaim, 50)
	if err != nil {
		c.logger.Error("failed to list v2ray package purchases", zap.Error(err))
		return ctx.JSON(http.StatusInternalServerError, schema.InternalServerErrorResponse)
	}

	resp := make([]schema.V2RayPackagePurchaseResponse, 0, len(purchases))
	for i := range purchases {
		resp = append(resp, toV2RayPackagePurchaseResponse(&purchases[i]))
	}
	return ctx.JSON(http.StatusOK, schema.BasicResponseData[[]schema.V2RayPackagePurchaseResponse]{
		BasicResponse: schema.OkBasicResponse,
		Data:          resp,
	})
}
