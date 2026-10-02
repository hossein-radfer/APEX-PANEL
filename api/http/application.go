package http

import (
	"io"
	"net/http"
	"strconv"
	"time"

	"github.com/golang-jwt/jwt/v5"
	"github.com/labstack/echo-jwt/v4"
	"github.com/labstack/echo/v4"
	"go.uber.org/zap"

	"github.com/maahdima/mwp/api/http/schema"
	"github.com/maahdima/mwp/api/service"
)

// ApplicationController manages the "Applications" (اپلیکیشن) bundle
// feature -- structurally mirrors V2RayPackageController, reusing the
// same package-level errAlreadyHandled/getRoleAndResellerFromContext/
// peerScopeFromContext/adminResellerIDFromParam/forbidUnlessAdmin
// helpers, since they're role/claims-only, not Application-specific.
type ApplicationController struct {
	appService *service.ApplicationService
	logger     *zap.Logger
}

func NewApplicationController(appService *service.ApplicationService) *ApplicationController {
	return &ApplicationController{
		appService: appService,
		logger:     zap.L().Named("ApplicationController"),
	}
}

func (c *ApplicationController) internalError(ctx echo.Context, action string, err error) error {
	c.logger.Error("failed to "+action, zap.Error(err))
	return ctx.JSON(http.StatusInternalServerError, schema.ErrorResponse{
		StatusCode: http.StatusInternalServerError,
		Status:     "error",
		Message:    "failed to " + action + ": " + err.Error(),
	})
}

// ListApplications serves both admin (own, reseller_id IS NULL) and
// reseller (own reseller_id) callers via the same route -- mirrors
// V2RayPackageController.ListPackages exactly.
func (c *ApplicationController) ListApplications(ctx echo.Context) error {
	resellerID, err := peerScopeFromContext(ctx)
	if err != nil {
		return err
	}

	apps, err := c.appService.ListApplications(resellerID)
	if err != nil {
		return c.internalError(ctx, "list applications", err)
	}

	return ctx.JSON(http.StatusOK, schema.BasicResponseData[schema.ApplicationsResponse]{
		BasicResponse: schema.OkBasicResponse,
		Data:          schema.ApplicationsResponse{Applications: apps},
	})
}

// ListApplicationsByReseller returns all Applications owned by a specific
// reseller. Admin only.
func (c *ApplicationController) ListApplicationsByReseller(ctx echo.Context) error {
	resellerID, err := adminResellerIDFromParam(ctx)
	if err != nil {
		return err
	}

	apps, err := c.appService.GetApplicationsByReseller(resellerID)
	if err != nil {
		return c.internalError(ctx, "list reseller applications", err)
	}

	return ctx.JSON(http.StatusOK, schema.BasicResponseData[schema.ApplicationsResponse]{
		BasicResponse: schema.OkBasicResponse,
		Data:          schema.ApplicationsResponse{Applications: apps},
	})
}

func (c *ApplicationController) CreateApplication(ctx echo.Context) error {
	var req schema.CreateApplicationRequest
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

	app, err := c.appService.CreateApplication(&req, resellerID)
	if err != nil {
		return c.internalError(ctx, "create application", err)
	}

	return ctx.JSON(http.StatusCreated, schema.BasicResponseData[schema.ApplicationResponse]{
		BasicResponse: schema.OkBasicResponse,
		Data:          *app,
	})
}

// CreateApplicationForReseller lets an admin create an Application on
// behalf of a specific reseller -- mirrors CreatePackageForReseller.
func (c *ApplicationController) CreateApplicationForReseller(ctx echo.Context) error {
	resellerID, err := adminResellerIDFromParam(ctx)
	if err != nil {
		return err
	}

	var req schema.CreateApplicationRequest
	if err := ctx.Bind(&req); err != nil {
		c.logger.Warn("failed to bind request", zap.Error(err))
		return ctx.JSON(http.StatusBadRequest, schema.BadParamsErrorResponse)
	}
	if err := ctx.Validate(&req); err != nil {
		c.logger.Warn("failed to validate request", zap.Error(err))
		return ctx.JSON(http.StatusBadRequest, schema.BadParamsErrorResponse)
	}

	app, err := c.appService.CreateApplication(&req, &resellerID)
	if err != nil {
		return c.internalError(ctx, "create application for reseller", err)
	}

	return ctx.JSON(http.StatusCreated, schema.BasicResponseData[schema.ApplicationResponse]{
		BasicResponse: schema.OkBasicResponse,
		Data:          *app,
	})
}

func (c *ApplicationController) UpdateApplication(ctx echo.Context) error {
	id, err := strconv.ParseUint(ctx.Param("id"), 10, 32)
	if err != nil {
		return ctx.JSON(http.StatusBadRequest, schema.BadParamsErrorResponse)
	}

	var req schema.UpdateApplicationRequest
	if err := ctx.Bind(&req); err != nil {
		c.logger.Warn("failed to bind request", zap.Error(err))
		return ctx.JSON(http.StatusBadRequest, schema.BadParamsErrorResponse)
	}

	resellerID, err := peerScopeFromContext(ctx)
	if err != nil {
		return err
	}

	app, err := c.appService.UpdateApplication(uint(id), &req, resellerID)
	if err != nil {
		return c.internalError(ctx, "update application", err)
	}

	return ctx.JSON(http.StatusOK, schema.BasicResponseData[schema.ApplicationResponse]{
		BasicResponse: schema.OkBasicResponse,
		Data:          *app,
	})
}

// UpdateApplicationForReseller mirrors CreateApplicationForReseller's own
// admin-on-behalf-of-reseller pattern -- a confirmed, reported gap: the
// generic PUT /application/:id route resolves its scope via
// peerScopeFromContext, which returns nil (unscoped) for ANY admin
// session regardless of which reseller's Application the admin actually
// meant to edit, so an admin editing a reseller's Application through
// that route never had the reseller-ownership checks (ensureResellerCan
// UseInterface/UsePanel/ensureResellerCanUseGroup/UseProfile inside
// UpdateApplication's own reconcile* helpers) applied at the API layer
// at all -- only the frontend's own candidate-list filtering limited
// what got selected, a much weaker guarantee than every sibling
// resource (V2Ray packages, User Manager accounts, WireGuard peers)
// already has via their own dedicated .../reseller/:reseller_id/:id
// routes.
func (c *ApplicationController) UpdateApplicationForReseller(ctx echo.Context) error {
	resellerID, err := adminResellerIDFromParam(ctx)
	if err != nil {
		return err
	}

	id, err := strconv.ParseUint(ctx.Param("id"), 10, 32)
	if err != nil {
		return ctx.JSON(http.StatusBadRequest, schema.BadParamsErrorResponse)
	}

	var req schema.UpdateApplicationRequest
	if err := ctx.Bind(&req); err != nil {
		c.logger.Warn("failed to bind request", zap.Error(err))
		return ctx.JSON(http.StatusBadRequest, schema.BadParamsErrorResponse)
	}

	app, err := c.appService.UpdateApplication(uint(id), &req, &resellerID)
	if err != nil {
		return c.internalError(ctx, "update application for reseller", err)
	}

	return ctx.JSON(http.StatusOK, schema.BasicResponseData[schema.ApplicationResponse]{
		BasicResponse: schema.OkBasicResponse,
		Data:          *app,
	})
}

func (c *ApplicationController) DeleteApplication(ctx echo.Context) error {
	id, err := strconv.ParseUint(ctx.Param("id"), 10, 32)
	if err != nil {
		return ctx.JSON(http.StatusBadRequest, schema.BadParamsErrorResponse)
	}

	resellerID, err := peerScopeFromContext(ctx)
	if err != nil {
		return err
	}

	if err := c.appService.DeleteApplication(uint(id), resellerID); err != nil {
		return c.internalError(ctx, "delete application", err)
	}

	return ctx.JSON(http.StatusOK, schema.OkBasicResponse)
}

// --- Admin-only resource-location labels ---

func (c *ApplicationController) SetResourceLocation(ctx echo.Context) error {
	if forbidden, err := forbidUnlessAdmin(ctx); forbidden {
		return err
	}

	var req schema.SetApplicationResourceLocationRequest
	if err := ctx.Bind(&req); err != nil {
		return ctx.JSON(http.StatusBadRequest, schema.BadParamsErrorResponse)
	}
	if err := ctx.Validate(&req); err != nil {
		return ctx.JSON(http.StatusBadRequest, schema.BadParamsErrorResponse)
	}

	if err := c.appService.SetResourceLocation(&req); err != nil {
		return c.internalError(ctx, "set application resource location", err)
	}

	return ctx.JSON(http.StatusOK, schema.OkBasicResponse)
}

func (c *ApplicationController) ListResourceLocations(ctx echo.Context) error {
	if forbidden, err := forbidUnlessAdmin(ctx); forbidden {
		return err
	}

	locations, err := c.appService.ListResourceLocations()
	if err != nil {
		return c.internalError(ctx, "list application resource locations", err)
	}

	return ctx.JSON(http.StatusOK, schema.BasicResponseData[schema.ApplicationResourceLocationsResponse]{
		BasicResponse: schema.OkBasicResponse,
		Data:          schema.ApplicationResourceLocationsResponse{Locations: locations},
	})
}

// --- Admin-only global OpenVPN template ---

const maxOpenVpnTemplateUploadSize = 5 << 20 // 5 MiB, mirrors UserManagerConfigFile's own cap

// UploadOpenVpnTemplate stores the ONE global .ovpn template every
// Application's OpenVPN connect-config is built from -- see
// model.ApplicationOpenVpnTemplate's own doc comment. Admin only.
func (c *ApplicationController) UploadOpenVpnTemplate(ctx echo.Context) error {
	if forbidden, err := forbidUnlessAdmin(ctx); forbidden {
		return err
	}

	fileHeader, err := ctx.FormFile("template")
	if err != nil {
		return ctx.JSON(http.StatusBadRequest, schema.ErrorResponse{
			StatusCode: http.StatusBadRequest,
			Status:     "error",
			Message:    "missing 'template' file in upload",
		})
	}
	if fileHeader.Size > maxOpenVpnTemplateUploadSize {
		return ctx.JSON(http.StatusBadRequest, schema.ErrorResponse{
			StatusCode: http.StatusBadRequest,
			Status:     "error",
			Message:    "uploaded file exceeds the maximum allowed template size",
		})
	}

	src, err := fileHeader.Open()
	if err != nil {
		return c.internalError(ctx, "open uploaded openvpn template", err)
	}
	defer src.Close()

	content, err := io.ReadAll(src)
	if err != nil {
		return c.internalError(ctx, "read uploaded openvpn template", err)
	}

	if err := c.appService.UploadOpenVpnTemplate(content); err != nil {
		return c.internalError(ctx, "upload openvpn template", err)
	}

	return ctx.JSON(http.StatusOK, schema.OkBasicResponse)
}

// GetOpenVpnTemplateStatus backs the admin settings page's "is a
// template uploaded, and when" display. Admin only.
func (c *ApplicationController) GetOpenVpnTemplateStatus(ctx echo.Context) error {
	if forbidden, err := forbidUnlessAdmin(ctx); forbidden {
		return err
	}

	exists, uploadedAt, err := c.appService.GetOpenVpnTemplateStatus()
	if err != nil {
		return c.internalError(ctx, "get openvpn template status", err)
	}

	resp := schema.ApplicationOpenVpnTemplateStatusResponse{Exists: exists}
	if uploadedAt != nil {
		formatted := uploadedAt.Format(time.RFC3339)
		resp.UploadedAt = &formatted
	}

	return ctx.JSON(http.StatusOK, schema.BasicResponseData[schema.ApplicationOpenVpnTemplateStatusResponse]{
		BasicResponse: schema.OkBasicResponse,
		Data:          resp,
	})
}

// DownloadOpenVpnTemplate lets an admin re-download the currently
// uploaded template, to verify what was actually saved. Admin only.
func (c *ApplicationController) DownloadOpenVpnTemplate(ctx echo.Context) error {
	if forbidden, err := forbidUnlessAdmin(ctx); forbidden {
		return err
	}

	path, err := c.appService.GetOpenVpnTemplatePath()
	if err != nil {
		return ctx.JSON(http.StatusNotFound, schema.ErrorResponse{
			StatusCode: http.StatusNotFound,
			Status:     "error",
			Message:    err.Error(),
		})
	}

	return attachFile(ctx, path, "application-openvpn-template.ovpn")
}

// --- Mobile app's own public (non-admin/reseller-JWT) endpoints ---

// AppAuthController issues and verifies the mobile app's OWN JWTs --
// deliberately a separate signing secret from Authentication's
// AccessSecret/RefreshSecret, so an app-user session can never be replayed
// against an admin/reseller-only route (and vice versa) even though both
// use the same JWT library/shape.
type AppAuthController struct {
	appService     *service.ApplicationService
	versionService *service.ApplicationVersionService
	accessSecret   string
	logger         *zap.Logger
}

func NewAppAuthController(appService *service.ApplicationService, versionService *service.ApplicationVersionService, accessSecret string) *AppAuthController {
	return &AppAuthController{
		appService:     appService,
		versionService: versionService,
		accessSecret:   accessSecret,
		logger:         zap.L().Named("AppAuthController"),
	}
}

// AppJWTConfig returns the echojwt middleware config for the app-user
// route group, gated on this controller's own accessSecret (never the
// admin/reseller one) -- registered separately in api.go, mirroring how
// the admin/reseller jwtConfig is built and applied per-group there.
func (c *AppAuthController) AppJWTConfig() echojwt.Config {
	return echojwt.Config{SigningKey: []byte(c.accessSecret)}
}

// Login authenticates AppUsername/AppPassword and mints a token -- a
// public (unauthenticated) endpoint, registered directly on the router
// like setupV2RayPublicRoutes, never inside the admin/reseller echojwt
// group.
func (c *AppAuthController) Login(ctx echo.Context) error {
	var req schema.AppLoginRequest
	if err := ctx.Bind(&req); err != nil {
		return ctx.JSON(http.StatusBadRequest, schema.BadParamsErrorResponse)
	}
	if err := ctx.Validate(&req); err != nil {
		return ctx.JSON(http.StatusBadRequest, schema.BadParamsErrorResponse)
	}

	app, err := c.appService.AuthenticateApp(req.Username, req.Password)
	if err != nil {
		return ctx.JSON(http.StatusUnauthorized, schema.ErrorResponse{
			StatusCode: http.StatusUnauthorized,
			Status:     "error",
			Message:    "invalid username or password",
		})
	}

	claims := jwt.MapClaims{
		"sub":            app.AppUsername,
		"role":           "app_user",
		"application_id": app.ID,
		"exp":            time.Now().Add(30 * 24 * time.Hour).Unix(),
	}
	token := jwt.NewWithClaims(jwt.SigningMethodHS256, claims)
	signed, err := token.SignedString([]byte(c.accessSecret))
	if err != nil {
		return c.internalError(ctx, "sign app token", err)
	}

	c.appService.UpsertDeviceSession(app.ID, req.DeviceID, req.DeviceLabel)

	return ctx.JSON(http.StatusOK, schema.BasicResponseData[schema.AppLoginResponse]{
		BasicResponse: schema.OkBasicResponse,
		Data:          schema.AppLoginResponse{AccessToken: signed},
	})
}

// CheckVersion backs the public GET /api/app/version-check endpoint --
// the mobile app's own first network call on every launch, BEFORE
// login, so a user who cannot currently authenticate (or has never
// logged in at all) still finds out they must update or that the
// service is under maintenance. Unauthenticated by design, mirroring
// Login's own "registered directly on the router, never inside the
// echojwt group" placement.
func (c *AppAuthController) CheckVersion(ctx echo.Context) error {
	currentVersion, err := strconv.Atoi(ctx.QueryParam("current_version"))
	if err != nil {
		currentVersion = 0
	}

	latest, updateRequired, err := c.versionService.CheckVersion(currentVersion)
	if err != nil {
		return c.internalError(ctx, "check app version", err)
	}

	maintenanceEnabled, maintenanceMessage := c.versionService.GetMaintenanceMode()

	resp := schema.AppVersionCheckResponse{
		MaintenanceMode:    maintenanceEnabled,
		MaintenanceMessage: maintenanceMessage,
	}
	if latest != nil {
		resp.LatestVersionCode = latest.VersionCode
		resp.LatestVersionName = latest.VersionName
		resp.ReleaseNotes = latest.ReleaseNotes
		resp.DownloadURL = latest.DownloadURL
	}
	resp.UpdateRequired = updateRequired

	return ctx.JSON(http.StatusOK, schema.BasicResponseData[schema.AppVersionCheckResponse]{
		BasicResponse: schema.OkBasicResponse,
		Data:          resp,
	})
}

// deviceIDFromHeader reads the X-Device-Id header the mobile app sends
// on every authenticated request (see apiFetch's own doc comment on the
// JS side) -- a plain header rather than a JWT claim since the device ID
// is generated client-side AFTER install, independent of login/token
// issuance, and the same token is reused across app restarts on the same
// device without a new login.
func deviceIDFromHeader(ctx echo.Context) string {
	return ctx.Request().Header.Get("X-Device-Id")
}

func (c *AppAuthController) internalError(ctx echo.Context, action string, err error) error {
	c.logger.Error("failed to "+action, zap.Error(err))
	return ctx.JSON(http.StatusInternalServerError, schema.InternalServerErrorResponse)
}

// applicationIDFromAppToken reads the "application_id" claim off the
// app-user JWT that echojwt has already verified for this request --
// mirrors usernameFromContext's own "read out of ctx.Get(\"user\")"
// pattern (http/authentication.go), just against this controller's own
// distinct token shape.
func applicationIDFromAppToken(ctx echo.Context) (uint, error) {
	token, ok := ctx.Get("user").(*jwt.Token)
	if !ok {
		return 0, errAlreadyHandled
	}
	claims, ok := token.Claims.(jwt.MapClaims)
	if !ok {
		return 0, errAlreadyHandled
	}
	idFloat, ok := claims["application_id"].(float64)
	if !ok {
		return 0, errAlreadyHandled
	}
	return uint(idFloat), nil
}

// Me returns the authenticated app-user's own Application bundle -- the
// mobile app's account/quota/status screen.
func (c *AppAuthController) Me(ctx echo.Context) error {
	appID, err := applicationIDFromAppToken(ctx)
	if err != nil {
		return ctx.JSON(http.StatusUnauthorized, schema.ErrorResponse{StatusCode: http.StatusUnauthorized, Status: "error", Message: "unauthorized"})
	}

	app, err := c.appService.GetApplicationByID(appID)
	if err != nil {
		return c.internalError(ctx, "get application", err)
	}

	c.appService.TouchDeviceSession(appID, deviceIDFromHeader(ctx))

	return ctx.JSON(http.StatusOK, schema.BasicResponseData[schema.ApplicationResponse]{
		BasicResponse: schema.OkBasicResponse,
		Data:          *app,
	})
}

// Devices backs GET /api/app/devices -- see
// schema.AppDeviceSessionsResponse's own doc comment.
func (c *AppAuthController) Devices(ctx echo.Context) error {
	appID, err := applicationIDFromAppToken(ctx)
	if err != nil {
		return ctx.JSON(http.StatusUnauthorized, schema.ErrorResponse{StatusCode: http.StatusUnauthorized, Status: "error", Message: "unauthorized"})
	}

	devices, err := c.appService.ListDeviceSessions(appID, deviceIDFromHeader(ctx))
	if err != nil {
		return c.internalError(ctx, "list application device sessions", err)
	}

	return ctx.JSON(http.StatusOK, schema.BasicResponseData[schema.AppDeviceSessionsResponse]{
		BasicResponse: schema.OkBasicResponse,
		Data:          *devices,
	})
}

// RevokeDevice backs POST /api/app/devices/revoke -- see
// ApplicationService.RevokeDeviceSession's own doc comment on what this
// does and does not accomplish today.
func (c *AppAuthController) RevokeDevice(ctx echo.Context) error {
	appID, err := applicationIDFromAppToken(ctx)
	if err != nil {
		return ctx.JSON(http.StatusUnauthorized, schema.ErrorResponse{StatusCode: http.StatusUnauthorized, Status: "error", Message: "unauthorized"})
	}

	var req schema.AppRevokeDeviceRequest
	if err := ctx.Bind(&req); err != nil {
		return ctx.JSON(http.StatusBadRequest, schema.BadParamsErrorResponse)
	}
	if err := ctx.Validate(&req); err != nil {
		return ctx.JSON(http.StatusBadRequest, schema.BadParamsErrorResponse)
	}

	if err := c.appService.RevokeDeviceSession(appID, req.DeviceID); err != nil {
		return c.internalError(ctx, "revoke application device session", err)
	}

	return ctx.JSON(http.StatusOK, schema.OkBasicResponse)
}

// ReportDeviceResource backs POST /api/app/devices/report-resource -- see
// schema.AppReportDeviceResourceRequest's own doc comment for when the
// mobile app is expected to call this.
func (c *AppAuthController) ReportDeviceResource(ctx echo.Context) error {
	appID, err := applicationIDFromAppToken(ctx)
	if err != nil {
		return ctx.JSON(http.StatusUnauthorized, schema.ErrorResponse{StatusCode: http.StatusUnauthorized, Status: "error", Message: "unauthorized"})
	}

	var req schema.AppReportDeviceResourceRequest
	if err := ctx.Bind(&req); err != nil {
		return ctx.JSON(http.StatusBadRequest, schema.BadParamsErrorResponse)
	}
	if err := ctx.Validate(&req); err != nil {
		return ctx.JSON(http.StatusBadRequest, schema.BadParamsErrorResponse)
	}

	if err := c.appService.ReportDeviceResource(appID, req.DeviceID, req.ResourceType, req.ResourceID); err != nil {
		return c.internalError(ctx, "report application device resource", err)
	}

	return ctx.JSON(http.StatusOK, schema.OkBasicResponse)
}

// OnlineCount backs GET /api/app/online-count -- see
// schema.AppOnlineCountResponse's own doc comment.
func (c *AppAuthController) OnlineCount(ctx echo.Context) error {
	appID, err := applicationIDFromAppToken(ctx)
	if err != nil {
		return ctx.JSON(http.StatusUnauthorized, schema.ErrorResponse{StatusCode: http.StatusUnauthorized, Status: "error", Message: "unauthorized"})
	}

	count, err := c.appService.GetOnlineCount(appID)
	if err != nil {
		return c.internalError(ctx, "get application online count", err)
	}

	return ctx.JSON(http.StatusOK, schema.BasicResponseData[schema.AppOnlineCountResponse]{
		BasicResponse: schema.OkBasicResponse,
		Data:          *count,
	})
}

// ConnectConfigs backs GET /api/app/me/connect-configs -- see
// schema.AppConnectConfigsResponse's own doc comment. This is what the
// mobile app actually calls to get real, connectable WireGuard keys/
// User Manager credentials/V2Ray links, unlike Me (a summary-only view).
func (c *AppAuthController) ConnectConfigs(ctx echo.Context) error {
	appID, err := applicationIDFromAppToken(ctx)
	if err != nil {
		return ctx.JSON(http.StatusUnauthorized, schema.ErrorResponse{StatusCode: http.StatusUnauthorized, Status: "error", Message: "unauthorized"})
	}

	// Maintenance mode was previously advisory only -- CheckVersion (the
	// app's pre-login version-check call) surfaced it as a message, but
	// nothing actually stopped a device from logging in and receiving
	// real connect credentials while it was on, so toggling it in the
	// panel had no effect on whether users could keep connecting. Gating
	// it here (the actual credential hand-off) rather than at Login
	// keeps login itself working during maintenance -- the app can still
	// show the maintenance message from CheckVersion/me -- while genuinely
	// blocking new VPN connections, which is what an admin flipping this
	// switch actually expects to happen.
	if maintenanceEnabled, maintenanceMessage := c.versionService.GetMaintenanceMode(); maintenanceEnabled {
		message := "the service is currently under maintenance"
		if maintenanceMessage != nil && *maintenanceMessage != "" {
			message = *maintenanceMessage
		}
		return ctx.JSON(http.StatusServiceUnavailable, schema.ErrorResponse{
			StatusCode: http.StatusServiceUnavailable,
			Status:     "error",
			Message:    message,
		})
	}

	configs, err := c.appService.GetConnectConfigs(appID)
	if err != nil {
		return c.internalError(ctx, "get application connect configs", err)
	}

	return ctx.JSON(http.StatusOK, schema.BasicResponseData[schema.AppConnectConfigsResponse]{
		BasicResponse: schema.OkBasicResponse,
		Data:          *configs,
	})
}

// WeeklyUsage backs GET /api/app/me/weekly-usage -- see
// schema.AppWeeklyUsageResponse's own doc comment. Replaces the mobile
// app's own MOCK_WEEKLY placeholder with real per-day usage totals.
func (c *AppAuthController) WeeklyUsage(ctx echo.Context) error {
	appID, err := applicationIDFromAppToken(ctx)
	if err != nil {
		return ctx.JSON(http.StatusUnauthorized, schema.ErrorResponse{StatusCode: http.StatusUnauthorized, Status: "error", Message: "unauthorized"})
	}

	usage, err := c.appService.GetWeeklyUsage(appID)
	if err != nil {
		return c.internalError(ctx, "get application weekly usage", err)
	}

	return ctx.JSON(http.StatusOK, schema.BasicResponseData[schema.AppWeeklyUsageResponse]{
		BasicResponse: schema.OkBasicResponse,
		Data:          *usage,
	})
}
