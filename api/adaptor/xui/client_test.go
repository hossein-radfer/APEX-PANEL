package xui

import (
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	"github.com/maahdima/mwp/api/dataservice/model"
)

// TestLogin_EmptyBaseURLFailsClearly is a regression test for a real bug: a
// panel reaching Login with an empty APIBaseURL (e.g. from a request whose
// Content-Type caused Echo's binder to silently produce an all-zero-value
// struct upstream) previously failed with Go's own HTTP transport error
// ("unsupported protocol scheme \"\""), which gives no hint the root cause
// was simply a blank URL. Login now checks for this explicitly and fails
// with a clear message instead.
func TestLogin_EmptyBaseURLFailsClearly(t *testing.T) {
	_, err := Login(context.Background(), model.XuiPanel{
		APIBaseURL: "",
		Username:   "admin",
		Password:   "admin",
	})
	if err == nil {
		t.Fatal("expected an error for an empty APIBaseURL, got nil")
	}
	if !strings.Contains(err.Error(), "API base URL is empty") {
		t.Fatalf("expected a clear 'API base URL is empty' error, got: %v", err)
	}
	if strings.Contains(err.Error(), "unsupported protocol scheme") {
		t.Fatalf("expected the clear guard message, not the raw transport error: %v", err)
	}
}

// TestLogin_WrongCredentialsRejected is a regression test for a real bug
// caught by a user's live deployment: x-ui's real /login endpoint returns
// HTTP 200 even on a WRONG username/password, signaling failure only via
// success=false in the JSON body (confirmed against MHSanaei/3x-ui's
// source, web/controller/index.go). Login previously only checked the HTTP
// status code, so a rejected login was treated as successful, leading to
// every subsequent request being made with no valid session -- which is
// what produced the reported "invalid character '<' looking for beginning
// of value" error (the server serving its login page HTML instead of JSON
// to an unauthenticated follow-up request).
func TestLogin_WrongCredentialsRejected(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Path == "/login" {
			w.Header().Set("Content-Type", "application/json")
			w.WriteHeader(http.StatusOK) // x-ui always returns 200, even on bad credentials
			_ = json.NewEncoder(w).Encode(map[string]interface{}{
				"success": false,
				"msg":     "wrong username or password",
			})
			return
		}
		w.WriteHeader(http.StatusNotFound)
	}))
	defer server.Close()

	_, err := Login(context.Background(), model.XuiPanel{
		APIBaseURL: server.URL,
		Username:   "admin",
		Password:   "wrong-password",
	})
	if err == nil {
		t.Fatal("expected an error for credentials the panel rejected, got nil")
	}
	if !strings.Contains(err.Error(), "wrong username or password") {
		t.Fatalf("expected the panel's rejection message to be surfaced, got: %v", err)
	}
}

// TestLogin_SuccessUsesRealCookieName is a regression test for a second
// real bug: Login previously hardcoded the expected cookie name to
// "session" or "3x-ui", silently discarding any other name and sending a
// wrong/missing cookie on every subsequent request. x-ui's actual cookie
// name is application-defined (3x-ui's own source hardcodes the literal
// "3x-ui" via gin-contrib/sessions, but other forks/builds are not
// guaranteed to match) and must not be guessed. This test uses a
// deliberately different cookie name to prove the fix (a real
// net/http/cookiejar.Jar) replays whatever cookie the server actually set,
// under its real name, rather than one this client assumed.
func TestLogin_SuccessUsesRealCookieName(t *testing.T) {
	const unexpectedCookieName = "totally-different-cookie-name"

	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		switch r.URL.Path {
		case "/login":
			http.SetCookie(w, &http.Cookie{Name: unexpectedCookieName, Value: "authenticated-session-value", Path: "/"})
			w.Header().Set("Content-Type", "application/json")
			_ = json.NewEncoder(w).Encode(map[string]interface{}{"success": true, "msg": ""})
		case "/xui/API/inbounds/":
			cookie, err := r.Cookie(unexpectedCookieName)
			if err != nil || cookie.Value != "authenticated-session-value" {
				// Mirrors real x-ui's behavior of serving its login page
				// HTML to an unauthenticated request to this endpoint.
				w.Header().Set("Content-Type", "text/html")
				_, _ = w.Write([]byte("<html><body>login required</body></html>"))
				return
			}
			w.Header().Set("Content-Type", "application/json")
			_ = json.NewEncoder(w).Encode(map[string]interface{}{"success": true, "msg": "", "obj": []XuiInbound{}})
		default:
			w.WriteHeader(http.StatusNotFound)
		}
	}))
	defer server.Close()

	inbounds, err := ListInbounds(context.Background(), model.XuiPanel{
		APIBaseURL: server.URL,
		Username:   "admin",
		Password:   "admin",
	})
	if err != nil {
		t.Fatalf("expected the real (non-standard-named) session cookie to be replayed correctly, got error: %v", err)
	}
	if len(inbounds) != 0 {
		t.Fatalf("expected zero inbounds from the stub server, got %d", len(inbounds))
	}
}

// TestGetSubscription_NoLoginCall is a regression test for a real, live
// bug: GetSubscription previously called Login(ctx, panel) first and reused
// that authenticated *Client's cookie-jar-bearing httpClient to fetch the
// subscription URL. A real x-ui deployment serves the subscription endpoint
// (sub_base_url) on a completely separate, public, UNAUTHENTICATED server
// from the admin panel API (api_base_url) -- often the same hostname on a
// different port, which net/http/cookiejar (RFC 6265, host-scoped, not
// port-scoped) happily leaks the admin session cookie across. A
// subscription-side reverse proxy/WAF that rejects unrecognized foreign
// cookies then broke every subscription fetch permanently, which -- because
// the sync job treats a GetSubscription failure as non-fatal and keeps the
// previously cached (broken, raw-client-email-titled) link -- was the root
// cause behind a real customer's subscription URL serving a blank page and
// failing in v2rayNG/v2box.
//
// This test uses an XuiPanel whose APIBaseURL points at a server with NO
// /login handler at all (any request there 404s) -- proving GetSubscription
// truly never calls Login/the admin API, only the separate SubBaseURL
// server, exactly like a real V2Ray client app fetching a subscription URL.
func TestGetSubscription_NoLoginCall(t *testing.T) {
	adminServer := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		t.Errorf("GetSubscription must never call the admin panel API, but got a request to %s", r.URL.Path)
		w.WriteHeader(http.StatusNotFound)
	}))
	defer adminServer.Close()

	const rawContent = "vless://uuid-here@host:443?type=tcp#pkg_raw_email"
	subServer := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if len(r.Cookies()) != 0 {
			t.Errorf("expected no cookies on the unauthenticated subscription request, got %v", r.Cookies())
		}
		if r.URL.Path != "/sub/my-sub-id" {
			w.WriteHeader(http.StatusNotFound)
			return
		}
		_, _ = w.Write([]byte(rawContent))
	}))
	defer subServer.Close()

	got, err := GetSubscription(context.Background(), model.XuiPanel{
		// Deliberately NOT used by GetSubscription -- included only to
		// prove it's ignored (no Login call is made against it).
		APIBaseURL: adminServer.URL,
		Username:   "admin",
		Password:   "admin",
		SubBaseURL: subServer.URL + "/sub",
	}, "my-sub-id")
	if err != nil {
		t.Fatalf("expected an unauthenticated GET to the subscription server to succeed, got error: %v", err)
	}
	if got != rawContent {
		t.Fatalf("expected raw subscription content %q, got %q", rawContent, got)
	}
}

// TestGetOnlineClients_ParsesEmailList is a regression/contract test for
// the online-status polling feature: confirms GetOnlineClients logs in,
// POSTs to the onlines path, and turns x-ui's flat obj array of email
// strings into a lookup set -- including that an email NOT present in the
// response is correctly reported as not online (not just "unknown").
func TestGetOnlineClients_ParsesEmailList(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		switch {
		case r.URL.Path == "/login":
			http.SetCookie(w, &http.Cookie{Name: "3x-ui", Value: "tok", Path: "/"})
			w.Header().Set("Content-Type", "application/json")
			_ = json.NewEncoder(w).Encode(map[string]interface{}{"success": true, "msg": ""})
		case r.URL.Path == "/xui/API/inbounds/onlines" && r.Method == http.MethodPost:
			w.Header().Set("Content-Type", "application/json")
			_ = json.NewEncoder(w).Encode(map[string]interface{}{
				"success": true, "msg": "",
				"obj": []string{"pkg_abc123_1", "pkg_def456_1"},
			})
		default:
			w.WriteHeader(http.StatusNotFound)
		}
	}))
	defer server.Close()

	online, err := GetOnlineClients(context.Background(), model.XuiPanel{
		APIBaseURL: server.URL, Username: "admin", Password: "admin",
	})
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if !online["pkg_abc123_1"] || !online["pkg_def456_1"] {
		t.Fatalf("expected both stub-reported emails to be online, got %v", online)
	}
	if online["pkg_not_connected_1"] {
		t.Fatalf("expected an email absent from the stub's obj list to be reported as not online")
	}
}

// TestGetOnlineClients_ParsesAlirezaXuiObjectArrayShape is a regression
// test for a real, reported bug: a customer's second registered panel
// failed every online-status poll with "cannot unmarshal object into Go
// struct field ... of type string" because that panel runs alireza0/x-ui
// (a different fork than MHSanaei/3x-ui), whose /onlines endpoint returns
// `"obj": [{"email": "...", "ips": {...}}, ...]` -- an array of per-client
// objects, not TestGetOnlineClients_ParsesEmailList's flat array of email
// strings. Both shapes must work through the same GetOnlineClients call
// without the caller needing to know which fork a given panel runs.
func TestGetOnlineClients_ParsesAlirezaXuiObjectArrayShape(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		switch {
		case r.URL.Path == "/login":
			http.SetCookie(w, &http.Cookie{Name: "3x-ui", Value: "tok", Path: "/"})
			w.Header().Set("Content-Type", "application/json")
			_ = json.NewEncoder(w).Encode(map[string]interface{}{"success": true, "msg": ""})
		case r.URL.Path == "/xui/API/inbounds/onlines" && r.Method == http.MethodPost:
			w.Header().Set("Content-Type", "application/json")
			_ = json.NewEncoder(w).Encode(map[string]interface{}{
				"success": true, "msg": "",
				"obj": []map[string]interface{}{
					{"email": "pkg_abc123_2", "ips": map[string]int64{"1.2.3.4": 1737400000000}},
					{"email": "pkg_def456_2", "ips": map[string]int64{}},
				},
			})
		default:
			w.WriteHeader(http.StatusNotFound)
		}
	}))
	defer server.Close()

	online, err := GetOnlineClients(context.Background(), model.XuiPanel{
		APIBaseURL: server.URL, Username: "admin", Password: "admin",
	})
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if !online["pkg_abc123_2"] || !online["pkg_def456_2"] {
		t.Fatalf("expected both stub-reported emails to be online, got %v", online)
	}
	if online["pkg_not_connected_2"] {
		t.Fatalf("expected an email absent from the stub's obj list to be reported as not online")
	}
}

// TestParseXuiOnlinesObj_EmptyAndNull confirms an empty or null obj (no
// clients currently online -- a perfectly normal, common response) parses
// to an empty/nil result rather than an error, distinct from a genuinely
// malformed obj.
func TestParseXuiOnlinesObj_EmptyAndNull(t *testing.T) {
	cases := []struct {
		name string
		raw  string
	}{
		{"null", "null"},
		{"empty array", "[]"},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			emails, err := parseXuiOnlinesObj([]byte(tc.raw))
			if err != nil {
				t.Fatalf("unexpected error: %v", err)
			}
			if len(emails) != 0 {
				t.Fatalf("expected no emails, got %v", emails)
			}
		})
	}
}

// TestParseXuiOnlinesObj_MalformedReturnsError confirms an obj matching
// neither known shape (not a flat string array, not an array of
// {email,...} objects) still surfaces a clear error rather than silently
// returning zero online clients, which would be indistinguishable from a
// panel with genuinely nobody connected.
func TestParseXuiOnlinesObj_MalformedReturnsError(t *testing.T) {
	_, err := parseXuiOnlinesObj([]byte(`{"unexpected": "shape"}`))
	if err == nil {
		t.Fatal("expected an error for an obj shape matching neither known format")
	}
}

// newInboundDetailStub returns a stub x-ui admin server implementing
// /login and GET /xui/API/inbounds/get/:id, with streamSettings.security
// set to securityValue -- used by both ResolveClientFlow tests below.
func newInboundDetailStub(t *testing.T, securityValue string) *httptest.Server {
	t.Helper()

	streamSettingsJSON, err := json.Marshal(map[string]string{"security": securityValue})
	if err != nil {
		t.Fatalf("failed to marshal stub streamSettings: %v", err)
	}

	return httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		switch {
		case r.URL.Path == "/login":
			http.SetCookie(w, &http.Cookie{Name: "3x-ui", Value: "tok", Path: "/"})
			w.Header().Set("Content-Type", "application/json")
			_ = json.NewEncoder(w).Encode(map[string]interface{}{"success": true, "msg": ""})
		case r.URL.Path == "/xui/API/inbounds/get/5":
			w.Header().Set("Content-Type", "application/json")
			_ = json.NewEncoder(w).Encode(map[string]interface{}{
				"success": true, "msg": "",
				"obj": map[string]interface{}{"streamSettings": string(streamSettingsJSON)},
			})
		default:
			w.WriteHeader(http.StatusNotFound)
		}
	}))
}

// TestResolveClientFlow_NoFlowOnPlainInbound is a regression test for a
// real bug reported by a customer's own V2Ray engineer, with side-by-side
// evidence from a real x-ui panel: flow="xtls-rprx-vision" was hardcoded
// on every client this codebase created, regardless of the inbound's own
// security setting. On a security:"none" inbound, x-ui creates the client
// but it can never actually connect -- VLESS XTLS flow control requires
// the TLS/Reality handshake underneath it. A client manually created
// through x-ui's own web UI on the same inbound has flow="" and connects
// fine; the only difference was this one field.
func TestResolveClientFlow_NoFlowOnPlainInbound(t *testing.T) {
	server := newInboundDetailStub(t, "none")
	defer server.Close()

	flow, err := ResolveClientFlow(context.Background(), model.XuiPanel{
		APIBaseURL: server.URL, Username: "admin", Password: "admin",
	}, 5)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if flow != "" {
		t.Fatalf("expected empty flow on a security:none inbound, got %q", flow)
	}
}

// TestResolveClientFlow_VisionFlowOnRealityInbound is
// TestResolveClientFlow_NoFlowOnPlainInbound's counterpart -- confirms
// xtls-rprx-vision IS still correctly applied on inbounds where it's
// actually valid, so the fix isn't a blanket "always empty" regression.
func TestResolveClientFlow_VisionFlowOnRealityInbound(t *testing.T) {
	for _, security := range []string{"reality", "tls", "REALITY", "TLS"} {
		t.Run(security, func(t *testing.T) {
			server := newInboundDetailStub(t, security)
			defer server.Close()

			flow, err := ResolveClientFlow(context.Background(), model.XuiPanel{
				APIBaseURL: server.URL, Username: "admin", Password: "admin",
			}, 5)
			if err != nil {
				t.Fatalf("unexpected error: %v", err)
			}
			if flow != xtlsRprxVisionFlow {
				t.Fatalf("expected %q on a security:%s inbound, got %q", xtlsRprxVisionFlow, security, flow)
			}
		})
	}
}
