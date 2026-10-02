package service

import (
	"fmt"
	"strconv"
	"sync"

	tgbotapi "github.com/go-telegram-bot-api/telegram-bot-api/v5"
	"go.uber.org/zap"
	"gorm.io/gorm"
)

// BotService owns the live Telegram bot connection and is the single place
// both outbound notifications (from anywhere else in the backend) and the
// inbound update poller (BotPoller, in bot_poller.go) go through. It is
// re-created whenever bot settings change (token/enabled toggle), so a
// settings update takes effect without restarting the whole panel process.
type BotService struct {
	db     *gorm.DB
	logger *zap.Logger

	mu      sync.RWMutex
	api     *tgbotapi.BotAPI // nil when the bot is disabled or misconfigured
	poller  *botPoller
	handler BotUpdateHandler
}

// BotUpdateHandler processes a single incoming Telegram update. It is
// implemented by BotCommandHandler (bot_commands.go) and injected rather
// than imported directly, so this file has no dependency on the specific
// business logic behind each command/callback.
type BotUpdateHandler interface {
	HandleUpdate(bot *BotService, update tgbotapi.Update)
}

func NewBotService(db *gorm.DB) *BotService {
	return &BotService{
		db:     db,
		logger: zap.L().Named("BotService"),
	}
}

// SetHandler wires the update dispatcher. Must be called once before Reload
// is first invoked with a working bot token, otherwise incoming
// messages/callbacks are received but silently dropped.
func (b *BotService) SetHandler(handler BotUpdateHandler) {
	b.mu.Lock()
	defer b.mu.Unlock()
	b.handler = handler
}

// Reload (re)reads BotSettings from the database and starts or stops the
// live bot connection + poller to match: a valid token and Enabled=true
// starts (or restarts, if the token changed) the connection; anything else
// tears it down. Safe to call any number of times, including from the
// settings-update HTTP handler, so a token/enabled change takes effect
// immediately.
func (b *BotService) Reload() error {
	settingsService := NewBotSettingsService(b.db)
	settings, err := settingsService.GetOrCreate()
	if err != nil {
		return err
	}

	b.mu.Lock()
	defer b.mu.Unlock()

	b.stopLocked()

	if !settings.Enabled || settings.BotToken == "" {
		b.logger.Info("telegram bot is disabled or has no token configured; not starting")
		return nil
	}

	api, err := newTelegramBotAPI(settings.BotToken, settings.Socks5Enabled, settings.Socks5Address, settings.Socks5Username, settings.Socks5Password)
	if err != nil {
		b.logger.Error("failed to initialize telegram bot with configured token", zap.Error(err))
		return err
	}

	b.api = api
	b.logger.Info("telegram bot connected", zap.String("username", api.Self.UserName))

	if b.handler != nil {
		b.poller = newBotPoller(b, api, b.db, b.logger)
		b.poller.start(settings.LastUpdateID)
	} else {
		b.logger.Warn("telegram bot started with no update handler set; incoming messages will be ignored")
	}

	return nil
}

// stopLocked tears down the current connection/poller. Caller must hold mu.
func (b *BotService) stopLocked() {
	if b.poller != nil {
		b.poller.stop()
		b.poller = nil
	}
	b.api = nil
}

// Stop shuts down the bot connection and poller. Safe to call even if the
// bot was never started.
func (b *BotService) Stop() {
	b.mu.Lock()
	defer b.mu.Unlock()
	b.stopLocked()
}

// IsRunning reports whether a live bot connection is currently active.
func (b *BotService) IsRunning() bool {
	b.mu.RLock()
	defer b.mu.RUnlock()
	return b.api != nil
}

// SendMessage sends plain text to a chat ID (as a string, matching how chat
// IDs are stored on BotSettings/Reseller). A no-op, not an error, if the bot
// isn't currently running or chatID is empty -- every notification call site
// in this codebase treats "notifications not configured" as normal, not
// exceptional, matching TelegramNotifier's existing behavior.
func (b *BotService) SendMessage(chatID string, text string) error {
	return b.SendMessageWithKeyboard(chatID, text, nil)
}

// SendMessageWithKeyboard sends text with an optional inline keyboard
// attached (pass nil for a plain message).
func (b *BotService) SendMessageWithKeyboard(chatID string, text string, keyboard *tgbotapi.InlineKeyboardMarkup) error {
	var markup interface{}
	if keyboard != nil {
		markup = *keyboard
	}
	return b.sendRaw(chatID, text, markup)
}

// sendMessageWithReplyKeyboard sends text with a persistent reply (bottom)
// keyboard attached, replacing whatever reply keyboard the chat currently
// has. Unexported: only bot_commands.go's main-menu message needs this, an
// inline keyboard is the right choice for every other notification.
func (b *BotService) sendMessageWithReplyKeyboard(chatID string, text string, keyboard tgbotapi.ReplyKeyboardMarkup) error {
	return b.sendRaw(chatID, text, keyboard)
}

func (b *BotService) sendRaw(chatID string, text string, replyMarkup interface{}) error {
	b.mu.RLock()
	api := b.api
	b.mu.RUnlock()

	if api == nil || chatID == "" {
		return nil
	}

	id, err := strconv.ParseInt(chatID, 10, 64)
	if err != nil {
		return fmt.Errorf("invalid telegram chat id %q: %w", chatID, err)
	}

	msg := tgbotapi.NewMessage(id, text)
	// Every message text in this codebase is authored with Telegram's HTML
	// subset (<b>, <i>, <code>, etc.) for readable, well-formatted alerts --
	// see bot_commands.go/bot_notifications.go for the actual message copy.
	msg.ParseMode = tgbotapi.ModeHTML
	if replyMarkup != nil {
		msg.ReplyMarkup = replyMarkup
	}

	if _, err := api.Send(msg); err != nil {
		b.logger.Error("failed to send telegram message", zap.String("chat_id", chatID), zap.Error(err))
		return err
	}

	return nil
}

// SendDocument uploads a local file to a chat as a Telegram document
// attachment, with an optional caption. Used by the scheduled auto-backup
// job to deliver the database snapshot directly to the admin's chat. Same
// no-op-if-not-configured behavior as SendMessage.
func (b *BotService) SendDocument(chatID string, filePath string, caption string) error {
	b.mu.RLock()
	api := b.api
	b.mu.RUnlock()

	if api == nil || chatID == "" {
		return nil
	}

	id, err := strconv.ParseInt(chatID, 10, 64)
	if err != nil {
		return fmt.Errorf("invalid telegram chat id %q: %w", chatID, err)
	}

	doc := tgbotapi.NewDocument(id, tgbotapi.FilePath(filePath))
	if caption != "" {
		doc.Caption = caption
		doc.ParseMode = tgbotapi.ModeHTML
	}

	if _, err := api.Send(doc); err != nil {
		b.logger.Error("failed to send telegram document", zap.String("chat_id", chatID), zap.Error(err))
		return err
	}

	return nil
}

// AnswerCallback acknowledges an inline button press (Telegram requires this
// or the client shows a perpetual loading spinner on the button). text is
// shown as a small transient popup/toast in the client; pass "" for none.
func (b *BotService) AnswerCallback(callbackQueryID, text string) {
	b.mu.RLock()
	api := b.api
	b.mu.RUnlock()

	if api == nil {
		return
	}

	if _, err := api.Request(tgbotapi.NewCallback(callbackQueryID, text)); err != nil {
		b.logger.Warn("failed to answer telegram callback query", zap.Error(err))
	}
}
