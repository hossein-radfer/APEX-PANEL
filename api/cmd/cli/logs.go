package cli

import (
	"bufio"
	"fmt"

	"github.com/maahdima/mwp/api/service"
)

// actionViewLogs reuses the exact same LogReader that backs the in-panel,
// JWT-protected Log Viewer (api/service/log_reader.go / api/http/log.go)
// -- this menu is just another caller of that same read-only primitive,
// with no new log-parsing logic.
func actionViewLogs(reader *bufio.Reader) {
	fmt.Println()
	levelStr := readLine(reader, "Filter by level (blank for all; error/warn/info/debug): ")
	searchStr := readLine(reader, "Filter by text (blank for none): ")
	limitStr := readLine(reader, "How many recent entries to show [default 50]: ")

	limit := 50
	if limitStr != "" {
		if parsed, err := parsePositiveInt(limitStr); err == nil {
			limit = parsed
		}
	}

	logReader := service.NewLogReader()
	entries, err := logReader.ListLogs(service.LogFilter{
		Level:  levelStr,
		Search: searchStr,
		Limit:  limit,
	})
	if err != nil {
		fmt.Printf("Failed to read logs: %v\n", err)
		return
	}
	if len(entries) == 0 {
		fmt.Println("No matching log entries.")
		return
	}

	// Print oldest-of-the-selected-window first, so output reads top-to-
	// bottom in chronological order like `journalctl -f` / `tail`, even
	// though ListLogs itself returns most-recent-first.
	for i := len(entries) - 1; i >= 0; i-- {
		e := entries[i]
		fmt.Printf("[%s] %-5s %-20s %s\n", e.Timestamp, e.Level, e.Logger, e.Message)
	}
}

func parsePositiveInt(s string) (int, error) {
	var n int
	_, err := fmt.Sscanf(s, "%d", &n)
	if err != nil {
		return 0, err
	}
	if n <= 0 {
		return 0, fmt.Errorf("must be positive")
	}
	return n, nil
}
