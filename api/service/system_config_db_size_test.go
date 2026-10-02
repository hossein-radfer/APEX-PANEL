package service

import (
	"fmt"
	"os"
	"strings"
	"testing"
	"time"

	"github.com/glebarez/sqlite"
	"gorm.io/gorm"

	"github.com/maahdima/mwp/api/dataservice/model"
)

// TestGetDatabaseSizeBreakdown_ReportsTablesLargestFirst is the regression
// test for the admin-only database size breakdown feature (requested
// after this session's own manual dbstat investigation of a bloated
// production database): confirms the total file size is read from the
// real on-disk file, per-table byte usage comes back non-zero for tables
// with actual rows, and results are sorted largest-first as documented.
func TestGetDatabaseSizeBreakdown_ReportsTablesLargestFirst(t *testing.T) {
	dbFile, err := os.CreateTemp(t.TempDir(), "db_size_test_*.db")
	if err != nil {
		t.Fatalf("failed to create temp db file: %v", err)
	}
	dbPath := dbFile.Name()
	_ = dbFile.Close()
	// gorm/sqlite creates the actual file content on Open; the temp file
	// above just reserves a real path on disk for os.Stat to measure.
	_ = os.Remove(dbPath)

	db, err := gorm.Open(sqlite.Open(dbPath), &gorm.Config{})
	if err != nil {
		t.Fatalf("failed to open test db: %v", err)
	}
	t.Cleanup(func() {
		if sqlDB, err := db.DB(); err == nil {
			_ = sqlDB.Close()
		}
	})
	if err := db.AutoMigrate(&model.Reseller{}, &model.AuditLog{}); err != nil {
		t.Fatalf("failed to migrate test db: %v", err)
	}

	// Seed AuditLog with many more rows than Reseller, so it should
	// dominate the byte-usage ranking.
	padding := strings.Repeat("x", 200)
	for i := 0; i < 200; i++ {
		if err := db.Create(&model.AuditLog{
			ResellerID:   1,
			ResellerName: "size-test",
			Action:       "test",
			Description:  fmt.Sprintf("padding-%d-%s", i, padding),
		}).Error; err != nil {
			t.Fatalf("failed to seed audit log row: %v", err)
		}
	}
	if err := db.Create(&model.Reseller{Name: "size-test", Username: "size-test"}).Error; err != nil {
		t.Fatalf("failed to seed reseller: %v", err)
	}

	t.Setenv("DB_DIALECT", "sqlite")
	t.Setenv("DB_NAME", dbPath)

	svc := NewSystemConfigService(db)
	breakdown, err := svc.GetDatabaseSizeBreakdown()
	if err != nil {
		t.Fatalf("GetDatabaseSizeBreakdown failed: %v", err)
	}

	if breakdown.TotalBytes <= 0 {
		t.Fatalf("expected a positive TotalBytes read from the real database file, got %d", breakdown.TotalBytes)
	}
	if len(breakdown.Tables) == 0 {
		t.Fatal("expected at least one table entry in the breakdown")
	}

	for i := 1; i < len(breakdown.Tables); i++ {
		if breakdown.Tables[i-1].BytesUsed < breakdown.Tables[i].BytesUsed {
			t.Fatalf("expected tables sorted largest-first, but entry %d (%d bytes) is smaller than entry %d (%d bytes)",
				i-1, breakdown.Tables[i-1].BytesUsed, i, breakdown.Tables[i].BytesUsed)
		}
	}

	var auditLogBytes, resellerBytes int64
	for _, entry := range breakdown.Tables {
		switch entry.Name {
		case "audit_logs":
			auditLogBytes = entry.BytesUsed
		case "resellers":
			resellerBytes = entry.BytesUsed
		}
	}
	if auditLogBytes == 0 {
		t.Fatal("expected an audit_logs entry with nonzero bytes given 200 seeded rows")
	}
	if auditLogBytes <= resellerBytes {
		t.Fatalf("expected audit_logs (%d bytes, 200 rows) to be larger than resellers (%d bytes, 1 row)", auditLogBytes, resellerBytes)
	}
}

// TestGetDatabaseSizeBreakdown_RejectsNonSqlite confirms the Postgres
// guard: this feature is SQLite-only (dbstat is a SQLite-specific virtual
// table), and a Postgres-backed install must get an explicit, clear error
// rather than a wrong or empty result.
func TestGetDatabaseSizeBreakdown_RejectsNonSqlite(t *testing.T) {
	dsn := fmt.Sprintf("file:db_size_reject_test_%d?mode=memory&cache=shared", time.Now().UnixNano())
	db, err := gorm.Open(sqlite.Open(dsn), &gorm.Config{})
	if err != nil {
		t.Fatalf("failed to open test db: %v", err)
	}

	t.Setenv("DB_DIALECT", "postgres")

	svc := NewSystemConfigService(db)
	if _, err := svc.GetDatabaseSizeBreakdown(); err == nil {
		t.Fatal("expected GetDatabaseSizeBreakdown to reject a non-sqlite dialect, got nil error")
	}
}
