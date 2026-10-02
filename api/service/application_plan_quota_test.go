package service

import (
	"fmt"
	"testing"
	"time"

	"github.com/glebarez/sqlite"
	"go.uber.org/zap"
	"gorm.io/gorm"

	"github.com/maahdima/mwp/api/dataservice/model"
	"github.com/maahdima/mwp/api/http/schema"
)

func newTestApplicationServiceForPlanQuota(t *testing.T) (*ApplicationService, *gorm.DB) {
	t.Helper()

	dsn := fmt.Sprintf("file:app_plan_quota_test_%d?mode=memory&cache=shared", time.Now().UnixNano())
	db, err := gorm.Open(sqlite.Open(dsn), &gorm.Config{})
	if err != nil {
		t.Fatalf("failed to open test db: %v", err)
	}

	if err := db.AutoMigrate(
		&model.Reseller{},
		&model.Application{},
		&model.ApplicationPlan{},
	); err != nil {
		t.Fatalf("failed to migrate test db: %v", err)
	}

	svc := &ApplicationService{db: db, logger: zap.NewNop()}
	svc.SetPlanService(NewApplicationPlanService(db))
	return svc, db
}

// TestApplyPlanIfSet_FillsOnlyZeroFields is the regression test for
// applyPlanIfSet's core "manual override wins" contract, mirroring
// DNSAccountService's identical applyPlanIfSet behavior: a plan fills in
// whatever the request left at zero/nil, but never overwrites a value the
// caller explicitly set.
func TestApplyPlanIfSet_FillsOnlyZeroFields(t *testing.T) {
	svc, db := newTestApplicationServiceForPlanQuota(t)

	downloadLimit := 50
	plan := model.ApplicationPlan{
		Name: "Gold", TotalVolumeBytes: 100 * 1024 * 1024 * 1024, DurationDays: 30,
		MaxOnlineUsers: 3, DownloadSpeedLimitMbps: &downloadLimit, IsActive: true,
	}
	if err := db.Create(&plan).Error; err != nil {
		t.Fatalf("failed to create plan: %v", err)
	}

	// Case 1: everything zero -- plan fills all of it.
	var totalVolumeBytes int64
	var durationDays, maxOnlineUsers int
	var dl, ul *int
	svc.applyPlanIfSet(&plan.ID, &totalVolumeBytes, &durationDays, &maxOnlineUsers, &dl, &ul)
	if totalVolumeBytes != plan.TotalVolumeBytes || durationDays != plan.DurationDays || maxOnlineUsers != plan.MaxOnlineUsers {
		t.Fatalf("expected plan bundle to fill all zero fields, got volume=%d duration=%d online=%d", totalVolumeBytes, durationDays, maxOnlineUsers)
	}
	if dl == nil || *dl != downloadLimit {
		t.Fatalf("expected plan's download speed limit to fill nil field, got %v", dl)
	}

	// Case 2: caller explicitly set TotalVolumeBytes and MaxOnlineUsers --
	// those must survive untouched; DurationDays (left zero) still gets
	// filled from the plan.
	explicitVolume := int64(5 * 1024 * 1024 * 1024)
	explicitOnline := 1
	var durationDays2 int
	svc.applyPlanIfSet(&plan.ID, &explicitVolume, &durationDays2, &explicitOnline, nil, nil)
	if explicitVolume != 5*1024*1024*1024 {
		t.Fatalf("expected explicit TotalVolumeBytes to survive untouched, got %d", explicitVolume)
	}
	if explicitOnline != 1 {
		t.Fatalf("expected explicit MaxOnlineUsers to survive untouched, got %d", explicitOnline)
	}
	if durationDays2 != plan.DurationDays {
		t.Fatalf("expected DurationDays (left zero) to be filled from plan, got %d", durationDays2)
	}
}

// TestApplyPlanIfSet_MissingPlanIsNoop confirms a stale/deleted plan
// reference never blocks the caller -- mirrors
// DNSAccountService.applyPlanIfSet's identical "missing plan silently
// no-ops" contract.
func TestApplyPlanIfSet_MissingPlanIsNoop(t *testing.T) {
	svc, _ := newTestApplicationServiceForPlanQuota(t)

	missingID := uint(99999)
	totalVolumeBytes := int64(0)
	durationDays, maxOnlineUsers := 0, 0
	svc.applyPlanIfSet(&missingID, &totalVolumeBytes, &durationDays, &maxOnlineUsers, nil, nil)

	if totalVolumeBytes != 0 || durationDays != 0 || maxOnlineUsers != 0 {
		t.Fatalf("expected a missing plan reference to leave every field untouched, got volume=%d duration=%d online=%d", totalVolumeBytes, durationDays, maxOnlineUsers)
	}
}

// TestCreateApplication_PlanFillsFieldsAndPassesValidation is an end-to-end
// check that a plan-only CreateApplicationRequest (no manual
// volume/duration/online-user values) succeeds and the created Application
// carries the plan's bundle -- this is the schema-level half of the fix
// (required_without=ApplicationPlanID), exercised through the real
// CreateApplication path, not just the validator tag in isolation.
func TestCreateApplication_PlanFillsFieldsAndPassesValidation(t *testing.T) {
	svc, db := newTestApplicationServiceForPlanQuota(t)

	plan := model.ApplicationPlan{
		Name: "Bronze", TotalVolumeBytes: 10 * 1024 * 1024 * 1024, DurationDays: 15,
		MaxOnlineUsers: 2, IsActive: true,
	}
	if err := db.Create(&plan).Error; err != nil {
		t.Fatalf("failed to create plan: %v", err)
	}

	app, err := svc.CreateApplication(&schema.CreateApplicationRequest{
		Name:              "plan-test-app",
		ApplicationPlanID: &plan.ID,
	}, nil)
	if err != nil {
		t.Fatalf("failed to create application from plan: %v", err)
	}

	if app.TotalVolumeBytes != plan.TotalVolumeBytes {
		t.Fatalf("expected TotalVolumeBytes from plan (%d), got %d", plan.TotalVolumeBytes, app.TotalVolumeBytes)
	}
	if app.DurationDays != plan.DurationDays {
		t.Fatalf("expected DurationDays from plan (%d), got %d", plan.DurationDays, app.DurationDays)
	}
	if app.MaxOnlineUsers != plan.MaxOnlineUsers {
		t.Fatalf("expected MaxOnlineUsers from plan (%d), got %d", plan.MaxOnlineUsers, app.MaxOnlineUsers)
	}

	var stored model.Application
	if err := db.First(&stored, "app_username = ?", app.AppUsername).Error; err != nil {
		t.Fatalf("failed to reload created application: %v", err)
	}
	if stored.ApplicationPlanID == nil || *stored.ApplicationPlanID != plan.ID {
		t.Fatalf("expected ApplicationPlanID to be recorded on the created row, got %v", stored.ApplicationPlanID)
	}
}

// TestApplyResellerApplicationQuota_SuspendsOverQuotaApplications confirms
// the reseller-level pool suspends every one of a reseller's Applications
// once their combined UsedBytes crosses ApplicationQuotaBytes -- mirrors
// DNSSyncService.applyResellerDNSQuota's own core test shape.
func TestApplyResellerApplicationQuota_SuspendsOverQuotaApplications(t *testing.T) {
	svc, db := newTestApplicationServiceForPlanQuota(t)

	quota := int64(100)
	reseller := model.Reseller{Name: "over-quota-apps", Username: "over-quota-apps", PasswordHash: "x", ApplicationQuotaBytes: &quota}
	if err := db.Create(&reseller).Error; err != nil {
		t.Fatalf("failed to create reseller: %v", err)
	}

	app := model.Application{
		ResellerID: &reseller.ID, Name: "a1", AppUsername: "a1user", AppPassword: "x",
		TotalVolumeBytes: 1000, DurationDays: 30, UsedBytes: 150, Status: "active",
	}
	if err := db.Create(&app).Error; err != nil {
		t.Fatalf("failed to create application: %v", err)
	}

	svc.applyResellerApplicationQuota(reseller.ID)

	var reloadedApp model.Application
	if err := db.First(&reloadedApp, app.ID).Error; err != nil {
		t.Fatalf("failed to reload application: %v", err)
	}
	if reloadedApp.Status != "suspended" || !reloadedApp.SuspendedByResellerQuota {
		t.Fatalf("expected application to be suspended by reseller quota, got status=%q suspended_by_reseller_quota=%v",
			reloadedApp.Status, reloadedApp.SuspendedByResellerQuota)
	}

	var reloadedReseller model.Reseller
	if err := db.First(&reloadedReseller, reseller.ID).Error; err != nil {
		t.Fatalf("failed to reload reseller: %v", err)
	}
	if reloadedReseller.ApplicationUsedBytes != 150 {
		t.Fatalf("expected reseller ApplicationUsedBytes to be updated to the live sum (150), got %d", reloadedReseller.ApplicationUsedBytes)
	}
}

// TestApplyResellerApplicationQuota_ResumesUnderQuotaApplications confirms
// an Application previously suspended by the reseller pool comes back once
// usage drops back under quota (or the quota is raised) -- and that a
// manually-suspended Application (SuspendedByResellerQuota=false) is left
// alone, matching resumeApplication's own "never resurrect something
// suspended on purpose" guarantee.
func TestApplyResellerApplicationQuota_ResumesUnderQuotaApplications(t *testing.T) {
	svc, db := newTestApplicationServiceForPlanQuota(t)

	quota := int64(1000)
	reseller := model.Reseller{Name: "under-quota-apps", Username: "under-quota-apps", PasswordHash: "x", ApplicationQuotaBytes: &quota}
	if err := db.Create(&reseller).Error; err != nil {
		t.Fatalf("failed to create reseller: %v", err)
	}

	quotaSuspended := model.Application{
		ResellerID: &reseller.ID, Name: "was-suspended", AppUsername: "wassuspended", AppPassword: "x",
		TotalVolumeBytes: 5000, DurationDays: 30, UsedBytes: 100, Status: "suspended",
		SuspendedByResellerQuota: true, WasActiveBeforeSuspend: true,
	}
	if err := db.Create(&quotaSuspended).Error; err != nil {
		t.Fatalf("failed to create quota-suspended application: %v", err)
	}

	manuallySuspended := model.Application{
		ResellerID: &reseller.ID, Name: "manual", AppUsername: "manualuser", AppPassword: "x",
		TotalVolumeBytes: 5000, DurationDays: 30, UsedBytes: 50, Status: "suspended",
		SuspendedByResellerQuota: false, Disabled: true,
	}
	if err := db.Create(&manuallySuspended).Error; err != nil {
		t.Fatalf("failed to create manually-suspended application: %v", err)
	}

	svc.applyResellerApplicationQuota(reseller.ID)

	var reloadedQuota model.Application
	if err := db.First(&reloadedQuota, quotaSuspended.ID).Error; err != nil {
		t.Fatalf("failed to reload quota-suspended application: %v", err)
	}
	if reloadedQuota.Status != "active" || reloadedQuota.SuspendedByResellerQuota {
		t.Fatalf("expected quota-suspended application to resume, got status=%q suspended_by_reseller_quota=%v",
			reloadedQuota.Status, reloadedQuota.SuspendedByResellerQuota)
	}

	var reloadedManual model.Application
	if err := db.First(&reloadedManual, manuallySuspended.ID).Error; err != nil {
		t.Fatalf("failed to reload manually-suspended application: %v", err)
	}
	if reloadedManual.Status != "suspended" {
		t.Fatalf("expected manually-suspended application (not suspended by reseller quota) to remain suspended, got status=%q", reloadedManual.Status)
	}
}

// TestApplyResellerApplicationQuota_UnlimitedNeverSuspends confirms a nil
// ApplicationQuotaBytes (unlimited) skips the suspend/resume pass entirely,
// mirroring DNS/V2Ray's identical "nil quota = unlimited, function returns
// early" convention.
func TestApplyResellerApplicationQuota_UnlimitedNeverSuspends(t *testing.T) {
	svc, db := newTestApplicationServiceForPlanQuota(t)

	reseller := model.Reseller{Name: "unlimited-apps", Username: "unlimited-apps", PasswordHash: "x"}
	if err := db.Create(&reseller).Error; err != nil {
		t.Fatalf("failed to create reseller: %v", err)
	}

	app := model.Application{
		ResellerID: &reseller.ID, Name: "huge", AppUsername: "hugeuser", AppPassword: "x",
		TotalVolumeBytes: 1_000_000, DurationDays: 30, UsedBytes: 999_000, Status: "active",
	}
	if err := db.Create(&app).Error; err != nil {
		t.Fatalf("failed to create application: %v", err)
	}

	svc.applyResellerApplicationQuota(reseller.ID)

	var reloaded model.Application
	if err := db.First(&reloaded, app.ID).Error; err != nil {
		t.Fatalf("failed to reload application: %v", err)
	}
	if reloaded.Status != "active" {
		t.Fatalf("expected application under an unlimited (nil) reseller quota to remain active, got status=%q", reloaded.Status)
	}
}
