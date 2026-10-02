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

type PricePlanController struct {
	priceplanService *service.PricePlan
	logger           *zap.Logger
}

func NewPricePlanController(priceplanService *service.PricePlan) *PricePlanController {
	return &PricePlanController{
		priceplanService: priceplanService,
		logger:           zap.L().Named("PricePlanController"),
	}
}

// ListPricePlans returns all active pricing plans (accessible to all)
func (ppc *PricePlanController) ListPricePlans(ctx echo.Context) error {
	plans, err := ppc.priceplanService.ListActivePricePlans()
	if err != nil {
		ppc.logger.Error("failed to list price plans", zap.Error(err))
		return ctx.JSON(http.StatusInternalServerError, schema.ErrorResponse{
			StatusCode: http.StatusInternalServerError,
			Status:     "error",
			Message:    err.Error(),
		})
	}

	response := make([]schema.PricePlanResponse, len(plans))
	for i, plan := range plans {
		response[i] = schema.PricePlanResponse{
			ID:               plan.ID,
			Name:             plan.Name,
			Description:      plan.Description,
			BasePriceAmount:  plan.BasePriceAmount,
			BillingInterval:  plan.BillingInterval,
			TrafficAllowance: plan.TrafficAllowance,
			MaxPeers:         plan.MaxPeers,
			MaxServers:       plan.MaxServers,
			IsActive:         plan.IsActive,
		}
	}

	return ctx.JSON(http.StatusOK, schema.BasicResponseData[[]schema.PricePlanResponse]{
		BasicResponse: schema.OkBasicResponse,
		Data:          response,
	})
}

// GetPricePlan returns a specific pricing plan
func (ppc *PricePlanController) GetPricePlan(ctx echo.Context) error {
	planIDStr := ctx.Param("plan_id")
	if planIDStr == "" {
		return ctx.JSON(http.StatusBadRequest, schema.BadParamsErrorResponse)
	}

	planID, err := strconv.ParseUint(planIDStr, 10, 32)
	if err != nil {
		ppc.logger.Warn("invalid plan_id", zap.Error(err))
		return ctx.JSON(http.StatusBadRequest, schema.BadParamsErrorResponse)
	}

	plan, err := ppc.priceplanService.GetPricePlan(uint(planID))
	if err != nil {
		if errors.Is(err, gorm.ErrRecordNotFound) {
			return ctx.JSON(http.StatusNotFound, schema.ErrorResponse{
				StatusCode: http.StatusNotFound,
				Status:     "error",
				Message:    "price plan not found",
			})
		}
		ppc.logger.Error("failed to get price plan", zap.Error(err))
		return ctx.JSON(http.StatusInternalServerError, schema.ErrorResponse{
			StatusCode: http.StatusInternalServerError,
			Status:     "error",
			Message:    err.Error(),
		})
	}

	response := schema.PricePlanResponse{
		ID:               plan.ID,
		Name:             plan.Name,
		Description:      plan.Description,
		BasePriceAmount:  plan.BasePriceAmount,
		BillingInterval:  plan.BillingInterval,
		TrafficAllowance: plan.TrafficAllowance,
		MaxPeers:         plan.MaxPeers,
		MaxServers:       plan.MaxServers,
		IsActive:         plan.IsActive,
	}

	return ctx.JSON(http.StatusOK, schema.BasicResponseData[schema.PricePlanResponse]{
		BasicResponse: schema.OkBasicResponse,
		Data:          response,
	})
}

// CreatePricePlan creates a new pricing plan (admin only)
func (ppc *PricePlanController) CreatePricePlan(ctx echo.Context) error {
	role, _ := getRoleAndResellerFromContext(ctx)
	if role == "reseller" {
		return ctx.JSON(http.StatusForbidden, schema.ErrorResponse{
			StatusCode: http.StatusForbidden,
			Status:     "error",
			Message:    "forbidden",
		})
	}

	var req schema.CreatePricePlanRequest
	if err := ctx.Bind(&req); err != nil {
		ppc.logger.Warn("failed to bind request", zap.Error(err))
		return ctx.JSON(http.StatusBadRequest, schema.BadParamsErrorResponse)
	}

	if err := ctx.Validate(&req); err != nil {
		ppc.logger.Warn("failed to validate request", zap.Error(err))
		return ctx.JSON(http.StatusBadRequest, schema.BadParamsErrorResponse)
	}

	plan, err := ppc.priceplanService.CreatePricePlan(
		req.Name,
		req.Description,
		req.BasePriceAmount,
		req.BillingInterval,
		req.TrafficAllowance,
		req.MaxPeers,
		req.MaxServers,
	)
	if err != nil {
		ppc.logger.Error("failed to create price plan", zap.Error(err))
		return ctx.JSON(http.StatusInternalServerError, schema.ErrorResponse{
			StatusCode: http.StatusInternalServerError,
			Status:     "error",
			Message:    err.Error(),
		})
	}

	response := schema.PricePlanResponse{
		ID:               plan.ID,
		Name:             plan.Name,
		Description:      plan.Description,
		BasePriceAmount:  plan.BasePriceAmount,
		BillingInterval:  plan.BillingInterval,
		TrafficAllowance: plan.TrafficAllowance,
		MaxPeers:         plan.MaxPeers,
		MaxServers:       plan.MaxServers,
		IsActive:         plan.IsActive,
	}

	return ctx.JSON(http.StatusCreated, schema.BasicResponseData[schema.PricePlanResponse]{
		BasicResponse: schema.OkBasicResponse,
		Data:          response,
	})
}

// UpdatePricePlan updates an existing pricing plan (admin only)
func (ppc *PricePlanController) UpdatePricePlan(ctx echo.Context) error {
	role, _ := getRoleAndResellerFromContext(ctx)
	if role == "reseller" {
		return ctx.JSON(http.StatusForbidden, schema.ErrorResponse{
			StatusCode: http.StatusForbidden,
			Status:     "error",
			Message:    "forbidden",
		})
	}

	planIDStr := ctx.Param("plan_id")
	if planIDStr == "" {
		return ctx.JSON(http.StatusBadRequest, schema.BadParamsErrorResponse)
	}

	planID, err := strconv.ParseUint(planIDStr, 10, 32)
	if err != nil {
		return ctx.JSON(http.StatusBadRequest, schema.BadParamsErrorResponse)
	}

	var req schema.UpdatePricePlanRequest
	if err := ctx.Bind(&req); err != nil {
		ppc.logger.Warn("failed to bind request", zap.Error(err))
		return ctx.JSON(http.StatusBadRequest, schema.BadParamsErrorResponse)
	}

	if err := ctx.Validate(&req); err != nil {
		ppc.logger.Warn("failed to validate request", zap.Error(err))
		return ctx.JSON(http.StatusBadRequest, schema.BadParamsErrorResponse)
	}

	plan, err := ppc.priceplanService.UpdatePricePlan(
		uint(planID),
		req.Name,
		req.Description,
		req.BasePriceAmount,
		req.BillingInterval,
		req.TrafficAllowance,
		req.MaxPeers,
		req.MaxServers,
	)
	if err != nil {
		if errors.Is(err, gorm.ErrRecordNotFound) {
			return ctx.JSON(http.StatusNotFound, schema.ErrorResponse{
				StatusCode: http.StatusNotFound,
				Status:     "error",
				Message:    "price plan not found",
			})
		}
		ppc.logger.Error("failed to update price plan", zap.Error(err))
		return ctx.JSON(http.StatusInternalServerError, schema.ErrorResponse{
			StatusCode: http.StatusInternalServerError,
			Status:     "error",
			Message:    err.Error(),
		})
	}

	response := schema.PricePlanResponse{
		ID:               plan.ID,
		Name:             plan.Name,
		Description:      plan.Description,
		BasePriceAmount:  plan.BasePriceAmount,
		BillingInterval:  plan.BillingInterval,
		TrafficAllowance: plan.TrafficAllowance,
		MaxPeers:         plan.MaxPeers,
		MaxServers:       plan.MaxServers,
		IsActive:         plan.IsActive,
	}

	return ctx.JSON(http.StatusOK, schema.BasicResponseData[schema.PricePlanResponse]{
		BasicResponse: schema.OkBasicResponse,
		Data:          response,
	})
}

// DeactivatePricePlan marks a plan as inactive (admin only)
func (ppc *PricePlanController) DeactivatePricePlan(ctx echo.Context) error {
	role, _ := getRoleAndResellerFromContext(ctx)
	if role == "reseller" {
		return ctx.JSON(http.StatusForbidden, schema.ErrorResponse{
			StatusCode: http.StatusForbidden,
			Status:     "error",
			Message:    "forbidden",
		})
	}

	planIDStr := ctx.Param("plan_id")
	if planIDStr == "" {
		return ctx.JSON(http.StatusBadRequest, schema.BadParamsErrorResponse)
	}

	planID, err := strconv.ParseUint(planIDStr, 10, 32)
	if err != nil {
		return ctx.JSON(http.StatusBadRequest, schema.BadParamsErrorResponse)
	}

	if err := ppc.priceplanService.DeactivatePricePlan(uint(planID)); err != nil {
		if errors.Is(err, gorm.ErrRecordNotFound) {
			return ctx.JSON(http.StatusNotFound, schema.ErrorResponse{
				StatusCode: http.StatusNotFound,
				Status:     "error",
				Message:    "price plan not found",
			})
		}
		ppc.logger.Error("failed to deactivate price plan", zap.Error(err))
		return ctx.JSON(http.StatusInternalServerError, schema.ErrorResponse{
			StatusCode: http.StatusInternalServerError,
			Status:     "error",
			Message:    err.Error(),
		})
	}

	return ctx.JSON(http.StatusOK, schema.OkBasicResponse)
}
