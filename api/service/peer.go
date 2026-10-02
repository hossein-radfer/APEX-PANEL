package service

import (
	"bytes"
	"context"
	"errors"
	"fmt"
	mathrand "math/rand"
	"net"
	"sort"
	"strconv"
	"strings"
	"sync"
	"time"

	"github.com/google/uuid"
	"go.uber.org/zap"
	"gorm.io/gorm"

	"github.com/maahdima/mwp/api/adaptor/mikrotik"
	"github.com/maahdima/mwp/api/common"
	"github.com/maahdima/mwp/api/dataservice/model"
	"github.com/maahdima/mwp/api/http/schema"
	"github.com/maahdima/mwp/api/utils"
	"github.com/maahdima/mwp/api/utils/timehelper"
	"github.com/maahdima/mwp/api/utils/wireguard"
)

type WgPeer struct {
	db              *gorm.DB
	mikrotikAdaptor *mikrotik.Adaptor
	scheduler       *Scheduler
	queue           *Queue
	configGenerator *ConfigGenerator
	qrCodeGenerator *QRCodeGenerator
	auditLog        *AuditLog
	botNotifier     *BotNotifier
	licenseLimiter  freeTierLimiter
	logger          *zap.Logger

	// interfaceLocks serializes CreatePeer calls per WireGuard interface.
	// Confirmed, reported bug this fixes: GetNewPeerAllowedAddress picks
	// "next free address" by reading the current highest AllowedAddress
	// among this interface's peers with a plain, unlocked SELECT --
	// with no lock, two CreatePeer calls racing on the same interface
	// (e.g. an admin's "All address" auto-pick firing twice in quick
	// succession, two resellers creating peers on a shared interface at
	// the same moment, or CreateApplication's own concurrent
	// provisionInterface goroutines -- see that function's own doc
	// comment -- hitting the same interface from two different
	// Application resource requests) can both resolve the SAME "next"
	// address, both successfully create a RouterOS peer with it, and
	// only the DB's uniqueIndex on allowed_address catches the
	// collision -- by then the losing request's RouterOS peer already
	// exists, orphaned (no matching DB row), permanently occupying that
	// address on the router with no way for the panel's own
	// address-resolution logic (which only ever looks at DB rows) to
	// ever see it as taken. This is the exact mechanism behind the
	// live-reported "IP pool exhausted" errors despite the pool having
	// real free addresses. Locking the whole resolve-create-persist
	// sequence per interface closes the race for every within-process
	// caller (which covers all realistic concurrent-request scenarios
	// against a single panel instance); createPeerOnMikrotikWithCleanup
	// additionally deletes the RouterOS peer if the DB insert still
	// fails for some other reason, so a failure never leaves an orphan
	// behind regardless.
	interfaceLocks sync.Map // map[uint]*sync.Mutex

	// lastPeerCreateAt records, per interface, the wall-clock time of the
	// last RouterOS peer-create PUT that was actually sent -- used by
	// throttleInterfaceCreate to pace successive creates on the same
	// interface. A confirmed, reported bug: interfaceLocks alone
	// serializes callers (no two ever race each other for an address),
	// but under real bursty load (e.g. 15-20+ purchases arriving within
	// the same second, as happens when a bot is announced in a channel)
	// RouterOS itself intermittently rejected a create submitted only
	// ~30-60ms after the previous one succeeded with a bare "entry
	// already exists" -- no address or name collision involved, RouterOS
	// had simply not finished settling the prior peer into its own
	// internal WireGuard peer table yet. This is a genuine RouterOS-side
	// throughput ceiling under bursty creation, not a locking bug in this
	// codebase -- the only reliable fix is pacing, which retrying a
	// failed create (createMikrotikPeerWithRetry) could not fully absorb
	// on its own at this request volume.
	lastPeerCreateAt sync.Map // map[uint]time.Time
}

// minPeerCreateSpacing is the minimum wall-clock gap enforced between two
// successive RouterOS peer-create PUTs on the SAME interface -- see
// lastPeerCreateAt's own doc comment for why this exists. Chosen from
// load-test evidence: spacing under ~100ms still produced occasional
// "already exists" rejections from RouterOS even with createMikrotikPeerWithRetry's
// own backoff layered on top; 250ms consistently did not. Adds at most a
// few hundred ms of extra wait to a real customer's purchase even under a
// 200+ simultaneous burst -- imperceptible next to the multi-second network
// round trips this whole request chain already involves.
const minPeerCreateSpacing = 250 * time.Millisecond

// throttleInterfaceCreate blocks, if necessary, until minPeerCreateSpacing
// has elapsed since the last RouterOS peer-create on this interface, then
// records the current time as the new "last create" -- called while
// holding this interface's lockInterface mutex, so the wait+record is
// itself race-free (only one caller can be inside this function for a
// given interface at a time).
func (w *WgPeer) throttleInterfaceCreate(interfaceId uint) {
	if last, ok := w.lastPeerCreateAt.Load(interfaceId); ok {
		elapsed := time.Since(last.(time.Time))
		if elapsed < minPeerCreateSpacing {
			time.Sleep(minPeerCreateSpacing - elapsed)
		}
	}
	w.lastPeerCreateAt.Store(interfaceId, time.Now())
}

func NewWGPeer(db *gorm.DB, mikrotikAdaptor *mikrotik.Adaptor, scheduler *Scheduler, queue *Queue, configGenerator *ConfigGenerator, qrCodeGenerator *QRCodeGenerator, auditLog *AuditLog) *WgPeer {
	return &WgPeer{
		db:              db,
		mikrotikAdaptor: mikrotikAdaptor,
		scheduler:       scheduler,
		queue:           queue,
		configGenerator: configGenerator,
		qrCodeGenerator: qrCodeGenerator,
		auditLog:        auditLog,
		logger:          zap.L().Named("WgPeerService"),
	}
}

// lockInterface returns the mutex serializing peer-creation for the given
// interface, creating it on first use. sync.Map.LoadOrStore is safe for
// concurrent first-time creation (only one *sync.Mutex ever wins per key).
func (w *WgPeer) lockInterface(interfaceId uint) *sync.Mutex {
	actual, _ := w.interfaceLocks.LoadOrStore(interfaceId, &sync.Mutex{})
	return actual.(*sync.Mutex)
}

// SetBotNotifier wires the Telegram live-log notifier after construction,
// since BotNotifier depends on the bot service, which is constructed after
// WgPeer during startup. Safe to leave unset.
func (w *WgPeer) SetBotNotifier(notifier *BotNotifier) {
	w.botNotifier = notifier
}

// SetLicenseLimiter wires the free-tier cap lookup in (فاز ۴-۱۲) -- see
// freeTierLimiter's own doc comment (reseller.go) for why this is an
// injected narrow interface rather than a direct *LicenseService
// dependency. Safe to leave unset.
func (w *WgPeer) SetLicenseLimiter(limiter freeTierLimiter) {
	w.licenseLimiter = limiter
}

// ErrFreeTierPeerLimitReached mirrors ErrFreeTierResellerLimitReached
// (reseller.go) exactly, for the "max_peers" free-tier cap.
var ErrFreeTierPeerLimitReached = errors.New("this license has expired and is running in restricted mode: the wireguard peer limit for the free tier has been reached")

// SuspendOldestForFreeTier implements the retroactive side of phase 4-12's
// free-tier system: when a license newly transitions into Restricted mode
// (or the admin lowers an already-Restricted cap) and MORE peers already
// exist than the new "max_peers" cap allows, the OLDEST excess peers (by
// CreatedAt, matching the user's explicit "oldest resources first"
// decision) are force-disabled -- not deleted, and not the newest ones,
// which is what a customer would actually be using right now.
//
// This only touches the DB (Disabled/SuspendedByQuota/WasActiveBeforeSuspend
// -- the exact same flags/columns resumeQuotaSuspendedPeers already
// understands, so a later license upgrade resumes these peers through that
// same existing path with no new code needed). It deliberately does NOT
// call RouterOS directly: the periodic traffic-sync job
// (cmd/jobs/traffic.go) reads Disabled from the DB and reconciles the
// actual RouterOS peer state on its own next tick, exactly like it already
// does for every other Disabled-setting path in this codebase. This keeps
// the free-tier enforcement point itself fast and network-call-free, at
// the cost of a short window (up to one sync tick) before the peer is
// actually cut off on the router -- an accepted, explicit trade-off given
// this is a downgrade/expiry path, not a security boundary.
//
// Idempotent and safe to call on every heartbeat: peers already disabled
// (Disabled=true) are excluded from the candidate query, so a repeat call
// with the same cap is a no-op after the first one converges.
func (w *WgPeer) SuspendOldestForFreeTier(cap int64) (int, error) {
	if cap <= 0 {
		return 0, nil
	}

	var activeCount int64
	if err := w.db.Model(&model.Peer{}).Where("disabled = ?", false).Count(&activeCount).Error; err != nil {
		return 0, err
	}

	excess := activeCount - cap
	if excess <= 0 {
		return 0, nil
	}

	var toSuspend []model.Peer
	if err := w.db.Where("disabled = ?", false).Order("created_at ASC").Limit(int(excess)).Find(&toSuspend).Error; err != nil {
		return 0, err
	}

	suspended := 0
	for _, p := range toSuspend {
		if err := w.db.Model(&model.Peer{}).Where("id = ?", p.ID).Updates(map[string]interface{}{
			"disabled":                  true,
			"suspended_by_quota":        true,
			"was_active_before_suspend": true,
		}).Error; err != nil {
			w.logger.Error("failed to suspend peer for free-tier cap", zap.Uint("peer_id", p.ID), zap.Error(err))
			continue
		}
		suspended++
	}

	return suspended, nil
}

func (w *WgPeer) TogglePeerStatus(id uint, resellerID *uint) error {
	peer, err := w.getPeerByIDScoped(id, resellerID)
	if err != nil {
		w.logger.Error("failed to find peer in database", zap.Error(err))
		return fmt.Errorf("peer not found: %w", err)
	}

	disabled := strconv.FormatBool(!peer.Disabled)

	wgPeer := mikrotik.WireGuardPeer{
		Disabled: disabled,
	}
	wgScheduler := mikrotik.Scheduler{
		Disabled: disabled,
	}
	wgQueue := mikrotik.Queue{
		Disabled: disabled,
	}

	if _, err := w.mikrotikAdaptor.UpdateWgPeer(context.Background(), peer.PeerID, wgPeer); err != nil {
		w.logger.Error("failed to update wireguard peer in Mikrotik", zap.Error(err))
		return fmt.Errorf("failed to update wireguard peer: %w", err)
	}

	if peer.SchedulerID != nil {
		if _, err := w.mikrotikAdaptor.UpdateScheduler(context.Background(), *peer.SchedulerID, wgScheduler); err != nil {
			w.logger.Error("failed to update scheduler for wireguard peer", zap.Error(err))
			return fmt.Errorf("failed to update scheduler: %w", err)
		}
	}
	if peer.QueueID != nil {
		if _, err := w.mikrotikAdaptor.UpdateSimpleQueue(context.Background(), *peer.QueueID, wgQueue); err != nil {
			w.logger.Error("failed to update queue for wireguard peer", zap.Error(err))
			return fmt.Errorf("failed to update queue: %w", err)
		}
	}

	if err := w.db.Model(&peer).Update("disabled", disabled).Error; err != nil {
		w.logger.Error("failed to update peer status in database", zap.Error(err))
		return fmt.Errorf("failed to update peer status in database: %w", err)
	}

	return nil
}

func (w *WgPeer) GetPeerCredentials() (*schema.PeerCredentialsResponse, error) {
	privKey, privateKey, err := wireguard.GeneratePrivateKey()
	if err != nil {
		w.logger.Error("failed to generate private key", zap.Error(err))
		return nil, err
	}

	publicKey, err := wireguard.GeneratePublicKey(privKey)
	if err != nil {
		w.logger.Error("failed to generate public key from private key", zap.Error(err))
		return nil, err
	}

	return &schema.PeerCredentialsResponse{
		PrivateKey: privateKey,
		PublicKey:  publicKey,
	}, nil
}

func (w *WgPeer) GetNewPeerAllowedAddress(interfaceId uint, resellerID *uint) (*schema.NewPeerAllowedAddressResponse, error) {
	if resellerID != nil {
		assigned, err := w.isInterfaceAssignedToReseller(*resellerID, interfaceId)
		if err != nil {
			w.logger.Error("failed to check reseller interface assignment", zap.Error(err))
			return nil, err
		}
		if !assigned {
			return nil, fmt.Errorf("reseller is not assigned to interface %d", interfaceId)
		}
	}

	address, err := w.resolveNextFreeAllowedAddress(interfaceId, nil)
	if err != nil {
		return nil, err
	}
	return &schema.NewPeerAllowedAddressResponse{AllowedAddress: address}, nil
}

// resolveNextFreeAllowedAddress is GetNewPeerAllowedAddress's own
// computation, factored out so CreatePeer can re-run it AFTER acquiring
// interfaceLocks (see that field's own doc comment) to pick a genuinely
// free address itself instead of trusting a caller-supplied one -- a
// confirmed, reported bug: GetNewPeerAllowedAddress is its own separate,
// unauthenticated-of-any-lock HTTP endpoint, called by every client
// (including this panel's own web UI) BEFORE the actual CreatePeer
// request, so under concurrent load many callers resolve the exact same
// "next free" address in the gap between the two calls -- interfaceLocks
// only serializes CreatePeer against itself, it can't retroactively fix an
// address that was already stale before the caller even reached the lock.
// exclude, when non-nil, is treated as already-taken even if the caller's
// own in-flight peer row isn't persisted yet (unused today, kept for a
// future caller that already holds a tentative address).
func (w *WgPeer) resolveNextFreeAllowedAddress(interfaceId uint, exclude map[string]struct{}) (string, error) {
	var iface model.Interface
	if err := w.db.Preload("IPPool").First(&iface, "id = ?", interfaceId).Error; err != nil {
		if errors.Is(err, gorm.ErrRecordNotFound) {
			w.logger.Error("interface not found in database", zap.Uint("interfaceId", interfaceId))
			return "", fmt.Errorf("interface %d not found", interfaceId)
		}
		w.logger.Error("failed to query interface from database", zap.Error(err))
		return "", err
	}

	if iface.IPPool == nil {
		w.logger.Warn("no IP pool associated with interface", zap.Uint("interfaceId", interfaceId))
		return "", nil
	}

	startIP := net.ParseIP(strings.TrimSuffix(iface.IPPool.StartIP, "/32")).To4()
	endIP := net.ParseIP(strings.TrimSuffix(iface.IPPool.EndIP, "/32")).To4()

	if startIP == nil || endIP == nil {
		w.logger.Error("invalid IP pool format",
			zap.String("start_ip", iface.IPPool.StartIP),
			zap.String("end_ip", iface.IPPool.EndIP))
		return "", fmt.Errorf("invalid IP pool format")
	}

	var peers []model.Peer
	if err := w.db.Find(&peers, "interface = ?", iface.Name).Error; err != nil {
		w.logger.Error("failed to query peers from database", zap.Error(err))
		return "", fmt.Errorf("failed to find peers: %w", err)
	}

	taken := make(map[string]struct{}, len(peers))
	var highestIP net.IP
	for _, peer := range peers {
		currentIP, _, err := net.ParseCIDR(peer.AllowedAddress)
		if err != nil {
			w.logger.Warn("skipping peer with invalid allowed_address",
				zap.String("allowed_address", peer.AllowedAddress), zap.Error(err))
			continue
		}
		currentIP = currentIP.To4()
		if currentIP == nil {
			continue
		}
		taken[currentIP.String()] = struct{}{}

		if highestIP == nil || bytes.Compare(currentIP, highestIP) > 0 {
			highestIP = currentIP
		}
	}

	var lastIP net.IP
	if highestIP == nil {
		w.logger.Info("no peers found for interface, assigning start IP", zap.Uint("interfaceId", interfaceId))
		lastIP = make(net.IP, len(startIP))
		copy(lastIP, startIP)
		for i := len(lastIP) - 1; i >= 0; i-- {
			lastIP[i]--
			if lastIP[i] != 255 {
				break
			}
		}
	} else {
		lastIP = highestIP
	}

	nextIP := make(net.IP, len(lastIP))
	copy(nextIP, lastIP)
	for {
		for i := len(nextIP) - 1; i >= 0; i-- {
			nextIP[i]++
			if nextIP[i] != 0 {
				break
			}
		}

		if bytes.Compare(nextIP, endIP) > 0 {
			w.logger.Error("IP pool exhausted or next IP out of range",
				zap.String("next_ip", nextIP.String()),
				zap.String("end_ip", endIP.String()))
			return "", fmt.Errorf("IP pool exhausted for interface %d", interfaceId)
		}

		_, isTaken := taken[nextIP.String()]
		_, isExcluded := exclude[nextIP.String()]
		if !isTaken && !isExcluded {
			break
		}
	}

	return fmt.Sprintf("%s/32", nextIP.String()), nil
}

func (w *WgPeer) GetPeerShareStatus(id uint, resellerID *uint) (*schema.PeerShareStatusResponse, error) {
	peer, err := w.getPeerByIDScoped(id, resellerID)
	if err != nil {
		if errors.Is(err, gorm.ErrRecordNotFound) {
			w.logger.Error("peer not found in database", zap.Uint("id", id))
			return nil, gorm.ErrRecordNotFound
		}
		w.logger.Error("failed to find peer in database", zap.Error(err))
		return nil, err
	}

	if !peer.IsShared {
		return &schema.PeerShareStatusResponse{
			IsShared:   false,
			UUID:       nil,
			ExpireTime: nil,
		}, nil
	}

	return &schema.PeerShareStatusResponse{
		IsShared:   peer.IsShared,
		UUID:       &peer.UUID,
		ExpireTime: peer.ShareExpireTime,
	}, nil
}

func (w *WgPeer) TogglePeerShareStatus(id uint, resellerID *uint) error {
	peer, err := w.getPeerByIDScoped(id, resellerID)
	if err != nil {
		w.logger.Error("failed to find peer in database", zap.Error(err))
		return fmt.Errorf("peer not found: %w", err)
	}

	isShared := !peer.IsShared

	if err := w.db.Model(&peer).Update("is_shared", isShared).Error; err != nil {
		w.logger.Error("failed to update peer share status in database", zap.Error(err))
		return fmt.Errorf("failed to update peer share status: %w", err)
	}

	return nil
}

func (w *WgPeer) UpdatePeerShareExpireTime(id uint, expireTime *string, resellerID *uint) error {
	peer, err := w.getPeerByIDScoped(id, resellerID)
	if err != nil {
		w.logger.Error("failed to find peer in database", zap.Error(err))
		return fmt.Errorf("peer not found: %w", err)
	}

	if !peer.IsShared {
		w.logger.Error("peer is not shared, cannot set expire time", zap.Uint("id", id))
		return fmt.Errorf("peer is not shared, cannot set expire time")
	}

	if err := w.db.Model(&peer).Update("share_expire_time", expireTime).Error; err != nil {
		w.logger.Error("failed to update peer share expire time in database", zap.Error(err))
		return fmt.Errorf("failed to update peer share expire time: %w", err)
	}

	return nil
}

func (w *WgPeer) GetPeerDetails(uuid string) (*schema.PeerDetailsResponse, error) {
	var peer model.Peer
	if err := w.db.First(&peer, "uuid = ?", uuid).Error; err != nil {
		if errors.Is(err, gorm.ErrRecordNotFound) {
			w.logger.Error("peer not found in database", zap.String("uuid", uuid))
			return nil, err
		}
		w.logger.Error("failed to find peer in database", zap.Error(err))
		return nil, err
	}

	isSharable := utils.IsPeerSharable(peer.IsShared, peer.ShareExpireTime)
	if !isSharable {
		return nil, common.ErrPeerNotShared
	}

	totalUsage := peer.DownloadUsage + peer.UploadUsage

	var usagePercent, trafficLimit *string
	if peer.TrafficLimit != nil {
		trafficLimit = utils.Ptr(utils.BytesToGB(*peer.TrafficLimit))
		percent := float64(totalUsage) / float64(*peer.TrafficLimit) * 100
		usagePercent = utils.Ptr(fmt.Sprintf("%.1f", percent))
	}

	mtPeer, err := w.mikrotikAdaptor.FetchWgPeer(context.Background(), peer.PeerID)
	if err != nil {
		w.logger.Error("failed to fetch wireguard peer from Mikrotik", zap.String("peer_id", peer.PeerID), zap.Error(err))
		return nil, fmt.Errorf("failed to fetch wireguard peer: %w", err)
	}

	_, isOnline, err := w.handshakeData(mtPeer)
	if err != nil {
		w.logger.Error("failed to parse last handshake duration", zap.String("peer_id", peer.PeerID), zap.Error(err))
		return nil, fmt.Errorf("failed to parse last handshake duration: %w", err)
	}

	// Location is the same admin-set "لوکیشن" label Applications already
	// show for their own WireGuard interfaces (see ApplicationService's
	// own locationLabelsByKey/lookupLabel, keyed by
	// ResourceTypeWireGuardInterface + the interface's numeric ID) --
	// surfaced here too so the public share page can show it exactly like
	// the mobile app's own location picker (see the mobile app's own
	// AppConnectWireGuardConfig.Label, which reads the identical mapping).
	// nil when no admin label has been set for this peer's interface, or
	// when the interface itself no longer exists in the DB (a stale
	// reference is not a request failure -- same "just skip it" stance
	// every other lookup in this codebase already takes toward a missing
	// underlying resource).
	var location *string
	var iface model.Interface
	if err := w.db.Where("name = ?", peer.Interface).First(&iface).Error; err == nil {
		location = lookupLabel(w.locationLabelsByKey(), model.ResourceTypeWireGuardInterface, fmt.Sprintf("%d", iface.ID))
	}

	return &schema.PeerDetailsResponse{
		Name:          peer.Name,
		TrafficLimit:  trafficLimit,
		ExpireTime:    peer.ExpireTime,
		DownloadUsage: utils.BytesToGB(peer.DownloadUsage),
		UploadUsage:   utils.BytesToGB(peer.UploadUsage),
		TotalUsage:    utils.BytesToGB(totalUsage),
		UsagePercent:  usagePercent,
		IsOnline:      isOnline,
		Location:      location,
	}, nil
}

// locationLabelsByKey mirrors ApplicationService's own identically-named
// helper -- both read the same ApplicationResourceLocation table (a
// single shared admin-label mapping across every resource type/owner, not
// scoped to one Application), so there is deliberately no reason to
// duplicate it as a method on ApplicationService alone.
func (w *WgPeer) locationLabelsByKey() map[string]string {
	var rows []model.ApplicationResourceLocation
	w.db.Find(&rows)
	result := make(map[string]string, len(rows))
	for _, r := range rows {
		result[locationKey(r.ResourceType, r.ResourceKey)] = r.Label
	}
	return result
}

func (w *WgPeer) GetPeers(resellerID *uint) (*[]schema.PeerResponse, error) {
	peers, err := w.mikrotikAdaptor.FetchWgPeers(context.Background())
	if err != nil {
		w.logger.Error("failed to fetch wireguard peers from Mikrotik", zap.Error(err))
		return nil, fmt.Errorf("failed to fetch wireguard peers: %w", err)
	}

	var dbPeers []model.Peer
	peerQuery := w.db.Model(&model.Peer{})
	if resellerID != nil {
		peerQuery = peerQuery.Where("reseller_id = ?", *resellerID)
	} else {
		// A confirmed, reported bug: without this branch, an admin caller
		// (resellerID == nil) saw literally every peer in the database --
		// their own AND every reseller's -- pooled together with no way to
		// tell them apart (no reseller/owner column exists on this list at
		// all). Mirrors V2RayPackageService.listPackagesScoped's identical
		// "reseller_id IS NULL means admin-direct only" convention.
		// GetPeersByReseller below remains the separate, deliberate
		// "admin viewing one specific reseller's peers" path.
		peerQuery = peerQuery.Where("reseller_id IS NULL")
	}
	if err := peerQuery.Find(&dbPeers).Error; err != nil {
		w.logger.Error("failed to fetch peers from database", zap.Error(err))
		return nil, fmt.Errorf("failed to fetch peers from database: %w", err)
	}

	dbPeerMap := make(map[string]model.Peer, len(dbPeers))
	for _, p := range dbPeers {
		dbPeerMap[p.PeerID] = p
	}

	var wgPeers []schema.PeerResponse
	for _, peer := range peers {
		dbPeer, exists := dbPeerMap[peer.ID]
		if !exists {
			continue
		}

		wgPeer := w.transformPeerToResponse(dbPeer)

		_, isOnline, err := w.handshakeData(&peer)
		if err != nil {
			w.logger.Error("failed to parse last handshake duration", zap.String("peer_id", peer.ID), zap.Error(err))
			continue
		}

		wgPeer.IsOnline = isOnline
		wgPeers = append(wgPeers, wgPeer)
	}

	return &wgPeers, nil
}

// GetPeersByReseller returns all peers owned by a specific reseller, for
// the admin "Reseller Peers" page -- an unscoped variant of GetPeers that
// takes the target resellerID directly rather than from the caller's own
// JWT claims.
func (w *WgPeer) GetPeersByReseller(resellerID uint) (*[]schema.PeerResponse, error) {
	return w.GetPeers(&resellerID)
}

// GetResellerActivitySummary reports, for every reseller, how many of their
// peers are online right now and how much traffic they've used today, plus
// grand totals across all resellers. Used by the admin dashboard.
//
// page/pageSize paginate the per-reseller breakdown (sorted by name);
// page defaults to 1 and pageSize to 20 (max 100) when out of range. The
// grand totals (TotalOnlinePeers/TotalTodayUsageGB) are always computed
// across ALL resellers, not just the current page.
func (w *WgPeer) GetResellerActivitySummary(page, pageSize int) (*schema.ResellerActivitySummaryResponse, error) {
	mtPeers, err := w.mikrotikAdaptor.FetchWgPeers(context.Background())
	if err != nil {
		w.logger.Error("failed to fetch wireguard peers from Mikrotik", zap.Error(err))
		return nil, fmt.Errorf("failed to fetch wireguard peers: %w", err)
	}

	onlineByPeerID := make(map[string]bool, len(mtPeers))
	for i := range mtPeers {
		_, isOnline, err := w.handshakeData(&mtPeers[i])
		if err != nil {
			continue
		}
		onlineByPeerID[mtPeers[i].ID] = isOnline
	}

	var dbPeers []model.Peer
	if err := w.db.Where("reseller_id IS NOT NULL").Find(&dbPeers).Error; err != nil {
		w.logger.Error("failed to fetch reseller-owned peers", zap.Error(err))
		return nil, err
	}

	var resellers []model.Reseller
	if err := w.db.Find(&resellers).Error; err != nil {
		w.logger.Error("failed to fetch resellers", zap.Error(err))
		return nil, err
	}
	resellerNames := make(map[uint]string, len(resellers))
	for _, r := range resellers {
		resellerNames[r.ID] = r.Name
	}

	today := time.Now().UTC().Format("2006-01-02")
	var todayUsages []model.PeerDailyUsage
	if err := w.db.Where("date = ? AND reseller_id IS NOT NULL", today).Find(&todayUsages).Error; err != nil {
		w.logger.Error("failed to fetch today's peer usage", zap.Error(err))
		return nil, err
	}
	usageBytesByReseller := make(map[uint]int64, len(resellers))
	for _, u := range todayUsages {
		if u.ResellerID != nil {
			usageBytesByReseller[*u.ResellerID] += u.DownloadUsage + u.UploadUsage
		}
	}

	type counts struct {
		online int
		total  int
	}
	countsByReseller := make(map[uint]*counts)
	for _, p := range dbPeers {
		if p.ResellerID == nil {
			continue
		}
		c, ok := countsByReseller[*p.ResellerID]
		if !ok {
			c = &counts{}
			countsByReseller[*p.ResellerID] = c
		}
		c.total++
		if onlineByPeerID[p.PeerID] {
			c.online++
		}
	}

	response := make([]schema.ResellerActivityResponse, 0, len(countsByReseller))
	totalOnline := 0
	var totalUsageBytes int64
	for resellerID, c := range countsByReseller {
		usageBytes := usageBytesByReseller[resellerID]
		response = append(response, schema.ResellerActivityResponse{
			ResellerID:   resellerID,
			ResellerName: resellerNames[resellerID],
			OnlinePeers:  c.online,
			TotalPeers:   c.total,
			TodayUsageGB: utils.BytesToGB(usageBytes),
		})
		totalOnline += c.online
		totalUsageBytes += usageBytes
	}

	sort.Slice(response, func(i, j int) bool {
		return response[i].ResellerName < response[j].ResellerName
	})

	if page < 1 {
		page = 1
	}
	if pageSize < 1 || pageSize > 100 {
		pageSize = 20
	}

	totalCount := len(response)
	totalPages := (totalCount + pageSize - 1) / pageSize
	start := (page - 1) * pageSize
	end := start + pageSize
	if start > totalCount {
		start = totalCount
	}
	if end > totalCount {
		end = totalCount
	}
	pagedResponse := response[start:end]

	return &schema.ResellerActivitySummaryResponse{
		Resellers:         pagedResponse,
		TotalOnlinePeers:  totalOnline,
		TotalTodayUsageGB: utils.BytesToGB(totalUsageBytes),
		Page:              page,
		PageSize:          pageSize,
		TotalCount:        totalCount,
		TotalPages:        totalPages,
	}, nil
}

// GetSelfActivity reports a single reseller's own online-peer count and
// today's traffic usage. Used by the reseller dashboard.
func (w *WgPeer) GetSelfActivity(resellerID uint) (*schema.SelfActivityResponse, error) {
	mtPeers, err := w.mikrotikAdaptor.FetchWgPeers(context.Background())
	if err != nil {
		w.logger.Error("failed to fetch wireguard peers from Mikrotik", zap.Error(err))
		return nil, fmt.Errorf("failed to fetch wireguard peers: %w", err)
	}

	onlineByPeerID := make(map[string]bool, len(mtPeers))
	for i := range mtPeers {
		_, isOnline, err := w.handshakeData(&mtPeers[i])
		if err != nil {
			continue
		}
		onlineByPeerID[mtPeers[i].ID] = isOnline
	}

	var dbPeers []model.Peer
	if err := w.db.Where("reseller_id = ?", resellerID).Find(&dbPeers).Error; err != nil {
		w.logger.Error("failed to fetch reseller peers", zap.Uint("resellerID", resellerID), zap.Error(err))
		return nil, err
	}

	online := 0
	for _, p := range dbPeers {
		if onlineByPeerID[p.PeerID] {
			online++
		}
	}

	today := time.Now().UTC().Format("2006-01-02")
	var todayUsages []model.PeerDailyUsage
	if err := w.db.Where("reseller_id = ? AND date = ?", resellerID, today).Find(&todayUsages).Error; err != nil {
		w.logger.Error("failed to fetch today's usage", zap.Uint("resellerID", resellerID), zap.Error(err))
		return nil, err
	}
	var usageBytes int64
	for _, u := range todayUsages {
		usageBytes += u.DownloadUsage + u.UploadUsage
	}

	return &schema.SelfActivityResponse{
		OnlinePeers:  online,
		TotalPeers:   len(dbPeers),
		TodayUsageGB: utils.BytesToGB(usageBytes),
	}, nil
}

// ensureResellerUnderPeerLimit mirrors UserManagerService.
// ensureResellerUnderUserManagerAccountLimit exactly, swapping in
// Peer/MaxPeers. Called from both BulkCreatePeers (up front, against the
// whole batch) and CreatePeer itself (per single peer) -- a confirmed,
// reported bug: CreatePeer used to have NO enforcement of Reseller.
// MaxPeers at all, so a reseller could create unlimited individual peers
// one at a time through the normal single-create endpoint even with a
// cap configured, despite BulkCreatePeers already enforcing it for the
// batch-create endpoint.
func (w *WgPeer) ensureResellerUnderPeerLimit(resellerID uint, additional int) error {
	var reseller model.Reseller
	if err := w.db.First(&reseller, resellerID).Error; err != nil {
		return err
	}
	if reseller.MaxPeers == nil {
		return nil
	}

	var count int64
	if err := w.db.Model(&model.Peer{}).Where("reseller_id = ?", resellerID).Count(&count).Error; err != nil {
		return err
	}
	if int(count)+additional > *reseller.MaxPeers {
		return fmt.Errorf("creating %d peers would exceed the reseller's maximum allowed peers (%d, %d already exist)", additional, *reseller.MaxPeers, count)
	}
	return nil
}

// BulkCreatePeers creates req.Count peers sharing the same interface/
// endpoint/traffic-limit/bandwidth selection, each via a plain call to
// CreatePeer below (reusing its existing RouterOS fan-out, per-interface
// locking, and orphan-cleanup behavior verbatim, rather than duplicating
// any of that logic here) -- mirrors V2RayPackageService.
// BulkCreatePackages' exact structure. Unlike a package, each peer needs
// its own keypair and allowed_address: GetPeerCredentials/
// GetNewPeerAllowedAddress are called fresh for every iteration (the
// latter via CreatePeer's own per-interface lock, so each successive call
// correctly sees the previous iteration's just-persisted address and never
// hands out the same address twice). The reseller peer-limit check happens
// UP FRONT against the full req.Count, matching BulkCreatePackages'
// "whole batch or nothing" contract.
func (w *WgPeer) BulkCreatePeers(req *schema.BulkCreatePeerRequest, resellerID *uint) ([]schema.PeerResponse, error) {
	if resellerID != nil {
		if err := w.ensureResellerUnderPeerLimit(*resellerID, req.Count); err != nil {
			return nil, err
		}
	}

	expireTime := time.Now().UTC().AddDate(0, 0, req.DurationDays).Format("2006-01-02")

	results := make([]schema.PeerResponse, 0, req.Count)
	for i := 0; i < req.Count; i++ {
		credentials, err := w.GetPeerCredentials()
		if err != nil {
			return results, fmt.Errorf("failed after creating %d of %d peers: %w", len(results), req.Count, err)
		}

		addr, err := w.GetNewPeerAllowedAddress(req.InterfaceId, resellerID)
		if err != nil {
			return results, fmt.Errorf("failed after creating %d of %d peers: %w", len(results), req.Count, err)
		}

		adjective := bulkLabelAdjectives[mathrand.Intn(len(bulkLabelAdjectives))]
		noun := bulkLabelNouns[mathrand.Intn(len(bulkLabelNouns))]
		name := fmt.Sprintf("%s%s%d", adjective, noun, i+1)

		peer, err := w.CreatePeer(&schema.CreatePeerRequest{
			Name:                name,
			InterfaceId:         req.InterfaceId,
			PrivateKey:          credentials.PrivateKey,
			PublicKey:           credentials.PublicKey,
			AllowedAddress:      addr.AllowedAddress,
			DNSServers:          req.DNSServers,
			PersistentKeepAlive: req.PersistentKeepAlive,
			Endpoint:            req.Endpoint,
			ExpireTime:          &expireTime,
			TrafficLimit:        req.TrafficLimit,
			DownloadBandwidth:   req.DownloadBandwidth,
			UploadBandwidth:     req.UploadBandwidth,
		}, resellerID)
		if err != nil {
			w.logger.Error("bulk create failed partway through -- returning peers created so far", zap.Int("created", len(results)), zap.Int("requested", req.Count), zap.Error(err))
			return results, fmt.Errorf("failed after creating %d of %d peers: %w", len(results), req.Count, err)
		}

		// Sharing is off by default on every peer (see buildAndStoreDbPeer --
		// IsShared is never set there, so it's Go's bool zero value, false).
		// A bulk-created batch exists specifically to be exported with its
		// share links (the admin's whole point in using this flow), so
		// turning sharing on here -- once, right after creation -- is what
		// makes the exported link resolve to a working share page instead of
		// one that 404s against GetPeerShareStatus's own IsShared check.
		// Mirrors V2RayPackageService.BulkCreatePackages' identical
		// "enable sharing right after bulk creation" step.
		if updateErr := w.db.Model(&model.Peer{}).Where("id = ?", peer.Id).Update("is_shared", true).Error; updateErr != nil {
			w.logger.Warn("failed to enable sharing on bulk-created peer, its share link will not resolve until sharing is turned on manually", zap.Uint("peer_id", peer.Id), zap.Error(updateErr))
		} else {
			peer.IsShared = true
		}

		results = append(results, *peer)
	}

	return results, nil
}

func (w *WgPeer) CreatePeer(req *schema.CreatePeerRequest, resellerID *uint) (*schema.PeerResponse, error) {
	if w.licenseLimiter != nil {
		if maxPeers, ok := w.licenseLimiter.GetFreeTierLimit("max_peers"); ok {
			var activeCount int64
			if err := w.db.Model(&model.Peer{}).Count(&activeCount).Error; err != nil {
				return nil, err
			}
			if activeCount >= maxPeers {
				return nil, ErrFreeTierPeerLimitReached
			}
		}
	}

	iface, err := w.getInterface(req.InterfaceId)
	if err != nil {
		return nil, err
	}

	if resellerID != nil {
		assigned, err := w.isInterfaceAssignedToReseller(*resellerID, req.InterfaceId)
		if err != nil {
			w.logger.Error("failed to check reseller interface assignment", zap.Error(err))
			return nil, err
		}
		if !assigned {
			return nil, fmt.Errorf("reseller is not assigned to interface %d", req.InterfaceId)
		}
		// A confirmed, reported bug: ensureResellerUnderPeerLimit's own doc
		// comment already admitted this single-peer path never enforced
		// Reseller.MaxPeers at all (only BulkCreatePeers did) -- a reseller
		// with a peer cap set could still create unlimited individual peers
		// one at a time through this endpoint, which is how most real
		// reseller usage actually creates peers. Checked with additional=1
		// since this call creates exactly one peer.
		if err := w.ensureResellerUnderPeerLimit(*resellerID, 1); err != nil {
			return nil, err
		}
	}

	// Serialized per interface -- see interfaceLocks' own doc comment for
	// the exact race (two concurrent CreatePeer calls on the same
	// interface both resolving the same "next free" address) this
	// closes. Held across the RouterOS create AND the DB persist below,
	// so a second caller waiting on this lock always sees the first
	// caller's peer already reflected in the DB by the time IT reaches
	// ensureAllowedAddressIsUnique.
	lock := w.lockInterface(req.InterfaceId)
	lock.Lock()
	defer lock.Unlock()

	// A confirmed, reported bug (load-tested with 15 concurrent purchases
	// on a fresh interface: 14 of 15 failed with "already in use"): the
	// caller's req.AllowedAddress was resolved via the separate
	// GetNewPeerAllowedAddress endpoint BEFORE this lock was ever
	// acquired, so under real concurrent load every caller in flight at
	// the same moment resolves the identical "next free" address -- this
	// lock only serializes what happens after that address is already
	// stale. Rather than reject the request (which is what
	// ensureAllowedAddressIsUnique used to do here), re-resolve a
	// genuinely free address ourselves now that we hold the lock and the
	// DB reflects every peer created by an earlier holder; this makes
	// req.AllowedAddress an optimistic hint rather than a hard
	// requirement, so 100+ simultaneous purchases (e.g. a bot announced
	// in a channel) each still get a distinct real address instead of
	// mostly failing.
	if err := w.ensureAllowedAddressIsUnique(req.AllowedAddress); err != nil {
		freshAddress, resolveErr := w.resolveNextFreeAllowedAddress(req.InterfaceId, nil)
		if resolveErr != nil {
			return nil, resolveErr
		}
		w.logger.Info("requested allowed address was stale, re-resolved under interface lock",
			zap.String("requested", req.AllowedAddress), zap.String("resolved", freshAddress))
		req.AllowedAddress = freshAddress
	}

	// See lastPeerCreateAt's own doc comment: paces successive RouterOS
	// creates on this interface so a burst of purchases (e.g. a bot
	// announced in a channel) doesn't submit them faster than RouterOS
	// itself can settle each one into its own peer table.
	w.throttleInterfaceCreate(req.InterfaceId)

	mtPeer, err := w.createMikrotikPeerWithRetry(req, iface)
	if err != nil {
		return nil, err
	}

	schedulerId, err := w.scheduler.createScheduler(mtPeer.ID, mtPeer.Name, req.ExpireTime)
	if err != nil {
		w.cleanupOrphanedMikrotikPeer(mtPeer.ID)
		return nil, err
	}

	queueId, err := w.queue.createQueue(mtPeer.Name, mtPeer.AllowedAddress, req.DownloadBandwidth, req.UploadBandwidth)
	if err != nil {
		w.cleanupOrphanedMikrotikPeer(mtPeer.ID)
		return nil, err
	}

	dbPeer, err := w.buildAndStoreDbPeer(req, iface, mtPeer, schedulerId, queueId, resellerID)
	if err != nil {
		// The RouterOS peer (and its scheduler/queue) were already
		// created above -- without this cleanup, a DB-side failure here
		// (e.g. a rare cross-process uniqueIndex collision this lock
		// can't cover, since it only serializes callers within THIS
		// panel process) leaves a real, orphaned RouterOS peer
		// permanently occupying its address with no DB row to ever
		// surface it to GetNewPeerAllowedAddress again. See
		// interfaceLocks' own doc comment for the full incident this
		// class of orphaning caused.
		w.cleanupOrphanedMikrotikPeer(mtPeer.ID)
		return nil, err
	}

	if err := w.generatePeerAssets(req.PrivateKey, dbPeer, iface.PublicKey); err != nil {
		return nil, err
	}

	resp := w.transformPeerToResponse(dbPeer)
	return &resp, nil
}

// isMikrotikNotFoundError reports whether err is RouterOS's HTTP 404
// response -- meaning the scheduler/queue/peer being deleted is already
// gone from the router, not that the delete request itself failed. There's
// no typed error for this in httphelper.doRequest (it always formats as
// "request failed with status code %d: %s"), so this is a string check
// rather than an errors.As/Is match -- narrow and deliberate, matched only
// against exactly the substring that function produces for a 404.
func isMikrotikNotFoundError(err error) bool {
	return err != nil && strings.Contains(err.Error(), "status code 404")
}

// cleanupOrphanedMikrotikPeer best-effort deletes a RouterOS peer that was
// just created but whose corresponding DB row failed to persist -- a
// failure here is logged but never returned/propagated, matching this
// codebase's own "best-effort teardown" convention elsewhere (e.g.
// WgPeer.stop's identical stance): the caller's ORIGINAL error is what the
// admin needs to see, not a secondary cleanup failure, and an orphan left
// behind by a failed cleanup is still strictly better than the alternative
// (never even trying).
func (w *WgPeer) cleanupOrphanedMikrotikPeer(peerID string) {
	if err := w.mikrotikAdaptor.DeleteWgPeer(context.Background(), peerID); err != nil {
		w.logger.Error("failed to clean up orphaned mikrotik peer after a later step failed -- this peer/address is now stranded on the router with no DB row",
			zap.String("mikrotik_peer_id", peerID), zap.Error(err))
	}
}

func (w *WgPeer) UpdatePeer(id uint, req *schema.UpdatePeerRequest, resellerID *uint) (*schema.PeerResponse, error) {
	peer, err := w.getPeerByIDScoped(id, resellerID)
	if err != nil {
		w.logger.Error("failed to get peer from database", zap.Error(err))
		return nil, err
	}

	if err := w.updateMikrotikPeer(peer.PeerID, req); err != nil {
		return nil, err
	}

	schedulerID, err := w.handleScheduler(&peer, req)
	if err != nil {
		return nil, err
	}

	queueID, err := w.handleQueue(&peer, req)
	if err != nil {
		return nil, err
	}

	updateData := w.preparePeerUpdate(&peer, req, schedulerID, queueID)
	if err := w.db.Model(&peer).Updates(updateData).Error; err != nil {
		return nil, err
	}

	transformed := w.transformPeerToResponse(peer)
	return &transformed, nil
}

func (w *WgPeer) DeletePeer(id uint, resellerID *uint) error {
	peer, err := w.getPeerByIDScoped(id, resellerID)
	if err != nil {
		w.logger.Error("failed to find peer in database", zap.Error(err))
		return fmt.Errorf("peer not found: %w", err)
	}

	// A confirmed, reproduced bug: a scheduler/queue entry that no longer
	// exists on RouterOS (e.g. it was already removed by an earlier
	// partial delete attempt, or manually on the router) makes RouterOS
	// respond 404, which used to be treated as fatal and aborted the
	// entire delete before even reaching DeleteWgPeer -- permanently
	// blocking removal of a peer whose stale scheduler/queue reference can
	// never resolve on its own (reproduced live: peer 356 could not be
	// deleted through this endpoint at all because of exactly this, even
	// though its scheduler was already long gone). A 404 here means
	// "already gone, nothing to do", not a real failure -- mirrors
	// DeleteAccount's identical "one dead/stale external reference never
	// blocks deletion" convention (see its own doc comment on the
	// RouterOS profile-link delete). Any OTHER error (auth, network,
	// RouterOS down) still aborts the delete, since that peer's scheduler/
	// queue may well still be live and orphaning it on the router would be
	// a real leak.
	if err := w.scheduler.deleteScheduler(peer.SchedulerID); err != nil && !isMikrotikNotFoundError(err) {
		return fmt.Errorf("failed to delete scheduler: %w", err)
	}

	if err := w.queue.deleteQueue(peer.QueueID); err != nil && !isMikrotikNotFoundError(err) {
		return fmt.Errorf("failed to delete simple queue: %w", err)
	}

	if err := w.mikrotikAdaptor.DeleteWgPeer(context.Background(), peer.PeerID); err != nil && !isMikrotikNotFoundError(err) {
		w.logger.Error("failed to delete wireguard peer from Mikrotik", zap.Error(err))
		return fmt.Errorf("failed to delete wireguard peer: %w", err)
	}

	// A confirmed, reproduced bug: once DeleteWgPeer above succeeds, the
	// peer is already gone from RouterOS, and GetPeers only ever lists a DB
	// row when it still has a matching LIVE RouterOS entry (see GetPeers'
	// map-and-skip logic) -- so from this point on, the peer is invisible
	// to every list in the UI no matter what happens next. A QR/config
	// file that was never generated, or was already removed by an earlier
	// partial delete, made os.Remove return ENOENT here, which used to be
	// treated as fatal and returned BEFORE the DB row was deleted --
	// leaving a permanently invisible, permanently undeletable orphan row
	// behind (reproduced live: peer 356 stayed in the peers table forever
	// after a QR-file ENOENT, silently vanishing from every list without
	// ever actually being removed). File cleanup is best-effort from here
	// on: log and continue, exactly like DeleteAccount's identical
	// "one dead/stale reference never blocks the rest" convention (see its
	// own doc comment on the RouterOS profile-link delete for the same
	// principle applied to an external reference).
	if err := w.qrCodeGenerator.RemovePeerQRCode(id); err != nil {
		w.logger.Warn("failed to remove QR Code file -- proceeding to delete the peer anyway", zap.Uint("peer_id", id), zap.Error(err))
	}

	if err := w.configGenerator.RemovePeerConfig(id); err != nil {
		w.logger.Warn("failed to remove peer config file -- proceeding to delete the peer anyway", zap.Uint("peer_id", id), zap.Error(err))
	}

	if err := w.db.Unscoped().Delete(&peer).Error; err != nil {
		w.logger.Error("failed to delete peer from database", zap.Error(err))
		return fmt.Errorf("failed to delete peer from database: %w", err)
	}

	return nil
}

// BulkDeletePeers deletes each given peer via the exact same DeletePeer path
// (scheduler/queue/Mikrotik/QR/config/DB cleanup) one at a time -- there is
// no batch-optimized Mikrotik call for this, and a partial failure on one
// peer (e.g. a stale RouterOS reference) must never block deleting the rest,
// so failures are collected per-ID instead of aborting the whole request.
// Used by the "expired/quota-exhausted" bulk cleanup action.
func (w *WgPeer) BulkDeletePeers(ids []uint, resellerID *uint) (deleted []uint, failed map[uint]string) {
	failed = make(map[uint]string)
	for _, id := range ids {
		if err := w.DeletePeer(id, resellerID); err != nil {
			failed[id] = err.Error()
			continue
		}
		deleted = append(deleted, id)
	}
	return deleted, failed
}

func (w *WgPeer) GetPeersData() (*schema.PeerStatsResponse, error) {
	peers, err := w.mikrotikAdaptor.FetchWgPeers(context.Background())
	if err != nil {
		w.logger.Error("failed to fetch peers from mikrotik", zap.Error(err))
		return nil, fmt.Errorf("failed to fetch peers from mikrotik: %w", err)
	}

	var dbPeers []model.Peer
	if err := w.db.Find(&dbPeers).Error; err != nil {
		w.logger.Error("failed to fetch peers from database", zap.Error(err))
		return nil, fmt.Errorf("failed to fetch peers from database: %w", err)
	}

	dbPeerMap := make(map[string]model.Peer)
	for _, p := range dbPeers {
		dbPeerMap[p.PeerID] = p
	}

	type peerWithDuration struct {
		peer     mikrotik.WireGuardPeer
		duration time.Duration
	}

	var (
		onlinePeers   []peerWithDuration
		disabledPeers []mikrotik.WireGuardPeer
	)

	for _, peer := range peers {
		dbPeer, exists := dbPeerMap[peer.ID]
		if !exists {
			w.logger.Warn("peer not found in database", zap.String("peerID", peer.ID))
			continue
		}

		if peer.Disabled == "true" {
			disabledPeers = append(disabledPeers, peer)
			continue
		}

		duration, isOnline, err := w.handshakeData(&peer)
		if err != nil {
			w.logger.Error("failed to parse last handshake duration", zap.String("peerID", dbPeer.PeerID), zap.Error(err))
			continue
		}

		if isOnline && duration < 150*time.Second {
			onlinePeers = append(onlinePeers, peerWithDuration{
				peer:     peer,
				duration: duration,
			})
		}
	}

	sort.Slice(onlinePeers, func(i, j int) bool {
		return onlinePeers[i].duration < onlinePeers[j].duration
	})

	recentCount := min(5, len(onlinePeers))
	recentPeers := make([]schema.RecentOnlinePeers, 0, recentCount)

	for _, item := range onlinePeers[:recentCount] {
		lastSeenStr := utils.FormatDuration(item.duration)

		recentPeers = append(recentPeers, schema.RecentOnlinePeers{
			Name:     item.peer.Name,
			LastSeen: lastSeenStr,
		})
	}

	return &schema.PeerStatsResponse{
		RecentOnlinePeers: &recentPeers,
		TotalPeers:        len(dbPeers),
		OnlinePeers:       len(onlinePeers),
		OfflinePeers:      len(dbPeers) - len(onlinePeers) - len(disabledPeers),
		DisabledPeers:     len(disabledPeers),
	}, nil
}

func (w *WgPeer) getInterface(id uint) (model.Interface, error) {
	var iface model.Interface
	if err := w.db.First(&iface, "id = ?", id).Error; err != nil {
		if errors.Is(err, gorm.ErrRecordNotFound) {
			w.logger.Error("interface not found", zap.Uint("interfaceId", id))
			return iface, fmt.Errorf("interface %d not found", id)
		}
		w.logger.Error("db error while fetching interface", zap.Error(err))
		return iface, err
	}
	return iface, nil
}

func (w *WgPeer) isInterfaceAssignedToReseller(resellerID uint, interfaceID uint) (bool, error) {
	var count int64
	if err := w.db.Model(&model.ResellerInterface{}).
		Where("reseller_id = ? AND interface_id = ?", resellerID, interfaceID).
		Count(&count).Error; err != nil {
		return false, err
	}
	return count > 0, nil
}

func (w *WgPeer) ensureAllowedAddressIsUnique(address string) error {
	var existing model.Peer
	if err := w.db.Where("allowed_address = ?", address).First(&existing).Error; err == nil {
		return fmt.Errorf("allowed address %s is already in use by peer %s", address, existing.Name)
	} else if !errors.Is(err, gorm.ErrRecordNotFound) {
		w.logger.Error("allowed address lookup failed", zap.Error(err))
		return err
	}
	return nil
}

func (w *WgPeer) createMikrotikPeer(req *schema.CreatePeerRequest, ifaceName string) (*mikrotik.WireGuardPeer, error) {
	peer := &mikrotik.WireGuardPeer{
		Comment:        req.Comment,
		Name:           req.Name,
		AllowedAddress: req.AllowedAddress,
		Interface:      ifaceName,
		PresharedKey:   req.PresharedKey,
		PrivateKey:     &req.PrivateKey,
		PublicKey:      req.PublicKey,
	}
	return w.mikrotikAdaptor.CreateWgPeer(context.Background(), *peer)
}

// createMikrotikPeerWithRetry wraps createMikrotikPeer with a short,
// bounded retry specifically for RouterOS's own "entry already exists"
// rejection -- a confirmed, reported bug found by load-testing 15
// concurrent purchases against a fresh interface (with interfaceLocks
// already serializing every call, and req.AllowedAddress already
// re-resolved fresh under that lock): RouterOS itself still rejected
// several of the serialized, individually-valid PUT requests with this
// exact error, all within a ~0.3s window (~30-60ms apart) -- i.e. even
// one-at-a-time, submitted faster than RouterOS's own internal
// WireGuard peer table can settle a just-added entry, it can bounce a
// perfectly valid next request. This is RouterOS's own throughput limit
// under bursty peer creation, not a bug in this codebase's own locking or
// address resolution -- there is nothing to serialize further; the only
// available recovery IS to back off briefly and let the caller's OWN
// in-flight peer settle before retrying. Only "already exists" is
// retried (that's the one error this specific lag is known to cause);
// every other error still fails immediately.
func (w *WgPeer) createMikrotikPeerWithRetry(req *schema.CreatePeerRequest, iface model.Interface) (*mikrotik.WireGuardPeer, error) {
	// maxAttempts/backoff widened from an initial 4 attempts / 150ms base
	// after load-testing 20 concurrent purchases WITH throttleInterfaceCreate's
	// own 250ms inter-request spacing already in place: spacing alone cut
	// failures roughly in half but did not eliminate them outright under
	// this burst size, meaning RouterOS's own settling time is not a fixed
	// constant -- it can exceed one spacing interval under load. This
	// retry is now genuinely a fallback for that tail, not the primary
	// defense (throttleInterfaceCreate is), so it can afford to be patient.
	const maxAttempts = 6
	backoff := 400 * time.Millisecond

	var lastErr error
	for attempt := 1; attempt <= maxAttempts; attempt++ {
		mtPeer, err := w.createMikrotikPeer(req, iface.Name)
		if err == nil {
			return mtPeer, nil
		}
		lastErr = err

		if !strings.Contains(err.Error(), "entry already exists") || attempt == maxAttempts {
			return nil, err
		}

		w.logger.Warn("RouterOS rejected peer creation with a transient 'already exists', retrying",
			zap.Int("attempt", attempt), zap.String("name", req.Name), zap.Error(err))
		time.Sleep(backoff)
		backoff *= 2
	}
	return nil, lastErr
}

func (w *WgPeer) buildAndStoreDbPeer(req *schema.CreatePeerRequest, iface model.Interface, mtPeer *mikrotik.WireGuardPeer, schedulerId, queueId *string, resellerID *uint) (model.Peer, error) {
	keepalive := common.DefaultKeepalive
	if req.PersistentKeepAlive != nil {
		parsed, err := timehelper.ParseTime(*req.PersistentKeepAlive)
		if err != nil {
			w.logger.Error("invalid keepalive", zap.Error(err))
			return model.Peer{}, err
		}
		keepalive = strconv.Itoa(parsed)
	}

	disabled, err := strconv.ParseBool(mtPeer.Disabled)
	if err != nil {
		w.logger.Error("invalid mikrotik disabled field", zap.Error(err))
		return model.Peer{}, err
	}

	var trafficLimit *int64
	if req.TrafficLimit != nil {
		trafficBytes := utils.GBToBytes(utils.DerefString(req.TrafficLimit))
		trafficLimit = &trafficBytes
	}

	telegramUsername := normalizeTelegramUsername(req.TelegramUsername)

	dbPeer := model.Peer{
		UUID:                uuid.New().String(),
		PeerID:              mtPeer.ID,
		Disabled:            disabled,
		Comment:             mtPeer.Comment,
		Name:                mtPeer.Name,
		PrivateKey:          *mtPeer.PrivateKey,
		PublicKey:           mtPeer.PublicKey,
		Interface:           mtPeer.Interface,
		AllowedAddress:      mtPeer.AllowedAddress,
		Endpoint:            req.Endpoint,
		EndpointPort:        iface.ListenPort,
		PersistentKeepalive: keepalive,
		SchedulerID:         schedulerId,
		QueueID:             queueId,
		ResellerID:          resellerID,
		ExpireTime:          req.ExpireTime,
		TrafficLimit:        trafficLimit,
		TelegramUsername:    telegramUsername,
		DNSServers:          req.DNSServers,
		DownloadBandwidth:   req.DownloadBandwidth,
		UploadBandwidth:     req.UploadBandwidth,
	}

	if err := w.db.Create(&dbPeer).Error; err != nil {
		w.logger.Error("failed to persist peer", zap.Error(err))
		return model.Peer{}, err
	}

	return dbPeer, nil
}

func (w *WgPeer) generatePeerAssets(privateKey string, peer model.Peer, ifacePubKey string) error {
	dnsServers := resolveDnsServers(w.mikrotikAdaptor, w.logger)
	if peer.DNSServers != nil && *peer.DNSServers != "" {
		dnsServers = *peer.DNSServers
	}

	peerConfig := fmt.Sprintf(
		wireguard.Template,
		privateKey,
		peer.AllowedAddress,
		dnsServers,
		ifacePubKey,
		peer.Endpoint,
		peer.EndpointPort,
		common.AllowedIpsIncludeLocal,
		peer.PersistentKeepalive,
	)

	if err := w.configGenerator.BuildPeerConfig(peerConfig, peer.UUID); err != nil {
		return err
	}
	return w.qrCodeGenerator.BuildPeerQRCode(peerConfig, peer.UUID)
}

func (w *WgPeer) updateMikrotikPeer(peerID string, req *schema.UpdatePeerRequest) error {
	wgPeer := mikrotik.WireGuardPeer{}

	if req.Disabled != nil {
		disabledStr := strconv.FormatBool(*req.Disabled)
		wgPeer.Disabled = disabledStr
	}
	if req.Comment != nil {
		wgPeer.Comment = req.Comment
	}

	wgPeer.Name = req.Name
	wgPeer.AllowedAddress = req.AllowedAddress

	if req.PersistentKeepAlive != nil {
		wgPeer.PersistentKeepAlive = req.PersistentKeepAlive
	}
	if req.PresharedKey != nil {
		wgPeer.PresharedKey = req.PresharedKey
	}

	_, err := w.mikrotikAdaptor.UpdateWgPeer(context.Background(), peerID, wgPeer)
	if err != nil {
		w.logger.Error("failed to update wireguard peer in Mikrotik", zap.Error(err))
	}

	return err
}

func (w *WgPeer) handleScheduler(peer *model.Peer, req *schema.UpdatePeerRequest) (*string, error) {
	if req.ExpireTime == nil && peer.SchedulerID != nil {
		err := w.scheduler.deleteScheduler(peer.SchedulerID)
		if err != nil {
			w.logger.Error("failed to delete scheduler for wireguard peer", zap.Error(err))
			return peer.SchedulerID, err
		}
		return nil, nil
	}

	if req.ExpireTime != nil && peer.SchedulerID == nil {
		return w.scheduler.createScheduler(peer.PeerID, peer.Name, req.ExpireTime)
	}

	if req.ExpireTime != nil && peer.SchedulerID != nil {
		err := w.scheduler.updateScheduler(peer.SchedulerID, req.ExpireTime)
		if err != nil {
			w.logger.Error("failed to update scheduler for wireguard peer", zap.Error(err))
			return peer.SchedulerID, err
		}
	}

	return peer.SchedulerID, nil
}

func (w *WgPeer) handleQueue(peer *model.Peer, req *schema.UpdatePeerRequest) (*string, error) {
	download := req.DownloadBandwidth
	upload := req.UploadBandwidth
	queueID := peer.QueueID

	if download == nil && upload == nil {
		if queueID != nil {
			err := w.queue.deleteQueue(queueID)
			if err != nil {
				w.logger.Error("failed to delete queue for wireguard peer", zap.Error(err))
				return queueID, err
			}
		}
		return nil, nil
	}

	if queueID == nil {
		newQueueID, err := w.queue.createQueue(peer.Name, peer.AllowedAddress, download, upload)
		if err != nil {
			w.logger.Error("failed to create queue for wireguard peer", zap.Error(err))
			return nil, err
		}
		return newQueueID, nil
	}

	if !w.bandwidthsEqual(peer.DownloadBandwidth, download) || !w.bandwidthsEqual(peer.UploadBandwidth, upload) {
		err := w.queue.updateQueue(queueID, download, upload)
		if err != nil {
			w.logger.Error("failed to update queue for wireguard peer", zap.Error(err))
			return queueID, err
		}
	}

	return queueID, nil
}

func (w *WgPeer) preparePeerUpdate(peer *model.Peer, req *schema.UpdatePeerRequest, schedulerID, queueID *string) map[string]interface{} {
	updateData := map[string]interface{}{}

	trafficBytes := utils.GBToBytes(utils.DerefString(req.TrafficLimit))
	var trafficLimit *int64
	if trafficBytes > 0 {
		trafficLimit = &trafficBytes
		updateData["traffic_limit"] = trafficBytes
	} else {
		updateData["traffic_limit"] = nil
	}

	telegramUsername := normalizeTelegramUsername(req.TelegramUsername)
	updateData["telegram_username"] = telegramUsername

	if !int64PtrEqual(peer.TrafficLimit, trafficLimit) || !stringPtrEqual(peer.TelegramUsername, telegramUsername) {
		updateData["first_notify"] = false
		updateData["second_notify"] = false
		updateData["third_notify"] = false
	}

	updateData["disabled"] = req.Disabled
	updateData["comment"] = req.Comment
	updateData["name"] = req.Name
	updateData["allowed_address"] = req.AllowedAddress
	updateData["dns_servers"] = req.DNSServers
	updateData["persistent_keepalive"] = req.PersistentKeepAlive
	updateData["expire_time"] = req.ExpireTime
	updateData["download_bandwidth"] = req.DownloadBandwidth
	updateData["upload_bandwidth"] = req.UploadBandwidth
	updateData["scheduler_id"] = schedulerID
	updateData["queue_id"] = queueID

	return updateData
}

func (w *WgPeer) transformPeerToResponse(peer model.Peer) schema.PeerResponse {
	statuses := w.transformPeerStatus(peer)

	var trafficLimit *string

	if peer.TrafficLimit == nil {
		trafficLimit = nil
	} else {
		trafficLimit = utils.Ptr(utils.BytesToGB(*peer.TrafficLimit))
	}

	return schema.PeerResponse{
		Id:                peer.ID,
		UUID:              peer.UUID,
		Disabled:          peer.Disabled,
		Comment:           peer.Comment,
		TelegramUsername:  peer.TelegramUsername,
		Name:              peer.Name,
		Interface:         peer.Interface,
		AllowedAddress:    peer.AllowedAddress,
		DNSServers:        peer.DNSServers,
		TrafficLimit:      trafficLimit,
		ExpireTime:        peer.ExpireTime,
		DownloadBandwidth: peer.DownloadBandwidth,
		UploadBandwidth:   peer.UploadBandwidth,
		TotalUsage:        utils.BytesToGB(peer.DownloadUsage + peer.UploadUsage),
		Status:            statuses,
		IsShared:          peer.IsShared,
	}
}

func (w *WgPeer) transformPeerStatus(peer model.Peer) []schema.PeerStatus {
	var peerStatus []schema.PeerStatus

	if peer.Disabled {
		peerStatus = append(peerStatus, schema.InactivePeer)
	} else {
		peerStatus = append(peerStatus, schema.ActivePeer)
	}

	if peer.ExpireTime != nil {
		expireTime, err := time.Parse("2006-01-02", *peer.ExpireTime)
		if err == nil && time.Now().After(expireTime) {
			peerStatus = append(peerStatus, schema.ExpiredPeer)
		}
	}

	if peer.TrafficLimit != nil {
		totalUsedTraffic := peer.DownloadUsage + peer.UploadUsage
		if totalUsedTraffic > *peer.TrafficLimit {
			peerStatus = append(peerStatus, schema.SuspendedPeer)
		}
	}

	return peerStatus
}

func (w *WgPeer) handshakeData(peer *mikrotik.WireGuardPeer) (duration time.Duration, isOnline bool, err error) {
	if peer.LastHandshake != nil {
		duration, err = utils.ParseCustomDuration(*peer.LastHandshake)
		if err != nil {
			w.logger.Error("failed to parse last handshake duration", zap.Error(err))
			return
		}

		if peer.Disabled == "false" && duration < 150*time.Second {
			isOnline = true
			return
		}
	}

	return
}

func (w *WgPeer) bandwidthsEqual(a, b *string) bool {
	if a == nil && b == nil {
		return true
	}
	if a != nil && b != nil {
		return *a == *b
	}
	return false
}

func normalizeTelegramUsername(username *string) *string {
	if username == nil {
		return nil
	}
	trimmed := strings.TrimSpace(*username)
	if trimmed == "" {
		return nil
	}
	return &trimmed
}

func stringPtrEqual(a, b *string) bool {
	if a == nil && b == nil {
		return true
	}
	if a == nil || b == nil {
		return false
	}
	return *a == *b
}

func int64PtrEqual(a, b *int64) bool {
	if a == nil && b == nil {
		return true
	}
	if a == nil || b == nil {
		return false
	}
	return *a == *b
}

func (w *WgPeer) EnsurePeerAccess(id uint, resellerID *uint) error {
	_, err := w.getPeerByIDScoped(id, resellerID)
	return err
}

func (w *WgPeer) GetPeerByID(id uint, resellerID *uint) (*schema.PeerResponse, error) {
	peer, err := w.getPeerByIDScoped(id, resellerID)
	if err != nil {
		return nil, err
	}

	resp := w.transformPeerToResponse(peer)

	mtPeer, err := w.mikrotikAdaptor.FetchWgPeer(context.Background(), peer.PeerID)
	if err == nil {
		_, isOnline, handshakeErr := w.handshakeData(mtPeer)
		if handshakeErr == nil {
			resp.IsOnline = isOnline
		}
	}

	return &resp, nil
}

func (w *WgPeer) getPeerByIDScoped(id uint, resellerID *uint) (model.Peer, error) {
	var peer model.Peer
	query := w.db.Where("id = ?", id)
	if resellerID != nil {
		query = query.Where("reseller_id = ?", *resellerID)
	}
	if err := query.First(&peer).Error; err != nil {
		return model.Peer{}, err
	}

	return peer, nil
}
