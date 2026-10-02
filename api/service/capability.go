package service

import (
	"encoding/json"
	"fmt"
	"time"

	"github.com/maahdima/mwp/api/license"
)

// CapabilityProof is what RequestCapabilityProof returns on success --
// deliberately does NOT expose the raw signed bytes to the caller,
// forcing every capability check to go through this one function rather
// than each call site re-implementing (and potentially getting wrong)
// the nonce-echo verification.
type CapabilityProof struct {
	CapabilityKey string
	CheckedAt     time.Time
}

// RequestCapabilityProof is the client-side half of the فاز پنجم-۱ live
// challenge/response mechanism -- the general-purpose primitive future
// sensitive features (e.g. "raise this reseller's user cap beyond the
// license's normal limit") call to prove, RIGHT NOW, that the license
// server actively approved capabilityKey, rather than trusting a cached
// signed response that could be months old.
//
// Deliberately bypasses EVERYTHING this file's own heartbeat/activate
// caching does: it does NOT read systemConfigLastValidResponseKey, does
// NOT fall back to restoreFromCachedValidResponse, and does NOT persist
// its own result anywhere for later reuse. Every call does a full,
// fresh two-step round trip (request a nonce, redeem it) against the
// live license server; if the server is unreachable or rejects the
// nonce, this returns an error and the caller must treat the capability
// as NOT granted -- there is no grace period here, unlike the
// general "is this license still valid" check, because a sensitive
// capability unlock is exactly the kind of decision that must never be
// approved on stale information.
func (s *LicenseService) RequestCapabilityProof(capabilityKey string) (*CapabilityProof, error) {
	licenseKey, err := s.getStoredLicenseKey()
	if err != nil {
		return nil, fmt.Errorf("failed to read stored license key: %w", err)
	}
	if licenseKey == "" {
		return nil, fmt.Errorf("no license key configured")
	}

	fingerprint := license.Fingerprint(s.dataDirPath)

	nonceRaw, err := s.client.RequestNonce(license.NonceRequest{Fingerprint: fingerprint})
	if err != nil {
		return nil, fmt.Errorf("failed to request a nonce from the license server: %w", err)
	}
	var nonceResp license.NonceResponse
	if err := json.Unmarshal(nonceRaw, &nonceResp); err != nil {
		return nil, fmt.Errorf("failed to parse nonce response: %w", err)
	}
	if nonceResp.Nonce == "" {
		return nil, fmt.Errorf("license server returned an empty nonce")
	}

	validateRaw, err := s.client.ValidateCapability(license.CapabilityValidateRequest{
		LicenseKey:    licenseKey,
		Fingerprint:   fingerprint,
		Nonce:         nonceResp.Nonce,
		CapabilityKey: capabilityKey,
	})
	if err != nil {
		return nil, fmt.Errorf("failed to validate capability with the license server: %w", err)
	}

	signed, err := license.VerifySignedCapabilityResponse(s.cfg.PublicKeyB64, validateRaw, nonceResp.Nonce)
	if err != nil {
		return nil, fmt.Errorf("capability response failed verification: %w", err)
	}

	if !signed.Payload.Valid {
		reason := signed.Payload.Reason
		if reason == "" {
			reason = "capability not granted"
		}
		return nil, fmt.Errorf("license server denied capability %q: %s", capabilityKey, reason)
	}
	if signed.Payload.CapabilityKey != capabilityKey {
		return nil, fmt.Errorf("license server response is for a different capability (%q) than requested (%q)", signed.Payload.CapabilityKey, capabilityKey)
	}

	checkedAt, _ := time.Parse(time.RFC3339, signed.Payload.CheckedAt)

	return &CapabilityProof{
		CapabilityKey: capabilityKey,
		CheckedAt:     checkedAt,
	}, nil
}
