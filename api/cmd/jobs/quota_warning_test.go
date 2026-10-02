package traffic

import (
	"sync"
	"testing"

	"github.com/maahdima/mwp/api/dataservice/model"
)

// spyQuotaNotifier records every NotifyQuotaWarning call.
type spyQuotaNotifier struct {
	calls []struct {
		reseller         model.Reseller
		percentRemaining int
	}
}

func (s *spyQuotaNotifier) NotifyQuotaWarning(reseller model.Reseller, percentRemaining int) {
	s.calls = append(s.calls, struct {
		reseller         model.Reseller
		percentRemaining int
	}{reseller, percentRemaining})
}

func TestApplyResellerQuota_WarnsOnceUnderTenPercentRemaining(t *testing.T) {
	db := openTestDB(t)
	calc := &Calculator{db: db, mikrotikAdaptor: nil, mu: &sync.Mutex{}}
	spy := &spyQuotaNotifier{}
	calc.SetQuotaNotifier(spy)

	quota := int64(1000)
	res := model.Reseller{Name: "warn-test", QuotaBytes: &quota, UsedBytes: 850, IsActive: true}
	if err := db.Create(&res).Error; err != nil {
		t.Fatalf("failed to create reseller: %v", err)
	}

	// Delta pushes usage from 850 to 920 -> 8% remaining, under the 10% threshold.
	calc.applyResellerQuota(&res.ID, "wg0", 70)

	if len(spy.calls) != 1 {
		t.Fatalf("expected exactly 1 quota warning, got %d", len(spy.calls))
	}
	if spy.calls[0].percentRemaining != 8 {
		t.Fatalf("expected 8%% remaining reported, got %d%%", spy.calls[0].percentRemaining)
	}

	var updated model.Reseller
	if err := db.First(&updated, res.ID).Error; err != nil {
		t.Fatalf("failed to reload reseller: %v", err)
	}
	if !updated.QuotaWarningSent {
		t.Fatalf("expected QuotaWarningSent to be persisted as true")
	}

	// A second tick, still under threshold, must NOT warn again.
	calc.applyResellerQuota(&res.ID, "wg0", 10)
	if len(spy.calls) != 1 {
		t.Fatalf("expected still exactly 1 warning after a second under-threshold tick, got %d", len(spy.calls))
	}
}

func TestApplyResellerQuota_DoesNotWarnAboveTenPercentRemaining(t *testing.T) {
	db := openTestDB(t)
	calc := &Calculator{db: db, mikrotikAdaptor: nil, mu: &sync.Mutex{}}
	spy := &spyQuotaNotifier{}
	calc.SetQuotaNotifier(spy)

	quota := int64(1000)
	res := model.Reseller{Name: "no-warn-test", QuotaBytes: &quota, UsedBytes: 100, IsActive: true}
	if err := db.Create(&res).Error; err != nil {
		t.Fatalf("failed to create reseller: %v", err)
	}

	// Delta pushes usage from 100 to 200 -> 80% remaining, well above threshold.
	calc.applyResellerQuota(&res.ID, "wg0", 100)

	if len(spy.calls) != 0 {
		t.Fatalf("expected no quota warning above 10%% remaining, got %d", len(spy.calls))
	}
}

func TestApplyResellerQuota_DoesNotWarnForUnlimitedQuota(t *testing.T) {
	db := openTestDB(t)
	calc := &Calculator{db: db, mikrotikAdaptor: nil, mu: &sync.Mutex{}}
	spy := &spyQuotaNotifier{}
	calc.SetQuotaNotifier(spy)

	res := model.Reseller{Name: "unlimited-test", QuotaBytes: nil, UsedBytes: 0, IsActive: true}
	if err := db.Create(&res).Error; err != nil {
		t.Fatalf("failed to create reseller: %v", err)
	}

	calc.applyResellerQuota(&res.ID, "wg0", 1_000_000)

	if len(spy.calls) != 0 {
		t.Fatalf("expected no quota warning for an unlimited-quota reseller, got %d", len(spy.calls))
	}
}
