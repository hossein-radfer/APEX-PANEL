package http

import (
	"errors"
	"fmt"
	"net/http"
	"strings"

	"github.com/labstack/echo/v4"
	"go.uber.org/zap"
	"gorm.io/gorm"

	"github.com/maahdima/mwp/api/common"
	"github.com/maahdima/mwp/api/http/schema"
	"github.com/maahdima/mwp/api/service"
)

// V2RayPublicController serves the two unauthenticated V2Ray endpoints: the
// share-page details (JSON) and the raw combined subscription content
// (text/plain, what customers' V2Ray apps actually fetch). Both are pure
// DB reads -- see V2RayPackageService.GetPackageShareDetails/
// GetCombinedSubscription's doc comments for why neither ever calls any
// x-ui server live.
type V2RayPublicController struct {
	packageService *service.V2RayPackageService
	logger         *zap.Logger
}

func NewV2RayPublicController(packageService *service.V2RayPackageService) *V2RayPublicController {
	return &V2RayPublicController{
		packageService: packageService,
		logger:         zap.L().Named("V2RayPublicController"),
	}
}

func (c *V2RayPublicController) GetPackageShareDetails(ctx echo.Context) error {
	uuid := ctx.Param("uuid")
	if uuid == "" {
		return ctx.JSON(http.StatusBadRequest, schema.BadParamsErrorResponse)
	}

	scheme := "http"
	if ctx.Request().TLS != nil {
		scheme = "https"
	}
	publicBaseURL := scheme + "://" + ctx.Request().Host

	details, err := c.packageService.GetPackageShareDetails(uuid, publicBaseURL)
	if err != nil {
		if errors.Is(err, gorm.ErrRecordNotFound) || errors.Is(err, common.ErrV2RayPackageNotShared) {
			return ctx.JSON(http.StatusNotFound, schema.ErrorResponse{StatusCode: http.StatusNotFound, Status: "error", Message: "v2ray package not found"})
		}
		c.logger.Error("failed to retrieve v2ray package share details", zap.Error(err))
		return ctx.JSON(http.StatusInternalServerError, schema.InternalServerErrorResponse)
	}

	return ctx.JSON(http.StatusOK, schema.BasicResponseData[schema.V2RayPackageShareDetailsResponse]{
		BasicResponse: schema.OkBasicResponse,
		Data:          *details,
	})
}

// v2raySubUserAgentTokens are substrings of the User-Agent header actual
// V2Ray client apps are confirmed to send when fetching a subscription URL
// programmatically -- matched case-insensitively. A request whose
// User-Agent contains none of these (i.e. an ordinary web browser, per its
// own Mozilla/Chrome/Safari/Firefox-shaped User-Agent, opened this link
// directly) is redirected to the human-facing subscription landing page
// instead of receiving the raw base64 body -- a confirmed, reported bug:
// opening this URL directly in a browser showed a blank black page full
// of raw encoded text, which is confusing/unusable for a customer who
// doesn't have a V2Ray app installed and just wants to see their account
// info or copy a link by hand.
var v2raySubUserAgentTokens = []string{
	"v2ray", "v2rayng", "v2box", "v2raya", "shadowrocket", "clash",
	"quantumult", "surge", "stash", "sing-box", "singbox", "nekoray",
	"nekobox", "hiddify", "streisand", "loon", "karing", "flclash",
}

// isV2RayClientRequest reports whether userAgent looks like a V2Ray/proxy
// client app fetching this URL programmatically, per
// v2raySubUserAgentTokens. An EMPTY User-Agent is also treated as a client
// app, not a browser -- real browsers always send a non-empty User-Agent,
// so an empty one is far more likely to be a script/app that omitted it
// than a browser, and defaulting to "serve raw" here is the safer failure
// mode (an app that fails to parse HTML is a broken subscription; a human
// who sees raw text instead of the pretty page is merely inconvenienced).
func isV2RayClientRequest(userAgent string) bool {
	if userAgent == "" {
		return true
	}
	lower := strings.ToLower(userAgent)
	for _, token := range v2raySubUserAgentTokens {
		if strings.Contains(lower, token) {
			return true
		}
	}
	return false
}

// GetSubscription serves the combined base64 subscription content at
// /api/v2ray-sub/:uuid -- deliberately outside the JSON envelope
// convention every other endpoint uses, since V2Ray client apps (v2rayN,
// v2box, Shadowrocket, etc.) expect a bare text/plain body at the
// subscription URL, not a wrapped JSON response. It also sets the
// Subscription-Userinfo/Profile-Update-Interval/Profile-Title headers most
// of those apps expect -- several treat a subscription response missing
// Subscription-Userinfo as invalid/unparsable and show an error even when
// the base64 body itself decodes to perfectly valid config links, which
// is the likely cause of a "v2box gives an error" report even after the
// body content itself was confirmed correct.
//
// A plain web browser opening this SAME URL directly (identified by
// User-Agent, see isV2RayClientRequest) is instead redirected to
// /v2ray-sub?shareId=:uuid, the human-facing subscription landing page --
// this keeps the one URL customers actually copy/paste/scan working
// correctly for both audiences, rather than needing two different links.
func (c *V2RayPublicController) GetSubscription(ctx echo.Context) error {
	uuid := ctx.Param("uuid")
	if uuid == "" {
		return ctx.String(http.StatusBadRequest, "")
	}

	if !isV2RayClientRequest(ctx.Request().UserAgent()) {
		return ctx.Redirect(http.StatusFound, "/v2ray-sub?shareId="+uuid)
	}

	content, err := c.packageService.GetCombinedSubscription(uuid)
	if err != nil {
		if errors.Is(err, gorm.ErrRecordNotFound) || errors.Is(err, common.ErrV2RayPackageNotShared) {
			return ctx.String(http.StatusNotFound, "")
		}
		c.logger.Error("failed to retrieve v2ray subscription content", zap.Error(err))
		return ctx.String(http.StatusInternalServerError, "")
	}

	expireEpoch := int64(0)
	if content.ExpireAt != nil {
		expireEpoch = content.ExpireAt.Unix()
	}
	ctx.Response().Header().Set("Subscription-Userinfo", fmt.Sprintf(
		"upload=0; download=%d; total=%d; expire=%d",
		content.UsedBytes, content.TotalBytes, expireEpoch,
	))
	ctx.Response().Header().Set("Profile-Update-Interval", "6")

	return ctx.Blob(http.StatusOK, "text/plain; charset=utf-8", []byte(content.Body))
}
