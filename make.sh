#!/bin/sh
set -eu

target=${1:-}

usage() {
  echo "usage: $0 install" >&2
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

  copy_opctl "$src_bin" "$prefix" "$dest"
  echo "installed $dest (from $goos/$goarch build)"

  if ! path_has_dir "$prefix"; then
    print_path_hint "$prefix"
  fi
}

case "$target" in
  install)
    install_opctl
    ;;
  *)
    usage
    ;;
esac
