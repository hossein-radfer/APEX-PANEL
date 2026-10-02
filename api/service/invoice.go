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

type Invoice struct {
	db     *gorm.DB
	logger *zap.Logger
}

func NewInvoice(db *gorm.DB) *Invoice {
	return &Invoice{
		db:     db,
		logger: zap.L().Named("InvoiceService"),
	}
}

func generateInvoiceNumber() string {
	return fmt.Sprintf("INV-%s-%s", time.Now().Format("20060102"), uuid.NewString()[:8])
}

func (s *Invoice) CreateInvoice(
	resellerID uint,
	amount int64,
	description string,
	dueDate *time.Time,
	periodStart *time.Time,
	periodEnd *time.Time,
	notes *string,
	autoIssue bool,
) (*model.Invoice, error) {
	if amount <= 0 {
		return nil, errors.New("invoice amount must be positive")
	}

	now := time.Now()
	status := "DRAFT"
	var issuedAt *time.Time
	if autoIssue {
		status = "ISSUED"
		issuedAt = &now
	}

	invoice := model.Invoice{
		ResellerID:    resellerID,
		InvoiceNumber: generateInvoiceNumber(),
		Status:        status,
		Amount:        amount,
		Description:   description,
		DueDate:       dueDate,
		IssuedAt:      issuedAt,
		Notes:         notes,
	}

	if periodStart != nil {
		invoice.PeriodStart = *periodStart
	}
	if periodEnd != nil {
		invoice.PeriodEnd = *periodEnd
	}

	if err := s.db.Create(&invoice).Error; err != nil {
		s.logger.Error("failed to create invoice", zap.Uint("reseller_id", resellerID), zap.Error(err))
		return nil, err
	}

	return &invoice, nil
}

func (s *Invoice) GetInvoice(invoiceID uint) (*model.Invoice, error) {
	var invoice model.Invoice
	if err := s.db.First(&invoice, "id = ?", invoiceID).Error; err != nil {
		return nil, err
	}
	return &invoice, nil
}

func (s *Invoice) ListInvoicesByReseller(resellerID uint, limit int) ([]model.Invoice, error) {
	var invoices []model.Invoice
	if err := s.db.
		Where("reseller_id = ?", resellerID).
		Order("created_at DESC").
		Limit(limit).
		Find(&invoices).Error; err != nil {
		s.logger.Error("failed to list invoices", zap.Uint("reseller_id", resellerID), zap.Error(err))
		return nil, err
	}

	return invoices, nil
}

func (s *Invoice) IssueInvoice(invoiceID uint) (*model.Invoice, error) {
	invoice, err := s.GetInvoice(invoiceID)
	if err != nil {
		return nil, err
	}

	if invoice.Status == "PAID" {
		return nil, errors.New("paid invoice cannot be issued")
	}
	if invoice.Status == "CANCELLED" {
		return nil, errors.New("cancelled invoice cannot be issued")
	}
	if invoice.Status == "ISSUED" {
		return invoice, nil
	}

	now := time.Now()
	if err := s.db.Model(invoice).Updates(map[string]interface{}{
		"status":    "ISSUED",
		"issued_at": now,
	}).Error; err != nil {
		s.logger.Error("failed to issue invoice", zap.Uint("invoice_id", invoiceID), zap.Error(err))
		return nil, err
	}

	invoice.Status = "ISSUED"
	invoice.IssuedAt = &now
	return invoice, nil
}

func (s *Invoice) CancelInvoice(invoiceID uint) (*model.Invoice, error) {
	invoice, err := s.GetInvoice(invoiceID)
	if err != nil {
		return nil, err
	}

	if invoice.Status == "PAID" {
		return nil, errors.New("paid invoice cannot be cancelled")
	}
	if invoice.Status == "CANCELLED" {
		return invoice, nil
	}

	if err := s.db.Model(invoice).Update("status", "CANCELLED").Error; err != nil {
		s.logger.Error("failed to cancel invoice", zap.Uint("invoice_id", invoiceID), zap.Error(err))
		return nil, err
	}

	invoice.Status = "CANCELLED"
	return invoice, nil
}

func (s *Invoice) PayInvoice(invoiceID uint) (*model.Invoice, error) {
	tx := s.db.Begin()
	defer func() {
		if r := recover(); r != nil {
			tx.Rollback()
		}
	}()

	var invoice model.Invoice
	if err := tx.Clauses(clause.Locking{Strength: "UPDATE"}).First(&invoice, "id = ?", invoiceID).Error; err != nil {
		tx.Rollback()
		return nil, err
	}

	if invoice.Status != "ISSUED" {
		tx.Rollback()
		return nil, fmt.Errorf("invoice is not payable in status: %s", invoice.Status)
	}

	var wallet model.Wallet
	if err := tx.Where("reseller_id = ?", invoice.ResellerID).First(&wallet).Error; err != nil {
		if !errors.Is(err, gorm.ErrRecordNotFound) {
			tx.Rollback()
			return nil, err
		}

		wallet = model.Wallet{
			ResellerID:    invoice.ResellerID,
			BalanceAmount: 0,
		}
		if err := tx.Create(&wallet).Error; err != nil {
			tx.Rollback()
			return nil, err
		}
	}

	if wallet.IsFrozen {
		tx.Rollback()
		return nil, errors.New("wallet is frozen")
	}

	if wallet.BalanceAmount < invoice.Amount {
		tx.Rollback()
		return nil, fmt.Errorf("insufficient funds: have %d, need %d", wallet.BalanceAmount, invoice.Amount)
	}

	newBalance := wallet.BalanceAmount - invoice.Amount
	refType := "INVOICE_PAYMENT"
	refID := invoice.ID
	ledgerEntry := model.LedgerEntry{
		ResellerID:    invoice.ResellerID,
		WalletID:      wallet.ID,
		TransactionID: uuid.NewString(),
		EntryType:     "DEBIT",
		Amount:        -invoice.Amount,
		BalanceAfter:  newBalance,
		Description:   "Invoice payment " + invoice.InvoiceNumber,
		ReferenceType: &refType,
		ReferenceID:   &refID,
	}

	if err := tx.Create(&ledgerEntry).Error; err != nil {
		tx.Rollback()
		return nil, err
	}

	now := time.Now()
	if err := tx.Model(&wallet).Updates(map[string]interface{}{
		"balance_amount": newBalance,
		"last_used_at":   now,
	}).Error; err != nil {
		tx.Rollback()
		return nil, err
	}

	if err := tx.Model(&invoice).Updates(map[string]interface{}{
		"status":  "PAID",
		"paid_at": now,
	}).Error; err != nil {
		tx.Rollback()
		return nil, err
	}

	if err := tx.Commit().Error; err != nil {
		return nil, err
	}

	invoice.Status = "PAID"
	invoice.PaidAt = &now
	return &invoice, nil
}
