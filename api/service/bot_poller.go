package service

import (
	"errors"
	"strconv"
	"time"

	tgbotapi "github.com/go-telegram-bot-api/telegram-bot-api/v5"
	"go.uber.org/zap"
	"gorm.io/gorm"

	"github.com/maahdima/mwp/api/dataservice/model"
)

func formatChatID(chatID int64) string {
	return strconv.FormatInt(chatID, 10)
}

// botPoller runs Telegram's long-polling getUpdates loop in a background
// goroutine and dispatches each accepted update to the BotService's
// registered handler. It is created fresh by BotService.Reload every time
// the bot is (re)started, and torn down via stop() when the bot is disabled
// or reconfigured.
//
// This deliberately does NOT use tgbotapi.BotAPI.GetUpdatesChan: that method
// spawns its own internal goroutine with no panic recovery of any kind, and
// a panic in an unrecovered goroutine crashes the entire Go process --
// there is no way for any other code in the same process, including
// middleware.Recover() on the HTTP server, to catch it. Calling the
// lower-level GetUpdates directly and driving the poll loop ourselves means
// every iteration runs under our own recoverPanic guard instead.
type botPoller struct {
	bot    *BotService
	api    *tgbotapi.BotAPI
	db     *gorm.DB
	logger *zap.Logger
	done   chan struct{}
}

func newBotPoller(bot *BotService, api *tgbotapi.BotAPI, db *gorm.DB, logger *zap.Logger) *botPoller {
	return &botPoller{
		bot:    bot,
		api:    api,
		db:     db,
		logger: logger.Named("BotPoller"),
		done:   make(chan struct{}),
	}
}

// start begins long-polling from lastUpdateID+1 (the checkpoint persisted in
// BotSettings.LastUpdateID from the previous run, so a process restart
// doesn't reprocess or permanently skip updates that arrived while the
// panel was down) and processes updates in its own goroutine until stop is
// called.
func (p *botPoller) start(lastUpdateID int) {
	go p.run(lastUpdateID + 1)
}

// run drives the poll loop itself (rather than ranging over
// tgbotapi.UpdatesChannel -- see the botPoller doc comment for why). Each
// iteration is wrapped in its own panic recovery, and any error from the
// Telegram API (network blip, rate limit, transient 5xx) is logged and
// backed off rather than propagated, so nothing here can ever bring down
// the process.
func (p *botPoller) run(offset int) {
	p.logger.Info("telegram bot update polling started")
	for {
		select {
		case <-p.done:
			p.logger.Info("telegram bot update polling stopped")
			return
		default:
		}

		nextOffset := p.pollOnce(offset)
		if nextOffset > offset {
			offset = nextOffset
		}
	}
}

// pollOnce performs a single getUpdates call plus dispatch, guarded by its
// own recover so a panic anywhere in this call (library bug, malformed
// response, a bug in a command handler that somehow escapes handleUpdate's
// own recover) only aborts this one iteration instead of the whole
// goroutine -- and therefore the whole process. Returns the offset the next
// call should use (unchanged if nothing was processed or a panic/error
// occurred).
func (p *botPoller) pollOnce(offset int) (nextOffset int) {
	defer func() {
		if r := recover(); r != nil {
			p.logger.Error("recovered from panic in telegram poll loop", zap.Any("panic", r))
			time.Sleep(3 * time.Second)
		}
	}()

	updateConfig := tgbotapi.NewUpdate(offset)
	updateConfig.Timeout = 30 // seconds; Telegram long-polling wait per request

	updates, err := p.api.GetUpdates(updateConfig)
	if err != nil {
		p.logger.Warn("failed to fetch telegram updates, retrying shortly", zap.Error(err))
		time.Sleep(3 * time.Second)
		return offset
	}

	for _, update := range updates {
		if update.UpdateID >= offset {
			offset = update.UpdateID + 1
		}
		p.handleUpdate(update)
	}

	return offset
}

func (p *botPoller) stop() {
	close(p.done)
}

// handleUpdate enforces the security allowlist (only the configured admin
// chat ID, or a chat ID belonging to a registered reseller, may interact
// with the bot) before dispatching to the registered handler. Every
// incoming update -- message or callback -- goes through this same check;
// there is no code path that reaches BotUpdateHandler.HandleUpdate without
// first passing it.
func (p *botPoller) handleUpdate(update tgbotapi.Update) {
	defer p.persistCheckpoint(update.UpdateID)
	defer p.recoverPanic()

	chat := update.FromChat()
	if chat == nil {
		return
	}

	role, err := p.resolveChatRole(chat.ID)
	if err != nil {
		p.logger.Error("failed to resolve telegram chat role", zap.Int64("chat_id", chat.ID), zap.Error(err))
		return
	}
	if role == chatRoleUnknown {
		p.logger.Warn("rejected telegram update from unrecognized chat id", zap.Int64("chat_id", chat.ID))
		return
	}

	if p.bot == nil {
		return
	}

	b := p.bot
	b.mu.RLock()
	handler := b.handler
	b.mu.RUnlock()

	if handler == nil {
		return
	}

	handler.HandleUpdate(b, update)
}

// recoverPanic ensures one malformed update or a bug in a command handler
// can never crash the whole polling goroutine (and therefore the bot) --
// consistent with why middleware.Recover() was added to the HTTP server:
// isolate the blast radius of an unexpected panic to the single unit of
// work that triggered it.
func (p *botPoller) recoverPanic() {
	if r := recover(); r != nil {
		p.logger.Error("recovered from panic while handling telegram update", zap.Any("panic", r))
	}
}

func (p *botPoller) persistCheckpoint(updateID int) {
	settingsService := NewBotSettingsService(p.db)
	if err := settingsService.SetLastUpdateID(updateID); err != nil {
		p.logger.Warn("failed to persist telegram update checkpoint", zap.Error(err))
	}
}

type chatRole int

const (
	chatRoleUnknown chatRole = iota
	chatRoleAdmin
	chatRoleReseller
)

// resolveChatRole checks chatID against the configured admin chat ID and
// every reseller's TelegramChatID. This is the security boundary: the bot
// must never act on behalf of, or leak data to, an unrecognized Telegram
// chat, so every command handler can assume the caller has already been
// authenticated as either "the admin" or "a specific known reseller" by the
// time it runs.
func (p *botPoller) resolveChatRole(chatID int64) (chatRole, error) {
	settingsService := NewBotSettingsService(p.db)
	settings, err := settingsService.GetOrCreate()
	if err != nil {
		return chatRoleUnknown, err
	}

	chatIDStr := formatChatID(chatID)

	if settings.AdminChatID != "" && settings.AdminChatID == chatIDStr {
		return chatRoleAdmin, nil
	}

	var reseller model.Reseller
	err = p.db.Where("telegram_chat_id = ?", chatIDStr).First(&reseller).Error
	if err == nil {
		return chatRoleReseller, nil
	}
	if !errors.Is(err, gorm.ErrRecordNotFound) {
		return chatRoleUnknown, err
	}

	return chatRoleUnknown, nil
}
