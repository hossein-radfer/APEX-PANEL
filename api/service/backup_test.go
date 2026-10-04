package service

import (
	"os"
	"path/filepath"
	"testing"

	"github.com/glebarez/sqlite"
	"github.com/maahdima/mwp/api/dataservice/model"
	"gorm.io/gorm"
)

// openFileBackedTestDB opens a real file-backed (not :memory:) sqlite
// database, since VACUUM INTO operates on the actual database file on disk.
// The connection is registered for automatic closing via t.Cleanup --
// unlike POSIX, Windows refuses to delete a file that's still open, so
// t.TempDir()'s own end-of-test removal would otherwise fail here.
func openFileBackedTestDB(t *testing.T) (*gorm.DB, string) {
	t.Helper()

	dir := t.TempDir()
	dbPath := filepath.Join(dir, "test.db")

	db, err := gorm.Open(sqlite.Open(dbPath), &gorm.Config{})
	if err != nil {
		t.Fatalf("failed to open file-backed test db: %v", err)
	}
	if err := db.AutoMigrate(&model.Reseller{}); err != nil {
		t.Fatalf("failed to migrate test db: %v", err)
	}

	t.Cleanup(func() {
		if sqlDB, err := db.DB(); err == nil {
			_ = sqlDB.Close()
		}
	})

	return db, dbPath
}

func TestCreateBackupProducesValidSnapshot(t *testing.T) {
	db, dbPath := openFileBackedTestDB(t)
	dataDir := filepath.Dir(dbPath)

	if err := db.Create(&model.Reseller{Name: "backup-test-reseller", Username: "backup-test-reseller"}).Error; err != nil {
		t.Fatalf("failed to seed reseller: %v", err)
	}

	svc := NewBackupService(db, "sqlite", dbPath, dataDir)

	backupPath, cleanup, err := svc.CreateBackup()
	if err != nil {
		t.Fatalf("CreateBackup failed: %v", err)
	}
	defer cleanup()

	if _, err := os.Stat(backupPath); err != nil {
		t.Fatalf("expected backup file to exist at %s: %v", backupPath, err)
	}

	content, err := os.ReadFile(backupPath)
	if err != nil {
		t.Fatalf("failed to read backup file: %v", err)
	}
	if err := validateSQLiteFile(content); err != nil {
		t.Fatalf("backup file is not a valid SQLite database: %v", err)
	}

	// Open the backup independently and confirm the seeded data is present.
	// The connection is explicitly closed before cleanup() runs: on Windows
	// an open file handle blocks deletion (unlike POSIX, which allows
	// removing an open file), so this mirrors what the production
	// DownloadBackup handler must also get right.
	backupDB, err := gorm.Open(sqlite.Open(backupPath), &gorm.Config{})
	if err != nil {
		t.Fatalf("failed to open backup file as a database: %v", err)
	}
	var reseller model.Reseller
	if err := backupDB.Where("username = ?", "backup-test-reseller").First(&reseller).Error; err != nil {
		t.Fatalf("expected seeded reseller to be present in backup, got error: %v", err)
	}
	backupSqlDB, err := backupDB.DB()
	if err != nil {
		t.Fatalf("failed to get underlying sql.DB for backup inspection connection: %v", err)
	}
	if err := backupSqlDB.Close(); err != nil {
		t.Fatalf("failed to close backup inspection connection: %v", err)
	}

	// The cleanup func must actually remove the file.
	cleanup()
	if _, err := os.Stat(backupPath); !os.IsNotExist(err) {
		t.Fatalf("expected backup file to be removed after cleanup, stat err: %v", err)
	}
}

func TestCreateBackupRejectsNonSqliteDialect(t *testing.T) {
	db, dbPath := openFileBackedTestDB(t)
	svc := NewBackupService(db, "postgres", dbPath, filepath.Dir(dbPath))

	if _, _, err := svc.CreateBackup(); err != ErrBackupUnsupportedDialect {
		t.Fatalf("expected ErrBackupUnsupportedDialect, got %v", err)
	}
}

func TestStageRestoreAcceptsValidSqliteFile(t *testing.T) {
	db, dbPath := openFileBackedTestDB(t)
	dataDir := filepath.Dir(dbPath)
	svc := NewBackupService(db, "sqlite", dbPath, dataDir)

	// Produce a real, valid sqlite file to use as the "uploaded" content.
	backupPath, cleanup, err := svc.CreateBackup()
	if err != nil {
		t.Fatalf("CreateBackup failed: %v", err)
	}
	defer cleanup()

	content, err := os.ReadFile(backupPath)
	if err != nil {
		t.Fatalf("failed to read generated backup: %v", err)
	}

	stagedPath, err := svc.StageRestore(content)
	if err != nil {
		t.Fatalf("StageRestore failed on a valid sqlite file: %v", err)
	}

	stagedContent, err := os.ReadFile(stagedPath)
	if err != nil {
		t.Fatalf("failed to read staged file: %v", err)
	}
	if len(stagedContent) != len(content) {
		t.Fatalf("staged file size %d does not match uploaded content size %d", len(stagedContent), len(content))
	}

	// Staging must never touch the live database file.
	liveContentAfter, err := os.ReadFile(dbPath)
	if err != nil {
		t.Fatalf("failed to read live db file after staging: %v", err)
	}
	if len(liveContentAfter) == 0 {
		t.Fatalf("live db file appears to have been truncated/touched by StageRestore")
	}
}

func TestStageRestoreRejectsNonSqliteContent(t *testing.T) {
	db, dbPath := openFileBackedTestDB(t)
	svc := NewBackupService(db, "sqlite", dbPath, filepath.Dir(dbPath))

	garbage := []byte("this is definitely not a sqlite database file")
	if _, err := svc.StageRestore(garbage); err == nil {
		t.Fatalf("expected StageRestore to reject non-sqlite content, got nil error")
	}
}

func TestStageRestoreRejectsTooSmallContent(t *testing.T) {
	db, dbPath := openFileBackedTestDB(t)
	svc := NewBackupService(db, "sqlite", dbPath, filepath.Dir(dbPath))

	if _, err := svc.StageRestore([]byte("short")); err == nil {
		t.Fatalf("expected StageRestore to reject too-small content, got nil error")
	}
}

// TestRetainAsLatestAutoBackup_MovesFileToFixedPath is the regression
// test for the confirmed, reported feature request: the scheduled
// automatic backup must persist on disk at a known, stable path (not the
// ephemeral timestamped temp path CreateBackup returns).
func TestRetainAsLatestAutoBackup_MovesFileToFixedPath(t *testing.T) {
	db, dbPath := openFileBackedTestDB(t)
	dataDir := filepath.Dir(dbPath)
	svc := NewBackupService(db, "sqlite", dbPath, dataDir)

	tempPath, cleanup, err := svc.CreateBackup()
	if err != nil {
		t.Fatalf("CreateBackup failed: %v", err)
	}
	defer cleanup()

	retainedPath, err := svc.RetainAsLatestAutoBackup(tempPath)
	if err != nil {
		t.Fatalf("RetainAsLatestAutoBackup failed: %v", err)
	}

	if _, err := os.Stat(tempPath); !os.IsNotExist(err) {
		t.Errorf("expected the original temp backup file to no longer exist after retention, stat err: %v", err)
	}
	if _, err := os.Stat(retainedPath); err != nil {
		t.Fatalf("expected retained backup file to exist at %s: %v", retainedPath, err)
	}
	if filepath.Base(retainedPath) != autoBackupRetainedFileName {
		t.Errorf("expected retained file name %q, got %q", autoBackupRetainedFileName, filepath.Base(retainedPath))
	}
}

// TestRetainAsLatestAutoBackup_DeletesPreviousVersionFirst is the second
// half of the same regression test: when a new scheduled backup is
// retained, any PREVIOUS retained backup must be deleted first -- never
// accumulating multiple versions.
func TestRetainAsLatestAutoBackup_DeletesPreviousVersionFirst(t *testing.T) {
	db, dbPath := openFileBackedTestDB(t)
	dataDir := filepath.Dir(dbPath)
	svc := NewBackupService(db, "sqlite", dbPath, dataDir)

	firstTemp, firstCleanup, err := svc.CreateBackup()
	if err != nil {
		t.Fatalf("first CreateBackup failed: %v", err)
	}
	defer firstCleanup()
	firstRetained, err := svc.RetainAsLatestAutoBackup(firstTemp)
	if err != nil {
		t.Fatalf("first RetainAsLatestAutoBackup failed: %v", err)
	}

	secondTemp, secondCleanup, err := svc.CreateBackup()
	if err != nil {
		t.Fatalf("second CreateBackup failed: %v", err)
	}
	defer secondCleanup()
	secondRetained, err := svc.RetainAsLatestAutoBackup(secondTemp)
	if err != nil {
		t.Fatalf("second RetainAsLatestAutoBackup failed: %v", err)
	}

	if firstRetained != secondRetained {
		t.Fatalf("expected both retentions to resolve to the same fixed path, got %q then %q", firstRetained, secondRetained)
	}

	backupDir := filepath.Join(dataDir, "backups")
	entries, err := os.ReadDir(backupDir)
	if err != nil {
		t.Fatalf("failed to read backup dir: %v", err)
	}
	dbFileCount := 0
	for _, e := range entries {
		if filepath.Ext(e.Name()) == ".db" {
			dbFileCount++
		}
	}
	if dbFileCount != 1 {
		t.Fatalf("expected exactly 1 retained .db backup file after two scheduled backups, found %d", dbFileCount)
	}
}
