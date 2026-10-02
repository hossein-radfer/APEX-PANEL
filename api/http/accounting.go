package http

import (
	"io"
	"net/http"
	"path/filepath"
	"strconv"
	"time"

	"github.com/labstack/echo/v4"
	"go.uber.org/zap"

	"github.com/maahdima/mwp/api/dataservice/model"
	"github.com/maahdima/mwp/api/http/schema"
	"github.com/maahdima/mwp/api/service"
)

// AccountingController is admin-only across every single method -- see
// model/accounting.go's own doc comment for why this whole subsystem is
// the admin's own private bookkeeping, never exposed to resellers.
type AccountingController struct {
	accountingService *service.AccountingService
	logger            *zap.Logger
}

func NewAccountingController(accountingService *service.AccountingService) *AccountingController {
	return &AccountingController{
		accountingService: accountingService,
		logger:            zap.L().Named("AccountingController"),
	}
}

const accountingDateLayout = "2006-01-02"

func parseOptionalDate(s *string) (*time.Time, error) {
	if s == nil || *s == "" {
		return nil, nil
	}
	t, err := time.Parse(accountingDateLayout, *s)
	if err != nil {
		return nil, err
	}
	return &t, nil
}

func formatOptionalDate(t *time.Time) *string {
	if t == nil {
		return nil
	}
	s := t.Format(accountingDateLayout)
	return &s
}

// --- Partners ---

func (c *AccountingController) ListPartners(ctx echo.Context) error {
	if forbidden, err := forbidUnlessAdmin(ctx); forbidden {
		return err
	}

	partners, err := c.accountingService.ListPartners()
	if err != nil {
		return ctx.JSON(http.StatusInternalServerError, schema.InternalServerErrorResponse)
	}

	resp := make([]schema.AccountingPartnerResponse, 0, len(partners))
	for _, p := range partners {
		resp = append(resp, schema.AccountingPartnerResponse{Id: p.ID, Name: p.Name, Comment: p.Comment})
	}
	return ctx.JSON(http.StatusOK, schema.BasicResponseData[[]schema.AccountingPartnerResponse]{
		BasicResponse: schema.OkBasicResponse,
		Data:          resp,
	})
}

func (c *AccountingController) CreatePartner(ctx echo.Context) error {
	if forbidden, err := forbidUnlessAdmin(ctx); forbidden {
		return err
	}

	var req schema.CreatePartnerRequest
	if err := ctx.Bind(&req); err != nil {
		return ctx.JSON(http.StatusBadRequest, schema.BadParamsErrorResponse)
	}
	if err := ctx.Validate(&req); err != nil {
		return ctx.JSON(http.StatusBadRequest, schema.BadParamsErrorResponse)
	}

	partner, err := c.accountingService.CreatePartner(req.Name, req.Comment)
	if err != nil {
		c.logger.Error("failed to create partner", zap.Error(err))
		return ctx.JSON(http.StatusInternalServerError, schema.InternalServerErrorResponse)
	}

	return ctx.JSON(http.StatusCreated, schema.BasicResponseData[schema.AccountingPartnerResponse]{
		BasicResponse: schema.OkBasicResponse,
		Data:          schema.AccountingPartnerResponse{Id: partner.ID, Name: partner.Name, Comment: partner.Comment},
	})
}

func (c *AccountingController) UpdatePartner(ctx echo.Context) error {
	if forbidden, err := forbidUnlessAdmin(ctx); forbidden {
		return err
	}

	id, err := strconv.Atoi(ctx.Param("id"))
	if err != nil {
		return ctx.JSON(http.StatusBadRequest, schema.BadParamsErrorResponse)
	}

	var req schema.CreatePartnerRequest
	if err := ctx.Bind(&req); err != nil {
		return ctx.JSON(http.StatusBadRequest, schema.BadParamsErrorResponse)
	}
	if err := ctx.Validate(&req); err != nil {
		return ctx.JSON(http.StatusBadRequest, schema.BadParamsErrorResponse)
	}

	partner, err := c.accountingService.UpdatePartner(uint(id), req.Name, req.Comment)
	if err != nil {
		return ctx.JSON(http.StatusInternalServerError, schema.ErrorResponse{StatusCode: http.StatusInternalServerError, Status: "error", Message: err.Error()})
	}

	return ctx.JSON(http.StatusOK, schema.BasicResponseData[schema.AccountingPartnerResponse]{
		BasicResponse: schema.OkBasicResponse,
		Data:          schema.AccountingPartnerResponse{Id: partner.ID, Name: partner.Name, Comment: partner.Comment},
	})
}

func (c *AccountingController) DeletePartner(ctx echo.Context) error {
	if forbidden, err := forbidUnlessAdmin(ctx); forbidden {
		return err
	}

	id, err := strconv.Atoi(ctx.Param("id"))
	if err != nil {
		return ctx.JSON(http.StatusBadRequest, schema.BadParamsErrorResponse)
	}

	if err := c.accountingService.DeletePartner(uint(id)); err != nil {
		return ctx.JSON(http.StatusBadRequest, schema.ErrorResponse{StatusCode: http.StatusBadRequest, Status: "error", Message: err.Error()})
	}

	return ctx.NoContent(http.StatusNoContent)
}

// --- Costs ---

func (c *AccountingController) ListCosts(ctx echo.Context) error {
	if forbidden, err := forbidUnlessAdmin(ctx); forbidden {
		return err
	}

	costs, err := c.accountingService.ListCosts()
	if err != nil {
		return ctx.JSON(http.StatusInternalServerError, schema.InternalServerErrorResponse)
	}
	partnerNames, err := c.accountingService.PartnerNamesByID()
	if err != nil {
		return ctx.JSON(http.StatusInternalServerError, schema.InternalServerErrorResponse)
	}
	serverNames, err := c.accountingService.ServerNamesByID()
	if err != nil {
		return ctx.JSON(http.StatusInternalServerError, schema.InternalServerErrorResponse)
	}

	resp := make([]schema.AccountingCostResponse, 0, len(costs))
	for _, cst := range costs {
		row := schema.AccountingCostResponse{
			Id:                     cst.ID,
			PartnerID:              cst.PartnerID,
			PartnerName:            partnerNames[cst.PartnerID],
			ServerID:               cst.ServerID,
			Protocol:               cst.Protocol,
			LocationKey:            cst.LocationKey,
			AmountToman:            cst.AmountToman,
			Description:            cst.Description,
			PaidAt:                 formatOptionalDate(cst.PaidAt),
			DueAt:                  formatOptionalDate(cst.DueAt),
			RecurrenceIntervalDays: cst.RecurrenceIntervalDays,
			NextDueAt:              formatOptionalDate(cst.NextDueAt),
		}
		if cst.ServerID != nil {
			if name, ok := serverNames[*cst.ServerID]; ok {
				row.ServerName = &name
			}
		}
		if cst.Protocol != nil && cst.LocationKey != nil {
			row.LocationLabel = c.accountingService.LocationLabelFor(*cst.Protocol, *cst.LocationKey)
		}
		resp = append(resp, row)
	}

	return ctx.JSON(http.StatusOK, schema.BasicResponseData[[]schema.AccountingCostResponse]{
		BasicResponse: schema.OkBasicResponse,
		Data:          resp,
	})
}

// toCostResponse resolves PartnerName/ServerName for a single cost -- used
// by CreateCost/UpdateCost's response (ListCosts uses the bulk
// PartnerNamesByID/ServerNamesByID maps instead, to avoid N+1 queries
// there).
func (c *AccountingController) toCostResponse(cost *model.AccountingCost) schema.AccountingCostResponse {
	partnerName, _ := c.accountingService.PartnerName(cost.PartnerID)
	resp := schema.AccountingCostResponse{
		Id: cost.ID, PartnerID: cost.PartnerID, PartnerName: partnerName,
		ServerID: cost.ServerID, Protocol: cost.Protocol, LocationKey: cost.LocationKey,
		AmountToman: cost.AmountToman, Description: cost.Description,
		PaidAt: formatOptionalDate(cost.PaidAt), DueAt: formatOptionalDate(cost.DueAt),
		RecurrenceIntervalDays: cost.RecurrenceIntervalDays, NextDueAt: formatOptionalDate(cost.NextDueAt),
	}
	if cost.ServerID != nil {
		if name, err := c.accountingService.ServerName(*cost.ServerID); err == nil {
			resp.ServerName = &name
		}
	}
	if cost.Protocol != nil && cost.LocationKey != nil {
		resp.LocationLabel = c.accountingService.LocationLabelFor(*cost.Protocol, *cost.LocationKey)
	}
	return resp
}

func bindCostRequest(ctx echo.Context) (*service.CreateCostInput, error) {
	var req schema.CreateCostRequest
	if err := ctx.Bind(&req); err != nil {
		return nil, err
	}
	if err := ctx.Validate(&req); err != nil {
		return nil, err
	}

	paidAt, err := parseOptionalDate(req.PaidAt)
	if err != nil {
		return nil, err
	}
	dueAt, err := parseOptionalDate(req.DueAt)
	if err != nil {
		return nil, err
	}

	return &service.CreateCostInput{
		PartnerID:              req.PartnerID,
		ServerID:               req.ServerID,
		Protocol:               req.Protocol,
		LocationKey:            req.LocationKey,
		AmountToman:            req.AmountToman,
		Description:            req.Description,
		PaidAt:                 paidAt,
		DueAt:                  dueAt,
		RecurrenceIntervalDays: req.RecurrenceIntervalDays,
	}, nil
}

func (c *AccountingController) CreateCost(ctx echo.Context) error {
	if forbidden, err := forbidUnlessAdmin(ctx); forbidden {
		return err
	}

	input, err := bindCostRequest(ctx)
	if err != nil {
		return ctx.JSON(http.StatusBadRequest, schema.BadParamsErrorResponse)
	}

	cost, err := c.accountingService.CreateCost(*input)
	if err != nil {
		return ctx.JSON(http.StatusInternalServerError, schema.ErrorResponse{StatusCode: http.StatusInternalServerError, Status: "error", Message: err.Error()})
	}

	return ctx.JSON(http.StatusCreated, schema.BasicResponseData[schema.AccountingCostResponse]{
		BasicResponse: schema.OkBasicResponse,
		Data:          c.toCostResponse(cost),
	})
}

func (c *AccountingController) UpdateCost(ctx echo.Context) error {
	if forbidden, err := forbidUnlessAdmin(ctx); forbidden {
		return err
	}

	id, err := strconv.Atoi(ctx.Param("id"))
	if err != nil {
		return ctx.JSON(http.StatusBadRequest, schema.BadParamsErrorResponse)
	}

	input, err := bindCostRequest(ctx)
	if err != nil {
		return ctx.JSON(http.StatusBadRequest, schema.BadParamsErrorResponse)
	}

	cost, err := c.accountingService.UpdateCost(uint(id), *input)
	if err != nil {
		return ctx.JSON(http.StatusInternalServerError, schema.ErrorResponse{StatusCode: http.StatusInternalServerError, Status: "error", Message: err.Error()})
	}

	return ctx.JSON(http.StatusOK, schema.BasicResponseData[schema.AccountingCostResponse]{
		BasicResponse: schema.OkBasicResponse,
		Data:          c.toCostResponse(cost),
	})
}

func (c *AccountingController) DeleteCost(ctx echo.Context) error {
	if forbidden, err := forbidUnlessAdmin(ctx); forbidden {
		return err
	}

	id, err := strconv.Atoi(ctx.Param("id"))
	if err != nil {
		return ctx.JSON(http.StatusBadRequest, schema.BadParamsErrorResponse)
	}

	if err := c.accountingService.DeleteCost(uint(id)); err != nil {
		return ctx.JSON(http.StatusInternalServerError, schema.InternalServerErrorResponse)
	}
	return ctx.NoContent(http.StatusNoContent)
}

func (c *AccountingController) AdvanceRecurringCost(ctx echo.Context) error {
	if forbidden, err := forbidUnlessAdmin(ctx); forbidden {
		return err
	}

	id, err := strconv.Atoi(ctx.Param("id"))
	if err != nil {
		return ctx.JSON(http.StatusBadRequest, schema.BadParamsErrorResponse)
	}

	if err := c.accountingService.AdvanceRecurringCost(uint(id)); err != nil {
		return ctx.JSON(http.StatusBadRequest, schema.ErrorResponse{StatusCode: http.StatusBadRequest, Status: "error", Message: err.Error()})
	}
	return ctx.NoContent(http.StatusNoContent)
}

// --- Payments ---

func (c *AccountingController) toPaymentResponse(p *model.AccountingCustomerPayment) schema.AccountingPaymentResponse {
	resp := schema.AccountingPaymentResponse{
		Id: p.ID, CustomerLabel: p.CustomerLabel, AmountToman: p.AmountToman,
		CostToman: p.CostToman, ProfitToman: p.AmountToman - p.CostToman,
		Protocol: p.Protocol, ResourceID: p.ResourceID, LocationKey: p.LocationKey,
		Note: p.Note, HasReceipt: p.ReceiptFilePath != nil, PaidAt: p.PaidAt.Format(accountingDateLayout),
	}
	if p.Protocol != nil && p.LocationKey != nil {
		resp.LocationLabel = c.accountingService.LocationLabelFor(*p.Protocol, *p.LocationKey)
	}
	return resp
}

func (c *AccountingController) ListPayments(ctx echo.Context) error {
	if forbidden, err := forbidUnlessAdmin(ctx); forbidden {
		return err
	}

	payments, err := c.accountingService.ListPayments()
	if err != nil {
		return ctx.JSON(http.StatusInternalServerError, schema.InternalServerErrorResponse)
	}

	resp := make([]schema.AccountingPaymentResponse, 0, len(payments))
	for _, p := range payments {
		resp = append(resp, c.toPaymentResponse(&p))
	}

	return ctx.JSON(http.StatusOK, schema.BasicResponseData[[]schema.AccountingPaymentResponse]{
		BasicResponse: schema.OkBasicResponse,
		Data:          resp,
	})
}

func bindPaymentRequest(ctx echo.Context) (*service.CreatePaymentInput, error) {
	var req schema.CreatePaymentRequest
	if err := ctx.Bind(&req); err != nil {
		return nil, err
	}
	if err := ctx.Validate(&req); err != nil {
		return nil, err
	}
	paidAt, err := time.Parse(accountingDateLayout, req.PaidAt)
	if err != nil {
		return nil, err
	}
	return &service.CreatePaymentInput{
		CustomerLabel: req.CustomerLabel,
		AmountToman:   req.AmountToman,
		CostToman:     req.CostToman,
		Protocol:      req.Protocol,
		ResourceID:    req.ResourceID,
		Note:          req.Note,
		PaidAt:        paidAt,
	}, nil
}

func (c *AccountingController) CreatePayment(ctx echo.Context) error {
	if forbidden, err := forbidUnlessAdmin(ctx); forbidden {
		return err
	}

	input, err := bindPaymentRequest(ctx)
	if err != nil {
		return ctx.JSON(http.StatusBadRequest, schema.BadParamsErrorResponse)
	}

	payment, err := c.accountingService.CreatePayment(*input)
	if err != nil {
		return ctx.JSON(http.StatusInternalServerError, schema.InternalServerErrorResponse)
	}

	return ctx.JSON(http.StatusCreated, schema.BasicResponseData[schema.AccountingPaymentResponse]{
		BasicResponse: schema.OkBasicResponse,
		Data:          c.toPaymentResponse(payment),
	})
}

func (c *AccountingController) UpdatePayment(ctx echo.Context) error {
	if forbidden, err := forbidUnlessAdmin(ctx); forbidden {
		return err
	}

	id, err := strconv.Atoi(ctx.Param("id"))
	if err != nil {
		return ctx.JSON(http.StatusBadRequest, schema.BadParamsErrorResponse)
	}

	input, err := bindPaymentRequest(ctx)
	if err != nil {
		return ctx.JSON(http.StatusBadRequest, schema.BadParamsErrorResponse)
	}

	payment, err := c.accountingService.UpdatePayment(uint(id), *input)
	if err != nil {
		return ctx.JSON(http.StatusInternalServerError, schema.ErrorResponse{StatusCode: http.StatusInternalServerError, Status: "error", Message: err.Error()})
	}

	return ctx.JSON(http.StatusOK, schema.BasicResponseData[schema.AccountingPaymentResponse]{
		BasicResponse: schema.OkBasicResponse,
		Data:          c.toPaymentResponse(payment),
	})
}

func (c *AccountingController) DeletePayment(ctx echo.Context) error {
	if forbidden, err := forbidUnlessAdmin(ctx); forbidden {
		return err
	}

	id, err := strconv.Atoi(ctx.Param("id"))
	if err != nil {
		return ctx.JSON(http.StatusBadRequest, schema.BadParamsErrorResponse)
	}

	if err := c.accountingService.DeletePayment(uint(id)); err != nil {
		return ctx.JSON(http.StatusInternalServerError, schema.ErrorResponse{StatusCode: http.StatusInternalServerError, Status: "error", Message: err.Error()})
	}
	return ctx.NoContent(http.StatusNoContent)
}

// UploadReceipt accepts a single image/PDF file for one payment -- mirrors
// UserManagerAccountController.UploadAccountConfig's multipart-form
// pattern, capped at 10 MiB (a generous bound for a photographed receipt,
// still small enough to guard against a runaway upload).
func (c *AccountingController) UploadReceipt(ctx echo.Context) error {
	if forbidden, err := forbidUnlessAdmin(ctx); forbidden {
		return err
	}

	id, err := strconv.Atoi(ctx.Param("id"))
	if err != nil {
		return ctx.JSON(http.StatusBadRequest, schema.BadParamsErrorResponse)
	}

	fileHeader, err := ctx.FormFile("receipt")
	if err != nil {
		return ctx.JSON(http.StatusBadRequest, schema.ErrorResponse{StatusCode: http.StatusBadRequest, Status: "error", Message: "missing 'receipt' file in upload"})
	}

	const maxReceiptUploadSize = 10 << 20 // 10 MiB
	if fileHeader.Size > maxReceiptUploadSize {
		return ctx.JSON(http.StatusBadRequest, schema.ErrorResponse{StatusCode: http.StatusBadRequest, Status: "error", Message: "uploaded file exceeds the maximum allowed receipt size"})
	}

	src, err := fileHeader.Open()
	if err != nil {
		return ctx.JSON(http.StatusInternalServerError, schema.InternalServerErrorResponse)
	}
	defer src.Close()

	data, err := io.ReadAll(src)
	if err != nil {
		return ctx.JSON(http.StatusInternalServerError, schema.InternalServerErrorResponse)
	}

	ext := filepath.Ext(fileHeader.Filename)
	if ext == "" {
		ext = ".bin"
	}

	if _, err := c.accountingService.SaveReceiptFile(uint(id), ext, data); err != nil {
		return ctx.JSON(http.StatusInternalServerError, schema.ErrorResponse{StatusCode: http.StatusInternalServerError, Status: "error", Message: err.Error()})
	}

	return ctx.NoContent(http.StatusNoContent)
}

func (c *AccountingController) DownloadReceipt(ctx echo.Context) error {
	if forbidden, err := forbidUnlessAdmin(ctx); forbidden {
		return err
	}

	id, err := strconv.Atoi(ctx.Param("id"))
	if err != nil {
		return ctx.JSON(http.StatusBadRequest, schema.BadParamsErrorResponse)
	}

	path, err := c.accountingService.GetReceiptFilePath(uint(id))
	if err != nil {
		return ctx.JSON(http.StatusNotFound, schema.ErrorResponse{StatusCode: http.StatusNotFound, Status: "error", Message: err.Error()})
	}

	return ctx.File(path)
}

// --- Summary ---

// parseSummaryRange reads the ?since=&until= query params shared by every
// reporting endpoint below (GetSummary/GetUserProfitability/
// GetLocationProfitability), defaulting to the trailing 30 days when
// either is missing or unparseable.
func parseSummaryRange(ctx echo.Context) (since, until time.Time) {
	sinceStr := ctx.QueryParam("since")
	untilStr := ctx.QueryParam("until")

	until = time.Now()
	since = until.AddDate(0, -1, 0)
	if sinceStr != "" {
		if t, err := time.Parse(accountingDateLayout, sinceStr); err == nil {
			since = t
		}
	}
	if untilStr != "" {
		if t, err := time.Parse(accountingDateLayout, untilStr); err == nil {
			until = t
		}
	}
	return since, until
}

func (c *AccountingController) GetSummary(ctx echo.Context) error {
	if forbidden, err := forbidUnlessAdmin(ctx); forbidden {
		return err
	}

	since, until := parseSummaryRange(ctx)

	summary, err := c.accountingService.GetSummary(since, until)
	if err != nil {
		return ctx.JSON(http.StatusInternalServerError, schema.InternalServerErrorResponse)
	}

	series := make([]schema.AccountingPeriodSummary, 0, len(summary.Series))
	for _, s := range summary.Series {
		series = append(series, schema.AccountingPeriodSummary{Period: s.Period, IncomeToman: s.IncomeToman, CostToman: s.CostToman})
	}

	return ctx.JSON(http.StatusOK, schema.BasicResponseData[schema.AccountingSummaryResponse]{
		BasicResponse: schema.OkBasicResponse,
		Data: schema.AccountingSummaryResponse{
			TotalIncomeToman: summary.TotalIncomeToman,
			TotalCostToman:   summary.TotalCostToman,
			ProfitToman:      summary.ProfitToman,
			Series:           series,
		},
	})
}

// ListSellableResources powers the payment-creation form's "what was sold"
// picker -- ?protocol=wireguard|user_manager|v2ray selects which of Peer/
// UserManagerAccount/V2RayPackage to list.
func (c *AccountingController) ListSellableResources(ctx echo.Context) error {
	if forbidden, err := forbidUnlessAdmin(ctx); forbidden {
		return err
	}

	protocol := ctx.QueryParam("protocol")
	if protocol == "" {
		return ctx.JSON(http.StatusBadRequest, schema.BadParamsErrorResponse)
	}

	resources, err := c.accountingService.ListSellableResources(protocol)
	if err != nil {
		return ctx.JSON(http.StatusBadRequest, schema.ErrorResponse{StatusCode: http.StatusBadRequest, Status: "error", Message: err.Error()})
	}

	resp := make([]schema.SellableResourceResponse, 0, len(resources))
	for _, r := range resources {
		resp = append(resp, schema.SellableResourceResponse{Id: r.Id, Name: r.Name})
	}

	return ctx.JSON(http.StatusOK, schema.BasicResponseData[[]schema.SellableResourceResponse]{
		BasicResponse: schema.OkBasicResponse,
		Data:          resp,
	})
}

// GetUserProfitability lists every sale in [since, until] with its own
// sale-price/cost/profit -- the admin's own explicit per-user profit
// requirement (see service.UserProfitability's own doc comment).
func (c *AccountingController) GetUserProfitability(ctx echo.Context) error {
	if forbidden, err := forbidUnlessAdmin(ctx); forbidden {
		return err
	}

	since, until := parseSummaryRange(ctx)

	rows, err := c.accountingService.GetUserProfitability(since, until)
	if err != nil {
		return ctx.JSON(http.StatusInternalServerError, schema.InternalServerErrorResponse)
	}

	resp := make([]schema.UserProfitabilityResponse, 0, len(rows))
	for _, r := range rows {
		resp = append(resp, schema.UserProfitabilityResponse{
			PaymentID: r.PaymentID, CustomerLabel: r.CustomerLabel, Protocol: r.Protocol,
			ResourceID: r.ResourceID, LocationKey: r.LocationKey, LocationLabel: r.LocationLabel,
			AmountToman: r.AmountToman, CostToman: r.CostToman, ProfitToman: r.ProfitToman, PaidAt: r.PaidAt,
		})
	}

	return ctx.JSON(http.StatusOK, schema.BasicResponseData[[]schema.UserProfitabilityResponse]{
		BasicResponse: schema.OkBasicResponse,
		Data:          resp,
	})
}

// GetLocationProfitability rolls sales/costs up per (protocol, location)
// within [since, until] -- the admin's own explicit per-location revenue
// requirement (see service.LocationProfitability's own doc comment).
func (c *AccountingController) GetLocationProfitability(ctx echo.Context) error {
	if forbidden, err := forbidUnlessAdmin(ctx); forbidden {
		return err
	}

	since, until := parseSummaryRange(ctx)

	rows, err := c.accountingService.GetLocationProfitability(since, until)
	if err != nil {
		return ctx.JSON(http.StatusInternalServerError, schema.InternalServerErrorResponse)
	}

	resp := make([]schema.LocationProfitabilityResponse, 0, len(rows))
	for _, r := range rows {
		resp = append(resp, schema.LocationProfitabilityResponse{
			Protocol: r.Protocol, LocationKey: r.LocationKey, LocationLabel: r.LocationLabel,
			IncomeToman: r.IncomeToman, CostToman: r.CostToman, ProfitToman: r.ProfitToman,
		})
	}

	return ctx.JSON(http.StatusOK, schema.BasicResponseData[[]schema.LocationProfitabilityResponse]{
		BasicResponse: schema.OkBasicResponse,
		Data:          resp,
	})
}
