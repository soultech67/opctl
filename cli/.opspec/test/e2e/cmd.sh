#!/usr/bin/env sh

# Run the opctl daemon at debug level so the node log shows how the private-op
# pull resolved its credentials (or didn't). Set before any opctl command so the
# first daemon spawned picks it up; both vars are in the daemon env passlist.
export OPCTL_LOG_LEVEL=debug
export OPCTL_LOG_FILE=/tmp/opctl-node.log

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

# MOST DECISIVE TEST: hit the PRIVATE repo's git smart-http endpoint with NO auth
# at all (plain wget). This is exactly the request go-git's ls-remote makes.
#   - 401/403 here  => the repo genuinely needs auth at the network level, so
#                      opctl/go-git is sourcing a credential from somewhere (an
#                      opctl issue worth fixing).
#   - 200 here      => the runner/network grants unauthenticated access to this
#                      same-account private repo, so the test's premise is invalid
#                      in this CI environment (not an opctl bug).
# -S prints the server's RESPONSE headers (status line) to stderr; no request
# headers / token are printed, so nothing secret leaks.
echo "=== raw unauth reachability of the PRIVATE repo (no creds) ==="
apk add --no-cache curl >/dev/null 2>&1 || true
echo "[git info/refs, NO auth] (401/403 => needs auth; 200 => reachable unauth):"
curl -s -o /dev/null -w 'status=%{http_code}\n' "https://github.com/soultech67/test-suite-auth.git/info/refs?service=git-upload-pack" 2>&1
echo "[git info/refs, WITH token] (sanity, expect 200):"
curl -s -o /dev/null -w 'status=%{http_code}\n' -u ":$githubAccessToken" "https://github.com/soultech67/test-suite-auth.git/info/refs?service=git-upload-pack" 2>&1
echo "[github api, NO auth] (404 => private/hidden; 200 => visible):"
curl -s -o /dev/null -w 'status=%{http_code}\n' "https://api.github.com/repos/soultech67/test-suite-auth" 2>&1

# DECISIVE PROBE: with a brand-new, empty data dir (no `auth add` was ever run
# against it), can opctl pull the PRIVATE op ref directly? If this exits 0, opctl
# is resolving a private repo with a guaranteed-empty auth store -- i.e. the pull
# does not actually require the credentials the negative-auth test assumes, which
# is why `expect: failure` scenarios "got success". Isolated via --data-dir so it
# can't affect the scenario's own run.
PROBE_DIR=$(mktemp -d)
echo "=== PROBE: clean-data-dir pull of $githubAuthTestOpRef (no auth stored) ==="
opctl --data-dir "$PROBE_DIR" auth list 2>&1 || true
set +e
opctl --data-dir "$PROBE_DIR" run --no-progress "$githubAuthTestOpRef" >/tmp/probe.out 2>&1
probe_rc=$?
set -e
echo "PROBE exit: $probe_rc"
echo "PROBE output (first 20 lines):"
head -20 /tmp/probe.out

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

# Diagnostics: the daemon log (debug level) -- auth-decision lines. The daemon
# writes node.log under its data dir (not OPCTL_LOG_FILE), so locate it.
echo "=== daemon node.log (auth-decision lines) ==="
for LOG in $(find / -maxdepth 8 -name 'node.log' 2>/dev/null); do
  echo "--- $LOG ($(wc -l < "$LOG" 2>/dev/null) lines) ---"
  grep -iE "auth resolve|resolveData|op-call auth|matchedResources|injected|test-suite-auth|github.com|clone|ls-remote" "$LOG" 2>/dev/null | tail -40
done

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
