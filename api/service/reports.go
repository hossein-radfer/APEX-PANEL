package service

import (
	"fmt"
	"sort"
	"time"

	"go.uber.org/zap"
	"gorm.io/gorm"

	"github.com/maahdima/mwp/api/dataservice/model"
	"github.com/maahdima/mwp/api/http/schema"
)

// ReportsService answers every read behind the admin-only "Reports"
// section -- every query here reads from data other jobs already
// maintain (model.UsageSnapshot, model.ResourceSample, and the existing
// Peer/UserManagerAccount/V2RayPackage/LedgerEntry tables); this service
// never writes anything and never calls any external system live.
type ReportsService struct {
	db     *gorm.DB
	logger *zap.Logger
}

func NewReportsService(db *gorm.DB) *ReportsService {
	return &ReportsService{
		db:     db,
		logger: zap.L().Named("ReportsService"),
	}
}

// resolveDateRange turns the frontend's "today"/"7d"/"30d" picker value
// into a concrete [start, end) Unix-second window, always ending at "now"
// -- every report's own query uses this same window, so switching the
// picker always means the same thing across every chart on the page.
func resolveDateRange(rangeParam string) (start, end int64) {
	now := time.Now()
	end = now.Unix()

	switch rangeParam {
	case schema.ReportsRangeToday:
		start = now.Truncate(24 * time.Hour).Unix()
	case schema.ReportsRange30Days:
		start = now.AddDate(0, 0, -30).Unix()
	default: // "7d" and any unrecognized value both default to 7 days
		start = now.AddDate(0, 0, -7).Unix()
	}
	return start, end
}

// dayBucket formats a Unix timestamp as a UTC calendar-day key, matching
// PeerDailyUsage.Date's own "YYYY-MM-DD" convention elsewhere in this
// codebase.
func dayBucket(unixSeconds int64) string {
	return time.Unix(unixSeconds, 0).UTC().Format("2006-01-02")
}

// hourBucket returns the UTC hour-of-day (0-23) a timestamp falls in --
// used to aggregate usage by time-of-day across every day in a range,
// rather than dayBucket's per-calendar-day grouping.
func hourBucket(unixSeconds int64) int {
	return time.Unix(unixSeconds, 0).UTC().Hour()
}

// GetDailyUsage is report 1: total usage per day in the range, broken
// down by protocol. protocolFilter is "" (all), "wireguard",
// "user_manager", or "v2ray".
func (s *ReportsService) GetDailyUsage(rangeParam, protocolFilter string) (*schema.ReportsDailyUsageResponse, error) {
	start, end := resolveDateRange(rangeParam)

	var snapshots []model.UsageSnapshot
	query := s.db.Where("timestamp >= ? AND timestamp <= ?", start, end)
	if protocolFilter != "" {
		query = query.Where("protocol = ?", protocolFilter)
	}
	if err := query.Find(&snapshots).Error; err != nil {
		s.logger.Error("failed to fetch usage snapshots for daily usage report", zap.Error(err))
		return nil, err
	}

	byDay := make(map[string]*schema.ReportsDailyUsagePoint)
	for _, snap := range snapshots {
		day := dayBucket(snap.Timestamp)
		point, ok := byDay[day]
		if !ok {
			point = &schema.ReportsDailyUsagePoint{Date: day}
			byDay[day] = point
		}
		switch snap.Protocol {
		case model.UsageProtocolWireGuard:
			point.WireGuardBytes += snap.TotalBytes
		case model.UsageProtocolUserManager:
			point.UserManagerBytes += snap.TotalBytes
		case model.UsageProtocolV2Ray:
			point.V2RayBytes += snap.TotalBytes
		}
		point.TotalBytes += snap.TotalBytes
	}

	points := make([]schema.ReportsDailyUsagePoint, 0, len(byDay))
	for _, p := range byDay {
		points = append(points, *p)
	}
	sort.Slice(points, func(i, j int) bool { return points[i].Date < points[j].Date })

	return &schema.ReportsDailyUsageResponse{Points: points}, nil
}

// GetPeakUsage is report 2: the single largest usage reading per day in
// the range -- system-wide when entityName is empty, or scoped to one
// searchable user (matched by Peer.Name/UserManagerAccount.Username/
// V2RayPackage.CustomerLabel) when provided.
func (s *ReportsService) GetPeakUsage(rangeParam, entityName string) (*schema.ReportsPeakUsageResponse, error) {
	start, end := resolveDateRange(rangeParam)

	query := s.db.Model(&model.UsageSnapshot{}).Where("timestamp >= ? AND timestamp <= ?", start, end)

	if entityName != "" {
		var peerIDs, accountIDs, packageIDs []uint
		s.db.Model(&model.Peer{}).Where("name LIKE ?", "%"+entityName+"%").Pluck("id", &peerIDs)
		s.db.Model(&model.UserManagerAccount{}).Where("username LIKE ?", "%"+entityName+"%").Pluck("id", &accountIDs)
		s.db.Model(&model.V2RayPackage{}).Where("customer_label LIKE ?", "%"+entityName+"%").Pluck("id", &packageIDs)

		query = query.Where(
			"(peer_id IN ? AND peer_id IS NOT NULL) OR (account_id IN ? AND account_id IS NOT NULL) OR (package_id IN ? AND package_id IS NOT NULL)",
			peerIDs, accountIDs, packageIDs,
		)
	}

	var snapshots []model.UsageSnapshot
	if err := query.Find(&snapshots).Error; err != nil {
		s.logger.Error("failed to fetch usage snapshots for peak usage report", zap.Error(err))
		return nil, err
	}

	byDay := make(map[string]int64)
	byHour := make(map[int]int64, 24)
	for _, snap := range snapshots {
		byDay[dayBucket(snap.Timestamp)] += snap.TotalBytes
		byHour[hourBucket(snap.Timestamp)] += snap.TotalBytes
	}

	points := make([]schema.ReportsPeakUsagePoint, 0, len(byDay))
	for day, bytes := range byDay {
		points = append(points, schema.ReportsPeakUsagePoint{Date: day, Bytes: bytes})
	}
	sort.Slice(points, func(i, j int) bool { return points[i].Date < points[j].Date })

	hourlyPoints := make([]schema.ReportsHourlyUsagePoint, 24)
	for h := 0; h < 24; h++ {
		hourlyPoints[h] = schema.ReportsHourlyUsagePoint{Hour: h, Bytes: byHour[h]}
	}

	resp := &schema.ReportsPeakUsageResponse{Points: points, HourlyPoints: hourlyPoints}
	for _, p := range points {
		if p.Bytes > resp.PeakBytes {
			pDate := p.Date
			resp.PeakDate = &pDate
			resp.PeakBytes = p.Bytes
		}
	}
	for _, hp := range hourlyPoints {
		if hp.Bytes > resp.PeakHourBytes {
			peakHour := hp.Hour
			resp.PeakHour = &peakHour
			resp.PeakHourBytes = hp.Bytes
		}
	}
	return resp, nil
}

// GetResellerUsageRanking is report 3: resellers ranked by total usage
// (sum across all three protocols) in the range.
func (s *ReportsService) GetResellerUsageRanking(rangeParam string) (*schema.ReportsRankingResponse, error) {
	start, end := resolveDateRange(rangeParam)

	type row struct {
		ResellerID uint
		Total      int64
	}
	var rows []row
	if err := s.db.Model(&model.UsageSnapshot{}).
		Select("reseller_id, SUM(total_bytes) as total").
		Where("timestamp >= ? AND timestamp <= ? AND reseller_id IS NOT NULL", start, end).
		Group("reseller_id").
		Scan(&rows).Error; err != nil {
		s.logger.Error("failed to fetch reseller usage ranking", zap.Error(err))
		return nil, err
	}

	resellerNames := s.resellerNameMap()

	result := make([]schema.ReportsRankingRow, 0, len(rows))
	for _, r := range rows {
		name := resellerNames[r.ResellerID]
		if name == "" {
			name = fmt.Sprintf("Reseller #%d", r.ResellerID)
		}
		result = append(result, schema.ReportsRankingRow{ID: r.ResellerID, Name: name, Bytes: r.Total})
	}
	sort.Slice(result, func(i, j int) bool { return result[i].Bytes > result[j].Bytes })

	return &schema.ReportsRankingResponse{Rows: result}, nil
}

// GetUserUsageRanking is report 4: individual users ranked by usage,
// scoped to one protocol (required -- there is no single shared "user"
// identity across all three products, see V2RayPackage's own doc comment
// on why these stay fully separate tables).
func (s *ReportsService) GetUserUsageRanking(rangeParam, protocol string) (*schema.ReportsRankingResponse, error) {
	start, end := resolveDateRange(rangeParam)

	switch protocol {
	case model.UsageProtocolWireGuard:
		return s.rankBy(start, end, model.UsageProtocolWireGuard, "peer_id", func(ids []uint) map[uint]string {
			var peers []model.Peer
			s.db.Where("id IN ?", ids).Find(&peers)
			m := make(map[uint]string, len(peers))
			for _, p := range peers {
				m[p.ID] = p.Name
			}
			return m
		})
	case model.UsageProtocolUserManager:
		return s.rankBy(start, end, model.UsageProtocolUserManager, "account_id", func(ids []uint) map[uint]string {
			var accounts []model.UserManagerAccount
			s.db.Where("id IN ?", ids).Find(&accounts)
			m := make(map[uint]string, len(accounts))
			for _, a := range accounts {
				m[a.ID] = a.Username
			}
			return m
		})
	case model.UsageProtocolV2Ray:
		return s.rankBy(start, end, model.UsageProtocolV2Ray, "package_id", func(ids []uint) map[uint]string {
			var packages []model.V2RayPackage
			s.db.Where("id IN ?", ids).Find(&packages)
			m := make(map[uint]string, len(packages))
			for _, p := range packages {
				name := "N/A"
				if p.CustomerLabel != nil && *p.CustomerLabel != "" {
					name = *p.CustomerLabel
				}
				m[p.ID] = name
			}
			return m
		})
	default:
		return nil, fmt.Errorf("unknown protocol %q", protocol)
	}
}

// rankBy is the shared aggregation behind GetUserUsageRanking's three
// protocol branches -- groups UsageSnapshot by idColumn, sums TotalBytes,
// then resolves each id to a display name via resolveNames (a small
// per-protocol lookup, since each protocol's "name" lives on a different
// table/column).
func (s *ReportsService) rankBy(start, end int64, protocol, idColumn string, resolveNames func([]uint) map[uint]string) (*schema.ReportsRankingResponse, error) {
	type row struct {
		ID    uint
		Total int64
	}
	var rows []row
	if err := s.db.Model(&model.UsageSnapshot{}).
		Select(idColumn+" as id, SUM(total_bytes) as total").
		Where("timestamp >= ? AND timestamp <= ? AND protocol = ?", start, end, protocol).
		Group(idColumn).
		Scan(&rows).Error; err != nil {
		s.logger.Error("failed to fetch user usage ranking", zap.String("protocol", protocol), zap.Error(err))
		return nil, err
	}

	ids := make([]uint, len(rows))
	for i, r := range rows {
		ids[i] = r.ID
	}
	names := resolveNames(ids)

	result := make([]schema.ReportsRankingRow, 0, len(rows))
	for _, r := range rows {
		name := names[r.ID]
		if name == "" {
			name = fmt.Sprintf("#%d", r.ID)
		}
		result = append(result, schema.ReportsRankingRow{ID: r.ID, Name: name, Protocol: protocol, Bytes: r.Total})
	}
	sort.Slice(result, func(i, j int) bool { return result[i].Bytes > result[j].Bytes })

	return &schema.ReportsRankingResponse{Rows: result}, nil
}

// GetResellerActivityRanking is report 5: resellers ranked by total
// activity (sum of their customers' usage + number of packages/accounts/
// peers created) in the range -- "activity" is deliberately usage PLUS
// package count, not usage alone (that's report 3), so a reseller who
// sells many small packages ranks differently than one with few large
// ones.
func (s *ReportsService) GetResellerActivityRanking(rangeParam string) (*schema.ReportsRankingResponse, error) {
	start, end := resolveDateRange(rangeParam)

	type usageRow struct {
		ResellerID uint
		Total      int64
	}
	var usageRows []usageRow
	if err := s.db.Model(&model.UsageSnapshot{}).
		Select("reseller_id, SUM(total_bytes) as total").
		Where("timestamp >= ? AND timestamp <= ? AND reseller_id IS NOT NULL", start, end).
		Group("reseller_id").
		Scan(&usageRows).Error; err != nil {
		s.logger.Error("failed to fetch reseller usage for activity ranking", zap.Error(err))
		return nil, err
	}
	usageByReseller := make(map[uint]int64, len(usageRows))
	for _, r := range usageRows {
		usageByReseller[r.ResellerID] = r.Total
	}

	// Model.CreatedAt is a Unix-seconds uint64 (see model.Model's own
	// BeforeCreate hook), not a SQL timestamp/time.Time column -- comparing
	// it against a time.Time value here would silently never match.
	type countRow struct {
		ResellerID uint
		Count      int
	}
	countByReseller := make(map[uint]int)
	addCounts := func(table string) {
		var rows []countRow
		s.db.Table(table).
			Select("reseller_id, COUNT(*) as count").
			Where("reseller_id IS NOT NULL AND created_at >= ? AND created_at < ? AND deleted_at IS NULL", start, end).
			Group("reseller_id").
			Scan(&rows)
		for _, r := range rows {
			countByReseller[r.ResellerID] += r.Count
		}
	}
	addCounts("peers")
	addCounts("user_manager_accounts")
	addCounts("v2_ray_packages")

	resellerNames := s.resellerNameMap()

	resellerIDs := make(map[uint]struct{})
	for id := range usageByReseller {
		resellerIDs[id] = struct{}{}
	}
	for id := range countByReseller {
		resellerIDs[id] = struct{}{}
	}

	result := make([]schema.ReportsRankingRow, 0, len(resellerIDs))
	for id := range resellerIDs {
		name := resellerNames[id]
		if name == "" {
			name = fmt.Sprintf("Reseller #%d", id)
		}
		result = append(result, schema.ReportsRankingRow{
			ID: id, Name: name, Bytes: usageByReseller[id], PackageCount: countByReseller[id],
		})
	}
	sort.Slice(result, func(i, j int) bool {
		// Ranked by usage first, package count as tiebreaker -- usage is
		// the dominant, harder-to-game signal of real activity.
		if result[i].Bytes != result[j].Bytes {
			return result[i].Bytes > result[j].Bytes
		}
		return result[i].PackageCount > result[j].PackageCount
	})

	return &schema.ReportsRankingResponse{Rows: result}, nil
}

func (s *ReportsService) resellerNameMap() map[uint]string {
	var resellers []model.Reseller
	s.db.Select("id, name").Find(&resellers)
	m := make(map[uint]string, len(resellers))
	for _, r := range resellers {
		m[r.ID] = r.Name
	}
	return m
}

// GetExpiringSoon is report 6: two independent lists -- entities expiring
// soon BY DATE (peers/accounts/packages whose expiry falls within the
// next 7 days) and entities expiring soon BY VOLUME (usage above 90% of
// their own quota), each broken down by protocol.
func (s *ReportsService) GetExpiringSoon() (*schema.ReportsExpiringResponse, error) {
	const daysAheadThreshold = 7
	const usagePercentThreshold = 90.0

	now := time.Now()
	soonCutoff := now.AddDate(0, 0, daysAheadThreshold)

	byDate := make([]schema.ReportsExpiringEntity, 0)
	byVolume := make([]schema.ReportsExpiringEntity, 0)

	// WireGuard peers: ExpireTime is a nullable "YYYY-MM-DD" string, no
	// volume-based expiry concept exists for WireGuard (TrafficLimit has
	// no "remaining %" surfaced anywhere in this codebase either), so
	// peers only ever appear in the by-date list.
	var peers []model.Peer
	s.db.Where("expire_time IS NOT NULL AND expire_time != '' AND disabled = ?", false).Find(&peers)
	for _, p := range peers {
		expireAt, err := time.Parse("2006-01-02", *p.ExpireTime)
		if err != nil || expireAt.After(soonCutoff) || expireAt.Before(now.AddDate(0, 0, -1)) {
			continue
		}
		days := int(time.Until(expireAt).Hours() / 24)
		if days < 0 {
			days = 0
		}
		expireStr := *p.ExpireTime
		byDate = append(byDate, schema.ReportsExpiringEntity{
			ID: p.ID, Name: p.Name, Protocol: model.UsageProtocolWireGuard,
			ResellerID: p.ResellerID, ExpireAt: &expireStr, DaysRemaining: &days,
		})
	}

	// User Manager accounts: same ExpireTime convention as Peer, plus a
	// TrafficLimit-vs-usage volume check.
	var accounts []model.UserManagerAccount
	s.db.Where("disabled = ?", false).Find(&accounts)
	for _, a := range accounts {
		if a.ExpireTime != nil && *a.ExpireTime != "" {
			if expireAt, err := time.Parse("2006-01-02", *a.ExpireTime); err == nil &&
				!expireAt.After(soonCutoff) && !expireAt.Before(now.AddDate(0, 0, -1)) {
				days := int(time.Until(expireAt).Hours() / 24)
				if days < 0 {
					days = 0
				}
				expireStr := *a.ExpireTime
				byDate = append(byDate, schema.ReportsExpiringEntity{
					ID: a.ID, Name: a.Username, Protocol: model.UsageProtocolUserManager,
					ResellerID: a.ResellerID, ExpireAt: &expireStr, DaysRemaining: &days,
				})
			}
		}
		if a.TrafficLimit != nil && *a.TrafficLimit > 0 {
			usedBytes := a.DownloadUsage + a.UploadUsage
			percent := float64(usedBytes) / float64(*a.TrafficLimit) * 100
			if percent >= usagePercentThreshold {
				remaining := *a.TrafficLimit - usedBytes
				if remaining < 0 {
					remaining = 0
				}
				byVolume = append(byVolume, schema.ReportsExpiringEntity{
					ID: a.ID, Name: a.Username, Protocol: model.UsageProtocolUserManager,
					ResellerID: a.ResellerID, UsagePercent: &percent, RemainingBytes: &remaining,
				})
			}
		}
	}

	// V2Ray packages: ExpireAt is a real *time.Time, plus a
	// TotalVolumeBytes-vs-summed-location-usage volume check.
	var packages []model.V2RayPackage
	s.db.Where("status = ?", "active").Find(&packages)
	for _, pkg := range packages {
		name := "N/A"
		if pkg.CustomerLabel != nil && *pkg.CustomerLabel != "" {
			name = *pkg.CustomerLabel
		}
		if pkg.ExpireAt != nil && !pkg.ExpireAt.After(soonCutoff) && !pkg.ExpireAt.Before(now.AddDate(0, 0, -1)) {
			days := int(time.Until(*pkg.ExpireAt).Hours() / 24)
			if days < 0 {
				days = 0
			}
			expireStr := pkg.ExpireAt.Format("2006-01-02")
			byDate = append(byDate, schema.ReportsExpiringEntity{
				ID: pkg.ID, Name: name, Protocol: model.UsageProtocolV2Ray,
				ResellerID: pkg.ResellerID, ExpireAt: &expireStr, DaysRemaining: &days,
			})
		}
		if pkg.TotalVolumeBytes > 0 {
			var usedBytes int64
			s.db.Model(&model.V2RayPackageLocation{}).
				Where("package_id = ?", pkg.ID).
				Select("COALESCE(SUM(used_bytes_cached), 0)").Scan(&usedBytes)
			percent := float64(usedBytes) / float64(pkg.TotalVolumeBytes) * 100
			if percent >= usagePercentThreshold {
				remaining := pkg.TotalVolumeBytes - usedBytes
				if remaining < 0 {
					remaining = 0
				}
				byVolume = append(byVolume, schema.ReportsExpiringEntity{
					ID: pkg.ID, Name: name, Protocol: model.UsageProtocolV2Ray,
					ResellerID: pkg.ResellerID, UsagePercent: &percent, RemainingBytes: &remaining,
				})
			}
		}
	}

	sortExpiringByDate(byDate)
	sortExpiringByVolume(byVolume)

	return &schema.ReportsExpiringResponse{
		ExpiringBySoonDate: byDate,
		ExpiringByVolume:   byVolume,
	}, nil
}

func sortExpiringByDate(rows []schema.ReportsExpiringEntity) {
	for i := 1; i < len(rows); i++ {
		for j := i; j > 0 && rows[j].DaysRemaining != nil && rows[j-1].DaysRemaining != nil &&
			*rows[j].DaysRemaining < *rows[j-1].DaysRemaining; j-- {
			rows[j], rows[j-1] = rows[j-1], rows[j]
		}
	}
}

func sortExpiringByVolume(rows []schema.ReportsExpiringEntity) {
	for i := 1; i < len(rows); i++ {
		for j := i; j > 0 && rows[j].UsagePercent != nil && rows[j-1].UsagePercent != nil &&
			*rows[j].UsagePercent > *rows[j-1].UsagePercent; j-- {
			rows[j], rows[j-1] = rows[j-1], rows[j]
		}
	}
}

// GetResourceUsage is report 15: the Mikrotik router's own CPU/memory
// sample history in the range, plus each metric's peak reading -- a pure
// read over model.ResourceSample (see ResourceSampler for the write
// side).
func (s *ReportsService) GetResourceUsage(rangeParam string) (*schema.ReportsResourceResponse, error) {
	start, end := resolveDateRange(rangeParam)

	var samples []model.ResourceSample
	if err := s.db.Where("timestamp >= ? AND timestamp <= ?", start, end).Order("timestamp ASC").Find(&samples).Error; err != nil {
		s.logger.Error("failed to fetch resource samples", zap.Error(err))
		return nil, err
	}

	resp := &schema.ReportsResourceResponse{Points: make([]schema.ReportsResourcePoint, 0, len(samples))}
	var cpuSum, memSum float64
	resp.LowestCPUPercent = -1
	resp.LowestMemoryPercent = -1
	for _, sample := range samples {
		resp.Points = append(resp.Points, schema.ReportsResourcePoint{
			Timestamp:         sample.Timestamp,
			CPULoadPercent:    sample.CPULoadPercent,
			MemoryUsedPercent: sample.MemoryUsedPercent,
		})
		if sample.CPULoadPercent > resp.PeakCPUPercent {
			resp.PeakCPUPercent = sample.CPULoadPercent
		}
		if sample.MemoryUsedPercent > resp.PeakMemoryPercent {
			resp.PeakMemoryPercent = sample.MemoryUsedPercent
		}
		if resp.LowestCPUPercent < 0 || sample.CPULoadPercent < resp.LowestCPUPercent {
			resp.LowestCPUPercent = sample.CPULoadPercent
		}
		if resp.LowestMemoryPercent < 0 || sample.MemoryUsedPercent < resp.LowestMemoryPercent {
			resp.LowestMemoryPercent = sample.MemoryUsedPercent
		}
		cpuSum += sample.CPULoadPercent
		memSum += sample.MemoryUsedPercent
	}
	if len(samples) > 0 {
		resp.AverageCPUPercent = cpuSum / float64(len(samples))
		resp.AverageMemoryPercent = memSum / float64(len(samples))
	} else {
		resp.LowestCPUPercent = 0
		resp.LowestMemoryPercent = 0
	}

	return resp, nil
}

// GetProtocolShare is report 7: each protocol's share of total usage in
// the range -- feeds the donut/pie chart.
func (s *ReportsService) GetProtocolShare(rangeParam string) (*schema.ReportsProtocolShareResponse, error) {
	start, end := resolveDateRange(rangeParam)

	type row struct {
		Protocol string
		Total    int64
	}
	var rows []row
	if err := s.db.Model(&model.UsageSnapshot{}).
		Select("protocol, SUM(total_bytes) as total").
		Where("timestamp >= ? AND timestamp <= ?", start, end).
		Group("protocol").
		Scan(&rows).Error; err != nil {
		s.logger.Error("failed to fetch protocol share", zap.Error(err))
		return nil, err
	}

	resp := &schema.ReportsProtocolShareResponse{}
	for _, r := range rows {
		switch r.Protocol {
		case model.UsageProtocolWireGuard:
			resp.WireGuardBytes = r.Total
		case model.UsageProtocolUserManager:
			resp.UserManagerBytes = r.Total
		case model.UsageProtocolV2Ray:
			resp.V2RayBytes = r.Total
		}
	}
	return resp, nil
}

// quotaPredictionLookbackDays is how far back GetResellerQuotaPrediction
// averages usage from -- 7 days, matching the frontend's own existing "7d"
// range-picker default (resolveDateRange's own default case) elsewhere in
// this file, so a reseller's "days remaining" figure reflects the same
// recent-usage window as every other chart in the Reports section, not an
// arbitrarily different one.
const quotaPredictionLookbackDays = 7

// GetResellerQuotaPrediction is category 2 item 2: "days until quota
// exhausted," per protocol, for the calling reseller's own dashboard.
// Deliberately reuses model.UsageSnapshot -- the same fine-grained,
// per-reseller, per-protocol delta history every existing traffic/sync job
// already writes as a side effect of its own quota bookkeeping (see
// UsageSnapshotWriter's own doc comment) -- rather than any new tracking
// table or job; this method is pure computation over data that already
// exists.
//
// AvgDailyUsageBytes is simply (sum of TotalBytes over the lookback
// window) / quotaPredictionLookbackDays -- NOT divided by however many
// days actually have snapshot rows, so a reseller who only started
// generating traffic 2 days ago (or whose snapshot history is still
// shallow, per UsageSnapshot's own "deliberately not backfilled" doc
// comment) gets a conservative, deflated rate rather than an inflated one
// from a tiny sample -- the projection only gets MORE accurate as more
// real days accrue, never wildly optimistic from a short window.
//
// DaysRemaining is nil (not zero, not a fabricated huge number) whenever a
// real projection can't be made: unlimited quota, or zero recent usage to
// extrapolate a rate from. This mirrors QuotaBytes' own nil-means-unlimited
// convention throughout this codebase (see model.Reseller.QuotaBytes'
// own doc comment) rather than inventing a number the underlying data
// can't actually support.
func (s *ReportsService) GetResellerQuotaPrediction(resellerID uint) (*schema.ResellerQuotaPredictionResponse, error) {
	var reseller model.Reseller
	if err := s.db.First(&reseller, resellerID).Error; err != nil {
		s.logger.Error("failed to fetch reseller for quota prediction", zap.Uint("resellerID", resellerID), zap.Error(err))
		return nil, err
	}

	lookbackStart := time.Now().AddDate(0, 0, -quotaPredictionLookbackDays).Unix()

	type protocolSum struct {
		Protocol string
		Total    int64
	}
	var rows []protocolSum
	if err := s.db.Model(&model.UsageSnapshot{}).
		Select("protocol, COALESCE(SUM(total_bytes), 0) as total").
		Where("reseller_id = ? AND timestamp >= ?", resellerID, lookbackStart).
		Group("protocol").
		Scan(&rows).Error; err != nil {
		s.logger.Error("failed to fetch usage snapshots for quota prediction", zap.Uint("resellerID", resellerID), zap.Error(err))
		return nil, err
	}

	recentTotals := make(map[string]int64, 3)
	for _, r := range rows {
		recentTotals[r.Protocol] = r.Total
	}

	protocols := []struct {
		key        string
		quotaBytes *int64
		usedBytes  int64
	}{
		{model.UsageProtocolWireGuard, reseller.QuotaBytes, reseller.UsedBytes},
		{model.UsageProtocolUserManager, reseller.UserManagerQuotaBytes, reseller.UserManagerUsedBytes},
		{model.UsageProtocolV2Ray, reseller.V2RayQuotaBytes, reseller.V2RayUsedBytes},
	}

	resp := &schema.ResellerQuotaPredictionResponse{
		Protocols: make([]schema.ResellerQuotaPredictionProtocol, 0, len(protocols)),
	}
	for _, p := range protocols {
		avgDaily := recentTotals[p.key] / quotaPredictionLookbackDays

		var remaining *int64
		var daysRemaining *int
		if p.quotaBytes != nil {
			r := *p.quotaBytes - p.usedBytes
			if r < 0 {
				r = 0
			}
			remaining = &r

			if avgDaily > 0 {
				days := int(r / avgDaily)
				daysRemaining = &days
			}
		}

		resp.Protocols = append(resp.Protocols, schema.ResellerQuotaPredictionProtocol{
			Protocol:           p.key,
			QuotaBytes:         p.quotaBytes,
			UsedBytes:          p.usedBytes,
			RemainingBytes:     remaining,
			AvgDailyUsageBytes: avgDaily,
			DaysRemaining:      daysRemaining,
		})
	}

	return resp, nil
}

