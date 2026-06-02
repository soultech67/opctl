#!/usr/bin/env sh

echo "starting docker daemon as background process"
nohup dockerd \
  --host=unix:///var/run/docker.sock \
  --storage-driver=overlay2 &

# dummy account for these tests so we don't hit rate limits
# it is not secret as it has no access to anything
# and is use solely for this purpose
opctl auth add docker.io -u 3hhyyicl1mzqsr6tggmg -p '%7Oe^4#fGGwc96rGcV&4'

if [[ $authAddGithub == "true" ]]; then
  opctl auth add github.com -u " " -p $githubAccessToken
fi

exec <&-

opRef="${opRef:-/test}"
if [[ "$opRef" == "__githubAuthTestOpRef__" ]]; then
  opRef="$githubAuthTestOpRef"
fi

echo "op: $op"

# Diagnostics: stored auth (passwords are never printed by `auth list`) so we can
# see exactly which credentials are present when a scenario runs.
echo "=== stored auth entries ==="
opctl auth list 2>&1 || true

# Diagnostics: is a github credential reaching this container some other way than
# `opctl auth add`? Print only the token LENGTH (never the value), plus any
# ambient git credential sources go-git/git could pick up, with values redacted.
echo "=== github cred environment ==="
printf 'githubAccessToken length: %s\n' "$(printf %s "$githubAccessToken" | wc -c | tr -d ' ')"
ls -la ~/.netrc ~/.git-credentials ~/.gitconfig /etc/gitconfig 2>&1 || true
git config --list --show-origin 2>&1 | sed -E 's/(token|pass|auth|header|=).*/\1<redacted>/I' || echo "(no git config / git not present)"

# Capture the exit code explicitly. The script runs under `sh -e`, where
# `blah=$(opctl run ...)` aborts the WHOLE script the instant opctl exits
# non-zero -- so a negative-auth scenario that *correctly* fails could never
# reach the assertion below. Disable errexit around the run, capture combined
# stdout+stderr (so a pull/auth error is visible in CI), then assert on $rc.
# Progress is left ON so the op-pull steps are visible (did it clone, or use a
# cached/local copy?).
echo "=== opctl run $opRef ==="
set +e
blah=$(opctl run --arg-file /args.yml "$opRef" 2>&1)
rc=$?
set -e
echo "$blah"
echo "=== opctl run exit code: $rc ==="

# Diagnostics: did the private op actually get cloned into the ops cache?
echo "=== ops cache (test-suite-auth present => it was pulled) ==="
find /root /home /tmp ~ -maxdepth 8 -type d -name 'test-suite-auth*' 2>/dev/null | head

case "$expect" in
  success)
    if [ "$rc" -eq 0 ]; then
      echo "expected $expect and got success"
      exit 0
    else
      echo "expected $expect but got failure"
      exit 1
    fi
    ;;
  failure)
    if [ "$rc" -eq 0 ]; then
      echo "expected $expect but got success"
      exit 1
    else
      echo "expected $expect and got failure"
      exit 0
    fi
    ;;
esac
