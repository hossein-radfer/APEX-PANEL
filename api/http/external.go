package http

import (
	"net/http"

	"github.com/labstack/echo/v4"

	"github.com/maahdima/mwp/api/http/schema"
)

// ExternalController holds phase 4-3's external, API-key-gated endpoints --
// see api.go's setupApiKeyRoutes for how this differs from every other
// controller in this package (no JWT, no admin/reseller role, gated
// purely by middleware.APIKeyMiddleware). Kept as its own controller
// (rather than folding Ping into an existing one) so this external surface
// stays easy to find and grow as a single, deliberately reviewed set of
// endpoints, distinct from the admin-facing API.
type ExternalController struct{}

func NewExternalController() *ExternalController {
	return &ExternalController{}
}

// Ping lets an external client confirm its api key is valid with no side
// effects -- the first, minimal endpoint on this surface.
func (c *ExternalController) Ping(ctx echo.Context) error {
	return ctx.JSON(http.StatusOK, schema.OkBasicResponse)
}
