package seeds

import (
	"fmt"
	"testing"
	"time"

	"github.com/glebarez/sqlite"
	"github.com/maahdima/mwp/api/dataservice/model"
	"golang.org/x/crypto/bcrypt"
	"gorm.io/gorm"
)

// openAdminSeedTestDB uses a uniquely-named in-memory DB per call -- the
// bare "file::memory:?cache=shared" DSN is a SHARED cache across every
// connection in the same process using that exact DSN string, which would
// make two calls in the same test binary silently reuse one underlying
// database instead of two independent ones (see newTestV2RaySyncService's
// identical precedent in service/v2ray_sync_test.go).
func openAdminSeedTestDB(t *testing.T) *gorm.DB {
	t.Helper()

	dsn := fmt.Sprintf("file:admin_seed_test_%d?mode=memory&cache=shared", time.Now().UnixNano())
	db, err := gorm.Open(sqlite.Open(dsn), &gorm.Config{})
	if err != nil {
		t.Fatalf("failed to open test db: %v", err)
	}
	if err := db.AutoMigrate(&model.Admin{}); err != nil {
		t.Fatalf("failed to migrate test db: %v", err)
	}
	return db
}

// TestAdminSeed_NoPasswordConfiguredGeneratesRandomPassword is the
// regression test for a confirmed finding from an authorized penetration
// test: this codebase used to fall back to the fixed, well-known
// credentials "mwpadmin"/"mwpadmin" whenever ADMIN_PASSWORD wasn't set --
// live-verified to successfully authenticate against a fresh install's
// license-exempt /api/license/activate endpoint. With no ADMIN_PASSWORD
// in the environment, the seeded admin's password must NOT be the old
// hardcoded default, and it must vary from run to run (proving it's
// actually random, not just a different fixed string).
func TestAdminSeed_NoPasswordConfiguredGeneratesRandomPassword(t *testing.T) {
	t.Setenv("ADMIN_USERNAME", "")
	t.Setenv("ADMIN_PASSWORD", "")

	db1 := openAdminSeedTestDB(t)
	if err := AdminSeed(db1); err != nil {
		t.Fatalf("AdminSeed failed: %v", err)
	}
	var admin1 model.Admin
	if err := db1.First(&admin1).Error; err != nil {
		t.Fatalf("failed to load seeded admin: %v", err)
	}

	if err := bcrypt.CompareHashAndPassword([]byte(admin1.Password), []byte("mwpadmin")); err == nil {
		t.Fatalf("seeded admin password must NOT be the old hardcoded default \"mwpadmin\"")
	}

	// Confirm a SECOND, independent seed (fresh DB, same unset env) produces
	// a DIFFERENT stored hash -- proving the generated password is actually
	// random per-install, not a different-but-still-fixed replacement
	// constant.
	db2 := openAdminSeedTestDB(t)
	if err := AdminSeed(db2); err != nil {
		t.Fatalf("AdminSeed (second db) failed: %v", err)
	}
	var admin2 model.Admin
	if err := db2.First(&admin2).Error; err != nil {
		t.Fatalf("failed to load second seeded admin: %v", err)
	}

	if admin1.Password == admin2.Password {
		t.Fatalf("two independent AdminSeed runs with no ADMIN_PASSWORD produced the IDENTICAL bcrypt hash -- password generation is not actually random")
	}
}

// TestAdminSeed_ExplicitPasswordIsHonored confirms the existing, unchanged
// behavior: an operator who DOES set ADMIN_PASSWORD gets exactly that
// password, not a generated one.
func TestAdminSeed_ExplicitPasswordIsHonored(t *testing.T) {
	t.Setenv("ADMIN_USERNAME", "customadmin")
	t.Setenv("ADMIN_PASSWORD", "my-explicit-password")

	db := openAdminSeedTestDB(t)
	if err := AdminSeed(db); err != nil {
		t.Fatalf("AdminSeed failed: %v", err)
	}

	var admin model.Admin
	if err := db.First(&admin).Error; err != nil {
		t.Fatalf("failed to load seeded admin: %v", err)
	}
	if admin.Username != "customadmin" {
		t.Fatalf("expected username customadmin, got %q", admin.Username)
	}
	if err := bcrypt.CompareHashAndPassword([]byte(admin.Password), []byte("my-explicit-password")); err != nil {
		t.Fatalf("expected the explicitly-configured password to be stored, comparison failed: %v", err)
	}
}

// TestAdminSeed_TooShortPasswordIsRejectedAndReplaced is the regression
// test for a confirmed gap: an operator who sets ADMIN_PASSWORD to
// something shorter than the panel's own login form will accept (see
// loginRequestSchema's min(7) in ui/src/schema/authentication.ts) used to
// get that password seeded and hashed with no complaint -- locking
// themselves out of their own fresh install, since the login FORM itself
// refuses to even submit a password that short (the request never reaches
// the backend to produce an error explaining why). A too-short configured
// password must be treated exactly like an unset one: rejected in favor of
// a safe, generated, actually-usable one.
func TestAdminSeed_TooShortPasswordIsRejectedAndReplaced(t *testing.T) {
	t.Setenv("ADMIN_USERNAME", "")
	t.Setenv("ADMIN_PASSWORD", "admin") // 5 characters -- below the login form's min(7)

	db := openAdminSeedTestDB(t)
	if err := AdminSeed(db); err != nil {
		t.Fatalf("AdminSeed failed: %v", err)
	}

	var admin model.Admin
	if err := db.First(&admin).Error; err != nil {
		t.Fatalf("failed to load seeded admin: %v", err)
	}

	if err := bcrypt.CompareHashAndPassword([]byte(admin.Password), []byte("admin")); err == nil {
		t.Fatalf("the too-short configured password must NOT have been stored -- it must be rejected and replaced")
	}
}

// TestAdminSeed_SkipsWhenAdminAlreadyExists confirms the existing,
// unchanged safety behavior: AdminSeed never touches an already-seeded
// install, regardless of ADMIN_PASSWORD's current value -- an admin who
// already changed their password must never have it silently reset.
func TestAdminSeed_SkipsWhenAdminAlreadyExists(t *testing.T) {
	t.Setenv("ADMIN_USERNAME", "")
	t.Setenv("ADMIN_PASSWORD", "")

	db := openAdminSeedTestDB(t)
	existingHash, err := bcrypt.GenerateFromPassword([]byte("already-changed-password"), bcrypt.DefaultCost)
	if err != nil {
		t.Fatalf("test setup: failed to hash password: %v", err)
	}
	if err := db.Create(&model.Admin{Username: "existing", Password: string(existingHash)}).Error; err != nil {
		t.Fatalf("test setup: failed to seed existing admin: %v", err)
	}

	if err := AdminSeed(db); err != nil {
		t.Fatalf("AdminSeed failed: %v", err)
	}

	var count int64
	if err := db.Model(&model.Admin{}).Count(&count).Error; err != nil {
		t.Fatalf("failed to count admins: %v", err)
	}
	if count != 1 {
		t.Fatalf("expected AdminSeed to skip entirely with an existing admin present, but admin count is now %d", count)
	}
}
