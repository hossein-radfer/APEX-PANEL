package http

import (
	"net/http"
	"strconv"

	"github.com/labstack/echo/v4"
	"go.uber.org/zap"

	"github.com/maahdima/mwp/api/http/schema"
	"github.com/maahdima/mwp/api/service"
)

// ApiKeyController is the JWT-admin-only management surface for phase
// 4-3's external API keys -- see model.ApiKey's own doc comment for what
// these keys unlock (the separate, JWT-independent setupExternalApiRoutes
// surface, gated instead by middleware.APIKeyMiddleware).
type ApiKeyController struct {
	keyService *service.ApiKeyService
	logger     *zap.Logger
}

func NewApiKeyController(keyService *service.ApiKeyService) *ApiKeyController {
	return &ApiKeyController{
		keyService: keyService,
		logger:     zap.L().Named("ApiKeyController"),
	}
}

func (c *ApiKeyController) requireAdmin(ctx echo.Context) bool {
	role, _ := getRoleAndResellerFromContext(ctx)
	if role != "admin" {
		_ = ctx.JSON(http.StatusForbidden, schema.ErrorResponse{StatusCode: http.StatusForbidden, Status: "error", Message: "forbidden"})
		return false
	}
	return true
}

func (c *ApiKeyController) Create(ctx echo.Context) error {
	if !c.requireAdmin(ctx) {
		return nil
	}

	var req schema.CreateApiKeyRequest
	if err := ctx.Bind(&req); err != nil {
		return ctx.JSON(http.StatusBadRequest, schema.BadParamsErrorResponse)
	}
	if err := ctx.Validate(&req); err != nil {
		return ctx.JSON(http.StatusBadRequest, schema.BadParamsErrorResponse)
	}

	rawKey, id, err := c.keyService.Generate(req.Label)
	if err != nil {
		c.logger.Error("failed to generate api key", zap.Error(err))
		return ctx.JSON(http.StatusInternalServerError, schema.InternalServerErrorResponse)
	}

	return ctx.JSON(http.StatusOK, schema.BasicResponseData[schema.CreateApiKeyResponse]{
		BasicResponse: schema.OkBasicResponse,
		Data: schema.CreateApiKeyResponse{
			ID:    id,
			Key:   rawKey,
			Label: req.Label,
		},
	})
}

func (c *ApiKeyController) List(ctx echo.Context) error {
	if !c.requireAdmin(ctx) {
		return nil
	}

	keys, err := c.keyService.List()
	if err != nil {
		c.logger.Error("failed to list api keys", zap.Error(err))
		return ctx.JSON(http.StatusInternalServerError, schema.InternalServerErrorResponse)
	}

	response := make([]schema.ApiKeyResponse, len(keys))
	for i, k := range keys {
		item := schema.ApiKeyResponse{
			ID:        k.ID,
			Label:     k.Label,
			KeyPrefix: k.KeyPrefix,
			Revoked:   k.Revoked,
			CreatedAt: int64(k.CreatedAt),
		}
		if k.LastUsedAt != nil {
			item.LastUsedAt = k.LastUsedAt.Unix()
		}
		response[i] = item
	}

	return ctx.JSON(http.StatusOK, schema.BasicResponseData[schema.ListApiKeysResponse]{
		BasicResponse: schema.OkBasicResponse,
		Data:          schema.ListApiKeysResponse{Keys: response},
	})
}

func (c *ApiKeyController) Revoke(ctx echo.Context) error {
	if !c.requireAdmin(ctx) {
		return nil
	}

	id, err := strconv.ParseUint(ctx.Param("id"), 10, 64)
	if err != nil {
		return ctx.JSON(http.StatusBadRequest, schema.BadParamsErrorResponse)
	}

	if err := c.keyService.Revoke(uint(id)); err != nil {
		c.logger.Error("failed to revoke api key", zap.Error(err))
		return ctx.JSON(http.StatusInternalServerError, schema.InternalServerErrorResponse)
	}

	return ctx.JSON(http.StatusOK, schema.OkBasicResponse)
}
