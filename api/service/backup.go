package service

import (
	"bytes"
	"compress/gzip"
	"errors"
	"fmt"
	"io"
	"os"
	"path/filepath"
	"time"

	"go.uber.org/zap"
	"gorm.io/gorm"
)

// sqliteHeaderMagic is the first 16 bytes of every valid SQLite database
// file ("SQLite format 3\x00"). Used to reject non-database uploads before
// they're ever considered for a restore.
var sqliteHeaderMagic = []byte("SQLite format 3\x00")

// BackupService creates and restores point-in-time snapshots of the sqlite
// database file. Restricted to sqlite: Postgres deployments have their own
// standard backup tooling (pg_dump) and VACUUM INTO is a sqlite-only
// statement, so this service explicitly refuses to operate against a
// Postgres-backed panel rather than doing something meaningless.
type BackupService struct {
	db         *gorm.DB
	dialect    string
	dbFilePath string
	dataDir    string
	logger     *zap.Logger
}

func NewBackupService(db *gorm.DB, dialect, dbFilePath, dataDir string) *BackupService {
	return &BackupService{
		db:         db,
		dialect:    dialect,
		dbFilePath: dbFilePath,
		dataDir:    dataDir,
		logger:     zap.L().Named("BackupService"),
	}
}

var ErrBackupUnsupportedDialect = errors.New("database backup is only supported for sqlite deployments")

// CreateBackup produces a consistent, point-in-time snapshot of the live
// database using SQLite's VACUUM INTO, writing it to a fresh temp file
// rather than ever exposing the live mwp.db file directly (which could be
// mid-write, or locked, at the moment of download). The caller is
// responsible for removing the returned path once it has been streamed to
// the client (see the returned cleanup func).
func (s *BackupService) CreateBackup() (path string, cleanup func(), err error) {
	if s.dialect != "sqlite" {
		return "", nil, ErrBackupUnsupportedDialect
	}

	sqlDB, err := s.db.DB()
	if err != nil {
		s.logger.Error("failed to get underlying sql.DB for backup", zap.Error(err))
		return "", nil, err
	}

	backupDir := filepath.Join(s.dataDir, "backups")
	if err := os.MkdirAll(backupDir, 0o755); err != nil {
		s.logger.Error("failed to create backup staging directory", zap.Error(err))
		return "", nil, err
	}

	backupPath := filepath.Join(backupDir, fmt.Sprintf("mwp-backup-%s.db", time.Now().UTC().Format("20060102-150405")))

	// VACUUM INTO requires the destination not to already exist.
	if _, statErr := os.Stat(backupPath); statErr == nil {
		if removeErr := os.Remove(backupPath); removeErr != nil {
			return "", nil, removeErr
		}
	}

	// SQLite's VACUUM INTO takes its own internal read lock and produces a
	// fully consistent copy even while the live database is being written
	// to concurrently by other connections in the pool -- this is the
	// documented, safe way to snapshot a live SQLite database without
	// stopping the server.
	quotedPath := sqliteQuoteLiteral(backupPath)
	if _, execErr := sqlDB.Exec(fmt.Sprintf("VACUUM INTO %s", quotedPath)); execErr != nil {
		s.logger.Error("VACUUM INTO failed", zap.Error(execErr))
		_ = os.Remove(backupPath)
		return "", nil, execErr
	}

	cleanup = func() {
		if removeErr := os.Remove(backupPath); removeErr != nil && !os.IsNotExist(removeErr) {
			s.logger.Warn("failed to remove temporary backup file", zap.String("path", backupPath), zap.Error(removeErr))
		}
	}

	return backupPath, cleanup, nil
}

// autoBackupRetainedFileName is the fixed (non-timestamped) filename the
// scheduled automatic backup is kept under -- see RetainAsLatestAutoBackup.
const autoBackupRetainedFileName = "auto-backup-latest.db"

// RetainAsLatestAutoBackup moves tempPath (a fresh CreateBackup result)
// into this install's fixed "latest scheduled backup" slot, deleting
// whatever was there before FIRST -- confirmed, reported feature
// request: scheduled automatic backups must stay recoverable on disk at
// a known path (matching the terminal-menu "backup" feature's own
// "show the exact file path" requirement elsewhere in this project),
// but only ever ONE version at a time, never accumulating. Manual
// backups (SendInstantBackup, the web download endpoint) are deliberately
// NOT affected by this -- those remain the existing ephemeral
// create-send-delete behavior, since an admin explicitly requesting a
// one-off backup already has it in hand (sent to Telegram / downloaded)
// and has no expectation of it also being kept here.
func (s *BackupService) RetainAsLatestAutoBackup(tempPath string) (string, error) {
	backupDir := filepath.Join(s.dataDir, "backups")
	retainedPath := filepath.Join(backupDir, autoBackupRetainedFileName)

	if removeErr := os.Remove(retainedPath); removeErr != nil && !os.IsNotExist(removeErr) {
		return "", fmt.Errorf("failed to remove previous auto-backup: %w", removeErr)
	}

	// Rename (not copy) -- tempPath and retainedPath are always in the
	// same directory (backupDir), so this is an atomic, same-filesystem
	// move with no partial-file window.
	if err := os.Rename(tempPath, retainedPath); err != nil {
		return "", fmt.Errorf("failed to move backup into retained slot: %w", err)
	}

	return retainedPath, nil
}

// CompressFile gzips the file at path into a sibling "<path>.gz" file,
// returning its path and a cleanup func that removes just the compressed
// copy (the original, uncompressed file is left untouched -- the caller
// still owns its own cleanup for that). Confirmed, reported bug this fixes:
// Telegram's Bot API hard-rejects any document over 50MB (regardless of
// this codebase's own logic -- it's a platform limit enforced by Telegram's
// servers), so once this panel's live database grew past that threshold
// (674MB observed, up from 27MB two weeks earlier), every scheduled/instant
// backup send to Telegram started silently failing. A SQLite database file
// is highly compressible (large runs of zero-padding in unused page space,
// and repeated structural bytes across similar rows/indexes), so gzip
// alone gets many real-world backups back under the limit even before
// combining this with a self-hosted Bot API server (which raises the
// ceiling to 2000MB) for the deployments large enough that compression
// alone isn't enough.
func (s *BackupService) CompressFile(path string) (gzPath string, cleanup func(), err error) {
	gzPath = path + ".gz"

	src, err := os.Open(path)
	if err != nil {
		return "", nil, fmt.Errorf("failed to open backup file for compression: %w", err)
	}
	defer src.Close()

	dst, err := os.Create(gzPath)
	if err != nil {
		return "", nil, fmt.Errorf("failed to create compressed backup file: %w", err)
	}

	gz := gzip.NewWriter(dst)
	if _, copyErr := io.Copy(gz, src); copyErr != nil {
		gz.Close()
		dst.Close()
		_ = os.Remove(gzPath)
		return "", nil, fmt.Errorf("failed to compress backup file: %w", copyErr)
	}
	if closeErr := gz.Close(); closeErr != nil {
		dst.Close()
		_ = os.Remove(gzPath)
		return "", nil, fmt.Errorf("failed to finalize compressed backup file: %w", closeErr)
	}
	if closeErr := dst.Close(); closeErr != nil {
		_ = os.Remove(gzPath)
		return "", nil, fmt.Errorf("failed to close compressed backup file: %w", closeErr)
	}

	cleanup = func() {
		if removeErr := os.Remove(gzPath); removeErr != nil && !os.IsNotExist(removeErr) {
			s.logger.Warn("failed to remove temporary compressed backup file", zap.String("path", gzPath), zap.Error(removeErr))
		}
	}
	return gzPath, cleanup, nil
}

// TelegramMaxDocumentBytes is Telegram Bot API's hard per-document upload
// ceiling (50MB), enforced server-side by Telegram regardless of anything
// this codebase does. SplitFileForTelegram below leaves a safety margin
// under this exact number (see its own doc comment).
const TelegramMaxDocumentBytes = 50 * 1024 * 1024

// telegramChunkTargetBytes is the actual per-chunk size SplitFileForTelegram
// targets -- deliberately well under TelegramMaxDocumentBytes (45MB, a 5MB/
// 10% margin) rather than the exact limit, since Telegram's own multipart
// upload overhead and any future minor library change should never be able
// to push an intentionally-exact-50MB chunk over the real server-side
// ceiling.
const telegramChunkTargetBytes = 45 * 1024 * 1024

// SplitFileForTelegram is this panel's fix for the confirmed, reported gap
// left by CompressFile alone: gzip shrinks most backups under Telegram's
// 50MB cap, but a large enough live database (674MB+ observed, see
// CompressFile's own doc comment) can still compress to something over the
// limit, and neither compressForTelegramOrFallback nor SendDocument
// previously had any fallback beyond "let Telegram's API reject the
// upload" -- which BotScheduler.SendInstantBackup/sendScheduledBackup both
// then swallow as a best-effort delivery failure, silently. Splitting the
// (already gzip-compressed, when possible) file into fixed-size
// telegramChunkTargetBytes parts lets the whole backup still reach the
// admin's Telegram chat as N ordered documents, reassembled later with a
// single `cat` command (see the caption BotScheduler attaches to each
// part). Returns a single-element slice unchanged (never splits) when path
// is already under TelegramMaxDocumentBytes -- most backups, most of the
// time -- so the common case pays zero extra I/O cost.
func (s *BackupService) SplitFileForTelegram(path string) (parts []string, cleanup func(), err error) {
	info, err := os.Stat(path)
	if err != nil {
		return nil, nil, fmt.Errorf("failed to stat file for splitting: %w", err)
	}
	if info.Size() <= TelegramMaxDocumentBytes {
		return []string{path}, func() {}, nil
	}

	src, err := os.Open(path)
	if err != nil {
		return nil, nil, fmt.Errorf("failed to open file for splitting: %w", err)
	}
	defer src.Close()

	var partPaths []string
	cleanup = func() {
		for _, p := range partPaths {
			if removeErr := os.Remove(p); removeErr != nil && !os.IsNotExist(removeErr) {
				s.logger.Warn("failed to remove temporary backup part file", zap.String("path", p), zap.Error(removeErr))
			}
		}
	}

	for partNum := 1; ; partNum++ {
		partPath := fmt.Sprintf("%s.part%03d", path, partNum)
		dst, createErr := os.Create(partPath)
		if createErr != nil {
			cleanup()
			return nil, nil, fmt.Errorf("failed to create backup part file: %w", createErr)
		}
		partPaths = append(partPaths, partPath)

		written, copyErr := io.CopyN(dst, src, telegramChunkTargetBytes)
		closeErr := dst.Close()

		if copyErr != nil && !errors.Is(copyErr, io.EOF) {
			cleanup()
			return nil, nil, fmt.Errorf("failed to write backup part file: %w", copyErr)
		}
		if closeErr != nil {
			cleanup()
			return nil, nil, fmt.Errorf("failed to close backup part file: %w", closeErr)
		}

		reachedEOF := errors.Is(copyErr, io.EOF) || written < telegramChunkTargetBytes
		if reachedEOF {
			break
		}
	}

	return partPaths, cleanup, nil
}

// sqliteQuoteLiteral quotes a filesystem path as a SQLite string literal by
// doubling embedded single quotes -- VACUUM INTO's destination is a plain
// SQL string literal, not a bind parameter, so it can't use the driver's
// normal parameterized-query path.
func sqliteQuoteLiteral(s string) string {
	escaped := ""
	for _, r := range s {
		if r == '\'' {
			escaped += "''"
		} else {
			escaped += string(r)
		}
	}
	return "'" + escaped + "'"
}

// StageRestore validates an uploaded database file and writes it to a
// pending-restore location alongside the live database, WITHOUT touching
// the live database file. The live process keeps running against the
// existing file/connections untouched; an operator must stop the service,
// move the staged file into place over the live one, and start the service
// again -- see the panel documentation for the restore procedure. This
// deliberately does not attempt a hot swap: GORM/the sqlite driver hold an
// open connection pool against the current file, and replacing the file out
// from under an active pool risks corruption or an inconsistent mix of old
// and new state across different pooled connections.
func (s *BackupService) StageRestore(content []byte) (stagedPath string, err error) {
	if s.dialect != "sqlite" {
		return "", ErrBackupUnsupportedDialect
	}

	if err := validateSQLiteFile(content); err != nil {
		return "", err
	}

	restoreDir := filepath.Join(s.dataDir, "backups")
	if err := os.MkdirAll(restoreDir, 0o755); err != nil {
		s.logger.Error("failed to create restore staging directory", zap.Error(err))
		return "", err
	}

	stagedPath = filepath.Join(restoreDir, "mwp-pending-restore.db")

	if err := os.WriteFile(stagedPath, content, 0o600); err != nil {
		s.logger.Error("failed to write staged restore file", zap.Error(err))
		return "", err
	}

	s.logger.Warn("database restore staged; service must be stopped, the staged file moved over the live database, and restarted to apply it",
		zap.String("staged_path", stagedPath), zap.String("live_db_path", s.dbFilePath))

	return stagedPath, nil
}

// validateSQLiteFile rejects anything that isn't a genuine SQLite database
// file before it's ever written to disk as a restore candidate. This is a
// structural check only (the magic header every SQLite file starts with) --
// it doesn't guarantee the file's *schema* is compatible with this panel,
// which is caught later when the operator restarts the service and
// AutoMigrate runs against it.
func validateSQLiteFile(content []byte) error {
	if len(content) < len(sqliteHeaderMagic) {
		return errors.New("uploaded file is too small to be a SQLite database")
	}
	if !bytes.Equal(content[:len(sqliteHeaderMagic)], sqliteHeaderMagic) {
		return errors.New("uploaded file is not a valid SQLite database (bad header)")
	}
	return nil
}
