package service

import (
	"fmt"

	"go.uber.org/zap"
	"gorm.io/gorm"
	"gorm.io/gorm/clause"

	"github.com/maahdima/mwp/api/dataservice/model"
)

const userManagerPackagePurchaseReferenceType = "USER_MANAGER_TRAFFIC_PACKAGE_PURCHASE"

// UserManagerPackagePurchaseService mirrors PackagePurchaseService exactly,
// but credits the reseller's SEPARATE UserManagerQuotaBytes/UsedBytes pool
// instead of QuotaBytes/UsedBytes. See PackagePurchaseService's own doc
// comment for the debit/grant transaction-boundary rationale, which applies
// identically here.
type UserManagerPackagePurchaseService struct {
	db             *gorm.DB
	walletService  *Wallet
	packageService *UserManagerTrafficPackageService
	botNotifier    *BotNotifier
	logger         *zap.Logger
}

func NewUserManagerPackagePurchaseService(db *gorm.DB, walletService *Wallet, packageService *UserManagerTrafficPackageService) *UserManagerPackagePurchaseService {
	return &UserManagerPackagePurchaseService{
		db:             db,
		walletService:  walletService,
		packageService: packageService,
		logger:         zap.L().Named("UserManagerPackagePurchaseService"),
	}
}

// SetBotNotifier wires the Telegram purchase-receipt notifier after
// construction. Safe to leave unset -- PurchasePackage no-ops the
// notification if nil.
func (s *UserManagerPackagePurchaseService) SetBotNotifier(notifier *BotNotifier) {
	s.botNotifier = notifier
}

// PurchasePackage debits resellerID's wallet for trafficPackageID's price,
// raises the reseller's UserManagerQuotaBytes by the package's
// TrafficBytes, and records a UserManagerPackagePurchase receipt.
func (s *UserManagerPackagePurchaseService) PurchasePackage(resellerID, trafficPackageID uint) (*model.UserManagerPackagePurchase, error) {
	pkg, err := s.packageService.GetTrafficPackage(trafficPackageID)
	if err != nil {
		return nil, err
	}

	description := fmt.Sprintf("Purchased User Manager traffic package: %s", pkg.Name)
	ledgerEntry, err := s.walletService.Debit(resellerID, pkg.PriceAmount, description, refString(userManagerPackagePurchaseReferenceType), &pkg.ID)
	if err != nil {
		return nil, err
	}

	purchase, grantErr := s.grantPurchase(resellerID, pkg, ledgerEntry.ID)
	if grantErr != nil {
		s.logger.Error("failed to grant purchased user manager package after successful debit; issuing automatic refund",
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

func (s *UserManagerPackagePurchaseService) grantPurchase(resellerID uint, pkg *model.UserManagerTrafficPackage, ledgerEntryID uint) (*model.UserManagerPackagePurchase, error) {
	var purchase model.UserManagerPackagePurchase

	err := s.db.Transaction(func(tx *gorm.DB) error {
		var reseller model.Reseller
		if err := tx.Clauses(clause.Locking{Strength: "UPDATE"}).Where("id = ?", resellerID).First(&reseller).Error; err != nil {
			return err
		}

		if reseller.UserManagerQuotaBytes != nil {
			newQuota := *reseller.UserManagerQuotaBytes + pkg.TrafficBytes
			updates := map[string]interface{}{"user_manager_quota_bytes": newQuota}

			if err := tx.Model(&model.Reseller{}).Where("id = ?", resellerID).Updates(updates).Error; err != nil {
				return err
			}
		}
		// A nil UserManagerQuotaBytes means "unlimited" — nothing to raise.

		purchase = model.UserManagerPackagePurchase{
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

	return &purchase, nil
}

// ListPurchases returns a reseller's User Manager package purchase history,
// most recent first.
func (s *UserManagerPackagePurchaseService) ListPurchases(resellerID uint, limit int) ([]model.UserManagerPackagePurchase, error) {
	if limit <= 0 || limit > 200 {
		limit = 50
	}

	var purchases []model.UserManagerPackagePurchase
	if err := s.db.Where("reseller_id = ?", resellerID).Order("created_at DESC, id DESC").Limit(limit).Find(&purchases).Error; err != nil {
		s.logger.Error("failed to list user manager package purchases", zap.Uint("reseller_id", resellerID), zap.Error(err))
		return nil, err
	}

	return purchases, nil
}
