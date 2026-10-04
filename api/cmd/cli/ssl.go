package cli

import (
	"bufio"
	"fmt"
	"os"
	"os/exec"
	"regexp"

	"github.com/maahdima/mwp/api/service"
)

// domainPattern is a conservative, intentionally non-exhaustive check --
// good enough to catch an obviously empty/garbage answer (a stray blank
// line, a pasted URL instead of a bare hostname) before ever shelling out
// to certbot, which would otherwise fail several steps later with a much
// more confusing error.
var domainPattern = regexp.MustCompile(`^[a-zA-Z0-9]([a-zA-Z0-9-]*[a-zA-Z0-9])?(\.[a-zA-Z0-9]([a-zA-Z0-9-]*[a-zA-Z0-9])?)+$`)

// actionAutoSSL obtains a real Let's Encrypt certificate via certbot and
// wires the resulting file paths straight into the panel's own SSLSettings
// row (service/ssl_settings.go) -- the exact same two paths an admin would
// otherwise have to find and paste into the web UI's SSL settings page by
// hand. This is NOT an nginx reverse-proxy setup: the panel's own HTTP
// server already terminates TLS natively (see http-server.go's
// LoadTLSConfig wiring) once SSLSettings.Enabled is true, so nothing else
// needs to sit in front of it.
//
// certbot runs in --standalone mode, binding port 80 itself just long
// enough to answer the ACME HTTP-01 challenge -- this requires the panel
// (or anything else) to NOT already be listening on port 80, and for the
// domain's DNS A record to already point at this server. Using
// --standalone rather than --webroot avoids depending on an nginx/Apache
// docroot that may not exist on a fresh install; a host that already runs
// its own webroot-based renewal for other domains (as this project's own
// production server does) is untouched by this, since this only ever
// requests a certificate for the NEW domain given here.
func actionAutoSSL(reader *bufio.Reader) {
	fmt.Println()
	fmt.Println("This obtains a real TLS certificate from Let's Encrypt (via certbot, in")
	fmt.Println("--standalone mode) for a domain you provide, and configures the panel to")
	fmt.Println("serve HTTPS directly using it. The domain must already point at this")
	fmt.Println("server's IP address, and port 80 must be free for the few seconds certbot")
	fmt.Println("needs it to complete the verification.")
	fmt.Println()

	domain := readLine(reader, "Domain name (e.g. panel.example.com): ")
	if !domainPattern.MatchString(domain) {
		fmt.Println("That doesn't look like a valid domain name. Aborting.")
		return
	}

	confirm := readLine(reader, fmt.Sprintf("Request a certificate for %s now? [y/N]: ", domain))
	if confirm != "y" && confirm != "Y" {
		fmt.Println("Cancelled.")
		return
	}

	fmt.Println()
	fmt.Println("==> Installing certbot (apt-get), if not already present")
	if err := runCmdLive("apt-get", "update", "-qq"); err != nil {
		fmt.Printf("Failed to run apt-get update: %v\n", err)
		return
	}
	if err := runCmdLive("apt-get", "install", "-y", "-qq", "certbot"); err != nil {
		fmt.Printf("Failed to install certbot: %v\n", err)
		fmt.Println("This menu assumes a Debian/Ubuntu host with apt-get -- install it manually on other distros.")
		return
	}

	fmt.Println("==> Requesting a certificate from Let's Encrypt (certbot --standalone)")
	fmt.Println("If this hangs or fails, confirm nothing else is listening on port 80 right now.")
	if err := runCmdLive("certbot", "certonly", "--standalone", "-d", domain,
		"--non-interactive", "--agree-tos", "-m", "admin@"+domain, "--no-eff-email"); err != nil {
		fmt.Printf("certbot failed: %v\n", err)
		fmt.Println("Common causes: the domain's DNS A record does not point at this server yet,")
		fmt.Println("or port 80 is already in use by another process.")
		return
	}

	certPath := fmt.Sprintf("/etc/letsencrypt/live/%s/fullchain.pem", domain)
	keyPath := fmt.Sprintf("/etc/letsencrypt/live/%s/privkey.pem", domain)

	fmt.Println("==> Saving certificate paths into the panel's own SSL settings")
	db, err := connectDB()
	if err != nil {
		fmt.Printf("Certificate obtained at %s, but failed to connect to the database to enable it: %v\n", certPath, err)
		fmt.Println("Set it from the web UI's SSL settings page instead, using the paths above.")
		return
	}

	sslSettingsService := service.NewSSLSettingsService(db)
	enabled := true
	if _, err := sslSettingsService.UpdateSettings(service.UpdateSSLSettingsInput{
		Domain:          &domain,
		CertificatePath: &certPath,
		PrivateKeyPath:  &keyPath,
		Enabled:         &enabled,
	}); err != nil {
		fmt.Printf("Certificate obtained at %s, but failed to enable it in the panel: %v\n", certPath, err)
		fmt.Println("Set it from the web UI's SSL settings page instead, using the paths above.")
		return
	}

	fmt.Println("==> Restarting service to apply it")
	if err := exec.Command("systemctl", "restart", serviceName).Run(); err != nil {
		fmt.Printf("SSL enabled, but failed to restart %s automatically: %v\n", serviceName, err)
		fmt.Printf("Run `systemctl restart %s` by hand for it to take effect.\n", serviceName)
		return
	}

	fmt.Println()
	fmt.Printf("Done. The panel now serves HTTPS directly at https://%s:<panel-port>\n", domain)
	fmt.Println("Certbot has also installed its own systemd timer that renews the certificate automatically before it expires.")
	fmt.Println("Note: certbot's standalone renewal needs port 80 free each time it runs -- if that stops being true, renewal will fail silently until the cert expires.")
}

// runCmdLive runs a command with no working-directory override, streaming
// output live -- same rationale as runCmd in build.go, but for commands
// that operate on the system (apt-get, certbot) rather than inside a
// cloned repo.
func runCmdLive(name string, args ...string) error {
	cmd := exec.Command(name, args...)
	cmd.Stdout = os.Stdout
	cmd.Stderr = os.Stderr
	return cmd.Run()
}
