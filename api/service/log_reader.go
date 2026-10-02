package service

import (
	"archive/zip"
	"bufio"
	"compress/gzip"
	"encoding/json"
	"fmt"
	"io"
	"os"
	"path/filepath"
	"sort"
	"strings"

	"go.uber.org/zap"

	"github.com/maahdima/mwp/api/config"
)

// LogEntry is one parsed line from the panel's own structured JSON log file
// (see utils/log.InitLogger -- the file sink is always JSON regardless of
// console format, specifically so this reader can parse it reliably).
type LogEntry struct {
	Timestamp string                 `json:"timestamp"`
	Level     string                 `json:"level"`
	Logger    string                 `json:"logger"`
	Caller    string                 `json:"caller"`
	Message   string                 `json:"message"`
	Fields    map[string]interface{} `json:"fields,omitempty"`
}

// LogFilter narrows ListLogs' results. All fields are optional/zero-value
// means "no filter on this dimension".
type LogFilter struct {
	// Level restricts to one severity, e.g. "error", "warn", "info", "debug".
	// Empty means all levels.
	Level string
	// Logger restricts to one category (the "logger" field zap.L().Named(...)
	// sets, e.g. "TrafficCalculatorJob", "WgPeerController"). Empty means all
	// categories. Matched case-insensitively as a substring, so "job" matches
	// every cron-job logger without needing the exact name.
	Logger string
	// Search matches (case-insensitively) against the message and the
	// caller. Empty means no text filter.
	Search string
	// Limit caps how many of the most recent matching entries are returned
	// (e.g. "last 10", "last 20"). 0 or negative means unlimited.
	Limit int
	// IncludeRotated also scans compressed rotated backups
	// (mwp-<timestamp>.log.gz) in addition to the current file, for a
	// "search my whole history" pass. Slower -- off by default.
	IncludeRotated bool
}

// LogReader reads and filters the panel's own persistent log file --
// powers the admin-only in-panel Log Viewer, so a cron-job failure (e.g.
// "why did the User Manager usage job not update this account") can be
// diagnosed directly from the UI instead of requiring SSH access.
type LogReader struct {
	logger *zap.Logger
}

func NewLogReader() *LogReader {
	return &LogReader{
		logger: zap.L().Named("LogReader"),
	}
}

// logFilePath mirrors utils/log.LogFilePath exactly, without importing
// that package (which would create an import cycle: utils/log is
// initialized before any service exists). Both must stay in sync -- see
// the constant assembly in utils/log/logger.go's LogFilePath function.
func logFilePath() string {
	appCfg := config.GetAppConfig()
	return filepath.Join(appCfg.DataDirPath, "logs", "mwp.log")
}

func logDir() string {
	return filepath.Dir(logFilePath())
}

// ListLogs returns matching entries, most recent first.
func (r *LogReader) ListLogs(filter LogFilter) ([]LogEntry, error) {
	files, err := r.candidateFiles(filter.IncludeRotated)
	if err != nil {
		return nil, err
	}

	var matched []LogEntry
	for _, f := range files {
		entries, err := r.readFile(f)
		if err != nil {
			r.logger.Warn("failed to read log file, skipping", zap.String("file", f), zap.Error(err))
			continue
		}
		for _, e := range entries {
			if matchesFilter(e, filter) {
				matched = append(matched, e)
			}
		}
	}

	// Entries within a single file are already in chronological (oldest
	// first) order since they're appended in write order; across multiple
	// files (current + rotated), sort by timestamp to interleave correctly
	// before reversing to most-recent-first.
	sort.SliceStable(matched, func(i, j int) bool {
		return matched[i].Timestamp < matched[j].Timestamp
	})

	// Reverse to most-recent-first.
	for i, j := 0, len(matched)-1; i < j; i, j = i+1, j-1 {
		matched[i], matched[j] = matched[j], matched[i]
	}

	if filter.Limit > 0 && len(matched) > filter.Limit {
		matched = matched[:filter.Limit]
	}

	return matched, nil
}

// ListLoggerNames returns the distinct "logger" (category) values seen in
// the current log file, sorted alphabetically -- powers the category
// filter dropdown in the UI without hardcoding the list of every
// zap.L().Named(...) call site in the codebase.
func (r *LogReader) ListLoggerNames() ([]string, error) {
	entries, err := r.readFile(logFilePath())
	if err != nil {
		return nil, err
	}

	seen := make(map[string]struct{})
	for _, e := range entries {
		if e.Logger != "" {
			seen[e.Logger] = struct{}{}
		}
	}

	names := make([]string, 0, len(seen))
	for name := range seen {
		names = append(names, name)
	}
	sort.Strings(names)
	return names, nil
}

// CurrentLogFilePath exposes the live log file's path, for a "download
// current log" endpoint.
func (r *LogReader) CurrentLogFilePath() string {
	return logFilePath()
}

// BuildDownloadArchive writes every log file (current + all rotated
// backups) into a single zip archive written to w -- powers the "download
// all logs" button in the UI. Returns an error if no log files exist at
// all; a per-file read failure is logged and that file is skipped rather
// than failing the whole archive.
func (r *LogReader) BuildDownloadArchive(w io.Writer) error {
	files, err := r.candidateFiles(true)
	if err != nil {
		return err
	}
	if len(files) == 0 {
		return fmt.Errorf("no log files found")
	}

	zw := zip.NewWriter(w)
	defer zw.Close()

	for _, f := range files {
		if err := addFileToZip(zw, f); err != nil {
			r.logger.Warn("failed to add log file to archive", zap.String("file", f), zap.Error(err))
		}
	}

	return nil
}

// ClearLogs empties the current log file and deletes every rotated
// backup, freeing disk space on demand. The active file is truncated in
// place (os.Truncate), NOT deleted+recreated -- zap/lumberjack hold an
// open file handle to it for the lifetime of the process, and deleting
// the file out from under them would leave that handle writing into an
// unlinked inode (invisible to anything reading the path afterward,
// including this same reader) until the next log rotation or restart.
// Truncating the same open file is safe and takes effect immediately.
func (r *LogReader) ClearLogs() error {
	current := logFilePath()
	if _, err := os.Stat(current); err == nil {
		if err := os.Truncate(current, 0); err != nil {
			return fmt.Errorf("failed to clear current log file: %w", err)
		}
	}

	dir := logDir()
	entries, err := os.ReadDir(dir)
	if err != nil {
		if os.IsNotExist(err) {
			return nil
		}
		return fmt.Errorf("failed to list log directory: %w", err)
	}

	base := filepath.Base(current)
	baseNoExt := strings.TrimSuffix(base, filepath.Ext(base))

	for _, entry := range entries {
		if entry.IsDir() {
			continue
		}
		name := entry.Name()
		if name == base {
			continue // already truncated above, not deleted
		}
		if strings.HasPrefix(name, baseNoExt) {
			if err := os.Remove(filepath.Join(dir, name)); err != nil {
				r.logger.Warn("failed to remove rotated log backup", zap.String("file", name), zap.Error(err))
			}
		}
	}

	r.logger.Info("logs cleared by admin request")
	return nil
}

func addFileToZip(zw *zip.Writer, path string) error {
	src, err := os.Open(path)
	if err != nil {
		return err
	}
	defer src.Close()

	dst, err := zw.Create(filepath.Base(path))
	if err != nil {
		return err
	}

	_, err = io.Copy(dst, src)
	return err
}

// candidateFiles returns the current log file plus, if includeRotated,
// every rotated backup lumberjack has created
// (mwp-2026-07-18T12-00-00.000.log or mwp-2026-07-18T12-00-00.000.log.gz),
// oldest first.
func (r *LogReader) candidateFiles(includeRotated bool) ([]string, error) {
	current := logFilePath()

	var files []string
	if _, err := os.Stat(current); err == nil {
		files = append(files, current)
	}

	if !includeRotated {
		return files, nil
	}

	dir := logDir()
	entries, err := os.ReadDir(dir)
	if err != nil {
		if os.IsNotExist(err) {
			return files, nil
		}
		return nil, err
	}

	base := filepath.Base(current)
	baseNoExt := strings.TrimSuffix(base, filepath.Ext(base))

	var rotated []string
	for _, entry := range entries {
		if entry.IsDir() {
			continue
		}
		name := entry.Name()
		if name == base {
			continue // already added as "current"
		}
		if strings.HasPrefix(name, baseNoExt) {
			rotated = append(rotated, filepath.Join(dir, name))
		}
	}
	sort.Strings(rotated)

	return append(rotated, files...), nil
}

// readFile parses one log file (plain or .gz) into entries, tolerating
// individual malformed lines (logged and skipped, never failing the whole
// read) since a truncated line from an in-progress write should not hide
// every other entry in the file.
func (r *LogReader) readFile(path string) ([]LogEntry, error) {
	f, err := os.Open(path)
	if err != nil {
		return nil, err
	}
	defer f.Close()

	var reader io.Reader = f
	if strings.HasSuffix(path, ".gz") {
		gz, err := gzip.NewReader(f)
		if err != nil {
			return nil, err
		}
		defer gz.Close()
		reader = gz
	}

	var entries []LogEntry
	scanner := bufio.NewScanner(reader)
	// Log lines with a large stack trace field can exceed bufio's default
	// 64KiB token limit; raise it generously rather than truncating/erroring.
	scanner.Buffer(make([]byte, 0, 64*1024), 1024*1024)

	for scanner.Scan() {
		line := strings.TrimSpace(scanner.Text())
		if line == "" {
			continue
		}

		entry, err := parseLogLine(line)
		if err != nil {
			// Not every line is guaranteed to be well-formed JSON (e.g. a
			// panic's raw stack trace can be written by something other
			// than zap in rare cases) -- skip silently rather than
			// failing the whole file's worth of otherwise-good entries.
			continue
		}
		entries = append(entries, entry)
	}

	if err := scanner.Err(); err != nil {
		return entries, err
	}

	return entries, nil
}

// parseLogLine parses one JSON log line into a LogEntry, moving every
// field not in the fixed set (timestamp/level/logger/caller/msg) into
// Fields, so structured context (zap.String/zap.Error/etc. calls) is
// preserved and shown in the UI without needing to know every possible
// field name ahead of time.
func parseLogLine(line string) (LogEntry, error) {
	var raw map[string]interface{}
	if err := json.Unmarshal([]byte(line), &raw); err != nil {
		return LogEntry{}, err
	}

	entry := LogEntry{Fields: make(map[string]interface{})}

	for k, v := range raw {
		switch k {
		case "timestamp":
			entry.Timestamp, _ = v.(string)
		case "level":
			entry.Level, _ = v.(string)
		case "logger":
			entry.Logger, _ = v.(string)
		case "caller":
			entry.Caller, _ = v.(string)
		case "msg":
			entry.Message, _ = v.(string)
		default:
			entry.Fields[k] = v
		}
	}

	if len(entry.Fields) == 0 {
		entry.Fields = nil
	}

	return entry, nil
}

func matchesFilter(e LogEntry, filter LogFilter) bool {
	if filter.Level != "" && !strings.EqualFold(e.Level, filter.Level) {
		return false
	}
	if filter.Logger != "" && !strings.Contains(strings.ToLower(e.Logger), strings.ToLower(filter.Logger)) {
		return false
	}
	if filter.Search != "" {
		search := strings.ToLower(filter.Search)
		if !strings.Contains(strings.ToLower(e.Message), search) &&
			!strings.Contains(strings.ToLower(e.Caller), search) {
			return false
		}
	}
	return true
}
