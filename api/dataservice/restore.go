package dataservice

import (
	"os"
	"path/filepath"

	"go.uber.org/zap"

	"github.com/maahdima/mwp/api/config"
)

// stagedRestoreFileName mirrors service.BackupService.StageRestore's own
// hardcoded staged-file name -- kept as a literal here (rather than an
// import of the service package, which would create an import cycle since
// service already imports dataservice) since both sides need to agree on
// exactly one fixed path.
const stagedRestoreFileName = "mwp-pending-restore.db"

// ApplyPendingRestore checks for a database file staged by the web panel's
// "Restore Backup" upload (service.BackupService.StageRestore) and, if
// present, atomically replaces the live database file with it before the
// caller opens any connection to that file.
//
// This MUST run before ConnectDB is ever called for the process -- once
// GORM/the sqlite driver hold an open connection pool against the live
// file, replacing that file out from under the pool risks corruption or an
// inconsistent mix of old and new state across different pooled
// connections (see StageRestore's doc comment). Running here, before any
// connection exists, is what makes the swap safe: there is nothing to be
// "out from under".
//
// This is the actual restore-application step -- staging alone (what the
// upload endpoint does) never touches the live database. Restoring a
// backup therefore takes effect on the panel's next restart, which matches
// what the web UI already tells the admin to expect.
func ApplyPendingRestore(dbCfg config.DBConfig) error {
	logger := zap.L().Named("Restore")

	if dbCfg.Dialect != "sqlite" {
		return nil
	}

	appCfg := config.GetAppConfig()
	stagedPath := filepath.Join(appCfg.DataDirPath, "backups", stagedRestoreFileName)

	if _, err := os.Stat(stagedPath); os.IsNotExist(err) {
		return nil
	} else if err != nil {
		logger.Error("failed to check for staged restore file", zap.Error(err))
		return err
	}

	logger.Warn("staged database restore found; applying it now, before opening the live database",
		zap.String("staged_path", stagedPath), zap.String("live_db_path", dbCfg.Database))

	// os.Rename is atomic on the same filesystem (both paths are under the
	// same DataDirPath here), so there is no window where the live DB file
	// is missing or partially written.
	if err := os.Rename(stagedPath, dbCfg.Database); err != nil {
		logger.Error("failed to apply staged restore", zap.Error(err))
		return err
	}

	// Sidecar WAL/SHM files from the PREVIOUS live database would otherwise
	// be silently reused against the newly-restored file, which belongs to
	// a different logical database -- sqlite would try to replay a WAL
	// journal that has nothing to do with the restored content. Removing
	// them forces a clean open.
	for _, suffix := range []string{"-wal", "-shm"} {
		sidecar := dbCfg.Database + suffix
		if err := os.Remove(sidecar); err != nil && !os.IsNotExist(err) {
			logger.Warn("failed to remove stale sqlite sidecar file after restore", zap.String("path", sidecar), zap.Error(err))
		}
	}

	logger.Info("database restore applied successfully")
	return nil
}
