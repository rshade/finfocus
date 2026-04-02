package config

// History configuration constants. These live in the config package to avoid a
// dependency inversion where config would otherwise import internal/history.
const (
	// HistoryEnvEnabled is the environment variable for enabling/disabling history.
	HistoryEnvEnabled = "FINFOCUS_HISTORY_ENABLED"

	// HistoryEnvDir is the environment variable for history directory.
	HistoryEnvDir = "FINFOCUS_HISTORY_DIR"

	// HistoryEnvRetentionDays is the environment variable for retention period.
	HistoryEnvRetentionDays = "FINFOCUS_HISTORY_RETENTION_DAYS"
)
