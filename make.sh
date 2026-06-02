#!/bin/sh
set -eu

target=${1:-}

usage() {
  echo "usage: $0 {install|uninstall|reset-backup|docker-logs|docker-daemon-logs|up|doctor|docker-restart}" >&2
  exit 2
}

# run_bounded <seconds> <stdout-file> <cmd...>: run cmd with a wall-clock
# timeout (macOS has no `timeout`/`gtimeout` by default). cmd's stdout goes to
# stdout-file. Returns cmd's exit code on completion, or 124 on timeout.
# Used to keep `docker run`-based introspection from hanging forever when
# Docker itself is wedged at the container-start level.
run_bounded() {
  _rb_timeout=$1
  _rb_out=$2
  shift 2

  "$@" >"$_rb_out" 2>/dev/null &
  _rb_pid=$!

  ( sleep "$_rb_timeout"; kill "$_rb_pid" 2>/dev/null ) &
  _rb_killer=$!

  if wait "$_rb_pid" 2>/dev/null; then
    _rb_rc=0
  else
    _rb_rc=$?
  fi

  # Stop the killer if it's still sleeping; reap it.
  kill "$_rb_killer" 2>/dev/null || true
  wait "$_rb_killer" 2>/dev/null || true

  # A process killed by our timer exits with 128+SIGTERM(15)=143. Normalize to
  # 124 (the conventional "timed out" code) so callers can treat it uniformly.
  if [ "$_rb_rc" -eq 143 ]; then
    return 124
  fi
  return "$_rb_rc"
}

path_index() {
  needle=$1
  index=0
  old_ifs=$IFS
  IFS=:

  for entry in ${PATH:-}; do
    if [ "$entry" = "$needle" ]; then
      IFS=$old_ifs
      echo "$index"
      return 0
    fi

    index=$((index + 1))
  done

  IFS=$old_ifs
  return 1
}

path_has_dir() {
  path_index "$1" >/dev/null 2>&1
}

first_path_candidate() {
  home_bin=$1
  local_bin=$2
  old_ifs=$IFS
  IFS=:

  for entry in ${PATH:-}; do
    if [ "$entry" = "$home_bin" ] && [ -d "$home_bin" ]; then
      IFS=$old_ifs
      echo "$home_bin"
      return 0
    fi

    if [ "$entry" = "$local_bin" ] && [ -d "$local_bin" ]; then
      IFS=$old_ifs
      echo "$local_bin"
      return 0
    fi
  done

  IFS=$old_ifs
  return 1
}

select_prefix() {
  manual_prefix=${PREFIX:-}
  if [ -n "$manual_prefix" ]; then
    echo "$manual_prefix"
    return 0
  fi

  existing_opctl=${1:-}
  if [ -n "$existing_opctl" ] && [ -f "$existing_opctl" ]; then
    dirname "$existing_opctl"
    return 0
  fi

  home_bin=$HOME/bin
  local_bin=$HOME/.local/bin

  if prefix=$(first_path_candidate "$home_bin" "$local_bin"); then
    echo "$prefix"
    return 0
  fi

  if [ -d "$home_bin" ] && [ ! -d "$local_bin" ]; then
    echo "$home_bin"
    return 0
  fi

  if [ -d "$local_bin" ]; then
    echo "$local_bin"
    return 0
  fi

  if [ -d "$home_bin" ]; then
    echo "$home_bin"
    return 0
  fi

  echo "$local_bin"
}

display_path() {
  path=$1
  case "$path" in
    "$HOME"/*)
      printf '%s/%s\n' "\$HOME" "${path#"$HOME"/}"
      ;;
    *)
      printf '%s\n' "$path"
      ;;
  esac
}

print_path_hint() {
  prefix=$1
  shell_name=$(basename "${SHELL:-sh}")
  shown_prefix=$(display_path "$prefix")

  echo
  echo "$prefix is not on your PATH."

  case "$shell_name" in
    fish)
      echo "Add it for fish with:"
      echo "  fish_add_path $shown_prefix"
      ;;
    zsh)
      echo "Add it for zsh with:"
      echo "  echo 'export PATH=\"$shown_prefix:\$PATH\"' >> ~/.zshrc"
      echo "  source ~/.zshrc"
      ;;
    bash)
      if [ "$(uname -s)" = "Darwin" ]; then
        profile=~/.bash_profile
      else
        profile=~/.bashrc
      fi

      echo "Add it for bash with:"
      echo "  echo 'export PATH=\"$shown_prefix:\$PATH\"' >> $profile"
      echo "  source $profile"
      ;;
    *)
      echo "Add it to your PATH with:"
      echo "  export PATH=\"$shown_prefix:\$PATH\""
      ;;
  esac
}

owner_uid() {
  path=$1

  stat -f %u "$path" 2>/dev/null || stat -c %u "$path" 2>/dev/null
}

first_existing_parent() {
  path=$1

  while [ ! -d "$path" ]; do
    parent=$(dirname "$path")
    if [ "$parent" = "$path" ]; then
      return 1
    fi

    path=$parent
  done

  echo "$path"
}

can_install_without_sudo() {
  prefix=$1
  dest=$2

  if [ -e "$dest" ]; then
    [ -w "$dest" ] || return 1
    dest_owner_uid=$(owner_uid "$dest") || return 1
    [ "$dest_owner_uid" = "$(id -u)" ] || return 1
    # copy_opctl now installs via a temp file + atomic rename, which creates and
    # renames entries in the parent dir -- so the parent must be writable too,
    # not just $dest (otherwise we'd pick the no-sudo path and then fail to write
    # the temp).
    [ -w "$(dirname "$dest")" ]
    return
  fi

  parent=$(first_existing_parent "$prefix") || return 1
  [ -w "$parent" ]
}

ensure_sudo() {
  command -v sudo >/dev/null 2>&1 || {
    echo "error: installing to $1 requires elevated permissions, but sudo is not available" >&2
    exit 1
  }
}

copy_opctl() {
  src_bin=$1
  prefix=$2
  dest=$3

  # Install via a temp file + atomic rename instead of overwriting $dest in
  # place. macOS caches a binary's code signature per vnode/inode; `cp`-ing over
  # a binary that was previously launched at this path reuses the inode, so the
  # new bytes are verified against the OLD cached signature and the process is
  # SIGKILLed on launch (`opctl -v` -> "killed", exit 137). A rename gives $dest
  # a fresh inode so the new signature is checked cleanly, and the swap is atomic
  # (no window where $dest is missing or half-written). Temp lives in the dest
  # dir so the rename stays on one filesystem.
  tmp="$dest.install.$$"

  if can_install_without_sudo "$prefix" "$dest"; then
    mkdir -p "$prefix"
    cp "$src_bin" "$tmp" && chmod +x "$tmp" && mv -f "$tmp" "$dest" || { rm -f "$tmp"; return 1; }
    return
  fi

  ensure_sudo "$dest"
  echo "installing $dest with sudo"
  sudo mkdir -p "$prefix"
  sudo cp "$src_bin" "$tmp" && sudo chmod +x "$tmp" && sudo mv -f "$tmp" "$dest" || { sudo rm -f "$tmp"; return 1; }
}

# extract_opctl_version invokes "<bin> -v" to read the version string baked in
# via -ldflags at compile time. Returns success and prints the version on
# stdout, or returns failure for dev builds without ldflags (where -v doesn't
# exist because cobra hides the flag when the Version field is empty).
extract_opctl_version() {
  bin=$1
  raw=$("$bin" -v 2>/dev/null) || return 1
  # strip whitespace; opctl's version template appends a newline
  version=$(printf '%s' "$raw" | tr -d '[:space:]')
  if [ -z "$version" ]; then
    return 1
  fi
  printf '%s' "$version"
}

# any_opctl_backup_exists reports whether the prefix directory already holds
# any opctl-<version> file. Used to enforce the "back up only if no backup is
# present" rule — preserves the ORIGINAL pre-fork release as the restore
# target across repeated `make install` invocations.
any_opctl_backup_exists() {
  prefix=$1
  for candidate in "$prefix"/opctl-*; do
    if [ -f "$candidate" ]; then
      return 0
    fi
  done
  return 1
}

# backup_existing_opctl copies the currently-installed opctl to opctl-<version>
# inside the same prefix, but only if no opctl-<version> backup already exists.
# Caller-supplied $existing is the absolute path to the currently-installed
# binary (or empty if opctl isn't on PATH); $prefix is the directory it lives
# in (or would).
backup_existing_opctl() {
  existing=$1
  prefix=$2

  if [ -z "$existing" ] || [ ! -f "$existing" ]; then
    return 0
  fi

  if any_opctl_backup_exists "$prefix"; then
    existing_backup=$(ls "$prefix"/opctl-* 2>/dev/null | head -1)
    echo "backup already exists ($existing_backup); leaving as-is"
    return 0
  fi

  if version=$(extract_opctl_version "$existing"); then
    backup_name=opctl-$version
  else
    backup_name=opctl-snapshot-$(date +%Y%m%d-%H%M%S)
    echo "could not determine current opctl version (likely a dev build); using $backup_name"
  fi
  backup_path=$prefix/$backup_name

  echo "backing up $existing to $backup_path"
  if can_install_without_sudo "$prefix" "$backup_path"; then
    cp "$existing" "$backup_path"
  else
    ensure_sudo "$backup_path"
    sudo cp "$existing" "$backup_path"
  fi
}

# find_highest_opctl_backup prints the absolute path to the highest-version
# opctl-* backup in $prefix (semver-sorted via `sort -V`). Empty stdout if
# none exist.
find_highest_opctl_backup() {
  prefix=$1
  candidates=
  for candidate in "$prefix"/opctl-*; do
    if [ -f "$candidate" ]; then
      candidates="$candidates$candidate
"
    fi
  done
  if [ -z "$candidates" ]; then
    return 0
  fi
  printf '%s' "$candidates" | sort -V | tail -1
}

install_opctl() {
  goos=${GOOS:-$(uname -s | tr '[:upper:]' '[:lower:]')}
  raw_arch=$(uname -m)
  case "$raw_arch" in
    x86_64)
      default_goarch=amd64
      ;;
    aarch64 | arm64)
      default_goarch=arm64
      ;;
    *)
      default_goarch=$raw_arch
      ;;
  esac

  goarch=${GOARCH:-$default_goarch}
  src_bin=${SRC_BIN:-./cli/opctl-$goos-$goarch}

  if [ ! -f "$src_bin" ]; then
    echo "error: $src_bin not found - run 'make build' first" >&2
    exit 1
  fi

  existing_opctl=$(which opctl 2>/dev/null || true)
  prefix=$(select_prefix "$existing_opctl")
  dest=$prefix/opctl

  if [ -n "$existing_opctl" ] && [ -f "$existing_opctl" ]; then
    # Stop the daemon (so the next run picks up the new binary) but KEEP the
    # data dir. `node delete` would rm -rf it, and on macOS Docker Desktop's
    # VirtioFS that delete+recreate trips a stale-inode bug surfacing as
    # "bind source path does not exist" on the next run (see
    # docs/macos-docker-filesystem.md). Keeping the data dir also preserves
    # event history and the node log across dev installs.
    echo "running 'sudo $existing_opctl node kill' to stop the running daemon (requires root)..."
    sudo "$existing_opctl" node kill
  else
    echo "opctl not on PATH; skipping node stop"
  fi

  if [ -z "${PREFIX:-}" ] && [ -z "$existing_opctl" ] && [ ! -d "$HOME/bin" ] && [ ! -d "$HOME/.local/bin" ]; then
    echo "no opctl found on PATH and neither ~/bin nor ~/.local/bin exists; creating $prefix"
  fi

  # Back up the currently-installed binary (once) before overwriting it.
  # `make uninstall` restores from this backup. Only the first install creates
  # the backup so subsequent dev installs don't clobber the pristine release
  # that's the restore target.
  backup_existing_opctl "$existing_opctl" "$prefix"

  copy_opctl "$src_bin" "$prefix" "$dest"
  echo "installed $dest (from $goos/$goarch build)"

  if ! path_has_dir "$prefix"; then
    print_path_hint "$prefix"
  fi
}

uninstall_opctl() {
  existing_opctl=$(which opctl 2>/dev/null || true)
  if [ -z "$existing_opctl" ] || [ ! -f "$existing_opctl" ]; then
    echo "error: opctl not found on PATH; nothing to uninstall" >&2
    exit 1
  fi

  prefix=$(dirname "$existing_opctl")
  dest=$prefix/opctl

  backup=$(find_highest_opctl_backup "$prefix")
  if [ -z "$backup" ]; then
    echo "error: no opctl-* backup found in $prefix; nothing to restore" >&2
    echo "       to fetch a fresh release run: opctl self-update" >&2
    exit 1
  fi

  echo "running 'sudo $existing_opctl node delete' (requires root)..."
  sudo "$existing_opctl" node delete

  echo "restoring $backup to $dest"
  if can_install_without_sudo "$prefix" "$dest"; then
    cp "$backup" "$dest"
    chmod +x "$dest"
  else
    ensure_sudo "$dest"
    sudo cp "$backup" "$dest"
    sudo chmod +x "$dest"
  fi

  restored_version=$(extract_opctl_version "$dest" 2>/dev/null || echo "?")
  echo "restored opctl @ $dest (version: $restored_version)"
  echo "backup left in place at $backup; remove it manually if you want a clean state"
}

reset_opctl_backup() {
  existing_opctl=$(which opctl 2>/dev/null || true)
  if [ -z "$existing_opctl" ] || [ ! -f "$existing_opctl" ]; then
    echo "error: opctl not found on PATH; cannot determine prefix to clear" >&2
    exit 1
  fi

  prefix=$(dirname "$existing_opctl")

  # Collect current backups so we can show the user exactly what will be
  # deleted (and exit cleanly if there's nothing to do).
  backups=
  for candidate in "$prefix"/opctl-*; do
    if [ -f "$candidate" ]; then
      backups="$backups$candidate
"
    fi
  done

  if [ -z "$backups" ]; then
    echo "no opctl-* backups found in $prefix; nothing to reset"
    return 0
  fi

  echo "the following opctl backup(s) in $prefix will be REMOVED:"
  printf '%s' "$backups" | sed 's/^/  /'

  # FORCE=1 mirrors `opctl container prune --force` — useful for scripts /
  # non-interactive flows. Default is to prompt because this destroys the
  # restore target.
  if [ "${FORCE:-}" != "1" ]; then
    printf "proceed? [y/N]: "
    read -r answer
    case "$(printf '%s' "$answer" | tr '[:upper:]' '[:lower:]' | tr -d '[:space:]')" in
      y|yes) ;;
      *)
        echo "reset cancelled"
        return 0
        ;;
    esac
  fi

  for backup in $(printf '%s' "$backups"); do
    if can_install_without_sudo "$prefix" "$backup"; then
      rm -f "$backup"
    else
      ensure_sudo "$backup"
      sudo rm -f "$backup"
    fi
  done

  count=$(printf '%s' "$backups" | grep -c .)
  echo "removed $count backup(s) from $prefix; next \`make install\` will create a fresh backup of the current opctl"
}

# docker_logs streams two independent Docker observability sources to files
# so we have a paper trail when reproducing a hang, without flooding the
# terminal:
#
#   1. tail -F of Docker Desktop's VM init.log, filtered to lines mentioning
#      opctl, apiproxy POSTs (every container/network operation), or any
#      warning/error level. This is what dockerd actually does on every API
#      request.
#   2. docker events filtered to opctl-managed containers — the canonical
#      "what containers did Docker create / start / kill" stream.
#
# Both run in the background; foreground waits until Ctrl+C, then cleans up.
# Log paths overridable via OPCTL_DOCKER_LOG_DIR (default: ./docker-logs).
docker_logs() {
  log_dir=${OPCTL_DOCKER_LOG_DIR:-./docker-logs}
  mkdir -p "$log_dir"

  apiproxy_log=$log_dir/apiproxy.log
  events_log=$log_dir/events.log
  vm_log=$HOME/Library/Containers/com.docker.docker/Data/log/vm/init.log

  # The grep filter pattern lives in a variable used BOTH to launch grep and
  # to find+kill it on cleanup, so the two always match.
  grep_pattern='opctl_|apiproxy.*POST|level":"(warn|error)'

  if [ ! -f "$vm_log" ]; then
    echo "warning: Docker Desktop VM log not found at:" >&2
    echo "  $vm_log" >&2
    echo "this target is macOS Docker Desktop specific; apiproxy stream will be empty" >&2
  fi

  # Truncate (not append) so each capture session is self-contained.
  : > "$apiproxy_log"
  : > "$events_log"

  echo "streaming Docker logs to:"
  echo "  $apiproxy_log  (filtered VM init.log: opctl/apiproxy POSTs/warn|error)"
  echo "  $events_log    (docker events --filter label=opctl.managed=true)"
  echo
  echo "Reproduce your hang now. Press Ctrl+C here when done."
  echo

  # Launch the tail|grep pipeline and the docker events stream backgrounded.
  #
  # IMPORTANT: we do NOT try to clean up by killing a wrapping subshell. On
  # macOS, when the subshell parent dies the pipeline's children (tail, grep)
  # get reparented to init (PID 1) and keep running forever — `tail -F` never
  # exits on its own. We observed this leaking dozens of tail/grep processes
  # across repeated runs. Instead, cleanup pkills them by their distinctive
  # command lines, which is robust against reparenting.
  tail -F "$vm_log" 2>/dev/null \
    | grep --line-buffered -E "$grep_pattern" > "$apiproxy_log" &

  docker events --filter label=opctl.managed=true > "$events_log" &
  events_pid=$!

  cleanup_docker_logs() {
    echo
    echo "stopping log streams..."
    # Kill by distinctive command line — robust against the child-reparenting
    # described above. Match on a fixed prefix of each command (no regex
    # metachars) so pkill's pattern engine behaves. NB: if you run two
    # `make docker-logs` at once, this stops both — fine for a diagnostic tool.
    pkill -f "tail -F $vm_log" 2>/dev/null || true
    pkill -f "grep --line-buffered -E opctl_" 2>/dev/null || true
    kill "$events_pid" 2>/dev/null || true
    pkill -P "$events_pid" 2>/dev/null || true

    apiproxy_lines=$(wc -l <"$apiproxy_log" 2>/dev/null | tr -d ' ' || echo 0)
    events_lines=$(wc -l <"$events_log" 2>/dev/null | tr -d ' ' || echo 0)
    echo "captured:"
    echo "  $apiproxy_log  ($apiproxy_lines lines)"
    echo "  $events_log    ($events_lines lines)"
  }
  trap cleanup_docker_logs EXIT INT TERM

  wait
}

# docker_daemon_logs sends SIGUSR1 to dockerd inside the Docker Desktop VM,
# triggering dockerd to dump every goroutine's stack trace to its own log.
# That's the most concrete way to see WHY a Docker API call (e.g. our 20s
# ContainerCreate hang) is stuck — the dump names the goroutine and the
# function/line it's blocked on (lock acquire, channel recv, etc.).
#
# Fire this WHILE the hang is in progress (within the 20s opctl spinner
# window). After dockerd writes the dump, it persists in the log file.
docker_daemon_logs() {
  vm_log=$HOME/Library/Containers/com.docker.docker/Data/log/vm/init.log
  if [ ! -f "$vm_log" ]; then
    echo "error: Docker Desktop VM log not found at:" >&2
    echo "  $vm_log" >&2
    echo "this target is macOS Docker Desktop specific" >&2
    exit 1
  fi

  echo "discovering dockerd PID inside the Docker Desktop VM..."
  # CRITICAL: this whole approach uses `docker run` to reach the VM. But the
  # wedge we're often diagnosing is "Docker can't start ANY container" — in
  # which case `docker run` itself hangs forever (observed). So bound every
  # `docker run` with a timeout: if it can't start a throwaway container in
  # 15s, Docker is too wedged to introspect this way and the only recovery is
  # `make docker-restart`.
  pid_out=$(mktemp)
  if ! run_bounded 15 "$pid_out" docker run --rm --privileged --pid=host alpine \
    sh -c 'ps -o pid,comm | awk "/dockerd\$/{print \$1; exit}"'; then
    rm -f "$pid_out"
    echo "✗ 'docker run' to introspect the VM did not return within 15s." >&2
    echo "  Docker can't start a throwaway container — it's wedged at the" >&2
    echo "  container-start level, so a goroutine dump isn't reachable this way." >&2
    echo "  Recovery: make docker-restart" >&2
    exit 1
  fi
  dockerd_pid=$(tr -d ' ' < "$pid_out")
  rm -f "$pid_out"
  if [ -z "$dockerd_pid" ]; then
    echo "error: could not locate dockerd PID inside the VM" >&2
    echo "  is Docker Desktop running? try \`docker info\`" >&2
    exit 1
  fi
  echo "dockerd PID inside VM: $dockerd_pid"

  echo "sending SIGUSR1 → goroutine stack dump..."
  sig_out=$(mktemp)
  if ! run_bounded 15 "$sig_out" docker run --rm --privileged --pid=host alpine \
    kill -USR1 "$dockerd_pid"; then
    rm -f "$sig_out"
    echo "✗ 'docker run' to signal dockerd did not return within 15s — Docker is wedged." >&2
    echo "  Recovery: make docker-restart" >&2
    exit 1
  fi
  rm -f "$sig_out"

  # Give dockerd a moment to actually write the dump.
  sleep 2

  log_dir=${OPCTL_DOCKER_LOG_DIR:-./docker-logs}
  mkdir -p "$log_dir"
  dump_file=$log_dir/dockerd-goroutines-$(date +%Y%m%d-%H%M%S).log

  # dockerd writes the dump to a separate file *inside the VM*, NOT inline
  # in init.log. The path it announces in init.log looks like:
  #   "goroutine stacks written to /var/run/docker/goroutine-stacks-<ts>.log"
  # We retrieve it by mounting the VM's /var/run/docker into a throwaway
  # alpine container — when docker creates a bind mount from "/var/run/docker"
  # the source path resolves on the VM's filesystem, not the macOS host.
  docker run --rm -v /var/run/docker:/dockerrun alpine sh -c \
    'ls -1t /dockerrun/goroutine-stacks-*.log 2>/dev/null | head -1 | xargs -r cat' \
    > "$dump_file" 2>/dev/null || true

  if [ -s "$dump_file" ]; then
    line_count=$(wc -l <"$dump_file" | tr -d ' ')
    echo "dump captured: $dump_file ($line_count lines)"

    # Note the source path inside the VM so the user can correlate with
    # the apiproxy log line if they want to.
    src=$(grep "goroutine stacks written" "$vm_log" 2>/dev/null | tail -1 \
      | grep -oE "/var/run/docker/goroutine-stacks-[^\"]*\.log" | head -1)
    if [ -n "$src" ]; then
      echo "source path inside VM: $src"
    fi
  else
    rm -f "$dump_file"
    echo "could not retrieve goroutine dump from VM"
    echo "the dump should exist at /var/run/docker/goroutine-stacks-*.log inside the VM"
    echo "manual check:"
    echo "  grep 'goroutine stacks written' $vm_log | tail -1"
  fi
}

# opctl_up kills any background daemon and re-launches it in the foreground
# with OPCTL_DEBUG_DOCKER=1, so the daemon's [opctl docker] / [opctl kill]
# instrumentation prints to this terminal in real time. Pair with
# `make docker-logs` and `make docker-daemon-logs` (in other terminals) for
# full visibility while reproducing a hang.
opctl_up() {
  if ! command -v opctl >/dev/null 2>&1; then
    echo "error: opctl not on PATH; run \`make install\` first" >&2
    exit 1
  fi

  echo "killing any background opctl daemon so we can start fresh in foreground..."
  # Don't fail if none is running.
  sudo opctl node kill 2>/dev/null || true

  # Forward the timeout multiplier if the user has set one. OPCTL_DEBUG_DOCKER
  # we set unconditionally (the whole point of `make up` is to see those logs)
  # — user can still override by exporting OPCTL_DEBUG_DOCKER=0 if they want.
  multiplier_env=""
  if [ -n "${OPCTL_DOCKER_TIMEOUT_MULTIPLIER:-}" ]; then
    multiplier_env="OPCTL_DOCKER_TIMEOUT_MULTIPLIER=$OPCTL_DOCKER_TIMEOUT_MULTIPLIER"
  fi

  debug_value=${OPCTL_DEBUG_DOCKER:-1}

  echo
  echo "starting opctl node create in foreground:"
  echo "  OPCTL_DEBUG_DOCKER=$debug_value${multiplier_env:+, $multiplier_env}"
  echo "  container-runtime=docker"
  echo "  daemon logs ([opctl docker] / [opctl kill]) stream to THIS terminal"
  echo "  Ctrl+C to stop the daemon"
  echo

  # exec so Ctrl+C reaches the daemon directly (no extra shell hop). sudo's
  # VAR=val syntax forwards env without needing -E.
  # shellcheck disable=SC2086  # intentional word split for optional env var
  exec sudo \
    OPCTL_DEBUG_DOCKER="$debug_value" \
    $multiplier_env \
    opctl --container-runtime docker node create
}

# doctor runs read-only diagnostics for the classic Docker-Desktop-wedged
# pathology. No side effects — safe to invoke any time, especially before
# debugging a hang. Each check prints ✓ / ⚠ / ✗ and a fix hint when relevant.
doctor() {
  cwd=$(pwd)

  echo "=== opctl-managed containers (any state) ==="
  if docker ps -a --filter label=opctl.managed=true --format \
    'table {{.ID}}\t{{.Names}}\t{{.Status}}' 2>/dev/null; then :
  else
    echo "✗ docker not responsive — cannot list containers"
  fi
  echo

  echo "=== Created-state containers (the invisible-orphan pathology) ==="
  created_count=$(docker ps -a --filter status=created --filter label=opctl.managed=true -q 2>/dev/null | wc -l | tr -d ' ')
  if [ "$created_count" -gt 0 ]; then
    echo "⚠ $created_count opctl container(s) stuck in Created state — these block subsequent ContainerCreate calls with overlapping mounts"
    echo "  fix: make clean"
  else
    echo "✓ none"
  fi
  echo

  echo "=== Docker daemon responsiveness ==="
  start=$(date +%s)
  if docker info >/dev/null 2>&1; then
    elapsed=$(( $(date +%s) - start ))
    if [ "$elapsed" -le 1 ]; then
      echo "✓ docker info responded in <1s"
    else
      echo "⚠ docker info took ${elapsed}s (>1s suggests dockerd is overloaded)"
      echo "  fix: make docker-restart"
    fi
  else
    echo "✗ docker info FAILED — Docker Desktop may need a restart"
    echo "  fix: make docker-restart"
  fi
  echo

  echo "=== Spotlight indexing pressure ==="
  if command -v mdutil >/dev/null 2>&1; then
    # mdutil works per-volume, not per-directory; querying a subdirectory
    # returns "Error: unknown indexing state". Resolve to the volume root.
    volume=$(df "$cwd" 2>/dev/null | awk 'NR==2 {print $NF}')
    md_state=""
    if [ -n "$volume" ]; then
      md_state=$(mdutil -s "$volume" 2>&1 | tail -1)
      echo "  volume: $volume"
      echo "  $md_state"
    fi

    # Count active metadata workers — this is the reliable signal.
    # Empirically on Docker Desktop + gRPC-FUSE, a burst of concurrent
    # mdworker_shared processes (we've seen 15-18) correlates with dockerd
    # ContainerCreate hangs in syscall.fstatat (gRPC-FUSE starved). When idle,
    # mdworker_shared drops to 0.
    #
    # We deliberately DON'T trigger on mdsync/mdbulkimport: a single
    # mdbulkimport (MDSImporterBundleFinder) runs as a persistent baseline
    # daemon for days at 0% CPU, so keying on its presence would warn forever.
    # We report its count for context only.
    #
    # Use `pgrep -f ... | wc -l` rather than `pgrep -cf`: macOS pgrep doesn't
    # accept `-cf` combined (exits 2). And `wc -l` always outputs a number so
    # we don't trip the "grep/pgrep exits 1 + ||echo 0 → '0\n0'" bug.
    worker_count=$(pgrep -f mdworker_shared 2>/dev/null | wc -l | tr -d ' ')
    sync_running=$(pgrep -f "mdsync|mdbulkimport" 2>/dev/null | wc -l | tr -d ' ')
    echo "  active mdworker_shared processes: $worker_count"
    echo "  mdsync/mdbulkimport (baseline daemons, informational): $sync_running"

    if [ "$worker_count" -gt 5 ]; then
      echo "⚠ Spotlight is heavily active right now — this competes with gRPC-FUSE for"
      echo "  macOS-side inode reads when dockerd calls os.Stat() on bind-mount sources."
      echo "  empirical correlation: goroutine dumps during ContainerCreate hangs show dockerd"
      echo "  stuck in syscall.fstatat going through gRPC-FUSE while mdworker is busy on the host."
      echo "  workarounds (mdutil -i works on VOLUMES, not directories — use these for a dir):"
      echo "    - exclude this dir + everything under it: touch $cwd/.metadata_never_index"
      echo "      (drop the same file in your other project roots; delete it to re-enable)"
      echo "    - or System Settings → Spotlight → Search Privacy → add the folder"
      echo "    - nuclear (whole volume): sudo mdutil -i off $volume"
    elif [ "$worker_count" -eq 0 ]; then
      echo "✓ no active Spotlight indexing (mdworker_shared idle)"
    else
      echo "✓ light Spotlight activity ($worker_count workers) — not a concern unless builds hang"
    fi
  else
    echo "(mdutil not available)"
  fi
  echo

  echo "=== node_modules sizes (gRPC-FUSE chokes on big trees) ==="
  found_any=0
  for dir in node_modules webapp/node_modules website/node_modules; do
    if [ -d "$dir" ]; then
      found_any=1
      size=$(du -sh "$dir" 2>/dev/null | awk '{print $1}')
      count=$(find "$dir" -type f 2>/dev/null | wc -l | tr -d ' ')
      echo "  $dir: $size ($count files)"
    fi
  done
  if [ $found_any -eq 0 ]; then
    echo "  (none in this directory)"
  fi
  echo

  echo "=== Time Machine status ==="
  if command -v tmutil >/dev/null 2>&1; then
    # First: is TM even configured? Many users have no destination set, in
    # which case TM never runs and isn't a concern. Use grep -q (binary
    # match) to avoid the "grep -c outputs '0' AND exits 1 + ||echo 0
    # appends another '0'" bug that breaks numeric comparisons.
    if tmutil destinationinfo 2>/dev/null | grep -q "^Name "; then
      dest_count=$(tmutil destinationinfo 2>/dev/null | grep -c "^Name " | tr -d ' ')
      # Configured. Check whether a backup is currently in flight via tmutil's
      # own status field rather than process scan (mdworker ≠ TM).
      tm_running=$(tmutil status 2>/dev/null | awk '/Running/{print $3}' | tr -d ';')
      if [ "$tm_running" = "1" ]; then
        tm_phase=$(tmutil status 2>/dev/null | awk '/BackupPhase/{print $3}' | tr -d '";')
        echo "⚠ Time Machine is CURRENTLY backing up (phase: $tm_phase)"
        echo "  workarounds:"
        echo "    - exclude this dir: sudo tmutil addexclusion $cwd"
        echo "    - schedule TM via TimeMachineEditor (free third-party app)"
      else
        echo "✓ Time Machine is configured ($dest_count destination(s)) but not currently active"
      fi
      excluded=$(tmutil isexcluded "$cwd" 2>/dev/null | tail -1)
      if [ -n "$excluded" ]; then
        echo "  this dir's exclusion status: $excluded"
      fi
    else
      echo "✓ no Time Machine destination configured — TM is not running on this machine"
    fi
  else
    echo "(tmutil not available)"
  fi
  echo

  echo "=== installed opctl vs latest local commit ==="
  if command -v opctl >/dev/null 2>&1; then
    installed_path=$(which opctl)
    installed_mtime=$(stat -f "%Sm" "$installed_path" 2>/dev/null || echo unknown)
    last_commit_time=$(git -C "$(dirname "$0")" log -1 --format="%ad" --date=local 2>/dev/null || echo unknown)
    echo "  installed: $installed_path (mtime: $installed_mtime)"
    echo "  HEAD:      $last_commit_time"
    echo "  if HEAD is newer, run: make build && sudo make install"
  fi
}

# docker_restart is the nuclear recovery: kills opctl daemon, quits and
# relaunches Docker Desktop, waits for the daemon to come back. Use when
# `make doctor` reports docker info as unresponsive or after a Goroutine
# dump shows dockerd stuck in syscall.fstatat (gRPC-FUSE wedge).
docker_restart() {
  echo "killing opctl daemon (if any)..."
  if command -v opctl >/dev/null 2>&1; then
    sudo opctl node kill 2>/dev/null || true
  fi

  echo "quitting Docker Desktop..."
  osascript -e 'quit app "Docker"' 2>/dev/null || true

  # Give Docker Desktop time to actually exit and tear down its VM. Too
  # short and the relaunch races against the previous instance shutting
  # down; symptom is "Docker Desktop is already running" + empty docker info.
  sleep 5

  echo "relaunching Docker Desktop..."
  open -a Docker

  echo "waiting for Docker daemon to come back online (up to 90s)..."
  i=0
  while [ $i -lt 90 ]; do
    if docker info >/dev/null 2>&1; then
      echo "✓ docker info responded after ${i}s"
      return 0
    fi
    i=$((i + 1))
    sleep 1
  done

  echo "✗ Docker daemon did not respond within 90s — check Docker Desktop UI manually" >&2
  exit 1
}

case "$target" in
  install)
    install_opctl
    ;;
  uninstall)
    uninstall_opctl
    ;;
  reset-backup)
    reset_opctl_backup
    ;;
  docker-logs)
    docker_logs
    ;;
  docker-daemon-logs)
    docker_daemon_logs
    ;;
  up)
    opctl_up
    ;;
  doctor)
    doctor
    ;;
  docker-restart)
    docker_restart
    ;;
  *)
    usage
    ;;
esac
