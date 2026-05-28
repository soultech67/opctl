# macOS Docker Desktop file sharing: hangs, tuning, and the VirtioFS decision

A field guide for the class of problem where `opctl run` (or `make bld`,
`make up`, etc.) hangs on macOS with no error, and Docker eventually has to be
restarted. Most of this is a Docker Desktop / macOS filesystem-layer issue, not
an opctl bug — but opctl is where you feel it, so the diagnostics and recovery
tooling live here.

## Symptom

An op stalls during container startup. The daemon log (see `make up` below)
shows a Docker call that never returns:

```
[opctl docker] ContainerCreate timed out after 20.000804333s (opctl_node_<id>):
  Post "http://%2Fvar%2Frun%2Fdocker.sock/.../containers/create?name=...":
  context deadline exceeded
```

`docker info` still responds. `docker ps` shows nothing running. But new
container creates hang for the full 20s opctl timeout (or forever, on an opctl
build without the timeouts).

## Root cause

Captured from a real `dockerd` goroutine dump (via `make docker-daemon-logs`)
while a hang was in progress:

```
syscall.Syscall6(0x4f, ...)              # 0x4f = fstatat on linux/arm64
syscall.Stat(...)
os.Stat({...bind-mount source path...})
daemon/volume/mounts.validateMountConfigImpl
daemon.validateHostConfig
daemon.(*Daemon).verifyContainerSettings
daemon.(*Daemon).containerCreate
```

dockerd's `ContainerCreate` validates every bind-mount source path with
`os.Stat()` before creating the container. On macOS those host paths route
through Docker Desktop's file-sharing bridge (gRPC-FUSE or VirtioFS) to the Mac
filesystem. When that bridge is slow or starved, the stat blocks **in the
kernel**, and the create goroutine is parked there. Multiple stuck creates
accumulate (the dump showed three, aged 1–4 minutes), depleting dockerd's
worker pool — which is why subsequent attempts also hang.

Two things make the bridge slow/starved:

1. **Large bind-mounted trees.** `node_modules/` is the classic offender —
   tens of thousands of files, each requiring a stat. opctl ops that mount a
   project root (e.g. `dirs: { /src: $(../../..) }`) drag the whole tree
   through the bridge.
2. **Competing host-side filesystem scanners.** Spotlight (`mdworker_shared`,
   `mdsync`, `mdbulkimport`) and Time Machine both stat-walk the same files.
   When they're busy on a path the bridge is also serving, they contend.

## Two DISTINCT problems (do not conflate)

| Problem | Cause | gRPC-FUSE | VirtioFS |
|---|---|---|---|
| Slow `os.Stat` on bind mounts (hangs) | bridge slow on big trees + scanner contention | slow | **fast** |
| Stale-inode after delete+recreate | VirtioFS FS-layer caching bug | not affected | **the bug itself** |

The historical reason this fork moved from VirtioFS → gRPC-FUSE was the
stale-inode bug: after a `rm -rf <dir>; cp -a <dir>` (e.g.
common-iac's `scripts/tests/run-tests.sh`), a container would read stale/old
file contents — surfacing as "bind source path does not exist" for a file that
was just recreated. That is a Docker Desktop VirtioFS correctness bug,
independent of opctl. **None of opctl's reliability fixes (per-call timeouts,
create-context detach, orphan reconcile) address it** — they prevent opctl from
leaking orphan containers; they do not change how the FS bridge caches inodes.

## Fixes, cheapest first

### 1. Exclude project trees from Spotlight (helps BOTH bridges)

`mdutil -i on|off` operates on **volumes, not directories** — `mdutil -i off
~/projects` fails with "invalid operation". To exclude a directory:

```sh
# Option A — GUI, takes effect immediately, no full reindex:
#   System Settings → Spotlight → Search Privacy → + → add ~/projects

# Option B — CLI marker file (skips the dir + everything beneath):
touch ~/projects/.metadata_never_index
sudo mdutil -E /System/Volumes/Data    # erase+rebuild index honoring the exclusion

# Option C — nuclear, whole volume (loses Cmd-Space file search):
sudo mdutil -i off /System/Volumes/Data
```

Note: adding a privacy exclusion triggers a one-time reindex burst (you'll see
`mdworker_shared` count spike in `make doctor`) while Spotlight purges the
excluded paths. It subsides.

Time Machine: if configured, exclude the same trees with
`sudo tmutil addexclusion ~/projects/...`, or schedule it to off-hours with
the third-party TimeMachineEditor (macOS has no native TM hour scheduling).
`make doctor` reports whether TM is even configured — on a machine with no TM
destination this is moot.

### 2. A/B test VirtioFS (the speed win — with a tripwire)

VirtioFS is substantially faster than gRPC-FUSE on stat-heavy workloads, which
directly targets the hang. The risk is the stale-inode bug. Docker Desktop
29.x has had many VirtioFS fixes since this fork last tried it, but **only a
test tells you if it's fixed in your version.**

Procedure:

1. Docker Desktop → Settings → General → file sharing implementation →
   **VirtioFS**. Apply & restart.
2. Run the workload that *specifically broke before* — the delete+recreate
   tripwire (common-iac `make test`, which does `rm -rf "${OPSPEC_DIR}"; cp -a`).
   Run it **several times back-to-back**.
3. Watch for the stale-inode symptom: a container reading old/wrong file
   contents after the delete+recreate, e.g. "bind source path does not exist"
   for a file that was just recreated, or a stale script body.
4. Also rerun `make bld` a few times — VirtioFS should make the big-tree stats
   fast and the `ContainerCreate` hang should not recur.

Decision rule:

- **Stale-inode does NOT recur** after several cycles → stay on VirtioFS; you
  get the speed.
- **Stale-inode recurs** → switch back to gRPC-FUSE. The real fix then becomes
  architectural (see #3).

### 3. Stop bind-mounting huge trees (the architectural fix)

Independent of file-sharing implementation: if an op bind-mounts a 40k-file
`node_modules/`, every `ContainerCreate` stat-walks it. The durable fix is to
not mount it from the host at all — install deps *inside* the container against
a named volume, so the host tree is never stat-walked by dockerd. This removes
the root cause for both bridges.

## Diagnostic + recovery tooling (Makefile targets)

All macOS Docker Desktop specific. See the root `Makefile` / `make.sh`.

| Target | What it does |
|---|---|
| `make doctor` | Read-only health check: orphan containers, `docker info` latency, Spotlight pressure (active `mdworker_shared` count), `node_modules` sizes, Time Machine status, installed-vs-HEAD opctl. Run this first. |
| `make up` | Run the daemon in the foreground with `OPCTL_DEBUG_DOCKER=1` so `[opctl docker]` / `[opctl kill]` timing logs stream to the terminal. |
| `make docker-logs` | Stream filtered Docker VM `init.log` + `docker events` to `./docker-logs/`. Run in a second terminal while reproducing. |
| `make docker-daemon-logs` | SIGUSR1 dockerd → retrieve its goroutine stack dump to `./docker-logs/`. Run *while a hang is in progress* — this is what captured the `syscall.fstatat` evidence above. |
| `make docker-restart` | Nuclear recovery: kill opctl daemon, quit + relaunch Docker Desktop, wait for the daemon. Clears stuck-in-kernel goroutines that won't recover on their own. |
| `make clean` | Remove cross-compiled binaries + orphaned `opctl.managed` containers (the invisible `Created`-state ones that block subsequent creates). |

### The full repro-and-capture recipe

Three terminals (after `make build && sudo make install`):

1. `make up` — opctl daemon stderr in real time
2. `make docker-logs` — Docker's view, to files
3. reproduce the hang (`make bld`, etc.)

When it hangs, in a fourth terminal run `make docker-daemon-logs` **while the
spinner is still spinning**. You then have: opctl's per-call timing (term 1),
Docker's API traffic (term 2 files), and dockerd's goroutine stacks (the dump
file) — enough to see which side is stuck and on what.

## What opctl itself does about this

opctl can't fix the FS bridge, but it no longer makes things worse:

- **Per-call Docker timeouts** (Ping 5s, inspect 10s, mutations 20s) — a wedged
  Docker surfaces an actionable error instead of an infinite spinner.
- **`ContainerCreate` runs on a detached context** — it is not cancelled when
  the parent op is cancelled, because dockerd can't abort an in-flight create
  anyway and cancelling mid-create leaves an invisible `Created`-state orphan.
- **Orphan reconcile** — if our create times out but dockerd finishes it
  afterward, we find the orphan by its `opctl.container-id` label and remove it.
- **Bounded deferred cleanup** + **kill-path instrumentation** — cleanup can't
  block the daemon forever, and the kill cascade is fully logged.

See `sdks/go/node/containerruntime/docker/{runContainer,timeouts,instrumentation}.go`.
