# GitHub configuration for the `soultech67/opctl` fork

This repository is wired to build, test, and release from
`soultech67/opctl`.

The Go module/import path intentionally remains the original module path. Do
not treat those internal imports as release ownership settings.

## Current CI and Release Targets

`.github/workflows/build.yml` bootstraps every job from the fork release:

```yaml
OPCTL_BOOTSTRAP_RELEASE_REPO: soultech67/opctl
```

That means each job installs:

```sh
curl -L https://github.com/soultech67/opctl/releases/latest/download/opctl-linux-amd64.tgz | sudo tar -xzv -C /usr/local/bin
```

The release ops also target the fork:

- `.opspec/release/to-github/op.yml` creates releases and uploads assets to
  `owner: soultech67`, `repo: opctl`.
- `.opspec/release/to-ghcr/op.yml` pushes
  `ghcr.io/soultech67/opctl:<version>-{dind,dood}`.
- `.opspec/release/check.sh` checks the same GHCR path before publishing.
- `.opspec/compile/op.yml` defaults `selfUpdateRepo` to `soultech67/opctl`, so
  release-built binaries use fork releases for `opctl self-update`.

## Required GitHub Settings

Actions must be enabled on the fork. The release job uses `${{ github.token }}`
to create GitHub releases and push GHCR images, so the workflow token needs:

```yaml
permissions:
  contents: write
  packages: write
```

Those can be granted repo-wide under Settings -> Actions -> General, or added
as an explicit permissions block in the workflow.

## Test Auth Fixture

The PR build job requires the `TEST_GITHUB_ACCESS_TOKEN` repository secret. The
token only needs read access to the private fixture repo:

```text
github.com/soultech67/test-suite-auth#1.0.0
```

The test op passes that ref through `githubAuthTestOpRef`, so local runs can
override it if needed:

```sh
opctl run \
  -a githubAccessToken="$TOKEN" \
  -a githubAuthTestOpRef='github.com/soultech67/test-suite-auth#1.0.0' \
  test
```

## GHCR Visibility

The first release push creates the GHCR package under `soultech67`. If users or
unauthenticated environments need to pull `ghcr.io/soultech67/opctl`, make the
package public in the GitHub package settings. Otherwise, consumers must log in
to GHCR before pulling images.
