// Command signbinary is a release-time-only tool (never run by mwp itself,
// never shipped inside the mwp binary) that signs a built mwp executable
// against tampering -- phase 4-12-ج. Run this ONCE after cross-compiling
// mwp, before uploading it to a server:
//
//	go run ./cmd/signbinary -key <path-or-env> -binary /path/to/mwp
//
// This writes /path/to/mwp.sig next to the binary. Both the binary and its
// .sig file must be deployed together; mwp reads the .sig at startup via
// license/binaryintegrity.Verify (see service/license.go's own
// checkBinaryIntegrity).
//
// The private key must NEVER be committed to the repo or shipped with the
// binary -- only RELEASE_PUBLIC_KEY (baked into config, see
// config.GetLicenseConfig's own doc comment) is safe to distribute, since
// possessing the public key only allows VERIFYING signatures, not forging
// new ones for a tampered binary.
package main

import (
	"crypto/ed25519"
	"encoding/base64"
	"flag"
	"fmt"
	"os"
	"strings"

	"github.com/maahdima/mwp/api/license/binaryintegrity"
)

func main() {
	binaryPath := flag.String("binary", "", "path to the built mwp executable to sign")
	keyFlag := flag.String("key", "", "base64 Ed25519 private key, or a path to a file containing it (if RELEASE_PRIVATE_KEY env var is not set)")
	generate := flag.Bool("generate", false, "generate a brand new Ed25519 keypair and print it (does not sign anything)")
	flag.Parse()

	if *generate {
		pub, priv, err := ed25519GenerateKey()
		if err != nil {
			fmt.Fprintln(os.Stderr, "failed to generate keypair:", err)
			os.Exit(1)
		}
		fmt.Println("RELEASE_PRIVATE_KEY (keep this secret, never commit it):")
		fmt.Println(base64.StdEncoding.EncodeToString(priv))
		fmt.Println()
		fmt.Println("RELEASE_PUBLIC_KEY (safe to bake into config.go / ship with mwp):")
		fmt.Println(base64.StdEncoding.EncodeToString(pub))
		return
	}

	if *binaryPath == "" {
		fmt.Fprintln(os.Stderr, "usage: signbinary -binary /path/to/mwp [-key <base64-or-file>]")
		os.Exit(1)
	}

	privateKeyB64 := os.Getenv("RELEASE_PRIVATE_KEY")
	if privateKeyB64 == "" {
		privateKeyB64 = *keyFlag
	}
	if privateKeyB64 == "" {
		fmt.Fprintln(os.Stderr, "no private key given -- set RELEASE_PRIVATE_KEY or pass -key")
		os.Exit(1)
	}
	// -key may itself be a file path rather than the raw base64 value.
	if data, err := os.ReadFile(privateKeyB64); err == nil {
		privateKeyB64 = strings.TrimSpace(string(data))
	}

	privateKey, err := decodePrivateKey(privateKeyB64)
	if err != nil {
		fmt.Fprintln(os.Stderr, "invalid private key:", err)
		os.Exit(1)
	}

	if err := binaryintegrity.Sign(*binaryPath, privateKey); err != nil {
		fmt.Fprintln(os.Stderr, "failed to sign binary:", err)
		os.Exit(1)
	}

	fmt.Printf("signed %s -> %s%s\n", *binaryPath, *binaryPath, binaryintegrity.SignatureSuffix)
}

func ed25519GenerateKey() (ed25519.PublicKey, ed25519.PrivateKey, error) {
	return ed25519.GenerateKey(nil)
}

func decodePrivateKey(b64 string) (ed25519.PrivateKey, error) {
	decoded, err := base64.StdEncoding.DecodeString(b64)
	if err != nil {
		return nil, fmt.Errorf("decoding private key: %w", err)
	}
	if len(decoded) != ed25519.PrivateKeySize {
		return nil, fmt.Errorf("private key has wrong length: got %d, want %d", len(decoded), ed25519.PrivateKeySize)
	}
	return ed25519.PrivateKey(decoded), nil
}
