# Implementation Plan

Based on analysis of `specs/default.md` against the current codebase.

## TASK 1: Update Embedded Build Prompt [HIGH PRIORITY]
**Status: DONE**
Replaced `internal/prompt/assets/prompt.md` with the content from `specs/PROMPT_build.md`. The embedded prompt now includes rules for parallel subagents, AGENTS.md updates, no placeholders, etc.

## TASK 2: Add `ralph --version` Flag [HIGH PRIORITY]
**Status: DONE**
- Added `Version` variable to `internal/config/config.go` (default: "dev", set via `-ldflags` at build time)
- Added `--version` flag to `ParseFlags()`
- Added early exit in `main()` printing `ralph <version>`
- Updated `.goreleaser.yaml` to inject version via `-X` ldflags

## TASK 3: Rename "Loop Details" to "Ralph Details" [HIGH PRIORITY]
**Status: DONE**
Changed panel title from "Loop Details" to "Ralph Details" in `internal/tui/tui.go`.

## TASK 4: Human-Readable Token Counts [HIGH PRIORITY]
**Status: DONE**
- Added `FormatTokens()` function to `internal/stats/stats.go` (formats as "36.87m", "300k", "1.5k", "42")
- Applied to Total Tokens display in Usage & Cost panel
- Added comprehensive tests in `tests/stats_test.go`

## TASK 5: Left-Align Hotkey Bar [MEDIUM PRIORITY]
**Status: DONE**
Changed hotkey bar from `Align(lipgloss.Center)` to `Align(lipgloss.Left)` with `PaddingLeft(1)`.

## TASK 6: Add Bottom Status Bar [MEDIUM PRIORITY]
**Status: TODO**
Add a colored status bar below the footer panels showing:
```
[current loop: #3/5      tokens: xxxxxx      elapsed: 33:99:00]
```
This is a new UI element distinct from the existing footer panels. Needs:
- A new render function for the status bar
- Integration into the footer layout
- Human-readable token count using `FormatTokens()`
- Elapsed time formatting matching the existing format

## TASK 7: Add `ralph plan` Subcommand [LOW PRIORITY]
**Status: TODO**
Add a `plan` subcommand/mode that uses `specs/PROMPT_plan.md` as the loop prompt instead of the build prompt. Requires:
- Detect `plan` as a positional argument before flag parsing
- Embed `PROMPT_plan.md` alongside `prompt.md` in the `internal/prompt/assets/` directory
- When `ralph plan` is run, use the plan prompt instead of the build prompt
- May need to restructure CLI argument parsing to support subcommands

## Notes
- All 112 tests pass (verified 2026-02-22)
- Go 1.25.3, BubbleTea TUI framework
- Build: `go build -o ralph ./cmd/ralph`
- Test: `go test -v ./tests/`
- `--version` outputs "ralph dev" in dev builds, version injected via goreleaser in releases
