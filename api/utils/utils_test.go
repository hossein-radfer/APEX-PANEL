package utils

import (
	"strconv"
	"strings"
	"testing"
)

func TestParseRouterOSByteSize(t *testing.T) {
	tests := []struct {
		name      string
		input     string
		want      int64
		wantError bool
	}{
		{name: "zero", input: "0", want: 0},
		{name: "bare bytes", input: "512B", want: 512},
		{name: "decimal MiB", input: "1.5MiB", want: 1572864}, // 1.5 * 1024^2
		{name: "decimal-labeled GB treated as binary", input: "2GB", want: 2 * 1024 * 1024 * 1024},
		{name: "raw integer no unit", input: "1048576", want: 1048576},
		{name: "whitespace tolerant", input: "  4 KiB  ", want: 4 * 1024},
		{name: "lowercase unit", input: "3gib", want: 3 * 1024 * 1024 * 1024},
		{name: "malformed text", input: "abc", wantError: true},
		{name: "empty string", input: "", wantError: true},
		{name: "unrecognized unit", input: "5XB", wantError: true},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got, err := ParseRouterOSByteSize(tt.input)
			if tt.wantError {
				if err == nil {
					t.Fatalf("expected an error for input %q, got value %d", tt.input, got)
				}
				return
			}
			if err != nil {
				t.Fatalf("unexpected error for input %q: %v", tt.input, err)
			}
			if got != tt.want {
				t.Fatalf("ParseRouterOSByteSize(%q) = %d, want %d", tt.input, got, tt.want)
			}
		})
	}

	// Fractional binary sizes checked with a tolerance rather than an exact
	// literal, since hand-computing the exact truncated int64 for a
	// fractional multiplication is fragile across platforms/runtimes.
	t.Run("fractional GiB within tolerance", func(t *testing.T) {
		got, err := ParseRouterOSByteSize("8.6GiB")
		if err != nil {
			t.Fatalf("unexpected error: %v", err)
		}
		factor, err := strconv.ParseFloat("8.6", 64)
		if err != nil {
			t.Fatalf("test setup error: %v", err)
		}
		want := int64(factor * float64(1024*1024*1024))
		diff := got - want
		if diff < -1 || diff > 1 {
			t.Fatalf("ParseRouterOSByteSize(\"8.6GiB\") = %d, want approximately %d", got, want)
		}
	})
}

// TestRandomString_LengthCharsetAndUniqueness is the regression test for a
// confirmed finding from an authorized penetration test: RandomString
// backs every JWT/webhook/support-token secret in this codebase (see its
// own doc comment) and previously drew from math/rand, not crypto/rand --
// this only checks the observable contract (length, charset), since
// verifying the RNG's cryptographic properties isn't something a unit
// test can do, but a uniqueness check across many calls at least catches
// a broken/fallback generator that repeats output.
func TestRandomString_LengthCharsetAndUniqueness(t *testing.T) {
	const charset = "abcdefghijklmnopqrstuvwxyzABCDEFGHIJKLMNOPQRSTUVWXYZ0123456789"

	seen := make(map[string]bool)
	for i := 0; i < 100; i++ {
		got := RandomString(24)
		if len(got) != 24 {
			t.Fatalf("RandomString(24) returned length %d, want 24", len(got))
		}
		for _, r := range got {
			if !strings.ContainsRune(charset, r) {
				t.Fatalf("RandomString(24) contained unexpected character %q", r)
			}
		}
		if seen[got] {
			t.Fatalf("RandomString(24) produced the same value twice across 100 calls: %q", got)
		}
		seen[got] = true
	}
}
