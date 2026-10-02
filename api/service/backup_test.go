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
