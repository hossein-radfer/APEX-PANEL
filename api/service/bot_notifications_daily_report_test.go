package service

import (
	"strings"
	"testing"

	"github.com/maahdima/mwp/api/dataservice/model"
)

// TestBuildDailyReportText_IncludesDatabaseSizeWhenPositive is the
// regression/feature test for فاز پنجم-۷ ("خلاصه‌ی مصرف، حجم دیتابیس...
// در گزارش دوره‌ای خودکار"): a positive databaseSizeBytes must produce a
// visible database-size line in the report text.
func TestBuildDailyReportText_IncludesDatabaseSizeWhenPositive(t *testing.T) {
	entries := []DailyReportEntry{
		{Reseller: model.Reseller{Name: "r1"}, WalletBalance: 100000},
	}

	text := buildDailyReportText(entries, 2*1024*1024*1024) // 2 GiB

	if !strings.Contains(text, "حجم پایگاه داده پنل") {
		t.Fatalf("expected the report to mention database size, got: %s", text)
	}
	if !strings.Contains(text, "2.00") {
		t.Fatalf("expected the report to show ~2.00 GB, got: %s", text)
	}
}

// TestBuildDailyReportText_OmitsDatabaseSizeWhenZero confirms the
// "unavailable" case (e.g. Postgres-backed install) omits the line
// entirely rather than showing a misleading "0 GB".
func TestBuildDailyReportText_OmitsDatabaseSizeWhenZero(t *testing.T) {
	entries := []DailyReportEntry{
		{Reseller: model.Reseller{Name: "r1"}, WalletBalance: 100000},
	}

	text := buildDailyReportText(entries, 0)

	if strings.Contains(text, "حجم پایگاه داده") {
		t.Fatalf("expected no database-size line when databaseSizeBytes is 0, got: %s", text)
	}
}

// TestBuildDailyReportText_EmptyEntriesIgnoresDatabaseSize confirms the
// "no resellers" early-return path is unaffected by the new parameter.
func TestBuildDailyReportText_EmptyEntriesIgnoresDatabaseSize(t *testing.T) {
	text := buildDailyReportText(nil, 5*1024*1024*1024)

	if !strings.Contains(text, "هیچ نماینده‌ای ثبت نشده است") {
		t.Fatalf("expected the empty-entries message, got: %s", text)
	}
}
