package service

import (
	"errors"
	"fmt"
	"path/filepath"
	"time"

	tgbotapi "github.com/go-telegram-bot-api/telegram-bot-api/v5"
	"go.uber.org/zap"
	"gorm.io/gorm"

	"github.com/maahdima/mwp/api/dataservice/model"
	"github.com/maahdima/mwp/api/utils/timehelper"
)

// BotScheduler is polled once a minute (see cmd/main.go) to fire the two
// admin-configured daily Telegram jobs -- automatic database backup and the
// daily usage report -- at whatever HH:MM the admin set via the bot's
// /setbackup and /setreport commands. A minute-granularity poll plus a
// "last sent date" column (rather than scheduling a one-shot gocron job
// whenever the time changes) keeps this correct across schedule changes and
// process restarts without any extra bookkeeping: every tick just asks "is
// it currently HH:MM in UTC, and have I already sent today's copy?".
type BotScheduler struct {
	db                  *gorm.DB
	settingsService     *BotSettingsService
	backupService       *BackupService
	botService          *BotService
	notifier            *BotNotifier
	walletService       *Wallet
	resellerService     *Reseller
	systemConfigService *SystemConfigService
	logger              *zap.Logger
}

func NewBotScheduler(db *gorm.DB, settingsService *BotSettingsService, backupService *BackupService, botService *BotService, notifier *BotNotifier, walletService *Wallet, resellerService *Reseller, systemConfigService *SystemConfigService) *BotScheduler {
	return &BotScheduler{
		db:                  db,
		settingsService:     settingsService,
		backupService:       backupService,
		botService:          botService,
		notifier:            notifier,
		walletService:       walletService,
		resellerService:     resellerService,
		systemConfigService: systemConfigService,
		logger:              zap.L().Named("BotScheduler"),
	}
}

// Tick checks the current UTC time against both schedules and fires any job
// whose time has arrived and hasn't already been sent today. Safe to call
// every minute indefinitely.
func (s *BotScheduler) Tick() {
	settings, err := s.settingsService.GetOrCreate()
	if err != nil {
		s.logger.Error("failed to load bot settings for scheduler tick", zap.Error(err))
		return
	}

	// If the bot itself is off, BotService.SendMessage/SendDocument would
	// silently no-op anyway -- but without this check, sendScheduledBackup/
	// sendScheduledReport would still mark today as "sent" even though
	// nothing went out, permanently skipping the day once the bot is
	// re-enabled later. Bailing out here instead means a schedule set while
	// the bot was enabled correctly resumes firing as soon as it's
	// re-enabled, rather than silently going stale.
	if !settings.Enabled {
		return
	}

	now := time.Now().UTC()
	today := now.Format("2006-01-02")

	if settings.AutoBackupEnabled &&
		now.Hour() == settings.AutoBackupHour &&
		now.Minute() == settings.AutoBackupMinute &&
		settings.LastBackupSentDate != today {
		s.sendScheduledBackup(settings, today)
	}

	if settings.AutoReportEnabled &&
		now.Hour() == settings.AutoReportHour &&
		now.Minute() == settings.AutoReportMinute &&
		settings.LastReportSentDate != today {
		s.sendScheduledReport(today)
	}
}

// SendInstantBackup creates a fresh backup and pushes it to every configured
// admin Telegram recipient right now, on demand -- the "instant backup"
// button requested for the web panel, distinct from sendScheduledBackup in
// that it deliberately does NOT call MarkBackupSent, so triggering it never
// suppresses (or double-counts against) the day's regularly scheduled
// automatic backup. Returns an error only if the backup itself couldn't be
// created; a delivery failure to Telegram is logged but not surfaced as an
// error here, matching how every other bot notification in this codebase
// treats delivery as best-effort.
func (s *BotScheduler) SendInstantBackup() error {
	settings, err := s.settingsService.GetOrCreate()
	if err != nil {
		return err
	}

	path, cleanup, err := s.backupService.CreateBackup()
	if err != nil {
		return err
	}
	defer cleanup()

	sendPath, sendCleanup := s.compressForTelegramOrFallback(path)
	defer sendCleanup()

	caption := "بکاپ فوری (" + timehelper.TehranDateStamp(time.Now()) + ")"
	s.broadcastDocumentToAdmins(settings, sendPath, caption)
	return nil
}

func (s *BotScheduler) sendScheduledBackup(settings *model.BotSettings, today string) {
	path, cleanup, err := s.backupService.CreateBackup()
	if err != nil {
		s.logger.Error("scheduled backup failed", zap.Error(err))
		s.notifier.NotifyCriticalAlert("خطا در تهیه‌ی بکاپ خودکار پایگاه‌داده: " + err.Error())
		return
	}
	// Confirmed, reported feature request: unlike SendInstantBackup/the
	// web download endpoint (which stay ephemeral -- create, send,
	// delete), the SCHEDULED automatic backup must stay recoverable on
	// disk at a known path, with the previous one deleted first rather
	// than accumulating. RetainAsLatestAutoBackup moves `path` into that
	// fixed slot; `cleanup` is still deferred as a safety net (it
	// no-ops once the file has already been moved away, see its own
	// os.IsNotExist handling) in case retention fails and the original
	// temp file is still sitting there.
	defer cleanup()
	retainedPath, retainErr := s.backupService.RetainAsLatestAutoBackup(path)
	if retainErr != nil {
		s.logger.Error("failed to retain scheduled backup on disk", zap.Error(retainErr))
		// Not fatal to the rest of this run -- the backup still gets
		// sent to Telegram from its original temp path below, the admin
		// just won't also have a local recoverable copy this time.
		retainedPath = path
	}

	sendPath, sendCleanup := s.compressForTelegramOrFallback(retainedPath)
	defer sendCleanup()

	caption := "بکاپ خودکار روزانه (" + timehelper.TehranDateStamp(time.Now()) + ")"
	if !s.broadcastDocumentToAdmins(settings, sendPath, caption) {
		return
	}

	if err := s.settingsService.MarkBackupSent(today); err != nil {
		s.logger.Error("failed to record scheduled backup as sent", zap.Error(err))
	}
}

// broadcastDocumentToAdmins sends the file at path to settings.AdminChatID
// (the primary recipient) AND every configured extra admin chat ID --
// mirrors BotNotifier.broadcastToAdmins for the one document-send case
// (backups) that lives outside BotNotifier itself. Returns whether the
// primary send succeeded, since sendScheduledBackup's own "mark today as
// sent" bookkeeping should track the primary recipient's delivery, not an
// extra recipient's.
//
// Splits path into Telegram-sized parts first (SplitFileForTelegram is a
// no-op single-part result for the common case where path is already under
// the 50MB limit) -- see that method's own doc comment for why this exists:
// gzip compression alone isn't a guarantee for large enough databases, and
// this was previously the point where an oversized file silently failed
// Telegram's own server-side size rejection. Each part is captioned with
// its 1-of-N position and, for a multi-part backup, a trailing reassembly
// hint (`cat mwp-backup-*.part* > mwp-backup.db.gz`) so the admin can
// reconstruct the original file without needing to already know how this
// splitting works.
func (s *BotScheduler) broadcastDocumentToAdmins(settings *model.BotSettings, path, caption string) bool {
	parts, partsCleanup, err := s.backupService.SplitFileForTelegram(path)
	if err != nil {
		s.logger.Error("failed to split backup for telegram delivery", zap.Error(err))
		return false
	}
	defer partsCleanup()

	captions := make([]string, len(parts))
	if len(parts) == 1 {
		captions[0] = caption
	} else {
		baseName := filepath.Base(path)
		reassembleHint := fmt.Sprintf("\n\nبازسازی: تمام بخش‌ها را در یک پوشه قرار داده و دستور زیر را اجرا کنید:\ncat %s.part* > %s",
			baseName, baseName)
		for i := range parts {
			captions[i] = fmt.Sprintf("%s (بخش %d از %d)", caption, i+1, len(parts))
			if i == len(parts)-1 {
				captions[i] += reassembleHint
			}
		}
	}

	primarySucceeded := true
	if settings.AdminChatID != "" {
		primarySucceeded = s.sendPartsToChatID(settings.AdminChatID, parts, captions, "primary admin")
	}

	var extras []model.BotExtraAdminChatID
	if err := s.db.Find(&extras).Error; err != nil {
		s.logger.Warn("failed to load extra admin chat ids for scheduled backup", zap.Error(err))
	}
	for _, e := range extras {
		s.sendPartsToChatID(e.ChatID, parts, captions, "extra recipient "+e.ChatID)
	}

	return primarySucceeded
}

// interPartDelay is a small defensive pause between two consecutive part
// sends to the SAME chat -- a large backup can split into 90+ parts (see
// SplitFileForTelegram's own doc comment on the ~4GB case), and firing all
// of them back-to-back risks tripping Telegram's per-chat flood control
// mid-transfer, which sendPartsWithRetry below can recover from but is
// cheaper to avoid triggering in the first place.
const interPartDelay = 1500 * time.Millisecond

// maxSendRetries bounds how many times a single part is retried after a
// flood-control (429) response before this recipient's whole transfer is
// given up on -- a bound is needed because RetryAfter is server-reported and
// nothing stops Telegram from repeatedly re-issuing it under sustained load.
const maxSendRetries = 5

// sendPartsToChatID sends every part in order to a single chat, stopping
// (and reporting failure) at the first part that fails -- a partial,
// out-of-order backup is worse than useless to reassemble, so there is no
// value in continuing to send later parts once an earlier one has failed.
// Each part is retried with the server-reported backoff (see
// sendPartWithRetry) rather than being treated as an immediate hard failure,
// since a mid-transfer flood-control response is expected, recoverable
// behavior for a multi-part send, not an exceptional one.
func (s *BotScheduler) sendPartsToChatID(chatID string, parts, captions []string, recipientLabel string) bool {
	for i, part := range parts {
		if i > 0 {
			time.Sleep(interPartDelay)
		}
		if err := s.sendPartWithRetry(chatID, part, captions[i]); err != nil {
			s.logger.Error("failed to send backup part to telegram",
				zap.String("recipient", recipientLabel), zap.Int("part", i+1), zap.Int("total_parts", len(parts)), zap.Error(err))
			return false
		}
	}
	return true
}

// sendPartWithRetry sends a single document, retrying up to maxSendRetries
// times if Telegram responds with flood-control (a tgbotapi.Error whose
// RetryAfter is set) -- waiting exactly as long as Telegram itself reports
// is needed rather than guessing a fixed backoff. Any other error (network
// failure, invalid chat, etc.) is returned immediately, unretried, since
// those are not expected to resolve themselves on a timer.
func (s *BotScheduler) sendPartWithRetry(chatID, path, caption string) error {
	var lastErr error
	for attempt := 0; attempt <= maxSendRetries; attempt++ {
		err := s.botService.SendDocument(chatID, path, caption)
		if err == nil {
			return nil
		}
		lastErr = err

		var tgErr *tgbotapi.Error
		if !errors.As(err, &tgErr) || tgErr.RetryAfter <= 0 {
			return err
		}

		s.logger.Warn("telegram flood control hit while sending backup part, waiting before retry",
			zap.String("chat_id", chatID), zap.Int("retry_after_seconds", tgErr.RetryAfter), zap.Int("attempt", attempt+1))
		time.Sleep(time.Duration(tgErr.RetryAfter) * time.Second)
	}
	return fmt.Errorf("exceeded %d retries for flood control: %w", maxSendRetries, lastErr)
}

// compressForTelegramOrFallback gzips the backup file for Telegram delivery
// -- see BackupService.CompressFile's own doc comment for why this exists
// at all (Telegram's Bot API hard-rejects anything over 50MB, and this
// panel's own database has grown well past that). Compression failure
// (e.g. disk full) is logged and swallowed rather than blocking the send
// entirely: falling back to the raw, uncompressed path preserves this
// codebase's existing behavior for that case (still likely to fail
// Telegram's size check on a large database, but no worse than before this
// fix existed), and the returned cleanup func is always safe to call
// unconditionally regardless of which path won.
func (s *BotScheduler) compressForTelegramOrFallback(path string) (sendPath string, cleanup func()) {
	gzPath, gzCleanup, err := s.backupService.CompressFile(path)
	if err != nil {
		s.logger.Warn("failed to compress backup for telegram delivery, sending uncompressed", zap.Error(err))
		return path, func() {}
	}
	return gzPath, gzCleanup
}

func (s *BotScheduler) sendScheduledReport(today string) {
	var resellers []model.Reseller
	if err := s.db.Find(&resellers).Error; err != nil {
		s.logger.Error("failed to load resellers for scheduled report", zap.Error(err))
		return
	}

	entries := make([]DailyReportEntry, 0, len(resellers))
	for _, r := range resellers {
		balance, err := s.walletService.GetWalletBalance(r.ID)
		if err != nil {
			s.logger.Warn("failed to load wallet balance for scheduled report", zap.Uint("reseller_id", r.ID), zap.Error(err))
		}

		peerCount, err := s.resellerService.CountPeers(r.ID)
		if err != nil {
			s.logger.Warn("failed to count peers for scheduled report", zap.Uint("reseller_id", r.ID), zap.Error(err))
		}

		umCount, err := s.resellerService.CountUserManagerAccounts(r.ID)
		if err != nil {
			s.logger.Warn("failed to count user manager accounts for scheduled report", zap.Uint("reseller_id", r.ID), zap.Error(err))
		}

		v2rayCount, err := s.resellerService.CountV2RayPackages(r.ID)
		if err != nil {
			s.logger.Warn("failed to count v2ray packages for scheduled report", zap.Uint("reseller_id", r.ID), zap.Error(err))
		}

		entries = append(entries, DailyReportEntry{
			Reseller:         r,
			WalletBalance:    balance,
			PeerCount:        int(peerCount),
			UserManagerCount: int(umCount),
			V2RayCount:       int(v2rayCount),
		})
	}

	// Total database size (فاز پنجم-۷: "خلاصه‌ی مصرف، حجم دیتابیس... در
	// گزارش دوره‌ای خودکار") -- best-effort, since GetDatabaseSizeBreakdown
	// deliberately errors out for a Postgres-backed install (dbstat is
	// SQLite-only, see that method's own doc comment); a nil pointer here
	// just means NotifyDailyReport omits the database-size line rather
	// than failing the whole report.
	var databaseSizeBytes int64
	if s.systemConfigService != nil {
		if breakdown, err := s.systemConfigService.GetDatabaseSizeBreakdown(); err != nil {
			s.logger.Warn("failed to read database size for scheduled report", zap.Error(err))
		} else {
			databaseSizeBytes = breakdown.TotalBytes
		}
	}

	s.notifier.NotifyDailyReport(entries, databaseSizeBytes)

	if err := s.settingsService.MarkReportSent(today); err != nil {
		s.logger.Error("failed to record scheduled report as sent", zap.Error(err))
	}
}
