# Changelog

All notable changes to this project will be documented in this file in
accordance with
[![keepachangelog 1.0.0](https://img.shields.io/badge/keepachangelog-1.0.0-brightgreen.svg)](http://keepachangelog.com/en/1.0.0/)

## [0.1.81] - 2026-07-14

### Added

- Container calls support a new `volumes` property mapping an absolute container path to the name of a container-runtime-managed named volume
  (`--mount type=volume` semantics in the Docker runtime). Unlike `dirs` bindings, named volumes live inside the container runtime rather than
  on a host-shared path, so high-write-rate workloads (e.g. database data directories) don't stream filesystem-change events across Docker
  Desktop's file-sharing layer, and the data persists across container runs — opctl's container cleanup uses `docker rm -v` semantics, which
  removes anonymous volumes only, never named ones. Values are string expressions (literals, `$(ref)`, or interpolation) validated against
  Docker's volume-name rules at interpret time; missing volumes are created on first use. The k8s container runtime ignores `volumes` (as it
  already does `sockets` and `ports`)

## [0.1.80] - 2026-06-25

### Fixed

- `make install` now actually replaces `opctl` with the binary it just built. A shell-variable leak left `opctl -v` reporting the *old* version after a
  successful build+install: the install helpers are POSIX `sh` functions with no scoping, and `can_install_without_sudo` assigned the global `$dest`
  while backing up the existing binary, redirecting the install to the backup path (`opctl-<oldversion>`) and leaving `opctl` itself untouched. The
  helpers in the install/uninstall path now use uniquely-named (function-prefixed) variables — staying POSIX `sh`, no `local` — so they can't clobber
  the caller's install target
- `make install` no longer creates an `opctl-0.0.0` backup. A binary reporting the default dev version (`0.0.0` — `make install` with no `VERSION`) is a
  throwaway build, not a release worth preserving, and it only cluttered the backup set `make uninstall` restores from; that case is now skipped
- `make install` no longer silently destroys the binary it overwrites. `backup_existing_opctl` now keys the backup to the version of the binary being
  replaced (`opctl-<version>`) and skips only if *that exact version* is already backed up. Previously it skipped whenever any `opctl-*` backup existed,
  so e.g. `make install VERSION=1.0.80` over an installed `1.0.79` (with an old `opctl-0.1.77` already present) lost `1.0.79` with no recoverable backup
- `make uninstall` now restores the previous binary through the same temp-file + atomic-rename path as install (`copy_opctl`) instead of an in-place
  `cp`, avoiding the macOS per-inode code-signature SIGKILL ("killed", exit 137) that the new bytes would otherwise hit when verified against the
  replaced binary's cached signature
- `find_highest_opctl_backup` now selects the backup to restore by the version embedded in the filename, treating non-version snapshot names as a last
  resort only. Raw `sort -V` had ranked `opctl-snapshot-*` above real releases, so uninstall could restore an older snapshot over a newer release

### Changed

- `make uninstall` now stops the daemon with `opctl node kill` (which keeps the data dir) instead of the destructive `node delete`; the restore only
  needs the daemon stopped, not the node's data removed
- The CLI no-progress hint now probes `node.Liveness` when the event stream goes quiet, distinguishing a genuinely wedged daemon from an op that is
  simply idle instead of always blaming Docker after a fixed timeout. The healthy-but-idle case prints a calm, non-actionable note (new
  `CliOutput.Info`) and the red warning is reserved for an unresponsive daemon. The misleading `docker info` lead is dropped (it reports healthy during
  these lockups) in favour of the recovery ladder that works: restart Docker, then `opctl node delete`

## [0.1.79] - 2026-06-03

### Fixed

- Resolver-config cleanup is now idempotent. A concurrent per-container cleanup removing an `/etc/resolver/opctl_*` file between the bulk sweep's
  directory scan and its remove no longer fails the whole sweep with `remove …: no such file or directory`; "already gone" is treated as success and
  remaining files are still removed. The daemon's shutdown cleanup also logs this as a non-fatal warning (it never affected the node's exit) rather
  than a scary `ERROR`
- Stored auth lookup (`TryGetAuth`) no longer lets a blank-resources entry act as a wildcard. A stored auth whose resources prefix was empty would
  `HasPrefix`-match every ref and silently supply its credentials to unrelated pulls (e.g. a private `github.com` clone with no github auth
  configured); blank entries are now skipped, and when multiple entries match the most specific (longest) prefix wins regardless of key order
- `opctl auth add` now rejects an empty/whitespace `RESOURCES` argument, so a match-everything blank entry can no longer be stored in the first place
- `opctl auth add` now waits until the credential is durably stored before returning. It previously published the add as an asynchronous event and
  returned immediately, so an `opctl auth ls` (or an auth-dependent pull) run right after could read before the write landed and see nothing
- `make install` no longer produces a binary the kernel SIGKILLs on launch (`opctl -v` → "killed", exit 137). It overwrote the binary in place,
  reusing the inode, so on macOS the new bytes were verified against the *previous* binary's cached code signature. It now installs via a temp file +
  atomic rename (fresh inode), so the new signature is checked cleanly
- The daemon now reconciles leaked `/etc/resolver/opctl_*` files: it sweeps stale ones on startup and removes its own on graceful shutdown. Previously
  only `opctl node kill` cleaned them, so any non-graceful stop (SIGKILL, crash, terminal close) leaked them and they accumulated across restarts —
  over time degrading host DNS for the whole opctl domain set (the "opctl DNS" flakiness)
- The container-subnet host route is now set up idempotently (delete-before-add), so a stale route left by an unclean prior daemon is replaced instead
  of making `route add` fail with "File exists" and continuing to blackhole the container subnet
- Removed dead, broken darwin tun-teardown that ran `ip link delete tun<N>` — a Linux command with a macOS-wrong interface name — and silently
  discarded its error. The kernel reclaims the WireGuard utun when the daemon process exits, so no explicit delete is needed

### Added

- New `opctl container down NAME` command cleanly stops + removes the RUNNING opctl-managed container(s) with that name (a positional name, not a
  `--label` flag) — the everyday "take a service down". A single running match is shut down directly; several running under the same name prompt for an
  interactive selection, or `--force` takes them all down; stopped containers are ignored. It complements `delete` (remove by label, any state) and
  `prune` (remove stopped only), and the `opctl container` help now contrasts the three
- `opctl container ls --filter NAME` shows only opctl-managed containers whose name contains NAME (case-insensitive). The `opctl_` prefix is implied
  (`--filter artifacts-api` matches `opctl_artifacts-api_<id>`), and `_`/`-` are interchangeable; it matches the displayed name or the raw container name
- `opctl container rm` is now a Docker-style alias for `opctl container delete`
- New nightly informational GitHub Actions workflow (`nightly-cli-e2e.yml`) runs the full conformance CLI e2e (`cliE2eFull=true`) on a schedule and
  posts start + result with timing to a Slack webhook (`SLACK_WEBHOOK_URL` secret; no-ops gracefully if unset). It does not gate PRs
- The `Release` job now posts new releases to Slack (`SLACK_WEBHOOK_URL`): the version, release URL (notes + binaries), the triggering author/commit,
  and the changelog notes for that version. It only fires for a genuinely new tag and is best-effort (never fails the release)

### Changed

- `make install` now warns and prompts before stopping the running node. Installing swaps the binary, which requires stopping the daemon (a graceful
  `opctl node kill`), and that takes down every running opctl-managed container and any in-progress ops — a frequent surprise. It now reports the
  running-container count and asks to continue; `FORCE=1` (or a non-interactive shell) skips the prompt
- `opctl auth ls` is now the primary name of the list command (`opctl auth list` remains as an alias)
- The PR `Test` check's CLI e2e now runs only the fast, reliable `test-suite/auth` subset instead of the entire conformance suite (227 tests, each in
  its own nested dind, ~30 min and timeout-flaky). The e2e op takes a `testsDir` input; interpreter conformance stays covered by the Go unit and
  integration suites, and the full suite runs nightly (informational)
- The PR `Test` check skips the CLI e2e on PRs from forks. Forked PRs don't receive `TEST_GITHUB_ACCESS_TOKEN`, so the auth e2e's required token would
  be empty; fork PRs now run the rest of the suite (unit/integration/sdk/opspec/webapp/gofmt) while same-repo PRs still run the e2e
- The CLI e2e now builds the `linux/amd64` opctl binary it mounts from the branch's own source as a gated step before the suite runs. The binary is
  gitignored and nothing else in the test graph built it, so the e2e previously ran against a stale or missing binary instead of the code under test
- Auth resolution now emits debug-level decision logs (which stored resources/username, if any, is used for a pull — never the password) at the node
  resolve and op-call injection points, so unexpected authenticated pulls are diagnosable via `OPCTL_LOG_LEVEL=debug`
- CI now runs the full test suite (including the docker-in-docker CLI e2e) as a dedicated `Test` GitHub check, split out from the `Build` (compile) job so test
  results are visible on their own instead of buried inside the build step
- Fixed the CLI e2e test harness so negative-auth scenarios assert correctly. It ran under `sh -e`, so capturing `opctl run`'s output in a command substitution aborted
  the script before the assertion whenever the run (correctly) failed; the exit code is now captured with errexit disabled around the run, and the run's combined output
  is logged for diagnosis

## [0.1.78] - 2026-05-17

### Added

- Successful CLI commands now print a cached update hint when a newer fork release is available
- Update-hint release checks use the same build-time GitHub owner/repo as `self-update`
- New `opctl auth list` (alias `ls`) command shows stored default auth entries (resources + usernames; passwords are not printed)
- New `opctl auth remove RESOURCES` (alias `rm`) command removes a previously-stored auth entry by its resources prefix
- `opctl auth add` now prints a confirmation line showing the stored resources and username
- Image pull output now states whether the pull is authenticated (and as which username) or anonymous, so silent fallbacks to anonymous pulls (and the rate limits they
  incur) are visible
- New `make clean` target removes the cross-compiled CLI binaries under `cli/` and any orphaned opctl-managed containers in Docker `Created` state
- `make install` now backs up the currently-installed `opctl` to `opctl-<version>` (in the same prefix dir) before overwriting it, but only if no `opctl-*` backup already
  exists — so the *original* pre-fork release is preserved as the restore target across repeated dev installs. Version is read via `opctl -v`; dev builds without ldflags
  fall back to `opctl-snapshot-<timestamp>`
- New `make uninstall` target restores the highest-version `opctl-*` backup (semver-sorted via `sort -V`) over the current binary, after killing the running daemon. Errors
  out if no backup is present, with a pointer to `opctl self-update` for a fresh release
- New `make reset-backup` target removes the `opctl-*` backup(s) in the install prefix so the next `make install` captures the *current* binary as the new restore target.
  Lists what will be removed and prompts before deleting; `FORCE=1` skips the prompt for non-interactive use
- New `make docker-logs` target tees two Docker observability streams to `./docker-logs/`: a filtered tail of Docker Desktop's VM `init.log` (every opctl / apiproxy POST /
  warning / error line) and `docker events --filter label=opctl.managed=true`. Foreground process; Ctrl+C cleanly tears down the streams and reports the captured file
  sizes. macOS Docker Desktop specific. Output dir overridable via `OPCTL_DOCKER_LOG_DIR`
- New `make docker-daemon-logs` target signals dockerd inside the Docker Desktop VM with SIGUSR1, triggering a goroutine stack-trace dump. Retrieves the dump file
  (`/var/run/docker/goroutine-stacks-<ts>.log`) from the VM and saves it under `./docker-logs/dockerd-goroutines-<ts>.log` on the host. Use while a hang is in progress to
  capture which goroutine/function dockerd is stuck on. macOS Docker Desktop specific
- New `make up` target runs the opctl daemon in the foreground with `OPCTL_DEBUG_DOCKER=1` (and forwards any `OPCTL_DOCKER_TIMEOUT_MULTIPLIER` from the shell). Kills any
  background daemon first so the foreground one becomes the active one; the daemon's `[opctl docker]` / `[opctl kill]` instrumentation prints to the terminal in real time.
  Pair with `make docker-logs` (in another terminal) for complete visibility while reproducing a hang
- New `make doctor` target runs read-only diagnostics for the classic Docker-Desktop-wedged pathology: lists opctl-managed containers (any state), counts Created-state
  orphans, times `docker info`, checks Spotlight indexing on the project's volume, reports `node_modules/` sizes (since large trees + gRPC-FUSE is the empirically-observed
  wedge cause), reports Time Machine status and the current directory's TM exclusion state, and compares the installed `opctl` binary's mtime to the latest git commit so
  you can tell at a glance whether you need to reinstall. Each finding prints ✓ / ⚠ / ✗ with a fix hint when relevant. macOS Docker Desktop specific
- New `make docker-restart` target is the nuclear recovery: kills the opctl daemon, `osascript`-quits Docker Desktop, sleeps 5s for the VM to actually tear down,
  relaunches, and polls `docker info` for up to 90s waiting for the daemon to come back. Use when `make doctor` shows `docker info` is unresponsive or when a `make
  docker-daemon-logs` goroutine dump shows dockerd stuck in `syscall.fstatat` (gRPC-FUSE wedge). macOS Docker Desktop specific
- New `opctl container prune` command removes all opctl-managed containers that are not currently running (created, exited, dead, restarting); mirrors `docker container
  prune` and accepts `-f/--force` to skip the confirmation prompt
- `opctl container ls` now includes a `STATUS` column (e.g. `Up 5 minutes`, `Exited (0) 2 hours ago`) so it's obvious which containers are actually running
- `opctl container ls -i/--images` adds the IMAGE column to the table (hidden by default because long image refs were the main cause of wrapped rows)
- `opctl container ls -v/--verbose` prints the `DELETE LABELS` filters as a separate section below the table, one block per container, suitable for copy-paste into `opctl
  container delete --label`
- Up-front Docker `Ping` health check on container-runtime construction and at the top of every `RunContainer`; if Docker is unresponsive the op fails fast (within ~5s)
  with an actionable "try `docker info` or restart Docker Desktop" message instead of blocking inside `ContainerCreate`
- Per-call timeouts on every Docker API call (Ping 5s, Inspect/List 10s, Create/Start/Stop/Remove/NetworkCreate/NetworkRemove 20s). `ContainerWait` and `ImagePull` remain
  untimed by design (long-running on purpose). Scale all timeouts with `OPCTL_DOCKER_TIMEOUT_MULTIPLIER=2.5` for slow CI/underpowered machines
- Deferred per-container cleanup is now bounded (30s default, multiplier-scaled). When cleanup exceeds its budget the daemon publishes a `ContainerStdErrWrittenTo` event
  surfacing `warning: cleanup of container <name> timed out after <duration> — Docker may be unresponsive`, so a wedged Docker no longer silently keeps the CLI spinning
  forever waiting for a `CallEnded` event that will never fire
- `opctl run` now emits a one-shot warning when no events arrive from the daemon for 2 minutes, pointing the user at `docker info` and a Docker Desktop restart as the
  recovery path. (Threshold widened from 30s to 2 min after observing it false-positive against legitimate steady-state services like LocalStack whose internal polling
  cycle leaves ~30s gaps in event output.)
- Kill-path instrumentation: the daemon now logs `[opctl kill]` lines at each cleanup step (KillOp received, callKiller.Kill enter/exit with duration, child-propagation
  count, DeleteContainerIfExists timing) and `[opctl docker]` lines on every Docker call timeout/cancel, so the next time `Ctrl+C` leaves Docker in a bad state we have a
  paper trail to debug from. Per-call success timings are also available when `OPCTL_DEBUG_DOCKER=1` is set
- The daemon spawn now forwards `OPCTL_DEBUG_DOCKER` and `OPCTL_DOCKER_TIMEOUT_MULTIPLIER` from the calling shell so these tuning vars can be set in the environment without
  requiring command-line flags. Note: the daemon is long-lived; `opctl node kill` is required for an env change to take effect
- The daemon now persists each container's stdout/stderr to durable, rotating log files (in addition to the live event stream), so per-op logs are explorable after the op —
  or the daemon — has stopped. On by default; files land at `<data-dir>/logs/containers/<name>_<opHash>/{stdout,stderr}.log` (a path stable across runs, so `tail -F`
  follows it), rotated by size with capped/aged backups. Configure per container via the opfile `container.log` block (`enabled`, `dir`, `maxSizeMB`, `maxBackups`,
  `maxAgeDays`, `compress`) — set `dir` to a host folder (e.g. the host side of your `workDir` bind mount) to land logs in your project — or globally via the
  `OPCTL_CONTAINER_LOG*` env vars (see `docs/environment-variables.md`)
- `opctl events` now accepts `--since` (a duration like `90m`/`24h`, or an RFC3339 timestamp) and `--roots` (comma-separated/repeated root call IDs) to replay just a subset
  of the durable event history — e.g. one op's output after it (or the daemon) has stopped — instead of the full firehose

### Changed

- Release ops now pass the self-update repository through compile so published binaries consistently target fork releases
- Compile and Makefile guidance now describe the repository setting as shared by self-update and update hints
- `make build` / `make bld` now warns at the end when opctl-managed containers in `Created` state grow during the build (or already exist), with a pointer to `make clean`;
  failed builds frequently leak these and they can block subsequent host file operations on macOS Docker Desktop
- `opctl container ls` now shows only running containers by default, mirroring `docker ps` semantics; pass `-a/--all` to include stopped/created/other non-running
  containers (the prior all-states behavior)
- `opctl container ls` table no longer interleaves per-container `DELETE LABELS` sub-rows, which were breaking column alignment whenever a long image ref or label value
  caused the terminal to wrap; the labels section is now opt-in via `-v/--verbose` and prints below the table

### Fixed

- Local node startup now repairs ownership of opctl data-dir entries left root-owned by prior sudo'd invocations, so subsequent non-root opctl runs can read and traverse
  them
- The macOS WireGuard `mac-net-connect` helper's per-connection `IpcHandle` goroutine (spawned in `ensureNetworkAttached.go`) is now wrapped in a panic recoverer that logs
  the stack instead of taking down the whole daemon process. An unrecovered panic in this nested goroutine is the most likely cause of the "daemon vanished mid-op,
  containers left running" symptom observed during local-dev `make up` runs
- `ContainerCreate` no longer inherits cancellation from the parent op context. Docker's apiproxy cannot abort an in-flight create when the HTTP client disconnects, so
  cancelling mid-create previously left dockerd running the create on its side and producing an invisible `Created`-state container with bind-mount references — exactly the
  pathology that wedges subsequent `ContainerCreate` calls. The call now runs to completion (or to opctl's own 20s mutation timeout) so we always know definitively whether
  a container exists. Trade-off: `Ctrl+C` during an in-flight create may take up to 20s to take effect; in exchange, no more invisible orphans
- When opctl's own `ContainerCreate` timeout fires (e.g. dockerd genuinely wedged), opctl now reconciles by listing for any container carrying the call's
  `opctl.container-id=<callID>` label and force-removes it if found. Handles the rare-but-real case where dockerd completes the create *after* our deadline. Reports either
  "FOUND orphan ... killing" or "no orphan found; dockerd really did not create the container" so the trace is unambiguous
- Successful `ContainerCreate` calls now log the container ID returned by Docker (`[opctl docker] ContainerCreate created id=<id> name=<name>`), making opctl traces
  correlatable with `docker ps -a` output, the `make docker-logs` apiproxy stream, and any `make docker-daemon-logs` goroutine dumps. Previously the ID was discarded with
  `_, err := ...`
- `instrumentedDockerCall` now demotes the three expected-by-design Docker error patterns from `[opctl docker debug] <op> failed` to `[opctl docker debug] <op> noop
  (<reason>)`: `ContainerStop`/`ContainerRemove` on a not-found container (the if-exists path), `ContainerRemove` racing an in-progress removal (kill cascade vs. cleanup
  defer), and `NetworkCreate` against an already-existing network (race-tolerant ensure pattern). Cleans up the daemon log so real errors stand out when they happen

## [0.1.77] - 2026-05-17

### Added

- Added RTK command guidance for Codex and Claude workflows, including token-optimized command examples and verification steps
- Added project-local RTK filter configuration for future command-output compaction
- Added project memory guidance requiring workbranch changes to maintain the next patch-version `CHANGELOG.md` entry

### Changed

- Contributor guidance now links to the RTK command instructions from `AGENTS.md`

## [0.1.76] - 2026-05-17

### Added

- Docker containers created by opctl now include opctl-managed labels for container ID, container name, image ref, and management ownership
- New `opctl container ls` and `opctl node container ls` commands list opctl-managed containers with copyable delete labels
- New `opctl container delete --label` and `opctl node container delete --label` commands delete opctl-managed containers by Docker labels,
  with interactive selection when multiple containers match
- Compile ops and Makefile helpers now support passing a `selfUpdateRepo` so fork builds can self-update from fork releases

### Changed

- Docker container names now include a readable slug from the container name or image plus a short ID, while preserving the `opctl_` prefix
- Container cleanup now deletes by labels or exact selected container ID/name instead of relying only on legacy full-ID container names
- `self-update` now uses a build-time configurable GitHub owner/repo instead of always checking `opctl/opctl`
- Fork release, GitHub Actions, and issue-template configuration now target the `soultech67/opctl` fork

### Fixed

- `opctl node delete` no longer prints command usage for runtime cleanup errors
- Docker cleanup treats containers already being removed as successfully deleted
- Build ldflags for version and self-update repo are passed directly to `go build`, avoiding broken `GOFLAGS` parsing for multiple `-X` values

## [0.1.75] - 2026-03-19

### Fixed

- Git ops with re-pointed tags (e.g. floating `v1` tags) are now re-fetched when the remote tag points to a new commit

## [0.1.74] - 2025-09-26

### Fixed

- ls and ui command examples now reference valid ops
- embedded container runtime now uses caller UID & GID (instead of hardcoded UID & GID)

## [0.1.73] - 2025-09-16

### Added

- Introduce embedded container runtime

### Changed

- Deprecate docker container runtime; use embedded runtime

## [0.1.72] - 2025-04-13

### Changed

- CLI command examples now are useable when possible and comments now use full sentences

### Fixed

- CLI command help no longer includes empty `Examples` section when no examples exist
- CLI `op validate` command broken for remote ops

## [0.1.71] - 2025-04-10

### Added

- Examples in CLI help/usage
- Autogenerate CLI docs for website from actual CLI

### Changed

- CLI version command now outputs to stdout (instead of stderr)

### Fixed

- `sudo: opctl command not found` error if `opctl` not on PATH

## [0.1.70] - 2025-04-02

### Added

- Support default auth for pulling ops via `opctl auth add [...]`

## [0.1.69] - 2025-04-02

### Added

- Support for specifying image.platform.arch in container calls

## [0.1.68] - 2025-03-22

### Fixed

- Fix accessing containers by name from opctl nodes stops working due to prior registrations not being cleaned up on container exit

## [0.1.67] - 2025-03-08

### Fixed

- Fix accessing Docker For Mac 4.39.0+ containers by name from opctl nodes

## [0.1.66] - 2025-02-26

### Added

- Support for environment variables for each CLI command options and arguments

## [0.1.65] - 2025-02-10

### Added

- Automatic handling of mDNSResponder port conflict on OSX

### Fixed

- Binding opctl DNS to a non-standard port (e.g. 54)

## [0.1.64] - 2025-02-10

### Added

- Leverage native privilege escalation handling for self-update

## [0.1.63] - 2025-02-08

### Added

- Native privilege escalation handling (i.e. no more calling with sudo)

## [0.1.62] - 2025-02-05

### Fixed

- Put in a short term fix so that when `pullCreds` is used on Linux against
something other than `docker.io`, things don't error out any more.

## [0.1.60] - 2024-11-24

### Fixed

- Fix a regression in the 'opctl op install' command introduced in 0.1.48 which caused it to be much slower than previous
- Fix a potential race condition encountered when pulling/using multiple remote ops from the same repo at once

## [0.1.59] - 2024-11-17

### Fixed

- `No such container: opctl_[...]` errors if container exits too fast (race condition)

## [0.1.58] - 2024-11-13

### Fixed

- Fix executable bit not maintained within remote ops

## [0.1.57] - 2024-11-12

### Changed

- Don't log "not found in graph" to logs (noisy without value)

### Fixed

- Fix 'opctl node kill' panics if you've never run an op
- Rate limit errors surfacing as "image not found" errors
- Fixed file initialization results in incorrect (empty) file data

## [0.1.56] - 2024-11-11

### Added

- Access containers by their name from opctl nodes

### Deprecated

- `container.ports`; access containers by name instead

## [0.1.56-alpha.1] - 2024-11-08

### Added

- Access containers by their name from opctl nodes

### Deprecated

- `container.ports`; access containers by name instead

## [0.1.56-alpha.0] - 2024-10-30

### Added

- Access containers by their name from opctl nodes

### Deprecated

- `container.ports`; access containers by name instead

## [0.1.55] - 2024-09-04

### Fixed

- Fix long time to resolve non-existent op

## [0.1.53] - 2024-04-18

### Fixed

- A few dependency updates
  - in the Golang SDK the only breaking upgrade is Docker v23 -> v25.0.3+incompatible
  - in the JS SDK nock (a devDependency) was updated from 9 -> 13.
  - in the JS SDK react-ace was updated from 9 -> 11

### Removed

- Deleted the React SDK because it is unused

## [0.1.52] - 2023-06-06

### Fixed

- [Escaped references are not escaped if an unescaped '$' exists prior in the string](https://github.com/opctl/opctl/issues/1063)

## [0.1.51] - 2023-06-01

### Added

- [Variable reference as `container.cmd`](https://github.com/opctl/opctl/issues/1064)

### Fixed

- Simultaneously defining a default plus constraints on an object or array input results in a validation error

## [0.1.50] - 2023-05-01

### Added

- Automatic GPU detection/passthrough for docker runtime

### Fixed

- `opctl ls` renders ops with no description as blank line
- `lte` predicate
- In some cases, stacktraces logged as bytes rather than strings
- Containers aren't automatically removed after exit for docker runtime
- In some cases, `Runtime error: invalid memory address or nil pointer dereference` error occurs
- CLI not logging errors from the root op

## [0.1.49] - 2022-09-21

### Added

- opspec now supports gt, gte, lt, lte predicates
- `opctl node kill` will now stop and remove any opctl managed containers
- introduced `opctl node delete` command which "Deletes a node. This is destructive! all node data including auth, caches, and operation state will be permanently removed."

### Changed

- upgrading to this version from prior versions is destructive! all node data including auth, caches, and operation state will be permanently removed.
- K8s container runtime now explicitly deletes terminated pods

### Fixed

- [Node locking mechanism doesn't ensure process is opctl](https://github.com/opctl/opctl/issues/913)
- [CLI no longer logs errors occurring in parallel calls](https://github.com/opctl/opctl/issues/1032)

## [0.1.48] - 2021-08-13

### Added

- When running an op via opctl run, display progress via a live call graph
- When running an op via opctl run, prefix log lines emitted by workloads with their op id & ref
- Basic support for sending local files and directories to remote nodes when using the API client
- [Allow defining description on call graph nodes](https://github.com/opctl/opctl/issues/900)

### Changed

- Self-update now uses github releases instead of equinox.io
- API now limits request body to 40Mb
- [Improved error output when op resolution fails. You'll now see a list of resolutions tried and why each failed.](https://github.com/opctl/opctl/pull/883)
- [More consistent error messaging formats](https://github.com/opctl/opctl/pull/885)
- [Detect invalid op output names](https://github.com/opctl/opctl/issues/798)
- [Allow using type initializers in input/output defaults](https://github.com/opctl/opctl/issues/957)
- [Deprecated absolute paths as file/dir input/output defaults](https://github.com/opctl/opctl/issues/957)
- [Deprecated op output binding syntax; use same syntax as binding inputs](https://github.com/opctl/opctl/issues/721)
- [Deprecated param.<datatype>.description; use param.description](https://github.com/opctl/opctl/issues/898)
- [Docker images will only be pulled if using the `latest` tag (or untagged) or have not been pulled previously](https://github.com/opctl/opctl/issues/920)
- Go SDK models now use DataRef rather than PkgRef

### Fixed

- [vscode intellisense error](https://github.com/opctl/opctl/issues/615)

### Removed

- pkgs API endpoint; use data API endpoint
- Windows build; use linux build via WSL 2 instead

## [0.1.47] - 2021-01-22

### Added

- [Improve CLI prompts for username and password](https://github.com/opctl/opctl/issues/745)

### Fixed

- Dir initializer doesn't initialize more than one child entry

## [0.1.46] - 2021-01-04

### Fixed

- [container.envVars string double interpreted](https://github.com/opctl/opctl/issues/849)

## [0.1.45] - 2020-11-17

### Fixed

- Calls killed by needs declaration exiting non-zero

## [0.1.44] - 2020-11-16

### Added

- [Dir Initializer Syntax](https://github.com/opctl/opctl/issues/500)

### Changed

- [Opspec) use relative paths for file/dir refs](https://github.com/opctl/opctl/issues/834)
- [Make input/output binding when calling ops consistent](https://github.com/opctl/opctl/issues/721)

### Fixed

- Certain child call errors not shown.

## [0.1.43] - 2020-11-04

### Fixed

- ParallelLoop loop iteration vars sometimes get set to values from other iterations.

## [0.1.42] - 2020-11-03

### Added

- [Add ability to add auth to opctl for OCI image registries](https://github.com/opctl/opctl/issues/823)
- [Better messages for parallel/parallelLoop child errors](https://github.com/opctl/opctl/issues/827)

### Changed

- [Make OpKill Event Driven](https://github.com/opctl/opctl/issues/809)
- [Remove CallKilled event](https://github.com/opctl/opctl/issues/810)
- [Remove ContainerExited event](https://github.com/opctl/opctl/issues/825)
- [Remove OpErred event](https://github.com/opctl/opctl/issues/812)
- [Remove Event suffix from event types](https://github.com/opctl/opctl/issues/814)
- [Rename types from SCG/DCG](https://github.com/opctl/opctl/issues/816)

### Fixed

- [Gracefully handle docker restarts](https://github.com/opctl/opctl/issues/678)
- [Running an op should never kill a node](https://github.com/opctl/opctl/issues/756)

## [0.1.41] - 2020-06-03

### Changed

- [Listen on localhost by default](https://github.com/opctl/opctl/issues/738)

## [0.1.39] - 2020-05-04

### Fixed

- ["manifest has unsupported version: 4" errors on newer versions of opctl](https://github.com/opctl/opctl/issues/768)

## [0.1.38] - 2020-05-03

### Changed

- [Make opctl ls error if invalid ops are encountered](https://github.com/opctl/opctl/issues/708)
- [Return ref instead of name from opctl ls](https://github.com/opctl/opctl/issues/634)

### Fixed

- [Nested ops can't be referenced using relative path](https://github.com/opctl/opctl/issues/762)
- [Inconsistent behavior when running locally installed vs remotely referenced ops.](https://github.com/opctl/opctl/issues/732)

## [0.1.37] - 2020-05-03

### Added

- [ui subcommand to open webui](https://github.com/opctl/opctl/issues/758)
- Render op icons in UI
- Automatically expand mount ancestors in explorer UI
- Make call bounding box extend from call summary rather than start below in UI
- Remove extraneous lines extending from top and bottom of parallel call in UI

## [0.1.35] - 2020-04-29

### Changed

- [Stop logging "Replaying from value pointer: {Fid:0 Len:0 Offset:0}"](https://github.com/opctl/opctl/issues/754)

## [0.1.34] - 2020-04-23

### Added

- [UI: visualize referenced ops](https://github.com/opctl/opctl/issues/739)

## [0.1.33] - 2020-04-20

### Fixed

- [Nonexistent sub dirs bound to containers aren't sync'd](https://github.com/opctl/opctl/issues/725)
- [image.ref with multi-variable templated string not working since v0.1.28](https://github.com/opctl/opctl/issues/722)

## [0.1.32] - 2020-04-16

### Added

- [Prefix opctl managed container names with opctl\_](https://github.com/opctl/opctl/issues/735)

### Fixed

- variable reference validation triggered for valid refs

## [0.1.31] - 2020-04-15

### Fixed

- variable reference validation triggered for valid refs
- failure interpreting needed call panics

## [0.1.30] - 2020-04-15

### Added

- [Named calls and needs](https://github.com/opctl/opctl/issues/643)

## [0.1.29] - 2020-04-02

### Fixed

- [Running `op install` twice wipes out op file contents](https://github.com/opctl/opctl/issues/718)

## [0.1.28] - 2020-03-26

### Added

- UI: Workspace page (explorer, op visualizer with pan/zoom)
- [Support in scope dir as op](https://github.com/opctl/opctl/issues/646)
- Liveness method to node API Client
- Variable reference as `image.ref`.

### Changed

- When daemonizing opctl node, parent process env vars no longer inherited by daemonized process. This for example thwarts Jenkins ProcessTreeKiller's killing abilities.

### Deprecated

- `image.src`; use `image.ref`

### Fixed

- API Liveness endpoint incorrectly returning 404

### Removed

- UI: Events/Op/Vars pages

## [0.1.27] - 2020-02-04

### Fixed

- Object initializers passed as inputs to constrained parameters don't pass validation

## [0.1.26] - 2020-01-30

### Added

- [Support in scope dir as container image](https://github.com/opctl/opctl/issues/498)
- [Pass thru errors encountered when cli auto daemonizes a node](https://github.com/opctl/opctl/issues/368)
- [Allow Interpolating Container `workDir`](https://github.com/opctl/opctl/issues/648)
- `Container.sockets` bindings with variable reference syntax i.e. `/my/socket: $(mySocket)`

### Changed

- Don't cleanup OPCTL_DATA_DIR on node creation.

### Deprecated

- `Container.sockets` bindings without variable reference syntax i.e. instead of `/my/socket: mySocket`, use `/my/socket: $(mySocket)`.

### Fixed

- [Referencing Non Directories As Directories Hangs](https://github.com/opctl/opctl/issues/637)
- Implicit inputs not coerced
- Results of Serial Call Children Running For > 10s Ignored
- [Child Op Call Inputs Not Required](https://github.com/opctl/opctl/pull/665)

### Removed

- Remove support for `.` in op parameter names (to avoid ambiguity between referencing object properties)

## [0.1.25] - 2019-07-13

### Added

- Allow dynamically setting env vars of a container
- NotExists predicate
- Exists predicate
- up to 10x disk performance improvement on OSX
- Ability to specify custom node data dir
- Allow Numbers for Container Ports
- Interpolate Container Name
- Conditional running
- serialLoop call
- parallelLoop call

### Fixed

- opctl ls on windows does not list anything
- object & array initializers don't support multiline values
- errors from parallel calls not logged

### Removed

- `stdOut` & `stdErr` attributes from container call. Use files.
- `pkg` attribute in opcall; `ref` & `pullCreds` raised up a level, nesting within `pkg` unnecessary.

### Changed

- website/docs moved to [https://github.com/opctl/website](https://github.com/opctl/website)

## [0.1.24] - 2018-04-06

### Added

- `opspec` property in op schema
- Client back pressure in `GET event-stream` endpoint via `ack` query param
- Support declaring SVG icon for op
- Support CommonMark for op & param descriptions
- Boolean type
- Support type, description, writeOnly, & title keywords in constraints of object params
- Support paths in object refs
- Object & Array initializers
- Support referencing object properties via `object[propertyName]`
- Support referencing array items via `array[index]` or `array[-index]` to index from end of array
- Interpolation escape syntax by prefixing w/ a single backslash; i.e. `\$(not-a-ref)`
- Unified data API

### Deprecated

- `pkg` attribute in opcall; `ref` & `pullCreds` raised up a level, nesting within `pkg` unnecessary
- Deprecate pkgs API
- `stdOut` & `stdErr` attributes from container call. Use files.

### Removed

- References in/as expressions w/out explicit opener $( and closer )

## [0.1.23] - 2018-01-15

### Added

- opspec 0.1.6) Support declaring SVG icon for pkg
- opspec 0.1.6) Support CommonMark for pkg & param descriptions

### Fixed

- coercion doesn't occur when de referencing scope object paths
- scope file path refs don't de reference

## [0.1.22] - 2017-11-05

### Added

- Always pull container images when running ops

### Fixed

- Auto node creation requires opctl bin within path

## [0.1.21] - 2017-10-01

### Added

- Validation of outputs
- Remote pkg run
- Remote pkg validate
- Type coercion
- Add /pkgs/{ref}/contents endpoints to node API
- Support binding strings &/or numbers to/from container files
- Add support for object param type
- Add support for array param type

### Deprecated

- op.yml without `opspec` property
- References in/as expressions w/out explicit opener `$(` and closer `)`

### Fixed

- [Color flags not working](https://github.com/opctl/opctl/issues/278)
- [Race condition for non-cached pkgs](https://github.com/opctl/opctl/issues/253)
- [Binding pkg file/dir to sub op input doesn't work](https://github.com/opctl/opctl/issues/249)
- [Outputs not defaulted](https://github.com/opctl/opctl/issues/314)

### Removed

- `ref` attribute in opcall
  Use new `pkg` attribute.
- `pullIdentity` & `pullSecret` attributes in container call.
  Use new `pullCreds` attribute.

### Changed

- api schema updated to OAS 3.0.0
  syntax

## [0.1.20] - 2017-06-23

### Fixed

- [Unexpected git capabilities encountered during pkg pull not gracefully handled](https://github.com/opctl/opctl/issues/257)

## [0.1.19] - 2017-06-05

### Added

- Support using dir/file embedded in op as input/output param default
- Allow path expansion w/in sub op call inputs
- Allow string/number literals as sub op call inputs
- Implicitly bind env vars to in scope refs if names are identical
- `pkg install` command
- [Validate file/dir inputs are valid files/dirs (respectively)](https://github.com/opctl/opctl/issues/175)
- [Fail fast during parallel call](https://github.com/opctl/opctl/issues/154)
- [Support since in event filter](https://github.com/opctl/opctl/issues/187)
- [Add github style heading links to web docs](https://github.com/opctl/opctl/issues/194)
- [Add copy code to clipboard link to web docs](https://github.com/opctl/opctl/issues/195)

### Deprecated

- `ref` attribute in
  [op.yml.schema.json#/definitions/opCall](spec/op.yml.schema.json#/definitions/opCall).
  Use new `pkg` attribute.
- `pullIdentity` & `pullSecret` attributes in
  [op.yml.schema.json#/definitions/containerCall](spec/op.yml.schema.json#/definitions/containerCall).
  Use new `pullCreds` attribute.

### Removed

- `pkg set` command

### Fixed

- [Killing a run (ctrl+c) from powershell hangs](https://github.com/opctl/opctl/issues/199)
- [Network creation race condition](https://github.com/opctl/opctl/issues/190)
- [Param defaults w/ values equal to type default are not defaulted](https://github.com/opctl/opctl/issues/185)
- [stdOut/stdErr output race condition](https://github.com/opctl/opctl/issues/174)
- [Unable to run ops w/ containers if using docker 4 windows](https://github.com/opctl/opctl/issues/200)

## [0.1.18] - 2017-03-28

### Changed

- [Don't recreate node on self-update](https://github.com/opctl/opctl/issues/169)

### Fixed

- [Multiple opctl networks created leading to lack of inter-container connectivity](https://github.com/opctl/opctl/issues/167)

## [0.1.16] - 2017-03-26

### Fixed

- [Outputs internal to op call graph not initialized](https://github.com/opctl/opctl/issues/165)

## [0.1.15] - 2017-03-23

### Added

- Add `node` command w/ `create` and `kill` subcommands
- [Add ability to override default (`.opspec`) package location for `pkg set`, `pkg create`, `run`, and `ls` commands](https://github.com/opctl/opctl/issues/44)
- [Add output coloring](https://github.com/opctl/opctl/issues/49)
- Add input validation
- Added package validation via `pkg validate` command & before `run`
- Add `pkg` command w/ `validate`, `set`, `create` subcommands
- typed params; `dir`, `file`, `number`, `socket`, `string`
- `string` and `number` parameter constraints
- support for container calls
- `filter` to node API `/events/stream` resource
- support for private images

### Changed

- op call changed from `string` to `object` w/ `ref`, `inputs`, and
  `outputs` attributes. To migrate, replace string value with object
  having `ref` attribute equal to existing string and pass
  `inputs`/`outputs` as applicable.
- String parameters must now be declared as an object:

```yaml
paramName:
  string:
    description: ...
    # and so on...
```

### Removed

- `docker-compose.yml`; replaced with container calls
- collections
- bubbling of default collection lookup
- support for < [opspec 0.1.3](https://opspec.io)
- `collection` command

## [0.1.10] - 2016-11-21

### Added

- [Add support for "default" input values](https://github.com/opctl/opctl/issues/41)

## [0.1.9] - 2016-11-06

### Added

- `serial`, `op`, and `parallel` calls
- nested calls (applicable to `serial` & `parallel` calls)
- json schema

### Changed

- refactored to use [sdks/go](https://github.com/opctl/opctl/sdks/go)
- params no longer support `type` attribute;
- `subOps` call; use new `op` call

### Fixed

- [Emitted ContainerStd\*WrittenToEvent.Data Incomplete](https://github.com/opctl/opctl/issues/32)

## [0.1.8] - 2016-09-09

### Added

- support for [opspec 0.1.2](https://opspec.io)

### Fixed

- [failure of serial operation run does not immediately fail all following operations](https://github.com/opctl/cli/issues/5)

### Removed

- support for < [opspec 0.1.2](https://opspec.io)

## [0.1.7] - 2016-09-02

### Fixed

- [opctl does not wait for parallel op containers to die before returning](https://github.com/opctl/cli/issues/8)
- [Many parallel ops crash engine](https://github.com/opctl/opctl/issues/17)

## [0.1.6] - 2016-08-21

### Fixed

- OpEnded event not sent on `Failed` outcome

## [0.1.5] - 2016-08-02

### Added

- support for [opspec 0.1.1](https://opspec.io)

### Removed

- support for [opspec 0.1.0](https://opspec.io)

## [0.1.4] - 2016-07-20

### Added

- normalization of windows paths if provided to op run

## [0.1.3] - 2016-07-09

### Added

- [Support new opspec subop `isParallel` flag](https://github.com/opctl/opctl/issues/11)

### Fixed

- [Unable to simultaneously run multiple ops from same collection](https://github.com/opctl/opctl/issues/10)

## [0.1.2] - 2016-06-22

### Fixed

- [Missleading `variable is not set` message on op finish](https://github.com/opctl/opctl/issues/5)
- [Engine not observing exitcode of op entrypoint](https://github.com/opctl/opctl/issues/9)

## [0.1.1] - 2016-06-22

### Changed

- refactored to use opspec engine sdk

### Fixed

- kill op run use case killing all ops
- [cannot run multiple ops with same name simultaneously](https://github.com/opctl/opctl/issues/8)

### Removed

- add sub-op use case

## [0.1.0] - 2016-06-16

### Removed

- set op description use case
- add op use case
- list ops use case
