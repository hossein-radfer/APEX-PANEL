package service

import (
	"testing"

	"github.com/maahdima/mwp/api/dataservice/model"
	"gorm.io/gorm"
)

// openBotCommandsPurchaseTestDB mirrors openBotCommandsTestDB (see
// bot_commands_test.go) plus the tables PurchasePackage's own flow needs.
func openBotCommandsPurchaseTestDB(t *testing.T) *gorm.DB {
	t.Helper()
	db := openBotCommandsTestDB(t)
	if err := db.AutoMigrate(&model.TrafficPackage{}, &model.PackagePurchase{}); err != nil {
		t.Fatalf("failed to migrate purchase tables: %v", err)
	}
	return db
}

// TestBuyPackageCommand_TextCommandMirrorsCallbackPurchase is the
// regression test for a confirmed, reported gap: the Telegram bot's
// package purchase/renewal flow was reachable ONLY via the "buy_pkg:<id>"
// inline-button callback, with no typed-command equivalent -- a reseller
// scripting their own renewal, or on a Telegram client where inline
// buttons are inconvenient, had no way in. "/buy <id>" must perform the
// exact same purchase (debit the wallet, grant the traffic) as the button.
func TestBuyPackageCommand_TextCommandMirrorsCallbackPurchase(t *testing.T) {
	db := openBotCommandsPurchaseTestDB(t)
	resellerSvc := NewReseller(db, nil)
	walletSvc := NewWallet(db)
	auditLogSvc := NewAuditLog(db)
	settingsSvc := NewBotSettingsService(db)
	handler := NewBotCommandHandler(db, resellerSvc, walletSvc, auditLogSvc, settingsSvc)

	trafficPackageSvc := NewTrafficPackageService(db)
	purchaseSvc := NewPackagePurchaseService(db, walletSvc, trafficPackageSvc)
	handler.SetPackageServices(trafficPackageSvc, purchaseSvc)

	reseller := model.Reseller{Name: "buy-cmd-test", Username: "buy-cmd-test"}
	if err := db.Create(&reseller).Error; err != nil {
		t.Fatalf("failed to create reseller: %v", err)
	}
	chatID := "444444"
	if err := db.Model(&reseller).Update("telegram_chat_id", chatID).Error; err != nil {
		t.Fatalf("failed to set telegram_chat_id: %v", err)
	}

	if _, err := walletSvc.Credit(reseller.ID, 100000, "top-up for test", nil, nil); err != nil {
		t.Fatalf("failed to credit wallet: %v", err)
	}

	pkg := model.TrafficPackage{Name: "10GB", TrafficBytes: 10 * 1024 * 1024 * 1024, PriceAmount: 5000, IsActive: true}
	if err := db.Create(&pkg).Error; err != nil {
		t.Fatalf("failed to create traffic package: %v", err)
	}

	message, err := handler.purchasePackageForChat(chatID, pkg.ID)
	if err != nil {
		t.Fatalf("expected /buy's underlying purchase to succeed, got error: %v", err)
	}
	if message == "" {
		t.Fatal("expected a non-empty confirmation message")
	}

	balance, err := walletSvc.GetWalletBalance(reseller.ID)
	if err != nil {
		t.Fatalf("failed to read wallet balance: %v", err)
	}
	if balance != 95000 {
		t.Fatalf("expected wallet balance 95000 after a 5000 Toman purchase from a 100000 top-up, got %d", balance)
	}

	var purchaseCount int64
	if err := db.Model(&model.PackagePurchase{}).Where("reseller_id = ?", reseller.ID).Count(&purchaseCount).Error; err != nil {
		t.Fatalf("failed to count purchases: %v", err)
	}
	if purchaseCount != 1 {
		t.Fatalf("expected exactly one purchase receipt recorded, got %d", purchaseCount)
	}
}

// TestBuyPackageCommand_UnregisteredChatIsRejected confirms the text
// command has the same reseller-identity guard as the button callback --
// an unregistered chat cannot purchase anything.
func TestBuyPackageCommand_UnregisteredChatIsRejected(t *testing.T) {
	db := openBotCommandsPurchaseTestDB(t)
	resellerSvc := NewReseller(db, nil)
	walletSvc := NewWallet(db)
	auditLogSvc := NewAuditLog(db)
	settingsSvc := NewBotSettingsService(db)
	handler := NewBotCommandHandler(db, resellerSvc, walletSvc, auditLogSvc, settingsSvc)

	trafficPackageSvc := NewTrafficPackageService(db)
	purchaseSvc := NewPackagePurchaseService(db, walletSvc, trafficPackageSvc)
	handler.SetPackageServices(trafficPackageSvc, purchaseSvc)

	pkg := model.TrafficPackage{Name: "10GB", TrafficBytes: 10 * 1024 * 1024 * 1024, PriceAmount: 5000, IsActive: true}
	if err := db.Create(&pkg).Error; err != nil {
		t.Fatalf("failed to create traffic package: %v", err)
	}

	if _, err := handler.purchasePackageForChat("not-a-registered-chat", pkg.ID); err == nil {
		t.Fatal("expected purchasePackageForChat to reject an unregistered chat id")
	}
}
