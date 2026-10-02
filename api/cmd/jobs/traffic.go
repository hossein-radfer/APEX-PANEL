package traffic

import (
	"context"
	"errors"
	"fmt"
	"strconv"
	"strings"
	"sync"
	"time"

	"github.com/maahdima/mwp/api/adaptor/mikrotik"
	"github.com/maahdima/mwp/api/dataservice/model"
	"github.com/maahdima/mwp/api/utils"

	"go.uber.org/zap"
	"gorm.io/gorm"
)

type PeerUsageNotifier interface {
	NotifyPeerUsage(ctx context.Context, peerName, telegramUsername string, percent int64, totalUsage, limit int64) error
}

// ResellerQuotaNotifier is the narrow capability this package needs from
// service.BotNotifier, mirroring how PeerUsageNotifier decouples this
// package from *service.TelegramNotifier -- cmd/jobs never imports the
// service package directly, main.go satisfies this interface with the
// concrete type at wiring time.
type ResellerQuotaNotifier interface {
	NotifyQuotaWarning(reseller model.Reseller, percentRemaining int)
}

// CriticalAlertNotifier is the narrow capability this package needs for
// outage alerts, satisfied by *service.BotNotifier at wiring time.
type CriticalAlertNotifier interface {
	NotifyCriticalAlert(message string)
}

// UsageRecorder is the narrow capability this package needs to feed the
// Reports section's historical usage table, satisfied by
// *service.UsageSnapshotWriter at wiring time -- mirrors this file's own
// established pattern of a small interface per external capability
// (PeerUsageNotifier, ResellerQuotaNotifier, CriticalAlertNotifier) rather
// than cmd/jobs importing the service package directly.
type UsageRecorder interface {
	RecordWireGuard(peerID uint, resellerID *uint, deltaUpload, deltaDownload int64)
	RecordUserManager(accountID uint, resellerID *uint, deltaUpload, deltaDownload int64)
}

// ResellerBiller is the narrow capability this package needs from
// *service.ResellerBillingService, mirroring UsageRecorder's own "small
// interface per external capability" pattern -- ChargeUsage no-ops for any
// reseller not on Payment-based billing (see that method's own doc
// comment), so it's safe to call unconditionally from every usage-delta
// point below regardless of which billing mode any given reseller is on.
// A non-nil error means the charge was REJECTED (insufficient funds /
// debt ceiling exceeded) -- the caller must then disable that reseller's
// configs for the product in question, exactly like exceeding a
// byte-quota already does.
type ResellerBiller interface {
	ChargeUsage(resellerID uint, product string, locationKey string, deltaBytes int64) error
}

// MikrotikAdaptor defines the subset of methods from the real adaptor used by the
// traffic calculator. Using an interface allows tests to inject a fake adaptor.
type MikrotikAdaptor interface {
	FetchWgPeer(ctx context.Context, peerID string) (*mikrotik.WireGuardPeer, error)
	FetchWgPeers(ctx context.Context) ([]mikrotik.WireGuardPeer, error)
	FetchInterface(ctx context.Context, interfaceID string) (*mikrotik.Interface, error)
	UpdateWgPeer(ctx context.Context, peerID string, wgPeer mikrotik.WireGuardPeer) (*mikrotik.WireGuardPeer, error)
	UpdateScheduler(ctx context.Context, id string, s mikrotik.Scheduler) (*mikrotik.Scheduler, error)
	UpdateSimpleQueue(ctx context.Context, id string, q mikrotik.Queue) (*mikrotik.Queue, error)

	MonitorUserManagerUser(ctx context.Context, userID string) (*mikrotik.UserManagerMonitorResult, error)
	SetUserManagerUserDisabled(ctx context.Context, userID string, disabled string) (*mikrotik.UserManagerUser, error)
}

type Calculator struct {
	db              *gorm.DB
	mikrotikAdaptor MikrotikAdaptor
	// mu guards WireGuard peer state only -- CalculatePeerTraffic,
	// ResetPeerUsage(s), and ResetTotalTrafficUsage (the only three
	// methods that ever take it) all read/write model.Peer /
	// model.TotalTrafficUsage exclusively; none of them touch
	// model.UserManagerAccount. userManagerMu is the separate lock for
	// CalculateUserManagerUsage. A confirmed, reported production bug:
	// these two jobs used to share this SAME mutex despite having no
	// data in common, so on a fleet with ~1400 WireGuard peers AND ~3000
	// User Manager accounts, whichever scheduled tick grabbed the lock
	// first (they run on the same TRAFFIC_JOB_INTERVAL) blocked the other
	// job from EVER starting for its own full multi-minute pass --
	// journalctl showed CalculatePeerTraffic actively running while
	// "User manager usage calculation job completed" never once appeared
	// in 25+ minutes. Splitting the lock lets both jobs' scheduled ticks
	// actually run concurrently, exactly as gocron's own separate
	// interval registrations for each already intended.
	mu                    *sync.Mutex
	userManagerMu         *sync.Mutex
	logger                *zap.Logger
	notifier              PeerUsageNotifier
	quotaNotifier         ResellerQuotaNotifier
	criticalAlertNotifier CriticalAlertNotifier
	usageRecorder         UsageRecorder
	billing               ResellerBiller
	// wasMikrotikReachable tracks the last known connectivity state so the
	// critical alert fires once on the disconnect transition, not on every
	// single tick while still down (which would spam the admin every
	// TRAFFIC_JOB_INTERVAL seconds for the whole duration of an outage).
	wasMikrotikReachable bool
	// resellerQuotaMu serializes any of this Calculator's read-then-write
	// sequences against the resellers table PER RESELLER -- confirmed,
	// reported production incident this fixes: with
	// userManagerUsagePollConcurrency=20 goroutines processing accounts
	// concurrently, every account belonging to the SAME reseller triggered
	// its own SELECT SUM(...) + UPDATE resellers SET user_manager_used_bytes
	// pair with no coordination between them, so a reseller with many
	// accounts had up to 20 goroutines racing to read-modify-write the
	// exact same reseller row simultaneously. SQLite allows only one
	// writer at a time regardless of busy_timeout, so this manifested as a
	// sustained, non-bursty stream of "database is locked (SQLITE_BUSY)"
	// errors (tens per minute, continuously) -- busy_timeout(5000) alone
	// could not absorb it because the contention was structural (N
	// goroutines genuinely racing for the same row every tick), not an
	// occasional collision. A per-reseller mutex here makes those N
	// updates queue up cheaply in Go instead of colliding expensively in
	// SQLite -- correctness is unchanged (the SUM is still always
	// recomputed fresh under the lock), only the ordering is now
	// deterministic instead of a free-for-all. applyResellerQuota (the
	// WireGuard usage path) shares this SAME lock -- see its own call site's
	// doc comment -- because it writes a different column of the identical
	// resellers row and was found still colliding with
	// applyUserManagerResellerQuota in production after the original fix
	// only covered the User Manager side.
	resellerQuotaMu   map[uint]*sync.Mutex
	resellerQuotaMuMu sync.Mutex
}

func NewTrafficCalculator(db *gorm.DB, mikrotikAdaptor MikrotikAdaptor, notifier PeerUsageNotifier) *Calculator {
	return &Calculator{
		db:                   db,
		mikrotikAdaptor:      mikrotikAdaptor,
		mu:                   &sync.Mutex{},
		userManagerMu:        &sync.Mutex{},
		logger:               zap.L().Named("TrafficCalculatorJob"),
		notifier:             notifier,
		wasMikrotikReachable: true,
		resellerQuotaMu:      make(map[uint]*sync.Mutex),
	}
}

// lockForReseller returns the (lazily created) mutex serializing
// applyUserManagerResellerQuota/applyResellerQuota calls for one specific
// reseller ID -- see resellerQuotaMu's own doc comment for why this exists.
// resellerQuotaMuMu itself is only ever held for the brief map lookup/
// insert, never across the actual DB work. The map itself is also
// lazily created here (not just in NewTrafficCalculator) so a Calculator
// built directly as a struct literal -- as several existing tests do,
// predating this map's introduction -- doesn't panic on a nil map write.
func (c *Calculator) lockForReseller(resellerID uint) *sync.Mutex {
	c.resellerQuotaMuMu.Lock()
	defer c.resellerQuotaMuMu.Unlock()
	if c.resellerQuotaMu == nil {
		c.resellerQuotaMu = make(map[uint]*sync.Mutex)
	}
	lock, ok := c.resellerQuotaMu[resellerID]
	if !ok {
		lock = &sync.Mutex{}
		c.resellerQuotaMu[resellerID] = lock
	}
	return lock
}

// SetQuotaNotifier wires the Telegram quota-warning notifier after
// construction, since it depends on the bot service which is itself
// constructed after the traffic calculator during startup (see main.go).
// Safe to leave unset -- applyResellerQuota no-ops the warning if nil.
func (c *Calculator) SetQuotaNotifier(notifier ResellerQuotaNotifier) {
	c.quotaNotifier = notifier
}

// SetCriticalAlertNotifier wires the Telegram outage-alert notifier after
// construction, for the same reason as SetQuotaNotifier. Safe to leave
// unset -- the connectivity check simply no-ops the alert if nil.
func (c *Calculator) SetCriticalAlertNotifier(notifier CriticalAlertNotifier) {
	c.criticalAlertNotifier = notifier
}

// SetUsageRecorder wires the Reports section's usage-history writer after
// construction, for the same reason as SetQuotaNotifier. Safe to leave
// unset -- both call sites below simply skip recording a snapshot if nil.
func (c *Calculator) SetUsageRecorder(recorder UsageRecorder) {
	c.usageRecorder = recorder
}

// SetResellerBiller wires the Payment-based billing engine after
// construction, for the same reason as SetQuotaNotifier. Safe to leave
// unset -- every call site below simply skips the payment-based charge
// (Volume-based enforcement is entirely unaffected either way) if nil.
func (c *Calculator) SetResellerBiller(biller ResellerBiller) {
	c.billing = biller
}

func (c *Calculator) CalculatePeerTraffic() {
	c.mu.Lock()
	defer c.mu.Unlock()

	c.checkConnectivity()

	peers, err := c.fetchPeers()
	if err != nil {
		return
	}
	if len(peers) == 0 {
		c.logger.Info("No peers found, skipping traffic calculation")
		return
	}

	const maxCounter = 4294967296 // mikrotik 32-bit counter bug in wg peers (2^32)

	for _, peer := range peers {
		c.processPeerTraffic(peer, maxCounter)
	}

	c.logger.Info("Peer Traffic calculation job completed")
}

func (c *Calculator) CalculateDailyTraffic() {
	c.mu.Lock()
	defer c.mu.Unlock()

	var interfaces []model.Interface
	if err := c.db.Find(&interfaces).Error; err != nil {
		c.logger.Error("Failed to fetch interfaces from database", zap.Error(err))
		return
	}

	if len(interfaces) == 0 {
		c.logger.Info("No interfaces found, skipping daily traffic calculation")
		return
	}

	for _, iface := range interfaces {
		wgInterface, err := c.mikrotikAdaptor.FetchInterface(context.Background(), iface.InterfaceID)
		if err != nil {
			c.logger.Error("Failed to fetch WireGuard interface", zap.String("interfaceID", iface.InterfaceID), zap.Error(err))
			continue
		}

		currentDownload := utils.ParseStringToInt(wgInterface.TxByte)
		currentUpload := utils.ParseStringToInt(wgInterface.RxByte)
		currentTotal := currentDownload + currentUpload

		var lastTraffic model.Traffic
		err = c.db.
			Where("interface_id = ?", iface.ID).
			Order("created_at DESC").
			First(&lastTraffic).Error

		var diffDownload, diffUpload, diffTotal int64
		if err == nil {
			diffDownload = currentDownload - lastTraffic.DownloadUsage
			diffUpload = currentUpload - lastTraffic.UploadUsage
			diffTotal = currentTotal - lastTraffic.TotalUsage
		} else if errors.Is(err, gorm.ErrRecordNotFound) {
			diffDownload = currentDownload
			diffUpload = currentUpload
			diffTotal = currentTotal
		} else {
			c.logger.Error("Failed to fetch previous traffic record", zap.String("interfaceID", iface.InterfaceID), zap.Error(err))
			continue
		}

		newTraffic := model.Traffic{
			InterfaceID:   iface.ID,
			DownloadUsage: diffDownload,
			UploadUsage:   diffUpload,
			TotalUsage:    diffTotal,
		}

		if err := c.db.Create(&newTraffic).Error; err != nil {
			c.logger.Error("Failed to save daily traffic data", zap.String("interfaceID", iface.InterfaceID), zap.Error(err))
			continue
		}
	}

	c.logger.Info("Daily traffic calculation completed")
}

// CalculateUserManagerUsage is a fully separate polling pass for User
// Manager accounts (L2TP/PPTP/SSTP/OpenVPN) -- it never touches model.Peer,
// model.Reseller.UsedBytes/QuotaBytes/IsActive, or any WireGuard state.
// Exhausting a reseller's User Manager quota disables only that reseller's
// UserManagerAccount rows (see applyUserManagerResellerQuota below), per the
// explicit "separate quota pool" requirement.
// userManagerUsagePollConcurrency bounds how many accounts this job polls
// against RouterOS at once. A confirmed, reported production bug: at
// scale (this fleet already has ~3000 User Manager accounts), the
// original one-account-at-a-time loop took OVER 10 MINUTES per pass --
// several times longer than the default 120s TRAFFIC_JOB_INTERVAL -- so
// c.mu.Lock() above left every subsequent scheduled tick queued behind
// the still-running previous one indefinitely. Accounts created partway
// through an in-progress pass (appended at the end of the unordered
// query below) could easily wait 10+ minutes before ever being reached
// even once, which is exactly the "I added users and waited 10 minutes,
// still no usage shown" symptom this fixes. 20 was chosen as a
// deliberately conservative starting point -- enough to cut wall-clock
// time by roughly an order of magnitude without hammering RouterOS's own
// REST API with hundreds of simultaneous requests; each account's own DB
// update is already scoped to its own row by ID (see
// processUserManagerAccountUsage), so concurrent execution introduces no
// new write conflicts beyond what SQLite's busy_timeout already absorbs.
const userManagerUsagePollConcurrency = 20

func (c *Calculator) CalculateUserManagerUsage() {
	c.userManagerMu.Lock()
	defer c.userManagerMu.Unlock()

	var accounts []model.UserManagerAccount
	if err := c.db.Find(&accounts).Error; err != nil {
		c.logger.Error("Failed to fetch user manager accounts from database", zap.Error(err))
		return
	}
	if len(accounts) == 0 {
		c.logger.Info("No user manager accounts found, skipping usage calculation")
		return
	}

	// Confirmed, reported production incident this fixes: applyUserManagerResellerQuota
	// used to be called once PER ACCOUNT (below, inside processUserManagerAccountUsage),
	// even though it always recomputes the SAME reseller-wide SUM+UPDATE
	// regardless of which account triggered it -- see that function's own
	// doc comment. A reseller with 40 accounts therefore ran that identical
	// transaction 40 times per tick, purely duplicated work that multiplied
	// by userManagerUsagePollConcurrency=20 concurrent goroutines turned an
	// occasional SQLite lock collision into a sustained flood of
	// "database is locked (SQLITE_BUSY)" errors. Collecting each account's
	// billing delta here (keyed by reseller, with each reseller's own
	// per-group deltas kept separate since ChargeUsage prices different
	// RouterOS groups differently -- see its own locationKey doc comment)
	// and applying each distinct RESELLER exactly ONCE after every account
	// has been processed cuts that per-tick transaction count from "one per
	// account" to "one per reseller that has at least one account", while
	// every group's own delta is still billed separately within that one
	// call. touchedResellers additionally records every reseller with at
	// least one account regardless of delta, so applyUserManagerResellerQuota
	// still runs unconditionally for an all-idle reseller too -- see that
	// function's own doc comment on why the SUM+UPDATE must never skip a
	// reseller just because nothing changed this tick.
	var deltasMu sync.Mutex
	deltasByReseller := make(map[uint]map[string]int64)
	touchedResellers := make(map[uint]struct{})

	jobs := make(chan model.UserManagerAccount)
	var wg sync.WaitGroup
	for i := 0; i < userManagerUsagePollConcurrency; i++ {
		wg.Add(1)
		go func() {
			defer wg.Done()
			for account := range jobs {
				resellerID, group, delta := c.processUserManagerAccountUsage(account)
				if resellerID == nil {
					continue
				}
				deltasMu.Lock()
				touchedResellers[*resellerID] = struct{}{}
				if delta > 0 {
					if deltasByReseller[*resellerID] == nil {
						deltasByReseller[*resellerID] = make(map[string]int64)
					}
					deltasByReseller[*resellerID][group] += delta
				}
				deltasMu.Unlock()
			}
		}()
	}
	for _, account := range accounts {
		jobs <- account
	}
	close(jobs)
	wg.Wait()

	for resellerID := range touchedResellers {
		resellerID := resellerID
		var groupDeltas []groupUsage
		for group, delta := range deltasByReseller[resellerID] {
			groupDeltas = append(groupDeltas, groupUsage{group: group, delta: delta})
		}
		c.applyUserManagerResellerQuota(&resellerID, groupDeltas)
	}

	c.logger.Info("User manager usage calculation job completed")
}

// processUserManagerAccountUsage sets DownloadUsage/UploadUsage to a
// DIRECT COPY of RouterOS's own cumulative counters every tick -- by
// design decision, this account's "usage" is always exactly what
// RouterOS itself reports right now, not a panel-accumulated running
// total. This deliberately diverges from CalculatePeerTraffic's
// delta-accumulation approach for WireGuard peers (see calculateDelta's
// other call site) -- User Manager's RouterOS-side counter IS the single
// source of truth here, so there is nothing to accumulate.
//
// LastTotalDownload/LastTotalUpload are still read every tick, but
// PURELY to detect a RouterOS-side counter reset (the account was
// re-added, or RouterOS zeroed it for any other reason) for logging/
// auditing -- a reset is never allowed to auto re-enable an account this
// job itself disabled for exceeding TrafficLimit (see the `!account.Disabled`
// gate below, unchanged from before): only an admin manually re-enabling
// the account, or the admin raising/removing TrafficLimit, can undo that.
//
// The reseller-level UserManagerQuotaBytes pool (applyUserManagerResellerQuota
// below) is now a LIVE SUM of this same per-account DisplayedUsage across
// the reseller's accounts, recomputed every tick -- see that function's
// own doc comment for the confirmed drift bug this replaces (an earlier
// delta-accumulated version of this pool never counted a bulk-imported
// pre-existing account's PRE-IMPORT usage, while the account's own
// DownloadUsage/UploadUsage always did, so the two diverged by
// terabytes for a reseller with many bulk-imported accounts).
// deltaDownload/deltaUpload below are still computed, but ONLY to detect
// a counter reset for the logging/auditing purpose described above --
// they no longer feed the reseller pool at all.
//
// Returns this account's ResellerID/Group/fresh-usage-delta for
// CalculateUserManagerUsage to batch across every account before calling
// applyUserManagerResellerQuota -- see that call site's own doc comment for
// why this is no longer called directly from here. A nil resellerID (on
// any early return below -- a monitor/parse failure, or a failed DB write)
// tells the caller this account contributed nothing usable this tick.
func (c *Calculator) processUserManagerAccountUsage(account model.UserManagerAccount) (resellerID *uint, group string, delta int64) {
	result, err := c.mikrotikAdaptor.MonitorUserManagerUser(context.Background(), account.RouterOSUserID)
	if err != nil {
		c.logger.Error("Failed to monitor user manager user", zap.String("username", account.Username), zap.Error(err))
		return nil, "", 0
	}

	currentDownload, err := utils.ParseRouterOSByteSize(result.TotalDownload)
	if err != nil {
		c.logger.Warn("Failed to parse user manager download usage, skipping this account this tick",
			zap.String("username", account.Username), zap.String("raw", result.TotalDownload), zap.Error(err))
		return nil, "", 0
	}
	currentUpload, err := utils.ParseRouterOSByteSize(result.TotalUpload)
	if err != nil {
		c.logger.Warn("Failed to parse user manager upload usage, skipping this account this tick",
			zap.String("username", account.Username), zap.String("raw", result.TotalUpload), zap.Error(err))
		return nil, "", 0
	}

	// Delta computed ONLY to feed the reseller-level quota pool below and
	// to detect/log a counter reset -- never written into this account's
	// own DownloadUsage/UploadUsage (those are set directly from
	// current* a few lines down instead).
	deltaDownload, resetDown := calculateDelta(account.LastTotalDownload, currentDownload, 0)
	deltaUpload, resetUp := calculateDelta(account.LastTotalUpload, currentUpload, 0)
	if resetDown || resetUp {
		c.logger.Info("Detected user manager counter reset -- recorded usage now mirrors RouterOS's post-reset total, does not re-enable a quota-disabled account",
			zap.String("username", account.Username),
			zap.Int64("prevDownload", account.LastTotalDownload),
			zap.Int64("currentDownload", currentDownload),
			zap.Int64("prevUpload", account.LastTotalUpload),
			zap.Int64("currentUpload", currentUpload),
		)
	}

	// DisplayedUsage = raw RouterOS total - UsageOffset (clamped at 0) --
	// see UsageOffsetDownload/UsageOffsetUpload's own doc comment on
	// model.UserManagerAccount for why this offset exists (RouterOS has
	// no reset-counter call) and why it must never affect
	// LastTotalDownload/LastTotalUpload or the reseller quota delta below,
	// both of which stay computed from the untouched raw currentDownload/
	// currentUpload exactly as before.
	account.DownloadUsage = currentDownload - account.UsageOffsetDownload
	if account.DownloadUsage < 0 {
		account.DownloadUsage = 0
	}
	account.UploadUsage = currentUpload - account.UsageOffsetUpload
	if account.UploadUsage < 0 {
		account.UploadUsage = 0
	}
	account.LastTotalDownload = currentDownload
	account.LastTotalUpload = currentUpload

	updates := map[string]interface{}{
		"download_usage":      account.DownloadUsage,
		"upload_usage":        account.UploadUsage,
		"last_total_download": account.LastTotalDownload,
		"last_total_upload":   account.LastTotalUpload,
	}

	// Gated on !account.Disabled so this job never re-evaluates (or
	// re-disables/re-logs) an account it already disabled -- this is also
	// what stops a counter reset (usage now reading low) from having any
	// effect on an already-quota-disabled account; only a manual re-enable
	// (or, per the fix for "ریست حجم کار نمی‌کند", a usage reset -- see
	// UserManagerService.ResetUsage) clears Disabled, at which point this
	// job resumes normal evaluation. suspended_by_traffic_limit (NOT
	// suspended_by_quota -- see SuspendedByTrafficLimit's own doc comment
	// on model.UserManagerAccount for the cross-contamination bug this
	// fixes) is what lets that reset path tell "disabled because of THIS
	// account's own limit" apart from "an admin disabled this manually".
	if account.TrafficLimit != nil && (account.DownloadUsage+account.UploadUsage) > *account.TrafficLimit && !account.Disabled {
		c.logger.Warn("User manager account traffic limit exceeded", zap.String("username", account.Username))
		updates["disabled"] = true
		updates["suspended_by_traffic_limit"] = true
		account.Disabled = true

		if _, err := c.mikrotikAdaptor.SetUserManagerUserDisabled(context.Background(), account.RouterOSUserID, "true"); err != nil {
			c.logger.Error("Failed to disable user manager account on Mikrotik", zap.String("username", account.Username), zap.Error(err))
		}
	}

	if err := c.db.Model(&model.UserManagerAccount{}).Where("id = ?", account.ID).Updates(updates).Error; err != nil {
		c.logger.Error("Failed to update user manager account usage in database", zap.String("username", account.Username), zap.Error(err))
		return nil, "", 0
	}

	if c.usageRecorder != nil {
		c.usageRecorder.RecordUserManager(account.ID, account.ResellerID, deltaUpload, deltaDownload)
	}

	// deltaDownload+deltaUpload (this account's fresh usage THIS tick only,
	// not the live-summed total) is what Payment-based billing actually
	// charges for -- returned to the caller rather than applied here
	// directly; see this function's own doc comment.
	return account.ResellerID, account.Group, deltaDownload + deltaUpload
}

// applyUserManagerResellerQuota mirrors applyResellerQuota's structure
// but operates on the separate UserManagerQuotaBytes/UserManagerUsedBytes/
// UserManagerMaxAccounts pool, and on quota exhaustion disables only that
// reseller's UserManagerAccount rows -- it never sets Reseller.IsActive=
// false and never touches model.Peer.
//
// UserManagerUsedBytes is now a LIVE SUM of the reseller's own accounts'
// DownloadUsage+UploadUsage (sumUserManagerUsage below), recomputed and
// re-persisted every call, rather than an incrementally-accumulated
// running total. A confirmed, reported bug with the old delta-accumulation
// design: BulkImportAccounts' own seedUsageBaseline seeds a newly-imported
// pre-existing RouterOS user's LastTotalDownload/LastTotalUpload to
// whatever RouterOS already shows at import time (a deliberate "start
// counting fresh from here" import behavior) -- but the account's OWN
// displayed DownloadUsage/UploadUsage (used everywhere else, including
// this same account's row in the reseller's own account list) is the
// FULL raw RouterOS total minus UsageOffset, not relative to that import
// baseline. The delta pool above only ever counted usage accrued AFTER
// import, while every other reader of "this account's usage" counted the
// full all-time total -- for a reseller bulk-importing many pre-existing
// accounts with real prior usage, this reportedly diverged from the true
// sum by terabytes. A live sum is definitionally always consistent with
// what the account list itself shows, with no separate accumulator state
// that can drift.
// groupUsage is one (RouterOS group, delta bytes) pair accrued for a
// reseller since applyUserManagerResellerQuota's last call -- see this
// function's own call site in CalculateUserManagerUsage for why billing is
// batched per-group (ChargeUsage prices different RouterOS groups
// differently -- see its own locationKey doc comment) while the SUM+UPDATE
// below stays a single per-reseller pass regardless of how many groups are
// present.
type groupUsage struct {
	group string
	delta int64
}

func (c *Calculator) applyUserManagerResellerQuota(resellerID *uint, groupDeltas []groupUsage) {
	if resellerID == nil {
		return
	}

	// See resellerQuotaMu's own doc comment -- serializes the
	// read-sum-then-write sequence below across every concurrent
	// goroutine touching THIS SAME reseller, without blocking goroutines
	// working on other resellers. ChargeUsage is now called INSIDE this
	// lock (it used to run before acquiring it) -- confirmed, reported
	// production incident this fixes: with
	// userManagerUsagePollConcurrency=20 goroutines each calling this once
	// PER ACCOUNT, a reseller with many accounts had that many concurrent,
	// completely unserialized Wallet.recordTransaction calls racing each
	// other's own SQLite transaction on the SAME wallet row every tick --
	// the per-reseller mutex below only ever protected the SUM+UPDATE
	// further down, never this billing call. Also confirmed, reported: this
	// function used to be called once PER ACCOUNT even though the SUM+UPDATE
	// below always recomputes the exact same reseller-wide total regardless
	// of which account triggered it -- CalculateUserManagerUsage now batches
	// every account's delta by (reseller, group) first and calls this once
	// per reseller per tick instead, cutting a 40-account reseller's
	// per-tick transaction count from 40 down to 1.
	lock := c.lockForReseller(*resellerID)
	lock.Lock()
	defer lock.Unlock()

	if c.billing != nil {
		for _, gd := range groupDeltas {
			if gd.delta <= 0 {
				continue
			}
			if err := c.billing.ChargeUsage(*resellerID, "USER_MANAGER", gd.group, gd.delta); err != nil {
				c.logger.Warn("charge usage failed for reseller", zap.Uint("reseller_id", *resellerID), zap.Error(err))
			}
		}
	}

	_ = c.db.Transaction(func(tx *gorm.DB) error {
		var reseller model.Reseller
		if err := tx.Where("id = ?", *resellerID).First(&reseller).Error; err != nil {
			c.logger.Error("failed to fetch reseller for user manager quota", zap.Uint("reseller_id", *resellerID), zap.Error(err))
			return nil
		}

		var summedUsage int64
		if err := tx.Model(&model.UserManagerAccount{}).
			Where("reseller_id = ?", reseller.ID).
			Select("COALESCE(SUM(download_usage + upload_usage), 0)").Scan(&summedUsage).Error; err != nil {
			c.logger.Error("failed to sum reseller user manager usage", zap.Uint("reseller_id", reseller.ID), zap.Error(err))
			return nil
		}
		// + UserManagerDeletedUsageBytes: a confirmed, reported bug's fix --
		// see that field's own doc comment. Without this, deleting an
		// account would make this live sum (and therefore the reseller's
		// billable total) silently shrink.
		reseller.UserManagerUsedBytes = summedUsage + reseller.UserManagerDeletedUsageBytes

		if err := tx.Model(&model.Reseller{}).Where("id = ?", reseller.ID).Update("user_manager_used_bytes", reseller.UserManagerUsedBytes).Error; err != nil {
			c.logger.Error("failed to update reseller user manager usage", zap.Uint("reseller_id", reseller.ID), zap.Error(err))
			return err
		}

		// Same billing-mode guard as applyResellerQuota's own identical fix
		// above -- a Payment-based reseller's leftover UserManagerQuotaBytes
		// (kept, not cleared, on purpose so switching back to Volume-based
		// needs no migration) must never itself trigger disablement; only a
		// genuine ChargeUsage financial rejection may disable a Payment
		// reseller. Reading reseller.BillingSuspended (fetched fresh just
		// above, after ChargeUsage already ran under this same lock)
		// instead of a local bool set from ChargeUsage returning any
		// non-nil error avoids wrongly force-disabling a funded reseller
		// over a transient, unrelated database error.
		if reseller.BillingMode == model.ResellerBillingModePayment {
			if !reseller.BillingSuspended {
				return nil
			}
		} else if reseller.UserManagerQuotaBytes == nil || reseller.UserManagerUsedBytes <= *reseller.UserManagerQuotaBytes {
			return nil
		}

		var accounts []model.UserManagerAccount
		if err := tx.Where("reseller_id = ?", reseller.ID).Find(&accounts).Error; err != nil {
			c.logger.Error("failed to fetch reseller user manager accounts", zap.Uint("reseller_id", reseller.ID), zap.Error(err))
			return nil
		}

		for _, account := range accounts {
			if c.mikrotikAdaptor != nil {
				if _, err := c.mikrotikAdaptor.SetUserManagerUserDisabled(context.Background(), account.RouterOSUserID, "true"); err != nil {
					c.logger.Error("failed to disable user manager account on Mikrotik", zap.String("username", account.Username), zap.Error(err))
				}
			}

			updates := map[string]interface{}{
				"disabled":           true,
				"suspended_by_quota": true,
			}
			if !account.Disabled {
				updates["was_active_before_suspend"] = true
			}

			if err := tx.Model(&model.UserManagerAccount{}).Where("id = ?", account.ID).Updates(updates).Error; err != nil {
				c.logger.Error("failed to disable user manager account in DB", zap.String("username", account.Username), zap.Error(err))
			}
		}

		return nil
	})
}

// ResetPeerUsage zeroes one peer's displayed usage -- and, per the
// confirmed reported bug "ریست حجم کار نمی‌کند" (usage reset doesn't
// work), also clears a TrafficLimit-triggered disable: previously this
// only ever zeroed the usage counters, so a peer applyPeerTrafficLimit
// had already disabled for exceeding its old limit stayed disabled on
// RouterOS forever, with usage now reading 0 but the peer still unable
// to pass traffic -- indistinguishable from "the reset didn't do
// anything" from the admin's point of view. Only re-enables when
// SuspendedByTrafficLimit is true (this job's own doing); a peer an
// admin disabled manually for an unrelated reason is left exactly as
// they set it.
func (c *Calculator) ResetPeerUsage(id uint) error {
	c.mu.Lock()
	defer c.mu.Unlock()

	var peer model.Peer
	if err := c.db.Where("id = ?", id).First(&peer).Error; err != nil {
		c.logger.Error("Failed to find peer in DB", zap.Uint("id", id), zap.Error(err))
		return err
	}

	wgPeer, err := c.mikrotikAdaptor.FetchWgPeer(context.Background(), peer.PeerID)
	if err != nil {
		c.logger.Error("Failed to fetch peer from Mikrotik", zap.String("peerID", peer.PeerID), zap.Error(err))
		return err
	}

	currentTx := utils.ParseStringToInt(wgPeer.TransferTx)
	currentRx := utils.ParseStringToInt(wgPeer.TransferRx)

	wasSuspendedByTrafficLimit := peer.SuspendedByTrafficLimit
	if wasSuspendedByTrafficLimit {
		if _, err := c.mikrotikAdaptor.UpdateWgPeer(context.Background(), peer.PeerID, mikrotik.WireGuardPeer{
			Disabled: strconv.FormatBool(false),
		}); err != nil {
			c.logger.Error("Failed to re-enable peer on Mikrotik after usage reset", zap.String("peerID", peer.PeerID), zap.Error(err))
			return err
		}
	}

	err = c.db.Transaction(func(tx *gorm.DB) error {
		peer.DownloadUsage = 0
		peer.UploadUsage = 0
		peer.LastTx = currentTx
		peer.LastRx = currentRx
		peer.FirstNotify = false
		peer.SecondNotify = false
		peer.ThirdNotify = false
		if wasSuspendedByTrafficLimit {
			peer.Disabled = false
			peer.SuspendedByTrafficLimit = false
		}

		if err := tx.Save(&peer).Error; err != nil {
			return err
		}

		return nil
	})

	if err != nil {
		c.logger.Error("Failed to reset peer usage", zap.String("peerID", peer.PeerID), zap.Error(err))
		return err
	}

	c.logger.Info("Peer usage reset successfully", zap.String("peerID", peer.PeerID), zap.Bool("reenabled", wasSuspendedByTrafficLimit))

	return nil
}

func (c *Calculator) ResetPeerUsages() error {
	c.mu.Lock()
	defer c.mu.Unlock()

	var peers []model.Peer
	if err := c.db.Find(&peers).Error; err != nil {
		c.logger.Error("Failed to find peers in DB", zap.Error(err))
		return err
	}

	wgPeers, err := c.mikrotikAdaptor.FetchWgPeers(context.Background())
	if err != nil {
		c.logger.Error("Failed to fetch peers from Mikrotik", zap.Error(err))
		return err
	}

	wgPeerMap := make(map[string]mikrotik.WireGuardPeer)
	for _, wgPeer := range wgPeers {
		wgPeerMap[wgPeer.ID] = wgPeer
	}

	for _, peer := range peers {
		wgPeer, found := wgPeerMap[peer.PeerID]
		if !found {
			continue
		}

		currentTx := utils.ParseStringToInt(wgPeer.TransferTx)
		currentRx := utils.ParseStringToInt(wgPeer.TransferRx)

		wasSuspendedByTrafficLimit := peer.SuspendedByTrafficLimit
		if wasSuspendedByTrafficLimit {
			if _, err := c.mikrotikAdaptor.UpdateWgPeer(context.Background(), peer.PeerID, mikrotik.WireGuardPeer{
				Disabled: strconv.FormatBool(false),
			}); err != nil {
				c.logger.Error("Failed to re-enable peer on Mikrotik after bulk usage reset", zap.String("peerID", peer.PeerID), zap.Error(err))
				continue
			}
		}

		peer.DownloadUsage = 0
		peer.UploadUsage = 0
		peer.LastTx = currentTx
		peer.LastRx = currentRx
		peer.FirstNotify = false
		peer.SecondNotify = false
		peer.ThirdNotify = false
		if wasSuspendedByTrafficLimit {
			peer.Disabled = false
			peer.SuspendedByTrafficLimit = false
		}

		if err := c.db.Save(&peer).Error; err != nil {
			c.logger.Error("Failed to reset peer usage", zap.String("peerID", peer.PeerID), zap.Error(err))
			return err
		}
	}

	c.logger.Info("Peer usages reset successfully")
	return nil
}

// checkConnectivity probes Mikrotik and database reachability once per
// traffic-job tick and fires a CRITICAL alert exactly once on the
// transition into an outage (not on every tick for the outage's whole
// duration), and once more when service recovers. FetchWgPeers is used
// purely as a connectivity probe here -- its result isn't otherwise needed
// in this method, fetchPeers() below gets the DB-side peer records
// separately.
func (c *Calculator) checkConnectivity() {
	if c.criticalAlertNotifier == nil {
		return
	}

	_, mikrotikErr := c.mikrotikAdaptor.FetchWgPeers(context.Background())
	dbErr := c.db.Exec("SELECT 1").Error

	reachable := mikrotikErr == nil && dbErr == nil

	if reachable == c.wasMikrotikReachable {
		return
	}
	c.wasMikrotikReachable = reachable

	if reachable {
		c.criticalAlertNotifier.NotifyCriticalAlert("اتصال برقرار شد: میکروتیک و پایگاه‌داده دوباره در دسترس هستند.")
		return
	}

	switch {
	case mikrotikErr != nil && dbErr != nil:
		c.criticalAlertNotifier.NotifyCriticalAlert(fmt.Sprintf("اتصال به میکروتیک و پایگاه‌داده هر دو قطع شد.\nمیکروتیک: %v\nپایگاه‌داده: %v", mikrotikErr, dbErr))
	case mikrotikErr != nil:
		c.criticalAlertNotifier.NotifyCriticalAlert(fmt.Sprintf("اتصال به میکروتیک قطع شد: %v", mikrotikErr))
	default:
		c.criticalAlertNotifier.NotifyCriticalAlert(fmt.Sprintf("اتصال به پایگاه‌داده قطع شد: %v", dbErr))
	}
}

func (c *Calculator) fetchPeers() ([]model.Peer, error) {
	var peers []model.Peer
	if err := c.db.Find(&peers).Error; err != nil {
		c.logger.Error("Failed to fetch peers from database", zap.Error(err))
		return nil, err
	}
	return peers, nil
}

func (c *Calculator) processPeerTraffic(peer model.Peer, maxCounter int64) {
	wgPeer, err := c.mikrotikAdaptor.FetchWgPeer(context.Background(), peer.PeerID)
	if err != nil {
		// A confirmed, reported log-flood incident this fixes: a peer row
		// that still exists in this panel's own DB but was deleted directly
		// on RouterOS (outside the panel) 404s here on EVERY single tick of
		// this job, forever -- there is nothing transient about it, and
		// nothing here ever removes or skips such a row. Logged at Warn
		// (not Error) specifically for the 404 case, since this is a data-
		// integrity mismatch to eventually clean up, not an operational
		// failure needing on-call attention every 10-30 seconds. Any OTHER
		// failure (network, auth, RouterOS unreachable) stays at Error,
		// since those genuinely indicate something an admin should act on.
		if strings.Contains(err.Error(), "status code 404") {
			c.logger.Warn("wireguard peer no longer exists on RouterOS (deleted outside the panel?) -- skipping this tick", zap.String("peerID", peer.PeerID), zap.Uint("peerRowID", peer.ID))
		} else {
			c.logger.Error("Failed to fetch wireguard peer", zap.String("peerID", peer.PeerID), zap.Error(err))
		}
		return
	}

	currentTx := utils.ParseStringToInt(wgPeer.TransferTx)
	currentRx := utils.ParseStringToInt(wgPeer.TransferRx)

	deltaTx, deltaRx, resetDetected := c.calculatePeerDeltas(peer, currentTx, currentRx, maxCounter)
	if delta := deltaTx + deltaRx; delta > 0 {
		c.accumulateTotalTraffic(delta)
		// update reseller usage and enforce quota per reseller
		c.applyResellerQuota(peer.ResellerID, peer.Interface, delta)
		c.accumulatePeerDailyUsage(peer.ID, peer.ResellerID, deltaTx, deltaRx)
	}
	if resetDetected {
		c.logger.Debug("Detected peer counter reset",
			zap.String("peerID", peer.PeerID),
			zap.Int64("prevTx", peer.LastTx),
			zap.Int64("currentTx", currentTx),
			zap.Int64("prevRx", peer.LastRx),
			zap.Int64("currentRx", currentRx),
		)
	}

	peer.DownloadUsage += deltaTx
	peer.UploadUsage += deltaRx
	peer.LastTx = currentTx
	peer.LastRx = currentRx

	updates := map[string]interface{}{
		"download_usage": peer.DownloadUsage,
		"upload_usage":   peer.UploadUsage,
		"last_tx":        peer.LastTx,
		"last_rx":        peer.LastRx,
	}

	c.applyPeerTrafficNotifications(&peer, updates)
	c.applyPeerTrafficLimit(&peer, updates)
	c.persistPeerTraffic(peer, updates)

}

func (c *Calculator) applyResellerQuota(resellerID *uint, interfaceName string, delta int64) {
	if resellerID == nil || delta <= 0 {
		return
	}

	// Confirmed, reported production incident this fixes: this function and
	// applyUserManagerResellerQuota write different columns
	// (used_bytes vs. user_manager_used_bytes) of the SAME resellers row, but
	// SQLite still serializes at the row/table level for writers -- so a
	// reseller with both WireGuard peers and User Manager accounts had this
	// function's UPDATE collide with the other job's concurrent UPDATE for
	// the same reseller, flooding the log with "database is locked
	// (SQLITE_BUSY)" every time the two jobs' ticks overlapped (they run as
	// separate long-running gocron jobs, so overlap is frequent, not rare --
	// see dataservice/db.go's own busy_timeout doc comment). Sharing
	// resellerQuotaMu (see its own doc comment) here makes the two paths
	// queue up cheaply in Go instead of colliding in SQLite, exactly like it
	// already does between concurrent applyUserManagerResellerQuota calls.
	lock := c.lockForReseller(*resellerID)
	lock.Lock()
	defer lock.Unlock()

	// Payment-based billing runs OUTSIDE the transaction below (it has its
	// own internal transaction via Wallet.recordTransaction, and reads/
	// writes model.Reseller itself) -- a no-op for any reseller that isn't
	// BillingMode=PAYMENT (see ChargeUsage's own doc comment), so this is
	// safe to call unconditionally before the byte-quota bookkeeping below,
	// which still runs regardless of billing mode (see
	// ResellerBillingService's own top-level doc comment on why both paths
	// always execute). interfaceName is this peer's own WireGuard
	// interface -- the "location" a reseller may be priced differently
	// for (see ChargeUsage's own doc comment on locationKey).
	if c.billing != nil {
		if err := c.billing.ChargeUsage(*resellerID, "WIREGUARD", interfaceName, delta); err != nil {
			c.logger.Warn("charge usage failed for reseller", zap.Uint("reseller_id", *resellerID), zap.Error(err))
		}
	}

	// run in transaction
	_ = c.db.Transaction(func(tx *gorm.DB) error {
		var reseller model.Reseller
		if err := tx.Where("id = ?", *resellerID).First(&reseller).Error; err != nil {
			c.logger.Error("failed to fetch reseller", zap.Uint("reseller_id", *resellerID), zap.Error(err))
			return nil
		}

		reseller.UsedBytes += delta

		if err := tx.Model(&model.Reseller{}).Where("id = ?", reseller.ID).Update("used_bytes", reseller.UsedBytes).Error; err != nil {
			c.logger.Error("failed to update reseller usage", zap.Uint("reseller_id", reseller.ID), zap.Error(err))
			return err
		}

		// Reading reseller.BillingSuspended fresh from this same row fetch
		// (just above, which happens after ChargeUsage already wrote that
		// flag synchronously when it suspends) reflects only a genuine
		// financial rejection ChargeUsage itself confirmed, never a
		// transient, unrelated database error it simply propagated without
		// suspending.
		if reseller.BillingSuspended {
			return c.disablePeersForBillingSuspension(tx, reseller.ID)
		}

		// Warn once when remaining quota drops under 10% -- before the
		// reseller actually runs out and their peers get force-disabled
		// below, so the admin/reseller have a chance to act first. Only
		// fires once per low-quota period (QuotaWarningSent), and only for
		// resellers with a finite quota (an unlimited reseller has no
		// "remaining" to warn about).
		if c.quotaNotifier != nil && reseller.QuotaBytes != nil && *reseller.QuotaBytes > 0 && !reseller.QuotaWarningSent {
			remaining := *reseller.QuotaBytes - reseller.UsedBytes
			percentRemaining := int(remaining * 100 / *reseller.QuotaBytes)
			if percentRemaining <= 10 {
				if err := tx.Model(&model.Reseller{}).Where("id = ?", reseller.ID).Update("quota_warning_sent", true).Error; err != nil {
					c.logger.Error("failed to mark quota warning as sent", zap.Uint("reseller_id", reseller.ID), zap.Error(err))
				} else {
					reseller.QuotaWarningSent = true
					c.quotaNotifier.NotifyQuotaWarning(reseller, percentRemaining)
				}
			}
		}

		// Confirmed bug (billing-mode bidirectional switching, category 2
		// item 1): this byte-quota check ran unconditionally regardless of
		// BillingMode, so a reseller switched from Volume-based to
		// Payment-based -- whose QuotaBytes is documented as "simply
		// unused/ignored" (see model.Reseller.BillingMode's own doc
		// comment) but is deliberately left in place rather than cleared,
		// specifically so switching BACK to Volume-based needs no data
		// migration -- would still get hard-disabled the moment old usage
		// crossed that leftover quota number, even though they're now
		// billed per-GB from their wallet instead. Payment-mode resellers
		// are exclusively governed by ChargeUsage's billingRejected path
		// above; this branch must only ever fire for Volume-based ones.
		if reseller.BillingMode != model.ResellerBillingModePayment &&
			reseller.QuotaBytes != nil && reseller.UsedBytes > *reseller.QuotaBytes {
			// disable reseller and all its peers
			if err := tx.Model(&model.Reseller{}).Where("id = ?", reseller.ID).Update("is_active", false).Error; err != nil {
				c.logger.Error("failed to disable reseller in DB", zap.Uint("reseller_id", reseller.ID), zap.Error(err))
			}

			return c.disablePeersForBillingSuspension(tx, reseller.ID)
		}

		return nil
	})
}

// disablePeersForBillingSuspension force-disables every not-already-disabled
// peer belonging to resellerID and marks it suspended_by_quota (reusing the
// exact same flag/resume mechanism as byte-quota exhaustion --
// resumeQuotaSuspendedPeers doesn't care WHICH enforcement path set it,
// only that it's set). Shared by both the byte-quota-exceeded branch above
// and the Payment-based billing-rejected branch in applyResellerQuota,
// since disabling a reseller's peers is identical work regardless of WHY
// they're being disabled.
func (c *Calculator) disablePeersForBillingSuspension(tx *gorm.DB, resellerID uint) error {
	var peers []model.Peer
	if err := tx.Where("reseller_id = ?", resellerID).Find(&peers).Error; err != nil {
		c.logger.Error("failed to fetch reseller peers", zap.Uint("reseller_id", resellerID), zap.Error(err))
		return nil
	}

	for _, p := range peers {
		if c.mikrotikAdaptor != nil {
			if _, err := c.mikrotikAdaptor.UpdateWgPeer(context.Background(), p.PeerID, mikrotik.WireGuardPeer{Disabled: strconv.FormatBool(true)}); err != nil {
				c.logger.Error("failed to disable peer on Mikrotik", zap.String("peerID", p.PeerID), zap.Error(err))
			}
		}

		if p.Disabled {
			continue
		}

		updates := map[string]interface{}{
			"disabled":                  true,
			"suspended_by_quota":        true,
			"was_active_before_suspend": true,
		}
		if err := tx.Model(&model.Peer{}).Where("id = ?", p.ID).Updates(updates).Error; err != nil {
			c.logger.Error("failed to disable peer in DB", zap.String("peerID", p.PeerID), zap.Error(err))
		}
	}

	return nil
}

func (c *Calculator) calculatePeerDeltas(peer model.Peer, currentTx, currentRx, maxCounter int64) (int64, int64, bool) {
	deltaTx, resetTx := calculateDelta(peer.LastTx, currentTx, maxCounter)
	deltaRx, resetRx := calculateDelta(peer.LastRx, currentRx, maxCounter)
	return deltaTx, deltaRx, resetTx || resetRx
}

// applyPeerTrafficLimit disables a peer once its own TrafficLimit is
// exceeded -- gated on !peer.Disabled (mirroring
// processUserManagerAccountUsage's own identical guard) so this job
// never re-evaluates, re-disables, or re-hits RouterOS for a peer it
// already disabled on a previous tick. SuspendedByTrafficLimit is set
// here (a dedicated flag, deliberately NOT SuspendedByQuota -- that one
// is reserved for the completely separate RESELLER-level quota pool
// across Peer/UserManagerAccount/V2RayPackage/Application, and reusing
// it here would make a reseller topping up their OWN overall quota
// incorrectly re-enable a peer that individually exceeded its own
// distinct TrafficLimit) so ResetPeerUsage/ResetPeerUsages below can
// tell "disabled because THIS job hit the traffic limit" apart from "an
// admin disabled this peer manually for an unrelated reason" -- only the
// former should be auto-cleared by a usage reset.
func (c *Calculator) applyPeerTrafficLimit(peer *model.Peer, updates map[string]interface{}) {
	if peer.TrafficLimit != nil && (peer.DownloadUsage+peer.UploadUsage) > *peer.TrafficLimit && !peer.Disabled {
		c.logger.Warn("Peer traffic limit exceeded", zap.String("peerID", peer.PeerID))
		peer.Disabled = true
		updates["disabled"] = true
		updates["suspended_by_traffic_limit"] = true

		_, err := c.mikrotikAdaptor.UpdateWgPeer(context.Background(), peer.PeerID, mikrotik.WireGuardPeer{
			Disabled: strconv.FormatBool(true),
		})
		if err != nil {
			c.logger.Error("Failed to disable peer on Mikrotik", zap.String("peerID", peer.PeerID), zap.Error(err))
		}
	}
}

func (c *Calculator) applyPeerTrafficNotifications(peer *model.Peer, updates map[string]interface{}) {
	if c.notifier == nil || peer.TrafficLimit == nil || peer.TelegramUsername == nil {
		return
	}

	username := strings.TrimSpace(*peer.TelegramUsername)
	if username == "" {
		return
	}

	limit := *peer.TrafficLimit
	if limit <= 0 {
		return
	}

	totalUsage := peer.DownloadUsage + peer.UploadUsage
	percent := (totalUsage * 100) / limit

	c.notifyThreshold(peer, updates, username, percent, totalUsage, limit, 80, "first_notify", &peer.FirstNotify)
	c.notifyThreshold(peer, updates, username, percent, totalUsage, limit, 90, "second_notify", &peer.SecondNotify)
	c.notifyThreshold(peer, updates, username, percent, totalUsage, limit, 100, "third_notify", &peer.ThirdNotify)
}

func (c *Calculator) notifyThreshold(peer *model.Peer, updates map[string]interface{}, username string, percent, totalUsage, limit, threshold int64, updateKey string, notified *bool) {
	if percent < threshold || *notified {
		return
	}

	err := c.notifier.NotifyPeerUsage(context.Background(), peer.Name, username, percent, totalUsage, limit)
	if err != nil {
		c.logger.Error("Failed to send peer usage notification", zap.String("peerID", peer.PeerID), zap.Error(err))
		return
	}

	*notified = true
	updates[updateKey] = true
}

func (c *Calculator) persistPeerTraffic(peer model.Peer, updates map[string]interface{}) {
	if err := c.db.Model(&model.Peer{}).Where("id = ?", peer.ID).Updates(updates).Error; err != nil {
		c.logger.Error("Failed to update peer usage in database", zap.String("peerID", peer.PeerID), zap.Error(err))
	}
}

func (c *Calculator) accumulateTotalTraffic(delta int64) {
	var totalTraffic model.TotalTrafficUsage
	err := c.db.FirstOrCreate(&totalTraffic, model.TotalTrafficUsage{Model: model.Model{ID: model.TotalTrafficUsageSingletonID}}).Error
	if err != nil {
		c.logger.Error("Failed to fetch total traffic usage record", zap.Error(err))
		return
	}

	if err := c.db.Model(&model.TotalTrafficUsage{}).
		Where("id = ?", model.TotalTrafficUsageSingletonID).
		UpdateColumn("total_usage", gorm.Expr("total_usage + ?", delta)).Error; err != nil {
		c.logger.Error("Failed to accumulate total traffic usage", zap.Error(err))
	}
}

// accumulatePeerDailyUsage adds today's traffic delta to the peer's running
// total for the current UTC calendar day, creating the day's row on first
// use. This lets daily usage be reported per peer/reseller without
// recomputing it from raw counters.
func (c *Calculator) accumulatePeerDailyUsage(peerID uint, resellerID *uint, deltaTx, deltaRx int64) {
	today := time.Now().UTC().Format("2006-01-02")

	var usage model.PeerDailyUsage
	err := c.db.FirstOrCreate(&usage, model.PeerDailyUsage{
		PeerID: peerID,
		Date:   today,
	}).Error
	if err != nil {
		c.logger.Error("Failed to fetch peer daily usage record", zap.Uint("peerID", peerID), zap.Error(err))
		return
	}

	// ResellerID can change after the row is created (peer reassigned); keep
	// it in sync so aggregation by reseller stays accurate.
	updates := map[string]interface{}{
		"download_usage": gorm.Expr("download_usage + ?", deltaTx),
		"upload_usage":   gorm.Expr("upload_usage + ?", deltaRx),
		"reseller_id":    resellerID,
	}

	if err := c.db.Model(&model.PeerDailyUsage{}).
		Where("id = ?", usage.ID).
		Updates(updates).Error; err != nil {
		c.logger.Error("Failed to accumulate peer daily usage", zap.Uint("peerID", peerID), zap.Error(err))
	}

	if c.usageRecorder != nil {
		c.usageRecorder.RecordWireGuard(peerID, resellerID, deltaRx, deltaTx)
	}
}

func (c *Calculator) ResetTotalTrafficUsage() error {
	c.mu.Lock()
	defer c.mu.Unlock()

	var totalTraffic model.TotalTrafficUsage
	if err := c.db.FirstOrCreate(&totalTraffic, model.TotalTrafficUsage{Model: model.Model{ID: model.TotalTrafficUsageSingletonID}}).Error; err != nil {
		c.logger.Error("Failed to fetch total traffic usage record", zap.Error(err))
		return err
	}

	if err := c.db.Model(&model.TotalTrafficUsage{}).
		Where("id = ?", model.TotalTrafficUsageSingletonID).
		Update("total_usage", 0).Error; err != nil {
		c.logger.Error("Failed to reset total traffic usage", zap.Error(err))
		return err
	}

	c.logger.Info("Total traffic usage reset successfully")
	return nil
}

func calculateDelta(prev, current, maxCounter int64) (int64, bool) {
	if current >= prev {
		return current - prev, false
	}

	// If counters are already beyond the expected 32-bit max, treat this as a reset.
	if maxCounter <= 0 || prev > maxCounter || current > maxCounter {
		return current, true
	}

	// If the counter moved backwards by a large amount, assume a wrap.
	if (prev - current) > (maxCounter / 2) {
		return (maxCounter - prev) + current, false
	}

	// Otherwise treat it as a reset to avoid a large, incorrect delta.
	return current, true
}
