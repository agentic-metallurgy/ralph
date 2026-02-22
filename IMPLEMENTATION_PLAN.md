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

## TASK 8: Keyboard Shortcuts Rename + Visibility Fix [HIGH PRIORITY]
**Status: DONE**
- Spec: Final shortcut set should be `(q)uit (r)esume (p)ause (+) add loop (-) subtract loop`
- Spec: Only pause/resume should dim/illuminate based on state; all others always visible
- Spec: Make all token counts human-readable (individual breakdown fields were using raw `%d`)
- Changed `internal/tui/tui.go`:
  - Renamed key binding `"o"` → `"p"` for pause
  - Renamed key binding `"a"` → `"r"` for resume
  - Updated hotkey bar labels: `st(o)p`/`st(a)rt` → `(p)ause`/`(r)esume`
  - Updated hotkey bar labels: `(+)add`/`(-)subtract` → `(+) add loop`/`(-) subtract loop` (matches spec format)
  - Fixed visibility: `(+) add loop` and `(-) subtract loop` now always use `highlightStyle` (were incorrectly dim)
  - Only `(p)ause` and `(r)esume` dim/illuminate based on paused state
  - Changed individual token display (Input, Output, Cache Write, Cache Read) from `%d` to `stats.FormatTokens()` for human-readable formatting
- Updated tests in `tests/tui_test.go`:
  - Renamed `TestStopHotkey` → `TestPauseHotkey`: tests `'p'` key instead of `'o'`
  - Renamed `TestStartHotkey` → `TestResumeHotkey`: tests `'r'` key instead of `'a'`
  - Updated `TestCacheTokenBreakdownDisplayed`: checks for "12.3k"/"67.9k" instead of raw "12345"/"67890"
- Validation: all 211 tests pass, `go vet ./...` clean, `go build` succeeds

## TASK 9: Pause/Resume with Session Resumption [HIGH PRIORITY]
**Status: DONE**
- Spec: On resume after pause, use `claude --resume <session-id>` to continue from where it left off
- Spec: If the user pauses, quits, comes back, and does NOT resume, start a fresh Claude session
- Changed `internal/parser/parser.go`:
  - Added `SessionID string` field to `ParsedMessage` struct with `json:"session_id,omitempty"`
  - Added `GetSessionID(msg *ParsedMessage) string` method: returns session ID from system messages only
- Changed `internal/loop/loop.go`:
  - Added `sessionID string` and `resumeSessionID string` fields to Loop struct (mutex-protected)
  - Added `SetSessionID(id string)` method: stores latest session ID from Claude CLI output
  - Added `GetSessionID() string` method: retrieves current session ID
  - Updated `Pause()`: captures current `sessionID` into `resumeSessionID` for next resume
  - Updated `executeIteration()`: if `resumeSessionID` is set, appends `--resume <id>` to cmd.Args, then clears it
  - Fresh iterations (not after pause) never use `--resume` — each starts a new session
  - Quit and restart naturally starts fresh because no session ID persists across app restarts
- Changed `cmd/ralph/main.go`:
  - Added `claudeLoop` parameter to `processMessage()` function
  - In output processing, extracts session ID from system messages via `parser.GetSessionID()` and stores it in the loop via `claudeLoop.SetSessionID()`
- Updated mock `TestHelperProcess` in `tests/loop_test.go`:
  - Mock now detects `--resume` flag in args and outputs different session_id accordingly
  - Fresh sessions output `session_id: "fresh-session-001"`; resumed sessions echo back the provided session ID
  - Updated system message format to include `session_id` and `subtype` fields matching real Claude CLI output
- Added tests in `tests/parser_test.go`:
  - `TestParseLineSessionID`: verifies session_id parsing from system, assistant, and result messages
  - `TestGetSessionIDNilMessage`: verifies nil safety
  - `TestSessionIDFieldParsed`: verifies direct field access on parsed message
- Added tests in `tests/loop_test.go`:
  - `TestSetSessionID`: verifies Set/Get session ID round-trip
  - `TestGetSessionIDDefault`: verifies empty default
  - `TestResumeUsesSessionID`: end-to-end test — starts loop, captures session ID, pauses, resumes, verifies `--resume` flag was passed
  - `TestFreshIterationNoResume`: verifies normal iterations don't use `--resume`
  - `TestPauseCapturesSessionID`: verifies session ID preserved after pause
- Updated `TestLoopOutputMessages` assertion to match new mock output format
- Validation: all 219 tests pass, `go vet ./...` clean, `go build` succeeds

## TASK 10: Remove Old In-App Status Bar + Full-Width Tmux Bar [MEDIUM PRIORITY]
**Status: DONE**
- Spec: "the status bar we had before should be removed"
- Spec: "the status bar that we've moved into the tmux status bar should extend the full width and override the default tmux statusbar completely"
- Changed `internal/tui/tui.go`:
  - Removed `renderStatusBar()` method entirely (was rendering duplicate of tmux status bar content)
  - Removed status bar call from `renderFooter()` — footer now only renders panels + hotkey bar
  - Reduced `footerHeight` from 12 to 11 (one fewer row without status bar)
  - Changed panel height formula from `footerHeight - 4` to `footerHeight - 3` (no longer accounting for status bar row)
- Changed `internal/tmux/tmux.go`:
  - `NewStatusBar()` now sets `status-left ""` and `status-left-length 0` to clear the left side of the tmux bar
  - Increased `status-right-length` from 100 to 200 for full-width coverage
  - `Restore()` now also unsets `status-left` and `status-left-length` session overrides on cleanup
- Updated tests in `tests/tui_test.go`:
  - Replaced `TestStatusBarDisplayed` with `TestInAppStatusBarRemoved` (verifies old labels are NOT present)
  - Renamed `TestStatusBarShowsLoopProgress` → `TestFooterShowsLoopProgress`
  - Renamed `TestStatusBarShowsTokenCount` → `TestFooterShowsTokenCount`
  - Renamed `TestStatusBarDefaultLoopProgress` → `TestFooterDefaultLoopProgress`
  - Updated comment in `TestTimerPausesOnCompletion`
- Validation: all 219 tests pass, `go vet ./...` clean, `go build` succeeds

## TASK 11: Truncation Fix [MEDIUM PRIORITY]
**Status: DONE**
- Spec: Responses/thinking from Claude are sometimes truncated; they should not be
- Spec: Scrollback in main log window should be 100000 lines
- Root causes identified and fixed:
  1. Scanner buffer in `loop.go` was 1MB max — increased to 10MB to handle very large JSON lines from Claude CLI (tool results with full file contents, long assistant messages)
  2. Scanner errors were silently ignored — added `scanner.Err()` check in `streamOutput` that reports errors as `"error"` type messages on the output channel
  3. TUI message buffer (`maxMessages`) was 20 — increased to 100000 (spec requirement for scrollback)
- Fixed race condition: `streamOutput` goroutines could race against `close(l.output)` in `run()`. Added `sync.WaitGroup` in `executeIteration` to wait for both stdout/stderr goroutines to finish before returning. Also fixed pipe ordering: `wg.Wait()` is called before `cmd.Wait()` per Go docs ("it is incorrect to call Wait before all reads from the pipe have completed")
- Tool use `inputJSON` truncation to 150 chars is intentional display abbreviation — left as-is
- Changed `internal/loop/loop.go`:
  - Increased scanner max buffer from `1024*1024` (1MB) to `10*1024*1024` (10MB)
  - Added `scanner.Err()` check after scan loop — sends error message on output channel
  - Added `sync.WaitGroup` in `executeIteration` for stdout/stderr goroutine synchronization
  - Fixed pipe read ordering: `wg.Wait()` before `cmd.Wait()` to prevent "file already closed" errors
- Changed `internal/tui/tui.go`:
  - Increased `maxMessages` from 20 to 100000 (spec: scrollback should be 100000 lines)
- Added tests in `tests/loop_test.go`:
  - `TestLargeOutputNotTruncated`: verifies 1.5MB JSON lines pass through scanner without truncation or error
  - `TestScannerErrorReported`: verifies normal output does not produce spurious scanner errors
  - Added `mockLargeOutputCommandBuilder` and `"claude-large-output"` mock case in `TestHelperProcess`
- Added tests in `tests/tui_test.go`:
  - `TestScrollbackRetainsMessages`: verifies 50 messages are all retained (not dropped by old 20-message limit) and accessible via scrolling
- Validation: all 222 tests pass, `go vet ./...` clean, `go build` succeeds

## TASK 12: Integration Tests for Start/Pause/Start Flow [LOW PRIORITY]
**Status: TODO**
- Spec: Write integration tests for the start/pause/start flow
- Should test: start loop → pause → resume → verify continuation
- Make pause -> resume work based on test results
