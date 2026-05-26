#!/bin/sh
set -eu

target=${1:-}

usage() {
  echo "usage: $0 {install|uninstall|reset-backup}" >&2
  exit 2
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
    [ "$dest_owner_uid" = "$(id -u)" ]
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

  if can_install_without_sudo "$prefix" "$dest"; then
    mkdir -p "$prefix"
    cp "$src_bin" "$dest"
    chmod +x "$dest"
    return
  fi

  ensure_sudo "$dest"
  echo "installing $dest with sudo"
  sudo mkdir -p "$prefix"
  sudo cp "$src_bin" "$dest"
  sudo chmod +x "$dest"
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
    echo "running 'sudo $existing_opctl node delete' (requires root)..."
    sudo "$existing_opctl" node delete
  else
    echo "opctl not on PATH; skipping 'node delete'"
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
  *)
    usage
    ;;
esac
