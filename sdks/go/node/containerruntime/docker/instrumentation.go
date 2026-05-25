package docker

import (
	"fmt"
	"log"
	"os"
	"strings"
	"time"
)

// Instrumentation for the Docker container runtime.
//
// Two levels:
//   - kill-path / cleanup events log unconditionally (low volume; fires only
//     when an op is cancelled or a cleanup is racing a wedged Docker — exactly
//     the situations we want a paper trail for).
//   - per-call Docker API timings log only when OPCTL_DEBUG_DOCKER is set to a
//     truthy value (1/true/yes/on). These would otherwise be too noisy.
//
// Output goes to the daemon process's stderr via the stdlib log package, with
// a "[opctl docker]" prefix so it's easy to grep.

const opctlDebugDockerEnvVar = "OPCTL_DEBUG_DOCKER"

func dockerDebugEnabled() bool {
	switch strings.ToLower(strings.TrimSpace(os.Getenv(opctlDebugDockerEnvVar))) {
	case "1", "true", "yes", "on":
		return true
	}

	return false
}

// dockerInstrInfof logs a kill-path or cleanup event. Always on.
func dockerInstrInfof(format string, args ...any) {
	log.Output(2, fmt.Sprintf("[opctl docker] "+format, args...))
}

// dockerInstrDebugf logs a per-call Docker API timing or transient detail.
// No-op unless OPCTL_DEBUG_DOCKER is set.
func dockerInstrDebugf(format string, args ...any) {
	if !dockerDebugEnabled() {
		return
	}
	log.Output(2, fmt.Sprintf("[opctl docker debug] "+format, args...))
}

// instrumentedDockerCall times a Docker API call and reports the result.
// The op label should be the Docker API method name (e.g. "ContainerCreate").
// detail is appended to identify the target (container name, image ref, etc.).
//
// Errors timing out, returning, or being cancelled are all logged at the
// appropriate level: timeouts and cancellations are notable (always-on),
// successes and ordinary errors are debug-level.
func instrumentedDockerCall(op, detail string, fn func() error) error {
	startedAt := time.Now()
	err := fn()
	elapsed := time.Since(startedAt)

	switch {
	case err == nil:
		dockerInstrDebugf("%s ok in %s (%s)", op, elapsed, detail)
	case isDeadlineExceeded(err):
		// timeouts are noteworthy regardless of debug setting
		dockerInstrInfof("%s timed out after %s (%s): %v", op, elapsed, detail, err)
	case isContextCanceled(err):
		dockerInstrInfof("%s cancelled after %s (%s): %v", op, elapsed, detail, err)
	default:
		dockerInstrDebugf("%s failed in %s (%s): %v", op, elapsed, detail, err)
	}

	return err
}

func isDeadlineExceeded(err error) bool {
	if err == nil {
		return false
	}
	// avoid pulling in errors.Is here to keep this helper dependency-free;
	// match on the canonical wrapped string both context.DeadlineExceeded
	// and Docker client wrappers produce.
	msg := err.Error()
	return strings.Contains(msg, "context deadline exceeded") ||
		strings.Contains(msg, "deadline exceeded")
}

func isContextCanceled(err error) bool {
	if err == nil {
		return false
	}
	msg := err.Error()
	return strings.Contains(msg, "context canceled") ||
		strings.Contains(msg, "context cancelled")
}
