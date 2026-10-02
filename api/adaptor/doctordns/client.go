// Package doctordns talks to one or more externally-registered doctor-dns
// exit-node installations, over the /apex/* routes added to that project's
// smartdns-panel process for this integration. Unlike xui.Client, there is
// no login/cookie-jar step: every call carries a single static Bearer
// token (model.DNSPanel.APIKey, the installation's own APEX_API_KEY) set
// once at install time on the doctor-dns side, so a Client here is a plain
// stateless HTTP helper, not a per-operation authenticated session.
package doctordns

import (
	"bytes"
	"context"
	"crypto/tls"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"time"

	"go.uber.org/zap"

	"github.com/maahdima/mwp/api/dataservice/model"
)

// tlsServerNamePlaceholder is a fixed, made-up hostname sent as the TLS SNI
// value on every /apex/ call -- confirmed necessary, not cosmetic. DNSPanel.
// APIBaseURL is a bare IP address (there is no DNS name for a doctor-dns
// exit node), and per RFC 6066 Go's TLS client correctly omits the SNI
// extension entirely when the destination is a literal IP rather than a
// hostname. Live testing from an Iranian relay server against a real exit
// node reproduced this exactly: every TLS handshake with NO SNI was reset
// by the network (errno 104, mid-handshake) on EVERY port tried (8443, a
// throwaway test port, and even the plain HTTPS port with no SNI), while
// the identical handshake WITH any SNI value present succeeded every time,
// including on the original 8443 port -- so this is not a port- or
// certificate-fingerprint-based block, specifically a missing-SNI one. The
// placeholder's actual value is never validated by anything (doctor-dns's
// own smartdns-panel does no SNI-based routing, unlike a relay's port 443
// SNI-preread proxy), it only needs to be present and hostname-shaped.
const tlsServerNamePlaceholder = "sync.smartdns.local"

// httpClient is shared across every call/panel -- there is no per-panel
// state to isolate (no cookie jar, no session), so one *http.Client with a
// connection pool is strictly better than constructing a fresh one per
// call, matching the reasoning already used for xui's own
// subscriptionHTTPClient.
var httpClient = &http.Client{
	Transport: func() *http.Transport {
		t := http.DefaultTransport.(*http.Transport).Clone()
		t.TLSClientConfig = &tls.Config{
			// doctor-dns's own installer issues a self-signed certificate
			// for this port (see its own CERT/KEY constants) -- same
			// tolerance already accepted for x-ui panels via xui.Client's
			// identical InsecureSkipVerify.
			InsecureSkipVerify: true,
			// See tlsServerNamePlaceholder's own doc comment -- this is
			// what actually keeps the handshake from being reset when
			// APIBaseURL is a bare IP.
			ServerName: tlsServerNamePlaceholder,
		}
		return t
	}(),
	Timeout: 15 * time.Second,
}

var logger = zap.L().Named("doctordns")

// apiError mirrors every doctor-dns /apex/ 4xx/404 response shape,
// confirmed from smartdns-panel's own reply(400/404, {"error": "..."})
// calls.
type apiError struct {
	Error string `json:"error"`
}

// doJSON posts reqBody (or GETs if reqBody is nil) as JSON to
// panel.APIBaseURL+path with the panel's own Bearer key, and decodes the
// response into respBody. A non-2xx status is surfaced as an error carrying
// whatever message doctor-dns's own {"error": "..."} body contains, falling
// back to the raw body text if it doesn't parse as that shape (mirrors
// xui.Client.doJSON's identical fallback).
func doJSON(ctx context.Context, panel model.DNSPanel, path string, reqBody, respBody interface{}) error {
	if panel.APIBaseURL == "" {
		return fmt.Errorf("DNS panel API base URL is empty")
	}

	var bodyReader io.Reader
	if reqBody != nil {
		jsonBody, err := json.Marshal(reqBody)
		if err != nil {
			return fmt.Errorf("marshal request body: %w", err)
		}
		bodyReader = bytes.NewReader(jsonBody)
	}

	req, err := http.NewRequestWithContext(ctx, http.MethodPost, panel.APIBaseURL+path, bodyReader)
	if err != nil {
		return fmt.Errorf("build request: %w", err)
	}
	req.Header.Set("Content-Type", "application/json")
	req.Header.Set("Authorization", "Bearer "+panel.APIKey)

	resp, err := httpClient.Do(req)
	if err != nil {
		logger.Warn("request failed", zap.String("path", path), zap.Error(err))
		return fmt.Errorf("calling %s: %w", path, err)
	}
	defer resp.Body.Close()

	body, err := io.ReadAll(resp.Body)
	if err != nil {
		return fmt.Errorf("reading response from %s: %w", path, err)
	}

	if resp.StatusCode == http.StatusUnauthorized {
		return fmt.Errorf("DNS panel rejected the request as unauthorised -- check the panel's API key")
	}
	if resp.StatusCode != http.StatusOK {
		var apiErr apiError
		if json.Unmarshal(body, &apiErr) == nil && apiErr.Error != "" {
			return fmt.Errorf("request to %s failed with status %d: %s", path, resp.StatusCode, apiErr.Error)
		}
		logger.Warn("non-200 response", zap.String("path", path), zap.Int("status", resp.StatusCode), zap.String("body", string(body)))
		return fmt.Errorf("request to %s failed with status %d: %s", path, resp.StatusCode, string(body))
	}

	if respBody == nil || len(body) == 0 {
		return nil
	}
	if err := json.Unmarshal(body, respBody); err != nil {
		return fmt.Errorf("unmarshalling response from %s: %w", path, err)
	}
	return nil
}
