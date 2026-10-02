package http

import (
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/labstack/echo/v4"
)

// TestAttachFile_ForcesOctetStreamContentType is the core regression test
// for the confirmed reported bug "دانلود سرتیفیکیت openvpn در ساب که با
// فرمت txt دانلود میشه" (OpenVPN certificate in the sub page downloads as
// .txt): a plain-text file saved with an extension Go's mime registry
// doesn't recognize (.ovpn, .conf, .pem, ...) previously let Echo's
// Attachment -> http.ServeContent fall back to content-sniffing, which
// reads plain-text content as text/plain -- several browsers then rename
// the download to end in .txt regardless of the Content-Disposition
// filename actually given. attachFile must force Content-Type to
// application/octet-stream BEFORE ServeContent ever runs, so no sniffing
// happens at all.
func TestAttachFile_ForcesOctetStreamContentType(t *testing.T) {
	dir := t.TempDir()
	// .ovpn is a real-world example confirmed to have no entry in Go's
	// mime type registry -- this is deliberately plain ASCII text (a real
	// OpenVPN config/certificate is exactly this), which is what
	// http.DetectContentType would otherwise sniff as text/plain.
	path := filepath.Join(dir, "cert.ovpn")
	if err := os.WriteFile(path, []byte("-----BEGIN CERTIFICATE-----\nMIIB...\n-----END CERTIFICATE-----\n"), 0o600); err != nil {
		t.Fatalf("failed to write test file: %v", err)
	}

	e := echo.New()
	e.GET("/download", func(ctx echo.Context) error {
		return attachFile(ctx, path, "my-cert.ovpn")
	})

	req := httptest.NewRequest(http.MethodGet, "/download", nil)
	rec := httptest.NewRecorder()
	e.ServeHTTP(rec, req)

	if rec.Code != http.StatusOK {
		t.Fatalf("expected 200, got %d (body: %s)", rec.Code, rec.Body.String())
	}

	gotContentType := rec.Header().Get(echo.HeaderContentType)
	if gotContentType != "application/octet-stream" {
		t.Errorf("expected Content-Type application/octet-stream, got %q -- a browser may sniff this as text/plain and save the file as .txt", gotContentType)
	}

	gotDisposition := rec.Header().Get(echo.HeaderContentDisposition)
	if gotDisposition == "" || !strings.Contains(gotDisposition, "my-cert.ovpn") {
		t.Errorf("expected Content-Disposition to name my-cert.ovpn, got %q", gotDisposition)
	}
}

// TestAttachFile_PreservesActualFileContent confirms the fix only
// changes response headers, never the served content itself.
func TestAttachFile_PreservesActualFileContent(t *testing.T) {
	dir := t.TempDir()
	path := filepath.Join(dir, "config.conf")
	want := "client\ndev tun\nproto udp\n"
	if err := os.WriteFile(path, []byte(want), 0o600); err != nil {
		t.Fatalf("failed to write test file: %v", err)
	}

	e := echo.New()
	e.GET("/download", func(ctx echo.Context) error {
		return attachFile(ctx, path, "peer.conf")
	})

	req := httptest.NewRequest(http.MethodGet, "/download", nil)
	rec := httptest.NewRecorder()
	e.ServeHTTP(rec, req)

	if got := rec.Body.String(); got != want {
		t.Errorf("expected served content %q, got %q", want, got)
	}
}
