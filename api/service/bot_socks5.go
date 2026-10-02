package service

import (
	"context"
	"fmt"
	"net"
	"net/http"
	"time"

	tgbotapi "github.com/go-telegram-bot-api/telegram-bot-api/v5"
	"golang.org/x/net/proxy"
)

// newTelegramBotAPI connects a *tgbotapi.BotAPI either directly
// (socks5Enabled=false, the original/default behavior for every existing
// install) or through a SOCKS5 proxy (item 7) when enabled -- the single
// shared connection helper both BotService.Reload (the panel's own admin
// bot) and StartCustom (every reseller's bot-as-a-service instance) call,
// so the panel-wide proxy setting applies uniformly to every bot instance
// this process ever runs.
func newTelegramBotAPI(token string, socks5Enabled bool, socks5Address, socks5Username, socks5Password string) (*tgbotapi.BotAPI, error) {
	if !socks5Enabled || socks5Address == "" {
		// A confirmed, reported production incident: tgbotapi.NewBotAPI's
		// own default *http.Client has NO timeout at all. On a server
		// with no route to api.telegram.org (exactly the "Telegram is
		// network-filtered here" case this file's own SOCKS5 path exists
		// for -- see its doc comment), any outbound call this bot makes
		// (e.g. NotifyFailedLogin firing synchronously from inside the
		// admin/reseller Login HTTP handler) blocks its goroutine
		// forever on a silently-dropped connection, not a fast
		// connection-refused. Every later request sharing that same
		// resource (DB connections, the SQLite single-writer lock) then
		// queues up behind it, which is what actually made the WHOLE
		// panel look hung -- not just the bot -- until the process was
		// killed and restarted. A 15s timeout here matches this
		// codebase's own general precedent of never leaving an outbound
		// HTTP client with the zero-value (no timeout) default.
		return tgbotapi.NewBotAPIWithClient(token, tgbotapi.APIEndpoint, &http.Client{Timeout: 15 * time.Second})
	}

	client, err := buildSocks5HTTPClient(socks5Address, socks5Username, socks5Password)
	if err != nil {
		return nil, err
	}

	return tgbotapi.NewBotAPIWithClient(token, tgbotapi.APIEndpoint, client)
}

// buildSocks5HTTPClient returns an *http.Client whose every outbound
// connection is dialed through the given SOCKS5 proxy (address is
// "host:port"; username/password may both be empty for an unauthenticated
// proxy) -- this is item 7's core mechanism: a panel installed on an
// Iran-hosted server has no direct route to api.telegram.org (Telegram is
// filtered at the network level there), so the bot's own outbound HTTP
// client needs to tunnel through a SOCKS5 proxy the admin configures
// instead. Passed to tgbotapi.NewBotAPIWithClient by both BotService.Reload
// (the panel's own shared admin bot) and StartCustom (every reseller's
// bot-as-a-service instance) when BotSettings.Socks5Enabled is true -- one
// panel-wide proxy setting applies to every bot instance this process
// runs, since the network path out of the server is the same for all of
// them regardless of whose bot token is being used.
func buildSocks5HTTPClient(address, username, password string) (*http.Client, error) {
	var auth *proxy.Auth
	if username != "" || password != "" {
		auth = &proxy.Auth{User: username, Password: password}
	}

	dialer, err := proxy.SOCKS5("tcp", address, auth, proxy.Direct)
	if err != nil {
		return nil, fmt.Errorf("failed to create SOCKS5 dialer: %w", err)
	}

	// proxy.Dialer has no context-aware DialContext of its own -- wrap it
	// so it can be plugged into http.Transport.DialContext (required by
	// net/http's connection pooling / timeout machinery), matching the
	// standard library's own documented pattern for adapting a plain
	// proxy.Dialer.
	contextDialer, ok := dialer.(proxy.ContextDialer)
	if !ok {
		return nil, fmt.Errorf("SOCKS5 dialer does not support DialContext")
	}

	transport := &http.Transport{
		DialContext: func(ctx context.Context, network, addr string) (net.Conn, error) {
			return contextDialer.DialContext(ctx, network, addr)
		},
	}

	return &http.Client{
		Transport: transport,
		Timeout:   30 * time.Second,
	}, nil
}

// telegramProbeURL is the fixed endpoint TestSocks5Connection dials through
// the configured proxy -- api.telegram.org is exactly what this proxy
// exists to reach (see buildSocks5HTTPClient's own doc comment), so a
// successful round trip here is the one measurement that actually answers
// "will my bot work," not just "is this proxy reachable at all."
const telegramProbeURL = "https://api.telegram.org"

// Socks5TestResult is the outcome of one live connectivity probe -- surfaced
// to the admin settings page (item 5) as "وضعیت اتصال SOCKS5 + پینگ زنده."
// Connected=false always carries a non-empty Error explaining why (dial
// refused, auth rejected, timeout, ...); Connected=true always carries a
// PingMS.
type Socks5TestResult struct {
	Connected bool
	PingMS    int64
	Error     string
}

// TestSocks5Connection dials the given proxy and performs one real HTTPS
// request against Telegram's API, timing the full round trip. Takes explicit
// parameters rather than reading BotSettings itself so the admin can test a
// proxy BEFORE saving it (the settings page's own "Test Connection" button
// sends whatever is currently typed in the form, matching the pattern of
// testing a config before committing to it). A 10s timeout is used here
// (vs. buildSocks5HTTPClient's 30s for real bot traffic) since this is an
// interactive, synchronous check the admin is actively waiting on.
func TestSocks5Connection(address, username, password string) Socks5TestResult {
	if address == "" {
		return Socks5TestResult{Connected: false, Error: "آدرس پروکسی وارد نشده است."}
	}

	client, err := buildSocks5HTTPClient(address, username, password)
	if err != nil {
		return Socks5TestResult{Connected: false, Error: err.Error()}
	}
	client.Timeout = 10 * time.Second

	req, err := http.NewRequest(http.MethodGet, telegramProbeURL, nil)
	if err != nil {
		return Socks5TestResult{Connected: false, Error: err.Error()}
	}

	start := time.Now()
	resp, err := client.Do(req)
	elapsed := time.Since(start)
	if err != nil {
		return Socks5TestResult{Connected: false, Error: "اتصال برقرار نشد: " + err.Error()}
	}
	defer resp.Body.Close()

	// api.telegram.org with no bot token in the path replies 404 -- that's
	// still a successful round trip through the proxy (the request reached
	// Telegram and got a real HTTP response back), so any status code here
	// counts as "connected." Only a transport-level error above means the
	// proxy/network path itself failed.
	return Socks5TestResult{Connected: true, PingMS: elapsed.Milliseconds()}
}
