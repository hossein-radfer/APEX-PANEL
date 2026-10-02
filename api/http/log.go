package http

import (
	"net/http"
	"strconv"
	"time"

	"github.com/labstack/echo/v4"
	"go.uber.org/zap"

	"github.com/maahdima/mwp/api/http/schema"
	"github.com/maahdima/mwp/api/service"
)

// LogController exposes the panel's own structured log file to the admin
// UI (the "Log Viewer" sidebar page), so backend failures -- e.g. a cron
// job silently failing to fetch RouterOS User Manager usage -- can be
// diagnosed from the panel itself instead of requiring server/SSH access.
// Every endpoint here is admin-only: resellers never see panel-internal
// logs.
type LogController struct {
	logReader *service.LogReader
	logger    *zap.Logger
}

func NewLogController(logReader *service.LogReader) *LogController {
	return &LogController{
		logReader: logReader,
		logger:    zap.L().Named("LogController"),
	}
}

func (c *LogController) requireAdmin(ctx echo.Context) bool {
	role, _ := getRoleAndResellerFromContext(ctx)
	if role != "admin" {
		_ = ctx.JSON(http.StatusForbidden, schema.ErrorResponse{StatusCode: http.StatusForbidden, Status: "error", Message: "forbidden"})
		return false
	}
	return true
}

// ListLogs returns filtered, most-recent-first log entries plus the set
// of known categories. Query params: level, logger (category, substring
// match), search, limit (defaults to 100, "0"/absent for the category
// list, capped at 2000 to keep responses bounded), include_rotated
// ("true" to also scan compressed rotated backups for older history).
func (c *LogController) ListLogs(ctx echo.Context) error {
	if !c.requireAdmin(ctx) {
		return nil
	}

	limit := 100
	if limitStr := ctx.QueryParam("limit"); limitStr != "" {
		if parsed, err := strconv.Atoi(limitStr); err == nil && parsed > 0 && parsed <= 2000 {
			limit = parsed
		}
	}

	filter := service.LogFilter{
		Level:          ctx.QueryParam("level"),
		Logger:         ctx.QueryParam("logger"),
		Search:         ctx.QueryParam("search"),
		Limit:          limit,
		IncludeRotated: ctx.QueryParam("include_rotated") == "true",
	}

	entries, err := c.logReader.ListLogs(filter)
	if err != nil {
		c.logger.Error("failed to list logs", zap.Error(err))
		return ctx.JSON(http.StatusInternalServerError, schema.ErrorResponse{
			StatusCode: http.StatusInternalServerError,
			Status:     "error",
			Message:    "failed to read logs: " + err.Error(),
		})
	}

	loggerNames, err := c.logReader.ListLoggerNames()
	if err != nil {
		c.logger.Warn("failed to list logger names", zap.Error(err))
		loggerNames = []string{}
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
		Data: schema.ListLogsResponse{
			Entries:     response,
			LoggerNames: loggerNames,
		},
	})
}

// DownloadCurrentLog streams the active (uncompressed, unrotated) log
// file as-is -- "download this category" from the UI is implemented by
// the frontend re-requesting ListLogs with a logger filter and exporting
// the JSON client-side, since the on-disk file interleaves every
// category together.
func (c *LogController) DownloadCurrentLog(ctx echo.Context) error {
	if !c.requireAdmin(ctx) {
		return nil
	}

	return ctx.Attachment(c.logReader.CurrentLogFilePath(), "mwp.log")
}

// DownloadAllLogs bundles the current log file plus every rotated backup
// into a single zip archive -- the "download everything" button.
func (c *LogController) DownloadAllLogs(ctx echo.Context) error {
	if !c.requireAdmin(ctx) {
		return nil
	}

	filename := "mwp-logs-" + time.Now().Format("2006-01-02-150405") + ".zip"
	ctx.Response().Header().Set(echo.HeaderContentType, "application/zip")
	ctx.Response().Header().Set(echo.HeaderContentDisposition, "attachment; filename=\""+filename+"\"")
	ctx.Response().WriteHeader(http.StatusOK)

	if err := c.logReader.BuildDownloadArchive(ctx.Response()); err != nil {
		c.logger.Error("failed to build log download archive", zap.Error(err))
		return err
	}

	return nil
}

// ClearLogs empties the active log file and deletes every rotated backup
// -- an on-demand escape hatch alongside the automatic 7-day rotation
// (see utils/log.InitLogger's lumberjack MaxAge), for an admin who wants
// disk space back immediately rather than waiting for rotation.
func (c *LogController) ClearLogs(ctx echo.Context) error {
	if !c.requireAdmin(ctx) {
		return nil
	}

	if err := c.logReader.ClearLogs(); err != nil {
		c.logger.Error("failed to clear logs", zap.Error(err))
		return ctx.JSON(http.StatusInternalServerError, schema.ErrorResponse{
			StatusCode: http.StatusInternalServerError,
			Status:     "error",
			Message:    "failed to clear logs: " + err.Error(),
		})
	}

	return ctx.JSON(http.StatusOK, schema.OkBasicResponse)
}
