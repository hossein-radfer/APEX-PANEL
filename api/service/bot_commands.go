package service

import (
	"errors"
	"fmt"
	"strconv"
	"strings"
	"sync"

	tgbotapi "github.com/go-telegram-bot-api/telegram-bot-api/v5"
	"go.uber.org/zap"
	"gorm.io/gorm"

	"github.com/maahdima/mwp/api/dataservice/model"
	"github.com/maahdima/mwp/api/http/schema"
)

// Callback data prefixes. Telegram callback_data is a single opaque string
// (64 bytes max), so multi-part payloads are encoded as
// "prefix:arg1:arg2..." and split on ":" when handled.
const (
	callbackQuickTopUp      = "topup"        // topup:<reseller_id>
	callbackReportPayments  = "report_pay"   // report_pay
	callbackReportTraffic   = "report_traf"  // report_traf
	callbackBuyPackage      = "buy_pkg"      // buy_pkg:<traffic_package_id>
	callbackMainMenu        = "main_menu"    // main_menu
	callbackAdminResellers  = "admin_resl"   // admin_resl -- opens the reseller-management menu
	callbackResellerAdd     = "resl_add"     // resl_add -- prompts for the add-reseller message
	callbackResellerToggle  = "resl_toggle"  // resl_toggle -- prompts for a username to enable/disable
	callbackResellerSearch  = "resl_search"  // resl_search -- prompts for a username to look up
	callbackResellerEnable  = "resl_enable"  // resl_enable:<reseller_id>
	callbackResellerDisable = "resl_disable" // resl_disable:<reseller_id>
)

// pendingAdminAction tracks that the admin's NEXT plain-text message should
// be interpreted as input for a specific reseller-management action,
// rather than routed through the normal command switch in handleMessage.
// Telegram inline buttons carry no free-text payload, so any action needing
// more than a fixed set of choices (adding a reseller, searching by
// username) necessarily becomes a two-step "tap button, then type a
// message" conversation. Keyed by chat ID; process-local and intentionally
// not persisted -- losing an in-progress prompt on a restart is an
// acceptable trade-off for not needing a new DB table for ephemeral state.
type pendingAdminAction string

const (
	pendingActionNone           pendingAdminAction = ""
	pendingActionAddReseller    pendingAdminAction = "add_reseller"
	pendingActionToggleReseller pendingAdminAction = "toggle_reseller"
	pendingActionSearchReseller pendingAdminAction = "search_reseller"
)

// BotCommandHandler implements BotUpdateHandler: it's the business-logic
// layer behind every button press and text command the bot receives. It's
// intentionally the only file in this package that mixes Telegram-specific
// dispatch with panel domain logic (reseller/wallet/audit-log lookups) --
// bot.go and bot_poller.go stay generic transport plumbing.
//
// All outbound message text here is Persian, formatted with Telegram's HTML
// parse mode (BotService sends every message with ParseMode=HTML) -- see
// bot_notifications.go's doc comment for the same convention and the esc()
// escaping helper shared with this file.
type BotCommandHandler struct {
	db              *gorm.DB
	resellerService *Reseller
	walletService   *Wallet
	auditLogService *AuditLog
	settingsService *BotSettingsService
	packageService  *TrafficPackageService
	purchaseService *PackagePurchaseService
	logger          *zap.Logger

	pendingMu sync.Mutex
	pending   map[string]pendingAdminAction // keyed by chat ID
}

func NewBotCommandHandler(db *gorm.DB, resellerService *Reseller, walletService *Wallet, auditLogService *AuditLog, settingsService *BotSettingsService) *BotCommandHandler {
	return &BotCommandHandler{
		db:              db,
		resellerService: resellerService,
		walletService:   walletService,
		auditLogService: auditLogService,
		settingsService: settingsService,
		pending:         make(map[string]pendingAdminAction),
		logger:          zap.L().Named("BotCommandHandler"),
	}
}

func (h *BotCommandHandler) setPending(chatID string, action pendingAdminAction) {
	h.pendingMu.Lock()
	defer h.pendingMu.Unlock()
	if action == pendingActionNone {
		delete(h.pending, chatID)
		return
	}
	h.pending[chatID] = action
}

func (h *BotCommandHandler) takePending(chatID string) pendingAdminAction {
	h.pendingMu.Lock()
	defer h.pendingMu.Unlock()
	action, ok := h.pending[chatID]
	if !ok {
		return pendingActionNone
	}
	delete(h.pending, chatID)
	return action
}

// SetPackageServices wires the traffic-package catalog and purchase flow
// after construction (both depend on the wallet service, which is
// constructed alongside this handler, so a two-step wiring keeps
// main.go/http-server.go's construction order simple). Safe to leave unset
// -- the /packages command and buy_pkg callback report "not available" if
// nil rather than panicking.
func (h *BotCommandHandler) SetPackageServices(packageService *TrafficPackageService, purchaseService *PackagePurchaseService) {
	h.packageService = packageService
	h.purchaseService = purchaseService
}

// resellerForChat looks up which reseller (if any) owns telegramChatID --
// the same lookup botPoller.resolveChatRole uses to admit the update in the
// first place, re-run here because HandleUpdate isn't told which specific
// reseller an already-admitted update came from, only that it was admitted.
func (h *BotCommandHandler) resellerForChat(telegramChatID string) *model.Reseller {
	var reseller model.Reseller
	if err := h.db.Where("telegram_chat_id = ?", telegramChatID).First(&reseller).Error; err != nil {
		return nil
	}
	return &reseller
}

// isAdminChat reports whether telegramChatID is specifically the configured
// AdminChatID -- botPoller.handleUpdate's allowlist only proves "this chat
// is either the admin or some registered reseller", which is not the same
// thing as "this chat is the admin". Every command/report that spans
// cross-reseller data (payments/traffic reports, backup scheduling) must
// use this, not just rely on having been admitted past the poller.
func (h *BotCommandHandler) isAdminChat(telegramChatID string) bool {
	settings, err := h.settingsService.GetOrCreate()
	if err != nil {
		return false
	}
	return settings.AdminChatID != "" && settings.AdminChatID == telegramChatID
}

func (h *BotCommandHandler) HandleUpdate(bot *BotService, update tgbotapi.Update) {
	if update.CallbackQuery != nil {
		h.handleCallback(bot, update.CallbackQuery)
		return
	}
	if update.Message != nil {
		h.handleMessage(bot, update.Message)
		return
	}
}

func (h *BotCommandHandler) handleMessage(bot *BotService, msg *tgbotapi.Message) {
	chatID := formatChatID(msg.Chat.ID)
	text := strings.TrimSpace(msg.Text)

	// A pending admin action (set by tapping "➕ افزودن نماینده", "فعال/غیرفعال
	// نماینده" or "جستجوی نماینده" in the reseller-management menu) takes the
	// caller's very next message as that action's free-text input, bypassing
	// the normal command switch below entirely -- except /start /menu, which
	// always cancels back out to the main menu regardless of a pending
	// prompt, so a stuck/forgotten conversation is never a dead end.
	if text != "/start" && text != "/menu" {
		if action := h.takePending(chatID); action != pendingActionNone {
			h.handlePendingAdminInput(bot, chatID, action, text)
			return
		}
	}

	switch {
	case text == "/start" || text == "/menu":
		h.setPending(chatID, pendingActionNone)
		h.sendMainMenu(bot, chatID)

	case text == "\U0001F4B0 گزارش پرداخت‌ها" || text == "/payments":
		h.sendPaymentsReport(bot, chatID)

	case text == "\U0001F4CA گزارش ترافیک" || text == "/traffic":
		h.sendTrafficReport(bot, chatID)

	case strings.HasPrefix(text, "/setbackup "):
		h.handleSetBackupCommand(bot, chatID, strings.TrimPrefix(text, "/setbackup "))

	case strings.HasPrefix(text, "/setreport "):
		h.handleSetReportCommand(bot, chatID, strings.TrimPrefix(text, "/setreport "))

	case strings.HasPrefix(text, "/addadmin "):
		h.handleAddAdminCommand(bot, chatID, strings.TrimPrefix(text, "/addadmin "))

	case strings.HasPrefix(text, "/removeadmin "):
		h.handleRemoveAdminCommand(bot, chatID, strings.TrimPrefix(text, "/removeadmin "))

	case text == "/listadmins":
		h.handleListAdminsCommand(bot, chatID)

	case text == "\U0001F4E6 خرید بسته ترافیک" || text == "/packages":
		h.sendPackagesList(bot, chatID)

	case strings.HasPrefix(text, "/buy "):
		h.handleBuyPackageCommand(bot, chatID, strings.TrimPrefix(text, "/buy "))

	case text == "\U0001F465 مدیریت نمایندگان" || text == "/resellers":
		h.sendResellerManagementMenu(bot, chatID)

	default:
		h.sendMainMenu(bot, chatID)
	}
}

func (h *BotCommandHandler) handleCallback(bot *BotService, cb *tgbotapi.CallbackQuery) {
	data := cb.Data
	chatID := formatChatID(cb.Message.Chat.ID)

	parts := strings.Split(data, ":")
	action := parts[0]

	switch action {
	case callbackQuickTopUp:
		h.handleQuickTopUpCallback(bot, cb, parts)
	case callbackReportPayments:
		bot.AnswerCallback(cb.ID, "")
		h.sendPaymentsReport(bot, chatID)
	case callbackReportTraffic:
		bot.AnswerCallback(cb.ID, "")
		h.sendTrafficReport(bot, chatID)
	case callbackBuyPackage:
		h.handleBuyPackageCallback(bot, cb, parts)
	case callbackMainMenu:
		bot.AnswerCallback(cb.ID, "")
		h.setPending(chatID, pendingActionNone)
		h.sendMainMenu(bot, chatID)
	case callbackAdminResellers:
		bot.AnswerCallback(cb.ID, "")
		h.sendResellerManagementMenu(bot, chatID)
	case callbackResellerAdd:
		bot.AnswerCallback(cb.ID, "")
		h.promptAddReseller(bot, chatID)
	case callbackResellerToggle:
		bot.AnswerCallback(cb.ID, "")
		h.promptToggleReseller(bot, chatID)
	case callbackResellerSearch:
		bot.AnswerCallback(cb.ID, "")
		h.promptSearchReseller(bot, chatID)
	case callbackResellerEnable:
		h.handleResellerSetActiveCallback(bot, cb, parts, true)
	case callbackResellerDisable:
		h.handleResellerSetActiveCallback(bot, cb, parts, false)
	default:
		bot.AnswerCallback(cb.ID, "دستور نامشخص")
	}
}

// sendMainMenu presents a role-appropriate reply keyboard (persistent
// bottom buttons, the "دکمه‌ای" half of the bot's UI) together with an
// inline keyboard on the welcome message itself (the "شیشه‌ای" half) so
// every report/action is reachable with a single tap from the very first
// message, not just from the bottom keyboard. The admin sees cross-reseller
// reports; a reseller only sees their own package-purchase entry point,
// matching the access each role's underlying commands actually allow.
func (h *BotCommandHandler) sendMainMenu(bot *BotService, chatID string) {
	if h.isAdminChat(chatID) {
		replyKeyboard := tgbotapi.NewReplyKeyboard(
			tgbotapi.NewKeyboardButtonRow(
				tgbotapi.NewKeyboardButton("\U0001F4B0 گزارش پرداخت‌ها"),
				tgbotapi.NewKeyboardButton("\U0001F4CA گزارش ترافیک"),
			),
			tgbotapi.NewKeyboardButtonRow(
				tgbotapi.NewKeyboardButton("\U0001F465 مدیریت نمایندگان"),
			),
		)
		text := "🤖 <b>به ربات مدیریت ApexPanel خوش آمدید</b>\n\n" +
			"از دکمه‌های زیر برای مشاهده گزارش‌ها و مدیریت نمایندگان استفاده کنید.\n\n" +
			"⏰ برای تنظیم ساعت بک‌آپ خودکار:\n<code>/setbackup HH:MM</code>\n\n" +
			"📅 برای تنظیم ساعت گزارش روزانه:\n<code>/setreport HH:MM</code>\n\n" +
			"مثال: <code>/setbackup 03:00</code>\n\n" +
			"➕ برای افزودن گیرنده‌ی مدیریتی دیگر:\n<code>/addadmin CHAT_ID [برچسب]</code>\n" +
			"➖ برای حذف: <code>/removeadmin ID</code>\n" +
			"📋 برای مشاهده‌ی فهرست: <code>/listadmins</code>"
		if err := bot.sendMessageWithReplyKeyboard(chatID, text, replyKeyboard); err != nil {
			h.logger.Error("failed to send admin main menu", zap.Error(err))
		}

		inlineKeyboard := tgbotapi.NewInlineKeyboardMarkup(
			tgbotapi.NewInlineKeyboardRow(
				tgbotapi.NewInlineKeyboardButtonData("💰 گزارش پرداخت‌ها", callbackReportPayments),
				tgbotapi.NewInlineKeyboardButtonData("📊 گزارش ترافیک", callbackReportTraffic),
			),
			tgbotapi.NewInlineKeyboardRow(
				tgbotapi.NewInlineKeyboardButtonData("👥 مدیریت نمایندگان", callbackAdminResellers),
			),
		)
		if err := bot.SendMessageWithKeyboard(chatID, "⚡️ دسترسی سریع:", &inlineKeyboard); err != nil {
			h.logger.Error("failed to send admin quick-access keyboard", zap.Error(err))
		}
		return
	}

	replyKeyboard := tgbotapi.NewReplyKeyboard(
		tgbotapi.NewKeyboardButtonRow(
			tgbotapi.NewKeyboardButton("\U0001F4E6 خرید بسته ترافیک"),
		),
	)
	text := "🤖 <b>به ربات ApexPanel خوش آمدید</b>\n\n📦 برای خرید بسته ترافیک مازاد از دکمه زیر استفاده کنید."
	if err := bot.sendMessageWithReplyKeyboard(chatID, text, replyKeyboard); err != nil {
		h.logger.Error("failed to send reseller main menu", zap.Error(err))
	}
}

// sendPaymentsReport reports every reseller's current wallet balance -- an
// on-demand pull, not a toggleable push notification, so it isn't gated by
// any BotSettings.Notify* flag. Admin-only: botPoller's allowlist admits
// both the admin chat and any registered reseller's chat, but this report
// spans ALL resellers' financial data, so it must additionally check the
// caller is specifically the configured admin chat, not just "some
// recognized chat" -- otherwise a reseller could read every other
// reseller's wallet balance.
func (h *BotCommandHandler) sendPaymentsReport(bot *BotService, chatID string) {
	if !h.isAdminChat(chatID) {
		_ = bot.SendMessage(chatID, "⛔ این گزارش فقط برای مدیر سیستم قابل مشاهده است.")
		return
	}

	resellers, err := h.resellerService.ListResellers()
	if err != nil {
		h.logger.Error("failed to list resellers for payments report", zap.Error(err))
		_ = bot.SendMessage(chatID, "❌ خطا در دریافت گزارش پرداخت‌ها.")
		return
	}

	if len(resellers) == 0 {
		_ = bot.SendMessage(chatID, "📭 هیچ نماینده‌ای ثبت نشده است.")
		return
	}

	var sb strings.Builder
	sb.WriteString("💰 <b>گزارش پرداخت‌ها (موجودی کیف پول)</b>\n\n")
	for _, r := range resellers {
		balance, err := h.walletService.GetWalletBalance(r.ID)
		if err != nil {
			continue
		}
		icon := "🟢"
		if balance <= 0 {
			icon = "🔴"
		}
		sb.WriteString(fmt.Sprintf("%s <b>%s</b>: %s تومان\n", icon, esc(r.Name), formatToman(balance)))
	}

	keyboard := tgbotapi.NewInlineKeyboardMarkup(
		tgbotapi.NewInlineKeyboardRow(
			tgbotapi.NewInlineKeyboardButtonData("🔄 بروزرسانی", callbackReportPayments),
		),
	)
	if err := bot.SendMessageWithKeyboard(chatID, sb.String(), &keyboard); err != nil {
		h.logger.Error("failed to send payments report", zap.Error(err))
	}
}

// sendTrafficReport reports every reseller's current quota usage.
// Admin-only, for the same reason sendPaymentsReport is: this spans all
// resellers, not just the caller's own data.
func (h *BotCommandHandler) sendTrafficReport(bot *BotService, chatID string) {
	if !h.isAdminChat(chatID) {
		_ = bot.SendMessage(chatID, "⛔ این گزارش فقط برای مدیر سیستم قابل مشاهده است.")
		return
	}

	resellers, err := h.resellerService.ListResellers()
	if err != nil {
		h.logger.Error("failed to list resellers for traffic report", zap.Error(err))
		_ = bot.SendMessage(chatID, "❌ خطا در دریافت گزارش ترافیک.")
		return
	}

	if len(resellers) == 0 {
		_ = bot.SendMessage(chatID, "📭 هیچ نماینده‌ای ثبت نشده است.")
		return
	}

	const bytesPerGB = 1024 * 1024 * 1024

	var sb strings.Builder
	sb.WriteString("📊 <b>گزارش مصرف ترافیک نمایندگان</b>\n\n")
	for _, r := range resellers {
		usedGB := float64(r.UsedBytes) / bytesPerGB
		if r.QuotaBytes == nil {
			sb.WriteString(fmt.Sprintf("🟢 <b>%s</b>: %.2f گیگابایت (نامحدود)\n", esc(r.Name), usedGB))
			continue
		}

		quotaGB := float64(*r.QuotaBytes) / bytesPerGB
		percent := 0.0
		if quotaGB > 0 {
			percent = usedGB / quotaGB * 100
		}
		icon := "🟢"
		if percent >= 90 {
			icon = "🔴"
		} else if percent >= 70 {
			icon = "🟡"
		}
		sb.WriteString(fmt.Sprintf("%s <b>%s</b>: %.2f / %.2f گیگابایت (%%%.0f)\n", icon, esc(r.Name), usedGB, quotaGB, percent))
	}

	keyboard := tgbotapi.NewInlineKeyboardMarkup(
		tgbotapi.NewInlineKeyboardRow(
			tgbotapi.NewInlineKeyboardButtonData("🔄 بروزرسانی", callbackReportTraffic),
		),
	)
	if err := bot.SendMessageWithKeyboard(chatID, sb.String(), &keyboard); err != nil {
		h.logger.Error("failed to send traffic report", zap.Error(err))
	}
}

// sendPackagesList shows every purchasable traffic package to a reseller,
// each with its own inline "خرید" button -- the same catalog
// (TrafficPackageService) and purchase flow (PackagePurchaseService) the web
// panel's Traffic Packages page uses, so pricing/availability/purchase
// history are always identical in both places by construction (one shared
// database table, not a separate copy synced between them).
func (h *BotCommandHandler) sendPackagesList(bot *BotService, chatID string) {
	if h.packageService == nil {
		_ = bot.SendMessage(chatID, "⛔ در حال حاضر امکان خرید بسته ترافیک وجود ندارد.")
		return
	}

	reseller := h.resellerForChat(chatID)
	if reseller == nil {
		_ = bot.SendMessage(chatID, "⛔ فقط نمایندگان ثبت‌شده می‌توانند بسته ترافیک خریداری کنند.")
		return
	}

	packages, err := h.packageService.ListActiveTrafficPackages()
	if err != nil {
		h.logger.Error("failed to list traffic packages for bot", zap.Error(err))
		_ = bot.SendMessage(chatID, "❌ خطا در دریافت لیست بسته‌های ترافیک.")
		return
	}
	if len(packages) == 0 {
		_ = bot.SendMessage(chatID, "📭 در حال حاضر بسته ترافیکی برای خرید موجود نیست.")
		return
	}

	const bytesPerGB = 1024 * 1024 * 1024
	var rows [][]tgbotapi.InlineKeyboardButton
	var sb strings.Builder
	sb.WriteString("📦 <b>بسته‌های ترافیک قابل خرید</b>\n\n")
	for _, pkg := range packages {
		gb := float64(pkg.TrafficBytes) / bytesPerGB
		price := formatToman(pkg.PriceAmount)
		sb.WriteString(fmt.Sprintf("🔹 <b>%s</b>: %.2f گیگابایت به قیمت %s تومان\n", esc(pkg.Name), gb, price))
		rows = append(rows, tgbotapi.NewInlineKeyboardRow(
			tgbotapi.NewInlineKeyboardButtonData(
				fmt.Sprintf("🛒 خرید %s (%s تومان)", pkg.Name, price),
				fmt.Sprintf("%s:%d", callbackBuyPackage, pkg.ID),
			),
		))
	}

	keyboard := tgbotapi.NewInlineKeyboardMarkup(rows...)
	if err := bot.SendMessageWithKeyboard(chatID, sb.String(), &keyboard); err != nil {
		h.logger.Error("failed to send packages list", zap.Error(err))
	}
}

// handleBuyPackageCallback lets a reseller purchase a traffic package
// directly from the bot -- the exact same PackagePurchaseService.
// PurchasePackage call the web panel's "Buy Now" button makes, so a
// purchase made here shows up identically in both the web panel's purchase
// history and the bot's own confirmation message.
func (h *BotCommandHandler) handleBuyPackageCallback(bot *BotService, cb *tgbotapi.CallbackQuery, parts []string) {
	if len(parts) < 2 {
		bot.AnswerCallback(cb.ID, "درخواست نامعتبر")
		return
	}

	packageID, err := strconv.ParseUint(parts[1], 10, 32)
	if err != nil {
		bot.AnswerCallback(cb.ID, "بسته نامعتبر")
		return
	}

	chatID := formatChatID(cb.Message.Chat.ID)
	message, err := h.purchasePackageForChat(chatID, uint(packageID))
	if err != nil {
		bot.AnswerCallback(cb.ID, "❌ خرید ناموفق بود: "+err.Error())
		return
	}

	bot.AnswerCallback(cb.ID, "✅ خرید با موفقیت انجام شد!")
	_ = bot.SendMessage(chatID, message)
}

// purchasePackageForChat is the shared purchase/renewal logic behind both
// the "buy_pkg:<id>" inline-button callback (handleBuyPackageCallback
// above) and the "/buy <id>" text command (handleMessage's switch) -- a
// confirmed, reported gap: renewal was previously reachable ONLY via the
// graphical button, with no typed-command equivalent, which matters for
// any reseller scripting their own renewal or using a Telegram client
// where inline buttons are inconvenient. Returns the ready-to-send
// success message on success so both callers format their own
// acknowledgment (an answered callback vs. a plain reply) around it.
func (h *BotCommandHandler) purchasePackageForChat(chatID string, packageID uint) (string, error) {
	if h.purchaseService == nil {
		return "", fmt.Errorf("خرید در حال حاضر امکان‌پذیر نیست")
	}

	reseller := h.resellerForChat(chatID)
	if reseller == nil {
		return "", fmt.Errorf("فقط نمایندگان ثبت‌شده می‌توانند خرید کنند")
	}

	purchase, err := h.purchaseService.PurchasePackage(reseller.ID, packageID)
	if err != nil {
		return "", err
	}

	const bytesPerGB = 1024 * 1024 * 1024
	return fmt.Sprintf(
		"🧾 <b>خرید موفق</b>\n\n📦 بسته: %s\n➕ حجم اضافه‌شده: %.2f گیگابایت\n\n✅ به حجم شما اضافه شد.",
		esc(purchase.TrafficPackageName), float64(purchase.TrafficBytes)/bytesPerGB,
	), nil
}

// handleBuyPackageCommand is the "/buy <package_id>" text-command
// equivalent of the buy_pkg inline button -- see purchasePackageForChat's
// own doc comment for the bug this closes.
func (h *BotCommandHandler) handleBuyPackageCommand(bot *BotService, chatID string, arg string) {
	packageID, err := strconv.ParseUint(strings.TrimSpace(arg), 10, 32)
	if err != nil {
		_ = bot.SendMessage(chatID, "شناسه بسته نامعتبر است. مثال: /buy 3")
		return
	}

	message, err := h.purchasePackageForChat(chatID, uint(packageID))
	if err != nil {
		_ = bot.SendMessage(chatID, "❌ خرید ناموفق بود: "+err.Error())
		return
	}

	_ = bot.SendMessage(chatID, message)
}

// handleQuickTopUpCallback implements the "شارژ سریع" scenario: the admin
// taps the inline button attached to a quota-warning alert, and this credits
// a fixed top-up amount to that specific reseller's wallet without the
// admin ever needing to open the web panel.
func (h *BotCommandHandler) handleQuickTopUpCallback(bot *BotService, cb *tgbotapi.CallbackQuery, parts []string) {
	chatID := formatChatID(cb.Message.Chat.ID)
	if !h.isAdminChat(chatID) {
		bot.AnswerCallback(cb.ID, "فقط مدیر سیستم می‌تواند این کار را انجام دهد")
		return
	}

	if len(parts) < 2 {
		bot.AnswerCallback(cb.ID, "درخواست نامعتبر")
		return
	}

	resellerID, err := strconv.ParseUint(parts[1], 10, 32)
	if err != nil {
		bot.AnswerCallback(cb.ID, "نماینده نامعتبر")
		return
	}

	// A fixed top-up amount keeps this a single tap with no further input
	// required from the admin (Telegram callback buttons carry no free-text
	// payload) -- consistent with the spec's "بدون نیاز به ورود به وب‌سایت"
	// requirement. Larger/custom amounts still go through the web panel.
	const quickTopUpAmount = 50000 // whole Toman

	reseller, err := h.resellerService.GetReseller(uint(resellerID))
	if err != nil {
		bot.AnswerCallback(cb.ID, "نماینده یافت نشد")
		return
	}

	refType := "RECHARGE"
	if _, err := h.walletService.Credit(uint(resellerID), quickTopUpAmount, "شارژ سریع از طریق ربات تلگرام", &refType, nil); err != nil {
		h.logger.Error("quick top-up failed", zap.Uint64("reseller_id", resellerID), zap.Error(err))
		bot.AnswerCallback(cb.ID, "❌ شارژ ناموفق بود")
		return
	}

	bot.AnswerCallback(cb.ID, fmt.Sprintf("✅ مبلغ %s تومان به %s اضافه شد", formatToman(quickTopUpAmount), reseller.Name))
	_ = bot.SendMessage(chatID, fmt.Sprintf(
		"💳 <b>شارژ سریع انجام شد</b>\n\n👤 نماینده: <b>%s</b>\n➕ مبلغ: %s تومان",
		esc(reseller.Name), formatToman(quickTopUpAmount),
	))
}

// sendResellerManagementMenu presents the admin's glass/inline menu for
// managing resellers directly from the bot: add, disable, enable, search.
// Admin-only -- mirrors isAdminChat's guard used by every other
// cross-reseller command in this file.
func (h *BotCommandHandler) sendResellerManagementMenu(bot *BotService, chatID string) {
	if !h.isAdminChat(chatID) {
		_ = bot.SendMessage(chatID, "⛔ این بخش فقط برای مدیر سیستم قابل استفاده است.")
		return
	}

	keyboard := tgbotapi.NewInlineKeyboardMarkup(
		tgbotapi.NewInlineKeyboardRow(
			tgbotapi.NewInlineKeyboardButtonData("➕ افزودن نماینده جدید", callbackResellerAdd),
		),
		tgbotapi.NewInlineKeyboardRow(
			tgbotapi.NewInlineKeyboardButtonData("🚫 غیرفعال‌سازی نماینده", callbackResellerToggle),
			tgbotapi.NewInlineKeyboardButtonData("✅ فعال‌سازی نماینده", callbackResellerToggle),
		),
		tgbotapi.NewInlineKeyboardRow(
			tgbotapi.NewInlineKeyboardButtonData("🔍 جستجوی نماینده", callbackResellerSearch),
		),
		tgbotapi.NewInlineKeyboardRow(
			tgbotapi.NewInlineKeyboardButtonData("↩️ بازگشت به منو", callbackMainMenu),
		),
	)
	if err := bot.SendMessageWithKeyboard(chatID, "👥 <b>مدیریت نمایندگان</b>\n\nیک گزینه را انتخاب کنید:", &keyboard); err != nil {
		h.logger.Error("failed to send reseller management menu", zap.Error(err))
	}
}

// promptAddReseller asks the admin to type the new reseller's details as a
// single line, then arms pendingActionAddReseller so the next plain-text
// message from this chat is parsed as that input (see handlePendingAdminInput).
func (h *BotCommandHandler) promptAddReseller(bot *BotService, chatID string) {
	if !h.isAdminChat(chatID) {
		return
	}
	h.setPending(chatID, pendingActionAddReseller)
	_ = bot.SendMessage(chatID, "➕ <b>افزودن نماینده جدید</b>\n\n"+
		"اطلاعات نماینده را در یک پیام، به این شکل ارسال کنید:\n"+
		"<code>name=نام‌نمایش‌داده‌شده username=نام‌کاربری pass=رمزعبور</code>\n\n"+
		"مثال:\n<code>name=Ali Reseller username=ali123 pass=Xy9!secret</code>\n\n"+
		"برای انصراف /start را ارسال کنید.")
}

// promptToggleReseller asks the admin for a username to enable/disable --
// the actual enable/disable choice is then presented as two inline buttons
// on the found reseller's card (handlePendingAdminInput ->
// handleToggleResellerInput), not asked as free text.
func (h *BotCommandHandler) promptToggleReseller(bot *BotService, chatID string) {
	if !h.isAdminChat(chatID) {
		return
	}
	h.setPending(chatID, pendingActionToggleReseller)
	_ = bot.SendMessage(chatID, "🔁 <b>فعال/غیرفعال‌سازی نماینده</b>\n\nنام کاربری نماینده مورد نظر را ارسال کنید.\n\nبرای انصراف /start را ارسال کنید.")
}

// promptSearchReseller asks the admin for a username to look up detailed info.
func (h *BotCommandHandler) promptSearchReseller(bot *BotService, chatID string) {
	if !h.isAdminChat(chatID) {
		return
	}
	h.setPending(chatID, pendingActionSearchReseller)
	_ = bot.SendMessage(chatID, "🔍 <b>جستجوی نماینده</b>\n\nنام کاربری نماینده مورد نظر را ارسال کنید.\n\nبرای انصراف /start را ارسال کنید.")
}

// handlePendingAdminInput routes the admin's free-text reply to whichever
// reseller-management prompt is currently pending for this chat.
func (h *BotCommandHandler) handlePendingAdminInput(bot *BotService, chatID string, action pendingAdminAction, text string) {
	if !h.isAdminChat(chatID) {
		return
	}

	switch action {
	case pendingActionAddReseller:
		h.handleAddResellerInput(bot, chatID, text)
	case pendingActionToggleReseller:
		h.handleToggleResellerInput(bot, chatID, text)
	case pendingActionSearchReseller:
		h.handleSearchResellerInput(bot, chatID, text)
	}
}

// parseFieldedInput parses a "key=value key2=value with spaces" line into a
// map, splitting only on "key=" boundaries so values themselves may contain
// spaces (a display name like "Ali Reseller" must survive intact) -- a
// simple hand-rolled parser is enough here since the field set is small and
// fixed (name/username/pass), not general-purpose shell-style quoting.
func parseFieldedInput(text string, keys ...string) map[string]string {
	result := make(map[string]string, len(keys))
	remaining := text
	for {
		bestKey := ""
		bestIdx := -1
		for _, k := range keys {
			marker := k + "="
			idx := strings.Index(remaining, marker)
			if idx == -1 {
				continue
			}
			if bestIdx == -1 || idx < bestIdx {
				bestIdx = idx
				bestKey = k
			}
		}
		if bestIdx == -1 {
			break
		}

		valueStart := bestIdx + len(bestKey) + 1
		valueEnd := len(remaining)
		for _, k := range keys {
			marker := " " + k + "="
			if idx := strings.Index(remaining[valueStart:], marker); idx != -1 && valueStart+idx < valueEnd {
				valueEnd = valueStart + idx
			}
		}

		result[bestKey] = strings.TrimSpace(remaining[valueStart:valueEnd])
		remaining = remaining[:bestIdx] + remaining[valueEnd:]
	}
	return result
}

func (h *BotCommandHandler) handleAddResellerInput(bot *BotService, chatID, text string) {
	fields := parseFieldedInput(text, "name", "username", "pass")
	name := fields["name"]
	username := fields["username"]
	password := fields["pass"]

	if name == "" || username == "" || password == "" {
		_ = bot.SendMessage(chatID, "⚠️ فرمت نامعتبر است. لطفاً به این شکل ارسال کنید:\n"+
			"<code>name=نام username=نام‌کاربری pass=رمزعبور</code>")
		h.setPending(chatID, pendingActionAddReseller)
		return
	}
	if len(password) < 8 {
		_ = bot.SendMessage(chatID, "⚠️ رمز عبور باید حداقل ۸ کاراکتر باشد.")
		h.setPending(chatID, pendingActionAddReseller)
		return
	}

	reseller, err := h.resellerService.CreateReseller(&schema.CreateResellerRequest{
		Name:     name,
		Username: username,
		Password: password,
	})
	if err != nil {
		_ = bot.SendMessage(chatID, "❌ افزودن نماینده ناموفق بود: "+esc(err.Error()))
		return
	}

	_ = bot.SendMessage(chatID, fmt.Sprintf(
		"✅ <b>نماینده جدید ایجاد شد</b>\n\n👤 نام: <b>%s</b>\n🔑 نام کاربری: <code>%s</code>\n🆔 شناسه: <code>%d</code>",
		esc(reseller.Name), esc(reseller.Username), reseller.ID,
	))
}

func (h *BotCommandHandler) findResellerByUsername(username string) *model.Reseller {
	var reseller model.Reseller
	if err := h.db.Where("username = ?", strings.TrimSpace(username)).First(&reseller).Error; err != nil {
		return nil
	}
	return &reseller
}

func (h *BotCommandHandler) handleToggleResellerInput(bot *BotService, chatID, username string) {
	reseller := h.findResellerByUsername(username)
	if reseller == nil {
		_ = bot.SendMessage(chatID, "❌ نماینده‌ای با این نام کاربری یافت نشد.")
		return
	}

	statusText := "✅ فعال"
	if !reseller.IsActive {
		statusText = "⛔ غیرفعال"
	}

	keyboard := tgbotapi.NewInlineKeyboardMarkup(
		tgbotapi.NewInlineKeyboardRow(
			tgbotapi.NewInlineKeyboardButtonData("✅ فعال‌سازی", fmt.Sprintf("%s:%d", callbackResellerEnable, reseller.ID)),
			tgbotapi.NewInlineKeyboardButtonData("🚫 غیرفعال‌سازی", fmt.Sprintf("%s:%d", callbackResellerDisable, reseller.ID)),
		),
	)
	text := fmt.Sprintf(
		"👤 <b>%s</b>\n🔑 نام کاربری: <code>%s</code>\n📌 وضعیت فعلی: %s\n\nعملیات مورد نظر را انتخاب کنید:",
		esc(reseller.Name), esc(reseller.Username), statusText,
	)
	if err := bot.SendMessageWithKeyboard(chatID, text, &keyboard); err != nil {
		h.logger.Error("failed to send reseller toggle prompt", zap.Error(err))
	}
}

func (h *BotCommandHandler) handleResellerSetActiveCallback(bot *BotService, cb *tgbotapi.CallbackQuery, parts []string, isActive bool) {
	chatID := formatChatID(cb.Message.Chat.ID)
	if !h.isAdminChat(chatID) {
		bot.AnswerCallback(cb.ID, "فقط مدیر سیستم می‌تواند این کار را انجام دهد")
		return
	}
	if len(parts) < 2 {
		bot.AnswerCallback(cb.ID, "درخواست نامعتبر")
		return
	}
	resellerID, err := strconv.ParseUint(parts[1], 10, 32)
	if err != nil {
		bot.AnswerCallback(cb.ID, "نماینده نامعتبر")
		return
	}

	// UpdateReseller (not a raw DB write) so the existing
	// NotifyAccountStatusChange notification to the reseller's own chat
	// fires exactly as it does when an admin toggles status from the web
	// panel -- one code path, one notification behavior, regardless of
	// which surface triggered it.
	active := isActive
	reseller, err := h.resellerService.UpdateReseller(uint(resellerID), &schema.UpdateResellerRequest{IsActive: &active})
	if err != nil {
		bot.AnswerCallback(cb.ID, "❌ عملیات ناموفق بود")
		return
	}

	verb := "فعال"
	if !isActive {
		verb = "غیرفعال"
	}
	bot.AnswerCallback(cb.ID, fmt.Sprintf("✅ نماینده %s شد", verb))
	_ = bot.SendMessage(chatID, fmt.Sprintf("✅ نماینده <b>%s</b> با موفقیت %s شد.", esc(reseller.Name), verb))
}

func (h *BotCommandHandler) handleSearchResellerInput(bot *BotService, chatID, username string) {
	reseller := h.findResellerByUsername(username)
	if reseller == nil {
		_ = bot.SendMessage(chatID, "❌ نماینده‌ای با این نام کاربری یافت نشد.")
		return
	}

	balance, err := h.walletService.GetWalletBalance(reseller.ID)
	if err != nil {
		balance = 0
	}
	peerCount, _ := h.resellerService.CountPeers(reseller.ID)
	umCount, _ := h.resellerService.CountUserManagerAccounts(reseller.ID)

	statusText := "✅ فعال"
	if !reseller.IsActive {
		statusText = "⛔ غیرفعال"
	}

	quotaStr := "نامحدود"
	if reseller.QuotaBytes != nil {
		quotaStr = gbString(*reseller.QuotaBytes)
	}

	var sb strings.Builder
	sb.WriteString(fmt.Sprintf("🔍 <b>%s</b>\n\n", esc(reseller.Name)))
	sb.WriteString(fmt.Sprintf("🔑 نام کاربری: <code>%s</code>\n", esc(reseller.Username)))
	sb.WriteString(fmt.Sprintf("📌 وضعیت: %s\n", statusText))
	sb.WriteString(fmt.Sprintf("💰 موجودی کیف پول: %s تومان\n", formatToman(balance)))
	sb.WriteString(fmt.Sprintf("🔌 وایرگارد: %d اکانت — %s از %s\n", peerCount, gbString(reseller.UsedBytes), quotaStr))
	if umCount > 0 {
		umQuotaStr := "نامحدود"
		if reseller.UserManagerQuotaBytes != nil {
			umQuotaStr = gbString(*reseller.UserManagerQuotaBytes)
		}
		sb.WriteString(fmt.Sprintf("🌐 User Manager: %d اکانت — %s از %s\n", umCount, gbString(reseller.UserManagerUsedBytes), umQuotaStr))
	}
	if reseller.TelegramChatID != nil {
		sb.WriteString("📱 تلگرام: متصل\n")
	}

	_ = bot.SendMessage(chatID, sb.String())
}

func (h *BotCommandHandler) handleSetBackupCommand(bot *BotService, chatID, arg string) {
	if !h.isAdminChat(chatID) {
		_ = bot.SendMessage(chatID, "⛔ فقط مدیر سیستم می‌تواند زمان بک‌آپ را تنظیم کند.")
		return
	}

	parts := strings.Split(strings.TrimSpace(arg), ":")
	if len(parts) != 2 {
		_ = bot.SendMessage(chatID, "⚠️ فرمت صحیح:\n<code>/setbackup HH:MM</code>\n(ساعت ۲۴ ساعته، به وقت UTC)")
		return
	}

	hour, err1 := strconv.Atoi(parts[0])
	minute, err2 := strconv.Atoi(parts[1])
	if err1 != nil || err2 != nil {
		_ = bot.SendMessage(chatID, "⚠️ فرمت صحیح:\n<code>/setbackup HH:MM</code>\n(ساعت ۲۴ ساعته، به وقت UTC)")
		return
	}

	if err := h.settingsService.SetAutoBackupSchedule(true, hour, minute); err != nil {
		_ = bot.SendMessage(chatID, "❌ خطا در تنظیم زمان بک‌آپ: "+err.Error())
		return
	}

	_ = bot.SendMessage(chatID, fmt.Sprintf(
		"✅ <b>بک‌آپ خودکار تنظیم شد</b>\n\n⏰ هر روز ساعت <code>%02d:%02d</code> (UTC) یک نسخه پشتیبان از دیتابیس برای شما ارسال می‌شود.",
		hour, minute,
	))
}

// handleSetReportCommand configures the daily all-resellers usage-report
// time, mirroring handleSetBackupCommand exactly. This is the fix for the
// report schedule previously having no time control anywhere (the web panel
// only had an on/off toggle) and never actually being wired to a scheduled
// job at all.
func (h *BotCommandHandler) handleSetReportCommand(bot *BotService, chatID, arg string) {
	if !h.isAdminChat(chatID) {
		_ = bot.SendMessage(chatID, "⛔ فقط مدیر سیستم می‌تواند زمان گزارش روزانه را تنظیم کند.")
		return
	}

	parts := strings.Split(strings.TrimSpace(arg), ":")
	if len(parts) != 2 {
		_ = bot.SendMessage(chatID, "⚠️ فرمت صحیح:\n<code>/setreport HH:MM</code>\n(ساعت ۲۴ ساعته، به وقت UTC)")
		return
	}

	hour, err1 := strconv.Atoi(parts[0])
	minute, err2 := strconv.Atoi(parts[1])
	if err1 != nil || err2 != nil {
		_ = bot.SendMessage(chatID, "⚠️ فرمت صحیح:\n<code>/setreport HH:MM</code>\n(ساعت ۲۴ ساعته، به وقت UTC)")
		return
	}

	if err := h.settingsService.SetAutoReportSchedule(true, hour, minute); err != nil {
		_ = bot.SendMessage(chatID, "❌ خطا در تنظیم زمان گزارش: "+err.Error())
		return
	}

	_ = bot.SendMessage(chatID, fmt.Sprintf(
		"✅ <b>گزارش روزانه تنظیم شد</b>\n\n⏰ هر روز ساعت <code>%02d:%02d</code> (UTC) گزارش مصرف تمام نمایندگان برای شما ارسال می‌شود.",
		hour, minute,
	))
}

// handleAddAdminCommand registers an additional Telegram chat ID to receive
// every admin-facing notification alongside the primary AdminChatID -- a
// confirmed, reported request: the admin wanted more than one recipient for
// their own notifications, with the ability to later remove/rename any one
// of them individually (handleRemoveAdminCommand/ListExtraAdminChatIDs).
func (h *BotCommandHandler) handleAddAdminCommand(bot *BotService, chatID, arg string) {
	if !h.isAdminChat(chatID) {
		_ = bot.SendMessage(chatID, "⛔ فقط مدیر سیستم می‌تواند گیرنده‌ی جدید اضافه کند.")
		return
	}

	parts := strings.Fields(strings.TrimSpace(arg))
	if len(parts) == 0 {
		_ = bot.SendMessage(chatID, "⚠️ فرمت صحیح:\n<code>/addadmin شماره‌آیدی [برچسب]</code>\nمثال واقعی: <code>/addadmin 123456789 گوشی علی</code>\n\n⚠️ به‌جای «شماره‌آیدی» باید آیدی عددی واقعی چت تلگرام را بنویسید، نه خودِ کلمه‌ی «شماره‌آیدی» یا «CHAT_ID».")
		return
	}

	newChatID := parts[0]
	var label *string
	if len(parts) > 1 {
		l := strings.Join(parts[1:], " ")
		label = &l
	}

	entry, err := h.settingsService.AddExtraAdminChatID(newChatID, label)
	if err != nil {
		if errors.Is(err, ErrInvalidTelegramChatID) {
			_ = bot.SendMessage(chatID, fmt.Sprintf("❌ آیدی <code>%s</code> یک عدد نیست. آیدی چت تلگرام همیشه یک عدد است (مثلاً <code>123456789</code>) -- به‌جای آن کلمه‌ای مثل «CHAT_ID» یا «شماره‌آیدی» را عیناً ننویسید.", esc(newChatID)))
			return
		}
		_ = bot.SendMessage(chatID, "❌ این آیدی قبلاً اضافه شده یا خطایی رخ داد: "+err.Error())
		return
	}

	labelText := ""
	if entry.Label != nil {
		labelText = " (" + *entry.Label + ")"
	}
	_ = bot.SendMessage(chatID, fmt.Sprintf(
		"✅ <b>گیرنده‌ی جدید اضافه شد</b>\n\n👤 آیدی: <code>%s</code>%s\n\nاز این پس تمام پیام‌های مدیریتی برای این آیدی هم ارسال می‌شود.",
		newChatID, labelText,
	))
}

// handleRemoveAdminCommand deletes one additional admin recipient by its
// own row ID (shown by /listadmins), mirroring handleAddAdminCommand.
func (h *BotCommandHandler) handleRemoveAdminCommand(bot *BotService, chatID, arg string) {
	if !h.isAdminChat(chatID) {
		_ = bot.SendMessage(chatID, "⛔ فقط مدیر سیستم می‌تواند گیرنده حذف کند.")
		return
	}

	id, err := strconv.Atoi(strings.TrimSpace(arg))
	if err != nil || id <= 0 {
		_ = bot.SendMessage(chatID, "⚠️ فرمت صحیح:\n<code>/removeadmin ID</code>\n(ID را از <code>/listadmins</code> ببینید)")
		return
	}

	if err := h.settingsService.RemoveExtraAdminChatID(uint(id)); err != nil {
		_ = bot.SendMessage(chatID, "❌ خطا در حذف گیرنده: "+err.Error())
		return
	}

	_ = bot.SendMessage(chatID, "✅ گیرنده با موفقیت حذف شد.")
}

// handleListAdminsCommand lists every additional admin recipient currently
// configured, with its row ID (needed for /removeadmin).
func (h *BotCommandHandler) handleListAdminsCommand(bot *BotService, chatID string) {
	if !h.isAdminChat(chatID) {
		_ = bot.SendMessage(chatID, "⛔ فقط مدیر سیستم می‌تواند این فهرست را ببیند.")
		return
	}

	extras, err := h.settingsService.ListExtraAdminChatIDs()
	if err != nil {
		_ = bot.SendMessage(chatID, "❌ خطا در دریافت فهرست: "+err.Error())
		return
	}

	if len(extras) == 0 {
		_ = bot.SendMessage(chatID, "📋 <b>گیرنده‌های اضافه</b>\n\nهیچ گیرنده‌ی اضافه‌ای ثبت نشده است.\nبرای افزودن: <code>/addadmin CHAT_ID [برچسب]</code>")
		return
	}

	var sb strings.Builder
	sb.WriteString("📋 <b>گیرنده‌های اضافه</b>\n\n")
	for _, e := range extras {
		labelText := ""
		if e.Label != nil {
			labelText = " -- " + *e.Label
		}
		sb.WriteString(fmt.Sprintf("🆔 <code>%d</code> | <code>%s</code>%s\n", e.ID, e.ChatID, labelText))
	}
	sb.WriteString("\nبرای حذف: <code>/removeadmin ID</code>")
	_ = bot.SendMessage(chatID, sb.String())
}

// QuickTopUpKeyboard builds the inline keyboard attached to a quota-warning
// alert (see BotNotifier in bot_notifications.go), letting the admin credit
// the affected reseller's wallet with a single tap.
func QuickTopUpKeyboard(reseller model.Reseller) tgbotapi.InlineKeyboardMarkup {
	return tgbotapi.NewInlineKeyboardMarkup(
		tgbotapi.NewInlineKeyboardRow(
			tgbotapi.NewInlineKeyboardButtonData(
				"➕ شارژ سریع کیف پول",
				fmt.Sprintf("%s:%d", callbackQuickTopUp, reseller.ID),
			),
		),
	)
}
