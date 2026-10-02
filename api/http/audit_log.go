package http

import (
	"net/http"
	"strconv"

	"github.com/labstack/echo/v4"
	"go.uber.org/zap"

	"github.com/maahdima/mwp/api/http/schema"
	"github.com/maahdima/mwp/api/service"
)

type AuditLogController struct {
	auditLogService *service.AuditLog
	logger          *zap.Logger
}

func NewAuditLogController(auditLogService *service.AuditLog) *AuditLogController {
	return &AuditLogController{
		auditLogService: auditLogService,
		logger:          zap.L().Named("AuditLogController"),
	}
}

// ListRecent returns the most recent reseller activity entries. Admin only.
func (c *AuditLogController) ListRecent(ctx echo.Context) error {
	if role, _ := getRoleAndResellerFromContext(ctx); role == "reseller" {
		return ctx.JSON(http.StatusForbidden, schema.ErrorResponse{StatusCode: http.StatusForbidden, Status: "error", Message: "forbidden"})
	}

	limit := 50
	if limitStr := ctx.QueryParam("limit"); limitStr != "" {
		if parsed, err := strconv.Atoi(limitStr); err == nil && parsed > 0 && parsed <= 200 {
			limit = parsed
		}
	}

	entries, err := c.auditLogService.ListRecent(limit)
	if err != nil {
		c.logger.Error("failed to list audit log entries", zap.Error(err))
		return ctx.JSON(http.StatusInternalServerError, schema.ErrorResponse{
			StatusCode: http.StatusInternalServerError,
			Status:     "error",
			Message:    "failed to retrieve audit log: " + err.Error(),
		})
	}

	response := make([]schema.AuditLogResponse, len(entries))
	for i, e := range entries {
		response[i] = schema.AuditLogResponse{
			ID:           e.ID,
			ResellerID:   e.ResellerID,
			ResellerName: e.ResellerName,
			Action:       e.Action,
			Description:  e.Description,
			CreatedAt:    int64(e.CreatedAt),
		}
	}

	return ctx.JSON(http.StatusOK, schema.BasicResponseData[[]schema.AuditLogResponse]{
		BasicResponse: schema.OkBasicResponse,
		Data:          response,
	})
}
