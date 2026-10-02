package http

import (
	"net/http"
	"strconv"

	"github.com/labstack/echo/v4"
	"go.uber.org/zap"

	"github.com/maahdima/mwp/api/http/schema"
	"github.com/maahdima/mwp/api/service"
)

type WalletController struct {
	walletService    *service.Wallet
	resellerService  *service.Reseller
	v2raySyncService *service.V2RaySyncService
	logger           *zap.Logger
}

func NewWalletController(walletService *service.Wallet, resellerService *service.Reseller, v2raySyncService *service.V2RaySyncService) *WalletController {
	return &WalletController{
		walletService:    walletService,
		resellerService:  resellerService,
		v2raySyncService: v2raySyncService,
		logger:           zap.L().Named("WalletController"),
	}
}

// GetWalletBalance retrieves the current balance for a reseller's wallet.
// Admins can check any wallet; resellers can only check their own.
func (wc *WalletController) GetWalletBalance(ctx echo.Context) error {
	resellerIDStr := ctx.Param("reseller_id")
	if resellerIDStr == "" {
		return ctx.JSON(http.StatusBadRequest, schema.BadParamsErrorResponse)
	}

	resellerID, err := strconv.ParseUint(resellerIDStr, 10, 32)
	if err != nil {
		wc.logger.Warn("invalid reseller_id", zap.Error(err))
		return ctx.JSON(http.StatusBadRequest, schema.BadParamsErrorResponse)
	}

	role, loggedInResellerID := getRoleAndResellerFromContext(ctx)
	if role == "reseller" && (loggedInResellerID == nil || *loggedInResellerID != uint(resellerID)) {
		return ctx.JSON(http.StatusForbidden, schema.ErrorResponse{
			StatusCode: http.StatusForbidden,
			Status:     "error",
			Message:    "forbidden",
		})
	}

	wallet, err := wc.walletService.GetOrCreateWallet(uint(resellerID))
	if err != nil {
		wc.logger.Error("failed to get wallet balance", zap.Error(err))
		return ctx.JSON(http.StatusInternalServerError, schema.ErrorResponse{
			StatusCode: http.StatusInternalServerError,
			Status:     "error",
			Message:    err.Error(),
		})
	}

	return ctx.JSON(http.StatusOK, schema.BasicResponseData[map[string]interface{}]{
		BasicResponse: schema.OkBasicResponse,
		Data: map[string]interface{}{
			"balance_amount": wallet.BalanceAmount,
			"reseller_id":    resellerID,
			"is_frozen":      wallet.IsFrozen,
			"frozen_reason":  wallet.FrozenReason,
		},
	})
}

// Credit adds funds to a reseller's wallet
func (wc *WalletController) Credit(ctx echo.Context) error {
	resellerIDStr := ctx.Param("reseller_id")
	if resellerIDStr == "" {
		return ctx.JSON(http.StatusBadRequest, schema.BadParamsErrorResponse)
	}

	resellerID, err := strconv.ParseUint(resellerIDStr, 10, 32)
	if err != nil {
		return ctx.JSON(http.StatusBadRequest, schema.BadParamsErrorResponse)
	}

	// Only admins can credit wallets
	role, _ := getRoleAndResellerFromContext(ctx)
	if role == "reseller" {
		return ctx.JSON(http.StatusForbidden, schema.ErrorResponse{
			StatusCode: http.StatusForbidden,
			Status:     "error",
			Message:    "forbidden",
		})
	}

	var req schema.CreditRequest
	if err := ctx.Bind(&req); err != nil {
		wc.logger.Warn("failed to bind request", zap.Error(err))
		return ctx.JSON(http.StatusBadRequest, schema.BadParamsErrorResponse)
	}

	if err := ctx.Validate(&req); err != nil {
		wc.logger.Warn("failed to validate request", zap.Error(err))
		return ctx.JSON(http.StatusBadRequest, schema.BadParamsErrorResponse)
	}

	refType := "MANUAL_CREDIT"
	ledgerEntry, err := wc.walletService.Credit(uint(resellerID), req.Amount, req.Description, &refType, req.ReferenceID)
	if err != nil {
		wc.logger.Error("failed to credit wallet", zap.Error(err))
		return ctx.JSON(http.StatusInternalServerError, schema.ErrorResponse{
			StatusCode: http.StatusInternalServerError,
			Status:     "error",
			Message:    err.Error(),
		})
	}

	// Payment-based billing: a manual top-up may bring a billing-suspended
	// reseller back to solvent -- re-check and resume their configs if so.
	// No-op for a Volume-based reseller or one that isn't currently
	// suspended (see ResumeBillingSuspension's own doc comment).
	if err := wc.resellerService.ResumeBillingSuspension(uint(resellerID), wc.walletService, wc.v2raySyncService); err != nil {
		wc.logger.Warn("failed to resume billing-suspended reseller after credit", zap.Uint64("reseller_id", resellerID), zap.Error(err))
	}

	return ctx.JSON(http.StatusCreated, schema.BasicResponseData[map[string]interface{}]{
		BasicResponse: schema.OkBasicResponse,
		Data: map[string]interface{}{
			"transaction_id": ledgerEntry.TransactionID,
			"balance_after":  ledgerEntry.BalanceAfter,
			"amount":         req.Amount,
		},
	})
}

// Debit subtracts funds from a reseller's wallet
func (wc *WalletController) Debit(ctx echo.Context) error {
	resellerIDStr := ctx.Param("reseller_id")
	if resellerIDStr == "" {
		return ctx.JSON(http.StatusBadRequest, schema.BadParamsErrorResponse)
	}

	resellerID, err := strconv.ParseUint(resellerIDStr, 10, 32)
	if err != nil {
		return ctx.JSON(http.StatusBadRequest, schema.BadParamsErrorResponse)
	}

	// Only admins can debit wallets
	role, _ := getRoleAndResellerFromContext(ctx)
	if role == "reseller" {
		return ctx.JSON(http.StatusForbidden, schema.ErrorResponse{
			StatusCode: http.StatusForbidden,
			Status:     "error",
			Message:    "forbidden",
		})
	}

	var req schema.DebitRequest
	if err := ctx.Bind(&req); err != nil {
		wc.logger.Warn("failed to bind request", zap.Error(err))
		return ctx.JSON(http.StatusBadRequest, schema.BadParamsErrorResponse)
	}

	if err := ctx.Validate(&req); err != nil {
		wc.logger.Warn("failed to validate request", zap.Error(err))
		return ctx.JSON(http.StatusBadRequest, schema.BadParamsErrorResponse)
	}

	refType := "MANUAL_DEBIT"
	ledgerEntry, err := wc.walletService.Debit(uint(resellerID), req.Amount, req.Description, &refType, req.ReferenceID)
	if err != nil {
		wc.logger.Error("failed to debit wallet", zap.Error(err))
		return ctx.JSON(http.StatusInternalServerError, schema.ErrorResponse{
			StatusCode: http.StatusInternalServerError,
			Status:     "error",
			Message:    err.Error(),
		})
	}

	return ctx.JSON(http.StatusOK, schema.BasicResponseData[map[string]interface{}]{
		BasicResponse: schema.OkBasicResponse,
		Data: map[string]interface{}{
			"transaction_id": ledgerEntry.TransactionID,
			"balance_after":  ledgerEntry.BalanceAfter,
			"amount":         req.Amount,
		},
	})
}

// GetLedgerHistory retrieves transaction history
func (wc *WalletController) GetLedgerHistory(ctx echo.Context) error {
	resellerIDStr := ctx.Param("reseller_id")
	if resellerIDStr == "" {
		return ctx.JSON(http.StatusBadRequest, schema.BadParamsErrorResponse)
	}

	resellerID, err := strconv.ParseUint(resellerIDStr, 10, 32)
	if err != nil {
		return ctx.JSON(http.StatusBadRequest, schema.BadParamsErrorResponse)
	}

	// Resellers can only view their own ledger
	role, loggedInResellerID := getRoleAndResellerFromContext(ctx)
	if role == "reseller" && (loggedInResellerID == nil || *loggedInResellerID != uint(resellerID)) {
		return ctx.JSON(http.StatusForbidden, schema.ErrorResponse{
			StatusCode: http.StatusForbidden,
			Status:     "error",
			Message:    "forbidden",
		})
	}

	limitStr := ctx.QueryParam("limit")
	limit := 50
	if limitStr != "" {
		if parsedLimit, err := strconv.Atoi(limitStr); err == nil && parsedLimit > 0 && parsedLimit <= 500 {
			limit = parsedLimit
		}
	}

	entries, err := wc.walletService.GetLedgerHistory(uint(resellerID), limit)
	if err != nil {
		wc.logger.Error("failed to get ledger history", zap.Error(err))
		return ctx.JSON(http.StatusInternalServerError, schema.ErrorResponse{
			StatusCode: http.StatusInternalServerError,
			Status:     "error",
			Message:    err.Error(),
		})
	}

	response := make([]schema.LedgerEntryResponse, len(entries))
	for i, entry := range entries {
		response[i] = schema.LedgerEntryResponse{
			ID:            entry.ID,
			ResellerID:    entry.ResellerID,
			TransactionID: entry.TransactionID,
			EntryType:     entry.EntryType,
			Amount:        entry.Amount,
			BalanceAfter:  entry.BalanceAfter,
			Description:   entry.Description,
			ReferenceType: entry.ReferenceType,
			ReferenceID:   entry.ReferenceID,
			CreatedAt:     entry.CreatedAt.Unix(),
		}
	}

	return ctx.JSON(http.StatusOK, schema.BasicResponseData[[]schema.LedgerEntryResponse]{
		BasicResponse: schema.OkBasicResponse,
		Data:          response,
	})
}

// FreezeWallet freezes a reseller's wallet (admin only)
func (wc *WalletController) FreezeWallet(ctx echo.Context) error {
	resellerIDStr := ctx.Param("reseller_id")
	if resellerIDStr == "" {
		return ctx.JSON(http.StatusBadRequest, schema.BadParamsErrorResponse)
	}

	resellerID, err := strconv.ParseUint(resellerIDStr, 10, 32)
	if err != nil {
		return ctx.JSON(http.StatusBadRequest, schema.BadParamsErrorResponse)
	}

	role, _ := getRoleAndResellerFromContext(ctx)
	if role == "reseller" {
		return ctx.JSON(http.StatusForbidden, schema.ErrorResponse{
			StatusCode: http.StatusForbidden,
			Status:     "error",
			Message:    "forbidden",
		})
	}

	var req schema.FreezeWalletRequest
	if err := ctx.Bind(&req); err != nil {
		return ctx.JSON(http.StatusBadRequest, schema.BadParamsErrorResponse)
	}

	if err := ctx.Validate(&req); err != nil {
		return ctx.JSON(http.StatusBadRequest, schema.BadParamsErrorResponse)
	}

	if err := wc.walletService.FreezeWallet(uint(resellerID), req.Reason); err != nil {
		wc.logger.Error("failed to freeze wallet", zap.Error(err))
		return ctx.JSON(http.StatusInternalServerError, schema.ErrorResponse{
			StatusCode: http.StatusInternalServerError,
			Status:     "error",
			Message:    err.Error(),
		})
	}

	return ctx.JSON(http.StatusOK, schema.OkBasicResponse)
}

// UnfreezeWallet unfreezes a reseller's wallet (admin only)
func (wc *WalletController) UnfreezeWallet(ctx echo.Context) error {
	resellerIDStr := ctx.Param("reseller_id")
	if resellerIDStr == "" {
		return ctx.JSON(http.StatusBadRequest, schema.BadParamsErrorResponse)
	}

	resellerID, err := strconv.ParseUint(resellerIDStr, 10, 32)
	if err != nil {
		return ctx.JSON(http.StatusBadRequest, schema.BadParamsErrorResponse)
	}

	role, _ := getRoleAndResellerFromContext(ctx)
	if role == "reseller" {
		return ctx.JSON(http.StatusForbidden, schema.ErrorResponse{
			StatusCode: http.StatusForbidden,
			Status:     "error",
			Message:    "forbidden",
		})
	}

	if err := wc.walletService.UnfreezeWallet(uint(resellerID)); err != nil {
		wc.logger.Error("failed to unfreeze wallet", zap.Error(err))
		return ctx.JSON(http.StatusInternalServerError, schema.ErrorResponse{
			StatusCode: http.StatusInternalServerError,
			Status:     "error",
			Message:    err.Error(),
		})
	}

	return ctx.JSON(http.StatusOK, schema.OkBasicResponse)
}
