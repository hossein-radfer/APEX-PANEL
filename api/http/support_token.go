package http

import (
	"errors"
	"net/http"
	"strconv"
	"time"

	"github.com/labstack/echo/v4"
	"go.uber.org/zap"

	"github.com/maahdima/mwp/api/http/schema"
	"github.com/maahdima/mwp/api/service"
)

// SupportTokenController exposes two very different surfaces over the
// same SupportTokenService, kept in one file since they're two views of
// one feature:
//   - Generate/Status/Revoke are JWT-admin-only, reached from this
//     install's own settings page ("Generate Support Token" button).
//   - Logs is PUBLIC (no JWT) but requires a valid, unexpired support
//     token passed as a bearer credential -- this is the one and only
//     thing an outsider holding a support token can ever retrieve: this
//     install's own log entries, read-only, filtered the same way the
//     admin-only Log Viewer already filters them. See
//     model.SupportToken's doc comment for the full privacy rationale
//     (no SSH, no shell, no server status -- logs only).
type SupportTokenController struct {
	tokenService *service.SupportTokenService
	logReader    *service.LogReader
	logger       *zap.Logger
}

func NewSupportTokenController(tokenService *service.SupportTokenService, logReader *service.LogReader) *SupportTokenController {
	return &SupportTokenController{
		tokenService: tokenService,
		logReader:    logReader,
		logger:       zap.L().Named("SupportTokenController"),
	}
}

func (c *SupportTokenController) requireAdmin(ctx echo.Context) bool {
	role, _ := getRoleAndResellerFromContext(ctx)
	if role != "admin" {
		_ = ctx.JSON(http.StatusForbidden, schema.ErrorResponse{StatusCode: http.StatusForbidden, Status: "error", Message: "forbidden"})
		return false
	}
	return true
}

func (c *SupportTokenController) Generate(ctx echo.Context) error {
	if !c.requireAdmin(ctx) {
		return nil
	}

	token, expiresAt, err := c.tokenService.Generate()
	if err != nil {
		c.logger.Error("failed to generate support token", zap.Error(err))
		return ctx.JSON(http.StatusInternalServerError, schema.InternalServerErrorResponse)
	}

	return ctx.JSON(http.StatusOK, schema.BasicResponseData[schema.GenerateSupportTokenResponse]{
		BasicResponse: schema.OkBasicResponse,
		Data: schema.GenerateSupportTokenResponse{
			Token:     token,
			ExpiresAt: expiresAt.UTC().Format(time.RFC3339),
		},
	})
}

func (c *SupportTokenController) Status(ctx echo.Context) error {
	if !c.requireAdmin(ctx) {
		return nil
	}

	active, expiresAt, err := c.tokenService.Status()
	if err != nil {
		c.logger.Error("failed to read support token status", zap.Error(err))
		return ctx.JSON(http.StatusInternalServerError, schema.InternalServerErrorResponse)
	}

	resp := schema.SupportTokenStatusResponse{Active: active}
	if active {
		resp.ExpiresAt = expiresAt.UTC().Format(time.RFC3339)
	}

	return ctx.JSON(http.StatusOK, schema.BasicResponseData[schema.SupportTokenStatusResponse]{
		BasicResponse: schema.OkBasicResponse,
		Data:          resp,
	})
}

func (c *SupportTokenController) Revoke(ctx echo.Context) error {
	if !c.requireAdmin(ctx) {
		return nil
	}

	if err := c.tokenService.Revoke(); err != nil {
		c.logger.Error("failed to revoke support token", zap.Error(err))
		return ctx.JSON(http.StatusInternalServerError, schema.InternalServerErrorResponse)
	}

	return ctx.JSON(http.StatusOK, schema.OkBasicResponse)
}

// supportTokenErrorStatus maps a Verify() error to the HTTP status the
// caller sees -- 401 for every rejection reason (invalid, expired, not
// configured), deliberately not distinguishing them in the response body
// so an outsider probing this endpoint can't use the error message to
// tell "no token exists" apart from "wrong token" apart from "right
// token, but expired" (all three should look identical from outside).
func supportTokenErrorStatus(err error) int {
	switch {
	case errors.Is(err, service.ErrSupportTokenNotConfigured),
		errors.Is(err, service.ErrSupportTokenExpired),
		errors.Is(err, service.ErrSupportTokenInvalid):
		return http.StatusUnauthorized
	default:
		return http.StatusInternalServerError
	}
}

// Logs is the public, token-gated log-read endpoint -- the token is read
// from the Authorization header (Bearer <token>) exactly like a JWT would
// be, so the license-panel side's HTTP client can reuse the same
// "Authorization: Bearer X" convention it already uses everywhere else,
// without needing a special case for this one call.
func (c *SupportTokenController) Logs(ctx echo.Context) error {
	token := bearerToken(ctx)
	if token == "" {
		return ctx.JSON(http.StatusUnauthorized, schema.ErrorResponse{StatusCode: http.StatusUnauthorized, Status: "error", Message: "missing support token"})
	}

	if err := c.tokenService.Verify(token); err != nil {
		return ctx.JSON(supportTokenErrorStatus(err), schema.ErrorResponse{StatusCode: http.StatusUnauthorized, Status: "error", Message: "invalid or expired support token"})
	}

	limit := 100
	if limitStr := ctx.QueryParam("limit"); limitStr != "" {
		if parsed, err := strconv.Atoi(limitStr); err == nil && parsed > 0 && parsed <= 2000 {
			limit = parsed
		}
	}

	filter := service.LogFilter{
		Level:  ctx.QueryParam("level"),
		Logger: ctx.QueryParam("logger"),
		Search: ctx.QueryParam("search"),
		Limit:  limit,
	}

	entries, err := c.logReader.ListLogs(filter)
	if err != nil {
		c.logger.Error("failed to list logs for support token request", zap.Error(err))
		return ctx.JSON(http.StatusInternalServerError, schema.InternalServerErrorResponse)
	}

	response := make([]schema.LogEntryResponse, len(entries))
	for i, e := range entries {
		response[i] = schema.LogEntryResponse{
			Timestamp: e.Timestamp,
			Level:     e.Level,
			Logger:    e.Logger,
			Caller:    e.Caller,
			Message:   e.Message,
			Fields:    e.Fields,
		}
	}

	return ctx.JSON(http.StatusOK, schema.BasicResponseData[schema.ListLogsResponse]{
		BasicResponse: schema.OkBasicResponse,
		Data:          schema.ListLogsResponse{Entries: response},
	})
}

// bearerToken extracts the token from "Authorization: Bearer <token>",
// returning "" if the header is missing or malformed.
func bearerToken(ctx echo.Context) string {
	const prefix = "Bearer "
	header := ctx.Request().Header.Get("Authorization")
	if len(header) <= len(prefix) || header[:len(prefix)] != prefix {
		return ""
	}
	return header[len(prefix):]
}
