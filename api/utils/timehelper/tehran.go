package timehelper

import (
	"fmt"
	"time"
)

// tehranLocation is loaded once at package init -- every caller needing
// Iran-local time formatting for a Telegram message shares this same
// *time.Location rather than re-resolving "Asia/Tehran" on every call.
var tehranLocation *time.Location

func init() {
	loc, err := time.LoadLocation("Asia/Tehran")
	if err != nil {
		// A missing tzdata database on the host is the only realistic
		// failure here -- UTC is a safe, always-available fallback rather
		// than panicking the whole process over a timezone label.
		loc = time.UTC
	}
	tehranLocation = loc
}

// FormatTehran renders t in Iran local time (Asia/Tehran, currently a
// fixed UTC+3:30 offset with no DST since Iran abolished its DST schedule)
// using the same RFC1123-style layout previously used for the UTC-only
// timestamps in every Telegram notification (login alerts, failed-login
// alerts, scheduled backup captions) -- a confirmed, reported request:
// these timestamps are shown to a Persian-speaking admin on their phone,
// so a UTC-labeled time (e.g. "21:02:54 UTC" for a login that actually
// happened at 00:32 local time) was confusing rather than merely
// international-audience-friendly.
func FormatTehran(t time.Time) string {
	return t.In(tehranLocation).Format("Mon, 02 Jan 2006 15:04:05") + " (وقت ایران)"
}

// TehranDateStamp renders t as a compact Iran-local "YYYY-MM-DD HH:MM"
// stamp -- used where a shorter form fits better (e.g. a backup file's own
// caption line) than FormatTehran's fuller RFC1123-style rendering.
func TehranDateStamp(t time.Time) string {
	return fmt.Sprintf("%s (وقت ایران)", t.In(tehranLocation).Format("2006-01-02 15:04"))
}

// TehranLocation exposes the shared *time.Location for callers that need
// to do their own time.Time arithmetic in Iran-local terms (e.g. bucketing
// timestamps into Iran-local calendar days) rather than just formatting a
// single value through FormatTehran/TehranDateStamp/TehranDateOnly.
func TehranLocation() *time.Location {
	return tehranLocation
}

// TehranDateOnly renders t as a bare Iran-local "YYYY-MM-DD" date, no
// time-of-day and no "(وقت ایران)" suffix -- for compact UI rows (e.g.
// the mobile app's subscription-start display) that just need a
// correctly-shifted calendar date, not a full timestamp. Confirmed,
// reported bug this fixes: Application.StartAt/ExpireAt were previously
// formatted with the bare .Format() call, rendering in whatever
// time.Location the value happened to carry (UTC, per this codebase's
// storage convention) instead of Iran local time -- a date stored as
// "2026-08-19T21:30:00Z" (00:00 Iran time on the 20th) would misprint as
// the 19th.
func TehranDateOnly(t time.Time) string {
	return t.In(tehranLocation).Format("2006-01-02")
}
