package http

import (
	"errors"
	"net/http"
	"strconv"

	"github.com/labstack/echo/v4"
	"go.uber.org/zap"
	"gorm.io/gorm"

	"github.com/maahdima/mwp/api/http/schema"
	"github.com/maahdima/mwp/api/service"
)

// DNSAccountController manages Smart DNS accounts -- mirrors
// V2RayPackageController's shape/access-rule conventions, reusing the same
// package-level peerScopeFromContext/adminResellerIDFromParam/
// getRoleAndResellerFromContext helpers.
type DNSAccountController struct {
	accountService *service.DNSAccountService
	logger         *zap.Logger
}

func NewDNSAccountController(accountService *service.DNSAccountService) *DNSAccountController {
	return &DNSAccountController{
		accountService: accountService,
		logger:         zap.L().Named("DNSAccountController"),
	}
}

// GetSelfSummary returns the caller reseller's own DNS account count/usage.
// Reseller only.
func (c *DNSAccountController) GetSelfSummary(ctx echo.Context) error {
	role, resellerID := getRoleAndResellerFromContext(ctx)
	if role != "reseller" || resellerID == nil {
		return ctx.JSON(http.StatusForbidden, schema.ErrorResponse{StatusCode: http.StatusForbidden, Status: "error", Message: "forbidden"})
	}

	summary, err := c.accountService.GetSelfSummary(*resellerID)
	if err != nil {
		c.logger.Error("failed to get DNS self summary", zap.Error(err))
		return ctx.JSON(http.StatusInternalServerError, schema.ErrorResponse{
			StatusCode: http.StatusInternalServerError,
			Status:     "error",
			Message:    "failed to retrieve summary: " + err.Error(),
		})
	}

	return ctx.JSON(http.StatusOK, schema.BasicResponseData[schema.DNSSelfSummaryResponse]{
		BasicResponse: schema.OkBasicResponse,
		Data:          *summary,
	})
}

// GetAdminSummary returns the panel-wide DNS rollup for the admin dashboard.
// Admin only.
func (c *DNSAccountController) GetAdminSummary(ctx echo.Context) error {
	if role, _ := getRoleAndResellerFromContext(ctx); role == "reseller" {
		return ctx.JSON(http.StatusForbidden, schema.ErrorResponse{StatusCode: http.StatusForbidden, Status: "error", Message: "forbidden"})
	}

	summary, err := c.accountService.GetAdminSummary()
	if err != nil {
		c.logger.Error("failed to get DNS admin summary", zap.Error(err))
		return ctx.JSON(http.StatusInternalServerError, schema.ErrorResponse{
			StatusCode: http.StatusInternalServerError,
			Status:     "error",
			Message:    "failed to retrieve summary: " + err.Error(),
		})
	}

	return ctx.JSON(http.StatusOK, schema.BasicResponseData[schema.DNSAdminSummaryResponse]{
		BasicResponse: schema.OkBasicResponse,
		Data:          *summary,
	})
}

func (c *DNSAccountController) ListAccounts(ctx echo.Context) error {
	resellerID, err := peerScopeFromContext(ctx)
	if err != nil {
		return err
	}

	accounts, err := c.accountService.ListAccounts(resellerID)
	if err != nil {
		c.logger.Error("failed to list DNS accounts", zap.Error(err))
		return ctx.JSON(http.StatusInternalServerError, schema.ErrorResponse{
			StatusCode: http.StatusInternalServerError,
			Status:     "error",
			Message:    "failed to retrieve DNS accounts: " + err.Error(),
		})
	}

	return ctx.JSON(http.StatusOK, schema.BasicResponseData[[]schema.DNSAccountResponse]{
		BasicResponse: schema.OkBasicResponse,
		Data:          accounts,
	})
}

// ListAccountsByReseller returns all DNS accounts owned by a specific
// reseller. Admin only.
func (c *DNSAccountController) ListAccountsByReseller(ctx echo.Context) error {
	if role, _ := getRoleAndResellerFromContext(ctx); role == "reseller" {
		return ctx.JSON(http.StatusForbidden, schema.ErrorResponse{StatusCode: http.StatusForbidden, Status: "error", Message: "forbidden"})
	}

	resellerID, err := strconv.ParseUint(ctx.Param("reseller_id"), 10, 32)
	if err != nil {
		return ctx.JSON(http.StatusBadRequest, schema.BadParamsErrorResponse)
	}

	accounts, err := c.accountService.ListAccountsByReseller(uint(resellerID))
	if err != nil {
		c.logger.Error("failed to list reseller DNS accounts", zap.Error(err))
		return ctx.JSON(http.StatusInternalServerError, schema.ErrorResponse{
			StatusCode: http.StatusInternalServerError,
			Status:     "error",
			Message:    "failed to retrieve reseller DNS accounts: " + err.Error(),
		})
	}

	return ctx.JSON(http.StatusOK, schema.BasicResponseData[[]schema.DNSAccountResponse]{
		BasicResponse: schema.OkBasicResponse,
		Data:          accounts,
	})
}

func (c *DNSAccountController) CreateAccount(ctx echo.Context) error {
	var req schema.CreateDNSAccountRequest
	if err := ctx.Bind(&req); err != nil {
		c.logger.Warn("failed to bind request", zap.Error(err))
		return ctx.JSON(http.StatusBadRequest, schema.BadParamsErrorResponse)
	}
	if err := ctx.Validate(&req); err != nil {
		c.logger.Warn("failed to validate request", zap.Error(err))
		return ctx.JSON(http.StatusBadRequest, schema.BadParamsErrorResponse)
	}

	resellerID, err := peerScopeFromContext(ctx)
	if err != nil {
		return err
	}

	acct, err := c.accountService.CreateAccount(&req, resellerID)
	if err != nil {
		c.logger.Error("failed to create DNS account", zap.Error(err))
		return ctx.JSON(http.StatusInternalServerError, schema.ErrorResponse{
			StatusCode: http.StatusInternalServerError,
			Status:     "error",
			Message:    "failed to create DNS account: " + err.Error(),
		})
	}

	return ctx.JSON(http.StatusCreated, schema.BasicResponseData[schema.DNSAccountResponse]{
		BasicResponse: schema.OkBasicResponse,
		Data:          *acct,
	})
}

func (c *DNSAccountController) CreateAccountForReseller(ctx echo.Context) error {
	resellerID, err := adminResellerIDFromParam(ctx)
	if err != nil {
		return err
	}

	var req schema.CreateDNSAccountRequest
	if err := ctx.Bind(&req); err != nil {
		c.logger.Warn("failed to bind request", zap.Error(err))
		return ctx.JSON(http.StatusBadRequest, schema.BadParamsErrorResponse)
	}
	if err := ctx.Validate(&req); err != nil {
		c.logger.Warn("failed to validate request", zap.Error(err))
		return ctx.JSON(http.StatusBadRequest, schema.BadParamsErrorResponse)
	}

	acct, err := c.accountService.CreateAccount(&req, &resellerID)
	if err != nil {
		c.logger.Error("failed to create DNS account for reseller", zap.Error(err))
		return ctx.JSON(http.StatusInternalServerError, schema.ErrorResponse{
			StatusCode: http.StatusInternalServerError,
			Status:     "error",
			Message:    "failed to create DNS account: " + err.Error(),
		})
	}

	return ctx.JSON(http.StatusCreated, schema.BasicResponseData[schema.DNSAccountResponse]{
		BasicResponse: schema.OkBasicResponse,
		Data:          *acct,
	})
}

func (c *DNSAccountController) UpdateAccount(ctx echo.Context) error {
	id, err := strconv.Atoi(ctx.Param("id"))
	if err != nil {
		return ctx.JSON(http.StatusBadRequest, schema.BadParamsErrorResponse)
	}

	var req schema.UpdateDNSAccountRequest
	if err := ctx.Bind(&req); err != nil {
		c.logger.Warn("failed to bind request", zap.Error(err))
		return ctx.JSON(http.StatusBadRequest, schema.BadParamsErrorResponse)
	}
	if err := ctx.Validate(&req); err != nil {
		c.logger.Warn("failed to validate request", zap.Error(err))
		return ctx.JSON(http.StatusBadRequest, schema.BadParamsErrorResponse)
	}

	resellerID, scopeErr := peerScopeFromContext(ctx)
	if scopeErr != nil {
		return scopeErr
	}

	acct, err := c.accountService.UpdateAccount(uint(id), &req, resellerID)
	if err != nil {
		if errors.Is(err, gorm.ErrRecordNotFound) {
			return ctx.JSON(http.StatusNotFound, schema.ErrorResponse{StatusCode: http.StatusNotFound, Status: "error", Message: "DNS account not found"})
		}
		c.logger.Error("failed to update DNS account", zap.Error(err))
		return ctx.JSON(http.StatusInternalServerError, schema.ErrorResponse{
			StatusCode: http.StatusInternalServerError,
			Status:     "error",
			Message:    "failed to update DNS account: " + err.Error(),
		})
	}

	return ctx.JSON(http.StatusOK, schema.BasicResponseData[schema.DNSAccountResponse]{
		BasicResponse: schema.OkBasicResponse,
		Data:          *acct,
	})
}

func (c *DNSAccountController) UpdateAccountForReseller(ctx echo.Context) error {
	resellerID, err := adminResellerIDFromParam(ctx)
	if err != nil {
		return err
	}

	id, err := strconv.Atoi(ctx.Param("id"))
	if err != nil {
		return ctx.JSON(http.StatusBadRequest, schema.BadParamsErrorResponse)
	}

	var req schema.UpdateDNSAccountRequest
	if err := ctx.Bind(&req); err != nil {
		c.logger.Warn("failed to bind request", zap.Error(err))
		return ctx.JSON(http.StatusBadRequest, schema.BadParamsErrorResponse)
	}
	if err := ctx.Validate(&req); err != nil {
		c.logger.Warn("failed to validate request", zap.Error(err))
		return ctx.JSON(http.StatusBadRequest, schema.BadParamsErrorResponse)
	}

	acct, err := c.accountService.UpdateAccount(uint(id), &req, &resellerID)
	if err != nil {
		if errors.Is(err, gorm.ErrRecordNotFound) {
			return ctx.JSON(http.StatusNotFound, schema.ErrorResponse{StatusCode: http.StatusNotFound, Status: "error", Message: "DNS account not found"})
		}
		c.logger.Error("failed to update reseller's DNS account", zap.Error(err))
		return ctx.JSON(http.StatusInternalServerError, schema.ErrorResponse{
			StatusCode: http.StatusInternalServerError,
			Status:     "error",
			Message:    "failed to update DNS account: " + err.Error(),
		})
	}

	return ctx.JSON(http.StatusOK, schema.BasicResponseData[schema.DNSAccountResponse]{
		BasicResponse: schema.OkBasicResponse,
		Data:          *acct,
	})
}

func (c *DNSAccountController) ResetAccountUsage(ctx echo.Context) error {
	id, err := strconv.Atoi(ctx.Param("id"))
	if err != nil {
		return ctx.JSON(http.StatusBadRequest, schema.BadParamsErrorResponse)
	}

	// Admin-only, per the admin's own explicit requirement -- see
	// requireAdminScope's own doc comment (wg_peer.go) for why a reseller
	// must never be able to zero their own usage.
	if _, scopeErr := requireAdminScope(ctx); scopeErr != nil {
		return scopeErr
	}

	if err := c.accountService.ResetUsage(uint(id), nil); err != nil {
		if errors.Is(err, gorm.ErrRecordNotFound) {
			return ctx.JSON(http.StatusNotFound, schema.ErrorResponse{StatusCode: http.StatusNotFound, Status: "error", Message: "DNS account not found"})
		}
		c.logger.Error("failed to reset DNS account usage", zap.Error(err))
		return ctx.JSON(http.StatusInternalServerError, schema.ErrorResponse{
			StatusCode: http.StatusInternalServerError,
			Status:     "error",
			Message:    "failed to reset usage: " + err.Error(),
		})
	}

	return ctx.JSON(http.StatusOK, schema.OkBasicResponse)
}

func (c *DNSAccountController) DeleteAccount(ctx echo.Context) error {
	id, err := strconv.Atoi(ctx.Param("id"))
	if err != nil {
		return ctx.JSON(http.StatusBadRequest, schema.BadParamsErrorResponse)
	}

	resellerID, scopeErr := peerScopeFromContext(ctx)
	if scopeErr != nil {
		return scopeErr
	}

	if err := c.accountService.DeleteAccount(uint(id), resellerID); err != nil {
		if errors.Is(err, gorm.ErrRecordNotFound) {
			return ctx.JSON(http.StatusNotFound, schema.ErrorResponse{StatusCode: http.StatusNotFound, Status: "error", Message: "DNS account not found"})
		}
		c.logger.Error("failed to delete DNS account", zap.Error(err))
		return ctx.JSON(http.StatusInternalServerError, schema.ErrorResponse{
			StatusCode: http.StatusInternalServerError,
			Status:     "error",
			Message:    "failed to delete DNS account: " + err.Error(),
		})
	}

	return ctx.NoContent(http.StatusNoContent)
}

func (c *DNSAccountController) DeleteAccountForReseller(ctx echo.Context) error {
	resellerID, err := adminResellerIDFromParam(ctx)
	if err != nil {
		return err
	}

	id, err := strconv.Atoi(ctx.Param("id"))
	if err != nil {
		return ctx.JSON(http.StatusBadRequest, schema.BadParamsErrorResponse)
	}

	if err := c.accountService.DeleteAccount(uint(id), &resellerID); err != nil {
		if errors.Is(err, gorm.ErrRecordNotFound) {
			return ctx.JSON(http.StatusNotFound, schema.ErrorResponse{StatusCode: http.StatusNotFound, Status: "error", Message: "DNS account not found"})
		}
		c.logger.Error("failed to delete reseller's DNS account", zap.Error(err))
		return ctx.JSON(http.StatusInternalServerError, schema.ErrorResponse{
			StatusCode: http.StatusInternalServerError,
			Status:     "error",
			Message:    "failed to delete DNS account: " + err.Error(),
		})
	}

	return ctx.NoContent(http.StatusNoContent)
}

func (c *DNSAccountController) BulkDeleteAccounts(ctx echo.Context) error {
	var req schema.BulkDeleteRequest
	if err := ctx.Bind(&req); err != nil || len(req.Ids) == 0 {
		return ctx.JSON(http.StatusBadRequest, schema.BadParamsErrorResponse)
	}

	resellerID, scopeErr := peerScopeFromContext(ctx)
	if scopeErr != nil {
		return scopeErr
	}

	deleted, failed := c.accountService.BulkDeleteAccounts(req.Ids, resellerID)

	failures := make([]schema.BulkDeleteFailure, 0, len(failed))
	for id, msg := range failed {
		failures = append(failures, schema.BulkDeleteFailure{Id: id, Error: msg})
	}

	return ctx.JSON(http.StatusOK, schema.BasicResponseData[schema.BulkDeleteResponse]{
		BasicResponse: schema.OkBasicResponse,
		Data: schema.BulkDeleteResponse{
			Deleted: deleted,
			Failed:  failures,
		},
	})
}

func (c *DNSAccountController) GetAccountShareStatus(ctx echo.Context) error {
	id, err := strconv.Atoi(ctx.Param("id"))
	if err != nil {
		return ctx.JSON(http.StatusBadRequest, schema.BadParamsErrorResponse)
	}

	resellerID, scopeErr := peerScopeFromContext(ctx)
	if scopeErr != nil {
		return scopeErr
	}

	status, err := c.accountService.GetAccountShareStatus(uint(id), resellerID)
	if err != nil {
		if errors.Is(err, gorm.ErrRecordNotFound) {
			return ctx.JSON(http.StatusNotFound, schema.ErrorResponse{StatusCode: http.StatusNotFound, Status: "error", Message: "DNS account not found"})
		}
		return ctx.JSON(http.StatusInternalServerError, schema.ErrorResponse{
			StatusCode: http.StatusInternalServerError,
			Status:     "error",
			Message:    "failed to retrieve share status: " + err.Error(),
		})
	}

	return ctx.JSON(http.StatusOK, schema.BasicResponseData[schema.DNSAccountShareStatusResponse]{
		BasicResponse: schema.OkBasicResponse,
		Data:          *status,
	})
}

func (c *DNSAccountController) UpdateAccountShareStatus(ctx echo.Context) error {
	id, err := strconv.Atoi(ctx.Param("id"))
	if err != nil {
		return ctx.JSON(http.StatusBadRequest, schema.BadParamsErrorResponse)
	}

	resellerID, scopeErr := peerScopeFromContext(ctx)
	if scopeErr != nil {
		return scopeErr
	}

	if err := c.accountService.UpdateAccountShareStatus(uint(id), resellerID); err != nil {
		if errors.Is(err, gorm.ErrRecordNotFound) {
			return ctx.JSON(http.StatusNotFound, schema.ErrorResponse{StatusCode: http.StatusNotFound, Status: "error", Message: "DNS account not found"})
		}
		c.logger.Error("failed to toggle DNS account share status", zap.Error(err))
		return ctx.JSON(http.StatusInternalServerError, schema.ErrorResponse{
			StatusCode: http.StatusInternalServerError,
			Status:     "error",
			Message:    "failed to toggle share status: " + err.Error(),
		})
	}

	return ctx.JSON(http.StatusOK, schema.OkBasicResponse)
}

func (c *DNSAccountController) UpdateAccountShareExpire(ctx echo.Context) error {
	id, err := strconv.Atoi(ctx.Param("id"))
	if err != nil {
		return ctx.JSON(http.StatusBadRequest, schema.BadParamsErrorResponse)
	}

	var body struct {
		ExpireTime *string `json:"expire_time"`
	}
	if err := ctx.Bind(&body); err != nil {
		c.logger.Warn("failed to bind request", zap.Error(err))
		return ctx.JSON(http.StatusBadRequest, schema.BadParamsErrorResponse)
	}

	resellerID, scopeErr := peerScopeFromContext(ctx)
	if scopeErr != nil {
		return scopeErr
	}

	if err := c.accountService.UpdateAccountShareExpire(uint(id), body.ExpireTime, resellerID); err != nil {
		if errors.Is(err, gorm.ErrRecordNotFound) {
			return ctx.JSON(http.StatusNotFound, schema.ErrorResponse{StatusCode: http.StatusNotFound, Status: "error", Message: "DNS account not found"})
		}
		c.logger.Error("failed to update DNS account share expire time", zap.Error(err))
		return ctx.JSON(http.StatusInternalServerError, schema.ErrorResponse{
			StatusCode: http.StatusInternalServerError,
			Status:     "error",
			Message:    "failed to update share expire time: " + err.Error(),
		})
	}

	return ctx.JSON(http.StatusOK, schema.OkBasicResponse)
}

// GetAssignedDNSPanels returns the DNS panel IDs a reseller is allowed to
// use. Admin, or the reseller owner.
func (c *DNSAccountController) GetAssignedDNSPanels(ctx echo.Context) error {
	nid, err := strconv.ParseUint(ctx.Param("reseller_id"), 10, 32)
	if err != nil {
		return ctx.JSON(http.StatusBadRequest, schema.BadParamsErrorResponse)
	}

	role, resellerClaim := getRoleAndResellerFromContext(ctx)
	if role == "reseller" && (resellerClaim == nil || *resellerClaim != uint(nid)) {
		return ctx.JSON(http.StatusForbidden, schema.ErrorResponse{StatusCode: http.StatusForbidden, Status: "error", Message: "forbidden"})
	}

	panelIDs, err := c.accountService.GetAssignedDNSPanels(uint(nid))
	if err != nil {
		c.logger.Error("failed to get assigned DNS panels", zap.Uint64("reseller_id", nid), zap.Error(err))
		return ctx.JSON(http.StatusInternalServerError, schema.InternalServerErrorResponse)
	}

	return ctx.JSON(http.StatusOK, schema.BasicResponseData[[]uint]{
		BasicResponse: schema.OkBasicResponse,
		Data:          panelIDs,
	})
}

// GetAssignedDNSPanelSummaries is GetAssignedDNSPanels' reseller-safe
// counterpart, returning id+name instead of bare IDs.
func (c *DNSAccountController) GetAssignedDNSPanelSummaries(ctx echo.Context) error {
	nid, err := strconv.ParseUint(ctx.Param("reseller_id"), 10, 32)
	if err != nil {
		return ctx.JSON(http.StatusBadRequest, schema.BadParamsErrorResponse)
	}

	role, resellerClaim := getRoleAndResellerFromContext(ctx)
	if role == "reseller" && (resellerClaim == nil || *resellerClaim != uint(nid)) {
		return ctx.JSON(http.StatusForbidden, schema.ErrorResponse{StatusCode: http.StatusForbidden, Status: "error", Message: "forbidden"})
	}

	panels, err := c.accountService.GetAssignedDNSPanelSummaries(uint(nid))
	if err != nil {
		c.logger.Error("failed to get assigned DNS panel summaries", zap.Uint64("reseller_id", nid), zap.Error(err))
		return ctx.JSON(http.StatusInternalServerError, schema.InternalServerErrorResponse)
	}

	summaries := make([]schema.DNSPanelSummary, 0, len(panels))
	for _, panel := range panels {
		summaries = append(summaries, schema.DNSPanelSummary{Id: panel.ID, Name: panel.Name})
	}

	return ctx.JSON(http.StatusOK, schema.BasicResponseData[[]schema.DNSPanelSummary]{
		BasicResponse: schema.OkBasicResponse,
		Data:          summaries,
	})
}

// TestPanelForReseller is the reseller-safe counterpart to
// DNSPanelController.TestConnection (admin-only) -- lets a reseller probe
// one of THEIR OWN assigned DNS panels for its template list while
// creating an account, without needing admin access. See
// DNSAccountService.TestPanelForReseller's own doc comment for the
// confirmed, reported bug this fixes.
func (c *DNSAccountController) TestPanelForReseller(ctx echo.Context) error {
	nid, err := strconv.ParseUint(ctx.Param("reseller_id"), 10, 32)
	if err != nil {
		return ctx.JSON(http.StatusBadRequest, schema.BadParamsErrorResponse)
	}

	role, resellerClaim := getRoleAndResellerFromContext(ctx)
	if role == "reseller" && (resellerClaim == nil || *resellerClaim != uint(nid)) {
		return ctx.JSON(http.StatusForbidden, schema.ErrorResponse{StatusCode: http.StatusForbidden, Status: "error", Message: "forbidden"})
	}

	panelID, err := strconv.ParseUint(ctx.Param("id"), 10, 32)
	if err != nil {
		return ctx.JSON(http.StatusBadRequest, schema.BadParamsErrorResponse)
	}

	result, err := c.accountService.TestPanelForReseller(uint(nid), uint(panelID))
	if err != nil {
		c.logger.Warn("DNS panel connection test failed for reseller", zap.Uint64("reseller_id", nid), zap.Uint64("panel_id", panelID), zap.Error(err))
		return ctx.JSON(http.StatusBadGateway, schema.ErrorResponse{
			StatusCode: http.StatusBadGateway,
			Status:     "error",
			Message:    "connection test failed: " + err.Error(),
		})
	}

	return ctx.JSON(http.StatusOK, schema.BasicResponseData[schema.TestDNSConnectionResponse]{
		BasicResponse: schema.OkBasicResponse,
		Data:          *result,
	})
}

// SetAssignedDNSPanels replaces the full set of DNS panels a reseller is
// allowed to use. Admin only.
func (c *DNSAccountController) SetAssignedDNSPanels(ctx echo.Context) error {
	if role, _ := getRoleFromContext(ctx); role != "admin" {
		return ctx.JSON(http.StatusForbidden, schema.ErrorResponse{StatusCode: http.StatusForbidden, Status: "error", Message: "forbidden"})
	}

	nid, err := strconv.ParseUint(ctx.Param("reseller_id"), 10, 32)
	if err != nil {
		return ctx.JSON(http.StatusBadRequest, schema.BadParamsErrorResponse)
	}

	var req schema.AssignResellerDNSPanelsRequest
	if err := ctx.Bind(&req); err != nil {
		c.logger.Warn("failed to bind request", zap.Error(err))
		return ctx.JSON(http.StatusBadRequest, schema.BadParamsErrorResponse)
	}

	if err := c.accountService.SetAssignedDNSPanels(uint(nid), req.PanelIDs); err != nil {
		c.logger.Error("failed to set assigned DNS panels", zap.Uint64("reseller_id", nid), zap.Error(err))
		return ctx.JSON(http.StatusInternalServerError, schema.ErrorResponse{
			StatusCode: http.StatusInternalServerError,
			Status:     "error",
			Message:    err.Error(),
		})
	}

	return ctx.JSON(http.StatusOK, schema.OkBasicResponse)
}
