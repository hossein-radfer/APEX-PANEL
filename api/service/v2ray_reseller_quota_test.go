package service

import (
	"encoding/json"
	"fmt"
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/maahdima/mwp/api/dataservice/model"
)

// fakeXuiPanelServer is a minimal x-ui stand-in covering exactly what
// applyResellerV2RayQuota's disable path touches: login + updateClient.
// Records every updateClient call's Enable value, keyed by client UUID.
// missingClientUUIDs (may be nil) simulates the confirmed production
// incident where a client was removed directly on the x-ui panel: any
// updateClient call for a UUID in this set is rejected exactly like real
// x-ui rejects an update for a client it can't find, and a successful
// addClient call for that UUID is recorded in `added` instead of `updates`.
func newFakeXuiPanelServer(t *testing.T, updates map[string]bool) *httptest.Server {
	return newFakeXuiPanelServerWithMissingClients(t, updates, nil, nil)
}

func newFakeXuiPanelServerWithMissingClients(t *testing.T, updates map[string]bool, missingClientUUIDs map[string]bool, added map[string]bool) *httptest.Server {
	t.Helper()
	return httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		switch {
		case r.URL.Path == "/login":
			http.SetCookie(w, &http.Cookie{Name: "3x-ui", Value: "tok", Path: "/"})
			w.Header().Set("Content-Type", "application/json")
			_ = json.NewEncoder(w).Encode(map[string]interface{}{"success": true, "msg": ""})
		case r.URL.Path == "/xui/API/inbounds/addClient":
			var body struct {
				Settings string `json:"settings"`
			}
			_ = json.NewDecoder(r.Body).Decode(&body)
			var parsed struct {
				Clients []struct {
					ID     string `json:"id"`
					Enable bool   `json:"enable"`
				} `json:"clients"`
			}
			_ = json.Unmarshal([]byte(body.Settings), &parsed)
			if len(parsed.Clients) > 0 && added != nil {
				added[parsed.Clients[0].ID] = parsed.Clients[0].Enable
			}
			w.Header().Set("Content-Type", "application/json")
			_ = json.NewEncoder(w).Encode(map[string]interface{}{"success": true, "msg": ""})
		case len(r.URL.Path) > len("/xui/API/inbounds/updateClient/") && r.URL.Path[:len("/xui/API/inbounds/updateClient/")] == "/xui/API/inbounds/updateClient/":
			clientUUID := r.URL.Path[len("/xui/API/inbounds/updateClient/"):]
			if missingClientUUIDs[clientUUID] {
				w.Header().Set("Content-Type", "application/json")
				_ = json.NewEncoder(w).Encode(map[string]interface{}{"success": false, "msg": "Something went wrong! Failed: empty client ID\n"})
				return
			}
			var body struct {
				Settings string `json:"settings"`
			}
			_ = json.NewDecoder(r.Body).Decode(&body)
			var parsed struct {
				Clients []struct {
					Enable bool `json:"enable"`
				} `json:"clients"`
			}
			_ = json.Unmarshal([]byte(body.Settings), &parsed)
			if len(parsed.Clients) > 0 {
				updates[clientUUID] = parsed.Clients[0].Enable
			}
			w.Header().Set("Content-Type", "application/json")
			_ = json.NewEncoder(w).Encode(map[string]interface{}{"success": true, "msg": ""})
		case len(r.URL.Path) > len("/xui/API/inbounds/get/") && r.URL.Path[:len("/xui/API/inbounds/get/")] == "/xui/API/inbounds/get/":
			w.Header().Set("Content-Type", "application/json")
			_ = json.NewEncoder(w).Encode(map[string]interface{}{
				"success": true, "msg": "",
				"obj": map[string]interface{}{
					"id": 1, "settings": `{"clients":[]}`, "streamSettings": `{"security":"none"}`,
				},
			})
		default:
			w.WriteHeader(http.StatusNotFound)
		}
	}))
}

// TestApplyResellerV2RayQuota_DisablesAllPackagesOverReselleWideQuota
// confirms that once a reseller's summed usage across many packages
// exceeds their overall V2RayQuotaBytes, every active package is
// disabled (marked suspended_by_reseller_quota), even though none of
// them individually crossed their own per-package limit.
func TestApplyResellerV2RayQuota_DisablesAllPackagesOverResellerWideQuota(t *testing.T) {
	sync, db := newTestV2RaySyncService(t)

	quota := int64(800) // small numbers for a readable test, same ratio as the real incident (800GB quota, >1000GB used)
	reseller := model.Reseller{Name: "mi_925", Username: "mi_92500", PasswordHash: "x", V2RayQuotaBytes: &quota}
	if err := db.Create(&reseller).Error; err != nil {
		t.Fatalf("failed to create reseller: %v", err)
	}

	updates := make(map[string]bool)
	server := newFakeXuiPanelServer(t, updates)
	defer server.Close()

	panel := model.XuiPanel{
		Name: "panel-1", SaleTitle: "Panel 1", APIBaseURL: server.URL,
		Username: "admin", Password: "admin", DefaultInboundID: 1, Protocol: "vless",
		SubBaseURL: server.URL + "/sub", Status: "active",
	}
	if err := db.Create(&panel).Error; err != nil {
		t.Fatalf("failed to create panel: %v", err)
	}

	// Many small packages, each well under its own limit (mirrors the real
	// incident: no single package individually over quota), summing to
	// well past the reseller's own 800-unit pool.
	var packageIDs []uint
	for i := 0; i < 5; i++ {
		pkg := model.V2RayPackage{
			UUID: fmt.Sprintf("pkg-quota-%d", i), ResellerID: &reseller.ID,
			TotalVolumeBytes: 1000, DurationDays: 30, Status: "active",
		}
		if err := db.Create(&pkg).Error; err != nil {
			t.Fatalf("failed to create package %d: %v", i, err)
		}
		packageIDs = append(packageIDs, pkg.ID)

		loc := model.V2RayPackageLocation{
			PackageID: pkg.ID, PanelID: panel.ID,
			ClientUUID: fmt.Sprintf("client-%d", i), ClientEmail: fmt.Sprintf("c%d@test", i), SubID: fmt.Sprintf("sub-%d", i),
			Enabled: true, UsedBytesCached: 200, // 5 * 200 = 1000, well over the 800 reseller quota, but each package's own 200/1000 usage is nowhere near ITS OWN limit
		}
		if err := db.Create(&loc).Error; err != nil {
			t.Fatalf("failed to create location %d: %v", i, err)
		}
	}

	sync.applyResellerV2RayQuota(reseller.ID, 0)

	var updatedReseller model.Reseller
	if err := db.First(&updatedReseller, reseller.ID).Error; err != nil {
		t.Fatalf("failed to reload reseller: %v", err)
	}
	if updatedReseller.V2RayUsedBytes != 1000 {
		t.Errorf("expected reseller v2ray_used_bytes updated to 1000, got %d", updatedReseller.V2RayUsedBytes)
	}

	for _, pkgID := range packageIDs {
		var pkg model.V2RayPackage
		if err := db.First(&pkg, pkgID).Error; err != nil {
			t.Fatalf("failed to reload package %d: %v", pkgID, err)
		}
		if pkg.Status != "suspended" {
			t.Errorf("expected package %d to be suspended, got status %q", pkgID, pkg.Status)
		}
		if !pkg.SuspendedByResellerQuota {
			t.Errorf("expected package %d SuspendedByResellerQuota=true, got false", pkgID)
		}
		if !pkg.WasActiveBeforeSuspend {
			t.Errorf("expected package %d WasActiveBeforeSuspend=true, got false", pkgID)
		}
		if pkg.SuspendedByQuota {
			t.Errorf("expected package %d's OWN SuspendedByQuota to remain false (reseller-quota suspension is a different flag), got true", pkgID)
		}
	}

	for i := 0; i < 5; i++ {
		clientUUID := fmt.Sprintf("client-%d", i)
		if enabled, touched := updates[clientUUID]; !touched || enabled {
			t.Errorf("expected x-ui client %s to be disabled, touched=%v enabled=%v", clientUUID, touched, enabled)
		}
	}
}

// TestApplyResellerV2RayQuota_LeavesPackagesAloneUnderQuota confirms no
// package is touched while the reseller's summed usage stays under
// V2RayQuotaBytes -- the counterpart to the test above.
func TestApplyResellerV2RayQuota_LeavesPackagesAloneUnderQuota(t *testing.T) {
	sync, db := newTestV2RaySyncService(t)

	quota := int64(800)
	reseller := model.Reseller{Name: "under-quota", Username: "under-quota", PasswordHash: "x", V2RayQuotaBytes: &quota}
	if err := db.Create(&reseller).Error; err != nil {
		t.Fatalf("failed to create reseller: %v", err)
	}

	server := newFakeXuiPanelServer(t, make(map[string]bool))
	defer server.Close()

	panel := model.XuiPanel{
		Name: "panel-2", SaleTitle: "Panel 2", APIBaseURL: server.URL,
		Username: "admin", Password: "admin", DefaultInboundID: 1, Protocol: "vless",
		SubBaseURL: server.URL + "/sub", Status: "active",
	}
	if err := db.Create(&panel).Error; err != nil {
		t.Fatalf("failed to create panel: %v", err)
	}

	pkg := model.V2RayPackage{UUID: "pkg-under-quota", ResellerID: &reseller.ID, TotalVolumeBytes: 1000, DurationDays: 30, Status: "active"}
	if err := db.Create(&pkg).Error; err != nil {
		t.Fatalf("failed to create package: %v", err)
	}
	loc := model.V2RayPackageLocation{
		PackageID: pkg.ID, PanelID: panel.ID,
		ClientUUID: "client-under", ClientEmail: "under@test", SubID: "sub-under",
		Enabled: true, UsedBytesCached: 500, // under the 800 reseller quota
	}
	if err := db.Create(&loc).Error; err != nil {
		t.Fatalf("failed to create location: %v", err)
	}

	sync.applyResellerV2RayQuota(reseller.ID, 0)

	var reloaded model.V2RayPackage
	if err := db.First(&reloaded, pkg.ID).Error; err != nil {
		t.Fatalf("failed to reload package: %v", err)
	}
	if reloaded.Status != "active" || reloaded.SuspendedByResellerQuota {
		t.Errorf("expected package to remain active and unsuspended while under reseller quota, got status=%q suspended_by_reseller_quota=%v",
			reloaded.Status, reloaded.SuspendedByResellerQuota)
	}
}

// TestApplyResellerV2RayQuota_NilQuotaNeverSuspends confirms a reseller
// with no V2RayQuotaBytes configured (nil = unlimited) is never
// suspended regardless of usage -- matching every other quota pool's own
// nil-means-unlimited convention in this codebase.
func TestApplyResellerV2RayQuota_NilQuotaNeverSuspends(t *testing.T) {
	sync, db := newTestV2RaySyncService(t)

	reseller := model.Reseller{Name: "unlimited", Username: "unlimited-v2ray", PasswordHash: "x", V2RayQuotaBytes: nil}
	if err := db.Create(&reseller).Error; err != nil {
		t.Fatalf("failed to create reseller: %v", err)
	}

	server := newFakeXuiPanelServer(t, make(map[string]bool))
	defer server.Close()

	panel := model.XuiPanel{
		Name: "panel-3", SaleTitle: "Panel 3", APIBaseURL: server.URL,
		Username: "admin", Password: "admin", DefaultInboundID: 1, Protocol: "vless",
		SubBaseURL: server.URL + "/sub", Status: "active",
	}
	if err := db.Create(&panel).Error; err != nil {
		t.Fatalf("failed to create panel: %v", err)
	}

	pkg := model.V2RayPackage{UUID: "pkg-unlimited", ResellerID: &reseller.ID, TotalVolumeBytes: 1_000_000_000_000, DurationDays: 30, Status: "active"}
	if err := db.Create(&pkg).Error; err != nil {
		t.Fatalf("failed to create package: %v", err)
	}
	loc := model.V2RayPackageLocation{
		PackageID: pkg.ID, PanelID: panel.ID,
		ClientUUID: "client-unlimited", ClientEmail: "unlimited@test", SubID: "sub-unlimited",
		Enabled: true, UsedBytesCached: 999_999_999_999,
	}
	if err := db.Create(&loc).Error; err != nil {
		t.Fatalf("failed to create location: %v", err)
	}

	sync.applyResellerV2RayQuota(reseller.ID, 0)

	var reloaded model.V2RayPackage
	if err := db.First(&reloaded, pkg.ID).Error; err != nil {
		t.Fatalf("failed to reload package: %v", err)
	}
	if reloaded.Status != "active" {
		t.Errorf("expected package to remain active for a reseller with no V2RayQuotaBytes configured, got status %q", reloaded.Status)
	}
}

// TestApplyResellerV2RayQuota_DeletedPackageUsageNeverCountsTowardEnforcement
// is the regression test for a confirmed, reported production incident
// (reseller "Mohammadreza", VOLUME-mode): quota enforcement compared
// totalUsed (the STORED total, which permanently includes
// V2RayDeletedUsageBytes -- credited at delete time so a reseller can't
// dodge Payment-mode tiered pricing or erase usage history by deleting a
// package) against V2RayQuotaBytes, instead of the LIVE sum over
// currently-existing packages. A reseller already at/near their quota who
// deleted an old package -- expecting to free up headroom, exactly like
// deleting a WireGuard peer or User Manager account already does for
// their own quota pools -- instead got permanently stuck over quota
// (deleting the package moved its usage from the live sum into
// V2RayDeletedUsageBytes without changing the STORED total at all), with
// no way back short of an admin manually raising V2RayQuotaBytes. This
// confirms the fix: a reseller whose live usage is comfortably under
// quota is never suspended, even with a large V2RayDeletedUsageBytes
// credit sitting on the stored total from an earlier deletion.
func TestApplyResellerV2RayQuota_DeletedPackageUsageNeverCountsTowardEnforcement(t *testing.T) {
	sync, db := newTestV2RaySyncService(t)

	quota := int64(200) // small unit, same ratio as the real incident (200GB quota)
	reseller := model.Reseller{
		Name: "mohammadreza-like", Username: "mohammadreza-like", PasswordHash: "x",
		V2RayQuotaBytes: &quota,
		// Simulates a package deleted earlier today whose 130-unit usage
		// was folded into this credit by DeletePackage -- comfortably over
		// the 200-unit quota BY ITSELF once added to any live usage at all,
		// exactly like Mohammadreza's real ~24.6GB credit sat on top of a
		// live sum that was actually well under 200GB.
		V2RayDeletedUsageBytes: 130,
	}
	if err := db.Create(&reseller).Error; err != nil {
		t.Fatalf("failed to create reseller: %v", err)
	}

	updates := make(map[string]bool)
	server := newFakeXuiPanelServer(t, updates)
	defer server.Close()

	panel := model.XuiPanel{
		Name: "panel-deleted-usage", SaleTitle: "Panel", APIBaseURL: server.URL,
		Username: "admin", Password: "admin", DefaultInboundID: 1, Protocol: "vless",
		SubBaseURL: server.URL + "/sub", Status: "active",
	}
	if err := db.Create(&panel).Error; err != nil {
		t.Fatalf("failed to create panel: %v", err)
	}

	// Only 50 units of LIVE usage remain across this reseller's still-
	// existing packages -- comfortably under the 200-unit quota on its
	// own, and even 50+130=180 (live + deleted credit) would still be
	// under quota here on purpose, so the assertion below is unambiguous:
	// any use of the stored total (which the OLD code would recompute as
	// live+deleted-credit=180, still under 200 in THIS example) wouldn't
	// even expose the bug. Bump live usage to 90 so live+deleted=220 (over
	// quota) while live alone (90) stays comfortably under -- the only
	// way the assertion can pass is if enforcement used the live sum.
	pkg := model.V2RayPackage{UUID: "pkg-live-only", ResellerID: &reseller.ID, TotalVolumeBytes: 1000, DurationDays: 30, Status: "active"}
	if err := db.Create(&pkg).Error; err != nil {
		t.Fatalf("failed to create package: %v", err)
	}
	loc := model.V2RayPackageLocation{
		PackageID: pkg.ID, PanelID: panel.ID,
		ClientUUID: "client-live-only", ClientEmail: "live-only@test", SubID: "sub-live-only",
		Enabled: true, UsedBytesCached: 90,
	}
	if err := db.Create(&loc).Error; err != nil {
		t.Fatalf("failed to create location: %v", err)
	}

	sync.applyResellerV2RayQuota(reseller.ID, 0)

	var reloadedPkg model.V2RayPackage
	if err := db.First(&reloadedPkg, pkg.ID).Error; err != nil {
		t.Fatalf("failed to reload package: %v", err)
	}
	if reloadedPkg.Status != "active" || reloadedPkg.SuspendedByResellerQuota {
		t.Fatalf("expected package to remain active (live usage 90 is under the 200-unit quota), but it was suspended -- "+
			"got status=%q suspended_by_reseller_quota=%v (bug: enforcement compared against live+deleted=220 instead of live=90)",
			reloadedPkg.Status, reloadedPkg.SuspendedByResellerQuota)
	}
	if _, touched := updates["client-live-only"]; touched {
		t.Error("expected the x-ui client to never be touched (never suspended)")
	}

	// The STORED total must still include the deleted-usage credit (unlike
	// the enforcement check above) -- Payment-mode tiered pricing and usage
	// reports still need it, only quota ENFORCEMENT must ignore it.
	var reloadedReseller model.Reseller
	if err := db.First(&reloadedReseller, reseller.ID).Error; err != nil {
		t.Fatalf("failed to reload reseller: %v", err)
	}
	if reloadedReseller.V2RayUsedBytes != 220 {
		t.Errorf("expected stored V2RayUsedBytes to still be live(90)+deleted-credit(130)=220, got %d", reloadedReseller.V2RayUsedBytes)
	}
}

// TestResumePackagesForResellerQuota_ReenablesOnlyResellerQuotaSuspended
// confirms the resume path targets exactly SuspendedByResellerQuota
// packages -- a package suspended by its OWN TotalVolumeBytes limit
// (SuspendedByQuota) must NOT be resumed by this call, since the two are
// unrelated causes (see V2RayPackage.SuspendedByResellerQuota's own doc
// comment on why conflating them was itself a bug for WireGuard/
// UserManager, now avoided here).
func TestResumePackagesForResellerQuota_ReenablesOnlyResellerQuotaSuspended(t *testing.T) {
	sync, db := newTestV2RaySyncService(t)

	reseller := model.Reseller{Name: "resume-test", Username: "resume-test", PasswordHash: "x"}
	if err := db.Create(&reseller).Error; err != nil {
		t.Fatalf("failed to create reseller: %v", err)
	}

	updates := make(map[string]bool)
	server := newFakeXuiPanelServer(t, updates)
	defer server.Close()

	panel := model.XuiPanel{
		Name: "panel-4", SaleTitle: "Panel 4", APIBaseURL: server.URL,
		Username: "admin", Password: "admin", DefaultInboundID: 1, Protocol: "vless",
		SubBaseURL: server.URL + "/sub", Status: "active",
	}
	if err := db.Create(&panel).Error; err != nil {
		t.Fatalf("failed to create panel: %v", err)
	}

	resellerQuotaSuspended := model.V2RayPackage{
		UUID: "pkg-resume-a", ResellerID: &reseller.ID, TotalVolumeBytes: 1000, DurationDays: 30,
		Status: "suspended", SuspendedByResellerQuota: true, WasActiveBeforeSuspend: true,
	}
	ownLimitSuspended := model.V2RayPackage{
		UUID: "pkg-resume-b", ResellerID: &reseller.ID, TotalVolumeBytes: 1000, DurationDays: 30,
		Status: "suspended", SuspendedByQuota: true, WasActiveBeforeSuspend: true,
	}
	if err := db.Create(&resellerQuotaSuspended).Error; err != nil {
		t.Fatalf("failed to create reseller-quota-suspended package: %v", err)
	}
	if err := db.Create(&ownLimitSuspended).Error; err != nil {
		t.Fatalf("failed to create own-limit-suspended package: %v", err)
	}

	locA := model.V2RayPackageLocation{PackageID: resellerQuotaSuspended.ID, PanelID: panel.ID, ClientUUID: "client-a", ClientEmail: "a@test", SubID: "sub-a"}
	locB := model.V2RayPackageLocation{PackageID: ownLimitSuspended.ID, PanelID: panel.ID, ClientUUID: "client-b", ClientEmail: "b@test", SubID: "sub-b"}
	if err := db.Create(&locA).Error; err != nil {
		t.Fatalf("failed to create location A: %v", err)
	}
	if err := db.Create(&locB).Error; err != nil {
		t.Fatalf("failed to create location B: %v", err)
	}

	sync.ResumePackagesForResellerQuota(reseller.ID)

	var reloadedA model.V2RayPackage
	if err := db.First(&reloadedA, resellerQuotaSuspended.ID).Error; err != nil {
		t.Fatalf("failed to reload package A: %v", err)
	}
	if reloadedA.Status != "active" || reloadedA.SuspendedByResellerQuota {
		t.Errorf("expected reseller-quota-suspended package to be resumed, got status=%q suspended_by_reseller_quota=%v",
			reloadedA.Status, reloadedA.SuspendedByResellerQuota)
	}

	var reloadedB model.V2RayPackage
	if err := db.First(&reloadedB, ownLimitSuspended.ID).Error; err != nil {
		t.Fatalf("failed to reload package B: %v", err)
	}
	if reloadedB.Status != "suspended" || !reloadedB.SuspendedByQuota {
		t.Errorf("expected own-limit-suspended package to remain untouched, got status=%q suspended_by_quota=%v",
			reloadedB.Status, reloadedB.SuspendedByQuota)
	}

	if enabled, touched := updates["client-a"]; !touched || !enabled {
		t.Errorf("expected client-a to be re-enabled, touched=%v enabled=%v", touched, enabled)
	}
	if _, touched := updates["client-b"]; touched {
		t.Errorf("expected client-b to NOT be touched by the reseller-quota resume call")
	}
}

// TestResumePackagesForResellerQuota_RecreatesClientMissingFromPanel
// verifies that resuming a package whose x-ui client was removed
// directly on the panel (so updateClient rejects the re-enable) falls
// back to recreating the client with the same uuid/email/subID, so the
// customer's existing config link keeps working, and the location is
// still correctly marked enabled afterward.
func TestResumePackagesForResellerQuota_RecreatesClientMissingFromPanel(t *testing.T) {
	sync, db := newTestV2RaySyncService(t)

	reseller := model.Reseller{Name: "missing-client-test", Username: "missing-client-test", PasswordHash: "x"}
	if err := db.Create(&reseller).Error; err != nil {
		t.Fatalf("failed to create reseller: %v", err)
	}

	updates := make(map[string]bool)
	added := make(map[string]bool)
	server := newFakeXuiPanelServerWithMissingClients(t, updates, map[string]bool{"client-missing": true}, added)
	defer server.Close()

	panel := model.XuiPanel{
		Name: "panel-5", SaleTitle: "Panel 5", APIBaseURL: server.URL,
		Username: "admin", Password: "admin", DefaultInboundID: 1, Protocol: "vless",
		SubBaseURL: server.URL + "/sub", Status: "active",
	}
	if err := db.Create(&panel).Error; err != nil {
		t.Fatalf("failed to create panel: %v", err)
	}

	pkg := model.V2RayPackage{
		UUID: "pkg-missing-client", ResellerID: &reseller.ID, TotalVolumeBytes: 1000, DurationDays: 30,
		Status: "suspended", SuspendedByResellerQuota: true, WasActiveBeforeSuspend: true,
	}
	if err := db.Create(&pkg).Error; err != nil {
		t.Fatalf("failed to create package: %v", err)
	}
	loc := model.V2RayPackageLocation{
		PackageID: pkg.ID, PanelID: panel.ID,
		ClientUUID: "client-missing", ClientEmail: "missing@test", SubID: "sub-missing",
		Enabled: false,
	}
	if err := db.Create(&loc).Error; err != nil {
		t.Fatalf("failed to create location: %v", err)
	}

	sync.ResumePackagesForResellerQuota(reseller.ID)

	var reloadedPkg model.V2RayPackage
	if err := db.First(&reloadedPkg, pkg.ID).Error; err != nil {
		t.Fatalf("failed to reload package: %v", err)
	}
	if reloadedPkg.Status != "active" || reloadedPkg.SuspendedByResellerQuota {
		t.Errorf("expected package to be resumed, got status=%q suspended_by_reseller_quota=%v",
			reloadedPkg.Status, reloadedPkg.SuspendedByResellerQuota)
	}

	var reloadedLoc model.V2RayPackageLocation
	if err := db.First(&reloadedLoc, loc.ID).Error; err != nil {
		t.Fatalf("failed to reload location: %v", err)
	}
	if !reloadedLoc.Enabled {
		t.Error("expected the location to be marked enabled after the addClient fallback succeeded")
	}

	if _, touchedUpdate := updates["client-missing"]; touchedUpdate {
		t.Error("expected updateClient to have been rejected (simulated missing client), not recorded as a successful update")
	}
	if enabled, touchedAdd := added["client-missing"]; !touchedAdd || !enabled {
		t.Errorf("expected addClient to recreate the missing client with Enable=true, touched=%v enabled=%v", touchedAdd, enabled)
	}
}

// TestRepairDisabledLocationsForPackage_FixesAlreadyActivePackage verifies
// RepairDisabledLocationsForPackage can fix a package that already has
// status="active" and every suspend flag clear, but still has one or
// more locations genuinely disabled (Enabled=false) because an earlier
// resume's x-ui push failed -- a case neither
// ResumePackagesForResellerQuota nor ResumePackagesForReseller can find
// again, since both query by the suspend flags, which are already clear.
func TestRepairDisabledLocationsForPackage_FixesAlreadyActivePackage(t *testing.T) {
	sync, db := newTestV2RaySyncService(t)

	updates := make(map[string]bool)
	added := make(map[string]bool)
	server := newFakeXuiPanelServerWithMissingClients(t, updates, map[string]bool{"client-orphan": true}, added)
	defer server.Close()

	panelA := model.XuiPanel{
		Name: "panel-6a", SaleTitle: "Panel 6a", APIBaseURL: server.URL,
		Username: "admin", Password: "admin", DefaultInboundID: 1, Protocol: "vless",
		SubBaseURL: server.URL + "/sub", Status: "active",
	}
	panelB := model.XuiPanel{
		Name: "panel-6b", SaleTitle: "Panel 6b", APIBaseURL: server.URL,
		Username: "admin", Password: "admin", DefaultInboundID: 1, Protocol: "vless",
		SubBaseURL: server.URL + "/sub", Status: "active",
	}
	if err := db.Create(&panelA).Error; err != nil {
		t.Fatalf("failed to create panel A: %v", err)
	}
	if err := db.Create(&panelB).Error; err != nil {
		t.Fatalf("failed to create panel B: %v", err)
	}

	// Already fully "active," no suspend flags set -- exactly the
	// already-resumed-but-broken state found live in production. Two
	// locations on two DIFFERENT panels (package_id+panel_id is unique per
	// V2RayPackageLocation), mirroring how the real incident had one
	// package with locations spread across multiple panels, only some of
	// which were actually orphaned.
	pkg := model.V2RayPackage{
		UUID: "pkg-already-active", TotalVolumeBytes: 1000, DurationDays: 30, Status: "active",
	}
	if err := db.Create(&pkg).Error; err != nil {
		t.Fatalf("failed to create package: %v", err)
	}

	healthyLoc := model.V2RayPackageLocation{
		PackageID: pkg.ID, PanelID: panelA.ID,
		ClientUUID: "client-healthy", ClientEmail: "healthy@test", SubID: "sub-healthy",
		Enabled: true,
	}
	orphanLoc := model.V2RayPackageLocation{
		PackageID: pkg.ID, PanelID: panelB.ID,
		ClientUUID: "client-orphan", ClientEmail: "orphan@test", SubID: "sub-orphan",
		Enabled: false,
	}
	if err := db.Create(&healthyLoc).Error; err != nil {
		t.Fatalf("failed to create healthy location: %v", err)
	}
	if err := db.Create(&orphanLoc).Error; err != nil {
		t.Fatalf("failed to create orphaned location: %v", err)
	}

	if err := sync.RepairDisabledLocationsForPackage(pkg.ID); err != nil {
		t.Fatalf("RepairDisabledLocationsForPackage failed: %v", err)
	}

	var reloadedOrphan model.V2RayPackageLocation
	if err := db.First(&reloadedOrphan, orphanLoc.ID).Error; err != nil {
		t.Fatalf("failed to reload orphaned location: %v", err)
	}
	if !reloadedOrphan.Enabled {
		t.Error("expected the orphaned location to be marked enabled after repair")
	}

	if enabled, touchedAdd := added["client-orphan"]; !touchedAdd || !enabled {
		t.Errorf("expected addClient to recreate the orphaned client, touched=%v enabled=%v", touchedAdd, enabled)
	}
	if _, touchedUpdate := updates["client-healthy"]; touchedUpdate {
		t.Error("expected the already-healthy location to be left untouched (only Enabled=false locations should be repaired)")
	}
}
