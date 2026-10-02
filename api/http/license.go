package http

import (
	"net/http"

	"github.com/maahdima/mwp/api/http/schema"
	"github.com/maahdima/mwp/api/service"

	"github.com/labstack/echo/v4"
	"go.uber.org/zap"
)

// LicenseController exposes this install's own license status (for the
// admin UI's warning banner), a manual "check for updates" action, and the
// license-activation endpoint. Admin-only: resellers never see or need this.
type LicenseController struct {
	licenseService *service.LicenseService
	authService    *service.Authentication
	logger         *zap.Logger
}

func NewLicenseController(licenseService *service.LicenseService, authService *service.Authentication) *LicenseController {
	return &LicenseController{
		licenseService: licenseService,
		authService:    authService,
		logger:         zap.L().Named("LicenseController"),
	}
}

func (c *LicenseController) requireAdmin(ctx echo.Context) bool {
	role, _ := getRoleAndResellerFromContext(ctx)
	if role != "admin" {
		_ = ctx.JSON(http.StatusForbidden, schema.ErrorResponse{StatusCode: http.StatusForbidden, Status: "error", Message: "forbidden"})
		return false
	}
	return true
}

func (c *LicenseController) GetStatus(ctx echo.Context) error {
	if !c.requireAdmin(ctx) {
		return nil
	}

	status := c.licenseService.Status()
	return ctx.JSON(http.StatusOK, schema.BasicResponseData[schema.LicenseStatusResponse]{
		BasicResponse: schema.OkBasicResponse,
		Data: schema.LicenseStatusResponse{
			Activated:     status.Activated,
			Valid:         status.Valid,
			Reason:        status.Reason,
			PlanName:      status.PlanName,
			ExpiresAt:     status.ExpiresAt,
			ServerCount:   status.ServerCount,
			MaxServers:    status.MaxServers,
			LastCheckedAt: status.LastCheckedAt.UTC().Format("2006-01-02T15:04:05Z07:00"),
			InGracePeriod: status.InGracePeriod,
		},
	})
}

// PublicStatus is the unauthenticated counterpart to GetStatus, used by the
// frontend's root-level lockdown guard (see ui/src/routes/__root.tsx) to
// decide whether to force the license-activation screen BEFORE the admin
// has even logged in -- GetStatus can't serve this purpose since it
// requires a JWT, and a never-activated or blocked install has no way to
// ever obtain one. Deliberately NOT exempted from LicenseMiddleware (unlike
// /license/activate): if the install is blocked, this endpoint itself gets
// intercepted and returns LicenseMiddleware's own 403 first, which the
// frontend treats identically to an unlicensed status. Safe to expose
// without auth -- see PublicLicenseStatusResponse's doc comment on what it
// deliberately omits.
func (c *LicenseController) PublicStatus(ctx echo.Context) error {
	status := c.licenseService.Status()
	return ctx.JSON(http.StatusOK, schema.BasicResponseData[schema.PublicLicenseStatusResponse]{
		BasicResponse: schema.OkBasicResponse,
		Data: schema.PublicLicenseStatusResponse{
			Activated:     status.Activated,
			Valid:         status.Valid,
			InGracePeriod: status.InGracePeriod,
			Reason:        status.Reason,
		},
	})
}

// Activate submits a license key to license-panel and, on success, persists
// it so future restarts heartbeat instead of re-activating.
//
// Deliberately NOT behind JWT auth (see setupLicenseRoutes/api.go): while an
// install is unlicensed, LicenseMiddleware blocks the entire API including
// login, so there is no way for an admin to ever obtain a JWT in the first
// place. This is the one path exempted from that block (see
// middleware.licenseMiddlewareExemptPaths). Since it's reachable without a
// session, the request itself must carry and verify admin credentials
// (matching AuthController.Login's own admin-only check) so a license
// binding can't be hijacked by an unauthenticated caller who merely knows
// the panel's URL.
func (c *LicenseController) Activate(ctx echo.Context) error {
	var req schema.ActivateLicenseRequest
	if err := ctx.Bind(&req); err != nil {
		return ctx.JSON(http.StatusBadRequest, schema.BadParamsErrorResponse)
	}
	if err := ctx.Validate(&req); err != nil {
		return ctx.JSON(http.StatusBadRequest, schema.BadParamsErrorResponse)
	}

	admin, err := c.authService.Login(req.Username, req.Password, ctx.RealIP())
	if err != nil || admin.OtpRequired || admin.Role != "admin" {
		return ctx.JSON(http.StatusUnauthorized, schema.ErrorResponse{
			StatusCode: http.StatusUnauthorized,
			Status:     "error",
			Message:    "invalid admin credentials",
		})
	}

	status, err := c.licenseService.ActivateWithKey(req.LicenseKey)
	if err != nil {
		c.logger.Warn("license activation failed", zap.Error(err))
		return ctx.JSON(http.StatusBadRequest, schema.ErrorResponse{
			StatusCode: http.StatusBadRequest,
			Status:     "error",
			Message:    "license activation failed: " + err.Error(),
		})
	}

	return ctx.JSON(http.StatusOK, schema.BasicResponseData[schema.LicenseStatusResponse]{
		BasicResponse: schema.OkBasicResponse,
		Data: schema.LicenseStatusResponse{
			Activated:     status.Activated,
			Valid:         status.Valid,
			Reason:        status.Reason,
			PlanName:      status.PlanName,
			ExpiresAt:     status.ExpiresAt,
			ServerCount:   status.ServerCount,
			MaxServers:    status.MaxServers,
			LastCheckedAt: status.LastCheckedAt.UTC().Format("2006-01-02T15:04:05Z07:00"),
			InGracePeriod: status.InGracePeriod,
		},
	})
}

// Trial submits a self-service free-trial claim to license-panel and, on
// success, persists the license key it hands back so future restarts
// heartbeat instead of re-claiming a trial. Same no-JWT exemption and
// admin-credential-in-body pattern as Activate above, for the identical
// reason (see that handler's doc comment) -- plus an Email field the
// self-service trial flow needs that a purchased-key activation doesn't.
func (c *LicenseController) Trial(ctx echo.Context) error {
	var req schema.StartTrialRequest
	if err := ctx.Bind(&req); err != nil {
		return ctx.JSON(http.StatusBadRequest, schema.BadParamsErrorResponse)
	}
	if err := ctx.Validate(&req); err != nil {
		return ctx.JSON(http.StatusBadRequest, schema.BadParamsErrorResponse)
	}

	admin, err := c.authService.Login(req.Username, req.Password, ctx.RealIP())
	if err != nil || admin.OtpRequired || admin.Role != "admin" {
		return ctx.JSON(http.StatusUnauthorized, schema.ErrorResponse{
			StatusCode: http.StatusUnauthorized,
			Status:     "error",
			Message:    "invalid admin credentials",
		})
	}

	status, err := c.licenseService.StartTrial(req.Email)
	if err != nil {
		c.logger.Warn("free trial claim failed", zap.Error(err))
		return ctx.JSON(http.StatusBadRequest, schema.ErrorResponse{
			StatusCode: http.StatusBadRequest,
			Status:     "error",
			Message:    "free trial could not be started: " + err.Error(),
		})
	}

	return ctx.JSON(http.StatusOK, schema.BasicResponseData[schema.LicenseStatusResponse]{
		BasicResponse: schema.OkBasicResponse,
		Data: schema.LicenseStatusResponse{
			Activated:     status.Activated,
			Valid:         status.Valid,
			Reason:        status.Reason,
			PlanName:      status.PlanName,
			ExpiresAt:     status.ExpiresAt,
			ServerCount:   status.ServerCount,
			MaxServers:    status.MaxServers,
			LastCheckedAt: status.LastCheckedAt.UTC().Format("2006-01-02T15:04:05Z07:00"),
			InGracePeriod: status.InGracePeriod,
		},
	})
}

// Revoke clears this install's locally-stored license key so the
// activation screen reappears -- e.g. before repurposing this server for
// a different customer's license, or moving to a new key. Unlike Activate/
// Trial, this IS behind JWT auth: an already-activated, working install
// always has a way to obtain a session first, so there's no bootstrapping
// problem to work around here, and requiring a session (rather than
// re-submitting admin credentials in the body) is the safer default for a
// destructive action reachable only from inside the authenticated Settings
// page.
func (c *LicenseController) Revoke(ctx echo.Context) error {
	if !c.requireAdmin(ctx) {
		return nil
	}

	if err := c.licenseService.Revoke(); err != nil {
		c.logger.Error("license revoke failed", zap.Error(err))
		return ctx.JSON(http.StatusInternalServerError, schema.ErrorResponse{
			StatusCode: http.StatusInternalServerError,
			Status:     "error",
			Message:    "failed to remove license: " + err.Error(),
		})
	}

	status := c.licenseService.Status()
	return ctx.JSON(http.StatusOK, schema.BasicResponseData[schema.LicenseStatusResponse]{
		BasicResponse: schema.OkBasicResponse,
		Data: schema.LicenseStatusResponse{
			Activated:     status.Activated,
			Valid:         status.Valid,
			Reason:        status.Reason,
			PlanName:      status.PlanName,
			ExpiresAt:     status.ExpiresAt,
			ServerCount:   status.ServerCount,
			MaxServers:    status.MaxServers,
			LastCheckedAt: status.LastCheckedAt.UTC().Format("2006-01-02T15:04:05Z07:00"),
			InGracePeriod: status.InGracePeriod,
		},
	})
}

func (c *LicenseController) CheckUpdate(ctx echo.Context) error {
	if !c.requireAdmin(ctx) {
		return nil
	}

	channel := ctx.QueryParam("channel")
	if channel == "" {
		channel = "stable"
	}

	resp, err := c.licenseService.CheckForUpdate(channel)
	if err != nil {
		c.logger.Error("update check failed", zap.Error(err))
		return ctx.JSON(http.StatusInternalServerError, schema.ErrorResponse{
			StatusCode: http.StatusInternalServerError,
			Status:     "error",
			Message:    "failed to check for updates: " + err.Error(),
		})
	}

	return ctx.JSON(http.StatusOK, schema.BasicResponseData[schema.UpdateCheckResponse]{
		BasicResponse: schema.OkBasicResponse,
		Data: schema.UpdateCheckResponse{
			UpdateAvailable: resp.UpdateAvailable,
			Version:         resp.Version,
			ChangeLog:       resp.ChangeLog,
			ForceUpdate:     resp.ForceUpdate,
			CurrentVersion:  service.MwpVersion(),
		},
	})
}
