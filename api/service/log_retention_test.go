package service

import "testing"

// TestLogRetentionMaxBytes_MatchesAdminRequestedCeiling pins down the
// 500MB ceiling the admin explicitly asked for -- a regression guard
// against an accidental unit mix-up (e.g. 500*1024 instead of
// 500*1024*1024) that would silently vacuum the journal down to ~500KB
// instead of 500MB.
func TestLogRetentionMaxBytes_MatchesAdminRequestedCeiling(t *testing.T) {
	const wantBytes = 500 * 1024 * 1024
	if LogRetentionMaxBytes != wantBytes {
		t.Fatalf("expected LogRetentionMaxBytes=%d (500MB), got %d", wantBytes, LogRetentionMaxBytes)
	}
}

// TestNewLogRetentionService_ReturnsNonNilLogger confirms construction
// never leaves the logger nil (VacuumJournal logs unconditionally on
// every path, success or failure).
func TestNewLogRetentionService_ReturnsNonNilLogger(t *testing.T) {
	svc := NewLogRetentionService()
	if svc == nil {
		t.Fatal("expected NewLogRetentionService to return a non-nil service")
	}
	if svc.logger == nil {
		t.Fatal("expected NewLogRetentionService to initialize a non-nil logger")
	}
}
