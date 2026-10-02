// Package xui talks to one or more externally-registered x-ui (alireza0)
// panels. Deliberately NOT built on common.MwpClients (that type is
// intentionally coupled to the single configured Mikrotik connection via
// the Server table) and not a session-caching client -- every operation
// logs in fresh and discards the session cookie afterward, matching how
// the Mikrotik adaptor resends Basic Auth on every request rather than
// caching any credential/session state. This is a deliberate simplicity
// choice: V2Ray operations (package create/sync) are infrequent enough
// that a login round-trip per operation is not a meaningful cost, and it
// avoids having to reason about concurrent-request-during-reauth races
// that a cached-session design would introduce.
package xui

import (
	"bytes"
	"context"
	"crypto/tls"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"net/http/cookiejar"
	"time"

	"go.uber.org/zap"

	"github.com/maahdima/mwp/api/common"
	"github.com/maahdima/mwp/api/dataservice/model"
)

// Client is a single logged-in session against one XuiPanel, valid only
// for the lifetime of the operation that created it -- callers get a fresh
// Client (and therefore a fresh login) per call via Login below, never a
// long-lived shared instance.
type Client struct {
	httpClient *http.Client
	baseURL    string
	subBaseURL string
	logger     *zap.Logger
}

// xuiLoginResponse mirrors x-ui's real /login response shape. Confirmed
// against MHSanaei/3x-ui's source (web/controller/index.go): a WRONG
// username/password still returns HTTP 200 -- auth failure is signaled
// ONLY by success=false in the body, never by the HTTP status code. A
// caller that only checks resp.StatusCode == 200 (as this adaptor
// previously did) will treat a rejected login as successful.
type xuiLoginResponse struct {
	Success bool   `json:"success"`
	Msg     string `json:"msg"`
}

// Login authenticates against panel and returns a Client whose underlying
// http.Client carries a real cookiejar.Jar loaded with whatever session
// cookie(s) x-ui actually set. Deliberately NOT a manually-extracted
// "grab the cookie named session/3x-ui" approach -- the real cookie name
// is application-defined (3x-ui's own source hardcodes the literal
// "3x-ui", not "session"; other forks/builds may differ or change it) and
// guessing it wrong silently breaks every subsequent request. A cookie
// jar sidesteps the naming question entirely: whatever Set-Cookie the
// server sends on login is exactly what gets replayed on every later
// request, the same way a real browser does it. Callers must not retain
// the returned Client beyond the current operation -- see the package
// doc comment for why nothing here is cached across operations.
func Login(ctx context.Context, panel model.XuiPanel) (*Client, error) {
	// Defense in depth: an empty APIBaseURL should already be rejected by
	// request validation upstream (see schema.TestXuiPanelConnectionRequest/
	// CreateXuiPanelRequest's validate:"required" tags), but if it ever
	// reaches here anyway, fail with a clear message instead of Go's HTTP
	// transport's much more confusing "unsupported protocol scheme ''"
	// error, which gives no hint that the URL itself was simply blank.
	if panel.APIBaseURL == "" {
		return nil, fmt.Errorf("panel API base URL is empty -- check that the panel was configured with a valid API base URL")
	}

	jar, err := cookiejar.New(nil)
	if err != nil {
		return nil, fmt.Errorf("create cookie jar: %w", err)
	}

	transport := http.DefaultTransport.(*http.Transport).Clone()
	transport.TLSClientConfig = &tls.Config{InsecureSkipVerify: true}

	httpClient := &http.Client{
		Transport: transport,
		Timeout:   15 * time.Second,
		Jar:       jar,
	}

	logger := zap.L().Named("xui.Client").With(zap.String("panel", panel.Name))

	body, err := json.Marshal(map[string]string{
		"username": panel.Username,
		"password": panel.Password,
	})
	if err != nil {
		return nil, fmt.Errorf("marshal login body: %w", err)
	}

	req, err := http.NewRequestWithContext(ctx, http.MethodPost, panel.APIBaseURL+common.XuiLoginPath, bytes.NewReader(body))
	if err != nil {
		return nil, fmt.Errorf("build login request: %w", err)
	}
	req.Header.Set("Content-Type", "application/json")

	resp, err := httpClient.Do(req)
	if err != nil {
		logger.Warn("login request failed", zap.Error(err))
		return nil, fmt.Errorf("connecting to %s: %w", panel.APIBaseURL, err)
	}
	defer resp.Body.Close()

	respBody, err := io.ReadAll(resp.Body)
	if err != nil {
		return nil, fmt.Errorf("reading login response: %w", err)
	}

	if resp.StatusCode != http.StatusOK {
		logger.Warn("login returned non-200", zap.Int("status", resp.StatusCode), zap.String("body", string(respBody)))
		return nil, fmt.Errorf("login failed with status %d: %s", resp.StatusCode, string(respBody))
	}

	var loginResp xuiLoginResponse
	if err := json.Unmarshal(respBody, &loginResp); err != nil {
		// x-ui's own login page (HTML) lands here if APIBaseURL is
		// missing a required base-path prefix the panel is configured
		// with (x-ui supports mounting its whole app under a custom
		// path, e.g. a randomized "/aBcDeFg/" prefix -- see 3x-ui's
		// settingService.GetBasePath()) -- surfaced as a clear,
		// actionable error instead of the raw JSON-unmarshal failure a
		// caller further down the chain would otherwise see.
		return nil, fmt.Errorf("login response was not valid JSON (got HTML or an unexpected body) -- if this panel uses a custom base path/URL prefix, include it in the API Base URL (e.g. https://host:port/your-base-path): %w", err)
	}
	if !loginResp.Success {
		msg := loginResp.Msg
		if msg == "" {
			msg = "check the username and password"
		}
		return nil, fmt.Errorf("login rejected by panel: %s", msg)
	}

	if len(jar.Cookies(req.URL)) == 0 {
		logger.Warn("login succeeded but no session cookie was set")
		return nil, fmt.Errorf("login succeeded but %s did not set a session cookie", panel.APIBaseURL)
	}

	return &Client{
		httpClient: httpClient,
		baseURL:    panel.APIBaseURL,
		subBaseURL: panel.SubBaseURL,
		logger:     logger,
	}, nil
}

func (c *Client) doJSON(ctx context.Context, method, path string, reqBody, respBody interface{}) error {
	var bodyReader io.Reader
	if reqBody != nil {
		jsonBody, err := json.Marshal(reqBody)
		if err != nil {
			return fmt.Errorf("marshal request body: %w", err)
		}
		bodyReader = bytes.NewReader(jsonBody)
	}

	req, err := http.NewRequestWithContext(ctx, method, c.baseURL+path, bodyReader)
	if err != nil {
		return fmt.Errorf("build request: %w", err)
	}
	if reqBody != nil {
		req.Header.Set("Content-Type", "application/json")
	}
	// No manual cookie attachment -- c.httpClient carries the cookiejar.Jar
	// populated during Login, which automatically replays whatever
	// cookie(s) the server actually set, under their real name(s), scoped
	// correctly by the jar's own domain/path matching. See Login's doc
	// comment for why this replaced a hardcoded cookie-name guess.

	resp, err := c.httpClient.Do(req)
	if err != nil {
		c.logger.Warn("request failed", zap.String("path", path), zap.Error(err))
		return fmt.Errorf("calling %s: %w", path, err)
	}
	defer resp.Body.Close()

	body, err := io.ReadAll(resp.Body)
	if err != nil {
		return fmt.Errorf("reading response from %s: %w", path, err)
	}

	if resp.StatusCode != http.StatusOK {
		c.logger.Warn("non-200 response", zap.String("path", path), zap.Int("status", resp.StatusCode), zap.String("body", string(body)))
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

// subscriptionHTTPClient is a plain, unauthenticated client used only for
// fetching a panel's public subscription endpoint (see getSubscriptionRaw)
// -- deliberately NOT an xui.Client/Login-derived client and, just as
// deliberately, NOT shared/cached across calls, matching this package's own
// "nothing is cached across operations" convention (see the package doc
// comment). Package-level so every call doesn't pay for a fresh
// *http.Transport (and its connection pool) for no reason; it carries no
// per-panel state (no cookie jar, no base URL), so sharing it is safe.
var subscriptionHTTPClient = &http.Client{
	Transport: func() *http.Transport {
		t := http.DefaultTransport.(*http.Transport).Clone()
		t.TLSClientConfig = &tls.Config{InsecureSkipVerify: true}
		return t
	}(),
	Timeout: 15 * time.Second,
}

// getSubscriptionRaw fetches url and returns the raw response body as text
// -- used exclusively by xui.GetSubscription. Deliberately a bare,
// unauthenticated GET with NO cookie jar and NO prior Login call: a real
// x-ui subscription endpoint is a separate, public, unauthenticated server
// (see xui.GetSubscription's doc comment for the full rationale and the bug
// this replaced -- reusing an admin-authenticated *Client here caused the
// admin panel's session cookie to leak onto subscription requests whenever
// api_base_url and sub_base_url shared a hostname on different ports).
func getSubscriptionRaw(ctx context.Context, url string) (string, error) {
	req, err := http.NewRequestWithContext(ctx, http.MethodGet, url, nil)
	if err != nil {
		return "", fmt.Errorf("build request: %w", err)
	}

	resp, err := subscriptionHTTPClient.Do(req)
	if err != nil {
		return "", fmt.Errorf("calling %s: %w", url, err)
	}
	defer resp.Body.Close()

	body, err := io.ReadAll(resp.Body)
	if err != nil {
		return "", fmt.Errorf("reading response from %s: %w", url, err)
	}

	if resp.StatusCode != http.StatusOK {
		return "", fmt.Errorf("request to %s failed with status %d", url, resp.StatusCode)
	}

	return string(body), nil
}
