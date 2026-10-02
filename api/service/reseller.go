package service

import (
	"context"
	"errors"
	"fmt"
	"strconv"

	"github.com/maahdima/mwp/api/adaptor/mikrotik"
	"github.com/maahdima/mwp/api/dataservice/model"
	"github.com/maahdima/mwp/api/http/schema"
	"go.uber.org/zap"
	"golang.org/x/crypto/bcrypt"
	"gorm.io/gorm"
)

// v2RayResellerQuotaResumer is the narrow capability UpdateReseller needs
// to re-enable V2Ray packages this reseller's overall quota exhaustion
// had disabled (*V2RaySyncService.ResumePackagesForResellerQuota) --
// injected rather than imported directly, same reasoning as
// ResumeBillingSuspension's own v2rayResumer parameter (avoids a
// Reseller<->V2Ray service constructor cycle).
type v2RayResellerQuotaResumer interface {
	ResumePackagesForResellerQuota(resellerID uint)
}

// dnsResellerQuotaResumer mirrors v2RayResellerQuotaResumer exactly, for
// DNSSyncService.ResumeAccountsForResellerQuota -- same "injected rather
// than imported directly, avoids a Reseller<->DNS service constructor
// cycle" reasoning.
type dnsResellerQuotaResumer interface {
	ResumeAccountsForResellerQuota(resellerID uint)
}

// applicationResellerQuotaResumer mirrors dnsResellerQuotaResumer/
// v2RayResellerQuotaResumer exactly, for
// ApplicationService.ResumeApplicationsForResellerQuota.
type applicationResellerQuotaResumer interface {
	ResumeApplicationsForResellerQuota(resellerID uint)
}

// freeTierLimiter is the narrow capability phase 4-12's granular free-tier
// system needs from *LicenseService, injected rather than imported
// directly -- same "avoid a constructor-order/import-cycle dependency"
// reasoning as v2RayResellerQuotaResumer/dnsResellerQuotaResumer above.
// Every service that needs to enforce a free-tier cap (Reseller, WgPeer,
// UserManagerService, V2RayPackageService, DNSAccountService,
// ApplicationService) takes this same interface, so none of them needs to
// depend on LicenseService's concrete type or its own construction order.
type freeTierLimiter interface {
	GetFreeTierLimit(key string) (int64, bool)
}

type Reseller struct {
	db              *gorm.DB
	mikrotikAdaptor *mikrotik.Adaptor
	botNotifier     *BotNotifier
	v2rayResumer    v2RayResellerQuotaResumer
	dnsResumer      dnsResellerQuotaResumer
	appResumer      applicationResellerQuotaResumer
	wallet          *Wallet
	licenseLimiter  freeTierLimiter
	logger          *zap.Logger
}

func NewReseller(db *gorm.DB, mikrotikAdaptor *mikrotik.Adaptor) *Reseller {
	return &Reseller{
		db:              db,
		mikrotikAdaptor: mikrotikAdaptor,
		logger:          zap.L().Named("ResellerService"),
	}
}

// SetV2RayResumer wires the V2Ray reseller-quota resume capability in
// after construction (V2RaySyncService is constructed after Reseller
// during startup) -- safe to leave unset, in which case
// UpdateReseller's own v2rayQuotaRaised branch simply no-ops, matching
// SetBotNotifier's identical "safe to leave unset" convention.
func (r *Reseller) SetV2RayResumer(resumer v2RayResellerQuotaResumer) {
	r.v2rayResumer = resumer
}

// SetDNSResumer mirrors SetV2RayResumer exactly (DNSSyncService is
// constructed after Reseller during startup, same ordering constraint) --
// safe to leave unset, in which case UpdateReseller's own dnsQuotaRaised
// branch simply no-ops.
func (r *Reseller) SetDNSResumer(resumer dnsResellerQuotaResumer) {
	r.dnsResumer = resumer
}

// SetApplicationResumer mirrors SetDNSResumer exactly (ApplicationService is
// constructed after Reseller during startup, same ordering constraint) --
// safe to leave unset, in which case UpdateReseller's own
// applicationQuotaRaised branch simply no-ops.
func (r *Reseller) SetApplicationResumer(resumer applicationResellerQuotaResumer) {
	r.appResumer = resumer
}

// SetLicenseLimiter wires the free-tier cap lookup in (فاز ۴-۱۲) -- safe to
// leave unset, in which case CreateReseller's own cap check simply no-ops
// (GetFreeTierLimit would have returned "unlimited" anyway for a nil
// limiter, but skipping the call entirely avoids a nil-interface call
// site).
func (r *Reseller) SetLicenseLimiter(limiter freeTierLimiter) {
	r.licenseLimiter = limiter
}

// SetWallet wires the wallet lookup UpdateReseller's own billing-mode-switch
// reconciliation needs (bootstrapping a wallet row on Volume->Payment) --
// same "safe to leave unset, no-ops the feature it enables" convention as
// SetV2RayResumer/SetBotNotifier above.
func (r *Reseller) SetWallet(wallet *Wallet) {
	r.wallet = wallet
}

// SetBotNotifier wires the Telegram account-status-change notifier after
// construction, since BotNotifier depends on the bot service, which is
// constructed after Reseller during startup. Safe to leave unset --
// UpdateReseller's notification no-ops if nil.
func (r *Reseller) SetBotNotifier(notifier *BotNotifier) {
	r.botNotifier = notifier
}

// ErrFreeTierResellerLimitReached is returned by CreateReseller when the
// license is Restricted (فاز ۴-۱۲) and the "max_resellers" free-tier cap
// has already been reached -- a distinct sentinel (not a bare
// fmt.Errorf) so the HTTP layer can surface a specific, actionable message
// distinguishing "your license expired and downgraded" from an ordinary
// validation failure.
var ErrFreeTierResellerLimitReached = errors.New("this license has expired and is running in restricted mode: the reseller limit for the free tier has been reached")

// SuspendOldestForFreeTier is the retroactive side of phase 4-12's free-tier
// system for resellers: when a license newly transitions into Restricted
// mode (or the admin lowers an already-Restricted "max_resellers" cap) and
// more resellers already exist than the new cap allows, the OLDEST excess
// resellers (by CreatedAt) are deactivated via IsActive=false -- the same
// flag this codebase's own billing-suspension path
// (cmd/jobs/traffic.go's applyResellerQuota) already uses to lock a
// reseller out, so every existing "reseller is inactive" check elsewhere in
// the panel already honors this with no new code needed. Idempotent: only
// currently-active resellers are counted/targeted, so a repeat call with
// the same cap is a no-op once converged.
func (r *Reseller) SuspendOldestForFreeTier(cap int64) (int, error) {
	if cap <= 0 {
		return 0, nil
	}

	var activeCount int64
	if err := r.db.Model(&model.Reseller{}).Where("is_active = ?", true).Count(&activeCount).Error; err != nil {
		return 0, err
	}

	excess := activeCount - cap
	if excess <= 0 {
		return 0, nil
	}

	var toSuspend []model.Reseller
	if err := r.db.Where("is_active = ?", true).Order("created_at ASC").Limit(int(excess)).Find(&toSuspend).Error; err != nil {
		return 0, err
	}

	suspended := 0
	for _, res := range toSuspend {
		if err := r.db.Model(&model.Reseller{}).Where("id = ?", res.ID).Update("is_active", false).Error; err != nil {
			r.logger.Error("failed to suspend reseller for free-tier cap", zap.Uint("reseller_id", res.ID), zap.Error(err))
			continue
		}
		suspended++
	}

	return suspended, nil
}

func (r *Reseller) CreateReseller(req *schema.CreateResellerRequest) (*schema.ResellerResponse, error) {
	if r.licenseLimiter != nil {
		if maxResellers, ok := r.licenseLimiter.GetFreeTierLimit("max_resellers"); ok {
			var activeCount int64
			if err := r.db.Model(&model.Reseller{}).Count(&activeCount).Error; err != nil {
				return nil, err
			}
			if activeCount >= maxResellers {
				return nil, ErrFreeTierResellerLimitReached
			}
		}
	}

	var existing model.Reseller
	if err := r.db.First(&existing, "username = ?", req.Username).Error; err == nil {
		return nil, fmt.Errorf("username already exists")
	} else if !errors.Is(err, gorm.ErrRecordNotFound) {
		return nil, err
	}

	var admin model.Admin
	if err := r.db.First(&admin, "username = ?", req.Username).Error; err == nil {
		return nil, fmt.Errorf("username already exists")
	} else if !errors.Is(err, gorm.ErrRecordNotFound) {
		return nil, err
	}

	hashedPassword, err := bcrypt.GenerateFromPassword([]byte(req.Password), bcrypt.DefaultCost)
	if err != nil {
		r.logger.Error("failed to hash reseller password", zap.Error(err))
		return nil, err
	}

	res := model.Reseller{
		Name:           req.Name,
		Username:       req.Username,
		PasswordHash:   string(hashedPassword),
		Email:          req.Email,
		IsActive:       true,
		QuotaBytes:     req.QuotaBytes,
		MaxPeers:       req.MaxPeers,
		TelegramChatID: req.TelegramChatID,

		UserManagerQuotaBytes:  req.UserManagerQuotaBytes,
		UserManagerMaxAccounts: req.UserManagerMaxAccounts,

		V2RayQuotaBytes:  req.V2RayQuotaBytes,
		V2RayMaxPackages: req.V2RayMaxPackages,

		DNSQuotaBytes:  req.DNSQuotaBytes,
		DNSMaxAccounts: req.DNSMaxAccounts,

		ApplicationQuotaBytes: req.ApplicationQuotaBytes,
		ApplicationMaxCount:   req.ApplicationMaxCount,
	}
	if req.CanCreateUserManagerAccounts != nil {
		res.CanCreateUserManagerAccounts = *req.CanCreateUserManagerAccounts
	}
	if req.CanResellV2Ray != nil {
		res.CanResellV2Ray = *req.CanResellV2Ray
	}
	if req.CanResellDNS != nil {
		res.CanResellDNS = *req.CanResellDNS
	}
	if req.CanCreateApplications != nil {
		res.CanCreateApplications = *req.CanCreateApplications
	}
	if req.OtpEnabled != nil {
		res.OtpEnabled = *req.OtpEnabled
	}
	if req.BillingMode != nil {
		res.BillingMode = *req.BillingMode
	}
	if req.PaymentSubMode != nil {
		res.PaymentSubMode = *req.PaymentSubMode
	}
	res.DebtLimitAmount = req.DebtLimitAmount
	if err := r.db.Create(&res).Error; err != nil {
		r.logger.Error("failed to create reseller", zap.Error(err))
		return nil, err
	}

	return r.toResellerResponse(res), nil
}

// CompleteOnboarding latches Reseller.HasCompletedOnboarding to true --
// a narrow, single-purpose mutation (rather than routing this through the
// general UpdateReseller) so a stray/incomplete client payload can never
// accidentally reset it back to false, matching that field's own doc
// comment. Idempotent: calling it again on an already-completed reseller
// is a harmless no-op, not an error.
func (r *Reseller) CompleteOnboarding(id uint) (*schema.ResellerResponse, error) {
	var res model.Reseller
	if err := r.db.First(&res, "id = ?", id).Error; err != nil {
		return nil, err
	}
	if !res.HasCompletedOnboarding {
		res.HasCompletedOnboarding = true
		if err := r.db.Model(&res).Update("has_completed_onboarding", true).Error; err != nil {
			r.logger.Error("failed to complete reseller onboarding", zap.Uint("id", id), zap.Error(err))
			return nil, err
		}
	}
	return r.toResellerResponse(res), nil
}

func (r *Reseller) GetReseller(id uint) (*schema.ResellerResponse, error) {
	var res model.Reseller
	if err := r.db.First(&res, "id = ?", id).Error; err != nil {
		// Deliberately not re-wrapped: callers use errors.Is(err,
		// gorm.ErrRecordNotFound) to distinguish "not found" (404) from a
		// genuine failure (500), which a fmt.Errorf() wrapper would break.
		if !errors.Is(err, gorm.ErrRecordNotFound) {
			r.logger.Error("failed to fetch reseller", zap.Uint("id", id), zap.Error(err))
		}
		return nil, err
	}

	return r.toResellerResponse(res), nil
}

// CountPeers returns how many peers a reseller currently owns.
func (r *Reseller) CountPeers(resellerID uint) (int64, error) {
	var count int64
	if err := r.db.Model(&model.Peer{}).Where("reseller_id = ?", resellerID).Count(&count).Error; err != nil {
		return 0, err
	}
	return count, nil
}

// CountUserManagerAccounts returns how many User Manager accounts a
// reseller currently owns -- mirrors CountPeers exactly, separate pool.
func (r *Reseller) CountUserManagerAccounts(resellerID uint) (int64, error) {
	var count int64
	if err := r.db.Model(&model.UserManagerAccount{}).Where("reseller_id = ?", resellerID).Count(&count).Error; err != nil {
		return 0, err
	}
	return count, nil
}

// CountV2RayPackages returns how many V2Ray packages a reseller currently
// owns -- mirrors CountPeers/CountUserManagerAccounts exactly, third
// independent pool.
func (r *Reseller) CountV2RayPackages(resellerID uint) (int64, error) {
	var count int64
	if err := r.db.Model(&model.V2RayPackage{}).Where("reseller_id = ?", resellerID).Count(&count).Error; err != nil {
		return 0, err
	}
	return count, nil
}

// CountDNSAccounts returns how many DNS accounts a reseller currently owns
// -- mirrors CountPeers/CountUserManagerAccounts/CountV2RayPackages exactly,
// fourth and final independent pool.
func (r *Reseller) CountDNSAccounts(resellerID uint) (int64, error) {
	var count int64
	if err := r.db.Model(&model.DNSAccount{}).Where("reseller_id = ?", resellerID).Count(&count).Error; err != nil {
		return 0, err
	}
	return count, nil
}

func (r *Reseller) toResellerResponse(res model.Reseller) *schema.ResellerResponse {
	peerCount, err := r.CountPeers(res.ID)
	if err != nil {
		r.logger.Error("failed to count reseller peers", zap.Uint("id", res.ID), zap.Error(err))
	}

	userManagerAccountCount, err := r.CountUserManagerAccounts(res.ID)
	if err != nil {
		r.logger.Error("failed to count reseller user manager accounts", zap.Uint("id", res.ID), zap.Error(err))
	}

	var v2rayPackageCount int64
	if err := r.db.Model(&model.V2RayPackage{}).Where("reseller_id = ?", res.ID).Count(&v2rayPackageCount).Error; err != nil {
		r.logger.Error("failed to count reseller v2ray packages", zap.Uint("id", res.ID), zap.Error(err))
	}

	var applicationCount int64
	if err := r.db.Model(&model.Application{}).Where("reseller_id = ?", res.ID).Count(&applicationCount).Error; err != nil {
		r.logger.Error("failed to count reseller applications", zap.Uint("id", res.ID), zap.Error(err))
	}

	dnsAccountCount, err := r.CountDNSAccounts(res.ID)
	if err != nil {
		r.logger.Error("failed to count reseller dns accounts", zap.Uint("id", res.ID), zap.Error(err))
	}

	return &schema.ResellerResponse{
		ID:             res.ID,
		Name:           res.Name,
		Username:       res.Username,
		Email:          res.Email,
		IsActive:       res.IsActive,
		QuotaBytes:     res.QuotaBytes,
		UsedBytes:      res.UsedBytes,
		MaxPeers:       res.MaxPeers,
		PeerCount:      int(peerCount),
		TelegramChatID: res.TelegramChatID,
		OtpEnabled:     res.OtpEnabled,

		CanCreateUserManagerAccounts: res.CanCreateUserManagerAccounts,
		UserManagerQuotaBytes:        res.UserManagerQuotaBytes,
		UserManagerUsedBytes:         res.UserManagerUsedBytes,
		UserManagerMaxAccounts:       res.UserManagerMaxAccounts,
		UserManagerAccountCount:      int(userManagerAccountCount),

		CanResellV2Ray:    res.CanResellV2Ray,
		V2RayQuotaBytes:   res.V2RayQuotaBytes,
		V2RayUsedBytes:    res.V2RayUsedBytes,
		V2RayMaxPackages:  res.V2RayMaxPackages,
		V2RayPackageCount: int(v2rayPackageCount),

		CanResellDNS:    res.CanResellDNS,
		DNSQuotaBytes:   res.DNSQuotaBytes,
		DNSUsedBytes:    res.DNSUsedBytes,
		DNSMaxAccounts:  res.DNSMaxAccounts,
		DNSAccountCount: int(dnsAccountCount),

		CanCreateApplications: res.CanCreateApplications,
		ApplicationQuotaBytes: res.ApplicationQuotaBytes,
		ApplicationUsedBytes:  res.ApplicationUsedBytes,
		ApplicationMaxCount:   res.ApplicationMaxCount,
		ApplicationCount:      int(applicationCount),

		BillingMode:      res.BillingMode,
		PaymentSubMode:   res.PaymentSubMode,
		DebtLimitAmount:  res.DebtLimitAmount,
		BillingSuspended: res.BillingSuspended,

		HasCompletedOnboarding: res.HasCompletedOnboarding,
	}
}

func (r *Reseller) UpdateReseller(id uint, req *schema.UpdateResellerRequest) (*schema.ResellerResponse, error) {
	var res model.Reseller
	if err := r.db.First(&res, "id = ?", id).Error; err != nil {
		return nil, err
	}

	if req.Name != nil {
		res.Name = *req.Name
	}
	if req.Username != nil {
		var existing model.Reseller
		if err := r.db.First(&existing, "username = ? AND id <> ?", *req.Username, id).Error; err == nil {
			return nil, fmt.Errorf("username already exists")
		} else if !errors.Is(err, gorm.ErrRecordNotFound) {
			return nil, err
		}
		var admin model.Admin
		if err := r.db.First(&admin, "username = ?", *req.Username).Error; err == nil {
			return nil, fmt.Errorf("username already exists")
		} else if !errors.Is(err, gorm.ErrRecordNotFound) {
			return nil, err
		}
		res.Username = *req.Username
	}
	if req.Password != nil && *req.Password != "" {
		hashedPassword, err := bcrypt.GenerateFromPassword([]byte(*req.Password), bcrypt.DefaultCost)
		if err != nil {
			return nil, err
		}
		res.PasswordHash = string(hashedPassword)
	}
	if req.Email != nil {
		res.Email = req.Email
	}

	statusChanged := false
	if req.IsActive != nil {
		statusChanged = *req.IsActive != res.IsActive
		res.IsActive = *req.IsActive
	}

	quotaRaised := false
	if req.QuotaBytes != nil {
		// Detect a quota increase that brings the reseller back under quota,
		// so peers that were auto-suspended for running out of quota can be
		// resumed below. A quota decrease, or one that still leaves UsedBytes
		// over the new cap, must not trigger a resume.
		wasOverQuota := res.QuotaBytes != nil && res.UsedBytes > *res.QuotaBytes
		willBeUnderQuota := *req.QuotaBytes >= res.UsedBytes
		quotaRaised = wasOverQuota && willBeUnderQuota

		// Independently of the full-resume case above: if the new quota
		// gives the reseller more than 10% headroom again, let the
		// low-quota Telegram warning fire again next time they cross back
		// under that threshold, rather than staying permanently silenced
		// from a warning sent under the old, smaller quota.
		if *req.QuotaBytes > 0 {
			newRemainingPercent := (*req.QuotaBytes - res.UsedBytes) * 100 / *req.QuotaBytes
			if newRemainingPercent > 10 {
				res.QuotaWarningSent = false
			}
		}

		res.QuotaBytes = req.QuotaBytes
	}
	if req.ClearMaxPeers != nil && *req.ClearMaxPeers {
		res.MaxPeers = nil
	} else if req.MaxPeers != nil {
		res.MaxPeers = req.MaxPeers
	}
	if req.TelegramChatID != nil {
		res.TelegramChatID = req.TelegramChatID
	}
	if req.OtpEnabled != nil {
		res.OtpEnabled = *req.OtpEnabled
	}
	if req.CanCreateUserManagerAccounts != nil {
		res.CanCreateUserManagerAccounts = *req.CanCreateUserManagerAccounts
	}

	// User Manager quota is a fully separate pool from QuotaBytes above --
	// its own raise-detection, its own resume pass, no interaction with the
	// WireGuard quota/peer logic in this method.
	userManagerQuotaRaised := false
	if req.UserManagerQuotaBytes != nil {
		wasOverUserManagerQuota := res.UserManagerQuotaBytes != nil && res.UserManagerUsedBytes > *res.UserManagerQuotaBytes
		willBeUnderUserManagerQuota := *req.UserManagerQuotaBytes >= res.UserManagerUsedBytes
		userManagerQuotaRaised = wasOverUserManagerQuota && willBeUnderUserManagerQuota

		res.UserManagerQuotaBytes = req.UserManagerQuotaBytes
	}
	if req.ClearUserManagerMaxAccounts != nil && *req.ClearUserManagerMaxAccounts {
		res.UserManagerMaxAccounts = nil
	} else if req.UserManagerMaxAccounts != nil {
		res.UserManagerMaxAccounts = req.UserManagerMaxAccounts
	}

	if req.CanResellV2Ray != nil {
		res.CanResellV2Ray = *req.CanResellV2Ray
	}
	// V2Ray quota is a fully separate pool from QuotaBytes/UserManagerQuotaBytes
	// above -- its own raise-detection, its own resume pass (see the
	// confirmed, reported incident behind V2RayPackage.
	// SuspendedByResellerQuota's own doc comment: this raise-detection was
	// previously entirely missing here, matching how enforcement itself
	// was missing from applyResellerV2RayQuota).
	v2rayQuotaRaised := false
	if req.V2RayQuotaBytes != nil {
		// Confirmed, reported production incident this fixes (reseller
		// "Mohammadreza"): res.V2RayUsedBytes is the STORED total, which
		// permanently includes V2RayDeletedUsageBytes (see
		// applyResellerV2RayQuota's own doc comment on why that credit must
		// exist for Payment-mode billing but must never be compared against
		// a Volume-mode quota) -- using it here meant raising
		// V2RayQuotaBytes to comfortably cover the reseller's REAL current
		// (live) usage could still fail to register as "now under quota"
		// whenever any package had ever been deleted, since the stored
		// total sits permanently higher than what's actually provisioned.
		// wasOverV2RayQuota/willBeUnderV2RayQuota must both compare against
		// the live sum over currently-existing packages, matching exactly
		// what applyResellerV2RayQuota itself now enforces.
		var liveV2RayUsed int64
		if err := r.db.Model(&model.V2RayPackageLocation{}).
			Joins("JOIN v2_ray_packages ON v2_ray_packages.id = v2_ray_package_locations.package_id").
			Where("v2_ray_packages.reseller_id = ?", res.ID).
			Select("COALESCE(SUM(v2_ray_package_locations.used_bytes_cached), 0)").
			Scan(&liveV2RayUsed).Error; err != nil {
			r.logger.Error("failed to sum live v2ray usage for quota-raise detection", zap.Uint("reseller_id", res.ID), zap.Error(err))
			liveV2RayUsed = res.V2RayUsedBytes // fall back to the old (safe, if imprecise) behavior rather than failing the whole update
		}

		wasOverV2RayQuota := res.V2RayQuotaBytes != nil && liveV2RayUsed > *res.V2RayQuotaBytes
		willBeUnderV2RayQuota := *req.V2RayQuotaBytes >= liveV2RayUsed
		v2rayQuotaRaised = wasOverV2RayQuota && willBeUnderV2RayQuota

		res.V2RayQuotaBytes = req.V2RayQuotaBytes
	}
	if req.ClearV2RayMaxPackages != nil && *req.ClearV2RayMaxPackages {
		res.V2RayMaxPackages = nil
	} else if req.V2RayMaxPackages != nil {
		res.V2RayMaxPackages = req.V2RayMaxPackages
	}

	if req.CanResellDNS != nil {
		res.CanResellDNS = *req.CanResellDNS
	}
	// DNS quota is a fourth, fully separate pool -- simplified raise-
	// detection relative to V2Ray's (no deleted-usage-credit subtlety to
	// account for, matching applyResellerDNSQuota's own "volume-mode
	// only, no Payment-mode branch" design for this product).
	dnsQuotaRaised := false
	if req.DNSQuotaBytes != nil {
		var liveDNSUsed int64
		if err := r.db.Model(&model.DNSAccount{}).Where("reseller_id = ?", res.ID).
			Select("COALESCE(SUM(used_bytes_cached), 0)").Scan(&liveDNSUsed).Error; err != nil {
			r.logger.Error("failed to sum live dns usage for quota-raise detection", zap.Uint("reseller_id", res.ID), zap.Error(err))
			liveDNSUsed = res.DNSUsedBytes
		}

		wasOverDNSQuota := res.DNSQuotaBytes != nil && liveDNSUsed > *res.DNSQuotaBytes
		willBeUnderDNSQuota := *req.DNSQuotaBytes >= liveDNSUsed
		dnsQuotaRaised = wasOverDNSQuota && willBeUnderDNSQuota

		res.DNSQuotaBytes = req.DNSQuotaBytes
	}
	if req.ClearDNSMaxAccounts != nil && *req.ClearDNSMaxAccounts {
		res.DNSMaxAccounts = nil
	} else if req.DNSMaxAccounts != nil {
		res.DNSMaxAccounts = req.DNSMaxAccounts
	}

	if req.CanCreateApplications != nil {
		res.CanCreateApplications = *req.CanCreateApplications
	}
	// Application quota is a fifth, fully separate pool -- mirrors DNS's
	// identical simplified raise-detection (no deleted-usage-credit
	// subtlety, no Payment-mode branch in applyResellerApplicationQuota).
	// Live sum is over Application.UsedBytes (itself already the
	// per-Application aggregate across its peer/account/package fan-out,
	// see sumApplicationUsage's own doc comment), not a raw usage column.
	applicationQuotaRaised := false
	if req.ApplicationQuotaBytes != nil {
		var liveApplicationUsed int64
		if err := r.db.Model(&model.Application{}).Where("reseller_id = ?", res.ID).
			Select("COALESCE(SUM(used_bytes), 0)").Scan(&liveApplicationUsed).Error; err != nil {
			r.logger.Error("failed to sum live application usage for quota-raise detection", zap.Uint("reseller_id", res.ID), zap.Error(err))
			liveApplicationUsed = res.ApplicationUsedBytes
		}

		wasOverApplicationQuota := res.ApplicationQuotaBytes != nil && liveApplicationUsed > *res.ApplicationQuotaBytes
		willBeUnderApplicationQuota := *req.ApplicationQuotaBytes >= liveApplicationUsed
		applicationQuotaRaised = wasOverApplicationQuota && willBeUnderApplicationQuota

		res.ApplicationQuotaBytes = req.ApplicationQuotaBytes
	}
	if req.ClearApplicationMaxCount != nil && *req.ClearApplicationMaxCount {
		res.ApplicationMaxCount = nil
	} else if req.ApplicationMaxCount != nil {
		res.ApplicationMaxCount = req.ApplicationMaxCount
	}

	// billingModeSwitched (category 2 item 1 -- bidirectional Volume<->
	// Payment switching): previously BillingMode was a bare field
	// assignment with zero reconciliation, confirmed to leave a reseller
	// in an inconsistent state in both directions:
	//   - Volume->Payment: any is_active=false/suspended_by_quota state
	//     left over from the byte-quota system becomes permanently stuck
	//     (nothing under Payment mode ever re-checks or clears it), even
	//     though the reseller is now billed per-GB and quota is meant to
	//     be irrelevant.
	//   - Payment->Volume: BillingSuspended (Payment's own suspend latch)
	//     was never cleared, becoming stale dead state that could
	//     spuriously resume unrelated peers if the reseller is ever
	//     switched back to Payment later (ResumeBillingSuspension only
	//     checks the flag's current value, not when/why it was set).
	// Both directions reuse the EXACT SAME underlying resume mechanism
	// (resumeQuotaSuspendedPeers/resumeQuotaSuspendedUserManagerAccounts/
	// ResumePackagesForResellerQuota target suspended_by_quota +
	// was_active_before_suspend on the resource itself, regardless of
	// which enforcement path set them), since a reseller switching modes
	// obviously wants their existing resources working again under
	// whichever mode they just chose, not stuck on a suspension reason
	// that no longer applies.
	oldBillingMode := res.BillingMode
	billingModeSwitched := false
	if req.BillingMode != nil {
		billingModeSwitched = *req.BillingMode != oldBillingMode
		res.BillingMode = *req.BillingMode
	}
	if req.PaymentSubMode != nil {
		res.PaymentSubMode = *req.PaymentSubMode
	}
	if req.ClearDebtLimitAmount != nil && *req.ClearDebtLimitAmount {
		res.DebtLimitAmount = nil
	} else if req.DebtLimitAmount != nil {
		res.DebtLimitAmount = req.DebtLimitAmount
	}

	// Switching AWAY from either mode means whatever THAT mode's own
	// suspend latch says no longer matters -- clear it so a future
	// switch back doesn't inherit a stale reason from an unrelated,
	// possibly much older, suspension event. resumeIsActive additionally
	// clears the Volume-only is_active=false flag (applyResellerQuota's
	// own hard-disable, never touched by any existing resume helper --
	// see that job's own doc comment on this pre-existing quirk) since a
	// mode switch is exactly the admin action that should un-stick it.
	resumeSuspendedResources := false
	resumeIsActive := false
	if billingModeSwitched {
		if oldBillingMode == model.ResellerBillingModePayment {
			// Payment->Volume: this reseller's own BillingSuspended latch
			// is now meaningless (nothing under Volume mode reads it) --
			// clear it so it can't resurface as stale state on a later
			// switch back to Payment.
			res.BillingSuspended = false
		} else {
			// Volume->Payment: the reseller may still be sitting on
			// is_active=false / suspended_by_quota from the byte-quota
			// system, which is now irrelevant under Payment mode.
			resumeIsActive = true
		}
		resumeSuspendedResources = true
	}

	if err := r.db.Transaction(func(tx *gorm.DB) error {
		if resumeIsActive {
			res.IsActive = true
		}

		if err := tx.Save(&res).Error; err != nil {
			return err
		}

		if quotaRaised || resumeSuspendedResources {
			if err := r.resumeQuotaSuspendedPeers(tx, res.ID); err != nil {
				return err
			}
		}

		if userManagerQuotaRaised || resumeSuspendedResources {
			if err := r.resumeQuotaSuspendedUserManagerAccounts(tx, res.ID); err != nil {
				return err
			}
		}

		return nil
	}); err != nil {
		return nil, err
	}

	// Run outside the transaction above -- ResumePackagesForResellerQuota
	// makes real x-ui network calls per package location (same reasoning
	// as ResumeBillingSuspension's own identically-placed v2rayResumer
	// call: a slow/unreachable x-ui panel must never hold the reseller
	// row's own DB transaction open).
	if (v2rayQuotaRaised || resumeSuspendedResources) && r.v2rayResumer != nil {
		r.v2rayResumer.ResumePackagesForResellerQuota(res.ID)
	}

	// Same reasoning as the v2rayResumer call above -- doctor-dns network
	// calls must never hold the reseller row's own DB transaction open.
	if (dnsQuotaRaised || resumeSuspendedResources) && r.dnsResumer != nil {
		r.dnsResumer.ResumeAccountsForResellerQuota(res.ID)
	}

	// Same reasoning as the v2rayResumer/dnsResumer calls above --
	// RouterOS/x-ui network calls (via suspendApplication/resumeApplication)
	// must never hold the reseller row's own DB transaction open.
	if (applicationQuotaRaised || resumeSuspendedResources) && r.appResumer != nil {
		r.appResumer.ResumeApplicationsForResellerQuota(res.ID)
	}

	// Volume->Payment: bootstrap a wallet row immediately (rather than
	// leaving it to whatever the first ChargeUsage/wallet-page-view
	// happens to lazily create later) so the admin's own "Wallet" tab
	// shows an explicit, real zero balance right after switching, not a
	// confusing blank/missing state. GetOrCreateWallet is itself already
	// idempotent, so this is safe even if a wallet row already exists
	// from a previous stint as Payment-based.
	if billingModeSwitched && oldBillingMode != model.ResellerBillingModePayment && r.wallet != nil {
		if _, err := r.wallet.GetOrCreateWallet(res.ID); err != nil {
			r.logger.Warn("failed to bootstrap wallet on switch to payment-based billing", zap.Uint("reseller_id", res.ID), zap.Error(err))
		}
	}

	if statusChanged && r.botNotifier != nil {
		r.botNotifier.NotifyAccountStatusChange(res, res.IsActive)
	}

	return r.toResellerResponse(res), nil
}

// resumeQuotaSuspendedPeers re-enables every peer belonging to this reseller
// that was force-disabled by the reseller-quota job (SuspendedByQuota) and
// was actually running beforehand (WasActiveBeforeSuspend) — peers the
// admin/reseller had manually disabled, or that hit their own separate
// TrafficLimit, are left untouched. Both flags are cleared once resumed so a
// future quota exhaustion starts the suspend/resume cycle fresh.
func (r *Reseller) resumeQuotaSuspendedPeers(tx *gorm.DB, resellerID uint) error {
	var peers []model.Peer
	if err := tx.Where(
		"reseller_id = ? AND suspended_by_quota = ? AND was_active_before_suspend = ?",
		resellerID, true, true,
	).Find(&peers).Error; err != nil {
		r.logger.Error("failed to fetch quota-suspended peers", zap.Uint("reseller_id", resellerID), zap.Error(err))
		return err
	}

	for _, p := range peers {
		if r.mikrotikAdaptor != nil {
			if _, err := r.mikrotikAdaptor.UpdateWgPeer(context.Background(), p.PeerID, mikrotik.WireGuardPeer{Disabled: strconv.FormatBool(false)}); err != nil {
				r.logger.Error("failed to re-enable peer on Mikrotik", zap.String("peerID", p.PeerID), zap.Error(err))
				continue
			}
		}

		if err := tx.Model(&model.Peer{}).Where("id = ?", p.ID).Updates(map[string]interface{}{
			"disabled":                  false,
			"suspended_by_quota":        false,
			"was_active_before_suspend": false,
		}).Error; err != nil {
			r.logger.Error("failed to re-enable peer in DB", zap.String("peerID", p.PeerID), zap.Error(err))
			return err
		}
	}

	return nil
}

// resumeQuotaSuspendedUserManagerAccounts mirrors resumeQuotaSuspendedPeers
// exactly, but for UserManagerAccount rows and RouterOS's
// SetUserManagerUserDisabled -- the two suspend/resume pools never interact.
func (r *Reseller) resumeQuotaSuspendedUserManagerAccounts(tx *gorm.DB, resellerID uint) error {
	var accounts []model.UserManagerAccount
	if err := tx.Where(
		"reseller_id = ? AND suspended_by_quota = ? AND was_active_before_suspend = ?",
		resellerID, true, true,
	).Find(&accounts).Error; err != nil {
		r.logger.Error("failed to fetch quota-suspended user manager accounts", zap.Uint("reseller_id", resellerID), zap.Error(err))
		return err
	}

	for _, account := range accounts {
		if r.mikrotikAdaptor != nil {
			if _, err := r.mikrotikAdaptor.SetUserManagerUserDisabled(context.Background(), account.RouterOSUserID, "false"); err != nil {
				r.logger.Error("failed to re-enable user manager account on Mikrotik", zap.String("username", account.Username), zap.Error(err))
				continue
			}
		}

		if err := tx.Model(&model.UserManagerAccount{}).Where("id = ?", account.ID).Updates(map[string]interface{}{
			"disabled":                  false,
			"suspended_by_quota":        false,
			"was_active_before_suspend": false,
		}).Error; err != nil {
			r.logger.Error("failed to re-enable user manager account in DB", zap.String("username", account.Username), zap.Error(err))
			return err
		}
	}

	return nil
}

// ResumeBillingSuspension re-checks a Payment-based reseller's wallet
// after a credit (top-up or refund) and, if solvent again, clears
// BillingSuspended and re-enables every peer/user-manager-account/V2Ray
// package that billing suspended -- the Payment-based mirror of
// UpdateReseller's quotaRaised/userManagerQuotaRaised resume branches
// above. Safe to call unconditionally after ANY wallet credit: a no-op for
// a reseller that isn't currently BillingSuspended, and a no-op (leaves
// suspension in place) if the new balance still doesn't clear the debt
// ceiling -- callers don't need to pre-check either condition themselves.
// v2rayResumer is the narrow capability needed to re-enable suspended
// V2Ray package locations (*V2RaySyncService.ResumePackagesForReseller),
// injected rather than imported directly to avoid a Reseller<->V2Ray
// service constructor cycle; nil-safe (V2Ray resume simply skipped).
func (r *Reseller) ResumeBillingSuspension(resellerID uint, wallet *Wallet, v2rayResumer interface {
	ResumePackagesForReseller(resellerID uint)
}) error {
	var reseller model.Reseller
	if err := r.db.Where("id = ?", resellerID).First(&reseller).Error; err != nil {
		return err
	}
	if !reseller.BillingSuspended {
		return nil
	}

	balance, err := wallet.GetWalletBalance(resellerID)
	if err != nil {
		return err
	}

	// Prepaid: solvent again the moment balance is non-negative. Postpaid:
	// solvent again as long as the current debt (a negative balance) is
	// still within DebtLimitAmount -- mirrors ChargeUsage's own
	// DebitAllowNegative ceiling check exactly, just evaluated at rest
	// instead of against a pending debit.
	solvent := balance >= 0
	if !solvent && reseller.PaymentSubMode == model.ResellerPaymentSubModePostpaid {
		solvent = reseller.DebtLimitAmount == nil || -balance <= *reseller.DebtLimitAmount
	}
	if !solvent {
		return nil
	}

	if err := r.db.Transaction(func(tx *gorm.DB) error {
		if err := tx.Model(&model.Reseller{}).Where("id = ?", resellerID).Update("billing_suspended", false).Error; err != nil {
			return err
		}
		if err := r.resumeQuotaSuspendedPeers(tx, resellerID); err != nil {
			return err
		}
		return r.resumeQuotaSuspendedUserManagerAccounts(tx, resellerID)
	}); err != nil {
		return err
	}

	if v2rayResumer != nil {
		v2rayResumer.ResumePackagesForReseller(resellerID)
	}

	return nil
}

func (r *Reseller) DeleteReseller(id uint) error {
	if err := r.db.Transaction(func(tx *gorm.DB) error {
		if err := tx.Model(&model.Peer{}).
			Where("reseller_id = ?", id).
			Update("reseller_id", nil).Error; err != nil {
			r.logger.Error("failed to detach peers from reseller", zap.Uint("id", id), zap.Error(err))
			return err
		}

		if err := tx.Model(&model.UserManagerAccount{}).
			Where("reseller_id = ?", id).
			Update("reseller_id", nil).Error; err != nil {
			r.logger.Error("failed to detach user manager accounts from reseller", zap.Uint("id", id), zap.Error(err))
			return err
		}

		result := tx.Delete(&model.Reseller{}, id)
		if result.Error != nil {
			r.logger.Error("failed to delete reseller", zap.Uint("id", id), zap.Error(result.Error))
			return result.Error
		}
		if result.RowsAffected == 0 {
			return gorm.ErrRecordNotFound
		}

		return nil
	}); err != nil {
		return err
	}

	return nil
}

// SetAssignedInterfaces replaces the full set of interfaces a reseller is allowed
// to create peers on.
func (r *Reseller) SetAssignedInterfaces(resellerID uint, interfaceIDs []uint) error {
	var reseller model.Reseller
	if err := r.db.First(&reseller, resellerID).Error; err != nil {
		return err
	}

	return r.db.Transaction(func(tx *gorm.DB) error {
		if err := tx.Unscoped().Where("reseller_id = ?", resellerID).Delete(&model.ResellerInterface{}).Error; err != nil {
			return err
		}

		if len(interfaceIDs) == 0 {
			return nil
		}

		var count int64
		if err := tx.Model(&model.Interface{}).Where("id IN ?", interfaceIDs).Count(&count).Error; err != nil {
			return err
		}
		if int(count) != len(interfaceIDs) {
			return fmt.Errorf("one or more interface ids do not exist")
		}

		links := make([]model.ResellerInterface, 0, len(interfaceIDs))
		for _, ifaceID := range interfaceIDs {
			links = append(links, model.ResellerInterface{ResellerID: resellerID, InterfaceID: ifaceID})
		}

		return tx.Create(&links).Error
	})
}

// GetAssignedInterfaceIDs returns the interface IDs a reseller is allowed to use.
func (r *Reseller) GetAssignedInterfaceIDs(resellerID uint) ([]uint, error) {
	var links []model.ResellerInterface
	if err := r.db.Where("reseller_id = ?", resellerID).Find(&links).Error; err != nil {
		return nil, err
	}

	ids := make([]uint, 0, len(links))
	for _, link := range links {
		ids = append(ids, link.InterfaceID)
	}
	return ids, nil
}

// IsInterfaceAssigned checks whether a reseller may use the given interface.
func (r *Reseller) IsInterfaceAssigned(resellerID uint, interfaceID uint) (bool, error) {
	var count int64
	if err := r.db.Model(&model.ResellerInterface{}).
		Where("reseller_id = ? AND interface_id = ?", resellerID, interfaceID).
		Count(&count).Error; err != nil {
		return false, err
	}
	return count > 0, nil
}

// SetAssignedUserManagerGroups replaces the full set of RouterOS User
// Manager groups a reseller is allowed to assign when creating accounts --
// mirrors SetAssignedInterfaces exactly. Group names are not validated
// against a local table (RouterOS is the only source of truth for which
// groups exist, and this panel doesn't cache them beyond a live listing),
// unlike interface IDs which ARE checked against the local Interface table.
func (r *Reseller) SetAssignedUserManagerGroups(resellerID uint, groupNames []string) error {
	var reseller model.Reseller
	if err := r.db.First(&reseller, resellerID).Error; err != nil {
		return err
	}

	return r.db.Transaction(func(tx *gorm.DB) error {
		if err := tx.Unscoped().Where("reseller_id = ?", resellerID).Delete(&model.ResellerUserManagerGroup{}).Error; err != nil {
			return err
		}

		if len(groupNames) == 0 {
			return nil
		}

		links := make([]model.ResellerUserManagerGroup, 0, len(groupNames))
		for _, name := range groupNames {
			links = append(links, model.ResellerUserManagerGroup{ResellerID: resellerID, GroupName: name})
		}

		return tx.Create(&links).Error
	})
}

// GetAssignedUserManagerGroups returns the group names a reseller is
// allowed to use.
func (r *Reseller) GetAssignedUserManagerGroups(resellerID uint) ([]string, error) {
	var links []model.ResellerUserManagerGroup
	if err := r.db.Where("reseller_id = ?", resellerID).Find(&links).Error; err != nil {
		return nil, err
	}

	names := make([]string, 0, len(links))
	for _, link := range links {
		names = append(names, link.GroupName)
	}
	return names, nil
}

// IsUserManagerGroupAssigned checks whether a reseller may use the given group.
func (r *Reseller) IsUserManagerGroupAssigned(resellerID uint, groupName string) (bool, error) {
	var count int64
	if err := r.db.Model(&model.ResellerUserManagerGroup{}).
		Where("reseller_id = ? AND group_name = ?", resellerID, groupName).
		Count(&count).Error; err != nil {
		return false, err
	}
	return count > 0, nil
}

// SetAssignedUserManagerProfiles mirrors SetAssignedUserManagerGroups
// exactly, for RouterOS User Manager profiles.
func (r *Reseller) SetAssignedUserManagerProfiles(resellerID uint, profileNames []string) error {
	var reseller model.Reseller
	if err := r.db.First(&reseller, resellerID).Error; err != nil {
		return err
	}

	return r.db.Transaction(func(tx *gorm.DB) error {
		if err := tx.Unscoped().Where("reseller_id = ?", resellerID).Delete(&model.ResellerUserManagerProfile{}).Error; err != nil {
			return err
		}

		if len(profileNames) == 0 {
			return nil
		}

		links := make([]model.ResellerUserManagerProfile, 0, len(profileNames))
		for _, name := range profileNames {
			links = append(links, model.ResellerUserManagerProfile{ResellerID: resellerID, ProfileName: name})
		}

		return tx.Create(&links).Error
	})
}

// GetAssignedUserManagerProfiles returns the profile names a reseller is
// allowed to use.
func (r *Reseller) GetAssignedUserManagerProfiles(resellerID uint) ([]string, error) {
	var links []model.ResellerUserManagerProfile
	if err := r.db.Where("reseller_id = ?", resellerID).Find(&links).Error; err != nil {
		return nil, err
	}

	names := make([]string, 0, len(links))
	for _, link := range links {
		names = append(names, link.ProfileName)
	}
	return names, nil
}

// IsUserManagerProfileAssigned checks whether a reseller may use the given profile.
func (r *Reseller) IsUserManagerProfileAssigned(resellerID uint, profileName string) (bool, error) {
	var count int64
	if err := r.db.Model(&model.ResellerUserManagerProfile{}).
		Where("reseller_id = ? AND profile_name = ?", resellerID, profileName).
		Count(&count).Error; err != nil {
		return false, err
	}
	return count > 0, nil
}

func (r *Reseller) ListResellers() ([]schema.ResellerResponse, error) {
	var list []model.Reseller
	if err := r.db.Find(&list).Error; err != nil {
		return nil, err
	}

	out := make([]schema.ResellerResponse, 0, len(list))
	for _, s := range list {
		out = append(out, *r.toResellerResponse(s))
	}

	return out, nil
}
