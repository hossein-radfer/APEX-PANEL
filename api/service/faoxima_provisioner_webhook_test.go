package service

import (
	"encoding/json"
	"errors"
	"fmt"
	"net/http"
	"net/http/httptest"
	"strings"
	"sync"
	"testing"
	"time"

	"github.com/glebarez/sqlite"
	"gorm.io/gorm"

	"github.com/maahdima/mwp/api/dataservice/model"
)

// testFaoximaDomain stands in for an admin-configured BotSettings.
// FaoximaDomain in every test that needs webhook/mini-app URLs built --
// since faoximaDomain is now a per-instance field (set via
// ReloadFaoximaDomain) rather than a package-level constant, tests must set
// it explicitly on their own *FaoximaProvisionerService rather than relying
// on a shared default.
const testFaoximaDomain = "bot.example.com"

// fakeTelegramAPI is a minimal Telegram Bot API stand-in covering exactly
// what VerifyWebhooks touches: GET .../getWebhookInfo (reports whatever
// URL was last registered, per bot token) and POST .../setWebhook
// (records the new URL). Keyed by bot token so multiple "instances" in
// one test never collide with each other's webhook state, exactly like
// real per-bot-token Telegram state.
type fakeTelegramAPI struct {
	mu             sync.Mutex
	registeredURLs map[string]string // bot token -> currently registered webhook url
	failSetWebhook map[string]bool   // bot token -> force setWebhook to fail for this token
	invalidTokens  map[string]bool   // bot token -> every call returns HTTP 401, like a revoked/deleted bot
}

func newFakeTelegramAPI(initial map[string]string) *fakeTelegramAPI {
	urls := make(map[string]string, len(initial))
	for k, v := range initial {
		urls[k] = v
	}
	return &fakeTelegramAPI{registeredURLs: urls, failSetWebhook: make(map[string]bool), invalidTokens: make(map[string]bool)}
}

func (f *fakeTelegramAPI) server() *httptest.Server {
	return httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		f.mu.Lock()
		defer f.mu.Unlock()

		// Path shape: /bot<TOKEN>/<method>
		parts := strings.SplitN(strings.TrimPrefix(r.URL.Path, "/bot"), "/", 2)
		if len(parts) != 2 {
			w.WriteHeader(http.StatusNotFound)
			return
		}
		token, method := parts[0], parts[1]

		if f.invalidTokens[token] {
			w.WriteHeader(http.StatusUnauthorized)
			return
		}

		switch method {
		case "getWebhookInfo":
			w.Header().Set("Content-Type", "application/json")
			_ = json.NewEncoder(w).Encode(map[string]interface{}{
				"ok": true,
				"result": map[string]interface{}{
					"url": f.registeredURLs[token],
				},
			})
		case "setWebhook":
			if f.failSetWebhook[token] {
				w.WriteHeader(http.StatusInternalServerError)
				return
			}
			_ = r.ParseForm()
			f.registeredURLs[token] = r.FormValue("url")
			w.Header().Set("Content-Type", "application/json")
			_ = json.NewEncoder(w).Encode(map[string]interface{}{"ok": true, "result": true})
		case "deleteWebhook":
			f.registeredURLs[token] = ""
			w.WriteHeader(http.StatusOK)
		default:
			w.WriteHeader(http.StatusNotFound)
		}
	}))
}

func openFaoximaTestDB(t *testing.T) *gorm.DB {
	t.Helper()
	dsn := fmt.Sprintf("file:faoxima_webhook_test_%d?mode=memory&cache=shared", time.Now().UnixNano())
	db, err := gorm.Open(sqlite.Open(dsn), &gorm.Config{})
	if err != nil {
		t.Fatalf("failed to open test db: %v", err)
	}
	if err := db.AutoMigrate(&model.FaoximaInstance{}, &model.Reseller{}, &model.BotSettings{}); err != nil {
		t.Fatalf("failed to migrate test db: %v", err)
	}
	return db
}

// withFakeTelegramAPI points telegramAPIBaseURL at fake for the duration
// of one test, restoring the real default afterward -- telegramAPIBaseURL
// is a package-level var specifically so tests can do this (see its own
// doc comment).
func withFakeTelegramAPI(t *testing.T, fake *fakeTelegramAPI) *httptest.Server {
	t.Helper()
	srv := fake.server()
	original := telegramAPIBaseURL
	telegramAPIBaseURL = srv.URL
	t.Cleanup(func() {
		srv.Close()
		telegramAPIBaseURL = original
	})
	return srv
}

// TestVerifyWebhooks_ReRegistersMismatchedWebhook is the core regression
// test for the admin's own explicit disaster-recovery concern: "می‌ترسم
// اگه سرور عوض بشه ربات‌های فاکسیما... ست وبهوک انجام نده." Simulates
// exactly that scenario -- an ENABLED instance whose webhook (per
// Telegram's own getWebhookInfo) still points at an OLD server's URL --
// and confirms VerifyWebhooks detects the mismatch and re-registers the
// correct URL.
func TestVerifyWebhooks_ReRegistersMismatchedWebhook(t *testing.T) {
	db := openFaoximaTestDB(t)

	instance := model.FaoximaInstance{
		ResellerID:    1,
		InstanceSlug:  "reseller1",
		DBName:        "faoxima_reseller1",
		DBUser:        "faoxima_reseller1",
		DBPassword:    "pw",
		BotToken:      "TOKEN_A",
		WebhookSecret: "secret-a",
		Status:        model.FaoximaInstanceStatusEnabled,
	}
	if err := db.Create(&instance).Error; err != nil {
		t.Fatalf("failed to create instance: %v", err)
	}

	oldURL := "https://old-server.example.com/faoxima-resellers/reseller1/index.php"
	fake := newFakeTelegramAPI(map[string]string{"TOKEN_A": oldURL})
	withFakeTelegramAPI(t, fake)

	svc := NewFaoximaProvisionerService(db)
	svc.faoximaDomain = testFaoximaDomain
	svc.VerifyWebhooks()

	fake.mu.Lock()
	defer fake.mu.Unlock()
	got := fake.registeredURLs["TOKEN_A"]
	want := fmt.Sprintf("https://%s/faoxima-resellers/reseller1/index.php", testFaoximaDomain)
	if got != want {
		t.Errorf("expected webhook re-registered to %q, got %q", want, got)
	}
}

// TestVerifyWebhooks_LeavesCorrectWebhookAlone confirms an instance whose
// webhook is ALREADY correct is never touched -- VerifyWebhooks must be a
// pure read (getWebhookInfo only) in the common case, never
// unconditionally re-registering on every tick.
func TestVerifyWebhooks_LeavesCorrectWebhookAlone(t *testing.T) {
	db := openFaoximaTestDB(t)

	instance := model.FaoximaInstance{
		ResellerID:    2,
		InstanceSlug:  "reseller2",
		DBName:        "faoxima_reseller2",
		DBUser:        "faoxima_reseller2",
		DBPassword:    "pw",
		BotToken:      "TOKEN_B",
		WebhookSecret: "secret-b",
		Status:        model.FaoximaInstanceStatusEnabled,
	}
	if err := db.Create(&instance).Error; err != nil {
		t.Fatalf("failed to create instance: %v", err)
	}

	correctURL := fmt.Sprintf("https://%s/faoxima-resellers/reseller2/index.php", testFaoximaDomain)
	fake := newFakeTelegramAPI(map[string]string{"TOKEN_B": correctURL})
	withFakeTelegramAPI(t, fake)

	// Force setWebhook to fail for this token -- if VerifyWebhooks
	// mistakenly tried to re-register anyway, this test would fail loudly
	// via the resulting ERROR-alert path instead of silently passing.
	fake.failSetWebhook["TOKEN_B"] = true

	svc := NewFaoximaProvisionerService(db)
	svc.faoximaDomain = testFaoximaDomain
	svc.VerifyWebhooks()

	fake.mu.Lock()
	defer fake.mu.Unlock()
	if fake.registeredURLs["TOKEN_B"] != correctURL {
		t.Errorf("expected already-correct webhook to remain %q, got %q", correctURL, fake.registeredURLs["TOKEN_B"])
	}
}

// TestVerifyWebhooks_SkipsDisabledAndErrorInstances confirms only
// ENABLED instances are checked -- a DISABLED instance's webhook is
// SUPPOSED to be unregistered (see Disable's own deleteWebhook call), so
// checking it would be a false positive every single tick.
func TestVerifyWebhooks_SkipsDisabledAndErrorInstances(t *testing.T) {
	db := openFaoximaTestDB(t)

	disabled := model.FaoximaInstance{
		ResellerID: 3, InstanceSlug: "reseller3", DBName: "d3", DBUser: "u3", DBPassword: "pw",
		BotToken: "TOKEN_C", WebhookSecret: "secret-c", Status: model.FaoximaInstanceStatusDisabled,
	}
	errored := model.FaoximaInstance{
		ResellerID: 4, InstanceSlug: "reseller4", DBName: "d4", DBUser: "u4", DBPassword: "pw",
		BotToken: "TOKEN_D", WebhookSecret: "secret-d", Status: model.FaoximaInstanceStatusError,
	}
	if err := db.Create(&disabled).Error; err != nil {
		t.Fatalf("failed to create disabled instance: %v", err)
	}
	if err := db.Create(&errored).Error; err != nil {
		t.Fatalf("failed to create errored instance: %v", err)
	}

	fake := newFakeTelegramAPI(nil)
	withFakeTelegramAPI(t, fake)

	svc := NewFaoximaProvisionerService(db)
	svc.faoximaDomain = testFaoximaDomain
	svc.VerifyWebhooks()

	fake.mu.Lock()
	defer fake.mu.Unlock()
	if len(fake.registeredURLs) != 0 {
		t.Errorf("expected no webhook calls for disabled/error instances, got %v", fake.registeredURLs)
	}
}

// TestVerifyWebhooks_MarksInstanceErrorOnInvalidToken is the regression
// test for the confirmed, reported incident this session fixed: a
// reseller deleted their Faoxima bot via @BotFather (and set up a new,
// different bot), leaving the panel's stored token permanently rejected
// by Telegram (HTTP 401) -- previously, VerifyWebhooks treated this
// identically to a transient network hiccup (log a warning, silently move
// on), leaving FaoximaInstance.Status stuck at ENABLED forever with no
// indication to the reseller (whose bot had, in fact, completely stopped
// working) or the admin. Confirms VerifyWebhooks now flips Status to
// ERROR with a reseller-actionable ErrorMessage instead.
func TestVerifyWebhooks_MarksInstanceErrorOnInvalidToken(t *testing.T) {
	db := openFaoximaTestDB(t)

	instance := model.FaoximaInstance{
		ResellerID:    5,
		InstanceSlug:  "reseller5",
		DBName:        "faoxima_reseller5",
		DBUser:        "faoxima_reseller5",
		DBPassword:    "pw",
		BotToken:      "TOKEN_REVOKED",
		WebhookSecret: "secret-e",
		Status:        model.FaoximaInstanceStatusEnabled,
	}
	if err := db.Create(&instance).Error; err != nil {
		t.Fatalf("failed to create instance: %v", err)
	}

	fake := newFakeTelegramAPI(nil)
	fake.invalidTokens["TOKEN_REVOKED"] = true
	withFakeTelegramAPI(t, fake)

	svc := NewFaoximaProvisionerService(db)
	svc.faoximaDomain = testFaoximaDomain
	svc.VerifyWebhooks()

	var reloaded model.FaoximaInstance
	if err := db.Where("reseller_id = ?", 5).First(&reloaded).Error; err != nil {
		t.Fatalf("failed to reload instance: %v", err)
	}
	if reloaded.Status != model.FaoximaInstanceStatusError {
		t.Fatalf("expected status ERROR after invalid token detection, got %q", reloaded.Status)
	}
	if reloaded.ErrorMessage == nil || *reloaded.ErrorMessage == "" {
		t.Fatal("expected a non-empty ErrorMessage explaining what the reseller must do")
	}

	fake.mu.Lock()
	defer fake.mu.Unlock()
	if _, attemptedSetWebhook := fake.registeredURLs["TOKEN_REVOKED"]; attemptedSetWebhook {
		t.Error("expected no setWebhook attempt for a token telegram already rejected")
	}
}

// TestGetWebhookInfo_ReturnsSentinelErrorOnUnauthorized confirms
// getWebhookInfo distinguishes a revoked/invalid token (HTTP 401) from
// any other failure via the errFaoximaTokenInvalid sentinel -- the
// specific signal VerifyWebhooks relies on to take the ERROR-marking path
// instead of the generic "log and retry next tick" path.
func TestGetWebhookInfo_ReturnsSentinelErrorOnUnauthorized(t *testing.T) {
	db := openFaoximaTestDB(t)
	fake := newFakeTelegramAPI(nil)
	fake.invalidTokens["TOKEN_BAD"] = true
	withFakeTelegramAPI(t, fake)

	svc := NewFaoximaProvisionerService(db)
	svc.faoximaDomain = testFaoximaDomain
	_, err := svc.getWebhookInfo("TOKEN_BAD")
	if !errors.Is(err, errFaoximaTokenInvalid) {
		t.Fatalf("expected errFaoximaTokenInvalid, got: %v", err)
	}
}

// TestGetWebhookInfo_ParsesRealShapedResponse confirms getWebhookInfo
// correctly parses Telegram's own real response shape (a nested
// {"ok":true,"result":{"url":...}} object).
func TestGetWebhookInfo_ParsesRealShapedResponse(t *testing.T) {
	db := openFaoximaTestDB(t)
	fake := newFakeTelegramAPI(map[string]string{"TOKEN_E": "https://example.com/webhook"})
	withFakeTelegramAPI(t, fake)

	svc := NewFaoximaProvisionerService(db)
	svc.faoximaDomain = testFaoximaDomain
	info, err := svc.getWebhookInfo("TOKEN_E")
	if err != nil {
		t.Fatalf("getWebhookInfo failed: %v", err)
	}
	if info.Result.URL != "https://example.com/webhook" {
		t.Errorf("expected parsed url %q, got %q", "https://example.com/webhook", info.Result.URL)
	}
}
