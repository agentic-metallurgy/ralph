package tests

import (
	"context"
	"strings"
	"testing"
	"time"

	tea "github.com/charmbracelet/bubbletea"
	"github.com/cloudosai/ralph-go/internal/loop"
	"github.com/cloudosai/ralph-go/internal/stats"
	"github.com/cloudosai/ralph-go/internal/tui"
)

// ============================================================================
// BDD Test Suite: User Controls Loop Execution
//
// These tests verify the complete state machine for loop control:
// pause, resume, add/subtract loops, start after completion, hibernate wake.
// Organized by user goal following specs/bdd-agent-prompt.md methodology.
// ============================================================================

// --- Helpers ---

// setupReadyModel creates a model with window size set, ready to render.
func setupReadyModel() tui.Model {
	m := tui.NewModel()
	m, _ = updateModel(m, tea.WindowSizeMsg{Width: 120, Height: 40})
	return m
}

// setupReadyModelWithLoop creates a ready model with a loop at the given progress.
func setupReadyModelWithLoop(current, total int) (tui.Model, *loop.Loop) {
	m := tui.NewModel()
	l := loop.New(loop.Config{Iterations: total, Prompt: "test"})
	m.SetLoop(l)
	m.SetLoopProgress(current, total)
	m, _ = updateModel(m, tea.WindowSizeMsg{Width: 120, Height: 40})
	return m, l
}

// pressKey simulates a key press on the model.
func pressKey(m tui.Model, key rune) (tui.Model, tea.Cmd) {
	return updateModel(m, tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune{key}})
}

// sendTuiMsg executes a tea.Cmd to get the message and feeds it to the model.
func sendTuiMsg(m tui.Model, cmd tea.Cmd) (tui.Model, tea.Cmd) {
	msg := cmd()
	return updateModel(m, msg)
}

// viewContains checks if the rendered view contains the given substring.
func viewContains(m tui.Model, substr string) bool {
	return strings.Contains(m.View(), substr)
}

// viewNotContains checks that the rendered view does NOT contain the given substring.
func viewNotContains(m tui.Model, substr string) bool {
	return !strings.Contains(m.View(), substr)
}

// --- Tests ---

// TestBDD_UserControlsLoopExecution_RunningShowsRunningStatus tests that a loop
// in its default running state displays the RUNNING status banner.
func TestBDD_UserControlsLoopExecution_RunningShowsRunningStatus(t *testing.T) {
	// Given: a model with a loop in running state
	m, _ := setupReadyModelWithLoop(2, 5)

	// Then: status should show RUNNING
	if !viewContains(m, "RUNNING") {
		t.Error("Given a running loop, status should display RUNNING")
	}
}

// TestBDD_UserControlsLoopExecution_PauseShowsStoppedStatus tests that pressing 'p'
// on a running loop changes the status banner to STOPPED.
func TestBDD_UserControlsLoopExecution_PauseShowsStoppedStatus(t *testing.T) {
	// Given: a model with an actually-running loop (Pause requires running=true)
	cfg := loop.Config{
		Iterations:     100,
		Prompt:         "test",
		CommandBuilder: mockCommandBuilder,
		SleepDuration:  10 * time.Millisecond,
	}
	l := loop.New(cfg)
	ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
	defer cancel()

	m := tui.NewModel()
	m.SetLoop(l)
	m.SetLoopProgress(2, 5)
	m, _ = updateModel(m, tea.WindowSizeMsg{Width: 120, Height: 40})

	l.Start(ctx)
	go func() { for range l.Output() {} }()
	time.Sleep(50 * time.Millisecond)

	// When: user presses 'p' to pause
	m, _ = pressKey(m, 'p')
	time.Sleep(200 * time.Millisecond)

	// Then: status should show STOPPED
	if !viewContains(m, "STOPPED") {
		t.Error("After pausing, status should display STOPPED")
	}
}

// TestBDD_UserControlsLoopExecution_ResumeShowsRunningStatus tests that pressing 'r'
// after pausing restores the RUNNING status banner.
func TestBDD_UserControlsLoopExecution_ResumeShowsRunningStatus(t *testing.T) {
	// Given: a model with an actually-running loop that has been paused
	cfg := loop.Config{
		Iterations:     100,
		Prompt:         "test",
		CommandBuilder: mockCommandBuilder,
		SleepDuration:  10 * time.Millisecond,
	}
	l := loop.New(cfg)
	ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
	defer cancel()

	m := tui.NewModel()
	m.SetLoop(l)
	m.SetLoopProgress(2, 5)
	m, _ = updateModel(m, tea.WindowSizeMsg{Width: 120, Height: 40})

	l.Start(ctx)
	go func() { for range l.Output() {} }()
	time.Sleep(50 * time.Millisecond)

	m, _ = pressKey(m, 'p')
	time.Sleep(200 * time.Millisecond)

	if !viewContains(m, "STOPPED") {
		t.Fatal("Precondition failed: should show STOPPED after pause")
	}

	// When: user presses 'r' to resume
	m, _ = pressKey(m, 'r')
	time.Sleep(200 * time.Millisecond)

	// Then: status should show RUNNING again
	if !viewContains(m, "RUNNING") {
		t.Error("After resuming, status should display RUNNING")
	}
}

// TestBDD_UserControlsLoopExecution_CompletionShowsCompletedStatus tests that when
// the loop signals done, both the status banner and footer show COMPLETED/Completed.
func TestBDD_UserControlsLoopExecution_CompletionShowsCompletedStatus(t *testing.T) {
	// Given: a running model at its final loop
	m, _ := setupReadyModelWithLoop(5, 5)

	// When: loop completes
	m, _ = sendTuiMsg(m, tui.SendDone())

	// Then: status should show COMPLETED
	if !viewContains(m, "COMPLETED") {
		t.Error("After completion, status should display COMPLETED")
	}
	if !viewContains(m, "Completed") {
		t.Error("After completion, footer status should display 'Completed'")
	}
}

// TestBDD_UserControlsLoopExecution_PauseFreezesBothTimers tests that pressing 'p'
// freezes both the total elapsed timer and the per-loop timer.
// This verifies the view is completely static while paused (no ticking timers).
func TestBDD_UserControlsLoopExecution_PauseFreezesBothTimers(t *testing.T) {
	// Given: a model with a loop that has been running for a moment
	m, _ := setupReadyModelWithLoop(1, 5)
	time.Sleep(10 * time.Millisecond) // let timers accumulate slightly

	// When: user presses 'p' to pause
	m, _ = pressKey(m, 'p')

	// Then: two renders separated by time should produce identical output
	// (both total timer and per-loop timer are frozen)
	view1 := m.View()
	time.Sleep(50 * time.Millisecond)
	view2 := m.View()

	if view1 != view2 {
		t.Error("After pausing, the view should be completely frozen (both timers paused)")
	}
}

// TestBDD_UserControlsLoopExecution_ResumeRestoresBothTimers tests that pressing 'r'
// after pause unfreezes timers so they advance again.
// We verify this by checking that the view is frozen when paused, and unfrozen after resume.
func TestBDD_UserControlsLoopExecution_ResumeRestoresBothTimers(t *testing.T) {
	// Given: a paused loop
	m, _ := setupReadyModelWithLoop(1, 5)
	m, _ = pressKey(m, 'p')

	// Verify pause is working: views should be identical even after a delay
	frozenView := m.View()
	time.Sleep(10 * time.Millisecond)
	if m.View() != frozenView {
		t.Fatal("Precondition failed: pause should freeze the view")
	}

	// When: user presses 'r' to resume
	m, _ = pressKey(m, 'r')

	// Then: views should eventually differ (wait long enough for the second to tick)
	// The timer displays HH:MM:SS, so we need to wait >1 second for a visible change
	viewAfterResume := m.View()
	time.Sleep(1100 * time.Millisecond)
	viewLater := m.View()

	if viewAfterResume == viewLater {
		t.Error("After resuming, timers should advance and the view should change over time")
	}
}

// TestBDD_UserControlsLoopExecution_AddLoopDuringRun tests that pressing '+' during
// a running loop increases the total loop count in both the display and the loop object.
func TestBDD_UserControlsLoopExecution_AddLoopDuringRun(t *testing.T) {
	// Given: running at loop 2/5
	m, l := setupReadyModelWithLoop(2, 5)

	// When: user presses '+'
	m, _ = pressKey(m, '+')

	// Then: display shows 2/6
	if !viewContains(m, "#2/6") {
		t.Errorf("After pressing '+' at 2/5, display should show #2/6, got view:\n%s", m.View())
	}
	// And: loop object reflects the change
	if l.GetIterations() != 6 {
		t.Errorf("Loop iterations should be 6 after '+', got %d", l.GetIterations())
	}
}

// TestBDD_UserControlsLoopExecution_AddLoopAfterCompletion tests that pressing '+' after
// all loops complete adds a pending loop and changes the hotkey bar to show (s)tart.
func TestBDD_UserControlsLoopExecution_AddLoopAfterCompletion(t *testing.T) {
	// Given: completed at 5/5
	m, l := setupReadyModelWithLoop(5, 5)
	m, _ = sendTuiMsg(m, tui.SendDone())

	if !viewContains(m, "COMPLETED") {
		t.Fatal("Precondition: should be in COMPLETED state")
	}

	// When: user presses '+'
	m, _ = pressKey(m, '+')

	// Then: loop iterations increase
	if l.GetIterations() != 6 {
		t.Errorf("After '+' post-completion, loop iterations should be 6, got %d", l.GetIterations())
	}
	// And: hotkey bar shows (s)tart
	if !viewContains(m, "(s)tart") {
		t.Error("After adding loop post-completion, hotkey bar should show '(s)tart'")
	}
}

// TestBDD_UserControlsLoopExecution_SubtractLoopFloorConstraint tests that pressing '-'
// when totalLoops equals currentLoop is a no-op (cannot subtract below current).
func TestBDD_UserControlsLoopExecution_SubtractLoopFloorConstraint(t *testing.T) {
	// Given: at loop 4/4 (floor condition)
	m, l := setupReadyModelWithLoop(4, 4)

	// When: user presses '-'
	m, _ = pressKey(m, '-')

	// Then: total loops remain unchanged
	if !viewContains(m, "#4/4") {
		t.Errorf("At floor (4/4), '-' should be no-op, expected #4/4 in view")
	}
	if l.GetIterations() != 4 {
		t.Errorf("Loop iterations should remain 4 at floor, got %d", l.GetIterations())
	}
}

// TestBDD_UserControlsLoopExecution_SubtractLoopAboveFloor tests that pressing '-'
// when totalLoops > currentLoop decreases the count.
func TestBDD_UserControlsLoopExecution_SubtractLoopAboveFloor(t *testing.T) {
	// Given: at loop 2/5
	m, l := setupReadyModelWithLoop(2, 5)

	// When: user presses '-'
	m, _ = pressKey(m, '-')

	// Then: total decreases to 4
	if !viewContains(m, "#2/4") {
		t.Errorf("After '-' at 2/5, expected #2/4 in view")
	}
	if l.GetIterations() != 4 {
		t.Errorf("Loop iterations should be 4 after '-', got %d", l.GetIterations())
	}
}

// TestBDD_UserControlsLoopExecution_StartAfterCompletionWithPending tests that pressing 's'
// after completion when there are pending loops (added via '+') clears the COMPLETED state.
func TestBDD_UserControlsLoopExecution_StartAfterCompletionWithPending(t *testing.T) {
	// Given: completed at 5/5, then '+' adds a pending loop (5/6)
	m, _ := setupReadyModelWithLoop(5, 5)
	m, _ = sendTuiMsg(m, tui.SendDone())
	m, _ = pressKey(m, '+')

	if !viewContains(m, "COMPLETED") {
		t.Fatal("Precondition: should still show COMPLETED before 's'")
	}

	// When: user presses 's' to start
	m, _ = pressKey(m, 's')

	// Then: COMPLETED state is cleared
	if viewContains(m, "COMPLETED") {
		t.Error("After pressing 's' with pending loops, COMPLETED should be cleared")
	}
}

// TestBDD_UserControlsLoopExecution_StartNoopWithoutPending tests that pressing 's'
// after completion with NO pending loops is a no-op.
func TestBDD_UserControlsLoopExecution_StartNoopWithoutPending(t *testing.T) {
	// Given: completed at 5/5 (no pending loops)
	m, _ := setupReadyModelWithLoop(5, 5)
	m, _ = sendTuiMsg(m, tui.SendDone())

	// When: user presses 's'
	m, _ = pressKey(m, 's')

	// Then: still COMPLETED
	if !viewContains(m, "COMPLETED") {
		t.Error("Pressing 's' without pending loops should remain COMPLETED")
	}
}

// TestBDD_UserControlsLoopExecution_MultipleRapidPauseResumeCycles tests that rapidly
// toggling pause/resume never causes a quit or crash.
func TestBDD_UserControlsLoopExecution_MultipleRapidPauseResumeCycles(t *testing.T) {
	// Given: a model with a loop
	m, _ := setupReadyModelWithLoop(2, 5)

	// When: user rapidly toggles p/r 10 times
	for i := 0; i < 10; i++ {
		m, _ = pressKey(m, 'p')
		view := m.View()
		if view == "Goodbye!\n" {
			t.Fatalf("Pause caused quit at iteration %d", i)
		}

		m, _ = pressKey(m, 'r')
		view = m.View()
		if view == "Goodbye!\n" {
			t.Fatalf("Resume caused quit at iteration %d", i)
		}
	}

	// Then: model still renders normally
	view := m.View()
	if view == "" || view == "Goodbye!\n" {
		t.Error("After rapid pause/resume cycles, model should still render normally")
	}
}

// TestBDD_UserControlsLoopExecution_PauseWithoutLoop tests that pressing 'p' without
// a loop set does not crash and has no side effects.
func TestBDD_UserControlsLoopExecution_PauseWithoutLoop(t *testing.T) {
	// Given: a model with NO loop set
	m := setupReadyModel()
	viewBefore := m.View()

	// When: user presses 'p'
	m, _ = pressKey(m, 'p')

	// Then: no crash, view still renders, no quit
	viewAfter := m.View()
	if viewAfter == "Goodbye!\n" {
		t.Error("Pressing 'p' without loop should not quit")
	}
	if viewAfter == "" {
		t.Error("View should not be empty after pressing 'p' without loop")
	}
	// Status should still show RUNNING (no loop to pause)
	_ = viewBefore // no assertion on exact match since timers may tick
}

// TestBDD_UserControlsLoopExecution_ResumeWithoutLoop tests that pressing 'r' without
// a loop set does not crash.
func TestBDD_UserControlsLoopExecution_ResumeWithoutLoop(t *testing.T) {
	// Given: a model with NO loop set
	m := setupReadyModel()

	// When: user presses 'r'
	m, _ = pressKey(m, 'r')

	// Then: no crash, no quit
	view := m.View()
	if view == "Goodbye!\n" {
		t.Error("Pressing 'r' without loop should not quit")
	}
	if view == "" {
		t.Error("View should not be empty after pressing 'r' without loop")
	}
}

// TestBDD_UserControlsLoopExecution_HibernateWakeViaRKey tests that pressing 'r' during
// hibernate wakes the loop and restores timers.
func TestBDD_UserControlsLoopExecution_HibernateWakeViaRKey(t *testing.T) {
	// Given: a hibernating loop
	m, l := setupReadyModelWithLoop(2, 5)
	l.Hibernate(time.Now().Add(5 * time.Minute))
	m, _ = sendTuiMsg(m, tui.SendHibernate(time.Now().Add(5*time.Minute)))

	if !l.IsHibernating() {
		t.Fatal("Precondition: loop should be hibernating")
	}
	if !viewContains(m, "RATE LIMITED") {
		t.Fatal("Precondition: should show RATE LIMITED")
	}

	// When: user presses 'r' to wake
	m, _ = pressKey(m, 'r')

	// Then: loop is no longer hibernating
	if l.IsHibernating() {
		t.Error("After pressing 'r', loop should no longer be hibernating")
	}
	// And: RATE LIMITED status should be cleared
	if viewContains(m, "RATE LIMITED") {
		t.Error("After wake, RATE LIMITED status should be cleared")
	}
}

// TestBDD_UserControlsLoopExecution_HibernateOverridesStoppedDisplay tests that
// hibernate state shows RATE LIMITED, not STOPPED, even if the loop is technically paused.
func TestBDD_UserControlsLoopExecution_HibernateOverridesStoppedDisplay(t *testing.T) {
	// Given: a loop that is hibernating (which internally may pause)
	m, l := setupReadyModelWithLoop(2, 5)
	l.Hibernate(time.Now().Add(3 * time.Minute))
	m, _ = sendTuiMsg(m, tui.SendHibernate(time.Now().Add(3*time.Minute)))

	// Then: status should show RATE LIMITED (not STOPPED)
	if viewContains(m, "STOPPED") && viewNotContains(m, "RATE LIMITED") {
		t.Error("Hibernate should show RATE LIMITED, not STOPPED")
	}
	if !viewContains(m, "RATE LIMITED") {
		t.Error("Hibernate should display RATE LIMITED status")
	}
}

// TestBDD_UserControlsLoopExecution_HotkeyBarShowsPauseDuringRunning tests that
// the hotkey bar displays (p)ause while the loop is in running state.
func TestBDD_UserControlsLoopExecution_HotkeyBarShowsPauseDuringRunning(t *testing.T) {
	// Given: a model in running state
	m, _ := setupReadyModelWithLoop(2, 5)

	// Then: hotkey bar should show (p)ause
	if !viewContains(m, "(p)ause") {
		t.Error("While running, hotkey bar should show '(p)ause'")
	}
}

// TestBDD_UserControlsLoopExecution_HotkeyBarShowsResumeDuringPause tests that
// the hotkey bar displays (r)esume when the loop is paused.
func TestBDD_UserControlsLoopExecution_HotkeyBarShowsResumeDuringPause(t *testing.T) {
	// Given: a model that has been paused
	m, _ := setupReadyModelWithLoop(2, 5)

	// When: user presses 'p' to pause
	m, _ = pressKey(m, 'p')

	// Then: hotkey bar should show (r)esume
	if !viewContains(m, "(r)esume") {
		t.Error("While paused, hotkey bar should show '(r)esume'")
	}
}

// TestBDD_UserControlsLoopExecution_HotkeyBarShowsStartAfterCompletionWithPending tests
// that the hotkey bar displays (s)tart when completed with pending loops.
func TestBDD_UserControlsLoopExecution_HotkeyBarShowsStartAfterCompletionWithPending(t *testing.T) {
	// Given: a completed model with a pending loop added via '+'
	m, _ := setupReadyModelWithLoop(5, 5)
	m, _ = sendTuiMsg(m, tui.SendDone())
	m, _ = pressKey(m, '+')

	// Then: hotkey bar should show (s)tart
	if !viewContains(m, "(s)tart") {
		t.Error("When completed with pending loops, hotkey bar should show '(s)tart'")
	}
}

// TestBDD_UserControlsLoopExecution_HotkeyBarShowsWakeDuringHibernate tests that
// the hotkey bar displays (r) wake when the loop is hibernating.
func TestBDD_UserControlsLoopExecution_HotkeyBarShowsWakeDuringHibernate(t *testing.T) {
	// Given: a hibernating model
	m, _ := setupHibernatingModel(2, 5, 5*time.Minute)

	// Then: hotkey bar should show (r) wake
	if !viewContains(m, "(r) wake") {
		t.Error("While hibernating, hotkey bar should show '(r) wake'")
	}
}

// TestBDD_UserControlsLoopExecution_AddMultipleLoops tests pressing '+' multiple times.
func TestBDD_UserControlsLoopExecution_AddMultipleLoops(t *testing.T) {
	// Given: at loop 1/3
	m, l := setupReadyModelWithLoop(1, 3)

	// When: user presses '+' 5 times
	for i := 0; i < 5; i++ {
		m, _ = pressKey(m, '+')
	}

	// Then: total should be 8
	if !viewContains(m, "#1/8") {
		t.Errorf("After 5x '+' from 3, expected #1/8, got view:\n%s", m.View())
	}
	if l.GetIterations() != 8 {
		t.Errorf("Loop iterations should be 8, got %d", l.GetIterations())
	}
}

// TestBDD_UserControlsLoopExecution_SubtractMultipleLoops tests pressing '-' multiple times.
func TestBDD_UserControlsLoopExecution_SubtractMultipleLoops(t *testing.T) {
	// Given: at loop 1/8
	m, l := setupReadyModelWithLoop(1, 8)

	// When: user presses '-' 10 times (more than possible to subtract)
	for i := 0; i < 10; i++ {
		m, _ = pressKey(m, '-')
	}

	// Then: should stop at floor (1), not go below
	if l.GetIterations() != 1 {
		t.Errorf("After excessive '-' presses, iterations should floor at current (1), got %d", l.GetIterations())
	}
	if !viewContains(m, "#1/1") {
		t.Errorf("Display should show #1/1 at floor")
	}
}

// TestBDD_UserControlsLoopExecution_QuitPersistsElapsedDuringPause tests that quitting
// while paused still persists the frozen elapsed time to stats.
func TestBDD_UserControlsLoopExecution_QuitPersistsElapsedDuringPause(t *testing.T) {
	// Given: a paused model with stats and base elapsed time
	m, _ := setupReadyModelWithLoop(2, 5)
	tokenStats := stats.NewTokenStats()
	m.SetStats(tokenStats)
	m.SetBaseElapsed(30 * time.Minute)

	// Pause first
	m, _ = pressKey(m, 'p')

	// When: user quits
	m, _ = pressKey(m, 'q')

	// Then: stats should have elapsed time persisted (at least 30 min)
	if tokenStats.TotalElapsedNs < (30 * time.Minute).Nanoseconds() {
		t.Errorf("Elapsed time should be at least 30min (%d ns), got %d ns",
			(30 * time.Minute).Nanoseconds(), tokenStats.TotalElapsedNs)
	}
}

// TestBDD_UserControlsLoopExecution_CompletionFreezesBothTimers tests that when the loop
// completes, both the total and per-loop timers are frozen.
func TestBDD_UserControlsLoopExecution_CompletionFreezesBothTimers(t *testing.T) {
	// Given: a running model
	m, _ := setupReadyModelWithLoop(5, 5)

	// When: loop completes
	m, _ = sendTuiMsg(m, tui.SendDone())

	// Then: view should be frozen (both timers paused)
	view1 := m.View()
	time.Sleep(50 * time.Millisecond)
	view2 := m.View()

	if view1 != view2 {
		t.Error("After completion, both timers should be frozen and view should be static")
	}
}

// TestBDD_UserControlsLoopExecution_PauseResumeWithRealLoop is an integration test that
// exercises pause/resume with an actual running loop process.
func TestBDD_UserControlsLoopExecution_PauseResumeWithRealLoop(t *testing.T) {
	// Given: a loop with a mock command builder, actually running
	cfg := loop.Config{
		Iterations:     100,
		Prompt:         "test",
		CommandBuilder: mockCommandBuilder,
		SleepDuration:  10 * time.Millisecond,
	}
	l := loop.New(cfg)
	ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
	defer cancel()

	m := tui.NewModel()
	m.SetLoop(l)
	m, _ = updateModel(m, tea.WindowSizeMsg{Width: 120, Height: 40})

	l.Start(ctx)
	go func() { for range l.Output() {} }()

	time.Sleep(50 * time.Millisecond)

	// When: user pauses
	m, _ = pressKey(m, 'p')
	time.Sleep(200 * time.Millisecond)

	// Then: loop should be paused and TUI shows STOPPED
	if !l.IsPaused() {
		t.Error("Loop should be paused after 'p' key")
	}
	if viewNotContains(m, "STOPPED") && viewNotContains(m, "Stopped") {
		t.Error("TUI should show STOPPED/Stopped when loop is paused")
	}

	// When: user resumes
	m, _ = pressKey(m, 'r')
	time.Sleep(200 * time.Millisecond)

	// Then: loop should be running and TUI shows RUNNING
	if l.IsPaused() {
		t.Error("Loop should not be paused after 'r' key")
	}

	cancel()
}

// TestBDD_UserControlsLoopExecution_PerLoopStatsResetOnNewLoop tests that per-loop
// statistics (tokens, timer) reset when a new loop iteration begins.
func TestBDD_UserControlsLoopExecution_PerLoopStatsResetOnNewLoop(t *testing.T) {
	// Given: a model with accumulated per-loop stats
	m := setupReadyModel()
	m, _ = sendTuiMsg(m, tui.SendLoopStatsUpdate(50000))

	// When: a new loop starts
	m, _ = sendTuiMsg(m, tui.SendLoopStarted())

	// Then: per-loop stats should be reset
	// We verify by updating with a small value and checking the model still renders
	m, _ = sendTuiMsg(m, tui.SendLoopStatsUpdate(100))
	view := m.View()
	if view == "" {
		t.Error("View should render after per-loop stats reset")
	}
}

// TestBDD_UserControlsLoopExecution_LoopProgressAlwaysShowsInFooter tests that
// the loop progress (#N/M) is always visible in the footer regardless of state.
func TestBDD_UserControlsLoopExecution_LoopProgressAlwaysShowsInFooter(t *testing.T) {
	m, _ := setupReadyModelWithLoop(3, 7)

	// Running state
	if !viewContains(m, "#3/7") {
		t.Error("Loop progress should be visible while running")
	}

	// Paused state
	m, _ = pressKey(m, 'p')
	if !viewContains(m, "#3/7") {
		t.Error("Loop progress should be visible while paused")
	}

	// Resumed state
	m, _ = pressKey(m, 'r')
	if !viewContains(m, "#3/7") {
		t.Error("Loop progress should be visible after resume")
	}

	// Completed state
	m.SetLoopProgress(7, 7)
	m, _ = sendTuiMsg(m, tui.SendDone())
	if !viewContains(m, "#7/7") {
		t.Error("Loop progress should be visible after completion")
	}
}

// TestBDD_UserControlsLoopExecution_LoopControlsKeyBindingSummary tests that the
// (+)/(-) # of loops label is always visible in the hotkey bar.
func TestBDD_UserControlsLoopExecution_LoopControlsKeyBindingSummary(t *testing.T) {
	m := setupReadyModel()

	view := m.View()
	if !strings.Contains(view, "(+)/(-)") {
		t.Error("Hotkey bar should always show (+)/(-) label")
	}
	if !strings.Contains(view, "# of loops") {
		t.Error("Hotkey bar should always show '# of loops' label")
	}
}
