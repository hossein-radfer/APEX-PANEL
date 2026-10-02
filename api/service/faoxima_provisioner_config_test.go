package service

import (
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/maahdima/mwp/api/dataservice/model"
)

// TestWriteConfig_IncludesProxyRequire is the regression test for a
// confirmed, reported bug found while migrating Faoxima's live instances:
// writeConfig generated config.php WITHOUT a `require_once .../proxy.php`
// line, even though every instance's copied template directory (and its
// own proxy.php helper) already supports per-scope SOCKS5 proxying for
// operators running on Iran-based hosts that cannot reach api.telegram.org
// directly. Without this line, faoxima_apply_curl_proxy/faoxima_proxy_for
// are simply undefined in the generated config.php's scope, so
// function_exists('faoxima_apply_curl_proxy') silently evaluates false and
// every outbound Telegram call is attempted directly -- exactly the
// "Connection refused" failure confirmed live on all four pre-existing
// instances during migration. New instances provisioned through this
// service must not repeat that gap.
func TestWriteConfig_IncludesProxyRequire(t *testing.T) {
	svc := &FaoximaProvisionerService{}

	dir := t.TempDir()
	instance := &model.FaoximaInstance{
		ResellerID:    1,
		InstanceSlug:  "reseller-1",
		BotToken:      "123:ABC",
		AdminChatID:   "999",
		WebhookSecret: "secret",
	}

	if err := svc.writeConfig(instance, dir, "faoxima_reseller1", "faoxima_reseller1", "pw"); err != nil {
		t.Fatalf("writeConfig failed: %v", err)
	}

	content, err := os.ReadFile(filepath.Join(dir, "config.php"))
	if err != nil {
		t.Fatalf("failed to read generated config.php: %v", err)
	}

	if !strings.Contains(string(content), "require_once __DIR__ . '/proxy.php';") {
		t.Fatalf("expected generated config.php to require proxy.php so faoxima_apply_curl_proxy/faoxima_proxy_for are defined, got:\n%s", content)
	}

	// The require must come AFTER $pdo is assigned, since proxy.php's own
	// faoxima_proxy_settings() reads $GLOBALS['pdo'] to load the stored
	// proxy configuration -- requiring it any earlier would see an unset
	// $pdo and silently fall back to "proxy disabled".
	pdoIndex := strings.Index(string(content), "$pdo = new PDO")
	requireIndex := strings.Index(string(content), "require_once __DIR__ . '/proxy.php';")
	if pdoIndex == -1 || requireIndex == -1 || requireIndex < pdoIndex {
		t.Fatalf("expected the proxy.php require to come after $pdo is assigned, got pdoIndex=%d requireIndex=%d", pdoIndex, requireIndex)
	}
}

// TestWriteConfig_PromotesLocalPdoToGlobals is the regression test for a
// confirmed, reported production incident: even with the proxy.php require
// present (see TestWriteConfig_IncludesProxyRequire above), a live instance's
// bot replied to every message with total silence -- traced over several
// hours of live debugging to faoxima_proxy_settings() (proxy.php) reading
// $GLOBALS['pdo'] specifically, not just any variable named $pdo in
// whatever scope happens to be active when it's called later (e.g. from
// inside a function in botapi.php/bootstrap.php, where a bare top-level
// $pdo is NOT automatically visible without `global $pdo;`). Without this
// explicit promotion, the proxy silently resolved to "not configured" on
// every real outbound sendMessage call, even though the instance's own
// stored proxy_telegram_url was correct and independently verified
// working via direct curl tests.
func TestWriteConfig_PromotesLocalPdoToGlobals(t *testing.T) {
	svc := &FaoximaProvisionerService{}

	dir := t.TempDir()
	instance := &model.FaoximaInstance{
		ResellerID:    2,
		InstanceSlug:  "reseller-2",
		BotToken:      "456:DEF",
		AdminChatID:   "888",
		WebhookSecret: "secret2",
	}

	if err := svc.writeConfig(instance, dir, "faoxima_reseller2", "faoxima_reseller2", "pw"); err != nil {
		t.Fatalf("writeConfig failed: %v", err)
	}

	content, err := os.ReadFile(filepath.Join(dir, "config.php"))
	if err != nil {
		t.Fatalf("failed to read generated config.php: %v", err)
	}

	if !strings.Contains(string(content), "$GLOBALS['pdo'] = $pdo;") {
		t.Fatalf("expected generated config.php to promote $pdo into $GLOBALS so proxy.php's faoxima_proxy_settings() can see it from any scope, got:\n%s", content)
	}

	// Must come after $pdo is assigned and before the proxy.php require,
	// otherwise proxy.php's own first read of $GLOBALS['pdo'] (which it
	// caches via a `static $cache` inside faoxima_proxy_settings) would
	// see nothing and permanently cache "proxy disabled" for the rest of
	// that request.
	pdoIndex := strings.Index(string(content), "$pdo = new PDO")
	globalsIndex := strings.Index(string(content), "$GLOBALS['pdo'] = $pdo;")
	requireIndex := strings.Index(string(content), "require_once __DIR__ . '/proxy.php';")
	if pdoIndex == -1 || globalsIndex == -1 || requireIndex == -1 || !(pdoIndex < globalsIndex && globalsIndex < requireIndex) {
		t.Fatalf("expected order $pdo assignment -> $GLOBALS promotion -> proxy.php require, got pdoIndex=%d globalsIndex=%d requireIndex=%d", pdoIndex, globalsIndex, requireIndex)
	}
}

// TestWriteConfig_DisablesStrictTelegramIpValidation verifies writeConfig
// disables the legacy Telegram-IP allowlist check, since a CDN/edge proxy
// in front of the webhook domain means the request's remote address is
// never actually Telegram's own IP; the webhook's secret-token check is
// the real authentication mechanism.
func TestWriteConfig_DisablesStrictTelegramIpValidation(t *testing.T) {
	svc := &FaoximaProvisionerService{}

	dir := t.TempDir()
	instance := &model.FaoximaInstance{
		ResellerID:    3,
		InstanceSlug:  "reseller-3",
		BotToken:      "789:GHI",
		AdminChatID:   "777",
		WebhookSecret: "secret3",
	}

	if err := svc.writeConfig(instance, dir, "faoxima_reseller3", "faoxima_reseller3", "pw"); err != nil {
		t.Fatalf("writeConfig failed: %v", err)
	}

	content, err := os.ReadFile(filepath.Join(dir, "config.php"))
	if err != nil {
		t.Fatalf("failed to read generated config.php: %v", err)
	}

	if !strings.Contains(string(content), "$telegramStrictIpValidation = false;") {
		t.Fatalf("expected generated config.php to disable checktelegramip()'s legacy IP allowlist (redundant with, and broken by, the Cloudflare-fronted webhook domain), got:\n%s", content)
	}
}
