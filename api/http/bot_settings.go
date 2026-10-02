package http

import (
	"errors"
	"net/http"
	"strconv"
	"strings"

	"github.com/maahdima/mwp/api/dataservice/model"
	"github.com/maahdima/mwp/api/http/schema"
	"github.com/maahdima/mwp/api/service"

	"github.com/labstack/echo/v4"
	"go.uber.org/zap"
)

type BotSettingsController struct {
	settingsService           *service.BotSettingsService
	botService                *service.BotService
	faoximaProvisionerService *service.FaoximaProvisionerService
	logger                    *zap.Logger
}

func NewBotSettingsController(settingsService *service.BotSettingsService, botService *service.BotService, faoximaProvisionerService *service.FaoximaProvisionerService) *BotSettingsController {
	return &BotSettingsController{
		settingsService:           settingsService,
		botService:                botService,
		faoximaProvisionerService: faoximaProvisionerService,
		logger:                    zap.L().Named("BotSettingsController"),
	}
}

// maskBotToken shows only the last 4 characters of a bot token (or nothing,
// if it's shorter than that) -- enough for an admin to recognize "yes,
// that's the token I set" without the full secret ever round-tripping back
// to the browser on every settings-page load.
func maskBotToken(token string) string {
	if token == "" {
		return ""
	}
	if len(token) <= 4 {
		return strings.Repeat("*", len(token))
	}
	return strings.Repeat("*", len(token)-4) + token[len(token)-4:]
}

func toBotSettingsResponse(s *model.BotSettings) schema.BotSettingsResponse {
	return schema.BotSettingsResponse{
		BotTokenSet:            s.BotToken != "",
		BotTokenMasked:         maskBotToken(s.BotToken),
		AdminChatID:            s.AdminChatID,
		Enabled:                s.Enabled,
		NotifyLoginAlerts:      s.NotifyLoginAlerts,
		NotifyPurchaseReceipts: s.NotifyPurchaseReceipts,
		NotifyLiveLog:          s.NotifyLiveLog,
		NotifyQuotaWarnings:    s.NotifyQuotaWarnings,
		NotifyCriticalAlerts:   s.NotifyCriticalAlerts,
		NotifyAccountStatus:    s.NotifyAccountStatus,
		OtpEnabled:             s.OtpEnabled,
		AutoBackupEnabled:      s.AutoBackupEnabled,
		AutoBackupHour:         s.AutoBackupHour,
		AutoBackupMinute:       s.AutoBackupMinute,
		AutoReportEnabled:      s.AutoReportEnabled,
		AutoReportHour:         s.AutoReportHour,
		AutoReportMinute:       s.AutoReportMinute,
		Socks5Enabled:          s.Socks5Enabled,
		Socks5Address:          s.Socks5Address,
		Socks5Username:         s.Socks5Username,
		Socks5PasswordSet:      s.Socks5Password != "",
		FaoximaDomain:          s.FaoximaDomain,
	}
}

// GetSettings returns the bot configuration. Admin only -- even the masked
// view leaks the admin chat ID and feature toggles, which aren't reseller
// information.
func (c *BotSettingsController) GetSettings(ctx echo.Context) error {
	if forbidden, err := forbidUnlessAdmin(ctx); forbidden {
		return err
	}

	settings, err := c.settingsService.GetOrCreate()
	if err != nil {
		c.logger.Error("failed to get bot settings", zap.Error(err))
		return ctx.JSON(http.StatusInternalServerError, schema.InternalServerErrorResponse)
	}

	return ctx.JSON(http.StatusOK, schema.BasicResponseData[schema.BotSettingsResponse]{
		BasicResponse: schema.OkBasicResponse,
		Data:          toBotSettingsResponse(settings),
	})
}

func (c *BotSettingsController) UpdateSettings(ctx echo.Context) error {
	if forbidden, err := forbidUnlessAdmin(ctx); forbidden {
		return err
	}

	var req schema.UpdateBotSettingsRequest
	if err := ctx.Bind(&req); err != nil {
		c.logger.Warn("failed to bind request", zap.Error(err))
		return ctx.JSON(http.StatusBadRequest, schema.BadParamsErrorResponse)
	}

	settings, err := c.settingsService.UpdateSettings(service.UpdateSettingsInput{
		BotToken:               req.BotToken,
		AdminChatID:            req.AdminChatID,
		Enabled:                req.Enabled,
		NotifyLoginAlerts:      req.NotifyLoginAlerts,
		NotifyPurchaseReceipts: req.NotifyPurchaseReceipts,
		NotifyLiveLog:          req.NotifyLiveLog,
		NotifyQuotaWarnings:    req.NotifyQuotaWarnings,
		NotifyCriticalAlerts:   req.NotifyCriticalAlerts,
		NotifyAccountStatus:    req.NotifyAccountStatus,
		OtpEnabled:             req.OtpEnabled,
		Socks5Enabled:          req.Socks5Enabled,
		Socks5Address:          req.Socks5Address,
		Socks5Username:         req.Socks5Username,
		Socks5Password:         req.Socks5Password,
		FaoximaDomain:          req.FaoximaDomain,
	})
	if err != nil {
		c.logger.Error("failed to update bot settings", zap.Error(err))
		return ctx.JSON(http.StatusInternalServerError, schema.ErrorResponse{
			StatusCode: http.StatusInternalServerError,
			Status:     "error",
			Message:    err.Error(),
		})
	}

	// Apply the new token/enabled state immediately -- without this, a
	// settings change would silently do nothing until the whole panel
	// process is restarted, which isn't obvious from the API response.
	if err := c.botService.Reload(); err != nil {
		c.logger.Warn("bot settings saved but failed to (re)connect with the new configuration", zap.Error(err))
	}

	// Every reseller's Faoxima instance shares this same SOCKS5 setting
	// (see BotSettings.Socks5Enabled's own doc comment) -- without this,
	// an admin fixing/rotating a proxy address here would have every
	// Faoxima webhook call keep using the STALE client until a full
	// process restart, with nothing in this response indicating that.
	// Same reasoning for FaoximaDomain: an admin changing it here must take
	// effect on the next provisioning/webhook call, not after a restart.
	if c.faoximaProvisionerService != nil {
		c.faoximaProvisionerService.ReloadHTTPClient(settings)
		c.faoximaProvisionerService.ReloadFaoximaDomain(settings)
	}

	return ctx.JSON(http.StatusOK, schema.BasicResponseData[schema.BotSettingsResponse]{
		BasicResponse: schema.OkBasicResponse,
		Data:          toBotSettingsResponse(settings),
	})
}

// ListExtraAdminChatIDs surfaces the same list the bot's own /listadmins
// command shows -- see schema.ExtraAdminChatIDResponse's own doc comment.
func (c *BotSettingsController) ListExtraAdminChatIDs(ctx echo.Context) error {
	if forbidden, err := forbidUnlessAdmin(ctx); forbidden {
		return err
	}

	ids, err := c.settingsService.ListExtraAdminChatIDs()
	if err != nil {
		c.logger.Error("failed to list extra admin chat ids", zap.Error(err))
		return ctx.JSON(http.StatusInternalServerError, schema.InternalServerErrorResponse)
	}

	resp := make([]schema.ExtraAdminChatIDResponse, 0, len(ids))
	for _, id := range ids {
		resp = append(resp, schema.ExtraAdminChatIDResponse{Id: id.ID, ChatID: id.ChatID, Label: id.Label})
	}

	return ctx.JSON(http.StatusOK, schema.BasicResponseData[[]schema.ExtraAdminChatIDResponse]{
		BasicResponse: schema.OkBasicResponse,
		Data:          resp,
	})
}

func (c *BotSettingsController) AddExtraAdminChatID(ctx echo.Context) error {
	if forbidden, err := forbidUnlessAdmin(ctx); forbidden {
		return err
	}

	var req schema.AddExtraAdminChatIDRequest
	if err := ctx.Bind(&req); err != nil {
		return ctx.JSON(http.StatusBadRequest, schema.BadParamsErrorResponse)
	}
	if err := ctx.Validate(&req); err != nil {
		return ctx.JSON(http.StatusBadRequest, schema.BadParamsErrorResponse)
	}

	entry, err := c.settingsService.AddExtraAdminChatID(req.ChatID, req.Label)
	if err != nil {
		if errors.Is(err, service.ErrInvalidTelegramChatID) {
			return ctx.JSON(http.StatusBadRequest, schema.ErrorResponse{StatusCode: http.StatusBadRequest, Status: "error", Message: "آیدی چت باید یک عدد باشد (آیدی عددی چت تلگرام)، نه یک متن دلخواه."})
		}
		return ctx.JSON(http.StatusBadRequest, schema.ErrorResponse{StatusCode: http.StatusBadRequest, Status: "error", Message: "این آیدی قبلاً اضافه شده یا خطایی رخ داد: " + err.Error()})
	}

	return ctx.JSON(http.StatusCreated, schema.BasicResponseData[schema.ExtraAdminChatIDResponse]{
		BasicResponse: schema.OkBasicResponse,
		Data:          schema.ExtraAdminChatIDResponse{Id: entry.ID, ChatID: entry.ChatID, Label: entry.Label},
	})
}

// TestSocks5 probes the given (not-yet-necessarily-saved) SOCKS5 proxy
// against Telegram's own API and reports connected/disconnected + ping --
// item 5's core feature. Never returns a 4xx/5xx for a failed PROXY
// connection (that's a normal, expected outcome the admin is actively
// checking for) -- only a malformed request itself is a 400.
func (c *BotSettingsController) TestSocks5(ctx echo.Context) error {
	if forbidden, err := forbidUnlessAdmin(ctx); forbidden {
		return err
	}

	var req schema.TestSocks5Request
	if err := ctx.Bind(&req); err != nil {
		return ctx.JSON(http.StatusBadRequest, schema.BadParamsErrorResponse)
	}
	if err := ctx.Validate(&req); err != nil {
		return ctx.JSON(http.StatusBadRequest, schema.BadParamsErrorResponse)
	}

	password := req.Password
	if req.PasswordUnchanged {
		settings, err := c.settingsService.GetOrCreate()
		if err != nil {
			c.logger.Error("failed to load bot settings for socks5 test", zap.Error(err))
			return ctx.JSON(http.StatusInternalServerError, schema.InternalServerErrorResponse)
		}
		password = settings.Socks5Password
	}

	result := service.TestSocks5Connection(req.Address, req.Username, password)

	return ctx.JSON(http.StatusOK, schema.BasicResponseData[schema.TestSocks5Response]{
		BasicResponse: schema.OkBasicResponse,
		Data: schema.TestSocks5Response{
			Connected: result.Connected,
			PingMs:    result.PingMS,
			Error:     result.Error,
		},
	})
}

func (c *BotSettingsController) RemoveExtraAdminChatID(ctx echo.Context) error {
	if forbidden, err := forbidUnlessAdmin(ctx); forbidden {
		return err
	}

	id, err := strconv.Atoi(ctx.Param("id"))
	if err != nil {
		return ctx.JSON(http.StatusBadRequest, schema.BadParamsErrorResponse)
	}

	if err := c.settingsService.RemoveExtraAdminChatID(uint(id)); err != nil {
		return ctx.JSON(http.StatusInternalServerError, schema.InternalServerErrorResponse)
	}
	return ctx.NoContent(http.StatusNoContent)
}
