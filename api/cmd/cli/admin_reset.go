package cli

import (
	"bufio"
	"fmt"
	"strconv"
	"strings"

	"golang.org/x/crypto/bcrypt"
	"gorm.io/gorm"

	"github.com/maahdima/mwp/api/config"
	"github.com/maahdima/mwp/api/dataservice"
	"github.com/maahdima/mwp/api/dataservice/model"
	"github.com/maahdima/mwp/api/service"
)

// actionResetAdmin overwrites an existing admin's password directly in
// the database, for recovering access when the web UI password has been
// lost. Only reachable via interactive/SSH access to the server itself.
func actionResetAdmin(reader *bufio.Reader) {
	fmt.Println()
	db, err := connectDB()
	if err != nil {
		fmt.Printf("Failed to connect to database: %v\n", err)
		return
	}

	var admins []model.Admin
	if err := db.Find(&admins).Error; err != nil {
		fmt.Printf("Failed to list admin accounts: %v\n", err)
		return
	}
	if len(admins) == 0 {
		fmt.Println("No admin accounts exist yet -- one will be created automatically the next time the panel starts (from ADMIN_USERNAME/ADMIN_PASSWORD).")
		return
	}

	fmt.Println("Existing admin accounts:")
	for i, a := range admins {
		fmt.Printf("  %d) %s\n", i+1, a.Username)
	}

	choice := readLine(reader, fmt.Sprintf("Select an account to reset [1-%d]: ", len(admins)))
	idx, err := strconv.Atoi(choice)
	if err != nil || idx < 1 || idx > len(admins) {
		fmt.Println("Invalid selection.")
		return
	}
	target := admins[idx-1]

	newUsername := readLine(reader, fmt.Sprintf("New username (leave empty to keep '%s'): ", target.Username))
	newPassword := readLine(reader, "New password (leave empty to keep the current one): ")

	if newUsername == "" && newPassword == "" {
		fmt.Println("Nothing to change.")
		return
	}

	if newUsername != "" {
		target.Username = newUsername
	}
	if newPassword != "" {
		hashed, err := bcrypt.GenerateFromPassword([]byte(newPassword), bcrypt.DefaultCost)
		if err != nil {
			fmt.Printf("Failed to hash new password: %v\n", err)
			return
		}
		target.Password = string(hashed)
	}
	target.IsActive = true

	if err := db.Save(&target).Error; err != nil {
		fmt.Printf("Failed to save admin account: %v\n", err)
		return
	}

	fmt.Printf("Admin account updated: username=%s\n", target.Username)
	if newPassword != "" {
		fmt.Println("Password changed. Existing login sessions for this account remain valid until their tokens expire.")
	}
}

// actionChangePort writes the SystemConfig port-override row that
// api/cmd/main.go reads on its NEXT start -- it cannot rebind the port of
// an already-running server (Echo binds its listener once at startup), so
// this always ends with an explicit instruction to restart.
func actionChangePort(reader *bufio.Reader) {
	fmt.Println()
	db, err := connectDB()
	if err != nil {
		fmt.Printf("Failed to connect to database: %v\n", err)
		return
	}

	appCfg := config.GetAppConfig()
	fmt.Printf("Current port (from SERVER_PORT/default, before any override): %s\n", appCfg.Port)

	portStr := readLine(reader, "New port [1-65535]: ")
	port, err := strconv.Atoi(strings.TrimSpace(portStr))
	if err != nil || port < 1 || port > 65535 {
		fmt.Println("Invalid port.")
		return
	}

	systemConfigService := service.NewSystemConfigService(db)
	if err := systemConfigService.SetPortOverride(port); err != nil {
		fmt.Printf("Failed to save port override: %v\n", err)
		return
	}

	fmt.Printf("Port override saved: %d\n", port)
	fmt.Printf("Run `systemctl restart %s` (or use option 4 in this menu) for it to take effect.\n", serviceName)
}

// connectDB opens a short-lived connection for one menu action.
// AutoMigrate is deliberately NOT run here -- the menu only ever operates
// against an existing, already-migrated install (a fresh install has
// nothing to reset/reconfigure yet), and running AutoMigrate concurrently
// with a possibly-running server process risks racing its own migration.
func connectDB() (*gorm.DB, error) {
	dbCfg := config.GetDBConfig()
	return dataservice.ConnectDB(dbCfg)
}
