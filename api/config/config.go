package config

import (
	"io/fs"
	"log"
	"os"
	"path/filepath"
	"strconv"
	"strings"

	"github.com/joho/godotenv"
	"github.com/labstack/echo/v4"

	"github.com/maahdima/mwp/ui"
)

type AppConfig struct {
	Mode               string
	Host               string
	Port               string
	ConsoleLogFormat   string
	DataDirPath        string
	UIAssetsFs         fs.FS
	PeerFilesDir       string
	TrafficJobInterval string
}

type DBConfig struct {
	Host     string
	Port     string
	Username string
	Password string
	Database string
	Dialect  string
}

type AdminConfig struct {
	Username string
	// Password is empty when ADMIN_PASSWORD is not set in the environment
	// -- see GetAdminConfig's own doc comment on why this codebase no
	// longer falls back to a fixed, well-known default here. AdminSeed
	// treats an empty Password as "generate a random one," never as "use
	// mwpadmin."
	Password string
}

type AuthConfig struct {
	AccessTokenTTL  string
	RefreshTokenTTL string
}

type TelegramConfig struct {
	Enabled    bool
	BotToken   string
	ApiBaseURL string
}

// licenseServerBaseURLDefault is the factory-default central license/
// update server an install checks against if LICENSE_SERVER_URL is never
// set. LICENSE_SERVER_URL can override this on first activation only;
// see LicenseService for why the effective URL is locked to the database
// after that point. The license server's public API listens on port
// 3131; its admin API/UI and backup API are separate, authenticated
// ports and must not be used here.
const licenseServerBaseURLDefault = "http://license.horanet.ir:3131"

// LicenseConfig points MWPanel at the central license/update server
// (license-panel, running separately -- see D:\license-panel) and carries
// the Ed25519 public key MWPanel uses to verify every signed response it
// receives from that server, so a compromised/spoofed network path can
// never forge a "license valid" or "here's a safe update" response.
type LicenseConfig struct {
	// BaseURL is licenseServerBaseURLDefault, unless LICENSE_SERVER_URL is
	// set -- see licenseServerBaseURLDefault's doc comment for why this
	// value only actually takes effect on an install's first activation
	// (LicenseService locks in whatever was used at that point).
	BaseURL string
	// LicenseKey is what the admin enters once during setup (see
	// service.LicenseService.Activate). Stored in SystemConfig after
	// first successful activation, not required in the environment on
	// every subsequent boot.
	LicenseKey string
	// PublicKeyB64 is the Ed25519 public key license-panel signs
	// responses with (see cmd/keygen in the license-panel repo). Safe to
	// commit/embed since a public key can only verify, never forge,
	// signatures.
	PublicKeyB64 string
	// CheckIntervalSeconds controls how often MWPanel re-validates its
	// license with a heartbeat call after the initial activation.
	CheckIntervalSeconds int
}

func GetLicenseConfig() LicenseConfig {
	interval := 3600
	if v := getEnv("LICENSE_CHECK_INTERVAL_SECONDS", ""); v != "" {
		if parsed, err := strconv.Atoi(v); err == nil && parsed > 0 {
			interval = parsed
		}
	}

	baseURL := getEnv("LICENSE_SERVER_URL", licenseServerBaseURLDefault)

	return LicenseConfig{
		BaseURL:              baseURL,
		LicenseKey:           getEnv("LICENSE_KEY", ""),
		PublicKeyB64:         getEnv("LICENSE_PUBLIC_KEY", ""),
		CheckIntervalSeconds: interval,
	}
}

// releasePublicKeyDefault is baked directly into the binary rather than
// requiring a RELEASE_PUBLIC_KEY env var on every install -- unlike
// LicenseConfig.PublicKeyB64 (which authenticates a SERVER this codebase
// doesn't control and that could plausibly be self-hosted/rotated by an
// operator), the release signing keypair belongs solely to whoever builds
// and ships mwp releases, so its public half is exactly as safe to
// hardcode as any other compiled-in constant -- and hardcoding it means a
// tampered binary can't simply unset an env var to disable the check
// (RELEASE_PUBLIC_KEY below still allows an override, e.g. for a fork
// signing its own releases with a different key, but the default requires
// no configuration at all). Empty until a real release keypair is
// generated via `go run ./cmd/signbinary -generate` and this constant is
// updated -- see checkBinaryIntegrity's own doc comment for why an empty
// key here simply skips the check entirely rather than failing closed.
const releasePublicKeyDefault = ""

// GetReleasePublicKey returns the Ed25519 public key mwp uses to verify its
// OWN executable's signature at startup (license/binaryintegrity.Verify) --
// a completely separate keypair from LicenseConfig.PublicKeyB64 (that one
// verifies license-panel's server responses; this one verifies the binary
// itself was not substituted/modified after its release build was signed).
func GetReleasePublicKey() string {
	return getEnv("RELEASE_PUBLIC_KEY", releasePublicKeyDefault)
}

func init() {
	_ = loadEnv()
}

func GetAppConfig() AppConfig {
	dataDir := getEnv("DATA_DIR", "")
	if dataDir == "" {
		// os.UserConfigDir() resolves via $XDG_CONFIG_HOME, falling back to
		// $HOME/.config on Linux -- a minimal systemd service (User=/Group=
		// set, no login shell/PAM session) commonly runs with HOME unset,
		// which previously made this log.Fatalf and crash the whole
		// process on every single start. DATA_DIR should always be set
		// explicitly in production (see .env.example and the systemd unit),
		// but when it isn't, fall back to a "data" directory relative to
		// the process's own working directory instead of crashing --
		// systemd's WorkingDirectory= is always defined and writable by
		// the service user, unlike HOME.
		userConfigDir, err := os.UserConfigDir()
		if err != nil {
			log.Printf("Warning: could not resolve user config directory (%v); falling back to ./data relative to the working directory. Set DATA_DIR explicitly to silence this.", err)
			userConfigDir = "data"
		}

		dataDir = filepath.Join(userConfigDir, "mwp")
	}

	if err := os.MkdirAll(dataDir, 0755); err != nil {
		log.Fatalf("Failed to create data directory: %v", err)
	}

	return AppConfig{
		Mode:               getEnv("MODE", "production"),
		Host:               getEnv("SERVER_HOST", "0.0.0.0"),
		Port:               getEnv("SERVER_PORT", "3000"),
		ConsoleLogFormat:   getEnv("CONSOLE_LOG_FORMAT", "plain"),
		UIAssetsFs:         echo.MustSubFS(ui.GetUIAssets(), "dist"),
		PeerFilesDir:       getEnv("PEER_FILES_DIR", filepath.Join(dataDir, "peer-files")),
		DataDirPath:        dataDir,
		TrafficJobInterval: getEnv("TRAFFIC_JOB_INTERVAL", "120"),
	}
}

func GetDBConfig() DBConfig {
	dialect := getEnv("DB_DIALECT", "sqlite")

	defaultDatabaseName := "mwp_db"
	if dialect == "sqlite" {
		appCfg := GetAppConfig()
		defaultDatabaseName = filepath.Join(appCfg.DataDirPath, "mwp.db")
	}

	return DBConfig{
		Host:     getEnv("DB_HOST", "127.0.0.1"),
		Port:     getEnv("DB_PORT", "5432"),
		Username: getEnv("DB_USERNAME", "root"),
		Password: getEnv("DB_PASSWORD", "1234"),
		Database: getEnv("DB_NAME", defaultDatabaseName),
		Dialect:  dialect,
	}
}

// GetAdminConfig reads the bootstrap admin account's username/password
// from ADMIN_USERNAME/ADMIN_PASSWORD. Password is left empty when
// ADMIN_PASSWORD isn't set -- see AdminSeed's own doc comment for what it
// does with that instead of falling back to a fixed default.
func GetAdminConfig() AdminConfig {
	return AdminConfig{
		Username: getEnv("ADMIN_USERNAME", "ApexPanel"),
		Password: getEnv("ADMIN_PASSWORD", ""),
	}
}

func GetAuthConfig() AuthConfig {
	return AuthConfig{
		AccessTokenTTL:  getEnv("AUTH_ACCESS_TOKEN_TTL", "900"),
		RefreshTokenTTL: getEnv("AUTH_REFRESH_TOKEN_TTL", "86400"),
	}
}

func GetTelegramConfig() TelegramConfig {
	return TelegramConfig{
		Enabled:    getEnvBool("TELEGRAM_BOT_ENABLED", false),
		BotToken:   getEnv("TELEGRAM_BOT_TOKEN", ""),
		ApiBaseURL: getEnv("TELEGRAM_BOT_API_BASE_URL", "https://api.telegram.org"),
	}
}

// GetFaoximaDefaultTelegramProxy returns the SOCKS5 proxy URL (format
// "socks5h://user:pass@host:port") seeded as the default Telegram proxy
// for every newly-provisioned Faoxima bot-as-a-service instance, read
// from FAOXIMA_DEFAULT_TELEGRAM_PROXY. Empty by default: on a server
// where Telegram's API is directly reachable, no proxy is needed, and
// every instance simply uses a direct connection until its own reseller
// changes it. If your deployment's network blocks direct access to
// api.telegram.org, set this to a SOCKS5 proxy you control -- a domain
// name (rather than a bare IP) registered through a provider like
// Cloudflare is recommended, since it lets you repoint the proxy to a
// new IP later without reconfiguring every already-provisioned instance.
func GetFaoximaDefaultTelegramProxy() string {
	return getEnv("FAOXIMA_DEFAULT_TELEGRAM_PROXY", "")
}

func getEnv(key, defaultValue string) string {
	value := os.Getenv(key)
	if value == "" {
		return defaultValue
	}
	return value
}

func getEnvBool(key string, defaultValue bool) bool {
	value := strings.ToLower(strings.TrimSpace(os.Getenv(key)))
	if value == "" {
		return defaultValue
	}
	return value == "1" || value == "true" || value == "yes" || value == "y"
}

func loadEnv() error {
	return godotenv.Load(getEnv("ENV_FILE", "config/.env"))
}
