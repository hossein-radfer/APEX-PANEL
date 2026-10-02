package http

import (
	"errors"
	"net/http"

	"github.com/labstack/echo/v4"
	"go.uber.org/zap"
	"gorm.io/gorm"

	"github.com/maahdima/mwp/api/common"
	"github.com/maahdima/mwp/api/http/schema"
	"github.com/maahdima/mwp/api/service"
)

// DNSPublicController serves the two unauthenticated DNS endpoints: the
// share-page details (JSON) and the "register my IP" action -- mirrors
// V2RayPublicController's role, adapted for DNS's own customer action
// (registering an address) instead of a subscription file.
type DNSPublicController struct {
	accountService *service.DNSAccountService
	logger         *zap.Logger
}

func NewDNSPublicController(accountService *service.DNSAccountService) *DNSPublicController {
	return &DNSPublicController{
		accountService: accountService,
		logger:         zap.L().Named("DNSPublicController"),
	}
}

func (c *DNSPublicController) GetAccountShareDetails(ctx echo.Context) error {
	uuid := ctx.Param("uuid")
	if uuid == "" {
		return ctx.JSON(http.StatusBadRequest, schema.BadParamsErrorResponse)
	}

	details, err := c.accountService.GetAccountShareDetails(uuid)
	if err != nil {
		if errors.Is(err, gorm.ErrRecordNotFound) || errors.Is(err, common.ErrDNSAccountNotShared) {
			return ctx.JSON(http.StatusNotFound, schema.ErrorResponse{StatusCode: http.StatusNotFound, Status: "error", Message: "dns account not found"})
		}
		c.logger.Error("failed to retrieve DNS account share details", zap.Error(err))
		return ctx.JSON(http.StatusInternalServerError, schema.InternalServerErrorResponse)
	}

	return ctx.JSON(http.StatusOK, schema.BasicResponseData[schema.DNSAccountShareDetailsResponse]{
		BasicResponse: schema.OkBasicResponse,
		Data:          *details,
	})
}

// RegisterIP registers the CALLER'S OWN request IP (ctx.RealIP(), never a
// client-supplied field in the body) against the account named by uuid.
// Deliberately never trusts a body-supplied IP: this endpoint is reached by
// UUID alone with no session/login in front of it (unlike doctor-dns's own
// do_user_claim, which trusts a body IP only because it sits behind an
// authenticated panel_sessions token) -- accepting an arbitrary IP here
// would let anyone holding a share link register ANY address on ANY
// account.
func (c *DNSPublicController) RegisterIP(ctx echo.Context) error {
	uuid := ctx.Param("uuid")
	if uuid == "" {
		return ctx.JSON(http.StatusBadRequest, schema.BadParamsErrorResponse)
	}

	result, err := c.accountService.RegisterCustomerIP(uuid, ctx.RealIP())
	if err != nil {
		if errors.Is(err, gorm.ErrRecordNotFound) || errors.Is(err, common.ErrDNSAccountNotShared) {
			return ctx.JSON(http.StatusNotFound, schema.ErrorResponse{StatusCode: http.StatusNotFound, Status: "error", Message: "dns account not found"})
		}
		c.logger.Error("failed to register DNS customer IP", zap.Error(err))
		return ctx.JSON(http.StatusInternalServerError, schema.InternalServerErrorResponse)
	}

	return ctx.JSON(http.StatusOK, schema.BasicResponseData[schema.RegisterDNSIPResponse]{
		BasicResponse: schema.OkBasicResponse,
		Data:          *result,
	})
}
