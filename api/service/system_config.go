package service

import (
	"errors"
	"fmt"
	"os"
	"sort"
	"strconv"

	"go.uber.org/zap"
	"gorm.io/gorm"

	"github.com/maahdima/mwp/api/config"
	"github.com/maahdima/mwp/api/dataservice/model"
	"github.com/maahdima/mwp/api/utils"
)

// systemConfigPortKey is the SystemConfig row key used to store an
// admin-chosen override for the port the panel's HTTP server listens on.
// Absent (no row) means "use SERVER_PORT / the built-in default (3000)",
// matching config.GetAppConfig()'s existing fallback chain.
const systemConfigPortKey = "server_port"

// systemConfigJwtSecretKeyPrefix + a purpose suffix ("admin"/"app") keys
// the persisted JWT signing secrets -- see GetOrCreateJwtSecret's own doc
// comment for why these must survive a restart, unlike the process's
// other random values.
const systemConfigJwtSecretKeyPrefix = "jwt_secret_"

// systemConfigGeoIPOnlineKey toggles GeoIPService.Lookup between its
// original offline-only mode (uploaded .mmdb files, see geoip.go's own doc
// comment) and querying a live third-party IP-lookup API instead. Absent
// (no row) means "offline" -- the behavior every existing install already
// has, so introducing this toggle changes nothing for an admin who never
// visits the new switch.
const systemConfigGeoIPOnlineKey = "geoip_online_enabled"

// SystemConfigService wraps the generic SystemConfig key-value table (see
// dataservice/model/system_config.go) for the handful of one-off settings
// that need to survive a restart but don't warrant their own dedicated
// table -- currently just the HTTP listen port override.
type SystemConfigService struct {
	db     *gorm.DB
	logger *zap.Logger
}

func NewSystemConfigService(db *gorm.DB) *SystemConfigService {
	return &SystemConfigService{
		db:     db,
		logger: zap.L().Named("SystemConfigService"),
	}
}

// GetPortOverride returns the admin-configured port override, or "" if none
// has been set (meaning the panel should fall back to SERVER_PORT / 3000).
func (s *SystemConfigService) GetPortOverride() (string, error) {
	var row model.SystemConfig
	if err := s.db.Where("key = ?", systemConfigPortKey).First(&row).Error; err != nil {
		if errors.Is(err, gorm.ErrRecordNotFound) {
			return "", nil
		}
		s.logger.Error("failed to read port override", zap.Error(err))
		return "", err
	}
	return row.Value, nil
}

// SetPortOverride validates and persists a new port override. It does NOT
// change the port the currently-running process is listening on -- Echo
// binds its listener once at startup, so this only takes effect the next
// time the panel process starts (see cmd/main.go, which reads this value
// and applies it to SERVER_PORT before config.GetAppConfig() is first
// called). Callers must make this "restart required" constraint clear to
// the admin.
func (s *SystemConfigService) SetPortOverride(port int) error {
	if port < 1 || port > 65535 {
		return errors.New("port must be between 1 and 65535")
	}

	value := strconv.Itoa(port)

	var existing model.SystemConfig
	err := s.db.Where("key = ?", systemConfigPortKey).First(&existing).Error
	if err == nil {
		existing.Value = value
		return s.db.Save(&existing).Error
	}
	if !errors.Is(err, gorm.ErrRecordNotFound) {
		s.logger.Error("failed to check existing port override", zap.Error(err))
		return err
	}

	return s.db.Create(&model.SystemConfig{Key: systemConfigPortKey, Value: value}).Error
}

// GetOrCreateJwtSecret returns a stable, persisted JWT signing secret for
// the given purpose ("admin" or "app") -- generating and saving a fresh
// random one on first call, then returning that SAME value on every
// subsequent call/process restart.
//
// Confirmed, reported bug this fixes: both Authentication.AccessSecret
// and AppAuthController's own accessSecret were previously generated
// fresh via utils.RandomString(24) on EVERY process start (see
// cmd/http-server/http-server.go) -- meaning every single deploy/restart
// silently invalidated every already-logged-in admin/reseller/mobile-app
// session's token, with no clear error surfaced to the client beyond a
// generic "invalid or expired jwt: signature is invalid" 401 (which the
// mobile app's own refreshIfPossible swallows into a background log
// line, making the whole failure look like stale/unrefreshed data
// instead of what it actually was: a silently expired session).
func (s *SystemConfigService) GetOrCreateJwtSecret(purpose string) (string, error) {
	key := systemConfigJwtSecretKeyPrefix + purpose

	var row model.SystemConfig
	err := s.db.Where("key = ?", key).First(&row).Error
	if err == nil {
		return row.Value, nil
	}
	if !errors.Is(err, gorm.ErrRecordNotFound) {
		s.logger.Error("failed to read jwt secret", zap.String("purpose", purpose), zap.Error(err))
		return "", err
	}

	secret := utils.RandomString(24)
	if err := s.db.Create(&model.SystemConfig{Key: key, Value: secret}).Error; err != nil {
		s.logger.Error("failed to persist jwt secret", zap.String("purpose", purpose), zap.Error(err))
		return "", err
	}
	return secret, nil
}

// GetGeoIPOnlineEnabled reports whether GeoIPService.Lookup should query a
// live third-party API instead of the uploaded offline .mmdb files.
// Defaults to false (offline) when no row exists yet.
func (s *SystemConfigService) GetGeoIPOnlineEnabled() (bool, error) {
	var row model.SystemConfig
	if err := s.db.Where("key = ?", systemConfigGeoIPOnlineKey).First(&row).Error; err != nil {
		if errors.Is(err, gorm.ErrRecordNotFound) {
			return false, nil
		}
		s.logger.Error("failed to read geoip online mode", zap.Error(err))
		return false, err
	}
	return row.Value == "1", nil
}

// SetGeoIPOnlineEnabled persists the admin's choice of online vs. offline
// IP lookup mode -- takes effect on the very next Lookup call (no restart
// needed, unlike GetPortOverride's own listener-bound setting), since
// GeoIPService re-reads this flag itself rather than caching it at startup.
func (s *SystemConfigService) SetGeoIPOnlineEnabled(enabled bool) error {
	value := "0"
	if enabled {
		value = "1"
	}

	var existing model.SystemConfig
	err := s.db.Where("key = ?", systemConfigGeoIPOnlineKey).First(&existing).Error
	if err == nil {
		existing.Value = value
		return s.db.Save(&existing).Error
	}
	if !errors.Is(err, gorm.ErrRecordNotFound) {
		s.logger.Error("failed to check existing geoip online mode", zap.Error(err))
		return err
	}

	return s.db.Create(&model.SystemConfig{Key: systemConfigGeoIPOnlineKey, Value: value}).Error
}

// databaseSizeRow mirrors dbstat's own column shape for the one query
// GetDatabaseSizeBreakdown runs -- see that function's own doc comment.
type databaseSizeRow struct {
	Name   string
	Pgsize int64
}

// DatabaseSizeTableEntry is this service's own plain result type for one
// table's (or index's) share of the database file -- kept independent of
// api/http/schema's identically-shaped response type, matching this
// codebase's existing layering (service methods never import http/schema;
// http/*.go handlers map a service's own result type onto the wire
// schema, e.g. SSLSettingsService.LoadTLSConfig's tls.Config vs
// http/ssl_settings.go's own response shape).
type DatabaseSizeTableEntry struct {
	Name      string
	BytesUsed int64
}

// DatabaseSizeBreakdown is GetDatabaseSizeBreakdown's own result type --
// see that function's doc comment for what TotalBytes/Tables mean.
type DatabaseSizeBreakdown struct {
	TotalBytes int64
	Tables     []DatabaseSizeTableEntry
}

// GetDatabaseSizeBreakdown reports the panel's own SQLite database file's
// total on-disk size (read directly from the filesystem, so it reflects
// reality even if SQLite's own page count and the file size have drifted,
// e.g. right after a bulk delete that hasn't been VACUUMed yet) alongside
// a per-table breakdown of what's actually consuming that space, queried
// from SQLite's own `dbstat` virtual table -- exact page-level accounting,
// not an estimate (see this session's own live investigation of the
// production database's bloat, which used this exact technique manually
// before this method existed as an admin-facing feature). Index pages are
// reported as their own entries (dbstat names them after the index, e.g.
// "idx_graph_nodes_deleted_at") rather than folded into their owning
// table's total, matching exactly what dbstat itself reports -- an admin
// investigating bloat needs to see a bloated index as distinctly as a
// bloated table, since the fix (and the underlying bug, if any) differs.
//
// Postgres-backed installs get an explicit "not supported" error rather
// than a wrong/empty result -- dbstat is a SQLite-only virtual table, and
// this codebase already treats SQLite as a first-class, still-default
// dialect worth dedicated tooling for (see e.g. this file's own port
// override, ConnectDB's own SQLite-specific pragmas).
func (s *SystemConfigService) GetDatabaseSizeBreakdown() (*DatabaseSizeBreakdown, error) {
	dbCfg := config.GetDBConfig()
	if dbCfg.Dialect != "sqlite" {
		return nil, fmt.Errorf("database size breakdown is only supported for sqlite installs (this install uses %s)", dbCfg.Dialect)
	}

	fileInfo, err := os.Stat(dbCfg.Database)
	if err != nil {
		s.logger.Error("failed to stat database file", zap.String("path", dbCfg.Database), zap.Error(err))
		return nil, err
	}

	var rows []databaseSizeRow
	if err := s.db.Raw("SELECT name, SUM(pgsize) as pgsize FROM dbstat GROUP BY name").Scan(&rows).Error; err != nil {
		s.logger.Error("failed to query dbstat for database size breakdown", zap.Error(err))
		return nil, err
	}

	tables := make([]DatabaseSizeTableEntry, 0, len(rows))
	for _, row := range rows {
		tables = append(tables, DatabaseSizeTableEntry{Name: row.Name, BytesUsed: row.Pgsize})
	}
	sort.Slice(tables, func(i, j int) bool { return tables[i].BytesUsed > tables[j].BytesUsed })

	return &DatabaseSizeBreakdown{
		TotalBytes: fileInfo.Size(),
		Tables:     tables,
	}, nil
}
