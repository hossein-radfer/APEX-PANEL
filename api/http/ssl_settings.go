package http

import (
	"net/http"

	"github.com/maahdima/mwp/api/dataservice/model"
	"github.com/maahdima/mwp/api/http/schema"
	"github.com/maahdima/mwp/api/service"

	"github.com/labstack/echo/v4"
	"go.uber.org/zap"
)

// SSLSettingsController backs the Settings > SSL tab -- a "bring your own
// certificate" configuration, not an ACME client. The admin runs Certbot
// (or any other tool) on the host themselves and pastes the resulting
// certificate/key file PATHS here; this panel only ever reads those two
// files, it never issues or renews anything itself.
type SSLSettingsController struct {
	settingsService *service.SSLSettingsService
	logger          *zap.Logger
}

func NewSSLSettingsController(settingsService *service.SSLSettingsService) *SSLSettingsController {
	return &SSLSettingsController{
		settingsService: settingsService,
		logger:          zap.L().Named("SSLSettingsController"),
	}
}

func toSSLSettingsResponse(s *model.SSLSettings) schema.SSLSettingsResponse {
	return schema.SSLSettingsResponse{
		Domain:          s.Domain,
		CertificatePath: s.CertificatePath,
		PrivateKeyPath:  s.PrivateKeyPath,
		Enabled:         s.Enabled,
	}
}

// GetSettings returns the current SSL configuration. Admin only -- mirrors
// BotSettingsController.GetSettings' own access rule.
func (c *SSLSettingsController) GetSettings(ctx echo.Context) error {
	if forbidden, err := forbidUnlessAdmin(ctx); forbidden {
		return err
	}

	settings, err := c.settingsService.GetOrCreate()
	if err != nil {
		c.logger.Error("failed to get ssl settings", zap.Error(err))
		return ctx.JSON(http.StatusInternalServerError, schema.InternalServerErrorResponse)
	}

	return ctx.JSON(http.StatusOK, schema.BasicResponseData[schema.SSLSettingsResponse]{
		BasicResponse: schema.OkBasicResponse,
		Data:          toSSLSettingsResponse(settings),
	})
}

// UpdateSettings saves the SSL configuration. Enabling SSL (or changing a
// path while already enabled) is validated against the actual files on
// disk before anything is persisted -- see
// SSLSettingsService.UpdateSettings' own doc comment. This endpoint never
// restarts the HTTP listener itself: switching between plain HTTP and TLS
// changes which port/protocol the server binds on at process startup (see
// cmd/http-server/http-server.go), so a change here takes effect on the
// panel's next restart, exactly like BotToken changes on this same
// settings page already require for some effects.
func (c *SSLSettingsController) UpdateSettings(ctx echo.Context) error {
	if forbidden, err := forbidUnlessAdmin(ctx); forbidden {
		return err
	}

	var req schema.UpdateSSLSettingsRequest
	if err := ctx.Bind(&req); err != nil {
		c.logger.Warn("failed to bind request", zap.Error(err))
		return ctx.JSON(http.StatusBadRequest, schema.BadParamsErrorResponse)
	}

	settings, err := c.settingsService.UpdateSettings(service.UpdateSSLSettingsInput{
		Domain:          req.Domain,
		CertificatePath: req.CertificatePath,
		PrivateKeyPath:  req.PrivateKeyPath,
		Enabled:         req.Enabled,
	})
	if err != nil {
		c.logger.Warn("failed to update ssl settings", zap.Error(err))
		return ctx.JSON(http.StatusBadRequest, schema.ErrorResponse{
			StatusCode: http.StatusBadRequest,
			Status:     "error",
			Message:    err.Error(),
		})
	}

	return ctx.JSON(http.StatusOK, schema.BasicResponseData[schema.SSLSettingsResponse]{
		BasicResponse: schema.OkBasicResponse,
		Data:          toSSLSettingsResponse(settings),
	})
}
