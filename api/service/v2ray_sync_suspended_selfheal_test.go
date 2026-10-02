package service

import (
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	"github.com/maahdima/mwp/api/dataservice/model"
)

// TestSyncPackageUsage_ReEvaluatesSuspendedPackageWithNoEnabledLocations is
// the regression test for a confirmed, reported bug: SyncPackageUsage's
// per-tick loop over locations only ever visits currently enabled=true
// rows, and enforcePackageQuota (the ONLY function that knows how to
// re-check BillingSuspended and call reenablePackageLocations) is only
// ever invoked for packages reached through that same loop. Once every one
// of a package's locations was disabled, the package could never be
// reached again by any future tick -- so even after
// ResumeBillingSuspension cleared the reseller's own BillingSuspended
// flag, the package stayed suspended forever, recoverable only via a
// manual admin/API resume. This test creates exactly that state (a
// Payment-mode reseller, solvent, BillingSuspended already cleared, but a
// package still marked suspended_by_quota=true with its one location
// still disabled=false) and confirms a single SyncPackageUsage call heals
// it without needing that location to already be enabled.
func TestSyncPackageUsage_ReEvaluatesSuspendedPackageWithNoEnabledLocations(t *testing.T) {
	sync, db := newTestV2RaySyncService(t)

	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		switch {
		case r.URL.Path == "/login":
			http.SetCookie(w, &http.Cookie{Name: "3x-ui", Value: "tok", Path: "/"})
			w.Header().Set("Content-Type", "application/json")
			_ = json.NewEncoder(w).Encode(map[string]interface{}{"success": true, "msg": ""})
		case strings.HasPrefix(r.URL.Path, "/xui/API/inbounds/get/"):
			w.Header().Set("Content-Type", "application/json")
			_ = json.NewEncoder(w).Encode(map[string]interface{}{
				"success": true, "msg": "",
				"obj": map[string]interface{}{"id": 1, "settings": `{"clients":[]}`},
			})
		case strings.HasPrefix(r.URL.Path, "/xui/API/inbounds/updateClient/"):
			w.Header().Set("Content-Type", "application/json")
			_ = json.NewEncoder(w).Encode(map[string]interface{}{"success": true, "msg": ""})
		default:
			w.WriteHeader(http.StatusNotFound)
		}
	}))
	defer server.Close()

	panel := model.XuiPanel{
		Name: "selfheal-panel", SaleTitle: "SelfHeal", APIBaseURL: server.URL,
		Username: "admin", Password: "admin", DefaultInboundID: 1, Protocol: "vless",
		SubBaseURL: server.URL + "/sub", Status: "active",
	}
	if err := db.Create(&panel).Error; err != nil {
		t.Fatalf("failed to create panel: %v", err)
	}

	reseller := model.Reseller{
		Name: "selfheal-reseller", Username: "selfheal-reseller", PasswordHash: "x",
		BillingMode:      model.ResellerBillingModePayment,
		BillingSuspended: false, // already resumed -- e.g. by a successful ChargeUsage debit
	}
	if err := db.Create(&reseller).Error; err != nil {
		t.Fatalf("failed to create reseller: %v", err)
	}

	pkg := model.V2RayPackage{
		ResellerID:              &reseller.ID,
		Status:                  "suspended",
		SuspendedByQuota:        true,
		WasActiveBeforeSuspend:  true,
	}
	if err := db.Create(&pkg).Error; err != nil {
		t.Fatalf("failed to create package: %v", err)
	}

	loc := model.V2RayPackageLocation{
		PackageID: pkg.ID, PanelID: panel.ID,
		ClientUUID: "c-selfheal", ClientEmail: "selfheal@test", SubID: "s-selfheal",
		Enabled:        false, // the actual bug condition: excluded from the enabled=true sync loop
		FlowRepairedAt: timeNow(),
	}
	if err := db.Create(&loc).Error; err != nil {
		t.Fatalf("failed to create location: %v", err)
	}

	sync.SyncPackageUsage()

	var gotPkg model.V2RayPackage
	if err := db.First(&gotPkg, pkg.ID).Error; err != nil {
		t.Fatalf("failed to reload package: %v", err)
	}
	if gotPkg.Status != "active" {
		t.Fatalf("expected package to be re-enabled to status=active, got %q", gotPkg.Status)
	}
	if gotPkg.SuspendedByQuota {
		t.Fatal("expected SuspendedByQuota to be cleared after self-heal")
	}

	var gotLoc model.V2RayPackageLocation
	if err := db.First(&gotLoc, loc.ID).Error; err != nil {
		t.Fatalf("failed to reload location: %v", err)
	}
	if !gotLoc.Enabled {
		t.Fatal("expected the location to be re-enabled on both x-ui and in the database")
	}
}
