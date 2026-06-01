# Environment Variables

This document lists the environment variables opctl itself reads, what each one
controls, and how they reach the long-lived node (daemon) process.

There are two distinct groups:

1. **[Flag-backed variables](#flag-backed-variables)** — every CLI flag can also
   be set via an env var whose name is derived from the command path and flag
   name. These are read by the **CLI**.
2. **[Daemon tuning & diagnostics](#daemon-tuning--diagnostics)** — `OPCTL_*`
   knobs read by the **node (daemon)** process to control logging, profiling,
   and the Docker runtime.

> **Generic/system variables.** opctl also consumes a few standard variables
> (`HOME`, `PATH`, and `SUDO_UID`/`SUDO_GID` set by `sudo`). These are not opctl
> configuration; they're forwarded to the daemon so it can locate the user's
> home/PATH and de-escalate privileges for child processes. They are not
> documented as knobs here.

---

## Flag-backed variables

opctl derives an environment variable name for **every** command flag
(`cli/cmd/root.go`, `populateFlagsFromEnvVars`). The rule is:

```
OPCTL_<COMMAND_PATH>_<FLAG_NAME>
```

where the command path and flag name are upper-cased and `-`/space become `_`.
The `(env $NAME)` annotation is shown next to each flag in `--help`.

Precedence: an explicitly-provided flag wins over the env var, which wins over
the flag's default. (The env var is applied only when the flag was not set on
the command line.)

### Root (persistent) flags

These are defined on the root command, so their env var prefix is just `OPCTL_`:

| Variable                    | Flag                     | Controls                                                              | Default            |
| --------------------------- | ------------------------ | --------------------------------------------------------------------- | ------------------ |
| `OPCTL_DATA_DIR`            | `--data-dir`             | Directory where opctl stores all node data (events DB, op cache, logs, pidfile). | platform app-data dir¹ |
| `OPCTL_API_LISTEN_ADDRESS`  | `--api-listen-address`   | `IP:PORT` the node API server listens on (also where the CLI reaches the node). | `127.0.0.1:42224`  |
| `OPCTL_CONTAINER_RUNTIME`   | `--container-runtime`    | Container runtime: `docker` (deprecated), `k8s`, or `embedded`.       | `docker`           |
| `OPCTL_DNS_LISTEN_ADDRESS`  | `--dns-listen-address`   | `IP:PORT` the node DNS server listens on.                             | `127.0.0.1:53`     |
| `OPCTL_NO_COLOR`            | `--no-color`             | Disable colored CLI output.                                           | `false`            |

¹ The default data dir is resolved per-platform via `appdataspec` — typically
`~/Library/Application Support/opctl` (macOS), `~/.local/share/opctl` (Linux),
`%APPDATA%\opctl` (Windows).

### Subcommand flags

The same rule applies to subcommand-local flags, prefixed by the full command
path. For example, a flag `--foo` on `opctl run` is settable via
`OPCTL_RUN_FOO`. Run `opctl <command> --help` to see the exact `(env $NAME)`
for any flag.

One commonly-useful one:

| Variable           | Flag (`opctl ui --no-open`) | Controls                                                                                                          | Default |
| ------------------ | --------------------------- | ---------------------------------------------------------------------------------------------------------------- | ------- |
| `OPCTL_UI_NO_OPEN` | `--no-open`                 | Print the web UI URL instead of opening it in your default browser. Set this to stop `opctl ui` from spawning browser tabs (e.g. on a headless box, or to keep test runs from hijacking your browser). | `false` |

---

## Daemon tuning & diagnostics

These `OPCTL_*` variables are read by the **node process**, not the CLI. Because
the node is spawned with a deliberately minimal environment (so external process
supervisors can't inject behavior), only an explicit pass-through list reaches
it — see [How variables reach the daemon](#how-variables-reach-the-daemon).

### Logging

The node writes a durable, rotating log file in addition to echoing to stderr,
so issues can be diagnosed after the fact. See `sdks/go/node/logging`.

| Variable           | Controls                                                              | Values / format                      | Default                       |
| ------------------ | -------------------------------------------------------------------- | ------------------------------------ | ----------------------------- |
| `OPCTL_LOG`        | Master on/off switch for daemon logging (file + stderr).             | `1/true/yes/on`, `0/false/no/off`    | on                            |
| `OPCTL_LOG_LEVEL`  | Minimum log level.                                                    | `debug`, `info`, `warn`, `error`     | `info`                        |
| `OPCTL_LOG_FORMAT` | Log line format.                                                     | `text`, `json`                       | `text`                        |
| `OPCTL_LOG_FILE`   | Override the log file path.                                          | filesystem path                      | `<data-dir>/logs/node.log`    |

**Log location & rotation.** Unless overridden, logs are written to
`<data-dir>/logs/node.log` and rotated by size (≈50 MB), keeping a handful of
compressed backups. Because the node runs as root, the log file is root-owned;
read it with `sudo` (e.g. `sudo tail -f "<data-dir>/logs/node.log"`).

**These set startup defaults only.** Changing them in your shell affects the
node only on its *next* start (i.e. after `opctl node kill`), because the node
is long-lived and reused across `opctl run` invocations. To change logging on a
**running** node without a restart, use the `opctl doctor` commands:

```bash
opctl doctor logs              # show current logging state (incl. file path)
opctl doctor logs on           # enable daemon logging
opctl doctor logs off          # disable daemon logging
opctl doctor log-level debug   # raise verbosity (debug|info|warn|error)
opctl doctor log-level info    # back to default
```

These talk to the running node over its localhost API and require no elevation.

> **Caution:** turning logging **off** (`OPCTL_LOG=off` or `opctl doctor logs
> off`) suppresses *all* daemon logging, including the always-on Docker
> kill-path/cleanup paper trail that is most useful for diagnosing wedged-Docker
> and cancellation races. To reduce noise without losing that trail, prefer
> raising the level (`opctl doctor log-level warn` / `error`) instead of turning
> logging off.

### Container logging

Separately from the node's own log, the node persists each **container's**
stdout/stderr to durable, rotating log files (in addition to the live event
stream), so per-op logs are explorable after shutdown. See
`sdks/go/node/containerlog`. These variables set the **node-level defaults**; an
opfile `container.log` block overrides any of them per container.

| Variable                           | Controls                                                            | Values / format                   | Default |
| ---------------------------------- | ------------------------------------------------------------------ | --------------------------------- | ------- |
| `OPCTL_CONTAINER_LOG`              | Master on/off for container stdout/stderr file persistence.        | `1/true/yes/on`, `0/false/no/off` | on      |
| `OPCTL_CONTAINER_LOG_MAX_SIZE_MB`  | Size (MB) a stream's log file reaches before it rotates.           | integer ≥ 1                       | `50`    |
| `OPCTL_CONTAINER_LOG_MAX_BACKUPS`  | Rotated backups kept per stream (`0` = keep all, subject to age).  | integer ≥ 0                       | `5`     |
| `OPCTL_CONTAINER_LOG_MAX_AGE_DAYS` | Max age (days) of a rotated backup before deletion (`0` = never).  | integer ≥ 0                       | `30`    |
| `OPCTL_CONTAINER_LOG_COMPRESS`     | gzip rotated backups.                                              | `1/true/yes/on`, `0/false/no/off` | on      |

**Log location.** Unless a container sets `log.dir`, files are written to
`<data-dir>/logs/containers/<name>_<opHash>/{stdout,stderr}.log` — a path stable
across runs of the same container, so `tail -F` follows it. The active files are
chowned to the invoking user; rotated backups may remain root-owned. Per-container
overrides (including a custom `dir`) live in the opfile `container.log` block.

### Profiling

| Variable            | Controls                                                                    | Values / format                   | Default |
| ------------------- | --------------------------------------------------------------------------- | --------------------------------- | ------- |
| `OPCTL_DEBUG_PPROF` | Enables Go `net/http/pprof` endpoints under `/debug/pprof/` on the API server (localhost-bound). Useful for diagnosing a wedged or leaking daemon. | `1/true/yes/on` | off |

When enabled, profiles are reachable at e.g.
`http://<api-listen-address>/debug/pprof/` (default
`http://127.0.0.1:42224/debug/pprof/`). The endpoint is only as reachable as the
API listen address, which is localhost by default.

### Docker runtime

| Variable                          | Controls                                                                                       | Values / format       | Default |
| --------------------------------- | ---------------------------------------------------------------------------------------------- | --------------------- | ------- |
| `OPCTL_DEBUG_DOCKER`              | Enables verbose per-Docker-call timing/debug logs (prefixed `[opctl docker]`). Kill-path & cleanup events log regardless. See `sdks/go/node/containerruntime/docker/instrumentation.go`. | `1/true/yes/on` | off |
| `OPCTL_DOCKER_TIMEOUT_MULTIPLIER` | Multiplier applied to every per-call Docker API timeout (ping/inspect/mutation/cleanup). Useful on slow CI / underpowered machines / network-mounted Docker. See `timeouts.go`. | float > 0 (e.g. `2.5`) | `1.0` |

> With daemon logging enabled, `[opctl docker]` instrumentation is captured in
> the node log file too.

---

## How variables reach the daemon

When the CLI needs a node and none is running, it spawns one (`opctl node
create`) with a **minimal** environment rather than the full shell environment.
Only these are forwarded
(`cli/internal/nodeprovider/local/createNodeIfNotExists.go`):

- System: `HOME`, `PATH`, `SUDO_UID`, `SUDO_GID`
- opctl tuning/diagnostics (forwarded only if set):
  `OPCTL_DEBUG_DOCKER`, `OPCTL_DOCKER_TIMEOUT_MULTIPLIER`,
  `OPCTL_LOG`, `OPCTL_LOG_LEVEL`, `OPCTL_LOG_FORMAT`, `OPCTL_LOG_FILE`,
  `OPCTL_CONTAINER_LOG`, `OPCTL_CONTAINER_LOG_MAX_SIZE_MB`,
  `OPCTL_CONTAINER_LOG_MAX_BACKUPS`, `OPCTL_CONTAINER_LOG_MAX_AGE_DAYS`,
  `OPCTL_CONTAINER_LOG_COMPRESS`, `OPCTL_DEBUG_PPROF`

Consequences:

- A daemon tuning/diagnostics variable takes effect on the **next** node spawn.
  If a node is already running, `opctl node kill` first, then set the variable
  and re-run.
- For logging specifically, prefer the runtime `opctl doctor` commands above —
  they change a running node immediately, no restart required.
