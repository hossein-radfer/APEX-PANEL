package service

import (
	"encoding/json"
	"errors"
	"fmt"
	"net/http"
	"net/url"
	"os"
	"os/exec"
	"path/filepath"
	"strconv"
	"strings"
	"time"

	"go.uber.org/zap"
	"gorm.io/gorm"

	"github.com/maahdima/mwp/api/config"
	"github.com/maahdima/mwp/api/dataservice/model"
	"github.com/maahdima/mwp/api/utils"
)

// defaultFaoximaHTTPClient is the no-proxy fallback every
// FaoximaProvisionerService starts with -- net/http's package-level Get/
// Post/PostForm all use http.DefaultClient, which has NO timeout. A
// confirmed, reported production incident (see newTelegramBotAPI's own
// doc comment) traced the whole panel becoming unresponsive to exactly
// this shape of bug (a Telegram call with no timeout, blocking its caller
// forever whenever Telegram is unreachable from this Iran-hosted server).
// See the httpClient field's own doc comment for why this is no longer
// the ONLY client this service ever uses.
var defaultFaoximaHTTPClient = &http.Client{Timeout: 15 * time.Second}

// telegramAPIBaseURL is a package-level override point for every direct
// Telegram Bot API call this file makes -- production code never
// changes it from its real default, but tests (see
// faoxima_provisioner_webhook_test.go) point it at a local httptest
// server instead, so setWebhook/deleteWebhook/getWebhookInfo can be
// exercised against a real HTTP request/response cycle without ever
// touching the real Telegram API.
var telegramAPIBaseURL = "https://api.telegram.org"

// FaoximaProvisionerService is the "bot-as-a-service" feature's real
// implementation: each reseller gets a FULL, INDEPENDENT copy of the
// Faoxima Telegram sales bot -- the same one already used standalone on
// this server -- rather than a lightweight Go-native reimplementation.
// The reseller only ever provides their own Telegram bot token + chat ID
// from their reseller panel; everything else (a dedicated MySQL database,
// a dedicated MySQL user, a filesystem copy of the Faoxima codebase, an
// Apache-served subpath, Telegram webhook registration) is provisioned
// automatically by this service.
//
// Design: full-copy-per-reseller (not a shared codebase parameterized by
// tenant) -- chosen because Faoxima's own existing "Additional Bot
// Management" feature (install.sh's install_additional_bot) already
// works exactly this way, is a known-good pattern in THIS codebase, and
// needs zero changes to Faoxima's own PHP source (every file's
// require_once/__DIR__ usage already assumes "config.php lives right
// next to me"). One dedicated MySQL user per instance, GRANTed privileges
// scoped to ONLY that instance's own database, guarantees one reseller's
// Faoxima instance can never read or write another's data or the main
// panel's own database, purely through MySQL's own permission system --
// no shared code path to get wrong.
type FaoximaProvisionerService struct {
	db          *gorm.DB
	logger      *zap.Logger
	botNotifier *BotNotifier

	// httpClient is used for EVERY direct Telegram Bot API call this file
	// makes (setWebhook/deleteWebhook/getWebhookInfo/setMenuButton) --
	// starts as defaultFaoximaHTTPClient (no proxy) and is swapped for a
	// SOCKS5-routed client by ReloadHTTPClient whenever the admin's
	// BotSettings.Socks5* configuration says to. A confirmed, reported
	// gap this closes: BotSettings.Socks5Enabled's own doc comment already
	// claims it routes "ALL outbound Telegram API calls... AND every
	// reseller's bot-as-a-service instance," but until this field existed
	// only BotService.Reload (the panel's own admin bot) actually
	// consulted it -- every Faoxima instance's setWebhook/getWebhookInfo
	// call still went out over a direct, unproxied connection. On a
	// server where Telegram is network-filtered (this codebase's own
	// stated reason the SOCKS5 feature exists at all -- see
	// buildSocks5HTTPClient's own doc comment), this meant every Faoxima
	// instance's webhook registration would fail outright even with a
	// working proxy correctly configured and already in use by the main
	// bot.
	httpClient *http.Client

	// faoximaDomain is the operator's own public domain each Faoxima
	// instance's webhook/mini-app URLs are built under -- starts empty
	// (so a brand-new install never silently inherits anyone else's
	// domain) and is set from the panel's current BotSettings.FaoximaDomain
	// by ReloadFaoximaDomain, mirroring ReloadHTTPClient's identical
	// "read current BotSettings, update in place" pattern. See
	// BotSettings.FaoximaDomain's own doc comment for why this must be
	// admin-editable rather than a build-time constant.
	faoximaDomain string
}

func NewFaoximaProvisionerService(db *gorm.DB) *FaoximaProvisionerService {
	return &FaoximaProvisionerService{
		db:         db,
		logger:     zap.L().Named("FaoximaProvisionerService"),
		httpClient: defaultFaoximaHTTPClient,
	}
}

// ReloadFaoximaDomain updates the domain every Faoxima instance's
// webhook/mini-app URL is built under from the panel's current
// BotSettings -- called once at startup (cmd/main.go) and again every time
// the admin saves bot settings (BotSettingsController.UpdateSettings),
// exactly like ReloadHTTPClient, so a domain change takes effect
// immediately without a restart.
func (s *FaoximaProvisionerService) ReloadFaoximaDomain(settings *model.BotSettings) {
	s.faoximaDomain = settings.FaoximaDomain
}

// SetBotNotifier wires the main panel bot's own Telegram alert channel in
// after construction (mirrors every other service's identical
// SetBotNotifier pattern) -- used by VerifyWebhooks to tell the admin
// when a Faoxima instance's webhook was found broken and self-healed (or
// couldn't be), since that condition would otherwise be invisible: the
// reseller's bot just silently stops receiving Telegram updates, with no
// error anywhere in the panel UI to notice.
func (s *FaoximaProvisionerService) SetBotNotifier(notifier *BotNotifier) {
	s.botNotifier = notifier
}

// ReloadHTTPClient (re)builds the HTTP client every direct Telegram call
// in this file uses, from the panel's current BotSettings -- mirrors
// BotService.Reload's identical SOCKS5-or-direct decision exactly (see
// buildSocks5HTTPClient for the actual dialer construction, shared by
// both). Called once at startup (cmd/main.go) and again every time the
// admin saves bot settings (BotSettingsController.UpdateSettings), so a
// proxy address change takes effect immediately for Faoxima instances
// too, not just the main bot -- without this second call site, an admin
// fixing a broken/rotated proxy address would have every Faoxima webhook
// call keep failing until a full process restart, with nothing in the
// settings-save response indicating that.
//
// Falls back to defaultFaoximaHTTPClient (no proxy, direct connection) on
// any SOCKS5 dialer construction error, logged rather than returned --
// this runs from a settings-save path and a startup path, neither of
// which has a good way to block on a proxy misconfiguration; a bad proxy
// address should surface as Faoxima's own getWebhookInfo/setWebhook calls
// failing (visible via VerifyWebhooks' existing alerting), not as a
// silent settings-save failure or a panel that won't start.
func (s *FaoximaProvisionerService) ReloadHTTPClient(settings *model.BotSettings) {
	if !settings.Socks5Enabled || settings.Socks5Address == "" {
		s.httpClient = defaultFaoximaHTTPClient
		return
	}

	client, err := buildSocks5HTTPClient(settings.Socks5Address, settings.Socks5Username, settings.Socks5Password)
	if err != nil {
		s.logger.Error("failed to build SOCKS5 http client for faoxima instances, falling back to direct connection", zap.Error(err))
		s.httpClient = defaultFaoximaHTTPClient
		return
	}
	s.httpClient = client
}

// Configuration constants for this server's filesystem layout. Not
// admin-configurable (unlike, say, SystemConfigService's port override):
// these were set up once, by hand, alongside this feature's supporting
// web-server configuration, and changing them without also changing that
// configuration would break every existing instance. Set these to match
// your own deployment's actual layout before building. The webhook/mini-app
// DOMAIN itself is NOT here -- see faoximaDomain field's own doc comment
// for why that one is admin-editable (BotSettings.FaoximaDomain) instead.
const (
	faoximaTemplateDir  = "/opt/faoxima-instances/template"
	faoximaInstancesDir = "/var/www/html/faoxima-resellers"

	// faoximaMysqlDefaultsFile is a MySQL client config file providing
	// admin-level database credentials for provisioning each instance's
	// own database/user, read via the mysql CLI's --defaults-extra-file.
	faoximaMysqlDefaultsFile = "/etc/mysql/debian.cnf"
)

// faoximaDefaultTelegramProxy is the SOCKS5 proxy URL seeded as the
// default Telegram proxy for every newly-provisioned Faoxima instance
// (see ensureTelegramProxy below) -- read from
// FAOXIMA_DEFAULT_TELEGRAM_PROXY (config.GetFaoximaDefaultTelegramProxy),
// never hardcoded. Empty by default, meaning a fresh instance uses a
// direct connection until its own reseller configures a proxy. If your
// deployment's network blocks direct access to api.telegram.org, set
// this env var to a SOCKS5 proxy you control; a domain name registered
// through a provider like Cloudflare (rather than a bare IP) is
// recommended, so the proxy can be repointed later without touching
// already-provisioned instances. This is independent of the panel's own
// BotSettings.Socks5* client (see ReloadHTTPClient above), since that
// only covers this process's own direct Telegram calls, never the
// separate PHP process that runs each Faoxima instance's bot
// conversation.
var faoximaDefaultTelegramProxy = config.GetFaoximaDefaultTelegramProxy()

// slugFor derives this reseller's stable, filesystem/MySQL-identifier-safe
// slug from their ID alone (never their name/username, which can change)
// -- used for the instance directory name, the MySQL database/user name
// prefix, and the webhook URL subpath.
func slugFor(resellerID uint) string {
	return fmt.Sprintf("reseller%d", resellerID)
}

// Provision creates a brand-new Faoxima instance for resellerID: a MySQL
// database + user, a filesystem copy of the template codebase, a written
// config.php, database table initialization, and Telegram webhook
// registration -- in that order, matching install_additional_bot's own
// sequence. Returns the created model.FaoximaInstance row. Idempotent
// guard: refuses if resellerID already has a WORKING instance (Enabled/
// Disabled/Provisioning); a prior instance stuck in Error status (e.g. a
// bad bot token that failed Telegram's setWebhook call) is automatically
// cleaned up and retried instead of permanently blocking this reseller
// from ever provisioning again -- confirmed, caught-in-testing bug: an
// admin/reseller typo-ing their token on the first attempt had no way to
// retry without knowing to call Remove first, since the failed row still
// occupied ResellerID's uniqueIndex slot.
func (s *FaoximaProvisionerService) Provision(resellerID uint, botToken, adminChatID string) (*model.FaoximaInstance, error) {
	var existing model.FaoximaInstance
	err := s.db.Where("reseller_id = ?", resellerID).First(&existing).Error
	if err == nil {
		if existing.Status != model.FaoximaInstanceStatusError {
			return nil, fmt.Errorf("این نماینده از قبل یک نمونه‌ی فعال از ربات ایکس دارد")
		}
		// Best-effort cleanup of whatever the failed attempt left behind
		// (a partially-created database/directory) before retrying fresh.
		if removeErr := s.Remove(resellerID); removeErr != nil {
			s.logger.Warn("failed to clean up errored faoxima instance before retry", zap.Uint("reseller_id", resellerID), zap.Error(removeErr))
		}
	} else if !errors.Is(err, gorm.ErrRecordNotFound) {
		return nil, err
	}

	slug := slugFor(resellerID)
	dbName := "faoxima_" + slug
	dbUser := "faoxima_" + slug
	dbPassword := utils.RandomString(20)
	webhookSecret := utils.RandomString(24)
	instanceDir := filepath.Join(faoximaInstancesDir, slug)

	instance := model.FaoximaInstance{
		ResellerID:    resellerID,
		InstanceSlug:  slug,
		DBName:        dbName,
		DBUser:        dbUser,
		DBPassword:    dbPassword,
		BotToken:      botToken,
		AdminChatID:   adminChatID,
		WebhookSecret: webhookSecret,
		Status:        model.FaoximaInstanceStatusProvisioning,
	}
	if err := s.db.Create(&instance).Error; err != nil {
		return nil, fmt.Errorf("failed to record instance: %w", err)
	}

	if err := s.provisionSteps(&instance, dbName, dbUser, dbPassword, instanceDir); err != nil {
		errMsg := err.Error()
		s.db.Model(&instance).Updates(map[string]interface{}{
			"status":        model.FaoximaInstanceStatusError,
			"error_message": errMsg,
		})
		return nil, err
	}

	s.db.Model(&instance).Updates(map[string]interface{}{
		"status":        model.FaoximaInstanceStatusEnabled,
		"error_message": nil,
	})
	instance.Status = model.FaoximaInstanceStatusEnabled
	return &instance, nil
}

func (s *FaoximaProvisionerService) provisionSteps(instance *model.FaoximaInstance, dbName, dbUser, dbPassword, instanceDir string) error {
	if err := s.createDatabase(dbName, dbUser, dbPassword); err != nil {
		return fmt.Errorf("ساخت پایگاه‌داده ناموفق بود: %w", err)
	}

	if err := s.copyTemplate(instanceDir); err != nil {
		return fmt.Errorf("کپی فایل‌های ربات ایکس ناموفق بود: %w", err)
	}

	if err := s.writeConfig(instance, instanceDir, dbName, dbUser, dbPassword); err != nil {
		return fmt.Errorf("نوشتن فایل تنظیمات ناموفق بود: %w", err)
	}

	if err := s.applyPermissions(instanceDir); err != nil {
		return fmt.Errorf("تنظیم دسترسی فایل‌ها ناموفق بود: %w", err)
	}

	// initializeTables (table.php) registers the Telegram webhook ITSELF,
	// internally, using its own freshly-generated secret -- see this
	// function's own doc comment on why this service must NEVER also call
	// setWebhook on its own. This must run BEFORE we read the real secret
	// back below.
	if err := s.initializeTables(instance.InstanceSlug); err != nil {
		return fmt.Errorf("مقداردهی اولیه‌ی جداول دیتابیس ناموفق بود: %w", err)
	}

	// Belt-and-suspenders for the proxy columns specifically -- see
	// faoximaDefaultTelegramProxy's own doc comment for the confirmed,
	// live-reproduced way table.php's own seeding of these can be
	// silently skipped even on an HTTP-200 initializeTables call.
	if err := s.ensureTelegramProxy(dbName); err != nil {
		return fmt.Errorf("تنظیم پروکسی تلگرام ناموفق بود: %w", err)
	}

	// CONFIRMED, LIVE-REPRODUCED BUG this works around: table.php (see
	// that file's own "setting.webhook_secret_token" block, around
	// addFieldToTable("setting", "webhook_secret_token", ...)) already
	// generates its own random secret, stores it in the setting table,
	// AND calls Telegram's setWebhook itself with the correct URL/secret
	// pair -- it does NOT read $secrettoken from config.php at all (that
	// field is effectively vestigial for the Telegram webhook path; the
	// earlier assumption in this file's history that Faoxima's webhook
	// auth checked admin.password was WRONG -- that column is used by a
	// completely different integration webhook in webhooks.php, unrelated
	// to Telegram). This service's OWN prior setWebhook call, made AFTER
	// initializeTables with ITS OWN generated WebhookSecret, was
	// therefore not just redundant but actively harmful: it overwrote
	// table.php's own correct registration with a secret botapi.php's
	// hash_equals check against setting.webhook_secret_token could never
	// match, silently rejecting every single real update from Telegram
	// with no visible error (confirmed on a live reseller instance --
	// botapi.php's own reject-marker flag files in the system temp
	// directory were the only trace). The fix: never call setWebhook
	// ourselves at all; only read back the real secret table.php already
	// registered, purely so this row's own WebhookSecret field reflects
	// reality (used by Disable/Enable's own deleteWebhook/setWebhook
	// calls, which must also use this same real value, not one this
	// service invents).
	realSecret, err := s.readSeededWebhookSecret(dbName, instance.DBUser, instance.DBPassword)
	if err != nil {
		return fmt.Errorf("خواندن رمز وبهوک تولیدشده ناموفق بود: %w", err)
	}
	instance.WebhookSecret = realSecret
	if err := s.db.Model(&model.FaoximaInstance{}).Where("id = ?", instance.ID).Update("webhook_secret", realSecret).Error; err != nil {
		return fmt.Errorf("ذخیره رمز وبهوک ناموفق بود: %w", err)
	}

	// Registers this instance's own app/index.php mini-app as the bot's
	// Telegram "Menu Button" -- confirmed, live-reproduced gap this fixes:
	// Faoxima's mini-app (app/index.php) is a genuine Telegram Mini App
	// that reads Telegram.WebApp.initData for auth, which Telegram ONLY
	// ever populates when the page is opened through a proper Mini App
	// launch surface (a menu-button web_app, or an inline/keyboard
	// web_app button) -- opening the same URL as a plain browser link
	// (which is all a customer/reseller could otherwise do with no menu
	// button configured) leaves initData empty and the app fails
	// immediately with "initData unavailable." Best-effort: a failure
	// here must not fail the whole provisioning flow, since the bot
	// itself is already fully functional without the mini-app menu
	// button (Faoxima's core sales flow is plain Telegram messages/
	// buttons, not the mini-app).
	miniAppURL := fmt.Sprintf("https://%s/faoxima-resellers/%s/app/index.php", s.faoximaDomain, instance.InstanceSlug)
	if err := s.setMenuButton(instance.BotToken, miniAppURL); err != nil {
		s.logger.Warn("failed to register mini-app menu button, bot itself is still fully usable", zap.Uint("reseller_id", instance.ResellerID), zap.Error(err))
	}

	return nil
}

// setMenuButton mirrors setWebhook's own shape -- registers a "web_app"
// type Telegram Chat Menu Button pointing at this instance's own mini-app
// URL, so the bot's own chat window shows a tappable menu button that
// launches the mini-app correctly (with Telegram.WebApp.initData
// populated), instead of the customer/reseller having no way to open the
// mini-app as an actual Telegram Mini App at all.
func (s *FaoximaProvisionerService) setMenuButton(botToken, miniAppURL string) error {
	apiURL := fmt.Sprintf(telegramAPIBaseURL+"/bot%s/setChatMenuButton", botToken)
	payload := fmt.Sprintf(`{"menu_button":{"type":"web_app","text":"پنل کاربری","web_app":{"url":"%s"}}}`, miniAppURL)
	// http.Post uses http.DefaultClient, which has NO timeout -- a
	// confirmed, reported production incident (see newTelegramBotAPI's
	// own doc comment) traced the panel becoming completely unresponsive
	// to exactly this pattern elsewhere (a Telegram call with no timeout,
	// blocking its caller forever on a server where Telegram is
	// sometimes network-filtered). This call sits on a real HTTP request
	// path (Provision/Enable), so the same fix applies here.
	resp, err := s.httpClient.Post(apiURL, "application/json", strings.NewReader(payload))
	if err != nil {
		return err
	}
	defer resp.Body.Close()
	if resp.StatusCode != http.StatusOK {
		return fmt.Errorf("telegram setChatMenuButton returned HTTP %d", resp.StatusCode)
	}
	return nil
}

// readSeededWebhookSecret reads back the ACTUAL webhook secret table.php
// generated and already registered with Telegram, from this instance's
// own setting.webhook_secret_token column -- see provisionSteps' own doc
// comment for why this service must read this value rather than ever
// calling Telegram's setWebhook itself.
func (s *FaoximaProvisionerService) readSeededWebhookSecret(dbName, dbUser, dbPassword string) (string, error) {
	cmd := exec.Command("mysql", "-u", dbUser, "-N", "-e", "SELECT webhook_secret_token FROM setting LIMIT 1;", dbName)
	cmd.Env = append(os.Environ(), "MYSQL_PWD="+dbPassword)
	output, err := cmd.Output()
	if err != nil {
		if exitErr, ok := err.(*exec.ExitError); ok {
			return "", fmt.Errorf("%w: %s", err, strings.TrimSpace(string(exitErr.Stderr)))
		}
		return "", err
	}
	secret := strings.TrimSpace(string(output))
	if secret == "" {
		return "", fmt.Errorf("admin table has no seeded password row yet")
	}
	return secret, nil
}

// ensureTelegramProxy makes sure this instance's own setting row has a
// working Telegram proxy configured, regardless of whether table.php's own
// seeding of proxy_telegram_status/proxy_telegram_url actually ran -- see
// faoximaDefaultTelegramProxy's own doc comment for the confirmed way that
// seeding can be silently skipped. Uses runMysqlAdmin (the debian-sys-maint
// account), not this instance's own dbUser/dbPassword, specifically so this
// still works even when the columns don't exist yet (this instance's own
// user has no ALTER privilege gap here since createDatabase already GRANTed
// ALL, but going through the admin account keeps this symmetric with
// createDatabase/table-creation, which must run before any per-instance
// credential is usable for anything).
//
// Idempotent by design (checks INFORMATION_SCHEMA before each ALTER, exactly
// like table.php's own addFieldToTable), so calling this on an
// already-correct instance (the common case once table.php's seeding did
// run) is a harmless no-op, not a second competing write.
func (s *FaoximaProvisionerService) ensureTelegramProxy(dbName string) error {
	columns := []struct {
		name, datatype string
	}{
		{"proxy_telegram_status", "VARCHAR(20)"},
		{"proxy_telegram_url", "VARCHAR(500)"},
		{"proxy_panel_status", "VARCHAR(20)"},
		{"proxy_panel_url", "VARCHAR(500)"},
	}
	for _, col := range columns {
		checkStmt := fmt.Sprintf(
			"SET @exists := (SELECT COUNT(*) FROM INFORMATION_SCHEMA.COLUMNS WHERE TABLE_SCHEMA = '%s' AND TABLE_NAME = 'setting' AND COLUMN_NAME = '%s'); "+
				"SET @ddl := IF(@exists = 0, 'ALTER TABLE `%s`.`setting` ADD COLUMN `%s` %s', 'SELECT 1'); "+
				"PREPARE stmt FROM @ddl; EXECUTE stmt; DEALLOCATE PREPARE stmt;",
			dbName, col.name, dbName, col.name, col.datatype,
		)
		if err := s.runMysqlAdmin(checkStmt); err != nil {
			return fmt.Errorf("failed to ensure column %s: %w", col.name, err)
		}
	}

	// Only force the working default onto a FRESH column (one this call
	// just added) -- an operator who deliberately disabled the proxy or
	// pointed it at their own address later must never have that choice
	// silently overwritten by a later Enable/re-provision.
	updateStmt := fmt.Sprintf(
		"UPDATE `%s`.`setting` SET "+
			"proxy_telegram_status = COALESCE(NULLIF(proxy_telegram_status, ''), '1'), "+
			"proxy_telegram_url = COALESCE(NULLIF(proxy_telegram_url, ''), '%s');",
		dbName, faoximaDefaultTelegramProxy,
	)
	if err := s.runMysqlAdmin(updateStmt); err != nil {
		return fmt.Errorf("failed to seed proxy defaults: %w", err)
	}
	return nil
}

// createDatabase runs the same CREATE DATABASE / CREATE USER / GRANT
// sequence install_additional_bot() already uses, via the
// debian-sys-maint administrative account instead of a stored root
// password (see faoximaMysqlDefaultsFile's own doc comment). MAX_USER_
// CONNECTIONS/MAX_QUERIES_PER_HOUR limits are added on the CREATE USER
// statement itself (a one-line addition over the reference bash script)
// so one runaway or malicious reseller instance cannot starve MySQL
// resources shared with every other instance and the main panel.
func (s *FaoximaProvisionerService) createDatabase(dbName, dbUser, dbPassword string) error {
	statements := []string{
		fmt.Sprintf("CREATE DATABASE `%s` CHARACTER SET utf8mb4 COLLATE utf8mb4_unicode_ci;", dbName),
		fmt.Sprintf("CREATE USER '%s'@'localhost' IDENTIFIED BY '%s' WITH MAX_USER_CONNECTIONS 50 MAX_QUERIES_PER_HOUR 100000;", dbUser, dbPassword),
		fmt.Sprintf("GRANT ALL PRIVILEGES ON `%s`.* TO '%s'@'localhost';", dbName, dbUser),
		"FLUSH PRIVILEGES;",
	}
	for _, stmt := range statements {
		if err := s.runMysqlAdmin(stmt); err != nil {
			return err
		}
	}
	return nil
}

func (s *FaoximaProvisionerService) runMysqlAdmin(sqlStatement string) error {
	cmd := exec.Command("mysql", "--defaults-file="+faoximaMysqlDefaultsFile, "-e", sqlStatement)
	output, err := cmd.CombinedOutput()
	if err != nil {
		return fmt.Errorf("%w: %s", err, strings.TrimSpace(string(output)))
	}
	return nil
}

// copyTemplate makes a full, independent filesystem copy of the template
// Faoxima codebase into this reseller's own instance directory -- `cp -r`
// is used (not a Go-native file-tree walk) since it's a single, well-
// understood syscall-level copy that correctly preserves the template's
// directory structure and permission bits in one step.
func (s *FaoximaProvisionerService) copyTemplate(instanceDir string) error {
	if _, err := os.Stat(faoximaTemplateDir); err != nil {
		return fmt.Errorf("template directory missing at %s: %w", faoximaTemplateDir, err)
	}
	if err := os.MkdirAll(filepath.Dir(instanceDir), 0o755); err != nil {
		return err
	}

	cmd := exec.Command("cp", "-r", faoximaTemplateDir, instanceDir)
	output, err := cmd.CombinedOutput()
	if err != nil {
		return fmt.Errorf("%w: %s", err, strings.TrimSpace(string(output)))
	}
	return nil
}

// writeConfig writes config.php in the exact shape Faoxima's own
// install.sh generates (see install_additional_bot's heredoc) -- the
// bot's entire PHP codebase reads $dbname/$usernamedb/$passworddb/
// $APIKEY/$adminnumber/$domainhosts/$usernamebot as plain global
// variables from this one file, with no other configuration mechanism.
//
// Writes this instance's config.php. Two settings worth noting:
//  1. $GLOBALS['pdo'] = $pdo -- the proxy configuration helper reads the
//     PDO handle from globals, not a local variable, so it must be
//     explicitly promoted here for the instance's configured SOCKS5
//     proxy (if any) to actually apply to its outbound Telegram calls.
//  2. $telegramStrictIpValidation = false -- the webhook endpoint sits
//     behind a CDN/edge proxy, so the request's remote address is the
//     edge network's IP, not Telegram's; the webhook's own secret-token
//     check is the authentication mechanism that matters here.
func (s *FaoximaProvisionerService) writeConfig(instance *model.FaoximaInstance, instanceDir, dbName, dbUser, dbPassword string) error {
	configPath := filepath.Join(instanceDir, "config.php")
	domainAndPath := fmt.Sprintf("%s/faoxima-resellers/%s", s.faoximaDomain, instance.InstanceSlug)

	content := fmt.Sprintf(`<?php
$APIKEY = %s;
$usernamedb = %s;
$passworddb = %s;
$dbname = %s;
$domainhosts = %s;
$adminnumber = %s;
$usernamebot = %s;
$secrettoken = %s;
$connect = mysqli_connect('localhost', $usernamedb, $passworddb, $dbname);
if ($connect->connect_error) {
    die('Database connection failed: ' . $connect->connect_error);
}
mysqli_set_charset($connect, 'utf8mb4');
$options = [
    PDO::ATTR_ERRMODE            => PDO::ERRMODE_EXCEPTION,
    PDO::ATTR_DEFAULT_FETCH_MODE => PDO::FETCH_ASSOC,
    PDO::ATTR_EMULATE_PREPARES   => false,
];
$dsn = "mysql:host=localhost;dbname=$dbname;charset=utf8mb4";
try {
    $pdo = new PDO($dsn, $usernamedb, $passworddb, $options);
} catch (\PDOException $e) {
    throw new \PDOException($e->getMessage(), (int)$e->getCode());
}
$GLOBALS['pdo'] = $pdo;
require_once __DIR__ . '/proxy.php';
$telegramStrictIpValidation = false;
`,
		phpStringLiteral(instance.BotToken),
		phpStringLiteral(dbUser),
		phpStringLiteral(dbPassword),
		phpStringLiteral(dbName),
		phpStringLiteral(domainAndPath),
		phpStringLiteral(instance.AdminChatID),
		phpStringLiteral(instance.InstanceSlug),
		phpStringLiteral(instance.WebhookSecret),
	)

	return os.WriteFile(configPath, []byte(content), 0o644)
}

// phpStringLiteral renders a Go string as a single-quoted PHP string
// literal, escaping backslashes and single quotes -- every value written
// into config.php (bot token, DB password, etc) goes through this rather
// than naive string interpolation, since a value containing a literal `'`
// would otherwise break out of the PHP string and corrupt the file.
func phpStringLiteral(value string) string {
	escaped := strings.ReplaceAll(value, `\`, `\\`)
	escaped = strings.ReplaceAll(escaped, `'`, `\'`)
	return "'" + escaped + "'"
}

// applyPermissions mirrors grant_file_permissions() from install.sh
// exactly: www-data ownership recursively, 755/644 baseline, config.php
// writable (Faoxima's own admin panel can rewrite some of its fields at
// runtime), and the specific runtime-writable subdirectories Faoxima
// itself expects to write logs/cache/session/payment data into.
func (s *FaoximaProvisionerService) applyPermissions(instanceDir string) error {
	if err := exec.Command("chown", "-R", "www-data:www-data", instanceDir).Run(); err != nil {
		return err
	}
	if err := exec.Command("find", instanceDir, "-type", "d", "-exec", "chmod", "755", "{}", "+").Run(); err != nil {
		return err
	}
	if err := exec.Command("find", instanceDir, "-type", "f", "-exec", "chmod", "644", "{}", "+").Run(); err != nil {
		return err
	}
	_ = exec.Command("chmod", "666", filepath.Join(instanceDir, "config.php")).Run()

	writableDirs := []string{"logs", "storage", "cache", "tmp", "sessions", "cron", "cronbot", "sub", "payment", "re", "vpnbot", "infocard_fonts"}
	for _, dir := range writableDirs {
		full := filepath.Join(instanceDir, dir)
		if _, err := os.Stat(full); err != nil {
			continue
		}
		_ = exec.Command("find", full, "-type", "d", "-exec", "chmod", "775", "{}", "+").Run()
		_ = exec.Command("find", full, "-type", "f", "-exec", "chmod", "664", "{}", "+").Run()
	}
	return nil
}

// setWebhook registers this instance's own webhook URL + secret with
// Telegram, exactly matching install_additional_bot's own curl call --
// Telegram itself enforces the secret_token on every subsequent webhook
// POST (via the X-Telegram-Bot-Api-Secret-Token header), and Faoxima's
// own admin table stores this same secret as its "admin.password" row
// (written by table.php's own seeding, keyed off $secrettoken already
// present in config.php by the time table.php runs) for its own webhook
// auth check.
func (s *FaoximaProvisionerService) setWebhook(botToken, webhookURL, secret string) error {
	apiURL := fmt.Sprintf(telegramAPIBaseURL+"/bot%s/setWebhook", botToken)
	form := url.Values{
		"url":          {webhookURL},
		"secret_token": {secret},
	}
	resp, err := s.httpClient.PostForm(apiURL, form)
	if err != nil {
		return err
	}
	defer resp.Body.Close()
	if resp.StatusCode != http.StatusOK {
		return fmt.Errorf("telegram setWebhook returned HTTP %d", resp.StatusCode)
	}
	return nil
}

// deleteWebhook unregisters this instance's webhook -- called on Disable
// and Remove so a disabled/removed instance's bot token stops receiving
// updates immediately, matching Telegram's own recommended lifecycle.
func (s *FaoximaProvisionerService) deleteWebhook(botToken string) error {
	apiURL := fmt.Sprintf(telegramAPIBaseURL+"/bot%s/deleteWebhook", botToken)
	resp, err := s.httpClient.Get(apiURL)
	if err != nil {
		return err
	}
	defer resp.Body.Close()
	return nil
}

// getWebhookInfo reads Telegram's own record of what URL is currently
// registered for botToken -- the read half of the setWebhook/deleteWebhook
// pair above, used by VerifyWebhooks to detect drift between "what
// Telegram thinks this bot's webhook is" and "what it should be," rather
// than blindly re-registering on every check.
type faoximaWebhookInfo struct {
	OK     bool `json:"ok"`
	Result struct {
		URL string `json:"url"`
	} `json:"result"`
}

// errFaoximaTokenInvalid is returned by getWebhookInfo when Telegram
// itself rejects the stored bot token outright (HTTP 401 -- Telegram's
// stable, documented response for a token that was revoked, e.g. the
// reseller deleted the bot via @BotFather, or never-valid). This is a
// DIFFERENT failure mode from "webhook URL drift" (the case
// VerifyWebhooks' own doc comment already covers, e.g. a server
// migration): drift can be self-healed by re-registering the SAME token's
// webhook, but an invalid token can never be healed automatically --
// there is no way for this panel to discover, on its own, what the
// reseller's replacement bot's token is. See VerifyWebhooks' own doc
// comment for the confirmed, reported incident this distinction exists
// to fix: previously, a dead token just logged a warning and was silently
// skipped forever, leaving FaoximaInstance.Status stuck at ENABLED with
// no indication to the reseller or admin that the bot had stopped working
// entirely.
var errFaoximaTokenInvalid = errors.New("telegram rejected this bot token (likely revoked/deleted)")

func (s *FaoximaProvisionerService) getWebhookInfo(botToken string) (*faoximaWebhookInfo, error) {
	apiURL := fmt.Sprintf(telegramAPIBaseURL+"/bot%s/getWebhookInfo", botToken)
	resp, err := s.httpClient.Get(apiURL)
	if err != nil {
		return nil, err
	}
	defer resp.Body.Close()
	if resp.StatusCode == http.StatusUnauthorized {
		return nil, errFaoximaTokenInvalid
	}
	if resp.StatusCode != http.StatusOK {
		return nil, fmt.Errorf("telegram getWebhookInfo returned HTTP %d", resp.StatusCode)
	}
	var info faoximaWebhookInfo
	if err := json.NewDecoder(resp.Body).Decode(&info); err != nil {
		return nil, err
	}
	if !info.OK {
		return nil, errors.New("telegram getWebhookInfo returned ok=false")
	}
	return &info, nil
}

// VerifyWebhooks is the scheduled job entry point for the disaster-
// recovery safety net the admin explicitly asked for: "می‌ترسم اگه سرور
// عوض بشه ربات‌های فاکسیما هیچ‌کدام کار نکنند یا ست وبهوک انجام نده."
// Every ENABLED instance's webhook is provisioned once, at Provision/
// Enable time, by directly calling Telegram's setWebhook API -- nothing
// in this codebase previously re-checked that registration ever again.
// Restoring the panel's database onto a NEW server (a different IP,
// though the same faoximaDomain is assumed to already point at it) does
// not, by itself, re-run that registration: Telegram's own webhook
// record for each instance's bot token is external state that a
// database restore cannot touch, so after a restore every Faoxima
// instance's bot would silently stop receiving any Telegram traffic
// with nothing in this panel's own UI ever indicating a problem, since
// FaoximaInstance.Status would still read ENABLED (that column only
// reflects this panel's OWN last action, never Telegram's actual
// current state).
//
// This is deliberately RE-VERIFY, not RE-REGISTER-UNCONDITIONALLY: it
// reads Telegram's own current webhook URL via getWebhookInfo and only
// calls setWebhook again when that URL doesn't match what this instance
// should have -- covering not just a full server migration but any
// other cause of drift (Telegram silently clearing a webhook after
// repeated delivery failures, for one, which is Telegram's own
// documented behavior). A instance found broken is both self-healed
// (re-registered immediately, best-effort) AND reported to the admin,
// so a migration is noticed even if the self-heal itself fails for some
// reason (e.g. the new server's own domain/DNS/TLS isn't ready yet,
// which setWebhook would surface as an error here).
func (s *FaoximaProvisionerService) VerifyWebhooks() {
	var instances []model.FaoximaInstance
	if err := s.db.Where("status = ?", model.FaoximaInstanceStatusEnabled).Find(&instances).Error; err != nil {
		s.logger.Error("failed to list enabled faoxima instances for webhook verification", zap.Error(err))
		return
	}

	for _, instance := range instances {
		expectedURL := fmt.Sprintf("https://%s/faoxima-resellers/%s/index.php", s.faoximaDomain, instance.InstanceSlug)

		info, err := s.getWebhookInfo(instance.BotToken)
		if errors.Is(err, errFaoximaTokenInvalid) {
			s.logger.Warn("faoxima instance bot token rejected by telegram, marking as error",
				zap.Uint("reseller_id", instance.ResellerID))
			s.markInstanceTokenInvalid(instance.ResellerID)
			continue
		}
		if err != nil {
			// A transient failure (network hiccup, Telegram outage) --
			// distinct from errFaoximaTokenInvalid above, this is not
			// treated as evidence the instance is actually broken; the
			// next scheduled tick gets another chance to check cleanly.
			s.logger.Warn("failed to read faoxima instance webhook info",
				zap.Uint("reseller_id", instance.ResellerID), zap.Error(err))
			continue
		}
		if info.Result.URL == expectedURL {
			continue
		}

		s.logger.Warn("faoxima instance webhook mismatch detected, re-registering",
			zap.Uint("reseller_id", instance.ResellerID),
			zap.String("expected", expectedURL),
			zap.String("actual", info.Result.URL))

		healErr := s.setWebhook(instance.BotToken, expectedURL, instance.WebhookSecret)
		s.notifyWebhookDrift(instance.ResellerID, info.Result.URL, expectedURL, healErr)
	}
}

// markInstanceTokenInvalid is VerifyWebhooks' response to
// errFaoximaTokenInvalid -- there is no automatic fix (see that error's
// own doc comment for why), so this does the two things that ARE possible
// without the reseller's input: (1) flip Status to ERROR with a
// Persian, reseller-facing ErrorMessage explaining exactly what to do
// (use "تغییر توکن ربات" with their NEW bot's token), so
// FaoximaForm.tsx's existing ERROR-status UI surfaces this immediately
// instead of the reseller seeing a silently-dead "Enabled" bot, and (2)
// alerts the admin the same way NotifyFaoximaWebhookBroken already does
// for the drift case, so this doesn't rely on the reseller noticing and
// reporting it themselves.
func (s *FaoximaProvisionerService) markInstanceTokenInvalid(resellerID uint) {
	const errorMessage = "توکن ربات دیگر معتبر نیست (احتمالاً ربات قبلی از طریق @BotFather حذف یا توکن آن بازنشانی شده است). لطفاً یک ربات جدید در تلگرام بسازید و توکن آن را از بخش «تغییر توکن ربات» وارد کنید."

	if err := s.db.Model(&model.FaoximaInstance{}).Where("reseller_id = ?", resellerID).Updates(map[string]interface{}{
		"status":        model.FaoximaInstanceStatusError,
		"error_message": errorMessage,
	}).Error; err != nil {
		s.logger.Error("failed to mark faoxima instance as error after invalid token detection",
			zap.Uint("reseller_id", resellerID), zap.Error(err))
	}

	if s.botNotifier == nil {
		return
	}
	settings, err := s.botNotifier.settingsService.GetOrCreate()
	if err != nil || settings.AdminChatID == "" {
		return
	}
	s.botNotifier.NotifyFaoximaTokenInvalid(settings, resellerID)
}

func (s *FaoximaProvisionerService) notifyWebhookDrift(resellerID uint, actualURL, expectedURL string, healErr error) {
	if s.botNotifier == nil {
		return
	}
	settings, err := s.botNotifier.settingsService.GetOrCreate()
	if err != nil || settings.AdminChatID == "" {
		return
	}

	if healErr == nil {
		s.botNotifier.NotifyFaoximaWebhookHealed(settings, resellerID, actualURL, expectedURL)
		return
	}
	s.botNotifier.NotifyFaoximaWebhookBroken(settings, resellerID, actualURL, expectedURL, healErr.Error())
}

// initializeTables triggers table.php over HTTP, exactly matching
// install_additional_bot's own "curl the freshly-deployed table.php URL"
// step -- table.php is fully idempotent (existence-checks before every
// CREATE/ALTER) and self-seeds every default row Faoxima needs to be
// immediately usable (admin row, shop settings, default text templates,
// etc -- see that file's own migration blocks), so no manual SQL seeding
// is needed beyond this one HTTP request.
func (s *FaoximaProvisionerService) initializeTables(slug string) error {
	tableURL := fmt.Sprintf("https://%s/faoxima-resellers/%s/table.php", s.faoximaDomain, slug)
	client := &http.Client{Timeout: 30 * time.Second}
	resp, err := client.Get(tableURL)
	if err != nil {
		return err
	}
	defer resp.Body.Close()
	if resp.StatusCode >= 400 {
		return fmt.Errorf("table.php returned HTTP %d", resp.StatusCode)
	}
	return nil
}

// GetInstance returns resellerID's own Faoxima instance row, or
// gorm.ErrRecordNotFound if they haven't provisioned one.
func (s *FaoximaProvisionerService) GetInstance(resellerID uint) (*model.FaoximaInstance, error) {
	var instance model.FaoximaInstance
	if err := s.db.Where("reseller_id = ?", resellerID).First(&instance).Error; err != nil {
		return nil, err
	}
	return &instance, nil
}

// ListInstances returns every provisioned Faoxima instance -- the
// admin's own "مدیریت ربات فاکسیمای نمایندگان" list view, letting an
// admin see every reseller's instance status/billing period in one
// place rather than checking one reseller at a time.
func (s *FaoximaProvisionerService) ListInstances() ([]model.FaoximaInstance, error) {
	var instances []model.FaoximaInstance
	if err := s.db.Order("id asc").Find(&instances).Error; err != nil {
		return nil, err
	}
	return instances, nil
}

// FaoximaInstanceWithResellerName pairs one instance with its owning
// reseller's own Name -- the HTTP layer's admin list view needs both
// without making a separate reseller lookup per row.
type FaoximaInstanceWithResellerName struct {
	Instance     model.FaoximaInstance
	ResellerName string
}

// ListInstancesWithResellerNames is ListInstances plus a single batched
// join against model.Reseller for each instance's own Name -- kept in
// this service (rather than the HTTP layer reaching into model.Reseller
// directly) so the HTTP controller never needs its own *gorm.DB, matching
// this codebase's own "controllers only ever call services" layering.
func (s *FaoximaProvisionerService) ListInstancesWithResellerNames() ([]FaoximaInstanceWithResellerName, error) {
	instances, err := s.ListInstances()
	if err != nil {
		return nil, err
	}
	if len(instances) == 0 {
		return nil, nil
	}

	resellerIDs := make([]uint, len(instances))
	for i, inst := range instances {
		resellerIDs[i] = inst.ResellerID
	}
	var resellers []model.Reseller
	if err := s.db.Where("id IN ?", resellerIDs).Find(&resellers).Error; err != nil {
		return nil, err
	}
	names := make(map[uint]string, len(resellers))
	for _, r := range resellers {
		names[r.ID] = r.Name
	}

	result := make([]FaoximaInstanceWithResellerName, len(instances))
	for i, inst := range instances {
		result[i] = FaoximaInstanceWithResellerName{Instance: inst, ResellerName: names[inst.ResellerID]}
	}
	return result, nil
}

// UpdateToken changes resellerID's bot token in-place: rewrites
// config.php's $APIKEY and re-registers the webhook with the new token
// (the old token's webhook is implicitly abandoned -- Telegram tokens
// are one-to-one with bots, so there's nothing to explicitly unregister
// on the old token once it's no longer used here).
func (s *FaoximaProvisionerService) UpdateToken(resellerID uint, newBotToken string) error {
	instance, err := s.GetInstance(resellerID)
	if err != nil {
		return err
	}

	instanceDir := filepath.Join(faoximaInstancesDir, instance.InstanceSlug)
	oldToken := instance.BotToken
	instance.BotToken = newBotToken

	if err := s.writeConfig(instance, instanceDir, instance.DBName, instance.DBUser, instance.DBPassword); err != nil {
		return err
	}
	_ = exec.Command("chmod", "666", filepath.Join(instanceDir, "config.php")).Run()

	webhookURL := fmt.Sprintf("https://%s/faoxima-resellers/%s/index.php", s.faoximaDomain, instance.InstanceSlug)
	if err := s.setWebhook(newBotToken, webhookURL, instance.WebhookSecret); err != nil {
		// Roll back to the old token's config so this instance keeps
		// working with its previous, still-valid token rather than being
		// left half-migrated to a token whose webhook registration failed.
		instance.BotToken = oldToken
		_ = s.writeConfig(instance, instanceDir, instance.DBName, instance.DBUser, instance.DBPassword)
		return fmt.Errorf("ثبت وبهوک با توکن جدید ناموفق بود، توکن قبلی حفظ شد: %w", err)
	}

	return s.db.Model(&model.FaoximaInstance{}).Where("reseller_id = ?", resellerID).Updates(map[string]interface{}{
		"bot_token":     newBotToken,
		"status":        model.FaoximaInstanceStatusEnabled,
		"error_message": nil,
	}).Error
}

// Disable unregisters this instance's webhook (so its bot stops receiving
// any Telegram traffic) but leaves the database and files entirely
// intact -- reversible via Enable, unlike Remove.
func (s *FaoximaProvisionerService) Disable(resellerID uint) error {
	instance, err := s.GetInstance(resellerID)
	if err != nil {
		return err
	}
	if err := s.deleteWebhook(instance.BotToken); err != nil {
		s.logger.Warn("failed to delete telegram webhook on disable", zap.Uint("reseller_id", resellerID), zap.Error(err))
	}
	return s.db.Model(&model.FaoximaInstance{}).Where("reseller_id = ?", resellerID).Update("status", model.FaoximaInstanceStatusDisabled).Error
}

// Enable re-registers this instance's webhook using its already-stored
// token/secret -- the counterpart to Disable.
func (s *FaoximaProvisionerService) Enable(resellerID uint) error {
	instance, err := s.GetInstance(resellerID)
	if err != nil {
		return err
	}
	webhookURL := fmt.Sprintf("https://%s/faoxima-resellers/%s/index.php", s.faoximaDomain, instance.InstanceSlug)
	if err := s.setWebhook(instance.BotToken, webhookURL, instance.WebhookSecret); err != nil {
		return err
	}
	// Best-effort, same reasoning as provisionSteps' own call -- a failed
	// menu-button re-registration must not block re-enabling the bot
	// itself.
	miniAppURL := fmt.Sprintf("https://%s/faoxima-resellers/%s/app/index.php", s.faoximaDomain, instance.InstanceSlug)
	if err := s.setMenuButton(instance.BotToken, miniAppURL); err != nil {
		s.logger.Warn("failed to re-register mini-app menu button on enable", zap.Uint("reseller_id", resellerID), zap.Error(err))
	}
	return s.db.Model(&model.FaoximaInstance{}).Where("reseller_id = ?", resellerID).Updates(map[string]interface{}{
		"status":        model.FaoximaInstanceStatusEnabled,
		"error_message": nil,
	}).Error
}

// AdminEnableWithPeriod is the admin-facing counterpart to Enable, per
// the admin's own explicit request: running a reseller's Faoxima
// instance costs the panel real resources (bandwidth + memory for a
// full separate PHP deployment), so it should not run for free/
// indefinitely for every reseller by default ("درست نیست که رایگان برای
// همه نمایندگان باشه"). Re-enables the instance exactly like Enable, AND
// stamps BillingPeriodDays/PeriodActivatedAt so CheckExpiredPeriods
// (this service's own scheduled job) will automatically disable it once
// that many days have elapsed -- e.g. a reseller pays for a 30-day
// period, the admin enables with periodDays=30, and the bot keeps
// running for exactly that long before needing the admin's own explicit
// AdminResetPeriod to resume it. periodDays <= 0 means "no billing
// period" (the instance stays enabled indefinitely, exactly like a
// plain Enable) -- an admin who doesn't want to opt this particular
// reseller into the period model at all can still use this.
func (s *FaoximaProvisionerService) AdminEnableWithPeriod(resellerID uint, periodDays int) error {
	if err := s.Enable(resellerID); err != nil {
		return err
	}

	updates := map[string]interface{}{}
	if periodDays > 0 {
		updates["billing_period_days"] = periodDays
		updates["period_activated_at"] = timeNow()
	} else {
		updates["billing_period_days"] = nil
		updates["period_activated_at"] = nil
	}
	return s.db.Model(&model.FaoximaInstance{}).Where("reseller_id = ?", resellerID).Updates(updates).Error
}

// AdminResetPeriod is the admin's own explicit "دوره را ریست کن" action
// once a reseller pays for their next period: re-stamps
// PeriodActivatedAt to now (restarting the BillingPeriodDays countdown
// from zero) and re-enables the instance if it had been auto-disabled by
// CheckExpiredPeriods. Requires BillingPeriodDays to already be set (via
// a prior AdminEnableWithPeriod call) -- resetting a period that was
// never configured wouldn't mean anything.
func (s *FaoximaProvisionerService) AdminResetPeriod(resellerID uint) error {
	instance, err := s.GetInstance(resellerID)
	if err != nil {
		return err
	}
	if instance.BillingPeriodDays == nil {
		return fmt.Errorf("برای این نماینده دوره‌ی صورتحساب تنظیم نشده است")
	}

	if instance.Status != model.FaoximaInstanceStatusEnabled {
		if err := s.Enable(resellerID); err != nil {
			return err
		}
	}

	return s.db.Model(&model.FaoximaInstance{}).Where("reseller_id = ?", resellerID).Update("period_activated_at", timeNow()).Error
}

// CheckExpiredPeriods is the scheduled job entry point that enforces
// BillingPeriodDays: every ENABLED instance with a configured period
// whose PeriodActivatedAt + BillingPeriodDays has elapsed is disabled
// (webhook unregistered, exactly like a manual Disable) -- the admin
// must then explicitly call AdminResetPeriod once the reseller pays for
// their next period, rather than the bot silently auto-renewing forever
// with no billing check at all.
func (s *FaoximaProvisionerService) CheckExpiredPeriods() {
	var instances []model.FaoximaInstance
	if err := s.db.Where("status = ? AND billing_period_days IS NOT NULL AND period_activated_at IS NOT NULL",
		model.FaoximaInstanceStatusEnabled).Find(&instances).Error; err != nil {
		s.logger.Error("failed to list faoxima instances for period expiry check", zap.Error(err))
		return
	}

	now := *timeNow()
	for _, instance := range instances {
		if instance.BillingPeriodDays == nil || instance.PeriodActivatedAt == nil {
			continue
		}
		expiresAt := instance.PeriodActivatedAt.AddDate(0, 0, *instance.BillingPeriodDays)
		if now.Before(expiresAt) {
			continue
		}

		s.logger.Info("faoxima instance billing period expired, disabling",
			zap.Uint("reseller_id", instance.ResellerID), zap.Int("period_days", *instance.BillingPeriodDays))

		if err := s.Disable(instance.ResellerID); err != nil {
			s.logger.Error("failed to disable expired faoxima instance", zap.Uint("reseller_id", instance.ResellerID), zap.Error(err))
			continue
		}
		s.notifyPeriodExpired(instance.ResellerID)
	}
}

func (s *FaoximaProvisionerService) notifyPeriodExpired(resellerID uint) {
	if s.botNotifier == nil {
		return
	}
	settings, err := s.botNotifier.settingsService.GetOrCreate()
	if err != nil || settings.AdminChatID == "" {
		return
	}
	s.botNotifier.NotifyFaoximaPeriodExpired(settings, resellerID)
}

// Remove permanently deletes resellerID's Faoxima instance: webhook,
// database, MySQL user, and instance directory -- mirrors
// remove_additional_bot()'s own destructive sequence. Irreversible; the
// caller (HTTP layer) is responsible for confirming this with the admin
// before calling it, matching every other destructive action in this
// codebase's own convention.
//
// Confirmed, caught-in-testing bug this fixes: the row itself was
// originally soft-deleted (gorm.Delete's default behavior via
// model.Model's embedded DeletedAt), which left the now-defunct row
// still occupying ResellerID's uniqueIndex slot -- a reseller who
// removed their instance could never provision a new one afterward,
// failing with "UNIQUE constraint failed: faoxima_instances.reseller_id"
// on the very next attempt. Unscoped().Delete() hard-deletes instead,
// which is correct here specifically: unlike, say, a Peer or
// UserManagerAccount (whose soft-deleted history the panel's own
// reporting/audit features read), nothing in this codebase ever needs to
// read a removed Faoxima instance's row again -- Backup already exists
// as the explicit "preserve what matters before removing" action for
// this feature.
func (s *FaoximaProvisionerService) Remove(resellerID uint) error {
	instance, err := s.GetInstance(resellerID)
	if err != nil {
		return err
	}

	_ = s.deleteWebhook(instance.BotToken)

	if err := s.runMysqlAdmin(fmt.Sprintf("DROP DATABASE IF EXISTS `%s`;", instance.DBName)); err != nil {
		s.logger.Warn("failed to drop instance database", zap.String("db", instance.DBName), zap.Error(err))
	}
	if err := s.runMysqlAdmin(fmt.Sprintf("DROP USER IF EXISTS '%s'@'localhost';", instance.DBUser)); err != nil {
		s.logger.Warn("failed to drop instance db user", zap.String("user", instance.DBUser), zap.Error(err))
	}

	instanceDir := filepath.Join(faoximaInstancesDir, instance.InstanceSlug)
	if err := os.RemoveAll(instanceDir); err != nil {
		s.logger.Warn("failed to remove instance directory", zap.String("dir", instanceDir), zap.Error(err))
	}

	return s.db.Unscoped().Delete(&model.FaoximaInstance{}, "reseller_id = ?", resellerID).Error
}

// Backup runs mysqldump against this instance's own database via its own
// scoped MySQL user (never the shared admin account -- a compromised or
// malformed dump command can only ever touch this one instance's data),
// returning the raw SQL dump bytes for the caller to stream back as a
// file download. Mirrors BackupService's own file-based backup for the
// panel's SQLite database, but MySQL-specific since Faoxima always uses
// MySQL (confirmed: never SQLite anywhere in its own code).
func (s *FaoximaProvisionerService) Backup(resellerID uint) ([]byte, string, error) {
	instance, err := s.GetInstance(resellerID)
	if err != nil {
		return nil, "", err
	}

	// Password passed via the MYSQL_PWD environment variable, never as a
	// --password=... command-line argument -- a CLI argument is visible
	// to any other user on this shared server via `ps`/`/proc`, which
	// would defeat the whole point of a per-instance scoped DB user.
	cmd := exec.Command("mysqldump", "-u", instance.DBUser, instance.DBName)
	cmd.Env = append(os.Environ(), "MYSQL_PWD="+instance.DBPassword)
	output, err := cmd.Output()
	if err != nil {
		if exitErr, ok := err.(*exec.ExitError); ok {
			return nil, "", fmt.Errorf("mysqldump failed: %s", strings.TrimSpace(string(exitErr.Stderr)))
		}
		return nil, "", fmt.Errorf("mysqldump failed: %w", err)
	}

	filename := fmt.Sprintf("faoxima-%s-%s.sql", instance.InstanceSlug, time.Now().UTC().Format("20060102-150405"))
	return output, filename, nil
}

// Restore imports a previously-exported SQL dump back into this
// instance's own database, via that same scoped MySQL user -- the
// dedicated per-instance user's privileges are scoped to only its own
// database (see createDatabase's own doc comment), so even a malicious
// dump file can only affect this one instance, never another reseller's
// data or the main panel's own database. sqlContent is validated for a
// minimal sanity check (non-empty, no multi-database statements) before
// executing; deeper SQL validation is intentionally not attempted here
// since this action is admin/reseller-gated already, matching this
// codebase's existing trust boundary for irreversible actions.
func (s *FaoximaProvisionerService) Restore(resellerID uint, sqlContent []byte) error {
	if len(sqlContent) == 0 {
		return fmt.Errorf("فایل بکاپ خالی است")
	}
	lowered := strings.ToLower(string(sqlContent))
	for _, forbidden := range []string{"drop database", "create database", "grant ", "create user"} {
		if strings.Contains(lowered, forbidden) {
			return fmt.Errorf("فایل بکاپ شامل دستورات غیرمجاز است (%s)", forbidden)
		}
	}

	instance, err := s.GetInstance(resellerID)
	if err != nil {
		return err
	}

	// Same MYSQL_PWD-over-environment-variable rationale as Backup above.
	cmd := exec.Command("mysql", "-u", instance.DBUser, instance.DBName)
	cmd.Env = append(os.Environ(), "MYSQL_PWD="+instance.DBPassword)
	cmd.Stdin = strings.NewReader(string(sqlContent))
	output, err := cmd.CombinedOutput()
	if err != nil {
		return fmt.Errorf("بازیابی ناموفق بود: %s", strings.TrimSpace(string(output)))
	}
	return nil
}

// resellerIDToUint is a small parsing helper the HTTP controller uses --
// kept here rather than duplicated since every method above takes a
// uint resellerID already.
func resellerIDToUint(s string) (uint, error) {
	v, err := strconv.ParseUint(s, 10, 32)
	if err != nil {
		return 0, err
	}
	return uint(v), nil
}
