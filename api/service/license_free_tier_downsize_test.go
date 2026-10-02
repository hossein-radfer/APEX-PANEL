package service

import (
	"errors"
	"testing"

	"go.uber.org/zap"
)

// fakeFreeTierDownsizer is a minimal test double for FreeTierDownsizer --
// records every cap it was called with and returns a scripted
// (count, error) pair, mirroring fakeFreeTierLimiter's own role for
// freeTierLimiter (reseller_free_tier_test.go).
type fakeFreeTierDownsizer struct {
	calls     []int64
	returnN   int
	returnErr error
}

func (f *fakeFreeTierDownsizer) SuspendOldestForFreeTier(cap int64) (int, error) {
	f.calls = append(f.calls, cap)
	return f.returnN, f.returnErr
}

func newTestLicenseServiceForDownsizing() *LicenseService {
	return &LicenseService{logger: zap.NewNop()}
}

// TestEnforceFreeTierDownsizing_NoDownsizersIsNoOp confirms an install that
// never calls SetFreeTierDownsizers (e.g. a build predating this wiring)
// never panics and calls nothing.
func TestEnforceFreeTierDownsizing_NoDownsizersIsNoOp(t *testing.T) {
	svc := newTestLicenseServiceForDownsizing()
	svc.enforceFreeTierDownsizing(map[string]int64{"max_peers": 5})
}

// TestEnforceFreeTierDownsizing_EmptyLimitsIsNoOp confirms an empty/nil
// FreeTierLimits payload (e.g. the admin never configured any cap) calls no
// downsizer even if some are wired.
func TestEnforceFreeTierDownsizing_EmptyLimitsIsNoOp(t *testing.T) {
	svc := newTestLicenseServiceForDownsizing()
	fake := &fakeFreeTierDownsizer{}
	svc.SetFreeTierDownsizers(map[string]FreeTierDownsizer{"max_peers": fake})

	svc.enforceFreeTierDownsizing(nil)

	if len(fake.calls) != 0 {
		t.Fatalf("expected no calls with empty limits, got %v", fake.calls)
	}
}

// TestEnforceFreeTierDownsizing_CallsOnlyConfiguredKeys is the core
// regression test: only downsizers whose key has a positive configured
// limit are invoked, each with exactly that limit -- an unrelated wired
// downsizer whose key isn't in the payload is left untouched.
func TestEnforceFreeTierDownsizing_CallsOnlyConfiguredKeys(t *testing.T) {
	svc := newTestLicenseServiceForDownsizing()
	peersFake := &fakeFreeTierDownsizer{}
	resellersFake := &fakeFreeTierDownsizer{}
	svc.SetFreeTierDownsizers(map[string]FreeTierDownsizer{
		"max_peers":     peersFake,
		"max_resellers": resellersFake,
	})

	svc.enforceFreeTierDownsizing(map[string]int64{"max_peers": 10})

	if len(peersFake.calls) != 1 || peersFake.calls[0] != 10 {
		t.Fatalf("expected max_peers downsizer called once with 10, got %v", peersFake.calls)
	}
	if len(resellersFake.calls) != 0 {
		t.Fatalf("expected max_resellers downsizer never called (not in payload), got %v", resellersFake.calls)
	}
}

// TestEnforceFreeTierDownsizing_ZeroOrNegativeLimitSkipsDownsizer confirms
// the same "0/negative means unlimited" sentinel GetFreeTierLimit itself
// honors is also respected here -- a stale/unlimited entry never triggers
// a suspend call.
func TestEnforceFreeTierDownsizing_ZeroOrNegativeLimitSkipsDownsizer(t *testing.T) {
	svc := newTestLicenseServiceForDownsizing()
	fake := &fakeFreeTierDownsizer{}
	svc.SetFreeTierDownsizers(map[string]FreeTierDownsizer{"max_peers": fake})

	svc.enforceFreeTierDownsizing(map[string]int64{"max_peers": 0})
	svc.enforceFreeTierDownsizing(map[string]int64{"max_peers": -1})

	if len(fake.calls) != 0 {
		t.Fatalf("expected no calls for a zero/negative limit, got %v", fake.calls)
	}
}

// TestEnforceFreeTierDownsizing_DownsizerErrorDoesNotBlockOthers confirms
// one downsizer failing never prevents the others from still running --
// matches this codebase's own established "one dead resource never blocks
// the rest of the batch" convention (see e.g. CreateApplication's own doc
// comment).
func TestEnforceFreeTierDownsizing_DownsizerErrorDoesNotBlockOthers(t *testing.T) {
	svc := newTestLicenseServiceForDownsizing()
	failingFake := &fakeFreeTierDownsizer{returnErr: errors.New("boom")}
	workingFake := &fakeFreeTierDownsizer{returnN: 3}
	svc.SetFreeTierDownsizers(map[string]FreeTierDownsizer{
		"max_peers":     failingFake,
		"max_resellers": workingFake,
	})

	svc.enforceFreeTierDownsizing(map[string]int64{"max_peers": 5, "max_resellers": 2})

	if len(failingFake.calls) != 1 {
		t.Fatalf("expected the failing downsizer to still be called once, got %v", failingFake.calls)
	}
	if len(workingFake.calls) != 1 || workingFake.calls[0] != 2 {
		t.Fatalf("expected the working downsizer to still be called with its own limit, got %v", workingFake.calls)
	}
}
