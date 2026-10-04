package cli

import (
	"bufio"
	"fmt"
	"os"
	"os/exec"

	"github.com/maahdima/mwp/api/config"
)

// actionDeletePanel permanently removes this panel install: stops and
// disables the systemd unit, then deletes the unit file, the installed
// binary, and /opt/mwp (config + database + backups). Two explicit Y/N
// confirmations are required before anything is touched -- this is the
// single most destructive action in the whole menu (every customer, every
// peer, every credential, gone with no undo), so it gets the same
// "confirm twice" friction this project already uses for its other
// genuinely irreversible actions (e.g. the web UI's reset-usage dialogs),
// just doubled since there is no database row left to inspect afterward
// if this runs by mistake.
func actionDeletePanel(reader *bufio.Reader) {
	fmt.Println()
	fmt.Println("This permanently deletes MWPanel: the systemd service, the installed")
	fmt.Println("binary, and ALL data under /opt/mwp (database, backups, config).")
	fmt.Println("This cannot be undone -- take a backup first (option 8) if you have not already.")
	fmt.Println()

	first := readLine(reader, "Type 'yes' to continue: ")
	if first != "yes" {
		fmt.Println("Cancelled.")
		return
	}

	second := readLine(reader, "Are you absolutely sure? Type 'DELETE' (all caps) to confirm: ")
	if second != "DELETE" {
		fmt.Println("Cancelled.")
		return
	}

	appCfg := config.GetAppConfig()
	dataDir := appCfg.DataDirPath
	if dataDir == "" {
		dataDir = "/opt/mwp/data"
	}

	fmt.Println()
	fmt.Println("Stopping and disabling the service...")
	_ = exec.Command("systemctl", "stop", serviceName).Run()
	_ = exec.Command("systemctl", "disable", serviceName).Run()

	unitPath := "/etc/systemd/system/mwp.service"
	if err := os.Remove(unitPath); err != nil && !os.IsNotExist(err) {
		fmt.Printf("Warning: failed to remove %s: %v\n", unitPath, err)
	}
	_ = exec.Command("systemctl", "daemon-reload").Run()

	binaryPath := "/usr/local/bin/mwp"
	if err := os.Remove(binaryPath); err != nil && !os.IsNotExist(err) {
		fmt.Printf("Warning: failed to remove %s: %v\n", binaryPath, err)
	}

	// /opt/mwp is this install's whole world (config + the data dir
	// AdminSeed/BackupService read and write under) -- removing the parent
	// covers both in one step rather than tracking the two paths
	// separately and risking one surviving a partial failure.
	installDir := "/opt/mwp"
	if err := os.RemoveAll(installDir); err != nil {
		fmt.Printf("Warning: failed to remove %s: %v\n", installDir, err)
		fmt.Println("Remove it by hand to finish the uninstall.")
	}

	fmt.Println()
	fmt.Println("MWPanel has been deleted from this server.")
	fmt.Println("This process will now exit -- there is nothing left for this menu to manage.")
	os.Exit(0)
}
