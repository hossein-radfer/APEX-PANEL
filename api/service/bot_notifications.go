package service

import (
	"fmt"
	"html"
	"strings"
	"time"

	tgbotapi "github.com/go-telegram-bot-api/telegram-bot-api/v5"
	"go.uber.org/zap"
	"gorm.io/gorm"

	"github.com/maahdima/mwp/api/dataservice/model"
	"github.com/maahdima/mwp/api/utils/timehelper"
)

// BotNotifier is the single entry point every other service calls into to
// push a Telegram notification. Each method independently checks its own
// BotSettings.Notify* toggle before sending, so callers never need to know
// or care whether that particular notification type is currently enabled --
// they just call, e.g., NotifyLoginAlert unconditionally on every login, and
// this type decides whether that actually results in a Telegram message.
//
// Every message body here is Persian with Telegram HTML formatting
// (<b>...</b> etc.) since BotService sends every message with
// ParseMode=HTML -- any admin/reseller-controlled string (name, IP,
// arbitrary error text) MUST go through html.EscapeString(...) (aliased
// esc below) before being interpolated, otherwise a name containing "<" or
// "&" would corrupt the message markup or swallow part of the text.
type BotNotifier struct {
	bot             *BotService
	db              *gorm.DB
	settingsService *BotSettingsService
	logger          *zap.Logger
}

func NewBotNotifier(bot *BotService, db *gorm.DB, settingsService *BotSettingsService) *BotNotifier {
	return &BotNotifier{
		bot:             bot,
		db:              db,
		settingsService: settingsService,
		logger:          zap.L().Named("BotNotifier"),
	}
}

const bytesPerGBForNotify = 1024 * 1024 * 1024

func gbString(bytes int64) string {
	return fmt.Sprintf("%.2f گیگابایت", float64(bytes)/bytesPerGBForNotify)
}

// esc escapes a string for safe interpolation into an HTML-parse-mode
// Telegram message. Short alias since it's used on nearly every line below.
func esc(s string) string {
	return html.EscapeString(s)
}

// extraAdminChatIDs returns every additional admin recipient's chat ID
// (see model.BotExtraAdminChatID's own doc comment) -- best-effort: a DB
// error here just means this tick sends only to the primary AdminChatID
// rather than failing the whole notification.
func (n *BotNotifier) extraAdminChatIDs() []string {
	var extras []model.BotExtraAdminChatID
	if err := n.db.Find(&extras).Error; err != nil {
		n.logger.Warn("failed to load extra admin chat ids, sending to primary admin only", zap.Error(err))
		return nil
	}
	ids := make([]string, 0, len(extras))
	for _, e := range extras {
		ids = append(ids, e.ChatID)
	}
	return ids
}

// broadcastToAdmins sends text to settings.AdminChatID (the primary
// recipient) AND every configured extra admin chat ID -- the single fan-out
// point every admin-facing notification below routes through, so "add a
// second/third admin recipient" (a confirmed, reported request) is a single
// change rather than one per notification type.
func (n *BotNotifier) broadcastToAdmins(settings *model.BotSettings, text string) {
	if settings.AdminChatID != "" {
		if err := n.bot.SendMessage(settings.AdminChatID, text); err != nil {
			n.logger.Warn("failed to send admin notification", zap.Error(err))
		}
	}
	for _, chatID := range n.extraAdminChatIDs() {
		if err := n.bot.SendMessage(chatID, text); err != nil {
			n.logger.Warn("failed to send admin notification to extra recipient", zap.String("chat_id", chatID), zap.Error(err))
		}
	}
}

// broadcastToAdminsWithKeyboard mirrors broadcastToAdmins, for the one
// notification (NotifyQuotaWarning) that attaches an inline action keyboard
// -- every configured admin recipient gets the same actionable button, not
// just the primary AdminChatID.
func (n *BotNotifier) broadcastToAdminsWithKeyboard(settings *model.BotSettings, text string, keyboard *tgbotapi.InlineKeyboardMarkup) {
	if settings.AdminChatID != "" {
		if err := n.bot.SendMessageWithKeyboard(settings.AdminChatID, text, keyboard); err != nil {
			n.logger.Warn("failed to send admin notification", zap.Error(err))
		}
	}
	for _, chatID := range n.extraAdminChatIDs() {
		if err := n.bot.SendMessageWithKeyboard(chatID, text, keyboard); err != nil {
			n.logger.Warn("failed to send admin notification to extra recipient", zap.String("chat_id", chatID), zap.Error(err))
		}
	}
}

// NotifyLoginAlert sends a security notice to a reseller's own Telegram
// chat when they log into the web panel, including the source IP and time.
func (n *BotNotifier) NotifyLoginAlert(resellerID uint, ipAddress, loginTime string) {
	settings, err := n.settingsService.GetOrCreate()
	if err != nil || !settings.NotifyLoginAlerts {
		return
	}

	var reseller model.Reseller
	if err := n.db.First(&reseller, resellerID).Error; err != nil || reseller.TelegramChatID == nil {
		return
	}

	text := fmt.Sprintf(
		"🔐 <b>هشدار ورود به حساب</b>\n\nحساب کاربری شما همین الان وارد پنل شد.\n\n👤 نام کاربری: <code>%s</code>\n📍 آی‌پی: <code>%s</code>\n🕒 زمان: %s\n\n⚠️ اگر این ورود توسط شما نبوده، فوراً با مدیر سیستم تماس بگیرید.",
		esc(reseller.Username), esc(ipAddress), esc(loginTime),
	)
	if err := n.bot.SendMessage(*reseller.TelegramChatID, text); err != nil {
		n.logger.Warn("failed to send login alert", zap.Uint("reseller_id", resellerID), zap.Error(err))
	}
}

// NotifyAdminLoginAlert sends the admin themselves a login notice whenever
// the admin account logs in -- the admin has no separate "reseller row" to
// hang NotifyLoginAlert's lookup off of, so this is a distinct method
// straight to settings.AdminChatID.
// NotifyOtpCode sends the admin-login two-factor code to every admin
// recipient (broadcastToAdmins -- primary AdminChatID plus every extra
// configured chat ID, so any of the admin's own trusted devices/people can
// relay the code) -- unlike every other Notify* method here, this one does
// NOT check a BotSettings.Notify* toggle first, because the caller
// (Authentication.maybeStartOtpChallenge) already gated on
// BotSettings.OtpEnabled itself; requiring a second toggle here would just
// be redundant, not an extra safety check.
func (n *BotNotifier) NotifyOtpCode(settings *model.BotSettings, username, ipAddress, code string, validitySeconds int) {
	text := fmt.Sprintf(
		"🔑 <b>کد ورود دو مرحله‌ای</b>\n\nیک نفر با نام کاربری <code>%s</code> از آی‌پی <code>%s</code> در حال ورود به پنل است.\n\nکد تایید: <code>%s</code>\n⏱ اعتبار: %d ثانیه\n\nاگر این تلاش برای ورود از طرف شما نبوده، این کد را در اختیار کسی قرار ندهید.",
		esc(username), esc(ipAddress), esc(code), validitySeconds,
	)
	n.broadcastToAdmins(settings, text)
}

// NotifyResellerOtpCode mirrors NotifyOtpCode exactly (same message shape,
// same "no extra toggle needed" reasoning -- the caller already gated on
// Reseller.OtpEnabled) but delivers to that ONE reseller's own
// TelegramChatID instead of the admin-wide broadcastToAdmins fan-out,
// since reseller OTP is a per-account opt-in, not a panel-wide setting.
func (n *BotNotifier) NotifyResellerOtpCode(chatID, username, ipAddress, code string, validitySeconds int) {
	text := fmt.Sprintf(
		"🔑 <b>کد ورود دو مرحله‌ای</b>\n\nیک نفر با نام کاربری <code>%s</code> از آی‌پی <code>%s</code> در حال ورود به پنل است.\n\nکد تایید: <code>%s</code>\n⏱ اعتبار: %d ثانیه\n\nاگر این تلاش برای ورود از طرف شما نبوده، این کد را در اختیار کسی قرار ندهید.",
		esc(username), esc(ipAddress), esc(code), validitySeconds,
	)
	if err := n.bot.SendMessage(chatID, text); err != nil {
		n.logger.Warn("failed to send reseller otp code", zap.String("username", username), zap.Error(err))
	}
}

// NotifyRecurringCostDue alerts the admin that one recurring cost (a
// server-renewal payment, typically) has come due -- part of the
// accounting module's "recurring server-renewal invoices" requirement
// (see AccountingScheduler.Tick, which calls this once per due cost per
// day). Always sent regardless of any Notify* toggle, same reasoning as
// NotifyOtpCode: this is its own opt-in feature (a recurring cost only
// exists if the admin explicitly created one with a
// RecurrenceIntervalDays), not a blanket notification category that needs
// a second gate.
func (n *BotNotifier) NotifyRecurringCostDue(settings *model.BotSettings, partnerName string, amountToman int64, dueDate string) {
	text := fmt.Sprintf(
		"🧾 <b>یادآوری هزینه دوره‌ای</b>\n\nپرداخت به <b>%s</b> سررسید شده است.\n\n💰 مبلغ: %s تومان\n📅 تاریخ سررسید: %s",
		esc(partnerName), esc(formatToman(amountToman)), esc(dueDate),
	)
	n.broadcastToAdmins(settings, text)
}

// NotifyTunnelHealthChange alerts the admin that an infrastructure
// tunnel's (GRE/IPIP/EoIP/WireGuard-as-link -- see
// TunnelGraphService.DiscoveredTunnelInterfaces for how "tunnel" is
// determined) computed health just transitioned into a worse state (see
// TunnelHealthService.recordTransition, part of the "tunnel-ai" system).
// Always sent regardless of any Notify* toggle, same reasoning as
// NotifyRecurringCostDue: this is its own dedicated feature, not a
// blanket notification category needing a second gate -- and the caller
// itself only invokes this on a healthy->(suspect|confirmed_down)
// TRANSITION, never repeatedly while a tunnel stays down.
func (n *BotNotifier) NotifyTunnelHealthChange(settings *model.BotSettings, interfaceName, severity, evidence string, detectedAt time.Time) {
	severityFa := "مشکوک"
	emoji := "⚠️"
	if severity == "confirmed_down" {
		severityFa = "قطع تایید شده"
		emoji = "🔴"
	}
	text := fmt.Sprintf(
		"%s <b>هشدار سلامت تانل</b>\n\nتانل <code>%s</code> به وضعیت <b>%s</b> تغییر کرد.\n\n🕒 زمان: %s\n📋 شواهد: %s",
		emoji, esc(interfaceName), esc(severityFa), esc(timehelper.TehranDateStamp(detectedAt)), esc(evidence),
	)
	n.broadcastToAdmins(settings, text)
}

// NotifyTunnelActionTaken alerts the admin about ONE remediation decision
// TunnelHealthService.logAction just recorded (Level 1 toggle, Level 2
// failover intent, a boot-grace/redline/cooldown rejection, or a
// fall-back-to-healthy recovery note) -- a confirmed, reported gap:
// NotifyTunnelHealthChange above only ever announces the DETECTED
// problem, never what the panel did about it or whether that action
// actually worked, leaving the admin to go check the "تاریخچه و
// گزارش‌ها" page manually for that. Every call into logAction already
// represents a rare, one-time event per incident (see logAction's own
// call sites: attemptLevel1/attemptLevel2 only fire once per unhealthy
// episode, considerFallbackRecovery only fires on the healthy transition
// edge), so this is safe to send unconditionally without risking a
// per-poll-tick spam pattern.
func (n *BotNotifier) NotifyTunnelActionTaken(settings *model.BotSettings, interfaceName, level, description string, simulated bool, result string, executedAt time.Time) {
	modeFa := "واقعی"
	if simulated {
		modeFa = "شبیه‌سازی (حالت آزمایشی)"
	}
	text := fmt.Sprintf(
		"🛠 <b>اقدام سلامت تانل</b>\n\nتانل <code>%s</code> -- سطح %s\n\n📝 اقدام: %s\n⚙️ حالت اجرا: %s\n📌 نتیجه: %s\n🕒 زمان: %s",
		esc(interfaceName), esc(level), esc(description), esc(modeFa), esc(result), esc(timehelper.TehranDateStamp(executedAt)),
	)
	n.broadcastToAdmins(settings, text)
}

// NotifyTunnelHealthDegrading is the EARLY-WARNING alert -- fired when a
// tunnel's rolling health score (see TunnelHealthService.computeHealthScore)
// has dropped substantially while the tunnel is STILL "healthy" per the
// binary severity computation, i.e. before Poll would otherwise say
// anything at all. Deliberately visually distinct (🟡, "در حال ضعیف
// شدن") from NotifyTunnelHealthChange's own ⚠️/🔴 so an admin never
// confuses an early warning with an already-confirmed detection --
// the whole point is to buy lead time before that alert ever fires.
func (n *BotNotifier) NotifyTunnelHealthDegrading(settings *model.BotSettings, interfaceName string, previousScore, currentScore int, now time.Time) {
	text := fmt.Sprintf(
		"🟡 <b>روند سلامت تانل در حال ضعیف شدن</b>\n\nتانل <code>%s</code> هنوز قطع نشده، ولی روند سلامتش رو به وخامت است.\n\n📉 امتیاز سلامت: %d ← %d (از ۱۰۰)\n🕒 زمان: %s\n\nℹ️ این فقط یک هشدار زودهنگام است؛ اقدام درمانی خودکاری انجام نشده.",
		esc(interfaceName), previousScore, currentScore, esc(timehelper.TehranDateStamp(now)),
	)
	n.broadcastToAdmins(settings, text)
}

// NotifyTunnelIncidentDiagnosis alerts the admin with the self-healing
// engine's own root-cause guess for a confirmed_down incident (see
// TunnelHealthService.diagnoseIncident) -- sent once per incident,
// alongside (not instead of) NotifyTunnelHealthChange's own detection
// alert, so the admin's very first message about an incident already
// hints at WHERE to look (this one tunnel's remote end, the whole
// router's uplink, or the router itself being overloaded) instead of
// leaving that entirely to manual investigation.
func (n *BotNotifier) NotifyTunnelIncidentDiagnosis(settings *model.BotSettings, interfaceName, cause, evidence string, now time.Time) {
	causeFa := "نامشخص"
	switch cause {
	case "remote_unreachable":
		causeFa = "به نظر می‌رسد مشکل مختص همین تانل است (سایر مقصدها از همین روتر در دسترس‌اند)"
	case "router_uplink_down":
		causeFa = "به نظر می‌رسد کل اتصال روتر قطع شده (سایر مقصدها هم از این روتر در دسترس نیستند)"
	case "router_resource_exhausted":
		causeFa = "به نظر می‌رسد خود روتر تحت فشار منابع (CPU/RAM) است"
	}
	text := fmt.Sprintf(
		"🔎 <b>تشخیص علت احتمالی قطعی</b>\n\nتانل <code>%s</code>\n\n🧭 حدس علت: %s\n📋 شواهد: %s\n🕒 زمان: %s",
		esc(interfaceName), esc(causeFa), esc(evidence), esc(timehelper.TehranDateStamp(now)),
	)
	n.broadcastToAdmins(settings, text)
}

// NotifyBackupPathUnreachable alerts the admin that a tunnel's OWN
// configured Level 2 backup gateway failed its nightly active probe (see
// TunnelHealthService.ProbeBackupPaths) -- the whole point of the
// active-probe capability: catch a dead backup BEFORE a real incident
// ever needs it, rather than discovering the double failure live.
func (n *BotNotifier) NotifyBackupPathUnreachable(settings *model.BotSettings, interfaceName, backupGatewayIP, evidence string, now time.Time) {
	text := fmt.Sprintf(
		"🔴 <b>مسیر بکاپ تانل در دسترس نیست</b>\n\nتانل <code>%s</code> -- آی‌پی واسطه‌ی بکاپ <code>%s</code>\n\n📋 شواهد: %s\n🕒 زمان: %s\n\n⚠️ اگر همین الان این تانل قطع شود، سطح ۲ نمی‌تواند ترافیک را با اطمینان به این بکاپ منتقل کند.",
		esc(interfaceName), esc(backupGatewayIP), esc(evidence), esc(timehelper.TehranDateStamp(now)),
	)
	n.broadcastToAdmins(settings, text)
}

// NotifyTunnelIncidentPatternReport sends the periodic (weekly) digest of
// recurring-failure patterns TunnelHealthService.BuildIncidentPatternReport
// computed from TunnelHealthEvent's own history -- an analytical,
// low-frequency message (unlike every other Notify* here, which fires on
// a specific transition/incident) meant to surface infrastructure-level
// insights (a chronically-flapping tunnel, two tunnels that keep failing
// together) that no single incident alert would ever reveal on its own.
// A caller that found nothing worth reporting should simply not call
// this at all, rather than sending an empty/uninteresting digest.
func (n *BotNotifier) NotifyTunnelIncidentPatternReport(settings *model.BotSettings, reportBody string, now time.Time) {
	text := fmt.Sprintf(
		"📊 <b>گزارش هفتگی الگوی قطعی‌های تانل</b>\n🕒 %s\n\n%s",
		esc(timehelper.TehranDateStamp(now)), reportBody,
	)
	n.broadcastToAdmins(settings, text)
}

// NotifyUserManagerProtocolDown alerts the admin that one User Manager
// protocol (L2TP/PPTP/SSTP/OpenVPN) failed its hybrid health check (see
// UserManagerProtocolHealthService.recordTransition) -- spec section
// ب-7's own explicit requirement that these four protocols are
// ALERT-ONLY, never fed into an automatic remediation action (restarting
// one could disrupt active customer sessions). Same "transition only,
// always sent" convention as NotifyTunnelHealthChange above.
func (n *BotNotifier) NotifyUserManagerProtocolDown(settings *model.BotSettings, protocol, evidence string) {
	text := fmt.Sprintf(
		"🔴 <b>هشدار سرویس User Manager</b>\n\nسرویس <code>%s</code> از دسترس خارج شده است.\n\n📋 شواهد: %s\n\n⚠️ این هشدار صرفاً اطلاع‌رسانی است -- هیچ اقدام خودکاری انجام نشد (طبق سیاست این پروتکل).",
		esc(protocol), esc(evidence),
	)
	n.broadcastToAdmins(settings, text)
}

// NotifyV2RayPanelDown alerts the admin that an x-ui panel just failed
// its own reachability check (see V2RayPanelHealthService.pollOnePanel)
// -- spec section ب-7's V2Ray/container row, alert-only in this phase
// since this codebase has no remote restart path for a third-party x-ui
// server (see that service's own top-level doc comment). Same
// "transition only, always sent" convention as
// NotifyUserManagerProtocolDown above.
func (n *BotNotifier) NotifyV2RayPanelDown(settings *model.BotSettings, panelName, errorDetail string) {
	text := fmt.Sprintf(
		"🔴 <b>هشدار پنل V2Ray</b>\n\nپنل <code>%s</code> از دسترس خارج شده است.\n\n📋 خطا: %s\n\n⚠️ این هشدار صرفاً اطلاع‌رسانی است -- هیچ اقدام خودکاری انجام نشد.",
		esc(panelName), esc(errorDetail),
	)
	n.broadcastToAdmins(settings, text)
}

// NotifyV2RayPanelContainerStopped alerts the admin with a MORE SPECIFIC
// diagnosis than NotifyV2RayPanelDown (item 9): this fires only when the
// panel is unreachable AND the RouterOS container it's mapped to
// (XuiPanel.ContainerServerID/ContainerName) was confirmed actually
// stopped/stopping on the router itself -- telling the admin exactly what
// broke ("the container stopped," fixable by restarting it on the router)
// rather than the generic "panel unreachable" NotifyV2RayPanelDown sends
// for every other kind of failure (network partition, x-ui crash inside a
// still-RUNNING container, wrong credentials, etc).
func (n *BotNotifier) NotifyV2RayPanelContainerStopped(settings *model.BotSettings, panelName, serverName, containerName, containerStatus string) {
	text := fmt.Sprintf(
		"🛑 <b>کانتینر پنل V2Ray متوقف شده</b>\n\nپنل <code>%s</code> از دسترس خارج شده و علت آن مشخص شد: کانتینر <code>%s</code> روی سرور <code>%s</code> در وضعیت <code>%s</code> است (اجرا نمی‌شود).\n\n💡 برای رفع مشکل، کانتینر را روی میکروتیک دوباره راه‌اندازی کنید.\n\n⚠️ این هشدار صرفاً اطلاع‌رسانی است -- هیچ اقدام خودکاری انجام نشد.",
		esc(panelName), esc(containerName), esc(serverName), esc(containerStatus),
	)
	n.broadcastToAdmins(settings, text)
}

// faoximaResellerLabel resolves resellerID to a human-readable name for
// the two Faoxima-webhook alerts below -- falls back to the bare ID if
// the reseller row itself can't be read, since the alert is still
// useful without a name (and must never be swallowed just because this
// one lookup failed).
func (n *BotNotifier) faoximaResellerLabel(resellerID uint) string {
	var reseller model.Reseller
	if err := n.db.First(&reseller, resellerID).Error; err != nil {
		return fmt.Sprintf("#%d", resellerID)
	}
	return fmt.Sprintf("%s (#%d)", reseller.Name, resellerID)
}

// NotifyFaoximaWebhookHealed alerts the admin that
// FaoximaProvisionerService.VerifyWebhooks found one reseller's Faoxima
// bot webhook pointing at the wrong URL (most likely: the panel was
// restored onto a different server, or Telegram itself cleared the
// webhook after delivery failures) and successfully re-registered it --
// the explicit disaster-recovery safety net the admin asked for
// ("می‌ترسم... ربات‌ها هیچ‌کدام کار نکنند یا ست وبهوک انجام نده"), so a
// migration's side effect on Faoxima bots is surfaced and confirmed
// fixed rather than silently going unnoticed.
func (n *BotNotifier) NotifyFaoximaWebhookHealed(settings *model.BotSettings, resellerID uint, previousURL, correctedURL string) {
	text := fmt.Sprintf(
		"🟡 <b>وبهوک ربات ایکس یک نماینده اصلاح شد</b>\n\n👤 نماینده: %s\n\nوبهوک این ربات روی آدرس نادرستی تنظیم شده بود (احتمالاً به‌خاطر جابه‌جایی سرور) و اکنون به‌صورت خودکار اصلاح شد.\n\n❌ آدرس قبلی: <code>%s</code>\n✅ آدرس صحیح: <code>%s</code>",
		esc(n.faoximaResellerLabel(resellerID)), esc(previousURL), esc(correctedURL),
	)
	n.broadcastToAdmins(settings, text)
}

// NotifyFaoximaWebhookBroken alerts the admin that a Faoxima instance's
// webhook was found pointing at the wrong URL AND the automatic
// re-registration attempt itself failed (e.g. the new server's own
// domain/DNS/TLS for faoximaDomain isn't ready yet) -- unlike
// NotifyFaoximaWebhookHealed's "found and fixed" case, this needs the
// admin's own manual intervention (re-run Enable once the underlying
// cause is resolved), so the message says so explicitly rather than
// implying the problem already resolved itself.
func (n *BotNotifier) NotifyFaoximaWebhookBroken(settings *model.BotSettings, resellerID uint, previousURL, expectedURL, errorDetail string) {
	text := fmt.Sprintf(
		"🔴 <b>وبهوک ربات ایکس یک نماینده خراب است</b>\n\n👤 نماینده: %s\n\nوبهوک این ربات روی آدرس نادرستی تنظیم شده (احتمالاً به‌خاطر جابه‌جایی سرور) و تلاش خودکار برای اصلاح آن هم ناموفق بود -- این ربات فعلاً پیام‌های تلگرام را دریافت نمی‌کند.\n\n❌ آدرس فعلی: <code>%s</code>\n🎯 آدرس مورد انتظار: <code>%s</code>\n📋 خطا: %s\n\n⚠️ لطفاً پس از رفع مشکل زیرساخت (مثلاً DNS/SSL دامنه)، از پنل این ربات را یک‌بار غیرفعال و دوباره فعال کنید.",
		esc(n.faoximaResellerLabel(resellerID)), esc(previousURL), esc(expectedURL), esc(errorDetail),
	)
	n.broadcastToAdmins(settings, text)
}

// NotifyFaoximaTokenInvalid alerts the admin that VerifyWebhooks found a
// Faoxima instance whose stored bot token Telegram itself now rejects
// (HTTP 401 -- almost always the reseller deleted/revoked that bot via
// @BotFather, often while replacing it with a new one) -- a confirmed,
// reported incident: a reseller's bot silently stopped working entirely
// after they did exactly this, with no admin or reseller notification of
// any kind, since VerifyWebhooks previously treated this identically to a
// transient network error (log and skip). Unlike NotifyFaoximaWebhookBroken,
// there is no "automatic re-registration attempt" to report on here --
// this can ONLY be fixed by the reseller supplying their new bot's token
// via the panel's own "تغییر توکن ربات" (Update Token) action, which is
// what the message tells them (FaoximaInstance.ErrorMessage, surfaced in
// FaoximaForm.tsx, carries the same instruction for the reseller
// directly).
func (n *BotNotifier) NotifyFaoximaTokenInvalid(settings *model.BotSettings, resellerID uint) {
	text := fmt.Sprintf(
		"🔴 <b>توکن ربات ایکس یک نماینده دیگر معتبر نیست</b>\n\n👤 نماینده: %s\n\nتلگرام توکن ذخیره‌شده‌ی این ربات را رد کرد -- به احتمال زیاد نماینده ربات قبلی را از طریق @BotFather حذف یا توکن آن را بازنشانی کرده است. این ربات دیگر هیچ پیامی از تلگرام دریافت نمی‌کند.\n\n⚠️ این مورد به‌صورت خودکار قابل رفع نیست: نماینده باید توکن ربات جدید خود را از بخش «تغییر توکن ربات» در پنل خودش وارد کند.",
		esc(n.faoximaResellerLabel(resellerID)),
	)
	n.broadcastToAdmins(settings, text)
}

// NotifyFaoximaPeriodExpired alerts the admin that
// FaoximaProvisionerService.CheckExpiredPeriods just auto-disabled one
// reseller's Faoxima instance because its configured billing period
// elapsed -- the admin's own explicit request: running a reseller's
// instance costs the panel real resources, so it should not silently
// keep running (or silently stop with no explanation) once its paid
// period is over. The admin must explicitly call AdminResetPeriod (from
// the panel) once that reseller pays for their next period.
func (n *BotNotifier) NotifyFaoximaPeriodExpired(settings *model.BotSettings, resellerID uint) {
	text := fmt.Sprintf(
		"⏳ <b>دوره‌ی ربات ایکس یک نماینده به پایان رسید</b>\n\n👤 نماینده: %s\n\nدوره‌ی صورتحساب این ربات تمام شده و به‌صورت خودکار غیرفعال شد.\n\nپس از دریافت پرداخت دوره‌ی بعد، از پنل «ریست دوره» را برای این نماینده بزنید تا ربات دوباره فعال شود.",
		esc(n.faoximaResellerLabel(resellerID)),
	)
	n.broadcastToAdmins(settings, text)
}

// formatToman renders a Toman amount with thousands separators (e.g.
// "1,250,000") -- Persian bookkeeping convention for legibility on amounts
// that are routinely in the millions.
func formatToman(amount int64) string {
	s := fmt.Sprintf("%d", amount)
	neg := false
	if strings.HasPrefix(s, "-") {
		neg = true
		s = s[1:]
	}
	var out []byte
	for i, c := range []byte(s) {
		if i > 0 && (len(s)-i)%3 == 0 {
			out = append(out, ',')
		}
		out = append(out, c)
	}
	if neg {
		return "-" + string(out)
	}
	return string(out)
}

func (n *BotNotifier) NotifyAdminLoginAlert(username, ipAddress, loginTime string) {
	settings, err := n.settingsService.GetOrCreate()
	if err != nil || !settings.NotifyLoginAlerts {
		return
	}

	text := fmt.Sprintf(
		"🔐 <b>هشدار ورود به حساب مدیر</b>\n\nحساب مدیر سیستم همین الان وارد پنل شد.\n\n👤 نام کاربری: <code>%s</code>\n📍 آی‌پی: <code>%s</code>\n🕒 زمان: %s",
		esc(username), esc(ipAddress), esc(loginTime),
	)
	n.broadcastToAdmins(settings, text)
}

// NotifyFailedLogin alerts BOTH the overall admin and (if the failed
// username matches a known reseller) that reseller's own chat about a
// failed login attempt -- a wrong-password attempt against a reseller
// account is exactly the kind of event the reseller themselves needs to see
// immediately, not just the admin. If username doesn't resolve to any known
// admin or reseller account, only the admin is notified (there is no
// specific account owner to alert, but a wrong-password sweep against
// unknown usernames is still worth the admin knowing about).
func (n *BotNotifier) NotifyFailedLogin(username, ipAddress, loginTime string) {
	settings, err := n.settingsService.GetOrCreate()
	if err != nil || !settings.NotifyLoginAlerts {
		return
	}

	text := fmt.Sprintf(
		"🚨 <b>تلاش ناموفق برای ورود</b>\n\nیک تلاش ورود با رمز عبور نادرست ثبت شد.\n\n👤 نام کاربری: <code>%s</code>\n📍 آی‌پی: <code>%s</code>\n🕒 زمان: %s\n\n⚠️ اگر این تلاش توسط شما نبوده، احتمال دارد کسی در حال حدس زدن رمز عبور شما باشد.",
		esc(username), esc(ipAddress), esc(loginTime),
	)

	n.broadcastToAdmins(settings, text)

	var reseller model.Reseller
	if err := n.db.Where("username = ?", username).First(&reseller).Error; err == nil && reseller.TelegramChatID != nil {
		if err := n.bot.SendMessage(*reseller.TelegramChatID, text); err != nil {
			n.logger.Warn("failed to send reseller failed-login alert", zap.Uint("reseller_id", reseller.ID), zap.Error(err))
		}
	}
}

// NotifyQuotaWarning alerts both the admin (with a one-tap quick top-up
// button) and the reseller themselves when a reseller's usage crosses the
// given threshold (e.g. under 10% remaining).
func (n *BotNotifier) NotifyQuotaWarning(reseller model.Reseller, percentRemaining int) {
	settings, err := n.settingsService.GetOrCreate()
	if err != nil || !settings.NotifyQuotaWarnings {
		return
	}

	quotaStr := "نامحدود"
	if reseller.QuotaBytes != nil {
		quotaStr = gbString(*reseller.QuotaBytes)
	}

	{
		text := fmt.Sprintf(
			"⚠️ <b>هشدار اتمام حجم نماینده</b>\n\n👤 نماینده: <b>%s</b>\n📊 مصرف: %s از %s\n🔻 باقیمانده: %%%d\n\n💳 برای شارژ سریع کیف پول این نماینده، روی دکمه زیر بزنید.",
			esc(reseller.Name), gbString(reseller.UsedBytes), quotaStr, percentRemaining,
		)
		keyboard := QuickTopUpKeyboard(reseller)
		n.broadcastToAdminsWithKeyboard(settings, text, &keyboard)
	}

	if reseller.TelegramChatID != nil {
		text := fmt.Sprintf(
			"⚠️ <b>حجم شما رو به اتمام است</b>\n\n📊 مصرف: %s از %s\n🔻 باقیمانده: %%%d\n\n📦 در صورت نیاز، می‌توانید از پنل یک بسته ترافیک مازاد خریداری کنید.",
			gbString(reseller.UsedBytes), quotaStr, percentRemaining,
		)
		if err := n.bot.SendMessage(*reseller.TelegramChatID, text); err != nil {
			n.logger.Warn("failed to send reseller quota warning", zap.Uint("reseller_id", reseller.ID), zap.Error(err))
		}
	}
}

// NotifyPurchaseReceipt confirms a completed traffic-package purchase to
// both the admin and the purchasing reseller. priceAmount is a whole-Toman
// integer (this panel bills exclusively in Toman), so it's rendered with
// formatToman's Persian thousands-separator grouping, not a decimal amount.
func (n *BotNotifier) NotifyPurchaseReceipt(reseller model.Reseller, packageName string, trafficBytes, priceAmount int64) {
	settings, err := n.settingsService.GetOrCreate()
	if err != nil || !settings.NotifyPurchaseReceipts {
		return
	}

	text := fmt.Sprintf(
		"🧾 <b>رسید خرید بسته ترافیک</b>\n\n👤 نماینده: <b>%s</b>\n📦 بسته: %s\n➕ حجم اضافه‌شده: %s\n💰 مبلغ: %s تومان\n\n✅ خرید با موفقیت انجام شد.",
		esc(reseller.Name), esc(packageName), gbString(trafficBytes), formatToman(priceAmount),
	)

	n.broadcastToAdmins(settings, text)
	if reseller.TelegramChatID != nil {
		if err := n.bot.SendMessage(*reseller.TelegramChatID, text); err != nil {
			n.logger.Warn("failed to send reseller purchase receipt", zap.Error(err))
		}
	}
}

// NotifyLiveLog pushes a real-time event (e.g. a new peer being created) to
// the admin's chat. This is a firehose of routine activity, gated by its
// own toggle since an admin may want other alerts but not this volume of
// traffic.
func (n *BotNotifier) NotifyLiveLog(event string) {
	settings, err := n.settingsService.GetOrCreate()
	if err != nil || !settings.NotifyLiveLog {
		return
	}

	text := "📋 <b>رویداد جدید</b>\n\n" + esc(event)
	n.broadcastToAdmins(settings, text)
}

// NotifyCriticalAlert sends a CRITICAL ERROR notice to the admin -- used for
// Mikrotik/database connectivity loss. Unlike every other notification
// type, this one is NOT gated behind BotSettings.Enabled/token checks
// beyond what SendMessage itself already no-ops on: an outage is exactly
// the moment an admin most needs to hear about it, so this method makes no
// assumption about the rest of the panel being healthy when it's called.
func (n *BotNotifier) NotifyCriticalAlert(message string) {
	settings, err := n.settingsService.GetOrCreate()
	if err != nil || !settings.NotifyCriticalAlerts {
		return
	}

	text := "🚨 <b>خطای بحرانی سیستم</b>\n\n" + esc(message) + "\n\n⛔ لطفاً هرچه سریع‌تر بررسی کنید."
	n.broadcastToAdmins(settings, text)
}

// NotifyAccountStatusChange informs a reseller their account was
// activated/deactivated by an admin.
func (n *BotNotifier) NotifyAccountStatusChange(reseller model.Reseller, isActive bool) {
	settings, err := n.settingsService.GetOrCreate()
	if err != nil || !settings.NotifyAccountStatus || reseller.TelegramChatID == nil {
		return
	}

	var text string
	if isActive {
		text = "✅ <b>حساب شما فعال شد</b>\n\nحساب کاربری شما توسط مدیر سیستم فعال شد و می‌توانید از پنل استفاده کنید."
	} else {
		text = "⛔ <b>حساب شما غیرفعال شد</b>\n\nحساب کاربری شما توسط مدیر سیستم غیرفعال شد. برای اطلاعات بیشتر با مدیر سیستم تماس بگیرید."
	}

	if err := n.bot.SendMessage(*reseller.TelegramChatID, text); err != nil {
		n.logger.Warn("failed to send account status change notice", zap.Uint("reseller_id", reseller.ID), zap.Error(err))
	}
}

// DailyReportEntry is one reseller's row of data for NotifyDailyReport --
// gathered by BotScheduler.sendScheduledReport (wallet balance + WireGuard +
// User Manager stats each come from a different service), so BotNotifier
// itself stays free of a Wallet/Reseller dependency.
type DailyReportEntry struct {
	Reseller         model.Reseller
	WalletBalance    int64 // whole Toman
	PeerCount        int
	UserManagerCount int
	V2RayCount       int
}

// NotifyDailyReport sends the admin a single, compact summary combining
// wallet balance and traffic usage per reseller -- called once a day by the
// scheduler tick in cmd/main.go, at the time configured via the bot's
// /setreport command (AutoReportEnabled/Hour/Minute), not a web-settings
// toggle.
//
// Formatting rules (per the redesign request): a resellers-need-attention
// section (low wallet balance or near/over quota) is listed first with a
// warning icon, everything else follows in a compact line each; fields that
// are zero/not applicable (no WireGuard quota assigned, no User Manager
// accounts, zero wallet balance) are omitted entirely rather than shown as
// "0" clutter.
// databaseSizeBytes is 0 when unavailable (e.g. a Postgres-backed install,
// see BotScheduler.sendScheduledReport's own doc comment) -- in that case
// the database-size line is simply omitted, matching this report's
// existing "omit zero/not-applicable fields rather than show a
// misleading 0" convention (see the doc comment on formatting rules
// above).
func (n *BotNotifier) NotifyDailyReport(entries []DailyReportEntry, databaseSizeBytes int64) {
	settings, err := n.settingsService.GetOrCreate()
	if err != nil || !settings.AutoReportEnabled {
		return
	}

	n.broadcastToAdmins(settings, buildDailyReportText(entries, databaseSizeBytes))
}

// buildDailyReportText is NotifyDailyReport's pure text-building step,
// split out so the formatting rules (attention-first ordering,
// zero-value omission, the database-size line) are unit-testable without
// a live BotService/BotSettings round trip.
func buildDailyReportText(entries []DailyReportEntry, databaseSizeBytes int64) string {
	if len(entries) == 0 {
		return "📅 <b>گزارش روزانه</b>\n\nهیچ نماینده‌ای ثبت نشده است."
	}

	var attention, normal []string
	for _, e := range entries {
		line, needsAttention := formatDailyReportLine(e)
		if needsAttention {
			attention = append(attention, line)
		} else {
			normal = append(normal, line)
		}
	}

	var sb strings.Builder
	sb.WriteString(fmt.Sprintf("📅 <b>گزارش روزانه (%d نماینده)</b>\n", len(entries)))

	if databaseSizeBytes > 0 {
		sb.WriteString(fmt.Sprintf("\n💾 حجم پایگاه داده پنل: <b>%s</b>\n", gbString(databaseSizeBytes)))
	}

	if len(attention) > 0 {
		sb.WriteString("\n⚠️ <b>نیاز به بررسی</b>\n")
		for _, l := range attention {
			sb.WriteString(l)
		}
	}

	if len(normal) > 0 {
		sb.WriteString("\n✅ <b>وضعیت عادی</b>\n")
		for _, l := range normal {
			sb.WriteString(l)
		}
	}

	return sb.String()
}

// formatDailyReportLine builds one reseller's report block (one line per
// protocol the reseller actually has accounts on, per the redesign
// request -- previously every protocol was crammed onto a single line
// with no way to tell which figure belonged to which protocol, or how many
// accounts/what account limit applied) and reports whether it belongs in
// the "needs attention" section (wallet balance at or below zero, or any
// protocol's usage at 90%+ of a finite quota, or its account count at/over
// its finite account limit).
func formatDailyReportLine(e DailyReportEntry) (string, bool) {
	r := e.Reseller

	wgLine, wgOver := formatProtocolLine("🔌 وایرگارد", e.PeerCount, r.MaxPeers, r.UsedBytes, r.QuotaBytes)
	umLine, umOver := formatProtocolLine("🌐 یوزرمنجیر", e.UserManagerCount, r.UserManagerMaxAccounts, r.UserManagerUsedBytes, r.UserManagerQuotaBytes)
	v2rayLine, v2rayOver := formatProtocolLine("📡 وی‌توری", e.V2RayCount, r.V2RayMaxPackages, r.V2RayUsedBytes, r.V2RayQuotaBytes)

	needsAttention := e.WalletBalance <= 0 || !r.IsActive || wgOver || umOver || v2rayOver

	statusIcon := "🟢"
	if !r.IsActive {
		statusIcon = "⛔"
	} else if needsAttention {
		statusIcon = "🔴"
	}

	var sb strings.Builder
	sb.WriteString(fmt.Sprintf("%s <b>%s</b>\n", statusIcon, esc(r.Name)))
	sb.WriteString(fmt.Sprintf("   💰 موجودی کیف پول: %s تومان\n", formatToman(e.WalletBalance)))
	sb.WriteString(wgLine)
	sb.WriteString(umLine)
	sb.WriteString(v2rayLine)

	return sb.String(), needsAttention
}

// formatProtocolLine builds one protocol's own indented sub-line: volume
// used/limit + percent, and account count/limit + remaining slots --
// omitted entirely (returns "") if this reseller has zero accounts on this
// protocol, matching the previous "don't show 0/Unlimited clutter" rule.
// Returns whether this protocol alone pushes the reseller into
// "needs attention" (volume at/over 90% of a finite quota, or account
// count at/over a finite account limit).
func formatProtocolLine(label string, accountCount int, maxAccounts *int, usedBytes int64, quotaBytes *int64) (string, bool) {
	if accountCount == 0 {
		return "", false
	}

	over := false

	volumeStr := fmt.Sprintf("%s (نامحدود)", gbString(usedBytes))
	if quotaBytes != nil {
		percent := 0.0
		if *quotaBytes > 0 {
			percent = float64(usedBytes) / float64(*quotaBytes) * 100
		}
		if percent >= 90 {
			over = true
		}
		volumeStr = fmt.Sprintf("%s/%s (%%%.0f)", gbString(usedBytes), gbString(*quotaBytes), percent)
	}

	accountsStr := fmt.Sprintf("%d اکانت (نامحدود)", accountCount)
	if maxAccounts != nil {
		remaining := *maxAccounts - accountCount
		if remaining < 0 {
			remaining = 0
		}
		if accountCount >= *maxAccounts {
			over = true
		}
		accountsStr = fmt.Sprintf("%d/%d اکانت (%d باقیمانده)", accountCount, *maxAccounts, remaining)
	}

	return fmt.Sprintf("   %s: %s | %s\n", label, volumeStr, accountsStr), over
}
