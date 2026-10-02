package http

import (
	"errors"
	"io"
	"net/http"

	"github.com/maahdima/mwp/api/http/schema"
	"github.com/maahdima/mwp/api/service"

	"github.com/labstack/echo/v4"
	"go.uber.org/zap"
)

type BackupController struct {
	backupService *service.BackupService
	botScheduler  *service.BotScheduler
	logger        *zap.Logger
}

func NewBackupController(backupService *service.BackupService, botScheduler *service.BotScheduler) *BackupController {
	return &BackupController{
		backupService: backupService,
		botScheduler:  botScheduler,
		logger:        zap.L().Named("BackupController"),
	}
}

// DownloadBackup streams a fresh, consistent point-in-time snapshot of the
// database to the caller. Admin only.
func (c *BackupController) DownloadBackup(ctx echo.Context) error {
	if forbidden, err := forbidUnlessAdmin(ctx); forbidden {
		return err
	}

	path, cleanup, err := c.backupService.CreateBackup()
	if err != nil {
		if errors.Is(err, service.ErrBackupUnsupportedDialect) {
			return ctx.JSON(http.StatusBadRequest, schema.ErrorResponse{
				StatusCode: http.StatusBadRequest,
				Status:     "error",
				Message:    err.Error(),
			})
		}
		c.logger.Error("failed to create backup", zap.Error(err))
		return ctx.JSON(http.StatusInternalServerError, schema.ErrorResponse{
			StatusCode: http.StatusInternalServerError,
			Status:     "error",
			Message:    "failed to create backup: " + err.Error(),
		})
	}
	defer cleanup()

	return ctx.Attachment(path, "mwp-backup.db")
}

// SendInstantBackup creates a fresh backup and pushes it to every configured
// admin Telegram recipient right now -- the "بکاپ فوری" button requested
// for the web panel, distinct from the scheduled daily job (never marks
// today as already-sent, so it can't suppress the regular automatic
// backup). Admin only.
func (c *BackupController) SendInstantBackup(ctx echo.Context) error {
	if forbidden, err := forbidUnlessAdmin(ctx); forbidden {
		return err
	}

	if err := c.botScheduler.SendInstantBackup(); err != nil {
		if errors.Is(err, service.ErrBackupUnsupportedDialect) {
			return ctx.JSON(http.StatusBadRequest, schema.ErrorResponse{
				StatusCode: http.StatusBadRequest,
				Status:     "error",
				Message:    err.Error(),
			})
		}
		c.logger.Error("failed to send instant backup", zap.Error(err))
		return ctx.JSON(http.StatusInternalServerError, schema.ErrorResponse{
			StatusCode: http.StatusInternalServerError,
			Status:     "error",
			Message:    "failed to send instant backup: " + err.Error(),
		})
	}

	return ctx.JSON(http.StatusOK, schema.OkBasicResponse)
}

// UploadRestoreBackup accepts an uploaded SQLite database file, validates
// it, and stages it for restore. It does NOT touch the live database --
// the running process keeps using its existing connection pool untouched.
// The staged file is applied automatically the next time the panel process
// starts (see dataservice.ApplyPendingRestore, which runs before any
// database connection is opened), so restoring is just: upload here, then
// restart the panel. Admin only.
func (c *BackupController) UploadRestoreBackup(ctx echo.Context) error {
	if forbidden, err := forbidUnlessAdmin(ctx); forbidden {
		return err
	}

	fileHeader, err := ctx.FormFile("backup")
	if err != nil {
		return ctx.JSON(http.StatusBadRequest, schema.ErrorResponse{
			StatusCode: http.StatusBadRequest,
			Status:     "error",
			Message:    "missing 'backup' file in upload",
		})
	}

	// A generous but bounded cap: reject anything absurdly large before
	// reading it fully into memory. 2GB is far beyond any realistic panel
	// database size and guards against a runaway/malicious upload.
	const maxBackupUploadSize = 2 << 30 // 2 GiB
	if fileHeader.Size > maxBackupUploadSize {
		return ctx.JSON(http.StatusBadRequest, schema.ErrorResponse{
			StatusCode: http.StatusBadRequest,
			Status:     "error",
			Message:    "uploaded file exceeds the maximum allowed backup size",
		})
	}

	src, err := fileHeader.Open()
	if err != nil {
		c.logger.Error("failed to open uploaded backup file", zap.Error(err))
		return ctx.JSON(http.StatusInternalServerError, schema.InternalServerErrorResponse)
	}
	defer src.Close()

	content, err := io.ReadAll(src)
	if err != nil {
		c.logger.Error("failed to read uploaded backup file", zap.Error(err))
		return ctx.JSON(http.StatusInternalServerError, schema.InternalServerErrorResponse)
	}

	stagedPath, err := c.backupService.StageRestore(content)
	if err != nil {
		if errors.Is(err, service.ErrBackupUnsupportedDialect) {
			return ctx.JSON(http.StatusBadRequest, schema.ErrorResponse{
				StatusCode: http.StatusBadRequest,
				Status:     "error",
				Message:    err.Error(),
			})
		}
		// Validation failures (bad header, too small) are the caller's
		// fault -- 400, not 500.
		return ctx.JSON(http.StatusBadRequest, schema.ErrorResponse{
			StatusCode: http.StatusBadRequest,
			Status:     "error",
			Message:    err.Error(),
		})
	}

	return ctx.JSON(http.StatusOK, schema.BasicResponseData[schema.RestoreStagedResponse]{
		BasicResponse: schema.OkBasicResponse,
		Data: schema.RestoreStagedResponse{
			StagedPath: stagedPath,
			Message:    "Backup staged successfully. It will automatically replace the live database the next time the panel restarts.",
		},
	})
}
