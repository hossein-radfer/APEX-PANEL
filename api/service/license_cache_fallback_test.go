package service

import (
	"crypto/ed25519"
	"encoding/base64"
	"encoding/json"
	"fmt"
	"net/http"
	"net/http/httptest"
	"testing"
	"time"

	"github.com/glebarez/sqlite"
	"gorm.io/gorm"

	"github.com/maahdima/mwp/api/config"
	"github.com/maahdima/mwp/api/dataservice/model"
	"github.com/maahdima/mwp/api/license"
)

// newTestLicenseDB gives each test its own uniquely-named in-memory
// sqlite DB, mirroring v2ray_permission_test.go's newTestV2RayPackageService
// pattern -- avoids the shared-cache DSN collision class of flakiness.
func newTestLicenseDB(t *testing.T) *gorm.DB {
	t.Helper()

	dsn := fmt.Sprintf("file:license_test_%d?mode=memory&cache=shared", time.Now().UnixNano())
	db, err := gorm.Open(sqlite.Open(dsn), &gorm.Config{})
	if err != nil {
		t.Fatalf("failed to open test db: %v", err)
	}
	if err := db.AutoMigrate(&model.SystemConfig{}); err != nil {
		t.Fatalf("failed to migrate test db: %v", err)
	}
	return db
}

// signTestResponse builds and signs a ValidateResponse the same way
// license-panel would, using a freshly-generated Ed25519 keypair. Returns
// the raw signed JSON bytes and the base64 public key needed to verify
// them (what would normally be LICENSE_PUBLIC_KEY / the cached
// license_public_key SystemConfig row).
func signTestResponse(t *testing.T, payload license.ValidateResponse) (raw []byte, publicKeyB64 string, privateKey ed25519.PrivateKey) {
	t.Helper()

	pub, priv, err := ed25519.GenerateKey(nil)
	if err != nil {
		t.Fatalf("failed to generate test keypair: %v", err)
	}

	payloadBytes, err := json.Marshal(payload)
	if err != nil {
		t.Fatalf("failed to marshal payload: %v", err)
	}
	sig := ed25519.Sign(priv, payloadBytes)

	signed := license.SignedValidateResponse{
		Payload:   payload,
		Signature: base64.StdEncoding.EncodeToString(sig),
	}
	rawOut, err := json.Marshal(signed)
	if err != nil {
		t.Fatalf("failed to marshal signed response: %v", err)
	}

	return rawOut, base64.StdEncoding.EncodeToString(pub), priv
}

// newTestLicenseService builds a LicenseService pointed at a stub HTTP
// server (heartbeatHandler decides what every /heartbeat call returns) and
// pre-seeds SystemConfig with the given public key so signature
// verification succeeds without a real license-panel round trip.
func newTestLicenseService(t *testing.T, db *gorm.DB, publicKeyB64 string, heartbeatHandler http.HandlerFunc) *LicenseService {
	t.Helper()

	server := httptest.NewServer(heartbeatHandler)
	t.Cleanup(server.Close)

	svc := NewLicenseService(db, config.LicenseConfig{
		BaseURL:              server.URL,
		PublicKeyB64:         publicKeyB64,
		CheckIntervalSeconds: 3600,
	}, "production", t.TempDir())

	return svc
}

// TestHeartbeat_FingerprintMismatchFallsBackToCachedUnexpiredLicense is a
// regression test for the reported bug: a backup restored onto a
// different DATA_DIR (losing install-id) changes this install's
// fingerprint, so a live heartbeat comes back rejected even though the
// underlying license was never actually revoked or expired. Confirms
// that once a valid response has ever been cached, a later rejected
// heartbeat falls back to that cached response (if still unexpired) and
// keeps the panel unlocked instead of locking out immediately.
func TestHeartbeat_FingerprintMismatchFallsBackToCachedUnexpiredLicense(t *testing.T) {
	db := newTestLicenseDB(t)

	futureExpiry := time.Now().Add(30 * 24 * time.Hour).Format(time.RFC3339)
	validPayload := license.ValidateResponse{
		Valid: true, LicenseKey: "key-123", PlanName: "pro",
		ExpiresAt: futureExpiry, IssuedAt: time.Now().Format(time.RFC3339),
		ServerCount: 1, MaxServers: 5, CheckedAt: time.Now().Format(time.RFC3339),
	}

	callCount := 0
	var validRaw []byte
	var pubKeyB64 string
	var privKey ed25519.PrivateKey

	svc := newTestLicenseService(t, db, "", func(w http.ResponseWriter, r *http.Request) {
		callCount++
		w.Header().Set("Content-Type", "application/json")
		if callCount == 1 {
			// First call: succeeds, this is what gets cached.
			_, _ = w.Write(validRaw)
			return
		}
		// Second call onward: server rejects (simulates a fingerprint
		// it no longer recognizes, e.g. after DATA_DIR/install-id was
		// lost across a mismatched backup restore).
		rejected := license.ValidateResponse{
			Valid: false, Reason: "unrecognized fingerprint for this license key",
			LicenseKey: "key-123", CheckedAt: time.Now().Format(time.RFC3339),
		}
		payloadBytes, _ := json.Marshal(rejected)
		sig := ed25519.Sign(privKey, payloadBytes)
		signed := license.SignedValidateResponse{Payload: rejected, Signature: base64.StdEncoding.EncodeToString(sig)}
		rawRejected, _ := json.Marshal(signed)
		_, _ = w.Write(rawRejected)
	})

	validRaw, pubKeyB64, privKey = signTestResponse(t, validPayload)
	svc.cfg.PublicKeyB64 = pubKeyB64
	svc.client = license.NewClient(svc.cfg.BaseURL)

	// First heartbeat: succeeds, caches the valid response.
	svc.heartbeatOnce("key-123")
	status := svc.Status()
	if !status.Valid {
		t.Fatalf("expected first heartbeat to succeed, got status=%+v", status)
	}

	cached, err := svc.getSystemConfig(systemConfigLastValidResponseKey)
	if err != nil || cached == "" {
		t.Fatalf("expected the valid response to be cached, err=%v cached=%q", err, cached)
	}

	// Second heartbeat: server rejects (fingerprint mismatch scenario).
	// Without the fallback, this would lock the panel immediately with
	// no grace period (see heartbeatOnce's own doc comment on rejections).
	svc.heartbeatOnce("key-123")
	status = svc.Status()
	if !status.Valid {
		t.Fatalf("expected the cached, still-unexpired license to keep the panel unlocked despite a rejected live heartbeat, got status=%+v", status)
	}
	if status.ExpiresAt != futureExpiry {
		t.Fatalf("expected fallback status to carry the cached ExpiresAt %q, got %q", futureExpiry, status.ExpiresAt)
	}
}

// TestHeartbeat_RejectedAndCachedLicenseExpiredLocksPanel confirms the
// fallback does NOT apply once the cached license's own ExpiresAt has
// actually passed -- a genuinely expired license must still lock the
// panel and show the activation screen, exactly as a live rejection
// already says.
func TestHeartbeat_RejectedAndCachedLicenseExpiredLocksPanel(t *testing.T) {
	db := newTestLicenseDB(t)

	pastExpiry := time.Now().Add(-24 * time.Hour).Format(time.RFC3339)
	expiredPayload := license.ValidateResponse{
		Valid: true, LicenseKey: "key-456", PlanName: "trial",
		ExpiresAt: pastExpiry, IssuedAt: time.Now().Add(-72 * time.Hour).Format(time.RFC3339),
		CheckedAt: time.Now().Format(time.RFC3339),
	}

	callCount := 0
	var cachedRaw []byte
	var pubKeyB64 string
	var privKey ed25519.PrivateKey

	svc := newTestLicenseService(t, db, "", func(w http.ResponseWriter, r *http.Request) {
		callCount++
		w.Header().Set("Content-Type", "application/json")
		if callCount == 1 {
			_, _ = w.Write(cachedRaw)
			return
		}
		rejected := license.ValidateResponse{
			Valid: false, Reason: "trial has ended",
			LicenseKey: "key-456", CheckedAt: time.Now().Format(time.RFC3339),
		}
		payloadBytes, _ := json.Marshal(rejected)
		sig := ed25519.Sign(privKey, payloadBytes)
		signed := license.SignedValidateResponse{Payload: rejected, Signature: base64.StdEncoding.EncodeToString(sig)}
		rawRejected, _ := json.Marshal(signed)
		_, _ = w.Write(rawRejected)
	})

	cachedRaw, pubKeyB64, privKey = signTestResponse(t, expiredPayload)
	svc.cfg.PublicKeyB64 = pubKeyB64
	svc.client = license.NewClient(svc.cfg.BaseURL)

	// First heartbeat "succeeds" (Valid=true) but with an already-past
	// ExpiresAt -- simulates a stale cache from an old install whose
	// trial/plan had already lapsed before the backup was even taken.
	svc.heartbeatOnce("key-456")

	// Second heartbeat: server rejects, and this time the cached fallback
	// must NOT rescue it, since the cached license's own expiry is in the
	// past.
	svc.heartbeatOnce("key-456")
	status := svc.Status()
	if status.Valid {
		t.Fatalf("expected an expired cached license to NOT be used as a fallback, got status=%+v", status)
	}
}

// TestHeartbeat_NoCacheRejectionLocksPanelImmediately confirms a
// never-before-cached install (first heartbeat ever rejected, e.g. a
// bad/revoked key entered fresh) still locks immediately with no grace --
// the fallback only ever helps an install that has a genuine prior valid
// check to fall back to, it never invents leniency out of nothing.
func TestHeartbeat_NoCacheRejectionLocksPanelImmediately(t *testing.T) {
	db := newTestLicenseDB(t)

	pub, priv, err := ed25519.GenerateKey(nil)
	if err != nil {
		t.Fatalf("failed to generate keypair: %v", err)
	}
	pubKeyB64 := base64.StdEncoding.EncodeToString(pub)

	svc := newTestLicenseService(t, db, pubKeyB64, func(w http.ResponseWriter, r *http.Request) {
		rejected := license.ValidateResponse{
			Valid: false, Reason: "license key not found",
			CheckedAt: time.Now().Format(time.RFC3339),
		}
		payloadBytes, _ := json.Marshal(rejected)
		sig := ed25519.Sign(priv, payloadBytes)
		signed := license.SignedValidateResponse{Payload: rejected, Signature: base64.StdEncoding.EncodeToString(sig)}
		raw, _ := json.Marshal(signed)
		w.Header().Set("Content-Type", "application/json")
		_, _ = w.Write(raw)
	})

	svc.heartbeatOnce("never-activated-key")
	status := svc.Status()
	if status.Valid {
		t.Fatalf("expected a rejected heartbeat with no prior cache to lock the panel, got status=%+v", status)
	}
}

// TestIsExpired covers isExpired's own parsing/fail-open contract
// directly, independent of the full heartbeat flow above.
func TestIsExpired(t *testing.T) {
	cases := []struct {
		name      string
		expiresAt string
		want      bool
	}{
		{"empty means no expiry", "", false},
		{"future RFC3339 not expired", time.Now().Add(24 * time.Hour).Format(time.RFC3339), false},
		{"past RFC3339 expired", time.Now().Add(-24 * time.Hour).Format(time.RFC3339), true},
		{"future plain date not expired", time.Now().Add(48 * time.Hour).Format("2006-01-02"), false},
		{"past plain date expired", time.Now().Add(-48 * time.Hour).Format("2006-01-02"), true},
		{"unparseable fails open (not expired)", "not-a-real-date", false},
	}

	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			if got := isExpired(tc.expiresAt); got != tc.want {
				t.Fatalf("isExpired(%q) = %v, want %v", tc.expiresAt, got, tc.want)
			}
		})
	}
}
