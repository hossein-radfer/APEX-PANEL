package http

import "testing"

// TestIsV2RayClientRequest covers the User-Agent sniffing that decides
// whether GetSubscription serves the raw base64 body (a real client app)
// or redirects to the human-facing subscription landing page (a browser)
// -- a confirmed, reported bug: opening the subscription URL directly in
// a browser showed a blank page of raw encoded text instead of a usable
// page.
func TestIsV2RayClientRequest(t *testing.T) {
	cases := []struct {
		name      string
		userAgent string
		want      bool
	}{
		{"empty defaults to client", "", true},
		{"v2rayNG", "v2rayNG/1.8.29", true},
		{"v2box", "v2Box/2.0", true},
		{"shadowrocket", "Shadowrocket/1810", true},
		{"clash", "ClashforWindows/0.20.39", true},
		{"sing-box", "sing-box/1.8.0", true},
		{"chrome desktop browser", "Mozilla/5.0 (Windows NT 10.0; Win64; x64) AppleWebKit/537.36 (KHTML, like Gecko) Chrome/120.0.0.0 Safari/537.36", false},
		{"safari mobile browser", "Mozilla/5.0 (iPhone; CPU iPhone OS 17_0 like Mac OS X) AppleWebKit/605.1.15 (KHTML, like Gecko) Version/17.0 Mobile/15E148 Safari/604.1", false},
		{"firefox browser", "Mozilla/5.0 (X11; Linux x86_64; rv:109.0) Gecko/20100101 Firefox/119.0", false},
	}

	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			got := isV2RayClientRequest(tc.userAgent)
			if got != tc.want {
				t.Fatalf("isV2RayClientRequest(%q) = %v, want %v", tc.userAgent, got, tc.want)
			}
		})
	}
}
