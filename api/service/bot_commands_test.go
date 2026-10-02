package service

import (
	"fmt"
	"testing"
	"time"

	"github.com/glebarez/sqlite"
	"github.com/maahdima/mwp/api/dataservice/model"
	"gorm.io/gorm"
)

func openBotCommandsTestDB(t *testing.T) *gorm.DB {
	t.Helper()

	dsn := fmt.Sprintf("file:bot_commands_%d?mode=memory&cache=shared", time.Now().UnixNano())
	db, err := gorm.Open(sqlite.Open(dsn), &gorm.Config{})
	if err != nil {
		t.Fatalf("failed to open test db: %v", err)
	}
	if err := db.AutoMigrate(&model.Reseller{}, &model.Wallet{}, &model.LedgerEntry{}, &model.BotSettings{}, &model.AuditLog{}); err != nil {
		t.Fatalf("failed to migrate test db: %v", err)
	}
	return db
}

// TestIsAdminChat_OnlyMatchesConfiguredAdminChatID pins down the security
// fix in sendPaymentsReport/sendTrafficReport/handleSetBackupCommand/
// handleQuickTopUpCallback: botPoller's allowlist admits both the admin
// chat and any registered reseller's chat, but cross-reseller reports and
// admin-only actions must additionally check the caller is SPECIFICALLY the
// admin, not just "some recognized chat" -- otherwise a reseller with
// TelegramChatID set could read every other reseller's wallet balance and
// traffic usage, or schedule backups / trigger wallet top-ups.
func TestIsAdminChat_OnlyMatchesConfiguredAdminChatID(t *testing.T) {
	db := openBotCommandsTestDB(t)
	resellerSvc := NewReseller(db, nil)
	walletSvc := NewWallet(db)
	auditLogSvc := NewAuditLog(db)
	settingsSvc := NewBotSettingsService(db)
	handler := NewBotCommandHandler(db, resellerSvc, walletSvc, auditLogSvc, settingsSvc)

	if _, err := settingsSvc.UpdateSettings(UpdateSettingsInput{
		AdminChatID: strPtr("111111"),
	}); err != nil {
		t.Fatalf("failed to set admin chat id: %v", err)
	}

	reseller := model.Reseller{Name: "chat-test", Username: "chat-test"}
	if err := db.Create(&reseller).Error; err != nil {
		t.Fatalf("failed to create reseller: %v", err)
	}
	chatID := "222222"
	if err := db.Model(&reseller).Update("telegram_chat_id", chatID).Error; err != nil {
		t.Fatalf("failed to set reseller telegram_chat_id: %v", err)
	}

	if !handler.isAdminChat("111111") {
		t.Fatalf("expected the configured admin chat id to be recognized as admin")
	}
	if handler.isAdminChat(chatID) {
		t.Fatalf("expected a registered RESELLER's chat id to NOT be recognized as admin -- this is the exact information-disclosure bug being regression-tested")
	}
	if handler.isAdminChat("999999999") {
		t.Fatalf("expected a completely unknown chat id to not be recognized as admin")
	}
}

// TestResellerForChat_ResolvesRegisteredResellerOnly confirms the lookup
// used by the /packages command and buy_pkg callback to identify "which
// reseller is texting me" only resolves chats that are actually linked to a
// reseller record, and returns nil (not a panic or wrong reseller) for any
// unregistered chat id.
func TestResellerForChat_ResolvesRegisteredResellerOnly(t *testing.T) {
	db := openBotCommandsTestDB(t)
	resellerSvc := NewReseller(db, nil)
	walletSvc := NewWallet(db)
	auditLogSvc := NewAuditLog(db)
	settingsSvc := NewBotSettingsService(db)
	handler := NewBotCommandHandler(db, resellerSvc, walletSvc, auditLogSvc, settingsSvc)

	reseller := model.Reseller{Name: "resolve-test", Username: "resolve-test"}
	if err := db.Create(&reseller).Error; err != nil {
		t.Fatalf("failed to create reseller: %v", err)
	}
	chatID := "333333"
	if err := db.Model(&reseller).Update("telegram_chat_id", chatID).Error; err != nil {
		t.Fatalf("failed to set telegram_chat_id: %v", err)
	}

	got := handler.resellerForChat(chatID)
	if got == nil {
		t.Fatalf("expected resellerForChat to resolve the registered chat id")
	}
	if got.ID != reseller.ID {
		t.Fatalf("expected resolved reseller id %d, got %d", reseller.ID, got.ID)
	}

	if handler.resellerForChat("not-a-registered-chat") != nil {
		t.Fatalf("expected resellerForChat to return nil for an unregistered chat id")
	}
}

func strPtr(s string) *string { return &s }
