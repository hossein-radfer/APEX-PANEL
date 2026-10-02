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

// DNSPlanController manages the admin-defined DNS tier catalog -- every
// endpoint is admin-only, mirroring DNSPanelController's CRUD shape.
type DNSPlanController struct {
	planService *service.DNSPlanService
	logger      *zap.Logger
}

func NewDNSPlanController(planService *service.DNSPlanService) *DNSPlanController {
	return &DNSPlanController{
		planService: planService,
		logger:      zap.L().Named("DNSPlanController"),
	}
}

func (c *DNSPlanController) ListPlans(ctx echo.Context) error {
	if forbidden, err := forbidUnlessAdmin(ctx); forbidden {
		return err
	}

	plans, err := c.planService.ListPlans()
	if err != nil {
		c.logger.Error("failed to list DNS plans", zap.Error(err))
		return ctx.JSON(http.StatusInternalServerError, schema.InternalServerErrorResponse)
	}

	return ctx.JSON(http.StatusOK, schema.BasicResponseData[[]schema.DNSPlanResponse]{
		BasicResponse: schema.OkBasicResponse,
		Data:          plans,
	})
}

func (c *DNSPlanController) CreatePlan(ctx echo.Context) error {
	if forbidden, err := forbidUnlessAdmin(ctx); forbidden {
		return err
	}

	var req schema.CreateDNSPlanRequest
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
		c.logger.Error("failed to create DNS plan", zap.Error(err))
		return ctx.JSON(http.StatusInternalServerError, schema.ErrorResponse{
			StatusCode: http.StatusInternalServerError,
			Status:     "error",
			Message:    "failed to create plan: " + err.Error(),
		})
	}

	return ctx.JSON(http.StatusCreated, schema.BasicResponseData[schema.DNSPlanResponse]{
		BasicResponse: schema.OkBasicResponse,
		Data:          *plan,
	})
}

func (c *DNSPlanController) UpdatePlan(ctx echo.Context) error {
	if forbidden, err := forbidUnlessAdmin(ctx); forbidden {
		return err
	}

	id, err := strconv.Atoi(ctx.Param("id"))
	if err != nil {
		return ctx.JSON(http.StatusBadRequest, schema.BadParamsErrorResponse)
	}

	var req schema.UpdateDNSPlanRequest
	if err := ctx.Bind(&req); err != nil {
		c.logger.Warn("failed to bind request", zap.Error(err))
		return ctx.JSON(http.StatusBadRequest, schema.BadParamsErrorResponse)
	}

	plan, err := c.planService.UpdatePlan(uint(id), &req)
	if err != nil {
		if errors.Is(err, gorm.ErrRecordNotFound) {
			return ctx.JSON(http.StatusNotFound, schema.ErrorResponse{StatusCode: http.StatusNotFound, Status: "error", Message: "plan not found"})
		}
		c.logger.Error("failed to update DNS plan", zap.Error(err))
		return ctx.JSON(http.StatusInternalServerError, schema.ErrorResponse{
			StatusCode: http.StatusInternalServerError,
			Status:     "error",
			Message:    "failed to update plan: " + err.Error(),
		})
	}

	return ctx.JSON(http.StatusOK, schema.BasicResponseData[schema.DNSPlanResponse]{
		BasicResponse: schema.OkBasicResponse,
		Data:          *plan,
	})
}

func (c *DNSPlanController) DeletePlan(ctx echo.Context) error {
	if forbidden, err := forbidUnlessAdmin(ctx); forbidden {
		return err
	}

	id, err := strconv.Atoi(ctx.Param("id"))
	if err != nil {
		return ctx.JSON(http.StatusBadRequest, schema.BadParamsErrorResponse)
	}

	if err := c.planService.DeletePlan(uint(id)); err != nil {
		c.logger.Error("failed to delete DNS plan", zap.Error(err))
		return ctx.JSON(http.StatusInternalServerError, schema.ErrorResponse{
			StatusCode: http.StatusInternalServerError,
			Status:     "error",
			Message:    "failed to delete plan: " + err.Error(),
		})
	}

	return ctx.NoContent(http.StatusNoContent)
}
