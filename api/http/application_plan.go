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

// ApplicationPlanController manages the admin-defined Application tier
// catalog -- every endpoint is admin-only, mirroring DNSPlanController's
// CRUD shape exactly.
type ApplicationPlanController struct {
	planService *service.ApplicationPlanService
	logger      *zap.Logger
}

func NewApplicationPlanController(planService *service.ApplicationPlanService) *ApplicationPlanController {
	return &ApplicationPlanController{
		planService: planService,
		logger:      zap.L().Named("ApplicationPlanController"),
	}
}

// ListPlans mirrors TrafficPackageController.ListTrafficPackages' own
// active/all split: reachable by any authenticated session (a reseller
// needs this to populate their own Application form's plan picker), and
// returns only active plans unless ?all=true, which is admin-only and also
// surfaces inactive plans (needed by the plan management page).
func (c *ApplicationPlanController) ListPlans(ctx echo.Context) error {
	if ctx.QueryParam("all") == "true" {
		if forbidden, err := forbidUnlessAdmin(ctx); forbidden {
			return err
		}

		plans, err := c.planService.ListAllPlans()
		if err != nil {
			c.logger.Error("failed to list all application plans", zap.Error(err))
			return ctx.JSON(http.StatusInternalServerError, schema.InternalServerErrorResponse)
		}

		return ctx.JSON(http.StatusOK, schema.BasicResponseData[[]schema.ApplicationPlanResponse]{
			BasicResponse: schema.OkBasicResponse,
			Data:          plans,
		})
	}

	plans, err := c.planService.ListActivePlans()
	if err != nil {
		c.logger.Error("failed to list application plans", zap.Error(err))
		return ctx.JSON(http.StatusInternalServerError, schema.InternalServerErrorResponse)
	}

	return ctx.JSON(http.StatusOK, schema.BasicResponseData[[]schema.ApplicationPlanResponse]{
		BasicResponse: schema.OkBasicResponse,
		Data:          plans,
	})
}

func (c *ApplicationPlanController) CreatePlan(ctx echo.Context) error {
	if forbidden, err := forbidUnlessAdmin(ctx); forbidden {
		return err
	}

	var req schema.CreateApplicationPlanRequest
	if err := ctx.Bind(&req); err != nil {
		c.logger.Warn("failed to bind request", zap.Error(err))
		return ctx.JSON(http.StatusBadRequest, schema.BadParamsErrorResponse)
	}
	if err := ctx.Validate(&req); err != nil {
		c.logger.Warn("failed to validate request", zap.Error(err))
		return ctx.JSON(http.StatusBadRequest, schema.BadParamsErrorResponse)
	}

	plan, err := c.planService.CreatePlan(&req)
	if err != nil {
		c.logger.Error("failed to create application plan", zap.Error(err))
		return ctx.JSON(http.StatusInternalServerError, schema.ErrorResponse{
			StatusCode: http.StatusInternalServerError,
			Status:     "error",
			Message:    "failed to create plan: " + err.Error(),
		})
	}

	return ctx.JSON(http.StatusCreated, schema.BasicResponseData[schema.ApplicationPlanResponse]{
		BasicResponse: schema.OkBasicResponse,
		Data:          *plan,
	})
}

func (c *ApplicationPlanController) UpdatePlan(ctx echo.Context) error {
	if forbidden, err := forbidUnlessAdmin(ctx); forbidden {
		return err
	}

	id, err := strconv.Atoi(ctx.Param("id"))
	if err != nil {
		return ctx.JSON(http.StatusBadRequest, schema.BadParamsErrorResponse)
	}

	var req schema.UpdateApplicationPlanRequest
	if err := ctx.Bind(&req); err != nil {
		c.logger.Warn("failed to bind request", zap.Error(err))
		return ctx.JSON(http.StatusBadRequest, schema.BadParamsErrorResponse)
	}

	plan, err := c.planService.UpdatePlan(uint(id), &req)
	if err != nil {
		if errors.Is(err, gorm.ErrRecordNotFound) {
			return ctx.JSON(http.StatusNotFound, schema.ErrorResponse{StatusCode: http.StatusNotFound, Status: "error", Message: "plan not found"})
		}
		c.logger.Error("failed to update application plan", zap.Error(err))
		return ctx.JSON(http.StatusInternalServerError, schema.ErrorResponse{
			StatusCode: http.StatusInternalServerError,
			Status:     "error",
			Message:    "failed to update plan: " + err.Error(),
		})
	}

	return ctx.JSON(http.StatusOK, schema.BasicResponseData[schema.ApplicationPlanResponse]{
		BasicResponse: schema.OkBasicResponse,
		Data:          *plan,
	})
}

func (c *ApplicationPlanController) DeletePlan(ctx echo.Context) error {
	if forbidden, err := forbidUnlessAdmin(ctx); forbidden {
		return err
	}

	id, err := strconv.Atoi(ctx.Param("id"))
	if err != nil {
		return ctx.JSON(http.StatusBadRequest, schema.BadParamsErrorResponse)
	}

	if err := c.planService.DeletePlan(uint(id)); err != nil {
		c.logger.Error("failed to delete application plan", zap.Error(err))
		return ctx.JSON(http.StatusInternalServerError, schema.ErrorResponse{
			StatusCode: http.StatusInternalServerError,
			Status:     "error",
			Message:    "failed to delete plan: " + err.Error(),
		})
	}

	return ctx.NoContent(http.StatusNoContent)
}
