# opctl CLI usage

A human-friendly reference for every command exposed by the `opctl` CLI, sourced
directly from the cobra command definitions under `cli/cmd/`.

## What is an "opctl node" and what happens when you start one?

`opctl` is split into two halves:

- **The CLI** — the `opctl` binary you invoke (`opctl run …`, `opctl ls …`,
  etc.). It is a thin HTTP client.
- **The node** — a long-running daemon (also the `opctl` binary, just invoked
  as `opctl node create`) that does the actual work: scheduling ops, running
  containers, caching pulled ops/images, storing events, and serving the web
  UI.

### Auto-start: how a node appears the first time you use the CLI

Almost every user-facing command (`run`, `ls`, `ui`, `events`, `auth add`, the
`op` family) needs a node. Each of those commands first pings the API at
`--api-listen-address`. If a node answers, the CLI just talks to it. If
nothing answers, the CLI re-execs itself as `opctl node create` in the
background (with the same `--api-listen-address`, `--container-runtime`,
`--data-dir`, and `--dns-listen-address` flags), waits for the API to become
reachable, and then continues. This is why most commands "just work" without
any setup step — and also why most commands need root: starting the daemon
binds privileged ports (DNS on `:53`) and writes to a shared data dir.

### What the node actually runs

Once `opctl node create` is up, the daemon spins up three things in parallel:

1. **An HTTP server on `--api-listen-address`** (default `127.0.0.1:42224`),
   serving two prefixes:
   - `/api/*` — the opctl REST + websocket API. This is the only surface the
     CLI uses; everything `run`, `events`, `op kill`, `auth add`, etc. do is
     just HTTP calls to here.
   - `/` — the embedded **opctl web UI** (a static webapp baked into the
     binary). `opctl ui` is a convenience wrapper that opens
     `http://<api-listen-address>/?mount=…` in your browser.
2. **A UDP DNS server on `--dns-listen-address`** (default `127.0.0.1:53`).
   When a container is started, its hostname is registered here. That lets
   sibling containers and the node itself resolve each other by name.
3. **A background event processor.** All op lifecycle activity flows through a
   pub/sub backed by a BadgerDB on disk; the processor consumes events (e.g.
   kill requests) and replays history when clients reconnect.

Everything is persisted under `--data-dir` (default a per-user app-data path):

- `dcg/events/` — the BadgerDB of all events. Survives restarts; that's why
  `opctl events` can replay past activity, and why `opctl node kill` is
  non-destructive while `opctl node delete` is destructive (it `rm -rf`'s
  this directory).
- `ops/` — the cache of pulled ops and images.
- `logs/` — durable log files. `logs/node.log` is the daemon's own rotating log;
  `logs/containers/<name>_<opHash>/{stdout,stderr}.log` capture each container's
  stdout/stderr to rotating files (**on by default**), so you can read an op's
  output after it — or the daemon — has stopped. Configure per container with the
  opfile `container.log` block (`enabled`, `dir`, `maxSizeMB`, `maxBackups`,
  `maxAgeDays`, `compress`; set `dir` to a host folder, e.g. the host side of your
  `workDir` bind mount, to land them in your project), or globally with the
  `OPCTL_CONTAINER_LOG*` env vars (see `docs/environment-variables.md`).

### Containers and the network the node builds

When an op runs, the node calls into the **container runtime** chosen by
`--container-runtime` (`docker` (deprecated), `k8s`, or `embedded`). Whichever
runtime is in use, the node:

- attaches every container it starts to a shared overlay network,
- registers the container's hostname with the local DNS server, so
- any container can reach any sibling — or reach the node's API — using
  ordinary DNS names. There is no port-mapping or service-discovery boilerplate
  to write inside ops; if two ops in the same run need to talk, they just use
  each other's names.

Containers are removed automatically when they exit. Pulled images are cached
and refreshed best-effort: if a refresh fails, the node falls back to the
cached copy so offline / flaky-registry runs still work.

### Using a node over a network

The fact that the CLI and the node communicate over plain HTTP is the main
extension point.

- **Point the CLI at a remote node.** Override `--api-listen-address` (or
  `OPCTL_API_LISTEN_ADDRESS`) to a remote `host:port`. The CLI's auto-start
  logic will only fire if the address is unreachable, so against a healthy
  remote node it will just connect. The same env var also lets CI agents,
  scripts, and IDE integrations target a shared node.
- **Expose a node on a LAN / inside a cluster.** Start it with
  `--api-listen-address 0.0.0.0:42224`. Anyone routable to that address can
  drive it. **The API has no built-in authentication**, so only expose nodes
  inside trusted networks (or behind your own reverse proxy / mTLS / VPN).
- **Hit the API directly.** Anything that speaks HTTP/WebSocket can use
  `/api/*` — the same endpoints the CLI uses for starting ops, killing ops,
  streaming events, and managing auth. This is how the bundled webapp and the
  Go/JS SDKs work.
- **Share registry/source credentials with the node.** `opctl auth add` stores
  credentials inside the node's data dir, so future pulls of ops or images
  from that host reuse them — useful for CI workers and for shared remote
  nodes.
- **Stream activity from anywhere.** `opctl events` (and the underlying
  websocket on `/api/`) is a real-time, replay-from-history feed. You can use
  it to drive dashboards, log shippers, or other tools without polling.
- **Lifecycle a remote node like any other daemon.** `opctl node kill`
  shuts the API + DNS + event loop down cleanly while leaving state intact;
  `opctl node delete` wipes the data dir entirely; `opctl self-update`
  kills the running node before swapping the binary.

## Command tree

The CLI is structured as a small tree:

```
opctl
├── auth
│   └── add
├── events
├── ls
├── node
│   ├── create
│   ├── delete
│   └── kill
├── op
│   ├── create
│   ├── install
│   ├── kill
│   └── validate
├── run
├── self-update
└── ui
```

## Conventions

- **Args in `UPPER_CASE`** are required positional arguments. Args in
  `[BRACKETS]` are optional.
- **Flags** behave like standard POSIX flags. Most have a short form (`-x`) and
  a long form (`--xxx`).
- **Env vars**: every local flag can also be set through an environment variable
  named `OPCTL_<COMMAND_PATH>_<FLAG_NAME>`, where the command path and flag are
  uppercased and `-` / spaces become `_`. For example, `opctl auth add
  --username` can be set with `OPCTL_AUTH_ADD_USERNAME`, and the persistent
  `--api-listen-address` flag can be set with `OPCTL_API_LISTEN_ADDRESS`. An
  explicit flag value always wins over the env var. The `--help` output for any
  command lists the exact env var name for every flag.
- **Auto-starting a node**: most commands that talk to opctl (`run`, `ls`,
  `ui`, `events`, `auth add`, the `op` family) will silently start a local
  node if one isn't already running.

## Global (persistent) flags

These flags are accepted by every command in the tree. They are defined on the
root command in `cli/cmd/root.go`.

| Flag | Default | Purpose |
| --- | --- | --- |
| `--api-listen-address` | `127.0.0.1:42224` | IP:PORT the opctl API server listens on. |
| `--container-runtime` | `docker` | Runtime used to run opctl containers. One of `docker` (deprecated), `k8s`, or `embedded`. |
| `--data-dir` | `<per-user-app-data>/opctl` | Directory used to store all opctl state (caches, auth, op data). |
| `--dns-listen-address` | `127.0.0.1:53` | IP:PORT the embedded DNS server listens on. |
| `--no-color` | `false` | Disable colored output. |
| `-h, --help` | — | Print help for the current command. |
| `-v, --version` | — | Print the CLI version and exit. |

---

## `opctl auth`

```
opctl auth
```

Parent command that groups authentication-related subcommands. It is not
runnable on its own — use one of the subcommands below.

### `opctl auth add RESOURCES`

Add default credentials that the node will use when pulling ops or container
images from a given source (a Docker registry, a git host, etc.).

**Arguments**

- `RESOURCES` (required) — the host/registry these credentials apply to, e.g.
  `docker.io` or `github.com`.

**Flags**

| Flag | Default | Purpose |
| --- | --- | --- |
| `-u, --username` | _empty_ | Username to authenticate with. |
| `-p, --password` | _empty_ | Password (or token) to authenticate with. |

**Examples**

```bash
# add default auth for docker.io
opctl auth add docker.io -u='my-username' -p='my-password'

# add default auth for github.com
opctl auth add github.com -u='my-username' -p='my-password'
```

---

## `opctl events`

```
opctl events
```

Stream events from an opctl node over a websocket. Past events are replayed
when streaming starts and new events are delivered in real time.

If no node is reachable on `--api-listen-address`, one is started automatically.

Because all events are persisted (`dcg/events/`), `--since` and `--roots` let you
pull a **subset of past activity** out of the durable store — e.g. one op's
stdout/stderr after it (or the daemon) has stopped.

**Arguments**: none.

**Flags** (in addition to the global flags):

| Flag      | Description                                                                                                  |
| --------- | ----------------------------------------------------------------------------------------------------------- |
| `--since` | Only show events at or after this point — a duration relative to now (`90m`, `24h`) or an RFC3339 timestamp. |
| `--roots` | Only show events under these root call IDs (comma-separated or repeated); e.g. the root op id of an `opctl run`. |

```bash
# Everything (default — full history, then live):
opctl events

# Just the last 2 hours:
opctl events --since 2h

# Just one op's activity (by its root call id) — e.g. to read its output after it ended:
opctl events --roots 1c7d3307-…
```

---

## `opctl ls [DIR_REF]`

```
opctl ls [DIR_REF]
```

List every op defined in a directory. By default this lists ops in the
`.opspec` directory of the current working directory; you can also point it at
a remote opspec package.

**Arguments**

- `DIR_REF` (optional) — one of:
  - a relative path (e.g. `./my-pkg`)
  - an absolute path (e.g. `/srv/pkgs`)
  - a remote ref `host/repo-path#tag` (e.g. `github.com/opspec-pkgs/foo#1.0.0`)
  - a remote ref with sub-path `host/repo-path#tag/path`

**Flags**: only the global flags apply.

**Examples**

```bash
# List ops in .opspec/ of the current directory.
opctl ls

# List ops in a remote opspec package at a specific tag.
opctl ls github.com/opspec-pkgs/github.release.create#3.0.0
```

---

## `opctl node`

```
opctl node
```

Parent command for managing local opctl nodes. Not runnable on its own.

Each `node` subcommand resolves the container runtime via the persistent
`--container-runtime` flag before running.

### `opctl node create`

Start a long-running opctl node in the current shell. It serves the API on
`--api-listen-address` and DNS on `--dns-listen-address`, and uses the
runtime chosen by `--container-runtime`.

- Requires elevated privileges (the binary checks for euid 0).
- Fails fast with a clear message if another node is already running.
- Press `Ctrl+C` or send `SIGTERM` to shut it down gracefully.

**Arguments**: none.
**Flags**: only the global flags apply (`--api-listen-address`,
`--dns-listen-address`, `--data-dir`, `--container-runtime`).

### `opctl node delete`

Stop the node and **destroy** all of its state — auth, caches, operation
state, and the `--data-dir` itself.

- Requires elevated privileges.
- This is destructive; use `node kill` if you want to stop the node but keep
  its data.

**Arguments**: none. **Flags**: only the global flags apply.

### `opctl node kill`

Stop the running opctl node and any operations it is executing. Non-destructive
— auth, caches, and operation state are preserved so the node can be restarted
later with the same data.

- Requires elevated privileges.

**Arguments**: none. **Flags**: only the global flags apply.

---

## `opctl op`

```
opctl op
```

Parent command for working with ops. Not runnable on its own. Every `op`
subcommand starts (or connects to) a local node before running.

### `opctl op create NAME`

Scaffold a new op on disk.

**Arguments**

- `NAME` (required) — the name of the op. Also used as the directory name.

**Flags**

| Flag | Default | Purpose |
| --- | --- | --- |
| `-p, --path` | `.opspec` | Parent directory the op will be created under. The op ends up at `<path>/<NAME>`. |
| `-d, --description` | _empty_ | Human-readable description written into the op's `op.yml`. |

### `opctl op install OP_REF`

Download a remote op (and the rest of its package, so intra-package refs keep
working) into a local directory.

**Arguments**

- `OP_REF` (required) — a remote reference: `host/repo-path#tag` or
  `host/repo-path#tag/path`. Bare local paths are rejected — install only
  makes sense for remote refs.

**Flags**

| Flag | Default | Purpose |
| --- | --- | --- |
| `--path` | `.opspec` | Directory the op package will be installed under. The package lands at `<path>/<host>/<repo-path>#<tag>`. |
| `-u, --username` | _empty_ | Username for the op source. Used only if both username and password are set. |
| `-p, --password` | _empty_ | Password for the op source. Used only if both username and password are set. |

If credentials aren't provided and authentication fails, the CLI re-prompts for
them. In non-interactive terminals it exits non-zero instead.

**Example**

```bash
# Install the op at github.com/opspec-pkgs/uuid.v4.generate#1.1.0 into
# .opspec/github.com/opspec-pkgs/uuid.v4.generate#1.1.0 .
opctl op install github.com/opspec-pkgs/uuid.v4.generate#1.1.0
```

### `opctl op kill OP_ID`

Stop a running op by its op (call) id.

**Arguments**

- `OP_ID` (required) — the id of the op to kill, as reported by `opctl run`
  or `opctl events`.

**Flags**: only the global flags apply.

### `opctl op validate OP_REF`

Make sure an op is well-formed: that `op.yml` exists and is syntactically
valid opspec.

**Arguments**

- `OP_REF` (required) — same forms as `opctl run`: relative path, absolute
  path, `host/repo-path#tag`, or `host/repo-path#tag/path`.

**Flags**: only the global flags apply.

If authentication is required and fails, the CLI re-prompts; in non-interactive
terminals it exits non-zero.

**Examples**

```bash
# Validate a local op.
opctl op validate myOp

# Validate a remote op at a specific tag.
opctl op validate github.com/opspec-pkgs/slack.chat.post-message#1.1.0
```

---

## `opctl run OP_REF`

```
opctl run OP_REF [-a key=value ...] [--arg-file FILE] [--no-progress]
```

Run an op end-to-end. If no node is reachable, one is started automatically.

**Arguments**

- `OP_REF` (required) — one of:
  - relative path (e.g. `myOp` resolves to `.opspec/myOp`)
  - absolute path
  - remote ref `host/path/repo#tag`
  - remote ref with sub-path `host/path/repo#tag/path`

**Flags**

| Flag | Default | Purpose |
| --- | --- | --- |
| `-a, --args` | _empty list_ | Pass a single op input as `key=value`. Repeat the flag to pass multiple inputs. |
| `--arg-file` | `.opspec/args.yml` | YAML file containing op inputs. |
| `--no-progress` | `true` if stdout isn't a terminal, otherwise `false` | Disable the live call-graph progress display. Useful for CI logs. |

**Input resolution order** (first match wins):

1. `-a key=value` flag
2. arg file (`--arg-file`)
3. environment variable
4. default declared in the op's `op.yml`
5. interactive prompt (only available in interactive terminals)

If inputs can't be satisfied, the CLI prompts in interactive terminals and exits
non-zero in non-interactive ones. If provided args don't satisfy declared
constraints, the CLI re-prompts until they do.

**Behavior notes**

- All pulled ops and images are cached under `--data-dir`.
- Image updates are pulled when possible; on failure opctl falls back to the
  cached image.
- Containers are attached to an overlay network and are reachable by name from
  the node and from sibling containers. They are removed when they exit.
- Signals:
  - `Ctrl+C` (`SIGINT`) triggers a graceful kill; pressing it a second time
    forces immediate termination (exit code 130).
  - `SIGTERM` triggers a graceful kill.
  - `SIGINFO` (where supported) prints a one-shot snapshot of the current
    call graph.

**Exit codes**

| Code | Meaning |
| --- | --- |
| `0` | Op succeeded. |
| `1` | Op failed. |
| `130` | Terminated by a second Ctrl+C. |
| `137` | Op was killed (e.g. via `SIGINT`/`SIGTERM`). |

**Examples**

```bash
# Run a local op.
opctl run myOp

# Run a remote op with explicit args.
opctl run \
  -a apiToken="my-token" \
  -a channelName="my-channel" \
  -a msg="hello!" \
  github.com/opspec-pkgs/slack.chat.post-message#1.1.0
```

---

## `opctl self-update`

```
opctl self-update
```

Update the `opctl` binary to the latest non pre-release version published to
`opctl/opctl` on GitHub. If a local node is running, it is killed so the new
binary can take over cleanly.

- Requires elevated privileges (it writes over the installed binary).
- If you're already on the latest release, it prints a success message and
  exits 0 without doing anything.
- If killing the running node fails, the update is still applied but you'll
  need to run `opctl node kill` yourself to finish.

**Arguments**: none. **Flags**: only the global flags apply.

---

## `opctl ui [MOUNT_REF]`

```
opctl ui [MOUNT_REF]
```

Open the opctl web UI in your default browser, optionally pointing it at a
specific directory or package.

**Arguments**

- `MOUNT_REF` (optional) — what to mount in the UI. One of:
  - omitted — defaults to the current working directory
  - a dot-prefixed path (`.`, `./foo`) — treated as a regular relative path
    and resolved against the current working directory
  - any other relative/absolute path or remote ref (`host/repo-path#tag[/path]`)
    — resolved the same way `opctl run` resolves `OP_REF`

The UI is opened at
`http://<api-listen-address>/?mount=<resolved-mount>`. If no node is reachable,
one is started automatically.

**Flags**: only the global flags apply.

**Examples**

```bash
# Open the UI for the current directory.
opctl ui

# Open the UI for a remote opspec package.
opctl ui github.com/opspec-pkgs/github.release.create#3.0.0
```
