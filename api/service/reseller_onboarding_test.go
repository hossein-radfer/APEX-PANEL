package service

import (
	"fmt"
	"testing"
	"time"

	"github.com/glebarez/sqlite"
	"gorm.io/gorm"

	"github.com/maahdima/mwp/api/dataservice/model"
)

func openResellerOnboardingTestDB(t *testing.T) *gorm.DB {
	t.Helper()

	dsn := fmt.Sprintf("file:reseller_onboarding_test_%d?mode=memory&cache=shared", time.Now().UnixNano())
	db, err := gorm.Open(sqlite.Open(dsn), &gorm.Config{})
	if err != nil {
		t.Fatalf("failed to open test db: %v", err)
	}
	if err := db.AutoMigrate(
		&model.Reseller{},
		&model.Peer{},
		&model.UserManagerAccount{},
		&model.V2RayPackage{},
		&model.Application{},
	); err != nil {
		t.Fatalf("failed to migrate test db: %v", err)
	}
	return db
}

// TestCompleteOnboarding_LatchesFlagToTrue confirms the happy path: a
// fresh reseller starts with HasCompletedOnboarding=false, and one call
// flips it to true both in the returned response and in the DB.
func TestCompleteOnboarding_LatchesFlagToTrue(t *testing.T) {
	db := openResellerOnboardingTestDB(t)
	svc := NewReseller(db, nil)

	reseller := model.Reseller{Name: "new-reseller", Username: "new-reseller"}
	if err := db.Create(&reseller).Error; err != nil {
		t.Fatalf("failed to create reseller: %v", err)
	}
	if reseller.HasCompletedOnboarding {
		t.Fatal("expected a freshly created reseller to default to HasCompletedOnboarding=false")
	}

	resp, err := svc.CompleteOnboarding(reseller.ID)
	if err != nil {
		t.Fatalf("CompleteOnboarding failed: %v", err)
	}
	if !resp.HasCompletedOnboarding {
		t.Fatal("expected response to report HasCompletedOnboarding=true")
	}

	var reloaded model.Reseller
	if err := db.First(&reloaded, reseller.ID).Error; err != nil {
		t.Fatalf("failed to reload reseller: %v", err)
	}
	if !reloaded.HasCompletedOnboarding {
		t.Fatal("expected the persisted row to have HasCompletedOnboarding=true")
	}
}

// TestCompleteOnboarding_IsIdempotent confirms calling it a second time
// on an already-completed reseller is a harmless no-op, not an error --
// per CompleteOnboarding's own doc comment.
func TestCompleteOnboarding_IsIdempotent(t *testing.T) {
	db := openResellerOnboardingTestDB(t)
	svc := NewReseller(db, nil)

	reseller := model.Reseller{Name: "twice", Username: "twice", HasCompletedOnboarding: true}
	if err := db.Create(&reseller).Error; err != nil {
		t.Fatalf("failed to create reseller: %v", err)
	}

	resp, err := svc.CompleteOnboarding(reseller.ID)
	if err != nil {
		t.Fatalf("expected no error calling CompleteOnboarding on an already-completed reseller, got %v", err)
	}
	if !resp.HasCompletedOnboarding {
		t.Fatal("expected response to still report HasCompletedOnboarding=true")
	}
}

// TestCompleteOnboarding_UnknownResellerReturnsError confirms a
// not-found reseller ID surfaces gorm.ErrRecordNotFound (via First),
// matching every other single-reseller lookup in this service.
func TestCompleteOnboarding_UnknownResellerReturnsError(t *testing.T) {
	db := openResellerOnboardingTestDB(t)
	svc := NewReseller(db, nil)

	if _, err := svc.CompleteOnboarding(999999); err == nil {
		t.Fatal("expected error completing onboarding for a nonexistent reseller")
	}
}
