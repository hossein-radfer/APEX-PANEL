package service

import (
	"errors"
	"testing"
	"time"

	"github.com/glebarez/sqlite"
	"github.com/maahdima/mwp/api/dataservice/model"
	"gorm.io/gorm"
)

func openInvoiceTestDB(t *testing.T) *gorm.DB {
	t.Helper()

	db, err := gorm.Open(sqlite.Open("file::memory:?cache=shared"), &gorm.Config{})
	if err != nil {
		t.Fatalf("failed to open test db: %v", err)
	}

	if err := db.AutoMigrate(&model.Reseller{}, &model.Wallet{}, &model.LedgerEntry{}, &model.Invoice{}); err != nil {
		t.Fatalf("failed to migrate test db: %v", err)
	}

	return db
}

func TestInvoiceCreateIssueAndPay(t *testing.T) {
	db := openInvoiceTestDB(t)
	walletSvc := NewWallet(db)
	invoiceSvc := NewInvoice(db)

	reseller := model.Reseller{Name: "invoice-owner"}
	if err := db.Create(&reseller).Error; err != nil {
		t.Fatalf("failed to create reseller: %v", err)
	}

	if _, err := walletSvc.Credit(reseller.ID, 5000, "seed", nil, nil); err != nil {
		t.Fatalf("failed to credit wallet: %v", err)
	}

	dueAt := time.Now().Add(48 * time.Hour)
	invoice, err := invoiceSvc.CreateInvoice(reseller.ID, 1200, "Monthly charge", &dueAt, nil, nil, nil, false)
	if err != nil {
		t.Fatalf("failed to create invoice: %v", err)
	}
	if invoice.Status != "DRAFT" {
		t.Fatalf("expected DRAFT invoice, got %s", invoice.Status)
	}

	issued, err := invoiceSvc.IssueInvoice(invoice.ID)
	if err != nil {
		t.Fatalf("failed to issue invoice: %v", err)
	}
	if issued.Status != "ISSUED" {
		t.Fatalf("expected ISSUED invoice, got %s", issued.Status)
	}

	paid, err := invoiceSvc.PayInvoice(invoice.ID)
	if err != nil {
		t.Fatalf("failed to pay invoice: %v", err)
	}
	if paid.Status != "PAID" {
		t.Fatalf("expected PAID invoice, got %s", paid.Status)
	}

	balance, err := walletSvc.GetWalletBalance(reseller.ID)
	if err != nil {
		t.Fatalf("failed to get wallet balance: %v", err)
	}
	if balance != 3800 {
		t.Fatalf("expected wallet balance 3800, got %d", balance)
	}

	entries, err := walletSvc.GetLedgerHistory(reseller.ID, 10)
	if err != nil {
		t.Fatalf("failed to load ledger: %v", err)
	}
	if len(entries) != 2 {
		t.Fatalf("expected 2 ledger entries, got %d", len(entries))
	}
	if entries[0].ReferenceType == nil || *entries[0].ReferenceType != "INVOICE_PAYMENT" {
		t.Fatalf("expected invoice payment ledger reference, got %#v", entries[0].ReferenceType)
	}
}

func TestInvoicePayFailsOnInsufficientFunds(t *testing.T) {
	db := openInvoiceTestDB(t)
	invoiceSvc := NewInvoice(db)

	reseller := model.Reseller{Name: "invoice-owner-2"}
	if err := db.Create(&reseller).Error; err != nil {
		t.Fatalf("failed to create reseller: %v", err)
	}

	invoice, err := invoiceSvc.CreateInvoice(reseller.ID, 1200, "Monthly charge", nil, nil, nil, nil, true)
	if err != nil {
		t.Fatalf("failed to create invoice: %v", err)
	}

	_, err = invoiceSvc.PayInvoice(invoice.ID)
	if err == nil {
		t.Fatalf("expected insufficient funds error")
	}

	loaded, getErr := invoiceSvc.GetInvoice(invoice.ID)
	if getErr != nil {
		t.Fatalf("failed to reload invoice: %v", getErr)
	}
	if loaded.Status != "ISSUED" {
		t.Fatalf("invoice status should remain ISSUED on failed payment, got %s", loaded.Status)
	}
}

func TestInvoiceCancelPreventsPay(t *testing.T) {
	db := openInvoiceTestDB(t)
	invoiceSvc := NewInvoice(db)

	reseller := model.Reseller{Name: "invoice-owner-3"}
	if err := db.Create(&reseller).Error; err != nil {
		t.Fatalf("failed to create reseller: %v", err)
	}

	invoice, err := invoiceSvc.CreateInvoice(reseller.ID, 900, "One-time", nil, nil, nil, nil, true)
	if err != nil {
		t.Fatalf("failed to create invoice: %v", err)
	}

	cancelled, err := invoiceSvc.CancelInvoice(invoice.ID)
	if err != nil {
		t.Fatalf("failed to cancel invoice: %v", err)
	}
	if cancelled.Status != "CANCELLED" {
		t.Fatalf("expected CANCELLED invoice, got %s", cancelled.Status)
	}

	_, err = invoiceSvc.PayInvoice(invoice.ID)
	if err == nil {
		t.Fatalf("expected payment to fail for cancelled invoice")
	}

	if errors.Is(err, gorm.ErrRecordNotFound) {
		t.Fatalf("unexpected not found error: %v", err)
	}
}
