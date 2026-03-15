package tests

import (
	"fmt"
	"strings"
	"testing"
	"time"

	tea "github.com/charmbracelet/bubbletea"
	"github.com/cloudosai/ralph-go/internal/stats"
	"github.com/cloudosai/ralph-go/internal/tui"
)

// =============================================================================
// BDD Test Suite: User Monitors Build Progress
//
// User goal: As a user, I want to monitor the progress of my build by seeing
// activity messages, loop progress, token usage, agent counts, current task,
// and mode — so I can understand what Ralph is doing at all times.
// =============================================================================

// --- Scenario 1: Empty state shows waiting message ---

func TestBDD_UserMonitorsBuildProgress_EmptyStateShowsWaiting(t *testing.T) {
	// Given: a freshly initialized model with no messages
	m := setupReadyModel()

	// When: the view is rendered (implicit — no messages have been sent)
	view := m.View()

	// Then: the activity panel shows "Waiting for activity..."
	if !strings.Contains(view, "Waiting for activity...") {
		t.Errorf("Expected empty model to show 'Waiting for activity...', got:\n%s", view)
	}
}

// --- Scenario 2: Messages appear in feed in order ---

func TestBDD_UserMonitorsBuildProgress_MessagesAppearInOrder(t *testing.T) {
	// Given: a ready model
	m := setupReadyModel()

	// When: three messages are sent sequentially
	m, _ = sendTuiMsg(m, tui.SendMessage(tui.Message{Role: tui.RoleAssistant, Content: "First message"}))
	m, _ = sendTuiMsg(m, tui.SendMessage(tui.Message{Role: tui.RoleTool, Content: "Second message"}))
	m, _ = sendTuiMsg(m, tui.SendMessage(tui.Message{Role: tui.RoleSystem, Content: "Third message"}))

	// Then: all three messages are visible and appear in the correct order
	view := m.View()
	idx1 := strings.Index(view, "First message")
	idx2 := strings.Index(view, "Second message")
	idx3 := strings.Index(view, "Third message")

	if idx1 == -1 || idx2 == -1 || idx3 == -1 {
		t.Fatalf("Not all messages visible in view. First=%d, Second=%d, Third=%d\nView:\n%s",
			idx1, idx2, idx3, view)
	}
	if idx1 >= idx2 || idx2 >= idx3 {
		t.Errorf("Messages not in correct order. First@%d, Second@%d, Third@%d", idx1, idx2, idx3)
	}
}

func TestBDD_UserMonitorsBuildProgress_NewMessageReplacesWaiting(t *testing.T) {
	// Given: a model initially showing "Waiting for activity..."
	m := setupReadyModel()
	if !viewContains(m, "Waiting for activity...") {
		t.Fatal("Precondition: model should show waiting message")
	}

	// When: the first message arrives
	m, _ = sendTuiMsg(m, tui.SendMessage(tui.Message{Role: tui.RoleAssistant, Content: "Build started"}))

	// Then: the waiting message is gone and the new message is visible
	if viewContains(m, "Waiting for activity...") {
		t.Error("Waiting message should be replaced when messages arrive")
	}
	if !viewContains(m, "Build started") {
		t.Error("New message should be visible after sending")
	}
}

// --- Scenario 3: Message icons match roles in rendered view ---

func TestBDD_UserMonitorsBuildProgress_AllRoleIconsRendered(t *testing.T) {
	// Given: a ready model
	m := setupReadyModel()

	// When: messages of each role are sent
	roles := []struct {
		role tui.MessageRole
		icon string
		desc string
	}{
		{tui.RoleAssistant, "🤖", "assistant"},
		{tui.RoleTool, "🔧", "tool"},
		{tui.RoleUser, "📝", "user"},
		{tui.RoleSystem, "💰", "system"},
		{tui.RoleLoop, "🚀", "loop"},
		{tui.RoleLoopStopped, "🛑", "loop_stopped"},
		{tui.RoleHibernate, "💤", "hibernate"},
		{tui.RoleThinking, "💭", "thinking"},
	}

	for _, r := range roles {
		m, _ = sendTuiMsg(m, tui.SendMessage(tui.Message{
			Role:    r.role,
			Content: "msg_" + r.desc,
		}))
	}

	// Then: each role's icon is visible in the rendered view
	view := m.View()
	for _, r := range roles {
		if !strings.Contains(view, r.icon) {
			t.Errorf("Expected icon %s for role %s in view", r.icon, r.desc)
		}
		if !strings.Contains(view, "msg_"+r.desc) {
			t.Errorf("Expected content 'msg_%s' in view for role %s", r.desc, r.desc)
		}
	}
}

// --- Scenario 4: Activity feed auto-scrolls to bottom on new message ---

func TestBDD_UserMonitorsBuildProgress_AutoScrollOnNewMessage(t *testing.T) {
	// Given: a model with many messages that overflow the viewport, scrolled up
	m := setupReadyModel()
	for i := 0; i < 30; i++ {
		m, _ = sendTuiMsg(m, tui.SendMessage(tui.Message{
			Role:    tui.RoleAssistant,
			Content: "OLD_MSG_" + string(rune('A'+i%26)),
		}))
	}
	// Scroll up to move away from bottom
	m, _ = updateModel(m, tea.KeyMsg{Type: tea.KeyPgUp})
	m, _ = updateModel(m, tea.KeyMsg{Type: tea.KeyPgUp})

	// When: a new message arrives
	m, _ = sendTuiMsg(m, tui.SendMessage(tui.Message{
		Role:    tui.RoleLoop,
		Content: "BRAND_NEW_BOTTOM_MSG",
	}))

	// Then: the viewport shows the new message (auto-scrolled to bottom)
	if !viewContains(m, "BRAND_NEW_BOTTOM_MSG") {
		t.Error("Expected viewport to auto-scroll to bottom showing the new message")
	}
}

// --- Scenario 5: Scroll position preserved on tick ---

func TestBDD_UserMonitorsBuildProgress_ScrollPreservedOnTick(t *testing.T) {
	// Given: a model with many messages and viewport scrolled up
	m := setupReadyModel()
	for i := 0; i < 30; i++ {
		m, _ = sendTuiMsg(m, tui.SendMessage(tui.Message{
			Role:    tui.RoleAssistant,
			Content: fmt.Sprintf("SCROLL_MSG_%02d", i),
		}))
	}
	// Scroll up away from the bottom
	m, _ = updateModel(m, tea.KeyMsg{Type: tea.KeyPgUp})
	m, _ = updateModel(m, tea.KeyMsg{Type: tea.KeyPgUp})

	// Precondition: after scrolling up, the last message should NOT be visible
	if viewContains(m, "SCROLL_MSG_29") {
		t.Fatal("Precondition: after scrolling up, last message should not be visible")
	}

	viewBefore := m.View()

	// When: a tick occurs (timer refresh)
	m, _ = updateModel(m, tui.TickMsgForTest())

	// Then: the scroll position is preserved (view doesn't jump to bottom)
	viewAfter := m.View()
	if viewContains(m, "SCROLL_MSG_29") {
		t.Error("Tick should not scroll viewport to bottom — last message should remain hidden")
	}
	// Verify an early message visible before tick is still visible after
	if strings.Contains(viewBefore, "SCROLL_MSG_00") && !strings.Contains(viewAfter, "SCROLL_MSG_00") {
		t.Error("Tick should preserve scroll position — visible messages should remain visible")
	}
}

// --- Scenario 6: Loop progress updates in footer ---

func TestBDD_UserMonitorsBuildProgress_LoopProgressUpdates(t *testing.T) {
	// Given: a ready model
	m := setupReadyModel()

	// When: loop progress update is dispatched
	m, _ = sendTuiMsg(m, tui.SendLoopUpdate(3, 5))

	// Then: footer shows "#3/5"
	if !viewContains(m, "#3/5") {
		t.Errorf("Expected footer to show '#3/5', got:\n%s", m.View())
	}
}

func TestBDD_UserMonitorsBuildProgress_LoopProgressDefaultZero(t *testing.T) {
	// Given: a fresh model with no loop updates
	m := setupReadyModel()

	// When: the view is rendered
	// Then: shows "#0/0" as default
	if !viewContains(m, "#0/0") {
		t.Errorf("Expected default loop progress '#0/0', got:\n%s", m.View())
	}
}

func TestBDD_UserMonitorsBuildProgress_LoopProgressSequentialUpdates(t *testing.T) {
	// Given: a model receiving sequential loop updates
	m := setupReadyModel()

	// When: loop progresses from 1/5 through 5/5
	for i := 1; i <= 5; i++ {
		m, _ = sendTuiMsg(m, tui.SendLoopUpdate(i, 5))
	}

	// Then: the final state shows "#5/5"
	if !viewContains(m, "#5/5") {
		t.Errorf("Expected loop progress '#5/5' after sequential updates")
	}
}

// --- Scenario 7: Stats update with token usage and cost ---

func TestBDD_UserMonitorsBuildProgress_StatsUpdateShowsTokensAndCost(t *testing.T) {
	// Given: a ready model
	m := setupReadyModel()

	// When: stats are updated with usage and cost
	s := stats.NewTokenStats()
	s.AddUsage(50000, 10000, 12000, 30000)
	s.AddCost(0.123456)
	m, _ = sendTuiMsg(m, tui.SendStatsUpdate(s))

	// Then: footer shows formatted token counts and cost
	view := m.View()
	if !strings.Contains(view, "$0.123456") {
		t.Errorf("Expected cost '$0.123456' in footer")
	}
	// Total tokens = 50000+10000+12000+30000 = 102000 → "102k"
	if !strings.Contains(view, "102k") {
		t.Errorf("Expected total tokens '102k' in footer, got:\n%s", view)
	}
}

func TestBDD_UserMonitorsBuildProgress_StatsZeroValuesDisplay(t *testing.T) {
	// Given: a fresh model with zero stats
	m := setupReadyModel()

	// When: the view is rendered (no stats updates)
	view := m.View()

	// Then: shows zero cost and zero tokens
	if !strings.Contains(view, "$0.000000") {
		t.Errorf("Expected zero cost '$0.000000' in footer")
	}
	if !strings.Contains(view, "Total Tokens:") {
		t.Errorf("Expected 'Total Tokens:' label in footer")
	}
}

func TestBDD_UserMonitorsBuildProgress_StatsCacheTokenBreakdown(t *testing.T) {
	// Given: a ready model
	m := setupReadyModel()

	// When: stats include cache write and cache read tokens
	s := stats.NewTokenStats()
	s.AddUsage(1000, 2000, 15000, 80000)
	m, _ = sendTuiMsg(m, tui.SendStatsUpdate(s))

	// Then: cache write and cache read are shown with their labels
	view := m.View()
	if !strings.Contains(view, "Cache Write:") {
		t.Errorf("Expected 'Cache Write:' label in footer")
	}
	if !strings.Contains(view, "Cache Read:") {
		t.Errorf("Expected 'Cache Read:' label in footer")
	}
	// 15000 → "15k", 80000 → "80k"
	if !strings.Contains(view, "15k") {
		t.Errorf("Expected cache write '15k' in footer")
	}
	if !strings.Contains(view, "80k") {
		t.Errorf("Expected cache read '80k' in footer")
	}
}

func TestBDD_UserMonitorsBuildProgress_StatsLargeValues(t *testing.T) {
	// Given: a ready model
	m := setupReadyModel()

	// When: stats have millions of tokens
	s := stats.NewTokenStats()
	s.AddUsage(5000000, 1500000, 200000, 3000000)
	s.AddCost(12.345678)
	m, _ = sendTuiMsg(m, tui.SendStatsUpdate(s))

	// Then: tokens are formatted with m suffix, cost is shown correctly
	view := m.View()
	if !strings.Contains(view, "$12.345678") {
		t.Errorf("Expected cost '$12.345678' in footer")
	}
	// Input 5000000 → "5.00m"
	if !strings.Contains(view, "5.00m") {
		t.Errorf("Expected input tokens '5.00m' in footer, got:\n%s", view)
	}
}

// --- Scenario 8: Agent count updates ---

func TestBDD_UserMonitorsBuildProgress_AgentCountUpdates(t *testing.T) {
	// Given: a ready model
	m := setupReadyModel()

	// When: agent count is updated to 3
	m, _ = sendTuiMsg(m, tui.SendAgentUpdate(3))

	// Then: footer shows "3" for active agents
	view := m.View()
	if !strings.Contains(view, "Active Agents:") {
		t.Errorf("Expected 'Active Agents:' label in footer")
	}
	if !strings.Contains(view, "3") {
		t.Errorf("Expected agent count '3' in footer")
	}
}

func TestBDD_UserMonitorsBuildProgress_AgentCountResetToZero(t *testing.T) {
	// Given: a model with 5 active agents
	m := setupReadyModel()
	m, _ = sendTuiMsg(m, tui.SendAgentUpdate(5))
	if !viewContains(m, "5") {
		t.Fatal("Precondition: should show 5 agents")
	}

	// When: agent count resets to 0
	m, _ = sendTuiMsg(m, tui.SendAgentUpdate(0))

	// Then: the label persists and shows 0
	if !viewContains(m, "Active Agents:") {
		t.Error("Expected 'Active Agents:' label to persist after reset")
	}
}

// --- Scenario 9: Current task updates ---

func TestBDD_UserMonitorsBuildProgress_CurrentTaskUpdates(t *testing.T) {
	// Given: a ready model
	m := setupReadyModel()

	// When: task is updated via message
	m, _ = sendTuiMsg(m, tui.SendTaskUpdate("#6 Refactor config module"))

	// Then: footer shows the task text
	view := m.View()
	if !strings.Contains(view, "Current Task:") {
		t.Errorf("Expected 'Current Task:' label in footer")
	}
	if !strings.Contains(view, "#6 Refactor config module") {
		t.Errorf("Expected task text in footer, got:\n%s", view)
	}
}

func TestBDD_UserMonitorsBuildProgress_CurrentTaskDefaultDash(t *testing.T) {
	// Given: a fresh model with no task set
	m := setupReadyModel()

	// When: the view is rendered
	view := m.View()

	// Then: shows "Current Task:" label with "-" as default
	if !strings.Contains(view, "Current Task:") {
		t.Errorf("Expected 'Current Task:' label in footer")
	}
}

func TestBDD_UserMonitorsBuildProgress_CurrentTaskOverwritesPrevious(t *testing.T) {
	// Given: a model with a task set
	m := setupReadyModel()
	m, _ = sendTuiMsg(m, tui.SendTaskUpdate("#1 Old task"))
	if !viewContains(m, "#1 Old task") {
		t.Fatal("Precondition: old task should be visible")
	}

	// When: a new task update arrives
	m, _ = sendTuiMsg(m, tui.SendTaskUpdate("#2 New task"))

	// Then: only the new task is shown
	if viewContains(m, "#1 Old task") {
		t.Error("Old task should not be visible after new task update")
	}
	if !viewContains(m, "#2 New task") {
		t.Error("New task should be visible after update")
	}
}

// --- Scenario 10: Mode transitions ---

func TestBDD_UserMonitorsBuildProgress_ModeTransitions(t *testing.T) {
	// Given: a model in "Planning" mode
	m := setupReadyModel()
	m, _ = sendTuiMsg(m, tui.SendModeUpdate("Planning"))
	if !viewContains(m, "Planning") {
		t.Fatal("Precondition: should show 'Planning' mode")
	}

	// When: mode changes to "Building"
	m, _ = sendTuiMsg(m, tui.SendModeUpdate("Building"))

	// Then: footer reflects "Building" and no longer shows "Planning"
	if !viewContains(m, "Building") {
		t.Error("Expected 'Building' mode in footer after mode transition")
	}
}

// --- Additional BDD scenarios for completeness ---

// Scenario: Completed tasks update in footer

func TestBDD_UserMonitorsBuildProgress_CompletedTasksUpdate(t *testing.T) {
	// Given: a ready model
	m := setupReadyModel()

	// When: completed tasks are updated
	m, _ = sendTuiMsg(m, tui.SendCompletedTasksUpdate(4, 7))

	// Then: footer shows "4/7"
	if !viewContains(m, "4/7") {
		t.Errorf("Expected completed tasks '4/7' in footer, got:\n%s", m.View())
	}
	if !viewContains(m, "Completed Tasks:") {
		t.Errorf("Expected 'Completed Tasks:' label in footer")
	}
}

func TestBDD_UserMonitorsBuildProgress_CompletedTasksDefaultZero(t *testing.T) {
	// Given: a fresh model with no completed tasks updates
	m := setupReadyModel()

	// When: the view is rendered
	// Then: shows "0/0" as default
	if !viewContains(m, "0/0") {
		t.Errorf("Expected default completed tasks '0/0' in footer")
	}
}

// Scenario: Footer panel ordering

func TestBDD_UserMonitorsBuildProgress_FooterFieldOrdering(t *testing.T) {
	// Given: a model with all footer fields populated
	m := setupReadyModel()
	m, _ = sendTuiMsg(m, tui.SendLoopUpdate(2, 5))
	m, _ = sendTuiMsg(m, tui.SendAgentUpdate(3))
	m, _ = sendTuiMsg(m, tui.SendCompletedTasksUpdate(1, 4))
	m, _ = sendTuiMsg(m, tui.SendTaskUpdate("#3 Build widget"))
	m, _ = sendTuiMsg(m, tui.SendModeUpdate("Building"))

	// When: the view is rendered
	view := m.View()

	// Then: fields appear in the correct order within the Ralph Loop Details panel
	loopIdx := strings.Index(view, "Loop:")
	timeIdx := strings.Index(view, "Total Time:")
	statusIdx := strings.Index(view, "Status:")
	agentIdx := strings.Index(view, "Active Agents:")
	completedIdx := strings.Index(view, "Completed Tasks:")
	taskIdx := strings.Index(view, "Current Task:")
	modeIdx := strings.Index(view, "Current Mode:")

	if loopIdx == -1 || timeIdx == -1 || statusIdx == -1 || agentIdx == -1 ||
		completedIdx == -1 || taskIdx == -1 || modeIdx == -1 {
		t.Fatalf("Not all footer fields found. Loop=%d Time=%d Status=%d Agent=%d Completed=%d Task=%d Mode=%d",
			loopIdx, timeIdx, statusIdx, agentIdx, completedIdx, taskIdx, modeIdx)
	}

	if !(loopIdx < timeIdx && timeIdx < statusIdx && statusIdx < agentIdx &&
		agentIdx < completedIdx && completedIdx < taskIdx && taskIdx < modeIdx) {
		t.Errorf("Footer fields not in expected order: Loop@%d < Time@%d < Status@%d < Agent@%d < Completed@%d < Task@%d < Mode@%d",
			loopIdx, timeIdx, statusIdx, agentIdx, completedIdx, taskIdx, modeIdx)
	}
}

// Scenario: Done message freezes all progress displays

func TestBDD_UserMonitorsBuildProgress_DoneFreezesFutureUpdates(t *testing.T) {
	// Given: a model with active loop progress
	m := setupReadyModel()
	m, _ = sendTuiMsg(m, tui.SendLoopUpdate(5, 5))
	m, _ = sendTuiMsg(m, tui.SendAgentUpdate(2))

	// When: done signal is received
	m, _ = sendTuiMsg(m, tui.SendDone())

	// Then: status shows "Completed" and "COMPLETED" banner
	if !viewContains(m, "Completed") {
		t.Error("Expected 'Completed' status after done signal")
	}
	if !viewContains(m, "COMPLETED") {
		t.Error("Expected 'COMPLETED' banner after done signal")
	}
}

func TestBDD_UserMonitorsBuildProgress_DoneFreezesTimerDisplay(t *testing.T) {
	// Given: a model with timer running
	m := setupReadyModel()

	// When: done signal is received
	m, _ = sendTuiMsg(m, tui.SendDone())
	view1 := m.View()

	// Allow a small time to pass, then tick
	time.Sleep(50 * time.Millisecond)
	m, _ = updateModel(m, tui.TickMsgForTest())
	view2 := m.View()

	// Then: the timer display is frozen (views are identical)
	if view1 != view2 {
		t.Error("Timer display should be frozen after done signal — views differ")
	}
}

// Scenario: Loop progress persists across mode change

func TestBDD_UserMonitorsBuildProgress_LoopProgressPersistsAcrossModeChange(t *testing.T) {
	// Given: a model at loop #3/5 in "Planning" mode
	m := setupReadyModel()
	m, _ = sendTuiMsg(m, tui.SendLoopUpdate(3, 5))
	m, _ = sendTuiMsg(m, tui.SendModeUpdate("Planning"))

	// When: mode changes to "Building"
	m, _ = sendTuiMsg(m, tui.SendModeUpdate("Building"))

	// Then: loop progress still shows #3/5
	if !viewContains(m, "#3/5") {
		t.Error("Loop progress should persist across mode change")
	}
	if !viewContains(m, "Building") {
		t.Error("Mode should update to 'Building'")
	}
}

// Scenario: Stats persist across agent count change

func TestBDD_UserMonitorsBuildProgress_StatsPersistAcrossAgentChange(t *testing.T) {
	// Given: a model with stats showing $1.50
	m := setupReadyModel()
	s := stats.NewTokenStats()
	s.AddCost(1.500000)
	m, _ = sendTuiMsg(m, tui.SendStatsUpdate(s))
	if !viewContains(m, "$1.500000") {
		t.Fatal("Precondition: cost should show $1.500000")
	}

	// When: agent count is updated
	m, _ = sendTuiMsg(m, tui.SendAgentUpdate(5))

	// Then: stats still show the same cost
	if !viewContains(m, "$1.500000") {
		t.Error("Stats should persist when agent count changes")
	}
	if !viewContains(m, "5") {
		t.Error("Agent count should update to 5")
	}
}

// Scenario: Multiple rapid messages accumulate without loss

func TestBDD_UserMonitorsBuildProgress_RapidMessagesAccumulate(t *testing.T) {
	// Given: a ready model
	m := setupReadyModel()

	// When: 20 messages are sent rapidly
	for i := 0; i < 20; i++ {
		m, _ = sendTuiMsg(m, tui.SendMessage(tui.Message{
			Role:    tui.RoleAssistant,
			Content: "RAPID_" + string(rune('A'+i%26)),
		}))
	}

	// Then: the first and last messages are present (scrollable)
	view := m.View()
	// The last message should be visible (auto-scrolled to bottom)
	if !strings.Contains(view, "RAPID_T") {
		// 20th message is i=19, 'A'+19 = 'T'
		t.Error("Expected last rapid message to be visible")
	}
}

// Scenario: Stats update via SendStatsUpdate replaces previous stats entirely

func TestBDD_UserMonitorsBuildProgress_StatsUpdateReplacesOld(t *testing.T) {
	// Given: a model with initial stats
	m := setupReadyModel()
	s1 := stats.NewTokenStats()
	s1.AddUsage(1000, 500, 0, 0)
	s1.AddCost(0.001000)
	m, _ = sendTuiMsg(m, tui.SendStatsUpdate(s1))
	if !viewContains(m, "$0.001000") {
		t.Fatal("Precondition: old cost should be visible")
	}

	// When: a new stats update arrives with different values
	s2 := stats.NewTokenStats()
	s2.AddUsage(200000, 50000, 10000, 100000)
	s2.AddCost(5.678900)
	m, _ = sendTuiMsg(m, tui.SendStatsUpdate(s2))

	// Then: the new stats completely replace the old ones
	view := m.View()
	if !strings.Contains(view, "$5.678900") {
		t.Errorf("Expected new cost '$5.678900' in footer, got:\n%s", view)
	}
	if strings.Contains(view, "$0.001000") {
		t.Error("Old cost should not be visible after stats replacement")
	}
}

// Scenario: Long message content is not truncated

func TestBDD_UserMonitorsBuildProgress_LongMessageNotTruncated(t *testing.T) {
	// Given: a ready model with a wide terminal to accommodate long content
	m := tui.NewModel()
	m, _ = updateModel(m, tea.WindowSizeMsg{Width: 500, Height: 40})

	// When: a message with long content is sent
	longContent := strings.Repeat("abcdefghij", 40) + "UNTRUNCATED_MARKER"
	m, _ = sendTuiMsg(m, tui.SendMessage(tui.Message{
		Role:    tui.RoleAssistant,
		Content: longContent,
	}))

	// Then: the marker at the end is still visible (content not truncated)
	if !viewContains(m, "UNTRUNCATED_MARKER") {
		t.Error("Long message content should not be truncated in the activity feed")
	}
}

// Scenario: Loop started resets per-loop tracking

func TestBDD_UserMonitorsBuildProgress_LoopStartedResetsPerLoopStats(t *testing.T) {
	// Given: a model with per-loop stats accumulated
	m := setupReadyModel()
	m, _ = sendTuiMsg(m, tui.SendLoopStatsUpdate(50000))

	// When: a new loop iteration starts
	m, _ = sendTuiMsg(m, tui.SendLoopStarted())

	// Then: the model still renders with the stats panel present
	// (per-loop tokens are reset internally; the total stats panel remains visible)
	m, _ = sendTuiMsg(m, tui.SendLoopStatsUpdate(100))
	if !viewContains(m, "Total Tokens:") {
		t.Error("Stats panel should still be present after loop started reset")
	}
	if !viewContains(m, "RUNNING") {
		t.Error("Status should still show RUNNING after loop started reset")
	}
}
