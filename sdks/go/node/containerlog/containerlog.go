// Package containerlog resolves a container call's effective log-persistence
// config by layering, highest precedence first: the opfile `container.log`
// overrides, the node-level OPCTL_CONTAINER_LOG* env defaults, and hardcoded
// defaults. It also owns the on-disk file-path layout. It lives in the node
// layer (it reads process env + the data dir), keeping the opspec interpreter
// free of env/data-dir concerns.
package containerlog

import (
	"fmt"
	"hash/fnv"
	"os"
	"path/filepath"
	"strconv"
	"strings"

	"github.com/opctl/opctl/sdks/go/model"
)

// Hardcoded defaults — mirror the daemon's own node.log rotation knobs
// (sdks/go/node/logging). Logging is on by default so logs persist without opt-in.
const (
	DefaultEnabled    = true
	DefaultMaxSizeMB  = 50
	DefaultMaxBackups = 5
	DefaultMaxAgeDays = 30
	DefaultCompress   = true
)

// Node-level default env vars (forwarded to the daemon by the local node
// provider). A per-container `log.*` value overrides these; these override the
// hardcoded defaults above.
const (
	EnvEnabled    = "OPCTL_CONTAINER_LOG"
	EnvMaxSizeMB  = "OPCTL_CONTAINER_LOG_MAX_SIZE_MB"
	EnvMaxBackups = "OPCTL_CONTAINER_LOG_MAX_BACKUPS"
	EnvMaxAgeDays = "OPCTL_CONTAINER_LOG_MAX_AGE_DAYS"
	EnvCompress   = "OPCTL_CONTAINER_LOG_COMPRESS"
)

// Config is the fully-resolved per-container log config consumed by the caller.
type Config struct {
	Enabled    bool
	StdOutPath string
	StdErrPath string
	MaxSizeMB  int
	MaxBackups int
	MaxAgeDays int
	Compress   bool
}

// Resolve produces the effective config for a container call. log may be nil
// (no `log` block) — logging is still on by default. The on-disk location is
// stable across runs of the same container (so `tail -F` follows it and the
// caller can cache one rotating writer per path): the default is
// <dataDirPath>/logs/containers/<name>_<opHash>/{stdout,stderr}.log; a custom
// log.Dir uses <dir>/<name>.{stdout,stderr}.log.
func Resolve(
	log *model.ContainerLog,
	dataDirPath string,
	opPath string,
	name *string,
) Config {
	enabled := DefaultEnabled
	switch {
	case log != nil && log.Enabled != nil:
		enabled = *log.Enabled
	default:
		if v, ok := parseBool(os.Getenv(EnvEnabled)); ok {
			enabled = v
		}
	}
	if !enabled {
		return Config{Enabled: false}
	}

	logName := resolveName(name)

	var logDir, stdOutName, stdErrName string
	switch {
	case log != nil && log.Dir != "":
		// stable, name-prefixed filenames so `tail -F` follows across runs and
		// multiple containers can share one dir without colliding.
		logDir = log.Dir
		stdOutName, stdErrName = logName+".stdout.log", logName+".stderr.log"
	case dataDirPath != "":
		logDir = DefaultDir(dataDirPath, opPath, name)
		stdOutName, stdErrName = "stdout.log", "stderr.log"
	default:
		// no explicit dir and no data dir => nowhere sensible to persist.
		return Config{Enabled: false}
	}

	var (
		pSize, pBackups, pAge *int
		pCompress             *bool
	)
	if log != nil {
		pSize, pBackups, pAge, pCompress = log.MaxSizeMB, log.MaxBackups, log.MaxAgeDays, log.Compress
	}

	return Config{
		Enabled:    true,
		StdOutPath: filepath.Join(logDir, stdOutName),
		StdErrPath: filepath.Join(logDir, stdErrName),
		MaxSizeMB:  resolveInt(pSize, EnvMaxSizeMB, DefaultMaxSizeMB),
		MaxBackups: resolveInt(pBackups, EnvMaxBackups, DefaultMaxBackups),
		MaxAgeDays: resolveInt(pAge, EnvMaxAgeDays, DefaultMaxAgeDays),
		Compress:   resolveBool(pCompress, EnvCompress, DefaultCompress),
	}
}

// DefaultDir is the default per-container log directory under the data dir. It
// is keyed by the (sanitized) container name plus a short hash of the op path,
// so it is stable across runs of the same container and disambiguated across
// ops. Exported so callers/tests can compute the same path.
func DefaultDir(dataDirPath, opPath string, name *string) string {
	return filepath.Join(dataDirPath, "logs", "containers", resolveName(name)+"_"+hash8(opPath))
}

func resolveName(name *string) string {
	if name != nil && *name != "" {
		return sanitize(*name)
	}
	return "container"
}

func resolveBool(specVal *bool, envKey string, def bool) bool {
	if specVal != nil {
		return *specVal
	}
	if v, ok := parseBool(os.Getenv(envKey)); ok {
		return v
	}
	return def
}

func resolveInt(specVal *int, envKey string, def int) int {
	v := def
	switch {
	case specVal != nil:
		v = *specVal
	default:
		if envVal, err := strconv.Atoi(strings.TrimSpace(os.Getenv(envKey))); err == nil {
			v = envVal
		}
	}
	if v < 0 {
		// negative rotation knobs are nonsensical; fall back to the default.
		return def
	}
	return v
}

func parseBool(s string) (bool, bool) {
	switch strings.ToLower(strings.TrimSpace(s)) {
	case "1", "true", "yes", "on":
		return true, true
	case "0", "false", "no", "off":
		return false, true
	default:
		return false, false
	}
}

// hash8 is a short, stable hex digest used to disambiguate same-named containers
// across ops in the default log path.
func hash8(s string) string {
	h := fnv.New32a()
	h.Write([]byte(s))
	return fmt.Sprintf("%08x", h.Sum32())
}

// sanitize maps a container name to a filesystem-safe token.
func sanitize(s string) string {
	var b strings.Builder
	for _, r := range s {
		switch {
		case r >= 'a' && r <= 'z',
			r >= 'A' && r <= 'Z',
			r >= '0' && r <= '9',
			r == '-', r == '_', r == '.':
			b.WriteRune(r)
		default:
			b.WriteRune('_')
		}
	}
	return b.String()
}
