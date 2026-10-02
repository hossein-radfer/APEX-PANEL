package http

import (
	"errors"
	"net/http"
	"strconv"
	"time"

	"github.com/labstack/echo/v4"
	"go.uber.org/zap"
	"gorm.io/gorm"

	"github.com/maahdima/mwp/api/dataservice/model"
	"github.com/maahdima/mwp/api/http/schema"
	"github.com/maahdima/mwp/api/service"
)

type InvoiceController struct {
	invoiceService *service.Invoice
	logger         *zap.Logger
}

func NewInvoiceController(invoiceService *service.Invoice) *InvoiceController {
	return &InvoiceController{
		invoiceService: invoiceService,
		logger:         zap.L().Named("InvoiceController"),
	}
}

func toUnixPtr(t *time.Time) *int64 {
	if t == nil {
		return nil
	}
	v := t.Unix()
	return &v
}

func parseUnixPtr(v *int64) *time.Time {
	if v == nil {
		return nil
	}
	t := time.Unix(*v, 0)
	return &t
}

func toInvoiceResponse(invoice *model.Invoice) schema.InvoiceResponse {
	return schema.InvoiceResponse{
		ID:            invoice.ID,
		ResellerID:    invoice.ResellerID,
		InvoiceNumber: invoice.InvoiceNumber,
		Status:        invoice.Status,
		Amount:        invoice.Amount,
		Description:   invoice.Description,
		PeriodStart:   invoice.PeriodStart.Unix(),
		PeriodEnd:     invoice.PeriodEnd.Unix(),
		IssuedAt:      toUnixPtr(invoice.IssuedAt),
		DueDate:       toUnixPtr(invoice.DueDate),
		PaidAt:        toUnixPtr(invoice.PaidAt),
		Notes:         invoice.Notes,
		CreatedAt:     invoice.CreatedAt.Unix(),
	}
}

func (ic *InvoiceController) CreateInvoice(ctx echo.Context) error {
	role, _ := getRoleAndResellerFromContext(ctx)
	if role == "reseller" {
		return ctx.JSON(http.StatusForbidden, schema.ErrorResponse{
			StatusCode: http.StatusForbidden,
			Status:     "error",
			Message:    "forbidden",
		})
	}

	resellerIDStr := ctx.Param("reseller_id")
	if resellerIDStr == "" {
		return ctx.JSON(http.StatusBadRequest, schema.BadParamsErrorResponse)
	}

	resellerID, err := strconv.ParseUint(resellerIDStr, 10, 32)
	if err != nil {
		return ctx.JSON(http.StatusBadRequest, schema.BadParamsErrorResponse)
	}

	var req schema.CreateInvoiceRequest
	if err := ctx.Bind(&req); err != nil {
		return ctx.JSON(http.StatusBadRequest, schema.BadParamsErrorResponse)
	}
	if err := ctx.Validate(&req); err != nil {
		return ctx.JSON(http.StatusBadRequest, schema.BadParamsErrorResponse)
	}

	invoice, err := ic.invoiceService.CreateInvoice(
		uint(resellerID),
		req.Amount,
		req.Description,
		parseUnixPtr(req.DueDate),
		parseUnixPtr(req.PeriodStart),
		parseUnixPtr(req.PeriodEnd),
		req.Notes,
		req.AutoIssue,
	)
	if err != nil {
		ic.logger.Error("failed to create invoice", zap.Error(err))
		return ctx.JSON(http.StatusInternalServerError, schema.InternalServerErrorResponse)
	}

	return ctx.JSON(http.StatusCreated, schema.BasicResponseData[schema.InvoiceResponse]{
		BasicResponse: schema.OkBasicResponse,
		Data:          toInvoiceResponse(invoice),
	})
}

func (ic *InvoiceController) ListInvoicesByReseller(ctx echo.Context) error {
	resellerIDStr := ctx.Param("reseller_id")
	if resellerIDStr == "" {
		return ctx.JSON(http.StatusBadRequest, schema.BadParamsErrorResponse)
	}

	resellerID, err := strconv.ParseUint(resellerIDStr, 10, 32)
	if err != nil {
		return ctx.JSON(http.StatusBadRequest, schema.BadParamsErrorResponse)
	}

	role, resellerClaim := getRoleAndResellerFromContext(ctx)
	if role == "reseller" && (resellerClaim == nil || *resellerClaim != uint(resellerID)) {
		return ctx.JSON(http.StatusForbidden, schema.ErrorResponse{
			StatusCode: http.StatusForbidden,
			Status:     "error",
			Message:    "forbidden",
		})
	}

	limit := 50
	if limitStr := ctx.QueryParam("limit"); limitStr != "" {
		if parsed, err := strconv.Atoi(limitStr); err == nil && parsed > 0 && parsed <= 500 {
			limit = parsed
		}
	}

	invoices, err := ic.invoiceService.ListInvoicesByReseller(uint(resellerID), limit)
	if err != nil {
		ic.logger.Error("failed to list invoices", zap.Error(err))
		return ctx.JSON(http.StatusInternalServerError, schema.InternalServerErrorResponse)
	}

	response := make([]schema.InvoiceResponse, len(invoices))
	for i := range invoices {
		response[i] = toInvoiceResponse(&invoices[i])
	}

	return ctx.JSON(http.StatusOK, schema.BasicResponseData[[]schema.InvoiceResponse]{
		BasicResponse: schema.OkBasicResponse,
		Data:          response,
	})
}

func (ic *InvoiceController) GetInvoice(ctx echo.Context) error {
	invoiceIDStr := ctx.Param("invoice_id")
	if invoiceIDStr == "" {
		return ctx.JSON(http.StatusBadRequest, schema.BadParamsErrorResponse)
	}

	invoiceID, err := strconv.ParseUint(invoiceIDStr, 10, 32)
	if err != nil {
		return ctx.JSON(http.StatusBadRequest, schema.BadParamsErrorResponse)
	}

	invoice, err := ic.invoiceService.GetInvoice(uint(invoiceID))
	if err != nil {
		if errors.Is(err, gorm.ErrRecordNotFound) {
			return ctx.JSON(http.StatusNotFound, schema.ErrorResponse{
				StatusCode: http.StatusNotFound,
				Status:     "error",
				Message:    "invoice not found",
			})
		}
		return ctx.JSON(http.StatusInternalServerError, schema.InternalServerErrorResponse)
	}

	role, resellerClaim := getRoleAndResellerFromContext(ctx)
	if role == "reseller" && (resellerClaim == nil || *resellerClaim != invoice.ResellerID) {
		return ctx.JSON(http.StatusForbidden, schema.ErrorResponse{
			StatusCode: http.StatusForbidden,
			Status:     "error",
			Message:    "forbidden",
		})
	}

	return ctx.JSON(http.StatusOK, schema.BasicResponseData[schema.InvoiceResponse]{
		BasicResponse: schema.OkBasicResponse,
		Data:          toInvoiceResponse(invoice),
	})
}

func (ic *InvoiceController) IssueInvoice(ctx echo.Context) error {
	role, _ := getRoleAndResellerFromContext(ctx)
	if role == "reseller" {
		return ctx.JSON(http.StatusForbidden, schema.ErrorResponse{StatusCode: http.StatusForbidden, Status: "error", Message: "forbidden"})
	}

	invoiceID, err := strconv.ParseUint(ctx.Param("invoice_id"), 10, 32)
	if err != nil {
		return ctx.JSON(http.StatusBadRequest, schema.BadParamsErrorResponse)
	}

	invoice, err := ic.invoiceService.IssueInvoice(uint(invoiceID))
	if err != nil {
		if errors.Is(err, gorm.ErrRecordNotFound) {
			return ctx.JSON(http.StatusNotFound, schema.ErrorResponse{StatusCode: http.StatusNotFound, Status: "error", Message: "invoice not found"})
		}
		return ctx.JSON(http.StatusBadRequest, schema.ErrorResponse{StatusCode: http.StatusBadRequest, Status: "error", Message: err.Error()})
	}

	return ctx.JSON(http.StatusOK, schema.BasicResponseData[schema.InvoiceResponse]{
		BasicResponse: schema.OkBasicResponse,
		Data:          toInvoiceResponse(invoice),
	})
}

func (ic *InvoiceController) PayInvoice(ctx echo.Context) error {
	invoiceID, err := strconv.ParseUint(ctx.Param("invoice_id"), 10, 32)
	if err != nil {
		return ctx.JSON(http.StatusBadRequest, schema.BadParamsErrorResponse)
	}

	invoice, err := ic.invoiceService.GetInvoice(uint(invoiceID))
	if err != nil {
		if errors.Is(err, gorm.ErrRecordNotFound) {
			return ctx.JSON(http.StatusNotFound, schema.ErrorResponse{StatusCode: http.StatusNotFound, Status: "error", Message: "invoice not found"})
		}
		return ctx.JSON(http.StatusInternalServerError, schema.InternalServerErrorResponse)
	}

	role, resellerClaim := getRoleAndResellerFromContext(ctx)
	if role == "reseller" && (resellerClaim == nil || *resellerClaim != invoice.ResellerID) {
		return ctx.JSON(http.StatusForbidden, schema.ErrorResponse{StatusCode: http.StatusForbidden, Status: "error", Message: "forbidden"})
	}

	paid, err := ic.invoiceService.PayInvoice(uint(invoiceID))
	if err != nil {
		return ctx.JSON(http.StatusBadRequest, schema.ErrorResponse{StatusCode: http.StatusBadRequest, Status: "error", Message: err.Error()})
	}

	return ctx.JSON(http.StatusOK, schema.BasicResponseData[schema.InvoiceResponse]{
		BasicResponse: schema.OkBasicResponse,
		Data:          toInvoiceResponse(paid),
	})
}

func (ic *InvoiceController) CancelInvoice(ctx echo.Context) error {
	role, _ := getRoleAndResellerFromContext(ctx)
	if role == "reseller" {
		return ctx.JSON(http.StatusForbidden, schema.ErrorResponse{StatusCode: http.StatusForbidden, Status: "error", Message: "forbidden"})
	}

	invoiceID, err := strconv.ParseUint(ctx.Param("invoice_id"), 10, 32)
	if err != nil {
		return ctx.JSON(http.StatusBadRequest, schema.BadParamsErrorResponse)
	}

	cancelled, err := ic.invoiceService.CancelInvoice(uint(invoiceID))
	if err != nil {
		if errors.Is(err, gorm.ErrRecordNotFound) {
			return ctx.JSON(http.StatusNotFound, schema.ErrorResponse{StatusCode: http.StatusNotFound, Status: "error", Message: "invoice not found"})
		}
		return ctx.JSON(http.StatusBadRequest, schema.ErrorResponse{StatusCode: http.StatusBadRequest, Status: "error", Message: err.Error()})
	}

	return ctx.JSON(http.StatusOK, schema.BasicResponseData[schema.InvoiceResponse]{
		BasicResponse: schema.OkBasicResponse,
		Data:          toInvoiceResponse(cancelled),
	})
}
