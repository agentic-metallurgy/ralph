---
name: release-ralph
description: Release a new Ralph version using CalVer (vYYYY.M.D). Bumps the version in config, commits, tags, and pushes to trigger the CI release workflow. Use when user says "release ralph", "ship ralph", "bump ralph version", or "cut a ralph release".
user-invocable: true
---

# Release Ralph

Cuts a new CalVer release of Ralph: bumps the version constant, commits, tags, and pushes. The CI workflow handles building binaries, creating the GitHub release, and publishing the Homebrew formula.

## Step 1: Determine the new version

Gather these in parallel:
- Latest tag: `git tag --sort=-v:refname | head -5`
- Today's date (for CalVer)

**CalVer format: `vYYYY.M.D[.PATCH]`**
- Use today's date to form the base: `v{year}.{month}.{day}` (no zero-padding on month/day)
- If the latest tag already matches today's base, increment its patch suffix (e.g. `v2026.3.21` -> `v2026.3.21.1` -> `v2026.3.21.2`)
- If the latest tag is from a different day, use today's base with no patch suffix
- Never ask the user for the version — always auto-calculate

## Step 2: Bump the version in source

Edit `internal/config/config.go` and update the `Version` variable to the new version string:

```go
var Version = "v{new_version}"
```

## Step 3: Commit and tag

Stage only the changed file and commit:

```bash
git add internal/config/config.go
git commit -m "v{new_version}"
git tag v{new_version}
```

Follow the user's commit message conventions (check CLAUDE.md for author instructions).

## Step 4: Push to trigger the release

```bash
git push origin main
git push origin v{new_version}
```

This triggers the CI workflow in `.github/workflows/ci.yml` which:
1. Runs tests via Dagger
2. Cross-compiles darwin/linux amd64/arm64 binaries
3. Creates a GitHub Release with tar.gz archives, standalone binaries, and checksums
4. Publishes the Homebrew formula to `agentic-metallurgy/homebrew-tap`

## Step 5: Report

Tell the user:
- The version that was released (e.g. `v2026.3.21`)
- That CI is now running and will publish the release
- Link: `https://github.com/agentic-metallurgy/ralph/actions`
