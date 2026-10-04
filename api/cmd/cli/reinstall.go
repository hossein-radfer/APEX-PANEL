package cli

import (
	"bufio"
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
)

// actionReinstall rebuilds mwp from source and reinstalls every FILE this
// panel depends on -- the binary, the systemd unit -- for the case the
// panel's own files were deleted, corrupted, or tampered with (e.g. a
// compromised server) and `mwp menu` itself still runs (built against a
// surviving copy of the binary, or invoked via `go run` from a fresh
// clone) but the installed pieces it's supposed to manage are broken.
// Unlike actionDeletePanel, this never removes /opt/mwp -- the whole
// point is recovering a broken install WITHOUT losing the data in it,
// mirroring install.sh's own "if $ENV_FILE already exists -- leave it
// untouched" rule.
func actionReinstall(reader *bufio.Reader) {
	fmt.Println()
	fmt.Println("This rebuilds mwp from the latest main branch and reinstalls the binary")
	fmt.Println("and systemd service file -- for recovering a panel whose own files were")
	fmt.Println("deleted, corrupted, or tampered with.")
	fmt.Println("Your data and configuration under /opt/mwp/data and /opt/mwp/config are kept exactly as they are.")
	confirm := readLine(reader, "Continue? [y/N]: ")
	if confirm != "y" && confirm != "Y" {
		fmt.Println("Cancelled.")
		return
	}

	built, err := buildFromSource()
	if err != nil {
		fmt.Printf("Reinstall failed: %v\n", err)
		return
	}
	defer built.Cleanup()

	fmt.Println("==> Ensuring /opt/mwp, /opt/mwp/data, /opt/mwp/config exist")
	if err := os.MkdirAll("/opt/mwp/data", 0o755); err != nil {
		fmt.Printf("Reinstall failed: %v\n", err)
		return
	}
	if err := os.MkdirAll("/opt/mwp/config", 0o755); err != nil {
		fmt.Printf("Reinstall failed: %v\n", err)
		return
	}

	fmt.Println("==> Installing binary")
	if err := installBinary(built.BinaryPath); err != nil {
		fmt.Printf("Reinstall failed: %v\n", err)
		return
	}

	fmt.Println("==> Reinstalling systemd service file")
	unitSrc := filepath.Join(built.RepoDir, "deploy", "mwp.service")
	unitData, err := os.ReadFile(unitSrc)
	if err != nil {
		fmt.Printf("Reinstall failed: could not read %s: %v\n", unitSrc, err)
		return
	}
	if err := os.WriteFile("/etc/systemd/system/mwp.service", unitData, 0o644); err != nil {
		fmt.Printf("Reinstall failed: %v\n", err)
		return
	}
	if err := exec.Command("systemctl", "daemon-reload").Run(); err != nil {
		fmt.Printf("Warning: systemctl daemon-reload failed: %v\n", err)
	}
	if err := exec.Command("systemctl", "enable", serviceName).Run(); err != nil {
		fmt.Printf("Warning: systemctl enable %s failed: %v\n", serviceName, err)
	}

	fmt.Println("==> Restarting service")
	if err := exec.Command("systemctl", "restart", serviceName).Run(); err != nil {
		fmt.Printf("Reinstall finished, but failed to start %s automatically: %v\n", serviceName, err)
		fmt.Printf("Run `systemctl restart %s` by hand.\n", serviceName)
		return
	}

	fmt.Println("Reinstall complete. Your existing data and configuration were preserved.")
}
