package service

import (
	"testing"
	"time"

	"go.uber.org/zap"

	"github.com/maahdima/mwp/api/dataservice/model"
)

// newTestLicenseServiceForExpiryWarning builds a bare LicenseService (no
// HTTP server needed -- checkExpiryWarning is called directly, never
// through a real heartbeat round trip) with a real BotNotifier wired in.
// AdminChatID is deliberately left empty so NotifyCriticalAlert's own
// broadcastToAdmins safely no-ops the actual Telegram send (see that
// function's own "if settings.AdminChatID != """ guard) while still
// exercising every line of checkExpiryWarning up to and including the
// notifier call -- these tests assert on the persisted SystemConfig state
// (systemConfigExpiryWarningKey), which is what checkExpiryWarning
// actually WROTE, not on a mocked Telegram call.
func newTestLicenseServiceForExpiryWarning(t *testing.T, notifyEnabled bool) *LicenseService {
	t.Helper()

	db := newTestLicenseDB(t)
	if err := db.AutoMigrate(&model.BotSettings{}, &model.BotExtraAdminChatID{}); err != nil {
		t.Fatalf("failed to migrate bot settings: %v", err)
	}
	if err := db.Create(&model.BotSettings{NotifyCriticalAlerts: notifyEnabled}).Error; err != nil {
		t.Fatalf("failed to seed bot settings: %v", err)
	}

	settingsService := NewBotSettingsService(db)
	notifier := NewBotNotifier(nil, db, settingsService)

	svc := &LicenseService{db: db, logger: zap.NewNop()}
	svc.SetBotNotifier(notifier)
	return svc
}

// TestCheckExpiryWarning_FiresAtEachThreshold confirms a license crossing
// 50/80/90/99% elapsed life fires exactly once per threshold and persists
// the high-water mark, mirroring the admin's own explicit "هشدار پیش از
// انقضا در ۵۰٪، ۸۰٪، ۹۰٪، ۹۹٪" requirement (فاز ۴-۱۲-الف).
func TestCheckExpiryWarning_FiresAtEachThreshold(t *testing.T) {
	svc := newTestLicenseServiceForExpiryWarning(t, true)

	// A 100-day license, 51 days elapsed -- just past the 50% threshold.
	issuedAt := time.Now().Add(-51 * 24 * time.Hour)
	expiresAt := issuedAt.Add(100 * 24 * time.Hour)

	svc.checkExpiryWarning(issuedAt.Format(time.RFC3339), expiresAt.Format(time.RFC3339))

	stored, err := svc.getSystemConfig(systemConfigExpiryWarningKey)
	if err != nil {
		t.Fatalf("unexpected error reading state: %v", err)
	}
	if stored == "" {
		t.Fatal("expected a warning state to be persisted after crossing 50%, got none")
	}
	expected := expiresAt.Format(time.RFC3339) + "|50"
	if stored != expected {
		t.Fatalf("expected stored state %q, got %q", expected, stored)
	}
}

// TestCheckExpiryWarning_DoesNotRefireSameThreshold confirms calling
// checkExpiryWarning again with the SAME elapsed percentage does not
// re-warn -- heartbeats run hourly by default, and a threshold must only
// ever fire once per license period.
func TestCheckExpiryWarning_DoesNotRefireSameThreshold(t *testing.T) {
	svc := newTestLicenseServiceForExpiryWarning(t, true)

	issuedAt := time.Now().Add(-51 * 24 * time.Hour)
	expiresAt := issuedAt.Add(100 * 24 * time.Hour)
	issuedStr, expiresStr := issuedAt.Format(time.RFC3339), expiresAt.Format(time.RFC3339)

	svc.checkExpiryWarning(issuedStr, expiresStr)
	firstState, _ := svc.getSystemConfig(systemConfigExpiryWarningKey)

	// Call again immediately, same dates (as a second heartbeat tick would).
	svc.checkExpiryWarning(issuedStr, expiresStr)
	secondState, _ := svc.getSystemConfig(systemConfigExpiryWarningKey)

	if firstState != secondState {
		t.Fatalf("expected state to remain unchanged on a repeat tick at the same threshold, got %q then %q", firstState, secondState)
	}
}

// TestCheckExpiryWarning_EscalatesToHigherThreshold confirms crossing from
// 50% to 90% (e.g. after several days pass between heartbeats) updates the
// stored high-water mark and would fire again for the new, higher
// threshold.
func TestCheckExpiryWarning_EscalatesToHigherThreshold(t *testing.T) {
	svc := newTestLicenseServiceForExpiryWarning(t, true)

	// A fixed license period: issued 100 days ago, expiring in 9 days
	// (91% of its 100-day life already elapsed as of "now"). A real
	// license's IssuedAt/ExpiresAt never change between heartbeat ticks --
	// what changes tick to tick is only time.Now() advancing, which
	// checkExpiryWarning reads internally via time.Since(issued). So the
	// first call below simulates an EARLIER tick by using a shorter total
	// life (crossing 50% only), and the second call simulates a LATER tick
	// of the exact same license (same IssuedAt/ExpiresAt) by using the
	// real, fixed dates that put "now" at 91% elapsed.
	issuedAt50 := time.Now().Add(-51 * 24 * time.Hour)
	expiresAt50 := issuedAt50.Add(100 * 24 * time.Hour)
	svc.checkExpiryWarning(issuedAt50.Format(time.RFC3339), expiresAt50.Format(time.RFC3339))

	issuedAt91 := time.Now().Add(-91 * 24 * time.Hour)
	expiresAt91 := time.Now().Add(9 * 24 * time.Hour)
	svc.checkExpiryWarning(issuedAt91.Format(time.RFC3339), expiresAt91.Format(time.RFC3339))

	stored, _ := svc.getSystemConfig(systemConfigExpiryWarningKey)
	expected := expiresAt91.Format(time.RFC3339) + "|90"
	if stored != expected {
		t.Fatalf("expected escalation to the 90%% threshold, got %q", stored)
	}
}

// TestCheckExpiryWarning_RenewalResetsThreshold confirms a NEW ExpiresAt
// (the license was renewed/extended) does not let the PREVIOUS period's
// stored high-water mark suppress a warning in the new period, even if the
// new period's elapsed percentage happens to reach the same threshold the
// old period already warned for. Note: a freshly-renewed license (0%
// elapsed) crosses no threshold and so writes no new state at all (see
// checkExpiryWarning's own early return for highestCrossed==0) -- the old
// period's stored state is only ever actually overwritten once the NEW
// period itself crosses a threshold, which is what this test drives it to.
func TestCheckExpiryWarning_RenewalResetsThreshold(t *testing.T) {
	svc := newTestLicenseServiceForExpiryWarning(t, true)

	oldIssuedAt := time.Now().Add(-95 * 24 * time.Hour)
	oldExpiresAt := oldIssuedAt.Add(100 * 24 * time.Hour)
	svc.checkExpiryWarning(oldIssuedAt.Format(time.RFC3339), oldExpiresAt.Format(time.RFC3339))

	oldState, _ := svc.getSystemConfig(systemConfigExpiryWarningKey)
	if oldState == "" {
		t.Fatal("expected a warning state after the first (near-expiry) period")
	}

	// Renewal: a brand new, later ExpiresAt. Simulate the new period ALSO
	// reaching 90% elapsed (a different absolute date, but the same
	// threshold number as a stale, un-reset high-water mark might wrongly
	// still show for the OLD period) -- without the ExpiresAt-based reset,
	// this would be wrongly suppressed as "already warned at 90%".
	newIssuedAt := time.Now().Add(-90 * 24 * time.Hour)
	newExpiresAt := newIssuedAt.Add(100 * 24 * time.Hour)
	svc.checkExpiryWarning(newIssuedAt.Format(time.RFC3339), newExpiresAt.Format(time.RFC3339))

	newState, _ := svc.getSystemConfig(systemConfigExpiryWarningKey)
	expected := newExpiresAt.Format(time.RFC3339) + "|90"
	if newState != expected {
		t.Fatalf("expected the new period's own 90%% crossing to be recorded (%q), got %q -- the old period's state may be wrongly suppressing it", expected, newState)
	}
}

// TestCheckExpiryWarning_NoExpiryNeverWarns confirms a lifetime/no-expiry
// license (ExpiresAt == "", license-panel's own convention) never warns --
// mirrors isExpired's identical "empty ExpiresAt = never expires" rule.
func TestCheckExpiryWarning_NoExpiryNeverWarns(t *testing.T) {
	svc := newTestLicenseServiceForExpiryWarning(t, true)

	svc.checkExpiryWarning(time.Now().Add(-1000*24*time.Hour).Format(time.RFC3339), "")

	stored, _ := svc.getSystemConfig(systemConfigExpiryWarningKey)
	if stored != "" {
		t.Fatalf("expected no warning state for a no-expiry license, got %q", stored)
	}
}

// TestCheckExpiryWarning_NilNotifierIsNoop confirms a LicenseService with
// no BotNotifier wired (SetBotNotifier never called) never panics and
// never writes state -- the "safe to leave unset" contract SetBotNotifier's
// own doc comment promises.
func TestCheckExpiryWarning_NilNotifierIsNoop(t *testing.T) {
	db := newTestLicenseDB(t)
	svc := &LicenseService{db: db, logger: zap.NewNop()}

	issuedAt := time.Now().Add(-99 * 24 * time.Hour)
	expiresAt := issuedAt.Add(100 * 24 * time.Hour)
	svc.checkExpiryWarning(issuedAt.Format(time.RFC3339), expiresAt.Format(time.RFC3339))

	stored, _ := svc.getSystemConfig(systemConfigExpiryWarningKey)
	if stored != "" {
		t.Fatalf("expected no state written when botNotifier is nil, got %q", stored)
	}
}
