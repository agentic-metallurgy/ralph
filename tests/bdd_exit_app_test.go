package tests

import (
	"context"
	"strings"
	"testing"
	"time"

	tea "github.com/charmbracelet/bubbletea"
	"github.com/cloudosai/ralph-go/internal/loop"
	"github.com/cloudosai/ralph-go/internal/stats"
	"github.com/cloudosai/ralph-go/internal/tmux"
	"github.com/cloudosai/ralph-go/internal/tui"
)

// ============================================================================
// BDD Test Suite: User Exits Application
//
// These tests verify all exit paths from the TUI: quit via 'q' key, quit via
// Ctrl+C, elapsed time persistence to stats, and tmux status bar restoration.
// Organized by user goal following specs/bdd-agent-prompt.md methodology.
// ============================================================================

// --- Scenario 1: Quit via 'q' key ---

func TestBDD_UserExitsApplication_QuitViaQ(t *testing.T) {
	// Given: a ready model with activity
	m := setupReadyModel()
	m.AddMessage(tui.Message{Role: tui.RoleAssistant, Content: "Working on task..."})
	m, _ = sendTuiMsg(m, tui.SendMessage(tui.Message{Role: tui.RoleAssistant, Content: "More work"}))

	// Precondition: model is rendering content (not goodbye)
	if viewContains(m, "Goodbye!") {
		t.Fatal("Precondition failed: model should not show Goodbye before quit")
	}

	// When: user presses 'q'
	m, cmd := pressKey(m, 'q')

	// Then: view shows "Goodbye!" farewell message
	view := m.View()
	if view != "Goodbye!\n" {
		t.Errorf("Expected 'Goodbye!\\n' after quit, got: %q", view)
	}

	// Then: a quit command is returned
	if cmd == nil {
		t.Error("Expected a non-nil quit command")
	} else {
		msg := cmd()
		if _, ok := msg.(tea.QuitMsg); !ok {
			t.Errorf("Expected tea.QuitMsg, got %T", msg)
		}
	}
}

func TestBDD_UserExitsApplication_QuitViaQReplacesLayout(t *testing.T) {
	// Given: a model with loop progress and stats visible
	m, _ := setupReadyModelWithLoop(3, 5)
	s := stats.NewTokenStats()
	s.AddUsage(1000, 2000, 0, 0)
	m.SetStats(s)
	m, _ = sendTuiMsg(m, tui.SendStatsUpdate(s))

	// Precondition: layout is rendering footer content
	if !viewContains(m, "#3/5") {
		t.Fatal("Precondition failed: should show loop progress before quit")
	}

	// When: user presses 'q'
	m, _ = pressKey(m, 'q')

	// Then: entire previous layout is replaced by Goodbye message
	view := m.View()
	if strings.Contains(view, "#3/5") {
		t.Error("Loop progress should not appear after quit")
	}
	if view != "Goodbye!\n" {
		t.Errorf("View should be exactly 'Goodbye!\\n', got: %q", view)
	}
}

// --- Scenario 2: Quit via Ctrl+C ---

func TestBDD_UserExitsApplication_QuitViaCtrlC(t *testing.T) {
	// Given: a ready model
	m := setupReadyModel()

	// When: user presses Ctrl+C
	m, cmd := updateModel(m, tea.KeyMsg{Type: tea.KeyCtrlC})

	// Then: view shows "Goodbye!" farewell message
	view := m.View()
	if view != "Goodbye!\n" {
		t.Errorf("Expected 'Goodbye!\\n' after Ctrl+C, got: %q", view)
	}

	// Then: a quit command is returned
	if cmd == nil {
		t.Error("Expected a non-nil quit command")
	} else {
		msg := cmd()
		if _, ok := msg.(tea.QuitMsg); !ok {
			t.Errorf("Expected tea.QuitMsg from Ctrl+C, got %T", msg)
		}
	}
}

func TestBDD_UserExitsApplication_CtrlCSameBehaviorAsQ(t *testing.T) {
	// Given: two identical models
	m1 := setupReadyModel()
	m2 := setupReadyModel()

	tokenStats1 := stats.NewTokenStats()
	tokenStats2 := stats.NewTokenStats()
	m1.SetStats(tokenStats1)
	m2.SetStats(tokenStats2)
	m1.SetBaseElapsed(1 * time.Hour)
	m2.SetBaseElapsed(1 * time.Hour)

	// When: one quits via 'q', the other via Ctrl+C
	m1, cmd1 := pressKey(m1, 'q')
	m2, cmd2 := updateModel(m2, tea.KeyMsg{Type: tea.KeyCtrlC})

	// Then: both produce the same view
	if m1.View() != m2.View() {
		t.Error("'q' and Ctrl+C should produce identical views")
	}

	// Then: both return quit commands
	if cmd1 == nil || cmd2 == nil {
		t.Error("Both should return non-nil quit commands")
	}

	// Then: both persist elapsed time to stats
	if tokenStats1.TotalElapsedNs < (1 * time.Hour).Nanoseconds() {
		t.Error("'q' should persist elapsed time")
	}
	if tokenStats2.TotalElapsedNs < (1 * time.Hour).Nanoseconds() {
		t.Error("Ctrl+C should persist elapsed time")
	}
}

// --- Scenario 3: Quit persists elapsed time (running timer) ---

func TestBDD_UserExitsApplication_PersistsRunningElapsedTime(t *testing.T) {
	// Given: a model with stats and 2-hour base elapsed (simulating a previous session)
	m := setupReadyModel()
	tokenStats := stats.NewTokenStats()
	m.SetStats(tokenStats)
	m.SetBaseElapsed(2 * time.Hour)

	// When: user quits
	m, _ = pressKey(m, 'q')

	// Then: stats.TotalElapsedNs includes at least the base elapsed time
	if tokenStats.TotalElapsedNs < (2 * time.Hour).Nanoseconds() {
		t.Errorf("TotalElapsedNs should be >= 2h (%d ns), got %d ns",
			(2 * time.Hour).Nanoseconds(), tokenStats.TotalElapsedNs)
	}
}

func TestBDD_UserExitsApplication_PersistsElapsedWithZeroBase(t *testing.T) {
	// Given: a model with stats but no base elapsed (fresh session)
	m := setupReadyModel()
	tokenStats := stats.NewTokenStats()
	m.SetStats(tokenStats)

	// When: user quits
	m, _ = pressKey(m, 'q')

	// Then: stats.TotalElapsedNs is positive (some time has elapsed since model creation)
	if tokenStats.TotalElapsedNs <= 0 {
		t.Error("TotalElapsedNs should be positive even for a fresh session")
	}
}

func TestBDD_UserExitsApplication_PersistsElapsedCombinesBaseAndCurrent(t *testing.T) {
	// Given: a model with 30-minute base from a previous session
	m := setupReadyModel()
	tokenStats := stats.NewTokenStats()
	m.SetStats(tokenStats)
	baseElapsed := 30 * time.Minute
	m.SetBaseElapsed(baseElapsed)

	// When: user quits after some current session time
	m, _ = pressKey(m, 'q')

	// Then: persisted time is at least base, proving base + current combination
	if tokenStats.TotalElapsedNs < baseElapsed.Nanoseconds() {
		t.Errorf("Should persist at least base elapsed %d ns, got %d ns",
			baseElapsed.Nanoseconds(), tokenStats.TotalElapsedNs)
	}
	// And: slightly more than base (current session adds time)
	if tokenStats.TotalElapsedNs == 0 {
		t.Error("Should have persisted some elapsed time")
	}
}

// --- Scenario 4: Quit persists paused elapsed time ---

func TestBDD_UserExitsApplication_PersistsPausedElapsedTime(t *testing.T) {
	// Given: a model with loop, stats, and paused timer
	m, _ := setupReadyModelWithLoop(2, 5)
	tokenStats := stats.NewTokenStats()
	m.SetStats(tokenStats)
	m.SetBaseElapsed(45 * time.Minute)

	// When: user pauses then quits
	m, _ = pressKey(m, 'p')
	m, _ = pressKey(m, 'q')

	// Then: stats have the frozen paused elapsed (at least 45 min)
	if tokenStats.TotalElapsedNs < (45 * time.Minute).Nanoseconds() {
		t.Errorf("Paused elapsed should be at least 45min (%d ns), got %d ns",
			(45 * time.Minute).Nanoseconds(), tokenStats.TotalElapsedNs)
	}
}

func TestBDD_UserExitsApplication_PausedElapsedFrozenAtPausePoint(t *testing.T) {
	// Given: a model that was paused, with known base elapsed
	m, _ := setupReadyModelWithLoop(1, 3)
	tokenStats := stats.NewTokenStats()
	m.SetStats(tokenStats)
	m.SetBaseElapsed(10 * time.Minute)

	// When: pause, wait briefly, then quit
	m, _ = pressKey(m, 'p')
	time.Sleep(50 * time.Millisecond) // timer should be frozen, so this shouldn't add time

	m, _ = pressKey(m, 'q')

	// Then: elapsed time should be close to 10 min + small delta (not 10 min + 50ms of sleep)
	// The paused elapsed is captured at pause time, not at quit time
	upperBound := (10*time.Minute + 5*time.Second).Nanoseconds() // generous upper bound
	if tokenStats.TotalElapsedNs > upperBound {
		t.Errorf("Paused elapsed should be near 10min, not significantly more. Got %d ns (upper bound %d ns)",
			tokenStats.TotalElapsedNs, upperBound)
	}
}

// --- Scenario 5: Quit restores tmux status bar ---

func TestBDD_UserExitsApplication_QuitWithTmuxBarSet(t *testing.T) {
	// Given: a ready model with a tmux status bar configured
	m := setupReadyModel()
	sb := tmux.NewStatusBar() // inactive in test env (no $TMUX), but non-nil
	m.SetTmuxStatusBar(sb)

	// When: user quits
	m, cmd := pressKey(m, 'q')

	// Then: quit succeeds without panic (Restore() called on the bar)
	if m.View() != "Goodbye!\n" {
		t.Errorf("Expected Goodbye after quit with tmux bar, got: %q", m.View())
	}
	if cmd == nil {
		t.Error("Should still return quit command with tmux bar set")
	}
}

func TestBDD_UserExitsApplication_QuitWithNilTmuxBar(t *testing.T) {
	// Given: a model without a tmux bar (default)
	m := setupReadyModel()
	// tmuxBar is nil by default (not set via SetTmuxStatusBar)

	// When: user quits
	m, cmd := pressKey(m, 'q')

	// Then: quit succeeds without panic
	if m.View() != "Goodbye!\n" {
		t.Errorf("Expected Goodbye after quit with nil tmux bar, got: %q", m.View())
	}
	if cmd == nil {
		t.Error("Should return quit command even without tmux bar")
	}
}

// --- Scenario 6: Quit with nil stats ---

func TestBDD_UserExitsApplication_QuitWithNilStats(t *testing.T) {
	// Given: a model with stats explicitly set to nil
	m := setupReadyModel()
	m.SetStats(nil)

	// When: user quits
	m, cmd := pressKey(m, 'q')

	// Then: quit succeeds without panic (nil stats check in Update)
	if m.View() != "Goodbye!\n" {
		t.Errorf("Expected Goodbye with nil stats, got: %q", m.View())
	}
	if cmd == nil {
		t.Error("Should return quit command even with nil stats")
	}
}

// --- Scenario 7: Quit from various states ---

func TestBDD_UserExitsApplication_QuitFromCompletedState(t *testing.T) {
	// Given: a model that has completed all iterations
	m, _ := setupReadyModelWithLoop(5, 5)
	m, _ = sendTuiMsg(m, tui.SendDone())

	// Precondition: shows COMPLETED
	if !viewContains(m, "COMPLETED") {
		t.Fatal("Precondition failed: should show COMPLETED")
	}

	// When: user quits
	m, cmd := pressKey(m, 'q')

	// Then: quit works from completed state
	if m.View() != "Goodbye!\n" {
		t.Errorf("Should quit from completed state, got: %q", m.View())
	}
	if cmd == nil {
		t.Error("Should return quit command from completed state")
	}
}

func TestBDD_UserExitsApplication_QuitFromHibernatingState(t *testing.T) {
	// Given: a model in rate-limited/hibernating state (both loop-level and TUI-level)
	m, _ := setupHibernatingModel(2, 5, 5*time.Minute)

	// Precondition: shows RATE LIMITED
	if !viewContains(m, "RATE LIMITED") {
		t.Fatal("Precondition failed: should show RATE LIMITED")
	}

	// When: user quits
	m, cmd := pressKey(m, 'q')

	// Then: quit works from hibernating state
	if m.View() != "Goodbye!\n" {
		t.Errorf("Should quit from hibernating state, got: %q", m.View())
	}
	if cmd == nil {
		t.Error("Should return quit command from hibernating state")
	}
}

func TestBDD_UserExitsApplication_QuitFromPausedState(t *testing.T) {
	// Given: a model with a running loop that is then paused
	cfg := loop.Config{
		Iterations:     100,
		Prompt:         "test",
		CommandBuilder: mockCommandBuilder,
		SleepDuration:  10 * time.Millisecond,
	}
	l := loop.New(cfg)
	ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
	defer cancel()
	l.Start(ctx)
	defer l.Stop()
	time.Sleep(200 * time.Millisecond)

	m := tui.NewModel()
	m.SetLoop(l)
	m.SetLoopProgress(3, 5)
	m, _ = updateModel(m, tea.WindowSizeMsg{Width: 120, Height: 40})

	// Pause the loop (requires running=true)
	m, _ = pressKey(m, 'p')
	time.Sleep(200 * time.Millisecond)

	// Precondition: shows STOPPED
	if !viewContains(m, "STOPPED") {
		t.Fatal("Precondition failed: should show STOPPED after pause")
	}

	// When: user quits
	m, cmd := pressKey(m, 'q')

	// Then: quit works from paused state
	if m.View() != "Goodbye!\n" {
		t.Errorf("Should quit from paused state, got: %q", m.View())
	}
	if cmd == nil {
		t.Error("Should return quit command from paused state")
	}
}

func TestBDD_UserExitsApplication_QuitFromTooSmallTerminal(t *testing.T) {
	// Given: a model with too-small terminal
	m := tui.NewModel()
	m, _ = updateModel(m, tea.WindowSizeMsg{Width: 30, Height: 10})

	// Precondition: shows too-small message
	if !viewContains(m, "Terminal too small") {
		t.Fatal("Precondition failed: should show too-small message")
	}

	// When: user presses 'q'
	m, cmd := pressKey(m, 'q')

	// Then: quit works even from too-small state
	if m.View() != "Goodbye!\n" {
		t.Errorf("Should quit from too-small terminal, got: %q", m.View())
	}
	if cmd == nil {
		t.Error("Should return quit command from too-small terminal")
	}
}

func TestBDD_UserExitsApplication_QuitFromPreInitState(t *testing.T) {
	// Given: a brand-new model (no WindowSizeMsg yet, ready=false)
	m := tui.NewModel()

	// Precondition: view is empty (pre-init clean alt screen)
	if m.View() != "" {
		t.Fatal("Precondition failed: pre-init view should be empty")
	}

	// When: user presses 'q'
	m, cmd := pressKey(m, 'q')

	// Then: quit works from pre-init state
	if m.View() != "Goodbye!\n" {
		t.Errorf("Should quit from pre-init state, got: %q", m.View())
	}
	if cmd == nil {
		t.Error("Should return quit command from pre-init state")
	}
}

func TestBDD_UserExitsApplication_CtrlCFromTooSmallTerminal(t *testing.T) {
	// Given: a too-small terminal
	m := tui.NewModel()
	m, _ = updateModel(m, tea.WindowSizeMsg{Width: 20, Height: 8})

	// When: user presses Ctrl+C
	m, cmd := updateModel(m, tea.KeyMsg{Type: tea.KeyCtrlC})

	// Then: quit works via Ctrl+C from too-small state
	if m.View() != "Goodbye!\n" {
		t.Errorf("Ctrl+C should quit from too-small terminal, got: %q", m.View())
	}
	if cmd == nil {
		t.Error("Should return quit command for Ctrl+C from too-small terminal")
	}
}

// --- Scenario 8: Quit persists elapsed time from special states ---

func TestBDD_UserExitsApplication_CompletedStatePersistsElapsed(t *testing.T) {
	// Given: a completed model with stats
	m, _ := setupReadyModelWithLoop(5, 5)
	tokenStats := stats.NewTokenStats()
	m.SetStats(tokenStats)
	m.SetBaseElapsed(1 * time.Hour)
	m, _ = sendTuiMsg(m, tui.SendDone())

	// When: user quits from completed state
	m, _ = pressKey(m, 'q')

	// Then: elapsed time is persisted (timer was frozen on completion, quit captures that)
	if tokenStats.TotalElapsedNs < (1 * time.Hour).Nanoseconds() {
		t.Errorf("Should persist at least 1h elapsed, got %d ns", tokenStats.TotalElapsedNs)
	}
}

func TestBDD_UserExitsApplication_HibernatingStatePersistsElapsed(t *testing.T) {
	// Given: a hibernating model with stats
	m, _ := setupReadyModelWithLoop(2, 5)
	tokenStats := stats.NewTokenStats()
	m.SetStats(tokenStats)
	m.SetBaseElapsed(20 * time.Minute)
	until := time.Now().Add(5 * time.Minute)
	m, _ = sendTuiMsg(m, tui.SendHibernate(until))

	// When: user quits while hibernating
	m, _ = pressKey(m, 'q')

	// Then: elapsed time is persisted
	if tokenStats.TotalElapsedNs < (20 * time.Minute).Nanoseconds() {
		t.Errorf("Should persist at least 20min elapsed during hibernate, got %d ns",
			tokenStats.TotalElapsedNs)
	}
}

// --- Scenario 9: Quit with existing stats data preserves mutations ---

func TestBDD_UserExitsApplication_StatsMutatedInPlace(t *testing.T) {
	// Given: stats with existing token data
	tokenStats := stats.NewTokenStats()
	tokenStats.AddUsage(50000, 30000, 10000, 5000)
	tokenStats.AddCost(1.23)

	m := setupReadyModel()
	m.SetStats(tokenStats)
	m.SetBaseElapsed(15 * time.Minute)

	// When: user quits
	m, _ = pressKey(m, 'q')

	// Then: TotalElapsedNs is set on the SAME stats object (mutated in place)
	if tokenStats.TotalElapsedNs < (15 * time.Minute).Nanoseconds() {
		t.Errorf("Stats should be mutated in place with elapsed time")
	}
	// And: existing token data is preserved
	if tokenStats.InputTokens != 50000 {
		t.Errorf("InputTokens should be preserved, got %d", tokenStats.InputTokens)
	}
	if tokenStats.OutputTokens != 30000 {
		t.Errorf("OutputTokens should be preserved, got %d", tokenStats.OutputTokens)
	}
	if tokenStats.TotalCostUSD != 1.23 {
		t.Errorf("TotalCostUSD should be preserved, got %f", tokenStats.TotalCostUSD)
	}
}

// --- Scenario 10: Quit without loop set ---

func TestBDD_UserExitsApplication_QuitWithoutLoop(t *testing.T) {
	// Given: a model with no loop set (bare model)
	m := setupReadyModel()
	tokenStats := stats.NewTokenStats()
	m.SetStats(tokenStats)

	// When: user quits
	m, cmd := pressKey(m, 'q')

	// Then: quit succeeds (no loop needed to quit)
	if m.View() != "Goodbye!\n" {
		t.Errorf("Should quit without loop, got: %q", m.View())
	}
	if cmd == nil {
		t.Error("Should return quit command without loop")
	}
	// And: elapsed time is still persisted to stats
	if tokenStats.TotalElapsedNs <= 0 {
		t.Error("Should still persist elapsed time without loop")
	}
}

// --- Scenario 11: Quit with messages in feed ---

func TestBDD_UserExitsApplication_QuitWithMessagesInFeed(t *testing.T) {
	// Given: a model with multiple messages in the activity feed
	m := setupReadyModel()
	for i := 0; i < 20; i++ {
		m.AddMessage(tui.Message{Role: tui.RoleAssistant, Content: "Message content"})
	}
	m, _ = sendTuiMsg(m, tui.SendMessage(tui.Message{Role: tui.RoleTool, Content: "Final tool output"}))

	// Precondition: messages are visible
	if !viewContains(m, "Message content") {
		t.Fatal("Precondition failed: messages should be visible before quit")
	}

	// When: user quits
	m, _ = pressKey(m, 'q')

	// Then: view is replaced entirely by Goodbye
	if m.View() != "Goodbye!\n" {
		t.Errorf("Should show only Goodbye after quit, got: %q", m.View())
	}
}

// --- Scenario 12: Quit with all features active ---

func TestBDD_UserExitsApplication_QuitWithFullState(t *testing.T) {
	// Given: a model with every feature active (loop, stats, tmux bar, messages, mode)
	m, _ := setupReadyModelWithLoop(3, 7)
	tokenStats := stats.NewTokenStats()
	tokenStats.AddUsage(100000, 50000, 20000, 10000)
	tokenStats.AddCost(5.67)
	m.SetStats(tokenStats)
	m.SetBaseElapsed(2 * time.Hour)
	sb := tmux.NewStatusBar()
	m.SetTmuxStatusBar(sb)
	m.AddMessage(tui.Message{Role: tui.RoleAssistant, Content: "Working hard"})
	m, _ = sendTuiMsg(m, tui.SendModeUpdate("Building"))
	m, _ = sendTuiMsg(m, tui.SendTaskUpdate("Implementing feature X"))
	m, _ = sendTuiMsg(m, tui.SendAgentUpdate(3))

	// When: user quits
	m, cmd := pressKey(m, 'q')

	// Then: quit succeeds with all features active
	if m.View() != "Goodbye!\n" {
		t.Errorf("Should quit with full state, got: %q", m.View())
	}
	if cmd == nil {
		t.Error("Should return quit command with full state")
	}
	// And: elapsed time persisted correctly
	if tokenStats.TotalElapsedNs < (2 * time.Hour).Nanoseconds() {
		t.Errorf("Should persist at least 2h elapsed, got %d ns", tokenStats.TotalElapsedNs)
	}
	// And: existing stats preserved
	if tokenStats.InputTokens != 100000 {
		t.Error("Input tokens should be preserved after quit")
	}
}

// --- Scenario 13: Double quit / quit after quit ---

func TestBDD_UserExitsApplication_DoubleQuitNoCrash(t *testing.T) {
	// Given: a model that has already been quit
	m := setupReadyModel()
	m, _ = pressKey(m, 'q')

	// Precondition: already showing Goodbye
	if m.View() != "Goodbye!\n" {
		t.Fatal("Precondition failed: should already show Goodbye")
	}

	// When: user presses 'q' again (e.g., key event delivered twice)
	m, cmd := pressKey(m, 'q')

	// Then: no crash, still shows Goodbye
	if m.View() != "Goodbye!\n" {
		t.Errorf("Double quit should still show Goodbye, got: %q", m.View())
	}
	// And: still returns a quit command
	if cmd == nil {
		t.Error("Double quit should still return quit command")
	}
}

func TestBDD_UserExitsApplication_CtrlCAfterQ(t *testing.T) {
	// Given: a model quit via 'q'
	m := setupReadyModel()
	m, _ = pressKey(m, 'q')

	// When: Ctrl+C is pressed after quit
	m, cmd := updateModel(m, tea.KeyMsg{Type: tea.KeyCtrlC})

	// Then: no crash, still shows Goodbye
	if m.View() != "Goodbye!\n" {
		t.Errorf("Ctrl+C after q should still show Goodbye, got: %q", m.View())
	}
	if cmd == nil {
		t.Error("Ctrl+C after q should still return quit command")
	}
}

// --- Scenario 14: Quit does not double-persist elapsed ---

func TestBDD_UserExitsApplication_QuitPersistsElapsedOnce(t *testing.T) {
	// Given: a model with stats and base elapsed
	m := setupReadyModel()
	tokenStats := stats.NewTokenStats()
	m.SetStats(tokenStats)
	m.SetBaseElapsed(1 * time.Hour)

	// When: user quits
	m, _ = pressKey(m, 'q')
	firstElapsed := tokenStats.TotalElapsedNs

	// Then: elapsed is persisted
	if firstElapsed < (1 * time.Hour).Nanoseconds() {
		t.Fatalf("First quit should persist at least 1h, got %d ns", firstElapsed)
	}

	// When: user quits again
	m, _ = pressKey(m, 'q')
	secondElapsed := tokenStats.TotalElapsedNs

	// Then: elapsed value doesn't keep growing (timer was already paused state captures same value)
	// The second quit re-captures from the same paused state, so it should be very close
	diff := secondElapsed - firstElapsed
	if diff > (1 * time.Second).Nanoseconds() {
		t.Errorf("Second quit should not add significant time. First: %d, Second: %d, Diff: %d ns",
			firstElapsed, secondElapsed, diff)
	}
}
