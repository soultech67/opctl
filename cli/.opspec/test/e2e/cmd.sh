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

# Capture the exit code explicitly. The script runs under `sh -e`, where
# `output=$(opctl run ...)` would abort the WHOLE script the instant opctl exits
# non-zero -- so a negative-auth scenario that *correctly* fails could never
# reach the assertion below. Disable errexit around the run, capture combined
# stdout+stderr (so a failure's output is visible in CI), then assert on $rc.
set +e
output=$(opctl run --no-progress --arg-file /args.yml "$opRef" 2>&1)
rc=$?
set -e
echo "$output"

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
