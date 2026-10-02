package service

import (
	"fmt"
	"net/http"
	"net/http/httptest"
	"sync"
	"testing"
	"time"

	"github.com/glebarez/sqlite"
	"gorm.io/gorm"

	"github.com/maahdima/mwp/api/dataservice/model"
)

func openGeoIPTestDB(t *testing.T) *gorm.DB {
	t.Helper()

	dsn := fmt.Sprintf("file:geoip_%d?mode=memory&cache=shared", time.Now().UnixNano())
	db, err := gorm.Open(sqlite.Open(dsn), &gorm.Config{})
	if err != nil {
		t.Fatalf("failed to open test db: %v", err)
	}

	if err := db.AutoMigrate(&model.IPGeoCache{}, &model.SystemConfig{}); err != nil {
		t.Fatalf("failed to migrate test db: %v", err)
	}

	return db
}

// TestGeoIPLookup_CacheHitNeverTouchesReaders pins down the admin's own
// explicit requirement ("دوباره گرفته نشه"): a cache hit must return
// immediately from IPGeoCache without erroring or attempting an mmdb
// lookup -- exercised here with NEITHER reader loaded (both nil), which
// would fail loudly if Lookup ever fell through to the mmdb path on a
// cache hit.
func TestGeoIPLookup_CacheHitNeverTouchesReaders(t *testing.T) {
	db := openGeoIPTestDB(t)

	country := "Iran"
	city := "Tehran"
	if err := db.Create(&model.IPGeoCache{
		IPAddress:  "5.202.1.1",
		Country:    &country,
		City:       &city,
		ResolvedAt: time.Now(),
	}).Error; err != nil {
		t.Fatalf("failed to seed cache row: %v", err)
	}

	svc := NewGeoIPService(db, t.TempDir(), NewSystemConfigService(db))
	result, err := svc.Lookup("5.202.1.1")
	if err != nil {
		t.Fatalf("unexpected error on cache hit: %v", err)
	}
	if result.Country == nil || *result.Country != "Iran" {
		t.Errorf("expected cached country 'Iran', got %v", result.Country)
	}
	if result.City == nil || *result.City != "Tehran" {
		t.Errorf("expected cached city 'Tehran', got %v", result.City)
	}
}

// TestGeoIPLookup_MissWithNoReadersStillCaches confirms a cache MISS with
// no mmdb readers loaded at all does not error (per Lookup's own doc
// comment: "no geo data for this IP" is a routine, non-error outcome) and
// still writes an (empty) cache row, so a subsequent call for the same IP
// stays a cache hit rather than repeatedly attempting (and failing) a
// lookup against absent readers.
func TestGeoIPLookup_MissWithNoReadersStillCaches(t *testing.T) {
	db := openGeoIPTestDB(t)
	svc := NewGeoIPService(db, t.TempDir(), NewSystemConfigService(db))

	result, err := svc.Lookup("8.8.8.8")
	if err != nil {
		t.Fatalf("expected no error for a miss with no readers loaded, got: %v", err)
	}
	if result.Country != nil || result.ASN != nil {
		t.Errorf("expected an entirely empty result with no readers loaded, got %+v", result)
	}

	var count int64
	db.Model(&model.IPGeoCache{}).Where("ip_address = ?", "8.8.8.8").Count(&count)
	if count != 1 {
		t.Errorf("expected a cache row to be written even for an empty result, got %d rows", count)
	}
}

// TestGeoIPLookup_InvalidIPErrors confirms a malformed IP address is
// rejected before ever touching the cache or the readers.
func TestGeoIPLookup_InvalidIPErrors(t *testing.T) {
	db := openGeoIPTestDB(t)
	svc := NewGeoIPService(db, t.TempDir(), NewSystemConfigService(db))

	if _, err := svc.Lookup("not-an-ip"); err == nil {
		t.Error("expected an error for an invalid IP address, got nil")
	}
}

// TestGeoIPClearCache confirms ClearCache actually empties the table --
// the admin's own explicit requirement for after uploading a newer mmdb
// pair.
func TestGeoIPClearCache(t *testing.T) {
	db := openGeoIPTestDB(t)
	if err := db.Create(&model.IPGeoCache{IPAddress: "1.1.1.1", ResolvedAt: time.Now()}).Error; err != nil {
		t.Fatalf("failed to seed cache row: %v", err)
	}

	svc := NewGeoIPService(db, t.TempDir(), NewSystemConfigService(db))
	if err := svc.ClearCache(); err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	var count int64
	db.Model(&model.IPGeoCache{}).Count(&count)
	if count != 0 {
		t.Errorf("expected cache to be empty after ClearCache, got %d rows", count)
	}
}

// TestGeoIPLookup_ConcurrentMissesForSameIPDoNotError is the regression
// test for a confirmed, reported production incident: many goroutines
// resolving the SAME uncached IP at the same time (routine at fleet scale
// -- e.g. many accounts/peers sharing one NAT egress IP) all pass the
// cache-miss check before any of them writes the cache row, so all but
// one lose the race on IPGeoCache's unique ip_address index. Before the
// OnConflict DoNothing fix, this surfaced as a logged "UNIQUE constraint
// failed" warning per losing goroutine; this test drives 20 concurrent
// Lookup calls for one IP and asserts none of them return an error and
// exactly one cache row survives.
func TestGeoIPLookup_ConcurrentMissesForSameIPDoNotError(t *testing.T) {
	db := openGeoIPTestDB(t)
	svc := NewGeoIPService(db, t.TempDir(), NewSystemConfigService(db))

	const workers = 20
	errs := make([]error, workers)
	var wg sync.WaitGroup
	for i := 0; i < workers; i++ {
		wg.Add(1)
		go func(idx int) {
			defer wg.Done()
			_, err := svc.Lookup("203.0.113.7")
			errs[idx] = err
		}(i)
	}
	wg.Wait()

	for i, err := range errs {
		if err != nil {
			t.Errorf("worker %d: expected no error from a concurrent cache-miss race, got: %v", i, err)
		}
	}

	var count int64
	db.Model(&model.IPGeoCache{}).Where("ip_address = ?", "203.0.113.7").Count(&count)
	if count != 1 {
		t.Errorf("expected exactly one surviving cache row after the race, got %d", count)
	}
}

// withFakeIPAPIServer points geoIPOnlineAPIBaseURL at a local httptest
// server for the duration of the test, restoring the real default
// afterwards -- see that variable's own doc comment for why production
// code never touches it.
func withFakeIPAPIServer(t *testing.T, handler http.HandlerFunc) {
	t.Helper()
	server := httptest.NewServer(handler)
	t.Cleanup(server.Close)

	original := geoIPOnlineAPIBaseURL
	geoIPOnlineAPIBaseURL = server.URL
	t.Cleanup(func() { geoIPOnlineAPIBaseURL = original })
}

// TestGeoIPLookup_OnlineModeUsesThirdPartyAPI confirms that switching
// SystemConfigService's online toggle on routes Lookup through the
// third-party API path instead of the (empty, in this test) offline mmdb
// readers, and that the parsed fields land in the same GeoIPLookupResult
// shape the offline path produces -- including the "AS15169 Google LLC" ->
// numeric-ASN parsing lookupOnlineAndCache does itself.
func TestGeoIPLookup_OnlineModeUsesThirdPartyAPI(t *testing.T) {
	db := openGeoIPTestDB(t)
	withFakeIPAPIServer(t, func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		_, _ = w.Write([]byte(`{
			"status": "success",
			"country": "Germany",
			"regionName": "Hesse",
			"city": "Frankfurt",
			"lat": 50.1109,
			"lon": 8.6821,
			"timezone": "Europe/Berlin",
			"isp": "Hetzner Online GmbH",
			"as": "AS24940 Hetzner Online GmbH"
		}`))
	})

	systemConfig := NewSystemConfigService(db)
	if err := systemConfig.SetGeoIPOnlineEnabled(true); err != nil {
		t.Fatalf("failed to enable online mode: %v", err)
	}

	svc := NewGeoIPService(db, t.TempDir(), systemConfig)
	result, err := svc.Lookup("5.9.1.1")
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	if result.Country == nil || *result.Country != "Germany" {
		t.Errorf("expected country 'Germany', got %v", result.Country)
	}
	if result.City == nil || *result.City != "Frankfurt" {
		t.Errorf("expected city 'Frankfurt', got %v", result.City)
	}
	if result.ISP == nil || *result.ISP != "Hetzner Online GmbH" {
		t.Errorf("expected ISP 'Hetzner Online GmbH', got %v", result.ISP)
	}
	if result.ASN == nil || *result.ASN != 24940 {
		t.Errorf("expected ASN 24940 parsed from 'AS24940 Hetzner Online GmbH', got %v", result.ASN)
	}

	var count int64
	db.Model(&model.IPGeoCache{}).Where("ip_address = ?", "5.9.1.1").Count(&count)
	if count != 1 {
		t.Errorf("expected the online result to be cached, got %d rows", count)
	}
}

// TestGeoIPLookup_OnlineModeFailureDegradesGracefully confirms an
// unreachable/erroring third-party API in online mode returns an empty
// result (never an error) -- same graceful-degradation contract the
// offline path already has for a private/reserved IP.
func TestGeoIPLookup_OnlineModeFailureDegradesGracefully(t *testing.T) {
	db := openGeoIPTestDB(t)
	withFakeIPAPIServer(t, func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusInternalServerError)
	})

	systemConfig := NewSystemConfigService(db)
	if err := systemConfig.SetGeoIPOnlineEnabled(true); err != nil {
		t.Fatalf("failed to enable online mode: %v", err)
	}

	svc := NewGeoIPService(db, t.TempDir(), systemConfig)
	result, err := svc.Lookup("5.9.1.2")
	if err != nil {
		t.Fatalf("expected no error on a failed online lookup, got: %v", err)
	}
	if result.Country != nil || result.ASN != nil {
		t.Errorf("expected an entirely empty result on API failure, got %+v", result)
	}
}

// TestGeoIPLookup_OfflineIsDefaultWhenTogglePresentButFalse confirms the
// online toggle actually gates the online path -- with it explicitly set
// to false, Lookup must take the offline path (no readers loaded here) and
// never call the fake server at all, which would fail this test via
// t.Fatal from inside the handler if it were reached.
func TestGeoIPLookup_OfflineIsDefaultWhenTogglePresentButFalse(t *testing.T) {
	db := openGeoIPTestDB(t)
	withFakeIPAPIServer(t, func(w http.ResponseWriter, r *http.Request) {
		t.Fatal("online API must not be called when the toggle is off")
	})

	systemConfig := NewSystemConfigService(db)
	if err := systemConfig.SetGeoIPOnlineEnabled(false); err != nil {
		t.Fatalf("failed to set online mode off: %v", err)
	}

	svc := NewGeoIPService(db, t.TempDir(), systemConfig)
	result, err := svc.Lookup("8.8.4.4")
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if result.Country != nil {
		t.Errorf("expected an empty offline result with no readers loaded, got %+v", result)
	}
}
