package service

import (
	"fmt"
	"testing"
	"time"

	"github.com/glebarez/sqlite"
	"github.com/maahdima/mwp/api/dataservice/model"
	"github.com/maahdima/mwp/api/http/schema"
	"gorm.io/gorm"
)

func openDNSQuotaResumeTestDB(t *testing.T) *gorm.DB {
	t.Helper()

	dsn := fmt.Sprintf("file:dns_reseller_quota_resume_%d?mode=memory&cache=shared", time.Now().UnixNano())
	db, err := gorm.Open(sqlite.Open(dsn), &gorm.Config{})
	if err != nil {
		t.Fatalf("failed to open test db: %v", err)
	}

	if err := db.AutoMigrate(
		&model.Reseller{}, &model.DNSAccount{},
		&model.Peer{}, &model.UserManagerAccount{}, &model.V2RayPackage{}, &model.Application{},
	); err != nil {
		t.Fatalf("failed to migrate test db: %v", err)
	}

	return db
}

// fakeDNSResellerQuotaResumer records which reseller IDs
// ResumeAccountsForResellerQuota was called for -- mirrors
// fakeV2RayResellerQuotaResumer's identical role for
// TestUpdateReseller_RaisingDNSQuota_CallsResumer below.
type fakeDNSResellerQuotaResumer struct {
	calledFor []uint
}

func (f *fakeDNSResellerQuotaResumer) ResumeAccountsForResellerQuota(resellerID uint) {
	f.calledFor = append(f.calledFor, resellerID)
}

// TestUpdateReseller_RaisingDNSQuota_CallsResumer is the regression test for
// the gap this session found and fixed: UpdateReseller previously had no
// CanResellDNS/DNSQuotaBytes/DNSMaxAccounts fields at all (a request setting
// them was silently ignored, see CreateResellerRequest/UpdateResellerRequest's
// own doc comments), and even after adding them, raising DNSQuotaBytes back
// above a reseller's live usage did nothing to actually resume their
// reseller-quota-suspended DNS accounts until the next background sync
// tick. This confirms both: the field is now genuinely persisted, AND
// raising it while over-quota calls the injected resumer immediately.
func TestUpdateReseller_RaisingDNSQuota_CallsResumer(t *testing.T) {
	db := openDNSQuotaResumeTestDB(t)
	svc := NewReseller(db, nil)
	resumer := &fakeDNSResellerQuotaResumer{}
	svc.SetDNSResumer(resumer)

	oldQuota := int64(1000)
	reseller := model.Reseller{
		Name:          "dns-quota-owner",
		Username:      "dns-quota-owner",
		CanResellDNS:  true,
		DNSQuotaBytes: &oldQuota,
		DNSUsedBytes:  1500, // already over quota
	}
	if err := db.Create(&reseller).Error; err != nil {
		t.Fatalf("failed to create reseller: %v", err)
	}

	acct := model.DNSAccount{
		UUID:                     "dns-quota-resume-acct",
		PanelID:                  1, // no such panel exists -- pushRemoteStatus must no-op safely, not panic/error
		ApexRef:                  "apex-ref-1",
		ResellerID:               &reseller.ID,
		Status:                   "suspended",
		SuspendedByResellerQuota: true,
		UsedBytesCached:          1500,
	}
	if err := db.Create(&acct).Error; err != nil {
		t.Fatalf("failed to create dns account: %v", err)
	}

	newQuota := int64(2000) // now above live usage (1500) -> should trigger resume
	updated, err := svc.UpdateReseller(reseller.ID, &schema.UpdateResellerRequest{
		DNSQuotaBytes: &newQuota,
	})
	if err != nil {
		t.Fatalf("UpdateReseller failed: %v", err)
	}

	// The core bug: this field did not exist on the request/response at
	// all before this session's fix, so this assertion alone would have
	// failed to even compile against the old schema.
	if updated.DNSQuotaBytes == nil || *updated.DNSQuotaBytes != newQuota {
		t.Fatalf("expected persisted DNSQuotaBytes=%d, got %+v", newQuota, updated.DNSQuotaBytes)
	}

	if len(resumer.calledFor) != 1 || resumer.calledFor[0] != reseller.ID {
		t.Fatalf("expected ResumeAccountsForResellerQuota called once for reseller %d, got calls: %v", reseller.ID, resumer.calledFor)
	}
}

// TestUpdateReseller_RaisingDNSQuota_StillOverQuota_DoesNotCallResumer
// confirms a quota increase that still leaves usage over the new cap does
// not spuriously call the resumer -- mirrors
// TestUpdateResellerDoesNotResumeWhenStillOverQuota's identical WireGuard
// case.
func TestUpdateReseller_RaisingDNSQuota_StillOverQuota_DoesNotCallResumer(t *testing.T) {
	db := openDNSQuotaResumeTestDB(t)
	svc := NewReseller(db, nil)
	resumer := &fakeDNSResellerQuotaResumer{}
	svc.SetDNSResumer(resumer)

	oldQuota := int64(1000)
	reseller := model.Reseller{
		Name: "dns-still-over", Username: "dns-still-over",
		CanResellDNS:  true,
		DNSQuotaBytes: &oldQuota,
		DNSUsedBytes:  5000,
	}
	if err := db.Create(&reseller).Error; err != nil {
		t.Fatalf("failed to create reseller: %v", err)
	}

	acct := model.DNSAccount{
		UUID: "dns-still-over-acct", PanelID: 1, ApexRef: "apex-ref-2",
		ResellerID: &reseller.ID, Status: "suspended",
		SuspendedByResellerQuota: true, UsedBytesCached: 5000,
	}
	if err := db.Create(&acct).Error; err != nil {
		t.Fatalf("failed to create dns account: %v", err)
	}

	// Raise the quota, but not enough to clear live usage (5000).
	newQuota := int64(2000)
	if _, err := svc.UpdateReseller(reseller.ID, &schema.UpdateResellerRequest{
		DNSQuotaBytes: &newQuota,
	}); err != nil {
		t.Fatalf("UpdateReseller failed: %v", err)
	}

	if len(resumer.calledFor) != 0 {
		t.Fatalf("expected resumer NOT called while still over quota, got calls: %v", resumer.calledFor)
	}

	var got model.DNSAccount
	if err := db.First(&got, acct.ID).Error; err != nil {
		t.Fatalf("failed to reload dns account: %v", err)
	}
	if got.Status != "suspended" || !got.SuspendedByResellerQuota {
		t.Errorf("expected account to remain suspended, got status=%q suspendedByResellerQuota=%v", got.Status, got.SuspendedByResellerQuota)
	}
}

// TestDNSSyncService_ResumeAccountsForResellerQuota_ClearsFlagsAndFiltersOthers
// tests DNSSyncService.ResumeAccountsForResellerQuota directly (not via
// UpdateReseller/the injected interface) -- confirms it resumes only
// accounts actually suspended by the RESELLER's pool quota, leaves an
// account's own-quota suspension (SuspendedByQuota, a different flag/reason
// entirely -- see model.DNSAccount's own doc comment) untouched, and never
// touches a different reseller's accounts.
func TestDNSSyncService_ResumeAccountsForResellerQuota_ClearsFlagsAndFiltersOthers(t *testing.T) {
	db := openDNSQuotaResumeTestDB(t)
	if err := db.AutoMigrate(&model.DNSPanel{}); err != nil {
		t.Fatalf("failed to migrate dns panel: %v", err)
	}
	panelService := NewDNSPanelService(db)
	syncSvc := NewDNSSyncService(db, panelService)

	resellerA := model.Reseller{Name: "a", Username: "a"}
	resellerB := model.Reseller{Name: "b", Username: "b"}
	if err := db.Create(&resellerA).Error; err != nil {
		t.Fatalf("failed to create reseller A: %v", err)
	}
	if err := db.Create(&resellerB).Error; err != nil {
		t.Fatalf("failed to create reseller B: %v", err)
	}

	// Account 1 (reseller A): suspended by the RESELLER's own pool quota --
	// must resume.
	acctResellerQuota := model.DNSAccount{
		UUID: "acct-reseller-quota", PanelID: 1, ApexRef: "ref-1",
		ResellerID: &resellerA.ID, Status: "suspended",
		SuspendedByResellerQuota: true,
	}
	// Account 2 (reseller A): suspended by its OWN TotalVolumeBytes limit
	// (SuspendedByQuota), a completely different reason -- must NOT resume
	// via this reseller-quota-only path.
	acctOwnQuota := model.DNSAccount{
		UUID: "acct-own-quota", PanelID: 1, ApexRef: "ref-2",
		ResellerID: &resellerA.ID, Status: "suspended",
		SuspendedByQuota: true, WasActiveBeforeSuspend: true,
	}
	// Account 3 (reseller B): also reseller-quota-suspended, but belongs to
	// a DIFFERENT reseller -- must not be touched when resuming reseller A.
	acctOtherReseller := model.DNSAccount{
		UUID: "acct-other-reseller", PanelID: 1, ApexRef: "ref-3",
		ResellerID: &resellerB.ID, Status: "suspended",
		SuspendedByResellerQuota: true,
	}
	for _, a := range []*model.DNSAccount{&acctResellerQuota, &acctOwnQuota, &acctOtherReseller} {
		if err := db.Create(a).Error; err != nil {
			t.Fatalf("failed to create dns account %s: %v", a.UUID, err)
		}
	}

	syncSvc.ResumeAccountsForResellerQuota(resellerA.ID)

	var got1, got2, got3 model.DNSAccount
	if err := db.First(&got1, acctResellerQuota.ID).Error; err != nil {
		t.Fatalf("failed to reload account 1: %v", err)
	}
	if err := db.First(&got2, acctOwnQuota.ID).Error; err != nil {
		t.Fatalf("failed to reload account 2: %v", err)
	}
	if err := db.First(&got3, acctOtherReseller.ID).Error; err != nil {
		t.Fatalf("failed to reload account 3: %v", err)
	}

	if got1.Status != "active" || got1.SuspendedByResellerQuota {
		t.Errorf("account 1: expected resumed (status=active, SuspendedByResellerQuota=false), got status=%q suspendedByResellerQuota=%v", got1.Status, got1.SuspendedByResellerQuota)
	}
	if got2.Status != "suspended" || !got2.SuspendedByQuota {
		t.Errorf("account 2: expected to remain suspended by its OWN quota, untouched by the reseller-quota resume, got status=%q suspendedByQuota=%v", got2.Status, got2.SuspendedByQuota)
	}
	if got3.Status != "suspended" || !got3.SuspendedByResellerQuota {
		t.Errorf("account 3: expected reseller B's account left untouched (still suspended), but resuming reseller A incorrectly changed it to status=%q suspendedByResellerQuota=%v", got3.Status, got3.SuspendedByResellerQuota)
	}
}
