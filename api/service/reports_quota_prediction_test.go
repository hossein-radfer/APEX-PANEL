package service

import (
	"testing"
	"time"

	"github.com/maahdima/mwp/api/dataservice/model"
)

// TestGetResellerQuotaPrediction_ComputesDaysRemainingFromRecentUsage is
// the core regression test: a reseller with a finite quota and steady
// recent usage should get a real DaysRemaining projection, derived purely
// from existing model.UsageSnapshot rows (no new tracking table/job
// needed).
func TestGetResellerQuotaPrediction_ComputesDaysRemainingFromRecentUsage(t *testing.T) {
	svc, db := newTestReportsService(t)

	quota := int64(10_000)
	reseller := model.Reseller{
		Name: "predict-1", Username: "predict-1",
		QuotaBytes: &quota, UsedBytes: 3_000,
	}
	if err := db.Create(&reseller).Error; err != nil {
		t.Fatalf("failed to create reseller: %v", err)
	}

	// 700 bytes/day of WireGuard usage over the last 7 days -> avg 100/day.
	now := time.Now()
	for i := 0; i < 7; i++ {
		snap := model.UsageSnapshot{
			Timestamp:  now.AddDate(0, 0, -i).Unix(),
			Protocol:   model.UsageProtocolWireGuard,
			ResellerID: &reseller.ID,
			TotalBytes: 100,
		}
		if err := db.Create(&snap).Error; err != nil {
			t.Fatalf("failed to create usage snapshot: %v", err)
		}
	}

	result, err := svc.GetResellerQuotaPrediction(reseller.ID)
	if err != nil {
		t.Fatalf("GetResellerQuotaPrediction failed: %v", err)
	}

	found := false
	for _, p := range result.Protocols {
		if p.Protocol != model.UsageProtocolWireGuard {
			continue
		}
		found = true
		if p.AvgDailyUsageBytes != 100 {
			t.Errorf("expected avg daily usage of 100 bytes, got %d", p.AvgDailyUsageBytes)
		}
		// Remaining = 10000 - 3000 = 7000; 7000 / 100 = 70 days.
		if p.DaysRemaining == nil {
			t.Fatal("expected a non-nil DaysRemaining")
		}
		if *p.DaysRemaining != 70 {
			t.Errorf("expected DaysRemaining=70, got %d", *p.DaysRemaining)
		}
	}
	if !found {
		t.Fatal("expected a wireguard entry in the response")
	}
}

// TestGetResellerQuotaPrediction_NilQuotaMeansNoDaysRemaining confirms an
// unlimited-quota reseller (QuotaBytes == nil) gets DaysRemaining=nil, not
// a fabricated number -- there is nothing to run out of.
func TestGetResellerQuotaPrediction_NilQuotaMeansNoDaysRemaining(t *testing.T) {
	svc, db := newTestReportsService(t)

	reseller := model.Reseller{
		Name: "predict-unlimited", Username: "predict-unlimited",
		QuotaBytes: nil, UsedBytes: 999_999,
	}
	if err := db.Create(&reseller).Error; err != nil {
		t.Fatalf("failed to create reseller: %v", err)
	}

	snap := model.UsageSnapshot{
		Timestamp:  time.Now().Unix(),
		Protocol:   model.UsageProtocolWireGuard,
		ResellerID: &reseller.ID,
		TotalBytes: 500,
	}
	if err := db.Create(&snap).Error; err != nil {
		t.Fatalf("failed to create usage snapshot: %v", err)
	}

	result, err := svc.GetResellerQuotaPrediction(reseller.ID)
	if err != nil {
		t.Fatalf("GetResellerQuotaPrediction failed: %v", err)
	}

	for _, p := range result.Protocols {
		if p.Protocol != model.UsageProtocolWireGuard {
			continue
		}
		if p.DaysRemaining != nil {
			t.Errorf("expected DaysRemaining=nil for unlimited quota, got %d", *p.DaysRemaining)
		}
		if p.RemainingBytes != nil {
			t.Errorf("expected RemainingBytes=nil for unlimited quota, got %d", *p.RemainingBytes)
		}
		return
	}
	t.Fatal("expected a wireguard entry in the response")
}

// TestGetResellerQuotaPrediction_ZeroRecentUsageMeansNoDaysRemaining
// confirms a reseller with a finite quota but ZERO usage in the lookback
// window gets DaysRemaining=nil rather than a division-by-zero-flavored
// "infinite"/fabricated value -- 0 recent usage must never be misread as
// "will never run out."
func TestGetResellerQuotaPrediction_ZeroRecentUsageMeansNoDaysRemaining(t *testing.T) {
	svc, db := newTestReportsService(t)

	quota := int64(10_000)
	reseller := model.Reseller{
		Name: "predict-idle", Username: "predict-idle",
		QuotaBytes: &quota, UsedBytes: 3_000,
	}
	if err := db.Create(&reseller).Error; err != nil {
		t.Fatalf("failed to create reseller: %v", err)
	}
	// No usage snapshots created at all -- genuinely idle reseller.

	result, err := svc.GetResellerQuotaPrediction(reseller.ID)
	if err != nil {
		t.Fatalf("GetResellerQuotaPrediction failed: %v", err)
	}

	for _, p := range result.Protocols {
		if p.Protocol != model.UsageProtocolWireGuard {
			continue
		}
		if p.AvgDailyUsageBytes != 0 {
			t.Errorf("expected AvgDailyUsageBytes=0 for a reseller with no recent usage, got %d", p.AvgDailyUsageBytes)
		}
		if p.DaysRemaining != nil {
			t.Errorf("expected DaysRemaining=nil when there's no usage rate to extrapolate from, got %d", *p.DaysRemaining)
		}
		if p.RemainingBytes == nil || *p.RemainingBytes != 7000 {
			t.Errorf("expected RemainingBytes=7000 (quota-used, still computable even with no rate), got %v", p.RemainingBytes)
		}
		return
	}
	t.Fatal("expected a wireguard entry in the response")
}

// TestGetResellerQuotaPrediction_OnlyCountsThisResellersOwnUsage confirms
// the query is properly scoped by reseller_id -- another reseller's usage
// snapshots must never leak into this reseller's average.
func TestGetResellerQuotaPrediction_OnlyCountsThisResellersOwnUsage(t *testing.T) {
	svc, db := newTestReportsService(t)

	quota := int64(10_000)
	resellerA := model.Reseller{Name: "predict-a", Username: "predict-a", QuotaBytes: &quota, UsedBytes: 0}
	resellerB := model.Reseller{Name: "predict-b", Username: "predict-b", QuotaBytes: &quota, UsedBytes: 0}
	if err := db.Create(&resellerA).Error; err != nil {
		t.Fatalf("failed to create reseller A: %v", err)
	}
	if err := db.Create(&resellerB).Error; err != nil {
		t.Fatalf("failed to create reseller B: %v", err)
	}

	// Reseller B has heavy usage; reseller A has none. A's prediction must
	// not be affected by B's snapshots.
	for i := 0; i < 7; i++ {
		snap := model.UsageSnapshot{
			Timestamp:  time.Now().AddDate(0, 0, -i).Unix(),
			Protocol:   model.UsageProtocolWireGuard,
			ResellerID: &resellerB.ID,
			TotalBytes: 5000,
		}
		if err := db.Create(&snap).Error; err != nil {
			t.Fatalf("failed to create usage snapshot for reseller B: %v", err)
		}
	}

	result, err := svc.GetResellerQuotaPrediction(resellerA.ID)
	if err != nil {
		t.Fatalf("GetResellerQuotaPrediction failed: %v", err)
	}

	for _, p := range result.Protocols {
		if p.Protocol != model.UsageProtocolWireGuard {
			continue
		}
		if p.AvgDailyUsageBytes != 0 {
			t.Errorf("expected reseller A's avg daily usage to be 0 (unaffected by reseller B's usage), got %d", p.AvgDailyUsageBytes)
		}
		return
	}
	t.Fatal("expected a wireguard entry in the response")
}

// TestGetResellerQuotaPrediction_ReturnsAllThreeProtocols confirms the
// response always includes an entry for wireguard/user_manager/v2ray, even
// when a reseller has zero usage/quota configured for some of them --
// the frontend dashboard renders one card per protocol unconditionally.
func TestGetResellerQuotaPrediction_ReturnsAllThreeProtocols(t *testing.T) {
	svc, db := newTestReportsService(t)

	reseller := model.Reseller{Name: "predict-all", Username: "predict-all"}
	if err := db.Create(&reseller).Error; err != nil {
		t.Fatalf("failed to create reseller: %v", err)
	}

	result, err := svc.GetResellerQuotaPrediction(reseller.ID)
	if err != nil {
		t.Fatalf("GetResellerQuotaPrediction failed: %v", err)
	}

	seen := map[string]bool{}
	for _, p := range result.Protocols {
		seen[p.Protocol] = true
	}
	for _, want := range []string{model.UsageProtocolWireGuard, model.UsageProtocolUserManager, model.UsageProtocolV2Ray} {
		if !seen[want] {
			t.Errorf("expected protocol %q to be present in the response, got protocols: %v", want, result.Protocols)
		}
	}
}
