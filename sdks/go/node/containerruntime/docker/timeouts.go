package docker

import (
	"context"
	"os"
	"strconv"
	"time"
)

// Default per-call timeouts for the Docker API. Picked so a healthy Docker
// Desktop comfortably finishes inside the window (with 2-3x headroom for
// observed spikes under load) but a wedged Docker fails fast enough that opctl
// surfaces a real error instead of staring at a spinner for minutes.
const (
	// defaultDockerPingTimeout bounds the Ping health check. Ping is a no-op
	// on Docker's side; if it can't answer in this window the daemon is broken.
	defaultDockerPingTimeout = 5 * time.Second

	// defaultDockerInspectTimeout bounds pure metadata reads:
	// ContainerInspect, ImageInspectWithRaw, ContainerList, NetworkInspect.
	defaultDockerInspectTimeout = 10 * time.Second

	// defaultDockerMutationTimeout bounds state-changing calls:
	// ContainerCreate, ContainerStart, ContainerStop, ContainerRemove,
	// NetworkCreate, NetworkRemove. ContainerCreate with many bind mounts on
	// Docker Desktop has been observed spiking to 5-8s; 20s gives ~3x headroom.
	defaultDockerMutationTimeout = 20 * time.Second

	// defaultDockerCleanupTimeout bounds the deferred cleanup that runs after
	// a RunContainer call returns (success, failure, or cancellation). Higher
	// than the mutation timeout because cleanup must complete even when the
	// parent call was cancelled — but bounded so a wedged Docker can't leave
	// the daemon goroutine blocked forever, which is what causes CallEnded
	// events never to fire and the CLI to hang on Ctrl+C.
	defaultDockerCleanupTimeout = 30 * time.Second
)

// dockerTimeoutMultiplierEnvVar lets users scale the per-call timeouts for
// genuinely slow environments (underpowered CI, network-mounted Docker, etc.)
// without rebuilding. e.g. OPCTL_DOCKER_TIMEOUT_MULTIPLIER=2.5 multiplies every
// timeout by 2.5. Values <=0 and unparseable values fall back to 1.0.
const dockerTimeoutMultiplierEnvVar = "OPCTL_DOCKER_TIMEOUT_MULTIPLIER"

func dockerTimeoutMultiplier() float64 {
	raw := os.Getenv(dockerTimeoutMultiplierEnvVar)
	if raw == "" {
		return 1.0
	}
	m, err := strconv.ParseFloat(raw, 64)
	if err != nil || m <= 0 {
		return 1.0
	}

	return m
}

func dockerPingTimeout() time.Duration {
	return time.Duration(float64(defaultDockerPingTimeout) * dockerTimeoutMultiplier())
}
func dockerInspectTimeout() time.Duration {
	return time.Duration(float64(defaultDockerInspectTimeout) * dockerTimeoutMultiplier())
}
func dockerMutationTimeout() time.Duration {
	return time.Duration(float64(defaultDockerMutationTimeout) * dockerTimeoutMultiplier())
}
func dockerCleanupTimeout() time.Duration {
	return time.Duration(float64(defaultDockerCleanupTimeout) * dockerTimeoutMultiplier())
}

// withDockerTimeout returns a child context bounded by d (scaled by the
// timeout multiplier). The caller MUST call the returned cancel func.
func withDockerTimeout(ctx context.Context, d time.Duration) (context.Context, context.CancelFunc) {
	return context.WithTimeout(ctx, d)
}
