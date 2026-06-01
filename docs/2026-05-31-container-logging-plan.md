# Container log persistence — implementation plan (2026-05-31)

## Context

Today opctl streams each container's stdout/stderr to the console (as
`ContainerStdOut/StdErrWrittenTo` events) and durably persists those events to a
Badger store at `<dataDir>/dcg/events`. So logs *technically* survive shutdown
(`opctl events` can replay them), but that path is a poor "log file":

- **Silent loss** — the Badger key is the event timestamp to nanoseconds
  (`pubsub/eventStore.go:42`); two chunks in the same nanosecond overwrite each
  other. `stateStore.go:55-57` already band-aids this for lifecycle events.
- **Unbounded** — no TTL/pruning; grows forever, replayed O(n) per subscription.
- **Not browsable** — JSON/base64 in Badger; `opctl events` can only dump
  *everything* (CLI hardcodes an empty filter, though the HTTP handler supports
  `since`/`roots`).
- `docker logs` is lost once the container is torn down.

**Why opctl (not the app):** opctl is the execution environment. Per 12-Factor
(Factor XI) and every comparable platform (Docker `json-file` driver, k8s kubelet,
journald), capturing/rotating/persisting stdout/stderr is the *platform's* job;
the app just writes to stdout/stderr. opctl already owns the stream — adding a
durable, rotating **file** sink gives byte-exact, bounded, greppable per-container
logs that survive shutdown. The app keeps owning *what* to log and at what level;
opctl only captures the raw streams verbatim (no parsing/leveling).

**Decisions (locked with user):** container-level `log` schema; **on by default**;
node-level `OPCTL_CONTAINER_LOG*` env/flag defaults; **separate** `stdout.log` /
`stderr.log`; plus closing the `opctl events` CLI filter gap. Event-store
retention + nanosecond-collision fix are a **deliberate follow-up** (they touch the
shared state-reconstruction store — see Part D).

Builds on branch `feat/iterate-on-release` (the daemon-detach fix `6f91dbd11` is in).

## Design summary

Mirror the existing `image` field end-to-end (`model/opSpec.go:46-50` is the template):

```
opfile YAML ──► SCG model ──► DCG model ──► interpret(+default path) ──► consume(tee→rotating files)
 jsonschema   ContainerCallSpec  ContainerCall   container/Interpret        containerCaller.interpretLogs
   .log          .Log              .Log         (+ new logs/ subpkg)        (+ per-stream lumberjack)
```

- **Path + defaulting** are computed in the **node layer** (`containerlog.Resolve` +
  `DefaultDir`), not the interpreter — `dataDirPath` is threaded into
  `containerCaller` via `newContainerCaller` (`core.go`). This keeps `sdks/go/opspec`
  from importing `sdks/go/node` (layering inversion) and from reading env/data-dir.
- **The interpreter** (`container/logs.Interpret`) only resolves the scope-dependent
  `log.dir` and carries the rotation overrides — and returns **nil** when there's no
  `log` block, so containers without logging config interpret to an unchanged
  `ContainerCall` (preserves existing exact-equality tests).
- **Rotation knobs + enabled** resolve spec → node env → hardcoded default.
- **Layout:** `<dataDir>/logs/containers/<name>_<opHash>/{stdout.log,stderr.log}`,
  beside the daemon's `node.log`, created via `unsudo.CreateDir` (root daemon).
  Keyed on container name + a stable hash of the op path so the path is **stable
  across runs** (tail-able; and lets the consumer cache **one** rotating writer per
  path — avoiding lumberjack's per-writer mill-goroutine leak). Active files are
  chowned to the invoking user on each call; rotated backups may remain root-owned.
- **Tee point:** `node/containerCaller.go:interpretLogs` — write each chunk to a
  per-stream `lumberjack.Logger` alongside the existing `pubSub.Publish`. Additive,
  runtime-agnostic (docker + k8s), no `RunContainer` signature change. Guarded so a
  file-write error can never break the event/console pipeline. Writers are **cached
  per log path** (`logWriters *sync.Map`) and reused for the daemon's lifetime —
  **not** closed per call, since a fresh per-call writer leaks lumberjack's mill
  goroutine (its `Close()` doesn't reap it).
- **Reuse:** `lumberjack.v2` (already a dep, `go.mod`), the rotation default
  constants from `node/logging/logging.go:56-64` (50 MB / 5 backups / 30 days /
  compress), and the `unsudo.CreateDir` ownership pattern.

**Precedence (per knob):** container `log.*` value → node `OPCTL_CONTAINER_LOG*` →
hardcoded default. Master `enabled` defaults **true**.

---

## Schema reference

### New opfile field: `container.log` (optional)

Omitting `log` entirely is the common case — logging is on with defaults. The block
only needs the knobs you want to override.

```yaml
container:
  image: { ref: 'ghcr.io/astral-sh/uv:python3.12-bookworm' }
  cmd: [ ... ]
  log:                 # optional — omit to use defaults (logging ON)
    dir: $(../../logs) # optional HOST dir for the files; default = opctl data dir (see below)
    enabled: true      # default true; set false to opt THIS container out
    maxSizeMB: 50      # rotate a stream file once it reaches this size
    maxBackups: 5      # how many rotated backups to keep per stream
    maxAgeDays: 30     # delete rotated backups older than this
    compress: true     # gzip rotated backups
```

| Property | Type | Possible values | Default | Meaning |
|---|---|---|---|---|
| `dir` | string (host dir expression) | absolute host path, op-relative `$(../../logs)`, or a `dir` input/output — resolved to a **host** path like a `dirs` value | unset → `<data-dir>/logs/containers/…` | Host directory opctl writes this container's log files into. Point it at the host side of your `workDir` bind mount to also see them at `workDir/logs` *inside* the container (see "Where the files go" below). |
| `enabled` | boolean | `true`, `false` | `true` | Persist this container's stdout/stderr to rotating files. `false` opts the container out even when the node default is on. |
| `maxSizeMB` | integer (MB) | `≥ 1` | `50` | Max size of a single `stdout.log` / `stderr.log` before it's rotated to a timestamped backup. |
| `maxBackups` | integer | `0` = keep **all** backups (subject to `maxAgeDays`); `≥ 1` = keep at most that many | `5` | Number of rotated backups retained **per stream**. |
| `maxAgeDays` | integer (days) | `0` = **never** delete by age; `≥ 1` = delete older backups | `30` | Max age of a rotated backup before deletion (by its filename timestamp). |
| `compress` | boolean | `true`, `false` | `true` | gzip rotated backups (`…-<ts>.log.gz`). The active file is never compressed. |

> **lumberjack semantics (authoritative):** if **both** `maxBackups` and
> `maxAgeDays` are `0`, no backup is ever deleted (unbounded — defeats the point).
> The `5`/`30` defaults bound retention. `maxSizeMB` is whole megabytes; a single
> write larger than `maxSizeMB` errors rather than splitting (matches lumberjack).

### Where the files go (`log.dir`) and the `workDir` question

opctl writes the log files on the **host** (the daemon tees the streams to a host
file); it never writes *into* the container's filesystem. So `workDir` — a path
*inside* the container — isn't somewhere opctl can write directly. `log.dir` instead
names a **host** directory, resolved exactly like a `dirs` value (`dir.Interpret` →
host path), so it accepts an absolute path, an op-relative `$(…)`, or a `dir` ref.

- **Default (unset):** `<data-dir>/logs/containers/<name>_<opHash>/` — self-contained,
  no project clutter, removed only by `opctl node delete`.
  ```
  <data-dir>/logs/containers/<name>_<opHash>/
    ├── stdout.log                         # active
    ├── stdout-2026-05-31T18-04-02.123.log[.gz]   # rotated backups
    ├── stderr.log
    └── stderr-….log[.gz]
  ```
  `<name>` = the call's `name` (sanitized) or `container` if unnamed; `<opHash>` = a
  short stable hash of the op path. The path is **stable across runs** of the same
  container — `tail -F` follows it, and runs append + rotate by size rather than
  getting a fresh dir (the consumer caches one rotating writer per path).

- **Custom host dir** (e.g. `dir: $(../../logs)` or `dir: /Users/you/logs`): files
  go in *that* dir with **stable** names — `<name>.stdout.log` / `<name>.stderr.log`,
  rotated in place — so `tail -F` always follows the current file across runs.

**Getting "logs in workDir/logs":** point `log.dir` at the **host side of the bind
mount that backs your workDir.** In the compute-runtime dev op, `/src` (workDir) is
bound to `$(../..)` (the repo root), so:

```yaml
container:
  dirs:    { /src: $(../..) }
  workDir: /src
  log:     { dir: $(../../logs) }   # host <repo>/logs  ==  /src/logs inside the container
```

opctl writes `<repo>/logs/<name>.{stdout,stderr}.log` on the host; because `/src` ←
`<repo>`, the very same files are visible inside the container at `/src/logs/…`
(i.e. `workDir/logs`). You get both, with opctl as the sole writer. Files are
created **user-owned** (`unsudo`) so you can read/grep them without `sudo`.

> **Rejected:** auto-deriving the host path *from* `workDir`'s bind mount. It only
> works when workDir is a host bind (not a named volume) and silently breaks
> otherwise — an explicit `log.dir` is predictable. *(macOS note: writing into a
> bind-mounted dir is normal host I/O; the container's **view** of rotations rides
> the same VirtioFS bridge as any bind mount, but opctl never depends on the
> container seeing the logs.)*

### JSON Schema to add (`opspec/opfile/jsonschema.json`, under `container.properties`)

`dir` uses the expression `$ref` (it's a host-dir reference, resolved like `dirs`
values); the rotation knobs are plain typed config (`workDir`-style), not
interpolated data:

```json
"log": {
  "description": "Persistence + rotation of this container's stdout/stderr to durable files. Omit to use defaults (on). See docs/environment-variables.md for the node-level OPCTL_CONTAINER_LOG* defaults.",
  "type": "object",
  "properties": {
    "dir":        { "description": "Host directory to write this container's log files into, resolved like a dirs value (absolute host path, op-relative $(…), or a dir ref). Default: <data-dir>/logs/containers/<name>_<opHash>/ (stable across runs). Tip: point at the host side of your workDir bind mount to also see logs at workDir/logs inside the container.", "$ref": "#/definitions/expression" },
    "enabled":    { "description": "Persist this container's stdout/stderr to rotating files. Defaults to the node setting (OPCTL_CONTAINER_LOG, on). Set false to opt this container out.", "type": "boolean" },
    "maxSizeMB":  { "description": "Max size (MB) of a stdout.log/stderr.log before rotation. Default 50.", "type": "number" },
    "maxBackups": { "description": "Rotated backups to retain per stream. 0 = keep all (subject to maxAgeDays). Default 5.", "type": "number" },
    "maxAgeDays": { "description": "Max age (days) of rotated backups before deletion. 0 = never delete by age. Default 30.", "type": "number" },
    "compress":   { "description": "gzip rotated backups. Default true.", "type": "boolean" }
  },
  "additionalProperties": false
}
```

> **Option — expression support:** if you want knobs parameterizable from op inputs
> (e.g. `maxSizeMB: $(size)`), change the numeric/boolean fields to opctl expression
> refs (`"$ref": "#/definitions/expression"`) and interpolate them in the new
> `container/logs/` sub-interpreter (Part A4 already routes through it). Deferred for
> v1 to keep the schema concrete; flag if you want it in.

### Node-level defaults (env vars, forwarded to the daemon)

Set the fallback for any knob a container leaves unspecified. Per-container `log.*`
always wins over these; these always win over the hardcoded defaults.

| Env var | Type | Values | Default | Controls |
|---|---|---|---|---|
| `OPCTL_CONTAINER_LOG` | bool | `1/true/yes/on`, `0/false/no/off` | on | Global master switch. Per-container `log.enabled` overrides it. |
| `OPCTL_CONTAINER_LOG_MAX_SIZE_MB` | int | `≥ 1` | `50` | Default `maxSizeMB`. |
| `OPCTL_CONTAINER_LOG_MAX_BACKUPS` | int | `≥ 0` | `5` | Default `maxBackups`. |
| `OPCTL_CONTAINER_LOG_MAX_AGE_DAYS` | int | `≥ 0` | `30` | Default `maxAgeDays`. |
| `OPCTL_CONTAINER_LOG_COMPRESS` | bool | `1/true/yes/on`, `0/false/no/off` | on | Default `compress`. |

Bool parsing reuses `node/logging/logging.go:parseEnabled` (`1/true/yes/on` ⇒ true).

---

## Part A — Container log files feature

> **Status: ✅ implemented & green** (A1–A6). `gofmt`/`go build ./...`/`go vet` clean;
> `containerlog`, `container/logs`, `container`, and `node` unit tests pass.
> New packages: `sdks/go/node/containerlog`, `sdks/go/opspec/interpreter/call/container/logs`.
>
> **Design refinement vs. the original A2/A4 sketch (for cleaner layering + smaller
> blast radius):** the interpreter (`logs.Interpret`) returns **nil** when there is
> no `log` block and otherwise only resolves `log.dir` + carries the rotation
> overrides — it does **not** compute file paths, touch the data dir, or read env.
> So containers without a `log` block interpret to an *unchanged* `ContainerCall`
> (preserving existing exact-equality tests). All default-path computation, env/
> hardcoded defaulting, the on-disk layout, dir creation, and user-ownership now
> live in the node layer: `containerlog.Resolve(log, dataDirPath, containerID, name)`
> (with a guard: empty `dir` *and* empty `dataDirPath` ⇒ disabled) + the consumer
> in `containerCaller.openContainerLogFiles` (lumberjack writers, `unsudo.CreateDir`,
> `unsudo.EnsureOwnership` on close). `dataDirPath` is threaded via
> `newContainerCaller` (`core.go`). `model.ContainerLog` carries `Dir string` +
> nil-able rotation pointers (not pre-resolved paths). A6 also updated the existing
> node caller/loop tests to pass `""` for the new `dataDirPath` arg.
>
> **Adversarial verification** (4 static skeptics) passed on backward-compat,
> resolve-correctness, and ownership/fidelity/schema, and caught **one real bug**
> (now fixed): a fresh `*lumberjack.Logger` per call leaked lumberjack's per-writer
> background "mill" goroutine (its `Close()` doesn't reap it). Fix: the consumer
> **caches one rotating writer per log-file path** (`logWriters *sync.Map`, never
> closed) so goroutines/FDs are bounded by distinct log targets, not call count;
> this is why the default path is now **stable** (`<name>_<opHash>`, not per-run
> `<id8>`). Regression test asserts 3 calls ⇒ exactly 2 cached writers. Also added:
> negative-rotation-knob clamp, and the empty-`dataDirPath` guard test.

### A1. Schema (`opspec/opfile/jsonschema.json`) — ✅ done
- [x] Added the `log` object to `container.properties` (`additionalProperties:false`, **not** in `required`) with `dir` (expression `$ref`) + typed `enabled`/`maxSizeMB`/`maxBackups`/`maxAgeDays`/`compress`. Schema is valid JSON.
- [x] Decided against JSON-Schema `default:` keywords (gojsonschema doesn't inject them) — defaults documented in the field descriptions and live in `containerlog`.

### A2. Models — ✅ done
- [x] SCG `ContainerCallSpec.Log *ContainerLogSpec` (`model/opSpec.go`) with `Dir interface{}` (expression) + pointer rotation fields (`Enabled/MaxSizeMB/MaxBackups/MaxAgeDays/Compress`).
- [x] DCG `ContainerCall.Log *ContainerLog` (`model/call.go`) carrying the resolved custom `Dir string` (empty ⇒ default location) + the **nil-able** rotation overrides. *(Diverged from the original sketch: no pre-resolved `StdOutPath/StdErrPath`, no `Enabled bool` — file paths + defaulting moved to the node layer; see A3 + the banner.)*

### A3. Defaults + node-level config — ✅ done
- [x] New pkg `sdks/go/node/containerlog/`: default constants (`DefaultEnabled`, `MaxSizeMB=50`, `MaxBackups=5`, `MaxAgeDays=30`, `Compress`); `Resolve(log, dataDirPath, opPath, name) Config` overlaying **spec › `OPCTL_CONTAINER_LOG*` env › hardcoded**; and `DefaultDir(dataDirPath, opPath, name)` owning the on-disk layout. Negative knobs clamp to default; empty `dir` *and* `dataDirPath` ⇒ disabled. *(Signature/owner changed from the `Resolve(spec, dirPath)` sketch.)*
- [x] Forwarded the five `OPCTL_CONTAINER_LOG*` vars via `daemonEnvPassThroughVars` (`createNodeIfNotExists.go`).
- [ ] (Deferred) `--container-log*` `nodeConfig` flags — env is sufficient for v1.

### A4. Interpreter — ✅ done
- [x] New sub-package `…/container/logs/` (`interpret.go`, `pkg.go`, `suite_test.go`). `Interpret(scope, spec, scratchDir)` returns **nil** when there's no `log` block; otherwise resolves `log.dir` via `dir.Interpret` and carries the rotation overrides. *(Refined: it does **not** compute file paths, touch the data dir, or `CreateDir` — that moved to the node layer, which keeps no-`log` containers byte-identical and the interpreter env/data-dir-free.)*
- [x] Wired into `container.Interpret` (sets `containerCall.Log` from `logs.Interpret`).

### A5. Consume — tee to rotating files (`sdks/go/node/containerCaller.go`) — ✅ done
- [x] `interpretLogs` resolves config via `containerlog.Resolve` and tees each chunk into a per-stream `lumberjack.Logger` **after** `pubSub.Publish` — best-effort, so a file error never touches the event/console path. `dataDirPath` threaded via `newContainerCaller` (`core.go`).
- [x] **Writers are cached per log path (`logWriters *sync.Map`) and NOT closed per call** — this replaced the original "`Close()` after join" step, which leaked lumberjack's per-writer mill goroutine (caught in verification). Active files chowned to the user (`unsudo.EnsureOwnership`); dir created via `unsudo.CreateDir`.

### A6. Tests (Ginkgo/Gomega, arrange/act/assert) — ✅ done
- [x] `container/logs/interpret_test.go` (+ `suite_test.go`): nil spec → nil; overrides carried; `log.dir` resolved via a scope ref.
- [x] `container/interpret_test.go` left unchanged and still green (no-`log` containers interpret to `Log == nil` — backward-compat).
- [x] `containerlog_test.go` (+ `suite_test.go`): `DefaultDir` layout/stability/sanitize, custom `log.Dir`, precedence (spec › env › default), disable (spec/env/empty-dataDir), negative-clamp.
- [x] `containerCaller_test.go`: default-location files + exact bytes; custom `log.dir`; `enabled=false` ⇒ no files; on-by-default; **cache regression** (3 calls ⇒ exactly 2 writers).
- [x] No new fakes needed (reused `containerruntime/fakes`); updated the existing `node` caller/loop tests for the new `newContainerCaller(dataDirPath)` arg.

---

## Part B — `opctl events` filtering (use the already-durable store)

> **Status: ✅ implemented & green.** CLI-only change — the HTTP client already
> serializes `Filter.Since`/`Filter.Roots` into `?since=`/`?roots=`
> (`api/client/events_streams.go:24-31`) and the handler already parses them, so
> only the flags were missing.

### B1. CLI flags (`cli/cmd/events.go`) — ✅ done
- [x] Added `--since` (a duration like `90m`/`24h` relative to now, **or** an RFC3339 timestamp — via a `parseSince` helper) and `--roots` (`StringSliceVar`, comma-separated/repeated) that populate `model.GetEventStreamReq.Filter`, replacing the hardcoded empty filter.
- [ ] (Deferred) `--container <id>` — would need a new `ContainerID` field on `model.EventFilter` + `pubsub` filter logic. Not needed for v1.

### B2. Tests — ✅ done
- [x] Added `cli/cmd/events_test.go` (stdlib `testing`, matching the `cli/cmd` pattern): `TestParseSince` covers the duration, RFC3339, and invalid-input cases.

---

## Part C — Docs & CHANGELOG

> **Status: ✅ done.**

### C1. Docs — ✅ done
- [x] `docs/environment-variables.md`: added a **Container logging** subsection (the five `OPCTL_CONTAINER_LOG*` vars with values/defaults + file location) and added them to the daemon passthrough list.
- [x] `docs/opctl-usage.md`: documented `opctl events --since/--roots` (with examples) under `## opctl events`, and added a `logs/` entry to the data-dir section covering `node.log` + per-container logs + the opfile `container.log` block fields.

### C2. CHANGELOG.md (keepachangelog format) — ✅ done
- [x] `### Added` — container stdout/stderr persisted to rotating per-container log files (on by default), configurable via `container.log` + `OPCTL_CONTAINER_LOG*`.
- [x] `### Added` — `opctl events --since` / `--roots`.
- [x] Appended both to the existing top section (`## [0.1.78]` → `### Added`).

---

## Part D — Follow-up (OUT of scope for this PR; tracked here)

Event-store hygiene — **separate** change because it touches shared
state-reconstruction infra (`stateStore` replays events to rebuild op state;
`AuthAdded` events are live credential state):
- [ ] Collision-safe Badger key (append a monotonic sequence to the timestamp, preserving lexical/seek ordering used by `eventStore.List` + the `Since` seek).
- [ ] Selective retention/pruning of *completed-op* container stdout/stderr events (must not expire auth/lifecycle events or running-op state). Needs its own design + tests.

---

## Verification

- [x] `gofmt -l` clean on all touched files; `GOFLAGS=-tags=containers_image_openpgp go build ./...` succeeds; `go vet` clean on touched packages. *(Two `containerruntime/docker` vet nits are pre-existing in files this work didn't touch.)*
- [x] `go test … ./sdks/go/opspec/interpreter/call/container/... ./sdks/go/node/ ./sdks/go/node/containerlog/` pass (11 packages, `-count=1`). *(Events test package is Part B — not yet.)*
- [ ] Manual e2e: `make install VERSION=…` (⚠ restarts the daemon — do it **between** work sessions, not while live containers matter), run an op with a long-lived container, confirm `<dataDir>/logs/containers/<name>_<opHash>/stdout.log` fills, survives the op ending **and** a daemon restart, and rotates at the configured size. Confirm `enabled:false` (opfile or `OPCTL_CONTAINER_LOG=false`) suppresses files. Confirm files are owned by the invoking user (readable without sudo). **(Not run yet — needs a daemon restart.)**
- [ ] `opctl events --roots <rootCallID>` returns just that op's history after shutdown. **(Implemented + `parseSince` unit-tested; not yet exercised end-to-end against a live daemon.)**
- [x] Safety: no daemon kill/respawn during dev — all automated verification used static analysis + non-Docker unit tests.
