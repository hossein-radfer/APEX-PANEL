package middleware

import (
	"net/http"

	"github.com/labstack/echo/v4"

	"github.com/maahdima/mwp/api/http/schema"
	"github.com/maahdima/mwp/api/service"
)

// apiKeyHeader is a dedicated header (not the JWT/support-token
// "Authorization: Bearer" convention) so an external client's request is
// visually and structurally distinct from an admin-JWT or support-token
// request at a glance -- these are three genuinely separate credential
// types (see model.ApiKey's own doc comment), not interchangeable bearer
// tokens for the same realm.
const apiKeyHeader = "X-Api-Key"

// APIKeyMiddleware gates phase 4-3's external API surface (see
// api.go's setupExternalApiRoutes) on a valid, non-revoked model.ApiKey --
// the JWT-independent credential external panels/automation clients use
// instead of an admin login. Mirrors ClientConnectionMiddleware's and
// LicenseMiddleware's existing shape (a factory closing over its one
// service dependency) rather than echojwt.WithConfig, since this is an
// opaque bearer credential, not a JWT.
func APIKeyMiddleware(apiKeyService *service.ApiKeyService) echo.MiddlewareFunc {
	return func(next echo.HandlerFunc) echo.HandlerFunc {
		return func(c echo.Context) error {
			key := c.Request().Header.Get(apiKeyHeader)
			if key == "" {
				return c.JSON(http.StatusUnauthorized, schema.ErrorResponse{
					StatusCode: http.StatusUnauthorized,
					Status:     "error",
					Message:    "missing " + apiKeyHeader + " header",
				})
			}

			if err := apiKeyService.Verify(key); err != nil {
				return c.JSON(http.StatusUnauthorized, schema.ErrorResponse{
					StatusCode: http.StatusUnauthorized,
					Status:     "error",
					Message:    "invalid or revoked api key",
				})
			}

			return next(c)
		}
	}
}
