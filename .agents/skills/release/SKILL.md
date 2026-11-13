---
name: release
description: 'Release a new version of better-git by tagging and publishing binaries to GitHub Releases. Use when asked to release, publish, ship, cut a version, tag, run make release, or to check or debug a release workflow run or missing release assets.'
---

# Releasing better-git

A release is a git tag. Pushing any tag makes `.github/workflows/build.yml` build `better-git-{darwin,linux}-{amd64,arm64}` and publish them as a GitHub release named after the tag. `install.sh` downloads from `releases/latest`, so a published release reaches every new install immediately.

Versions are `YY.MM.n` (for example `26.11.3`), where `n` starts at 0 each month and counts up. `make release` computes the next one from the existing tags and tags `HEAD` locally; it does not push. The binary's version string comes from `git describe`, so no file needs a version bump.

Pushing the tag is the point of no return, and it is public. Get the user's explicit go-ahead before that step, even if they asked for a release at the start: show them the commit and the tag name first.

More generally, when anything about the release is unclear (which commit, what to do with uncommitted or unpushed work, whether a failed run should be rerun or fixed forward), ask the user with concrete options instead of picking one.

## Procedure

1. **Check the state of the repository.**

   ```sh
   git status --short
   git fetch origin --tags
   git --no-pager log --oneline -1
   git rev-parse --abbrev-ref HEAD
   git rev-list --left-right --count origin/main...HEAD
   ```

   Required: clean tree, on `main`, and `0 0` from the last command (HEAD is exactly what is on `origin/main`). If there are uncommitted changes or unpushed commits, stop and ask the user how they want them handled; do not commit, stash, or push them on your own. The fetch matters because `make release` numbers from local tags, and a tag missing locally leads to a collision on push.

2. **Verify the commit being released.**

   ```sh
   make check
   .agents/skills/develop/scripts/smoke.sh
   ```

   Then confirm CI passed for it: `gh run list --commit "$(git rev-parse HEAD)"`. No run is expected if the commit touched no Go files, because the workflow is path-filtered on branch pushes.

3. **Summarize and ask.** List what has changed since the last tag and state the tag that will be created:

   ```sh
   git --no-pager log --oneline "$(git describe --tags --abbrev=0)..HEAD"
   ```

   If that list is empty, HEAD is already tagged and there is nothing to release.

4. **Tag and push**, once the user confirms.

   ```sh
   make release                 # prints "tagged YY.MM.n"
   git push origin YY.MM.n      # push that one tag by name, never --tags
   ```

5. **Watch the workflow and verify the result.**

   ```sh
   gh run watch "$(gh run list --workflow build.yml --limit 1 --json databaseId --jq '.[0].databaseId')" --exit-status
   gh release view YY.MM.n --json tagName,assets --jq '.tagName, .assets[].name'
   ```

   Expect four assets: `better-git-darwin-amd64`, `better-git-darwin-arm64`, `better-git-linux-amd64`, `better-git-linux-arm64`. To confirm the published binary itself, download the one for this machine and check its version:

   ```sh
   tmp="$(mktemp -d)" && gh release download YY.MM.n --pattern 'better-git-darwin-arm64' --dir "$tmp"
   chmod +x "$tmp/better-git-darwin-arm64" && "$tmp/better-git-darwin-arm64" --version
   ```

   The first line should read `bettergit version YY.MM.n` with no `-dirty` or `-g<sha>` suffix.

6. **Report** the tag, the release URL (`gh release view YY.MM.n --json url --jq .url`), and the verification results.

## When something goes wrong

- **Tagged locally, not yet pushed, and the tag is wrong:** `git tag -d YY.MM.n` is safe, since nothing has been published.
- **The workflow fails after the tag was pushed:** the `release` job only runs after all four builds succeed, so no release exists and `releases/latest` still points at the previous one. Read the failure with `gh run view <id> --log-failed`. If it was transient, `gh run rerun <id> --failed` publishes the same tag. If the code is at fault, fix it on `main` and release the next number.
- **A bad release is already published:** fix forward with the next number. Do not delete or move a pushed tag or delete a release without the user asking for exactly that; people and `install.sh` may already have fetched it.
- **`gh release create` fails because the release exists:** the tag was released before; a rerun cannot overwrite it. Release the next number.

## Related details

- `install.sh` needs `BETTERGIT_REPO` to be `thaisonzdx/better-git`; its default in the script is currently empty, so installs must set it in the environment until a default is filled in.
- Changes to asset names, target platforms, or the release step belong in `build.yml` and must stay in sync with `install.sh`; see the `develop` skill.
