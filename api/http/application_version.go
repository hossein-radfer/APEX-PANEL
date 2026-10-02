package http

import (
	"net/http"
	"strconv"

	"github.com/labstack/echo/v4"
	"go.uber.org/zap"

	"github.com/maahdima/mwp/api/http/schema"
	"github.com/maahdima/mwp/api/service"
)

// ApplicationVersionController is the admin-only "مدیریت اپلیکیشن"
// (Application Management) hamburger-menu section: publish a new mobile
// app release, mark it mandatory, and toggle global maintenance mode.
// Every endpoint here is gated by forbidUnlessAdmin -- resellers have no
// visibility into this at all, matching the admin's own explicit "فقط
// برای مدیر" requirement. The mobile app's own read side
// (GET /api/app/version-check) lives on AppAuthController instead, since
// it's a public, unauthenticated, app-facing endpoint, not an
// admin-panel one.
type ApplicationVersionController struct {
	versionService *service.ApplicationVersionService
	logger         *zap.Logger
}

func NewApplicationVersionController(versionService *service.ApplicationVersionService) *ApplicationVersionController {
	return &ApplicationVersionController{
		versionService: versionService,
		logger:         zap.L().Named("ApplicationVersionController"),
	}
}

func (c *ApplicationVersionController) internalError(ctx echo.Context, action string, err error) error {
	c.logger.Error("failed to "+action, zap.Error(err))
	return ctx.JSON(http.StatusInternalServerError, schema.ErrorResponse{
		StatusCode: http.StatusInternalServerError,
		Status:     "error",
		Message:    "failed to " + action + ": " + err.Error(),
	})
}

func toAppVersionResponse(id uint, versionCode int, versionName string, releaseNotes *string, downloadURL string, isMandatory bool, publishedAt string) schema.AppVersionResponse {
	return schema.AppVersionResponse{
		Id:           id,
		VersionCode:  versionCode,
		VersionName:  versionName,
		ReleaseNotes: releaseNotes,
		DownloadURL:  downloadURL,
		IsMandatory:  isMandatory,
		PublishedAt:  publishedAt,
	}
}

// PublishVersion publishes a new app release (or updates an existing
// one's metadata, if VersionCode already exists -- see
// ApplicationVersionService.PublishVersion's own doc comment). Admin only.
func (c *ApplicationVersionController) PublishVersion(ctx echo.Context) error {
	if forbidden, err := forbidUnlessAdmin(ctx); forbidden {
		return err
	}

	var req schema.PublishAppVersionRequest
	if err := ctx.Bind(&req); err != nil {
		c.logger.Warn("failed to bind request", zap.Error(err))
		return ctx.JSON(http.StatusBadRequest, schema.BadParamsErrorResponse)
	}
	if err := ctx.Validate(&req); err != nil {
		c.logger.Warn("failed to validate request", zap.Error(err))
		return ctx.JSON(http.StatusBadRequest, schema.BadParamsErrorResponse)
	}

	version, err := c.versionService.PublishVersion(req.VersionCode, req.VersionName, req.ReleaseNotes, req.DownloadURL, req.IsMandatory)
	if err != nil {
		return c.internalError(ctx, "publish app version", err)
	}

	return ctx.JSON(http.StatusCreated, schema.BasicResponseData[schema.AppVersionResponse]{
		BasicResponse: schema.OkBasicResponse,
		Data: toAppVersionResponse(version.ID, version.VersionCode, version.VersionName, version.ReleaseNotes, version.DownloadURL, version.IsMandatory, version.PublishedAt.Format("2006-01-02T15:04:05Z07:00")),
	})
}

// ListVersions returns every published version, newest first. Admin only.
func (c *ApplicationVersionController) ListVersions(ctx echo.Context) error {
	if forbidden, err := forbidUnlessAdmin(ctx); forbidden {
		return err
	}

	versions, err := c.versionService.ListVersions()
	if err != nil {
		return c.internalError(ctx, "list app versions", err)
	}

	resp := make([]schema.AppVersionResponse, len(versions))
	for i, v := range versions {
		resp[i] = toAppVersionResponse(v.ID, v.VersionCode, v.VersionName, v.ReleaseNotes, v.DownloadURL, v.IsMandatory, v.PublishedAt.Format("2006-01-02T15:04:05Z07:00"))
	}

	return ctx.JSON(http.StatusOK, schema.BasicResponseData[schema.AppVersionsResponse]{
		BasicResponse: schema.OkBasicResponse,
		Data:          schema.AppVersionsResponse{Versions: resp},
	})
}

// DeleteVersion removes one published version. Admin only.
func (c *ApplicationVersionController) DeleteVersion(ctx echo.Context) error {
	if forbidden, err := forbidUnlessAdmin(ctx); forbidden {
		return err
	}

	id, err := strconv.ParseUint(ctx.Param("id"), 10, 32)
	if err != nil {
		return ctx.JSON(http.StatusBadRequest, schema.BadParamsErrorResponse)
	}

	if err := c.versionService.DeleteVersion(uint(id)); err != nil {
		return c.internalError(ctx, "delete app version", err)
	}

	return ctx.JSON(http.StatusOK, schema.OkBasicResponse)
}

// GetMaintenanceMode reports the current global maintenance gate state.
// Admin only.
func (c *ApplicationVersionController) GetMaintenanceMode(ctx echo.Context) error {
	if forbidden, err := forbidUnlessAdmin(ctx); forbidden {
		return err
	}

	enabled, message := c.versionService.GetMaintenanceMode()

	return ctx.JSON(http.StatusOK, schema.BasicResponseData[schema.AppMaintenanceModeResponse]{
		BasicResponse: schema.OkBasicResponse,
		Data:          schema.AppMaintenanceModeResponse{Enabled: enabled, Message: message},
	})
}

// SetMaintenanceMode enables/disables the global maintenance gate --
// blocks EVERY app version, including the latest one, independent of
// the mandatory-update mechanism (e.g. during a server migration).
// Admin only.
func (c *ApplicationVersionController) SetMaintenanceMode(ctx echo.Context) error {
	if forbidden, err := forbidUnlessAdmin(ctx); forbidden {
		return err
	}

	var req schema.SetAppMaintenanceModeRequest
	if err := ctx.Bind(&req); err != nil {
		c.logger.Warn("failed to bind request", zap.Error(err))
		return ctx.JSON(http.StatusBadRequest, schema.BadParamsErrorResponse)
	}

	if err := c.versionService.SetMaintenanceMode(req.Enabled, req.Message); err != nil {
		return c.internalError(ctx, "set app maintenance mode", err)
	}

	enabled, message := c.versionService.GetMaintenanceMode()
	return ctx.JSON(http.StatusOK, schema.BasicResponseData[schema.AppMaintenanceModeResponse]{
		BasicResponse: schema.OkBasicResponse,
		Data:          schema.AppMaintenanceModeResponse{Enabled: enabled, Message: message},
	})
}
