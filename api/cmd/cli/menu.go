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
			actionStatus()
		case "2":
			actionStart()
		case "3":
			actionStop()
		case "4":
			actionRestart()
		case "5":
			actionResetAdmin(reader)
		case "6":
			actionChangePort(reader)
		case "7":
			actionViewLogs(reader)
		case "8":
			actionBackup()
		case "9":
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

func printMenu() {
	fmt.Println()
	fmt.Println("========================================")
	fmt.Println(" MWPanel Management Menu")
	fmt.Println("========================================")
	fmt.Println(" 1) Service status")
	fmt.Println(" 2) Start service")
	fmt.Println(" 3) Stop service")
	fmt.Println(" 4) Restart service")
	fmt.Println(" 5) Reset admin credentials")
	fmt.Println(" 6) Change listen port")
	fmt.Println(" 7) View live logs")
	fmt.Println(" 8) Create a database backup")
	fmt.Println(" 9) Restore a database backup")
	fmt.Println(" 0) Exit")
	fmt.Println("========================================")
}

func readLine(reader *bufio.Reader, prompt string) string {
	fmt.Print(prompt)
	line, _ := reader.ReadString('\n')
	return strings.TrimSpace(line)
}
