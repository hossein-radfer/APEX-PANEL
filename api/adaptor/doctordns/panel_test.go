package doctordns

import (
	"context"
	"crypto/tls"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	"github.com/maahdima/mwp/api/dataservice/model"
)

func testPanel(baseURL string) model.DNSPanel {
	return model.DNSPanel{APIBaseURL: baseURL, APIKey: "test-key"}
}

// TestDoJSON_SendsNonEmptySNI is a regression test for a real, reproduced
// production incident: DNSPanel.APIBaseURL is always a bare IP address (a
// doctor-dns exit node has no DNS name), and Go's TLS client correctly
// omits the SNI extension entirely for a literal-IP destination per RFC
// 6066. Live testing against a real exit node from an Iranian relay server
// found that the network resets ANY TLS handshake with no SNI present,
// regardless of port -- so a missing SNI silently broke every /apex/ call
// from Iran specifically, while working fine from every other network
// tested. This test asserts the client hello the adaptor actually sends
// carries a non-empty ServerName, using httptest.NewTLSServer plus a
// TLS-inspecting listener wrapper to capture the real negotiated
// connection state (a bare httptest server does not expose this on its
// own).
func TestDoJSON_SendsNonEmptySNI(t *testing.T) {
	var gotServerName string
	server := httptest.NewUnstartedServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.TLS != nil {
			gotServerName = r.TLS.ServerName
		}
		w.Header().Set("Content-Type", "application/json")
		_ = json.NewEncoder(w).Encode(map[string]interface{}{"templates": []interface{}{}})
	}))
	server.TLS = &tls.Config{}
	server.StartTLS()
	defer server.Close()

	if _, err := ListTemplates(context.Background(), testPanel(server.URL)); err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if gotServerName == "" {
		t.Fatal("expected a non-empty TLS SNI ServerName on the outgoing handshake, got empty -- this is the exact condition that gets reset by Iranian network filtering against a bare-IP APIBaseURL")
	}
}

// TestCreateOrUpdateUser_SendsBearerAndDecodesUser confirms the adaptor
// authenticates with the panel's own APIKey (not SYNC_SECRET or any other
// value) and correctly decodes doctor-dns's real _apex_user_view shape.
func TestCreateOrUpdateUser_SendsBearerAndDecodesUser(t *testing.T) {
	var gotAuth, gotPath string
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		gotAuth = r.Header.Get("Authorization")
		gotPath = r.URL.Path
		w.Header().Set("Content-Type", "application/json")
		_ = json.NewEncoder(w).Encode(map[string]interface{}{
			"ok": true,
			"user": map[string]interface{}{
				"apex_ref":    "apex:r1:dns:abcd1234",
				"status":      "active",
				"quota_bytes": 1073741824,
				"used_bytes":  0,
				"speed_kbps":  0,
				"expires_at":  nil,
				"template_id": nil,
				"max_ips":     2,
				"ip":          nil,
			},
		})
	}))
	defer server.Close()

	user, err := CreateOrUpdateUser(context.Background(), testPanel(server.URL), "apex:r1:dns:abcd1234", "label", 1073741824, 0, nil, nil, 2)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if gotAuth != "Bearer test-key" {
		t.Fatalf("expected Authorization %q, got %q", "Bearer test-key", gotAuth)
	}
	if gotPath != "/apex/user-create" {
		t.Fatalf("expected path /apex/user-create, got %q", gotPath)
	}
	if user.Status != "active" || user.MaxIPs != 2 || user.QuotaBytes != 1073741824 {
		t.Fatalf("unexpected decoded user: %+v", user)
	}
}

// TestDoJSON_SurfacesDoctorDNSErrorMessage confirms a non-200 response
// carrying doctor-dns's own {"error": "..."} shape is surfaced in the
// returned error's message, not just the bare HTTP status.
func TestDoJSON_SurfacesDoctorDNSErrorMessage(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		w.WriteHeader(http.StatusNotFound)
		_ = json.NewEncoder(w).Encode(map[string]string{"error": "no such apex_ref"})
	}))
	defer server.Close()

	_, err := SetUserStatus(context.Background(), testPanel(server.URL), "apex:missing", "active")
	if err == nil {
		t.Fatal("expected an error, got nil")
	}
	if !strings.Contains(err.Error(), "no such apex_ref") {
		t.Fatalf("expected the doctor-dns error message to be surfaced, got: %v", err)
	}
}

// TestDoJSON_UnauthorisedIsClear confirms a 401 (wrong/missing APIKey)
// produces a clear, actionable message rather than a generic HTTP-status
// error.
func TestDoJSON_UnauthorisedIsClear(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusUnauthorized)
	}))
	defer server.Close()

	_, err := ListTemplates(context.Background(), testPanel(server.URL))
	if err == nil {
		t.Fatal("expected an error, got nil")
	}
	if !strings.Contains(err.Error(), "API key") {
		t.Fatalf("expected a clear API-key-related message, got: %v", err)
	}
}

// TestClaimIP_ReturnsOkFalseAsValueNotError confirms doctor-dns's routine
// "ip already claimed by another account" rejection surfaces as
// ClaimIPResult.Ok=false, not a Go error -- callers must be able to show
// this message to the customer, not just log a failure.
func TestClaimIP_ReturnsOkFalseAsValueNotError(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		_ = json.NewEncoder(w).Encode(map[string]interface{}{
			"ok":      false,
			"message": "این آی‌پی به حساب دیگری ثبت شده است",
		})
	}))
	defer server.Close()

	result, err := ClaimIP(context.Background(), testPanel(server.URL), "apex:r1:dns:abcd1234", "203.0.113.5")
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if result.Ok {
		t.Fatalf("expected Ok=false, got true")
	}
	if result.Message == "" {
		t.Fatal("expected a non-empty rejection message")
	}
}

// TestListTemplates_DecodesTemplateList confirms the /apex/templates
// response shape decodes correctly.
func TestListTemplates_DecodesTemplateList(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		_ = json.NewEncoder(w).Encode(map[string]interface{}{
			"templates": []map[string]interface{}{
				{"id": 1, "name": "کامل", "is_default": true},
				{"id": 2, "name": "پایه", "is_default": false},
			},
		})
	}))
	defer server.Close()

	templates, err := ListTemplates(context.Background(), testPanel(server.URL))
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if len(templates) != 2 || !templates[0].IsDefault || templates[1].IsDefault {
		t.Fatalf("unexpected templates: %+v", templates)
	}
}
