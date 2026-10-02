package http

import (
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/golang-jwt/jwt/v5"
	"github.com/labstack/echo/v4"
)

// setResellerJWT injects a fake reseller-role JWT into the echo context, the
// same way reseller_test.go injects an admin-role JWT.
func setResellerJWT(ctx echo.Context, resellerID uint) {
	token := &jwt.Token{Claims: jwt.MapClaims{
		"role":        "reseller",
		"reseller_id": float64(resellerID),
	}}
	ctx.Set("user", token)
}

func setAdminJWT(ctx echo.Context) {
	token := &jwt.Token{Claims: jwt.MapClaims{"role": "admin"}}
	ctx.Set("user", token)
}

// TestResellerScopedPeerEndpointsForbidResellerCallers verifies that a
// reseller-role caller gets 403 on every admin-only "manage a specific
// reseller's peers" endpoint, before any Mikrotik-dependent service code
// runs. These endpoints let an admin create/update/delete/toggle/list peers
// on behalf of an arbitrary reseller (identified by the :reseller_id URL
// param, not the caller's own JWT), so they must never be reachable by a
// reseller caller regardless of which reseller_id they pass.
//
// A non-nil handler return here is expected once the response has already
// been committed (see errAlreadyHandled in wg_peer.go) — Echo's own default
// error handler no-ops on an already-committed response, so this is not a
// double-write. rec.Code is the source of truth for what the client sees.
func TestResellerScopedPeerEndpointsForbidResellerCallers(t *testing.T) {
	controller := &WgPeerController{}
	e := echo.New()

	cases := []struct {
		name    string
		method  string
		path    string
		handler echo.HandlerFunc
	}{
		{"GetPeersByReseller", http.MethodGet, "/api/peer/reseller/1", controller.GetPeersByReseller},
		{"CreatePeerForReseller", http.MethodPost, "/api/peer/reseller/1", controller.CreatePeerForReseller},
		{"UpdatePeerForReseller", http.MethodPut, "/api/peer/reseller/1/5", controller.UpdatePeerForReseller},
		{"DeletePeerForReseller", http.MethodDelete, "/api/peer/reseller/1/5", controller.DeletePeerForReseller},
		{"UpdatePeerStatusForReseller", http.MethodPatch, "/api/peer/reseller/1/5/status", controller.UpdatePeerStatusForReseller},
	}

	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			req := httptest.NewRequest(tc.method, tc.path, nil)
			rec := httptest.NewRecorder()
			ctx := e.NewContext(req, rec)
			ctx.SetParamNames("reseller_id", "id")
			ctx.SetParamValues("1", "5")

			// The caller is a reseller (id=2) trying to reach an endpoint
			// scoped to a *different* reseller (id=1 in the URL) — this must
			// be forbidden regardless of whose reseller_id is targeted.
			setResellerJWT(ctx, 2)

			_ = tc.handler(ctx)
			if rec.Code != http.StatusForbidden {
				t.Fatalf("expected 403 for reseller caller, got %d: %s", rec.Code, rec.Body.String())
			}
		})
	}
}

// TestAdminResellerIDFromParamForbidsReseller pins down the regression this
// file was written for: adminResellerIDFromParam must return a non-nil error
// when it writes a 403, so `if err != nil { return err }` in every caller
// actually stops execution instead of silently continuing with a zero-value
// resellerID.
func TestAdminResellerIDFromParamForbidsReseller(t *testing.T) {
	e := echo.New()
	req := httptest.NewRequest(http.MethodGet, "/api/peer/reseller/1", nil)
	rec := httptest.NewRecorder()
	ctx := e.NewContext(req, rec)
	ctx.SetParamNames("reseller_id")
	ctx.SetParamValues("1")
	setResellerJWT(ctx, 2)

	if _, err := adminResellerIDFromParam(ctx); err == nil {
		t.Fatalf("expected non-nil err (403 written) for reseller caller, got nil, rec.Code=%d body=%s", rec.Code, rec.Body.String())
	}
	if rec.Code != http.StatusForbidden {
		t.Fatalf("expected 403 for reseller caller, got %d: %s", rec.Code, rec.Body.String())
	}
}

// TestAdminResellerIDFromParamRejectsBadID confirms a malformed reseller_id
// path param yields a 400, not a 500 or panic, for an admin caller.
func TestAdminResellerIDFromParamRejectsBadID(t *testing.T) {
	e := echo.New()
	req := httptest.NewRequest(http.MethodGet, "/api/peer/reseller/not-a-number", nil)
	rec := httptest.NewRecorder()
	ctx := e.NewContext(req, rec)
	ctx.SetParamNames("reseller_id")
	ctx.SetParamValues("not-a-number")
	setAdminJWT(ctx)

	if _, err := adminResellerIDFromParam(ctx); err == nil {
		t.Fatalf("expected non-nil err (400 written) for malformed reseller_id, got nil")
	}
	if rec.Code != http.StatusBadRequest {
		t.Fatalf("expected 400 for malformed reseller_id, got %d: %s", rec.Code, rec.Body.String())
	}
}

// TestPeerScopeFromContextForbidsMalformedResellerToken confirms the same
// fix applied to peerScopeFromContext (used by GetPeers, CreatePeer,
// UpdatePeer, DeletePeer, and every other plain /peer/* endpoint): a token
// claiming role "reseller" with no reseller_id claim must not fall through
// to unscoped (nil resellerID / admin-level) access.
func TestPeerScopeFromContextForbidsMalformedResellerToken(t *testing.T) {
	e := echo.New()
	req := httptest.NewRequest(http.MethodGet, "/api/peer", nil)
	rec := httptest.NewRecorder()
	ctx := e.NewContext(req, rec)
	token := &jwt.Token{Claims: jwt.MapClaims{"role": "reseller"}} // no reseller_id
	ctx.Set("user", token)

	resellerID, err := peerScopeFromContext(ctx)
	if err == nil {
		t.Fatalf("expected non-nil err (403 written) for malformed reseller token, got nil (resellerID=%v)", resellerID)
	}
	if rec.Code != http.StatusForbidden {
		t.Fatalf("expected 403 for malformed reseller token, got %d: %s", rec.Code, rec.Body.String())
	}
}
