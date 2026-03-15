# BDD Test Suite Cleanup — Ambiguity Report

This report documents spec gaps, implicit requirements inferred from implementation, and remaining coverage gaps discovered during the BDD test suite cleanup (Tasks 1–8).

---

## 1. Spec Gaps Found

### 1.1 Undocumented `maxMessages` Cap

The TUI silently drops the oldest message when the activity feed exceeds 100,000 entries (`tui.go:182`). No spec documents this limit, its value, or the drop policy (oldest-first). The behavior is invisible to the user — no indicator appears when messages are being dropped.

**Gap:** No test verifies behavior at or beyond the cap boundary.

### 1.2 October Theme (Easter Egg)

`isOctober()` replaces the assistant icon (🤖 → 👻) during October. This is implemented via `timeNow()`, making it mockable, but it appears in no spec. It is not user-configurable.

**Gap:** Covered only by `tui_test.go`, not the BDD suite. Boundary behavior (Oct 1 exactly, Oct 31 → Nov 1) is untested.

### 1.3 Viewport Scroll Key Delegation

All scroll handling is delegated to `charmbracelet/bubbles/viewport` via `m.viewport.Update(msg)`. The TUI does not define custom scroll bindings. Keys supported by the viewport (Up/Down arrows, Home, End) work but are not listed in the hotkey bar and are untested.

**Gap:** Only PgUp/PgDown are tested. Arrow, Home, and End scroll keys have no test coverage.

### 1.4 `RoleThinking` Unused

`MessageRole = "thinking"` is defined with icon 💭 and italic dim gray style, but no code path ever sends a message with this role. It may be a forward-declared placeholder for extended thinking output.

**Gap:** No test exercises this role.

### 1.5 Activity Feed Line Spacing

`renderActivityContent()` inserts a blank line after every message. This spacing is critical for readability but is not specified anywhere and has no test coverage.

---

## 2. Implicit Requirements Inferred

### 2.1 Scroll Position Must Survive Ticks

The viewport content is refreshed every 250 ms (tick), but `GotoBottom()` is called only when new messages arrive — not on every tick. Without this invariant, manual scrolling would snap back to bottom every quarter-second, making history unreadable.

**Why it matters:** This is a critical UX constraint that falls out of a single comment (`// we do NOT call GotoBottom() here`) rather than any explicit spec.

### 2.2 Per-Loop vs. Cumulative Stats Are Tracked Separately

The TUI maintains two independent elapsed-time and token-count accumulators:
- **Cumulative** (footer): runs across all iterations, persisted to disk on quit
- **Per-loop** (tmux status bar): resets each time `loopStartedMsg` fires

The tmux bar shows per-loop tokens and time; the footer shows totals. This dual-tracking was inferred entirely from the field names (`loopTotalTokens`, `loopStartTime`, etc.) and the tmux update function — it is not documented in any spec.

### 2.3 Pause/Resume Are No-Ops Without a Running Loop

Pressing `p` or `r` when `m.loop == nil` is silently ignored (`if m.loop != nil { m.loop.Pause() }`). The hotkey bar still renders the key as active (dim vs bold), which could mislead users if the loop hasn't been set yet. This is unspecified behaviour.

### 2.4 Stats Are Persisted Only at Quit

`tokenStats.TotalElapsedNs` is accumulated in-memory and written to disk only when the user quits (via `q` or Ctrl+C). A crash or forced kill will lose the elapsed time for that session. No spec documents this persistence boundary.

### 2.5 Timer Freeze on Completion Is Mandatory UX

When the loop finishes (`doneMsg`), both the global and per-loop timers are frozen (`timerPaused = true`, `loopTimerPaused = true`). This ensures the footer shows the final build duration rather than continuing to advance. The freeze is load-bearing for correctness and tested (Tasks 3–4), but was never stated as a spec requirement.

### 2.6 Viewport Dimensions Are Guarded Against Zero

`max(value, 1)` guards are applied when computing viewport width and height from the terminal dimensions. This prevents a BubbleTea panic on very small or zero-dimension windows. The minimum terminal size check (40×15) is documented; the viewport guard is not.

---

## 3. Remaining Coverage Gaps

These are the LOW-priority gaps from `specs/cleanup.md` that were explicitly deferred:

| Gap | Status | Notes |
|-----|--------|-------|
| October theme in BDD suite | Not added | Covered adequately by 4 `tui_test.go` tests; BDD migration deferred |
| `maxMessages` boundary test | Not added | 100k cap has no test at or beyond the limit |
| Viewport scroll: Arrow/Home/End | Not added | Delegated to upstream widget; tested implicitly via PgUp/PgDown |
| `RoleThinking` in activity feed | Not added | Role is defined but never produced by any code path |
| Activity feed blank-line spacing | Not added | Rendered correctly; no regression risk currently |

All HIGH and MEDIUM gaps from `specs/cleanup.md` have been addressed:

| Gap | Task | Tests Added |
|-----|------|-------------|
| `loopRefMsg` / `SendLoopRef` | Task 6 | 6 BDD tests (`bdd_plan_build_test.go`) |
| Channel-based message flow | Task 7 | 7 BDD tests (`bdd_channel_test.go`) |
| tmux status bar content | Task 8 | 8 BDD tests (`bdd_tmux_test.go`) |

---

## 4. Decisions Made During Cleanup

| Decision | Rationale |
|----------|-----------|
| Kept `tokenStats.TotalElapsedNs` assertions in BDD tests | Non-observable in TUI; view cannot substitute for checking the persisted value |
| Kept real `time.Sleep` in goroutine-synchronization tests | `PauseShowsStoppedStatus`, `ResumeShowsRunningStatus`, `PauseResumeWithRealLoop`, `QuitFromPausedState` start a real goroutine and have no mock substitute |
| Replaced mock-able timer sleeps with `SetTimeNowForTest` | 7 tests that previously slept 10–1100 ms now run in under 1 ms each |
| Deleted 9 near-tautological/framework tests | Tests with no meaningful "When" action provide false coverage confidence |
| Removed 27 internal-state assertions from BDD tests | View assertions already cover the same behaviors; internal checks test implementation, not user-visible outcomes |
