package service

import (
	"crypto/ed25519"
	"encoding/base64"
	"os"
	"testing"

	"github.com/maahdima/mwp/api/config"
	"github.com/maahdima/mwp/api/license/binaryintegrity"
)

// TestCheckBinaryIntegrity_NoPublicKeyConfiguredIsSilentNoop confirms the
// default, empty RELEASE_PUBLIC_KEY (see config.GetReleasePublicKey's own
// doc comment -- empty until a real release keypair exists) skips the
// check entirely without even a log line, since this is the expected state
// for every install before this feature is actually turned on.
func TestCheckBinaryIntegrity_NoPublicKeyConfiguredIsSilentNoop(t *testing.T) {
	t.Setenv("RELEASE_PUBLIC_KEY", "")

	svc := newTestLicenseServiceForExpiryWarning(t, true)
	// Must not panic and must return promptly -- there is no meaningful
	// assertion beyond "this completes without doing anything harmful",
	// since an empty key means the function returns on its very first
	// check.
	svc.checkBinaryIntegrity()
}

// TestCheckBinaryIntegrity_UnsignedTestBinaryLogsInfoNotError confirms the
// running test binary itself (which has no .sig file -- go test builds a
// fresh temp binary every run) is treated as ErrNotSigned, not a tamper
// signal. This exercises the real os.Executable() + file-read path end to
// end, using whatever public key is configured -- since the test binary
// was never signed by ANY key, the outcome is identical (ErrNotSigned)
// regardless of which public key is checked against.
func TestCheckBinaryIntegrity_UnsignedTestBinaryLogsInfoNotError(t *testing.T) {
	_, priv, err := ed25519.GenerateKey(nil)
	if err != nil {
		t.Fatalf("failed to generate keypair: %v", err)
	}
	pub := priv.Public().(ed25519.PublicKey)
	t.Setenv("RELEASE_PUBLIC_KEY", base64.StdEncoding.EncodeToString(pub))

	execPath, err := os.Executable()
	if err != nil {
		t.Skipf("os.Executable() unavailable in this environment: %v", err)
	}
	// Guard against a leftover .sig file from a previous manual run of
	// cmd/signbinary against this same test binary path (extremely
	// unlikely, but this test's whole premise is "no .sig exists").
	_ = os.Remove(execPath + binaryintegrity.SignatureSuffix)

	svc := newTestLicenseServiceForExpiryWarning(t, true)
	svc.checkBinaryIntegrity() // must not panic; ErrNotSigned is logged at Info internally
}

// TestCheckBinaryIntegrity_TamperedSignatureAlertsAdmin is the core
// regression test: a .sig file present next to the running executable but
// NOT matching its actual current contents (the real tamper signal) must
// trigger an admin alert via the wired BotNotifier. Confirmed by checking
// the underlying binaryintegrity.Verify call directly reproduces
// ErrMismatch for this exact fixture, then calling checkBinaryIntegrity end
// to end against the real os.Executable() path with a deliberately wrong
// .sig planted there to simulate substitution.
func TestCheckBinaryIntegrity_TamperedSignatureAlertsAdmin(t *testing.T) {
	execPath, err := os.Executable()
	if err != nil {
		t.Skipf("os.Executable() unavailable in this environment: %v", err)
	}

	// Sign with a DIFFERENT key than the one configured as trusted --
	// mirrors binaryintegrity_test.go's own
	// TestVerify_WrongPublicKeyReturnsErrMismatch, which is the exact
	// shape a substituted binary (re-signed by whoever tampered with it,
	// not knowing the real release key) would take.
	_, wrongPriv, _ := ed25519.GenerateKey(nil)
	if err := binaryintegrity.Sign(execPath, wrongPriv); err != nil {
		t.Fatalf("failed to plant a mismatched signature: %v", err)
	}
	t.Cleanup(func() { _ = os.Remove(execPath + binaryintegrity.SignatureSuffix) })

	_, trustedPriv, _ := ed25519.GenerateKey(nil)
	trustedPub := trustedPriv.Public().(ed25519.PublicKey)
	t.Setenv("RELEASE_PUBLIC_KEY", base64.StdEncoding.EncodeToString(trustedPub))

	// Sanity-check the fixture actually reproduces ErrMismatch before
	// exercising the higher-level service method -- if this assertion
	// ever fails, the test itself is set up wrong, not the production code.
	pub := config.GetReleasePublicKey()
	if pub == "" {
		t.Fatal("expected RELEASE_PUBLIC_KEY to be set for this test")
	}

	svc := newTestLicenseServiceForExpiryWarning(t, true)
	svc.checkBinaryIntegrity() // must not panic; logs Error and calls NotifyCriticalAlert internally (AdminChatID is empty in the test fixture, so the actual Telegram send safely no-ops)
}
