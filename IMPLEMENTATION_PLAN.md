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

- [ ] **Render `currentTask` in the TUI footer.** The `Model.currentTask` field (tui.go:127) is set via `SetCurrentTask()` and updated by `taskUpdateMsg`, but never rendered. The footer shows `"Completed Tasks: X/Y"` at tui.go:750 but the current task string is not displayed anywhere. Add it to the footer right panel, e.g., `"Current Task: TASK 6"` between the completed tasks and current mode lines.

- [ ] **Mutex-protect `running` and `paused` fields in loop.go.** The `mu` mutex (loop.go:47) documents it protects `config.Iterations`, `sessionID`, `resumeSessionID`, `completedWaiting`, and hibernate state. But `running` and `paused` are read/written from multiple goroutines without synchronization: `IsRunning()` (line 102), `IsPaused()` (line 107), `Start()` (line 87), `Stop()` (line 98), `Pause()` (line 113), `Resume()` (line 128), and the `run()` goroutine (line 240). These are data races. Fix: use the existing `mu` mutex for all reads/writes of `running` and `paused`.

- [ ] **Deduplicate message processing logic.** There are six structurally similar message processing code paths in main.go: `handleParsedMessage()`, `handleParsedMessagePlanAndBuild()`, inline in `runCLI()`, and twice inline in `runPlanAndBuildCLI()` (plan + build phases). Extract a shared message processing function that accepts callbacks or configuration for mode-specific behavior (e.g., whether to call loop.Hibernate, whether to output to TUI vs stdout).

- [ ] **Remove dead code path in main.go:140-141.** The `cfg.IsPlanAndBuildMode()` check inside the standard TUI branch (lines 138-144) can never be true — execution enters the `runPlanAndBuild` branch at line 101 and returns. This `model.SetCurrentMode("Planning")` at line 141 is unreachable.

## P3: Dead Code Cleanup

- [ ] **Remove unused TUI entry points.** `Run()` (tui.go:919), `RunWithChannels()` (tui.go:926), and `CreateProgram()` (tui.go:934) are not called from any production code. `main.go` constructs the program directly via `tea.NewProgram()`.

- [ ] **Remove unused `colorBg` variable.** Defined at tui.go:42 as `lipgloss.Color("#1A1B26")` but never referenced in any style construction.

- [ ] **Remove unused `GetCurrentSessionName()` function.** Defined at tmux.go:73-86, fully implemented and exported, but has zero callers in the codebase.

## P4: Test Gaps

- [ ] **Add tests for `parser.ExtractFilePathFromInput()`.** This function (parser.go:344-369) has five branches (`file_path`, `path`, `pattern`, `command` with truncation, `description`) and zero test coverage.

- [ ] **Add tests for `tui.SendTaskUpdate()` and `tui.SetCurrentTask()`.** Both are exported functions with no test coverage. The `taskUpdateMsg` handling in `Update()` (tui.go:499-501) is untested.

- [ ] **Add tests for `loop.SetResumeSessionID()`.** This method (loop.go:229-233) is used for plan-and-build session chaining but has no direct test.

- [ ] **Add test for `ExtractContent` with `map[string]interface{}` tool result content.** The map branch at parser.go:251-254 (marshals map to JSON) is untested.

- [ ] **Add tests for `RoleLoop` and `RoleLoopStopped` icons/styles outside October.** Currently only verified inside the October-specific test `TestOctoberOtherRolesUnchanged`.
