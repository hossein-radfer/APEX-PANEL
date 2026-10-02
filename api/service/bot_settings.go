package service

import (
	"errors"
	"fmt"
	"strconv"
	"strings"

	"go.uber.org/zap"
	"gorm.io/gorm"

	"github.com/maahdima/mwp/api/dataservice/model"
)

// botSettingsSingletonID is the fixed primary key of the one BotSettings
// row this panel ever has -- there is exactly one Telegram bot per panel
// instance, so a singleton row (rather than a key-value SystemConfig store)
// keeps every setting strongly typed.
const botSettingsSingletonID = 1

type BotSettingsService struct {
	db     *gorm.DB
	logger *zap.Logger
}

func NewBotSettingsService(db *gorm.DB) *BotSettingsService {
	return &BotSettingsService{
		db:     db,
		logger: zap.L().Named("BotSettingsService"),
	}
}

// GetOrCreate returns the singleton BotSettings row, creating it with all
// defaults (bot disabled, every notification toggle off) the first time it's
// requested.
func (s *BotSettingsService) GetOrCreate() (*model.BotSettings, error) {
	var settings model.BotSettings
	if err := s.db.First(&settings, botSettingsSingletonID).Error; err != nil {
		if !errors.Is(err, gorm.ErrRecordNotFound) {
			s.logger.Error("failed to fetch bot settings", zap.Error(err))
			return nil, err
		}

		settings = model.BotSettings{Model: model.Model{ID: botSettingsSingletonID}}
		if err := s.db.Create(&settings).Error; err != nil {
			s.logger.Error("failed to create default bot settings", zap.Error(err))
			return nil, err
		}
	}

	return &settings, nil
}

// UpdateSettingsInput mirrors every admin-editable field on BotSettings.
// Pointer fields are optional partial updates; nil means "leave unchanged".
type UpdateSettingsInput struct {
	BotToken               *string
	AdminChatID            *string
	Enabled                *bool
	NotifyLoginAlerts      *bool
	NotifyPurchaseReceipts *bool
	NotifyLiveLog          *bool
	NotifyQuotaWarnings    *bool
	NotifyCriticalAlerts   *bool
	NotifyAccountStatus    *bool
	OtpEnabled             *bool

	Socks5Enabled  *bool
	Socks5Address  *string
	Socks5Username *string
	Socks5Password *string

	FaoximaDomain *string
}

func (s *BotSettingsService) UpdateSettings(input UpdateSettingsInput) (*model.BotSettings, error) {
	if _, err := s.GetOrCreate(); err != nil {
		return nil, err
	}

	updates := map[string]interface{}{}
	if input.BotToken != nil {
		updates["bot_token"] = *input.BotToken
	}
	if input.AdminChatID != nil {
		updates["admin_chat_id"] = *input.AdminChatID
	}
	if input.Enabled != nil {
		updates["enabled"] = *input.Enabled
	}
	if input.NotifyLoginAlerts != nil {
		updates["notify_login_alerts"] = *input.NotifyLoginAlerts
	}
	if input.NotifyPurchaseReceipts != nil {
		updates["notify_purchase_receipts"] = *input.NotifyPurchaseReceipts
	}
	if input.NotifyLiveLog != nil {
		updates["notify_live_log"] = *input.NotifyLiveLog
	}
	if input.NotifyQuotaWarnings != nil {
		updates["notify_quota_warnings"] = *input.NotifyQuotaWarnings
	}
	if input.NotifyCriticalAlerts != nil {
		updates["notify_critical_alerts"] = *input.NotifyCriticalAlerts
	}
	if input.NotifyAccountStatus != nil {
		updates["notify_account_status"] = *input.NotifyAccountStatus
	}
	if input.OtpEnabled != nil {
		updates["otp_enabled"] = *input.OtpEnabled
	}
	if input.Socks5Enabled != nil {
		updates["socks5_enabled"] = *input.Socks5Enabled
	}
	if input.Socks5Address != nil {
		updates["socks5_address"] = *input.Socks5Address
	}
	if input.Socks5Username != nil {
		updates["socks5_username"] = *input.Socks5Username
	}
	if input.Socks5Password != nil {
		updates["socks5_password"] = *input.Socks5Password
	}
	if input.FaoximaDomain != nil {
		updates["faoxima_domain"] = *input.FaoximaDomain
	}

	if len(updates) == 0 {
		return s.GetOrCreate()
	}

	if err := s.db.Model(&model.BotSettings{}).Where("id = ?", botSettingsSingletonID).Updates(updates).Error; err != nil {
		s.logger.Error("failed to update bot settings", zap.Error(err))
		return nil, err
	}

	return s.GetOrCreate()
}

// SetAutoBackupSchedule configures the daily automatic backup time. This is
// set via a bot command, not the web settings page, per the panel's design
// (the schedule is something the admin controls from their phone via the
// bot itself).
func (s *BotSettingsService) SetAutoBackupSchedule(enabled bool, hour, minute int) error {
	if hour < 0 || hour > 23 {
		return errors.New("ساعت باید بین ۰ تا ۲۳ باشد")
	}
	if minute < 0 || minute > 59 {
		return errors.New("دقیقه باید بین ۰ تا ۵۹ باشد")
	}

	if _, err := s.GetOrCreate(); err != nil {
		return err
	}

	return s.db.Model(&model.BotSettings{}).Where("id = ?", botSettingsSingletonID).Updates(map[string]interface{}{
		"auto_backup_enabled": enabled,
		"auto_backup_hour":    hour,
		"auto_backup_minute":  minute,
	}).Error
}

// SetAutoReportSchedule configures the daily usage-report time. Set via the
// bot's /setreport command, not the web settings page -- the report used to
// be a plain on/off toggle in the web UI with no way to pick a time at all,
// and (separately) was never actually wired into any scheduled job. Both are
// fixed by moving control here, mirroring SetAutoBackupSchedule exactly.
func (s *BotSettingsService) SetAutoReportSchedule(enabled bool, hour, minute int) error {
	if hour < 0 || hour > 23 {
		return errors.New("ساعت باید بین ۰ تا ۲۳ باشد")
	}
	if minute < 0 || minute > 59 {
		return errors.New("دقیقه باید بین ۰ تا ۵۹ باشد")
	}

	if _, err := s.GetOrCreate(); err != nil {
		return err
	}

	return s.db.Model(&model.BotSettings{}).Where("id = ?", botSettingsSingletonID).Updates(map[string]interface{}{
		"auto_report_enabled": enabled,
		"auto_report_hour":    hour,
		"auto_report_minute":  minute,
	}).Error
}

// MarkBackupSent/MarkReportSent record the UTC calendar date (YYYY-MM-DD) a
// scheduled backup/report last actually went out, so the once-a-minute
// scheduler tick (see cmd/main.go) can tell "already sent today" apart from
// "today's scheduled time just hasn't arrived yet" using only fields already
// on this row, without a separate cron library.
func (s *BotSettingsService) MarkBackupSent(date string) error {
	return s.db.Model(&model.BotSettings{}).Where("id = ?", botSettingsSingletonID).Update("last_backup_sent_date", date).Error
}

func (s *BotSettingsService) MarkReportSent(date string) error {
	return s.db.Model(&model.BotSettings{}).Where("id = ?", botSettingsSingletonID).Update("last_report_sent_date", date).Error
}

// SetLastUpdateID persists the Telegram getUpdates offset checkpoint so a
// process restart resumes polling from where it left off.
func (s *BotSettingsService) SetLastUpdateID(updateID int) error {
	return s.db.Model(&model.BotSettings{}).Where("id = ?", botSettingsSingletonID).Update("last_update_id", updateID).Error
}

// ListExtraAdminChatIDs returns every additional admin chat ID configured
// to receive notifications alongside BotSettings.AdminChatID.
func (s *BotSettingsService) ListExtraAdminChatIDs() ([]model.BotExtraAdminChatID, error) {
	var ids []model.BotExtraAdminChatID
	if err := s.db.Order("id asc").Find(&ids).Error; err != nil {
		return nil, err
	}
	return ids, nil
}

// ErrInvalidTelegramChatID is returned by AddExtraAdminChatID when chatID
// isn't a real Telegram chat id -- a confirmed, reported incident: an
// admin ran the bot's own /addadmin command by literally copy-pasting its
// usage hint's placeholder text ("CHAT_ID") instead of substituting their
// real numeric id, silently saving a row that could never receive a
// message (sendRaw's own strconv.ParseInt would simply fail and log a
// warning on every send attempt, with the admin never seeing why their
// "added" recipient never got anything). Validating up front turns that
// into an immediate, clear rejection instead of a silently-dead row.
var ErrInvalidTelegramChatID = errors.New("chat id must be a numeric telegram chat id")

// isValidTelegramChatID mirrors sendRaw's own strconv.ParseInt(chatID,
// 10, 64) parse exactly (a real Telegram chat id is always a plain
// integer -- negative for groups/channels, positive for a private chat),
// so a row that passes this check is guaranteed to be usable there.
func isValidTelegramChatID(chatID string) bool {
	chatID = strings.TrimSpace(chatID)
	if chatID == "" {
		return false
	}
	_, err := strconv.ParseInt(chatID, 10, 64)
	return err == nil
}

// AddExtraAdminChatID registers a new additional admin recipient. Returns
// gorm's unique-constraint error unchanged if chatID is already
// registered and NOT previously removed -- callers surface that as
// "already added" rather than a generic failure.
//
// chat_id carries a plain (not deleted_at-scoped) unique index -- a
// confirmed, reported-adjacent bug found while fixing "OTP doesn't reach
// the added IDs": removing a recipient only soft-deletes its row (see
// RemoveExtraAdminChatID), so re-adding the EXACT SAME chat id later
// would previously fail with a unique-constraint violation against that
// still-occupying soft-deleted row -- a confusing "already added" error
// for an id that, from the admin's point of view, had been removed and
// was no longer receiving anything. This now looks for a soft-deleted
// row with the same chat_id first and revives it (clearing deleted_at,
// applying the new label) instead of blind-inserting, mirroring
// pruneStaleTunnels's own "prior soft-delete must not block recreation"
// precedent (service/tunnel_health.go).
func (s *BotSettingsService) AddExtraAdminChatID(chatID string, label *string) (*model.BotExtraAdminChatID, error) {
	if !isValidTelegramChatID(chatID) {
		return nil, fmt.Errorf("%w: %q", ErrInvalidTelegramChatID, chatID)
	}

	var existing model.BotExtraAdminChatID
	err := s.db.Unscoped().Where("chat_id = ?", chatID).First(&existing).Error
	if err == nil {
		if existing.DeletedAt.Valid {
			existing.DeletedAt = gorm.DeletedAt{}
			existing.Label = label
			if err := s.db.Unscoped().Save(&existing).Error; err != nil {
				return nil, err
			}
			return &existing, nil
		}
		// A currently-active row already has this chat_id -- let Create
		// below hit the same unique-constraint error callers already
		// expect for "already added".
	} else if !errors.Is(err, gorm.ErrRecordNotFound) {
		return nil, err
	}

	entry := model.BotExtraAdminChatID{ChatID: chatID, Label: label}
	if err := s.db.Create(&entry).Error; err != nil {
		return nil, err
	}
	return &entry, nil
}

// RemoveExtraAdminChatID deletes one additional admin recipient by its own
// row ID.
func (s *BotSettingsService) RemoveExtraAdminChatID(id uint) error {
	return s.db.Delete(&model.BotExtraAdminChatID{}, id).Error
}

// UpdateExtraAdminChatIDLabel renames one additional admin recipient's
// human-readable label without touching its chat ID.
func (s *BotSettingsService) UpdateExtraAdminChatIDLabel(id uint, label *string) error {
	return s.db.Model(&model.BotExtraAdminChatID{}).Where("id = ?", id).Update("label", label).Error
}
