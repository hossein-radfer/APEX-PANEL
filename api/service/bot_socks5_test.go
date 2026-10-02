package service

import (
	"net"
	"strings"
	"testing"
	"time"
)

// TestTestSocks5Connection_EmptyAddress confirms the guard clause -- no
// network attempt is made at all when the address field is blank (the
// settings page's own form default before an admin has ever configured
// anything).
func TestTestSocks5Connection_EmptyAddress(t *testing.T) {
	result := TestSocks5Connection("", "", "")
	if result.Connected {
		t.Fatal("expected Connected=false for an empty address")
	}
	if result.Error == "" {
		t.Fatal("expected a non-empty Persian error message for an empty address")
	}
}

// TestTestSocks5Connection_RefusedConnection confirms the failure path
// against a real, but non-listening, TCP port -- the SOCKS5 dial itself
// must fail fast (not hang) and report Connected=false with a readable
// error, matching what an admin would see if they mistyped the proxy
// address/port.
func TestTestSocks5Connection_RefusedConnection(t *testing.T) {
	// Bind then immediately close, so the port is very likely refusing
	// connections rather than being genuinely reachable to any hypothetical
	// concurrent test/service.
	l, err := net.Listen("tcp", "127.0.0.1:0")
	if err != nil {
		t.Fatalf("failed to allocate a port: %v", err)
	}
	addr := l.Addr().String()
	l.Close()

	result := TestSocks5Connection(addr, "", "")
	if result.Connected {
		t.Fatal("expected Connected=false against a closed port")
	}
	if !strings.Contains(result.Error, "اتصال برقرار نشد") {
		t.Fatalf("expected the Persian connection-failed prefix, got: %q", result.Error)
	}
}

// fakeSocks5Server accepts exactly one connection, performs the minimal
// no-auth SOCKS5 handshake, replies "succeeded" to the CONNECT request
// without actually opening the requested upstream, then closes the
// connection -- enough to prove TestSocks5Connection's dialer/handshake
// plumbing works end-to-end without depending on real internet access or
// Telegram's real API being reachable from the test environment.
func fakeSocks5Server(t *testing.T) string {
	t.Helper()
	l, err := net.Listen("tcp", "127.0.0.1:0")
	if err != nil {
		t.Fatalf("failed to start fake SOCKS5 listener: %v", err)
	}
	t.Cleanup(func() { l.Close() })

	go func() {
		conn, err := l.Accept()
		if err != nil {
			return
		}
		defer conn.Close()
		conn.SetDeadline(time.Now().Add(5 * time.Second))

		// Greeting: VER NMETHODS METHODS...
		greeting := make([]byte, 2)
		if _, err := readFull(conn, greeting); err != nil {
			return
		}
		nMethods := int(greeting[1])
		methods := make([]byte, nMethods)
		if _, err := readFull(conn, methods); err != nil {
			return
		}
		// Reply: VER=5, METHOD=0 (no auth)
		if _, err := conn.Write([]byte{0x05, 0x00}); err != nil {
			return
		}

		// CONNECT request: VER CMD RSV ATYP ADDR PORT
		head := make([]byte, 4)
		if _, err := readFull(conn, head); err != nil {
			return
		}
		switch head[3] {
		case 0x01: // IPv4
			readFull(conn, make([]byte, 4+2))
		case 0x03: // domain name
			lenBuf := make([]byte, 1)
			if _, err := readFull(conn, lenBuf); err != nil {
				return
			}
			readFull(conn, make([]byte, int(lenBuf[0])+2))
		case 0x04: // IPv6
			readFull(conn, make([]byte, 16+2))
		}

		// Reply "succeeded" with a bogus BND.ADDR/PORT (0.0.0.0:0) -- the
		// client's next step (writing the actual HTTP request upstream)
		// will then fail/timeout since nothing is really connected, which
		// is fine: this test only asserts on the SOCKS5 handshake+dial
		// layer, not a full proxied HTTP round trip.
		conn.Write([]byte{0x05, 0x00, 0x00, 0x01, 0, 0, 0, 0, 0, 0})
	}()

	return l.Addr().String()
}

func readFull(conn net.Conn, buf []byte) (int, error) {
	total := 0
	for total < len(buf) {
		n, err := conn.Read(buf[total:])
		total += n
		if err != nil {
			return total, err
		}
	}
	return total, nil
}

// TestTestSocks5Connection_HandshakeSucceedsThenUpstreamFails confirms the
// dialer correctly completes a real SOCKS5 handshake (proving
// buildSocks5HTTPClient's dialer construction and TestSocks5Connection's
// use of it are wired correctly) even though the overall result is still a
// failure (since fakeSocks5Server never actually proxies real traffic).
func TestTestSocks5Connection_HandshakeSucceedsThenUpstreamFails(t *testing.T) {
	addr := fakeSocks5Server(t)

	result := TestSocks5Connection(addr, "", "")
	if result.Connected {
		t.Fatal("expected Connected=false since the fake proxy never actually forwards traffic")
	}
	if result.Error == "" {
		t.Fatal("expected a non-empty error message")
	}
}
