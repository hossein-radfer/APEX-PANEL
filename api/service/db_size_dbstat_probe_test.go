package service

import (
	"fmt"
	"testing"
	"time"

	"github.com/glebarez/sqlite"
	"gorm.io/gorm"
)

// TestDbstatVirtualTableIsQueryable is a probe, not a feature test: it
// confirms the glebarez/sqlite driver this codebase already uses is built
// with SQLITE_ENABLE_DBSTAT_VTAB, since GetDatabaseSizeBreakdown's entire
// approach depends on the `dbstat` virtual table being queryable. If this
// ever starts failing (e.g. a driver upgrade drops the build tag),
// GetDatabaseSizeBreakdown needs a fallback, not just this test.
func TestDbstatVirtualTableIsQueryable(t *testing.T) {
	dsn := fmt.Sprintf("file:dbstat_probe_test_%d?mode=memory&cache=shared", time.Now().UnixNano())
	db, err := gorm.Open(sqlite.Open(dsn), &gorm.Config{})
	if err != nil {
		t.Fatalf("failed to open test db: %v", err)
	}
	if err := db.Exec("CREATE TABLE probe (id INTEGER PRIMARY KEY, data TEXT)").Error; err != nil {
		t.Fatalf("failed to create probe table: %v", err)
	}
	if err := db.Exec("INSERT INTO probe (data) VALUES ('hello world')").Error; err != nil {
		t.Fatalf("failed to insert probe row: %v", err)
	}

	var rows []struct {
		Name   string
		Pgsize int64
	}
	if err := db.Raw("SELECT name, SUM(pgsize) as pgsize FROM dbstat GROUP BY name").Scan(&rows).Error; err != nil {
		t.Fatalf("dbstat virtual table is not queryable through this driver: %v", err)
	}
	if len(rows) == 0 {
		t.Fatal("expected at least one row from dbstat (the probe table itself), got none")
	}

	found := false
	for _, r := range rows {
		if r.Name == "probe" && r.Pgsize > 0 {
			found = true
		}
	}
	if !found {
		t.Fatalf("expected a 'probe' entry with positive pgsize in dbstat results, got: %+v", rows)
	}
}
