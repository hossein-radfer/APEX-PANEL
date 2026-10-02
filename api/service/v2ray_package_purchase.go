package service

import (
	"fmt"

	"go.uber.org/zap"
	"gorm.io/gorm"
	"gorm.io/gorm/clause"

	"github.com/maahdima/mwp/api/dataservice/model"
)

const v2rayPackagePurchaseReferenceType = "V2RAY_TRAFFIC_PACKAGE_PURCHASE"

// V2RayPackagePurchaseService mirrors UserManagerPackagePurchaseService
// exactly, but credits the reseller's SEPARATE V2RayQuotaBytes/
// V2RayUsedBytes pool instead of UserManagerQuotaBytes/UserManagerUsedBytes.
// See PackagePurchaseService's own doc comment for the debit/grant
// transaction-boundary rationale, which applies identically here.
type V2RayPackagePurchaseService struct {
	db             *gorm.DB
	walletService  *Wallet
	packageService *V2RayTrafficPackageService
	botNotifier    *BotNotifier
	v2rayResumer   v2RayResellerQuotaResumer
	logger         *zap.Logger
}

func NewV2RayPackagePurchaseService(db *gorm.DB, walletService *Wallet, packageService *V2RayTrafficPackageService) *V2RayPackagePurchaseService {
	return &V2RayPackagePurchaseService{
		db:             db,
		walletService:  walletService,
		packageService: packageService,
		logger:         zap.L().Named("V2RayPackagePurchaseService"),
	}
}

// SetBotNotifier wires the Telegram purchase-receipt notifier after
// construction. Safe to leave unset -- PurchasePackage no-ops the
// notification if nil.
func (s *V2RayPackagePurchaseService) SetBotNotifier(notifier *BotNotifier) {
	s.botNotifier = notifier
}

// SetV2RayResumer wires the same V2Ray reseller-quota resume capability
// Reseller.SetV2RayResumer uses (*V2RaySyncService.ResumePackagesForResellerQuota),
// so that a reseller topping up their V2Ray quota via this purchase flow
// also resumes any packages that were suspended for being over quota --
// matching the behavior of the admin's "Edit Reseller" update path. Safe
// to leave unset, in which case grantPurchase's own raise-detection
// below simply no-ops.
func (s *V2RayPackagePurchaseService) SetV2RayResumer(resumer v2RayResellerQuotaResumer) {
	s.v2rayResumer = resumer
}

// PurchasePackage debits resellerID's wallet for trafficPackageID's price,
// raises the reseller's V2RayQuotaBytes by the package's TrafficBytes, and
// records a V2RayPackagePurchase receipt.
func (s *V2RayPackagePurchaseService) PurchasePackage(resellerID, trafficPackageID uint) (*model.V2RayPackagePurchase, error) {
	pkg, err := s.packageService.GetTrafficPackage(trafficPackageID)
	if err != nil {
		return nil, err
	}

	description := fmt.Sprintf("Purchased V2Ray traffic package: %s", pkg.Name)
	ledgerEntry, err := s.walletService.Debit(resellerID, pkg.PriceAmount, description, refString(v2rayPackagePurchaseReferenceType), &pkg.ID)
	if err != nil {
		return nil, err
	}

	purchase, grantErr := s.grantPurchase(resellerID, pkg, ledgerEntry.ID)
	if grantErr != nil {
		s.logger.Error("failed to grant purchased v2ray package after successful debit; issuing automatic refund",
			zap.Uint("reseller_id", resellerID), zap.Uint("traffic_package_id", trafficPackageID), zap.Error(grantErr))

		refundDescription := fmt.Sprintf("Refund: failed to grant package %s", pkg.Name)
		if _, refundErr := s.walletService.Credit(resellerID, pkg.PriceAmount, refundDescription, refString("REFUND"), &ledgerEntry.ID); refundErr != nil {
			s.logger.Error("automatic refund also failed — reseller was charged without receiving the package",
				zap.Uint("reseller_id", resellerID), zap.Uint("traffic_package_id", trafficPackageID), zap.Error(refundErr))
		}

		return nil, fmt.Errorf("failed to grant package after payment, purchase was refunded: %w", grantErr)
	}

	if s.botNotifier != nil {
		var reseller model.Reseller
		if err := s.db.First(&reseller, resellerID).Error; err == nil {
			s.botNotifier.NotifyPurchaseReceipt(reseller, pkg.Name, pkg.TrafficBytes, pkg.PriceAmount)
		}
	}

	return purchase, nil
}

func (s *V2RayPackagePurchaseService) grantPurchase(resellerID uint, pkg *model.V2RayTrafficPackage, ledgerEntryID uint) (*model.V2RayPackagePurchase, error) {
	var purchase model.V2RayPackagePurchase
	quotaRaised := false

	err := s.db.Transaction(func(tx *gorm.DB) error {
		var reseller model.Reseller
		if err := tx.Clauses(clause.Locking{Strength: "UPDATE"}).Where("id = ?", resellerID).First(&reseller).Error; err != nil {
			return err
		}

		if reseller.V2RayQuotaBytes != nil {
			newQuota := *reseller.V2RayQuotaBytes + pkg.TrafficBytes

			// Same fix as UpdateReseller's own v2rayQuotaRaised (reseller.go)
			// and applyResellerV2RayQuota's own enforcement check
			// (v2ray_sync.go): reseller.V2RayUsedBytes is the STORED total,
			// which permanently includes V2RayDeletedUsageBytes (credited at
			// delete time so a reseller can't dodge Payment-mode tiered
			// pricing or erase usage history by deleting a package) -- using
			// it here meant a top-up sized to comfortably cover the
			// reseller's REAL current (live) usage could still fail to
			// register as "now under quota" whenever any package had ever
			// been deleted. wasOverQuota/willBeUnderQuota must compare
			// against the live sum over currently-existing packages instead,
			// matching enforcement exactly.
			var liveV2RayUsed int64
			if err := tx.Model(&model.V2RayPackageLocation{}).
				Joins("JOIN v2_ray_packages ON v2_ray_packages.id = v2_ray_package_locations.package_id").
				Where("v2_ray_packages.reseller_id = ?", resellerID).
				Select("COALESCE(SUM(v2_ray_package_locations.used_bytes_cached), 0)").
				Scan(&liveV2RayUsed).Error; err != nil {
				return err
			}

			wasOverQuota := liveV2RayUsed > *reseller.V2RayQuotaBytes
			willBeUnderQuota := newQuota >= liveV2RayUsed
			quotaRaised = wasOverQuota && willBeUnderQuota

			// GORM's default naming strategy splits "V2Ray" at the
			// letter/digit boundary, so the actual column is
			// "v2_ray_quota_bytes", not the more readable-looking
			// "v2ray_quota_bytes" (confirmed by the table name itself
			// being "v2_ray_packages", same rule).
			updates := map[string]interface{}{"v2_ray_quota_bytes": newQuota}

			if err := tx.Model(&model.Reseller{}).Where("id = ?", resellerID).Updates(updates).Error; err != nil {
				return err
			}
		}
		// A nil V2RayQuotaBytes means "unlimited" — nothing to raise.

		purchase = model.V2RayPackagePurchase{
			ResellerID:         resellerID,
			TrafficPackageID:   pkg.ID,
			TrafficPackageName: pkg.Name,
			TrafficBytes:       pkg.TrafficBytes,
			PriceAmount:        pkg.PriceAmount,
			LedgerEntryID:      &ledgerEntryID,
		}
		if err := tx.Create(&purchase).Error; err != nil {
			return err
		}

		return nil
	})
	if err != nil {
		return nil, err
	}

	// Run outside the transaction above -- same reasoning as
	// UpdateReseller's own identically-placed v2rayResumer call: real x-ui
	// network calls per package location must never hold this reseller
	// row's DB transaction open.
	if quotaRaised && s.v2rayResumer != nil {
		s.v2rayResumer.ResumePackagesForResellerQuota(resellerID)
	}

	return &purchase, nil
}

// ListPurchases returns a reseller's V2Ray package purchase history, most
// recent first.
func (s *V2RayPackagePurchaseService) ListPurchases(resellerID uint, limit int) ([]model.V2RayPackagePurchase, error) {
	if limit <= 0 || limit > 200 {
		limit = 50
	}

	var purchases []model.V2RayPackagePurchase
	if err := s.db.Where("reseller_id = ?", resellerID).Order("created_at DESC, id DESC").Limit(limit).Find(&purchases).Error; err != nil {
		s.logger.Error("failed to list v2ray package purchases", zap.Uint("reseller_id", resellerID), zap.Error(err))
		return nil, err
	}

	return purchases, nil
}
