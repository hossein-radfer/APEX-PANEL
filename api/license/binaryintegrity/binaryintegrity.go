// Package binaryintegrity implements a lightweight self-check that the
// currently-running mwp executable matches the signature its own release
// build produced -- intended to catch accidental substitution (e.g. an
// old binary left over from a botched deploy, or a corrupted copy), not
// to serve as a hardened anti-tamper system.
package binaryintegrity

import (
	"crypto/ed25519"
	"crypto/sha256"
	"encoding/base64"
	"errors"
	"fmt"
	"io"
	"os"
)

// SignatureSuffix is appended to the executable's own path to find its
// signature file -- e.g. /usr/local/bin/mwp -> /usr/local/bin/mwp.sig.
// Kept as a plain sibling file (not embedded in the binary itself) so the
// signing step can run AFTER the build, against the exact final bytes that
// will ship, without needing a second build pass to embed the result of
// hashing the first.
const SignatureSuffix = ".sig"

// hashFile streams path through SHA-256 without loading the whole
// (30+MB) binary into memory at once.
func hashFile(path string) ([]byte, error) {
	f, err := os.Open(path)
	if err != nil {
		return nil, err
	}
	defer f.Close()

	h := sha256.New()
	if _, err := io.Copy(h, f); err != nil {
		return nil, err
	}
	return h.Sum(nil), nil
}

// Sign computes binaryPath's SHA-256 and signs it with privateKey, writing
// the base64-encoded signature to binaryPath+SignatureSuffix. Used only by
// the standalone cmd/signbinary release tool -- never called by mwp itself
// (the running binary only ever verifies, it never holds the private key).
func Sign(binaryPath string, privateKey ed25519.PrivateKey) error {
	digest, err := hashFile(binaryPath)
	if err != nil {
		return fmt.Errorf("hashing binary: %w", err)
	}
	sig := ed25519.Sign(privateKey, digest)
	sigB64 := base64.StdEncoding.EncodeToString(sig)
	return os.WriteFile(binaryPath+SignatureSuffix, []byte(sigB64), 0o644)
}

// ErrNotSigned is returned by Verify when no .sig file exists next to the
// binary -- deliberately distinguished from a signature MISMATCH: an
// install built before this feature existed, or one where the operator
// simply never ran cmd/signbinary, is not "tampered", it's "not signed
// yet". Callers should treat this as a soft, backward-compatible no-op
// (log at most), not a lockout condition -- only ErrMismatch represents an
// actual integrity failure.
var ErrNotSigned = errors.New("no signature file found for this binary")

// ErrMismatch is returned by Verify when a .sig file exists but does not
// verify against the binary's current contents -- the actual "this
// executable was substituted/modified after it was signed" signal.
var ErrMismatch = errors.New("binary signature does not match its current contents")

// Verify re-hashes binaryPath and checks the base64 signature stored in
// binaryPath+SignatureSuffix against publicKey. Returns ErrNotSigned (soft)
// or ErrMismatch (the real tamper signal) as sentinel errors so callers can
// tell the two apart without string matching; any other error is a plain
// I/O/parsing failure (e.g. corrupt .sig file, unreadable binary).
func Verify(binaryPath string, publicKey ed25519.PublicKey) error {
	sigPath := binaryPath + SignatureSuffix
	sigB64, err := os.ReadFile(sigPath)
	if err != nil {
		if os.IsNotExist(err) {
			return ErrNotSigned
		}
		return fmt.Errorf("reading signature file: %w", err)
	}

	sig, err := base64.StdEncoding.DecodeString(string(sigB64))
	if err != nil {
		return fmt.Errorf("signature file is not valid base64: %w", err)
	}

	digest, err := hashFile(binaryPath)
	if err != nil {
		return fmt.Errorf("hashing binary: %w", err)
	}

	if !ed25519.Verify(publicKey, digest, sig) {
		return ErrMismatch
	}
	return nil
}
