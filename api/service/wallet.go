package service

import (
	"errors"
	"fmt"
	"time"

	"github.com/google/uuid"
	"go.uber.org/zap"
	"gorm.io/gorm"
	"gorm.io/gorm/clause"

	"github.com/maahdima/mwp/api/dataservice/model"
)

type Wallet struct {
	db     *gorm.DB
	logger *zap.Logger
}

// ErrInsufficientFunds, ErrWalletFrozen, and ErrDebtLimitExceeded are the
// only three rejection reasons recordTransaction can produce that reflect
// the reseller's actual financial state, as opposed to a transient
// infrastructure failure. Wrapping these as distinct sentinels lets
// ChargeUsage use errors.Is to suspend only on a genuine financial
// rejection, and simply retry next tick (propagating the error without
// suspending) on anything else.
var (
	ErrInsufficientFunds = errors.New("insufficient funds")
	ErrWalletFrozen      = errors.New("wallet is frozen")
	ErrDebtLimitExceeded = errors.New("debt limit exceeded")
)

func NewWallet(db *gorm.DB) *Wallet {
	return &Wallet{
		db:     db,
		logger: zap.L().Named("WalletService"),
	}
}

// GetOrCreateWallet ensures a reseller has a wallet
func (w *Wallet) GetOrCreateWallet(resellerID uint) (*model.Wallet, error) {
	var wallet model.Wallet
	if err := w.db.Where("reseller_id = ?", resellerID).First(&wallet).Error; err != nil {
		if !errors.Is(err, gorm.ErrRecordNotFound) {
			w.logger.Error("failed to fetch wallet", zap.Uint("reseller_id", resellerID), zap.Error(err))
			return nil, err
		}

		// Create new wallet
		wallet = model.Wallet{
			ResellerID:    resellerID,
			BalanceAmount: 0,
			IsFrozen:      false,
		}
		if err := w.db.Create(&wallet).Error; err != nil {
			w.logger.Error("failed to create wallet", zap.Uint("reseller_id", resellerID), zap.Error(err))
			return nil, err
		}
	}

	return &wallet, nil
}

// GetWalletBalance returns the current balance
func (w *Wallet) GetWalletBalance(resellerID uint) (int64, error) {
	wallet, err := w.GetOrCreateWallet(resellerID)
	if err != nil {
		return 0, err
	}

	return wallet.BalanceAmount, nil
}

// Credit adds funds to the wallet
func (w *Wallet) Credit(resellerID uint, amount int64, description string, referenceType *string, referenceID *uint) (*model.LedgerEntry, error) {
	if amount <= 0 {
		return nil, fmt.Errorf("credit amount must be positive")
	}

	return w.recordTransaction(resellerID, amount, "CREDIT", description, referenceType, referenceID, false, nil)
}

// Debit subtracts funds from the wallet. Rejected outright ("insufficient
// funds") if it would take the balance below zero -- this is the Prepaid
// enforcement primitive: Reseller.PaymentSubMode=PREPAID billing always
// calls this, never DebitAllowNegative.
func (w *Wallet) Debit(resellerID uint, amount int64, description string, referenceType *string, referenceID *uint) (*model.LedgerEntry, error) {
	if amount <= 0 {
		return nil, fmt.Errorf("debit amount must be positive")
	}

	return w.recordTransaction(resellerID, -amount, "DEBIT", description, referenceType, referenceID, true, nil)
}

// DebitAllowNegative subtracts funds from the wallet WITHOUT rejecting a
// resulting negative balance, up to debtLimit (nil = unlimited debt) --
// this is the Postpaid enforcement primitive: Reseller.PaymentSubMode=
// POSTPAID billing calls this instead of Debit, passing the reseller's own
// DebtLimitAmount through unchanged. Only exceeding debtLimit is rejected;
// a wallet already frozen is still rejected exactly like Debit.
func (w *Wallet) DebitAllowNegative(resellerID uint, amount int64, description string, referenceType *string, referenceID *uint, debtLimit *int64) (*model.LedgerEntry, error) {
	if amount <= 0 {
		return nil, fmt.Errorf("debit amount must be positive")
	}

	return w.recordTransaction(resellerID, -amount, "DEBIT", description, referenceType, referenceID, false, debtLimit)
}

// recordTransaction is the internal transaction recording mechanism.
// It locks the wallet row for the duration of the transaction so concurrent
// Credit/Debit calls on the same wallet cannot read a stale balance.
// debtLimit is only consulted when enforceFunds is false (DebitAllowNegative's
// case) -- nil means no ceiling at all; Credit/Debit's own calls always pass
// nil since enforceFunds=true already rejects any negative result outright.
func (w *Wallet) recordTransaction(resellerID uint, amount int64, entryType, description string, referenceType *string, referenceID *uint, enforceFunds bool, debtLimit *int64) (*model.LedgerEntry, error) {
	tx := w.db.Begin()
	if tx.Error != nil {
		return nil, tx.Error
	}
	defer func() {
		if r := recover(); r != nil {
			tx.Rollback()
		}
	}()

	var wallet model.Wallet
	if err := tx.Clauses(clause.Locking{Strength: "UPDATE"}).Where("reseller_id = ?", resellerID).First(&wallet).Error; err != nil {
		if !errors.Is(err, gorm.ErrRecordNotFound) {
			tx.Rollback()
			return nil, err
		}

		wallet = model.Wallet{
			ResellerID:    resellerID,
			BalanceAmount: 0,
		}
		if err := tx.Create(&wallet).Error; err != nil {
			tx.Rollback()
			return nil, err
		}
	}

	if enforceFunds {
		if wallet.IsFrozen {
			tx.Rollback()
			return nil, ErrWalletFrozen
		}
		if wallet.BalanceAmount < -amount {
			tx.Rollback()
			return nil, fmt.Errorf("%w: have %d, need %d", ErrInsufficientFunds, wallet.BalanceAmount, -amount)
		}
	}

	newBalance := wallet.BalanceAmount + amount

	// Postpaid debt ceiling: only relevant for a debit (amount < 0) made via
	// DebitAllowNegative (enforceFunds=false, debtLimit non-nil) -- a
	// frozen wallet still blocks the debit even though funds themselves
	// aren't enforced, matching Debit's own frozen-wallet check above.
	if !enforceFunds && amount < 0 {
		if wallet.IsFrozen {
			tx.Rollback()
			return nil, ErrWalletFrozen
		}
		if debtLimit != nil && -newBalance > *debtLimit {
			tx.Rollback()
			return nil, fmt.Errorf("%w: would owe %d, limit %d", ErrDebtLimitExceeded, -newBalance, *debtLimit)
		}
	}
	transactionID := uuid.New().String()

	ledgerEntry := model.LedgerEntry{
		ResellerID:    resellerID,
		WalletID:      wallet.ID,
		TransactionID: transactionID,
		EntryType:     entryType,
		Amount:        amount,
		BalanceAfter:  newBalance,
		Description:   description,
		ReferenceType: referenceType,
		ReferenceID:   referenceID,
	}

	if err := tx.Create(&ledgerEntry).Error; err != nil {
		tx.Rollback()
		w.logger.Error("failed to create ledger entry", zap.Error(err))
		return nil, err
	}

	if err := tx.Model(&wallet).Updates(map[string]interface{}{
		"balance_amount": newBalance,
		"last_used_at":   time.Now(),
	}).Error; err != nil {
		tx.Rollback()
		w.logger.Error("failed to update wallet balance", zap.Error(err))
		return nil, err
	}

	if err := tx.Commit().Error; err != nil {
		w.logger.Error("failed to commit transaction", zap.Error(err))
		return nil, err
	}

	return &ledgerEntry, nil
}

// GetLedgerHistory returns recent ledger entries for a reseller
func (w *Wallet) GetLedgerHistory(resellerID uint, limit int) ([]model.LedgerEntry, error) {
	var entries []model.LedgerEntry
	if err := w.db.
		Where("reseller_id = ?", resellerID).
		Order("created_at DESC").
		Limit(limit).
		Find(&entries).Error; err != nil {
		w.logger.Error("failed to fetch ledger history", zap.Uint("reseller_id", resellerID), zap.Error(err))
		return nil, err
	}

	return entries, nil
}

// FreezeWallet freezes a reseller's wallet
func (w *Wallet) FreezeWallet(resellerID uint, reason string) error {
	wallet, err := w.GetOrCreateWallet(resellerID)
	if err != nil {
		return err
	}

	if err := w.db.Model(&wallet).Update("is_frozen", true).Update("frozen_reason", reason).Error; err != nil {
		w.logger.Error("failed to freeze wallet", zap.Uint("reseller_id", resellerID), zap.Error(err))
		return err
	}

	return nil
}

// UnfreezeWallet unfreezes a reseller's wallet
func (w *Wallet) UnfreezeWallet(resellerID uint) error {
	wallet, err := w.GetOrCreateWallet(resellerID)
	if err != nil {
		return err
	}

	if err := w.db.Model(&wallet).Update("is_frozen", false).Update("frozen_reason", nil).Error; err != nil {
		w.logger.Error("failed to unfreeze wallet", zap.Uint("reseller_id", resellerID), zap.Error(err))
		return err
	}

	return nil
}
