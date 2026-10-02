package service

import (
	"crypto/ed25519"
	"encoding/base64"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"testing"

	"go.uber.org/zap"

	"github.com/maahdima/mwp/api/config"
	"github.com/maahdima/mwp/api/license"
)

// newTestLicenseServiceForCapability wires a bare LicenseService plus a
// real Ed25519 keypair so signature verification is exercised for real,
// not mocked away. The caller is responsible for standing up its own
// stub HTTP server and pointing svc.client at it (each test's handler
// needs to close over the returned private key to sign responses, so a
// single shared server can't be built here first).
func newTestLicenseServiceForCapability(t *testing.T) (svc *LicenseService, priv ed25519.PrivateKey, pubB64 string) {
	t.Helper()

	pub, priv, err := ed25519.GenerateKey(nil)
	if err != nil {
		t.Fatalf("failed to generate test keypair: %v", err)
	}
	pubB64 = base64.StdEncoding.EncodeToString(pub)

	db := newTestLicenseDB(t)

	svc = &LicenseService{
		db:  db,
		cfg: config.LicenseConfig{PublicKeyB64: pubB64},
	}
	svc.logger = zap.NewNop()

	if err := svc.storeLicenseKey("MWP-TEST-CAP-0000-0000"); err != nil {
		t.Fatalf("failed to store test license key: %v", err)
	}

	return svc, priv, pubB64
}

// signCapabilityResponseForTest signs a CapabilityValidateResponse the
// way license-panel's ValidateCapability would.
func signCapabilityResponseForTest(t *testing.T, priv ed25519.PrivateKey, payload license.CapabilityValidateResponse) []byte {
	t.Helper()
	payloadBytes, err := json.Marshal(payload)
	if err != nil {
		t.Fatalf("failed to marshal payload: %v", err)
	}
	sig := ed25519.Sign(priv, payloadBytes)
	signed := license.SignedCapabilityValidateResponse{
		Payload:   payload,
		Signature: base64.StdEncoding.EncodeToString(sig),
	}
	raw, err := json.Marshal(signed)
	if err != nil {
		t.Fatalf("failed to marshal signed response: %v", err)
	}
	return raw
}

// TestRequestCapabilityProof_HappyPath confirms the full two-step round
// trip succeeds end-to-end against a stub server that behaves exactly
// like license-panel's real nonce/validate-capability endpoints.
func TestRequestCapabilityProof_HappyPath(t *testing.T) {
	const testNonce = "server-issued-nonce-xyz"
	var priv ed25519.PrivateKey

	svc, generatedPriv, _ := newTestLicenseServiceForCapability(t)
	priv = generatedPriv

	handler := http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		switch r.URL.Path {
		case "/api/v1/license/nonce":
			_ = json.NewEncoder(w).Encode(license.NonceResponse{Nonce: testNonce, ExpiresAt: "2026-01-01T00:02:00Z"})
		case "/api/v1/license/validate-capability":
			var req license.CapabilityValidateRequest
			_ = json.NewDecoder(r.Body).Decode(&req)
			payload := license.CapabilityValidateResponse{
				Valid:         true,
				LicenseKey:    req.LicenseKey,
				CapabilityKey: req.CapabilityKey,
				Nonce:         req.Nonce, // echoes back the SAME nonce it received
				CheckedAt:     "2026-01-01T00:00:30Z",
			}
			w.Write(signCapabilityResponseForTest(t, priv, payload))
		default:
			w.WriteHeader(http.StatusNotFound)
		}
	})

	server := httptest.NewServer(handler)
	defer server.Close()
	svc.client = license.NewClient(server.URL)

	proof, err := svc.RequestCapabilityProof("raise_user_cap")
	if err != nil {
		t.Fatalf("expected RequestCapabilityProof to succeed, got: %v", err)
	}
	if proof.CapabilityKey != "raise_user_cap" {
		t.Fatalf("expected capability key to be echoed back, got %q", proof.CapabilityKey)
	}
}

// TestRequestCapabilityProof_RejectsServerDenial confirms a
// Valid=false response (e.g. license expired) is surfaced as an error,
// not silently treated as granted.
func TestRequestCapabilityProof_RejectsServerDenial(t *testing.T) {
	var priv ed25519.PrivateKey
	svc, generatedPriv, _ := newTestLicenseServiceForCapability(t)
	priv = generatedPriv

	handler := http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		switch r.URL.Path {
		case "/api/v1/license/nonce":
			_ = json.NewEncoder(w).Encode(license.NonceResponse{Nonce: "n1", ExpiresAt: "2026-01-01T00:02:00Z"})
		case "/api/v1/license/validate-capability":
			var req license.CapabilityValidateRequest
			_ = json.NewDecoder(r.Body).Decode(&req)
			payload := license.CapabilityValidateResponse{
				Valid:         false,
				Reason:        "license expired",
				LicenseKey:    req.LicenseKey,
				CapabilityKey: req.CapabilityKey,
				Nonce:         req.Nonce,
				CheckedAt:     "2026-01-01T00:00:30Z",
			}
			w.Write(signCapabilityResponseForTest(t, priv, payload))
		}
	})
	server := httptest.NewServer(handler)
	defer server.Close()
	svc.client = license.NewClient(server.URL)

	_, err := svc.RequestCapabilityProof("raise_user_cap")
	if err == nil {
		t.Fatal("expected an error when the license server denies the capability")
	}
}

// TestRequestCapabilityProof_RejectsServerReplayAttempt simulates a
// malicious/misbehaving server that signs a response for a DIFFERENT
// nonce than the one it just issued -- proving the client-side check
// (not just the server-side one already tested in license-panel) also
// enforces the nonce match independently.
func TestRequestCapabilityProof_RejectsServerReplayAttempt(t *testing.T) {
	var priv ed25519.PrivateKey
	svc, generatedPriv, _ := newTestLicenseServiceForCapability(t)
	priv = generatedPriv

	handler := http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		switch r.URL.Path {
		case "/api/v1/license/nonce":
			_ = json.NewEncoder(w).Encode(license.NonceResponse{Nonce: "the-real-current-nonce", ExpiresAt: "2026-01-01T00:02:00Z"})
		case "/api/v1/license/validate-capability":
			var req license.CapabilityValidateRequest
			_ = json.NewDecoder(r.Body).Decode(&req)
			// Deliberately sign a response for a STALE nonce, not the one
			// actually requested -- simulates a captured/replayed response.
			payload := license.CapabilityValidateResponse{
				Valid:         true,
				LicenseKey:    req.LicenseKey,
				CapabilityKey: req.CapabilityKey,
				Nonce:         "an-old-stale-nonce-from-earlier",
				CheckedAt:     "2026-01-01T00:00:30Z",
			}
			w.Write(signCapabilityResponseForTest(t, priv, payload))
		}
	})
	server := httptest.NewServer(handler)
	defer server.Close()
	svc.client = license.NewClient(server.URL)

	_, err := svc.RequestCapabilityProof("raise_user_cap")
	if err == nil {
		t.Fatal("expected RequestCapabilityProof to reject a response whose nonce doesn't match what was requested")
	}
}

// TestRequestCapabilityProof_RequiresStoredLicenseKey confirms a fresh
// install with no activated license can't obtain a capability proof.
func TestRequestCapabilityProof_RequiresStoredLicenseKey(t *testing.T) {
	svc, _, _ := newTestLicenseServiceForCapability(t)
	if err := svc.clearStoredLicenseKey(); err != nil {
		t.Fatalf("failed to clear license key: %v", err)
	}

	_, err := svc.RequestCapabilityProof("raise_user_cap")
	if err == nil {
		t.Fatal("expected an error when no license key is configured")
	}
}
