package service

import (
	"bufio"
	"fmt"
	"os"
	"strconv"
	"strings"
	"time"

	"go.uber.org/zap"
	"gorm.io/gorm"
)

// SystemHealthService reads live host resource usage (CPU/RAM/disk) plus
// this install's own backup freshness -- the admin's own explicit request
// (فاز پنجم-۵: "صفحه‌ی وضعیت سلامت پنل... مصرف CPU/RAM/دیسک زنده، اندازه‌ی
// دیتابیس، وضعیت آخرین بکاپ -- همه در یک نگاه"). Deliberately Linux-only,
// reading /proc directly rather than adding a new third-party dependency
// (gopsutil or similar) -- this codebase has zero existing system-metrics
// library, and every deploy target already documented in this repo's own
// deploy/README.md is a systemd unit on Debian/Ubuntu, so /proc is always
// present in practice. Falls back to a clear "unavailable" zero-value
// result (never panics/errors the whole request) on any non-Linux host or
// read failure, since a health check that itself crashes the page it's
// meant to diagnose would defeat the purpose.
type SystemHealthService struct {
	db             *gorm.DB
	dataDirPath    string
	botSettings    *BotSettingsService
	logger         *zap.Logger
	prevCPUSample  cpuSample
	havePrevSample bool
}

type cpuSample struct {
	idle  uint64
	total uint64
}

func NewSystemHealthService(db *gorm.DB, dataDirPath string, botSettings *BotSettingsService) *SystemHealthService {
	return &SystemHealthService{
		db:          db,
		dataDirPath: dataDirPath,
		botSettings: botSettings,
		logger:      zap.L().Named("SystemHealthService"),
	}
}

// MemoryHealth reports host-wide RAM usage in bytes, read from
// /proc/meminfo -- MemAvailable (not MemFree) is used for "free" since
// MemFree alone excludes reclaimable page cache/buffers and would report
// a Linux host as far more "full" than it actually behaves under memory
// pressure (this is the exact same distinction `free -h`'s own
// available column makes).
type MemoryHealth struct {
	TotalBytes     int64
	AvailableBytes int64
	UsedBytes      int64
}

// DiskHealth reports free/used space for the filesystem backing
// DataDirPath (where the SQLite database and peer config files live) --
// not the root filesystem in general, since DataDirPath's own filesystem
// is the one that actually matters for this panel's own health.
type DiskHealth struct {
	TotalBytes int64
	FreeBytes  int64
	UsedBytes  int64
}

// BackupHealth surfaces whether today's automatic backup already ran --
// see model.BotSettings.LastBackupSentDate's own doc comment: this is a
// date-only marker (UTC "2006-01-02"), not a timestamp, and there is no
// separate success/failure flag on it -- a stale/absent date is the only
// available "did it work" signal.
type BackupHealth struct {
	AutoBackupEnabled  bool
	LastBackupSentDate string // "" if never sent
	IsUpToDate         bool   // true if LastBackupSentDate is today (UTC)
}

// SystemHealth bundles every tile the health dashboard shows -- CPUPercent
// is -1 when unavailable (first call ever, or a read failure) since 0%
// load is a real, meaningful value and must not be confused with "unknown".
type SystemHealth struct {
	CPUPercent float64
	Memory     MemoryHealth
	Disk       DiskHealth
	Backup     BackupHealth
}

// GetSystemHealth assembles the full health snapshot. Each of the four
// readings is independently best-effort -- a failure reading one (e.g.
// disk stat failing on an exotic filesystem) never blocks the other
// three; the failed one just reports its own zero-value defaults.
func (s *SystemHealthService) GetSystemHealth() *SystemHealth {
	health := &SystemHealth{CPUPercent: -1}

	if cpuPercent, err := s.readCPUPercent(); err != nil {
		s.logger.Warn("failed to read cpu usage", zap.Error(err))
	} else {
		health.CPUPercent = cpuPercent
	}

	if mem, err := s.readMemory(); err != nil {
		s.logger.Warn("failed to read memory usage", zap.Error(err))
	} else {
		health.Memory = *mem
	}

	if disk, err := s.readDisk(); err != nil {
		s.logger.Warn("failed to read disk usage", zap.Error(err))
	} else {
		health.Disk = *disk
	}

	health.Backup = s.readBackupHealth()

	return health
}

// readCPUPercent computes overall CPU utilization as the delta between
// two /proc/stat samples taken between this call and the previous one --
// a single instantaneous /proc/stat read cannot give a percentage at all
// (the counters are cumulative since boot), so the first call after
// process start always returns (0, nil) with havePrevSample still false,
// and GetSystemHealth's caller sees CPUPercent become meaningful only
// from the second call onward. This matches every other "live CPU %"
// tool's own behavior (top, htop) -- they all need two samples too.
func (s *SystemHealthService) readCPUPercent() (float64, error) {
	sample, err := readCPUSample()
	if err != nil {
		return 0, err
	}

	if !s.havePrevSample {
		s.prevCPUSample = sample
		s.havePrevSample = true
		return 0, nil
	}

	totalDelta := float64(sample.total - s.prevCPUSample.total)
	idleDelta := float64(sample.idle - s.prevCPUSample.idle)
	s.prevCPUSample = sample

	if totalDelta <= 0 {
		return 0, nil
	}
	return (1 - idleDelta/totalDelta) * 100, nil
}

// readCPUSample parses the first "cpu " line of /proc/stat -- the
// aggregate across all cores. Fields (in order): user, nice, system,
// idle, iowait, irq, softirq, steal, guest, guest_nice. idle+iowait both
// count as "idle" for utilization purposes (iowait is CPU idle time
// waiting on disk I/O, not CPU work).
func readCPUSample() (cpuSample, error) {
	f, err := os.Open("/proc/stat")
	if err != nil {
		return cpuSample{}, err
	}
	defer f.Close()

	scanner := bufio.NewScanner(f)
	if !scanner.Scan() {
		return cpuSample{}, fmt.Errorf("empty /proc/stat")
	}
	fields := strings.Fields(scanner.Text())
	if len(fields) < 5 || fields[0] != "cpu" {
		return cpuSample{}, fmt.Errorf("unexpected /proc/stat format")
	}

	values := make([]uint64, 0, len(fields)-1)
	for _, f := range fields[1:] {
		v, err := strconv.ParseUint(f, 10, 64)
		if err != nil {
			return cpuSample{}, err
		}
		values = append(values, v)
	}

	var total uint64
	for _, v := range values {
		total += v
	}
	idle := values[3] // idle
	if len(values) > 4 {
		idle += values[4] // iowait
	}

	return cpuSample{idle: idle, total: total}, nil
}

// readMemory parses /proc/meminfo for MemTotal/MemAvailable, both
// reported in kB by the kernel.
func (s *SystemHealthService) readMemory() (*MemoryHealth, error) {
	f, err := os.Open("/proc/meminfo")
	if err != nil {
		return nil, err
	}
	defer f.Close()

	var totalKB, availableKB int64
	scanner := bufio.NewScanner(f)
	for scanner.Scan() {
		line := scanner.Text()
		switch {
		case strings.HasPrefix(line, "MemTotal:"):
			totalKB = parseMeminfoValue(line)
		case strings.HasPrefix(line, "MemAvailable:"):
			availableKB = parseMeminfoValue(line)
		}
	}
	if err := scanner.Err(); err != nil {
		return nil, err
	}

	total := totalKB * 1024
	available := availableKB * 1024
	return &MemoryHealth{
		TotalBytes:     total,
		AvailableBytes: available,
		UsedBytes:      total - available,
	}, nil
}

func parseMeminfoValue(line string) int64 {
	fields := strings.Fields(line)
	if len(fields) < 2 {
		return 0
	}
	v, _ := strconv.ParseInt(fields[1], 10, 64)
	return v
}

// readDisk reports free/used space for the filesystem backing
// DataDirPath -- implemented per-OS in system_health_disk_linux.go /
// system_health_disk_other.go, since the underlying syscall
// (syscall.Statfs) only exists on Linux/Unix, not Windows. This
// indirection lets the rest of this file (CPU/RAM/backup reading) still
// compile on any platform for local development, even though the CPU/RAM
// readers are themselves Linux-only in practice (see this file's own
// doc comment) -- only the disk syscall actually FAILS to compile
// elsewhere.

// readBackupHealth reads BotSettings' own backup-tracking fields --
// best-effort: if settings can't be loaded (shouldn't normally happen,
// GetOrCreate self-heals), reports everything as "unknown"/false rather
// than failing the whole health snapshot.
func (s *SystemHealthService) readBackupHealth() BackupHealth {
	if s.botSettings == nil {
		return BackupHealth{}
	}
	settings, err := s.botSettings.GetOrCreate()
	if err != nil {
		s.logger.Warn("failed to load bot settings for backup health", zap.Error(err))
		return BackupHealth{}
	}

	today := time.Now().UTC().Format("2006-01-02")
	return BackupHealth{
		AutoBackupEnabled:  settings.AutoBackupEnabled,
		LastBackupSentDate: settings.LastBackupSentDate,
		IsUpToDate:         settings.LastBackupSentDate == today,
	}
}
