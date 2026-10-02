package cli

import (
	"bufio"
	"fmt"
	"os"

	"github.com/maahdima/mwp/api/config"
	"github.com/maahdima/mwp/api/service"
)

// actionBackup reuses BackupService.CreateBackup exactly as the web UI's
// download-backup endpoint does, except the resulting file is left in
// place under <DataDirPath>/backups instead of being streamed out and
// cleaned up -- an operator running this from an SSH session wants the
// file to persist on disk (e.g. to scp it off afterward), not a one-shot
// HTTP download.
func actionBackup() {
	fmt.Println()
	appCfg := config.GetAppConfig()
	dbCfg := config.GetDBConfig()

	db, err := connectDB()
	if err != nil {
		fmt.Printf("Failed to connect to database: %v\n", err)
		return
	}

	backupService := service.NewBackupService(db, dbCfg.Dialect, dbCfg.Database, appCfg.DataDirPath)
	path, _, err := backupService.CreateBackup()
	if err != nil {
		fmt.Printf("Backup failed: %v\n", err)
		return
	}

	fmt.Printf("Backup created: %s\n", path)
}

// actionRestore stages an uploaded/local backup file the same way the web
// UI's restore endpoint does (BackupService.StageRestore) -- it never
// hot-swaps the live database (see that method's doc comment for why),
// so this always ends with an explicit restart instruction, exactly like
// the change-port action above.
func actionRestore(reader *bufio.Reader) {
	fmt.Println()
	path := readLine(reader, "Path to the backup .db file to restore: ")
	if path == "" {
		fmt.Println("No path given.")
		return
	}

	content, err := os.ReadFile(path)
	if err != nil {
		fmt.Printf("Failed to read file: %v\n", err)
		return
	}

	appCfg := config.GetAppConfig()
	dbCfg := config.GetDBConfig()

	db, err := connectDB()
	if err != nil {
		fmt.Printf("Failed to connect to database: %v\n", err)
		return
	}

	backupService := service.NewBackupService(db, dbCfg.Dialect, dbCfg.Database, appCfg.DataDirPath)
	stagedPath, err := backupService.StageRestore(content)
	if err != nil {
		fmt.Printf("Failed to stage restore: %v\n", err)
		return
	}

	fmt.Printf("Restore staged at: %s\n", stagedPath)
	fmt.Printf("Run `systemctl restart %s` (or use option 4 in this menu) to apply it -- the staged file is swapped in automatically on the next process start.\n", serviceName)
}
