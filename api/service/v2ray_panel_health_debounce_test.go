package service

import (
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"testing"
	"time"

	"github.com/maahdima/mwp/api/dataservice/model"
)

// newFailingXuiPanel points at a real httptest server that always rejects
// login (success=false, matching x-ui's documented behavior of returning
// HTTP 200 with success=false on a rejected login -- see xui.Login's own
// doc comment), so pollOnePanel's loginErr is deterministic without
// needing a real x-ui instance.
func newFailingXuiLoginServer(t *testing.T) *httptest.Server {
	t.Helper()
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		_ = json.NewEncoder(w).Encode(map[string]interface{}{"success": false, "msg": "invalid credentials"})
	}))
	t.Cleanup(srv.Close)
	return srv
}

// TestPollOnePanel_DebouncesTransientLoginFailureBeforeAlerting is the
// regression test for the confirmed, reported bug: a single failed login
// attempt used to immediately flip a healthy panel to "error" and fire a
// Telegram alert, with zero debounce. panelHealthFailureThreshold now
// requires that many CONSECUTIVE failures first.
func TestPollOnePanel_DebouncesTransientLoginFailureBeforeAlerting(t *testing.T) {
	svc, db := newTestV2RayPanelHealthService(t)
	srv := newFailingXuiLoginServer(t)

	panel := model.XuiPanel{
		Name: "Panel F", SaleTitle: "Panel F", APIBaseURL: srv.URL, Username: "u", Password: "p",
		DefaultInboundID: 1, Protocol: "vless", SubBaseURL: srv.URL + "/sub",
		Status: "active",
	}
	if err := db.Create(&panel).Error; err != nil {
		t.Fatalf("failed to create panel: %v", err)
	}

	// First failure: must stay "active" (debounced), not yet alerted.
	svc.pollOnePanel(panel, time.Now())
	var afterFirst model.XuiPanel
	if err := db.First(&afterFirst, panel.ID).Error; err != nil {
		t.Fatalf("failed to reload panel: %v", err)
	}
	if afterFirst.Status != "active" {
		t.Fatalf("expected status to stay 'active' after a single failure (debounced), got %q", afterFirst.Status)
	}
	if afterFirst.ConsecutiveLoginFailures != 1 {
		t.Errorf("expected ConsecutiveLoginFailures=1 after one failure, got %d", afterFirst.ConsecutiveLoginFailures)
	}

	// Second consecutive failure reaches panelHealthFailureThreshold (2):
	// now it must actually flip to "error".
	svc.pollOnePanel(afterFirst, time.Now())
	var afterSecond model.XuiPanel
	if err := db.First(&afterSecond, panel.ID).Error; err != nil {
		t.Fatalf("failed to reload panel: %v", err)
	}
	if afterSecond.Status != "error" {
		t.Fatalf("expected status to flip to 'error' after %d consecutive failures, got %q", panelHealthFailureThreshold, afterSecond.Status)
	}
}

// TestPollOnePanel_SingleSuccessResetsFailureStreakImmediately confirms
// the asymmetric recovery behavior: unlike the failure path, a single
// successful login immediately resets the streak and marks the panel
// healthy again -- "fast recovery, slow failure."
func TestPollOnePanel_SingleSuccessResetsFailureStreakImmediately(t *testing.T) {
	svc, db := newTestV2RayPanelHealthService(t)
	srv := newFailingXuiLoginServer(t)

	panel := model.XuiPanel{
		Name: "Panel G", SaleTitle: "Panel G", APIBaseURL: srv.URL, Username: "u", Password: "p",
		DefaultInboundID: 1, Protocol: "vless", SubBaseURL: srv.URL + "/sub",
		Status: "active", ConsecutiveLoginFailures: 1,
	}
	if err := db.Create(&panel).Error; err != nil {
		t.Fatalf("failed to create panel: %v", err)
	}

	// Point the panel at a server that always succeeds, confirming the
	// counter resets to 0 on the very next successful poll. A session
	// cookie is required -- xui.Login treats a success response with no
	// Set-Cookie header as a failure (see that function's own check).
	okSrv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		http.SetCookie(w, &http.Cookie{Name: "3x-ui", Value: "fake-session"})
		w.Header().Set("Content-Type", "application/json")
		_ = json.NewEncoder(w).Encode(map[string]interface{}{"success": true})
	}))
	t.Cleanup(okSrv.Close)
	panel.APIBaseURL = okSrv.URL
	if err := db.Save(&panel).Error; err != nil {
		t.Fatalf("failed to update panel url: %v", err)
	}

	svc.pollOnePanel(panel, time.Now())

	var after model.XuiPanel
	if err := db.First(&after, panel.ID).Error; err != nil {
		t.Fatalf("failed to reload panel: %v", err)
	}
	if after.ConsecutiveLoginFailures != 0 {
		t.Errorf("expected ConsecutiveLoginFailures to reset to 0 after a success, got %d", after.ConsecutiveLoginFailures)
	}
	if after.Status != "active" {
		t.Errorf("expected status 'active' after a successful login, got %q", after.Status)
	}
}
