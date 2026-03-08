# Implementation Plan

Spec: `specs/default.md` — The TUI shows too much verbose file dump output. It should only show agent thinking, and responses pertinent to a human following along. It should not show file dumps.

---

## P0: Core Spec Requirements (Verbosity Changes)

- [x] **Filter out file dumps (tool_result content) in TUI mode.** Both `handleParsedMessage()` and `handleParsedMessagePlanAndBuild()` now skip sending `MessageTypeUser` tool_result content to `msgChan`. Task reference scanning is still performed on tool results. CLI mode was already correct.

- [x] **Display thinking blocks in the TUI activity feed.** Added `RoleThinking` to TUI with 💭 icon and dim/italic style (`colorDimGray`). Both `handleParsedMessage()` and `handleParsedMessagePlanAndBuild()` now check `content.Thinking` and send it to `msgChan` as `RoleThinking`.

- [x] **Improve tool use display.** Added `FilePath` field to `parser.ToolUse` struct, populated during `ExtractContent()` via `ExtractFilePathFromInput()`. Both TUI handlers now display `"Using tool: Read — /path/to/file"` format when a file path is available.

## P1: Functional Bugs / Missing Behavior

- [x] **Add rate-limit/hibernate handling to plan-and-build mode.** Added `claudeLoop *loop.Loop` parameter to `handleParsedMessagePlanAndBuild()` with rate limit check (`IsRateLimitRejected` + `Hibernate` + `SendHibernate`). Both `processPlanPhase()` and `processBuildPhase()` now forward their respective loop references. Also added rate limit handling to both plan and build phases of `runPlanAndBuildCLI()`.

- [x] **Enable loop control hotkeys in plan-and-build TUI mode.** Added `loopRefMsg` / `SendLoopRef()` to tui.go so the loop reference can be swapped at runtime. `runPlanAndBuildPhases()` now sends `SendLoopRef(planLoop)` during plan phase and `SendLoopRef(buildLoop)` during build phase, enabling all hotkeys (p/r/s/+/-) to work in plan-and-build mode.

- [x] **Hibernate should capture resumeSessionID.** Added `l.resumeSessionID = l.sessionID` inside `Hibernate()` under the mutex, mirroring `Pause()` logic. After waking from rate limit, the retried iteration now correctly uses `--resume` with the captured session ID.

## P2: Code Quality / Correctness

- [x] **Render `currentTask` in the TUI footer.** Added `"Current Task:"` line to the Ralph Loop Details panel in `renderFooter()`, positioned between "Completed Tasks:" and "Current Mode:". Shows the task text (e.g., `#6 Refactor config`) or `"-"` when no task is set. No footerHeight change needed — the new 8th content line fills the existing Height(8) panel exactly. Four tests added: display, default dash, message update, and ordering.

- [x] **Mutex-protect `running` and `paused` fields in loop.go.** Added `mu` locking around all reads/writes of `running` and `paused`: `Start()`, `Stop()`, `IsRunning()`, `IsPaused()`, `Pause()`, `Resume()`, `run()` deferred cleanup, and both pause-check points in `run()`. The mutex comment now lists `running` and `paused`. `Pause()` was refactored to check and set `paused`/`running`/`resumeSessionID` in a single critical section. `Resume()` similarly checks and clears `paused` under the lock. All tests pass including `-race`.

- [x] **Deduplicate message processing logic.** Deleted `handleParsedMessagePlanAndBuild()` (was identical to `handleParsedMessage()`). Extracted `handleLoopMarker()` shared by `processMessage()`, `processPlanPhase()`, and `processBuildPhase()`. Extracted `handleParsedMessageCLI()` shared by `runCLI()` and both phases of `runPlanAndBuildCLI()`. Reduced main.go from 1204 to ~890 lines (~26% reduction) while preserving all behavior. All tests pass including `-race`.

- [x] **Remove dead code path in main.go:140-141.** Removed the unreachable `else if cfg.IsPlanAndBuildMode()` branch from the standard TUI mode's `SetCurrentMode` block. The condition could never be true because plan-and-build mode returns early at line 101. Simplified to a two-branch `if cfg.IsPlanMode()` / `else` pattern.

## P3: Dead Code Cleanup

- [x] **Remove unused TUI entry points.** Deleted `Run()`, `RunWithChannels()`, and `CreateProgram()` from tui.go. Also removed `TestCreateProgram` from tui_test.go (only caller). No production code used any of these — `main.go` constructs the program directly via `tea.NewProgram()`.

- [x] **Remove unused `colorBg` variable.** Removed from tui.go color palette. Was never referenced in any style construction.

- [x] **Remove unused `GetCurrentSessionName()` function.** Removed from tmux.go along with now-unused `strings` import. Had zero callers in the codebase.

## P4: Test Gaps

- [ ] **Add tests for `parser.ExtractFilePathFromInput()`.** This function (parser.go:344-369) has five branches (`file_path`, `path`, `pattern`, `command` with truncation, `description`) and zero test coverage.

- [ ] **Add tests for `tui.SendTaskUpdate()` and `tui.SetCurrentTask()`.** Both are exported functions with no test coverage. The `taskUpdateMsg` handling in `Update()` (tui.go:499-501) is untested.

- [ ] **Add tests for `loop.SetResumeSessionID()`.** This method (loop.go:229-233) is used for plan-and-build session chaining but has no direct test.

- [ ] **Add test for `ExtractContent` with `map[string]interface{}` tool result content.** The map branch at parser.go:251-254 (marshals map to JSON) is untested.

- [ ] **Add tests for `RoleLoop` and `RoleLoopStopped` icons/styles outside October.** Currently only verified inside the October-specific test `TestOctoberOtherRolesUnchanged`.
