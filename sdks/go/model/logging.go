package model

import "strings"

// Log levels accepted by the daemon logger. These map 1:1 to the four built-in
// log/slog levels. "warning" is accepted as an alias for "warn" on input.
const (
	LogLevelDebug = "debug"
	LogLevelInfo  = "info"
	LogLevelWarn  = "warn"
	LogLevelError = "error"
)

// LogLevels is the canonical, ordered set of accepted log levels (most to least
// verbose). Used for validation and help text.
var LogLevels = []string{
	LogLevelDebug,
	LogLevelInfo,
	LogLevelWarn,
	LogLevelError,
}

// NormalizeLogLevel lowercases/trims a level string and folds the "warning"
// alias to "warn". It returns the normalized value and whether it is valid.
func NormalizeLogLevel(level string) (string, bool) {
	switch normalized := strings.ToLower(strings.TrimSpace(level)); normalized {
	case LogLevelDebug, LogLevelInfo, LogLevelError:
		return normalized, true
	case LogLevelWarn, "warning":
		return LogLevelWarn, true
	default:
		return normalized, false
	}
}

// LogState is the daemon's current logging configuration, returned by the
// node's logging API.
type LogState struct {
	// Enabled reports whether the daemon is currently writing logs.
	Enabled bool `json:"enabled"`
	// Level is the active minimum log level (one of LogLevels).
	Level string `json:"level"`
	// Filepath is the absolute path of the rotating log file the daemon writes.
	Filepath string `json:"filepath"`
	// Format is the log line format ("text" or "json").
	Format string `json:"format"`
}

// SetLogStateReq is a partial update to the daemon's logging configuration.
// Nil fields are left unchanged, allowing independent toggling of enablement
// and level.
type SetLogStateReq struct {
	// Enabled, when non-nil, turns daemon logging on or off.
	Enabled *bool `json:"enabled,omitempty"`
	// Level, when non-nil, sets the active minimum log level.
	Level *string `json:"level,omitempty"`
}
