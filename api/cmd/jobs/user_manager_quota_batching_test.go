package traffic

import (
	"context"
	"fmt"
	"sync"
	"testing"

	"github.com/glebarez/sqlite"
	"gorm.io/gorm"

	"github.com/maahdima/mwp/api/adaptor/mikrotik"
	"github.com/maahdima/mwp/api/dataservice/model"
)

// perUserAdaptor is a MikrotikAdaptor fake whose MonitorUserManagerUser
// result is configurable PER RouterOSUserID, so a single test can give
// several accounts belonging to the same reseller different traffic
// amounts -- needed to exercise CalculateUserManagerUsage's per-reseller
// batching (see that function's own doc comment) with more than one
// account contributing to the same reseller in a single tick.
type perUserAdaptor struct {
	mu      sync.Mutex
	results map[string]mikrotik.UserManagerMonitorResult
}

func newPerUserAdaptor() *perUserAdaptor {
	return &perUserAdaptor{results: make(map[string]mikrotik.UserManagerMonitorResult)}
}

func (f *perUserAdaptor) set(userID string, downloadBytes, uploadBytes int64) {
	f.mu.Lock()
	defer f.mu.Unlock()
	f.results[userID] = mikrotik.UserManagerMonitorResult{
		TotalDownload: fmt.Sprintf("%d", downloadBytes),
		TotalUpload:   fmt.Sprintf("%d", uploadBytes),
	}
}

func (f *perUserAdaptor) MonitorUserManagerUser(ctx context.Context, userID string) (*mikrotik.UserManagerMonitorResult, error) {
	f.mu.Lock()
	defer f.mu.Unlock()
	res, ok := f.results[userID]
	if !ok {
		res = mikrotik.UserManagerMonitorResult{TotalDownload: "0", TotalUpload: "0"}
	}
	return &res, nil
}

func (f *perUserAdaptor) SetUserManagerUserDisabled(ctx context.Context, userID string, disabled string) (*mikrotik.UserManagerUser, error) {
	return &mikrotik.UserManagerUser{}, nil
}

func (f *perUserAdaptor) FetchWgPeer(ctx context.Context, peerID string) (*mikrotik.WireGuardPeer, error) {
	return &mikrotik.WireGuardPeer{ID: peerID, TransferTx: "0", TransferRx: "0"}, nil
}
func (f *perUserAdaptor) FetchWgPeers(ctx context.Context) ([]mikrotik.WireGuardPeer, error) {
	return []mikrotik.WireGuardPeer{}, nil
}
func (f *perUserAdaptor) FetchInterface(ctx context.Context, interfaceID string) (*mikrotik.Interface, error) {
	return &mikrotik.Interface{TxByte: "0", RxByte: "0"}, nil
}
func (f *perUserAdaptor) UpdateWgPeer(ctx context.Context, peerID string, wgPeer mikrotik.WireGuardPeer) (*mikrotik.WireGuardPeer, error) {
	return &wgPeer, nil
}
func (f *perUserAdaptor) UpdateScheduler(ctx context.Context, id string, s mikrotik.Scheduler) (*mikrotik.Scheduler, error) {
	return &s, nil
}
func (f *perUserAdaptor) UpdateSimpleQueue(ctx context.Context, id string, q mikrotik.Queue) (*mikrotik.Queue, error) {
	return &q, nil
}

func openUserManagerTestDB(t *testing.T) *gorm.DB {
	t.Helper()
	dsn := fmt.Sprintf("file:um_batching_%d?mode=memory&cache=shared", newTestSeq())
	db, err := gorm.Open(sqlite.Open(dsn), &gorm.Config{})
	if err != nil {
		t.Fatalf("failed to open test db: %v", err)
	}
	if err := db.AutoMigrate(&model.Reseller{}, &model.UserManagerAccount{}); err != nil {
		t.Fatalf("failed to migrate test db: %v", err)
	}
	return db
}

var testSeqMu sync.Mutex
var testSeqCounter int64

// newTestSeq gives each test its own memory-mode sqlite DSN so parallel (or
// simply sequential-but-cache-shared) tests in this file never collide on
// the same in-memory database -- traffic_test.go's own openTestDB uses one
// fixed shared name, which is fine for that file's serial tests but would
// leak state between the multi-account scenarios this file adds.
func newTestSeq() int64 {
	testSeqMu.Lock()
	defer testSeqMu.Unlock()
	testSeqCounter++
	return testSeqCounter
}

// TestCalculateUserManagerUsage_BatchesMultipleAccountsIntoOneQuotaUpdate is
// the core regression test for the confirmed, reported production incident:
// applyUserManagerResellerQuota used to run once PER ACCOUNT even though it
// always recomputes the identical reseller-wide SUM+UPDATE -- see that
// function's own doc comment. This only verifies the observable OUTCOME
// (every account's usage is summed correctly into the reseller's live
// total) stays correct after batching to one call per reseller; the
// call-count reduction itself is exercised implicitly by every account
// here funneling through a single CalculateUserManagerUsage pass.
func TestCalculateUserManagerUsage_BatchesMultipleAccountsIntoOneQuotaUpdate(t *testing.T) {
	db := openUserManagerTestDB(t)
	adaptor := newPerUserAdaptor()

	res := model.Reseller{Name: "batch-reseller", IsActive: true}
	if err := db.Create(&res).Error; err != nil {
		t.Fatalf("failed to create reseller: %v", err)
	}

	accounts := []model.UserManagerAccount{
		{UUID: "u1", Username: "acc1", Password: "p", RouterOSUserID: "*1", Group: "group-a", Profile: "prof", ResellerID: &res.ID},
		{UUID: "u2", Username: "acc2", Password: "p", RouterOSUserID: "*2", Group: "group-a", Profile: "prof", ResellerID: &res.ID},
		{UUID: "u3", Username: "acc3", Password: "p", RouterOSUserID: "*3", Group: "group-b", Profile: "prof", ResellerID: &res.ID},
	}
	for i := range accounts {
		if err := db.Create(&accounts[i]).Error; err != nil {
			t.Fatalf("failed to create account: %v", err)
		}
	}

	// Each account reports a distinct fresh usage amount this tick.
	adaptor.set("*1", 100, 0)
	adaptor.set("*2", 200, 0)
	adaptor.set("*3", 300, 0)

	calc := NewTrafficCalculator(db, adaptor, nil)
	calc.CalculateUserManagerUsage()

	var updatedRes model.Reseller
	if err := db.First(&updatedRes, "id = ?", res.ID).Error; err != nil {
		t.Fatalf("failed to fetch reseller: %v", err)
	}

	const wantTotal = int64(100 + 200 + 300)
	if updatedRes.UserManagerUsedBytes != wantTotal {
		t.Fatalf("expected reseller UserManagerUsedBytes=%d (sum of all 3 accounts), got %d", wantTotal, updatedRes.UserManagerUsedBytes)
	}
}

// TestCalculateUserManagerUsage_IdleResellerStillGetsQuotaRecomputed
// confirms the SUM+UPDATE pass still runs for a reseller with NO fresh
// traffic this tick -- see applyUserManagerResellerQuota's own doc comment
// on why an idle reseller's stored total must still be corrected rather
// than silently skipped just because its per-tick delta was 0. This is the
// invariant touchedResellers (in CalculateUserManagerUsage) exists to
// preserve after the per-account-call batching refactor.
func TestCalculateUserManagerUsage_IdleResellerStillGetsQuotaRecomputed(t *testing.T) {
	db := openUserManagerTestDB(t)
	adaptor := newPerUserAdaptor()

	res := model.Reseller{Name: "idle-reseller", IsActive: true}
	if err := db.Create(&res).Error; err != nil {
		t.Fatalf("failed to create reseller: %v", err)
	}

	// This account already has stored usage from a previous tick (matching
	// LastTotalDownload/LastTotalUpload so THIS tick's delta is 0), but the
	// reseller's own UserManagerUsedBytes starts out stale/wrong.
	account := model.UserManagerAccount{
		UUID: "u1", Username: "acc1", Password: "p", RouterOSUserID: "*1", Group: "group-a", Profile: "prof",
		ResellerID:        &res.ID,
		DownloadUsage:     500,
		LastTotalDownload: 500,
	}
	if err := db.Create(&account).Error; err != nil {
		t.Fatalf("failed to create account: %v", err)
	}

	adaptor.set("*1", 500, 0) // same as LastTotalDownload -- zero fresh delta this tick

	calc := NewTrafficCalculator(db, adaptor, nil)
	calc.CalculateUserManagerUsage()

	var updatedRes model.Reseller
	if err := db.First(&updatedRes, "id = ?", res.ID).Error; err != nil {
		t.Fatalf("failed to fetch reseller: %v", err)
	}
	if updatedRes.UserManagerUsedBytes != 500 {
		t.Fatalf("expected idle reseller's UserManagerUsedBytes to still be recomputed to 500, got %d", updatedRes.UserManagerUsedBytes)
	}
}

// TestCalculateUserManagerUsage_DisablesAccountsOverVolumeQuota confirms
// enforcement (disabling every account once the reseller's
// UserManagerQuotaBytes is exceeded) still fires correctly after batching
// multiple accounts' deltas into a single applyUserManagerResellerQuota
// call per reseller.
func TestCalculateUserManagerUsage_DisablesAccountsOverVolumeQuota(t *testing.T) {
	db := openUserManagerTestDB(t)
	adaptor := newPerUserAdaptor()

	quota := int64(250)
	res := model.Reseller{Name: "quota-reseller", IsActive: true, UserManagerQuotaBytes: &quota}
	if err := db.Create(&res).Error; err != nil {
		t.Fatalf("failed to create reseller: %v", err)
	}

	accounts := []model.UserManagerAccount{
		{UUID: "u1", Username: "acc1", Password: "p", RouterOSUserID: "*1", Group: "group-a", Profile: "prof", ResellerID: &res.ID},
		{UUID: "u2", Username: "acc2", Password: "p", RouterOSUserID: "*2", Group: "group-a", Profile: "prof", ResellerID: &res.ID},
	}
	for i := range accounts {
		if err := db.Create(&accounts[i]).Error; err != nil {
			t.Fatalf("failed to create account: %v", err)
		}
	}

	// Combined 100+300=400 > quota of 250.
	adaptor.set("*1", 100, 0)
	adaptor.set("*2", 300, 0)

	calc := NewTrafficCalculator(db, adaptor, nil)
	calc.CalculateUserManagerUsage()

	var updatedAccounts []model.UserManagerAccount
	if err := db.Where("reseller_id = ?", res.ID).Find(&updatedAccounts).Error; err != nil {
		t.Fatalf("failed to fetch accounts: %v", err)
	}
	for _, a := range updatedAccounts {
		if !a.Disabled {
			t.Fatalf("expected account %s to be disabled for exceeding reseller quota", a.Username)
		}
		if !a.SuspendedByQuota {
			t.Fatalf("expected account %s to be marked SuspendedByQuota", a.Username)
		}
	}
}
