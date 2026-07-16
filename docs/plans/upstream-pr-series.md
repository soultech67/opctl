# Upstream PR Series Plan — soultech67/opctl → opctl/opctl (v2)

**Date:** 2026-07-16 (v2 — regrouped into functionality verticals per owner feedback)
**Status:** Verified plan (not yet executed).
**Scope:** Code contributions only. No AI-workflow artifacts, no fork branding, no private tooling, nothing from the top-level `docs/` folder (see [Exclusions](#7-exclusions)).

**What changed in v2:**
- **17 PRs → 8 PRs**, grouped by functionality vertical so each PR ships the **fork-final, fork-tested state** of its files. The v1 split required assembling intermediate variants that never existed as tested commits (e.g., a "pre-instrumentation" container-identity PR shipping Docker calls with known hang bugs that a later PR fixed, and daemon diagnostics that went to a dead pipe until the logging PR landed). Verticals eliminate that entirely — no known-buggy intermediate states are ever pushed upstream.
- **New hard invariant (owner mandate):** `README.md` is **never touched in any PR**. The fork's README rebrand — including removal of the owner's Support-Ukraine header — stays fork-local, permanently. That header is the upstream owner's statement and we do not interfere with it. `opctl_icon.png` likewise never ships. (Verified: no PR file list includes README.md or the icon.)

---

## 1. Context

| Fact | Value |
|---|---|
| Fork | `soultech67/opctl` — a true GitHub fork of `opctl/opctl` (cross-repo PRs work) |
| Merge-base (fork point) | `6360c502da77a1bbb903f80054138f62dc49499c` (**MB** below, 2026-03-30) |
| Fork ahead | 8 commits, 209 files, +26,972/−543 |
| Upstream ahead | 3 commits (proxy-env propagation) — **already vendored byte-identically into the fork**; no real divergence conflict |
| Upstream latest release | 0.1.76 (2026-06-18); fork's own 0.1.77–0.1.81 numbers collide and must never be copied |
| Upstream activity | Bursty; last merge 2026-06-23 by gedi-remitly (active merger). Outside PRs land but slowly |

**Already upstream — never include:** `sdks/go/node/containerruntime/proxyEnv.go`, `proxyEnv_test.go`, `cli/internal/nodeprovider/local/daemonEnv_test.go`, `k8s/constructPod.go` (byte-identical to upstream/main), plus the `daemonEnv()` extraction inside `createNodeIfNotExists.go`.

## 2. Hard invariants (every PR)

1. **`README.md` is untouched.** The Support-Ukraine header and everything else in upstream's README stays exactly as the owner has it. No icon, no badges, no header changes — ever.
2. No AI-workflow artifacts: `.serena/**`, `CLAUDE.md`, `RTK.md`, `AGENTS.md`, `.rtk/**`, `.graphify_python`, `graphify-out/**`, `.github/workflows/claude*.yml`.
3. Nothing from the top-level `docs/` folder (upstream has no `docs/` at all; `website/docs/**` is a different path and IS shipped where relevant).
4. No fork branding: no `soultech67` refs, no Lium AI, no `opctl_icon.png`, no fork CHANGELOG version entries, no private `astro` tooling, no Slack machinery.
5. Pre-post grep on every assembled branch: `grep -rE 'soultech67|astro|Lium|0\.1\.7[7-9]|Slack|graphify' <changed files>` — zero hits.

## 3. Upstream conventions (verified from upstream/main + gh)

- **CHANGELOG is release-driving and CI-enforced twice.** `check-for-changelog` hard-fails any PR that doesn't touch `CHANGELOG.md`; `lint-changelog` enforces keep-a-changelog format (H2 `## [x.y.z] - YYYY-MM-DD`; H3 only Added/Changed/Deprecated/Removed/Fixed). **Every push to main auto-cuts a release from the newest version entry.** No Unreleased section exists — each merged PR adds one fresh version heading and effectively schedules a release. First PR to land takes **0.1.77**; renumber at each rebase. Never port the fork's CHANGELOG hunks.
- **Commit style:** Conventional Commits (`feat(scope):` / `fix(scope):` / `docs:`), per the active merger. PRs are **rebase-merged** — every commit lands on main verbatim, so **each commit must be buildable and single-topic**. Precedent (#1187): feature commit(s) + a separate `docs: add CHANGELOG entry` commit. This matters more in v2: the big vertical PRs are structured as a few buildable, topical commits so maintainers can review commit-by-commit.
- **Code conventions:** Ginkgo arrange/act/assert with `objectUnderTest`, counterfeiter fakes regenerated via `opctl run generate`, tests beside source, gofmt-clean (CI gate).
- **No CLA, no DCO, no PR template.** Gates: one @opctl/maintainers approval + changelog checks + `opctl run compile` / `opctl run test`.
- **Fork-PR CI limitation:** `secrets.TEST_GITHUB_ACCESS_TOKEN` is withheld from fork PRs, so the CLI e2e leg fails on every fork PR today (until PR 8 lands). Expect maintainer shepherding for the e2e leg.
- **Coordination required with two open upstream PRs:**
  - **#1181** (kshuta-remitly, scutil-based darwin DNS registration) — overlaps PR 1. Comment/coordinate before opening.
  - **#1188** (gedi-remitly — the active merger, containerd/nerdctl backend) — PR 6 adds three methods to the public `ContainerRuntime` interface, which breaks #1188's implementation. Coordinate before opening; possibly offer the interface change as a shared base.
- **Size reality check:** typical merged outside-contributor PRs run 2–40 files, +17 to +700 lines. PRs 2 and 6 below exceed that by design (the owner's call: fully-tested verticals over small PRs shipping reassembled, untested intermediate states). Mitigate with commit-by-commit structure and an explicit offer in each big PR's body to split along the marked commit seams if the maintainer prefers.

## 4. Build & port strategy

- **Never cherry-pick the fork's 8 multi-topic commits.** Cut every branch fresh from upstream HEAD:
  ```sh
  git fetch upstream
  git checkout -b upstream-pr/<n>-<slug> upstream/main
  git checkout main -- <whole-file includes>       # most files: fork-final state
  git diff 6360c502d..main -- <path>               # 'adapt' files: hand-apply listed hunks
  ```
  v2 advantage: within a vertical there is no hunk-splitting — most files port wholesale at fork-final state. Hunk surgery remains only at the few cross-vertical seams listed per PR (`createNodeIfNotExists.go`, `root.go`, `node/node.go`, `create.go`, `core.go`, `urlTemplates.go`, `jsonschema.json` + model/interpret log-vs-volumes hunks, `cli_test.go`).
- **Regenerate counterfeiter fakes** with upstream's pinned generator (`opctl run generate`); don't copy the fork's regenerated fakes.
- **Commits per PR:** topical, buildable commits (see each PR's commit plan) + one `docs: add CHANGELOG entry for <topic>` commit.
- **Land strictly serially.** Every PR touches CHANGELOG.md and every merge cuts a release; rebase + renumber after each merge. PRs 1, 3, 4, 5 are mutually independent and can be *prepared* in parallel; open at most 2–3 at a time.
- **No fork sync needed first** (verified: `git diff upstream/main main` equals `MB..main` for every claimed file except `constructContainerConfig*.go`, handled in PR 6). After the series lands, sync fork main by merging upstream/main and reconciling CHANGELOG numbering.
- **Validation before posting (every PR):** `gofmt -d -l ./cli ./sdks/go` clean → `opctl run -a version=0.0.0 compile` → `opctl run test` → `opctl run changelog/lint` → targeted `go test ./<pkg>/...` → the invariants grep (§2.5).

### Land order and dependencies

```
PR 1  dns-network fixes            (independent; coordinate #1181)
PR 2  daemon reliability + durable logging + doctor   (independent)
PR 3  CLI UX bundle                (rebases root.go over PR 2)
PR 4  volumes                      (independent)
PR 5  auth                         (rebases urlTemplates over PR 2)
PR 6  container management + docker stability          (needs 1, 2, 4; coordinate #1188)
PR 7  container log persistence    (needs 2; textual rebase over 4)
PR 8  CI & test-harness restructure (independent; DISCUSS FIRST)
```

If PR 1 stalls on #1181 coordination, PR 6's only hard need from it is `test_helpers_test.go` — fallback: move `test_helpers_test.go` + `ensureNetworkExists_test.go` into PR 6 and proceed.

---

## 5. The series at a glance

| # | id | Title | ~LOC | Absorbs (v1) | Depends on |
|---|---|---|---|---|---|
| 1 | dns-network | fix(node): clean up leaked DNS resolver configs; self-heal macOS container networking | 190 | 4 | — |
| 2 | daemon-observability | fix/feat(node): daemon lifecycle fixes + durable rotating logs + `opctl doctor` | 1,900 | 3, 14, 15 | — |
| 3 | cli-ux | feat(cli): events filters, `ui --no-open`, no-progress hint, update hint | 815 | 5, 6, 7, 8 | 2 (root.go rebase) |
| 4 | volumes | feat(opspec): `volumes` property on container calls | 410 | 9 | — |
| 5 | auth | feat(auth): `opctl auth ls` / `opctl auth remove` + resolution bugfix | 1,500 | 10 | 2 (urlTemplates rebase) |
| 6 | container-mgmt | feat(containerruntime,cli): labels, readable names, `opctl container` CLI, Docker timeouts & reconciliation | 4,000 | 11, 12, 13 | 1, 2, 4 |
| 7 | container-logs | feat(node): persist container stdout/stderr to rotating logs (+ data-dir ownership fix) | 1,390 | 1, 16 | 2 |
| 8 | ci-restructure | ci(test): self-contained test op, gated e2e, nightly full run, fork-PR support | 350 | 2, 17 | — (discuss first) |

Total: 8 PRs, versions 0.1.77–0.1.84 as they land.

---

## 6. Per-PR detail

### PR 1 — fix(node): clean up leaked DNS resolver configs and self-heal macOS container networking

- **Branch:** `upstream-pr/1-dns-network` · **~190 LOC** · **Absorbs v1 #4 (unchanged)** · **Verification:** ✅ standalone
- **⚠️ Coordinate with open upstream PR #1181 (scutil DNS) before opening.**
- **Goal:** Leak-proof/idempotent DNS resolver-config cleanup (startup sweep + graceful-shutdown removal; darwin tolerates concurrent removals; linux stops rewriting `/etc/resolv.conf` when no opctl line is present) and self-healing macOS docker networking after unclean daemon exits (delete-before-add subnet routes, WireGuard address-in-use retry, wgUp mutex, IpcHandle panic recovery, dead darwin tun-teardown removed). Fixes the docker network tests on darwin dev machines.
- **Files:** `sdks/go/node/dns/internal/resolvercfg/delete_darwin.go`, `delete_linux.go`, `cli/cmd/node/create.go` (DNS hunks only), `docker/ensureNetworkDetached.go`, `docker/ensureNetworkAttached.go` (lifecycle fixes), `docker/ensureNetworkExists_test.go`, `docker/test_helpers_test.go`, `CHANGELOG.md`
- **Commit plan:** 1 fix commit + changelog commit.
- **Port notes:**
  - `create.go`: ONLY the two `dns.DeleteResolverCfgs` hunks (startup sweep after pidfile lock; best-effort cleanup after `eg.Wait()`) + needed imports. The SIGPIPE and slog-startup hunks belong to PR 2.
  - `ensureNetworkAttached.go`: keep wgUpMutex, IpcHandle panic-recovery, ConfigureDevice address-in-use retry, route delete-before-add. Replace the retry loop's `dockerInstrInfof` with plain `log.Printf("[opctl docker] ...")` and keep MB's one-arg `getContainerName` — PR 6 lands the instrumentation wrappers and 2-arg call. (The only file that gets a second pass later; unavoidable since the wrappers need PR 6's `instrumentation.go`.)
  - Neutralize war-story comments; in the IpcHandle recoverer comment remove the "rotating log" phrase (that infrastructure arrives in PR 2).
- **Verification notes (from v1 adversarial pass):** drop the "resetting mode to 0600" claim (`os.WriteFile` doesn't change an existing file's mode — real churn is mtime + content-identical rewrite through a symlinked resolv.conf). Cheap win: `delete_linux_test.go` already injects `etcResolvConfPath` — add a case asserting no rewrite when no opctl-managed line is present. Mention (or gate) that the darwin startup sweep flushes the host DNS cache every start.
- **CHANGELOG (Fixed):** stale `/etc/resolver/opctl_*` swept at startup, removed on graceful shutdown; linux `/etc/resolv.conf` untouched when no opctl line present; macOS idempotent routes, WireGuard retry + serialization, IpcHandle panic contained, dead tun-teardown removed.
- **Test plan:** `go test ./sdks/go/node/containerruntime/docker/` on **both** darwin (previously failed) and linux; manual on macOS: SIGKILL a node with containers running → next start sweeps resolver files and reclaims the route; fast kill/create loop exercises the retry.
- **Risks:** #1181 overlap; startup sweep deletes host-global resolver files under a per-data-dir lock; darwin WireGuard paths are CI-invisible (keep the manual darwin test plan prominent in the body).

### PR 2 — fix/feat(node): daemon lifecycle fixes, durable rotating logs, and `opctl doctor`

- **Branch:** `upstream-pr/2-daemon-observability` · **~1,900 LOC / ~24 files** · **Absorbs v1 #3 + #14 + #15** · **Verification:** ✅ all three components verified standalone; merging removes the v1 incoherence (v1 #3's diagnostics went to a dead pipe until #14's log file existed — here they land together)
- **Goal:** One vertical: *the daemon survives, its diagnostics survive, and you can operate it live.*
  1. **Lifecycle fixes:** daemon runs in its own session (Setpgid → **Setsid**) so closing the spawning terminal no longer kills it and every running op; SIGPIPE ignored; pubsub `Subscribe` failures surfaced instead of hanging kill-loops/parallel calls forever (upstream's own `// @TODO: handle err channel`); API-handler panics recovered per-request (500) instead of constructor-only; opt-in localhost pprof (`OPCTL_DEBUG_PPROF`).
  2. **Durable logging:** new `sdks/go/node/logging` — slog + lumberjack rotation to `<data-dir>/logs/node.log`, stderr tee, crash-output capture, stdlib-log redirect; `OPCTL_LOG*` env config forwarded to the daemon via the new `daemonEnvPassThroughVars` passlist; daemon stdout pointed at the log file; call-lifecycle slog events (started/ended with duration, Warn on error) that make the log useful; runtime control via new `GET/POST /logging` API + SDK `LogControlClient`.
  3. **`opctl doctor`:** `doctor logs [on|off]`, `doctor log-level [debug|info|warn|error]`, `doctor tail-logs [-n N]` (follows the rotating log via `tail -F`; path resolved from the running node with data-dir fallback). Never auto-spawns a node.
- **Files:** `cli/internal/nodeprovider/local/createNodeIfNotExists.go`, `cli/cmd/node/create.go`, `cli/cmd/node/node.go`, `cli/cmd/root.go` (doctor hunks only), `cli/cmd/doctor/*` (5), `sdks/go/node/api/listen.go`, `api/urltemplates/urlTemplates.go` (Logging constant), `api/handler/handler.go` + `_test`, `api/handler/logging/*` (4), `api/client/logging.go` + `_test`, `sdks/go/node/logging/*` (2), `sdks/go/model/logging*` (2), `sdks/go/node/core.go`, `caller.go`, `parallelCaller.go`, `pubsub/pubSub.go`, `go.mod`, `go.sum` (lumberjack only), `CHANGELOG.md`
- **Commit plan (each buildable):** ① `fix(node): daemon session/lifecycle + silent-failure surfacing` ② `feat(node): durable rotating daemon logs + /logging API` ③ `feat(cli): opctl doctor command group` ④ `docs: add CHANGELOG entry`.
- **Port notes (cross-vertical seams only — everything else ports fork-final):**
  - `createNodeIfNotExists.go`: Setsid hunk, `daemonEnvPassThroughVars` with the `OPCTL_DEBUG_PPROF` + four `OPCTL_LOG*` entries, the passthrough loop, and the `cmd.Stdout → logging.LogFilePath` redirect. **Strip:** `OPCTL_DEBUG_DOCKER`/`OPCTL_DOCKER_TIMEOUT_MULTIPLIER` entries (→ PR 6), five `OPCTL_CONTAINER_LOG*` entries (→ PR 7), all `docs/environment-variables.md` comment references.
  - `create.go`: SIGPIPE hunk + slog startup/API-listening/exit-reason hunks; rebases over PR 1's DNS hunks.
  - `core.go`: Subscribe-error hunk only. **Strip:** the `[opctl kill]` printf (→ PR 6) and the `dataDirPath` newContainerCaller arg (→ PR 7). Upstream's `parallelCaller_test.go`/`core_test.go` then need no edits (verified).
  - `caller.go`: panic-recovery slog + call-lifecycle slog hunks (both live here now). **Strip:** the auth-injection slog.Debug (dropped from the series).
  - `urlTemplates.go`: Logging constant only (Auths constants → PR 5). `node.go`: logging import + `logging.Init` gate for `cmd.Name()=="create"` only (SilenceUsage + container cmd → PR 6). `root.go`: doctor import + registration only.
  - `doctor/logs.go` caution: use the generic "suppresses all daemon logging" wording (the kill-path paper-trail it referenced arrives in PR 6) and keep `doctor_test.go`'s TestLogsOff assertion in sync. The `logging.go` package-doc mention of `opctl doctor` is now accurate — doctor ships in this PR.
  - `listen.go`, `parallelCaller.go`, `pubsub/pubSub.go`, logging/model/API-logging files, doctor package: whole diffs, fork-final.
- **Verification notes (from v1 pass):** soften the Setsid mechanism claim (Setpgid already isolated the process group; the confirmed silent-killers are SIGPIPE, EIO on dead pty, and session-scoped shell/terminal/CI-reaper kills; Setsid is strictly stronger and verified safe — daemon is signalled by PID only). First `log/slog` usage in the repo — flag it; fallback is stderr prints + error returns (~15 LOC) but now much less likely to be requested since the log file ships in the same PR. Correct risk list: upstream compiles darwin/linux CLIs only (no Windows target). `tail -F` shell-out is fine for those targets; offer pure-Go follow-up if asked.
- **CHANGELOG (Fixed):** daemon session isolation; SIGPIPE; Subscribe-failure surfacing; per-request panic recovery. **(Added):** durable rotating node logs + `OPCTL_LOG*` config + `GET/POST /logging`; `opctl doctor` group; `OPCTL_DEBUG_PPROF`.
- **Test plan:** `go test ./sdks/go/node/... ./cli/...` incl. the 9 doctor tests (green on fork) and logging/model/API suites; compile op; manual: long op + close terminal → daemon and op survive; `node.log` exists/rotates; `OPCTL_LOG_LEVEL=debug` passthrough; `doctor log-level debug` live; `doctor tail-logs` across a rotation; pprof curl.
- **Risks:** ~1,900 LOC is beyond upstream's typical outside-contributor PR — offer the ①/②/③ commit seams as split points in the body. New dep (lumberjack; unreaped mill goroutine — reviewers will probe). `logging.Init` gated on `cmd.Name()=="create"`. Naming bikeshed on `doctor`. openapi.yaml lacks `/logging` (offer follow-up).

### PR 3 — feat(cli): quality-of-life — events filters, `ui --no-open`, no-progress hint, update hint

- **Branch:** `upstream-pr/3-cli-ux` · **~815 LOC** · **Absorbs v1 #5 + #6 + #7 + #8** · **Verification:** ✅ all four components verified standalone; no shared files among them (verified — root.go only in update-hint, cli_test.go only in ui)
- **Goal:** Four independent CLI improvements in one reviewable UX vertical:
  1. `opctl events` filtered replay (wiring over pre-existing `model.EventFilter`): `--since`/`-t` accepts s/m/h/**d** units (e.g. `-t 3d`) or RFC3339; `--roots <ids>` scopes to root call IDs. **Default unchanged** — the entire durable history is replayed, exactly as upstream does today (owner decision: don't risk breaking event-stream consumers).
  2. `opctl ui --no-open` — print the web-UI URL instead of hijacking the browser (also stops the test suite spawning browser tabs).
  3. **No-progress hint** — after 2 min without events, `opctl run` probes daemon liveness (off the render loop, stale-safe) and prints INFO (responsive: quiet long step is normal) or WARNING (wedged) with remediation; adds `CliOutput.Info()`.
  4. **Update hint** — after successful commands, max once/24h, cached, silent-on-failure: "opctl X.Y.Z is available"; **suppressed for streaming/long-blocking commands** (`run`, `events`, `node create`, `doctor tail-logs` — via `updateHintSkippedCommandPaths` keyed on `CommandPath()`) where it interleaved with live output as noise; for all other commands it is the final line of output. The GitHub repo for self-update + hint becomes build-time configurable (`-ldflags -X`, default **opctl/opctl**), exposed as a `selfUpdateRepo` input on compile/release ops (also fixes GOFLAGS' inability to carry two `-X` flags).
- **Fork status (2026-07-16):** items 1 and 4's fixes are implemented and tested on fork main (CHANGELOG 0.1.82): events `-t` shorthand + day units (default untouched — full replay), and the streaming-command hint skip with a command-tree coverage test. Port fork-final state as usual.
- **Files:** `cli/cmd/events.go` + `_test`, `cli/cmd/ui.go`, `cli/cli_test.go` (ui-spec hunks only), `cli/cmd/run.go`, `cli/internal/clioutput/cliOutput.go` + `_test`, `cli/cmd/updateHint.go` + `_test`, `cli/cmd/selfUpdate.go`, `cli/cmd/root.go` (update-hint hunks only), `cli/.opspec/compile/op.yml`, `.opspec/compile/op.yml`, `.opspec/release/op.yml`, `CHANGELOG.md`
- **Commit plan:** ① `feat(cli): events --since/--roots` ② `feat(cli): ui --no-open` ③ `feat(cli): no-progress liveness hint` ④ `feat(cli): update hint + configurable self-update repo` ⑤ changelog.
- **Port notes:**
  - **De-fork-ification (exact, verified locations):** flip defaults to `opctl/opctl` at `root.go:29`, `.opspec/compile/op.yml:10`, `cli/.opspec/compile/op.yml:10`, `.opspec/release/op.yml:22`; replace the **13** `soultech67/opctl` fixture strings in `updateHint_test.go`; reword the "fork repo" assertion message. `.opspec/release/op.yml`: the +5-line `selfUpdateRepo` input is this PR's entire claim on the file — `to-github`/`to-ghcr`/`check.sh` stay fork-only.
  - `root.go`: var-block + `ExecuteContextC` + `maybePrintUpdateHint` only; rebases over PR 2's doctor hunks; the container-cmd hunk → PR 6. Strip the doctor import if porting from fork-final.
  - `cli_test.go`: ONLY the two ui-spec hunks (`--no-open`, Exit(1)→Exit(0)); strip the liveness hunks **and their `net/http`/`time` imports** (→ PR 8). Exit(0) rationale verified: upstream's unit op runs browser-less but the node API comes up (existing `auth add` spec proves it).
  - `run.go`: trim the fork-anecdotal war-story comment block; "restarting Docker" → "restarting the Docker daemon"; body must say Info() writes to **stdout**; reword the WARNING text (daemon failed a 5s probe after 2m silence — it didn't "not respond for 2m").
  - Do NOT touch `.opspec/test/op.yml` (PR 8 owns it and drops the ldflag entirely).
- **CHANGELOG (Added):** all four features, one bullet each (events default is unchanged — no Changed entry needed).
- **Test plan:** `go test ./cli/cmd/ -run 'TestParseSince|TestEventsCmdFlagDefaults|TestGetUpdateHint|TestMaybePrint|TestUpdateHint'` (single regex — multiple `-run` flags don't compose) + clioutput suite. Includes `TestUpdateHintCommandTreeCoverage` (walks the real `NewRootCmd()` tree — auth/container/doctor/ls/node/op/run/events/ui/self-update — asserting exactly which commands emit the hint) and `TestUpdateHintIsEmittedOnceAfterCommandOutput` (hint exactly once, as the final line, after command output). Verify the built binary still carries `-tags=containers_image_openpgp` after the GOFLAGS→args move; manual: each feature.
- **Risks:** the update-hint's outbound GitHub call is the likely debate magnet (mitigations code-verified: 24h cache, stderr-only, silent failure, disabled for dev builds/CI; offer an opt-out env var) — **if it stalls the bundle, split commit ④ into its own PR; the seam is clean.** ui-spec Exit(0) is environment-dependent — verify in first CI run, fall back to Exit(1) expectations.

### PR 4 — feat(opspec): volumes property on container calls for runtime-managed named volumes

- **Branch:** `upstream-pr/4-volumes` · **~410 LOC** · **Absorbs v1 #9 (unchanged)** · **Verification:** ✅ standalone
- **Goal:** Additive opfile `volumes` map (absolute container path → named volume) rendered as `mount.TypeVolume` by the docker runtime: storage that never crosses the host file-sharing layer (fixes Docker Desktop event-storm wedging for high-write workloads) and persists across runs (survives opctl's anonymous-only `RemoveVolumes` cleanup). Interpret-time validation of names/paths; k8s warns on the call's stderr; embedded delegates unchanged.
- **Files:** `sdks/go/opspec/interpreter/call/container/volumes/*` (4 new), `container/interpret.go` + `_test`, `sdks/go/opspec/opfile/unmarshal_test.go`, `opspec/opfile/jsonschema.json` (volumes block only), `sdks/go/model/call.go` + `opSpec.go` (Volumes fields only), `docker/constructHostConfig.go` + `_test`, `docker/runContainer.go` (Volumes arg) + `_test` (volumes hunks), `k8s/k8s.go` (warning only), `sdks/go/node/caller_test.go` + loop-caller tests (Volumes expectation lines only), `website/docs/reference/opspec/op-directory/op/call/container/index.md`, `CHANGELOG.md`
- **Commit plan:** 1 feature commit + changelog.
- **Port notes:**
  - The log-block siblings in `jsonschema.json`/`model`/`interpret.go` → PR 7; the `""` constructor-arg test lines → PR 7.
  - `container/interpret.go` splice (verified against upstream's exact tail): after `containerCall.Sockets, err = sockets.Interpret(...)` insert `if err != nil { return nil, err }`, then the volumes block, then `return containerCall, err`.
  - `runContainer_test.go`: insert the Volumes map after `Sockets:` in providedReq and the `mount.TypeVolume` entry between the dir and socket bind mounts (matches append order; keeps the order-sensitive Equal deterministic).
  - `k8s.go` warning stays strictly inside `k8s.go` (upstream's proxy commit touched `constructPod.go`). The `len(req.Volumes)>0` check is correct because `volumes.Interpret` returns a non-nil empty map (verified).
  - Drop the private AstroMind issue reference; motivation = Docker Desktop event-storm + persistence.
  - `website/docs/**` is not the excluded top-level `docs/` — the +10-line reference section ships here.
- **CHANGELOG (Added):** the `volumes` property (validated at interpret time; created on first use; k8s warns on stderr).
- **Risks:** k8s stderr warning departs from the silent-ignore precedent (isolated, easy to change); schema pattern admits relative paths (enforced at interpret time; mirrors dirs/files precedent).
- **Post-release fix folded in (fork 0.1.82, 2026-07-16):** the original commit tagged `Volumes` with `json:"volumes,omitempty"`, which made an empty map vanish in the event store's JSON round-trip — the same `CallStarted` event differed in shape live vs replayed, failing the serialLoopCaller spec race-dependently on loaded CI runners (2 consecutive nightly failures). Fixed by dropping `omitempty` (matching `dirs`/`files`/`sockets`), with a round-trip regression test in `sdks/go/model/call_test.go` — **add that file to this PR's file list** and port `model/call.go` at fork-final state (plain tag + explanatory comment).

### PR 5 — feat(auth): add `opctl auth ls` and `opctl auth remove`; fix credential resolution

- **Branch:** `upstream-pr/5-auth` · **~1,500 LOC** · **Absorbs v1 #10 (unchanged)** · **Verification:** ✅ standalone (core/node/fakes/handler wiring verified against upstream)
- **Goal:** Complete the credential lifecycle end-to-end: CLI → new `GET /auths/lists` + `POST /auths/removes` → `Node.ListAuths`/`RemoveAuth` → `AuthRemoved` event applied by the state store. Hardening: `auth add` validates RESOURCES, confirms, waits for durable apply. **Lead with the real bug fix:** a blank stored resources prefix `HasPrefix`-matches every ref (credentials leak to unrelated pulls) and badger key order made overlapping-prefix matches nondeterministic — `TryGetAuth` now prefers the longest matching prefix and ignores blank prefixes.
- **Files (~30):** `cli/cmd/auth/{auth,list,remove,add}.go`; `sdks/go/node/{node,listAuths(+test),removeAuth(+test),addAuth(+test),stateStore(+test)}.go`; `sdks/go/model/{req,event}.go`; `sdks/go/node/fakes/{node,core}.go` (regenerate); `api/client/auths_{lists,removes}.go`; `api/urltemplates/urlTemplates.go` (Auths constants; rebases over PR 2's Logging constant); `api/handler/handler.go` + `_test`; `api/handler/auths/**` (lists + removes, tests, fakes); `CHANGELOG.md`
- **Commit plan:** ① `fix(node): auth resolution longest-prefix + blank-prefix guard` ② `feat(auth): ls/remove end-to-end + add hardening` ③ changelog. (Bugfix first — it's the acceptance hook.)
- **Port notes:** `stateStore.go`: keep all functional changes, STRIP the slog.Debug auth-resolve diagnostics (fork observability; `resolveData.go` is dropped from the series entirely). EXCLUDE `test-suite/auth/*` + `test-suite/README.md` (fork plumbing; upstream keeps hardcoded `opctl/test-suite-auth` refs).
- **Verification notes (must fix before posting):**
  - **Add the two missing tests for the headline bugfix** (currently untested — reviewers told to "lead with the bugfix" will look): (1) `applyAuthAdded` with `Resources:""` → `TryGetAuth("docker.io/anything")` returns nil; (2) store `docker.io` + `docker.io/library` → the longer prefix wins. ~15 lines each with the existing `newTestStateStore` helper.
  - **Correct the reviewer note:** the adds handler ignores `AddAuth`'s return (pre-existing upstream behavior) and always writes 201 — the 5s-timeout error is *swallowed* for remote/CLI callers, not "surfaced as 500". The wait still delays the 201 (read-your-write holds). Optionally include the one-line handler error propagation.
  - Flag the RemoveAuth asymmetry proactively (fire-and-forget; `auth remove && auth ls` can transiently show the entry).
- **CHANGELOG (Added):** ls/remove + endpoints + SDK methods; add validates/confirms/waits. **(Fixed):** longest-prefix preference; blank prefixes never match.
- **Risks:** AddAuth semantic change (fire-and-forget → bounded wait) — negotiate; `api/openapi.yaml` not updated (offer follow-up).

### PR 6 — feat(containerruntime,cli): container identity, `opctl container` CLI, and Docker stability

- **Branch:** `upstream-pr/6-container-mgmt` · **~4,000 LOC / ~40 files** · **Absorbs v1 #11 + #12 + #13** · **Depends on 1, 2, 4** · **Verification:** component boundaries verified in v1 (incl. PR 12 by an actual simulated build); **merging eliminates v1's biggest defect** — the "pre-instrumentation" reassembly of the docker files that never existed as a tested commit and shipped known hang-bugs fixed only two PRs later. This PR ships the docker runtime files at **fork-final state — the exact code the fork has run and tested since June**.
- **⚠️ Coordinate with open upstream PR #1188 (containerd backend, by the active merger) before opening — the `ContainerRuntime` interface additions break its implementation.**
- **Goal:** The container-management vertical, whole:
  1. **Identity:** every container gets `opctl.*` labels (managed marker, container-id, container-name, image-ref) and a human-readable name (`opctl_<slug>_<8-char-id>` instead of `opctl_<uuid>`); cleanup resolves via the container-id label with legacy full-id-name fallback and tolerates removal-in-progress races. `ContainerRuntime` gains `DeleteContainer`/`DeleteContainersByLabels`/`ListContainersByLabels` (new `Container` value type) — docker implements, embedded delegates, k8s not-supported.
  2. **CLI:** `opctl container` group (also at `opctl node container`): `ls` (running-only default; `--all`/`--images`/`--verbose`/`--filter`), `down NAME`, `delete`/`rm --label` (shorthand keys), `prune`; node subcommands get SilenceUsage.
  3. **Stability:** per-call Docker timeouts (Ping 5s / inspect-list 10s / mutations 20s / cleanup 30s; `OPCTL_DOCKER_TIMEOUT_MULTIPLIER`), always-on kill-path/cleanup logging + `OPCTL_DEBUG_DOCKER` per-call timings with expected-no-op demotion, fail-fast Ping probes, bounded cleanup defer with stderr warning event (fixes CLI hanging on Ctrl+C), detached+bounded ContainerCreate with label-based orphan reconciliation, pull-path improvements (bounded pull-skip inspect; authenticated-vs-anonymous announcement).
- **Files:** `containerruntime/containerRuntime.go`, `fakes/containerRuntime.go` (regenerate), `docker/{containerLabels, getContainerName(+test), deleteContainer(+test), delete_test, deleteContainerIfExists(+test), docker, constructContainerConfig(+test), runContainer(+test), reconcileContainerCreate_test, instrumentation(+test), timeouts, pullImage(+test), ensureNetworkExists, ensureNetworkAttached (wrappers + 2-arg name), isGpuSupported}.go`, `embedded/embedded.go`, `k8s/k8s.go` (stubs), `cli/cmd/node/container*` (8), `cli/cmd/node/node.go` (SilenceUsage + AddCommand), `cli/cmd/root.go` (container hunk), `sdks/go/node/{callKiller,killOp}.go`, `core.go` (kill-log hunk), `createNodeIfNotExists.go` (2 env entries), `CHANGELOG.md`
- **Commit plan (each buildable; these are also the offered split seams):** ① `feat(containerruntime): opctl labels, readable names, list/delete methods + Docker timeouts/instrumentation/reconciliation` — the runtime at fork-final state ② `feat(cli): opctl container command group` ③ changelog. (Runtime and CLI could be two PRs if a maintainer asks; the stability work is inseparable from identity at fork-final state and shouldn't be split again.)
- **Port notes:**
  - **No more 906c-shaped assembly** — port the docker package at fork-final state. `constructContainerConfig.go`/`_test.go` remain the only textual conflict with upstream HEAD (proxy-env): add the `labels` param + `Labels:` field to upstream's version and update upstream's proxy test to the 6-arg call.
  - The 6-arg `constructHostConfig` call with `req.Volumes` is now valid as-is (PR 4 landed — the v1 blocker disappears).
  - `test_helpers_test.go` arrives via PR 1 (fallback: pull it in here if PR 1 stalls).
  - `ensureNetworkAttached.go`: wrapper hunks + swap PR 1's inlined `log.Printf` back to `dockerInstrInfof` + the 2-arg `getContainerName` flip — one coherent rebase over PR 1.
  - `createNodeIfNotExists.go`: append `OPCTL_DEBUG_DOCKER` + `OPCTL_DOCKER_TIMEOUT_MULTIPLIER` to the PR-2 passlist (mechanism exists; closes the v1 functional gap where these vars never reached the daemon).
  - `core.go`: the `[opctl kill] CallKillRequested` log hunk only (Subscribe landed in PR 2, dataDirPath → PR 7). Kill-path logging style: **convert the `log.Printf("[opctl kill] ...")` lines to slog** — PR 2 established slog + the log file, so consistency now favors slog (was an open question in v1; resolved by the regroup).
  - `k8s.go`: the three not-supported stubs only (volumes warning landed in PR 4).
  - `runContainer_test.go`: fork-final (its `configureNetworkInspect` calls resolve via PR 1's helper; the ctx-assertion relaxations and volumes hunks are all internal now).
  - **Genericize fork fixtures:** `astro-local-localstack` (container.go Example + 13 in container_test.go), `opctl_lium-web_ccc`/`lium-web` (container_ls_test.go), `com.example.owner=scott` (container_test.go:381,389). War stories out; "restart Docker Desktop" → "restart the Docker daemon" (3 sites). Recovery messages referencing `opctl container prune` are now accurate — the command ships in the same PR (another v1 incoherence resolved).
  - `instrumentation_test.go`/`reconcileContainerCreate_test.go` are stdlib `testing` in a Ginkgo package — convert or explicitly flag, decide before posting.
  - Optional: one `delete_test.go` assertion for Container.State/Status passthrough (only untested new surface). Soften the description's label-scoping claim (empty filters fall back to legacy name+network filters).
- **CHANGELOG (Added):** labels + readable names; ContainerRuntime list/delete methods; `opctl container` group; `OPCTL_DEBUG_DOCKER`; pull announcements. **(Fixed):** per-call Docker timeouts (no more hung Ctrl+C); Created-orphan reconciliation; removal-race tolerance; kill-path cleanup errors logged. **(Changed):** `opctl node` subcommands no longer print usage on runtime errors.
- **Test plan:** `go test ./sdks/go/node/containerruntime/... ./cli/cmd/node/` on linux AND darwin; `opctl run generate` (fakes); compile + full `opctl run test`; manual: run op → `docker ps` shows `opctl_<slug>_<shortid>` + labels; legacy-container cleanup fallback; SIGSTOP dockerd mid-op → fast failure + bounded Ctrl+C with stderr warning; kill a create mid-flight → orphan reconciled; exercise ls/down/delete/prune incl. interactive paths.
- **Risks:** ~4,000 LOC is far beyond upstream's typical outside PR — the body must offer the ①/② seam and sell the vertical rationale (this is the fork's flagship, tested as a unit). Public interface addition breaks external implementers incl. #1188 (coordinate!). Name-shape change breaks scripts matching `opctl_<uuid>` (cleanup compat handled). Behavior change: detached create means Ctrl+C can take up to 20s mid-create; `RunContainer` returns `(nil, nil)` on parent-ctx cancel post-create (verified upstream callers nil-check). String-matching timeout classification (deliberate; will draw pushback). N+1 ContainerInspect in ls.

### PR 7 — feat(node): persist container stdout/stderr to rotating log files (+ data-dir ownership fix)

- **Branch:** `upstream-pr/7-container-logs` · **~1,390 LOC** · **Absorbs v1 #16 + #1 (unsudo)** · **Depends on 2** · **Verification:** ✅ both components verified; Log hunks verified fully independent of PR 4's Volumes hunks in all four shared files
- **Goal:** Each container call's stdout/stderr is additionally written — best-effort, never affecting the op or event stream — to rotating files at a run-stable path `<data-dir>/logs/containers/<name>_<opHash>/{stdout,stderr}.log` (on by default; configurable per container via a new opfile `container.log` block layered over `OPCTL_CONTAINER_LOG*` env defaults). Because the daemon runs as root, log files are chowned to the sudo-invoking user via new `unsudo.EnsureOwnership` — which also fixes a standing bug: `unsudo.CreateDir` only chowns dirs it newly creates, so a once-sudo'd data dir stays root-owned forever; `datadir.New` now repairs ownership at startup.
- **Files:** `sdks/go/internal/unsudo/ensureOwnership.go` + `_test`, `sdks/go/node/datadir/datadir.go`, `sdks/go/node/containerlog/*` (3), `sdks/go/opspec/interpreter/call/container/logs/*` (4), `container/interpret.go` (log hunk), `sdks/go/model/call.go` + `opSpec.go` (Log fields), `opspec/opfile/jsonschema.json` (log block), `sdks/go/node/containerCaller.go` + `_test`, `core.go` (dataDirPath arg), caller-suite tests (4, `""` args), `createNodeIfNotExists.go` (5 env entries), `CHANGELOG.md`
- **Commit plan:** ① `fix(node): repair data-dir ownership left root-owned by sudo'd runs` ② `feat(node): container stdout/stderr persistence` ③ changelog.
- **Port notes:**
  - Lumberjack + `node/logging` + the passthrough mechanism arrive via PR 2; `EnsureOwnership` ships here (commit ①).
  - `container/interpret.go` (order-explicit): logs import → `containerCall.Log, err = logs.Interpret(...)` → `return containerCall, err`; rebases over PR 4's volumes block. `jsonschema.json`: log block on top of PR 4's volumes block — **reword its description** to be self-contained ("Omit to use defaults (on). Node-level defaults via OPCTL_CONTAINER_LOG* env vars on the node machine.") — no `docs/environment-variables.md` references. Same fix in `createNodeIfNotExists.go`'s comment ("See sdks/go/node/containerlog.").
  - `createNodeIfNotExists.go`: append the five `OPCTL_CONTAINER_LOG*` entries to the passlist as PR 2 landed it.
  - Caller tests: only the `""` constructor-arg insertions (Volumes lines landed in PR 4).
  - Add `//go:build !windows` to `ensureOwnership_test.go` (`syscall.Stat_t`; upstream precedent exists).
  - **Either add** a log-block round-trip case to `opfile/unmarshal_test.go` (and list the file) **or drop** the "schema round-trip" claim — the fork's unmarshal additions are volumes-only (verified).
  - Add save/restore (or DeferCleanup) around `os.Unsetenv("OPCTL_CONTAINER_LOG")` in the two containerCaller specs.
  - Body wording: EnsureOwnership chowns the *active* files only, not rotated backups.
- **CHANGELOG (Added):** container output persistence + `container.log` block + `OPCTL_CONTAINER_LOG*`. **(Fixed):** data-dir ownership repair for sudo'd runs.
- **Test plan:** `go test ./sdks/go/internal/unsudo/ ./sdks/go/node/datadir/ ./sdks/go/node/containerlog/ ./sdks/go/opspec/... ./sdks/go/node/`; manual: run an op, `tail -F` the container log across two runs (stable path), verify sudo-user ownership and the opfile overrides; sudo-run a node → entries chowned back on next start.
- **Risks:** default-on persistence has disk-usage + sensitive-output implications — decide and defend default-on vs opt-in in the body (one-constant change; be explicit about willingness to flip). `datadir.New` now hard-fails on the first Lchown EPERM for non-root SDK consumers with foreign-owned entries (previously constructed; that state was already unusable) — reviewers may ask to gate on `SUDO_UID`. WalkDir+Lchown every start. Website reference docs for the block not included (fork documented it in the excluded `docs/` folder) — offer follow-up.

### PR 8 — ci(test): self-contained test op, cli integration op, gated dind e2e with nightly full run, fork-PR support

- **Branch:** `upstream-pr/8-ci-restructure` · **~350 LOC** · **Absorbs v1 #17 + #2 (harness)** · **Verification:** ✅ (with mandatory pre-post re-run) · **⚠️ DISCUSS FIRST — changes CI policy. Open an issue / Slack thread before cutting the branch. If rejected, the cmd.sh harness improvement still stands alone as a tiny PR.**
- **Goal:** (1) Fork PRs can't pass CI today (secret withheld → e2e minLength fails) — skip only the e2e with a `::notice::`, run everything else. (2) `opctl run test` isn't self-contained on a fresh checkout (go:embed artifacts missing; e2e can silently test a stale binary) — add a generate phase + build the e2e CLI from the branch under test. (3) Node-dependent `TestCli` specs ran in the runtime-less unit op — new cli integration op with a real docker socket + liveness-gated Skips. (4) The ~30-min dind conformance e2e is too slow/flaky as a PR gate — gate behind `runCliE2e`; PR gate runs the fast auth subset; full suite moves to a nightly scheduled run. Split Build/Test into parallel jobs. Plus the e2e harness improvement: echo each scenario's combined output (currently discarded — failing scenarios are invisible in CI logs) and assert on an explicitly captured exit code.
- **⚠️ Harness framing (verified):** the original "errexit kills expect:failure scenarios" theory was **empirically disproven** — `cmd.sh` is exec'd via its shebang as a child shell, so upstream's harness already evaluates those assertions correctly (verified in the exact `docker:27.3.1-dind` image). Frame the cmd.sh change as robustness/log-visibility, never as a bug fix; rewrite the in-script comment that asserts the false claim.
- **Files:** `.github/workflows/build.yml`, `.github/workflows/nightly-cli-e2e.yml`, `.opspec/test/op.yml`, `cli/.opspec/test/e2e/op.yml`, `cli/.opspec/test/e2e/cmd.sh`, `cli/.opspec/test/integration/op.yml`, `cli/cli_test.go` (liveness hunks), `CHANGELOG.md`
- **Commit plan:** ① `test(e2e): surface scenario output, capture exit codes explicitly` ② `ci(test): restructure` ③ changelog.
- **Port notes (heaviest hand-adaptation in the series — run the §2.5 grep plus `OPCTL_BOOTSTRAP_RELEASE_REPO|githubAuthTestOpRef|selfUpdateRepo|SLACK|make test`; zero hits expected):**
  - `cmd.sh`: keep `set +e` / `rc=$?` / `set -e`, combined `2>&1` echo, explicit `[ "$rc" -eq 0 ]` assertions; **strip the 3-line `__githubAuthTestOpRef__` substitution block** (fork-only; nothing upstream provides that variable — internal to this PR now, no cross-PR coordination needed).
  - `build.yml`: keep job split, IS_FORK_PR gating, `-a runCliE2e`. STRIP: `OPCTL_BOOTSTRAP_RELEASE_REPO` + templated install URLs (restore `github.com/opctl/opctl/releases/latest/...`), `-a githubAuthTestOpRef`, relmeta + Slack steps. Update the Test-step comment that still says "Full suite".
  - `nightly-cli-e2e.yml`: keep schedule + workflow_dispatch + 120-min timeout + full-suite invocation; strip Slack/continue-on-error/reflect-status scaffolding + fork env refs.
  - `.opspec/test/op.yml`: keep `runCliE2e`/`cliE2eFull`, `githubAccessToken` default `""`, generate phase, integration invocation, in-op linux CLI build, testsDir-scoped e2e. STRIP `githubAuthTestOpRef` + the `-X selfUpdateRepo` ldflag (drop entirely; PR 3's root.go default suffices and version=0.0.0 disables the hint in tests); reword `make test` references; **restore the root op's githubAccessToken description** to reference `github.com/opctl/test-suite-auth`.
  - `cli/.opspec/test/e2e/op.yml`: keep the `testsDir` input; strip `githubAuthTestOpRef`; restore the token description.
  - `cli_test.go`: the imports (`net/http`, `time`), `nodeAvailable`, `waitForNodeLiveness`, BeforeSuite assignment, Context-level BeforeEach Skip — rebases over PR 3's ui hunks.
  - Call out that the root test op's dockerSocket is satisfied by the committed `.opspec/args.yml`, so `opctl run test` must run from the repo root (verified empirically).
- **Verification note (blocking-before-post):** the assembled combination was never executed anywhere (fork CI ran with fork scenarios.json and placeholder plumbing). Mandatory: `opctl run -a runCliE2e=true -a githubAccessToken=<PAT> test` from repo root on the assembled branch, plus tokenless `opctl run test` on a fresh clone.
- **CHANGELOG (Fixed):** CI runs for fork PRs (e2e skipped with a notice). **(Changed):** self-contained test op; gated e2e; PR gate = auth subset; nightly full conformance; new cli integration op; harness output surfacing.
- **Risks:** policy change a maintainer could reject — get buy-in first. The two workflow files must land together (build.yml passes `runCliE2e=true`). `waitForNodeLiveness` hardcodes `127.0.0.1:42224`. Nightly dind runs will produce red runs needing triage. Job split doubles runner usage per PR (wall time drops). `schedule:` only activates on the default branch and needs the secret available to scheduled runs.

---

## 7. Exclusions

**Hard-excluded by mandate:** `.serena/**` (7), `CLAUDE.md`, `RTK.md`, `AGENTS.md`, `.rtk/filters.toml`, `.graphify_python`, `graphify-out/**` (12), `.github/workflows/claude*.yml` (2), the entire top-level `docs/` folder (5). Comments/descriptions referencing `docs/environment-variables.md` are rewritten in PRs 2/6/7.

**README.md — owner mandate (see §2.1):** never touched in any PR. The fork's README rebrand (Support-Ukraine header removal, soultech67 badges, Lium AI section, `opctl_icon.png`) is fork-local, permanently. The upstream owner's Support-Ukraine header stays exactly as they wrote it.

**Already upstream:** `proxyEnv.go`, `proxyEnv_test.go`, `daemonEnv_test.go`, `k8s/constructPod.go` (byte-identical), plus the `daemonEnv()` extraction hunks.

**Fork branding:** soultech67 URL rewrites in api/cli/webapp/website/sdks-js/opspec READMEs, `sdks/js/package.json`, `.github/ISSUE_TEMPLATE/config.yml`, website config/sidebars/setup (incl. `ghcr.io/soultech67` refs), `test-suite/README.md`, `test-suite/auth/default/op.yml` + the `__githubAuthTestOpRef__` scenarios.json placeholder.

**Private fork tooling & release plumbing:** `Makefile` + `make.sh` (private `astro` PAT minting, soultech67 defaults, macOS debug targets, install/uninstall backups — no upstream home), `.gitignore` additions, `.opspec/release/check.sh` + `to-ghcr` + `to-github` repointing, all Slack machinery. Exception: `.opspec/release/op.yml`'s +5-line `selfUpdateRepo` input ships in PR 3 with the default flipped to `opctl/opctl`.

**Fork CHANGELOG content:** all 0.1.77–0.1.81 entries — replaced by one fresh sequential-version entry per PR.

**Deliberately dropped code hunks:** `resolveData.go` slog.Debug auth diagnostics, stateStore.go slog.Debug lines, caller.go auth-injection slog.Debug — fork observability noise; easy to re-add on request. The e2e `-X selfUpdateRepo` ldflag is dropped (not re-pointed) in PR 8.

## 8. Open questions

1. **#1181 coordination (PR 1):** comment on the open scutil-DNS PR / Slack first, or open and offer to rebase?
2. **#1188 coordination (PR 6):** the interface change breaks the active merger's own containerd PR — coordinate proactively or offer the interface change as a shared base?
3. **PR 2 / PR 6 size:** both exceed anything an outside contributor has landed upstream. The bodies offer commit-seam splits — accept a maintainer's request to split, or hold the vertical line?
4. **Container log persistence default (PR 7):** on-by-default (fork behavior) or opt-in for upstream? One-constant change.
5. **AddAuth durability wait (PR 5):** keep the SDK-level 5s read-your-write, or handler-side only? Should RemoveAuth get the same wait?
6. **pprof gate (PR 2):** keep `OPCTL_DEBUG_PPROF`, or strip to keep the lifecycle commit a pure bugfix? (Self-contained either way.)
7. **CI policy (PR 8):** is upstream willing to reduce the PR gate to the auth-subset e2e + nightly full run? Issue/Slack first; the cmd.sh harness improvement survives a rejection as its own tiny PR.
8. **ui spec expectation (PR 3):** flip to `--no-open`/Exit(0) immediately (verified rationale says yes) or keep Exit(1) until CI proves it?
9. **Pull announcement (PR 6):** "pulling X anonymously" fires on every unauthenticated pull — keep (drives auth-add discovery) or drop as chatty?
10. **AI attribution:** draft bodies end with the Claude Code footer; upstream has precedent for `Co-Authored-By: Claude` trailers (#1187). Keep, or strip all AI attribution from upstream-facing text?
11. **Fork CHANGELOG hygiene after landing:** upstream will mint its own 0.1.77+ (colliding with the fork's historical numbers) — how should the fork renumber/annotate when syncing back?

## 9. How this plan was produced & verified

- **v1:** 12 parallel analysis agents classified every hunk of the 209-file `MB..main` diff (include/adapt/exclude per file, diffed against both MB and upstream/main); 1 agent established upstream conventions; a completeness critic re-walked all 209 files (31 hard-excluded, 10 orphans re-homed, 11 cross-cluster claims resolved); a synthesis pass produced a 17-PR series; a mechanical check confirmed zero excluded-artifact paths in any PR file list; 17 adversarial verification agents checked each PR for standalone compilability against upstream HEAD, symbol-level dependency closure, leftover fork strings, test reality, and conflict surfaces (one by actually building the assembled tree). Three blockers were found and fixed: the e2e-harness bug premise was empirically disproven in the `docker:27.3.1-dind` image; two PRs had missing dependency edges.
- **v2 (this document):** regrouped the 17 verified units into 8 functionality verticals per owner feedback. Every merge combines units whose boundaries were verified in v1; merging removes inter-PR seams (it cannot add new ones), and the remaining cross-PR seams (PR 6 ← 1/2/4, PR 7 ← 2, PR 3/5 ← 2 textual) are exactly the v1-verified boundaries. The regroup resolves three v1 defects outright: the never-tested "pre-instrumentation" docker assembly, diagnostics-to-dead-pipe before the log file existed, and recovery messages referencing a CLI command that didn't exist yet. Added the README.md invariant (owner mandate).
