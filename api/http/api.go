package http

import (
	"net/http"

	traffic "github.com/maahdima/mwp/api/cmd/jobs"
	"github.com/maahdima/mwp/api/common"
	"github.com/maahdima/mwp/api/http/middleware"
	"github.com/maahdima/mwp/api/service"

	echojwt "github.com/labstack/echo-jwt/v4"
	"github.com/labstack/echo/v4"
)

func SetupMwpAPI(
	app *echo.Echo,
	mwpClients *common.MwpClients,
	authenticationService *service.Authentication,
	serverService *service.Server,
	interfaceService *service.WgInterface,
	ipPoolService *service.IPPool,
	peerService *service.WgPeer,
	configGeneratorService *service.ConfigGenerator,
	qrCodeGeneratorService *service.QRCodeGenerator,
	excelGeneratorService *service.ExcelGenerator,
	deviceDataService *service.DeviceData,
	trafficCalculator *traffic.Calculator,
	syncService *service.SyncService,
	resellerService *service.Reseller,
	walletService *service.Wallet,
	pricePlanService *service.PricePlan,
	invoiceService *service.Invoice,
	auditLogService *service.AuditLog,
	trafficPackageService *service.TrafficPackageService,
	packagePurchaseService *service.PackagePurchaseService,
	userManagerTrafficPackageService *service.UserManagerTrafficPackageService,
	userManagerPackagePurchaseService *service.UserManagerPackagePurchaseService,
	backupService *service.BackupService,
	botSettingsService *service.BotSettingsService,
	botNotifier *service.BotNotifier,
	botService *service.BotService,
	systemConfigService *service.SystemConfigService,
	systemHealthService *service.SystemHealthService,
	userManagerService *service.UserManagerService,
	userManagerConfigFileService *service.UserManagerConfigFile,
	userManagerProtocolFileService *service.UserManagerProtocolFile,
	logReaderService *service.LogReader,
	licenseService *service.LicenseService,
	supportTokenService *service.SupportTokenService,
	helpCenterService *service.HelpCenterService,
	xuiPanelService *service.XuiPanelService,
	v2rayPackageService *service.V2RayPackageService,
	v2rayTrafficPackageService *service.V2RayTrafficPackageService,
	v2rayPackagePurchaseService *service.V2RayPackagePurchaseService,
	reportsService *service.ReportsService,
	reportsLiveService *service.ReportsLiveService,
	botScheduler *service.BotScheduler,
	sslSettingsService *service.SSLSettingsService,
	securityService *service.SecurityService,
	geoIPService *service.GeoIPService,
	securityRetentionService *service.SecurityRetentionService,
	applicationService *service.ApplicationService,
	applicationVersionService *service.ApplicationVersionService,
	appAccessSecret string,
	accountingService *service.AccountingService,
	tunnelHealthService *service.TunnelHealthService,
	tunnelPolicyService *service.TunnelPolicyService,
	tunnelRedlineValidator *service.TunnelRedlineValidator,
	tunnelGraphService *service.TunnelGraphService,
	userManagerProtocolHealthService *service.UserManagerProtocolHealthService,
	v2raySyncServiceForWallet *service.V2RaySyncService,
	resellerBillingService *service.ResellerBillingService,
	faoximaProvisionerService *service.FaoximaProvisionerService,
	dnsPanelService *service.DNSPanelService,
	dnsAccountService *service.DNSAccountService,
	apiKeyService *service.ApiKeyService,
	dnsPlanService *service.DNSPlanService,
	applicationPlanService *service.ApplicationPlanService,
) {
	router := app.Group("/api")
	// Registered before every other route in this group: a revoked/
	// expired license (confirmed by a real signed response, or an
	// elapsed grace period after the license server became unreachable
	// -- see LicenseService.IsBlocking) blocks the entire API, including
	// login, so a customer can't keep using an unlicensed install just
	// because they were already logged in.
	router.Use(middleware.LicenseMiddleware(licenseService))

	jwtConfig := echojwt.Config{
		SigningKey: authenticationService.AccessSecret,
		ErrorHandler: func(c echo.Context, err error) error {
			return echo.NewHTTPError(http.StatusUnauthorized, "Unauthorized")
		},
	}

	authController := NewAuthController(authenticationService, auditLogService, botNotifier)
	serverController := NewServerController(serverService, mwpClients)
	wgInterfaceController := NewWgInterfaceController(interfaceService, resellerService)
	ipPoolController := NewIPPoolController(ipPoolService)
	wgPeerController := NewWgPeerController(
		peerService,
		configGeneratorService,
		qrCodeGeneratorService,
		excelGeneratorService,
		trafficCalculator,
	)
	deviceInfoController := NewDeviceDataController(deviceDataService, trafficCalculator)
	syncController := NewSyncController(syncService)
	userController := NewUserController(peerService, configGeneratorService, qrCodeGeneratorService, userManagerService, userManagerConfigFileService, userManagerProtocolFileService)
	resellerController := NewResellerController(resellerService, resellerBillingService)
	faoximaController := NewFaoximaController(faoximaProvisionerService)
	walletController := NewWalletController(walletService, resellerService, v2raySyncServiceForWallet)
	pricePlanController := NewPricePlanController(pricePlanService)
	invoiceController := NewInvoiceController(invoiceService)
	auditLogController := NewAuditLogController(auditLogService)
	trafficPackageController := NewTrafficPackageController(trafficPackageService, packagePurchaseService)
	userManagerTrafficPackageController := NewUserManagerTrafficPackageController(userManagerTrafficPackageService, userManagerPackagePurchaseService)
	backupController := NewBackupController(backupService, botScheduler)
	botSettingsController := NewBotSettingsController(botSettingsService, botService, faoximaProvisionerService)
	sslSettingsController := NewSSLSettingsController(sslSettingsService)
	securityController := NewSecurityController(securityService, geoIPService, securityRetentionService, systemConfigService)
	systemConfigController := NewSystemConfigController(systemConfigService)
	systemHealthController := NewSystemHealthController(systemHealthService)
	userManagerAccountController := NewUserManagerAccountController(userManagerService, userManagerConfigFileService)
	userManagerGroupProfileController := NewUserManagerGroupProfileController(userManagerService)
	userManagerProtocolConfigController := NewUserManagerProtocolConfigController(userManagerService, userManagerProtocolFileService)
	logController := NewLogController(logReaderService)
	licenseController := NewLicenseController(licenseService, authenticationService)
	supportTokenController := NewSupportTokenController(supportTokenService, logReaderService)
	helpCenterController := NewHelpCenterController(helpCenterService)
	xuiPanelController := NewXuiPanelController(xuiPanelService)
	v2rayPackageController := NewV2RayPackageController(v2rayPackageService)
	v2rayTrafficPackageController := NewV2RayTrafficPackageController(v2rayTrafficPackageService, v2rayPackagePurchaseService)
	v2rayPublicController := NewV2RayPublicController(v2rayPackageService)
	reportsController := NewReportsController(reportsService, reportsLiveService)
	applicationController := NewApplicationController(applicationService)
	applicationVersionController := NewApplicationVersionController(applicationVersionService)
	appAuthController := NewAppAuthController(applicationService, applicationVersionService, appAccessSecret)
	accountingController := NewAccountingController(accountingService)
	tunnelHealthController := NewTunnelHealthController(tunnelHealthService, tunnelPolicyService, tunnelRedlineValidator, tunnelGraphService, userManagerProtocolHealthService)
	dnsPanelController := NewDNSPanelController(dnsPanelService)
	dnsPlanController := NewDNSPlanController(dnsPlanService)
	applicationPlanController := NewApplicationPlanController(applicationPlanService)
	dnsAccountController := NewDNSAccountController(dnsAccountService)
	dnsPublicController := NewDNSPublicController(dnsAccountService)
	apiKeyController := NewApiKeyController(apiKeyService)
	externalController := NewExternalController()

	setupAuthenticationRoutes(router, jwtConfig, authController)
	setupServerRoutes(router, jwtConfig, serverController)
	setupInterfaceRoutes(router, mwpClients, jwtConfig, wgInterfaceController)
	setupIPPoolRoutes(router, jwtConfig, ipPoolController)
	setupPeerRoutes(router, mwpClients, jwtConfig, wgPeerController)
	setupDeviceInfoRoutes(router, mwpClients, jwtConfig, deviceInfoController)
	setupSyncRoutes(router, mwpClients, jwtConfig, syncController)
	setupUserRoutes(router, userController)
	setupResellerRoutes(router, jwtConfig, resellerController)
	setupFaoximaRoutes(router, jwtConfig, faoximaController)
	setupWalletRoutes(router, jwtConfig, walletController)
	setupPricePlanRoutes(router, jwtConfig, pricePlanController)
	setupInvoiceRoutes(router, jwtConfig, invoiceController)
	setupAuditLogRoutes(router, jwtConfig, auditLogController)
	setupTrafficPackageRoutes(router, jwtConfig, trafficPackageController)
	setupUserManagerTrafficPackageRoutes(router, jwtConfig, userManagerTrafficPackageController)
	setupBackupRoutes(router, jwtConfig, backupController)
	setupBotSettingsRoutes(router, jwtConfig, botSettingsController)
	setupSSLSettingsRoutes(router, jwtConfig, sslSettingsController)
	setupSecurityRoutes(router, jwtConfig, securityController)
	setupSystemConfigRoutes(router, jwtConfig, systemConfigController, systemHealthController)
	setupUserManagerRoutes(router, mwpClients, jwtConfig, userManagerAccountController)
	setupUserManagerGroupProfileRoutes(router, mwpClients, jwtConfig, userManagerGroupProfileController)
	setupUserManagerProtocolConfigRoutes(router, jwtConfig, userManagerProtocolConfigController)
	setupLogRoutes(router, jwtConfig, logController)
	setupLicenseRoutes(router, jwtConfig, licenseController)
	setupSupportTokenRoutes(router, jwtConfig, supportTokenController)
	setupHelpCenterRoutes(router, jwtConfig, helpCenterController)
	setupXuiPanelRoutes(router, jwtConfig, xuiPanelController)
	setupV2RayPackageRoutes(router, jwtConfig, v2rayPackageController)
	setupV2RayTrafficPackageRoutes(router, jwtConfig, v2rayTrafficPackageController)
	setupV2RayPublicRoutes(router, v2rayPublicController)
	setupReportsRoutes(router, jwtConfig, reportsController)
	setupApplicationRoutes(router, jwtConfig, applicationController)
	setupApplicationPlanRoutes(router, jwtConfig, applicationPlanController)
	setupApplicationVersionRoutes(router, jwtConfig, applicationVersionController)
	setupAppUserRoutes(router, appAuthController)
	setupAccountingRoutes(router, jwtConfig, accountingController)
	setupTunnelHealthRoutes(router, jwtConfig, tunnelHealthController)
	setupDNSPanelRoutes(router, jwtConfig, dnsPanelController)
	setupDNSPlanRoutes(router, jwtConfig, dnsPlanController)
	setupDNSAccountRoutes(router, jwtConfig, dnsAccountController)
	setupDNSPublicRoutes(router, dnsPublicController)
	setupApiKeyRoutes(router, jwtConfig, apiKeyController, apiKeyService, externalController)
}

func setupAccountingRoutes(router *echo.Group, jwtConfig echojwt.Config, accountingController *AccountingController) {
	accountingGroup := router.Group("/accounting")
	accountingGroup.Use(echojwt.WithConfig(jwtConfig))

	accountingGroup.GET("/partners", accountingController.ListPartners)
	accountingGroup.POST("/partners", accountingController.CreatePartner)
	accountingGroup.PUT("/partners/:id", accountingController.UpdatePartner)
	accountingGroup.DELETE("/partners/:id", accountingController.DeletePartner)

	accountingGroup.GET("/costs", accountingController.ListCosts)
	accountingGroup.POST("/costs", accountingController.CreateCost)
	accountingGroup.PUT("/costs/:id", accountingController.UpdateCost)
	accountingGroup.DELETE("/costs/:id", accountingController.DeleteCost)
	accountingGroup.POST("/costs/:id/advance", accountingController.AdvanceRecurringCost)

	accountingGroup.GET("/payments", accountingController.ListPayments)
	accountingGroup.POST("/payments", accountingController.CreatePayment)
	accountingGroup.PUT("/payments/:id", accountingController.UpdatePayment)
	accountingGroup.DELETE("/payments/:id", accountingController.DeletePayment)
	accountingGroup.POST("/payments/:id/receipt", accountingController.UploadReceipt)
	accountingGroup.GET("/payments/:id/receipt", accountingController.DownloadReceipt)

	accountingGroup.GET("/summary", accountingController.GetSummary)
	accountingGroup.GET("/sellable-resources", accountingController.ListSellableResources)
	accountingGroup.GET("/profitability/users", accountingController.GetUserProfitability)
	accountingGroup.GET("/profitability/locations", accountingController.GetLocationProfitability)
}

// setupTunnelHealthRoutes exposes the "tunnel-ai" phase-1 read-only
// dashboard (see TunnelHealthController's own doc comment) -- admin-only,
// same jwtConfig gate as every other admin-facing group in this file.
func setupTunnelHealthRoutes(router *echo.Group, jwtConfig echojwt.Config, tunnelHealthController *TunnelHealthController) {
	tunnelHealthGroup := router.Group("/tunnel-health")
	tunnelHealthGroup.Use(echojwt.WithConfig(jwtConfig))

	tunnelHealthGroup.GET("/statuses", tunnelHealthController.ListStatuses)
	tunnelHealthGroup.GET("/events", tunnelHealthController.ListEvents)
	tunnelHealthGroup.GET("/actions", tunnelHealthController.ListActions)
	tunnelHealthGroup.GET("/health-scores", tunnelHealthController.ListHealthScoreHistory)
	tunnelHealthGroup.GET("/incident-diagnoses", tunnelHealthController.ListIncidentDiagnoses)
	tunnelHealthGroup.GET("/backup-probes", tunnelHealthController.ListBackupProbeResults)

	tunnelHealthGroup.GET("/settings", tunnelHealthController.GetSettings)
	tunnelHealthGroup.PATCH("/settings", tunnelHealthController.UpdateSettings)

	tunnelHealthGroup.GET("/policies", tunnelHealthController.ListPolicies)
	tunnelHealthGroup.PUT("/policies/:id", tunnelHealthController.UpdatePolicy)

	tunnelHealthGroup.GET("/redline", tunnelHealthController.ListRedlineEntries)
	tunnelHealthGroup.POST("/redline", tunnelHealthController.AddRedlineEntry)
	tunnelHealthGroup.DELETE("/redline/:id", tunnelHealthController.RemoveRedlineEntry)

	tunnelHealthGroup.GET("/graph", tunnelHealthController.GetLatestGraph)
	tunnelHealthGroup.GET("/tunnel-map", tunnelHealthController.GetTunnelMap)

	tunnelHealthGroup.GET("/user-manager-protocols", tunnelHealthController.ListUserManagerProtocolStatuses)
	tunnelHealthGroup.GET("/user-manager-protocols/events", tunnelHealthController.ListUserManagerProtocolEvents)
}

// setupReportsRoutes registers the admin-only "Reports" section's read
// endpoints -- every handler enforces admin-only itself (via
// ReportsController.requireAdmin), mirroring how xui-panel/v2ray-package
// route groups are JWT-protected at the group level but role-checked
// per-handler rather than via a separate admin-only middleware.
func setupReportsRoutes(router *echo.Group, jwtConfig echojwt.Config, reportsController *ReportsController) {
	reportsGroup := router.Group("/reports")
	reportsGroup.Use(echojwt.WithConfig(jwtConfig))

	reportsGroup.GET("/daily-usage", reportsController.DailyUsage)
	reportsGroup.GET("/peak-usage", reportsController.PeakUsage)
	reportsGroup.GET("/reseller-ranking", reportsController.ResellerRanking)
	reportsGroup.GET("/user-ranking", reportsController.UserRanking)
	reportsGroup.GET("/reseller-activity", reportsController.ResellerActivityRanking)
	reportsGroup.GET("/expiring-soon", reportsController.ExpiringSoon)
	reportsGroup.GET("/protocol-share", reportsController.ProtocolShare)
	reportsGroup.GET("/online-users", reportsController.OnlineUsers)
	reportsGroup.GET("/panel-health", reportsController.PanelHealth)
	reportsGroup.GET("/financial", reportsController.Financial)
	reportsGroup.GET("/renewal-rate", reportsController.RenewalRate)
	reportsGroup.GET("/popular-locations", reportsController.PopularLocations)
	reportsGroup.GET("/anomaly-alerts", reportsController.AnomalyAlerts)
	reportsGroup.GET("/resource-usage", reportsController.ResourceUsage)
	reportsGroup.GET("/export", reportsController.ExcelExport)
	reportsGroup.GET("/reseller-quota-prediction", reportsController.ResellerQuotaPrediction)
}

func setupXuiPanelRoutes(router *echo.Group, jwtConfig echojwt.Config, xuiPanelController *XuiPanelController) {
	panelGroup := router.Group("/xui-panel")
	panelGroup.Use(echojwt.WithConfig(jwtConfig))

	panelGroup.GET("", xuiPanelController.ListPanels)
	panelGroup.POST("", xuiPanelController.CreatePanel)
	panelGroup.PUT("/:id", xuiPanelController.UpdatePanel)
	panelGroup.DELETE("/:id", xuiPanelController.DeletePanel)
	panelGroup.POST("/:id/test", xuiPanelController.TestConnection)
	panelGroup.POST("/test", xuiPanelController.TestConnectionUnsaved)
}

func setupV2RayPackageRoutes(router *echo.Group, jwtConfig echojwt.Config, v2rayPackageController *V2RayPackageController) {
	packageGroup := router.Group("/v2ray-package")
	packageGroup.Use(echojwt.WithConfig(jwtConfig))

	packageGroup.GET("", v2rayPackageController.ListPackages)
	packageGroup.POST("", v2rayPackageController.CreatePackage)
	packageGroup.POST("/bulk", v2rayPackageController.BulkCreatePackages)
	packageGroup.POST("/export", v2rayPackageController.ExportPackages)
	packageGroup.PUT("/:id", v2rayPackageController.UpdatePackage)
	packageGroup.DELETE("/:id", v2rayPackageController.DeletePackage)
	packageGroup.PATCH("/:id/reset-usage", v2rayPackageController.ResetPackageUsage)
	packageGroup.PATCH("/:id/renew", v2rayPackageController.RenewPackage)
	packageGroup.POST("/bulk-delete", v2rayPackageController.BulkDeletePackages)
	packageGroup.GET("/:id/live-usage", v2rayPackageController.GetLiveUsage)
	packageGroup.GET("/summary/self", v2rayPackageController.GetSelfSummary)
	packageGroup.GET("/summary/admin", v2rayPackageController.GetAdminSummary)
	packageGroup.GET("/:id/share", v2rayPackageController.GetPackageShareStatus)
	packageGroup.PATCH("/:id/share/status", v2rayPackageController.UpdatePackageShareStatus)
	packageGroup.PATCH("/:id/share/expire", v2rayPackageController.UpdatePackageShareExpire)
	packageGroup.GET("/reseller/:reseller_id", v2rayPackageController.ListPackagesByReseller)
	packageGroup.POST("/reseller/:reseller_id", v2rayPackageController.CreatePackageForReseller)
	packageGroup.PUT("/reseller/:reseller_id/:id", v2rayPackageController.UpdatePackageForReseller)
	packageGroup.DELETE("/reseller/:reseller_id/:id", v2rayPackageController.DeletePackageForReseller)

	packageGroup.GET("/sale-title", v2rayPackageController.ListSaleTitles)
	packageGroup.PUT("/sale-title", v2rayPackageController.SetSaleTitle)
	packageGroup.PUT("/reseller/:reseller_id/sale-title", v2rayPackageController.SetSaleTitleForReseller)

	packageGroup.GET("/reseller/:reseller_id/panels", v2rayPackageController.GetAssignedXuiPanels)
	packageGroup.GET("/reseller/:reseller_id/panels/summary", v2rayPackageController.GetAssignedXuiPanelSummaries)
	packageGroup.PUT("/reseller/:reseller_id/panels", v2rayPackageController.SetAssignedXuiPanels)
}

func setupDNSPanelRoutes(router *echo.Group, jwtConfig echojwt.Config, dnsPanelController *DNSPanelController) {
	panelGroup := router.Group("/dns-panel")
	panelGroup.Use(echojwt.WithConfig(jwtConfig))

	panelGroup.GET("", dnsPanelController.ListPanels)
	panelGroup.POST("", dnsPanelController.CreatePanel)
	panelGroup.PUT("/:id", dnsPanelController.UpdatePanel)
	panelGroup.DELETE("/:id", dnsPanelController.DeletePanel)
	panelGroup.POST("/:id/test", dnsPanelController.TestConnection)
	panelGroup.POST("/test", dnsPanelController.TestConnectionUnsaved)
}

func setupDNSPlanRoutes(router *echo.Group, jwtConfig echojwt.Config, dnsPlanController *DNSPlanController) {
	planGroup := router.Group("/dns-plan")
	planGroup.Use(echojwt.WithConfig(jwtConfig))

	planGroup.GET("", dnsPlanController.ListPlans)
	planGroup.POST("", dnsPlanController.CreatePlan)
	planGroup.PUT("/:id", dnsPlanController.UpdatePlan)
	planGroup.DELETE("/:id", dnsPlanController.DeletePlan)
}

// setupApplicationPlanRoutes mirrors setupDNSPlanRoutes' route shape, but
// unlike DNS, GET here is reachable by both admin and reseller sessions --
// see ApplicationPlanController.ListPlans' own doc comment for why (a
// reseller needs the active-plan catalog to populate their own Application
// form's plan picker; only ?all=true is admin-only).
func setupApplicationPlanRoutes(router *echo.Group, jwtConfig echojwt.Config, applicationPlanController *ApplicationPlanController) {
	planGroup := router.Group("/application-plan")
	planGroup.Use(echojwt.WithConfig(jwtConfig))

	planGroup.GET("", applicationPlanController.ListPlans)
	planGroup.POST("", applicationPlanController.CreatePlan)
	planGroup.PUT("/:id", applicationPlanController.UpdatePlan)
	planGroup.DELETE("/:id", applicationPlanController.DeletePlan)
}

func setupDNSAccountRoutes(router *echo.Group, jwtConfig echojwt.Config, dnsAccountController *DNSAccountController) {
	accountGroup := router.Group("/dns-account")
	accountGroup.Use(echojwt.WithConfig(jwtConfig))

	accountGroup.GET("", dnsAccountController.ListAccounts)
	accountGroup.POST("", dnsAccountController.CreateAccount)
	accountGroup.PUT("/:id", dnsAccountController.UpdateAccount)
	accountGroup.DELETE("/:id", dnsAccountController.DeleteAccount)
	accountGroup.PATCH("/:id/reset-usage", dnsAccountController.ResetAccountUsage)
	accountGroup.POST("/bulk-delete", dnsAccountController.BulkDeleteAccounts)
	accountGroup.GET("/summary/self", dnsAccountController.GetSelfSummary)
	accountGroup.GET("/summary/admin", dnsAccountController.GetAdminSummary)
	accountGroup.GET("/:id/share", dnsAccountController.GetAccountShareStatus)
	accountGroup.PATCH("/:id/share/status", dnsAccountController.UpdateAccountShareStatus)
	accountGroup.PATCH("/:id/share/expire", dnsAccountController.UpdateAccountShareExpire)
	accountGroup.GET("/reseller/:reseller_id", dnsAccountController.ListAccountsByReseller)
	accountGroup.POST("/reseller/:reseller_id", dnsAccountController.CreateAccountForReseller)
	accountGroup.PUT("/reseller/:reseller_id/:id", dnsAccountController.UpdateAccountForReseller)
	accountGroup.DELETE("/reseller/:reseller_id/:id", dnsAccountController.DeleteAccountForReseller)

	accountGroup.GET("/reseller/:reseller_id/panels", dnsAccountController.GetAssignedDNSPanels)
	accountGroup.GET("/reseller/:reseller_id/panels/summary", dnsAccountController.GetAssignedDNSPanelSummaries)
	accountGroup.PUT("/reseller/:reseller_id/panels", dnsAccountController.SetAssignedDNSPanels)
	accountGroup.POST("/reseller/:reseller_id/panels/:id/test", dnsAccountController.TestPanelForReseller)
}

// setupDNSPublicRoutes registers the two unauthenticated DNS endpoints
// directly on router (no JWT group) -- mirrors setupV2RayPublicRoutes
// exactly.
func setupDNSPublicRoutes(router *echo.Group, dnsPublicController *DNSPublicController) {
	router.GET("/dns-share/:uuid", dnsPublicController.GetAccountShareDetails)
	router.POST("/dns-share/:uuid/register-ip", dnsPublicController.RegisterIP)
}

func setupV2RayTrafficPackageRoutes(router *echo.Group, jwtConfig echojwt.Config, v2rayTrafficPackageController *V2RayTrafficPackageController) {
	packageGroup := router.Group("/v2ray-traffic-package")
	packageGroup.Use(echojwt.WithConfig(jwtConfig))

	packageGroup.GET("", v2rayTrafficPackageController.ListTrafficPackages)
	packageGroup.POST("", v2rayTrafficPackageController.CreateTrafficPackage)
	packageGroup.PUT("/:id", v2rayTrafficPackageController.UpdateTrafficPackage)
	packageGroup.DELETE("/:id", v2rayTrafficPackageController.DeleteTrafficPackage)
	packageGroup.POST("/purchase", v2rayTrafficPackageController.PurchasePackage)
	packageGroup.GET("/purchases/self", v2rayTrafficPackageController.ListMyPurchases)
}

// setupV2RayPublicRoutes registers the two unauthenticated V2Ray endpoints
// directly on router (no JWT group) -- mirrors setupUserRoutes's public
// group exactly.
func setupV2RayPublicRoutes(router *echo.Group, v2rayPublicController *V2RayPublicController) {
	router.GET("/v2ray-share/:uuid", v2rayPublicController.GetPackageShareDetails)
	router.GET("/v2ray-sub/:uuid", v2rayPublicController.GetSubscription)
}

// setupApplicationRoutes registers the admin/reseller-facing "Applications"
// (اپلیکیشن) CRUD -- mirrors setupV2RayPackageRoutes' shape exactly (a
// plain, un-suffixed group auto-scoped via peerScopeFromContext for both
// an admin and a reseller caller, plus admin-only /reseller/:reseller_id
// variants).
func setupApplicationRoutes(router *echo.Group, jwtConfig echojwt.Config, applicationController *ApplicationController) {
	appGroup := router.Group("/application")
	appGroup.Use(echojwt.WithConfig(jwtConfig))

	appGroup.GET("", applicationController.ListApplications)
	appGroup.POST("", applicationController.CreateApplication)
	appGroup.PUT("/:id", applicationController.UpdateApplication)
	appGroup.DELETE("/:id", applicationController.DeleteApplication)
	appGroup.GET("/reseller/:reseller_id", applicationController.ListApplicationsByReseller)
	appGroup.POST("/reseller/:reseller_id", applicationController.CreateApplicationForReseller)
	appGroup.PUT("/reseller/:reseller_id/:id", applicationController.UpdateApplicationForReseller)

	appGroup.GET("/resource-location", applicationController.ListResourceLocations)
	appGroup.PUT("/resource-location", applicationController.SetResourceLocation)

	appGroup.POST("/openvpn-template", applicationController.UploadOpenVpnTemplate)
	appGroup.GET("/openvpn-template", applicationController.GetOpenVpnTemplateStatus)
	appGroup.GET("/openvpn-template/download", applicationController.DownloadOpenVpnTemplate)
}

// setupApplicationVersionRoutes registers the admin-only "مدیریت
// اپلیکیشن" (Application Management) hamburger-menu section -- every
// handler is additionally gated by forbidUnlessAdmin inside
// ApplicationVersionController itself (matching UploadOpenVpnTemplate's
// own belt-and-suspenders admin check), so a reseller session reaching
// this route group via the shared jwtConfig still gets rejected at the
// handler level, not just kept off the admin-only menu item client-side.
func setupApplicationVersionRoutes(router *echo.Group, jwtConfig echojwt.Config, applicationVersionController *ApplicationVersionController) {
	versionGroup := router.Group("/application-version")
	versionGroup.Use(echojwt.WithConfig(jwtConfig))

	versionGroup.GET("", applicationVersionController.ListVersions)
	versionGroup.POST("", applicationVersionController.PublishVersion)
	versionGroup.DELETE("/:id", applicationVersionController.DeleteVersion)

	versionGroup.GET("/maintenance-mode", applicationVersionController.GetMaintenanceMode)
	versionGroup.PUT("/maintenance-mode", applicationVersionController.SetMaintenanceMode)
}

// setupAppUserRoutes registers the MOBILE APP's own login (public, no JWT
// group at all -- mirrors setupV2RayPublicRoutes) plus its authenticated
// "me"/"online-count" endpoints, which are gated by AppAuthController's
// OWN separate JWT config/secret (see AppAuthController's own doc
// comment), never the admin/reseller jwtConfig this function's caller
// also builds -- app-user tokens and admin/reseller tokens must never be
// interchangeable.
func setupAppUserRoutes(router *echo.Group, appAuthController *AppAuthController) {
	appAuthGroup := router.Group("/app")
	appAuthGroup.POST("/login", appAuthController.Login)
	// version-check is public/unauthenticated, same reasoning as Login --
	// see CheckVersion's own doc comment for why it must work BEFORE
	// login (a user who cannot currently authenticate still needs to
	// find out they must update or that the service is under
	// maintenance).
	appAuthGroup.GET("/version-check", appAuthController.CheckVersion)

	appAuthProtected := appAuthGroup.Group("")
	appAuthProtected.Use(echojwt.WithConfig(appAuthController.AppJWTConfig()))
	appAuthProtected.GET("/me", appAuthController.Me)
	appAuthProtected.GET("/me/connect-configs", appAuthController.ConnectConfigs)
	appAuthProtected.GET("/me/weekly-usage", appAuthController.WeeklyUsage)
	appAuthProtected.GET("/online-count", appAuthController.OnlineCount)
	appAuthProtected.GET("/devices", appAuthController.Devices)
	appAuthProtected.POST("/devices/revoke", appAuthController.RevokeDevice)
	appAuthProtected.POST("/devices/report-resource", appAuthController.ReportDeviceResource)
}

func setupUserManagerRoutes(router *echo.Group, mwpClients *common.MwpClients, jwtConfig echojwt.Config, userManagerAccountController *UserManagerAccountController) {
	userManagerGroup := router.Group("/user-manager/account")
	userManagerGroup.Use(echojwt.WithConfig(jwtConfig))

	userManagerGroup.GET("/:id/share", userManagerAccountController.GetAccountShareStatus)
	userManagerGroup.PATCH("/:id/share/status", userManagerAccountController.UpdateAccountShareStatus)
	userManagerGroup.PATCH("/:id/share/expire", userManagerAccountController.UpdateAccountShareExpire)
	userManagerGroup.POST("/:id/config", userManagerAccountController.UploadAccountConfig)
	userManagerGroup.GET("/:id/config", userManagerAccountController.DownloadAccountConfig)
	userManagerGroup.POST("/reseller/:reseller_id/:id/config", userManagerAccountController.UploadAccountConfigForReseller)

	userManagerSecured := userManagerGroup.Group("")
	userManagerSecured.Use(middleware.ClientConnectionMiddleware(mwpClients))

	userManagerSecured.GET("", userManagerAccountController.ListAccounts)
	userManagerSecured.GET("/summary/self", userManagerAccountController.GetSelfSummary)
	userManagerSecured.POST("", userManagerAccountController.CreateAccount)
	userManagerSecured.POST("/bulk", userManagerAccountController.BulkCreateAccounts)
	userManagerSecured.POST("/bulk-import", userManagerAccountController.BulkImportAccounts)
	userManagerSecured.PUT("/:id", userManagerAccountController.UpdateAccount)
	userManagerSecured.DELETE("/:id", userManagerAccountController.DeleteAccount)
	userManagerSecured.PATCH("/:id/reset-usage", userManagerAccountController.ResetAccountUsage)
	userManagerSecured.POST("/bulk-delete", userManagerAccountController.BulkDeleteAccounts)
	userManagerSecured.PATCH("/:id/status", userManagerAccountController.ToggleAccountStatus)
	userManagerSecured.PATCH("/:id/password", userManagerAccountController.ChangeAccountPassword)
	userManagerSecured.GET("/reseller/:reseller_id", userManagerAccountController.ListAccountsByReseller)
	userManagerSecured.POST("/reseller/:reseller_id", userManagerAccountController.CreateAccountForReseller)
	userManagerSecured.POST("/reseller/:reseller_id/bulk-import", userManagerAccountController.BulkImportAccountsForReseller)
	userManagerSecured.PUT("/reseller/:reseller_id/:id", userManagerAccountController.UpdateAccountForReseller)
	userManagerSecured.DELETE("/reseller/:reseller_id/:id", userManagerAccountController.DeleteAccountForReseller)
	userManagerSecured.PATCH("/reseller/:reseller_id/:id/status", userManagerAccountController.ToggleAccountStatusForReseller)
	userManagerSecured.PATCH("/reseller/:reseller_id/:id/password", userManagerAccountController.ChangeAccountPasswordForReseller)
}

func setupUserManagerGroupProfileRoutes(router *echo.Group, mwpClients *common.MwpClients, jwtConfig echojwt.Config, userManagerGroupProfileController *UserManagerGroupProfileController) {
	groupProfileGroup := router.Group("/user-manager")
	groupProfileGroup.Use(echojwt.WithConfig(jwtConfig))

	groupProfileSecured := groupProfileGroup.Group("")
	groupProfileSecured.Use(middleware.ClientConnectionMiddleware(mwpClients))

	groupProfileSecured.GET("/group", userManagerGroupProfileController.ListGroups)
	groupProfileSecured.GET("/profile", userManagerGroupProfileController.ListProfiles)
}

func setupUserManagerProtocolConfigRoutes(router *echo.Group, jwtConfig echojwt.Config, userManagerProtocolConfigController *UserManagerProtocolConfigController) {
	protocolConfigGroup := router.Group("/user-manager/protocol-config")
	protocolConfigGroup.Use(echojwt.WithConfig(jwtConfig))

	protocolConfigGroup.GET("", userManagerProtocolConfigController.ListProtocolConfigs)
	protocolConfigGroup.PUT("", userManagerProtocolConfigController.UpsertProtocolConfig)
	protocolConfigGroup.POST("/:protocol/certificate", userManagerProtocolConfigController.UploadProtocolCertificateFile)
	protocolConfigGroup.GET("/:protocol/certificate", userManagerProtocolConfigController.DownloadProtocolCertificateFile)
	protocolConfigGroup.DELETE("/:protocol/certificate", userManagerProtocolConfigController.DeleteProtocolCertificateFile)
	protocolConfigGroup.POST("/:protocol/client-app", userManagerProtocolConfigController.UploadProtocolClientAppFile)
	protocolConfigGroup.GET("/:protocol/client-app", userManagerProtocolConfigController.DownloadProtocolClientAppFile)
	protocolConfigGroup.DELETE("/:protocol/client-app", userManagerProtocolConfigController.DeleteProtocolClientAppFile)
}

func setupLogRoutes(router *echo.Group, jwtConfig echojwt.Config, logController *LogController) {
	logGroup := router.Group("/logs")
	logGroup.Use(echojwt.WithConfig(jwtConfig))

	logGroup.GET("", logController.ListLogs)
	logGroup.GET("/download", logController.DownloadCurrentLog)
	logGroup.GET("/download-all", logController.DownloadAllLogs)
	logGroup.DELETE("", logController.ClearLogs)
}

// setupApiKeyRoutes wires phase 4-3's external API key management
// (/api-keys/*, JWT-admin-only -- generate/list/revoke) and the external
// API surface those keys unlock (/external/*, gated by
// middleware.APIKeyMiddleware instead of a JWT). /external/ping is the
// first endpoint on that surface: a minimal, side-effect-free way for an
// external panel/automation client to confirm its key is valid before this
// surface grows further endpoints in a later pass.
func setupApiKeyRoutes(router *echo.Group, jwtConfig echojwt.Config, apiKeyController *ApiKeyController, apiKeyService *service.ApiKeyService, externalController *ExternalController) {
	keyGroup := router.Group("/api-keys")
	keyGroup.Use(echojwt.WithConfig(jwtConfig))
	keyGroup.POST("", apiKeyController.Create)
	keyGroup.GET("", apiKeyController.List)
	keyGroup.DELETE("/:id", apiKeyController.Revoke)

	externalGroup := router.Group("/external")
	externalGroup.Use(middleware.APIKeyMiddleware(apiKeyService))
	externalGroup.GET("/ping", externalController.Ping)
}

// setupSupportTokenRoutes wires the "privacy-first remote debugging"
// feature: /support/token/* (admin-only, this install's own settings
// page) plus the public, token-gated /support/logs (see
// middleware.licenseMiddlewareExemptPaths -- Logs is exempted from the
// license lockdown too, so an unlicensed/blocked install can still be
// remotely diagnosed via a support token even while the rest of the API
// is locked out).
func setupSupportTokenRoutes(router *echo.Group, jwtConfig echojwt.Config, supportTokenController *SupportTokenController) {
	supportGroup := router.Group("/support")

	tokenGroup := supportGroup.Group("/token")
	tokenGroup.Use(echojwt.WithConfig(jwtConfig))
	tokenGroup.POST("", supportTokenController.Generate)
	tokenGroup.GET("", supportTokenController.Status)
	tokenGroup.DELETE("", supportTokenController.Revoke)

	supportGroup.GET("/logs", supportTokenController.Logs)
}

func setupHelpCenterRoutes(router *echo.Group, jwtConfig echojwt.Config, helpCenterController *HelpCenterController) {
	helpCenterGroup := router.Group("/help-center")
	helpCenterGroup.Use(echojwt.WithConfig(jwtConfig))

	helpCenterGroup.GET("/tickets", helpCenterController.ListTickets)
	helpCenterGroup.POST("/tickets", helpCenterController.CreateTicket)
	helpCenterGroup.GET("/tickets/:id", helpCenterController.GetTicketMessages)
	helpCenterGroup.POST("/tickets/:id/messages", helpCenterController.PostMessage)
	helpCenterGroup.POST("/tickets/:id/typing", helpCenterController.SetTyping)
	helpCenterGroup.GET("/tickets/:id/typing", helpCenterController.GetTypingStatus)
	helpCenterGroup.GET("/tickets/:id/messages/:messageId/attachment", helpCenterController.DownloadAttachment)
}

func setupLicenseRoutes(router *echo.Group, jwtConfig echojwt.Config, licenseController *LicenseController) {
	licenseGroup := router.Group("/license")

	// Activate is deliberately registered OUTSIDE the JWT-protected group
	// below and exempted from LicenseMiddleware (see
	// middleware.licenseMiddlewareExemptPaths) -- it's the one endpoint
	// that must stay reachable while the install is blocked as unlicensed,
	// since no admin can ever obtain a JWT under that condition otherwise.
	// It authenticates the caller itself via admin username/password in the
	// request body instead (see LicenseController.Activate).
	licenseGroup.POST("/activate", licenseController.Activate)

	// trial is exempted from LicenseMiddleware for the same reason as
	// activate above (see middleware.licenseMiddlewareExemptPaths) and
	// authenticates itself the same way (admin credentials in the body).
	licenseGroup.POST("/trial", licenseController.Trial)

	// public-status is also unauthenticated (see LicenseController.
	// PublicStatus's doc comment) -- the frontend's root-level lockdown
	// guard polls this before any login attempt to decide whether to force
	// the activation screen. Unlike /activate, this one is NOT exempted
	// from LicenseMiddleware, so a blocked install still gets
	// LicenseMiddleware's 403 here, which the frontend treats the same way.
	licenseGroup.GET("/public-status", licenseController.PublicStatus)

	licenseGroup.Use(echojwt.WithConfig(jwtConfig))
	licenseGroup.GET("/status", licenseController.GetStatus)
	licenseGroup.GET("/check-update", licenseController.CheckUpdate)
	licenseGroup.POST("/revoke", licenseController.Revoke)
}

func setupBotSettingsRoutes(router *echo.Group, jwtConfig echojwt.Config, botSettingsController *BotSettingsController) {
	botGroup := router.Group("/bot-settings")
	botGroup.Use(echojwt.WithConfig(jwtConfig))

	botGroup.GET("", botSettingsController.GetSettings)
	botGroup.PUT("", botSettingsController.UpdateSettings)
	botGroup.POST("/test-socks5", botSettingsController.TestSocks5)

	botGroup.GET("/extra-admins", botSettingsController.ListExtraAdminChatIDs)
	botGroup.POST("/extra-admins", botSettingsController.AddExtraAdminChatID)
	botGroup.DELETE("/extra-admins/:id", botSettingsController.RemoveExtraAdminChatID)
}

func setupSSLSettingsRoutes(router *echo.Group, jwtConfig echojwt.Config, sslSettingsController *SSLSettingsController) {
	sslGroup := router.Group("/ssl-settings")
	sslGroup.Use(echojwt.WithConfig(jwtConfig))

	sslGroup.GET("", sslSettingsController.GetSettings)
	sslGroup.PUT("", sslSettingsController.UpdateSettings)
}

func setupSecurityRoutes(router *echo.Group, jwtConfig echojwt.Config, securityController *SecurityController) {
	securityGroup := router.Group("/security")
	securityGroup.Use(echojwt.WithConfig(jwtConfig))

	securityGroup.GET("/geoip/status", securityController.GetGeoIPStatus)
	securityGroup.GET("/geoip/mode", securityController.GetGeoIPMode)
	securityGroup.PUT("/geoip/mode", securityController.SetGeoIPMode)
	securityGroup.POST("/geoip/city", securityController.UploadGeoIPCity)
	securityGroup.DELETE("/geoip/city", securityController.DeleteGeoIPCity)
	securityGroup.POST("/geoip/asn", securityController.UploadGeoIPASN)
	securityGroup.DELETE("/geoip/asn", securityController.DeleteGeoIPASN)
	securityGroup.DELETE("/geoip/cache", securityController.ClearGeoIPCache)

	securityGroup.GET("/identities", securityController.ListIdentities)
	securityGroup.GET("/identities/:protocol/:identity/history", securityController.GetIdentityHistory)
	securityGroup.DELETE("/connection-log", securityController.ClearConnectionHistory)

	securityGroup.GET("/ether-traffic", securityController.GetEtherTraffic)
	securityGroup.DELETE("/ether-traffic", securityController.ClearEtherTraffic)

	securityGroup.GET("/threats", securityController.GetThreats)

	securityGroup.POST("/retention/cleanup", securityController.RunRetentionCleanup)
}

func setupSystemConfigRoutes(router *echo.Group, jwtConfig echojwt.Config, systemConfigController *SystemConfigController, systemHealthController *SystemHealthController) {
	systemGroup := router.Group("/system-config")
	systemGroup.Use(echojwt.WithConfig(jwtConfig))

	systemGroup.GET("/port", systemConfigController.GetPortConfig)
	systemGroup.PUT("/port", systemConfigController.UpdatePortConfig)
	systemGroup.GET("/database-size", systemConfigController.GetDatabaseSize)
	systemGroup.GET("/health", systemHealthController.GetSystemHealth)
}

func setupBackupRoutes(router *echo.Group, jwtConfig echojwt.Config, backupController *BackupController) {
	backupGroup := router.Group("/backup")
	backupGroup.Use(echojwt.WithConfig(jwtConfig))

	backupGroup.GET("/download", backupController.DownloadBackup)
	backupGroup.POST("/restore", backupController.UploadRestoreBackup)
	backupGroup.POST("/send-now", backupController.SendInstantBackup)
}

func setupTrafficPackageRoutes(router *echo.Group, jwtConfig echojwt.Config, trafficPackageController *TrafficPackageController) {
	packageGroup := router.Group("/traffic-package")
	packageGroup.Use(echojwt.WithConfig(jwtConfig))

	packageGroup.GET("", trafficPackageController.ListTrafficPackages)
	packageGroup.POST("", trafficPackageController.CreateTrafficPackage)
	packageGroup.PUT("/:id", trafficPackageController.UpdateTrafficPackage)
	packageGroup.DELETE("/:id", trafficPackageController.DeleteTrafficPackage)
	packageGroup.POST("/purchase", trafficPackageController.PurchasePackage)
	packageGroup.GET("/purchases/self", trafficPackageController.ListMyPurchases)
}

func setupUserManagerTrafficPackageRoutes(router *echo.Group, jwtConfig echojwt.Config, userManagerTrafficPackageController *UserManagerTrafficPackageController) {
	packageGroup := router.Group("/user-manager-traffic-package")
	packageGroup.Use(echojwt.WithConfig(jwtConfig))

	packageGroup.GET("", userManagerTrafficPackageController.ListTrafficPackages)
	packageGroup.POST("", userManagerTrafficPackageController.CreateTrafficPackage)
	packageGroup.PUT("/:id", userManagerTrafficPackageController.UpdateTrafficPackage)
	packageGroup.DELETE("/:id", userManagerTrafficPackageController.DeleteTrafficPackage)
	packageGroup.POST("/purchase", userManagerTrafficPackageController.PurchasePackage)
	packageGroup.GET("/purchases/self", userManagerTrafficPackageController.ListMyPurchases)
}

func setupAuditLogRoutes(router *echo.Group, jwtConfig echojwt.Config, auditLogController *AuditLogController) {
	auditGroup := router.Group("/audit-log")
	auditGroup.Use(echojwt.WithConfig(jwtConfig))

	auditGroup.GET("", auditLogController.ListRecent)
}

func setupInvoiceRoutes(router *echo.Group, jwtConfig echojwt.Config, invoiceController *InvoiceController) {
	invoiceGroup := router.Group("/invoice")
	invoiceGroup.Use(echojwt.WithConfig(jwtConfig))

	invoiceGroup.POST("/reseller/:reseller_id", invoiceController.CreateInvoice)
	invoiceGroup.GET("/reseller/:reseller_id", invoiceController.ListInvoicesByReseller)
	invoiceGroup.GET("/:invoice_id", invoiceController.GetInvoice)
	invoiceGroup.POST("/:invoice_id/issue", invoiceController.IssueInvoice)
	invoiceGroup.POST("/:invoice_id/pay", invoiceController.PayInvoice)
	invoiceGroup.POST("/:invoice_id/cancel", invoiceController.CancelInvoice)
}

func setupWalletRoutes(router *echo.Group, jwtConfig echojwt.Config, walletController *WalletController) {
	walletGroup := router.Group("/wallet")
	walletGroup.Use(echojwt.WithConfig(jwtConfig))

	walletGroup.GET("/reseller/:reseller_id/balance", walletController.GetWalletBalance)
	walletGroup.POST("/reseller/:reseller_id/credit", walletController.Credit)
	walletGroup.POST("/reseller/:reseller_id/debit", walletController.Debit)
	walletGroup.GET("/reseller/:reseller_id/ledger", walletController.GetLedgerHistory)
	walletGroup.POST("/reseller/:reseller_id/freeze", walletController.FreezeWallet)
	walletGroup.POST("/reseller/:reseller_id/unfreeze", walletController.UnfreezeWallet)
}

func setupPricePlanRoutes(router *echo.Group, jwtConfig echojwt.Config, pricePlanController *PricePlanController) {
	planGroup := router.Group("/pricing/plan")
	planGroup.Use(echojwt.WithConfig(jwtConfig))

	planGroup.GET("", pricePlanController.ListPricePlans)
	planGroup.POST("", pricePlanController.CreatePricePlan)
	planGroup.GET("/:plan_id", pricePlanController.GetPricePlan)
	planGroup.PUT("/:plan_id", pricePlanController.UpdatePricePlan)
	planGroup.PATCH("/:plan_id/deactivate", pricePlanController.DeactivatePricePlan)
}

func setupFaoximaRoutes(router *echo.Group, jwtConfig echojwt.Config, controller *FaoximaController) {
	group := router.Group("/faoxima")
	group.Use(echojwt.WithConfig(jwtConfig))

	group.GET("", controller.GetStatus)
	group.POST("/provision", controller.Provision)
	group.PUT("/token", controller.UpdateToken)
	group.POST("/disable", controller.Disable)
	group.POST("/enable", controller.Enable)
	group.DELETE("", controller.Remove)
	group.GET("/backup", controller.Backup)
	group.POST("/restore", controller.Restore)

	// Admin-only "مدیریت ربات فاکسیمای نمایندگان": per-reseller enable/
	// disable + billing period, distinct from the self-service group
	// above (which always targets the CALLER's own reseller id) -- see
	// FaoximaController's own doc comment on why these are kept separate.
	adminGroup := router.Group("/admin/faoxima")
	adminGroup.Use(echojwt.WithConfig(jwtConfig))
	adminGroup.GET("", controller.AdminListInstances)
	adminGroup.POST("/:reseller_id/enable", controller.AdminEnableInstance)
	adminGroup.POST("/:reseller_id/disable", controller.AdminDisableInstance)
	adminGroup.POST("/:reseller_id/reset-period", controller.AdminResetPeriod)
}

func setupResellerRoutes(router *echo.Group, jwtConfig echojwt.Config, resellerController *ResellerController) {
	resellerGroup := router.Group("/reseller")
	resellerGroup.Use(echojwt.WithConfig(jwtConfig))

	resellerGroup.GET("", resellerController.ListResellers)
	resellerGroup.POST("", resellerController.CreateReseller)
	resellerGroup.GET("/:id", resellerController.GetReseller)
	resellerGroup.PUT("/:id", resellerController.UpdateReseller)
	resellerGroup.DELETE("/:id", resellerController.DeleteReseller)
	resellerGroup.GET("/:id/interfaces", resellerController.GetAssignedInterfaces)
	resellerGroup.PUT("/:id/interfaces", resellerController.SetAssignedInterfaces)
	resellerGroup.GET("/:id/user-manager-groups", resellerController.GetAssignedUserManagerGroups)
	resellerGroup.PUT("/:id/user-manager-groups", resellerController.SetAssignedUserManagerGroups)
	resellerGroup.GET("/:id/user-manager-profiles", resellerController.GetAssignedUserManagerProfiles)
	resellerGroup.PUT("/:id/user-manager-profiles", resellerController.SetAssignedUserManagerProfiles)
	resellerGroup.GET("/:id/billing-prices", resellerController.GetBillingPrices)
	resellerGroup.PUT("/:id/billing-prices", resellerController.SetBillingPrices)
	resellerGroup.GET("/:id/billing-tiers", resellerController.GetBillingTiers)
	resellerGroup.PUT("/:id/billing-tiers", resellerController.SetBillingTiers)
	resellerGroup.POST("/:id/onboarding-complete", resellerController.CompleteOnboarding)
}

func setupAuthenticationRoutes(router *echo.Group, jwtConfig echojwt.Config, authController *AuthController) {
	authGroup := router.Group("/auth")
	authGroup.POST("/login", authController.Login)
	authGroup.POST("/otp/verify", authController.VerifyOtp)

	authProtected := authGroup.Group("")
	authProtected.Use(echojwt.WithConfig(jwtConfig))
	authProtected.PUT("/profile", authController.UpdateProfile)
}

func setupServerRoutes(router *echo.Group, jwtConfig echojwt.Config, serverController *ServerController) {
	serverGroup := router.Group("/server")
	serverGroup.Use(echojwt.WithConfig(jwtConfig))

	serverGroup.GET("", serverController.GetServers)
	serverGroup.GET("/endpoints", serverController.GetServerEndpoints)
	serverGroup.GET("/connection-health", serverController.GetConnectionHealth)
	serverGroup.POST("", serverController.CreateServer)
	serverGroup.PATCH("/:id/status", serverController.UpdateServerStatus)
	serverGroup.PUT("/:id", serverController.UpdateServer)
	serverGroup.DELETE("/:id", serverController.DeleteServer)
}

func setupInterfaceRoutes(router *echo.Group, mwpClients *common.MwpClients, jwtConfig echojwt.Config, wgInterfaceController *WgInterfaceController) {
	interfaceGroup := router.Group("/interface")
	interfaceGroup.Use(echojwt.WithConfig(jwtConfig))

	secured := interfaceGroup.Group("")
	secured.Use(middleware.ClientConnectionMiddleware(mwpClients))

	secured.GET("", wgInterfaceController.GetInterfaces)
	secured.POST("", wgInterfaceController.CreateInterface)
	secured.PATCH("/:id/status", wgInterfaceController.UpdateInterfaceStatus)
	secured.PUT("/:id", wgInterfaceController.UpdateInterface)
	secured.DELETE("/:id", wgInterfaceController.DeleteInterface)
}

func setupIPPoolRoutes(router *echo.Group, jwtConfig echojwt.Config, ipPpolController *IPPoolController) {
	ipPoolGroup := router.Group("/ip-pool")
	ipPoolGroup.Use(echojwt.WithConfig(jwtConfig))

	ipPoolGroup.GET("", ipPpolController.GetIPPools)
	ipPoolGroup.POST("", ipPpolController.CreateIPPool)
	ipPoolGroup.PUT("/:id", ipPpolController.UpdateIPPool)
	ipPoolGroup.DELETE("/:id", ipPpolController.DeleteIPPool)
}

func setupPeerRoutes(router *echo.Group, mwpClients *common.MwpClients, jwtConfig echojwt.Config, wgPeerController *WgPeerController) {
	peerGroup := router.Group("/peer")
	peerGroup.Use(echojwt.WithConfig(jwtConfig))

	peerGroup.POST("/allowed-address", wgPeerController.GetNewPeerAllowedAddress)
	peerGroup.GET("/credentials", wgPeerController.GetPeerCredentials)
	peerGroup.GET("/:id/share", wgPeerController.GetPeerShareStatus)
	peerGroup.PATCH("/:id/share/status", wgPeerController.UpdatePeerShareStatus)
	peerGroup.PATCH("/:id/share/expire", wgPeerController.UpdatePeerShareExpire)
	peerGroup.GET("/:id/config", wgPeerController.GetPeerConfig)
	peerGroup.GET("/:id/qrcode", wgPeerController.GetPeerQRCode)

	peerSecured := peerGroup.Group("")
	peerSecured.Use(middleware.ClientConnectionMiddleware(mwpClients))

	peerSecured.GET("", wgPeerController.GetPeers)
	peerSecured.POST("", wgPeerController.CreatePeer)
	peerSecured.POST("/bulk", wgPeerController.BulkCreatePeers)
	peerSecured.PATCH("/:id/status", wgPeerController.UpdatePeerStatus)
	peerSecured.PATCH("/:id/reset-usage", wgPeerController.ResetPeerUsage)
	peerSecured.PATCH("/reset-usage", wgPeerController.ResetPeerUsages)
	peerSecured.PUT("/:id", wgPeerController.UpdatePeer)
	peerSecured.DELETE("/:id", wgPeerController.DeletePeer)
	peerSecured.POST("/bulk-delete", wgPeerController.BulkDeletePeers)
	peerSecured.POST("/traffic/export", wgPeerController.ExportPeersTrafficData)
	peerSecured.GET("/activity/resellers", wgPeerController.GetResellerActivitySummary)
	peerSecured.GET("/activity/self", wgPeerController.GetSelfActivity)
	peerSecured.GET("/reseller/:reseller_id", wgPeerController.GetPeersByReseller)
	peerSecured.POST("/reseller/:reseller_id", wgPeerController.CreatePeerForReseller)
	peerSecured.PUT("/reseller/:reseller_id/:id", wgPeerController.UpdatePeerForReseller)
	peerSecured.DELETE("/reseller/:reseller_id/:id", wgPeerController.DeletePeerForReseller)
	peerSecured.PATCH("/reseller/:reseller_id/:id/status", wgPeerController.UpdatePeerStatusForReseller)
}

func setupDeviceInfoRoutes(router *echo.Group, mwpClients *common.MwpClients, jwtConfig echojwt.Config, deviceInfoController *DeviceDataController) {
	deviceGroup := router.Group("/device")
	deviceGroup.Use(echojwt.WithConfig(jwtConfig))

	deviceSecured := deviceGroup.Group("")
	deviceSecured.Use(middleware.ClientConnectionMiddleware(mwpClients))

	deviceSecured.GET("/stats", deviceInfoController.GetDeviceInfo)
	deviceSecured.GET("/traffic", deviceInfoController.GetDailyTrafficUsage)
	deviceSecured.PATCH("/traffic/reset", deviceInfoController.ResetTotalTrafficUsage)
}

func setupSyncRoutes(router *echo.Group, mwpClients *common.MwpClients, jwtConfig echojwt.Config, syncController *SyncController) {
	syncGroup := router.Group("/sync")
	syncGroup.Use(echojwt.WithConfig(jwtConfig))

	syncSecured := syncGroup.Group("")
	syncSecured.Use(middleware.ClientConnectionMiddleware(mwpClients))

	syncSecured.GET("/interfaces", syncController.GetSyncInterfaces)
	syncSecured.GET("/peers", syncController.GetSyncPeers)
	syncSecured.POST("/interfaces/selected", syncController.SyncSelectedInterfaces)
	syncSecured.POST("/peers/selected", syncController.SyncSelectedPeers)
	syncSecured.POST("/peers", syncController.SyncPeers)
	syncSecured.POST("/interfaces", syncController.SyncInterfaces)
}

func setupUserRoutes(router *echo.Group, userController *UserController) {
	userGroup := router.Group("/user")

	userGroup.GET("/:uuid/config", userController.GetUserConfig)
	userGroup.GET("/:uuid/qrcode", userController.GetUserQRCode)
	userGroup.GET("/:uuid/details", userController.GetUserDetails)
	userGroup.GET("/:uuid/user-manager-account", userController.GetUserManagerAccountShareDetails)
	userGroup.GET("/:uuid/user-manager-account/config", userController.GetUserManagerAccountConfig)
	userGroup.GET("/:uuid/user-manager-account/protocol-certificate", userController.GetUserManagerProtocolCertificateFile)
	userGroup.GET("/:uuid/user-manager-account/protocol-client-app", userController.GetUserManagerProtocolClientAppFile)
}
