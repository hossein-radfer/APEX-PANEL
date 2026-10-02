package service

import (
	"fmt"

	"go.uber.org/zap"
	"gorm.io/gorm"
	"gorm.io/gorm/clause"

	"github.com/maahdima/mwp/api/dataservice/model"
)

const packagePurchaseReferenceType = "TRAFFIC_PACKAGE_PURCHASE"

// PackagePurchaseService is the reseller-facing "buy an extra traffic
// package" flow: checks the package is still on sale, debits the wallet
// (which itself atomically checks sufficient funds and isn't frozen), then
// grants the traffic and records the purchase receipt.
//
// Wallet.recordTransaction already opens and commits its own DB transaction
// internally (it needs SELECT ... FOR UPDATE row locking that isn't
// expressible from inside a nested db.Transaction callback on this driver),
// so it can't be composed inside one larger outer transaction here. Instead:
// the debit is treated as the durable, atomic "was payment taken" step, and
// the grant step (record the purchase + raise QuotaBytes) runs in its own
// transaction immediately after. If the grant step fails after a successful
// debit, the wallet charge is automatically reversed with a REFUND credit so
// a reseller is never charged for a package they didn't actually receive.
type PackagePurchaseService struct {
	db             *gorm.DB
	walletService  *Wallet
	packageService *TrafficPackageService
	botNotifier    *BotNotifier
	logger         *zap.Logger
}

func NewPackagePurchaseService(db *gorm.DB, walletService *Wallet, packageService *TrafficPackageService) *PackagePurchaseService {
	return &PackagePurchaseService{
		db:             db,
		walletService:  walletService,
		packageService: packageService,
		logger:         zap.L().Named("PackagePurchaseService"),
	}
}

// SetBotNotifier wires the Telegram purchase-receipt notifier after
// construction, since BotNotifier itself depends on the bot service, which
// is constructed after PackagePurchaseService during startup. Safe to leave
// unset -- PurchasePackage no-ops the notification if nil.
func (s *PackagePurchaseService) SetBotNotifier(notifier *BotNotifier) {
	s.botNotifier = notifier
}

// PurchasePackage debits resellerID's wallet for trafficPackageID's price,
// raises the reseller's QuotaBytes by the package's TrafficBytes, and
// records a PackagePurchase receipt. Returns the receipt.
func (s *PackagePurchaseService) PurchasePackage(resellerID, trafficPackageID uint) (*model.PackagePurchase, error) {
	pkg, err := s.packageService.GetTrafficPackage(trafficPackageID)
	if err != nil {
		return nil, err
	}

	description := fmt.Sprintf("Purchased traffic package: %s", pkg.Name)
	ledgerEntry, err := s.walletService.Debit(resellerID, pkg.PriceAmount, description, refString(packagePurchaseReferenceType), &pkg.ID)
	if err != nil {
		// Insufficient funds / frozen wallet / any other debit failure —
		// nothing was charged, nothing to roll back.
		return nil, err
	}

	purchase, grantErr := s.grantPurchase(resellerID, pkg, ledgerEntry.ID)
	if grantErr != nil {
		s.logger.Error("failed to grant purchased package after successful debit; issuing automatic refund",
			zap.Uint("reseller_id", resellerID), zap.Uint("traffic_package_id", trafficPackageID), zap.Error(grantErr))

		refundDescription := fmt.Sprintf("Refund: failed to grant package %s", pkg.Name)
		if _, refundErr := s.walletService.Credit(resellerID, pkg.PriceAmount, refundDescription, refString("REFUND"), &ledgerEntry.ID); refundErr != nil {
			// This is the one genuinely bad outcome: charged, not granted,
			// and the refund itself failed. Logged at Error so it's visible
			// to an operator; nothing more can be done automatically.
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

// grantPurchase raises the reseller's quota and writes the purchase receipt
// as a single atomic unit.
func (s *PackagePurchaseService) grantPurchase(resellerID uint, pkg *model.TrafficPackage, ledgerEntryID uint) (*model.PackagePurchase, error) {
	var purchase model.PackagePurchase

	err := s.db.Transaction(func(tx *gorm.DB) error {
		// Row-locked read: two concurrent purchases for the same reseller
		// must not both read the same starting QuotaBytes and clobber each
		// other's addition (lost-update).
		var reseller model.Reseller
		if err := tx.Clauses(clause.Locking{Strength: "UPDATE"}).Where("id = ?", resellerID).First(&reseller).Error; err != nil {
			return err
		}

		if reseller.QuotaBytes != nil {
			newQuota := *reseller.QuotaBytes + pkg.TrafficBytes
			updates := map[string]interface{}{"quota_bytes": newQuota}

			// Same reasoning as Reseller.UpdateReseller's quota-increase
			// path: if this purchase gives the reseller more than 10%
			// headroom again, let the low-quota Telegram warning fire again
			// next time they cross back under that threshold.
			if newQuota > 0 && (newQuota-reseller.UsedBytes)*100/newQuota > 10 {
				updates["quota_warning_sent"] = false
			}

			if err := tx.Model(&model.Reseller{}).Where("id = ?", resellerID).Updates(updates).Error; err != nil {
				return err
			}
		}
		// A nil QuotaBytes means "unlimited" — nothing to raise, the
		// purchase still grants a receipt but there's no cap to bump.

		purchase = model.PackagePurchase{
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

// ListPurchases returns a reseller's purchase history, most recent first.
func (s *PackagePurchaseService) ListPurchases(resellerID uint, limit int) ([]model.PackagePurchase, error) {
	if limit <= 0 || limit > 200 {
		limit = 50
	}

	var purchases []model.PackagePurchase
	// CreatedAt has only second resolution, so two purchases in the same
	// second would tie under "created_at DESC" alone; id DESC as a
	// secondary key keeps same-second purchases in actual insertion order.
	if err := s.db.Where("reseller_id = ?", resellerID).Order("created_at DESC, id DESC").Limit(limit).Find(&purchases).Error; err != nil {
		s.logger.Error("failed to list package purchases", zap.Uint("reseller_id", resellerID), zap.Error(err))
		return nil, err
	}

	return purchases, nil
}

func refString(s string) *string {
	return &s
}
