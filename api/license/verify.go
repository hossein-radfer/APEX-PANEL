package license

import (
	"crypto/ed25519"
	"encoding/base64"
	"encoding/json"
	"fmt"
)

// VerifySignedResponse checks that raw (the exact JSON bytes
// license-panel sent) carries a valid Ed25519 signature from the trusted
// public key, then decodes the payload. This is the check that makes a
// "valid: true" response untrustworthy unless it was actually produced
// by license-panel's private key -- HTTPS alone only protects against a
// passive eavesdropper, not a compromised proxy sitting in front of this
// server.
func VerifySignedResponse(publicKeyB64 string, raw []byte) (*SignedValidateResponse, error) {
	pubKey, err := decodePublicKey(publicKeyB64)
	if err != nil {
		return nil, fmt.Errorf("invalid trusted public key configured: %w", err)
	}

	var signed SignedValidateResponse
	if err := json.Unmarshal(raw, &signed); err != nil {
		return nil, fmt.Errorf("failed to parse license server response: %w", err)
	}

	// Re-marshal Payload to get the exact canonical bytes the server
	// signed -- json.Unmarshal followed by json.Marshal of the same Go
	// struct produces byte-identical output to the original as long as
	// field order in the struct definition matches (which it must, per
	// the doc comment on ValidateResponse).
	payloadBytes, err := json.Marshal(signed.Payload)
	if err != nil {
		return nil, fmt.Errorf("failed to re-serialize payload for verification: %w", err)
	}

	sig, err := base64.StdEncoding.DecodeString(signed.Signature)
	if err != nil {
		return nil, fmt.Errorf("invalid signature encoding: %w", err)
	}

	if !ed25519.Verify(pubKey, payloadBytes, sig) {
		return nil, fmt.Errorf("signature verification failed -- response did not come from the trusted license server")
	}

	return &signed, nil
}

// VerifySignedCapabilityResponse mirrors VerifySignedResponse's signature
// check, and additionally requires the payload's embedded Nonce to equal
// expectedNonce (the value requested via a fresh NonceRequest), so a
// previously-captured signed response cannot satisfy a new check.
func VerifySignedCapabilityResponse(publicKeyB64 string, raw []byte, expectedNonce string) (*SignedCapabilityValidateResponse, error) {
	pubKey, err := decodePublicKey(publicKeyB64)
	if err != nil {
		return nil, fmt.Errorf("invalid trusted public key configured: %w", err)
	}

	var signed SignedCapabilityValidateResponse
	if err := json.Unmarshal(raw, &signed); err != nil {
		return nil, fmt.Errorf("failed to parse license server response: %w", err)
	}

	payloadBytes, err := json.Marshal(signed.Payload)
	if err != nil {
		return nil, fmt.Errorf("failed to re-serialize payload for verification: %w", err)
	}

	sig, err := base64.StdEncoding.DecodeString(signed.Signature)
	if err != nil {
		return nil, fmt.Errorf("invalid signature encoding: %w", err)
	}

	if !ed25519.Verify(pubKey, payloadBytes, sig) {
		return nil, fmt.Errorf("signature verification failed -- response did not come from the trusted license server")
	}

	// The actual anti-replay check: a syntactically valid, correctly
	// signed response for a DIFFERENT nonce (e.g. an old one captured
	// earlier) must never be accepted here.
	if signed.Payload.Nonce != expectedNonce {
		return nil, fmt.Errorf("nonce mismatch -- response does not match the challenge this request issued (possible replay)")
	}

	return &signed, nil
}

func decodePublicKey(b64 string) (ed25519.PublicKey, error) {
	decoded, err := base64.StdEncoding.DecodeString(b64)
	if err != nil {
		return nil, err
	}
	if len(decoded) != ed25519.PublicKeySize {
		return nil, fmt.Errorf("public key has wrong length: got %d, want %d", len(decoded), ed25519.PublicKeySize)
	}
	return ed25519.PublicKey(decoded), nil
}
