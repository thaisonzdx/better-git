---
name: develop
description: 'Build, fix, and test better-git, the Go wrapper that sits in front of the real git binary. Use when adding a feature, fixing a bug, debugging the wrapper, changing config loading or install.sh, running make build/vet/check, writing Go tests, or running the smoke test.'
---

# Developing better-git

better-git is a single static binary installed as `~/.local/bin/git`, first on `PATH`. It locates the original git and replaces itself with it via `syscall.Exec`, passing arguments, environment, stdio, and exit code straight through. Every git invocation on the user's machine (shells, editors, scripts, other tools) goes through it, so a regression breaks git everywhere for that user.

## Layout

| File | Role |
|------|------|
| `main.go` | Entry point, original-git lookup, exec handoff, dispatch to the profile picker |
| `config.go` | Loads `~/.bettergit` (JSON, `gitPath` and `profiles` keys); a missing file is not an error. `saveProfile` appends a profile and preserves every other key |
| `profile.go` | Profile picker for bare `git config`: flow, line prompts, applying the choice |
| `menu.go` | Arrow-key menu: key decoding, drawing, raw terminal mode |
| `term_darwin.go`, `term_linux.go` | Per-OS ioctl constants for reading and setting terminal modes |
| `profile_test.go` | Unit tests for the picker, the menu, key decoding, and saving profiles |
| `install.sh` | curl-pipe installer: downloads the release asset, writes `~/.bettergit`, adds `~/.local/bin` to the shell profile |
| `Makefile` | `build`, `vet`, `check`, `release`, `clean` |
| `.github/workflows/build.yml` | Vets and cross-builds darwin/linux × amd64/arm64; publishes a release on any tag |

Original git lookup order: `gitPath` from the config if it is a regular executable file that is not the wrapper itself, otherwise the first such `git` on `PATH`. A stale `gitPath` falls back silently; an unreadable or invalid config is a hard error.

Profile picker: `git config` with no other arguments, when stdin and stderr are both terminals, lists the `profiles` from the config (`[{"name": ..., "email": ...}]`) plus an entry for typing a name and email, which can be saved as a new profile. It reads the effective `user.name` / `user.email` first (local, falling back to global) and marks the matching profile `(current)`, or prints a `Current:` line when none matches. The menu is driven with ↑/↓ or j/k and Enter, with the cursor starting on the current profile; digits 1-9 select directly; Esc, q, or Ctrl-C cancels with nothing changed. Color is off when `NO_COLOR` is set. With `TERM` unset or `dumb` the menu is replaced by a numbered list and a typed number. Empty or invalid typed input is asked again; end of input cancels. The choice is written with `git config --local user.name` / `user.email`. This is the one path that does not exec: it runs the original git as a child process. Any other `git config` invocation, or a non-terminal, is handed to git unchanged.

## Ask when the request is ambiguous

Feature requests for this tool are usually a single line, and the choices behind them (what triggers the behavior, what gets written and where, what happens without a terminal) change what every git call does. When a request leaves such a choice open and the code does not settle it, ask the user before writing code: one batch of questions, each with concrete options and a recommended default. Do not ask about things the repository already answers, and do not ask again once answered.

## Constraints

These follow from the wrapper being invisible in front of git:

- **stdout belongs to git.** Scripts parse `git rev-parse`, `git status --porcelain`, and the like. The wrapper writes to stdout only for flags it owns (currently `--version` / `-v`); everything else, including the profile picker's menu and prompts, goes to stderr, and messages carry the `better-git:` prefix.
- **Interactive behavior needs a terminal.** Anything that prompts must first check `isTerminal` on stdin and stderr and otherwise hand off to git untouched, so scripts, editors, and CI never block on a prompt.
- **Leave the terminal as it was found.** Raw mode turns off echo and signal keys, so every return path out of the menu must run the `restore` from `rawMode` and show the cursor again. Do not call `os.Exit` while raw mode is on.
- **Nothing runs after `syscall.Exec`.** On success the process image is replaced, so all wrapper work happens before the handoff. There is no way to post-process git's output or exit code without changing to a child-process model, which costs signal, TTY, and exit-code fidelity. Raise that trade-off with the user before going down that path.
- **Startup cost is paid on every git call.** Keep the path before exec to a few stats and one small file read. No network, no subprocesses.
- **Never exec the wrapper itself.** Any new lookup path must keep the `os.SameFile` guard, or a misconfiguration becomes an infinite exec loop.
- **Standard library only.** There is no `go.sum`; adding a dependency also means CI and release builds start fetching modules. Ask first.
- **Must cross-compile with `CGO_ENABLED=0`** for darwin and linux on amd64 and arm64. Windows is not a target (`syscall.Exec` does not exist there).
- **Shared contracts.** The asset name `better-git-<os>-<arch>` is used by both `build.yml` and `install.sh`. The config path and `gitPath` key are used by both `config.go` and `install.sh`. Change both sides together. Anything that rewrites `~/.bettergit` must keep keys it does not know about.
- `install.sh` runs under bash on macOS (bash 3.2) and Linux, is wrapped in `{ }` so a truncated download does nothing, and prefixes every function with `bettergit_`.

## Workflow

1. Run `git status --short` first. The tree often holds uncommitted work from the user; build on top of it and leave unrelated changes alone.
2. Make the change. For a bug, reproduce it first with a failing check in the smoke script or a Go test, so the fix is demonstrated.
3. Verify (see below) and report what was run and what it showed.
4. Do not commit, tag, or push unless asked. Releasing is covered by the `release` skill.

## Verifying

Run all three after any change to Go code:

```sh
make check                                   # go vet, build with version stamp, go test, bin/git --version
.agents/skills/develop/scripts/smoke.sh      # end-to-end behavior, see below
GOOS=linux GOARCH=amd64 CGO_ENABLED=0 go build -o /dev/null .   # the cross-build CI does
```

`make check` should print `bettergit version <git describe>` followed by the real git's version line.

The [smoke script](./scripts/smoke.sh) builds `bin/git` and runs it against a temporary `HOME`, so the real `~/.bettergit` is never read or written. It covers the PATH fallback, config `gitPath`, stale and invalid configs, stdout and exit-code passthrough, the self-exec guard, the no-git error, and bare `git config` passing through when there is no terminal. When behavior is added or changed, add a `check` line for it there.

The smoke script has no terminal, so it cannot reach the profile picker. The picker's logic is unit tested through `pickProfile`, `selectMenu`, and `nextKey`; raw mode and drawing are not, so after touching `menu.go`, exercise the real thing: run it by hand in a scratch repository with a temporary `HOME`:

```sh
scratch="$(mktemp -d)" && git -C "$scratch" init -q
(cd "$scratch" && HOME="$scratch" /path/to/better-git/bin/git config)
```

To try a build by hand without installing it:

```sh
HOME="$(mktemp -d)" bin/git status                 # no config, PATH fallback
PATH="$PWD/bin:$PATH" git status                   # wrapper first on PATH, as when installed
```

Do not copy the build over `~/.local/bin/git` or edit `~/.bettergit` to test; that changes the user's live git.

### Go tests

Tests live in `package main` next to the code and run as part of `make check` and in `build.yml`. Isolate them with `t.Setenv("HOME", t.TempDir())` and `t.Setenv("PATH", ...)`. The exec handoff cannot be unit tested (it replaces the test process), and neither can anything that needs a terminal; keep the logic in functions that take readers and writers or return the path and arguments, as `pickProfile` does, and leave the rest to the smoke script.

### install.sh

CI does not run for changes to `install.sh` (the workflow's path filter covers only Go files and the workflow itself), so verify locally:

```sh
bash -n install.sh
shellcheck install.sh        # if installed
```

Running it end to end downloads the latest published release, so it exercises the script but not local Go changes. Use a sandbox so nothing real is modified:

```sh
sandbox="$(mktemp -d)" && touch "$sandbox/.zshrc"
HOME="$sandbox" PROFILE="$sandbox/.zshrc" BETTERGIT_REPO=thaisonzdx/better-git bash install.sh
```

Then check `$sandbox/.local/bin/git`, `$sandbox/.bettergit`, and the line appended to `$sandbox/.zshrc`.
