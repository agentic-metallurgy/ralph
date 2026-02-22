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
**Status: DONE**
Added `plan` subcommand/mode that uses the embedded plan prompt (`specs/PROMPT_plan.md`) instead of the build prompt.
- Added `DetectSubcommand()` in `internal/config/config.go` — scans `os.Args` for "plan" before flag parsing and strips it so flags parse correctly
- Added `Subcommand` field to `Config` and `IsPlanMode()` accessor
- Embedded `plan_prompt.md` alongside `prompt.md` in `internal/prompt/assets/`
- Added `NewPlanLoader()`, `GetEmbeddedPlanPrompt()`, and `IsPlanMode()` to prompt package
- Updated `main.go` to select plan or build prompt based on subcommand (both `--show-prompt` and normal run)
- Updated usage message to show `[plan]` subcommand
- Added 8 tests: `TestLoadEmbeddedPlanPrompt`, `TestGetEmbeddedPlanPrompt`, `TestNewPlanLoader`, `TestPlanLoaderWithOverride`, `TestBuildAndPlanPromptsAreDifferent`, `TestPromptIsPlanMode`, `TestIsPlanMode`, `TestSubcommandFieldDefault`

## TASK 8: Highlight Quit Hotkey [LOW PRIORITY]
**Status: DONE**
Changed `(q)uit` hotkey to always use `highlightStyle` (bold, light gray) instead of `dimStyle`, matching spec requirement.
- Single change in `renderFooter()`: `quitKey` and `quitLabel` now use `highlightStyle`
- Added test: `TestQuitHotkeyAlwaysHighlighted`

## TASK 9: Remove Duplicate "Task" in Task Display [LOW PRIORITY]
**Status: DONE**
Strip leading "Task " from `currentTask` value in the footer to avoid "Task: Task 6: ..." display duplication.
- Added `strings.TrimPrefix(m.currentTask, "Task ")` in `renderFooter()` task display section
- Updated existing tests (`TestTaskUpdateDisplayed`, `TestTaskUpdateOverwritesPrevious`) to match new behavior
- Added 2 tests: `TestTaskDisplayStripsDuplicatePrefix`, `TestTaskDisplayWithoutPrefix`

## Notes
- All 128 tests pass (verified 2026-02-22)
- Go 1.25.3, BubbleTea TUI framework
- Build: `go build -o ralph ./cmd/ralph`
- Test: `go test -v ./tests/`
- `--version` outputs "ralph dev" in dev builds, version injected via goreleaser in releases
- All tasks from `specs/default.md` are now complete
