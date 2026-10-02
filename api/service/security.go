package service

import (
	"time"

	"go.uber.org/zap"
	"gorm.io/gorm"

	"github.com/maahdima/mwp/api/dataservice/model"
)

// SecurityService answers every read behind the admin-only Security page --
// a pure DB reader over IPConnectionLog/EtherTrafficSample/IPGeoCache
// (all written by SecurityIPCollectorService/SecurityEtherTorchService),
// mirroring ReportsService's own "never calls anything live, only reads
// what background jobs already wrote" convention.
type SecurityService struct {
	db     *gorm.DB
	logger *zap.Logger
}

func NewSecurityService(db *gorm.DB) *SecurityService {
	return &SecurityService{db: db, logger: zap.L().Named("SecurityService")}
}

// SecurityIdentity is one row of the Security page's main user list.
type SecurityIdentity struct {
	Protocol        string
	Identity        string
	PeerID          *uint
	AccountID       *uint
	LastIPAddress   string
	LastCountry     *string
	LastCity        *string
	LastLat         *float64
	LastLon         *float64
	LastASN         *uint
	LastISP         *string
	LastConnectedAt time.Time
	IsCurrentlyOpen bool
}

// ListIdentities returns every distinct identity that has ever appeared in
// IPConnectionLog, each with its most recent connection's summary.
//
// search matches (case-insensitive substring) across identity/city/
// country/ISP together -- confirmed, reported bug this fixes: the field
// only ever matched the username itself, so an admin trying to find
// "everyone connecting from Germany" or "everyone on this ISP" had no way
// to do it from this page at all.
//
// onlineOnly, when non-nil, additionally restricts to only-currently-open
// (true) or only-currently-closed (false) identities -- confirmed,
// reported bug this fixes: there was no way to see "who is online right
// now" separately from the full list at all.
func (s *SecurityService) ListIdentities(search string, onlineOnly *bool) ([]SecurityIdentity, error) {
	// The most recent row per (protocol, identity) -- SQLite/MySQL both
	// support this via a correlated MAX(id) subquery, avoiding a window
	// function dependency for broader DB compatibility with this
	// codebase's own dual sqlite/mysql support (see BackupService's own
	// dialect-awareness).
	//
	// The search/online filters must be applied to the OUTER query
	// (against the actual latest-row-per-identity set), not the inner
	// MAX(id) grouping subquery -- city/country/ISP/is-open state belong
	// to whichever specific row turns out to be the latest one, which
	// isn't known until the grouping itself resolves.
	var latestIDs []uint
	if err := s.db.Model(&model.IPConnectionLog{}).
		Select("MAX(id)").
		Group("protocol, identity").
		Find(&latestIDs).Error; err != nil {
		s.logger.Error("failed to find latest ip connection log ids", zap.Error(err))
		return nil, err
	}
	if len(latestIDs) == 0 {
		return []SecurityIdentity{}, nil
	}

	query := s.db.Where("id IN ?", latestIDs)
	if search != "" {
		like := "%" + search + "%"
		query = query.Where(
			"identity LIKE ? OR city LIKE ? OR country LIKE ? OR isp LIKE ?",
			like, like, like, like,
		)
	}
	if onlineOnly != nil {
		if *onlineOnly {
			query = query.Where("disconnected_at IS NULL")
		} else {
			query = query.Where("disconnected_at IS NOT NULL")
		}
	}

	var rows []model.IPConnectionLog
	if err := query.Order("connected_at DESC").Find(&rows).Error; err != nil {
		s.logger.Error("failed to fetch latest ip connection log rows", zap.Error(err))
		return nil, err
	}

	identities := make([]SecurityIdentity, 0, len(rows))
	for _, r := range rows {
		identities = append(identities, SecurityIdentity{
			Protocol: r.Protocol, Identity: r.Identity,
			PeerID: r.PeerID, AccountID: r.AccountID,
			LastIPAddress: r.IPAddress, LastCountry: r.Country, LastCity: r.City,
			LastLat: r.Lat, LastLon: r.Lon, LastASN: r.ASN, LastISP: r.ISP,
			LastConnectedAt: r.ConnectedAt, IsCurrentlyOpen: r.DisconnectedAt == nil,
		})
	}
	return identities, nil
}

// SecuritySession is one IPConnectionLog row, enriched with the usage
// figure computed on demand from UsageSnapshot (see this file's own top
// doc comment for why usage is never stored directly on IPConnectionLog).
type SecuritySession struct {
	IPAddress      string
	ConnectedAt    time.Time
	DisconnectedAt *time.Time
	Country        *string
	Region         *string
	City           *string
	Lat            *float64
	Lon            *float64
	Timezone       *string
	ASN            *uint
	ISP            *string
	UsedBytes      int64
}

// GetIdentityHistory returns every session (chronological, oldest first)
// for one (protocol, identity), each with its own usage figure summed from
// UsageSnapshot within [ConnectedAt, DisconnectedAt) (or up to now, for
// the currently-open session).
func (s *SecurityService) GetIdentityHistory(protocol, identity string) ([]SecuritySession, error) {
	var rows []model.IPConnectionLog
	if err := s.db.Where("protocol = ? AND identity = ?", protocol, identity).
		Order("connected_at ASC").Find(&rows).Error; err != nil {
		s.logger.Error("failed to fetch identity connection history", zap.String("protocol", protocol), zap.String("identity", identity), zap.Error(err))
		return nil, err
	}

	sessions := make([]SecuritySession, 0, len(rows))
	for _, r := range rows {
		usedBytes := s.sumUsageForWindow(protocol, r.PeerID, r.AccountID, r.ConnectedAt, r.DisconnectedAt)
		sessions = append(sessions, SecuritySession{
			IPAddress: r.IPAddress, ConnectedAt: r.ConnectedAt, DisconnectedAt: r.DisconnectedAt,
			Country: r.Country, Region: r.Region, City: r.City,
			Lat: r.Lat, Lon: r.Lon, Timezone: r.Timezone, ASN: r.ASN, ISP: r.ISP,
			UsedBytes: usedBytes,
		})
	}
	return sessions, nil
}

func (s *SecurityService) sumUsageForWindow(protocol string, peerID, accountID *uint, start time.Time, end *time.Time) int64 {
	query := s.db.Model(&model.UsageSnapshot{}).
		Where("protocol = ? AND timestamp >= ?", protocol, start.Unix())
	if end != nil {
		query = query.Where("timestamp < ?", end.Unix())
	}
	if peerID != nil {
		query = query.Where("peer_id = ?", *peerID)
	} else if accountID != nil {
		query = query.Where("account_id = ?", *accountID)
	} else {
		return 0
	}

	var total int64
	if err := query.Select("COALESCE(SUM(total_bytes), 0)").Scan(&total).Error; err != nil {
		s.logger.Warn("failed to sum usage for security session window", zap.Error(err))
		return 0
	}
	return total
}

// EtherTrafficRow is one live-ish flow row for the Security page's
// "ترافیک کلی" (overall traffic) section.
type EtherTrafficRow struct {
	SrcAddress       string
	IPProtocol       string
	SrcPort          *int
	TxBytesPerSecond int64
	RxBytesPerSecond int64
	TxPacketsRate    int64
	RxPacketsRate    int64
	Country          *string
	City             *string
	Lat              *float64
	Lon              *float64
	ISP              *string
	SampledAt        time.Time
}

// etherTrafficWindowTicks is how many of the most recent 10s torch poll
// ticks GetLatestEtherTraffic merges into one rolling view. Confirmed,
// reported bug this fixes: showing only the single latest tick (each one
// just a 1-second RouterOS torch sample, see SecurityEtherTorchService.Poll's
// own doc comment) made the table flicker empty/near-empty and "reset"
// every 10 seconds even while traffic was continuously flowing, since
// RouterOS's torch only ever reports whatever happened to be active during
// that specific 1-second window, not a running/sustained view. Merging the
// last few ticks smooths this into something that reads as "who is
// currently active" rather than "what happened in one random second."
const etherTrafficWindowTicks = 6

// GetLatestEtherTraffic returns one row per distinct flow (SrcAddress +
// SrcPort) across the last etherTrafficWindowTicks torch poll ticks, using
// each flow's most recent reading (so an ongoing flow shows its latest
// rate, not a stale one from several ticks ago) -- a flow that hasn't
// appeared in ANY of those recent ticks is dropped, so genuinely-idle
// flows still age out, just not all at once every single tick the way the
// single-latest-tick version did.
func (s *SecurityService) GetLatestEtherTraffic() ([]EtherTrafficRow, error) {
	var tickTimestamps []time.Time
	if err := s.db.Model(&model.EtherTrafficSample{}).
		Distinct("sampled_at").
		Order("sampled_at DESC").
		Limit(etherTrafficWindowTicks).
		Pluck("sampled_at", &tickTimestamps).Error; err != nil {
		s.logger.Error("failed to find recent ether traffic sample ticks", zap.Error(err))
		return nil, err
	}
	if len(tickTimestamps) == 0 {
		return []EtherTrafficRow{}, nil
	}
	oldestTick := tickTimestamps[len(tickTimestamps)-1]

	var rows []model.EtherTrafficSample
	if err := s.db.Where("sampled_at >= ?", oldestTick).Order("sampled_at ASC").Find(&rows).Error; err != nil {
		s.logger.Error("failed to fetch recent ether traffic samples", zap.Error(err))
		return nil, err
	}

	// Iterating oldest-to-newest and overwriting by flow key means each
	// key ends up holding its most recent reading once the loop finishes.
	type flowKey struct {
		addr string
		port int
	}
	merged := make(map[flowKey]model.EtherTrafficSample, len(rows))
	for _, r := range rows {
		port := -1
		if r.SrcPort != nil {
			port = *r.SrcPort
		}
		merged[flowKey{addr: r.SrcAddress, port: port}] = r
	}

	result := make([]EtherTrafficRow, 0, len(merged))
	for _, r := range merged {
		result = append(result, EtherTrafficRow{
			SrcAddress: r.SrcAddress, IPProtocol: r.IPProtocol, SrcPort: r.SrcPort,
			TxBytesPerSecond: r.TxBytesPerSecond, RxBytesPerSecond: r.RxBytesPerSecond,
			TxPacketsRate: r.TxPacketsRate, RxPacketsRate: r.RxPacketsRate,
			Country: r.Country, City: r.City, Lat: r.Lat, Lon: r.Lon, ISP: r.ISP, SampledAt: r.SampledAt,
		})
	}
	return result, nil
}

// ClearConnectionHistory deletes every IPConnectionLog row -- the "پاکسازی
// کامل" bulk-clear action, distinct from the time/criteria-scoped
// retention deletes in SecurityRetentionService.
func (s *SecurityService) ClearConnectionHistory() error {
	return s.db.Session(&gorm.Session{AllowGlobalUpdate: true}).Delete(&model.IPConnectionLog{}).Error
}

// ClearEtherTraffic deletes every EtherTrafficSample row.
func (s *SecurityService) ClearEtherTraffic() error {
	return s.db.Session(&gorm.Session{AllowGlobalUpdate: true}).Delete(&model.EtherTrafficSample{}).Error
}

// BackfillOpenConnectionGeoData re-resolves and rewrites the geo/ASN fields
// on every CURRENTLY OPEN IPConnectionLog row (DisconnectedAt IS NULL).
//
// A confirmed, reported bug: IPConnectionLog's own doc comment explains its
// geo fields are deliberately denormalized at row-CREATION time and never
// re-looked-up afterward -- by design, so a later cache clear doesn't
// silently rewrite what a PAST, already-closed session was recorded as.
// But SecurityIPCollectorService.reconcile only calls GeoIPService.Lookup
// when a NEW row opens (i.e. the identity's IP actually changed); a client
// connected from the same IP since before the admin ever uploaded their
// .mmdb files has an open row stamped with nil geo data that NOTHING would
// ever revisit -- clearing IPGeoCache alone only fixes lookups for IPs seen
// from this point forward, not the identity list's "last connection" row an
// admin is looking at right now. Deliberately scoped to open rows only
// (never closed/historical ones) so this stays consistent with that same
// "never rewrite the past" rule while still fixing the actual complaint:
// every row the identities table's "last known location" currently reads
// from IS an open row.
func (s *SecurityService) BackfillOpenConnectionGeoData(geoIP *GeoIPService) (int, error) {
	var openRows []model.IPConnectionLog
	if err := s.db.Where("disconnected_at IS NULL").Find(&openRows).Error; err != nil {
		s.logger.Error("failed to fetch open ip connection log rows for geo backfill", zap.Error(err))
		return 0, err
	}

	updated := 0
	for _, row := range openRows {
		geo, err := geoIP.Lookup(row.IPAddress)
		if err != nil {
			s.logger.Warn("geoip backfill lookup failed, leaving row unchanged", zap.String("ip", row.IPAddress), zap.Error(err))
			continue
		}
		if err := s.db.Model(&model.IPConnectionLog{}).Where("id = ?", row.ID).Updates(map[string]any{
			"country": geo.Country, "region": geo.Region, "city": geo.City,
			"lat": geo.Lat, "lon": geo.Lon, "timezone": geo.Timezone,
			"asn": geo.ASN, "isp": geo.ISP,
		}).Error; err != nil {
			s.logger.Error("failed to write backfilled geo data", zap.Uint("id", row.ID), zap.Error(err))
			continue
		}
		updated++
	}
	return updated, nil
}
