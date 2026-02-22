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
**Status: DONE**
Added a centered status bar between the footer panels and the hotkey bar showing:
```
[current loop: #3/5      tokens: 36.87m      elapsed: 01:23:45]
```
- Added `getElapsed()` helper to avoid duplicating elapsed time calculation
- Added `renderStatusBar()` method with purple labels and light gray values
- Integrated into `renderFooter()` between panels and hotkey bar
- Increased `footerHeight` from 11 to 12; panel height adjusted to `footerHeight - 4`
- Uses `FormatTokens()` for human-readable token count
- Added 4 tests: `TestStatusBarDisplayed`, `TestStatusBarShowsLoopProgress`, `TestStatusBarShowsTokenCount`, `TestStatusBarDefaultLoopProgress`

## TASK 7: Add `ralph plan` Subcommand [LOW PRIORITY]
**Status: TODO**
Add a `plan` subcommand/mode that uses `specs/PROMPT_plan.md` as the loop prompt instead of the build prompt. Requires:
- Detect `plan` as a positional argument before flag parsing
- Embed `PROMPT_plan.md` alongside `prompt.md` in the `internal/prompt/assets/` directory
- When `ralph plan` is run, use the plan prompt instead of the build prompt
- May need to restructure CLI argument parsing to support subcommands

## TASK 8: Highlight Quit Hotkey [LOW PRIORITY]
**Status: TODO**
Spec says: "light up the 'quit' option, just like we light up start, when we stop (even though it's available during running, too)". This means the `(q)uit` hotkey should always be highlighted (bold, light gray) regardless of running/paused state, matching how `st(a)rt` is highlighted when paused. Currently `(q)uit` is always dim gray.
- Single change in `renderFooter()`: set `quitKey` and `quitLabel` to use `highlightStyle` instead of `dimStyle`

## TASK 9: Remove Duplicate "Task" in Task Display [LOW PRIORITY]
**Status: TODO**
Spec says: "Task: should remove 'Task' from the title of the task it's showing so it doesn't look like `Task: Task: 6`". When `currentTask` starts with "Task", the label already says "Task:" so it would display as `Task: Task 6: ...`. Should strip leading "Task" from the value.
- Single change in `renderFooter()` task display section

## Notes
- All 116 tests pass (verified 2026-02-22)
- Go 1.25.3, BubbleTea TUI framework
- Build: `go build -o ralph ./cmd/ralph`
- Test: `go test -v ./tests/`
- `--version` outputs "ralph dev" in dev builds, version injected via goreleaser in releases
