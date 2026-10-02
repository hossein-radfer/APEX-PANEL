package http

import (
	"errors"
	"net/http"
	"time"

	"github.com/maahdima/mwp/api/http/schema"
	"github.com/maahdima/mwp/api/service"
	"github.com/maahdima/mwp/api/utils/timehelper"

	"github.com/golang-jwt/jwt/v5"
	"github.com/labstack/echo/v4"
	"go.uber.org/zap"
)

var errFailedToParseToken = errors.New("failed to parse token claims")

type AuthController struct {
	authService *service.Authentication
	auditLog    *service.AuditLog
	botNotifier *service.BotNotifier
	logger      *zap.Logger
}

func NewAuthController(authService *service.Authentication, auditLog *service.AuditLog, botNotifier *service.BotNotifier) *AuthController {
	return &AuthController{
		authService: authService,
		auditLog:    auditLog,
		botNotifier: botNotifier,
		logger:      zap.L().Named("AuthController"),
	}
}

func (a *AuthController) Login(ctx echo.Context) error {
	var req schema.LoginRequest

	if err := ctx.Bind(&req); err != nil {
		a.logger.Error("failed to bind login request", zap.Error(err))
		return ctx.JSON(http.StatusBadRequest, schema.BadParamsErrorResponse)
	}

	if err := ctx.Validate(&req); err != nil {
		a.logger.Error("validation failed for login request", zap.Error(err))
		return ctx.JSON(http.StatusBadRequest, schema.BadParamsErrorResponse)
	}

	admin, err := a.authService.Login(req.Username, req.Password, ctx.RealIP())
	if err != nil {
		a.logger.Error("failed to login", zap.Error(err))
		if errors.Is(err, service.ErrOtpBanned) {
			return ctx.JSON(http.StatusTooManyRequests, schema.ErrorResponse{
				StatusCode: http.StatusTooManyRequests,
				Status:     "error",
				Message:    "too many failed attempts, try again later",
			})
		}
		if a.botNotifier != nil {
			// Fire-and-forget: a best-effort admin alert must never hold
			// the login response hostage to Telegram's own reachability
			// -- see newTelegramBotAPI's own doc comment for the
			// confirmed production incident (the whole panel appearing
			// to hang, recoverable only by a restart) this exact
			// synchronous call caused when Telegram was unreachable.
			username, ip, loginTime := req.Username, ctx.RealIP(), timehelper.FormatTehran(time.Now())
			go a.botNotifier.NotifyFailedLogin(username, ip, loginTime)
		}
		return ctx.JSON(http.StatusNotFound, schema.ErrorResponse{
			StatusCode: http.StatusNotFound,
			Status:     "error",
			Message:    "failed to login: " + err.Error(),
		})
	}

	// An OTP-pending response carries no real identity/tokens yet (see
	// schema.LoginResponse's own doc comment) -- the login-alert
	// notification is deferred to VerifyOtp's own success path instead, so
	// the admin isn't alerted twice (once here, once after OTP) for a
	// single successful login.
	if admin.OtpRequired {
		return ctx.JSON(http.StatusOK, schema.BasicResponseData[schema.LoginResponse]{
			BasicResponse: schema.OkBasicResponse,
			Data:          *admin,
		})
	}

	if a.botNotifier != nil {
		loginTime := timehelper.FormatTehran(time.Now())
		ip := ctx.RealIP()
		if admin.Role == "reseller" && admin.ResellerID != nil {
			resellerID := *admin.ResellerID
			go a.botNotifier.NotifyLoginAlert(resellerID, ip, loginTime)
		} else {
			username := admin.Username
			go a.botNotifier.NotifyAdminLoginAlert(username, ip, loginTime)
		}
	}

	return ctx.JSON(http.StatusOK, schema.BasicResponseData[schema.LoginResponse]{
		BasicResponse: schema.OkBasicResponse,
		Data:          *admin,
	})
}

// VerifyOtp is the second step of the admin OTP login gate -- see
// service.Authentication.VerifyOtp's own doc comment. A successful
// verification here is exactly equivalent to a normal (non-OTP) login
// succeeding, so it gets the same login-alert notification Login's
// non-OTP path sends.
func (a *AuthController) VerifyOtp(ctx echo.Context) error {
	var req schema.VerifyOtpRequest
	if err := ctx.Bind(&req); err != nil {
		a.logger.Error("failed to bind otp verify request", zap.Error(err))
		return ctx.JSON(http.StatusBadRequest, schema.BadParamsErrorResponse)
	}
	if err := ctx.Validate(&req); err != nil {
		a.logger.Error("validation failed for otp verify request", zap.Error(err))
		return ctx.JSON(http.StatusBadRequest, schema.BadParamsErrorResponse)
	}

	admin, err := a.authService.VerifyOtp(req.OtpToken, req.Code, ctx.RealIP())
	if err != nil {
		if errors.Is(err, service.ErrOtpBanned) {
			return ctx.JSON(http.StatusTooManyRequests, schema.ErrorResponse{
				StatusCode: http.StatusTooManyRequests,
				Status:     "error",
				Message:    "too many failed attempts, try again later",
			})
		}
		return ctx.JSON(http.StatusUnauthorized, schema.ErrorResponse{
			StatusCode: http.StatusUnauthorized,
			Status:     "error",
			Message:    "invalid or expired code",
		})
	}

	// Mirrors Login's own role branch exactly -- a confirmed bug this fixes:
	// this always called NotifyAdminLoginAlert regardless of which account
	// actually completed OTP, so a reseller's successful OTP-gated login
	// (now possible since Reseller.OtpEnabled exists) sent its login alert
	// to the ADMIN's Telegram chat instead of the reseller's own.
	if a.botNotifier != nil {
		ip, loginTime := ctx.RealIP(), timehelper.FormatTehran(time.Now())
		if admin.Role == "reseller" && admin.ResellerID != nil {
			resellerID := *admin.ResellerID
			go a.botNotifier.NotifyLoginAlert(resellerID, ip, loginTime)
		} else {
			username := admin.Username
			go a.botNotifier.NotifyAdminLoginAlert(username, ip, loginTime)
		}
	}

	return ctx.JSON(http.StatusOK, schema.BasicResponseData[schema.LoginResponse]{
		BasicResponse: schema.OkBasicResponse,
		Data:          *admin,
	})
}

func (a *AuthController) UpdateProfile(ctx echo.Context) error {
	var req schema.UpdateProfileRequest
	if err := ctx.Bind(&req); err != nil {
		a.logger.Warn("failed to bind request", zap.Error(err))
		return ctx.JSON(http.StatusBadRequest, schema.BadParamsErrorResponse)
	}

	if err := ctx.Validate(&req); err != nil {
		a.logger.Warn("failed to validate request", zap.Error(err))
		return ctx.JSON(http.StatusBadRequest, schema.BadParamsErrorResponse)
	}

	// The profile being updated is always the caller's own account, identified by
	// the JWT subject claim, never a client-supplied username. Otherwise any
	// authenticated user who knows another account's password could rewrite
	// that account's credentials.
	callerUsername, err := usernameFromContext(ctx)
	if err != nil {
		a.logger.Warn("failed to resolve caller identity", zap.Error(err))
		return ctx.JSON(http.StatusUnauthorized, schema.ErrorResponse{
			StatusCode: http.StatusUnauthorized,
			Status:     "error",
			Message:    "unauthorized",
		})
	}

	if err := a.authService.UpdateProfile(callerUsername, req.OldPassword, req.NewUsername, req.NewPassword); err != nil {
		a.logger.Error("failed to update profile", zap.Error(err))
		return ctx.JSON(http.StatusInternalServerError, schema.ErrorResponse{
			StatusCode: http.StatusInternalServerError,
			Status:     "error",
			Message:    "failed to update profile: " + err.Error(),
		})
	}

	if role, resellerID := getRoleAndResellerFromContext(ctx); role == "reseller" && resellerID != nil {
		newName := callerUsername
		if req.NewUsername != nil {
			newName = *req.NewUsername
		}
		a.auditLog.Log(*resellerID, newName, service.AuditActionProfileUpdated, "Updated own profile (username/password)")
	}

	return ctx.JSON(http.StatusOK, schema.OkBasicResponse)
}

func usernameFromContext(ctx echo.Context) (string, error) {
	user := ctx.Get("user")
	token, ok := user.(*jwt.Token)
	if !ok {
		return "", errFailedToParseToken
	}
	claims, ok := token.Claims.(jwt.MapClaims)
	if !ok {
		return "", errFailedToParseToken
	}
	sub, ok := claims["sub"].(string)
	if !ok || sub == "" {
		return "", errFailedToParseToken
	}
	return sub, nil
}
