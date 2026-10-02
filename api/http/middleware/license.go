package middleware

import (
	"net/http"

	"github.com/labstack/echo/v4"

	"github.com/maahdima/mwp/api/service"
)

// licenseMiddlewareExemptPaths lists the routes that stay reachable while
// an install is blocked as unlicensed: the license-activation and
// free-trial endpoints (so a locked-out admin can still activate), the
// read-only support-log endpoint (gated by its own independent support
// token), and the public end-user share-link routes under
// /api/user/:uuid/... (see setupUserRoutes in api.go), which are a
// different customer's own concern, not this install admin's licensing
// state. Entries are route patterns as echo.Context.Path() returns them,
// not concrete UUIDs, so one entry covers every peer/account.
var licenseMiddlewareExemptPaths = map[string]bool{
	"/api/license/activate": true,
	"/api/license/trial":    true,
	"/api/support/logs":     true,

	"/api/user/:uuid/config":                                    true,
	"/api/user/:uuid/qrcode":                                    true,
	"/api/user/:uuid/details":                                   true,
	"/api/user/:uuid/user-manager-account":                      true,
	"/api/user/:uuid/user-manager-account/config":               true,
	"/api/user/:uuid/user-manager-account/protocol-certificate": true,
	"/api/user/:uuid/user-manager-account/protocol-client-app":  true,
}

// licenseBlockedResponse is a distinct shape (not schema.ErrorResponse)
// specifically so the frontend can reliably tell "blocked because
// unlicensed" apart from every other 403 in the panel (e.g. a reseller
// hitting an admin-only route) by checking error_code, rather than
// string-matching the human-readable message -- see
// ui/src/api/axios-instance.ts's response interceptor and the license
// activation page it redirects to.
type licenseBlockedResponse struct {
	StatusCode int    `json:"statusCode"`
	Status     string `json:"status"`
	Message    string `json:"message"`
	ErrorCode  string `json:"error_code"`
}

const LicenseBlockedErrorCode = "license_invalid"

// LicenseMiddleware blocks all API traffic once service.LicenseService
// reports the install as unlicensed -- see LicenseService.IsBlocking for
// the exact fail-open/fail-closed rules (a temporary license-server
// outage does NOT block requests; a confirmed revoked/expired/never-
// activated license does). Registered globally in api.go, ahead of
// auth/JWT middleware, so an expired license blocks even login attempts
// consistently rather than leaving some routes reachable and others not --
// except licenseMiddlewareExemptPaths, which must always stay reachable so
// the panel can be activated/re-activated in the first place.
func LicenseMiddleware(licenseService *service.LicenseService) echo.MiddlewareFunc {
	return func(next echo.HandlerFunc) echo.HandlerFunc {
		return func(c echo.Context) error {
			if licenseMiddlewareExemptPaths[c.Path()] {
				return next(c)
			}
			if licenseService.IsBlocking() {
				status := licenseService.Status()
				return c.JSON(http.StatusForbidden, licenseBlockedResponse{
					StatusCode: http.StatusForbidden,
					Status:     "error",
					Message:    "This installation's license is not valid: " + status.Reason,
					ErrorCode:  LicenseBlockedErrorCode,
				})
			}
			return next(c)
		}
	}
}
