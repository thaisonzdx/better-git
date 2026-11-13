#!/usr/bin/env bash

{ # this ensures the entire script is downloaded #

set -eu

BETTERGIT_DIR="$HOME/.bettergit"
BETTERGIT_BIN_DIR="$BETTERGIT_DIR/bin"
BETTERGIT_CONFIG="$BETTERGIT_DIR/config.json"

# GitHub "owner/repo" whose latest release hosts the binaries; fill in the default before publishing.
BETTERGIT_REPO="${BETTERGIT_REPO-}"

bettergit_echo() {
  command printf '%s\n' "$*" 2>/dev/null
}

bettergit_has() {
  type "$1" >/dev/null 2>&1
}

# First git on PATH that is not the better-git wrapper itself.
bettergit_find_original_git() {
  local dir
  local IFS=:
  for dir in $PATH; do
    if [ -z "$dir" ] || [ "$dir/git" -ef "$BETTERGIT_BIN_DIR/git" ]; then
      continue
    fi
    if [ -f "$dir/git" ] && [ -x "$dir/git" ]; then
      bettergit_echo "$dir/git"
      return
    fi
  done
  return 1
}

bettergit_try_profile() {
  if [ -z "${1-}" ] || [ ! -f "${1}" ]; then
    return 1
  fi
  bettergit_echo "${1}"
}

# Set $PROFILE to a file to force it, or to /dev/null to skip editing a profile.
bettergit_detect_profile() {
  if [ "${PROFILE-}" = '/dev/null' ]; then
    return
  fi

  if [ -n "${PROFILE-}" ] && [ -f "${PROFILE}" ]; then
    bettergit_echo "${PROFILE}"
    return
  fi

  local detected=''
  case "${SHELL-}" in
    */bash)
      if [ -f "$HOME/.bashrc" ]; then
        detected="$HOME/.bashrc"
      elif [ -f "$HOME/.bash_profile" ]; then
        detected="$HOME/.bash_profile"
      fi
      ;;
    */zsh)
      if [ -f "${ZDOTDIR:-$HOME}/.zshrc" ]; then
        detected="${ZDOTDIR:-$HOME}/.zshrc"
      elif [ -f "${ZDOTDIR:-$HOME}/.zprofile" ]; then
        detected="${ZDOTDIR:-$HOME}/.zprofile"
      fi
      ;;
  esac

  if [ -z "$detected" ]; then
    local name
    for name in ".profile" ".bashrc" ".bash_profile" ".zprofile" ".zshrc"; do
      if detected="$(bettergit_try_profile "$HOME/$name")"; then
        break
      fi
    done
  fi

  if [ -n "$detected" ]; then
    bettergit_echo "$detected"
  fi
}

# Prints the release asset suffix for this machine, e.g. darwin-arm64 (Go's GOOS-GOARCH names).
bettergit_platform() {
  local os arch
  case "$(uname -s)" in
    Darwin) os=darwin ;;
    Linux) os=linux ;;
    *)
      bettergit_echo >&2 "=> Unsupported OS: $(uname -s)"
      return 1
      ;;
  esac
  case "$(uname -m)" in
    x86_64 | amd64) arch=amd64 ;;
    arm64 | aarch64) arch=arm64 ;;
    *)
      bettergit_echo >&2 "=> Unsupported architecture: $(uname -m)"
      return 1
      ;;
  esac
  # A Rosetta shell reports x86_64 on Apple Silicon.
  if [ "$os" = darwin ] && [ "$(sysctl -n sysctl.proc_translated 2>/dev/null)" = 1 ]; then
    arch=arm64
  fi
  bettergit_echo "$os-$arch"
}

bettergit_download() {
  if [ -z "$BETTERGIT_REPO" ]; then
    bettergit_echo >&2 "=> BETTERGIT_REPO is not set (expected a GitHub owner/repo)"
    exit 1
  fi

  local platform
  platform="$(bettergit_platform)" || exit 1
  local url="https://github.com/$BETTERGIT_REPO/releases/latest/download/better-git-$platform"
  local tmp="$BETTERGIT_BIN_DIR/git.download"

  bettergit_echo "=> Downloading $url"
  mkdir -p "$BETTERGIT_BIN_DIR"
  if bettergit_has curl; then
    curl --fail --location --silent --show-error --output "$tmp" "$url"
  elif bettergit_has wget; then
    wget --quiet --output-document="$tmp" "$url"
  else
    bettergit_echo >&2 "=> curl or wget is required to install better-git"
    exit 1
  fi || {
    rm -f "$tmp"
    bettergit_echo >&2 "=> Failed to download $url"
    exit 1
  }

  chmod +x "$tmp"
  mv -f "$tmp" "$BETTERGIT_BIN_DIR/git"
  bettergit_echo "=> Installed $BETTERGIT_BIN_DIR/git"
}

bettergit_write_config() {
  local original_git="$1"
  if [ -f "$BETTERGIT_CONFIG" ]; then
    bettergit_echo "=> Keeping existing $BETTERGIT_CONFIG"
    return
  fi
  local escaped
  escaped="$(printf '%s' "$original_git" | sed 's/[\\"]/\\&/g')"
  printf '{\n  "gitPath": "%s"\n}\n' "$escaped" >"$BETTERGIT_CONFIG"
  bettergit_echo "=> Wrote $BETTERGIT_CONFIG"
}

bettergit_do_install() {
  local original_git
  if ! original_git="$(bettergit_find_original_git)"; then
    bettergit_echo >&2 "=> git not found on PATH; install git first"
    exit 1
  fi
  bettergit_echo "=> Original git: $original_git"

  bettergit_download
  bettergit_write_config "$original_git"

  # Literal $HOME and $PATH: they are expanded when the profile is sourced.
  # shellcheck disable=SC2016
  local source_str='export PATH="$HOME/.bettergit/bin:$PATH"'
  local profile
  profile="$(bettergit_detect_profile)"

  if [ -z "$profile" ]; then
    bettergit_echo "=> Profile not found. Tried ~/.bashrc, ~/.bash_profile, ~/.zprofile, ~/.zshrc, and ~/.profile."
    bettergit_echo "=> Create one of them and run this script again"
    bettergit_echo "   OR"
    bettergit_echo "=> Append the following line to the end of the correct file yourself:"
    bettergit_echo
    bettergit_echo "$source_str"
    bettergit_echo
    return
  fi

  if grep -qs '\.bettergit/bin' "$profile"; then
    bettergit_echo "=> better-git PATH entry already in $profile"
  else
    bettergit_echo "=> Appending better-git PATH entry to $profile"
    printf '\n%s  # better-git: must stay first on PATH\n' "$source_str" >>"$profile"
  fi

  bettergit_echo "=> Close and reopen your terminal to start using better-git, or run:"
  bettergit_echo
  bettergit_echo "$source_str"
  bettergit_echo
}

bettergit_do_install

} # this ensures the entire script is downloaded #
