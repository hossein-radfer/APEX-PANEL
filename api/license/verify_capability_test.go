package license

import (
	"crypto/ed25519"
	"crypto/rand"
	"encoding/base64"
	"encoding/json"
	"testing"
)

func newTestKeypair(t *testing.T) (pub ed25519.PublicKey, priv ed25519.PrivateKey, pubB64 string) {
	t.Helper()
	pub, priv, err := ed25519.GenerateKey(rand.Reader)
	if err != nil {
		t.Fatalf("failed to generate test keypair: %v", err)
	}
	return pub, priv, base64.StdEncoding.EncodeToString(pub)
}

func signCapabilityPayload(t *testing.T, priv ed25519.PrivateKey, payload CapabilityValidateResponse) []byte {
	t.Helper()
	payloadBytes, err := json.Marshal(payload)
	if err != nil {
		t.Fatalf("failed to marshal payload: %v", err)
	}
	sig := ed25519.Sign(priv, payloadBytes)
	signed := SignedCapabilityValidateResponse{
		Payload:   payload,
		Signature: base64.StdEncoding.EncodeToString(sig),
	}
	raw, err := json.Marshal(signed)
	if err != nil {
		t.Fatalf("failed to marshal signed response: %v", err)
	}
	return raw
}

// TestVerifySignedCapabilityResponse_AcceptsCorrectNonce confirms the
// happy path: a correctly signed response whose embedded Nonce matches
// what the caller expected is accepted.
func TestVerifySignedCapabilityResponse_AcceptsCorrectNonce(t *testing.T) {
	_, priv, pubB64 := newTestKeypair(t)

	payload := CapabilityValidateResponse{
		Valid:         true,
		LicenseKey:    "MWP-TEST-0000-0000-0000",
		CapabilityKey: "raise_user_cap",
		Nonce:         "nonce-abc-123",
		CheckedAt:     "2026-01-01T00:00:00Z",
	}
	raw := signCapabilityPayload(t, priv, payload)

	signed, err := VerifySignedCapabilityResponse(pubB64, raw, "nonce-abc-123")
	if err != nil {
		t.Fatalf("expected verification to succeed, got: %v", err)
	}
	if !signed.Payload.Valid {
		t.Fatal("expected Payload.Valid to be true")
	}
}

// TestVerifySignedCapabilityResponse_RejectsNonceMismatch is the core
// regression test for the entire mechanism: a syntactically valid,
// correctly-signed response for a DIFFERENT nonce than what the caller
// asked for (e.g. an old response captured from an earlier, legitimate
// exchange) must be rejected -- this is what makes the nonce a genuine
// anti-replay measure rather than just another field in the payload.
func TestVerifySignedCapabilityResponse_RejectsNonceMismatch(t *testing.T) {
	_, priv, pubB64 := newTestKeypair(t)

	payload := CapabilityValidateResponse{
		Valid:         true,
		LicenseKey:    "MWP-TEST-0000-0000-0000",
		CapabilityKey: "raise_user_cap",
		Nonce:         "old-nonce-captured-earlier",
		CheckedAt:     "2026-01-01T00:00:00Z",
	}
	raw := signCapabilityPayload(t, priv, payload)

	// The caller in this test asked for a DIFFERENT (newer) nonce -- this
	// is exactly the replay scenario: presenting an old, validly-signed
	// response to satisfy a new challenge.
	_, err := VerifySignedCapabilityResponse(pubB64, raw, "brand-new-nonce")
	if err == nil {
		t.Fatal("expected verification to fail on nonce mismatch (this is the replay-prevention check)")
	}
}

// TestVerifySignedCapabilityResponse_RejectsBadSignature confirms a
// tampered payload (signature doesn't match) is rejected regardless of
// the nonce matching.
func TestVerifySignedCapabilityResponse_RejectsBadSignature(t *testing.T) {
	_, priv, _ := newTestKeypair(t)
	_, _, wrongPubB64 := newTestKeypair(t) // a DIFFERENT keypair's public key

	payload := CapabilityValidateResponse{
		Valid:         true,
		LicenseKey:    "MWP-TEST-0000-0000-0000",
		CapabilityKey: "raise_user_cap",
		Nonce:         "nonce-abc-123",
		CheckedAt:     "2026-01-01T00:00:00Z",
	}
	raw := signCapabilityPayload(t, priv, payload)

	// Verifying against the WRONG public key must fail even though the
	// nonce matches perfectly.
	_, err := VerifySignedCapabilityResponse(wrongPubB64, raw, "nonce-abc-123")
	if err == nil {
		t.Fatal("expected verification to fail against the wrong public key")
	}
}
