package http

import (
	"net/http"
	"strconv"

	"github.com/maahdima/mwp/api/http/schema"
	"github.com/maahdima/mwp/api/service"

	"github.com/labstack/echo/v4"
	"go.uber.org/zap"
)

// parseOptionalServerIDParam reads an optional "server_id" query param --
// nil (not present / not a valid uint) lets the service fall back to this
// install's one server (see UserManagerService.ListGroups/ListProfiles's
// own doc comment). The UI starts sending this explicitly once more than
// one server is registered (see this codebase's multi-server plan).
func parseOptionalServerIDParam(ctx echo.Context) *uint {
	raw := ctx.QueryParam("server_id")
	if raw == "" {
		return nil
	}
	parsed, err := strconv.ParseUint(raw, 10, 64)
	if err != nil {
		return nil
	}
	id := uint(parsed)
	return &id
}

// UserManagerGroupProfileController exposes read-only RouterOS User Manager
// group/profile listings -- per the "list-only" design decision, creating
// or deleting groups/profiles happens in Winbox, not this panel.
type UserManagerGroupProfileController struct {
	accountService *service.UserManagerService
	logger         *zap.Logger
}

func NewUserManagerGroupProfileController(accountService *service.UserManagerService) *UserManagerGroupProfileController {
	return &UserManagerGroupProfileController{
		accountService: accountService,
		logger:         zap.L().Named("UserManagerGroupProfileController"),
	}
}

func (c *UserManagerGroupProfileController) ListGroups(ctx echo.Context) error {
	groups, err := c.accountService.ListGroups(parseOptionalServerIDParam(ctx))
	if err != nil {
		c.logger.Error("failed to list user manager groups", zap.Error(err))
		return ctx.JSON(http.StatusInternalServerError, schema.ErrorResponse{
			StatusCode: http.StatusInternalServerError,
			Status:     "error",
			Message:    "failed to retrieve user manager groups: " + err.Error(),
		})
	}

	return ctx.JSON(http.StatusOK, schema.BasicResponseData[[]schema.UserManagerGroupResponse]{
		BasicResponse: schema.OkBasicResponse,
		Data:          groups,
	})
}

func (c *UserManagerGroupProfileController) ListProfiles(ctx echo.Context) error {
	profiles, err := c.accountService.ListProfiles(parseOptionalServerIDParam(ctx))
	if err != nil {
		c.logger.Error("failed to list user manager profiles", zap.Error(err))
		return ctx.JSON(http.StatusInternalServerError, schema.ErrorResponse{
			StatusCode: http.StatusInternalServerError,
			Status:     "error",
			Message:    "failed to retrieve user manager profiles: " + err.Error(),
		})
	}

	return ctx.JSON(http.StatusOK, schema.BasicResponseData[[]schema.UserManagerProfileResponse]{
		BasicResponse: schema.OkBasicResponse,
		Data:          profiles,
	})
}
