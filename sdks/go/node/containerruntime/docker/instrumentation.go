package docker

import (
	"fmt"
	"log"
	"os"
	"strings"
	"time"

	dockerClientPkg "github.com/docker/docker/client"
	"github.com/docker/docker/errdefs"
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
		if reason := classifyExpectedDockerNoop(op, err); reason != "" {
			// Expected-by-design no-op (e.g. ContainerStop on a not-found
			// container during the if-exists path). Demote from "failed" to
			// "noop" so the daemon log isn't drowned in expected-failure
			// noise and real errors stand out when they happen.
			dockerInstrDebugf("%s noop in %s (%s): %s", op, elapsed, detail, reason)
		} else {
			dockerInstrDebugf("%s failed in %s (%s): %v", op, elapsed, detail, err)
		}
	}

	return err
}

// classifyExpectedDockerNoop returns a short human description if err matches
// a known expected-no-op pattern for op, else "". Used by
// instrumentedDockerCall to demote benign expected errors from "failed" to
// "noop" in the daemon log. The opctl codebase already handles each of these
// cases silently elsewhere (isContainerDeleteAlreadyDone, ensureNetworkExists'
// "exists" string-match); this just makes the log line match the semantic.
//
//   - ContainerStop / ContainerRemove + "no such container": expected on the
//     if-exists path when DeleteContainerIfExists is called with an op-level
//     callID that has no container.
//   - ContainerRemove + "removal already in progress": expected race when the
//     kill path and the RunContainer cleanup defer both reach the same
//     container.
//   - NetworkCreate + "already exists": expected because ensureNetworkExists
//     always tries to create and tolerates the race.
//
// Other patterns can be added here as we encounter them in real traces.
func classifyExpectedDockerNoop(op string, err error) string {
	if err == nil {
		return ""
	}
	switch op {
	case "ContainerStop", "ContainerRemove":
		if dockerClientPkg.IsErrNotFound(err) {
			return "not found"
		}
	}
	if op == "ContainerRemove" && errdefs.IsConflict(err) &&
		strings.Contains(err.Error(), "removal of container") &&
		strings.Contains(err.Error(), "already in progress") {
		return "already in progress"
	}
	if op == "NetworkCreate" && strings.Contains(err.Error(), "exists") {
		return "already exists"
	}

	return ""
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
