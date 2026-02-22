# Implementation Plan

Based on `specs/default.md`, the following tasks are needed:

## TASK 1: Rename "Agents:" to "Active Agents:" [HIGH PRIORITY]
**Status: DONE**
- Spec: Change 'Agents:' to `Active Agents: x`
- Changed label in `internal/tui/tui.go` from "Agents:" to "Active Agents:"
- Updated `labelStyle` width from 14 to 15 to accommodate the longer label
- Updated test assertions in `tests/tui_test.go` (TestAgentCountDisplayed, TestAgentCountZeroAfterReset)

## TASK 2: Auto-Pause Timer When Iterations Complete [HIGH PRIORITY]
**Status: DONE**
- Spec: total time count should be paused when iterations are reached and loop isn't running anymore
- Added `completed` field to Model struct
- In `doneMsg` handler: sets `completed = true`, freezes elapsed time via `timerPaused`/`pausedElapsed`
- Status display now shows "Completed" (green) in both header and footer panel when done
- Header border turns green on completion (blue = running, red = stopped, green = completed)
- Added `SendDone()` helper for testing
- Added `TestTimerPausesOnCompletion` test

## TASK 3: Tmux Statusbar Integration [MEDIUM PRIORITY]
**Status: DONE**
- Spec: bottom tmux statusbar should show `[current loop: #1/1   tokens: 128.58m   elapsed: 07:18:00]` as the actual tmux statusbar (not a separate TUI line)
- Added `StatusBar` struct to `internal/tmux/tmux.go` with `Update(content)` and `Restore()` methods
- `NewStatusBar()` detects if inside tmux; if not, returns inactive no-op bar
- On init, sets `status-right-length` to 100 (default 40 is too short)
- Uses `tmux set-option status-right "..."` (session-level) to set the bar content
- `Restore()` uses `tmux set-option -u` to unset session overrides, falling back to global defaults
- Added `FormatStatusRight()` helper that produces the spec-matching format string
- Added `tmuxBar *tmux.StatusBar` field to TUI Model struct with `SetTmuxStatusBar()` setter
- `updateTmuxStatusBar()` called on every tick (250ms) with current loop/token/elapsed values
- On quit (`q`/Ctrl+C), calls `tmuxBar.Restore()` to clean up
- Wired up in `cmd/ralph/main.go`: creates `tmux.NewStatusBar()` and attaches to model
- Added `TickMsgForTest()` exported helper for test access to tick messages
- All nil/inactive paths are safe (nil receiver checks on StatusBar methods)
- Tests: `TestNewStatusBar_NotInTmux`, `TestStatusBarUpdate_Inactive`, `TestStatusBarRestore_Inactive`, `TestStatusBarNilSafe`, `TestFormatStatusRight`, `TestFormatStatusRight_ZeroValues`, `TestSetTmuxStatusBar`
- Validation: all 148 tests pass, `go vet ./...` clean, `go build` succeeds

## TASK 4: Fix Log Window Scrollability [LOW PRIORITY]
**Status: DONE**
- Spec: log window should be scrollable
- Bug found: `tickMsg` handler called `m.viewport.GotoBottom()` every 250ms, snapping the viewport to the bottom on every tick. This made scrolling completely unusable — any user scroll would be overridden within 250ms.
- Fix: Removed `m.viewport.GotoBottom()` from the `tickMsg` handler in `internal/tui/tui.go`. The viewport still auto-scrolls to bottom on init (`WindowSizeMsg` handler) and when new messages arrive (`newMessageMsg` handler), which is the correct behavior.
- Keyboard scrolling (arrow keys, PgUp/PgDn) is handled by the viewport component via `m.viewport.Update(msg)` at the end of the `Update` function — unhandled keys fall through from the `KeyMsg` case to the viewport update.
- Added `TestViewportScrollPreservedOnTick` test: verifies that after scrolling up via PgUp keys, a tick does not snap the viewport back to bottom.
- Validation: all 149 tests pass, `go vet ./...` clean, `go build` succeeds

## TASK 5: Current Task Display Format [MEDIUM PRIORITY]
**Status: TODO**
- Spec: Change Task into `Current Task: <task title>` e.g. "Current Task: #6 Change the lib/gold into lib/silver"
- Spec: Add `Completed Tasks: count_completed/total_tasks` e.g. "Completed Tasks: 4/7"
- Note: task numbers and completed counts may be out of order (highest priority first)
- Requires changes to: `internal/tui/tui.go` (footer rendering), task tracking in `cmd/ralph/main.go`

## TASK 6: Plan Prompt Ultimate Goal [MEDIUM PRIORITY]
**Status: TODO**
- Spec: Add `--goal` flag to `ralph plan` for specifying an ultimate goal sentence
- Spec: Add placeholder in `plan_prompt.md`: "ULTIMATE GOAL: $ultimate_goal_sentence. Consider missing elements and plan accordingly."
- If `--goal` is not provided, derive from specs/ content or leave empty
- Requires changes to: `internal/config/config.go` (new flag), `internal/prompt/prompt.go` (template substitution), `internal/prompt/assets/plan_prompt.md`

## TASK 7: Add/Subtract Loop Keyboard Shortcuts [MEDIUM PRIORITY]
**Status: TODO**
- Spec: Add keyboard shortcuts `(+)` add and `(-)` subtract loops
- Floor constraint: can't subtract below current loop number (e.g., on loop 4, minimum is 4 loops)
- Shortcuts should appear in the bottom hotkey bar
- Requires changes to: `internal/tui/tui.go` (key handlers, hotkey bar), `internal/loop/loop.go` (dynamic iteration count adjustment)

## TASK 8: Commit plan_prompt.md Fix [LOW PRIORITY]
**Status: TODO**
- Spec: "also i fixed a problem with the plan_prompt.md please commit that along with things"
- Note: This appears to refer to a user-made fix that should be included in the next commit. Verify if `plan_prompt.md` has uncommitted changes.
