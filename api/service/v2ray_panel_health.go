package service

import (
	"context"
	"time"

	"go.uber.org/zap"
	"gorm.io/gorm"

	"github.com/maahdima/mwp/api/adaptor/mikrotik"
	"github.com/maahdima/mwp/api/adaptor/xui"
	"github.com/maahdima/mwp/api/dataservice/model"
)

// V2RayPanelHealthService implements spec section ب-7's V2Ray/container
// row.
//
// REVISED (item 9): the original version of this service was written
// against a confirmed-with-the-admin assumption that x-ui panels run on
// SEPARATE physical servers, not RouterOS containers -- that assumption
// turned out to be wrong for how this admin actually deploys alireza0's
// x-ui: "پنل های v2ray که علیرضا هستن برروی میکروتیک هستند که با استفاده
// از کانتینر ایجاد شدن" (the x-ui panels run ON the Mikrotik router,
// created via RouterOS 7's native container feature). A container
// stopping is therefore one of the MOST COMMON reasons a panel goes
// unreachable, and previously this service could only ever report the
// generic symptom ("x-ui login failed") with zero way to tell that apart
// from a genuine network partition or a crashed-but-still-running x-ui
// process.
//
// diagnoseContainer (below) is the new step: when a panel fails its login
// check AND has a container mapping configured (XuiPanel.
// ContainerServerID/ContainerName), this service additionally queries
// that SPECIFIC container's live status via the mikrotik adaptor's new
// RouterOS /container REST client, and fires a more specific
// NotifyV2RayPanelContainerStopped alert when the container is confirmed
// stopped/stopping -- still alert-only (no automatic restart -- this
// codebase still has no confirmed-safe RouterOS container start/stop
// action wired up), but now with an actionable diagnosis instead of a
// bare "unreachable." A panel with no container mapping configured (nil
// ContainerServerID) falls back to exactly the original x-ui-login-only
// behavior -- this is fully backward compatible with every panel that
// predates this feature.
//
// Deliberately independent of V2RaySyncService's own background job
// (that job's xui.GetOnlineClients call is billing/quota-critical and
// already delicate -- this makes its OWN separate xui.Login reachability
// call per panel, on tunnel-ai's own schedule, rather than piggybacking
// on and risking that existing job).
type V2RayPanelHealthService struct {
	db              *gorm.DB
	botNotifier     *BotNotifier
	settings        *BotSettingsService
	mikrotikAdaptor *mikrotik.Adaptor
	logger          *zap.Logger
}

func NewV2RayPanelHealthService(db *gorm.DB) *V2RayPanelHealthService {
	return &V2RayPanelHealthService{
		db:       db,
		settings: NewBotSettingsService(db),
		logger:   zap.L().Named("V2RayPanelHealthService"),
	}
}

func (s *V2RayPanelHealthService) SetBotNotifier(notifier *BotNotifier) {
	s.botNotifier = notifier
}

// SetMikrotikAdaptor wires the RouterOS container-status lookup in after
// construction, mirroring this codebase's own established "set the
// optional collaborator post-construction" pattern (e.g. Reseller.
// SetV2RayResumer) -- nil-safe: a service constructed without this call
// (or in a test) simply never attempts container diagnosis, matching a
// panel with no container mapping configured.
func (s *V2RayPanelHealthService) SetMikrotikAdaptor(adaptor *mikrotik.Adaptor) {
	s.mikrotikAdaptor = adaptor
}

// Poll checks every registered x-ui panel's own reachability via a fresh
// login attempt -- independent of V2RaySyncService's own usage-sync
// tick, so a slow/failing tunnel-ai check can never interfere with
// billing-critical traffic sync, and vice versa.
func (s *V2RayPanelHealthService) Poll() {
	var panels []model.XuiPanel
	if err := s.db.Find(&panels).Error; err != nil {
		s.logger.Error("failed to list xui panels", zap.Error(err))
		return
	}

	now := time.Now()
	for _, panel := range panels {
		s.pollOnePanel(panel, now)
	}
}

func (s *V2RayPanelHealthService) pollOnePanel(panel model.XuiPanel, now time.Time) {
	ctx, cancel := context.WithTimeout(context.Background(), 15*time.Second)
	defer cancel()

	_, loginErr := xui.Login(ctx, panel)
	reachable := loginErr == nil

	var status model.XuiPanel
	if err := s.db.First(&status, panel.ID).Error; err != nil {
		return
	}
	wasHealthy := status.Status != "error"

	updates := map[string]interface{}{
		"last_synced_at": now,
	}
	if reachable {
		updates["status"] = "active"
		updates["last_error"] = nil
	} else {
		updates["status"] = "error"
		errStr := loginErr.Error()
		updates["last_error"] = errStr
	}
	if err := s.db.Model(&model.XuiPanel{}).Where("id = ?", panel.ID).Updates(updates).Error; err != nil {
		s.logger.Warn("failed to record xui panel health", zap.Uint("panel_id", panel.ID), zap.Error(err))
	}

	if reachable == wasHealthy {
		return
	}
	// Transition only -- same "alert on change, not every tick" policy
	// as TunnelHealthService/UserManagerProtocolHealthService.
	if reachable {
		return // silent recovery, matches this codebase's own established convention
	}

	// A panel with a configured container mapping gets the MORE SPECIFIC
	// diagnosis (item 9) instead of the generic "unreachable" alert --
	// diagnoseContainer itself decides whether it could reach a useful
	// conclusion (falls back to false, "" when it couldn't, e.g. no
	// mapping configured, or the ROUTER itself is also unreachable).
	if containerStopped, containerStatus, serverName := s.diagnoseContainer(panel); containerStopped {
		s.notifyContainerStopped(panel, serverName, containerStatus)
	} else {
		s.notifyDown(panel, loginErr)
	}

	// attemptRestart is intentionally NOT implemented yet -- see this
	// service's own top-level doc comment. The admin explicitly asked
	// to leave room for this once an x-ui-API-based restart path is
	// investigated, rather than attempting an unsupported action now.
	_ = s.attemptRestart
}

// diagnoseContainer checks whether panel's mapped RouterOS container
// (ContainerServerID/ContainerName) is confirmed stopped -- called only
// after the panel has already failed its own x-ui login check. Returns
// stopped=false whenever a definitive "stopped" diagnosis ISN'T possible
// (no mapping configured, no mikrotik adaptor wired up, or the container
// lookup itself failed/returned a running status) -- in every one of
// those cases the caller falls back to the original generic "unreachable"
// alert rather than a false, unsupported claim about the container.
func (s *V2RayPanelHealthService) diagnoseContainer(panel model.XuiPanel) (stopped bool, containerStatus string, serverName string) {
	if panel.ContainerServerID == nil || panel.ContainerName == nil || *panel.ContainerName == "" {
		return false, "", ""
	}
	if s.mikrotikAdaptor == nil {
		return false, "", ""
	}

	var server model.Server
	if err := s.db.First(&server, *panel.ContainerServerID).Error; err != nil {
		s.logger.Warn("failed to resolve container mapping's server", zap.Uint("panel_id", panel.ID), zap.Error(err))
		return false, "", ""
	}

	ctx, cancel := context.WithTimeout(context.Background(), 15*time.Second)
	defer cancel()

	container, err := s.mikrotikAdaptor.FetchContainerByName(ctx, server.Name, *panel.ContainerName)
	if err != nil {
		s.logger.Warn("failed to fetch container status for panel health diagnosis",
			zap.Uint("panel_id", panel.ID), zap.String("server", server.Name), zap.String("container", *panel.ContainerName), zap.Error(err))
		return false, "", ""
	}

	// RouterOS's own container status values -- "running" is the only
	// healthy state; everything else ("stopped", "stopping", "error", …)
	// is reported as a stopped-container diagnosis, since the point here
	// is "is this container the reason x-ui is unreachable," not an
	// exhaustive state machine.
	if container.Status == "running" {
		return false, container.Status, server.Name
	}
	return true, container.Status, server.Name
}

func (s *V2RayPanelHealthService) notifyDown(panel model.XuiPanel, loginErr error) {
	if s.botNotifier == nil || s.settings == nil {
		return
	}
	settings, err := s.settings.GetOrCreate()
	if err != nil || settings.AdminChatID == "" {
		return
	}
	s.botNotifier.NotifyV2RayPanelDown(settings, panel.Name, loginErr.Error())
}

// notifyContainerStopped is diagnoseContainer's own success-path alert --
// kept separate from notifyDown (rather than a shared method with a
// conditional message) so each stays a simple, direct mapping to its own
// BotNotifier method, matching this file's existing style.
func (s *V2RayPanelHealthService) notifyContainerStopped(panel model.XuiPanel, serverName, containerStatus string) {
	if s.botNotifier == nil || s.settings == nil {
		return
	}
	settings, err := s.settings.GetOrCreate()
	if err != nil || settings.AdminChatID == "" {
		return
	}
	s.botNotifier.NotifyV2RayPanelContainerStopped(settings, panel.Name, serverName, *panel.ContainerName, containerStatus)
}

// attemptRestart is a documented placeholder for a future remediation
// step (spec section ب-7's own "if down, start it" for V2Ray/container)
// -- NOT callable yet, since no x-ui API endpoint for restarting the
// underlying service/container has been investigated or confirmed safe.
// Kept as a named, empty method (rather than omitted entirely) so the
// admin's own explicit "leave room for this" request is visible directly
// in the code, not just in a commit message.
func (s *V2RayPanelHealthService) attemptRestart(panel model.XuiPanel) error {
	return nil
}
