package http

import (
	"net/http"

	"github.com/labstack/echo/v4"
	"go.uber.org/zap"

	"github.com/maahdima/mwp/api/http/schema"
	"github.com/maahdima/mwp/api/service"
)

// ReportsController serves the admin-only "Reports" section -- every
// endpoint here is a pure read (ReportsService/ReportsLiveService never
// write anything), gated admin-only via the same
// getRoleAndResellerFromContext pattern every other admin-only controller
// in this codebase uses.
type ReportsController struct {
	reports     *service.ReportsService
	reportsLive *service.ReportsLiveService
	logger      *zap.Logger
}

func NewReportsController(reports *service.ReportsService, reportsLive *service.ReportsLiveService) *ReportsController {
	return &ReportsController{
		reports:     reports,
		reportsLive: reportsLive,
		logger:      zap.L().Named("ReportsController"),
	}
}

func (c *ReportsController) requireAdmin(ctx echo.Context) error {
	if role, _ := getRoleAndResellerFromContext(ctx); role == "reseller" {
		return ctx.JSON(http.StatusForbidden, schema.ErrorResponse{StatusCode: http.StatusForbidden, Status: "error", Message: "forbidden"})
	}
	return nil
}

func (c *ReportsController) internalError(ctx echo.Context, action string, err error) error {
	c.logger.Error(action, zap.Error(err))
	return ctx.JSON(http.StatusInternalServerError, schema.ErrorResponse{
		StatusCode: http.StatusInternalServerError,
		Status:     "error",
		Message:    action + ": " + err.Error(),
	})
}

func (c *ReportsController) DailyUsage(ctx echo.Context) error {
	if err := c.requireAdmin(ctx); err != nil {
		return err
	}
	data, err := c.reports.GetDailyUsage(ctx.QueryParam("range"), ctx.QueryParam("protocol"))
	if err != nil {
		return c.internalError(ctx, "failed to retrieve daily usage report", err)
	}
	return ctx.JSON(http.StatusOK, schema.BasicResponseData[schema.ReportsDailyUsageResponse]{BasicResponse: schema.OkBasicResponse, Data: *data})
}

func (c *ReportsController) PeakUsage(ctx echo.Context) error {
	if err := c.requireAdmin(ctx); err != nil {
		return err
	}
	data, err := c.reports.GetPeakUsage(ctx.QueryParam("range"), ctx.QueryParam("entity"))
	if err != nil {
		return c.internalError(ctx, "failed to retrieve peak usage report", err)
	}
	return ctx.JSON(http.StatusOK, schema.BasicResponseData[schema.ReportsPeakUsageResponse]{BasicResponse: schema.OkBasicResponse, Data: *data})
}

func (c *ReportsController) ResellerRanking(ctx echo.Context) error {
	if err := c.requireAdmin(ctx); err != nil {
		return err
	}
	data, err := c.reports.GetResellerUsageRanking(ctx.QueryParam("range"))
	if err != nil {
		return c.internalError(ctx, "failed to retrieve reseller usage ranking", err)
	}
	return ctx.JSON(http.StatusOK, schema.BasicResponseData[schema.ReportsRankingResponse]{BasicResponse: schema.OkBasicResponse, Data: *data})
}

func (c *ReportsController) UserRanking(ctx echo.Context) error {
	if err := c.requireAdmin(ctx); err != nil {
		return err
	}
	protocol := ctx.QueryParam("protocol")
	if protocol == "" {
		return ctx.JSON(http.StatusBadRequest, schema.BadParamsErrorResponse)
	}
	data, err := c.reports.GetUserUsageRanking(ctx.QueryParam("range"), protocol)
	if err != nil {
		return c.internalError(ctx, "failed to retrieve user usage ranking", err)
	}
	return ctx.JSON(http.StatusOK, schema.BasicResponseData[schema.ReportsRankingResponse]{BasicResponse: schema.OkBasicResponse, Data: *data})
}

func (c *ReportsController) ResellerActivityRanking(ctx echo.Context) error {
	if err := c.requireAdmin(ctx); err != nil {
		return err
	}
	data, err := c.reports.GetResellerActivityRanking(ctx.QueryParam("range"))
	if err != nil {
		return c.internalError(ctx, "failed to retrieve reseller activity ranking", err)
	}
	return ctx.JSON(http.StatusOK, schema.BasicResponseData[schema.ReportsRankingResponse]{BasicResponse: schema.OkBasicResponse, Data: *data})
}

func (c *ReportsController) ExpiringSoon(ctx echo.Context) error {
	if err := c.requireAdmin(ctx); err != nil {
		return err
	}
	data, err := c.reports.GetExpiringSoon()
	if err != nil {
		return c.internalError(ctx, "failed to retrieve expiring-soon report", err)
	}
	return ctx.JSON(http.StatusOK, schema.BasicResponseData[schema.ReportsExpiringResponse]{BasicResponse: schema.OkBasicResponse, Data: *data})
}

func (c *ReportsController) ProtocolShare(ctx echo.Context) error {
	if err := c.requireAdmin(ctx); err != nil {
		return err
	}
	data, err := c.reports.GetProtocolShare(ctx.QueryParam("range"))
	if err != nil {
		return c.internalError(ctx, "failed to retrieve protocol share report", err)
	}
	return ctx.JSON(http.StatusOK, schema.BasicResponseData[schema.ReportsProtocolShareResponse]{BasicResponse: schema.OkBasicResponse, Data: *data})
}

func (c *ReportsController) OnlineUsers(ctx echo.Context) error {
	if err := c.requireAdmin(ctx); err != nil {
		return err
	}
	data, err := c.reportsLive.GetOnlineUsers()
	if err != nil {
		return c.internalError(ctx, "failed to retrieve online users report", err)
	}
	return ctx.JSON(http.StatusOK, schema.BasicResponseData[schema.ReportsOnlineUsersResponse]{BasicResponse: schema.OkBasicResponse, Data: *data})
}

func (c *ReportsController) PanelHealth(ctx echo.Context) error {
	if err := c.requireAdmin(ctx); err != nil {
		return err
	}
	data, err := c.reportsLive.GetPanelHealth()
	if err != nil {
		return c.internalError(ctx, "failed to retrieve panel health report", err)
	}
	return ctx.JSON(http.StatusOK, schema.BasicResponseData[schema.ReportsPanelHealthResponse]{BasicResponse: schema.OkBasicResponse, Data: *data})
}

func (c *ReportsController) Financial(ctx echo.Context) error {
	if err := c.requireAdmin(ctx); err != nil {
		return err
	}
	bucketBy := ctx.QueryParam("bucket")
	if bucketBy == "" {
		bucketBy = "day"
	}
	data, err := c.reports.GetFinancialReport(ctx.QueryParam("range"), bucketBy)
	if err != nil {
		return c.internalError(ctx, "failed to retrieve financial report", err)
	}
	return ctx.JSON(http.StatusOK, schema.BasicResponseData[schema.ReportsFinancialResponse]{BasicResponse: schema.OkBasicResponse, Data: *data})
}

func (c *ReportsController) RenewalRate(ctx echo.Context) error {
	if err := c.requireAdmin(ctx); err != nil {
		return err
	}
	data, err := c.reports.GetRenewalRate(ctx.QueryParam("range"))
	if err != nil {
		return c.internalError(ctx, "failed to retrieve renewal rate report", err)
	}
	return ctx.JSON(http.StatusOK, schema.BasicResponseData[schema.ReportsRenewalRateResponse]{BasicResponse: schema.OkBasicResponse, Data: *data})
}

func (c *ReportsController) PopularLocations(ctx echo.Context) error {
	if err := c.requireAdmin(ctx); err != nil {
		return err
	}
	data, err := c.reports.GetPopularLocations(ctx.QueryParam("range"))
	if err != nil {
		return c.internalError(ctx, "failed to retrieve popular locations report", err)
	}
	return ctx.JSON(http.StatusOK, schema.BasicResponseData[schema.ReportsPopularLocationsResponse]{BasicResponse: schema.OkBasicResponse, Data: *data})
}

func (c *ReportsController) AnomalyAlerts(ctx echo.Context) error {
	if err := c.requireAdmin(ctx); err != nil {
		return err
	}
	data, err := c.reports.GetAnomalyAlerts()
	if err != nil {
		return c.internalError(ctx, "failed to retrieve anomaly alerts report", err)
	}
	return ctx.JSON(http.StatusOK, schema.BasicResponseData[schema.ReportsAnomalyResponse]{BasicResponse: schema.OkBasicResponse, Data: *data})
}

func (c *ReportsController) ResourceUsage(ctx echo.Context) error {
	if err := c.requireAdmin(ctx); err != nil {
		return err
	}
	data, err := c.reports.GetResourceUsage(ctx.QueryParam("range"))
	if err != nil {
		return c.internalError(ctx, "failed to retrieve resource usage report", err)
	}
	return ctx.JSON(http.StatusOK, schema.BasicResponseData[schema.ReportsResourceResponse]{BasicResponse: schema.OkBasicResponse, Data: *data})
}

// ExcelExport streams the export workbook directly -- deliberately
// ctx.Blob, not ctx.File, since GetExcelExport returns in-memory bytes
// rather than a path on disk (see reports_excel.go's own doc comment).
func (c *ReportsController) ExcelExport(ctx echo.Context) error {
	if err := c.requireAdmin(ctx); err != nil {
		return err
	}
	data, err := c.reports.GetExcelExport(ctx.QueryParam("range"))
	if err != nil {
		return c.internalError(ctx, "failed to generate excel export", err)
	}
	ctx.Response().Header().Set("Content-Disposition", "attachment; filename=\"reports.xlsx\"")
	return ctx.Blob(http.StatusOK, "application/vnd.openxmlformats-officedocument.spreadsheetml.sheet", data)
}

// ResellerQuotaPrediction is category 2 item 2's own endpoint -- the
// inverse guard of every other method on this controller: reseller-only,
// not admin-only, mirroring UserManagerAccountController.GetSelfSummary's
// exact "no single-reseller context to summarize for an admin" reasoning.
// Every other Reports endpoint stays admin-only; this one alone needed a
// reseller-scoped route since GetResellerQuotaPrediction's whole point is
// showing ONE reseller their OWN projection, not a system-wide report.
func (c *ReportsController) ResellerQuotaPrediction(ctx echo.Context) error {
	role, resellerID := getRoleAndResellerFromContext(ctx)
	if role != "reseller" || resellerID == nil {
		return ctx.JSON(http.StatusForbidden, schema.ErrorResponse{StatusCode: http.StatusForbidden, Status: "error", Message: "forbidden"})
	}
	data, err := c.reports.GetResellerQuotaPrediction(*resellerID)
	if err != nil {
		return c.internalError(ctx, "failed to retrieve quota prediction", err)
	}
	return ctx.JSON(http.StatusOK, schema.BasicResponseData[schema.ResellerQuotaPredictionResponse]{BasicResponse: schema.OkBasicResponse, Data: *data})
}
