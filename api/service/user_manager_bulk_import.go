package service

import (
	"regexp"
	"strconv"
	"strings"
)

// bulkImportLinePattern matches one line of a bulk-import TXT file:
//   - "username" alone -- imported with no traffic/expiry limit (unlimited)
//   - "username 12G-30d" -- imported with a 12GB traffic limit and a
//     30-day expiry counted from the moment of import
//
// The size unit is always interpreted as GB and the duration always as
// days -- this mirrors the manual single-account creation form's own
// units (CreateUserManagerAccountRequest.TrafficLimit is a GB string), so
// a bulk-imported account's limit means exactly the same thing as one
// entered by hand.
var bulkImportLinePattern = regexp.MustCompile(`^(\S+)(?:\s+(\d+(?:\.\d+)?)G-(\d+)d)?$`)

// BulkImportLine is one successfully-parsed row from an uploaded TXT file,
// before any RouterOS lookup has happened.
type BulkImportLine struct {
	Username       string
	TrafficLimitGB *string // nil = unlimited, matching CreateUserManagerAccountRequest's own convention
	ExpireDays     *int    // nil = no expiry; days from the moment of import
}

// ParseBulkImportFile splits raw TXT/CSV content into lines and parses
// each one. Blank lines and lines starting with "#" (comments) are
// silently skipped. Returns parsed lines in file order plus the raw text
// of any line that didn't match the expected format, so the caller can
// report exactly which lines were rejected without guessing.
func ParseBulkImportFile(content string) (lines []BulkImportLine, malformed []string) {
	for _, rawLine := range strings.Split(content, "\n") {
		line := strings.TrimSpace(rawLine)
		line = strings.TrimSuffix(line, "\r")
		if line == "" || strings.HasPrefix(line, "#") {
			continue
		}

		match := bulkImportLinePattern.FindStringSubmatch(line)
		if match == nil {
			malformed = append(malformed, line)
			continue
		}

		parsed := BulkImportLine{Username: match[1]}
		if match[2] != "" {
			parsed.TrafficLimitGB = &match[2]
		}
		if match[3] != "" {
			if days, err := strconv.Atoi(match[3]); err == nil {
				parsed.ExpireDays = &days
			}
		}

		lines = append(lines, parsed)
	}

	return lines, malformed
}
