package service

import (
	"crypto/ed25519"
	"encoding/base64"
	"errors"
	"fmt"
	"os"
	"strconv"
	"strings"
	"sync"
	"time"

	"github.com/maahdima/mwp/api/config"
	"github.com/maahdima/mwp/api/dataservice/model"
	"github.com/maahdima/mwp/api/license"
	"github.com/maahdima/mwp/api/license/binaryintegrity"

	"go.uber.org/zap"
	"gorm.io/gorm"
)

const (
	systemConfigLicenseKeyKey     = "license_key"
	systemConfigPublicKeyKey      = "license_public_key"
	systemConfigLicenseBaseURLKey = "license_base_url"
	// systemConfigLastValidResponseKey caches the raw bytes of the last
	// signed, Valid=true response license-panel returned for this install.
	// See restoreFromCachedValidResponse for how this is used as a
	// fallback when a live check is unexpectedly rejected.
	systemConfigLastValidResponseKey = "license_last_valid_response"
)

// dateLayoutsTried are the formats attempted, in order, when parsing
// ValidateResponse.ExpiresAt locally (see isExpired). license-panel's own
// wire format isn't controlled by this codebase (see proto.go's doc
// comment), so this defensively tries the layouts a Go time.Time or a
// plain date string would realistically be serialized as, rather than
// assuming one exact layout.
var dateLayoutsTried = []string{
	time.RFC3339,
	time.RFC3339Nano,
	"2006-01-02",
}

// isExpired reports whether expiresAt (ValidateResponse.ExpiresAt) is a
// date in the past. An empty string (license-panel's convention for "no
// expiry", e.g. an unlimited/lifetime plan) is never expired. A value
// that fails to parse under every known layout is treated as "not
// expired" -- fail open, not closed: this function is only ever
// consulted as a fallback after a live check already failed, and refusing
// to lock the panel over an unparseable date is far safer than locking a
// paying customer out over a formatting quirk.
func isExpired(expiresAt string) bool {
	if expiresAt == "" {
		return false
	}
	for _, layout := range dateLayoutsTried {
		if t, err := time.Parse(layout, expiresAt); err == nil {
			return time.Now().After(t)
		}
	}
	return false
}

// gracePeriod is how long MWPanel keeps running on its last known-valid
// signed response after the license server becomes unreachable, before
// treating the license as invalid. See LicenseService.currentStatus for
// where this is applied.
const gracePeriod = 72 * time.Hour

// LicenseStatus is the in-memory, currently-known license state, safe to
// read concurrently (see LicenseService.Status). It backs both the
// request-blocking middleware and the admin UI's license page.
type LicenseStatus struct {
	Activated bool
	Valid     bool
	Reason    string
	PlanName  string
	// IssuedAt/ExpiresAt together let a caller compute this license's
	// elapsed-life percentage (see checkExpiryWarning) -- IssuedAt was
	// already on the wire (ValidateResponse.IssuedAt) but previously
	// discarded here, since nothing read it before this field existed.
	IssuedAt      string
	ExpiresAt     string
	ServerCount   int
	MaxServers    int
	LastCheckedAt time.Time
	// InGracePeriod is true when the last successful check was Valid, the
	// license server is currently unreachable, but the grace period has
	// not yet elapsed -- MWPanel keeps running, but the admin UI should
	// visibly warn that connectivity to the license server needs to be
	// restored soon.
	InGracePeriod bool

	// Restricted/FreeTierLimits implement phase 4-12's granular free-tier
	// system: when Restricted is true (Valid is ALSO true -- see
	// license.ValidateResponse's own doc comment), this license has
	// expired but is configured to degrade to reduced caps instead of
	// locking the panel outright. FreeTierLimits is the admin-extensible
	// key-value table license-panel sent on the most recent successful
	// heartbeat -- read it via GetFreeTierLimit, never directly, so every
	// call site shares the identical "missing key = unlimited" fallback.
	Restricted     bool
	FreeTierLimits map[string]int64
}

// LicenseService owns MWPanel's side of activation/heartbeat against
// license-panel (see D:\license-panel). It is deliberately fail-open
// during the grace period and fail-closed only after it: a license
// server outage should degrade gracefully, not instantly lock every
// customer out of their own panel.
type LicenseService struct {
	db          *gorm.DB
	cfg         config.LicenseConfig
	client      *license.Client
	appMode     string // AppConfig.Mode -- see EnsureActivated's "no public key" branch
	dataDirPath string // AppConfig.DataDirPath -- see license.EnsureInstallID's doc comment on why this exact directory
	logger      *zap.Logger

	// botNotifier is optional (nil-safe, see SetBotNotifier's own doc
	// comment) -- used only by checkExpiryWarning to alert the admin as
	// their license approaches expiry.
	botNotifier *BotNotifier

	// freeTierDownsizers implements the retroactive side of phase 4-12's
	// free-tier system -- see SetFreeTierDownsizers' own doc comment.
	freeTierDownsizers map[string]FreeTierDownsizer

	mu             sync.RWMutex
	status         LicenseStatus
	lastValidCheck time.Time
}

// FreeTierDownsizer is the narrow capability each of the 6 resource
// services (Reseller/WgPeer/UserManagerService/V2RayPackageService/
// DNSAccountService/ApplicationService) exposes for phase 4-12's
// retroactive downsizing -- injected rather than imported directly, same
// "avoid a constructor-order/import-cycle dependency" reasoning as
// freeTierLimiter (reseller.go) itself. Exported (unlike freeTierLimiter)
// solely so cmd/http-server/http-server.go, a different package, can name
// it in the map literal passed to SetFreeTierDownsizers. See e.g.
// WgPeer.SuspendOldestForFreeTier's own doc comment for the full
// DB-only/deferred-reconciliation contract every implementation follows.
type FreeTierDownsizer interface {
	SuspendOldestForFreeTier(cap int64) (int, error)
}

func NewLicenseService(db *gorm.DB, cfg config.LicenseConfig, appMode string, dataDirPath string) *LicenseService {
	return &LicenseService{
		db:          db,
		cfg:         cfg,
		client:      license.NewClient(cfg.BaseURL),
		appMode:     appMode,
		dataDirPath: dataDirPath,
		logger:      zap.L().Named("LicenseService"),
	}
}

// SetBotNotifier wires the admin-alert capability in after construction --
// safe to leave unset (checkExpiryWarning simply no-ops), mirroring every
// other SetBotNotifier call site in this codebase (Authentication/WgPeer/
// Reseller all follow the identical "optional collaborator" convention).
func (s *LicenseService) SetBotNotifier(notifier *BotNotifier) {
	s.botNotifier = notifier
}

// SetFreeTierDownsizers wires the retroactive-downsizing capability in
// after construction -- safe to leave unset (enforceFreeTierDownsizing
// simply no-ops, so an install that never calls this behaves exactly like
// before this feature existed). key must match the same free-tier limit
// key GetFreeTierLimit is queried with (e.g. "max_peers"); see
// cmd/http-server/http-server.go's wiring call for the full set of 6.
func (s *LicenseService) SetFreeTierDownsizers(downsizers map[string]FreeTierDownsizer) {
	s.freeTierDownsizers = downsizers
}

// enforceFreeTierDownsizing is phase 4-12's retroactive-downsizing step,
// called from applySignedResponse every time a heartbeat/activate reports
// Restricted=true. For each configured free-tier limit key, it asks that
// resource's downsizer to suspend however many of its OLDEST active rows
// are needed to bring the count back under the (possibly just-lowered) cap
// -- see WgPeer.SuspendOldestForFreeTier's own doc comment for the full
// "oldest first, DB-only, deferred external reconciliation" contract every
// implementation shares.
//
// Deliberately unconditional on every Restricted heartbeat, not just a
// detected false->true transition: each downsizer's own query already
// excludes already-suspended rows (making a repeat call a cheap no-op once
// converged), and re-running this on every tick is what makes a cap the
// admin lowers AGAIN while already Restricted (e.g. 50 -> 20) get enforced
// on the very next heartbeat, without this service needing to track
// "did the cap change since last time" itself.
func (s *LicenseService) enforceFreeTierDownsizing(limits map[string]int64) {
	if len(s.freeTierDownsizers) == 0 || len(limits) == 0 {
		return
	}

	for key, downsizer := range s.freeTierDownsizers {
		limit, ok := limits[key]
		if !ok || limit <= 0 {
			continue
		}
		suspended, err := downsizer.SuspendOldestForFreeTier(limit)
		if err != nil {
			s.logger.Error("failed to enforce free-tier downsizing", zap.String("key", key), zap.Error(err))
			continue
		}
		if suspended > 0 {
			s.logger.Warn("suspended oldest excess resources to enforce lowered free-tier cap",
				zap.String("key", key), zap.Int64("cap", limit), zap.Int("suspended_count", suspended))
		}
	}
}

// Status returns the current, in-memory license state for the admin UI
// and the request-gating middleware to read. Safe for concurrent use.
func (s *LicenseService) Status() LicenseStatus {
	s.mu.RLock()
	defer s.mu.RUnlock()
	return s.status
}

// GetFreeTierLimit is the ONE way every enforcement point in this codebase
// should read a free-tier cap (فاز ۴-۱۲) -- never read
// Status().FreeTierLimits directly, so every call site shares the
// identical fallback rules below. Returns (0, false) whenever there is no
// active cap for key, which callers must treat as "unlimited, do not
// enforce anything" -- covering three distinct cases uniformly: (1) the
// license isn't Restricted at all (a paying, non-expired customer), (2) it
// IS Restricted but the admin never configured this particular key in
// license-panel's free-tier limits table, or (3) the key exists but its
// configured value is <= 0 (a deliberate "no cap" sentinel, since a
// genuine cap is always a positive count of allowed resources).
func (s *LicenseService) GetFreeTierLimit(key string) (int64, bool) {
	status := s.Status()
	if !status.Restricted {
		return 0, false
	}
	limit, ok := status.FreeTierLimits[key]
	if !ok || limit <= 0 {
		return 0, false
	}
	return limit, true
}

// IsBlocking reports whether the current status should cause API
// requests to be refused. False during a normal grace-period outage
// (see gracePeriod) -- only false-to-true once genuinely revoked/expired
// (confirmed by a real signed response) or once the grace period has
// fully elapsed with no successful contact.
func (s *LicenseService) IsBlocking() bool {
	status := s.Status()
	if !status.Activated {
		return true
	}
	if status.Valid {
		return false
	}
	return !status.InGracePeriod
}

// EnsureActivated runs once at startup. If no license key has been
// activated yet, it uses cfg.LicenseKey (from LICENSE_KEY env, set once
// during install) to activate. If a key was already activated in a
// previous run, this instead performs a heartbeat, so a restart doesn't
// count as a second server activation.
func (s *LicenseService) EnsureActivated() error {
	s.checkBinaryIntegrity()

	s.ensureBaseURL()
	s.ensurePublicKey()

	if s.cfg.PublicKeyB64 == "" {
		// License enforcement is only ever disabled in local development
		// (MODE=development with no public key configured); any other
		// MODE value blocks the panel with an actionable reason instead.
		if s.appMode == "development" {
			s.logger.Warn("LICENSE_PUBLIC_KEY is not configured and MODE=development -- license enforcement is disabled for this install")
			s.setStatus(LicenseStatus{Activated: true, Valid: true, Reason: "license enforcement disabled (development mode, no public key configured)"}, true)
			return nil
		}

		s.logger.Error("LICENSE_PUBLIC_KEY is not configured -- refusing to start unlicensed (set MODE=development to bypass this for local development only)")
		s.setStatus(LicenseStatus{Activated: false, Valid: false, Reason: "LICENSE_PUBLIC_KEY is not configured"}, false)
		return nil
	}

	storedKey, err := s.getStoredLicenseKey()
	if err != nil {
		return fmt.Errorf("failed to read stored license key: %w", err)
	}

	licenseKey := storedKey
	if licenseKey == "" {
		licenseKey = s.cfg.LicenseKey
	}

	if licenseKey == "" {
		s.logger.Error("no license key configured -- set LICENSE_KEY and restart to activate this install")
		s.setStatus(LicenseStatus{Activated: false, Valid: false, Reason: "no license key configured"}, false)
		return nil
	}

	if storedKey == "" {
		if err := s.activate(licenseKey); err != nil {
			s.logger.Error("license activation failed", zap.Error(err))
			s.setStatus(LicenseStatus{Activated: false, Valid: false, Reason: err.Error()}, false)
			return nil
		}
		if err := s.storeLicenseKey(licenseKey); err != nil {
			s.logger.Error("failed to persist activated license key", zap.Error(err))
		}
		return nil
	}

	s.heartbeatOnce(licenseKey)
	return nil
}

// StartHeartbeatLoop runs Heartbeat on cfg.CheckIntervalSeconds forever.
// Intended to be launched with `go service.StartHeartbeatLoop()` once at
// startup, after EnsureActivated's first synchronous check.
func (s *LicenseService) StartHeartbeatLoop() {
	interval := time.Duration(s.cfg.CheckIntervalSeconds) * time.Second
	if interval <= 0 {
		interval = time.Hour
	}

	ticker := time.NewTicker(interval)
	defer ticker.Stop()

	for range ticker.C {
		storedKey, err := s.getStoredLicenseKey()
		if err != nil || storedKey == "" {
			continue
		}
		s.heartbeatOnce(storedKey)
	}
}

// ActivateWithKey is the admin-facing entry point for submitting a license
// key from the web UI's activation page (see http/license.go's Activate
// handler) -- unlike EnsureActivated, which only runs once at startup, this
// can be called any time (first activation on a never-licensed install, or
// re-activation with a new key after a plan change). On success the key is
// persisted the same way a startup activation would be, so future restarts
// heartbeat instead of re-activating.
func (s *LicenseService) ActivateWithKey(licenseKey string) (*LicenseStatus, error) {
	s.ensureBaseURL()
	s.ensurePublicKey()
	if s.cfg.PublicKeyB64 == "" {
		return nil, fmt.Errorf("could not reach the license server to fetch its public key -- check connectivity to %s and try again", s.cfg.BaseURL)
	}
	if licenseKey == "" {
		return nil, fmt.Errorf("license key is required")
	}

	if err := s.activate(licenseKey); err != nil {
		return nil, err
	}
	if err := s.storeLicenseKey(licenseKey); err != nil {
		s.logger.Error("failed to persist activated license key", zap.Error(err))
		return nil, fmt.Errorf("license was accepted but could not be saved, please try again: %w", err)
	}

	status := s.Status()
	return &status, nil
}

// StartTrial is the entry point for the web UI's "Start Free Trial" tab
// (see http/license.go's Trial handler) -- the self-service counterpart to
// ActivateWithKey. Unlike activation with a purchased key, MWPanel doesn't
// know the license key ahead of time here: license-panel generates one and
// hands it back inside the signed response's Payload.LicenseKey, which is
// what gets persisted on success. Reachable under the same no-JWT
// exemption as /license/activate (see middleware.licenseMiddlewareExemptPaths)
// and for the same reason: while unlicensed, nothing else on the API is
// reachable to get here through.
func (s *LicenseService) StartTrial(email string) (*LicenseStatus, error) {
	s.ensureBaseURL()
	s.ensurePublicKey()
	if s.cfg.PublicKeyB64 == "" {
		return nil, fmt.Errorf("could not reach the license server to fetch its public key -- check connectivity to %s and try again", s.cfg.BaseURL)
	}
	if email == "" {
		return nil, fmt.Errorf("email is required")
	}

	hostname, _ := os.Hostname()
	req := license.TrialRequest{
		Email:       email,
		Fingerprint: license.Fingerprint(s.dataDirPath),
		Hostname:    hostname,
		MWPVersion:  mwpVersion(),
	}

	raw, err := s.client.Trial(req)
	if err != nil {
		// A non-200 here (trials disabled, this fingerprint already
		// claimed a trial) is a genuine request failure, not a signed
		// "invalid" response -- there is no license to attach a rejection
		// to server-side (see license.Client.Trial's doc comment). Surface
		// it directly rather than trying to force it through
		// applySignedResponse.
		return nil, err
	}

	signed, err := license.VerifySignedResponse(s.cfg.PublicKeyB64, raw)
	if err != nil {
		return nil, fmt.Errorf("received an unverifiable response from the license server: %w", err)
	}
	if !signed.Payload.Valid {
		// Shouldn't normally happen (license-panel only ever signs a
		// valid=true payload for a freshly-issued trial), but handle it
		// defensively rather than persisting a key that's already invalid.
		return nil, fmt.Errorf("trial license was rejected: %s", signed.Payload.Reason)
	}

	if err := s.storeLicenseKey(signed.Payload.LicenseKey); err != nil {
		s.logger.Error("failed to persist trial license key", zap.Error(err))
		return nil, fmt.Errorf("trial was granted but could not be saved, please try again: %w", err)
	}

	if err := s.applySignedResponse(raw); err != nil {
		return nil, err
	}

	s.logger.Info("free trial started", zap.String("email", email))

	status := s.Status()
	return &status, nil
}

func (s *LicenseService) activate(licenseKey string) error {
	hostname, _ := os.Hostname()
	req := license.ActivateRequest{
		LicenseKey:  licenseKey,
		Fingerprint: license.Fingerprint(s.dataDirPath),
		Hostname:    hostname,
		MWPVersion:  mwpVersion(),
	}

	raw, err := s.client.Activate(req)
	if err != nil {
		return err
	}

	return s.applySignedResponse(raw)
}

func (s *LicenseService) heartbeatOnce(licenseKey string) {
	fingerprint := license.Fingerprint(s.dataDirPath)
	req := license.HeartbeatRequest{
		LicenseKey:  licenseKey,
		Fingerprint: fingerprint,
		MWPVersion:  mwpVersion(),
	}

	s.logger.Debug("sending license heartbeat", zap.String("fingerprint", fingerprint))

	raw, err := s.client.Heartbeat(req)
	if err != nil {
		// Network-level failure (timeout, DNS, connection refused, non-200)
		// -- the license server may simply be unreachable. This is the ONE
		// case the 72h grace period covers (see applyUnreachable's doc
		// comment); it is NOT the same as the server actively rejecting
		// this fingerprint, which locks immediately with no grace (see the
		// Warn log a few lines below for that case).
		s.logger.Warn("license heartbeat request failed -- will retry on next interval", zap.Error(err))
		s.applyUnreachable(err.Error())
		return
	}

	if err := s.applySignedResponse(raw); err != nil {
		s.logger.Error("failed to apply license heartbeat response", zap.Error(err))
		s.applyUnreachable(err.Error())
		return
	}

	status := s.Status()
	if status.Valid {
		s.logger.Debug("license heartbeat succeeded", zap.String("fingerprint", fingerprint))
		return
	}

	// The license server was reachable and answered, but rejected this
	// install. Before locking the panel out, fall back to the last
	// signature-verified response this install ever received -- see
	// restoreFromCachedValidResponse.
	if s.restoreFromCachedValidResponse(fingerprint, status.Reason) {
		return
	}

	s.logger.Warn("license heartbeat rejected by server -- locking immediately (no grace period for an explicit rejection)",
		zap.String("fingerprint", fingerprint), zap.String("reason", status.Reason))
}

// restoreFromCachedValidResponse is the fallback consulted when a live
// heartbeat/activation is rejected by license-panel. It re-verifies the
// last signed response this install successfully cached (see
// systemConfigLastValidResponseKey) and, only if that cached license's
// own ExpiresAt is still in the future, restores it as the current status
// so the panel keeps running. Returns false (no override applied) if
// there's no cache, it fails to verify, or it has itself expired -- in
// every one of those cases the caller's original rejection stands.
//
// This deliberately does NOT re-trust an expired cached response, and
// does NOT touch StartTrial/the free-trial path at all -- a lapsed trial
// or a genuinely expired license must still fall through to the
// activation screen, exactly as the live check already says.
func (s *LicenseService) restoreFromCachedValidResponse(fingerprint, rejectReason string) bool {
	cachedRaw, err := s.getSystemConfig(systemConfigLastValidResponseKey)
	if err != nil || cachedRaw == "" {
		return false
	}

	signed, err := license.VerifySignedResponse(s.cfg.PublicKeyB64, []byte(cachedRaw))
	if err != nil {
		s.logger.Warn("cached license response failed signature verification, discarding", zap.Error(err))
		return false
	}

	payload := signed.Payload
	if !payload.Valid || isExpired(payload.ExpiresAt) {
		return false
	}

	s.logger.Warn("live license check was rejected, but a cached, signature-verified, still-unexpired license was found -- keeping this install unlocked",
		zap.String("fingerprint", fingerprint),
		zap.String("live_rejection_reason", rejectReason),
		zap.String("cached_expires_at", payload.ExpiresAt))

	s.setStatus(LicenseStatus{
		Activated:     true,
		Valid:         true,
		Reason:        "using cached license (last live check: " + rejectReason + ")",
		PlanName:      payload.PlanName,
		ExpiresAt:     payload.ExpiresAt,
		ServerCount:   payload.ServerCount,
		MaxServers:    payload.MaxServers,
		LastCheckedAt: time.Now(),
	}, true)
	return true
}

func (s *LicenseService) applySignedResponse(raw []byte) error {
	signed, err := license.VerifySignedResponse(s.cfg.PublicKeyB64, raw)
	if err != nil {
		return err
	}

	payload := signed.Payload
	now := time.Now()

	if payload.Valid {
		s.mu.Lock()
		s.lastValidCheck = now
		s.mu.Unlock()

		// Best-effort cache of the raw, already-verified bytes -- see
		// systemConfigLastValidResponseKey's doc comment. Never fails the
		// caller; a failure here just means a future fingerprint-mismatch
		// scenario won't have a fallback to use, same as if this cache
		// didn't exist at all.
		if err := s.setSystemConfig(systemConfigLastValidResponseKey, string(raw)); err != nil {
			s.logger.Warn("failed to cache last valid license response", zap.Error(err))
		}
	}

	s.setStatus(LicenseStatus{
		Activated:      true,
		Valid:          payload.Valid,
		Reason:         payload.Reason,
		PlanName:       payload.PlanName,
		IssuedAt:       payload.IssuedAt,
		ExpiresAt:      payload.ExpiresAt,
		ServerCount:    payload.ServerCount,
		MaxServers:     payload.MaxServers,
		LastCheckedAt:  now,
		Restricted:     payload.Restricted,
		FreeTierLimits: payload.FreeTierLimits,
	}, payload.Valid)

	if !payload.Valid {
		s.logger.Warn("license server reports this install is not valid", zap.String("reason", payload.Reason))
	}
	if payload.Restricted {
		s.logger.Warn("license has expired and is now running in restricted (free-tier) mode",
			zap.Int("limit_count", len(payload.FreeTierLimits)))
		s.enforceFreeTierDownsizing(payload.FreeTierLimits)
	}

	if payload.Valid {
		s.checkExpiryWarning(payload.IssuedAt, payload.ExpiresAt)
	}

	return nil
}

// checkBinaryIntegrity checks the running mwp executable's signature
// against its expected release signature, to catch accidental
// substitution (e.g. a stale binary from a botched deploy, or file
// corruption). Called once from EnsureActivated, before any network
// activity. Never blocks startup: an unconfigured key or an unsigned
// binary are both treated as expected, benign states; only an actual
// signature mismatch is logged as an error and (if wired) raises an
// admin alert.
func (s *LicenseService) checkBinaryIntegrity() {
	publicKeyB64 := config.GetReleasePublicKey()
	if publicKeyB64 == "" {
		return
	}
	publicKey, err := decodeReleasePublicKey(publicKeyB64)
	if err != nil {
		s.logger.Warn("RELEASE_PUBLIC_KEY is configured but invalid -- skipping binary integrity check", zap.Error(err))
		return
	}

	execPath, err := os.Executable()
	if err != nil {
		s.logger.Warn("failed to resolve this process's own executable path -- skipping binary integrity check", zap.Error(err))
		return
	}

	err = binaryintegrity.Verify(execPath, publicKey)
	switch {
	case err == nil:
		return
	case errors.Is(err, binaryintegrity.ErrNotSigned):
		s.logger.Info("this mwp binary has no signature file -- binary integrity check skipped (expected for an unsigned/dev build)")
	case errors.Is(err, binaryintegrity.ErrMismatch):
		s.logger.Error("mwp binary signature does not match its current contents -- this executable may have been substituted or modified after it was released", zap.String("path", execPath))
		if s.botNotifier != nil {
			s.botNotifier.NotifyCriticalAlert(fmt.Sprintf(
				"امضای فایل اجرایی پنل با محتوای فعلی آن مطابقت ندارد.\nمسیر: %s\nاین ممکن است نشانه‌ی جایگزینی یا دستکاری فایل اجرایی باشد. لطفاً بررسی کنید.",
				execPath,
			))
		}
	default:
		s.logger.Warn("failed to verify binary integrity", zap.Error(err))
	}
}

func decodeReleasePublicKey(b64 string) (ed25519.PublicKey, error) {
	decoded, err := base64.StdEncoding.DecodeString(b64)
	if err != nil {
		return nil, fmt.Errorf("decoding release public key: %w", err)
	}
	if len(decoded) != ed25519.PublicKeySize {
		return nil, fmt.Errorf("release public key has wrong length: got %d, want %d", len(decoded), ed25519.PublicKeySize)
	}
	return ed25519.PublicKey(decoded), nil
}

// expiryWarningThresholds are the elapsed-life percentages the admin's own
// explicit requirement asks to be warned at (فاز ۴-۱۲-الف) -- checked in
// descending order so a single tick that skips straight past an earlier
// threshold (e.g. the panel was offline for a while, or a long
// heartbeat interval) still fires the highest one actually crossed, not
// every one in sequence.
var expiryWarningThresholds = []int{99, 90, 80, 50}

// systemConfigExpiryWarningKey persists the highest threshold already
// warned about for the CURRENT license period (keyed loosely off ExpiresAt
// itself, see checkExpiryWarning's own doc comment) -- without this, every
// heartbeat tick after crossing a threshold would re-send the same alert
// (heartbeats run hourly by default, warnings should fire once per
// threshold, not once per hour).
const systemConfigExpiryWarningKey = "license_expiry_warning_state"

// checkExpiryWarning implements the admin's explicit "هشدار پیش از انقضا
// در ۵۰٪، ۸۰٪، ۹۰٪، ۹۹٪ از عمر لایسنس" requirement (فاز ۴-۱۲-الف). Called
// from applySignedResponse on every successful heartbeat/activate -- a
// no-op if issuedAt/expiresAt don't both parse (e.g. a lifetime/no-expiry
// plan, where ExpiresAt is empty by license-panel's own convention, or
// license-panel's format changes) or if botNotifier was never wired (see
// SetBotNotifier's own doc comment).
//
// The persisted state is a single "expiresAt|highestThresholdWarned"
// string rather than a plain int, specifically so a license RENEWAL
// (a new, later ExpiresAt from an upgrade/manual extension) is detected by
// noticing the stored ExpiresAt no longer matches the current one, and the
// threshold resets to 0 -- otherwise a renewed license would silently
// never warn again this cycle (the old high-water mark would look
// "already warned" forever) or, worse, immediately re-fire a stale warning
// if the new period happens to compute a similar elapsed percentage.
func (s *LicenseService) checkExpiryWarning(issuedAt, expiresAt string) {
	if s.botNotifier == nil || expiresAt == "" {
		return
	}

	issued, ok := parseLicenseDate(issuedAt)
	if !ok {
		return
	}
	expires, ok := parseLicenseDate(expiresAt)
	if !ok {
		return
	}

	totalLife := expires.Sub(issued)
	if totalLife <= 0 {
		return
	}
	elapsed := time.Since(issued)
	percentElapsed := int(elapsed * 100 / totalLife)

	highestCrossed := 0
	for _, threshold := range expiryWarningThresholds {
		if percentElapsed >= threshold {
			highestCrossed = threshold
			break
		}
	}
	if highestCrossed == 0 {
		return
	}

	stored, err := s.getSystemConfig(systemConfigExpiryWarningKey)
	if err != nil {
		s.logger.Warn("failed to read license expiry warning state, skipping this tick", zap.Error(err))
		return
	}
	storedExpiresAt, storedThreshold := "", 0
	if stored != "" {
		if parts := strings.SplitN(stored, "|", 2); len(parts) == 2 {
			storedExpiresAt = parts[0]
			storedThreshold, _ = strconv.Atoi(parts[1])
		}
	}
	if storedExpiresAt != expiresAt {
		// A different ExpiresAt than last time this ran means either the
		// license was just renewed (reset to 0, per this function's own
		// doc comment) or this is the very first check for this install --
		// either way, storedThreshold from a DIFFERENT period must never
		// suppress a warning for THIS period.
		storedThreshold = 0
	}
	if highestCrossed <= storedThreshold {
		return // already warned for this threshold (or higher) this period
	}

	remainingDays := int(time.Until(expires).Hours() / 24)
	s.botNotifier.NotifyCriticalAlert(fmt.Sprintf(
		"لایسنس این پنل به %d%% از عمر خود رسیده است.\nتاریخ انقضا: %s\nروزهای باقی‌مانده: %d\nلطفاً برای تمدید لایسنس اقدام کنید.",
		highestCrossed, expiresAt, remainingDays,
	))

	if err := s.setSystemConfig(systemConfigExpiryWarningKey, fmt.Sprintf("%s|%d", expiresAt, highestCrossed)); err != nil {
		s.logger.Warn("failed to persist license expiry warning state -- may re-warn on the next tick", zap.Error(err))
	}
}

// parseLicenseDate tries every layout isExpired itself already tries
// (dateLayoutsTried), for the identical "license-panel's own wire format
// isn't controlled by this codebase" reason.
func parseLicenseDate(value string) (time.Time, bool) {
	for _, layout := range dateLayoutsTried {
		if t, err := time.Parse(layout, value); err == nil {
			return t, true
		}
	}
	return time.Time{}, false
}

// applyUnreachable is called when the license server cannot be reached at
// all (network error, timeout, non-200). It does not overwrite Valid --
// the UI derives InGracePeriod from lastValidCheck vs. gracePeriod
// instead, so a transient outage never claims "your license is invalid"
// when the truth is simply "we couldn't ask."
func (s *LicenseService) applyUnreachable(reason string) {
	s.mu.Lock()
	defer s.mu.Unlock()

	inGrace := !s.lastValidCheck.IsZero() && time.Since(s.lastValidCheck) < gracePeriod

	s.status.LastCheckedAt = time.Now()
	s.status.InGracePeriod = inGrace
	if !inGrace {
		s.status.Valid = false
		s.status.Reason = "license server unreachable and grace period has elapsed: " + reason
	}
}

func (s *LicenseService) setStatus(status LicenseStatus, valid bool) {
	s.mu.Lock()
	defer s.mu.Unlock()
	status.InGracePeriod = false
	if valid {
		s.lastValidCheck = time.Now()
	}
	s.status = status
}

// getSystemConfig/setSystemConfig are the shared get/store primitives
// behind every SystemConfig-backed value this service persists (license
// key, public key, locked base URL) -- one key/value table, three
// distinct keys (systemConfigLicenseKeyKey/systemConfigPublicKeyKey/
// systemConfigLicenseBaseURLKey).
func (s *LicenseService) getSystemConfig(key string) (string, error) {
	var row model.SystemConfig
	err := s.db.Where("key = ?", key).First(&row).Error
	if errors.Is(err, gorm.ErrRecordNotFound) {
		return "", nil
	}
	if err != nil {
		return "", err
	}
	return row.Value, nil
}

func (s *LicenseService) setSystemConfig(key, value string) error {
	var existing model.SystemConfig
	err := s.db.Where("key = ?", key).First(&existing).Error
	if err == nil {
		existing.Value = value
		return s.db.Save(&existing).Error
	}
	if !errors.Is(err, gorm.ErrRecordNotFound) {
		return err
	}
	return s.db.Create(&model.SystemConfig{Key: key, Value: value}).Error
}

func (s *LicenseService) getStoredLicenseKey() (string, error) {
	return s.getSystemConfig(systemConfigLicenseKeyKey)
}

func (s *LicenseService) storeLicenseKey(key string) error {
	return s.setSystemConfig(systemConfigLicenseKeyKey, key)
}

func (s *LicenseService) getStoredPublicKey() (string, error) {
	return s.getSystemConfig(systemConfigPublicKeyKey)
}

func (s *LicenseService) storePublicKey(key string) error {
	return s.setSystemConfig(systemConfigPublicKeyKey, key)
}

// ensureBaseURL locks in the license server address the FIRST time this
// service ever resolves one, and ignores cfg.BaseURL (i.e. the
// LICENSE_SERVER_URL env var) on every boot after that -- see
// licenseServerBaseURLDefault's doc comment (api/config/config.go) for
// the full rationale: this is what stops an already-activated install
// from being silently redirected to a different license server later by
// anyone who can edit .env, while still letting a genuinely fresh
// install be pointed at a specific deployment without a source rebuild.
// Mutates s.cfg.BaseURL and rebuilds s.client in place so every other
// call site in this file that already reads/uses those keeps working
// unchanged.
func (s *LicenseService) ensureBaseURL() {
	locked, err := s.getSystemConfig(systemConfigLicenseBaseURLKey)
	if err != nil {
		s.logger.Warn("failed to read locked license server URL, using configured value for this run", zap.Error(err))
		return
	}

	if locked != "" {
		if locked != s.cfg.BaseURL {
			s.logger.Info("using previously-locked license server URL, ignoring LICENSE_SERVER_URL",
				zap.String("locked_url", locked), zap.String("configured_url", s.cfg.BaseURL))
			s.cfg.BaseURL = locked
			s.client = license.NewClient(locked)
		}
		return
	}

	// First time ever resolving a base URL for this install -- lock in
	// whatever cfg.BaseURL currently is (the env var if set, otherwise
	// licenseServerBaseURLDefault).
	if err := s.setSystemConfig(systemConfigLicenseBaseURLKey, s.cfg.BaseURL); err != nil {
		s.logger.Warn("failed to persist locked license server URL -- it may still be changed via LICENSE_SERVER_URL on the next restart", zap.Error(err))
		return
	}
	s.logger.Info("locked license server URL for this install", zap.String("url", s.cfg.BaseURL))
}

// ensurePublicKey resolves the Ed25519 public key used to verify every
// license-panel response, without requiring an admin to manually copy
// LICENSE_PUBLIC_KEY into .env: an explicitly-configured env value always
// wins, otherwise this reuses whatever was cached from a previous
// successful fetch, and only fetches a fresh copy from license-panel when
// neither is available (e.g. the very first boot of a fresh install). The
// fetched key is cached in SystemConfig so subsequent restarts never need
// network access for this.
func (s *LicenseService) ensurePublicKey() {
	if s.cfg.PublicKeyB64 != "" {
		return
	}

	cached, err := s.getStoredPublicKey()
	if err != nil {
		s.logger.Warn("failed to read cached license public key", zap.Error(err))
	} else if cached != "" {
		s.cfg.PublicKeyB64 = cached
		return
	}

	fetched, err := s.client.GetPublicKey()
	if err != nil {
		s.logger.Warn("failed to fetch license public key from license server -- will retry on next check",
			zap.String("license_server", s.cfg.BaseURL), zap.Error(err))
		return
	}

	if err := s.storePublicKey(fetched); err != nil {
		s.logger.Warn("failed to cache fetched license public key", zap.Error(err))
	}
	s.logger.Info("fetched and cached license public key from license server automatically")
	s.cfg.PublicKeyB64 = fetched
}

func (s *LicenseService) clearStoredLicenseKey() error {
	return s.db.Where("key = ?", systemConfigLicenseKeyKey).Delete(&model.SystemConfig{}).Error
}

// Revoke is the admin-facing entry point for "Remove License" on the
// Settings > License page. It deliberately does NOT contact license-panel
// (there is no server-side "deactivate" concept -- a license key stays
// valid there until its own expiry/plan). This only clears MWPanel's own
// local record of having activated a key, so the next request is blocked
// (see IsBlocking) and the activation screen reappears -- letting an admin
// cleanly detach this install before pointing it at a different license
// key (e.g. when moving a server, or handing this key to another install)
// without waiting for a stale heartbeat to fail first.
func (s *LicenseService) Revoke() error {
	if err := s.clearStoredLicenseKey(); err != nil {
		return fmt.Errorf("failed to clear stored license key: %w", err)
	}
	s.setStatus(LicenseStatus{Activated: false, Valid: false, Reason: "license removed by admin"}, false)
	return nil
}

// CheckForUpdate asks license-panel whether a newer MWPanel release
// exists. Used by the admin UI's "Check for Updates" button/banner --
// never called automatically on a schedule, since installing an update
// is always an explicit admin action (see the earlier decision: no silent
// auto-install).
func (s *LicenseService) CheckForUpdate(channel string) (*license.UpdateCheckResponse, error) {
	storedKey, err := s.getStoredLicenseKey()
	if err != nil {
		return nil, err
	}

	return s.client.CheckUpdate(license.UpdateCheckRequest{
		LicenseKey:     storedKey,
		Fingerprint:    license.Fingerprint(s.dataDirPath),
		CurrentVersion: mwpVersion(),
		Channel:        channel,
	})
}

// StoredLicenseKey exposes this install's currently-activated license key
// to other services that need to authenticate their own calls to
// license-panel (currently just HelpCenterService) without duplicating
// EnsureActivated/ActivateWithKey's storage logic. Empty string + no error
// means "not yet activated" -- callers should treat that as "feature
// unavailable," not retry or error loudly, exactly like every other
// license-gated feature in this codebase.
func (s *LicenseService) StoredLicenseKey() (string, error) {
	return s.getStoredLicenseKey()
}

// Client exposes the already-configured license.Client (base URL, dev-mode
// override already applied) for other services to reuse rather than each
// constructing their own -- currently just HelpCenterService, which needs
// to call license-panel endpoints CheckForUpdate/ActivateWithKey don't
// cover.
func (s *LicenseService) Client() *license.Client {
	return s.client
}

// buildVersion should be set at build time via:
//
//	go build -ldflags="-X github.com/maahdima/mwp/api/service.buildVersion=1.2.3"
//
// mirroring how the frontend's __APP_VERSION__ is injected from
// package.json (see ui/vite.config.ts). Falls back to "dev" so a local,
// non-release build never reports a misleading version number to the
// license server.
var buildVersion = "dev"

func mwpVersion() string {
	return buildVersion
}

// MwpVersion exposes the running build's version to callers outside this
// package (e.g. the update-check HTTP handler, which reports it alongside
// the latest-available version so the admin UI can show "you're on X,
// latest is Y" without a second endpoint).
func MwpVersion() string {
	return mwpVersion()
}
