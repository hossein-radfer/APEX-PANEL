package service

import (
	"time"

	"go.uber.org/zap"

	"github.com/maahdima/mwp/api/dataservice/model"
)

// Detection windows/thresholds -- fixed constants for now rather than
// admin-configurable settings, pending real production data to tune them.
const (
	// threatWindow is the lookback window shared by every detector below.
	threatWindow = 1 * time.Hour

	// threatSharedAccountMinIdentities flags an account/peer used from
	// this many or more distinct IPs within threatWindow.
	threatSharedAccountMinIdentities = 4

	// threatMultiCountryMinCountries flags a single identity connecting
	// from this many or more distinct countries within threatWindow.
	threatMultiCountryMinCountries = 2

	// threatScanningMinIdentities flags a single source IP that has
	// connected as this many or more distinct identities within
	// threatWindow.
	threatScanningMinIdentities = 5
)

// SecurityThreat is one row of the Security page's "تهدیدات" (Threats)
// panel -- a single detected anomaly, not a raw log row.
type SecurityThreat struct {
	// Kind is one of "shared_account", "multi_country", "scanning" --
	// see the three detector functions below for exactly what each means.
	Kind string
	// Subject is the flagged entity: an identity (username/peer name) for
	// shared_account/multi_country, or an IP address for scanning.
	Subject string
	// Detail is a short, already-Persian-formatted human explanation of
	// why this was flagged, e.g. "۵ آی‌پی متفاوت در ۱ ساعت اخیر" -- the
	// frontend displays this directly rather than reassembling it from
	// raw counts, so any future change to the wording only has one place
	// to change.
	Detail string
	// Severity is "high" or "medium" -- drives the frontend's badge
	// color. Kept coarse (two levels, not a numeric score) since a
	// numeric score would imply a precision this heuristic-based
	// detection doesn't actually have.
	Severity     string
	Country      *string
	City         *string
	LastSeenAt   time.Time
	OccurrenceCount int
}

// GetThreats runs all three detectors over the last threatWindow of
// IPConnectionLog activity and returns their combined findings, most
// severe/most recent first. A pure DB read (like every other SecurityService
// method), computed fresh on every call rather than cached -- these queries
// are cheap aggregates over an already-indexed, time-bounded slice of one
// table, not a full-table scan, so there is no need for a background job or
// a stored result the way EtherTrafficSample's own torch polling needs one.
func (s *SecurityService) GetThreats() ([]SecurityThreat, error) {
	since := time.Now().Add(-threatWindow)

	threats := make([]SecurityThreat, 0, 16)

	sharedAccountThreats, err := s.detectSharedAccounts(since)
	if err != nil {
		s.logger.Error("failed to run shared-account threat detection", zap.Error(err))
		return nil, err
	}
	threats = append(threats, sharedAccountThreats...)

	multiCountryThreats, err := s.detectMultiCountryIdentities(since)
	if err != nil {
		s.logger.Error("failed to run multi-country threat detection", zap.Error(err))
		return nil, err
	}
	threats = append(threats, multiCountryThreats...)

	scanningThreats, err := s.detectScanningSources(since)
	if err != nil {
		s.logger.Error("failed to run scanning-source threat detection", zap.Error(err))
		return nil, err
	}
	threats = append(threats, scanningThreats...)

	return threats, nil
}

// detectSharedAccounts flags any (protocol, identity) that connected from
// threatSharedAccountMinIdentities or more DISTINCT IP addresses within the
// window -- the classic "one credential, many simultaneous/rotating users"
// signature of a shared or leaked account.
func (s *SecurityService) detectSharedAccounts(since time.Time) ([]SecurityThreat, error) {
	type row struct {
		Identity      string
		IPCount       int
		LastConnected time.Time
		Country       *string
		City          *string
	}
	var rows []row
	if err := s.db.Model(&model.IPConnectionLog{}).
		Select("identity, COUNT(DISTINCT ip_address) AS ip_count, MAX(connected_at) AS last_connected, "+
			"(SELECT country FROM ip_connection_logs il2 WHERE il2.identity = ip_connection_logs.identity ORDER BY il2.connected_at DESC LIMIT 1) AS country, "+
			"(SELECT city FROM ip_connection_logs il2 WHERE il2.identity = ip_connection_logs.identity ORDER BY il2.connected_at DESC LIMIT 1) AS city").
		Where("connected_at >= ?", since).
		Group("identity").
		Having("COUNT(DISTINCT ip_address) >= ?", threatSharedAccountMinIdentities).
		Order("ip_count DESC").
		Scan(&rows).Error; err != nil {
		return nil, err
	}

	threats := make([]SecurityThreat, 0, len(rows))
	for _, r := range rows {
		severity := "medium"
		if r.IPCount >= threatSharedAccountMinIdentities*2 {
			severity = "high"
		}
		threats = append(threats, SecurityThreat{
			Kind:            "shared_account",
			Subject:         r.Identity,
			Detail:          formatPersianDetail(r.IPCount, "آی‌پی متفاوت در ۱ ساعت اخیر"),
			Severity:        severity,
			Country:         r.Country,
			City:            r.City,
			LastSeenAt:      r.LastConnected,
			OccurrenceCount: r.IPCount,
		})
	}
	return threats, nil
}

// detectMultiCountryIdentities flags any identity whose connections within
// the window span threatMultiCountryMinCountries or more distinct
// countries -- a stronger, harder-to-explain-away signal than distinct-IP
// alone (see threatMultiCountryMinCountries' own doc comment).
func (s *SecurityService) detectMultiCountryIdentities(since time.Time) ([]SecurityThreat, error) {
	type row struct {
		Identity      string
		CountryCount  int
		LastConnected time.Time
		Country       *string
		City          *string
	}
	var rows []row
	if err := s.db.Model(&model.IPConnectionLog{}).
		Select("identity, COUNT(DISTINCT country) AS country_count, MAX(connected_at) AS last_connected, "+
			"(SELECT country FROM ip_connection_logs il2 WHERE il2.identity = ip_connection_logs.identity ORDER BY il2.connected_at DESC LIMIT 1) AS country, "+
			"(SELECT city FROM ip_connection_logs il2 WHERE il2.identity = ip_connection_logs.identity ORDER BY il2.connected_at DESC LIMIT 1) AS city").
		Where("connected_at >= ? AND country IS NOT NULL", since).
		Group("identity").
		Having("COUNT(DISTINCT country) >= ?", threatMultiCountryMinCountries).
		Order("country_count DESC").
		Scan(&rows).Error; err != nil {
		return nil, err
	}

	threats := make([]SecurityThreat, 0, len(rows))
	for _, r := range rows {
		severity := "medium"
		if r.CountryCount >= threatMultiCountryMinCountries+2 {
			severity = "high"
		}
		threats = append(threats, SecurityThreat{
			Kind:            "multi_country",
			Subject:         r.Identity,
			Detail:          formatPersianDetail(r.CountryCount, "کشور متفاوت در ۱ ساعت اخیر"),
			Severity:        severity,
			Country:         r.Country,
			City:            r.City,
			LastSeenAt:      r.LastConnected,
			OccurrenceCount: r.CountryCount,
		})
	}
	return threats, nil
}

// detectScanningSources flags any source IP that has authenticated as
// threatScanningMinIdentities or more distinct identities within the
// window -- see threatScanningMinIdentities' own doc comment for why this
// shape reads as scanning/brute-force rather than legitimate shared
// infrastructure.
func (s *SecurityService) detectScanningSources(since time.Time) ([]SecurityThreat, error) {
	type row struct {
		IPAddress     string
		IdentityCount int
		LastConnected time.Time
		Country       *string
		City          *string
	}
	var rows []row
	if err := s.db.Model(&model.IPConnectionLog{}).
		Select("ip_address, COUNT(DISTINCT identity) AS identity_count, MAX(connected_at) AS last_connected, "+
			"(SELECT country FROM ip_connection_logs il2 WHERE il2.ip_address = ip_connection_logs.ip_address ORDER BY il2.connected_at DESC LIMIT 1) AS country, "+
			"(SELECT city FROM ip_connection_logs il2 WHERE il2.ip_address = ip_connection_logs.ip_address ORDER BY il2.connected_at DESC LIMIT 1) AS city").
		Where("connected_at >= ?", since).
		Group("ip_address").
		Having("COUNT(DISTINCT identity) >= ?", threatScanningMinIdentities).
		Order("identity_count DESC").
		Scan(&rows).Error; err != nil {
		return nil, err
	}

	threats := make([]SecurityThreat, 0, len(rows))
	for _, r := range rows {
		severity := "medium"
		if r.IdentityCount >= threatScanningMinIdentities*2 {
			severity = "high"
		}
		threats = append(threats, SecurityThreat{
			Kind:            "scanning",
			Subject:         r.IPAddress,
			Detail:          formatPersianDetail(r.IdentityCount, "هویت متفاوت از یک آی‌پی در ۱ ساعت اخیر"),
			Severity:        severity,
			Country:         r.Country,
			City:            r.City,
			LastSeenAt:      r.LastConnected,
			OccurrenceCount: r.IdentityCount,
		})
	}
	return threats, nil
}

// formatPersianDetail renders "<count> <suffix>" with the count written
// using Persian digits, matching this codebase's own convention of never
// showing raw Latin digits in admin-facing Persian text.
func formatPersianDetail(count int, suffix string) string {
	return toPersianDigits(count) + " " + suffix
}

var persianDigits = [10]rune{'۰', '۱', '۲', '۳', '۴', '۵', '۶', '۷', '۸', '۹'}

func toPersianDigits(n int) string {
	if n == 0 {
		return string(persianDigits[0])
	}
	negative := n < 0
	if negative {
		n = -n
	}
	var digits []rune
	for n > 0 {
		digits = append([]rune{persianDigits[n%10]}, digits...)
		n /= 10
	}
	if negative {
		digits = append([]rune{'-'}, digits...)
	}
	return string(digits)
}
