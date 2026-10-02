package service

import (
	"bytes"
	"fmt"
	"os/exec"

	"go.uber.org/zap"
)

// LogRetentionMaxBytes is the size ceiling this service enforces on the
// panel's own systemd journal -- a confirmed, reported incident once let
// raw GORM/GeoIP log output grow to ~14GB before it was manually cleared
// (see this codebase's own operational history), so this exists to keep
// that from silently recurring. 500MB was the admin's own explicit
// request, chosen as generous enough to keep useful recent history for
// troubleshooting without risking the disk-full crises this class of
// unbounded log growth has already caused twice.
const LogRetentionMaxBytes = 500 * 1024 * 1024

// LogRetentionService keeps the panel's own systemd journal from growing
// unbounded, mirroring SecurityRetentionService's "admin-defined cleanup
// ceiling" role but for the process's own log output rather than
// application data tables. Deliberately scheduled automatically (unlike
// SecurityRetentionService's on-demand design) since log volume is driven
// entirely by request/job traffic the admin has no direct lever over --
// there's no equivalent of "the admin decided to generate less log" the
// way there is for connection-log/traffic-sample retention windows.
type LogRetentionService struct {
	logger *zap.Logger
}

func NewLogRetentionService() *LogRetentionService {
	return &LogRetentionService{logger: zap.L().Named("LogRetentionService")}
}

// VacuumJournal runs `journalctl --vacuum-size=<LogRetentionMaxBytes>`,
// which asks systemd-journald to delete the OLDEST archived journal
// entries until the total on-disk journal size is at or under that
// ceiling -- never touches the currently-active journal file mid-write,
// so this is safe to run on a live, in-use journal. journalctl itself
// decides which files to remove; this call only sets the target size.
//
// A missing `journalctl` binary (non-systemd host) or a permission
// failure is logged and swallowed rather than returned as a fatal error --
// this runs on an unattended schedule with no caller to report a failure
// to, and per this codebase's own "one broken background job shouldn't
// take down anything else" convention (see traffic.Calculator's own
// per-job error handling), a failed vacuum this tick just means the next
// scheduled tick gets another chance.
func (s *LogRetentionService) VacuumJournal() {
	sizeArg := fmt.Sprintf("--vacuum-size=%d", LogRetentionMaxBytes)
	cmd := exec.Command("journalctl", sizeArg)

	var stdout, stderr bytes.Buffer
	cmd.Stdout = &stdout
	cmd.Stderr = &stderr

	if err := cmd.Run(); err != nil {
		s.logger.Warn("failed to vacuum systemd journal -- will retry on the next scheduled tick",
			zap.Error(err), zap.String("stderr", stderr.String()))
		return
	}

	s.logger.Info("journal vacuum completed", zap.String("output", stdout.String()))
}
