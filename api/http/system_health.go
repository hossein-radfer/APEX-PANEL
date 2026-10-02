package http

import (
	"net/http"

	"github.com/maahdima/mwp/api/http/schema"
	"github.com/maahdima/mwp/api/service"

	"github.com/labstack/echo/v4"
	"go.uber.org/zap"
)

// SystemHealthController exposes the admin-only "panel health dashboard"
// (فاز پنجم-۵) -- live CPU/RAM/disk usage plus backup freshness, all in
// one endpoint since the frontend renders them as a single card.
type SystemHealthController struct {
	systemHealthService *service.SystemHealthService
	logger              *zap.Logger
}

func NewSystemHealthController(systemHealthService *service.SystemHealthService) *SystemHealthController {
	return &SystemHealthController{
		systemHealthService: systemHealthService,
		logger:              zap.L().Named("SystemHealthController"),
	}
}

// GetSystemHealth reports live host resource usage and backup freshness.
// Admin only -- like GetDatabaseSize, this exposes host-level details
// (memory/disk totals) irrelevant to a reseller's own view of the panel.
func (c *SystemHealthController) GetSystemHealth(ctx echo.Context) error {
	if forbidden, err := forbidUnlessAdmin(ctx); forbidden {
		return err
	}

	health := c.systemHealthService.GetSystemHealth()

	return ctx.JSON(http.StatusOK, schema.BasicResponseData[schema.SystemHealthResponse]{
		BasicResponse: schema.OkBasicResponse,
		Data: schema.SystemHealthResponse{
			CPUPercent:         health.CPUPercent,
			MemoryTotalBytes:   health.Memory.TotalBytes,
			MemoryUsedBytes:    health.Memory.UsedBytes,
			DiskTotalBytes:     health.Disk.TotalBytes,
			DiskUsedBytes:      health.Disk.UsedBytes,
			AutoBackupEnabled:  health.Backup.AutoBackupEnabled,
			LastBackupSentDate: health.Backup.LastBackupSentDate,
			BackupIsUpToDate:   health.Backup.IsUpToDate,
		},
	})
}
