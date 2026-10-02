package http

import (
	"errors"
	"io"
	"net/http"

	"github.com/labstack/echo/v4"
	"go.uber.org/zap"
	"gorm.io/gorm"

	"github.com/maahdima/mwp/api/http/schema"
	"github.com/maahdima/mwp/api/service"
)

// FaoximaController exposes the full-Faoxima bot-as-a-service feature to
// a reseller's OWN panel session -- there is no :reseller_id URL param
// (mirrors WalletController's own reseller-self-service endpoints): the
// target reseller is always the caller's own JWT claim, so a reseller can
// only ever manage their own Faoxima instance, never another's. An admin
// session may also use these endpoints for support purposes (see
// getRoleAndResellerFromContext's own admin-bypass convention used
// elsewhere), but always still needs a resellerID reachable through
// context -- there is no separate admin-on-behalf-of-any-reseller path
// here since this is a self-service reseller feature, not an
// admin-configured one like V2Ray panel assignment.
type FaoximaController struct {
	provisioner *service.FaoximaProvisionerService
	logger      *zap.Logger
}

func NewFaoximaController(provisioner *service.FaoximaProvisionerService) *FaoximaController {
	return &FaoximaController{
		provisioner: provisioner,
		logger:      zap.L().Named("FaoximaController"),
	}
}

func (c *FaoximaController) resellerIDFromContext(ctx echo.Context) (uint, bool) {
	role, resellerID := getRoleAndResellerFromContext(ctx)
	if role != "reseller" || resellerID == nil {
		return 0, false
	}
	return *resellerID, true
}

func (c *FaoximaController) GetStatus(ctx echo.Context) error {
	resellerID, ok := c.resellerIDFromContext(ctx)
	if !ok {
		return ctx.JSON(http.StatusForbidden, schema.ErrorResponse{StatusCode: http.StatusForbidden, Status: "error", Message: "forbidden"})
	}

	instance, err := c.provisioner.GetInstance(resellerID)
	if err != nil {
		if errors.Is(err, gorm.ErrRecordNotFound) {
			return ctx.JSON(http.StatusOK, schema.BasicResponseData[*schema.FaoximaInstanceResponse]{
				BasicResponse: schema.OkBasicResponse,
				Data:          nil,
			})
		}
		c.logger.Error("failed to fetch faoxima instance", zap.Error(err))
		return ctx.JSON(http.StatusInternalServerError, schema.InternalServerErrorResponse)
	}

	return ctx.JSON(http.StatusOK, schema.BasicResponseData[*schema.FaoximaInstanceResponse]{
		BasicResponse: schema.OkBasicResponse,
		Data: &schema.FaoximaInstanceResponse{
			Status:       instance.Status,
			AdminChatID:  instance.AdminChatID,
			ErrorMessage: instance.ErrorMessage,
			CreatedAt:    instance.CreatedAt,
		},
	})
}

func (c *FaoximaController) Provision(ctx echo.Context) error {
	resellerID, ok := c.resellerIDFromContext(ctx)
	if !ok {
		return ctx.JSON(http.StatusForbidden, schema.ErrorResponse{StatusCode: http.StatusForbidden, Status: "error", Message: "forbidden"})
	}

	var req schema.ProvisionFaoximaRequest
	if err := ctx.Bind(&req); err != nil {
		return ctx.JSON(http.StatusBadRequest, schema.BadParamsErrorResponse)
	}
	if err := ctx.Validate(&req); err != nil {
		return ctx.JSON(http.StatusBadRequest, schema.BadParamsErrorResponse)
	}

	instance, err := c.provisioner.Provision(resellerID, req.BotToken, req.AdminChatID)
	if err != nil {
		c.logger.Error("failed to provision faoxima instance", zap.Uint("reseller_id", resellerID), zap.Error(err))
		return ctx.JSON(http.StatusInternalServerError, schema.ErrorResponse{
			StatusCode: http.StatusInternalServerError,
			Status:     "error",
			Message:    err.Error(),
		})
	}

	return ctx.JSON(http.StatusCreated, schema.BasicResponseData[schema.FaoximaInstanceResponse]{
		BasicResponse: schema.OkBasicResponse,
		Data: schema.FaoximaInstanceResponse{
			Status:      instance.Status,
			AdminChatID: instance.AdminChatID,
			CreatedAt:   instance.CreatedAt,
		},
	})
}

func (c *FaoximaController) UpdateToken(ctx echo.Context) error {
	resellerID, ok := c.resellerIDFromContext(ctx)
	if !ok {
		return ctx.JSON(http.StatusForbidden, schema.ErrorResponse{StatusCode: http.StatusForbidden, Status: "error", Message: "forbidden"})
	}

	var req schema.UpdateFaoximaTokenRequest
	if err := ctx.Bind(&req); err != nil {
		return ctx.JSON(http.StatusBadRequest, schema.BadParamsErrorResponse)
	}
	if err := ctx.Validate(&req); err != nil {
		return ctx.JSON(http.StatusBadRequest, schema.BadParamsErrorResponse)
	}

	if err := c.provisioner.UpdateToken(resellerID, req.BotToken); err != nil {
		c.logger.Error("failed to update faoxima token", zap.Uint("reseller_id", resellerID), zap.Error(err))
		return ctx.JSON(http.StatusInternalServerError, schema.ErrorResponse{
			StatusCode: http.StatusInternalServerError,
			Status:     "error",
			Message:    err.Error(),
		})
	}

	return ctx.JSON(http.StatusOK, schema.OkBasicResponse)
}

func (c *FaoximaController) Disable(ctx echo.Context) error {
	resellerID, ok := c.resellerIDFromContext(ctx)
	if !ok {
		return ctx.JSON(http.StatusForbidden, schema.ErrorResponse{StatusCode: http.StatusForbidden, Status: "error", Message: "forbidden"})
	}
	if err := c.provisioner.Disable(resellerID); err != nil {
		c.logger.Error("failed to disable faoxima instance", zap.Uint("reseller_id", resellerID), zap.Error(err))
		return ctx.JSON(http.StatusInternalServerError, schema.ErrorResponse{StatusCode: http.StatusInternalServerError, Status: "error", Message: err.Error()})
	}
	return ctx.JSON(http.StatusOK, schema.OkBasicResponse)
}

func (c *FaoximaController) Enable(ctx echo.Context) error {
	resellerID, ok := c.resellerIDFromContext(ctx)
	if !ok {
		return ctx.JSON(http.StatusForbidden, schema.ErrorResponse{StatusCode: http.StatusForbidden, Status: "error", Message: "forbidden"})
	}
	if err := c.provisioner.Enable(resellerID); err != nil {
		c.logger.Error("failed to enable faoxima instance", zap.Uint("reseller_id", resellerID), zap.Error(err))
		return ctx.JSON(http.StatusInternalServerError, schema.ErrorResponse{StatusCode: http.StatusInternalServerError, Status: "error", Message: err.Error()})
	}
	return ctx.JSON(http.StatusOK, schema.OkBasicResponse)
}

func (c *FaoximaController) Remove(ctx echo.Context) error {
	resellerID, ok := c.resellerIDFromContext(ctx)
	if !ok {
		return ctx.JSON(http.StatusForbidden, schema.ErrorResponse{StatusCode: http.StatusForbidden, Status: "error", Message: "forbidden"})
	}
	if err := c.provisioner.Remove(resellerID); err != nil {
		c.logger.Error("failed to remove faoxima instance", zap.Uint("reseller_id", resellerID), zap.Error(err))
		return ctx.JSON(http.StatusInternalServerError, schema.ErrorResponse{StatusCode: http.StatusInternalServerError, Status: "error", Message: err.Error()})
	}
	return ctx.JSON(http.StatusOK, schema.OkBasicResponse)
}

// Backup streams a mysqldump of this reseller's own Faoxima database back
// as a downloadable .sql file -- mirrors BackupController's own download
// pattern for the panel's SQLite backup.
func (c *FaoximaController) Backup(ctx echo.Context) error {
	resellerID, ok := c.resellerIDFromContext(ctx)
	if !ok {
		return ctx.JSON(http.StatusForbidden, schema.ErrorResponse{StatusCode: http.StatusForbidden, Status: "error", Message: "forbidden"})
	}

	content, filename, err := c.provisioner.Backup(resellerID)
	if err != nil {
		c.logger.Error("failed to backup faoxima instance", zap.Uint("reseller_id", resellerID), zap.Error(err))
		return ctx.JSON(http.StatusInternalServerError, schema.ErrorResponse{StatusCode: http.StatusInternalServerError, Status: "error", Message: err.Error()})
	}

	return ctx.Blob(http.StatusOK, "application/sql", contentWithFilename(ctx, filename, content))
}

// contentWithFilename sets Content-Disposition before returning the raw
// bytes -- factored out only so Backup's own return statement stays
// readable; not a general-purpose helper.
func contentWithFilename(ctx echo.Context, filename string, content []byte) []byte {
	ctx.Response().Header().Set("Content-Disposition", "attachment; filename=\""+filename+"\"")
	return content
}

// --- Admin-facing "مدیریت ربات فاکسیمای نمایندگان" endpoints below ---
// Deliberately separate from the reseller-self-service handlers above
// (which always resolve resellerID from the caller's own JWT claim):
// these take resellerID from the URL path and are admin-only, matching
// ResellerController's own :id-scoped admin routes -- the admin's own
// explicit request that running a reseller's Faoxima instance costs the
// panel real resources, so an admin (not the reseller themselves) needs
// to control enable/disable and the billing period.

// AdminListInstances returns every provisioned Faoxima instance with its
// owning reseller's name, admin only.
func (c *FaoximaController) AdminListInstances(ctx echo.Context) error {
	if forbidden, err := forbidUnlessAdmin(ctx); forbidden {
		return err
	}

	summaries, err := c.provisioner.ListInstancesWithResellerNames()
	if err != nil {
		c.logger.Error("failed to list faoxima instances", zap.Error(err))
		return ctx.JSON(http.StatusInternalServerError, schema.InternalServerErrorResponse)
	}

	resp := make([]schema.FaoximaAdminInstanceResponse, 0, len(summaries))
	for _, s := range summaries {
		item := schema.FaoximaAdminInstanceResponse{
			ResellerID:        s.Instance.ResellerID,
			ResellerName:      s.ResellerName,
			Status:            s.Instance.Status,
			ErrorMessage:      s.Instance.ErrorMessage,
			BillingPeriodDays: s.Instance.BillingPeriodDays,
			PeriodActivatedAt: s.Instance.PeriodActivatedAt,
			CreatedAt:         s.Instance.CreatedAt,
		}
		if s.Instance.BillingPeriodDays != nil && s.Instance.PeriodActivatedAt != nil {
			expiresAt := s.Instance.PeriodActivatedAt.AddDate(0, 0, *s.Instance.BillingPeriodDays)
			item.PeriodExpiresAt = &expiresAt
		}
		resp = append(resp, item)
	}
	return ctx.JSON(http.StatusOK, schema.BasicResponseData[[]schema.FaoximaAdminInstanceResponse]{
		BasicResponse: schema.OkBasicResponse,
		Data:          resp,
	})
}

// AdminEnableInstance is the admin's own "فعال‌سازی" action for one
// reseller's instance, optionally starting (or restarting) a billing
// period -- see service.FaoximaProvisionerService.AdminEnableWithPeriod's
// own doc comment.
func (c *FaoximaController) AdminEnableInstance(ctx echo.Context) error {
	if forbidden, err := forbidUnlessAdmin(ctx); forbidden {
		return err
	}
	resellerID, err := adminResellerIDFromParam(ctx)
	if err != nil {
		return err
	}

	var req schema.AdminEnableFaoximaRequest
	if err := ctx.Bind(&req); err != nil {
		return ctx.JSON(http.StatusBadRequest, schema.BadParamsErrorResponse)
	}

	if err := c.provisioner.AdminEnableWithPeriod(resellerID, req.PeriodDays); err != nil {
		c.logger.Error("failed to admin-enable faoxima instance", zap.Uint("reseller_id", resellerID), zap.Error(err))
		return ctx.JSON(http.StatusInternalServerError, schema.ErrorResponse{StatusCode: http.StatusInternalServerError, Status: "error", Message: err.Error()})
	}
	return ctx.JSON(http.StatusOK, schema.OkBasicResponse)
}

// AdminDisableInstance is the admin's own "غیرفعال‌سازی" action for one
// reseller's instance -- identical mechanically to the reseller's own
// Disable, just admin-triggered and :id-scoped.
func (c *FaoximaController) AdminDisableInstance(ctx echo.Context) error {
	if forbidden, err := forbidUnlessAdmin(ctx); forbidden {
		return err
	}
	resellerID, err := adminResellerIDFromParam(ctx)
	if err != nil {
		return err
	}

	if err := c.provisioner.Disable(resellerID); err != nil {
		c.logger.Error("failed to admin-disable faoxima instance", zap.Uint("reseller_id", resellerID), zap.Error(err))
		return ctx.JSON(http.StatusInternalServerError, schema.ErrorResponse{StatusCode: http.StatusInternalServerError, Status: "error", Message: err.Error()})
	}
	return ctx.JSON(http.StatusOK, schema.OkBasicResponse)
}

// AdminResetPeriod is the admin's own "ریست دوره" action once a reseller
// pays for their next billing period -- restarts the countdown and
// re-enables the instance if it had been auto-disabled by
// CheckExpiredPeriods.
func (c *FaoximaController) AdminResetPeriod(ctx echo.Context) error {
	if forbidden, err := forbidUnlessAdmin(ctx); forbidden {
		return err
	}
	resellerID, err := adminResellerIDFromParam(ctx)
	if err != nil {
		return err
	}

	if err := c.provisioner.AdminResetPeriod(resellerID); err != nil {
		c.logger.Error("failed to reset faoxima instance period", zap.Uint("reseller_id", resellerID), zap.Error(err))
		return ctx.JSON(http.StatusInternalServerError, schema.ErrorResponse{StatusCode: http.StatusInternalServerError, Status: "error", Message: err.Error()})
	}
	return ctx.JSON(http.StatusOK, schema.OkBasicResponse)
}

// Restore accepts an uploaded .sql file (multipart form field "file") and
// imports it into this reseller's own Faoxima database.
func (c *FaoximaController) Restore(ctx echo.Context) error {
	resellerID, ok := c.resellerIDFromContext(ctx)
	if !ok {
		return ctx.JSON(http.StatusForbidden, schema.ErrorResponse{StatusCode: http.StatusForbidden, Status: "error", Message: "forbidden"})
	}

	fileHeader, err := ctx.FormFile("file")
	if err != nil {
		return ctx.JSON(http.StatusBadRequest, schema.ErrorResponse{StatusCode: http.StatusBadRequest, Status: "error", Message: "فایل بکاپ ارسال نشده است"})
	}
	src, err := fileHeader.Open()
	if err != nil {
		return ctx.JSON(http.StatusBadRequest, schema.BadParamsErrorResponse)
	}
	defer src.Close()

	content, err := io.ReadAll(src)
	if err != nil {
		return ctx.JSON(http.StatusBadRequest, schema.BadParamsErrorResponse)
	}

	if err := c.provisioner.Restore(resellerID, content); err != nil {
		c.logger.Error("failed to restore faoxima instance", zap.Uint("reseller_id", resellerID), zap.Error(err))
		return ctx.JSON(http.StatusInternalServerError, schema.ErrorResponse{StatusCode: http.StatusInternalServerError, Status: "error", Message: err.Error()})
	}

	return ctx.JSON(http.StatusOK, schema.OkBasicResponse)
}
