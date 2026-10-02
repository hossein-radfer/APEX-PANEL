package service

import (
	"errors"
	"fmt"
	"testing"
	"time"

	"github.com/glebarez/sqlite"
	"gorm.io/gorm"

	"github.com/maahdima/mwp/api/dataservice/model"
)

func openApiKeyTestDB(t *testing.T) *gorm.DB {
	t.Helper()

	dsn := fmt.Sprintf("file:apikey_%d?mode=memory&cache=shared", time.Now().UnixNano())
	db, err := gorm.Open(sqlite.Open(dsn), &gorm.Config{})
	if err != nil {
		t.Fatalf("failed to open test db: %v", err)
	}

	if err := db.AutoMigrate(&model.ApiKey{}); err != nil {
		t.Fatalf("failed to migrate test db: %v", err)
	}

	return db
}

func TestApiKeyService_GenerateAndVerify(t *testing.T) {
	db := openApiKeyTestDB(t)
	svc := NewApiKeyService(db)

	raw, id, err := svc.Generate("Iran panel sync")
	if err != nil {
		t.Fatalf("Generate failed: %v", err)
	}
	if id == 0 {
		t.Fatal("expected a non-zero key id")
	}
	if len(raw) != apiKeyRawLength {
		t.Fatalf("expected raw key of length %d, got %d", apiKeyRawLength, len(raw))
	}

	if err := svc.Verify(raw); err != nil {
		t.Fatalf("Verify should accept the freshly generated key, got: %v", err)
	}
}

func TestApiKeyService_VerifyRejectsWrongKey(t *testing.T) {
	db := openApiKeyTestDB(t)
	svc := NewApiKeyService(db)

	if _, _, err := svc.Generate("some integration"); err != nil {
		t.Fatalf("Generate failed: %v", err)
	}

	if err := svc.Verify("totally-wrong-key-that-was-never-issued-by-this-panel"); !errors.Is(err, ErrApiKeyInvalid) {
		t.Fatalf("expected ErrApiKeyInvalid for a wrong key, got: %v", err)
	}
}

func TestApiKeyService_RevokedKeyIsRejected(t *testing.T) {
	db := openApiKeyTestDB(t)
	svc := NewApiKeyService(db)

	raw, id, err := svc.Generate("billing automation")
	if err != nil {
		t.Fatalf("Generate failed: %v", err)
	}

	if err := svc.Verify(raw); err != nil {
		t.Fatalf("expected key to be valid before revocation, got: %v", err)
	}

	if err := svc.Revoke(id); err != nil {
		t.Fatalf("Revoke failed: %v", err)
	}

	if err := svc.Verify(raw); !errors.Is(err, ErrApiKeyInvalid) {
		t.Fatalf("expected ErrApiKeyInvalid after revocation, got: %v", err)
	}
}

func TestApiKeyService_VerifyBumpsLastUsedAt(t *testing.T) {
	db := openApiKeyTestDB(t)
	svc := NewApiKeyService(db)

	raw, id, err := svc.Generate("usage tracked key")
	if err != nil {
		t.Fatalf("Generate failed: %v", err)
	}

	keys, err := svc.List()
	if err != nil {
		t.Fatalf("List failed: %v", err)
	}
	for _, k := range keys {
		if k.ID == id && k.LastUsedAt != nil {
			t.Fatal("expected LastUsedAt to be nil before first use")
		}
	}

	if err := svc.Verify(raw); err != nil {
		t.Fatalf("Verify failed: %v", err)
	}

	keys, err = svc.List()
	if err != nil {
		t.Fatalf("List failed: %v", err)
	}
	found := false
	for _, k := range keys {
		if k.ID == id {
			found = true
			if k.LastUsedAt == nil {
				t.Fatal("expected LastUsedAt to be set after a successful Verify")
			}
		}
	}
	if !found {
		t.Fatal("expected to find the generated key in List")
	}
}

func TestApiKeyService_ListOrdersNewestFirst(t *testing.T) {
	db := openApiKeyTestDB(t)
	svc := NewApiKeyService(db)

	if _, _, err := svc.Generate("first"); err != nil {
		t.Fatalf("Generate failed: %v", err)
	}
	// model.Model.CreatedAt has whole-second resolution (time.Now().Unix()),
	// so ordering by it needs a full second of separation to be observable.
	time.Sleep(1100 * time.Millisecond)
	if _, _, err := svc.Generate("second"); err != nil {
		t.Fatalf("Generate failed: %v", err)
	}

	keys, err := svc.List()
	if err != nil {
		t.Fatalf("List failed: %v", err)
	}
	if len(keys) != 2 {
		t.Fatalf("expected 2 keys, got %d", len(keys))
	}
	if keys[0].Label != "second" {
		t.Fatalf("expected newest key ('second') first, got %q", keys[0].Label)
	}
}
