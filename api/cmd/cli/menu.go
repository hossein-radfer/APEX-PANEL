// Package cli implements the interactive, SSH-invoked management menu --
// `mwp menu` -- offering the same "quick server admin" actions an x-ui-style
// panel provides, without requiring the operator to know this codebase's
// internals. It is entirely separate from the HTTP server: running `mwp`
// with no arguments still starts the panel exactly as before (see
// api/cmd/main.go), and this package is only reached via an explicit
// `menu` subcommand.
package cli

import (
	"bufio"
	"fmt"
	"os"
	"strings"
)

// serviceName is the systemd unit this menu's status/start/stop/restart
// actions operate on -- see deploy/mwp.service. Hardcoded rather than
// auto-detected since this project ships exactly one supported deployment
// shape (a systemd service named "mwp").
const serviceName = "mwp"

// Run is the entry point for `mwp menu`, invoked from api/cmd/main.go
// before any of the normal server-startup wiring happens -- the menu opens
// its own short-lived DB connection per action instead of reusing a
// long-running one, since most actions (backup, reset-admin) are one-shot
// and the process exits back to the shell after each menu interaction
// unless the operator chooses to stay in the loop.
func Run() {
	reader := bufio.NewReader(os.Stdin)

	for {
		printMenu()
		choice := readLine(reader, "Select an option: ")

		switch choice {
		case "1":
			actionUpdate(reader)
		case "2":
			actionDeletePanel(reader)
		case "3":
			actionReinstall(reader)
		case "4":
			actionBackup()
		case "5":
			actionResetAdmin(reader)
		case "6":
			actionDisableOtp()
		case "7":
			actionStatus()
		case "8":
			actionAutoSSL(reader)
		case "9":
			actionChangePort(reader)
		case "10":
			actionRestart()
		case "11":
			actionViewLogs(reader)
		case "12":
			actionStart()
		case "13":
			actionStop()
		case "14":
			actionRestore(reader)
		case "0", "q", "Q":
			fmt.Println("Goodbye.")
			return
		default:
			fmt.Println("Invalid option, try again.")
		}

		fmt.Println()
	}
}

// printMenu's numbering follows the admin's own originally-specified
// 11-item order (update, delete, reinstall, backup, admin credentials,
// disable 2FA, status, auto SSL, change port, restart, view logs) first,
// with the remaining actions this menu already had before that spec
// (start/stop individually, restore) appended after -- renumbering the
// pre-existing items would have silently broken anyone's muscle memory
// or scripted input from the menu's first shipped version for no benefit,
// since the spec itself never required a specific position for them.
func printMenu() {
	fmt.Println()
	fmt.Println("========================================")
	fmt.Println(" MWPanel Management Menu")
	fmt.Println("========================================")
	fmt.Println(" 1) Update panel (pull latest + rebuild)")
	fmt.Println(" 2) Delete panel (irreversible)")
	fmt.Println(" 3) Reinstall panel (keeps your data)")
	fmt.Println(" 4) Create a database backup")
	fmt.Println(" 5) Reset admin credentials")
	fmt.Println(" 6) Disable two-factor login")
	fmt.Println(" 7) Service status")
	fmt.Println(" 8) Get a free SSL certificate (nginx + Let's Encrypt)")
	fmt.Println(" 9) Change listen port")
	fmt.Println("10) Restart service")
	fmt.Println("11) View live logs")
	fmt.Println("12) Start service")
	fmt.Println("13) Stop service")
	fmt.Println("14) Restore a database backup")
	fmt.Println(" 0) Exit")
	fmt.Println("========================================")
}

func readLine(reader *bufio.Reader, prompt string) string {
	fmt.Print(prompt)
	line, _ := reader.ReadString('\n')
	return strings.TrimSpace(line)
}
