package schema

// ReportsDateRange is the shared query-param shape every report endpoint
// accepts -- "today" | "7d" | "30d", matching the frontend's own
// time-range picker. Resolved into a concrete [start,end) window in the
// service layer (see service/reports.go's resolveDateRange), never
// parsed twice.
type ReportsDateRangeParam = string

const (
	ReportsRangeToday = "today"
	ReportsRange7Days = "7d"
	ReportsRange30Days = "30d"
)

// ReportsDailyUsagePoint is one day's total usage, optionally split by
// protocol when a specific protocol filter is requested -- report 1.
type ReportsDailyUsagePoint struct {
	Date          string `json:"date"`
	WireGuardBytes   int64 `json:"wireguard_bytes"`
	UserManagerBytes int64 `json:"user_manager_bytes"`
	V2RayBytes       int64 `json:"v2ray_bytes"`
	TotalBytes       int64 `json:"total_bytes"`
}

type ReportsDailyUsageResponse struct {
	Points []ReportsDailyUsagePoint `json:"points"`
}

// ReportsPeakUsagePoint is one entity's single largest usage reading in
// the selected range -- report 2 (both the system-wide peak-per-day
// series and, when a specific user is searched, that one entity's own
// peak).
type ReportsPeakUsagePoint struct {
	Date  string `json:"date"`
	Bytes int64  `json:"bytes"`
}

// ReportsHourlyUsagePoint is one hour-of-day bucket (0-23, summed across
// every day in the selected range) -- lets an admin see e.g. "usage
// consistently peaks around 21:00" rather than only which single CALENDAR
// DAY had the most total traffic.
type ReportsHourlyUsagePoint struct {
	Hour  int   `json:"hour"`
	Bytes int64 `json:"bytes"`
}

type ReportsPeakUsageResponse struct {
	Points []ReportsPeakUsagePoint `json:"points"`
	// PeakDate/PeakBytes summarize the single highest point in Points, so
	// the frontend doesn't need to re-scan the series itself.
	PeakDate  *string `json:"peak_date,omitempty"`
	PeakBytes int64   `json:"peak_bytes"`
	// HourlyPoints/PeakHour/PeakHourBytes are the hour-of-day breakdown --
	// a confirmed, reported gap: the daily Points above answer "which day
	// was busiest" but not "what time of day is traffic heaviest," which
	// is the more actionable question for capacity planning.
	HourlyPoints  []ReportsHourlyUsagePoint `json:"hourly_points"`
	PeakHour      *int                      `json:"peak_hour,omitempty"`
	PeakHourBytes int64                     `json:"peak_hour_bytes"`
}

// ReportsRankingRow is one row of a ranking report (resellers by usage,
// users by usage, resellers by activity) -- report 3/4/5.
type ReportsRankingRow struct {
	ID         uint   `json:"id"`
	Name       string `json:"name"`
	Protocol   string `json:"protocol,omitempty"`
	Bytes      int64  `json:"bytes"`
	PackageCount int  `json:"package_count,omitempty"`
}

type ReportsRankingResponse struct {
	Rows []ReportsRankingRow `json:"rows"`
}

// ReportsExpiringEntity is one row of the expiring-soon report -- report 6.
type ReportsExpiringEntity struct {
	ID         uint    `json:"id"`
	Name       string  `json:"name"`
	Protocol   string  `json:"protocol"`
	ResellerID *uint   `json:"reseller_id,omitempty"`
	// ExpireAt/DaysRemaining populated for the time-based list;
	// UsagePercent for the volume-based list -- a single row is only ever
	// meaningful for the list it was returned in.
	ExpireAt      *string  `json:"expire_at,omitempty"`
	DaysRemaining *int     `json:"days_remaining,omitempty"`
	UsagePercent  *float64 `json:"usage_percent,omitempty"`
	// RemainingBytes is populated alongside UsagePercent on the volume-based
	// list -- the raw byte figure a percentage alone doesn't convey (e.g.
	// "90%" of a 5GB plan vs. "90%" of a 500GB plan represent very
	// different amounts of remaining headroom).
	RemainingBytes *int64 `json:"remaining_bytes,omitempty"`
}

type ReportsExpiringResponse struct {
	ExpiringBySoonDate []ReportsExpiringEntity `json:"expiring_by_soon_date"`
	ExpiringByVolume   []ReportsExpiringEntity `json:"expiring_by_volume"`
}

// ReportsProtocolShareResponse is the pie/donut-chart data for "share of
// total usage by protocol" -- report 7.
type ReportsProtocolShareResponse struct {
	WireGuardBytes   int64 `json:"wireguard_bytes"`
	UserManagerBytes int64 `json:"user_manager_bytes"`
	V2RayBytes       int64 `json:"v2ray_bytes"`
}

// ReportsOnlineUser is one currently-connected client -- report 8.
type ReportsOnlineUser struct {
	Name     string `json:"name"`
	Protocol string `json:"protocol"`
	Location string `json:"location,omitempty"`
}

type ReportsOnlineUsersResponse struct {
	TotalOnline int                  `json:"total_online"`
	Users       []ReportsOnlineUser `json:"users"`
}

// ReportsPanelHealthRow is one XuiPanel's own health row -- report 9.
type ReportsPanelHealthRow struct {
	PanelID       uint    `json:"panel_id"`
	PanelName     string  `json:"panel_name"`
	LastSyncedAt  *string `json:"last_synced_at,omitempty"`
	RecentErrorCount int  `json:"recent_error_count"`
	LocationCount int     `json:"location_count"`
	// RecentErrors is the actual error text behind RecentErrorCount --
	// previously only the count was surfaced, leaving an admin no way to
	// tell WHAT went wrong without going to the server logs directly.
	// Capped by GetPanelHealth to avoid an unbounded payload on a panel
	// with many failing locations.
	RecentErrors []string `json:"recent_errors,omitempty"`
}

type ReportsPanelHealthResponse struct {
	Rows []ReportsPanelHealthRow `json:"rows"`
}

// ReportsFinancialPoint is one bucket (day/week/month) of ledger activity
// -- report 10.
type ReportsFinancialPoint struct {
	Bucket        string `json:"bucket"`
	ChargeAmount  int64  `json:"charge_amount"`
	DebitAmount   int64  `json:"debit_amount"`
}

type ReportsFinancialByReseller struct {
	ResellerID   uint   `json:"reseller_id"`
	ResellerName string `json:"reseller_name"`
	TotalAmount  int64  `json:"total_amount"`
}

type ReportsFinancialResponse struct {
	Points     []ReportsFinancialPoint     `json:"points"`
	Resellers  []ReportsFinancialByReseller `json:"resellers"`
}

// ReportsRenewalRateResponse is report 11 -- percentage of expired
// packages/accounts/peers that got a fresh purchase/renewal shortly after
// expiry, vs. those that just lapsed.
type ReportsRenewalRateResponse struct {
	TotalExpired  int     `json:"total_expired"`
	Renewed       int     `json:"renewed"`
	RenewalRate   float64 `json:"renewal_rate"`
}

// ReportsPopularLocation is one row of the "most popular locations by
// sales/traffic" report -- report 12.
type ReportsPopularLocation struct {
	PanelID    uint   `json:"panel_id"`
	PanelName  string `json:"panel_name"`
	SalesCount int    `json:"sales_count"`
	Bytes      int64  `json:"bytes"`
}

type ReportsPopularLocationsResponse struct {
	Rows []ReportsPopularLocation `json:"rows"`
}

// ReportsAnomalyAlert is one row of the "unusual usage today" report --
// report 13: a user whose usage today is a large multiple of their own
// recent daily average.
type ReportsAnomalyAlert struct {
	Name         string  `json:"name"`
	Protocol     string  `json:"protocol"`
	TodayBytes   int64   `json:"today_bytes"`
	AverageBytes int64   `json:"average_bytes"`
	Multiplier   float64 `json:"multiplier"`
}

type ReportsAnomalyResponse struct {
	Alerts []ReportsAnomalyAlert `json:"alerts"`
}

// ReportsResourcePoint is one router CPU/memory sample -- report 15.
type ReportsResourcePoint struct {
	Timestamp         int64   `json:"timestamp"`
	CPULoadPercent    float64 `json:"cpu_load_percent"`
	MemoryUsedPercent float64 `json:"memory_used_percent"`
}

type ReportsResourceResponse struct {
	Points          []ReportsResourcePoint `json:"points"`
	PeakCPUPercent    float64 `json:"peak_cpu_percent"`
	PeakMemoryPercent float64 `json:"peak_memory_percent"`
	// AverageCPUPercent/AverageMemoryPercent/LowestCPUPercent/
	// LowestMemoryPercent round out the previously peak-only summary --
	// a confirmed, reported request: knowing only the worst reading gives
	// no sense of typical load or how much headroom exists on a quiet
	// reading.
	AverageCPUPercent    float64 `json:"average_cpu_percent"`
	AverageMemoryPercent float64 `json:"average_memory_percent"`
	LowestCPUPercent     float64 `json:"lowest_cpu_percent"`
	LowestMemoryPercent  float64 `json:"lowest_memory_percent"`
}

// ResellerQuotaPredictionProtocol is one protocol's "days until quota
// exhausted" projection for the calling reseller -- category 2 item 2.
// DaysRemaining is nil whenever a projection genuinely cannot be made
// (unlimited quota, i.e. QuotaBytes == nil; or AvgDailyUsageBytes == 0, a
// reseller with no recent usage to extrapolate from -- "0 recent usage"
// must never be misread as "infinite time left" the way dividing by zero
// would suggest, so it's surfaced as "unknown" instead of a fabricated
// number). AvgDailyUsageBytes is always populated (0 is a real, meaningful
// value here), computed from the last 7 days of model.UsageSnapshot rows
// actually on record for this reseller+protocol -- exactly the historical
// data every existing traffic/sync job already writes as a side effect of
// its own quota bookkeeping (see UsageSnapshotWriter), so this feature
// needed no new snapshot job.
type ResellerQuotaPredictionProtocol struct {
	Protocol           string `json:"protocol"`
	QuotaBytes         *int64 `json:"quota_bytes,omitempty"`
	UsedBytes          int64  `json:"used_bytes"`
	RemainingBytes     *int64 `json:"remaining_bytes,omitempty"`
	AvgDailyUsageBytes int64  `json:"avg_daily_usage_bytes"`
	DaysRemaining      *int   `json:"days_remaining,omitempty"`
}

type ResellerQuotaPredictionResponse struct {
	Protocols []ResellerQuotaPredictionProtocol `json:"protocols"`
}
