package main

import (
	"fmt"
	"os"
	"strconv"
	"time"

	"github.com/go-co-op/gocron/v2"

	"github.com/maahdima/mwp/api/adaptor/mikrotik"
	"github.com/maahdima/mwp/api/cmd/cli"
	"github.com/maahdima/mwp/api/cmd/http-server"
	"github.com/maahdima/mwp/api/cmd/jobs"
	"github.com/maahdima/mwp/api/common"
	"github.com/maahdima/mwp/api/config"
	"github.com/maahdima/mwp/api/dataservice"
	"github.com/maahdima/mwp/api/dataservice/seeds"
	"github.com/maahdima/mwp/api/service"
	"github.com/maahdima/mwp/api/utils/log"

	"go.uber.org/zap"
)

func init() {
	log.InitLogger(config.GetAppConfig())
}

func main() {
	// `mwp menu` opens the interactive SSH-invoked management menu instead
	// of starting the HTTP server -- checked before any of the heartbeat/
	// signal-logging/scheduler wiring below, since the menu is a short-
	// lived, one-shot interactive session, not a long-running server
	// process. Running with no arguments (the systemd unit's ExecStart)
	// is completely unaffected -- it falls through to the normal startup
	// sequence exactly as before.
	if len(os.Args) > 1 && os.Args[1] == "menu" {
		cli.Run()
		return
	}

	logger := zap.L()

	// Last line of defense for a panic in main()'s own goroutine (setup
	// code, or anything invoked synchronously from it) that isn't already
	// caught by a more specific recover -- e.g. middleware.Recover() for
	// HTTP handlers, gocron's per-job recovery for scheduled tasks, or
	// botPoller's own recover for the Telegram polling loop. This CANNOT
	// catch a panic in an already-spawned separate goroutine (Go doesn't
	// allow recovering across goroutine boundaries), and it cannot do
	// anything about the process being killed by the OS (e.g. an OOM
	// kill) -- there is no userspace code path for either of those. What
	// it does buy: if the crash reported as "the panel just turns off"
	// turns out to be a Go panic reachable from here, this guarantees it
	// gets logged (see utils/log for the persistent file sink) instead of
	// vanishing with the process, which is the difference between an
	// invisible crash and a diagnosable one.
	defer func() {
		if r := recover(); r != nil {
			logger.Error("recovered from panic in main(); the process would otherwise have exited silently",
				zap.Any("panic", r), zap.Stack("stacktrace"))
		}
	}()

	// Diagnostic instrumentation for "the panel silently died" -- see
	// utils/log/heartbeat.go and utils/log/signals.go for exactly what each
	// one is for and how to read them after the fact. Started as early as
	// possible so even a crash during the startup sequence below is
	// bracketed by at least one heartbeat and signal-logging is already
	// armed.
	log.StartResourceHeartbeat(5 * time.Minute)
	log.LogTerminationSignals()
	logger.Info("mwp starting up", zap.Int("pid", os.Getpid()))

	appCfg := config.GetAppConfig()
	dbConnConfig := config.GetDBConfig()

	// Must run before ConnectDB opens any connection to the database file --
	// see ApplyPendingRestore's doc comment for why applying a staged
	// restore after connections already exist would risk corruption. This
	// is what actually makes the web panel's "Restore Backup" upload take
	// effect; the upload endpoint itself only stages the file.
	if err := dataservice.ApplyPendingRestore(dbConnConfig); err != nil {
		logger.Panic("Failed to apply staged database restore", zap.Error(err))
	}

	db, err := dataservice.ConnectDB(dbConnConfig)
	if err != nil {
		logger.Panic("Failed to connect to database", zap.Error(err))
	}

	if err := dataservice.AutoMigrate(db); err != nil {
		logger.Panic("Failed to auto-migrate database", zap.Error(err))
	}

	// TODO: add db migration (gorm)

	// If the admin has set a port override from the web panel's settings
	// page, apply it now, before StartHttpServer's own config.GetAppConfig()
	// call reads SERVER_PORT -- this is the only point in the process
	// lifecycle where it's safe to do so (DB is ready, but the HTTP listener
	// hasn't bound yet). A change made while the panel is already running
	// only takes effect on the NEXT process start, since Echo binds its
	// listener once at startup and can't be rebound live.
	systemConfigService := service.NewSystemConfigService(db)
	if portOverride, err := systemConfigService.GetPortOverride(); err != nil {
		logger.Warn("failed to read port override, using SERVER_PORT/default", zap.Error(err))
	} else if portOverride != "" {
		if err := os.Setenv("SERVER_PORT", portOverride); err != nil {
			logger.Warn("failed to apply port override", zap.Error(err))
		} else {
			logger.Info("applying admin-configured port override", zap.String("port", portOverride))
		}
	}

	err = seeds.AdminSeed(db)
	if err != nil {
		fmt.Printf("cannot seed admin [%s]", err.Error())
		logger.Panic("cannot seed admin", zap.Error(err))
	}

	// Initialize http client for Mikrotik API
	mwpClients := common.NewMwpClients(db)
	mwpClients.InitClient()

	mikrotikAdaptor := mikrotik.NewAdaptor(mwpClients)
	telegramNotifier := service.NewTelegramNotifier(config.GetTelegramConfig())
	trafficCalculator := traffic.NewTrafficCalculator(db, mikrotikAdaptor, telegramNotifier)

	// Telegram bot: constructed here (not inside StartHttpServer) since the
	// traffic job below and the HTTP layer both need the same running
	// instances -- the bot's quota-warning notifications are wired into the
	// traffic calculator, while its settings/command endpoints live in HTTP.
	botSettingsService := service.NewBotSettingsService(db)
	resellerServiceForBot := service.NewReseller(db, mikrotikAdaptor)
	walletServiceForBot := service.NewWallet(db)
	// Payment-based billing engine (item 5) -- shared between the
	// WireGuard/User Manager traffic calculator and the V2Ray sync job
	// below, both of which charge usage as it accrues on this same
	// scheduler tick.
	resellerBillingService := service.NewResellerBillingService(db, walletServiceForBot)
	resellerBillingService.SetResumer(resellerServiceForBot)
	trafficCalculator.SetResellerBiller(resellerBillingService)
	auditLogServiceForBot := service.NewAuditLog(db)
	botService := service.NewBotService(db)
	botNotifier := service.NewBotNotifier(botService, db, botSettingsService)
	botCommandHandler := service.NewBotCommandHandler(db, resellerServiceForBot, walletServiceForBot, auditLogServiceForBot, botSettingsService)

	// Traffic packages: constructed here (not inside StartHttpServer, unlike
	// most other services) specifically so the SAME PackagePurchaseService
	// instance backs both the bot's /packages command and the HTTP purchase
	// endpoint -- it carries a botNotifier reference set exactly once below,
	// so a purchase made from either surface always sends the same
	// purchase-receipt notification instead of risking one path silently
	// missing it if a second, separately-wired instance existed.
	trafficPackageService := service.NewTrafficPackageService(db)
	packagePurchaseService := service.NewPackagePurchaseService(db, walletServiceForBot, trafficPackageService)
	packagePurchaseService.SetBotNotifier(botNotifier)
	botCommandHandler.SetPackageServices(trafficPackageService, packagePurchaseService)

	// User Manager traffic packages: same "constructed here, shared instance"
	// rationale as the WireGuard packages above, for the separate UM quota
	// pool's purchase flow.
	userManagerTrafficPackageService := service.NewUserManagerTrafficPackageService(db)
	userManagerPackagePurchaseService := service.NewUserManagerPackagePurchaseService(db, walletServiceForBot, userManagerTrafficPackageService)
	userManagerPackagePurchaseService.SetBotNotifier(botNotifier)

	// V2Ray traffic packages: same "constructed here, shared instance"
	// rationale as the WireGuard/User Manager packages above. xuiPanelService
	// and v2raySyncService are constructed here (rather than inside
	// StartHttpServer, unlike most other V2Ray services) specifically so the
	// sync job below can be registered on the scheduler alongside every
	// other traffic job -- mirrors how trafficCalculator itself is
	// constructed above for the same reason.
	v2rayTrafficPackageService := service.NewV2RayTrafficPackageService(db)
	v2rayPackagePurchaseService := service.NewV2RayPackagePurchaseService(db, walletServiceForBot, v2rayTrafficPackageService)
	v2rayPackagePurchaseService.SetBotNotifier(botNotifier)
	xuiPanelServiceForSync := service.NewXuiPanelService(db)
	// usageSnapshotWriter feeds the Reports section's historical usage
	// table (model.UsageSnapshot) -- shared across the WireGuard/User
	// Manager traffic job and the V2Ray sync job, mirroring botNotifier's
	// own "one shared instance wired into every job that needs it" pattern.
	usageSnapshotWriter := service.NewUsageSnapshotWriter(db)
	v2raySyncService := service.NewV2RaySyncService(db, xuiPanelServiceForSync)
	v2raySyncService.SetUsageRecorder(usageSnapshotWriter)
	v2raySyncService.SetResellerBiller(resellerBillingService)
	// Confirmed, reported incident this fixes: a reseller topping up
	// V2Ray quota through THIS purchase flow (rather than the admin's
	// "Edit Reseller" form, which already wired the equivalent
	// resellerService.SetV2RayResumer above) never resumed packages
	// applyResellerV2RayQuota had suspended for exceeding the reseller's
	// overall quota -- see v2RayPackagePurchaseService.SetV2RayResumer's
	// own doc comment for the full incident.
	v2rayPackagePurchaseService.SetV2RayResumer(v2raySyncService)
	resourceSampler := service.NewResourceSampler(db, mikrotikAdaptor)

	// DNS panel/account sync: same "constructed here, shared instance"
	// rationale as xuiPanelServiceForSync/v2raySyncService above, so the
	// sync job below can be registered on the scheduler alongside every
	// other traffic job.
	dnsPanelServiceForSync := service.NewDNSPanelService(db)
	dnsSyncService := service.NewDNSSyncService(db, dnsPanelServiceForSync)

	// Security page background collectors -- geoIPService here is this
	// entrypoint's own instance (see StartHttpServer's identical
	// construction for why each entrypoint holds its own), used by both
	// the IP-connection collector (WireGuard/User Manager) and the
	// ether-torch collector (see common.IPv4DefaultInterface for why
	// "ether1" is this codebase's existing default-interface convention,
	// reused here rather than inventing a new one).
	securityGeoIPService := service.NewGeoIPService(db, appCfg.DataDirPath, systemConfigService)
	securityIPCollector := service.NewSecurityIPCollectorService(db, mikrotikAdaptor, securityGeoIPService)
	securityEtherTorchService := service.NewSecurityEtherTorchService(db, mikrotikAdaptor, securityGeoIPService, common.IPv4DefaultInterface)

	// Applications quota-enforcement job -- constructed here (this
	// entrypoint's own WgPeer/UserManagerService/V2RayPackageService
	// instances, separate from StartHttpServer's own, mirroring
	// xuiPanelServiceForSync's identical "each entrypoint holds its own"
	// precedent above) so EnforceApplicationQuotas can be registered on
	// this same scheduler alongside every other traffic job. This job
	// calls into the full service package directly (UpdatePeer/
	// UpdateAccount/UpdatePackage), which cmd/jobs itself is not allowed
	// to import (see traffic.Calculator's own doc comment on why external
	// capabilities are injected via narrow interfaces instead) -- running
	// it from cmd/main.go, which already imports service freely, avoids
	// that import-cycle constraint entirely.
	applicationSchedulerConfigGenerator := service.NewConfigGenerator(db, mikrotikAdaptor)
	applicationSchedulerQrCodeGenerator := service.NewQRCodeGenerator(db, mikrotikAdaptor)
	applicationSchedulerAuditLog := service.NewAuditLog(db)
	applicationSchedulerPeerService := service.NewWGPeer(db, mikrotikAdaptor, service.NewScheduler(mikrotikAdaptor), service.NewQueue(mikrotikAdaptor), applicationSchedulerConfigGenerator, applicationSchedulerQrCodeGenerator, applicationSchedulerAuditLog)
	applicationSchedulerUserManagerService := service.NewUserManagerService(db, mikrotikAdaptor, applicationSchedulerAuditLog)
	applicationSchedulerXuiPanelService := service.NewXuiPanelService(db)
	applicationSchedulerV2RayPackageService := service.NewV2RayPackageService(db, applicationSchedulerXuiPanelService, applicationSchedulerAuditLog)
	applicationSchedulerOpenVpnTemplateService := service.NewApplicationOpenVpnTemplateService(db)
	applicationService := service.NewApplicationService(db, applicationSchedulerPeerService, applicationSchedulerUserManagerService, applicationSchedulerV2RayPackageService, applicationSchedulerOpenVpnTemplateService, applicationSchedulerAuditLog)
	applicationService.SetBilling(resellerBillingService)

	accountingService := service.NewAccountingService(db)
	accountingService.SetBotNotifier(botNotifier)

	// "tunnel-ai": full firewall/NAT/mangle/routing-table discovery
	// (TunnelGraphService) feeding infrastructure-tunnel detection +
	// Telegram alerting + the dry-run-capable self-healing decision
	// engine (TunnelHealthService) -- see each service's own doc
	// comment. TunnelHealthService is constructed AFTER
	// TunnelGraphService since it reads the graph's own discovered
	// tunnel list every poll tick, never scanning model.Interface
	// (customer WireGuard peers) directly.
	tunnelGraphService := service.NewTunnelGraphService(db, mikrotikAdaptor)
	tunnelHealthService := service.NewTunnelHealthService(db, mikrotikAdaptor, tunnelGraphService)
	tunnelHealthService.SetBotNotifier(botNotifier)
	tunnelPolicyService := service.NewTunnelPolicyService(db)
	tunnelRedlineValidator := service.NewTunnelRedlineValidator(db)

	// User Manager protocol health (spec section ب-7): a completely
	// separate, alert-only check for L2TP/PPTP/SSTP/OpenVPN -- never
	// feeds into TunnelHealthService's own Level 1/2 remediation engine
	// (restarting one of these could disrupt active customer sessions).
	userManagerProtocolHealthService := service.NewUserManagerProtocolHealthService(db, mikrotikAdaptor)
	userManagerProtocolHealthService.SetBotNotifier(botNotifier)

	// V2Ray/x-ui panel health (spec ب-7): alert-only reachability check,
	// independent of the billing-critical v2raySyncService job -- see
	// V2RayPanelHealthService's own doc comment for why no automatic
	// restart action exists yet (no remote access to a third-party x-ui
	// server modeled in this codebase).
	v2rayPanelHealthService := service.NewV2RayPanelHealthService(db)
	v2rayPanelHealthService.SetBotNotifier(botNotifier)
	// item 9: lets an unreachable panel's health check additionally query
	// its mapped RouterOS container's live status (XuiPanel.
	// ContainerServerID/ContainerName), distinguishing "the container
	// stopped" from a generic connectivity failure -- nil-safe/no-op for
	// any panel without a container mapping configured.
	v2rayPanelHealthService.SetMikrotikAdaptor(mikrotikAdaptor)

	// "bot-as-a-service" (Faoxima instances): constructed here, not inside
	// httpserver.StartHttpServer (where every OTHER service that only the
	// HTTP layer needs lives), specifically so its own scheduled
	// self-heal job (VerifyWebhooks, below) can be wired up alongside
	// every other gocron job in this file -- see that job's own
	// registration for why this exists (the admin's explicit disaster-
	// recovery concern about a server migration silently breaking every
	// reseller's Faoxima bot webhook).
	faoximaProvisionerService := service.NewFaoximaProvisionerService(db)
	faoximaProvisionerService.SetBotNotifier(botNotifier)
	if faoximaSettings, err := botSettingsService.GetOrCreate(); err != nil {
		logger.Warn("failed to load bot settings for faoxima http client, using a direct (unproxied) connection", zap.Error(err))
	} else {
		faoximaProvisionerService.ReloadHTTPClient(faoximaSettings)
		faoximaProvisionerService.ReloadFaoximaDomain(faoximaSettings)
	}

	botService.SetHandler(botCommandHandler)
	if err := botService.Reload(); err != nil {
		logger.Warn("telegram bot did not start (this is expected if it hasn't been configured yet)", zap.Error(err))
	}

	trafficCalculator.SetQuotaNotifier(botNotifier)
	trafficCalculator.SetCriticalAlertNotifier(botNotifier)
	trafficCalculator.SetUsageRecorder(usageSnapshotWriter)

	// Fires the admin's /setbackup and /setreport schedules (see
	// BotScheduler.Tick) -- both are otherwise inert database columns with
	// nothing to act on them.
	backupServiceForBot := service.NewBackupService(db, dbConnConfig.Dialect, dbConnConfig.Database, appCfg.DataDirPath)
	systemConfigServiceForBot := service.NewSystemConfigService(db)
	botScheduler := service.NewBotScheduler(db, botSettingsService, backupServiceForBot, botService, botNotifier, walletServiceForBot, resellerServiceForBot, systemConfigServiceForBot)

	// Start the traffic calculation job
	scheduler, err := gocron.NewScheduler()
	if err != nil {
		logger.Panic("Failed to create scheduler", zap.Error(err))
	}

	trafficJobInterval, _ := strconv.Atoi(appCfg.TrafficJobInterval)

	_, err = scheduler.NewJob(
		gocron.DurationJob(
			time.Duration(trafficJobInterval)*time.Second),
		gocron.NewTask(trafficCalculator.CalculatePeerTraffic))
	if err != nil {
		logger.Panic("Failed to create peer traffic calculation job", zap.Error(err))
	}

	// User Manager (L2TP/PPTP/SSTP/OpenVPN) usage polling -- fully separate
	// from the WireGuard peer job above, same interval/scheduler only for
	// operational simplicity.
	_, err = scheduler.NewJob(
		gocron.DurationJob(
			time.Duration(trafficJobInterval)*time.Second),
		gocron.NewTask(trafficCalculator.CalculateUserManagerUsage))
	if err != nil {
		logger.Panic("Failed to create user manager usage calculation job", zap.Error(err))
	}

	// V2Ray usage/subscription sync -- fully separate from the WireGuard and
	// User Manager jobs above, same interval only for operational
	// simplicity (matches CalculateUserManagerUsage's own precedent of
	// reusing TRAFFIC_JOB_INTERVAL rather than adding a new env var).
	_, err = scheduler.NewJob(
		gocron.DurationJob(
			time.Duration(trafficJobInterval)*time.Second),
		gocron.NewTask(v2raySyncService.SyncPackageUsage))
	if err != nil {
		logger.Panic("Failed to create v2ray usage sync job", zap.Error(err))
	}

	// DNS account usage/status sync -- same interval as the V2Ray job above
	// for operational simplicity, same "one dead panel never blocks the
	// rest of the tick" resilience (see DNSSyncService.SyncAccounts' own
	// doc comment).
	_, err = scheduler.NewJob(
		gocron.DurationJob(
			time.Duration(trafficJobInterval)*time.Second),
		gocron.NewTask(dnsSyncService.SyncAccounts))
	if err != nil {
		logger.Panic("Failed to create DNS account sync job", zap.Error(err))
	}

	_, err = scheduler.NewJob(
		gocron.DailyJob(
			1, gocron.NewAtTimes(
				gocron.NewAtTime(00, 00, 00))),
		gocron.NewTask(trafficCalculator.CalculateDailyTraffic))
	if err != nil {
		logger.Panic("Failed to create daily traffic calculation job", zap.Error(err))
	}

	// Reports section's Mikrotik CPU/memory history -- same interval as the
	// traffic jobs, for the same "operational simplicity" reasoning already
	// established by CalculateUserManagerUsage/SyncPackageUsage above.
	_, err = scheduler.NewJob(
		gocron.DurationJob(
			time.Duration(trafficJobInterval)*time.Second),
		gocron.NewTask(resourceSampler.Sample))
	if err != nil {
		logger.Panic("Failed to create resource sampler job", zap.Error(err))
	}

	_, err = scheduler.NewJob(
		gocron.DurationJob(time.Minute),
		gocron.NewTask(botScheduler.Tick))
	if err != nil {
		logger.Panic("Failed to create telegram bot scheduler job", zap.Error(err))
	}

	// Recurring server-renewal cost reminders -- once a day is enough since
	// AccountingCost.RecurrenceIntervalDays is always measured in whole
	// days, matching CalculateDailyTraffic's own once-a-day cadence above.
	_, err = scheduler.NewJob(
		gocron.DailyJob(
			1, gocron.NewAtTimes(
				gocron.NewAtTime(9, 0, 0))),
		gocron.NewTask(accountingService.Tick))
	if err != nil {
		logger.Panic("Failed to create accounting recurring-cost reminder job", zap.Error(err))
	}

	// Security page: WireGuard/User Manager IP-connection tracking -- same
	// interval as the traffic jobs above, for the same operational-
	// simplicity reasoning (this is a router-wide read, not tied to any
	// specific peer/account's own traffic delta timing).
	_, err = scheduler.NewJob(
		gocron.DurationJob(
			time.Duration(trafficJobInterval)*time.Second),
		gocron.NewTask(securityIPCollector.Collect))
	if err != nil {
		logger.Panic("Failed to create security IP collector job", zap.Error(err))
	}

	// Security page: ether interface torch polling -- a FIXED 10s interval
	// per the admin's own explicit choice, deliberately independent of
	// TRAFFIC_JOB_INTERVAL (a short-duration snapshot call, see
	// common.ToolTorchPath's own doc comment for why this must stay on a
	// short, predictable cadence rather than following whatever the admin
	// sets the general traffic interval to).
	_, err = scheduler.NewJob(
		gocron.DurationJob(10*time.Second),
		gocron.NewTask(securityEtherTorchService.Poll))
	if err != nil {
		logger.Panic("Failed to create security ether torch job", zap.Error(err))
	}

	// Applications: sums each Application's combined WireGuard+User
	// Manager+V2Ray usage and suspends/resumes the whole bundle -- same
	// interval as the traffic jobs above, for the same operational-
	// simplicity reasoning (this reads each resource's own already-
	// up-to-date running-total counters, which the jobs above just
	// finished updating on this same tick).
	_, err = scheduler.NewJob(
		gocron.DurationJob(
			time.Duration(trafficJobInterval)*time.Second),
		gocron.NewTask(applicationService.EnforceApplicationQuotas))
	if err != nil {
		logger.Panic("Failed to create application quota enforcement job", zap.Error(err))
	}

	// Tunnel dependency-graph discovery (tunnel-ai section ب-1/ب-2/ب-3): a
	// much longer, fixed 5-minute cadence -- deliberately independent of
	// both TRAFFIC_JOB_INTERVAL and the tunnel health poller's own 60s
	// cadence below. Each tick makes ~14 sequential REST calls (NAT,
	// mangle, filter, routes, bridges, tunnel interfaces, etc), and the
	// production router this initially shipped against was ALREADY
	// observed timing out under load from the existing traffic/security
	// jobs -- this graph-discovery pass is diagnostic/inspection tooling,
	// not correctness-critical, so it deliberately favors a light footprint
	// over a fast refresh rate.
	_, err = scheduler.NewJob(
		gocron.DurationJob(5*time.Minute),
		gocron.NewTask(tunnelGraphService.Poll))
	if err != nil {
		logger.Panic("Failed to create tunnel graph discovery job", zap.Error(err))
	}

	// Tunnel health (tunnel-ai phase 1): a fixed 60s cadence, deliberately
	// independent of TRAFFIC_JOB_INTERVAL -- the Tx/Rx asymmetry
	// detector's window (txRxAsymmetryWindow in service/tunnel_health.go)
	// assumes ~1 sample/minute, so this must stay on a short, predictable
	// cadence rather than following whatever the admin sets the general
	// traffic interval to (same reasoning as securityEtherTorchService's
	// own fixed 10s interval above).
	_, err = scheduler.NewJob(
		gocron.DurationJob(60*time.Second),
		gocron.NewTask(tunnelHealthService.Poll))
	if err != nil {
		logger.Panic("Failed to create tunnel health poll job", zap.Error(err))
	}

	// User Manager protocol health (spec ب-7): a lightweight TCP-connect
	// probe per enabled protocol -- same 60s cadence as the tunnel health
	// poller above, since both are cheap, fixed-cost checks independent
	// of TRAFFIC_JOB_INTERVAL.
	_, err = scheduler.NewJob(
		gocron.DurationJob(60*time.Second),
		gocron.NewTask(userManagerProtocolHealthService.Poll))
	if err != nil {
		logger.Panic("Failed to create user manager protocol health poll job", zap.Error(err))
	}

	// Tunnel health: nightly active backup-path probe -- deliberately its
	// OWN once-a-day schedule, separate from Poll's 60s cycle above,
	// since pinging every configured Level 2 backup gateway every minute
	// would be wasted load for a check whose whole value (confidence a
	// rarely-used standby path still works) does not need minute-by-
	// minute freshness. Run at 03:00 -- a low-traffic hour, same
	// reasoning as every other "quiet hour" scheduled task in this file.
	_, err = scheduler.NewJob(
		gocron.DailyJob(
			1, gocron.NewAtTimes(
				gocron.NewAtTime(3, 0, 0))),
		gocron.NewTask(tunnelHealthService.ProbeBackupPaths))
	if err != nil {
		logger.Panic("Failed to create tunnel backup path probe job", zap.Error(err))
	}

	// Tunnel health: weekly incident-pattern digest -- an analytical
	// report over the PAST week's TunnelHealthEvent history (chronically-
	// flapping tunnels, tunnels that keep failing together), sent only
	// when there is something worth reporting (see
	// BuildIncidentPatternReport's own doc comment). Monday mornings, so
	// the admin's first message of the work week is this summary.
	_, err = scheduler.NewJob(
		gocron.WeeklyJob(
			1, gocron.NewWeekdays(time.Monday),
			gocron.NewAtTimes(gocron.NewAtTime(9, 0, 0))),
		gocron.NewTask(tunnelHealthService.SendIncidentPatternReport))
	if err != nil {
		logger.Panic("Failed to create tunnel incident pattern report job", zap.Error(err))
	}

	// V2Ray/x-ui panel health -- a fixed 2-minute cadence, deliberately
	// separate from and slower than the tunnel/User-Manager pollers above
	// since each tick makes a real login call to every registered x-ui
	// panel (see V2RayPanelHealthService.Poll's own doc comment on why
	// it never reuses v2raySyncService's own tick).
	_, err = scheduler.NewJob(
		gocron.DurationJob(2*time.Minute),
		gocron.NewTask(v2rayPanelHealthService.Poll))
	if err != nil {
		logger.Panic("Failed to create v2ray panel health poll job", zap.Error(err))
	}

	// Faoxima "bot-as-a-service" webhook self-heal -- the admin's own
	// explicit disaster-recovery concern: "می‌ترسم اگه سرور عوض بشه
	// ربات‌های فاکسیما هیچ‌کدام کار نکنند یا ست وبهوک انجام نده." Each
	// instance's webhook is registered once, at provision/enable time,
	// directly against Telegram -- nothing previously re-checked that
	// registration against a server migration (or any other cause of
	// drift, e.g. Telegram clearing a webhook after delivery failures)
	// ever again. A 10-minute cadence -- frequent enough that a broken
	// migration is caught and reported/self-healed quickly, cheap enough
	// (one getWebhookInfo call per ENABLED instance, only re-registering
	// on an actual mismatch) to run indefinitely in the background.
	_, err = scheduler.NewJob(
		gocron.DurationJob(10*time.Minute),
		gocron.NewTask(faoximaProvisionerService.VerifyWebhooks))
	if err != nil {
		logger.Panic("Failed to create faoxima webhook verification job", zap.Error(err))
	}

	// Faoxima billing-period enforcement -- the admin's own explicit
	// request that running a reseller's Faoxima instance not be free/
	// indefinite for everyone by default (see
	// FaoximaProvisionerService.CheckExpiredPeriods's own doc comment).
	// A 1-hour cadence is frequent enough that a period rarely runs more
	// than an hour past its actual expiry, cheap enough (a handful of DB
	// rows checked, only touching Telegram for the rare instance that
	// actually just expired) to run indefinitely in the background.
	_, err = scheduler.NewJob(
		gocron.DurationJob(1*time.Hour),
		gocron.NewTask(faoximaProvisionerService.CheckExpiredPeriods))
	if err != nil {
		logger.Panic("Failed to create faoxima billing period check job", zap.Error(err))
	}

	// Systemd journal size cap -- the admin's own explicit request after a
	// confirmed, reported incident where raw log output grew to ~14GB
	// before being manually cleared (see LogRetentionService's own doc
	// comment). Once a day is enough: this is a slow-growing ceiling, not
	// a fast leak, and journalctl's own vacuum is itself a real (if
	// brief) disk-scanning operation not worth running more often than
	// that.
	logRetentionService := service.NewLogRetentionService()
	_, err = scheduler.NewJob(
		gocron.DailyJob(
			1, gocron.NewAtTimes(
				gocron.NewAtTime(3, 30, 00))),
		gocron.NewTask(logRetentionService.VacuumJournal))
	if err != nil {
		logger.Panic("Failed to create log retention job", zap.Error(err))
	}

	// Automatic nightly cleanup for IPConnectionLog/EtherTrafficSample --
	// per this project's own planning doc (فاز پنجم-۳ / فاز سوم-ج): these
	// two tables previously only shrank via an admin-triggered manual
	// button on the Security page, with no default ceiling, so a busy
	// panel's ether_traffic_samples (written every 10 seconds per
	// interface) could grow unbounded between admin visits. See
	// SecurityRetentionService.RunScheduledCleanup's own doc comment for
	// why the inactive-users criteria is deliberately NOT included here.
	securityRetentionService := service.NewSecurityRetentionService(db)
	_, err = scheduler.NewJob(
		gocron.DailyJob(
			1, gocron.NewAtTimes(
				gocron.NewAtTime(3, 45, 00))),
		gocron.NewTask(securityRetentionService.RunScheduledCleanup))
	if err != nil {
		logger.Panic("Failed to create security retention job", zap.Error(err))
	}

	// Automatic nightly cleanup for UsageSnapshot/ResourceSample -- a
	// confirmed, reported gap found during a production incident (disk
	// filled to 100%, crashing MySQL): unlike IPConnectionLog/
	// EtherTrafficSample above, these two Reports-section tables had NO
	// pruning at all, manual or scheduled. UsageSnapshot alone had grown to
	// 4.85 million rows. See ReportsRetentionService's own doc comment.
	reportsRetentionService := service.NewReportsRetentionService(db)
	_, err = scheduler.NewJob(
		gocron.DailyJob(
			1, gocron.NewAtTimes(
				gocron.NewAtTime(4, 00, 00))),
		gocron.NewTask(reportsRetentionService.RunScheduledCleanup))
	if err != nil {
		logger.Panic("Failed to create reports retention job", zap.Error(err))
	}

	scheduler.Start()

	// Start the HTTP server
	if err := httpserver.StartHttpServer(db, mwpClients, mikrotikAdaptor, trafficCalculator, botService, botNotifier, botSettingsService, trafficPackageService, packagePurchaseService, userManagerTrafficPackageService, userManagerPackagePurchaseService, v2rayTrafficPackageService, v2rayPackagePurchaseService, botScheduler, accountingService, tunnelHealthService, tunnelPolicyService, tunnelRedlineValidator, tunnelGraphService, userManagerProtocolHealthService, faoximaProvisionerService); err != nil {
		logger.Panic("Failed to start HTTP server", zap.Error(err))
	}
}
