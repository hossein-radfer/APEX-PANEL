package cli

import (
	"fmt"
	"os/exec"
)

// runSystemctl shells out to `systemctl <verb> mwp` -- this menu is only
// ever reached via `mwp menu` run interactively at an SSH prompt (see
// deploy/mwp.service's doc comment on why the panel itself runs as a
// systemd service rather than a bare nohup'd process), so the operator
// already has whatever privileges are needed to manage that unit; this
// simply saves them from having to remember/type the exact systemctl
// invocation themselves, mirroring x-ui's menu convenience.
func runSystemctl(verb string) error {
	cmd := exec.Command("systemctl", verb, serviceName)
	cmd.Stdout = nil
	out, err := cmd.CombinedOutput()
	if len(out) > 0 {
		fmt.Print(string(out))
	}
	return err
}

func actionStatus() {
	fmt.Println()
	if err := runSystemctl("status"); err != nil {
		// systemctl status exits non-zero when the service isn't running,
		// which is a normal, expected outcome here (not a failure of this
		// menu action) -- its own output already explained the state above.
		fmt.Println()
	}
}

func actionStart() {
	fmt.Println()
	if err := runSystemctl("start"); err != nil {
		fmt.Printf("Failed to start %s: %v\n", serviceName, err)
		return
	}
	fmt.Printf("%s started.\n", serviceName)
}

func actionStop() {
	fmt.Println()
	if err := runSystemctl("stop"); err != nil {
		fmt.Printf("Failed to stop %s: %v\n", serviceName, err)
		return
	}
	fmt.Printf("%s stopped.\n", serviceName)
}

func actionRestart() {
	fmt.Println()
	if err := runSystemctl("restart"); err != nil {
		fmt.Printf("Failed to restart %s: %v\n", serviceName, err)
		return
	}
	fmt.Printf("%s restarted.\n", serviceName)
}
