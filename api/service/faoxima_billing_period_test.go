package service

import (
	"fmt"
	"testing"
	"time"

	"github.com/glebarez/sqlite"
	"gorm.io/gorm"

	"github.com/maahdima/mwp/api/dataservice/model"
)

func openFaoximaPeriodTestDB(t *testing.T) *gorm.DB {
	t.Helper()
	dsn := fmt.Sprintf("file:faoxima_period_test_%d?mode=memory&cache=shared", time.Now().UnixNano())
	db, err := gorm.Open(sqlite.Open(dsn), &gorm.Config{})
	if err != nil {
		t.Fatalf("failed to open test db: %v", err)
	}
	if err := db.AutoMigrate(&model.FaoximaInstance{}, &model.Reseller{}, &model.BotSettings{}); err != nil {
		t.Fatalf("failed to migrate test db: %v", err)
	}
	return db
}

// TestAdminEnableWithPeriod_StampsBillingFields is the core regression
// test for the admin's own explicit request: "یک دکمه فعال یا غیر فعال
// برام بزار و به همراه دوره و ریست دوره" (a per-reseller enable/disable
// button along with a billing period and period-reset). Confirms
// AdminEnableWithPeriod both re-enables the instance AND stamps
// BillingPeriodDays/PeriodActivatedAt so CheckExpiredPeriods can later
// enforce it.
func TestAdminEnableWithPeriod_StampsBillingFields(t *testing.T) {
	db := openFaoximaPeriodTestDB(t)
	fake := newFakeTelegramAPI(nil)
	withFakeTelegramAPI(t, fake)

	instance := model.FaoximaInstance{
		ResellerID: 1, InstanceSlug: "reseller1", DBName: "d1", DBUser: "u1", DBPassword: "pw",
		BotToken: "TOKEN_A", WebhookSecret: "secret-a", Status: model.FaoximaInstanceStatusDisabled,
	}
	if err := db.Create(&instance).Error; err != nil {
		t.Fatalf("failed to create instance: %v", err)
	}

	svc := NewFaoximaProvisionerService(db)
	svc.faoximaDomain = testFaoximaDomain
	if err := svc.AdminEnableWithPeriod(1, 30); err != nil {
		t.Fatalf("AdminEnableWithPeriod failed: %v", err)
	}

	var reloaded model.FaoximaInstance
	if err := db.Where("reseller_id = ?", 1).First(&reloaded).Error; err != nil {
		t.Fatalf("failed to reload instance: %v", err)
	}
	if reloaded.Status != model.FaoximaInstanceStatusEnabled {
		t.Errorf("expected status ENABLED, got %q", reloaded.Status)
	}
	if reloaded.BillingPeriodDays == nil || *reloaded.BillingPeriodDays != 30 {
		t.Errorf("expected BillingPeriodDays=30, got %v", reloaded.BillingPeriodDays)
	}
	if reloaded.PeriodActivatedAt == nil {
		t.Error("expected PeriodActivatedAt to be stamped, got nil")
	}
}

// TestAdminEnableWithPeriod_ZeroDaysMeansNoPeriod confirms periodDays <=
// 0 clears any billing period (runs indefinitely), matching the
// documented convention.
func TestAdminEnableWithPeriod_ZeroDaysMeansNoPeriod(t *testing.T) {
	db := openFaoximaPeriodTestDB(t)
	fake := newFakeTelegramAPI(nil)
	withFakeTelegramAPI(t, fake)

	instance := model.FaoximaInstance{
		ResellerID: 1, InstanceSlug: "reseller1", DBName: "d1", DBUser: "u1", DBPassword: "pw",
		BotToken: "TOKEN_A", WebhookSecret: "secret-a", Status: model.FaoximaInstanceStatusDisabled,
	}
	if err := db.Create(&instance).Error; err != nil {
		t.Fatalf("failed to create instance: %v", err)
	}

	svc := NewFaoximaProvisionerService(db)
	svc.faoximaDomain = testFaoximaDomain
	if err := svc.AdminEnableWithPeriod(1, 0); err != nil {
		t.Fatalf("AdminEnableWithPeriod failed: %v", err)
	}

	var reloaded model.FaoximaInstance
	if err := db.Where("reseller_id = ?", 1).First(&reloaded).Error; err != nil {
		t.Fatalf("failed to reload instance: %v", err)
	}
	if reloaded.BillingPeriodDays != nil {
		t.Errorf("expected BillingPeriodDays nil for periodDays<=0, got %v", *reloaded.BillingPeriodDays)
	}
	if reloaded.PeriodActivatedAt != nil {
		t.Errorf("expected PeriodActivatedAt nil for periodDays<=0, got %v", *reloaded.PeriodActivatedAt)
	}
}

// TestCheckExpiredPeriods_DisablesExpiredInstance confirms an ENABLED
// instance whose PeriodActivatedAt + BillingPeriodDays has elapsed is
// disabled -- the actual enforcement half of the billing-period feature.
func TestCheckExpiredPeriods_DisablesExpiredInstance(t *testing.T) {
	db := openFaoximaPeriodTestDB(t)
	fake := newFakeTelegramAPI(map[string]string{
		"TOKEN_EXPIRED": fmt.Sprintf("https://%s/faoxima-resellers/reseller-expired/index.php", testFaoximaDomain),
	})
	withFakeTelegramAPI(t, fake)

	activatedAt := time.Now().AddDate(0, 0, -31) // 31 days ago
	periodDays := 30
	instance := model.FaoximaInstance{
		ResellerID: 2, InstanceSlug: "reseller-expired", DBName: "d2", DBUser: "u2", DBPassword: "pw",
		BotToken: "TOKEN_EXPIRED", WebhookSecret: "secret-b", Status: model.FaoximaInstanceStatusEnabled,
		BillingPeriodDays: &periodDays, PeriodActivatedAt: &activatedAt,
	}
	if err := db.Create(&instance).Error; err != nil {
		t.Fatalf("failed to create instance: %v", err)
	}

	svc := NewFaoximaProvisionerService(db)
	svc.faoximaDomain = testFaoximaDomain
	svc.CheckExpiredPeriods()

	var reloaded model.FaoximaInstance
	if err := db.Where("reseller_id = ?", 2).First(&reloaded).Error; err != nil {
		t.Fatalf("failed to reload instance: %v", err)
	}
	if reloaded.Status != model.FaoximaInstanceStatusDisabled {
		t.Errorf("expected status DISABLED after period expiry, got %q", reloaded.Status)
	}

	fake.mu.Lock()
	defer fake.mu.Unlock()
	if fake.registeredURLs["TOKEN_EXPIRED"] != "" {
		t.Errorf("expected webhook to be unregistered (empty) after expiry, got %q", fake.registeredURLs["TOKEN_EXPIRED"])
	}
}

// TestCheckExpiredPeriods_LeavesActivePeriodAlone confirms an instance
// still within its billing period is never touched.
func TestCheckExpiredPeriods_LeavesActivePeriodAlone(t *testing.T) {
	db := openFaoximaPeriodTestDB(t)
	fake := newFakeTelegramAPI(nil)
	withFakeTelegramAPI(t, fake)

	activatedAt := time.Now().AddDate(0, 0, -5) // only 5 days ago
	periodDays := 30
	instance := model.FaoximaInstance{
		ResellerID: 3, InstanceSlug: "reseller-active", DBName: "d3", DBUser: "u3", DBPassword: "pw",
		BotToken: "TOKEN_ACTIVE", WebhookSecret: "secret-c", Status: model.FaoximaInstanceStatusEnabled,
		BillingPeriodDays: &periodDays, PeriodActivatedAt: &activatedAt,
	}
	if err := db.Create(&instance).Error; err != nil {
		t.Fatalf("failed to create instance: %v", err)
	}

	svc := NewFaoximaProvisionerService(db)
	svc.faoximaDomain = testFaoximaDomain
	svc.CheckExpiredPeriods()

	var reloaded model.FaoximaInstance
	if err := db.Where("reseller_id = ?", 3).First(&reloaded).Error; err != nil {
		t.Fatalf("failed to reload instance: %v", err)
	}
	if reloaded.Status != model.FaoximaInstanceStatusEnabled {
		t.Errorf("expected status to remain ENABLED while period is still active, got %q", reloaded.Status)
	}
}

// TestCheckExpiredPeriods_IgnoresInstancesWithNoPeriodConfigured confirms
// an ENABLED instance with no BillingPeriodDays at all (nil = runs
// indefinitely) is never disabled -- existing instances created before
// this feature must never be unexpectedly cut off by a deploy.
func TestCheckExpiredPeriods_IgnoresInstancesWithNoPeriodConfigured(t *testing.T) {
	db := openFaoximaPeriodTestDB(t)
	fake := newFakeTelegramAPI(nil)
	withFakeTelegramAPI(t, fake)

	instance := model.FaoximaInstance{
		ResellerID: 4, InstanceSlug: "reseller-unlimited", DBName: "d4", DBUser: "u4", DBPassword: "pw",
		BotToken: "TOKEN_UNLIMITED", WebhookSecret: "secret-d", Status: model.FaoximaInstanceStatusEnabled,
	}
	if err := db.Create(&instance).Error; err != nil {
		t.Fatalf("failed to create instance: %v", err)
	}

	svc := NewFaoximaProvisionerService(db)
	svc.faoximaDomain = testFaoximaDomain
	svc.CheckExpiredPeriods()

	var reloaded model.FaoximaInstance
	if err := db.Where("reseller_id = ?", 4).First(&reloaded).Error; err != nil {
		t.Fatalf("failed to reload instance: %v", err)
	}
	if reloaded.Status != model.FaoximaInstanceStatusEnabled {
		t.Errorf("expected an instance with no billing period configured to stay ENABLED, got %q", reloaded.Status)
	}
}

// TestAdminResetPeriod_RestartsCountdownAndReenables is the regression
// test for the admin's own "ریست دوره" action: an instance disabled by
// period expiry must be re-enabled AND have its countdown restarted from
// now, once the admin confirms the reseller paid for their next period.
func TestAdminResetPeriod_RestartsCountdownAndReenables(t *testing.T) {
	db := openFaoximaPeriodTestDB(t)
	fake := newFakeTelegramAPI(nil)
	withFakeTelegramAPI(t, fake)

	oldActivatedAt := time.Now().AddDate(0, 0, -31)
	periodDays := 30
	instance := model.FaoximaInstance{
		ResellerID: 5, InstanceSlug: "reseller-reset", DBName: "d5", DBUser: "u5", DBPassword: "pw",
		BotToken: "TOKEN_RESET", WebhookSecret: "secret-e", Status: model.FaoximaInstanceStatusDisabled,
		BillingPeriodDays: &periodDays, PeriodActivatedAt: &oldActivatedAt,
	}
	if err := db.Create(&instance).Error; err != nil {
		t.Fatalf("failed to create instance: %v", err)
	}

	svc := NewFaoximaProvisionerService(db)
	svc.faoximaDomain = testFaoximaDomain
	beforeReset := time.Now()
	if err := svc.AdminResetPeriod(5); err != nil {
		t.Fatalf("AdminResetPeriod failed: %v", err)
	}

	var reloaded model.FaoximaInstance
	if err := db.Where("reseller_id = ?", 5).First(&reloaded).Error; err != nil {
		t.Fatalf("failed to reload instance: %v", err)
	}
	if reloaded.Status != model.FaoximaInstanceStatusEnabled {
		t.Errorf("expected status ENABLED after reset, got %q", reloaded.Status)
	}
	if reloaded.PeriodActivatedAt == nil || reloaded.PeriodActivatedAt.Before(beforeReset) {
		t.Errorf("expected PeriodActivatedAt to be restarted to now, got %v (reset called at %v)", reloaded.PeriodActivatedAt, beforeReset)
	}
	if reloaded.BillingPeriodDays == nil || *reloaded.BillingPeriodDays != 30 {
		t.Errorf("expected BillingPeriodDays to remain 30, got %v", reloaded.BillingPeriodDays)
	}
}

// TestAdminResetPeriod_RejectsInstanceWithNoPeriodConfigured confirms
// resetting a period that was never configured is a clear error, not a
// silent no-op or a surprising side effect.
func TestAdminResetPeriod_RejectsInstanceWithNoPeriodConfigured(t *testing.T) {
	db := openFaoximaPeriodTestDB(t)

	instance := model.FaoximaInstance{
		ResellerID: 6, InstanceSlug: "reseller-no-period", DBName: "d6", DBUser: "u6", DBPassword: "pw",
		BotToken: "TOKEN_NO_PERIOD", WebhookSecret: "secret-f", Status: model.FaoximaInstanceStatusEnabled,
	}
	if err := db.Create(&instance).Error; err != nil {
		t.Fatalf("failed to create instance: %v", err)
	}

	svc := NewFaoximaProvisionerService(db)
	svc.faoximaDomain = testFaoximaDomain
	if err := svc.AdminResetPeriod(6); err == nil {
		t.Fatal("expected an error when resetting a period that was never configured, got nil")
	}
}
