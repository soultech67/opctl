// Package logging owns the opctl daemon's process-wide logger.
//
// The daemon is long-lived and, historically, only ever logged to the stdout/
// stderr of whichever CLI invocation first spawned it — which is usually long
// gone by the time something goes wrong. This package gives those logs a
// durable, rotating home on disk (under the data dir) while still echoing them
// to stderr, so post-mortem diagnosis is possible.
//
// It is intentionally a package-level singleton, mirroring how the codebase
// already uses the stdlib log package: Init wires up the default slog logger
// and redirects the stdlib log package's output to the same destination, so
// pre-existing log.Output(...) instrumentation (e.g. the Docker runtime's
// "[opctl docker]" lines) is captured for free.
//
// Level and enablement are controlled at startup via env vars (see Init) and
// can be changed at runtime via SetLevel/SetEnabled, which back the node's
// logging API and the `opctl doctor` CLI commands.
package logging

import (
	"fmt"
	"io"
	"log"
	"log/slog"
	"os"
	"path/filepath"
	"runtime/debug"
	"strings"
	"sync"
	"sync/atomic"

	"github.com/opctl/opctl/sdks/go/internal/unsudo"
	"github.com/opctl/opctl/sdks/go/model"
	lumberjack "gopkg.in/natefinch/lumberjack.v2"
)

// Environment variables read by Init. These are forwarded to the daemon by the
// local node provider (see daemonEnvPassThroughVars).
const (
	// EnvEnabled toggles daemon logging on/off at startup (default on).
	// Accepts 1/true/yes/on and 0/false/no/off.
	EnvEnabled = "OPCTL_LOG"
	// EnvLevel sets the startup log level (one of model.LogLevels; default info).
	EnvLevel = "OPCTL_LOG_LEVEL"
	// EnvFormat sets the log line format: "text" (default) or "json".
	EnvFormat = "OPCTL_LOG_FORMAT"
	// EnvFile overrides the log file path (default <data-dir>/logs/node.log).
	EnvFile = "OPCTL_LOG_FILE"
)

const (
	formatText = "text"
	formatJSON = "json"
)

// rotation defaults (see gopkg.in/natefinch/lumberjack.v2).
const (
	maxLogSizeMB  = 50
	maxLogBackups = 5
	maxLogAgeDays = 30
	compressLogs  = true
	logsDirName   = "logs"
	logFileName   = "node.log"
)

var (
	mu                    sync.Mutex
	levelVar              = new(slog.LevelVar) // atomic; safe for concurrent Set/Level
	enabled               atomic.Bool
	logFile               string
	logFormat             = formatText
	currentRotatingWriter *lumberjack.Logger // closed & replaced on re-Init
	currentCrashFile      *os.File           // SetCrashOutput target; closed & replaced on re-Init
)

// gatedWriter forwards writes to inner only while logging is enabled. When
// disabled, writes are silently dropped (reported as fully written so callers
// don't treat the no-op as a short write / error). It backs both slog and the
// redirected stdlib log package, so a single SetEnabled toggle controls all
// daemon logging.
type gatedWriter struct {
	inner io.Writer
}

func (w gatedWriter) Write(p []byte) (int, error) {
	if !enabled.Load() {
		return len(p), nil
	}
	return w.inner.Write(p)
}

// Init configures the process-wide logger and must be called once during
// daemon (node) startup, before any logging occurs. dataDirPath is the opctl
// data dir; logs are written under <dataDirPath>/logs/node.log unless
// OPCTL_LOG_FILE overrides it.
//
// Init is idempotent and safe to call again (e.g. in tests); the most recent
// call wins.
func Init(dataDirPath string) error {
	mu.Lock()
	defer mu.Unlock()

	logFile = LogFilePath(dataDirPath)

	// create the logs dir as the invoking (non-root) user, matching how the
	// data dir itself is created; the daemon runs as root.
	if err := unsudo.CreateDir(filepath.Dir(logFile)); err != nil {
		return err
	}

	if level, ok := parseLevel(os.Getenv(EnvLevel)); ok {
		levelVar.Set(level)
	} else {
		levelVar.Set(slog.LevelInfo)
	}

	enabled.Store(parseEnabled(os.Getenv(EnvEnabled), true))

	logFormat = formatText
	if strings.EqualFold(strings.TrimSpace(os.Getenv(EnvFormat)), formatJSON) {
		logFormat = formatJSON
	}

	// close any previous rotating writer so re-Init (e.g. in tests) doesn't leak
	// the open file handle / mill goroutine.
	if currentRotatingWriter != nil {
		currentRotatingWriter.Close()
	}
	rotatingWriter := &lumberjack.Logger{
		Filename:   logFile,
		MaxSize:    maxLogSizeMB,
		MaxBackups: maxLogBackups,
		MaxAge:     maxLogAgeDays,
		Compress:   compressLogs,
	}
	currentRotatingWriter = rotatingWriter

	// Capture Go runtime panics / fatal errors into the durable log too. These
	// bypass slog (the runtime writes them straight to fd 2), so for a
	// backgrounded daemon they'd otherwise vanish — exactly the "daemon
	// vanished mid-op, no trace" case. SetCrashOutput tees the crash report to
	// this file in addition to stderr. Not gated by `enabled`: a crash is
	// always worth a post-mortem. Best-effort; ignore setup failures.
	if crashFile, err := os.OpenFile(logFile, os.O_APPEND|os.O_CREATE|os.O_WRONLY, 0o600); err == nil {
		if err := debug.SetCrashOutput(crashFile, debug.CrashOptions{}); err == nil {
			if currentCrashFile != nil {
				currentCrashFile.Close()
			}
			currentCrashFile = crashFile
		} else {
			crashFile.Close()
		}
	}

	// File first, stderr second. The daemon's stderr is often a pipe to the
	// short-lived `opctl run` that spawned it; once that exits the pipe breaks
	// and writes to it error. io.MultiWriter stops at the first write error, so
	// if stderr came first a broken pipe would also silence the durable file
	// log (the "log cut off mid-shutdown" symptom). Writing the file first
	// keeps post-mortem logging intact regardless of stderr's state.
	out := gatedWriter{inner: io.MultiWriter(rotatingWriter, os.Stderr)}

	handlerOpts := &slog.HandlerOptions{Level: levelVar}
	var handler slog.Handler
	if logFormat == formatJSON {
		handler = slog.NewJSONHandler(out, handlerOpts)
	} else {
		handler = slog.NewTextHandler(out, handlerOpts)
	}
	slog.SetDefault(slog.New(handler))

	// redirect the stdlib log package (used by existing instrumentation) to the
	// same gated, rotating destination so those lines persist too.
	log.SetOutput(out)

	return nil
}

// LogFilePath returns the path the daemon logs to for the given data dir:
// OPCTL_LOG_FILE if set, otherwise <dataDir>/logs/node.log. It is the single
// source of truth shared by Init (the daemon side) and the local node provider,
// which points the spawned daemon's stdout at this same file.
func LogFilePath(dataDir string) string {
	if f := strings.TrimSpace(os.Getenv(EnvFile)); f != "" {
		return f
	}
	abs, err := filepath.Abs(dataDir)
	if err != nil {
		abs = dataDir
	}
	return filepath.Join(abs, logsDirName, logFileName)
}

// SetLevel changes the active minimum log level at runtime. level is one of
// model.LogLevels (case-insensitive; "warning" accepted for "warn").
func SetLevel(level string) error {
	parsed, ok := parseLevel(level)
	if !ok {
		return &InvalidLevelError{Level: level}
	}
	levelVar.Set(parsed)
	return nil
}

// SetEnabled turns daemon logging on or off at runtime.
func SetEnabled(on bool) {
	enabled.Store(on)
}

// State returns the daemon's current logging configuration.
func State() model.LogState {
	mu.Lock()
	defer mu.Unlock()
	return model.LogState{
		Enabled:  enabled.Load(),
		Level:    levelString(levelVar.Level()),
		Filepath: logFile,
		Format:   logFormat,
	}
}

// InvalidLevelError is returned by SetLevel for an unrecognized level.
type InvalidLevelError struct {
	Level string
}

func (e *InvalidLevelError) Error() string {
	return fmt.Sprintf(
		"invalid log level %q; valid levels: %s",
		e.Level,
		strings.Join(model.LogLevels, ", "),
	)
}

func parseLevel(s string) (slog.Level, bool) {
	normalized, ok := model.NormalizeLogLevel(s)
	if !ok {
		return slog.LevelInfo, false
	}
	switch normalized {
	case model.LogLevelDebug:
		return slog.LevelDebug, true
	case model.LogLevelInfo:
		return slog.LevelInfo, true
	case model.LogLevelWarn:
		return slog.LevelWarn, true
	case model.LogLevelError:
		return slog.LevelError, true
	default:
		return slog.LevelInfo, false
	}
}

func levelString(l slog.Level) string {
	switch {
	case l < slog.LevelInfo:
		return model.LogLevelDebug
	case l < slog.LevelWarn:
		return model.LogLevelInfo
	case l < slog.LevelError:
		return model.LogLevelWarn
	default:
		return model.LogLevelError
	}
}

func parseEnabled(s string, def bool) bool {
	switch strings.ToLower(strings.TrimSpace(s)) {
	case "1", "true", "yes", "on":
		return true
	case "0", "false", "no", "off":
		return false
	default:
		return def
	}
}
