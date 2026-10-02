package service

import (
	"context"
	"errors"
	"fmt"
	"net"
	"os"
	"path/filepath"
	"sync"
	"time"

	"github.com/oschwald/geoip2-golang"
	"go.uber.org/zap"
	"gorm.io/gorm"
	"gorm.io/gorm/clause"

	"github.com/maahdima/mwp/api/dataservice/model"
	"github.com/maahdima/mwp/api/utils/httphelper"
)

// geoIPCityFileName/geoIPASNFileName are fixed on-disk filenames -- an
// upload always replaces whatever was there before (matching the admin's
// own explicit request: "بگو فایل سیتی رو اینجا وارد کن و فایل Asn رو
// اینجا", i.e. two fixed upload slots, not an arbitrary file list).
const (
	geoIPCityFileName = "GeoLite2-City.mmdb"
	geoIPASNFileName  = "GeoLite2-ASN.mmdb"
)

// geoIPOnlineAPIBaseURL is a package-level override point for the online
// lookup's base URL -- production code never changes it from the real
// ip-api.com default, but tests point it at a local httptest server
// instead, so lookupOnlineAndCache can be exercised against a real HTTP
// request/response cycle without ever calling the real third-party API.
// Mirrors faoxima_provisioner.go's identical telegramAPIBaseURL pattern for
// the same reason.
var geoIPOnlineAPIBaseURL = "http://ip-api.com"

// GeoIPLookupResult mirrors IPGeoCache's own resolved fields -- returned by
// Lookup and used directly to populate both IPGeoCache rows and the
// denormalized copies on IPConnectionLog/EtherTrafficSample.
type GeoIPLookupResult struct {
	Country  *string
	Region   *string
	City     *string
	Lat      *float64
	Lon      *float64
	Timezone *string
	ASN      *uint
	ISP      *string
}

// GeoIPService owns the two uploaded .mmdb readers and the IPGeoCache
// table sitting in front of them. Every lookup goes through Lookup, which
// checks the cache FIRST and only ever touches the mmdb readers on a
// cache miss -- the admin's own explicit requirement ("دوباره گرفته
// نشه"): once an IP has been resolved, it is never looked up again unless
// ClearCache is called (normally right after uploading a newer file pair).
//
// systemConfig backs the online/offline mode toggle (see
// SystemConfigService.GetGeoIPOnlineEnabled) -- read fresh on every
// Lookup call rather than cached at construction time, so an admin
// flipping the switch takes effect immediately without a panel restart.
type GeoIPService struct {
	db           *gorm.DB
	dataDir      string
	logger       *zap.Logger
	systemConfig *SystemConfigService
	onlineClient *httphelper.Client

	mu         sync.RWMutex
	cityReader *geoip2.Reader
	asnReader  *geoip2.Reader
}

func NewGeoIPService(db *gorm.DB, dataDir string, systemConfig *SystemConfigService) *GeoIPService {
	// ip-api.com's free tier needs no API key and no HTTPS -- see Lookup's
	// own doc comment for why a 5s timeout (matching this codebase's other
	// short-lived third-party lookups, e.g. device_data.go's identical
	// choice for the same API) rather than httphelper's own 30s default:
	// this call sits on the SAME request path as every offline lookup, so a
	// slow/unreachable third party must not make an unrelated HTTP handler
	// (e.g. backfilling a connection log's geo fields) hang for 30s.
	onlineClient, err := httphelper.NewClient(httphelper.Config{
		BaseURL: geoIPOnlineAPIBaseURL,
		Timeout: 5 * time.Second,
	})
	if err != nil {
		// BaseURL is a constant above, never empty -- this branch is
		// unreachable in practice, but NewClient's signature returns an
		// error, so it must be handled rather than ignored.
		zap.L().Named("GeoIPService").Error("failed to build online geoip client", zap.Error(err))
	}

	s := &GeoIPService{
		db:           db,
		dataDir:      filepath.Join(dataDir, "geoip"),
		logger:       zap.L().Named("GeoIPService"),
		systemConfig: systemConfig,
		onlineClient: onlineClient,
	}
	if err := os.MkdirAll(s.dataDir, 0o755); err != nil {
		s.logger.Error("failed to create geoip data directory", zap.Error(err))
	}
	s.openExistingReaders()
	return s
}

func (s *GeoIPService) cityPath() string { return filepath.Join(s.dataDir, geoIPCityFileName) }
func (s *GeoIPService) asnPath() string  { return filepath.Join(s.dataDir, geoIPASNFileName) }

// openExistingReaders is called once at startup so a previously-uploaded
// pair survives a panel restart without needing to be re-uploaded.
func (s *GeoIPService) openExistingReaders() {
	s.mu.Lock()
	defer s.mu.Unlock()

	if _, err := os.Stat(s.cityPath()); err == nil {
		if reader, err := geoip2.Open(s.cityPath()); err != nil {
			s.logger.Error("failed to open existing GeoLite2-City.mmdb", zap.Error(err))
		} else {
			s.cityReader = reader
		}
	}
	if _, err := os.Stat(s.asnPath()); err == nil {
		if reader, err := geoip2.Open(s.asnPath()); err != nil {
			s.logger.Error("failed to open existing GeoLite2-ASN.mmdb", zap.Error(err))
		} else {
			s.asnReader = reader
		}
	}
}

// UploadCityDatabase/UploadASNDatabase validate the uploaded bytes really
// are a readable mmdb file BEFORE writing/replacing anything on disk (a
// corrupt upload must never leave the panel with a half-written or
// unreadable database file), then atomically swap the in-memory reader.
func (s *GeoIPService) UploadCityDatabase(content []byte) error {
	reader, err := geoip2.FromBytes(content)
	if err != nil {
		return fmt.Errorf("فایل آپلودشده یک دیتابیس GeoLite2-City معتبر نیست: %w", err)
	}
	if err := os.WriteFile(s.cityPath(), content, 0o644); err != nil {
		reader.Close()
		return fmt.Errorf("ذخیره‌سازی فایل روی دیسک ناموفق بود: %w", err)
	}

	s.mu.Lock()
	old := s.cityReader
	s.cityReader = reader
	s.mu.Unlock()
	if old != nil {
		old.Close()
	}

	return nil
}

func (s *GeoIPService) UploadASNDatabase(content []byte) error {
	reader, err := geoip2.FromBytes(content)
	if err != nil {
		return fmt.Errorf("فایل آپلودشده یک دیتابیس GeoLite2-ASN معتبر نیست: %w", err)
	}
	if err := os.WriteFile(s.asnPath(), content, 0o644); err != nil {
		reader.Close()
		return fmt.Errorf("ذخیره‌سازی فایل روی دیسک ناموفق بود: %w", err)
	}

	s.mu.Lock()
	old := s.asnReader
	s.asnReader = reader
	s.mu.Unlock()
	if old != nil {
		old.Close()
	}

	return nil
}

// DeleteCityDatabase/DeleteASNDatabase remove the uploaded file and close
// the reader -- subsequent lookups for that half simply return no data
// (Lookup degrades gracefully field-by-field, see its own doc comment)
// rather than erroring.
func (s *GeoIPService) DeleteCityDatabase() error {
	s.mu.Lock()
	if s.cityReader != nil {
		s.cityReader.Close()
		s.cityReader = nil
	}
	s.mu.Unlock()

	if err := os.Remove(s.cityPath()); err != nil && !errors.Is(err, os.ErrNotExist) {
		return err
	}
	return nil
}

func (s *GeoIPService) DeleteASNDatabase() error {
	s.mu.Lock()
	if s.asnReader != nil {
		s.asnReader.Close()
		s.asnReader = nil
	}
	s.mu.Unlock()

	if err := os.Remove(s.asnPath()); err != nil && !errors.Is(err, os.ErrNotExist) {
		return err
	}
	return nil
}

// Status reports whether each database is currently loaded -- backs the
// Security page's two upload boxes so the admin can see at a glance
// whether a file is already in place.
type GeoIPStatus struct {
	CityLoaded bool
	ASNLoaded  bool
}

func (s *GeoIPService) Status() GeoIPStatus {
	s.mu.RLock()
	defer s.mu.RUnlock()
	return GeoIPStatus{CityLoaded: s.cityReader != nil, ASNLoaded: s.asnReader != nil}
}

// ClearCache deletes every cached IPGeoCache row -- the admin's own
// explicit requirement for after uploading a newer .mmdb pair, so already-
// resolved IPs get re-resolved against the new data instead of keeping
// stale results indefinitely.
func (s *GeoIPService) ClearCache() error {
	return s.db.Session(&gorm.Session{AllowGlobalUpdate: true}).Delete(&model.IPGeoCache{}).Error
}

// Lookup resolves ipAddress, checking IPGeoCache first (see this service's
// own doc comment for why a cache hit never touches the mmdb readers at
// all). A cache MISS is resolved from whichever of the two readers are
// currently loaded -- either, both, or neither; a private/reserved IP or
// one absent from the loaded database(s) simply comes back with those
// fields left nil, never an error, since "no geo data for this IP" is an
// expected, routine outcome (e.g. a peer connecting over a VPN's own
// internal address before NAT), not a failure.
func (s *GeoIPService) Lookup(ipAddress string) (*GeoIPLookupResult, error) {
	var cached model.IPGeoCache
	if err := s.db.Where("ip_address = ?", ipAddress).First(&cached).Error; err == nil {
		return &GeoIPLookupResult{
			Country: cached.Country, Region: cached.Region, City: cached.City,
			Lat: cached.Lat, Lon: cached.Lon, Timezone: cached.Timezone,
			ASN: cached.ASN, ISP: cached.ISP,
		}, nil
	} else if !errors.Is(err, gorm.ErrRecordNotFound) {
		return nil, err
	}

	ip := net.ParseIP(ipAddress)
	if ip == nil {
		return nil, fmt.Errorf("آدرس IP نامعتبر است: %s", ipAddress)
	}

	online, err := s.systemConfig.GetGeoIPOnlineEnabled()
	if err != nil {
		// A broken toggle read must not break every geo lookup in the
		// panel -- fall back to the offline behavior every existing
		// install already has, same as SystemConfigService's own
		// documented "absent means offline" default.
		s.logger.Warn("failed to read geoip online mode, defaulting to offline", zap.Error(err))
		online = false
	}
	if online {
		return s.lookupOnlineAndCache(ipAddress)
	}

	result := &GeoIPLookupResult{}

	s.mu.RLock()
	cityReader, asnReader := s.cityReader, s.asnReader
	s.mu.RUnlock()

	if cityReader != nil {
		if city, err := cityReader.City(ip); err == nil {
			if name := city.Country.Names["en"]; name != "" {
				result.Country = &name
			}
			if len(city.Subdivisions) > 0 {
				if name := city.Subdivisions[0].Names["en"]; name != "" {
					result.Region = &name
				}
			}
			if name := city.City.Names["en"]; name != "" {
				result.City = &name
			}
			if city.Location.Latitude != 0 || city.Location.Longitude != 0 {
				lat, lon := city.Location.Latitude, city.Location.Longitude
				result.Lat, result.Lon = &lat, &lon
			}
			if city.Location.TimeZone != "" {
				result.Timezone = &city.Location.TimeZone
			}
		} else {
			s.logger.Warn("city lookup failed", zap.String("ip", ipAddress), zap.Error(err))
		}
	}

	if asnReader != nil {
		if asn, err := asnReader.ASN(ip); err == nil {
			if asn.AutonomousSystemNumber != 0 {
				number := asn.AutonomousSystemNumber
				result.ASN = &number
			}
			if asn.AutonomousSystemOrganization != "" {
				result.ISP = &asn.AutonomousSystemOrganization
			}
		} else {
			s.logger.Warn("asn lookup failed", zap.String("ip", ipAddress), zap.Error(err))
		}
	}

	s.cacheResult(ipAddress, result)
	return result, nil
}

// cacheResult persists a freshly-resolved lookup into IPGeoCache -- shared
// by both the offline (.mmdb) and online (third-party API) paths in Lookup,
// so a cache hit on the next call skips whichever mode actually resolved it
// this time.
//
// OnConflict DoNothing -- confirmed, reported production incident this
// fixes: Lookup's own check-then-insert above (SELECT for a cache hit,
// then Create on a miss) has an inherent race whenever multiple
// goroutines resolve the SAME uncached IP concurrently (routine at
// this codebase's fleet scale -- e.g. many WireGuard peers sharing one
// NAT egress IP, or many User Manager accounts' RouterOS calls
// resolving the panel's own IP). Every racing goroutine passes the
// SELECT as a miss, then all but one lose the race on the unique
// ip_address index, previously surfacing as a logged (non-fatal)
// "UNIQUE constraint failed" warning PLUS the wasted write itself
// still contending for SQLite's single writer slot -- confirmed via
// journalctl showing repeated back-to-back constraint failures for
// the exact same IP within milliseconds of each other. DoNothing
// makes the losing writes true no-ops (no error, no log noise, and
// SQLite resolves them near-instantly rather than as a failed write
// that still had to take the writer lock).
func (s *GeoIPService) cacheResult(ipAddress string, result *GeoIPLookupResult) {
	cacheRow := model.IPGeoCache{
		IPAddress: ipAddress,
		Country:   result.Country, Region: result.Region, City: result.City,
		Lat: result.Lat, Lon: result.Lon, Timezone: result.Timezone,
		ASN: result.ASN, ISP: result.ISP,
		ResolvedAt: time.Now(),
	}
	if err := s.db.Clauses(clause.OnConflict{DoNothing: true}).Create(&cacheRow).Error; err != nil {
		// Losing the cache write is not fatal to this call -- the caller
		// already has a correct result, it just means this IP will be
		// looked up again next time. Logged, not returned as an error.
		s.logger.Warn("failed to persist geoip cache entry", zap.String("ip", ipAddress), zap.Error(err))
	}
}

// ipApiOnlineResponse mirrors ip-api.com's free-tier JSON shape for the
// fields this service actually keeps (see the "fields=" query parameter in
// lookupOnlineAndCache) -- deliberately a SEPARATE type from
// device_data.go's own IPApiResponse even though both call the same API,
// since that one only ever reads status/country/city/isp for a single
// server's own address, while this one also needs lat/lon/timezone/asn to
// match GeoIPLookupResult's full field set the offline .mmdb path already
// populates.
type ipApiOnlineResponse struct {
	Status     string  `json:"status"`
	Country    string  `json:"country"`
	RegionName string  `json:"regionName"`
	City       string  `json:"city"`
	Lat        float64 `json:"lat"`
	Lon        float64 `json:"lon"`
	Timezone   string  `json:"timezone"`
	ISP        string  `json:"isp"`
	AS         string  `json:"as"`
	Message    string  `json:"message"`
}

// lookupOnlineAndCache resolves ipAddress against ip-api.com's free JSON
// API instead of the uploaded .mmdb files -- used when the admin has
// switched GeoIPService into online mode (see SystemConfigService's own
// doc comment on that toggle). Same graceful-degradation contract as the
// offline path: a failed or unreachable API call returns an EMPTY result
// (all nil fields), never an error, since "no geo data for this IP right
// now" must not break whatever HTTP handler is backfilling a connection
// log's geo fields -- exactly as a private/reserved IP degrades on the
// offline path.
func (s *GeoIPService) lookupOnlineAndCache(ipAddress string) (*GeoIPLookupResult, error) {
	result := &GeoIPLookupResult{}

	if s.onlineClient == nil {
		s.logger.Warn("online geoip client unavailable, returning empty result", zap.String("ip", ipAddress))
		s.cacheResult(ipAddress, result)
		return result, nil
	}

	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()

	var apiResp ipApiOnlineResponse
	path := fmt.Sprintf("/json/%s?fields=status,message,country,regionName,city,lat,lon,timezone,isp,as", ipAddress)
	if err := s.onlineClient.Get(ctx, path, &apiResp); err != nil {
		s.logger.Warn("online geoip lookup failed", zap.String("ip", ipAddress), zap.Error(err))
		s.cacheResult(ipAddress, result)
		return result, nil
	}
	if apiResp.Status != "success" {
		// ip-api.com reports "fail" for private/reserved IPs and invalid
		// queries, with the reason in Message -- routine, not an error
		// worth logging above Debug, matching the offline path's own
		// silent degradation for the same class of address.
		s.logger.Debug("online geoip lookup returned no data", zap.String("ip", ipAddress), zap.String("message", apiResp.Message))
		s.cacheResult(ipAddress, result)
		return result, nil
	}

	if apiResp.Country != "" {
		result.Country = &apiResp.Country
	}
	if apiResp.RegionName != "" {
		result.Region = &apiResp.RegionName
	}
	if apiResp.City != "" {
		result.City = &apiResp.City
	}
	if apiResp.Lat != 0 || apiResp.Lon != 0 {
		lat, lon := apiResp.Lat, apiResp.Lon
		result.Lat, result.Lon = &lat, &lon
	}
	if apiResp.Timezone != "" {
		result.Timezone = &apiResp.Timezone
	}
	if apiResp.ISP != "" {
		result.ISP = &apiResp.ISP
	}
	if apiResp.AS != "" {
		// ip-api.com's "as" field looks like "AS15169 Google LLC" -- parse
		// out the numeric ASN, matching the offline .mmdb path's own
		// AutonomousSystemNumber field shape (a bare uint, no "AS" prefix).
		var asn uint
		if _, err := fmt.Sscanf(apiResp.AS, "AS%d", &asn); err == nil && asn != 0 {
			result.ASN = &asn
		}
	}

	s.cacheResult(ipAddress, result)
	return result, nil
}
