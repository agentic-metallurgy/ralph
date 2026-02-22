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
**Status: DONE**
- Spec: Change Task into `Current Task: <task title>` e.g. "Current Task: #6 Change the lib/gold into lib/silver"
- Spec: Add `Completed Tasks: count_completed/total_tasks` e.g. "Completed Tasks: 4/7"
- Changed `internal/tui/tui.go`:
  - Renamed "Task:" label to "Current Task:" in footer
  - Added "Completed Tasks:" row showing "X/Y" (completed/total) in footer
  - Added `completedTasks` and `totalTasks` fields to Model struct
  - Added `completedTasksUpdateMsg` message type and `SendCompletedTasksUpdate()` helper
  - Added `SetCompletedTasks(completed, total int)` setter method
  - Increased `labelStyle` width from 15 to 17 to accommodate "Completed Tasks:" label
  - Removed "Task " prefix stripping (no longer needed with new "#N Description" format)
- Changed `cmd/ralph/main.go`:
  - Added `parseTaskCounts(filepath)` to read IMPLEMENTATION_PLAN.md and count DONE/total tasks at startup
  - Changed task label format from "Task N: Description" to "#N Description" (spec format)
  - Calls `model.SetCompletedTasks()` with initial counts from plan file
- Updated tests in `tests/tui_test.go`:
  - Updated `TestTaskDisplayDefault` to check for "Current Task:" label
  - Updated `TestTaskUpdateDisplayed` for "#N Description" format
  - Updated `TestTaskUpdateOverwritesPrevious` for new format
  - Replaced `TestTaskDisplayStripsDuplicatePrefix` with `TestCurrentTaskDisplayFormat`
  - Replaced `TestTaskDisplayWithoutPrefix` with `TestTaskDisplayWithoutDescription`
  - Added `TestCompletedTasksDefault`, `TestCompletedTasksUpdate`, `TestSendCompletedTasksUpdateCmd`, `TestSetCompletedTasks`
- Validation: all 153 tests pass, `go vet ./...` clean, `go build` succeeds

## TASK 6: Plan Prompt Ultimate Goal [MEDIUM PRIORITY]
**Status: DONE**
- Spec: Add `--goal` flag to `ralph plan` for specifying an ultimate goal sentence
- Spec: Add placeholder in `plan_prompt.md`: "ULTIMATE GOAL: $ultimate_goal_sentence. Consider missing elements and plan accordingly."
- If `--goal` is not provided, placeholder and trailing ". " are removed for clean output
- Changed `internal/config/config.go`:
  - Added `Goal string` field to Config struct
  - Added `--goal` flag: `flag.StringVar(&cfg.Goal, "goal", "", "Ultimate goal sentence for plan mode (used in plan prompt)")`
- Changed `internal/prompt/assets/plan_prompt.md`:
  - Inserted `$ultimate_goal_sentence. ` placeholder before "Consider missing elements" in the ULTIMATE GOAL line
- Changed `internal/prompt/prompt.go`:
  - Added `goal string` field to Loader struct
  - Changed `NewPlanLoader(overridePath, goal string)` signature to accept goal
  - Added `substituteGoal(content, goal string)` function: if goal non-empty, replaces placeholder with goal text (trailing period trimmed to avoid double-period); if goal empty, removes placeholder + ". " for clean output
  - `Load()` now calls `substituteGoal()` after loading content in plan mode
  - Goal substitution also applies to override files (plan mode with `--loop-prompt`)
  - `GetEmbeddedPlanPrompt()` now reads raw template directly (returns unsubstituted placeholder) for introspection
- Changed `cmd/ralph/main.go`:
  - Updated `--show-prompt` handler to use Loader (so goal substitution is visible when debugging prompts)
  - Updated `NewPlanLoader` call to pass `cfg.Goal`
- Added tests in `tests/prompt_test.go`:
  - `TestPlanPromptGoalSubstitution`: verifies goal text replaces placeholder
  - `TestPlanPromptGoalEmpty`: verifies placeholder is cleanly removed when no goal
  - `TestPlanPromptGoalWithTrailingPeriod`: verifies no double period when goal ends with "."
  - `TestPlanPromptGoalDoesNotAffectBuildPrompt`: verifies build prompt is unaffected
  - Updated `TestGetEmbeddedPlanPrompt` to verify raw template contains placeholder
  - Updated `TestPlanLoaderWithOverride` to test goal substitution in override files
  - Updated all `NewPlanLoader("")` calls to `NewPlanLoader("", "")`
- Added tests in `tests/config_test.go`:
  - `TestGoalFieldDefault`: verifies Goal defaults to empty
  - `TestGoalFieldSet`: verifies Goal can be set
- Validation: all 161 tests pass, `go vet ./...` clean, `go build` succeeds

## TASK 7: Add/Subtract Loop Keyboard Shortcuts [MEDIUM PRIORITY]
**Status: DONE**
- Spec: Add keyboard shortcuts `(+)` add and `(-)` subtract loops
- Spec: Floor constraint: can't subtract below current loop number (e.g., on loop 4, minimum is 4 loops)
- Spec: Shortcuts should appear in the bottom hotkey bar as `(+)add` and `(-)subtract`
- Changed `internal/loop/loop.go`:
  - Added `sync.Mutex` (`mu`) field to Loop struct for thread-safe iteration adjustment
  - Added `SetIterations(n int)` method: mutex-protected setter for `config.Iterations`
  - Added `GetIterations() int` method: mutex-protected getter for `config.Iterations`
  - Updated `run()` to use `l.GetIterations()` instead of direct `l.config.Iterations` access in for-loop condition, all message `Total` fields, and sleep check
  - Updated `streamOutput()` to use `l.GetIterations()` for message `Total` field
- Changed `internal/tui/tui.go`:
  - Added `+` key handler: increments `m.totalLoops` and calls `m.loop.SetIterations(m.totalLoops)`, guarded by `m.loop != nil && !m.completed`
  - Added `-` key handler: decrements `m.totalLoops` and calls `m.loop.SetIterations(m.totalLoops)`, with floor constraint `m.totalLoops > m.currentLoop`
  - Updated hotkey bar to show `(+)add` and `(-)subtract` alongside existing `(q)uit`, `st(o)p`, `st(a)rt`
- Added tests in `tests/loop_test.go`:
  - `TestSetIterations`: verifies SetIterations/GetIterations round-trip
  - `TestGetIterationsDefault`: verifies GetIterations returns initial config value
  - `TestSetIterationsDuringRun`: verifies dynamically increasing iterations causes extra loops to execute
- Added tests in `tests/tui_test.go`:
  - `TestAddLoopHotkey`: verifies '+' increases totalLoops and updates loop's iteration count
  - `TestSubtractLoopHotkey`: verifies '-' decreases totalLoops and updates loop's iteration count
  - `TestSubtractLoopFloorConstraint`: verifies '-' is a no-op when currentLoop == totalLoops
  - `TestAddLoopNoopWithoutLoop`: verifies '+' is a no-op when no loop is set
  - `TestAddSubtractLoopNoopWhenCompleted`: verifies '+'/'-' are no-ops after completion
  - `TestHotkeyBarShowsAddSubtract`: verifies hotkey bar contains 'add' and 'subtract'
  - `TestMultipleAddLoopPresses`: verifies pressing '+' multiple times accumulates correctly
- Validation: all 211 tests pass, `go vet ./...` clean, `go build` succeeds

