package service

import (
	"errors"
	"fmt"
	"testing"
	"time"

	"github.com/glebarez/sqlite"
	"gorm.io/gorm"

	"github.com/maahdima/mwp/api/dataservice/model"
)

func openBotSettingsTestDB(t *testing.T) *gorm.DB {
	t.Helper()
	dsn := fmt.Sprintf("file:bot_settings_extra_admin_test_%d?mode=memory&cache=shared", time.Now().UnixNano())
	db, err := gorm.Open(sqlite.Open(dsn), &gorm.Config{})
	if err != nil {
		t.Fatalf("failed to open test db: %v", err)
	}
	if err := db.AutoMigrate(&model.BotSettings{}, &model.BotExtraAdminChatID{}); err != nil {
		t.Fatalf("failed to migrate test db: %v", err)
	}
	return db
}

// TestAddExtraAdminChatID_RejectsNonNumericChatID is the core regression
// test for the confirmed reported incident behind "تأیید دو مرحله‌ای...
// برای من ارسال میشه ولی به آیدی هایی که اظافه کردم نه" (OTP reaches me
// but not the IDs I added): the production database was found to have a
// row with chat_id literally equal to "CHAT_ID" -- the bot's own usage
// hint's placeholder text, typed verbatim instead of substituted with a
// real numeric id. sendRaw's own strconv.ParseInt would silently fail
// forever for a row like that. AddExtraAdminChatID must now reject this
// at add-time instead of persisting a permanently-dead recipient.
func TestAddExtraAdminChatID_RejectsNonNumericChatID(t *testing.T) {
	db := openBotSettingsTestDB(t)
	svc := NewBotSettingsService(db)

	_, err := svc.AddExtraAdminChatID("CHAT_ID", nil)
	if err == nil {
		t.Fatal("expected an error for a non-numeric chat id, got nil")
	}
	if !errors.Is(err, ErrInvalidTelegramChatID) {
		t.Errorf("expected ErrInvalidTelegramChatID, got %v", err)
	}

	var count int64
	db.Model(&model.BotExtraAdminChatID{}).Count(&count)
	if count != 0 {
		t.Errorf("expected no row to be persisted for an invalid chat id, got %d rows", count)
	}
}

// TestAddExtraAdminChatID_AcceptsValidNumericChatID confirms the normal,
// correct case still works -- both positive (private chat) and negative
// (group/channel) numeric ids.
func TestAddExtraAdminChatID_AcceptsValidNumericChatID(t *testing.T) {
	db := openBotSettingsTestDB(t)
	svc := NewBotSettingsService(db)

	for _, id := range []string{"7230103908", "-1001234567890"} {
		entry, err := svc.AddExtraAdminChatID(id, nil)
		if err != nil {
			t.Fatalf("expected %q to be accepted, got error: %v", id, err)
		}
		if entry.ChatID != id {
			t.Errorf("expected stored chat id %q, got %q", id, entry.ChatID)
		}
	}
}

// TestAddExtraAdminChatID_RevivesSoftDeletedRow is the regression test
// for the second, related bug found while investigating the OTP report:
// the production database's extra-admin rows were ALL soft-deleted (the
// admin had removed them at some point), and chat_id carries a plain
// unique index not scoped to deleted_at -- so re-adding the exact same
// id after a removal would previously fail with a unique-constraint
// violation, a confusing "already added" error for an id that, from the
// admin's point of view, was no longer registered at all. Add must
// revive the soft-deleted row instead.
func TestAddExtraAdminChatID_RevivesSoftDeletedRow(t *testing.T) {
	db := openBotSettingsTestDB(t)
	svc := NewBotSettingsService(db)

	oldLabel := "old label"
	entry, err := svc.AddExtraAdminChatID("7230103908", &oldLabel)
	if err != nil {
		t.Fatalf("failed to add initial entry: %v", err)
	}

	if err := svc.RemoveExtraAdminChatID(entry.ID); err != nil {
		t.Fatalf("failed to remove entry: %v", err)
	}

	var count int64
	db.Model(&model.BotExtraAdminChatID{}).Count(&count)
	if count != 0 {
		t.Fatalf("expected the soft-deleted row to be excluded from a normal count, got %d", count)
	}

	newLabel := "new label"
	revived, err := svc.AddExtraAdminChatID("7230103908", &newLabel)
	if err != nil {
		t.Fatalf("expected re-adding the same chat id after removal to succeed, got error: %v", err)
	}
	if revived.ChatID != "7230103908" {
		t.Errorf("expected revived row's chat id to be 7230103908, got %q", revived.ChatID)
	}
	if revived.Label == nil || *revived.Label != newLabel {
		t.Errorf("expected revived row's label to be updated to %q, got %v", newLabel, revived.Label)
	}

	ids, err := svc.ListExtraAdminChatIDs()
	if err != nil {
		t.Fatalf("failed to list extra admin chat ids: %v", err)
	}
	if len(ids) != 1 {
		t.Fatalf("expected exactly 1 active extra admin chat id after revival, got %d", len(ids))
	}
}

// TestAddExtraAdminChatID_StillRejectsDuplicateActiveChatID confirms the
// revival fix doesn't weaken the original duplicate-prevention behavior:
// adding a chat id that is CURRENTLY active (not soft-deleted) must still
// fail.
func TestAddExtraAdminChatID_StillRejectsDuplicateActiveChatID(t *testing.T) {
	db := openBotSettingsTestDB(t)
	svc := NewBotSettingsService(db)

	if _, err := svc.AddExtraAdminChatID("7230103908", nil); err != nil {
		t.Fatalf("failed to add initial entry: %v", err)
	}

	if _, err := svc.AddExtraAdminChatID("7230103908", nil); err == nil {
		t.Fatal("expected an error when adding a chat id that is already active, got nil")
	}
}
