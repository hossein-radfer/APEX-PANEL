package service

import (
	"strings"
	"sync"
	"testing"
	"time"

	"github.com/glebarez/sqlite"
	"github.com/maahdima/mwp/api/dataservice/model"
	"gorm.io/gorm"
)

func openWalletTestDB(t *testing.T) *gorm.DB {
	t.Helper()

	db, err := gorm.Open(sqlite.Open("file::memory:?cache=shared"), &gorm.Config{})
	if err != nil {
		t.Fatalf("failed to open test db: %v", err)
	}

	if err := db.AutoMigrate(&model.Reseller{}, &model.Wallet{}, &model.LedgerEntry{}); err != nil {
		t.Fatalf("failed to migrate test db: %v", err)
	}

	return db
}

func TestWalletCreditDebitAndLedger(t *testing.T) {
	db := openWalletTestDB(t)
	svc := NewWallet(db)

	reseller := model.Reseller{Name: "wallet-owner"}
	if err := db.Create(&reseller).Error; err != nil {
		t.Fatalf("failed to create reseller: %v", err)
	}

	if _, err := svc.Credit(reseller.ID, 1000, "initial topup", nil, nil); err != nil {
		t.Fatalf("credit failed: %v", err)
	}

	if _, err := svc.Debit(reseller.ID, 400, "usage", nil, nil); err != nil {
		t.Fatalf("debit failed: %v", err)
	}

	balance, err := svc.GetWalletBalance(reseller.ID)
	if err != nil {
		t.Fatalf("balance check failed: %v", err)
	}
	if balance != 600 {
		t.Fatalf("expected balance 600, got %d", balance)
	}

	entries, err := svc.GetLedgerHistory(reseller.ID, 10)
	if err != nil {
		t.Fatalf("ledger history failed: %v", err)
	}
	if len(entries) != 2 {
		t.Fatalf("expected 2 ledger entries, got %d", len(entries))
	}
	hasCredit := false
	hasDebit := false
	for _, entry := range entries {
		if entry.EntryType == "CREDIT" {
			hasCredit = true
		}
		if entry.EntryType == "DEBIT" {
			hasDebit = true
		}
	}
	if !hasCredit || !hasDebit {
		t.Fatalf("expected both CREDIT and DEBIT ledger entries, got %#v", entries)
	}
}

func TestWalletFreezeAndInsufficientFunds(t *testing.T) {
	db := openWalletTestDB(t)
	svc := NewWallet(db)

	reseller := model.Reseller{Name: "wallet-owner-2"}
	if err := db.Create(&reseller).Error; err != nil {
		t.Fatalf("failed to create reseller: %v", err)
	}

	if _, err := svc.Credit(reseller.ID, 200, "seed", nil, nil); err != nil {
		t.Fatalf("credit failed: %v", err)
	}

	if _, err := svc.Debit(reseller.ID, 500, "overdraw", nil, nil); err == nil {
		t.Fatalf("expected insufficient funds error")
	}

	if err := svc.FreezeWallet(reseller.ID, "risk-check"); err != nil {
		t.Fatalf("freeze failed: %v", err)
	}

	if _, err := svc.GetWalletBalance(reseller.ID); err != nil {
		t.Fatalf("balance should be readable while frozen: %v", err)
	}

	if _, err := svc.Debit(reseller.ID, 50, "blocked while frozen", nil, nil); err == nil || !strings.Contains(err.Error(), "wallet is frozen") {
		t.Fatalf("expected frozen wallet debit error, got %v", err)
	}

	if err := svc.UnfreezeWallet(reseller.ID); err != nil {
		t.Fatalf("unfreeze failed: %v", err)
	}

	if _, err := svc.Debit(reseller.ID, 50, "allowed after unfreeze", nil, nil); err != nil {
		t.Fatalf("debit should succeed after unfreeze: %v", err)
	}
}

func TestWalletConcurrentDebitsDoNotOverdraw(t *testing.T) {
	db := openWalletTestDB(t)
	svc := NewWallet(db)

	reseller := model.Reseller{Name: "wallet-owner-concurrent"}
	if err := db.Create(&reseller).Error; err != nil {
		t.Fatalf("failed to create reseller: %v", err)
	}

	const startingBalance = 1000
	if _, err := svc.Credit(reseller.ID, startingBalance, "seed", nil, nil); err != nil {
		t.Fatalf("credit failed: %v", err)
	}

	const workers = 20
	const debitAmount = 100 // workers*debitAmount = 2000, double the balance

	var wg sync.WaitGroup
	successCount := int32(0)
	var mu sync.Mutex

	for i := 0; i < workers; i++ {
		wg.Add(1)
		go func() {
			defer wg.Done()

			// SQLite is single-writer; under contention a transaction can be told
			// the database is locked instead of queued. Retry those transient
			// failures the same way a real caller would, so the test measures
			// the balance invariant rather than SQLite's lock behavior.
			var err error
			for attempt := 0; attempt < 20; attempt++ {
				_, err = svc.Debit(reseller.ID, debitAmount, "concurrent spend", nil, nil)
				if err == nil || !strings.Contains(err.Error(), "database is locked") {
					break
				}
				time.Sleep(5 * time.Millisecond)
			}

			if err == nil {
				mu.Lock()
				successCount++
				mu.Unlock()
			}
		}()
	}
	wg.Wait()

	balance, err := svc.GetWalletBalance(reseller.ID)
	if err != nil {
		t.Fatalf("balance check failed: %v", err)
	}

	if balance < 0 {
		t.Fatalf("wallet went negative: balance=%d successfulDebits=%d", balance, successCount)
	}

	expectedBalance := int64(startingBalance) - int64(successCount)*debitAmount
	if balance != expectedBalance {
		t.Fatalf("balance inconsistent with ledger: got %d, want %d (successfulDebits=%d)", balance, expectedBalance, successCount)
	}
}
