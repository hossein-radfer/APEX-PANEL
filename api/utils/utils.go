package utils

import (
	"crypto/rand"
	"fmt"
	"math/big"
	"net"
	"regexp"
	"strconv"
	"strings"
	"time"

	"github.com/labstack/gommon/log"
)

func Ptr(s string) *string { return &s }

func DerefString(s *string) string {
	if s == nil {
		return ""
	}
	return *s
}

func ParseStringToInt(s string) int64 {
	val, err := strconv.ParseInt(s, 10, 64)
	if err != nil {
		return 0
	}
	return val
}

func ParseCustomDuration(s string) (time.Duration, error) {
	re := regexp.MustCompile(`(\d+)([wdhms])`)
	matches := re.FindAllStringSubmatch(s, -1)

	var total time.Duration
	for _, match := range matches {
		num, err := strconv.Atoi(match[1])
		if err != nil {
			return 0, err
		}
		switch match[2] {
		case "w":
			total += time.Duration(num) * 7 * 24 * time.Hour
		case "d":
			total += time.Duration(num) * 24 * time.Hour
		case "h":
			total += time.Duration(num) * time.Hour
		case "m":
			total += time.Duration(num) * time.Minute
		case "s":
			total += time.Duration(num) * time.Second
		default:
			return 0, fmt.Errorf("unknown duration unit: %s", match[2])
		}
	}
	return total, nil
}

func BytesToGB(b int64) string {
	return fmt.Sprintf("%.1f", float64(b)/float64(1024*1024*1024))
}

func GBToBytes(s string) int64 {
	gb, err := strconv.ParseFloat(s, 64)
	if err != nil {
		return 0
	}
	return int64(gb * 1024 * 1024 * 1024)
}

// routerOSByteSizePattern matches RouterOS's human-readable unit-suffixed
// size strings, e.g. "8.6GiB", "512B", "1.5MiB", "2GB". Both the "true"
// binary suffixes (KiB/MiB/GiB/TiB) and the decimal-looking ones RouterOS
// sometimes renders (KB/MB/GB/TB) are treated as binary (×1024) multiples,
// matching RouterOS's actual observed behavior -- it does not use true SI
// decimal units despite the label.
var routerOSByteSizePattern = regexp.MustCompile(`(?i)^\s*([0-9]+(?:\.[0-9]+)?)\s*([KMGT]?I?B)\s*$`)

// ParseRouterOSByteSize converts a RouterOS human-readable size string (as
// returned by /user-manager/user/monitor's total-download/total-upload
// fields) into a raw byte count. Unlike GBToBytes, this returns an explicit
// error rather than silently returning 0 on anything unrecognized -- a
// parse failure here must never be mistaken for "genuinely zero usage" and
// must never overwrite a reseller's/account's previously-known usage value.
func ParseRouterOSByteSize(s string) (int64, error) {
	trimmed := strings.TrimSpace(s)
	if trimmed == "" {
		return 0, fmt.Errorf("empty size string")
	}

	// Bare integer/decimal with no unit suffix is treated as raw bytes.
	if raw, err := strconv.ParseFloat(trimmed, 64); err == nil {
		return int64(raw), nil
	}

	matches := routerOSByteSizePattern.FindStringSubmatch(trimmed)
	if matches == nil {
		return 0, fmt.Errorf("unrecognized RouterOS size format: %q", s)
	}

	value, err := strconv.ParseFloat(matches[1], 64)
	if err != nil {
		return 0, fmt.Errorf("failed to parse numeric part of %q: %w", s, err)
	}

	// Normalize "KiB"->"KB", "GiB"->"GB", etc. (strip the binary "I" infix,
	// not a trailing character -- the string always ends in "B") before the
	// switch below, which only recognizes the 5 four-or-fewer-character forms.
	unit := strings.ToUpper(matches[2])
	unit = strings.Replace(unit, "I", "", 1)

	var multiplier float64
	switch unit {
	case "B":
		multiplier = 1
	case "KB":
		multiplier = 1024
	case "MB":
		multiplier = 1024 * 1024
	case "GB":
		multiplier = 1024 * 1024 * 1024
	case "TB":
		multiplier = 1024 * 1024 * 1024 * 1024
	default:
		return 0, fmt.Errorf("unrecognized RouterOS size unit in %q", s)
	}

	return int64(value * multiplier), nil
}

func IsPeerSharable(isShared bool, shareExpireTime *string) bool {
	if !isShared {
		log.Errorf("peer is not shared")
		return false
	}

	if shareExpireTime != nil {
		expireTime, err := time.Parse("2006-01-02", *shareExpireTime)
		if err != nil {
			log.Errorf("failed to parse share expire time")
			return false
		}
		if time.Now().After(expireTime) {
			log.Errorf("share link has expired")
			return false
		}
	}

	return true
}

// RandomString returns an n-character string drawn from crypto/rand, for
// use anywhere a value's security property requires that it cannot be
// guessed or reconstructed (signing secrets, generated passwords, and
// similar). Falls back to a diagnostic placeholder only if the OS
// entropy source itself is unavailable.
func RandomString(n int) string {
	const letters = "abcdefghijklmnopqrstuvwxyzABCDEFGHIJKLMNOPQRSTUVWXYZ0123456789"

	s := make([]byte, n)
	for i := range s {
		idx, err := rand.Int(rand.Reader, big.NewInt(int64(len(letters))))
		if err != nil {
			log.Errorf("crypto/rand unavailable while generating a random string, falling back to a fixed placeholder: %v", err)
			return "insecure-rand-fallback"
		}
		s[i] = letters[idx.Int64()]
	}
	return string(s)
}

func IPToUint32(ip net.IP) uint32 {
	ip = ip.To4()
	if ip == nil {
		return 0
	}
	return uint32(ip[0])<<24 | uint32(ip[1])<<16 | uint32(ip[2])<<8 | uint32(ip[3])
}

func FormatDuration(d time.Duration) string {
	d = d.Truncate(time.Second)

	h := d / time.Hour
	d -= h * time.Hour
	m := d / time.Minute
	d -= m * time.Minute
	s := d / time.Second

	return fmt.Sprintf("%02d:%02d:%02d", h, m, s)
}
