package cli

import (
	"bufio"
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"regexp"

	"github.com/maahdima/mwp/api/config"
)

// domainPattern is a conservative, intentionally non-exhaustive check --
// good enough to catch an obviously empty/garbage answer (a stray blank
// line, a pasted URL instead of a bare hostname) before ever shelling out
// to certbot, which would otherwise fail several steps later with a much
// more confusing error.
var domainPattern = regexp.MustCompile(`^[a-zA-Z0-9]([a-zA-Z0-9-]*[a-zA-Z0-9])?(\.[a-zA-Z0-9]([a-zA-Z0-9-]*[a-zA-Z0-9])?)+$`)

// actionAutoSSL puts nginx in front of the panel's own port as a TLS-
// terminating reverse proxy and obtains a real Let's Encrypt certificate
// for it via certbot -- the panel's own Go binary only ever speaks plain
// HTTP (see deploy/install.sh's own doc comment: "This script does NOT
// set up nginx/TLS for MWPanel itself"), so reaching it over https://
// requires something else in front of it. nginx is that something, and
// certbot's own "--nginx" plugin edits the vhost it generates here in
// place to add the listener/cert directives, which is why this writes a
// plain-http vhost FIRST and only then calls certbot, rather than trying
// to hand-write the TLS block itself.
func actionAutoSSL(reader *bufio.Reader) {
	fmt.Println()
	fmt.Println("This installs nginx and certbot (if not already present), configures nginx")
	fmt.Println("as a reverse proxy in front of the panel's own port, and obtains a real")
	fmt.Println("TLS certificate for a domain you provide (which must already point at this")
	fmt.Println("server's IP address -- certbot verifies that over the network).")
	fmt.Println()

	domain := readLine(reader, "Domain name (e.g. panel.example.com): ")
	if !domainPattern.MatchString(domain) {
		fmt.Println("That doesn't look like a valid domain name. Aborting.")
		return
	}

	appCfg := config.GetAppConfig()
	panelPort := appCfg.Port
	if panelPort == "" {
		panelPort = "3000"
	}
	fmt.Printf("Proxying https://%s -> http://127.0.0.1:%s (the panel's own configured port).\n", domain, panelPort)
	confirm := readLine(reader, "Continue? [y/N]: ")
	if confirm != "y" && confirm != "Y" {
		fmt.Println("Cancelled.")
		return
	}

	fmt.Println()
	fmt.Println("==> Installing nginx and certbot (apt-get)")
	if err := runCmdLive("apt-get", "update", "-qq"); err != nil {
		fmt.Printf("Failed to run apt-get update: %v\n", err)
		return
	}
	if err := runCmdLive("apt-get", "install", "-y", "-qq", "nginx", "certbot", "python3-certbot-nginx"); err != nil {
		fmt.Printf("Failed to install nginx/certbot: %v\n", err)
		fmt.Println("This menu assumes a Debian/Ubuntu host with apt-get -- install them manually on other distros.")
		return
	}

	siteConfig := fmt.Sprintf(`server {
    listen 80;
    server_name %s;

    location / {
        proxy_pass http://127.0.0.1:%s;
        proxy_set_header Host $host;
        proxy_set_header X-Real-IP $remote_addr;
        proxy_set_header X-Forwarded-For $proxy_add_x_forwarded_for;
        proxy_set_header X-Forwarded-Proto $scheme;
        # WebSocket support, needed for the panel's live log stream.
        proxy_http_version 1.1;
        proxy_set_header Upgrade $http_upgrade;
        proxy_set_header Connection "upgrade";
    }
}
`, domain, panelPort)

	sitesAvailable := filepath.Join("/etc/nginx/sites-available", domain)
	sitesEnabled := filepath.Join("/etc/nginx/sites-enabled", domain)

	fmt.Printf("==> Writing nginx site config to %s\n", sitesAvailable)
	if err := os.WriteFile(sitesAvailable, []byte(siteConfig), 0o644); err != nil {
		fmt.Printf("Failed to write nginx config: %v\n", err)
		return
	}

	// Symlinking into sites-enabled is the standard Debian/Ubuntu nginx
	// packaging convention (sites-available holds every config ever
	// written, sites-enabled is what's actually live) -- some minimal
	// images ship nginx without sites-enabled/the default include line in
	// nginx.conf at all, so a missing directory here is reported clearly
	// rather than failing deep inside a later nginx -t with no context.
	if _, err := os.Stat("/etc/nginx/sites-enabled"); err == nil {
		_ = os.Remove(sitesEnabled) // re-running this action replaces a previous symlink cleanly
		if err := os.Symlink(sitesAvailable, sitesEnabled); err != nil {
			fmt.Printf("Failed to enable nginx site: %v\n", err)
			return
		}
	} else {
		fmt.Println("Warning: /etc/nginx/sites-enabled not found -- this nginx package may include")
		fmt.Println("sites-available configs directly. Check /etc/nginx/nginx.conf's own include lines")
		fmt.Printf("if %s is not picked up.\n", sitesAvailable)
	}

	fmt.Println("==> Testing nginx configuration")
	if err := runCmdLive("nginx", "-t"); err != nil {
		fmt.Printf("nginx configuration test failed: %v\n", err)
		fmt.Println("Fix the config above before retrying -- nginx was not reloaded.")
		return
	}

	fmt.Println("==> Reloading nginx")
	if err := exec.Command("systemctl", "reload", "nginx").Run(); err != nil {
		if err := exec.Command("systemctl", "restart", "nginx").Run(); err != nil {
			fmt.Printf("Failed to reload/restart nginx: %v\n", err)
			return
		}
	}

	fmt.Println("==> Requesting a certificate from Let's Encrypt (certbot)")
	fmt.Println("This fails if the domain does not already resolve to this server's IP address.")
	if err := runCmdLive("certbot", "--nginx", "-d", domain, "--non-interactive", "--agree-tos", "--redirect", "-m", "admin@"+domain, "--no-eff-email"); err != nil {
		fmt.Printf("certbot failed: %v\n", err)
		fmt.Println("Common causes: the domain's DNS A record does not point at this server yet,")
		fmt.Println("or port 80 is blocked by a firewall (certbot's nginx plugin needs it for the HTTP-01 challenge).")
		return
	}

	fmt.Println()
	fmt.Printf("Done. The panel is now reachable at https://%s\n", domain)
	fmt.Println("Certbot has also installed its own systemd timer that renews the certificate automatically before it expires.")
}

// runCmdLive runs a command with no working-directory override, streaming
// output live -- same rationale as runCmd in build.go, but for commands
// that operate on the system (apt-get, nginx, certbot) rather than inside
// a cloned repo.
func runCmdLive(name string, args ...string) error {
	cmd := exec.Command(name, args...)
	cmd.Stdout = os.Stdout
	cmd.Stderr = os.Stderr
	return cmd.Run()
}
