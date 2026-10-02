package httpserver

import (
	"errors"
	"fmt"
	stdhttp "net/http"
	"time"

	"golang.org/x/crypto/acme/autocert"

	"github.com/maahdima/mwp/api/adaptor/mikrotik"
	traffic "github.com/maahdima/mwp/api/cmd/jobs"
	"github.com/maahdima/mwp/api/common"
	"github.com/maahdima/mwp/api/config"
	"github.com/maahdima/mwp/api/http"
	"github.com/maahdima/mwp/api/service"
	"github.com/maahdima/mwp/api/utils"
	"github.com/maahdima/mwp/api/utils/validate"

	"github.com/go-playground/validator/v10"
	"github.com/labstack/echo/v4"
	"github.com/labstack/echo/v4/middleware"
	"go.uber.org/zap"
	"gorm.io/gorm"
)

func StartHttpServer(
	db *gorm.DB,
	mwpClients *common.MwpClients,
	mikrotikAdaptor *mikrotik.Adaptor,
	trafficCalculator *traffic.Calculator,
	botService *service.BotService,
	botNotifier *service.BotNotifier,
	botSettingsService *service.BotSettingsService,
	trafficPackageService *service.TrafficPackageService,
	packagePurchaseService *service.PackagePurchaseService,
	userManagerTrafficPackageService *service.UserManagerTrafficPackageService,
	userManagerPackagePurchaseService *service.UserManagerPackagePurchaseService,
	v2rayTrafficPackageService *service.V2RayTrafficPackageService,
	v2rayPackagePurchaseService *service.V2RayPackagePurchaseService,
	botScheduler *service.BotScheduler,
	accountingService *service.AccountingService,
	tunnelHealthService *service.TunnelHealthService,
	tunnelPolicyService *service.TunnelPolicyService,
	tunnelRedlineValidator *service.TunnelRedlineValidator,
	tunnelGraphService *service.TunnelGraphService,
	userManagerProtocolHealthService *service.UserManagerProtocolHealthService,
	faoximaProvisionerService *service.FaoximaProvisionerService,
) error {
	appCfg := config.GetAppConfig()
	dbCfg := config.GetDBConfig()

	authenticationService := service.NewAuthentication(db)
	authenticationService.SetBotNotifier(botNotifier)
	schedulerService := service.NewScheduler(mikrotikAdaptor)
	queueService := service.NewQueue(mikrotikAdaptor)
	configGenerator := service.NewConfigGenerator(db, mikrotikAdaptor)
	qrCodeGenerator := service.NewQRCodeGenerator(db, mikrotikAdaptor)
	excelGenerator := service.NewExcelGenerator(db)
	serverService := service.NewServerService(db, mwpClients, mikrotikAdaptor)
	interfaceService := service.NewWgInterface(db, mikrotikAdaptor)
	ipPoolService := service.NewIPPool(db)
	auditLogService := service.NewAuditLog(db)
	peerService := service.NewWGPeer(db, mikrotikAdaptor, schedulerService, queueService, configGenerator, qrCodeGenerator, auditLogService)
	peerService.SetBotNotifier(botNotifier)
	deviceDataService := service.NewDeviceData(db, mikrotikAdaptor, serverService, interfaceService, peerService)
	syncService := service.NewSyncService(db, mikrotikAdaptor, configGenerator, qrCodeGenerator)
	resellerService := service.NewReseller(db, mikrotikAdaptor)
	resellerService.SetBotNotifier(botNotifier)
	walletService := service.NewWallet(db)
	// Lets UpdateReseller bootstrap a wallet row immediately when an admin
	// switches a reseller from Volume-based to Payment-based billing (see
	// UpdateReseller's own billingModeSwitched doc comment) -- otherwise
	// safe to leave unset, matching SetBotNotifier/SetV2RayResumer's own
	// "optional collaborator" convention.
	resellerService.SetWallet(walletService)
	resellerBillingService := service.NewResellerBillingService(db, walletService)
	resellerBillingService.SetResumer(resellerService)
	faoximaProvisionerService.SetBotNotifier(botNotifier)
	pricePlanService := service.NewPricePlan(db)
	invoiceService := service.NewInvoice(db)
	backupService := service.NewBackupService(db, dbCfg.Dialect, dbCfg.Database, appCfg.DataDirPath)
	systemConfigService := service.NewSystemConfigService(db)
	systemHealthService := service.NewSystemHealthService(db, appCfg.DataDirPath, botSettingsService)
	sslSettingsService := service.NewSSLSettingsService(db)
	userManagerConfigFileService := service.NewUserManagerConfigFile(db)
	userManagerProtocolFileService := service.NewUserManagerProtocolFile(db)
	userManagerService := service.NewUserManagerService(db, mikrotikAdaptor, auditLogService)
	userManagerService.SetBotNotifier(botNotifier)
	userManagerService.SetConfigFileService(userManagerConfigFileService)
	userManagerPackagePurchaseService.SetBotNotifier(botNotifier)
	logReaderService := service.NewLogReader()
	supportTokenService := service.NewSupportTokenService(db)
	apiKeyService := service.NewApiKeyService(db)

	xuiPanelService := service.NewXuiPanelService(db)
	v2rayPackageService := service.NewV2RayPackageService(db, xuiPanelService, auditLogService)
	v2rayPackageService.SetBotNotifier(botNotifier)
	v2rayPackagePurchaseService.SetBotNotifier(botNotifier)

	// This entrypoint's own V2RaySyncService instance (see geoIPService's
	// own doc comment below for why each entrypoint holds its own rather
	// than sharing cmd/main.go's) -- needed here for
	// WalletController.Credit's ResumeBillingSuspension call, which
	// re-enables billing-suspended V2Ray packages on a top-up, AND for
	// resellerService's own reseller-quota resume (below) -- the actual
	// usage-polling SyncPackageUsage job itself still only ever runs on
	// cmd/main.go's scheduler, never here.
	v2raySyncServiceForWallet := service.NewV2RaySyncService(db, xuiPanelService)
	resellerService.SetV2RayResumer(v2raySyncServiceForWallet)

	reportsService := service.NewReportsService(db)
	reportsLiveService := service.NewReportsLiveService(db, mikrotikAdaptor)

	// applicationService orchestrates peerService/userManagerService/
	// v2rayPackageService directly (see ApplicationService's own doc
	// comment) -- constructed after all three since it depends on them.
	// appAccessSecret is persisted via SystemConfigService (see that
	// method's own doc comment for why), exactly like
	// Authentication.AccessSecret (see authenticationService above) --
	// a SEPARATE persisted secret, so an app-user token can never be
	// replayed against an admin/reseller-only route or vice versa, even
	// though both happen to use the same JWT library/shape.
	applicationOpenVpnTemplateService := service.NewApplicationOpenVpnTemplateService(db)
	applicationService := service.NewApplicationService(db, peerService, userManagerService, v2rayPackageService, applicationOpenVpnTemplateService, auditLogService)
	applicationPlanService := service.NewApplicationPlanService(db)
	applicationService.SetPlanService(applicationPlanService)
	applicationVersionService := service.NewApplicationVersionService(db)
	// Persisted, not regenerated per process start -- see
	// SystemConfigService.GetOrCreateJwtSecret's own doc comment for the
	// confirmed, reported bug this fixes (every restart used to silently
	// log out every mobile-app session).
	appAccessSecret, err := systemConfigService.GetOrCreateJwtSecret("app_access")
	if err != nil {
		zap.L().Error("failed to load/persist app access secret, falling back to a per-process random one", zap.Error(err))
		appAccessSecret = utils.RandomString(24)
	}

	// geoIPService is also constructed in cmd/main.go for the background
	// collector jobs (see that file's own comment on why v2raySyncService/
	// xuiPanelServiceForSync follow the same "constructed there for the
	// scheduler, here for HTTP" split) -- NewGeoIPService itself is cheap
	// (just opens whatever .mmdb files already exist on disk), so building
	// two independent instances here and in main.go is simpler than wiring
	// a shared one across both entrypoints, at the cost of each holding its
	// own file handle to the same underlying files.
	geoIPService := service.NewGeoIPService(db, appCfg.DataDirPath, systemConfigService)
	securityService := service.NewSecurityService(db)
	securityRetentionService := service.NewSecurityRetentionService(db)

	dnsPanelService := service.NewDNSPanelService(db)
	dnsPlanService := service.NewDNSPlanService(db)
	dnsAccountService := service.NewDNSAccountService(db, dnsPanelService, geoIPService, auditLogService)
	dnsAccountService.SetBotNotifier(botNotifier)
	dnsAccountService.SetPlanService(dnsPlanService)

	// This entrypoint's own DNSSyncService instance -- same "each
	// entrypoint holds its own" reasoning as v2raySyncServiceForWallet
	// above, needed here purely for resellerService's own DNS reseller-
	// quota resume call (the actual usage-polling SyncAccounts job itself
	// still only ever runs on cmd/main.go's scheduler, never here).
	dnsSyncServiceForReseller := service.NewDNSSyncService(db, dnsPanelService)
	resellerService.SetDNSResumer(dnsSyncServiceForReseller)
	resellerService.SetApplicationResumer(applicationService)

	licenseCfg := config.GetLicenseConfig()
	licenseService := service.NewLicenseService(db, licenseCfg, appCfg.Mode, appCfg.DataDirPath)
	licenseService.SetBotNotifier(botNotifier)
	resellerService.SetLicenseLimiter(licenseService)
	peerService.SetLicenseLimiter(licenseService)
	userManagerService.SetLicenseLimiter(licenseService)
	v2rayPackageService.SetLicenseLimiter(licenseService)
	dnsAccountService.SetLicenseLimiter(licenseService)
	applicationService.SetLicenseLimiter(licenseService)
	licenseService.SetFreeTierDownsizers(map[string]service.FreeTierDownsizer{
		"max_resellers":             resellerService,
		"max_peers":                 peerService,
		"max_user_manager_accounts": userManagerService,
		"max_v2ray_packages":        v2rayPackageService,
		"max_dns_accounts":          dnsAccountService,
		"max_applications":          applicationService,
	})

	// EnsureActivated does a real network round-trip to the license server
	// (activation or heartbeat) and used to run synchronously HERE, before
	// echo.New()/e.Start() below -- meaning the entire HTTP listener
	// (every route, not just license-gated ones) would not even bind until
	// that call returned. A live profiling run measured this at ~10s when
	// the configured license server is slow or unreachable, which reads
	// exactly like "the panel barely comes up" on every process restart.
	// This is safe to run in the background instead: LicenseService.Status()
	// starts at its zero value (Activated: false), which the request-gating
	// middleware already treats as "not yet licensed" and correctly blocks
	// on -- identical fail-closed behavior to what a caller would see
	// during the few hundred ms to ~10s this now takes in the background,
	// just without holding up the HTTP listener itself (static assets,
	// the public license-activation UI, and any genuinely public route
	// no longer wait on a license-server round-trip to become reachable).
	go func() {
		if err := licenseService.EnsureActivated(); err != nil {
			zap.L().Error("license activation check failed", zap.Error(err))
		}
		licenseService.StartHeartbeatLoop()
	}()
	helpCenterService := service.NewHelpCenterService(licenseService)

	e := echo.New()

	// Go's http.Server has NO timeouts by default -- a single slow,
	// hanging, or malicious connection (e.g. Slowloris, or just the
	// constant background noise of internet port scanners hitting an
	// exposed port directly) can occupy a goroutine/file-descriptor
	// indefinitely. Enough of those accumulating exhausts the process
	// (out of file descriptors, or effectively out of memory from
	// abandoned connections/buffers) and Echo's Start()/StartAutoTLS()
	// call e.Logger.Fatal on the resulting error, which calls os.Exit(1)
	// -- i.e. the process deliberately kills itself. This is the leading
	// suspect for "the panel dies when exposed directly on the open
	// internet, but never through a private tunnel that scanners can't
	// reach": a tunnel URL is unlisted, so it never receives that
	// background scanner traffic in the first place.
	serverTimeouts := func(s *stdhttp.Server) {
		s.ReadHeaderTimeout = 10 * time.Second
		s.ReadTimeout = 60 * time.Second
		// No WriteTimeout: this panel serves backup downloads up to 2GB
		// (see http/backup.go's maxBackupUploadSize) and streams peer
		// config/QR responses -- a fixed write deadline could abort a
		// legitimate large or slow download partway through. The
		// ReadHeaderTimeout/ReadTimeout/IdleTimeout below are what
		// actually guard against a hung/slow-loris connection tying up
		// a goroutine indefinitely; write-side abuse is comparatively
		// low-risk since the server controls how much it sends.
		s.IdleTimeout = 90 * time.Second
		s.MaxHeaderBytes = 1 << 20 // 1 MiB; default (net/http's DefaultMaxHeaderBytes) is also 1MiB, set explicitly for clarity
	}
	serverTimeouts(e.Server)
	serverTimeouts(e.TLSServer)

	// A custom LogErrorFunc routes recovered HTTP-handler panics through the
	// same zap logger everything else uses (see utils/log), which -- unlike
	// Echo's own default logger -- also writes to a persistent file, not
	// just stdout. Without this, a handler panic that crashed a single
	// request (recovered here, so the process itself survives) would still
	// be invisible after the fact if nobody was watching the terminal live
	// at that exact moment.
	e.Use(middleware.RecoverWithConfig(middleware.RecoverConfig{
		LogErrorFunc: func(c echo.Context, err error, stack []byte) error {
			zap.L().Error("recovered from panic in HTTP handler",
				zap.String("method", c.Request().Method),
				zap.String("path", c.Request().URL.Path),
				zap.Error(err),
				zap.ByteString("stacktrace", stack))
			return err
		},
	}))
	// Every request logged at Info through zap (persistent file, see
	// utils/log), not Echo's own default logger (stdout only, gone once
	// the terminal/process is). This is what makes it possible to tell
	// "the process died right after handling a flood of requests from
	// unfamiliar IPs" (points at scanner/bot load) apart from "the process
	// died with no unusual traffic beforehand" (points elsewhere) when
	// reading the log after the fact -- RemoteIP is included specifically
	// for that.
	e.Use(middleware.RequestLoggerWithConfig(middleware.RequestLoggerConfig{
		LogStatus:    true,
		LogURI:       true,
		LogMethod:    true,
		LogRemoteIP:  true,
		LogUserAgent: true,
		LogLatency:   true,
		LogError:     true,
		// HandleError is deliberately NOT set: Echo's own top-level
		// ServeHTTP already calls e.HTTPErrorHandler for any error the
		// middleware chain returns, regardless of this setting. Setting it
		// here would invoke the error handler a second time for the same
		// request (it's meant for callers who install a *replacement*
		// HTTPErrorHandler on Echo itself and want this middleware to be
		// the one that triggers it, which this codebase doesn't do).
		LogValuesFunc: func(c echo.Context, v middleware.RequestLoggerValues) error {
			if v.Error == nil {
				zap.L().Info("request",
					zap.String("remote_ip", v.RemoteIP),
					zap.String("method", v.Method),
					zap.String("uri", v.URI),
					zap.Int("status", v.Status),
					zap.Duration("latency", v.Latency),
					zap.String("user_agent", v.UserAgent),
				)
			} else {
				zap.L().Warn("request error",
					zap.String("remote_ip", v.RemoteIP),
					zap.String("method", v.Method),
					zap.String("uri", v.URI),
					zap.Int("status", v.Status),
					zap.Duration("latency", v.Latency),
					zap.String("user_agent", v.UserAgent),
					zap.Error(v.Error),
				)
			}
			return nil
		},
	}))
	// ExposeHeaders is required for any endpoint that streams a file
	// download (ctx.Attachment) with a real filename in Content-Disposition
	// -- browsers hide response headers from JS on cross-origin requests
	// unless the server explicitly allow-lists them here. Without this, the
	// frontend's blob-download code (protocol certificate/client-app files,
	// backups, etc.) can never recover the server-computed filename/
	// extension and has to guess or hardcode one instead.
	e.Use(middleware.CORSWithConfig(middleware.CORSConfig{
		AllowOrigins:  []string{"*"},
		ExposeHeaders: []string{"Content-Disposition"},
	}))
	e.Validator = &validate.CustomValidator{Validator: validator.New()}

	http.SetupMwpUI(e, appCfg.UIAssetsFs)
	http.SetupMwpAPI(
		e,
		mwpClients,
		authenticationService,
		serverService,
		interfaceService,
		ipPoolService,
		peerService,
		configGenerator,
		qrCodeGenerator,
		excelGenerator,
		deviceDataService,
		trafficCalculator,
		syncService,
		resellerService,
		walletService,
		pricePlanService,
		invoiceService,
		auditLogService,
		trafficPackageService,
		packagePurchaseService,
		userManagerTrafficPackageService,
		userManagerPackagePurchaseService,
		backupService,
		botSettingsService,
		botNotifier,
		botService,
		systemConfigService,
		systemHealthService,
		userManagerService,
		userManagerConfigFileService,
		userManagerProtocolFileService,
		logReaderService,
		licenseService,
		supportTokenService,
		helpCenterService,
		xuiPanelService,
		v2rayPackageService,
		v2rayTrafficPackageService,
		v2rayPackagePurchaseService,
		reportsService,
		reportsLiveService,
		botScheduler,
		sslSettingsService,
		securityService,
		geoIPService,
		securityRetentionService,
		applicationService,
		applicationVersionService,
		appAccessSecret,
		accountingService,
		tunnelHealthService,
		tunnelPolicyService,
		tunnelRedlineValidator,
		tunnelGraphService,
		userManagerProtocolHealthService,
		v2raySyncServiceForWallet,
		resellerBillingService,
		faoximaProvisionerService,
		dnsPanelService,
		dnsAccountService,
		apiKeyService,
		dnsPlanService,
		applicationPlanService,
	)

	// e.Start()/StartAutoTLS() only ever return once the listener itself
	// fails (e.g. the OS refuses a new "accept" under fd exhaustion) --
	// this is intentionally still a fatal, process-exiting condition
	// (there is no good way to keep serving without a working listener),
	// but it's logged through zap (persistent file, not just stdout) first
	// so the failure is diagnosable afterward, and left to a process
	// supervisor (see deploy/mwp.service's Restart=always) to bring the
	// panel back up rather than Echo's own logger + os.Exit swallowing the
	// reason.
	// Manual SSL (Settings > SSL -- a bring-your-own-certificate path, e.g.
	// files already issued by the admin's own Certbot run) takes priority
	// over the port-based AutoTLS heuristic below when the admin has
	// explicitly enabled it: it's an explicit, deliberate choice, whereas
	// the AutoTLS branch is just an implicit "you happened to configure
	// port 443/8443" inference. tlsConfig is nil (falls through to the
	// existing AutoTLS/plain-HTTP branch unchanged) whenever manual SSL is
	// simply not enabled -- see SSLSettingsService.LoadTLSConfig's own doc
	// comment for why this re-reads the files from disk on every startup
	// rather than trusting a cached result.
	manualTLSConfig, manualTLSErr := sslSettingsService.LoadTLSConfig()
	if manualTLSErr != nil {
		zap.L().Error("manual SSL is enabled but its certificate/key could not be loaded -- falling back to the port-based listener below", zap.Error(manualTLSErr))
	}

	var listenErr error
	if manualTLSConfig != nil {
		// e.StartServer builds its listener from s.Addr, not from an address
		// parameter -- unlike Start()/StartAutoTLS(), which take one
		// directly -- so it must be set explicitly here (StartTLS sets this
		// same field internally via its own configureTLS call).
		e.TLSServer.Addr = fmt.Sprintf("%s:%s", appCfg.Host, appCfg.Port)
		e.TLSServer.TLSConfig = manualTLSConfig
		listenErr = e.StartServer(e.TLSServer)
	} else if appCfg.Port == "443" || appCfg.Port == "8443" {
		e.AutoTLSManager.Cache = autocert.DirCache(appCfg.DataDirPath)
		listenErr = e.StartAutoTLS(fmt.Sprintf("%s:%s", appCfg.Host, appCfg.Port))
	} else {
		listenErr = e.Start(fmt.Sprintf("%s:%s", appCfg.Host, appCfg.Port))
	}

	if listenErr != nil && !errors.Is(listenErr, stdhttp.ErrServerClosed) {
		zap.L().Fatal("HTTP server stopped unexpectedly", zap.Error(listenErr))
	}

	return nil
}
