package service

import "testing"

// newTestLicenseServiceWithStatus builds a bare LicenseService and directly
// seeds its in-memory status -- setStatus is unexported and normally only
// reached via a real signed heartbeat round trip (already covered by
// license_cache_fallback_test.go), so these tests instead exercise
// GetFreeTierLimit's own fallback rules in isolation against a
// hand-constructed status, which is the actual unit under test here.
func newTestLicenseServiceWithStatus(status LicenseStatus) *LicenseService {
	svc := &LicenseService{}
	svc.status = status
	return svc
}

// TestGetFreeTierLimit_NotRestrictedIsAlwaysUnlimited confirms a
// non-Restricted license (the normal, paying-customer case) never enforces
// ANY cap, even if FreeTierLimits happens to be non-empty (e.g. stale data
// left over from a previous restricted period).
func TestGetFreeTierLimit_NotRestrictedIsAlwaysUnlimited(t *testing.T) {
	svc := newTestLicenseServiceWithStatus(LicenseStatus{
		Restricted:     false,
		FreeTierLimits: map[string]int64{"max_resellers": 1},
	})

	if _, ok := svc.GetFreeTierLimit("max_resellers"); ok {
		t.Fatal("expected no cap enforced when the license is not Restricted, even with a configured limit present")
	}
}

// TestGetFreeTierLimit_RestrictedWithConfiguredKeyReturnsLimit is the core
// happy-path case: a Restricted license with a positive configured value
// for the requested key returns that exact value.
func TestGetFreeTierLimit_RestrictedWithConfiguredKeyReturnsLimit(t *testing.T) {
	svc := newTestLicenseServiceWithStatus(LicenseStatus{
		Restricted:     true,
		FreeTierLimits: map[string]int64{"max_resellers": 5, "max_peers": 50},
	})

	limit, ok := svc.GetFreeTierLimit("max_resellers")
	if !ok || limit != 5 {
		t.Fatalf("expected (5, true), got (%d, %v)", limit, ok)
	}
	limit, ok = svc.GetFreeTierLimit("max_peers")
	if !ok || limit != 50 {
		t.Fatalf("expected (50, true), got (%d, %v)", limit, ok)
	}
}

// TestGetFreeTierLimit_RestrictedMissingKeyIsUnlimited confirms a Restricted
// license with NO configured value for a given key is unlimited for that
// specific key -- the admin adding "max_resellers" doesn't implicitly cap
// every other capability to zero.
func TestGetFreeTierLimit_RestrictedMissingKeyIsUnlimited(t *testing.T) {
	svc := newTestLicenseServiceWithStatus(LicenseStatus{
		Restricted:     true,
		FreeTierLimits: map[string]int64{"max_resellers": 5},
	})

	if _, ok := svc.GetFreeTierLimit("max_peers"); ok {
		t.Fatal("expected no cap for a key the admin never configured, even while Restricted")
	}
}

// TestGetFreeTierLimit_ZeroOrNegativeConfiguredValueIsUnlimited confirms a
// configured value of 0 or negative is treated as "no cap" rather than
// "cap everyone at zero" -- a genuine cap is always a positive count, and
// 0/negative is the deliberate sentinel for "not actually limited," per
// GetFreeTierLimit's own doc comment.
func TestGetFreeTierLimit_ZeroOrNegativeConfiguredValueIsUnlimited(t *testing.T) {
	svc := newTestLicenseServiceWithStatus(LicenseStatus{
		Restricted:     true,
		FreeTierLimits: map[string]int64{"max_resellers": 0, "max_peers": -1},
	})

	if _, ok := svc.GetFreeTierLimit("max_resellers"); ok {
		t.Fatal("expected a configured value of 0 to mean unlimited, not zero allowed")
	}
	if _, ok := svc.GetFreeTierLimit("max_peers"); ok {
		t.Fatal("expected a configured negative value to mean unlimited")
	}
}

// TestGetFreeTierLimit_NilLimitsMapIsUnlimited confirms a Restricted status
// with a nil FreeTierLimits map (e.g. the very first restricted heartbeat
// before any limits were ever configured in license-panel) never panics
// and is treated as unlimited for every key.
func TestGetFreeTierLimit_NilLimitsMapIsUnlimited(t *testing.T) {
	svc := newTestLicenseServiceWithStatus(LicenseStatus{
		Restricted:     true,
		FreeTierLimits: nil,
	})

	if _, ok := svc.GetFreeTierLimit("max_resellers"); ok {
		t.Fatal("expected a nil limits map to be unlimited for every key, not panic or wrongly enforce")
	}
}
