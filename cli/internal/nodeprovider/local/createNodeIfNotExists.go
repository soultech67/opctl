//go:build darwin || dragonfly || freebsd || linux || nacl || netbsd || openbsd || solaris
// +build darwin dragonfly freebsd linux nacl netbsd openbsd solaris

package local

import (
	"bytes"
	"context"
	"errors"
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"syscall"

	"github.com/opctl/opctl/cli/internal/euid0"
	"github.com/opctl/opctl/sdks/go/node"
	"github.com/opctl/opctl/sdks/go/node/logging"
)

// daemonEnvPassThroughVars lists OPCTL_* env vars whose values must reach the
// spawned daemon process to take effect. The daemon otherwise gets a near-empty
// env (see CreateNodeIfNotExists below); anything in this list is forwarded
// only if set in the calling shell.
//
// Important: the daemon is long-lived (one process, reused across `opctl run`
// invocations). Changing one of these in your shell takes effect only on the
// NEXT spawn — i.e. after `opctl node kill`.
var daemonEnvPassThroughVars = []string{
	// Verbose per-Docker-call timing logs. See sdks/go/node/containerruntime/docker/instrumentation.go.
	"OPCTL_DEBUG_DOCKER",
	// Multiplier (default 1.0) applied to every Docker API timeout. Useful on
	// slow CI / underpowered machines. See timeouts.go.
	"OPCTL_DOCKER_TIMEOUT_MULTIPLIER",
	// Daemon logging controls. See sdks/go/node/logging and docs/environment-variables.md.
	// Note: these set startup defaults; level/enablement can also be changed at
	// runtime (no restart) via `opctl doctor log-level` / `opctl doctor logs`.
	"OPCTL_LOG",        // master on/off (default on)
	"OPCTL_LOG_LEVEL",  // debug|info|warn|error (default info)
	"OPCTL_LOG_FORMAT", // text|json (default text)
	"OPCTL_LOG_FILE",   // override log file path (default <data-dir>/logs/node.log)
	// Enables the localhost-only /debug/pprof endpoints on the API server.
	// See sdks/go/node/api/listen.go.
	"OPCTL_DEBUG_PPROF",
	// Node-level defaults for per-container stdout/stderr log persistence +
	// rotation. Overridden per-container by the opfile `container.log` block.
	// See sdks/go/node/containerlog and docs/environment-variables.md.
	"OPCTL_CONTAINER_LOG",              // master on/off (default on)
	"OPCTL_CONTAINER_LOG_MAX_SIZE_MB",  // rotate-at size in MB (default 50)
	"OPCTL_CONTAINER_LOG_MAX_BACKUPS",  // backups kept per stream (default 5)
	"OPCTL_CONTAINER_LOG_MAX_AGE_DAYS", // backup retention in days (default 30)
	"OPCTL_CONTAINER_LOG_COMPRESS",     // gzip rotated backups (default on)
}

func (np nodeProvider) CreateNodeIfNotExists(
	ctx context.Context,
) (node.Node, error) {
	apiClientNode, err := newAPIClientNode(np.config.APIListenAddress)
	if err != nil {
		return nil, err
	}

	// check if node API reachable
	if err := apiClientNode.Liveness(ctx); err == nil {
		return apiClientNode, nil
	}

	// node API unreachable, need to daemonize node...
	if err := euid0.Ensure(); err != nil {
		return nil, err
	}

	pathToOpctlBin, err := os.Executable()
	if err != nil {
		return nil, err
	}

	pathToOpctlBin, err = filepath.EvalSymlinks(pathToOpctlBin)
	if err != nil {
		return nil, err
	}

	cmd := exec.Command(
		pathToOpctlBin,
		"--api-listen-address",
		np.config.APIListenAddress,
		"--container-runtime",
		np.config.ContainerRuntime,
		"--data-dir",
		np.config.DataDir,
		"--dns-listen-address",
		np.config.DNSListenAddress,
		"node",
		"create",
	)

	// Point the daemon's stdout at its durable log file rather than the
	// spawning terminal. Diagnostics that bypass the logger and fmt.Print
	// straight to stdout (e.g. panic recoveries in the op/runtime code) are then
	// captured for post-mortem instead of vanishing with the terminal. A regular
	// file (unlike the terminal or a pipe) never breaks, so this also keeps
	// stdout off the list of things that can SIGPIPE the daemon. Best-effort: if
	// the log file can't be opened yet (e.g. logs dir not created on first run),
	// fall back to /dev/null (nil) — the daemon's own logger still writes it.
	if logF, openErr := os.OpenFile(
		logging.LogFilePath(np.config.DataDir),
		os.O_APPEND|os.O_CREATE|os.O_WRONLY,
		0o600,
	); openErr == nil {
		cmd.Stdout = logF
	} else {
		cmd.Stdout = nil
	}

	var stderr bytes.Buffer
	cmd.Stderr = &stderr

	// don't inherit env wholesale; some things like jenkins track and kill
	// processes via injecting env vars. Forward only the minimum the daemon
	// genuinely needs, plus an opt-in passlist of OPCTL_* tuning vars that
	// influence runtime behavior (see daemonEnvPassThroughVars).
	cmd.Env = []string{
		fmt.Sprintf("HOME=%s", os.Getenv("HOME")),
		fmt.Sprintf("PATH=%s", os.Getenv("PATH")),
		// set by sudo; passthru so we maintain provenance for use by "unsudo"
		fmt.Sprintf("SUDO_GID=%s", os.Getenv("SUDO_GID")),
		fmt.Sprintf("SUDO_UID=%s", os.Getenv("SUDO_UID")),
	}
	for _, name := range daemonEnvPassThroughVars {
		if value, ok := os.LookupEnv(name); ok {
			cmd.Env = append(cmd.Env, fmt.Sprintf("%s=%s", name, value))
		}
	}

	cmd.SysProcAttr = &syscall.SysProcAttr{
		// Run the daemon in a new session: its own session and process group,
		// with no controlling terminal. Without this the daemon stayed in the
		// spawning shell's session and was taken down when that terminal closed
		// (SIGHUP) or on Ctrl-C (SIGINT to the terminal's foreground group),
		// killing every running op with it. The daemon is signalled by PID
		// (pidfile.TryGetProcess), so dropping the explicit process group is safe.
		Setsid: true,
	}

	ctx, cancel := context.WithCancel(ctx)

	var daemonErr error
	go func() {
		daemonErr = cmd.Run()
		defer cancel()
	}()

	var timeoutErr error
	go func() {
		timeoutErr = apiClientNode.Liveness(ctx)
		defer cancel()
	}()

	<-ctx.Done()

	// stop buffering Stderr
	cmd.Stderr = os.Stderr

	// handle error daemonizing
	if _, ok := daemonErr.(*exec.ExitError); ok {
		// handle race
		if strings.Contains(stderr.String(), "node already running") {
			return apiClientNode, nil
		}

		return nil, errors.New(stderr.String())
	}

	// handle timeout reaching node API
	if timeoutErr != nil {
		return nil, fmt.Errorf(
			"timeout reaching node API at %s; try re-running command",
			np.config.APIListenAddress,
		)
	}

	return apiClientNode, nil
}
