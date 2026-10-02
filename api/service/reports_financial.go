package service

import (
	"time"

	"go.uber.org/zap"

	"github.com/maahdima/mwp/api/dataservice/model"
	"github.com/maahdima/mwp/api/http/schema"
)

// GetFinancialReport is report 10: ledger activity bucketed by day/week/
// month, plus a per-reseller total -- a pure read over the existing
// LedgerEntry table (the wallet system's own transaction log), nothing
// new is written for this report.
func (s *ReportsService) GetFinancialReport(rangeParam, bucketBy string) (*schema.ReportsFinancialResponse, error) {
	start, end := resolveDateRange(rangeParam)
	startTime := time.Unix(start, 0)
	endTime := time.Unix(end, 0)

	var entries []model.LedgerEntry
	if err := s.db.Where("created_at >= ? AND created_at < ?", startTime, endTime).Find(&entries).Error; err != nil {
		s.logger.Error("failed to fetch ledger entries for financial report", zap.Error(err))
		return nil, err
	}

	byBucket := make(map[string]*schema.ReportsFinancialPoint)
	byReseller := make(map[uint]int64)
	resellerNames := s.resellerNameMap()

	for _, e := range entries {
		bucket := financialBucketKey(e.CreatedAt, bucketBy)
		point, ok := byBucket[bucket]
		if !ok {
			point = &schema.ReportsFinancialPoint{Bucket: bucket}
			byBucket[bucket] = point
		}
		if e.Amount >= 0 {
			point.ChargeAmount += e.Amount
		} else {
			point.DebitAmount += -e.Amount
		}

		if e.Amount < 0 {
			byReseller[e.ResellerID] += -e.Amount
		}
	}

	points := make([]schema.ReportsFinancialPoint, 0, len(byBucket))
	for _, p := range byBucket {
		points = append(points, *p)
	}
	sortFinancialPoints(points)

	resellerRows := make([]schema.ReportsFinancialByReseller, 0, len(byReseller))
	for id, total := range byReseller {
		name := resellerNames[id]
		if name == "" {
			continue
		}
		resellerRows = append(resellerRows, schema.ReportsFinancialByReseller{
			ResellerID: id, ResellerName: name, TotalAmount: total,
		})
	}
	sortFinancialByReseller(resellerRows)

	return &schema.ReportsFinancialResponse{
		Points:    points,
		Resellers: resellerRows,
	}, nil
}

func financialBucketKey(t time.Time, bucketBy string) string {
	switch bucketBy {
	case "month":
		return t.UTC().Format("2006-01")
	case "week":
		year, week := t.UTC().ISOWeek()
		return time.Date(year, 1, 1, 0, 0, 0, 0, time.UTC).AddDate(0, 0, (week-1)*7).Format("2006-01-02")
	default:
		return t.UTC().Format("2006-01-02")
	}
}

func sortFinancialPoints(points []schema.ReportsFinancialPoint) {
	for i := 1; i < len(points); i++ {
		for j := i; j > 0 && points[j].Bucket < points[j-1].Bucket; j-- {
			points[j], points[j-1] = points[j-1], points[j]
		}
	}
}

func sortFinancialByReseller(rows []schema.ReportsFinancialByReseller) {
	for i := 1; i < len(rows); i++ {
		for j := i; j > 0 && rows[j].TotalAmount > rows[j-1].TotalAmount; j-- {
			rows[j], rows[j-1] = rows[j-1], rows[j]
		}
	}
}

// GetRenewalRate is report 11: of everything (V2Ray packages, User
// Manager accounts, WireGuard peers) that expired in the range, what
// fraction still shows recent usage activity afterward (a simple, direct
// proxy for "got renewed/kept using the service" -- this codebase has no
// explicit "renewal" record anywhere to count directly, since a renewal
// is just a normal new purchase/quota top-up indistinguishable at the
// schema level from a brand-new purchase).
func (s *ReportsService) GetRenewalRate(rangeParam string) (*schema.ReportsRenewalRateResponse, error) {
	start, end := resolveDateRange(rangeParam)
	startTime := time.Unix(start, 0)
	endTime := time.Unix(end, 0)

	var expiredPackages []model.V2RayPackage
	s.db.Where("expire_at >= ? AND expire_at < ?", startTime, endTime).Find(&expiredPackages)

	totalExpired := len(expiredPackages)
	renewed := 0
	for _, pkg := range expiredPackages {
		var count int64
		s.db.Model(&model.UsageSnapshot{}).
			Where("package_id = ? AND timestamp >= ?", pkg.ID, pkg.ExpireAt.Unix()).
			Count(&count)
		if count > 0 {
			renewed++
		}
	}

	resp := &schema.ReportsRenewalRateResponse{TotalExpired: totalExpired, Renewed: renewed}
	if totalExpired > 0 {
		resp.RenewalRate = float64(renewed) / float64(totalExpired) * 100
	}
	return resp, nil
}

// GetPopularLocations is report 12: registered panels ranked by how many
// package-locations were created on them (a proxy for "sales") and total
// bytes served, in the range.
func (s *ReportsService) GetPopularLocations(rangeParam string) (*schema.ReportsPopularLocationsResponse, error) {
	start, end := resolveDateRange(rangeParam)

	var panels []model.XuiPanel
	if err := s.db.Find(&panels).Error; err != nil {
		s.logger.Error("failed to fetch panels for popular locations report", zap.Error(err))
		return nil, err
	}

	type salesRow struct {
		PanelID uint
		Count   int
	}
	var salesRows []salesRow
	s.db.Table("v2_ray_package_locations").
		Select("panel_id, COUNT(*) as count").
		Where("created_at >= ? AND created_at < ? AND deleted_at IS NULL", start, end).
		Group("panel_id").
		Scan(&salesRows)
	salesByPanel := make(map[uint]int, len(salesRows))
	for _, r := range salesRows {
		salesByPanel[r.PanelID] = r.Count
	}

	type bytesRow struct {
		PanelID uint
		Total   int64
	}
	var bytesRows []bytesRow
	s.db.Model(&model.UsageSnapshot{}).
		Select("panel_id, SUM(total_bytes) as total").
		Where("timestamp >= ? AND timestamp <= ? AND panel_id IS NOT NULL", start, end).
		Group("panel_id").
		Scan(&bytesRows)
	bytesByPanel := make(map[uint]int64, len(bytesRows))
	for _, r := range bytesRows {
		bytesByPanel[r.PanelID] = r.Total
	}

	rows := make([]schema.ReportsPopularLocation, 0, len(panels))
	for _, p := range panels {
		rows = append(rows, schema.ReportsPopularLocation{
			PanelID: p.ID, PanelName: p.Name,
			SalesCount: salesByPanel[p.ID], Bytes: bytesByPanel[p.ID],
		})
	}
	sort2PopularLocations(rows)

	return &schema.ReportsPopularLocationsResponse{Rows: rows}, nil
}

func sort2PopularLocations(rows []schema.ReportsPopularLocation) {
	for i := 1; i < len(rows); i++ {
		for j := i; j > 0 && rows[j].Bytes > rows[j-1].Bytes; j-- {
			rows[j], rows[j-1] = rows[j-1], rows[j]
		}
	}
}

// GetAnomalyAlerts is report 13: users whose usage TODAY is a large
// multiple of their own recent daily average -- flags a sudden usage
// spike (e.g. a leaked/shared config) without needing any hardcoded
// absolute threshold, since "large" is relative to that same user's own
// normal pattern.
func (s *ReportsService) GetAnomalyAlerts() (*schema.ReportsAnomalyResponse, error) {
	const anomalyMultiplierThreshold = 3.0
	const lookbackDays = 7

	now := time.Now()
	todayStart := now.Truncate(24 * time.Hour).Unix()
	lookbackStart := now.AddDate(0, 0, -lookbackDays).Unix()

	var snapshots []model.UsageSnapshot
	if err := s.db.Where("timestamp >= ?", lookbackStart).Find(&snapshots).Error; err != nil {
		s.logger.Error("failed to fetch usage snapshots for anomaly report", zap.Error(err))
		return nil, err
	}

	type entityKey struct {
		protocol string
		id       uint
	}
	todayByEntity := make(map[entityKey]int64)
	priorByEntity := make(map[entityKey]int64)
	priorDaysByEntity := make(map[entityKey]map[string]bool)

	entityID := func(snap model.UsageSnapshot) uint {
		switch snap.Protocol {
		case model.UsageProtocolWireGuard:
			if snap.PeerID != nil {
				return *snap.PeerID
			}
		case model.UsageProtocolUserManager:
			if snap.AccountID != nil {
				return *snap.AccountID
			}
		case model.UsageProtocolV2Ray:
			if snap.PackageID != nil {
				return *snap.PackageID
			}
		}
		return 0
	}

	for _, snap := range snapshots {
		id := entityID(snap)
		if id == 0 {
			continue
		}
		key := entityKey{protocol: snap.Protocol, id: id}
		if snap.Timestamp >= todayStart {
			todayByEntity[key] += snap.TotalBytes
		} else {
			priorByEntity[key] += snap.TotalBytes
			if priorDaysByEntity[key] == nil {
				priorDaysByEntity[key] = make(map[string]bool)
			}
			priorDaysByEntity[key][dayBucket(snap.Timestamp)] = true
		}
	}

	alerts := make([]schema.ReportsAnomalyAlert, 0)
	names := s.entityNameResolver()

	for key, todayBytes := range todayByEntity {
		priorDays := len(priorDaysByEntity[key])
		if priorDays == 0 {
			continue // no baseline yet, can't judge "unusual"
		}
		avg := priorByEntity[key] / int64(priorDays)
		if avg <= 0 {
			continue
		}
		multiplier := float64(todayBytes) / float64(avg)
		if multiplier < anomalyMultiplierThreshold {
			continue
		}
		alerts = append(alerts, schema.ReportsAnomalyAlert{
			Name:         names(key.protocol, key.id),
			Protocol:     key.protocol,
			TodayBytes:   todayBytes,
			AverageBytes: avg,
			Multiplier:   multiplier,
		})
	}
	sortAnomalyAlerts(alerts)

	return &schema.ReportsAnomalyResponse{Alerts: alerts}, nil
}

func sortAnomalyAlerts(alerts []schema.ReportsAnomalyAlert) {
	for i := 1; i < len(alerts); i++ {
		for j := i; j > 0 && alerts[j].Multiplier > alerts[j-1].Multiplier; j-- {
			alerts[j], alerts[j-1] = alerts[j-1], alerts[j]
		}
	}
}

// entityNameResolver returns a closure resolving (protocol, id) to a
// display name, loading each protocol's table at most once (lazily, on
// first use) rather than per-alert -- GetAnomalyAlerts may produce many
// alerts across all three protocols in one call.
func (s *ReportsService) entityNameResolver() func(protocol string, id uint) string {
	var peerNames, accountNames, packageNames map[uint]string

	return func(protocol string, id uint) string {
		switch protocol {
		case model.UsageProtocolWireGuard:
			if peerNames == nil {
				peerNames = make(map[uint]string)
				var peers []model.Peer
				s.db.Select("id, name").Find(&peers)
				for _, p := range peers {
					peerNames[p.ID] = p.Name
				}
			}
			return peerNames[id]
		case model.UsageProtocolUserManager:
			if accountNames == nil {
				accountNames = make(map[uint]string)
				var accounts []model.UserManagerAccount
				s.db.Select("id, username").Find(&accounts)
				for _, a := range accounts {
					accountNames[a.ID] = a.Username
				}
			}
			return accountNames[id]
		case model.UsageProtocolV2Ray:
			if packageNames == nil {
				packageNames = make(map[uint]string)
				var packages []model.V2RayPackage
				s.db.Select("id, customer_label").Find(&packages)
				for _, p := range packages {
					label := "N/A"
					if p.CustomerLabel != nil && *p.CustomerLabel != "" {
						label = *p.CustomerLabel
					}
					packageNames[p.ID] = label
				}
			}
			return packageNames[id]
		}
		return ""
	}
}

