package http

import (
	"errors"
	"fmt"
	"io"
	"net/http"
	"strconv"

	"github.com/maahdima/mwp/api/http/schema"
	"github.com/maahdima/mwp/api/service"

	"github.com/labstack/echo/v4"
	"go.uber.org/zap"
	"gorm.io/gorm"
)

// UserManagerAccountController manages RouterOS User Manager accounts
// (L2TP/PPTP/SSTP/OpenVPN) -- structurally mirrors WgPeerController, reusing
// the package-level errAlreadyHandled/getRoleAndResellerFromContext/
// peerScopeFromContext/adminResellerIDFromParam helpers directly since
// they're role/claims-only, not peer-specific.
type UserManagerAccountController struct {
	accountService *service.UserManagerService
	configFile     *service.UserManagerConfigFile
	logger         *zap.Logger
}

func NewUserManagerAccountController(accountService *service.UserManagerService, configFile *service.UserManagerConfigFile) *UserManagerAccountController {
	return &UserManagerAccountController{
		accountService: accountService,
		configFile:     configFile,
		logger:         zap.L().Named("UserManagerAccountController"),
	}
}

func (c *UserManagerAccountController) ListAccounts(ctx echo.Context) error {
	resellerID, err := peerScopeFromContext(ctx)
	if err != nil {
		return err
	}

	accounts, err := c.accountService.ListAccounts(resellerID)
	if err != nil {
		c.logger.Error("failed to list user manager accounts", zap.Error(err))
		return ctx.JSON(http.StatusInternalServerError, schema.ErrorResponse{
			StatusCode: http.StatusInternalServerError,
			Status:     "error",
			Message:    "failed to retrieve user manager accounts: " + err.Error(),
		})
	}

	return ctx.JSON(http.StatusOK, schema.BasicResponseData[[]schema.UserManagerAccountResponse]{
		BasicResponse: schema.OkBasicResponse,
		Data:          *accounts,
	})
}

// ListAccountsByReseller returns all accounts owned by a specific reseller. Admin only.
func (c *UserManagerAccountController) ListAccountsByReseller(ctx echo.Context) error {
	if role, _ := getRoleAndResellerFromContext(ctx); role == "reseller" {
		return ctx.JSON(http.StatusForbidden, schema.ErrorResponse{StatusCode: http.StatusForbidden, Status: "error", Message: "forbidden"})
	}

	resellerIDStr := ctx.Param("reseller_id")
	resellerID, err := strconv.ParseUint(resellerIDStr, 10, 32)
	if err != nil {
		return ctx.JSON(http.StatusBadRequest, schema.BadParamsErrorResponse)
	}

	accounts, err := c.accountService.ListAccountsByReseller(uint(resellerID))
	if err != nil {
		c.logger.Error("failed to list reseller user manager accounts", zap.Error(err))
		return ctx.JSON(http.StatusInternalServerError, schema.ErrorResponse{
			StatusCode: http.StatusInternalServerError,
			Status:     "error",
			Message:    "failed to retrieve reseller user manager accounts: " + err.Error(),
		})
	}

	return ctx.JSON(http.StatusOK, schema.BasicResponseData[[]schema.UserManagerAccountResponse]{
		BasicResponse: schema.OkBasicResponse,
		Data:          *accounts,
	})
}

func (c *UserManagerAccountController) CreateAccount(ctx echo.Context) error {
	var req schema.CreateUserManagerAccountRequest
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

	account, err := c.accountService.CreateAccount(&req, resellerID)
	if err != nil {
		c.logger.Error("failed to create user manager account", zap.Error(err))
		return ctx.JSON(http.StatusInternalServerError, schema.ErrorResponse{
			StatusCode: http.StatusInternalServerError,
			Status:     "error",
			Message:    "failed to create user manager account: " + err.Error(),
		})
	}

	return ctx.JSON(http.StatusCreated, schema.BasicResponseData[schema.UserManagerAccountResponse]{
		BasicResponse: schema.OkBasicResponse,
		Data:          *account,
	})
}

// BulkCreateAccounts mirrors V2RayPackageController.BulkCreatePackages/
// WgPeerController.BulkCreatePeers exactly -- see UserManagerService.
// BulkCreateAccounts' own doc comment for the batch-creation behavior this
// powers.
func (c *UserManagerAccountController) BulkCreateAccounts(ctx echo.Context) error {
	var req schema.BulkCreateUserManagerAccountRequest
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

	accounts, err := c.accountService.BulkCreateAccounts(&req, resellerID)
	if err != nil {
		c.logger.Error("failed to bulk create user manager accounts", zap.Error(err))
		return ctx.JSON(http.StatusInternalServerError, schema.ErrorResponse{
			StatusCode: http.StatusInternalServerError,
			Status:     "error",
			Message:    "failed to bulk create user manager accounts: " + err.Error(),
		})
	}

	return ctx.JSON(http.StatusCreated, schema.BasicResponseData[schema.BulkCreateUserManagerAccountResponse]{
		BasicResponse: schema.OkBasicResponse,
		Data:          schema.BulkCreateUserManagerAccountResponse{Accounts: accounts},
	})
}

// CreateAccountForReseller lets an admin create an account on behalf of a specific reseller.
func (c *UserManagerAccountController) CreateAccountForReseller(ctx echo.Context) error {
	resellerID, err := adminResellerIDFromParam(ctx)
	if err != nil {
		return err
	}

	var req schema.CreateUserManagerAccountRequest
	if err := ctx.Bind(&req); err != nil {
		c.logger.Warn("failed to bind request", zap.Error(err))
		return ctx.JSON(http.StatusBadRequest, schema.BadParamsErrorResponse)
	}
	if err := ctx.Validate(&req); err != nil {
		c.logger.Warn("failed to validate request", zap.Error(err))
		return ctx.JSON(http.StatusBadRequest, schema.BadParamsErrorResponse)
	}

	account, err := c.accountService.CreateAccount(&req, &resellerID)
	if err != nil {
		c.logger.Error("failed to create user manager account for reseller", zap.Error(err))
		return ctx.JSON(http.StatusInternalServerError, schema.ErrorResponse{
			StatusCode: http.StatusInternalServerError,
			Status:     "error",
			Message:    "failed to create user manager account: " + err.Error(),
		})
	}

	return ctx.JSON(http.StatusCreated, schema.BasicResponseData[schema.UserManagerAccountResponse]{
		BasicResponse: schema.OkBasicResponse,
		Data:          *account,
	})
}

func (c *UserManagerAccountController) UpdateAccount(ctx echo.Context) error {
	id, err := strconv.Atoi(ctx.Param("id"))
	if err != nil {
		return ctx.JSON(http.StatusBadRequest, schema.BadParamsErrorResponse)
	}

	var req schema.UpdateUserManagerAccountRequest
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

	account, err := c.accountService.UpdateAccount(uint(id), &req, resellerID)
	if err != nil {
		if errors.Is(err, gorm.ErrRecordNotFound) {
			return ctx.JSON(http.StatusNotFound, schema.ErrorResponse{StatusCode: http.StatusNotFound, Status: "error", Message: "user manager account not found"})
		}
		c.logger.Error("failed to update user manager account", zap.Error(err))
		return ctx.JSON(http.StatusInternalServerError, schema.ErrorResponse{
			StatusCode: http.StatusInternalServerError,
			Status:     "error",
			Message:    "failed to update user manager account: " + err.Error(),
		})
	}

	return ctx.JSON(http.StatusOK, schema.BasicResponseData[schema.UserManagerAccountResponse]{
		BasicResponse: schema.OkBasicResponse,
		Data:          *account,
	})
}

func (c *UserManagerAccountController) UpdateAccountForReseller(ctx echo.Context) error {
	resellerID, err := adminResellerIDFromParam(ctx)
	if err != nil {
		return err
	}

	id, err := strconv.Atoi(ctx.Param("id"))
	if err != nil {
		return ctx.JSON(http.StatusBadRequest, schema.BadParamsErrorResponse)
	}

	var req schema.UpdateUserManagerAccountRequest
	if err := ctx.Bind(&req); err != nil {
		c.logger.Warn("failed to bind request", zap.Error(err))
		return ctx.JSON(http.StatusBadRequest, schema.BadParamsErrorResponse)
	}
	if err := ctx.Validate(&req); err != nil {
		c.logger.Warn("failed to validate request", zap.Error(err))
		return ctx.JSON(http.StatusBadRequest, schema.BadParamsErrorResponse)
	}

	account, err := c.accountService.UpdateAccount(uint(id), &req, &resellerID)
	if err != nil {
		if errors.Is(err, gorm.ErrRecordNotFound) {
			return ctx.JSON(http.StatusNotFound, schema.ErrorResponse{StatusCode: http.StatusNotFound, Status: "error", Message: "user manager account not found"})
		}
		c.logger.Error("failed to update reseller's user manager account", zap.Error(err))
		return ctx.JSON(http.StatusInternalServerError, schema.ErrorResponse{
			StatusCode: http.StatusInternalServerError,
			Status:     "error",
			Message:    "failed to update user manager account: " + err.Error(),
		})
	}

	return ctx.JSON(http.StatusOK, schema.BasicResponseData[schema.UserManagerAccountResponse]{
		BasicResponse: schema.OkBasicResponse,
		Data:          *account,
	})
}

func (c *UserManagerAccountController) DeleteAccount(ctx echo.Context) error {
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
			return ctx.JSON(http.StatusNotFound, schema.ErrorResponse{StatusCode: http.StatusNotFound, Status: "error", Message: "user manager account not found"})
		}
		c.logger.Error("failed to delete user manager account", zap.Error(err))
		return ctx.JSON(http.StatusInternalServerError, schema.ErrorResponse{
			StatusCode: http.StatusInternalServerError,
			Status:     "error",
			Message:    "failed to delete user manager account: " + err.Error(),
		})
	}

	return ctx.JSON(http.StatusNoContent, schema.BasicResponse{StatusCode: http.StatusNoContent, Status: "success"})
}

// ResetAccountUsage mirrors WgPeerController.ResetPeerUsage -- the admin's
// own explicit "User Manager needs a Reset Usage button just like
// WireGuard" request. See UserManagerService.ResetUsage's own doc comment
// for why this is offset-based rather than a direct RouterOS counter
// reset.
func (c *UserManagerAccountController) ResetAccountUsage(ctx echo.Context) error {
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
			return ctx.JSON(http.StatusNotFound, schema.ErrorResponse{StatusCode: http.StatusNotFound, Status: "error", Message: "user manager account not found"})
		}
		c.logger.Error("failed to reset user manager account usage", zap.Error(err))
		return ctx.JSON(http.StatusInternalServerError, schema.ErrorResponse{
			StatusCode: http.StatusInternalServerError,
			Status:     "error",
			Message:    "failed to reset account usage: " + err.Error(),
		})
	}

	return ctx.JSON(http.StatusOK, schema.OkBasicResponse)
}

// BulkDeleteAccounts mirrors WgPeerController.BulkDeletePeers exactly --
// see its doc comment for the "delete expired/quota-exhausted" action this
// powers.
func (c *UserManagerAccountController) BulkDeleteAccounts(ctx echo.Context) error {
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

func (c *UserManagerAccountController) DeleteAccountForReseller(ctx echo.Context) error {
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
			return ctx.JSON(http.StatusNotFound, schema.ErrorResponse{StatusCode: http.StatusNotFound, Status: "error", Message: "user manager account not found"})
		}
		c.logger.Error("failed to delete reseller's user manager account", zap.Error(err))
		return ctx.JSON(http.StatusInternalServerError, schema.ErrorResponse{
			StatusCode: http.StatusInternalServerError,
			Status:     "error",
			Message:    "failed to delete user manager account: " + err.Error(),
		})
	}

	return ctx.JSON(http.StatusNoContent, schema.BasicResponse{StatusCode: http.StatusNoContent, Status: "success"})
}

func (c *UserManagerAccountController) ToggleAccountStatus(ctx echo.Context) error {
	id, err := strconv.Atoi(ctx.Param("id"))
	if err != nil {
		return ctx.JSON(http.StatusBadRequest, schema.BadParamsErrorResponse)
	}

	resellerID, scopeErr := peerScopeFromContext(ctx)
	if scopeErr != nil {
		return scopeErr
	}

	if err := c.accountService.ToggleAccountStatus(uint(id), resellerID); err != nil {
		if errors.Is(err, gorm.ErrRecordNotFound) {
			return ctx.JSON(http.StatusNotFound, schema.ErrorResponse{StatusCode: http.StatusNotFound, Status: "error", Message: "user manager account not found"})
		}
		c.logger.Error("failed to toggle user manager account status", zap.Error(err))
		return ctx.JSON(http.StatusInternalServerError, schema.ErrorResponse{
			StatusCode: http.StatusInternalServerError,
			Status:     "error",
			Message:    "failed to toggle user manager account status: " + err.Error(),
		})
	}

	return ctx.JSON(http.StatusOK, schema.OkBasicResponse)
}

func (c *UserManagerAccountController) ToggleAccountStatusForReseller(ctx echo.Context) error {
	resellerID, err := adminResellerIDFromParam(ctx)
	if err != nil {
		return err
	}

	id, err := strconv.Atoi(ctx.Param("id"))
	if err != nil {
		return ctx.JSON(http.StatusBadRequest, schema.BadParamsErrorResponse)
	}

	if err := c.accountService.ToggleAccountStatus(uint(id), &resellerID); err != nil {
		if errors.Is(err, gorm.ErrRecordNotFound) {
			return ctx.JSON(http.StatusNotFound, schema.ErrorResponse{StatusCode: http.StatusNotFound, Status: "error", Message: "user manager account not found"})
		}
		c.logger.Error("failed to toggle reseller's user manager account status", zap.Error(err))
		return ctx.JSON(http.StatusInternalServerError, schema.ErrorResponse{
			StatusCode: http.StatusInternalServerError,
			Status:     "error",
			Message:    "failed to toggle user manager account status: " + err.Error(),
		})
	}

	return ctx.JSON(http.StatusOK, schema.OkBasicResponse)
}

func (c *UserManagerAccountController) ChangeAccountPassword(ctx echo.Context) error {
	id, err := strconv.Atoi(ctx.Param("id"))
	if err != nil {
		return ctx.JSON(http.StatusBadRequest, schema.BadParamsErrorResponse)
	}

	resellerID, scopeErr := peerScopeFromContext(ctx)
	if scopeErr != nil {
		return scopeErr
	}

	var req schema.ChangeUserManagerAccountPasswordRequest
	if err := ctx.Bind(&req); err != nil {
		return ctx.JSON(http.StatusBadRequest, schema.BadParamsErrorResponse)
	}
	if err := ctx.Validate(&req); err != nil {
		return ctx.JSON(http.StatusBadRequest, schema.BadParamsErrorResponse)
	}

	if err := c.accountService.ChangeAccountPassword(uint(id), req.Password, resellerID); err != nil {
		if errors.Is(err, gorm.ErrRecordNotFound) {
			return ctx.JSON(http.StatusNotFound, schema.ErrorResponse{StatusCode: http.StatusNotFound, Status: "error", Message: "user manager account not found"})
		}
		c.logger.Error("failed to change user manager account password", zap.Error(err))
		return ctx.JSON(http.StatusInternalServerError, schema.ErrorResponse{
			StatusCode: http.StatusInternalServerError,
			Status:     "error",
			Message:    "failed to change password: " + err.Error(),
		})
	}

	return ctx.JSON(http.StatusOK, schema.OkBasicResponse)
}

func (c *UserManagerAccountController) ChangeAccountPasswordForReseller(ctx echo.Context) error {
	resellerID, err := adminResellerIDFromParam(ctx)
	if err != nil {
		return err
	}

	id, err := strconv.Atoi(ctx.Param("id"))
	if err != nil {
		return ctx.JSON(http.StatusBadRequest, schema.BadParamsErrorResponse)
	}

	var req schema.ChangeUserManagerAccountPasswordRequest
	if err := ctx.Bind(&req); err != nil {
		return ctx.JSON(http.StatusBadRequest, schema.BadParamsErrorResponse)
	}
	if err := ctx.Validate(&req); err != nil {
		return ctx.JSON(http.StatusBadRequest, schema.BadParamsErrorResponse)
	}

	if err := c.accountService.ChangeAccountPassword(uint(id), req.Password, &resellerID); err != nil {
		if errors.Is(err, gorm.ErrRecordNotFound) {
			return ctx.JSON(http.StatusNotFound, schema.ErrorResponse{StatusCode: http.StatusNotFound, Status: "error", Message: "user manager account not found"})
		}
		c.logger.Error("failed to change reseller's user manager account password", zap.Error(err))
		return ctx.JSON(http.StatusInternalServerError, schema.ErrorResponse{
			StatusCode: http.StatusInternalServerError,
			Status:     "error",
			Message:    "failed to change password: " + err.Error(),
		})
	}

	return ctx.JSON(http.StatusOK, schema.OkBasicResponse)
}

func (c *UserManagerAccountController) GetAccountShareStatus(ctx echo.Context) error {
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
			return ctx.JSON(http.StatusNotFound, schema.ErrorResponse{StatusCode: http.StatusNotFound, Status: "error", Message: "user manager account not found"})
		}
		return ctx.JSON(http.StatusInternalServerError, schema.ErrorResponse{
			StatusCode: http.StatusInternalServerError,
			Status:     "error",
			Message:    "failed to retrieve share status: " + err.Error(),
		})
	}

	return ctx.JSON(http.StatusOK, schema.BasicResponseData[schema.UserManagerAccountShareStatusResponse]{
		BasicResponse: schema.OkBasicResponse,
		Data:          *status,
	})
}

func (c *UserManagerAccountController) UpdateAccountShareStatus(ctx echo.Context) error {
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
			return ctx.JSON(http.StatusNotFound, schema.ErrorResponse{StatusCode: http.StatusNotFound, Status: "error", Message: "user manager account not found"})
		}
		c.logger.Error("failed to toggle user manager account share status", zap.Error(err))
		return ctx.JSON(http.StatusInternalServerError, schema.ErrorResponse{
			StatusCode: http.StatusInternalServerError,
			Status:     "error",
			Message:    "failed to toggle share status: " + err.Error(),
		})
	}

	return ctx.JSON(http.StatusOK, schema.OkBasicResponse)
}

func (c *UserManagerAccountController) UpdateAccountShareExpire(ctx echo.Context) error {
	id, err := strconv.Atoi(ctx.Param("id"))
	if err != nil {
		return ctx.JSON(http.StatusBadRequest, schema.BadParamsErrorResponse)
	}

	var req schema.UpdateUserManagerAccountShareExpireRequest
	if err := ctx.Bind(&req); err != nil {
		c.logger.Warn("failed to bind request", zap.Error(err))
		return ctx.JSON(http.StatusBadRequest, schema.BadParamsErrorResponse)
	}

	resellerID, scopeErr := peerScopeFromContext(ctx)
	if scopeErr != nil {
		return scopeErr
	}

	if err := c.accountService.UpdateAccountShareExpire(uint(id), req.ExpireTime, resellerID); err != nil {
		if errors.Is(err, gorm.ErrRecordNotFound) {
			return ctx.JSON(http.StatusNotFound, schema.ErrorResponse{StatusCode: http.StatusNotFound, Status: "error", Message: "user manager account not found"})
		}
		c.logger.Error("failed to update user manager account share expire time", zap.Error(err))
		return ctx.JSON(http.StatusInternalServerError, schema.ErrorResponse{
			StatusCode: http.StatusInternalServerError,
			Status:     "error",
			Message:    "failed to update share expire time: " + err.Error(),
		})
	}

	return ctx.JSON(http.StatusOK, schema.OkBasicResponse)
}

// UploadAccountConfig accepts an admin-supplied config file (typically an
// OpenVPN .ovpn profile -- RouterOS User Manager has no concept of one to
// generate) and attaches it to the account. Admin only, since only the
// admin-facing pages expose this action (both the admin's own account list
// and the "Reseller User Manager" on-behalf-of page); a reseller never
// uploads a config file for their own accounts.
func (c *UserManagerAccountController) UploadAccountConfig(ctx echo.Context) error {
	if role, _ := getRoleAndResellerFromContext(ctx); role != "admin" {
		return ctx.JSON(http.StatusForbidden, schema.ErrorResponse{StatusCode: http.StatusForbidden, Status: "error", Message: "forbidden"})
	}

	id, err := strconv.Atoi(ctx.Param("id"))
	if err != nil {
		return ctx.JSON(http.StatusBadRequest, schema.BadParamsErrorResponse)
	}

	fileHeader, err := ctx.FormFile("config")
	if err != nil {
		return ctx.JSON(http.StatusBadRequest, schema.ErrorResponse{
			StatusCode: http.StatusBadRequest,
			Status:     "error",
			Message:    "missing 'config' file in upload",
		})
	}

	// A generous but bounded cap -- config files are small text profiles,
	// this only guards against a runaway/malicious upload.
	const maxConfigUploadSize = 5 << 20 // 5 MiB
	if fileHeader.Size > maxConfigUploadSize {
		return ctx.JSON(http.StatusBadRequest, schema.ErrorResponse{
			StatusCode: http.StatusBadRequest,
			Status:     "error",
			Message:    "uploaded file exceeds the maximum allowed config size",
		})
	}

	src, err := fileHeader.Open()
	if err != nil {
		c.logger.Error("failed to open uploaded config file", zap.Error(err))
		return ctx.JSON(http.StatusInternalServerError, schema.InternalServerErrorResponse)
	}
	defer src.Close()

	content, err := io.ReadAll(src)
	if err != nil {
		c.logger.Error("failed to read uploaded config file", zap.Error(err))
		return ctx.JSON(http.StatusInternalServerError, schema.InternalServerErrorResponse)
	}

	if err := c.configFile.UploadConfig(uint(id), nil, content); err != nil {
		if errors.Is(err, gorm.ErrRecordNotFound) {
			return ctx.JSON(http.StatusNotFound, schema.ErrorResponse{StatusCode: http.StatusNotFound, Status: "error", Message: "user manager account not found"})
		}
		c.logger.Error("failed to upload user manager account config", zap.Error(err))
		return ctx.JSON(http.StatusInternalServerError, schema.ErrorResponse{
			StatusCode: http.StatusInternalServerError,
			Status:     "error",
			Message:    "failed to upload config: " + err.Error(),
		})
	}

	return ctx.JSON(http.StatusOK, schema.OkBasicResponse)
}

// UploadAccountConfigForReseller mirrors UploadAccountConfig exactly, for
// the admin "Reseller User Manager" on-behalf-of page.
func (c *UserManagerAccountController) UploadAccountConfigForReseller(ctx echo.Context) error {
	resellerID, err := adminResellerIDFromParam(ctx)
	if err != nil {
		return err
	}

	id, err := strconv.Atoi(ctx.Param("id"))
	if err != nil {
		return ctx.JSON(http.StatusBadRequest, schema.BadParamsErrorResponse)
	}

	fileHeader, err := ctx.FormFile("config")
	if err != nil {
		return ctx.JSON(http.StatusBadRequest, schema.ErrorResponse{
			StatusCode: http.StatusBadRequest,
			Status:     "error",
			Message:    "missing 'config' file in upload",
		})
	}

	const maxConfigUploadSize = 5 << 20 // 5 MiB
	if fileHeader.Size > maxConfigUploadSize {
		return ctx.JSON(http.StatusBadRequest, schema.ErrorResponse{
			StatusCode: http.StatusBadRequest,
			Status:     "error",
			Message:    "uploaded file exceeds the maximum allowed config size",
		})
	}

	src, err := fileHeader.Open()
	if err != nil {
		c.logger.Error("failed to open uploaded config file", zap.Error(err))
		return ctx.JSON(http.StatusInternalServerError, schema.InternalServerErrorResponse)
	}
	defer src.Close()

	content, err := io.ReadAll(src)
	if err != nil {
		c.logger.Error("failed to read uploaded config file", zap.Error(err))
		return ctx.JSON(http.StatusInternalServerError, schema.InternalServerErrorResponse)
	}

	if err := c.configFile.UploadConfig(uint(id), &resellerID, content); err != nil {
		if errors.Is(err, gorm.ErrRecordNotFound) {
			return ctx.JSON(http.StatusNotFound, schema.ErrorResponse{StatusCode: http.StatusNotFound, Status: "error", Message: "user manager account not found"})
		}
		c.logger.Error("failed to upload user manager account config for reseller", zap.Error(err))
		return ctx.JSON(http.StatusInternalServerError, schema.ErrorResponse{
			StatusCode: http.StatusInternalServerError,
			Status:     "error",
			Message:    "failed to upload config: " + err.Error(),
		})
	}

	return ctx.JSON(http.StatusOK, schema.OkBasicResponse)
}

// bulkImportMaxUploadSize bounds a bulk-import TXT/CSV upload -- generous
// for a plain-text username list (tens of thousands of lines easily fit
// well under this), but still a guard against a runaway/malicious upload.
const bulkImportMaxUploadSize = 5 << 20 // 5 MiB

// BulkImportAccounts syncs already-existing RouterOS User Manager accounts
// (created by an external script) into the panel's database from an
// uploaded TXT/CSV file -- see UserManagerService.BulkImportAccounts's doc
// comment for the full sync algorithm.
func (c *UserManagerAccountController) BulkImportAccounts(ctx echo.Context) error {
	resellerID, scopeErr := peerScopeFromContext(ctx)
	if scopeErr != nil {
		return scopeErr
	}

	fileHeader, err := ctx.FormFile("file")
	if err != nil {
		return ctx.JSON(http.StatusBadRequest, schema.ErrorResponse{
			StatusCode: http.StatusBadRequest,
			Status:     "error",
			Message:    "missing 'file' in upload",
		})
	}
	if fileHeader.Size > bulkImportMaxUploadSize {
		return ctx.JSON(http.StatusBadRequest, schema.ErrorResponse{
			StatusCode: http.StatusBadRequest,
			Status:     "error",
			Message:    "uploaded file exceeds the maximum allowed size",
		})
	}

	src, err := fileHeader.Open()
	if err != nil {
		c.logger.Error("failed to open uploaded bulk import file", zap.Error(err))
		return ctx.JSON(http.StatusInternalServerError, schema.InternalServerErrorResponse)
	}
	defer src.Close()

	content, err := io.ReadAll(src)
	if err != nil {
		c.logger.Error("failed to read uploaded bulk import file", zap.Error(err))
		return ctx.JSON(http.StatusInternalServerError, schema.InternalServerErrorResponse)
	}

	result, err := c.accountService.BulkImportAccounts(string(content), resellerID)
	if err != nil {
		c.logger.Error("bulk import failed", zap.Error(err))
		return ctx.JSON(http.StatusBadRequest, schema.ErrorResponse{
			StatusCode: http.StatusBadRequest,
			Status:     "error",
			Message:    "bulk import failed: " + err.Error(),
		})
	}

	return ctx.JSON(http.StatusOK, schema.BasicResponseData[schema.BulkImportAccountsResult]{
		BasicResponse: schema.OkBasicResponse,
		Data:          *result,
	})
}

// BulkImportAccountsForReseller mirrors BulkImportAccounts exactly, for the
// admin "Reseller User Manager" on-behalf-of page.
func (c *UserManagerAccountController) BulkImportAccountsForReseller(ctx echo.Context) error {
	resellerID, err := adminResellerIDFromParam(ctx)
	if err != nil {
		return err
	}

	fileHeader, err := ctx.FormFile("file")
	if err != nil {
		return ctx.JSON(http.StatusBadRequest, schema.ErrorResponse{
			StatusCode: http.StatusBadRequest,
			Status:     "error",
			Message:    "missing 'file' in upload",
		})
	}
	if fileHeader.Size > bulkImportMaxUploadSize {
		return ctx.JSON(http.StatusBadRequest, schema.ErrorResponse{
			StatusCode: http.StatusBadRequest,
			Status:     "error",
			Message:    "uploaded file exceeds the maximum allowed size",
		})
	}

	src, err := fileHeader.Open()
	if err != nil {
		c.logger.Error("failed to open uploaded bulk import file", zap.Error(err))
		return ctx.JSON(http.StatusInternalServerError, schema.InternalServerErrorResponse)
	}
	defer src.Close()

	content, err := io.ReadAll(src)
	if err != nil {
		c.logger.Error("failed to read uploaded bulk import file", zap.Error(err))
		return ctx.JSON(http.StatusInternalServerError, schema.InternalServerErrorResponse)
	}

	result, err := c.accountService.BulkImportAccounts(string(content), &resellerID)
	if err != nil {
		c.logger.Error("bulk import for reseller failed", zap.Error(err))
		return ctx.JSON(http.StatusBadRequest, schema.ErrorResponse{
			StatusCode: http.StatusBadRequest,
			Status:     "error",
			Message:    "bulk import failed: " + err.Error(),
		})
	}

	return ctx.JSON(http.StatusOK, schema.BasicResponseData[schema.BulkImportAccountsResult]{
		BasicResponse: schema.OkBasicResponse,
		Data:          *result,
	})
}

// DownloadAccountConfig lets the owning caller (admin, or the reseller who
// owns the account) download the previously-uploaded config file.
func (c *UserManagerAccountController) DownloadAccountConfig(ctx echo.Context) error {
	id, err := strconv.Atoi(ctx.Param("id"))
	if err != nil {
		return ctx.JSON(http.StatusBadRequest, schema.BadParamsErrorResponse)
	}

	resellerID, scopeErr := peerScopeFromContext(ctx)
	if scopeErr != nil {
		return scopeErr
	}

	path, err := c.configFile.GetConfigPath(uint(id), resellerID)
	if err != nil {
		if errors.Is(err, gorm.ErrRecordNotFound) {
			return ctx.JSON(http.StatusNotFound, schema.ErrorResponse{StatusCode: http.StatusNotFound, Status: "error", Message: "user manager account not found"})
		}
		return ctx.JSON(http.StatusNotFound, schema.ErrorResponse{
			StatusCode: http.StatusNotFound,
			Status:     "error",
			Message:    err.Error(),
		})
	}

	return attachFile(ctx, path, fmt.Sprintf("user-manager-%d.ovpn", id))
}

// GetSelfSummary returns the caller reseller's own User Manager online
// count and quota/usage, for the reseller dashboard. Reseller only,
// mirroring WgPeerController.GetSelfActivity's exact guard (admins have
// no single-reseller context to summarize).
func (c *UserManagerAccountController) GetSelfSummary(ctx echo.Context) error {
	role, resellerID := getRoleAndResellerFromContext(ctx)
	if role != "reseller" || resellerID == nil {
		return ctx.JSON(http.StatusForbidden, schema.ErrorResponse{StatusCode: http.StatusForbidden, Status: "error", Message: "forbidden"})
	}

	summary, err := c.accountService.GetSelfSummary(*resellerID)
	if err != nil {
		c.logger.Error("failed to get user manager self summary", zap.Error(err))
		return ctx.JSON(http.StatusInternalServerError, schema.ErrorResponse{
			StatusCode: http.StatusInternalServerError,
			Status:     "error",
			Message:    "failed to retrieve summary: " + err.Error(),
		})
	}

	return ctx.JSON(http.StatusOK, schema.BasicResponseData[schema.UserManagerSelfSummaryResponse]{
		BasicResponse: schema.OkBasicResponse,
		Data:          *summary,
	})
}
