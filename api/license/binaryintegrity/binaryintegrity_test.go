package binaryintegrity

import (
	"crypto/ed25519"
	"errors"
	"os"
	"path/filepath"
	"testing"
)

func writeTestBinary(t *testing.T, dir string, content []byte) string {
	t.Helper()
	path := filepath.Join(dir, "fake-binary")
	if err := os.WriteFile(path, content, 0o755); err != nil {
		t.Fatalf("failed to write test binary: %v", err)
	}
	return path
}

// TestSignAndVerify_RoundTrip confirms a freshly signed, unmodified binary
// verifies successfully.
func TestSignAndVerify_RoundTrip(t *testing.T) {
	dir := t.TempDir()
	binPath := writeTestBinary(t, dir, []byte("this is a fake mwp binary, version 1"))

	pub, priv, err := ed25519.GenerateKey(nil)
	if err != nil {
		t.Fatalf("failed to generate keypair: %v", err)
	}

	if err := Sign(binPath, priv); err != nil {
		t.Fatalf("failed to sign: %v", err)
	}

	if err := Verify(binPath, pub); err != nil {
		t.Fatalf("expected verification to succeed for an unmodified signed binary, got: %v", err)
	}
}

// TestVerify_NoSignatureFileReturnsErrNotSigned confirms an unsigned binary
// (no .sig file at all) is distinguished from a genuine mismatch -- see
// ErrNotSigned's own doc comment for why callers must treat these
// differently (an unsigned/legacy install is not "tampered").
func TestVerify_NoSignatureFileReturnsErrNotSigned(t *testing.T) {
	dir := t.TempDir()
	binPath := writeTestBinary(t, dir, []byte("never signed"))

	pub, _, _ := ed25519.GenerateKey(nil)

	err := Verify(binPath, pub)
	if !errors.Is(err, ErrNotSigned) {
		t.Fatalf("expected ErrNotSigned, got %v", err)
	}
}

// TestVerify_ModifiedBinaryReturnsErrMismatch is the core regression test
// for the actual tamper-detection purpose of this package: a binary
// modified AFTER being signed must fail verification with the specific
// ErrMismatch sentinel.
func TestVerify_ModifiedBinaryReturnsErrMismatch(t *testing.T) {
	dir := t.TempDir()
	binPath := writeTestBinary(t, dir, []byte("original, trusted contents"))

	pub, priv, _ := ed25519.GenerateKey(nil)
	if err := Sign(binPath, priv); err != nil {
		t.Fatalf("failed to sign: %v", err)
	}

	// Simulate substitution: overwrite the binary's contents after signing,
	// the .sig file is left as-is (exactly what a naive "swap the exe"
	// tamper would look like).
	if err := os.WriteFile(binPath, []byte("modified/substituted contents"), 0o755); err != nil {
		t.Fatalf("failed to overwrite binary: %v", err)
	}

	err := Verify(binPath, pub)
	if !errors.Is(err, ErrMismatch) {
		t.Fatalf("expected ErrMismatch for a modified binary, got %v", err)
	}
}

// TestVerify_WrongPublicKeyReturnsErrMismatch confirms a signature made
// with a DIFFERENT private key (e.g. someone re-signing a tampered binary
// with their own key, not knowing the real release key) also fails as a
// mismatch, not a false "valid" -- this is what actually makes the check
// meaningful against the "simple/accidental" substitution case: a replaced
// binary won't carry a signature this specific trusted public key accepts.
func TestVerify_WrongPublicKeyReturnsErrMismatch(t *testing.T) {
	dir := t.TempDir()
	binPath := writeTestBinary(t, dir, []byte("some binary"))

	_, attackerPriv, _ := ed25519.GenerateKey(nil)
	if err := Sign(binPath, attackerPriv); err != nil {
		t.Fatalf("failed to sign with attacker key: %v", err)
	}

	trustedPub, _, _ := ed25519.GenerateKey(nil)

	err := Verify(binPath, trustedPub)
	if !errors.Is(err, ErrMismatch) {
		t.Fatalf("expected ErrMismatch when verifying against a public key that didn't sign it, got %v", err)
	}
}
