package cli

import (
	"bufio"
	"fmt"
	"os/exec"
)

// actionUpdate pulls the latest main branch from repoURL, rebuilds the
// frontend and binary exactly as a fresh install would, and replaces
// /usr/local/bin/mwp in place. /opt/mwp (config + database + backups) is
// never touched -- AutoMigrate (api/cmd/main.go) handles any new,
// additive schema changes the next time the service starts, same as it
// already does on every normal restart.
func actionUpdate(reader *bufio.Reader) {
	fmt.Println()
	fmt.Println("This rebuilds mwp from the latest main branch and replaces the running binary.")
	fmt.Println("Your data and configuration under /opt/mwp are not touched.")
	confirm := readLine(reader, "Continue? [y/N]: ")
	if confirm != "y" && confirm != "Y" {
		fmt.Println("Cancelled.")
		return
	}

	built, err := buildFromSource()
	if err != nil {
		fmt.Printf("Update failed: %v\n", err)
		return
	}
	defer built.Cleanup()

	fmt.Println("==> Installing new binary")
	if err := installBinary(built.BinaryPath); err != nil {
		fmt.Printf("Update failed: %v\n", err)
		return
	}

	fmt.Println("==> Restarting service")
	if err := exec.Command("systemctl", "restart", serviceName).Run(); err != nil {
		fmt.Printf("Binary updated, but failed to restart %s automatically: %v\n", serviceName, err)
		fmt.Printf("Run `systemctl restart %s` by hand.\n", serviceName)
		return
	}

	fmt.Println("Update complete. The service has been restarted on the new version.")
}
