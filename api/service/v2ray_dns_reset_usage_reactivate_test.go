package service

import (
	"encoding/json"
	"net/http"
	"testing"

	"github.com/maahdima/mwp/api/dataservice/model"
)

// TestV2RayResetUsage_ReactivatesPackageSuspendedByQuota is the regression
// test for a confirmed, reported bug (phase-7 item #2, "ریست مصرف V2Ray/DNS
// ناقص است"): a package the quota job had already suspended
// (SuspendedByQuota=true, Status="suspended") stayed suspended after an
// admin's "Reset Usage" action -- ResetUsage only ever bumped
// UsageOffsetBytes, so the package displayed 0 used while its x-ui client
// (and its DB Status) remained exactly as suspended as before. Confirms the
// fix: resetting a quota-suspended package's usage now also flips it back
// to active and clears every suspend flag, mirroring UpdatePackage's own
// Status="active" reactivation branch.
func TestV2RayResetUsage_ReactivatesPackageSuspendedByQuota(t *testing.T) {
	svc, db := newTestV2RayPackageService(t)

	pkg := model.V2RayPackage{
		UUID: "pkg-reset-reactivate", TotalVolumeBytes: 1024, DurationDays: 30,
		Status: "suspended", SuspendedByQuota: true, WasActiveBeforeSuspend: true,
		SuspendedByResellerQuota: true,
	}
	if err := db.Create(&pkg).Error; err != nil {
		t.Fatalf("failed to create package: %v", err)
	}

	if err := svc.ResetUsage(pkg.ID, nil); err != nil {
		t.Fatalf("failed to reset package usage: %v", err)
	}

	var reloaded model.V2RayPackage
	if err := db.First(&reloaded, pkg.ID).Error; err != nil {
		t.Fatalf("failed to reload package: %v", err)
	}
	if reloaded.Status != "active" {
		t.Fatalf("expected package status to be active after usage reset, got %q -- "+
			"a quota-suspended package's customer would stay cut off even though the panel now shows 0 used", reloaded.Status)
	}
	if reloaded.SuspendedByQuota || reloaded.WasActiveBeforeSuspend || reloaded.SuspendedByResellerQuota {
		t.Fatalf("expected every suspend flag to be cleared after usage reset, got suspended_by_quota=%v was_active_before_suspend=%v suspended_by_reseller_quota=%v",
			reloaded.SuspendedByQuota, reloaded.WasActiveBeforeSuspend, reloaded.SuspendedByResellerQuota)
	}
}

// TestV2RayResetUsage_DoesNotReactivateAManuallySuspendedPackage confirms
// the fix above is scoped to quota-triggered suspensions only: a package an
// admin deliberately suspended for an unrelated reason (SuspendedByQuota
// never set) must not be silently reactivated as a side effect of a usage
// reset.
func TestV2RayResetUsage_DoesNotReactivateAManuallySuspendedPackage(t *testing.T) {
	svc, db := newTestV2RayPackageService(t)

	pkg := model.V2RayPackage{
		UUID: "pkg-reset-manual-suspend", TotalVolumeBytes: 1024, DurationDays: 30,
		Status: "suspended",
	}
	if err := db.Create(&pkg).Error; err != nil {
		t.Fatalf("failed to create package: %v", err)
	}

	if err := svc.ResetUsage(pkg.ID, nil); err != nil {
		t.Fatalf("failed to reset package usage: %v", err)
	}

	var reloaded model.V2RayPackage
	if err := db.First(&reloaded, pkg.ID).Error; err != nil {
		t.Fatalf("failed to reload package: %v", err)
	}
	if reloaded.Status != "suspended" {
		t.Fatalf("expected a manually-suspended package (SuspendedByQuota=false) to remain suspended after a usage reset, got %q", reloaded.Status)
	}
}

// TestDNSResetUsage_ClearsLocalQuotaSuspendFlags is the DNS counterpart of
// TestV2RayResetUsage_ReactivatesPackageSuspendedByQuota: DNSAccount's own
// three suspend flags (SuspendedByQuota/WasActiveBeforeSuspend/
// SuspendedByResellerQuota) were never cleared by ResetUsage at all -- the
// remote doctor-dns call's own reported status was the only thing that
// could flip Status back to "active", but the LOCAL flags
// applyResellerDNSQuota's own suspend/resume logic actually reads stayed
// stuck true regardless, letting the very next quota sync tick re-suspend
// the account moments after the admin's reset appeared to succeed. The fake
// doctor-dns server here reports "active" back (the common case: doctor-dns
// itself has no notion of a reseller-level pool, so it agrees the account is
// fine) to confirm the local flags are ALSO cleared, not just Status.
func TestDNSResetUsage_ClearsLocalQuotaSuspendFlags(t *testing.T) {
	svc, db, srv := newTestDNSAccountService(t, func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		_ = json.NewEncoder(w).Encode(map[string]interface{}{
			"user": map[string]interface{}{"status": "active"},
		})
	})
	_ = srv

	var panel model.DNSPanel
	if err := db.First(&panel).Error; err != nil {
		t.Fatalf("failed to load test panel: %v", err)
	}

	acct := createTestDNSAccount(t, db, panel.ID, func(a *model.DNSAccount) {
		a.Status = "suspended"
		a.SuspendedByQuota = true
		a.WasActiveBeforeSuspend = true
		a.SuspendedByResellerQuota = true
		a.TotalVolumeBytes = 1024
	})

	if err := svc.ResetUsage(acct.ID, nil); err != nil {
		t.Fatalf("failed to reset DNS account usage: %v", err)
	}

	var reloaded model.DNSAccount
	if err := db.First(&reloaded, acct.ID).Error; err != nil {
		t.Fatalf("failed to reload DNS account: %v", err)
	}
	if reloaded.Status != "active" {
		t.Fatalf("expected DNS account status to be active after usage reset, got %q", reloaded.Status)
	}
	if reloaded.SuspendedByQuota || reloaded.WasActiveBeforeSuspend || reloaded.SuspendedByResellerQuota {
		t.Fatalf("expected every suspend flag to be cleared after usage reset, got suspended_by_quota=%v was_active_before_suspend=%v suspended_by_reseller_quota=%v",
			reloaded.SuspendedByQuota, reloaded.WasActiveBeforeSuspend, reloaded.SuspendedByResellerQuota)
	}
}
