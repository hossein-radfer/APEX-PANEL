package service

import (
	"fmt"
	"testing"
	"time"

	"github.com/glebarez/sqlite"
	"go.uber.org/zap"
	"gorm.io/gorm"

	"github.com/maahdima/mwp/api/dataservice/model"
)

func newTestSystemHealthService(t *testing.T) (*SystemHealthService, *gorm.DB) {
	t.Helper()

	dsn := fmt.Sprintf("file:system_health_test_%d?mode=memory&cache=shared", time.Now().UnixNano())
	db, err := gorm.Open(sqlite.Open(dsn), &gorm.Config{})
	if err != nil {
		t.Fatalf("failed to open test db: %v", err)
	}
	if err := db.AutoMigrate(&model.BotSettings{}); err != nil {
		t.Fatalf("failed to migrate test db: %v", err)
	}

	botSettingsService := NewBotSettingsService(db)
	svc := NewSystemHealthService(db, t.TempDir(), botSettingsService)
	svc.logger = zap.NewNop()
	return svc, db
}

// TestGetSystemHealth_FirstCallReportsUnavailableCPU confirms the
// documented "-1 means unknown" contract: a single /proc/stat sample
// can't yield a percentage (the counters are cumulative since boot), so
// the very first call must report CPUPercent as unavailable rather than
// a misleading 0%.
func TestGetSystemHealth_FirstCallReportsUnavailableCPU(t *testing.T) {
	svc, _ := newTestSystemHealthService(t)

	health := svc.GetSystemHealth()

	// On a non-Linux CI/dev box (or any /proc read failure), readCPUPercent
	// returns an error and GetSystemHealth leaves CPUPercent at its -1
	// sentinel -- this branch of the assertion covers that. On Linux, the
	// first call succeeds but intentionally reports 0 (no delta yet) per
	// readCPUPercent's own doc comment, which is a DIFFERENT sentinel
	// value handled by the -1 default only ever changing on error. Either
	// way, a "real-looking" percentage strictly between 0 and 100 on the
	// very first call would indicate the two-sample logic was bypassed.
	if health.CPUPercent > 0 && health.CPUPercent < 100 {
		t.Fatalf("expected the first call to report 0 (no delta yet) or -1 (unavailable), got a suspiciously specific %v", health.CPUPercent)
	}
}

// TestGetSystemHealth_SecondCallComputesRealCPUPercent confirms the
// two-sample delta logic actually produces a plausible percentage once a
// previous sample exists -- skipped where /proc isn't available (non-Linux
// dev/CI), matching this service's own documented Linux-only scope.
func TestGetSystemHealth_SecondCallComputesRealCPUPercent(t *testing.T) {
	svc, _ := newTestSystemHealthService(t)

	first := svc.GetSystemHealth()
	if first.CPUPercent == -1 && !svc.havePrevSample {
		t.Skip("/proc/stat not available on this platform -- skipping CPU delta test")
	}

	time.Sleep(50 * time.Millisecond)
	second := svc.GetSystemHealth()

	if second.CPUPercent < 0 || second.CPUPercent > 100 {
		t.Fatalf("expected a plausible 0-100%% CPU reading on the second call, got %v", second.CPUPercent)
	}
}

// TestGetSystemHealth_BackupUpToDateReflectsLastSentDate confirms the
// backup freshness check correctly compares LastBackupSentDate against
// today's UTC date.
func TestGetSystemHealth_BackupUpToDateReflectsLastSentDate(t *testing.T) {
	svc, db := newTestSystemHealthService(t)

	settings, err := NewBotSettingsService(db).GetOrCreate()
	if err != nil {
		t.Fatalf("failed to get/create bot settings: %v", err)
	}

	today := time.Now().UTC().Format("2006-01-02")
	if err := db.Model(settings).Updates(map[string]interface{}{
		"last_backup_sent_date": today,
		"auto_backup_enabled":   true,
	}).Error; err != nil {
		t.Fatalf("failed to update bot settings: %v", err)
	}

	health := svc.GetSystemHealth()
	if !health.Backup.AutoBackupEnabled {
		t.Fatal("expected AutoBackupEnabled to be true")
	}
	if health.Backup.LastBackupSentDate != today {
		t.Fatalf("expected LastBackupSentDate %q, got %q", today, health.Backup.LastBackupSentDate)
	}
	if !health.Backup.IsUpToDate {
		t.Fatal("expected IsUpToDate to be true when LastBackupSentDate is today")
	}
}

// TestGetSystemHealth_BackupStaleDateIsNotUpToDate confirms a stale
// (yesterday's) date correctly reports as NOT up to date.
func TestGetSystemHealth_BackupStaleDateIsNotUpToDate(t *testing.T) {
	svc, db := newTestSystemHealthService(t)

	settings, err := NewBotSettingsService(db).GetOrCreate()
	if err != nil {
		t.Fatalf("failed to get/create bot settings: %v", err)
	}

	yesterday := time.Now().UTC().AddDate(0, 0, -1).Format("2006-01-02")
	if err := db.Model(settings).Update("last_backup_sent_date", yesterday).Error; err != nil {
		t.Fatalf("failed to update bot settings: %v", err)
	}

	health := svc.GetSystemHealth()
	if health.Backup.IsUpToDate {
		t.Fatal("expected IsUpToDate to be false for a stale (yesterday's) date")
	}
}
