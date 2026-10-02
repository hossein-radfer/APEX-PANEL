package license

import (
	"bytes"
	"encoding/json"
	"fmt"
	"io"
	"mime/multipart"
	"net/http"
	"net/url"
	"time"
)

// Client talks to license-panel's public API (port 3131). A short,
// bounded HTTP timeout is critical here: this runs on MWPanel's own
// startup path (see service.LicenseService.EnsureActivated), and a hung
// TCP connection to an unreachable license server must never be able to
// hang MWPanel's own boot indefinitely.
type Client struct {
	baseURL    string
	httpClient *http.Client
}

func NewClient(baseURL string) *Client {
	return &Client{
		baseURL:    baseURL,
		httpClient: &http.Client{Timeout: 10 * time.Second},
	}
}

func (c *Client) Activate(req ActivateRequest) ([]byte, error) {
	return c.post("/api/v1/license/activate", req)
}

// Trial claims a free trial license. Unlike Activate/Heartbeat, license-
// panel can reject this with a genuine non-200 HTTP error (trials
// disabled, this fingerprint already claimed one) rather than a signed
// "invalid" payload -- see license-panel's LicenseController.Trial. This
// method returns that error as-is (post() already formats it with the
// server's message included) so the caller can show it directly; there is
// no signed response to verify on a rejection since none was produced.
func (c *Client) Trial(req TrialRequest) ([]byte, error) {
	return c.post("/api/v1/license/trial", req)
}

func (c *Client) Heartbeat(req HeartbeatRequest) ([]byte, error) {
	return c.post("/api/v1/license/heartbeat", req)
}

// RequestNonce and ValidateCapability are the two halves of the فاز
// پنجم-۱ live challenge/response flow (see service.RequestCapabilityProof
// for the combined, ready-to-call helper most callers should use instead
// of these two directly).
func (c *Client) RequestNonce(req NonceRequest) ([]byte, error) {
	return c.post("/api/v1/license/nonce", req)
}

func (c *Client) ValidateCapability(req CapabilityValidateRequest) ([]byte, error) {
	return c.post("/api/v1/license/validate-capability", req)
}

// GetPublicKey fetches license-panel's Ed25519 public key -- lets an
// install bootstrap LICENSE_PUBLIC_KEY automatically on first contact
// instead of requiring it to be manually copied into .env (see
// LicenseService.ensurePublicKey).
func (c *Client) GetPublicKey() (string, error) {
	raw, err := c.get("/api/v1/license/public-key", nil)
	if err != nil {
		return "", err
	}
	var resp struct {
		PublicKey string `json:"public_key"`
	}
	if err := json.Unmarshal(raw, &resp); err != nil {
		return "", fmt.Errorf("failed to parse public-key response: %w", err)
	}
	if resp.PublicKey == "" {
		return "", fmt.Errorf("license server returned an empty public key")
	}
	return resp.PublicKey, nil
}

// ListTickets, CreateTicket, GetTicketMessages, PostTicketMessage,
// SetTyping, GetTypingStatus, and DownloadAttachment are MWPanel's side of
// the Help Center feature -- this install's browser never talks to
// license-panel directly (never holds/sends the license key itself); it
// only ever talks to MWPanel's own API, which uses this Client (already
// holding the license key server-side, same as Activate/Heartbeat) to
// relay each request. See http.HelpCenterController for the MWPanel-side
// handlers that call these.
func (c *Client) ListTickets(licenseKey string) ([]byte, error) {
	return c.get("/api/v1/help-center/tickets", url.Values{"license_key": {licenseKey}})
}

func (c *Client) CreateTicket(licenseKey, subject, message string) ([]byte, error) {
	return c.post("/api/v1/help-center/tickets", map[string]string{
		"license_key": licenseKey,
		"subject":     subject,
		"message":     message,
	})
}

func (c *Client) GetTicketMessages(licenseKey string, ticketID uint) ([]byte, error) {
	return c.get(fmt.Sprintf("/api/v1/help-center/tickets/%d", ticketID), url.Values{"license_key": {licenseKey}})
}

func (c *Client) PostTicketMessage(licenseKey string, ticketID uint, body, kind, fileName string, fileContent io.Reader) ([]byte, error) {
	fields := map[string]string{"license_key": licenseKey, "body": body}
	if kind != "" {
		fields["kind"] = kind
	}
	return c.postForm(fmt.Sprintf("/api/v1/help-center/tickets/%d/messages", ticketID), fields, "file", fileName, fileContent)
}

func (c *Client) SetTyping(licenseKey string, ticketID uint) error {
	_, err := c.postForm(fmt.Sprintf("/api/v1/help-center/tickets/%d/typing", ticketID), map[string]string{"license_key": licenseKey}, "", "", nil)
	return err
}

func (c *Client) GetTypingStatus(licenseKey string, ticketID uint) ([]byte, error) {
	return c.get(fmt.Sprintf("/api/v1/help-center/tickets/%d/typing", ticketID), url.Values{"license_key": {licenseKey}})
}

// DownloadAttachment streams a message attachment's raw bytes plus its
// Content-Type/Content-Disposition headers -- the caller (http.
// HelpCenterController.DownloadAttachment) copies both straight through
// to MWPanel's own browser response so the file behaves identically to a
// direct download (correct filename, correct player/preview behavior for
// audio attachments) without buffering the whole file in memory.
func (c *Client) DownloadAttachment(licenseKey string, ticketID, messageID uint) (*http.Response, error) {
	query := url.Values{"license_key": {licenseKey}}
	fullURL := fmt.Sprintf("%s/api/v1/help-center/tickets/%d/messages/%d/attachment?%s", c.baseURL, ticketID, messageID, query.Encode())
	resp, err := c.httpClient.Get(fullURL)
	if err != nil {
		return nil, fmt.Errorf("request to license server failed: %w", err)
	}
	if resp.StatusCode < 200 || resp.StatusCode >= 300 {
		defer resp.Body.Close()
		raw, _ := io.ReadAll(resp.Body)
		return nil, fmt.Errorf("license server returned status %d: %s", resp.StatusCode, string(raw))
	}
	return resp, nil
}

func (c *Client) CheckUpdate(req UpdateCheckRequest) (*UpdateCheckResponse, error) {
	raw, err := c.post("/api/v1/update/check", req)
	if err != nil {
		return nil, err
	}
	var resp UpdateCheckResponse
	if err := json.Unmarshal(raw, &resp); err != nil {
		return nil, fmt.Errorf("failed to parse update-check response: %w", err)
	}
	return &resp, nil
}

func (c *Client) post(path string, body interface{}) ([]byte, error) {
	payload, err := json.Marshal(body)
	if err != nil {
		return nil, fmt.Errorf("marshalling request: %w", err)
	}

	resp, err := c.httpClient.Post(c.baseURL+path, "application/json", bytes.NewReader(payload))
	if err != nil {
		return nil, fmt.Errorf("request to license server failed: %w", err)
	}
	defer resp.Body.Close()

	return c.readResponse(resp)
}

// get issues a plain GET, used by every Help Center read endpoint --
// license-panel expects license_key as a query param on those (see
// internal/http/help_center_public_controller.go's resolveLicense calls),
// not a JSON body, since GET requests conventionally carry no body.
func (c *Client) get(path string, query url.Values) ([]byte, error) {
	fullURL := c.baseURL + path
	if len(query) > 0 {
		fullURL += "?" + query.Encode()
	}

	resp, err := c.httpClient.Get(fullURL)
	if err != nil {
		return nil, fmt.Errorf("request to license server failed: %w", err)
	}
	defer resp.Body.Close()

	return c.readResponse(resp)
}

// postForm issues a multipart/form-data POST -- used for Help Center
// message posting, which needs to carry an optional file field alongside
// the plain text fields license-panel's PostMessage handler expects (see
// internal/http/help_center_attachment.go's parseMessageAttachment).
// fileField/fileName/fileContent are all optional (empty fileField skips
// attaching a file entirely, for a plain text message).
func (c *Client) postForm(path string, fields map[string]string, fileFieldName, fileName string, fileContent io.Reader) ([]byte, error) {
	var buf bytes.Buffer
	writer := multipart.NewWriter(&buf)

	for key, value := range fields {
		if err := writer.WriteField(key, value); err != nil {
			return nil, fmt.Errorf("writing form field %q: %w", key, err)
		}
	}

	if fileContent != nil {
		part, err := writer.CreateFormFile(fileFieldName, fileName)
		if err != nil {
			return nil, fmt.Errorf("creating form file part: %w", err)
		}
		if _, err := io.Copy(part, fileContent); err != nil {
			return nil, fmt.Errorf("writing form file content: %w", err)
		}
	}

	if err := writer.Close(); err != nil {
		return nil, fmt.Errorf("closing multipart writer: %w", err)
	}

	resp, err := c.httpClient.Post(c.baseURL+path, writer.FormDataContentType(), &buf)
	if err != nil {
		return nil, fmt.Errorf("request to license server failed: %w", err)
	}
	defer resp.Body.Close()

	return c.readResponse(resp)
}

func (c *Client) readResponse(resp *http.Response) ([]byte, error) {
	raw, err := io.ReadAll(resp.Body)
	if err != nil {
		return nil, fmt.Errorf("reading license server response: %w", err)
	}

	if resp.StatusCode < 200 || resp.StatusCode >= 300 {
		return nil, fmt.Errorf("license server returned status %d: %s", resp.StatusCode, string(raw))
	}

	return raw, nil
}
