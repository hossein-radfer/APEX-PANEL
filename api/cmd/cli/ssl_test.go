package cli

import "testing"

// TestDomainPattern confirms the pre-certbot sanity check accepts real
// domain shapes and rejects the obviously-wrong input an operator might
// paste by mistake (a full URL, a blank line, a bare IP with no label) --
// catching these here gives a clear "that doesn't look like a valid
// domain name" message instead of a confusing failure several steps
// later inside certbot/nginx.
func TestDomainPattern(t *testing.T) {
	valid := []string{
		"panel.example.com",
		"example.com",
		"sub.sub.example.co.uk",
		"a-b.example.com",
	}
	for _, d := range valid {
		if !domainPattern.MatchString(d) {
			t.Errorf("expected %q to be accepted as a valid domain", d)
		}
	}

	invalid := []string{
		"",
		"https://panel.example.com",
		"panel.example.com/",
		"not a domain",
		"-leadingdash.com",
		"trailing-.com",
		"nodotatall",
	}
	for _, d := range invalid {
		if domainPattern.MatchString(d) {
			t.Errorf("expected %q to be rejected as an invalid domain", d)
		}
	}
}
