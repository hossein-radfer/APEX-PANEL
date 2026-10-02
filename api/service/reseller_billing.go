package service

import (
	"errors"
	"fmt"

	"go.uber.org/zap"
	"gorm.io/gorm"
	"gorm.io/gorm/clause"

	"github.com/maahdima/mwp/api/dataservice/model"
	"github.com/maahdima/mwp/api/http/schema"
)

// ResellerBillingService is the single entry point every usage-delta
// producer (WireGuard's traffic.Calculator, User Manager's same
// Calculator, V2Ray's V2RaySyncService) calls to price and charge newly
// accrued usage for a Payment-based (BillingMode=PAYMENT) reseller -- a
// Volume-based reseller (the original, still-default behavior) never
// reaches this service at all; ChargeUsage's very first check is a no-op
// for anything but a Payment-based reseller, so wiring this into all three
// usage-delta points is safe even before any reseller has opted in.
//
// One reseller-scoped write path (this file) intentionally sits alongside
// three separate byte-quota write paths (applyResellerQuota in
// cmd/jobs/traffic.go, applyUserManagerResellerQuota in the same file,
// applyResellerV2RayQuota in v2ray_sync.go) rather than replacing them --
// BillingMode picks exactly one of the two to actually take effect for a
// given reseller (see model.Reseller.BillingMode's own doc comment), so
// both paths always run per delta, and the inactive one's writes
// (UsedBytes bookkeeping) are simply cosmetic/ignored for a Payment-based
// reseller rather than harmful to skip rewiring.
type ResellerBillingService struct {
	db      *gorm.DB
	wallet  *Wallet
	logger  *zap.Logger
	resumer billingSuspensionResumer
}

// billingSuspensionResumer is the narrow capability needed to clear a
// solvent-again reseller's BillingSuspended flag (Reseller.
// ResumeBillingSuspension's exact signature), injected via SetResumer
// rather than imported directly -- mirrors this codebase's existing
// optional-collaborator convention (Reseller.SetBotNotifier/
// SetV2RayResumer/SetWallet) and avoids a Reseller<->ResellerBillingService
// constructor cycle. Nil-safe: ChargeUsage simply skips the self-heal
// below when unset.
type billingSuspensionResumer interface {
	ResumeBillingSuspension(resellerID uint, wallet *Wallet, v2rayResumer interface {
		ResumePackagesForReseller(resellerID uint)
	}) error
}

func NewResellerBillingService(db *gorm.DB, wallet *Wallet) *ResellerBillingService {
	return &ResellerBillingService{
		db:     db,
		wallet: wallet,
		logger: zap.L().Named("ResellerBillingService"),
	}
}

// SetResumer wires the capability ChargeUsage needs to self-heal a stuck
// BillingSuspended flag the moment a debit succeeds again -- see this
// call's own doc comment inside ChargeUsage for the exact incident this
// closes. Safe to leave unset (nil-safe no-op).
func (s *ResellerBillingService) SetResumer(resumer billingSuspensionResumer) {
	s.resumer = resumer
}

const bytesPerGB = 1024 * 1024 * 1024

// ChargeUsage prices deltaBytes of newly accrued usage for one product on
// one reseller and debits the resulting whole-Toman amount from that
// reseller's wallet, honoring PaymentSubMode (Prepaid rejects the debit
// outright on insufficient funds; Postpaid allows going into debt up to
// DebtLimitAmount) -- then, on rejection, disables every one of that
// reseller's configs for the given product exactly like the byte-quota
// enforcement paths already do for BillingMode=VOLUME, and marks
// BillingSuspended so ResumeIfFunded (called after every wallet credit /
// DebtLimitAmount raise) knows to re-check this reseller.
//
// No-ops immediately (returns nil) for a reseller that is not
// BillingMode=PAYMENT, or for a product with no configured
// ResellerBillingPrice row (or a zero price) -- usage still accrues
// (the caller's own byte-quota bookkeeping is unaffected either way),
// simply nothing is charged. deltaBytes<=0 also no-ops; there's nothing to
// price.
// ChargeUsage prices deltaBytes of newly accrued usage for one product
// (optionally scoped to one location within that product -- see
// resolvePricePerGB's own doc comment) on one reseller, then debits the
// resulting whole-Toman amount from that reseller's wallet.
//
// locationKey identifies WHERE within the product this usage happened --
// a WireGuard interface name, a User Manager group name, or a V2Ray
// PanelID formatted as a decimal string -- so a reseller can be charged
// different per-GB rates for e.g. cheaper European WireGuard interfaces
// vs pricier Middle-Eastern ones, not just one flat rate for the whole
// product. Pass "" when no location is meaningful (there is none today,
// but this keeps the signature uniform for every caller).
func (s *ResellerBillingService) ChargeUsage(resellerID uint, product string, locationKey string, deltaBytes int64) error {
	if deltaBytes <= 0 {
		return nil
	}

	var reseller model.Reseller
	if err := s.db.Where("id = ?", resellerID).First(&reseller).Error; err != nil {
		s.logger.Error("failed to fetch reseller for billing", zap.Uint("reseller_id", resellerID), zap.Error(err))
		return err
	}
	if reseller.BillingMode != model.ResellerBillingModePayment {
		return nil
	}

	// Volume tiers, when configured for this product, REPLACE flat/
	// location pricing entirely for that product (see
	// model.ResellerBillingTier's own doc comment on why the two never
	// stack) -- checked first so tiering, once opted into, always wins.
	var tierCount int64
	if err := s.db.Model(&model.ResellerBillingTier{}).Where("reseller_id = ? AND product = ?", resellerID, product).Count(&tierCount).Error; err != nil {
		s.logger.Error("failed to check for reseller billing tiers", zap.Uint("reseller_id", resellerID), zap.String("product", product), zap.Error(err))
		return err
	}

	var chargeAmount int64
	if tierCount > 0 {
		amount, err := s.chargeAcrossTiers(resellerID, product, deltaBytes)
		if err != nil {
			return err
		}
		chargeAmount = amount
	} else {
		pricePerGB, err := s.resolveFlatPricePerGB(resellerID, product, locationKey)
		if err != nil {
			return err
		}
		if pricePerGB <= 0 {
			// No price row configured yet for this (product, location) --
			// accrue usage, charge nothing (see ResellerBillingPrice's own
			// doc comment).
			return nil
		}
		// Integer-GB pricing: fractional GB usage this tick is charged
		// proportionally via integer math, rounding down -- consistent
		// with every other Toman amount in this codebase being a plain
		// integer with no sub-unit, and avoiding float drift entirely. A
		// very small delta (a few KB) can legitimately round down to a 0
		// Toman charge on any single tick; it accumulates correctly
		// across ticks since this always prices the FRESH delta, not a
		// running remainder.
		chargeAmount = (deltaBytes * pricePerGB) / bytesPerGB
	}
	if chargeAmount <= 0 {
		return nil
	}

	referenceType := "USAGE_" + product
	debitDescription := fmt.Sprintf("مصرف %s: %.3f گیگابایت", productLabelFa(product), float64(deltaBytes)/float64(bytesPerGB))

	var debitErr error
	if reseller.PaymentSubMode == model.ResellerPaymentSubModePostpaid {
		_, debitErr = s.wallet.DebitAllowNegative(resellerID, chargeAmount, debitDescription, &referenceType, nil, reseller.DebtLimitAmount)
	} else {
		_, debitErr = s.wallet.Debit(resellerID, chargeAmount, debitDescription, &referenceType, nil)
	}

	if debitErr == nil {
		// Confirmed, reported production incident this fixes: BillingSuspended
		// previously had no self-healing path at all -- once set true by a
		// rejected debit below, the ONLY way it was ever cleared again was
		// the admin's manual wallet-credit HTTP handler explicitly calling
		// ResumeBillingSuspension (see reseller.go). A reseller topped up
		// through ANY other path (a different admin action, a balance that
		// simply recovered) kept BillingSuspended=true forever, which
		// enforcePackageQuota's own overQuota-merge (see this file's own
		// call site there) then used to keep re-suspending every one of
		// that reseller's V2Ray packages on every sync tick -- even while
		// ChargeUsage itself kept succeeding tick after tick, because this
		// method never even checked the flag before debiting. A debit that
		// just succeeded is the single most direct signal available that
		// the reseller is solvent again, so this is the natural place to
		// self-heal the flag -- mirrors ResumeBillingSuspension's own
		// solvency check, just triggered by the event that proves it
		// instead of only an admin's manual action.
		if reseller.BillingSuspended && s.resumer != nil {
			if err := s.resumer.ResumeBillingSuspension(resellerID, s.wallet, nil); err != nil {
				s.logger.Error("failed to resume billing suspension after successful debit",
					zap.Uint("reseller_id", resellerID), zap.Error(err))
			}
		}
		return nil
	}

	// Only treat one of the three genuine financial rejections below
	// (insufficient funds, frozen wallet, debt limit exceeded) as a real
	// suspension reason. Any other error is transient (e.g. a database
	// contention error unrelated to the reseller's actual solvency) and
	// is simply propagated so this tick's usage/charge is retried next
	// tick once it clears, rather than wrongly suspending a solvent
	// reseller.
	if !errors.Is(debitErr, ErrInsufficientFunds) && !errors.Is(debitErr, ErrWalletFrozen) && !errors.Is(debitErr, ErrDebtLimitExceeded) {
		s.logger.Error("reseller billing debit failed due to a transient error, not suspending",
			zap.Uint("reseller_id", resellerID), zap.String("product", product), zap.Error(debitErr))
		return debitErr
	}

	s.logger.Warn("reseller billing debit rejected, suspending product configs",
		zap.Uint("reseller_id", resellerID), zap.String("product", product), zap.Error(debitErr))

	if !reseller.BillingSuspended {
		if err := s.db.Model(&model.Reseller{}).Where("id = ?", resellerID).Update("billing_suspended", true).Error; err != nil {
			s.logger.Error("failed to mark reseller billing-suspended", zap.Uint("reseller_id", resellerID), zap.Error(err))
		}
	}

	return debitErr
}

// resolveFlatPricePerGB implements the non-tiered pricing lookup: a
// location-specific ResellerBillingPrice row (LocationKey == locationKey)
// wins if one exists, otherwise the product-wide fallback row
// (LocationKey == "") applies -- see ResellerBillingPrice's own doc
// comment for why "" is the fallback rather than a third, separate
// concept. Returns 0 (not an error) when neither row exists or the
// resolved row's price is <=0, matching ChargeUsage's existing "nothing
// configured yet, charge nothing" behavior.
func (s *ResellerBillingService) resolveFlatPricePerGB(resellerID uint, product string, locationKey string) (int64, error) {
	if locationKey != "" {
		var located model.ResellerBillingPrice
		err := s.db.Where("reseller_id = ? AND product = ? AND location_key = ?", resellerID, product, locationKey).First(&located).Error
		if err == nil {
			return located.PricePerGBAmount, nil
		}
		if !errors.Is(err, gorm.ErrRecordNotFound) {
			return 0, err
		}
	}

	var fallback model.ResellerBillingPrice
	err := s.db.Where("reseller_id = ? AND product = ? AND location_key = ?", resellerID, product, "").First(&fallback).Error
	if err != nil {
		if errors.Is(err, gorm.ErrRecordNotFound) {
			return 0, nil
		}
		return 0, err
	}
	return fallback.PricePerGBAmount, nil
}

// cumulativeUsedBytes reads the reseller's own running per-product usage
// total (the SAME byte-quota bookkeeping columns Volume-based billing has
// always maintained -- see ResellerBillingService's own top-level doc
// comment on why both paths always run regardless of BillingMode) as of
// BEFORE the current tick's deltaBytes is applied by the caller's own
// separate bookkeeping update. This is the baseline chargeAcrossTiers
// needs to know which tier(s) the fresh delta falls into.
func (s *ResellerBillingService) cumulativeUsedBytes(reseller model.Reseller, product string) int64 {
	switch product {
	case model.ResellerBillingProductWireGuard:
		return reseller.UsedBytes
	case model.ResellerBillingProductUserManager:
		return reseller.UserManagerUsedBytes
	case model.ResellerBillingProductV2Ray:
		return reseller.V2RayUsedBytes
	default:
		return 0
	}
}

// chargeAcrossTiers prices deltaBytes against this reseller's manually
// configured ResellerBillingTier breakpoints for product, splitting the
// delta across however many tiers it spans -- e.g. if a reseller is at
// 490GB cumulative usage with a 0-500GB tier and a 500GB+ tier, and this
// tick's delta pushes them to 520GB, the first 10GB of that delta is
// priced at the lower tier's rate and the remaining 10GB at the higher
// tier's rate, rather than the whole delta landing arbitrarily on
// whichever tier the START or END point happens to fall in (either of
// which would make the exact tier boundary crossing point matter for
// revenue in a way the admin would find surprising and hard to reconcile
// against their own manual pricing table).
func (s *ResellerBillingService) chargeAcrossTiers(resellerID uint, product string, deltaBytes int64) (int64, error) {
	var reseller model.Reseller
	if err := s.db.Where("id = ?", resellerID).First(&reseller).Error; err != nil {
		return 0, err
	}
	usedBefore := s.cumulativeUsedBytes(reseller, product)

	var tiers []model.ResellerBillingTier
	if err := s.db.Where("reseller_id = ? AND product = ?", resellerID, product).Order("min_gb asc").Find(&tiers).Error; err != nil {
		return 0, err
	}
	if len(tiers) == 0 {
		return 0, nil
	}

	rangeStart := usedBefore
	rangeEnd := usedBefore + deltaBytes
	var totalCharge int64

	for _, tier := range tiers {
		tierStartBytes := tier.MinGB * bytesPerGB
		tierEndBytes := int64(1<<62 - 1) // effectively unbounded for a nil MaxGB
		if tier.MaxGB != nil {
			tierEndBytes = *tier.MaxGB * bytesPerGB
		}

		overlapStart := rangeStart
		if tierStartBytes > overlapStart {
			overlapStart = tierStartBytes
		}
		overlapEnd := rangeEnd
		if tierEndBytes < overlapEnd {
			overlapEnd = tierEndBytes
		}
		if overlapEnd <= overlapStart {
			continue
		}

		overlapBytes := overlapEnd - overlapStart
		if tier.PricePerGBAmount > 0 {
			totalCharge += (overlapBytes * tier.PricePerGBAmount) / bytesPerGB
		}
	}

	return totalCharge, nil
}

// ClearBillingSuspension clears BillingSuspended once a Payment-based
// reseller's wallet is topped up (or their DebtLimitAmount raised) enough
// to cover their current balance -- called from UpdateReseller/wallet
// credit flows, mirroring resumeQuotaSuspendedPeers/
// resumeQuotaSuspendedUserManagerAccounts's own "raise detected, resume"
// pattern for the byte-quota system. Only clears the flag itself; actually
// re-enabling the reseller's disabled peers/accounts/packages is the
// caller's responsibility via the same resume helpers volume-based quota
// raises already use (a suspended-by-billing resource sets the identical
// suspended_by_quota/was_active_before_suspend flags, so no separate resume
// path is needed).
func (s *ResellerBillingService) ClearBillingSuspension(resellerID uint) error {
	return s.db.Model(&model.Reseller{}).Where("id = ?", resellerID).Update("billing_suspended", false).Error
}

// GetPrices returns every ResellerBillingPrice row configured for
// resellerID, including the product-wide fallback rows (LocationKey ==
// "") alongside any location-scoped ones -- a (product, location) with no
// row simply isn't in the returned slice (see model.ResellerBillingPrice's
// own doc comment on why a missing row is meaningfully different from a
// zero-price row).
func (s *ResellerBillingService) GetPrices(resellerID uint) ([]schema.ResellerBillingPriceResponse, error) {
	var prices []model.ResellerBillingPrice
	if err := s.db.Where("reseller_id = ?", resellerID).Find(&prices).Error; err != nil {
		return nil, err
	}

	result := make([]schema.ResellerBillingPriceResponse, 0, len(prices))
	for _, p := range prices {
		result = append(result, schema.ResellerBillingPriceResponse{
			Product:          p.Product,
			LocationKey:      p.LocationKey,
			PricePerGBAmount: p.PricePerGBAmount,
		})
	}
	return result, nil
}

// SetPrices upserts one ResellerBillingPrice row per entry -- each entry
// independently keyed by (Product, LocationKey) so a single call can set
// the product-wide fallback (LocationKey == "") for one product and a
// location-scoped override for another in the same request. See
// UpdateResellerBillingPricesRequest's own doc comment for why this is a
// partial upsert (untouched product/location combinations keep whatever
// price they already had) rather than a full replace-all.
func (s *ResellerBillingService) SetPrices(resellerID uint, prices []schema.ResellerBillingPriceEntry) error {
	for _, entry := range prices {
		row := model.ResellerBillingPrice{
			ResellerID:       resellerID,
			Product:          entry.Product,
			LocationKey:      entry.LocationKey,
			PricePerGBAmount: entry.PricePerGBAmount,
		}
		if err := s.db.Clauses(clause.OnConflict{
			Columns:   []clause.Column{{Name: "reseller_id"}, {Name: "product"}, {Name: "location_key"}},
			DoUpdates: clause.AssignmentColumns([]string{"price_per_gb_amount"}),
		}).Create(&row).Error; err != nil {
			return fmt.Errorf("failed to set price for product %s (location %q): %w", entry.Product, entry.LocationKey, err)
		}
	}
	return nil
}

// GetTiers returns every ResellerBillingTier row configured for
// resellerID, ordered by product then MinGB ascending -- the natural
// display order for a tier table.
func (s *ResellerBillingService) GetTiers(resellerID uint) ([]schema.ResellerBillingTierResponse, error) {
	var tiers []model.ResellerBillingTier
	if err := s.db.Where("reseller_id = ?", resellerID).Order("product asc, min_gb asc").Find(&tiers).Error; err != nil {
		return nil, err
	}

	result := make([]schema.ResellerBillingTierResponse, 0, len(tiers))
	for _, t := range tiers {
		result = append(result, schema.ResellerBillingTierResponse{
			Product:          t.Product,
			MinGB:            t.MinGB,
			MaxGB:            t.MaxGB,
			PricePerGBAmount: t.PricePerGBAmount,
		})
	}
	return result, nil
}

// SetTiers REPLACES every ResellerBillingTier row for resellerID+product
// with the given tiers -- a full replace (unlike SetPrices' partial
// upsert) since tiers only make sense as one coherent, non-overlapping
// breakpoint table per product; a partial upsert could leave stale tiers
// from a previous edit still active. Passing an empty tiers slice removes
// tiering for that product entirely, reverting to flat/location pricing.
func (s *ResellerBillingService) SetTiers(resellerID uint, product string, tiers []schema.ResellerBillingTierEntry) error {
	return s.db.Transaction(func(tx *gorm.DB) error {
		if err := tx.Unscoped().Where("reseller_id = ? AND product = ?", resellerID, product).Delete(&model.ResellerBillingTier{}).Error; err != nil {
			return fmt.Errorf("failed to clear existing tiers for product %s: %w", product, err)
		}
		for _, tier := range tiers {
			row := model.ResellerBillingTier{
				ResellerID:       resellerID,
				Product:          product,
				MinGB:            tier.MinGB,
				MaxGB:            tier.MaxGB,
				PricePerGBAmount: tier.PricePerGBAmount,
			}
			if err := tx.Create(&row).Error; err != nil {
				return fmt.Errorf("failed to create tier (min_gb=%d) for product %s: %w", tier.MinGB, product, err)
			}
		}
		return nil
	})
}

func productLabelFa(product string) string {
	switch product {
	case model.ResellerBillingProductWireGuard:
		return "وایرگارد"
	case model.ResellerBillingProductUserManager:
		return "یوزرمنیجر"
	case model.ResellerBillingProductV2Ray:
		return "V2Ray"
	default:
		return product
	}
}
