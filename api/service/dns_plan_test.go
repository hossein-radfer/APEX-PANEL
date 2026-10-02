package service

import (
	"fmt"
	"net/http"
	"net/http/httptest"
	"testing"
	"time"

	"github.com/glebarez/sqlite"
	"gorm.io/gorm"

	"github.com/maahdima/mwp/api/dataservice/model"
	"github.com/maahdima/mwp/api/http/schema"
)

func openDNSPlanTestDB(t *testing.T) *gorm.DB {
	t.Helper()
	dsn := fmt.Sprintf("file:dns_plan_test_%d?mode=memory&cache=shared", time.Now().UnixNano())
	db, err := gorm.Open(sqlite.Open(dsn), &gorm.Config{})
	if err != nil {
		t.Fatalf("failed to open test db: %v", err)
	}
	if err := db.AutoMigrate(
		&model.DNSPlan{},
		&model.DNSPanel{},
		&model.DNSAccount{},
		&model.DNSAccountAllowedCountry{},
		&model.DNSIPRegistrationLog{},
		&model.Reseller{},
		&model.IPGeoCache{},
		&model.SystemConfig{},
	); err != nil {
		t.Fatalf("failed to migrate test db: %v", err)
	}
	return db
}

func TestDNSPlanService_CreateListUpdateDelete(t *testing.T) {
	db := openDNSPlanTestDB(t)
	svc := NewDNSPlanService(db)

	created, err := svc.CreatePlan(&schema.CreateDNSPlanRequest{
		Name:                     "برنزی",
		PriceAmount:              50000,
		TotalVolumeBytes:         10 * 1024 * 1024 * 1024,
		SpeedKbps:                1024,
		DurationDays:             30,
		MaxConcurrentIPs:         2,
		DailyIPRegistrationLimit: 3,
	})
	if err != nil {
		t.Fatalf("CreatePlan failed: %v", err)
	}
	if created.Id == 0 {
		t.Fatal("expected a non-zero plan id")
	}
	if !created.IsActive {
		t.Fatal("expected a newly created plan to be active")
	}

	plans, err := svc.ListPlans()
	if err != nil {
		t.Fatalf("ListPlans failed: %v", err)
	}
	if len(plans) != 1 {
		t.Fatalf("expected 1 plan, got %d", len(plans))
	}

	newName := "نقره‌ای"
	newPrice := int64(80000)
	updated, err := svc.UpdatePlan(created.Id, &schema.UpdateDNSPlanRequest{
		Name:        &newName,
		PriceAmount: &newPrice,
	})
	if err != nil {
		t.Fatalf("UpdatePlan failed: %v", err)
	}
	if updated.Name != newName || updated.PriceAmount != newPrice {
		t.Fatalf("expected updated name/price, got %+v", updated)
	}
	// Fields not touched by the update request must survive unchanged.
	if updated.MaxConcurrentIPs != 2 {
		t.Fatalf("expected untouched MaxConcurrentIPs to remain 2, got %d", updated.MaxConcurrentIPs)
	}

	if err := svc.DeletePlan(created.Id); err != nil {
		t.Fatalf("DeletePlan failed: %v", err)
	}
	plans, err = svc.ListPlans()
	if err != nil {
		t.Fatalf("ListPlans after delete failed: %v", err)
	}
	if len(plans) != 0 {
		t.Fatalf("expected 0 plans after delete, got %d", len(plans))
	}
}

// TestDNSAccountService_CreateAccount_AppliesPlanBundle is the core
// regression test for phase 4-3's DNS tier system: selecting a plan on
// account creation must copy its bundle into the account's own fields.
func TestDNSAccountService_CreateAccount_AppliesPlanBundle(t *testing.T) {
	db := openDNSPlanTestDB(t)

	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		_, _ = w.Write([]byte(`{"id":1,"status":"active"}`))
	}))
	t.Cleanup(server.Close)

	panel := model.DNSPanel{Name: "test-panel", SaleTitle: "Smart DNS", APIBaseURL: server.URL, APIKey: "k", Status: "active"}
	if err := db.Create(&panel).Error; err != nil {
		t.Fatalf("failed to create test panel: %v", err)
	}

	planService := NewDNSPlanService(db)
	plan, err := planService.CreatePlan(&schema.CreateDNSPlanRequest{
		Name:                     "طلایی",
		PriceAmount:              150000,
		TotalVolumeBytes:         100 * 1024 * 1024 * 1024,
		SpeedKbps:                4096,
		DurationDays:             60,
		MaxConcurrentIPs:         5,
		DailyIPRegistrationLimit: 10,
	})
	if err != nil {
		t.Fatalf("CreatePlan failed: %v", err)
	}

	panelService := NewDNSPanelService(db)
	geoIP := NewGeoIPService(db, t.TempDir(), NewSystemConfigService(db))
	accountService := NewDNSAccountService(db, panelService, geoIP, nil)
	accountService.SetPlanService(planService)

	resp, err := accountService.CreateAccount(&schema.CreateDNSAccountRequest{
		PanelID:          panel.ID,
		PlanID:           &plan.Id,
		MaxConcurrentIPs: 1, // zero-value default the plan should override
	}, nil)
	if err != nil {
		t.Fatalf("CreateAccount failed: %v", err)
	}

	if resp.TotalVolumeBytes != plan.TotalVolumeBytes {
		t.Errorf("expected TotalVolumeBytes=%d from plan, got %d", plan.TotalVolumeBytes, resp.TotalVolumeBytes)
	}
	if resp.SpeedKbps != plan.SpeedKbps {
		t.Errorf("expected SpeedKbps=%d from plan, got %d", plan.SpeedKbps, resp.SpeedKbps)
	}
	if resp.DurationDays != plan.DurationDays {
		t.Errorf("expected DurationDays=%d from plan, got %d", plan.DurationDays, resp.DurationDays)
	}
	if resp.DailyIPRegistrationLimit != plan.DailyIPRegistrationLimit {
		t.Errorf("expected DailyIPRegistrationLimit=%d from plan, got %d", plan.DailyIPRegistrationLimit, resp.DailyIPRegistrationLimit)
	}
	if resp.PlanID == nil || *resp.PlanID != plan.Id {
		t.Errorf("expected PlanID=%d to be recorded on the account, got %v", plan.Id, resp.PlanID)
	}
	if resp.PlanName == nil || *resp.PlanName != plan.Name {
		t.Errorf("expected PlanName=%q, got %v", plan.Name, resp.PlanName)
	}
}

// TestDNSAccountService_CreateAccount_ExplicitFieldsOverridePlan confirms
// the "manual override in special cases" requirement: a field explicitly
// set in the same create request beats the plan's own value for that field.
func TestDNSAccountService_CreateAccount_ExplicitFieldsOverridePlan(t *testing.T) {
	db := openDNSPlanTestDB(t)

	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		_, _ = w.Write([]byte(`{"id":1,"status":"active"}`))
	}))
	t.Cleanup(server.Close)

	panel := model.DNSPanel{Name: "test-panel", SaleTitle: "Smart DNS", APIBaseURL: server.URL, APIKey: "k", Status: "active"}
	if err := db.Create(&panel).Error; err != nil {
		t.Fatalf("failed to create test panel: %v", err)
	}

	planService := NewDNSPlanService(db)
	plan, err := planService.CreatePlan(&schema.CreateDNSPlanRequest{
		Name:                     "طلایی",
		PriceAmount:              150000,
		TotalVolumeBytes:         100 * 1024 * 1024 * 1024,
		SpeedKbps:                4096,
		DurationDays:             60,
		MaxConcurrentIPs:         5,
		DailyIPRegistrationLimit: 10,
	})
	if err != nil {
		t.Fatalf("CreatePlan failed: %v", err)
	}

	panelService := NewDNSPanelService(db)
	geoIP := NewGeoIPService(db, t.TempDir(), NewSystemConfigService(db))
	accountService := NewDNSAccountService(db, panelService, geoIP, nil)
	accountService.SetPlanService(planService)

	// Explicitly set a different volume than the plan's own -- this must win.
	customVolume := int64(5 * 1024 * 1024 * 1024)
	resp, err := accountService.CreateAccount(&schema.CreateDNSAccountRequest{
		PanelID:          panel.ID,
		PlanID:           &plan.Id,
		TotalVolumeBytes: customVolume,
		MaxConcurrentIPs: 9,
	}, nil)
	if err != nil {
		t.Fatalf("CreateAccount failed: %v", err)
	}

	if resp.TotalVolumeBytes != customVolume {
		t.Errorf("expected explicit TotalVolumeBytes=%d to override the plan, got %d", customVolume, resp.TotalVolumeBytes)
	}
	if resp.MaxConcurrentIPs != 9 {
		t.Errorf("expected explicit MaxConcurrentIPs=9 to override the plan, got %d", resp.MaxConcurrentIPs)
	}
	// Fields NOT explicitly set still come from the plan.
	if resp.SpeedKbps != plan.SpeedKbps {
		t.Errorf("expected SpeedKbps=%d from plan (not overridden), got %d", plan.SpeedKbps, resp.SpeedKbps)
	}
}
