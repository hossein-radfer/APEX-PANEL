package http

import (
	"errors"
	"io"
	"net/http"
	"strconv"

	"github.com/maahdima/mwp/api/http/schema"
	"github.com/maahdima/mwp/api/service"

	"github.com/labstack/echo/v4"
	"go.uber.org/zap"
)

// HelpCenterController relays this install's Help Center requests to
// license-panel via HelpCenterService, which resolves the stored license
// key server-side -- the browser never sees or sends it. Admin-only,
// same as LicenseController: resellers have no license-server relationship
// of their own.
type HelpCenterController struct {
	helpCenterService *service.HelpCenterService
	logger            *zap.Logger
}

func NewHelpCenterController(helpCenterService *service.HelpCenterService) *HelpCenterController {
	return &HelpCenterController{helpCenterService: helpCenterService, logger: zap.L().Named("HelpCenterController")}
}

func (c *HelpCenterController) requireAdmin(ctx echo.Context) bool {
	role, _ := getRoleAndResellerFromContext(ctx)
	if role != "admin" {
		_ = ctx.JSON(http.StatusForbidden, schema.ErrorResponse{StatusCode: http.StatusForbidden, Status: "error", Message: "forbidden"})
		return false
	}
	return true
}

// writeRawJSON passes license-panel's already-valid JSON response straight
// through as this endpoint's own body -- avoiding a redundant unmarshal-
// then-remarshal round trip for data this server has no need to inspect
// or transform.
func (c *HelpCenterController) writeRawJSON(ctx echo.Context, status int, raw []byte) error {
	return ctx.Blob(status, echo.MIMEApplicationJSON, raw)
}

func (c *HelpCenterController) handleError(ctx echo.Context, err error) error {
	if errors.Is(err, service.ErrNotLicensed) {
		return ctx.JSON(http.StatusPreconditionFailed, schema.ErrorResponse{
			StatusCode: http.StatusPreconditionFailed,
			Status:     "error",
			Message:    "activate a license before using the Help Center",
		})
	}
	c.logger.Error("help center request failed", zap.Error(err))
	return ctx.JSON(http.StatusInternalServerError, schema.ErrorResponse{
		StatusCode: http.StatusInternalServerError,
		Status:     "error",
		Message:    "failed to reach the support server: " + err.Error(),
	})
}

func (c *HelpCenterController) ListTickets(ctx echo.Context) error {
	if !c.requireAdmin(ctx) {
		return nil
	}
	raw, err := c.helpCenterService.ListTickets()
	if err != nil {
		return c.handleError(ctx, err)
	}
	return c.writeRawJSON(ctx, http.StatusOK, raw)
}

type createTicketRequest struct {
	Subject string `json:"subject"`
	Message string `json:"message" validate:"required"`
}

func (c *HelpCenterController) CreateTicket(ctx echo.Context) error {
	if !c.requireAdmin(ctx) {
		return nil
	}
	var req createTicketRequest
	if err := ctx.Bind(&req); err != nil {
		return ctx.JSON(http.StatusBadRequest, schema.BadParamsErrorResponse)
	}
	if err := ctx.Validate(&req); err != nil {
		return ctx.JSON(http.StatusBadRequest, schema.BadParamsErrorResponse)
	}

	raw, err := c.helpCenterService.CreateTicket(req.Subject, req.Message)
	if err != nil {
		return c.handleError(ctx, err)
	}
	return c.writeRawJSON(ctx, http.StatusCreated, raw)
}

func (c *HelpCenterController) parseTicketID(ctx echo.Context) (uint, bool) {
	id, err := strconv.ParseUint(ctx.Param("id"), 10, 64)
	if err != nil {
		_ = ctx.JSON(http.StatusBadRequest, schema.BadParamsErrorResponse)
		return 0, false
	}
	return uint(id), true
}

func (c *HelpCenterController) GetTicketMessages(ctx echo.Context) error {
	if !c.requireAdmin(ctx) {
		return nil
	}
	id, ok := c.parseTicketID(ctx)
	if !ok {
		return nil
	}
	raw, err := c.helpCenterService.GetTicketMessages(id)
	if err != nil {
		return c.handleError(ctx, err)
	}
	return c.writeRawJSON(ctx, http.StatusOK, raw)
}

// PostMessage relays a multipart form (body + optional file/kind fields)
// straight through to license-panel -- same shape as license-panel's own
// PostMessage handler expects, see HelpCenterService.PostTicketMessage.
func (c *HelpCenterController) PostMessage(ctx echo.Context) error {
	if !c.requireAdmin(ctx) {
		return nil
	}
	id, ok := c.parseTicketID(ctx)
	if !ok {
		return nil
	}

	body := ctx.FormValue("body")
	kind := ctx.FormValue("kind")

	var fileName string
	var fileReader io.Reader
	fileHeader, err := ctx.FormFile("file")
	if err == nil {
		const maxAttachmentSize = 25 << 20
		if fileHeader.Size > maxAttachmentSize {
			return ctx.JSON(http.StatusBadRequest, schema.ErrorResponse{
				StatusCode: http.StatusBadRequest,
				Status:     "error",
				Message:    "file exceeds maximum allowed size (25 MiB)",
			})
		}
		src, openErr := fileHeader.Open()
		if openErr != nil {
			c.logger.Error("failed to open uploaded help center attachment", zap.Error(openErr))
			return ctx.JSON(http.StatusInternalServerError, schema.ErrorResponse{StatusCode: http.StatusInternalServerError, Status: "error", Message: "internal error"})
		}
		defer src.Close()
		fileName = fileHeader.Filename
		fileReader = src
	}

	if body == "" && fileReader == nil {
		return ctx.JSON(http.StatusBadRequest, schema.ErrorResponse{
			StatusCode: http.StatusBadRequest,
			Status:     "error",
			Message:    "message body or file is required",
		})
	}

	raw, err := c.helpCenterService.PostTicketMessage(id, body, kind, fileName, fileReader)
	if err != nil {
		return c.handleError(ctx, err)
	}
	return c.writeRawJSON(ctx, http.StatusCreated, raw)
}

func (c *HelpCenterController) SetTyping(ctx echo.Context) error {
	if !c.requireAdmin(ctx) {
		return nil
	}
	id, ok := c.parseTicketID(ctx)
	if !ok {
		return nil
	}
	if err := c.helpCenterService.SetTyping(id); err != nil {
		return c.handleError(ctx, err)
	}
	return ctx.JSON(http.StatusOK, map[string]string{"status": "ok"})
}

func (c *HelpCenterController) GetTypingStatus(ctx echo.Context) error {
	if !c.requireAdmin(ctx) {
		return nil
	}
	id, ok := c.parseTicketID(ctx)
	if !ok {
		return nil
	}
	raw, err := c.helpCenterService.GetTypingStatus(id)
	if err != nil {
		return c.handleError(ctx, err)
	}
	return c.writeRawJSON(ctx, http.StatusOK, raw)
}

func (c *HelpCenterController) DownloadAttachment(ctx echo.Context) error {
	if !c.requireAdmin(ctx) {
		return nil
	}
	ticketID, ok := c.parseTicketID(ctx)
	if !ok {
		return nil
	}
	messageID, err := strconv.ParseUint(ctx.Param("messageId"), 10, 64)
	if err != nil {
		return ctx.JSON(http.StatusBadRequest, schema.BadParamsErrorResponse)
	}

	stream, err := c.helpCenterService.DownloadAttachment(ticketID, uint(messageID))
	if err != nil {
		return c.handleError(ctx, err)
	}
	defer stream.Body.Close()

	if stream.ContentType != "" {
		ctx.Response().Header().Set(echo.HeaderContentType, stream.ContentType)
	}
	if stream.ContentDisposition != "" {
		ctx.Response().Header().Set("Content-Disposition", stream.ContentDisposition)
	}
	ctx.Response().WriteHeader(http.StatusOK)
	_, copyErr := io.Copy(ctx.Response(), stream.Body)
	return copyErr
}
