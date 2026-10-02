package schema

// LogEntryResponse mirrors service.LogEntry for the admin-only Log Viewer.
type LogEntryResponse struct {
	Timestamp string                 `json:"timestamp"`
	Level     string                 `json:"level"`
	Logger    string                 `json:"logger"`
	Caller    string                 `json:"caller"`
	Message   string                 `json:"message"`
	Fields    map[string]interface{} `json:"fields,omitempty"`
}

// ListLogsResponse wraps the filtered log entries plus the full set of
// known categories, so the frontend can populate its category filter
// dropdown from the same response that supplies the rows.
type ListLogsResponse struct {
	Entries     []LogEntryResponse `json:"entries"`
	LoggerNames []string           `json:"logger_names"`
}
