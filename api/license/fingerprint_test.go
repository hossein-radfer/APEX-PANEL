package license

import (
	"os"
	"path/filepath"
	"testing"

	"github.com/google/uuid"
)

// TestEnsureInstallID_GeneratesValidUUIDOnFirstRun confirms the baseline,
// unchanged behavior: a fresh dataDirPath with no install-id file gets a
// freshly generated, well-formed UUID written to it.
func TestEnsureInstallID_GeneratesValidUUIDOnFirstRun(t *testing.T) {
	dir := t.TempDir()

	id, err := EnsureInstallID(dir)
	if err != nil {
		t.Fatalf("EnsureInstallID failed: %v", err)
	}
	if _, parseErr := uuid.Parse(id); parseErr != nil {
		t.Fatalf("expected a valid UUID, got %q: %v", id, parseErr)
	}

	data, readErr := os.ReadFile(filepath.Join(dir, installIDFilename))
	if readErr != nil {
		t.Fatalf("expected install-id file to be written: %v", readErr)
	}
	if string(data) != id {
		t.Fatalf("expected file content to match returned id, got %q vs %q", data, id)
	}
}

// TestEnsureInstallID_ReusesExistingValidUUID confirms a second call
// against the same dataDirPath returns the SAME id, not a new one --
// this is the entire point of persisting it.
func TestEnsureInstallID_ReusesExistingValidUUID(t *testing.T) {
	dir := t.TempDir()

	first, err := EnsureInstallID(dir)
	if err != nil {
		t.Fatalf("first EnsureInstallID failed: %v", err)
	}
	second, err := EnsureInstallID(dir)
	if err != nil {
		t.Fatalf("second EnsureInstallID failed: %v", err)
	}
	if first != second {
		t.Fatalf("expected the same id across calls, got %q then %q", first, second)
	}
}

// TestEnsureInstallID_RegeneratesOnMalformedContent verifies a non-UUID
// install-id file is treated as corrupt and regenerated, same as the
// empty-file case.
func TestEnsureInstallID_RegeneratesOnMalformedContent(t *testing.T) {
	dir := t.TempDir()
	path := filepath.Join(dir, installIDFilename)

	const malformedContent = "not-a-valid-uuid"
	if err := os.WriteFile(path, []byte(malformedContent), 0o600); err != nil {
		t.Fatalf("failed to seed malformed file: %v", err)
	}

	id, err := EnsureInstallID(dir)
	if err != nil {
		t.Fatalf("EnsureInstallID failed: %v", err)
	}
	if _, parseErr := uuid.Parse(id); parseErr != nil {
		t.Fatalf("expected a freshly generated valid UUID after malformed content, got %q: %v", id, parseErr)
	}

	data, readErr := os.ReadFile(path)
	if readErr != nil {
		t.Fatalf("failed to reread file: %v", readErr)
	}
	if string(data) == malformedContent {
		t.Fatal("expected the malformed content to be overwritten, but it was left unchanged")
	}
}

// TestEnsureInstallID_RegeneratesOnEmptyContent confirms the pre-existing
// empty-file behavior still works unchanged after adding UUID validation.
func TestEnsureInstallID_RegeneratesOnEmptyContent(t *testing.T) {
	dir := t.TempDir()
	path := filepath.Join(dir, installIDFilename)

	if err := os.WriteFile(path, []byte(""), 0o600); err != nil {
		t.Fatalf("failed to seed empty file: %v", err)
	}

	id, err := EnsureInstallID(dir)
	if err != nil {
		t.Fatalf("EnsureInstallID failed: %v", err)
	}
	if _, parseErr := uuid.Parse(id); parseErr != nil {
		t.Fatalf("expected a freshly generated valid UUID after empty content, got %q: %v", id, parseErr)
	}
}

// TestFingerprint_ChangesWhenInstallIDDeleted documents the residual,
// expected behavior after this hardening: deleting the install-id file
// (the actual pentest-confirmed abuse primitive) STILL produces a
// different fingerprint -- this client-side check only closes the
// separate "forge the file's content directly" gap, not the "delete it
// and let it regenerate" gap. The real fix for that is server-side (see
// license-panel's IssueSelfTrial IP-cooldown check) -- this test exists
// so that fact stays explicit and isn't mistaken for something this
// client-side change was ever meant to solve.
func TestFingerprint_ChangesWhenInstallIDDeleted(t *testing.T) {
	dir := t.TempDir()

	first := Fingerprint(dir)

	if err := os.Remove(filepath.Join(dir, installIDFilename)); err != nil {
		t.Fatalf("failed to remove install-id: %v", err)
	}

	second := Fingerprint(dir)

	if first == second {
		t.Fatal("expected the fingerprint to change after deleting install-id (this is the known, server-side-mitigated gap, not something this test expects to be fixed client-side)")
	}
}
