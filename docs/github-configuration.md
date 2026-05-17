# GitHub configuration for the `soultech67/opctl` fork

This doc captures everything that needs to be set up — on GitHub *and* in this
repo's opspec files — for the CI defined in `.github/workflows/build.yml` to
build, test, and release `opctl` cleanly through the fork at
[github.com/soultech67/opctl](https://github.com/soultech67/opctl) instead of
the upstream `opctl/opctl`.

> **TL;DR**: The workflow itself is owner-agnostic (it uses `${{ github.token }}`
> and `${{ github.actor }}`), but several of the opspec files it invokes have
> the upstream `opctl/opctl` and `ghcr.io/opctl/opctl` paths **hardcoded**.
> Those need to be edited (see [Required opspec edits](#required-opspec-edits)
> below). Yes — `.opspec/release/op.yml` does not need to change directly, but
> its sub-ops (`release/to-ghcr/op.yml`, `release/to-github/op.yml`,
> `release/check.sh`) do, and you should pass `-a
> selfUpdateRepo=soultech67/opctl` to `compile`.

## What the CI workflow actually does

`.github/workflows/build.yml` defines four jobs, all of which bootstrap by
`curl`ing the **upstream** opctl release tarball
(`https://github.com/opctl/opctl/releases/latest/...`) just to get a working
`opctl` binary, then drive everything through `opctl run`:

| Job | Trigger | What it runs | What it needs |
| --- | --- | --- | --- |
| `check-for-changelog` | PRs against `main` | `opctl run changelog/find-in-diff` | Just checkout + opctl binary. |
| `lint-changelog` | every push/PR | `opctl run changelog/lint` | Just checkout + opctl binary. |
| `build` | PRs not on `main` | `opctl run -a version=0.0.0 compile` then `opctl run -a githubAccessToken=$TEST_GITHUB_ACCESS_TOKEN test` | Docker on the runner (already there on `ubuntu-latest`); `TEST_GITHUB_ACCESS_TOKEN` secret for the auth test-suite. |
| `create-release` | push to `main` | `opctl run -a github='{"username":"${{ github.actor }}","accessToken":"${{ github.token }}"}' release` | Workflow `GITHUB_TOKEN` with `contents: write` (for the GitHub Release) and `packages: write` (for `ghcr.io`). |

`${{ github.token }}` is scoped to the repo the workflow runs in, so on the
fork it authenticates as `soultech67/opctl`. The trouble is what the `release`
op then *does* with that token.

## What the `release` op actually does

`.opspec/release/op.yml` runs (in order):

1. `../changelog/getLatestRelease` — local, no remote refs.
2. `github.com/opspec-pkgs/base64.encode#1.1.0` — public op-pkg; works the same
   from any fork.
3. A short shell container (`release/check.sh`) that does
   `docker pull ghcr.io/opctl/opctl:${version}-dind` to decide whether the
   version was already released. **This path is hardcoded to the upstream
   image** — on the fork it will either get denied (private fork-only image),
   pull *upstream*'s image and conclude "already published", or hit
   not-found and proceed; none of those are what you want.
4. If not already published:
   - `../compile` — builds the four-platform binaries. `compile` accepts
     `selfUpdateRepo` (default `opctl/opctl`) which is baked into the binary
     via `-ldflags -X=github.com/opctl/opctl/cli/cmd.selfUpdateRepo=…` (see
     `cli/.opspec/compile/op.yml:43` and `cli/cmd/root.go:28`). Leaving this
     at the default means `opctl self-update` will pull releases from
     **upstream** rather than from your fork.
   - `./to-ghcr` — pushes `ghcr.io/opctl/opctl:<version>-{dind,dood}` —
     **hardcoded path**.
   - `./to-github` — calls `github.com/opspec-pkgs/github.release.create#3.0.0`
     and `github.com/opspec-pkgs/github.release.upload#2.0.0` with
     `owner: opctl, repo: opctl` — **hardcoded owner/repo**.

So although `op.yml` looks owner-agnostic, the sub-ops it invokes will keep
trying to publish to upstream paths regardless of which fork the workflow runs
in.

## Required opspec edits

These edits are necessary for releases to flow through the fork. None of them
are required just to run `compile` or `test` from CI — only `release`.

### 1. `.opspec/release/to-github/op.yml`

Change the two `owner`/`repo` pairs from `opctl`/`opctl` to your fork:

```yaml
        owner: soultech67
        repo: opctl
```

(both inside the `github.release.create` step and the `github.release.upload`
step).

### 2. `.opspec/release/to-ghcr/op.yml`

Change the image reference being pushed:

```yaml
--output type=image,name=ghcr.io/soultech67/opctl:$(version)-$(imageVariant),push=true
```

(line 33).

### 3. `.opspec/release/check.sh`

Change the existence-check pull to match the new image path:

```sh
error=$(docker pull ghcr.io/soultech67/opctl:${version}-dind 2>&1 1>/dev/null)
```

(line 3).

### 4. (Optional but recommended) `selfUpdateRepo` for compiled binaries

The release workflow currently invokes `compile` indirectly via `release`,
which calls `../compile` with **no** `selfUpdateRepo` input, so the binary
falls back to the default `opctl/opctl`. If you want
`opctl self-update` on installed fork binaries to pull from your fork's
releases, override the default in **`.opspec/compile/op.yml`**:

```yaml
  selfUpdateRepo:
    string:
      default: soultech67/opctl
      description: GitHub owner/repo used by self-update to find releases.
```

…or wire `selfUpdateRepo` through `.opspec/release/op.yml` into the
`../compile` call. The plumbing already exists end-to-end (release op →
`.opspec/compile/op.yml` → `cli/.opspec/compile/op.yml` → `-ldflags`); only the
value is missing.

### 5. (Optional) `.opspec/release/to-ghcr/op.yml` description

The `description:` field on line 2 is for humans only; consider updating it to
point at `ghcr.io/soultech67/opctl/`.

### Does `.opspec/release/op.yml` itself need changes?

**No.** The release op delegates to `../compile`, `./to-ghcr`, and `./to-github`
without referencing any owner/repo strings. All the owner-specific values live
in the files listed in §1–§4 above. If you ever want
`selfUpdateRepo` to be a release-time knob instead of a global default, you'd
add it as an input on `release/op.yml` and pass it through to `../compile`.

## What needs to be set up in the `soultech67/opctl` GitHub repo

These are all done in the **fork's** GitHub UI / settings — none of them are
code changes.

### A. Enable Actions on the fork

By default, forks have GitHub Actions disabled. Open
`https://github.com/soultech67/opctl/actions` and click **"I understand my
workflows, go ahead and enable them"**. Until this is done none of the four
jobs will run.

### B. Grant the workflow `GITHUB_TOKEN` the right scopes

`create-release` in `.github/workflows/build.yml` passes `${{ github.token }}`
into the `release` op, which uses it for **two** things: creating a GitHub
Release on the fork repo, and pushing the container images to
`ghcr.io/soultech67/opctl`. Both require permissions the default fork
`GITHUB_TOKEN` does not have.

There are two ways to grant them — pick one:

**Option 1 — repo-wide default (simplest):**

Settings → Actions → General → **Workflow permissions**:

- Select **"Read and write permissions"**.
- Check **"Allow GitHub Actions to create and approve pull requests"** if you
  want PR-aware tooling later; not required for builds.

This gives `GITHUB_TOKEN` both `contents: write` (for GitHub Releases) and
`packages: write` (for `ghcr.io`), repo-wide.

**Option 2 — per-workflow (tighter):**

Add an explicit `permissions:` block to `.github/workflows/build.yml`. Either
at the top of the file or scoped to just the `create-release` job:

```yaml
permissions:
  contents: write   # required to create GitHub Releases
  packages: write   # required to push images to ghcr.io/soultech67/...
```

This is the minimum the fork's `create-release` job needs.

### C. Make the GHCR package public (or accept that pulls require auth)

The first time the workflow pushes `ghcr.io/soultech67/opctl:<version>-dind`,
GHCR will create the package as **private** under your user. Two implications:

- `release/check.sh` does an unauthenticated `docker pull` to decide whether
  a release is already published. Against a private package this will return
  `denied` and the check script's `denied` branch will fail the release.
  You can either:
  - Make the package public once it exists: visit
    `https://github.com/users/soultech67/packages/container/opctl/settings`
    and set **Change visibility → Public**, or
  - Modify `release/check.sh` to authenticate the pull (e.g. `docker login
    ghcr.io` first using `$GITHUB_TOKEN`).
- Users (and your own CI bootstrap) pulling the image at runtime will need to
  log in unless you make it public.

### D. Provide the test secret (only needed for the `build` job)

The `build` job (PRs not on `main`) runs:

```yaml
opctl run -a githubAccessToken='${{ secrets.TEST_GITHUB_ACCESS_TOKEN }}' test
```

`.opspec/test/op.yml` declares `githubAccessToken` with `minLength: 1` and
forwards it to `cli/.opspec/test/e2e`, which uses it to clone
`github.com/opctl/test-suite-auth` — a private upstream fixture used to test
auth-protected git fetches.

Options for the fork:

- **Create the secret with any token that can read `opctl/test-suite-auth`.**
  Repository Settings → Secrets and variables → Actions → **New repository
  secret** → name `TEST_GITHUB_ACCESS_TOKEN`. A fine-grained PAT with
  read access to that one repo is enough. You don't need to fork
  `test-suite-auth`; the e2e test just needs *any* token that can clone it.
- **Skip the `build` job** if you don't need PR-time test runs. Delete the
  job from `.github/workflows/build.yml` or guard it behind
  `if: ${{ secrets.TEST_GITHUB_ACCESS_TOKEN != '' }}`. The other three jobs
  don't depend on this secret.

### E. (Optional) Branch protection on `main`

Not required for any job to run, but if you want the `create-release` job to
gate on green checks before merging to `main`, add a branch protection rule
under Settings → Branches that requires `check-for-changelog`,
`lint-changelog`, and `build` to pass.

### F. (Optional) Switch the bootstrap download to your fork's releases

Every job starts with:

```bash
curl -L https://github.com/opctl/opctl/releases/latest/download/opctl-linux-amd64.tgz | sudo tar -xzv -C /usr/local/bin
```

Today this downloads the **upstream** `opctl` binary just to run the opspec
ops. Once your fork has produced its own releases, you can switch this URL to
`https://github.com/soultech67/opctl/releases/latest/...` so CI runs the
fork's own binary end-to-end. Until you have a release in the fork, leave it
pointing at upstream — it's only a bootstrap.

## Quick checklist

- [ ] Enable Actions on the fork (§A).
- [ ] Set workflow `GITHUB_TOKEN` permissions to allow `contents: write` and
      `packages: write` (§B).
- [ ] Create the `TEST_GITHUB_ACCESS_TOKEN` repo secret, or remove/guard the
      `build` job (§D).
- [ ] Edit `.opspec/release/to-github/op.yml` to use
      `owner: soultech67`, `repo: opctl` (§1).
- [ ] Edit `.opspec/release/to-ghcr/op.yml` to push
      `ghcr.io/soultech67/opctl:...` (§2).
- [ ] Edit `.opspec/release/check.sh` to pull from
      `ghcr.io/soultech67/opctl` (§3).
- [ ] After the first push to `ghcr.io`, change the package visibility to
      **Public** (§C) — or modify `check.sh` to authenticate.
- [ ] (Optional) Override `selfUpdateRepo` default to `soultech67/opctl` in
      `.opspec/compile/op.yml` (§4).
- [ ] (Optional) After the first release, repoint the bootstrap `curl` URL in
      `.github/workflows/build.yml` to the fork (§F).
