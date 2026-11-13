#!/usr/bin/env bash
# End-to-end checks of bin/git against a throwaway HOME; never touches ~/.bettergit.
set -u

cd "$(dirname "$0")/../../../.."
make -s build || exit 1
bin="$PWD/bin/git"

# First git on PATH that is not an installed better-git wrapper.
real=""
while IFS= read -r candidate; do
  if ! "$candidate" --version 2>/dev/null | grep -q '^bettergit version'; then
    real="$candidate"
    break
  fi
done < <(type -ap git)
if [ -z "$real" ]; then
  echo "smoke: no real git on PATH" >&2
  exit 1
fi

tmp="$(mktemp -d)"
trap 'rm -rf "$tmp"' EXIT
export HOME="$tmp/home"
mkdir -p "$HOME" "$tmp/fake" "$tmp/empty"
printf '#!/bin/sh\necho "fake $*"\n' >"$tmp/fake/git"
chmod +x "$tmp/fake/git"

failed=0
check() { # check <name> <expected> <actual>
  if [ "$2" = "$3" ]; then
    echo "ok   $1"
  else
    echo "FAIL $1"
    echo "  expected: $2"
    echo "  actual:   $3"
    failed=1
  fi
}

# No config: falls back to PATH.
out="$("$bin" --version)"
check "--version prints the wrapper version first" "bettergit version" "$(echo "$out" | sed -n '1s/^\(bettergit version\) .*/\1/p')"
check "--version then prints git's own version" "$("$real" --version)" "$(echo "$out" | sed -n 2p)"
check "stdout is passed through untouched" "$("$real" rev-parse HEAD)" "$("$bin" rev-parse HEAD)"

"$real" not-a-command >/dev/null 2>&1
want=$?
"$bin" not-a-command >/dev/null 2>&1
check "exit code is passed through" "$want" "$?"

check "wrapper first on PATH does not exec itself" "$("$real" rev-parse HEAD)" "$(PATH="$PWD/bin:$PATH" git rev-parse HEAD)"

# The profile picker only opens on a terminal; anything else gets git's own behavior.
"$real" config </dev/null >/dev/null 2>&1
want=$?
"$bin" config </dev/null >/dev/null 2>&1
check "bare config without a terminal is passed through" "$want" "$?"
check "bare config without a terminal writes no config" "" "$(ls -A "$HOME")"

out="$(PATH="$tmp/empty" "$bin" status 2>&1)"
check "no git anywhere: exit 1" "1" "$?"
check "no git anywhere: error on stderr" "better-git: original git not found; set gitPath in ~/.bettergit" "$out"

# Config pointing at a valid git wins over PATH.
printf '{"gitPath": "%s"}\n' "$tmp/fake/git" >"$HOME/.bettergit"
check "gitPath from config is used" "fake a b" "$("$bin" a b)"

# Stale config falls back to PATH.
printf '{"gitPath": "%s"}\n' "$tmp/missing/git" >"$HOME/.bettergit"
check "stale gitPath falls back to PATH" "$("$real" rev-parse HEAD)" "$("$bin" rev-parse HEAD)"

# Broken config is an error, not a silent fallback.
echo '{not json' >"$HOME/.bettergit"
"$bin" status >/dev/null 2>&1
check "invalid config: exit 1" "1" "$?"

exit "$failed"
