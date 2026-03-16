package tests

import (
	"testing"
	"time"

	"github.com/cloudosai/ralph-go/internal/loop"
	"github.com/cloudosai/ralph-go/internal/stats"
	"github.com/cloudosai/ralph-go/internal/tui"
)

// ============================================================================
// BDD Test Suite: User Runs Plan-and-Build Mode
//
// These tests verify the TUI behavior when plan-and-build mode swaps the loop
// reference mid-session via SendLoopRef. Covers: stats continuity, message
// preservation, progress display, hotkey bar state, and the full
// plan-to-build mode transition.
// ============================================================================

// --- Scenario 1: Loop swap preserves accumulated messages ---

// TestBDD_UserRunsPlanAndBuildMode_LoopSwapPreservesMessages verifies that
// activity messages from the planning phase remain visible after the loop
// reference is swapped to the build loop.
func TestBDD_UserRunsPlanAndBuildMode_LoopSwapPreservesMessages(t *testing.T) {
	// Given: a model in planning phase with activity messages
	m, _ := setupReadyModelWithLoop(1, 1)
	m, _ = sendTuiMsg(m, tui.SendMessage(tui.Message{Role: tui.RoleAssistant, Content: "Planning output"}))

	if !viewContains(m, "Planning output") {
		t.Fatal("Precondition: planning message should be visible")
	}

	// When: loop is swapped to the build loop
	buildLoop := loop.New(loop.Config{Iterations: 5, Prompt: "build"})
	m, _ = sendTuiMsg(m, tui.SendLoopRef(buildLoop))

	// Then: planning messages are still visible in the activity feed
	if !viewContains(m, "Planning output") {
		t.Error("After loop swap, planning-phase messages should remain visible in the activity feed")
	}
}

// --- Scenario 2: Loop swap preserves accumulated stats ---

// TestBDD_UserRunsPlanAndBuildMode_LoopSwapPreservesStats verifies that token
// stats accumulated during planning are preserved after the loop reference swap.
func TestBDD_UserRunsPlanAndBuildMode_LoopSwapPreservesStats(t *testing.T) {
	// Given: a model with accumulated stats from the planning phase (2,500 tokens)
	m, _ := setupReadyModelWithLoop(1, 1)
	tokenStats := stats.NewTokenStats()
	tokenStats.InputTokens = 2500
	m.SetStats(tokenStats)
	m, _ = sendTuiMsg(m, tui.SendStatsUpdate(tokenStats))

	if !viewContains(m, "2.5k") {
		t.Fatal("Precondition: stats panel should show 2.5k input tokens")
	}

	// When: loop is swapped to the build loop
	buildLoop := loop.New(loop.Config{Iterations: 5, Prompt: "build"})
	m, _ = sendTuiMsg(m, tui.SendLoopRef(buildLoop))

	// Then: accumulated stats are preserved in the footer
	if !viewContains(m, "2.5k") {
		t.Error("After loop swap, accumulated token stats should be preserved in the footer panel")
	}
}

// --- Scenario 3: Loop swap preserves loop progress display ---

// TestBDD_UserRunsPlanAndBuildMode_LoopSwapPreservesProgress verifies that the
// loop progress display is unchanged by merely swapping the loop reference.
func TestBDD_UserRunsPlanAndBuildMode_LoopSwapPreservesProgress(t *testing.T) {
	// Given: a model at planning progress 1/1
	m, _ := setupReadyModelWithLoop(1, 1)

	if !viewContains(m, "#1/1") {
		t.Fatal("Precondition: progress should show #1/1")
	}

	// When: loop is swapped (plan phase done, build phase begins)
	buildLoop := loop.New(loop.Config{Iterations: 5, Prompt: "build"})
	m, _ = sendTuiMsg(m, tui.SendLoopRef(buildLoop))

	// Then: progress display is unchanged — it tracks independently of the loop reference
	if !viewContains(m, "#1/1") {
		t.Error("Loop swap should not change the loop progress display (#1/1)")
	}
}

// --- Scenario 4: Swap to a hibernating loop shows RATE LIMITED ---

// TestBDD_UserRunsPlanAndBuildMode_LoopSwapToHibernatingLoopShowsRateLimited
// verifies that when the build loop is already hibernating (e.g., hit rate
// limit during handoff), the TUI immediately shows RATE LIMITED and (r) wake.
func TestBDD_UserRunsPlanAndBuildMode_LoopSwapToHibernatingLoopShowsRateLimited(t *testing.T) {
	// Given: a model with a normal running loop (shows (p)ause, no RATE LIMITED)
	m, _ := setupReadyModelWithLoop(1, 5)

	if !viewContains(m, "(p)ause") {
		t.Fatal("Precondition: running loop should show (p)ause")
	}
	if viewContains(m, "RATE LIMITED") {
		t.Fatal("Precondition: should not show RATE LIMITED before swap")
	}

	// When: loop is swapped to a hibernating build loop
	buildLoop := loop.New(loop.Config{Iterations: 5, Prompt: "build"})
	buildLoop.Hibernate(time.Now().Add(5 * time.Minute))
	m, _ = sendTuiMsg(m, tui.SendLoopRef(buildLoop))

	// Then: status shows RATE LIMITED (driven by new loop's IsHibernating())
	if !viewContains(m, "RATE LIMITED") {
		t.Error("After swapping to a hibernating loop, status should show RATE LIMITED")
	}
	// And: hotkey bar shows (r) wake
	if !viewContains(m, "(r) wake") {
		t.Error("After swapping to a hibernating loop, hotkey bar should show '(r) wake'")
	}
}

// --- Scenario 5: Loop controls affect the new loop after swap ---

// TestBDD_UserRunsPlanAndBuildMode_AddLoopAfterSwapUpdatesNewLoop verifies that
// pressing '+' after a loop swap increases the count using the new loop's
// SetIterations, not the old planning loop's.
func TestBDD_UserRunsPlanAndBuildMode_AddLoopAfterSwapUpdatesNewLoop(t *testing.T) {
	// Given: a model with planning loop at 1/1, then swapped to build loop at 1/3
	m, _ := setupReadyModelWithLoop(1, 1)

	buildLoop := loop.New(loop.Config{Iterations: 3, Prompt: "build"})
	m, _ = sendTuiMsg(m, tui.SendLoopRef(buildLoop))
	m, _ = sendTuiMsg(m, tui.SendLoopUpdate(1, 3))

	if !viewContains(m, "#1/3") {
		t.Fatal("Precondition: after swap and update, progress should show #1/3")
	}

	// When: user presses '+' to add a loop
	m, _ = pressKey(m, '+')

	// Then: display shows #1/4 (new loop's iteration count increased)
	if !viewContains(m, "#1/4") {
		t.Errorf("After '+' post-swap, expected #1/4, got view:\n%s", m.View())
	}
}

// --- Scenario 6: Full plan-to-build mode transition ---

// TestBDD_UserRunsPlanAndBuildMode_PlanToBuildModeTransition verifies the
// complete sequence: planning loop → build loop swap, mode update to
// "Building", and progress update — all reflected correctly in the TUI view.
func TestBDD_UserRunsPlanAndBuildMode_PlanToBuildModeTransition(t *testing.T) {
	// Given: a model in Planning mode at loop 1/1
	m, _ := setupReadyModelWithLoop(1, 1)
	m.SetCurrentMode("Planning")
	m, _ = sendTuiMsg(m, tui.SendModeUpdate("Planning"))

	if !viewContains(m, "Planning") {
		t.Fatal("Precondition: should show Planning mode")
	}
	if !viewContains(m, "#1/1") {
		t.Fatal("Precondition: should show #1/1 progress")
	}

	// When: plan phase completes and build phase begins
	buildLoop := loop.New(loop.Config{Iterations: 5, Prompt: "build"})
	m, _ = sendTuiMsg(m, tui.SendLoopRef(buildLoop))
	m, _ = sendTuiMsg(m, tui.SendModeUpdate("Building"))
	m, _ = sendTuiMsg(m, tui.SendLoopUpdate(1, 5))

	// Then: mode displays "Building"
	if !viewContains(m, "Building") {
		t.Error("After plan-to-build transition, mode should display 'Building'")
	}
	// And: progress shows the build loop's progress
	if !viewContains(m, "#1/5") {
		t.Errorf("After plan-to-build transition, expected #1/5 progress, got view:\n%s", m.View())
	}
	// And: hotkey bar shows (p)ause (new running build loop)
	if !viewContains(m, "(p)ause") {
		t.Error("After plan-to-build transition, hotkey bar should show '(p)ause' for running build loop")
	}
}
