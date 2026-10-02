package service

import (
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"os"
	"strings"
	"sync/atomic"
	"testing"

	tgbotapi "github.com/go-telegram-bot-api/telegram-bot-api/v5"
	"go.uber.org/zap"
)

// newTestBotAPI stands up a fake Telegram Bot API server and a *BotService
// wired to it, so sendPartWithRetry/sendPartsToChatID can be exercised
// against real tgbotapi request/response plumbing instead of mocking
// BotService itself (its api field is unexported and every real send goes
// through tgbotapi's own MakeRequest/UploadFiles). handler answers every
// call except "/bot<token>/getMe" (needed by tgbotapi.NewBotAPIWithClient's
// own constructor before it hands back a usable *BotAPI).
func newTestBotAPI(t *testing.T, handler http.HandlerFunc) *BotService {
	t.Helper()

	mux := http.NewServeMux()
	mux.HandleFunc("/bottest-token/getMe", func(w http.ResponseWriter, r *http.Request) {
		resp := tgbotapi.APIResponse{Ok: true, Result: mustMarshal(t, tgbotapi.User{ID: 1, FirstName: "TestBot"})}
		_ = json.NewEncoder(w).Encode(resp)
	})
	mux.HandleFunc("/bottest-token/", handler)

	server := httptest.NewServer(mux)
	t.Cleanup(server.Close)

	api, err := tgbotapi.NewBotAPIWithClient("test-token", server.URL+"/bot%s/%s", server.Client())
	if err != nil {
		t.Fatalf("failed to construct fake bot api: %v", err)
	}

	return &BotService{api: api, logger: zap.NewNop()}
}

func mustMarshal(t *testing.T, v interface{}) json.RawMessage {
	t.Helper()
	raw, err := json.Marshal(v)
	if err != nil {
		t.Fatalf("failed to marshal fixture: %v", err)
	}
	return raw
}

// TestSendPartWithRetry_RecoversFromFloodControl confirms the core anti-flood
// mechanism: a first attempt rejected with Telegram's own flood-control
// response (error_code 429 + retry_after) is retried, not treated as a hard
// failure, and a subsequent success is reported as an overall success.
func TestSendPartWithRetry_RecoversFromFloodControl(t *testing.T) {
	var attempts int32

	scheduler := &BotScheduler{logger: zap.NewNop()}
	scheduler.botService = newTestBotAPI(t, func(w http.ResponseWriter, r *http.Request) {
		if !strings.Contains(r.URL.Path, "sendDocument") {
			_ = json.NewEncoder(w).Encode(tgbotapi.APIResponse{Ok: true, Result: mustMarshal(t, tgbotapi.Message{})})
			return
		}

		n := atomic.AddInt32(&attempts, 1)
		if n == 1 {
			resp := tgbotapi.APIResponse{
				Ok:          false,
				ErrorCode:   429,
				Description: "Too Many Requests: retry later",
				Parameters:  &tgbotapi.ResponseParameters{RetryAfter: 1},
			}
			_ = json.NewEncoder(w).Encode(resp)
			return
		}
		_ = json.NewEncoder(w).Encode(tgbotapi.APIResponse{Ok: true, Result: mustMarshal(t, tgbotapi.Message{})})
	})

	part := writeTempTestFile(t, "fake backup bytes")

	err := scheduler.sendPartWithRetry("12345", part, "caption")
	if err != nil {
		t.Fatalf("expected sendPartWithRetry to recover from flood control, got: %v", err)
	}
	if got := atomic.LoadInt32(&attempts); got != 2 {
		t.Fatalf("expected exactly 2 send attempts (1 flood-controlled + 1 success), got %d", got)
	}
}

// TestSendPartWithRetry_GivesUpAfterMaxRetries confirms repeated flood
// control does not retry forever -- it must eventually surface as a real
// failure so the caller (sendPartsToChatID) stops sending later parts of an
// already-broken multi-part backup rather than hanging indefinitely.
func TestSendPartWithRetry_GivesUpAfterMaxRetries(t *testing.T) {
	scheduler := &BotScheduler{logger: zap.NewNop()}
	scheduler.botService = newTestBotAPI(t, func(w http.ResponseWriter, r *http.Request) {
		if !strings.Contains(r.URL.Path, "sendDocument") {
			_ = json.NewEncoder(w).Encode(tgbotapi.APIResponse{Ok: true, Result: mustMarshal(t, tgbotapi.Message{})})
			return
		}
		resp := tgbotapi.APIResponse{
			Ok:          false,
			ErrorCode:   429,
			Description: "Too Many Requests: retry later",
			Parameters:  &tgbotapi.ResponseParameters{RetryAfter: 1},
		}
		_ = json.NewEncoder(w).Encode(resp)
	})

	part := writeTempTestFile(t, "fake backup bytes")

	err := scheduler.sendPartWithRetry("12345", part, "caption")
	if err == nil {
		t.Fatal("expected sendPartWithRetry to eventually give up under sustained flood control")
	}
}

// TestSendPartWithRetry_NonFloodErrorFailsImmediately confirms a non-flood
// error (e.g. an invalid chat ID) is NOT retried -- only RetryAfter-bearing
// responses are, since retrying a permanent failure just wastes time before
// reporting what was always going to fail.
func TestSendPartWithRetry_NonFloodErrorFailsImmediately(t *testing.T) {
	var attempts int32

	scheduler := &BotScheduler{logger: zap.NewNop()}
	scheduler.botService = newTestBotAPI(t, func(w http.ResponseWriter, r *http.Request) {
		if !strings.Contains(r.URL.Path, "sendDocument") {
			_ = json.NewEncoder(w).Encode(tgbotapi.APIResponse{Ok: true, Result: mustMarshal(t, tgbotapi.Message{})})
			return
		}
		atomic.AddInt32(&attempts, 1)
		resp := tgbotapi.APIResponse{Ok: false, ErrorCode: 400, Description: "Bad Request: chat not found"}
		_ = json.NewEncoder(w).Encode(resp)
	})

	part := writeTempTestFile(t, "fake backup bytes")

	err := scheduler.sendPartWithRetry("12345", part, "caption")
	if err == nil {
		t.Fatal("expected a non-flood error to be surfaced")
	}
	if got := atomic.LoadInt32(&attempts); got != 1 {
		t.Fatalf("expected exactly 1 attempt for a non-retryable error, got %d", got)
	}
}

func writeTempTestFile(t *testing.T, content string) string {
	t.Helper()
	f, err := os.CreateTemp(t.TempDir(), "backup-part-*.txt")
	if err != nil {
		t.Fatalf("failed to create temp file: %v", err)
	}
	defer f.Close()
	if _, err := f.WriteString(content); err != nil {
		t.Fatalf("failed to write temp file: %v", err)
	}
	return f.Name()
}
