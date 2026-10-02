package http

import (
	"io"
	"net/http"
	"strconv"
	"time"

	"github.com/maahdima/mwp/api/http/schema"
	"github.com/maahdima/mwp/api/service"

	"github.com/labstack/echo/v4"
	"go.uber.org/zap"
)

// maxGeoIPUploadSize bounds an uploaded .mmdb file -- GeoLite2-City is
// typically ~70MB, GeoLite2-ASN ~10MB; 200MB comfortably covers both with
// headroom while still rejecting a runaway/wrong-file upload before it's
// read fully into memory (mirrors BackupController's own upload-size-cap
// pattern).
const maxGeoIPUploadSize = 200 << 20 // 200 MiB

type SecurityController struct {
	securityService     *service.SecurityService
	geoIPService        *service.GeoIPService
	retentionService    *service.SecurityRetentionService
	systemConfigService *service.SystemConfigService
	logger              *zap.Logger
}

func NewSecurityController(
	securityService *service.SecurityService,
	geoIPService *service.GeoIPService,
	retentionService *service.SecurityRetentionService,
	systemConfigService *service.SystemConfigService,
) *SecurityController {
	return &SecurityController{
		securityService:     securityService,
		geoIPService:        geoIPService,
		retentionService:    retentionService,
		systemConfigService: systemConfigService,
		logger:              zap.L().Named("SecurityController"),
	}
}

func (c *SecurityController) internalError(ctx echo.Context, action string, err error) error {
	c.logger.Error("failed to "+action, zap.Error(err))
	return ctx.JSON(http.StatusInternalServerError, schema.InternalServerErrorResponse)
}

// GetGeoIPStatus reports whether each mmdb is currently loaded -- backs
// the two upload boxes' "already uploaded" indicator. Admin only.
func (c *SecurityController) GetGeoIPStatus(ctx echo.Context) error {
	if forbidden, err := forbidUnlessAdmin(ctx); forbidden {
		return err
	}
	status := c.geoIPService.Status()
	return ctx.JSON(http.StatusOK, schema.BasicResponseData[schema.GeoIPStatusResponse]{
		BasicResponse: schema.OkBasicResponse,
		Data:          schema.GeoIPStatusResponse{CityLoaded: status.CityLoaded, ASNLoaded: status.ASNLoaded},
	})
}

// GetGeoIPMode reports whether IP lookups currently use the uploaded
// offline .mmdb files or a live third-party API -- backs the Security
// page's online/offline switch. Admin only.
func (c *SecurityController) GetGeoIPMode(ctx echo.Context) error {
	if forbidden, err := forbidUnlessAdmin(ctx); forbidden {
		return err
	}
	online, err := c.systemConfigService.GetGeoIPOnlineEnabled()
	if err != nil {
		return c.internalError(ctx, "read geoip mode", err)
	}
	return ctx.JSON(http.StatusOK, schema.BasicResponseData[schema.GeoIPModeResponse]{
		BasicResponse: schema.OkBasicResponse,
		Data:          schema.GeoIPModeResponse{OnlineEnabled: online},
	})
}

// SetGeoIPMode flips the online/offline toggle -- takes effect on the very
// next IP lookup, no restart needed (see SystemConfigService.
// SetGeoIPOnlineEnabled's own doc comment). Admin only.
func (c *SecurityController) SetGeoIPMode(ctx echo.Context) error {
	if forbidden, err := forbidUnlessAdmin(ctx); forbidden {
		return err
	}
	var req schema.SetGeoIPModeRequest
	if err := ctx.Bind(&req); err != nil {
		return ctx.JSON(http.StatusBadRequest, schema.BadParamsErrorResponse)
	}
	if err := c.systemConfigService.SetGeoIPOnlineEnabled(req.OnlineEnabled); err != nil {
		return c.internalError(ctx, "set geoip mode", err)
	}
	return ctx.JSON(http.StatusOK, schema.OkBasicResponse)
}

func (c *SecurityController) readUploadedFile(ctx echo.Context, field string) ([]byte, error) {
	fileHeader, err := ctx.FormFile(field)
	if err != nil {
		return nil, err
	}
	if fileHeader.Size > maxGeoIPUploadSize {
		return nil, echo.NewHTTPError(http.StatusBadRequest, "uploaded file exceeds the maximum allowed size")
	}
	src, err := fileHeader.Open()
	if err != nil {
		return nil, err
	}
	defer src.Close()
	return io.ReadAll(src)
}

// UploadGeoIPCity/UploadGeoIPASN accept a raw .mmdb file upload -- see
// GeoIPService.UploadCityDatabase/UploadASNDatabase for the validate-
// before-write contract. Admin only.
//
// Both handlers clear IPGeoCache and then re-resolve every currently-open
// IPConnectionLog row right after a successful upload -- a confirmed,
// reported bug with two stacked causes:
//  1. Lookup caches even an all-nil result forever (see GeoIPService.Lookup's
//     own doc comment), so any IP resolved BEFORE the admin uploaded their
//     .mmdb files stayed permanently blank afterward, even though the upload
//     itself succeeded and Status() correctly reported the database as
//     loaded. Requiring the admin to separately press "Clear Cache" after
//     every upload was easy to miss (and, before this fix, the UI never
//     surfaced that button at all).
//  2. Clearing the cache alone only fixes FUTURE lookups -- an identity
//     whose IP hasn't changed since before the upload has an IPConnectionLog
//     row that was already stamped with nil geo data at creation time, and
//     SecurityIPCollectorService only re-resolves geo when a NEW row opens
//     (see reconcile's own doc comment). Without the backfill below, the
//     admin would see the SAME blank identities list even immediately after
//     a fresh upload, until every currently-connected client happened to
//     roam to a different IP.
//
// Together these make "upload a database" and "make the page show correct
// data right now" the same action, matching what an admin uploading a new
// database would expect -- instead of a multi-step, undiscoverable manual
// recovery.
func (c *SecurityController) UploadGeoIPCity(ctx echo.Context) error {
	if forbidden, err := forbidUnlessAdmin(ctx); forbidden {
		return err
	}
	content, err := c.readUploadedFile(ctx, "file")
	if err != nil {
		return ctx.JSON(http.StatusBadRequest, schema.ErrorResponse{StatusCode: http.StatusBadRequest, Status: "error", Message: err.Error()})
	}
	if err := c.geoIPService.UploadCityDatabase(content); err != nil {
		return ctx.JSON(http.StatusBadRequest, schema.ErrorResponse{StatusCode: http.StatusBadRequest, Status: "error", Message: err.Error()})
	}
	c.refreshGeoDataAfterUpload()
	return ctx.JSON(http.StatusOK, schema.OkBasicResponse)
}

func (c *SecurityController) UploadGeoIPASN(ctx echo.Context) error {
	if forbidden, err := forbidUnlessAdmin(ctx); forbidden {
		return err
	}
	content, err := c.readUploadedFile(ctx, "file")
	if err != nil {
		return ctx.JSON(http.StatusBadRequest, schema.ErrorResponse{StatusCode: http.StatusBadRequest, Status: "error", Message: err.Error()})
	}
	if err := c.geoIPService.UploadASNDatabase(content); err != nil {
		return ctx.JSON(http.StatusBadRequest, schema.ErrorResponse{StatusCode: http.StatusBadRequest, Status: "error", Message: err.Error()})
	}
	c.refreshGeoDataAfterUpload()
	return ctx.JSON(http.StatusOK, schema.OkBasicResponse)
}

func (c *SecurityController) refreshGeoDataAfterUpload() {
	if err := c.geoIPService.ClearCache(); err != nil {
		c.logger.Warn("failed to clear geoip cache after database upload", zap.Error(err))
		return
	}
	if updated, err := c.securityService.BackfillOpenConnectionGeoData(c.geoIPService); err != nil {
		c.logger.Warn("failed to backfill open connection geo data after database upload", zap.Error(err))
	} else {
		c.logger.Info("backfilled geo data for open connections after database upload", zap.Int("updated", updated))
	}
}

// DeleteGeoIPCity/DeleteGeoIPASN remove an uploaded database file. Admin
// only.
func (c *SecurityController) DeleteGeoIPCity(ctx echo.Context) error {
	if forbidden, err := forbidUnlessAdmin(ctx); forbidden {
		return err
	}
	if err := c.geoIPService.DeleteCityDatabase(); err != nil {
		return c.internalError(ctx, "delete geoip city database", err)
	}
	return ctx.JSON(http.StatusOK, schema.OkBasicResponse)
}

func (c *SecurityController) DeleteGeoIPASN(ctx echo.Context) error {
	if forbidden, err := forbidUnlessAdmin(ctx); forbidden {
		return err
	}
	if err := c.geoIPService.DeleteASNDatabase(); err != nil {
		return c.internalError(ctx, "delete geoip asn database", err)
	}
	return ctx.JSON(http.StatusOK, schema.OkBasicResponse)
}

// ClearGeoIPCache deletes every cached IP lookup -- the admin's own
// explicit "بتونم این لیست رو حذف کنم" requirement for after uploading a
// newer mmdb pair -- and then backfills every currently-open
// IPConnectionLog row against the now-empty cache, exactly like the upload
// handlers (see refreshGeoDataAfterUpload's own doc comment for why the
// backfill step is required, not optional). Admin only.
func (c *SecurityController) ClearGeoIPCache(ctx echo.Context) error {
	if forbidden, err := forbidUnlessAdmin(ctx); forbidden {
		return err
	}
	if err := c.geoIPService.ClearCache(); err != nil {
		return c.internalError(ctx, "clear geoip cache", err)
	}
	if updated, err := c.securityService.BackfillOpenConnectionGeoData(c.geoIPService); err != nil {
		c.logger.Warn("failed to backfill open connection geo data after cache clear", zap.Error(err))
	} else {
		c.logger.Info("backfilled geo data for open connections after cache clear", zap.Int("updated", updated))
	}
	return ctx.JSON(http.StatusOK, schema.OkBasicResponse)
}

// ListIdentities backs the Security page's main user table + search field.
// Admin only.
func (c *SecurityController) ListIdentities(ctx echo.Context) error {
	if forbidden, err := forbidUnlessAdmin(ctx); forbidden {
		return err
	}
	search := ctx.QueryParam("search")
	var onlineOnly *bool
	if raw := ctx.QueryParam("online_only"); raw != "" {
		parsed, parseErr := strconv.ParseBool(raw)
		if parseErr != nil {
			return ctx.JSON(http.StatusBadRequest, schema.BadParamsErrorResponse)
		}
		onlineOnly = &parsed
	}
	identities, err := c.securityService.ListIdentities(search, onlineOnly)
	if err != nil {
		return c.internalError(ctx, "list security identities", err)
	}

	rows := make([]schema.SecurityIdentityResponse, 0, len(identities))
	for _, id := range identities {
		rows = append(rows, schema.SecurityIdentityResponse{
			Protocol: id.Protocol, Identity: id.Identity,
			LastIPAddress: id.LastIPAddress, LastCountry: id.LastCountry, LastCity: id.LastCity,
			LastLat: id.LastLat, LastLon: id.LastLon, LastASN: id.LastASN, LastISP: id.LastISP,
			LastConnectedAt: id.LastConnectedAt.Format(time.RFC3339), IsCurrentlyOpen: id.IsCurrentlyOpen,
		})
	}
	return ctx.JSON(http.StatusOK, schema.BasicResponseData[schema.SecurityIdentitiesResponse]{
		BasicResponse: schema.OkBasicResponse,
		Data:          schema.SecurityIdentitiesResponse{Identities: rows},
	})
}

// GetIdentityHistory backs the per-user IP-history drill-down. Admin only.
func (c *SecurityController) GetIdentityHistory(ctx echo.Context) error {
	if forbidden, err := forbidUnlessAdmin(ctx); forbidden {
		return err
	}
	protocol := ctx.Param("protocol")
	identity := ctx.Param("identity")

	sessions, err := c.securityService.GetIdentityHistory(protocol, identity)
	if err != nil {
		return c.internalError(ctx, "get security identity history", err)
	}

	rows := make([]schema.SecuritySessionResponse, 0, len(sessions))
	for _, s := range sessions {
		var disconnectedAt *string
		if s.DisconnectedAt != nil {
			formatted := s.DisconnectedAt.Format(time.RFC3339)
			disconnectedAt = &formatted
		}
		rows = append(rows, schema.SecuritySessionResponse{
			IPAddress: s.IPAddress, ConnectedAt: s.ConnectedAt.Format(time.RFC3339), DisconnectedAt: disconnectedAt,
			Country: s.Country, Region: s.Region, City: s.City,
			Lat: s.Lat, Lon: s.Lon, Timezone: s.Timezone, ASN: s.ASN, ISP: s.ISP,
			UsedBytes: s.UsedBytes,
		})
	}
	return ctx.JSON(http.StatusOK, schema.BasicResponseData[schema.SecurityHistoryResponse]{
		BasicResponse: schema.OkBasicResponse,
		Data:          schema.SecurityHistoryResponse{Protocol: protocol, Identity: identity, Sessions: rows},
	})
}

// GetEtherTraffic backs the "ترافیک کلی" live-ish flow table. Admin only.
func (c *SecurityController) GetEtherTraffic(ctx echo.Context) error {
	if forbidden, err := forbidUnlessAdmin(ctx); forbidden {
		return err
	}
	flows, err := c.securityService.GetLatestEtherTraffic()
	if err != nil {
		return c.internalError(ctx, "get ether traffic", err)
	}

	rows := make([]schema.EtherTrafficRowResponse, 0, len(flows))
	for _, f := range flows {
		rows = append(rows, schema.EtherTrafficRowResponse{
			SrcAddress: f.SrcAddress, IPProtocol: f.IPProtocol, SrcPort: f.SrcPort,
			TxBytesPerSecond: f.TxBytesPerSecond, RxBytesPerSecond: f.RxBytesPerSecond,
			TxPacketsRate: f.TxPacketsRate, RxPacketsRate: f.RxPacketsRate,
			Country: f.Country, City: f.City, Lat: f.Lat, Lon: f.Lon, ISP: f.ISP,
			SampledAt: f.SampledAt.Format(time.RFC3339),
		})
	}
	return ctx.JSON(http.StatusOK, schema.BasicResponseData[schema.EtherTrafficResponse]{
		BasicResponse: schema.OkBasicResponse,
		Data:          schema.EtherTrafficResponse{Flows: rows},
	})
}

// GetThreats backs the "تهدیدات" panel -- see service.SecurityThreat's own
// doc comment for what's actually detected. Admin only.
func (c *SecurityController) GetThreats(ctx echo.Context) error {
	if forbidden, err := forbidUnlessAdmin(ctx); forbidden {
		return err
	}
	threats, err := c.securityService.GetThreats()
	if err != nil {
		return c.internalError(ctx, "get security threats", err)
	}

	rows := make([]schema.SecurityThreatResponse, 0, len(threats))
	for _, t := range threats {
		rows = append(rows, schema.SecurityThreatResponse{
			Kind: t.Kind, Subject: t.Subject, Detail: t.Detail, Severity: t.Severity,
			Country: t.Country, City: t.City,
			LastSeenAt: t.LastSeenAt.Format(time.RFC3339), OccurrenceCount: t.OccurrenceCount,
		})
	}
	return ctx.JSON(http.StatusOK, schema.BasicResponseData[schema.SecurityThreatsResponse]{
		BasicResponse: schema.OkBasicResponse,
		Data:          schema.SecurityThreatsResponse{Threats: rows},
	})
}

// ClearConnectionHistory / ClearEtherTraffic implement the admin's "پاکسازی
// کامل" bulk-clear buttons. Admin only.
func (c *SecurityController) ClearConnectionHistory(ctx echo.Context) error {
	if forbidden, err := forbidUnlessAdmin(ctx); forbidden {
		return err
	}
	if err := c.securityService.ClearConnectionHistory(); err != nil {
		return c.internalError(ctx, "clear connection history", err)
	}
	return ctx.JSON(http.StatusOK, schema.OkBasicResponse)
}

func (c *SecurityController) ClearEtherTraffic(ctx echo.Context) error {
	if forbidden, err := forbidUnlessAdmin(ctx); forbidden {
		return err
	}
	if err := c.securityService.ClearEtherTraffic(); err != nil {
		return c.internalError(ctx, "clear ether traffic", err)
	}
	return ctx.JSON(http.StatusOK, schema.OkBasicResponse)
}

// RunRetentionCleanup implements the admin's time-window/inactive-user
// cleanup criteria (see schema.RetentionCleanupRequest's own doc comment).
// Admin only.
func (c *SecurityController) RunRetentionCleanup(ctx echo.Context) error {
	if forbidden, err := forbidUnlessAdmin(ctx); forbidden {
		return err
	}

	var req schema.RetentionCleanupRequest
	if err := ctx.Bind(&req); err != nil {
		return ctx.JSON(http.StatusBadRequest, schema.BadParamsErrorResponse)
	}
	if err := ctx.Validate(&req); err != nil {
		return ctx.JSON(http.StatusBadRequest, schema.BadParamsErrorResponse)
	}

	var deleted int64

	if req.InactiveUsersOnly && req.Target == "connection_log" {
		count, err := c.retentionService.DeleteConnectionLogForInactiveUsers()
		if err != nil {
			return c.internalError(ctx, "run retention cleanup for inactive users", err)
		}
		deleted += count
	}

	if req.OlderThanDays != nil {
		cutoff := time.Now().AddDate(0, 0, -*req.OlderThanDays)
		var count int64
		var err error
		switch req.Target {
		case "ether_traffic":
			count, err = c.retentionService.DeleteEtherTrafficOlderThan(cutoff)
		default:
			count, err = c.retentionService.DeleteConnectionLogOlderThan(cutoff)
		}
		if err != nil {
			return c.internalError(ctx, "run retention cleanup", err)
		}
		deleted += count
	}

	return ctx.JSON(http.StatusOK, schema.BasicResponseData[schema.RetentionCleanupResponse]{
		BasicResponse: schema.OkBasicResponse,
		Data:          schema.RetentionCleanupResponse{DeletedCount: deleted},
	})
}
