package tests

import (
	"fmt"
	"strings"
	"testing"

	tea "github.com/charmbracelet/bubbletea"
	"github.com/cloudosai/ralph-go/internal/tui"
)

// ============================================================================
// BDD Test Suite: User Adapts to Terminal Changes
//
// These tests verify the complete user experience when the terminal is resized
// or starts at various dimensions: too-small message, transition from tiny to
// normal, message preservation across resizes, boundary conditions, and
// pre-init state. Covers state transitions, boundary conditions, negative
// paths, and cross-feature interactions.
// Organized by user goal following specs/bdd-agent-prompt.md methodology.
// ============================================================================

// --- Scenario 1: Too-small terminal shows warning message ---

func TestBDD_UserAdaptsToTerminal_TooSmallWidthShowsWarning(t *testing.T) {
	// Given: a model with terminal width below minimum (40)
	m := tui.NewModel()
	m, _ = updateModel(m, tea.WindowSizeMsg{Width: 30, Height: 40})

	// Then: the view shows a "Terminal too small" message with actual dimensions
	view := m.View()
	if !strings.Contains(view, "Terminal too small") {
		t.Errorf("Expected 'Terminal too small' message for narrow terminal, got: %q", view)
	}
	if !strings.Contains(view, "30x40") {
		t.Errorf("Expected current dimensions '30x40' in message, got: %q", view)
	}
	if !strings.Contains(view, "40x15") {
		t.Errorf("Expected minimum dimensions '40x15' in message, got: %q", view)
	}
}

func TestBDD_UserAdaptsToTerminal_TooSmallHeightShowsWarning(t *testing.T) {
	// Given: a model with terminal height below minimum (15)
	m := tui.NewModel()
	m, _ = updateModel(m, tea.WindowSizeMsg{Width: 120, Height: 10})

	// Then: the view shows a "Terminal too small" message with actual dimensions
	view := m.View()
	if !strings.Contains(view, "Terminal too small") {
		t.Errorf("Expected 'Terminal too small' message for short terminal, got: %q", view)
	}
	if !strings.Contains(view, "120x10") {
		t.Errorf("Expected current dimensions '120x10' in message, got: %q", view)
	}
}

func TestBDD_UserAdaptsToTerminal_BothDimensionsTooSmall(t *testing.T) {
	// Given: a model with both width and height below minimum
	m := tui.NewModel()
	m, _ = updateModel(m, tea.WindowSizeMsg{Width: 20, Height: 10})

	// Then: the view shows a "Terminal too small" message
	view := m.View()
	if !strings.Contains(view, "Terminal too small") {
		t.Errorf("Expected 'Terminal too small' message, got: %q", view)
	}
	if !strings.Contains(view, "20x10") {
		t.Errorf("Expected dimensions '20x10' in message, got: %q", view)
	}
}

// --- Scenario 2: Resize from tiny to normal restores full layout ---

func TestBDD_UserAdaptsToTerminal_ResizeFromTinyToNormal(t *testing.T) {
	// Given: a model that started with a too-small terminal
	m := tui.NewModel()
	m.AddMessage(tui.Message{Role: tui.RoleAssistant, Content: "RESIZE_RECOVERY_TEST"})
	m, _ = updateModel(m, tea.WindowSizeMsg{Width: 20, Height: 10})

	// Verify precondition: too-small message is shown
	if !viewContains(m, "Terminal too small") {
		t.Fatal("Precondition failed: should show too-small message")
	}

	// When: the terminal is resized to normal dimensions
	m, _ = updateModel(m, tea.WindowSizeMsg{Width: 120, Height: 40})

	// Then: the full layout is displayed with messages visible
	view := m.View()
	if strings.Contains(view, "Terminal too small") {
		t.Error("Should not show too-small message after resize to normal")
	}
	if !strings.Contains(view, "RESIZE_RECOVERY_TEST") {
		t.Error("Messages added before resize should be visible after recovery")
	}
	if !strings.Contains(view, "RUNNING") {
		t.Error("Status title should be visible after resize to normal")
	}
}

func TestBDD_UserAdaptsToTerminal_ResizeFromNormalToTiny(t *testing.T) {
	// Given: a model rendering normally at full size
	m := setupReadyModel()
	m, _ = sendTuiMsg(m, tui.SendMessage(tui.Message{Role: tui.RoleAssistant, Content: "VISIBLE_CONTENT"}))

	// Verify precondition: full layout is showing
	if viewContains(m, "Terminal too small") {
		t.Fatal("Precondition failed: should show full layout")
	}

	// When: the terminal is resized to below minimum
	m, _ = updateModel(m, tea.WindowSizeMsg{Width: 30, Height: 10})

	// Then: the too-small message replaces the full layout
	if !viewContains(m, "Terminal too small") {
		t.Error("Should show too-small message after resize to tiny")
	}
	if viewContains(m, "VISIBLE_CONTENT") {
		t.Error("Activity feed content should not be visible when terminal is too small")
	}
}

// --- Scenario 3: Resize preserves messages ---

func TestBDD_UserAdaptsToTerminal_ResizePreservesMessages(t *testing.T) {
	// Given: a model with messages rendered at one size
	m := tui.NewModel()
	m.AddMessage(tui.Message{Role: tui.RoleAssistant, Content: "MESSAGE_ALPHA"})
	m.AddMessage(tui.Message{Role: tui.RoleUser, Content: "MESSAGE_BETA"})
	m, _ = updateModel(m, tea.WindowSizeMsg{Width: 100, Height: 30})

	// Verify precondition: messages are visible
	if !viewContains(m, "MESSAGE_ALPHA") {
		t.Fatal("Precondition failed: first message should be visible")
	}

	// When: the terminal is resized to a different (still valid) size
	m, _ = updateModel(m, tea.WindowSizeMsg{Width: 150, Height: 50})

	// Then: all messages remain visible
	if !viewContains(m, "MESSAGE_ALPHA") {
		t.Error("First message should survive resize")
	}
	if !viewContains(m, "MESSAGE_BETA") {
		t.Error("Second message should survive resize")
	}
}

func TestBDD_UserAdaptsToTerminal_ResizePreservesMessagesSmaller(t *testing.T) {
	// Given: a model with messages at a large size
	m := tui.NewModel()
	m.AddMessage(tui.Message{Role: tui.RoleAssistant, Content: "SHRINK_TEST"})
	m, _ = updateModel(m, tea.WindowSizeMsg{Width: 200, Height: 60})

	// When: resized to a smaller (but still valid) size
	m, _ = updateModel(m, tea.WindowSizeMsg{Width: 80, Height: 25})

	// Then: messages remain visible
	if !viewContains(m, "SHRINK_TEST") {
		t.Error("Messages should survive resize to smaller window")
	}
}

func TestBDD_UserAdaptsToTerminal_ResizePreservesLoopProgress(t *testing.T) {
	// Given: a model showing loop progress
	m, _ := setupReadyModelWithLoop(3, 7)

	// Verify precondition
	if !viewContains(m, "#3/7") {
		t.Fatal("Precondition failed: loop progress should be visible")
	}

	// When: the terminal is resized
	m, _ = updateModel(m, tea.WindowSizeMsg{Width: 100, Height: 35})

	// Then: loop progress is still displayed
	if !viewContains(m, "#3/7") {
		t.Error("Loop progress should survive terminal resize")
	}
}

// --- Scenario 4: Exact minimum boundary renders full layout ---

func TestBDD_UserAdaptsToTerminal_ExactMinimumBoundaryRendersLayout(t *testing.T) {
	// Given: a model at exactly the minimum dimensions (40x15)
	m := tui.NewModel()
	m, _ = updateModel(m, tea.WindowSizeMsg{Width: 40, Height: 15})

	// Then: the full layout renders (not the too-small message)
	view := m.View()
	if strings.Contains(view, "Terminal too small") {
		t.Error("Should render full layout at exact minimum dimensions (40x15)")
	}
	if view == "" {
		t.Error("View should not be empty at minimum dimensions")
	}
	// Should show the status title
	if !strings.Contains(view, "RUNNING") {
		t.Error("Status title should be visible at minimum dimensions")
	}
}

func TestBDD_UserAdaptsToTerminal_OnePixelBelowMinWidth(t *testing.T) {
	// Given: width is one pixel below minimum (39x15)
	m := tui.NewModel()
	m, _ = updateModel(m, tea.WindowSizeMsg{Width: 39, Height: 15})

	// Then: too-small message shown
	if !viewContains(m, "Terminal too small") {
		t.Error("Should show too-small message at 39x15 (one below min width)")
	}
}

func TestBDD_UserAdaptsToTerminal_OnePixelBelowMinHeight(t *testing.T) {
	// Given: height is one pixel below minimum (40x14)
	m := tui.NewModel()
	m, _ = updateModel(m, tea.WindowSizeMsg{Width: 40, Height: 14})

	// Then: too-small message shown
	if !viewContains(m, "Terminal too small") {
		t.Error("Should show too-small message at 40x14 (one below min height)")
	}
}

func TestBDD_UserAdaptsToTerminal_OnePixelAboveMinimum(t *testing.T) {
	// Given: dimensions just above minimum (41x16)
	m := tui.NewModel()
	m, _ = updateModel(m, tea.WindowSizeMsg{Width: 41, Height: 16})

	// Then: full layout renders
	view := m.View()
	if strings.Contains(view, "Terminal too small") {
		t.Error("Should render full layout at 41x16")
	}
	if view == "" {
		t.Error("View should not be empty at 41x16")
	}
}

// --- Scenario 5: Pre-init state (no WindowSizeMsg yet) ---

func TestBDD_UserAdaptsToTerminal_PreInitReturnsEmptyString(t *testing.T) {
	// Given: a freshly created model with no WindowSizeMsg received
	m := tui.NewModel()

	// Then: the view returns an empty string (clean alt screen)
	view := m.View()
	if view != "" {
		t.Errorf("Pre-init view should be empty string for clean alt screen, got: %q", view)
	}
}

func TestBDD_UserAdaptsToTerminal_PreInitDoesNotShowLayout(t *testing.T) {
	// Given: a model with messages but no WindowSizeMsg
	m := tui.NewModel()
	m.AddMessage(tui.Message{Role: tui.RoleAssistant, Content: "PRE_INIT_MESSAGE"})

	// Then: view is still empty (no layout without window size)
	view := m.View()
	if view != "" {
		t.Errorf("Pre-init view should be empty even with messages, got: %q", view)
	}
}

// --- Additional scenarios: Cross-feature interactions ---

func TestBDD_UserAdaptsToTerminal_ResizeFromTinyPreservesCompletedState(t *testing.T) {
	// Given: a completed model at too-small size
	m := tui.NewModel()
	m, _ = updateModel(m, tea.WindowSizeMsg{Width: 20, Height: 10})
	m, _ = sendTuiMsg(m, tui.SendDone())

	// When: resized to normal
	m, _ = updateModel(m, tea.WindowSizeMsg{Width: 120, Height: 40})

	// Then: the completed state is preserved and displayed
	if !viewContains(m, "COMPLETED") {
		t.Error("Completed state should be preserved through tiny→normal resize")
	}
}

func TestBDD_UserAdaptsToTerminal_ResizeFromTinyPreservesStatsAndProgress(t *testing.T) {
	// Given: a model with stats and loop progress, then resized to tiny
	m, _ := setupReadyModelWithLoop(2, 5)
	m, _ = sendTuiMsg(m, tui.SendModeUpdate("Building"))

	// Resize to tiny
	m, _ = updateModel(m, tea.WindowSizeMsg{Width: 20, Height: 10})
	if !viewContains(m, "Terminal too small") {
		t.Fatal("Precondition failed: should be too small")
	}

	// When: resized back to normal
	m, _ = updateModel(m, tea.WindowSizeMsg{Width: 120, Height: 40})

	// Then: all state is preserved
	if !viewContains(m, "#2/5") {
		t.Error("Loop progress should be preserved through tiny→normal resize")
	}
	if !viewContains(m, "Building") {
		t.Error("Mode should be preserved through tiny→normal resize")
	}
}

func TestBDD_UserAdaptsToTerminal_MultipleResizeCycles(t *testing.T) {
	// Given: a model with content
	m := tui.NewModel()
	m.AddMessage(tui.Message{Role: tui.RoleAssistant, Content: "CYCLE_TEST"})

	// When: we cycle through multiple resize events (normal → tiny → normal → tiny → normal)
	sizes := []tea.WindowSizeMsg{
		{Width: 120, Height: 40},
		{Width: 20, Height: 10},
		{Width: 100, Height: 30},
		{Width: 15, Height: 8},
		{Width: 80, Height: 25},
	}
	for _, size := range sizes {
		m, _ = updateModel(m, size)
	}

	// Then: after final normal size, content is visible and layout is correct
	if viewContains(m, "Terminal too small") {
		t.Error("Should show full layout after final resize to normal")
	}
	if !viewContains(m, "CYCLE_TEST") {
		t.Error("Messages should survive multiple resize cycles")
	}
}

func TestBDD_UserAdaptsToTerminal_MessagesAddedDuringTinyAreVisibleAfterResize(t *testing.T) {
	// Given: a model at too-small dimensions
	m := tui.NewModel()
	m, _ = updateModel(m, tea.WindowSizeMsg{Width: 20, Height: 10})

	// When: messages are added while terminal is too small
	m.AddMessage(tui.Message{Role: tui.RoleAssistant, Content: "TINY_MSG_1"})
	m.AddMessage(tui.Message{Role: tui.RoleUser, Content: "TINY_MSG_2"})

	// And then the terminal is resized to normal
	m, _ = updateModel(m, tea.WindowSizeMsg{Width: 120, Height: 40})

	// Then: the messages added during the too-small phase are visible
	if !viewContains(m, "TINY_MSG_1") {
		t.Error("Message added during tiny phase should be visible after resize")
	}
	if !viewContains(m, "TINY_MSG_2") {
		t.Error("Second message added during tiny phase should be visible after resize")
	}
}

func TestBDD_UserAdaptsToTerminal_TooSmallDoesNotShowActivityFeed(t *testing.T) {
	// Given: a model with messages at too-small dimensions
	m := tui.NewModel()
	m.AddMessage(tui.Message{Role: tui.RoleAssistant, Content: "HIDDEN_CONTENT"})
	m, _ = updateModel(m, tea.WindowSizeMsg{Width: 30, Height: 10})

	// Then: the activity feed content is NOT visible (only the too-small message is)
	view := m.View()
	if strings.Contains(view, "HIDDEN_CONTENT") {
		t.Error("Activity feed content should not be rendered when terminal is too small")
	}
	if !strings.Contains(view, "Terminal too small") {
		t.Error("Only the too-small message should be visible")
	}
}

func TestBDD_UserAdaptsToTerminal_TooSmallDoesNotShowStatusBar(t *testing.T) {
	// Given: a model at too-small dimensions
	m := tui.NewModel()
	m, _ = updateModel(m, tea.WindowSizeMsg{Width: 30, Height: 10})

	// Then: the RUNNING status title is NOT visible
	if viewContains(m, "RUNNING") {
		t.Error("Status bar should not be rendered when terminal is too small")
	}
}

func TestBDD_UserAdaptsToTerminal_TooSmallDoesNotShowHotkeys(t *testing.T) {
	// Given: a model with loop at too-small dimensions
	m, _ := setupReadyModelWithLoop(1, 5)
	m, _ = updateModel(m, tea.WindowSizeMsg{Width: 30, Height: 10})

	// Then: hotkey bar is NOT visible
	if viewContains(m, "(p)ause") {
		t.Error("Hotkey bar should not be rendered when terminal is too small")
	}
	if viewContains(m, "(q)uit") {
		t.Error("Quit hotkey should not be rendered when terminal is too small")
	}
}

func TestBDD_UserAdaptsToTerminal_LargeTerminalRendersWithoutPanic(t *testing.T) {
	// Given: a model with a very large terminal
	m := tui.NewModel()
	m.AddMessage(tui.Message{Role: tui.RoleAssistant, Content: "LARGE_TERM_TEST"})
	m, _ = updateModel(m, tea.WindowSizeMsg{Width: 500, Height: 200})

	// Then: renders without panic and content is visible
	view := m.View()
	if view == "" {
		t.Error("View should not be empty at large terminal size")
	}
	if strings.Contains(view, "Terminal too small") {
		t.Error("Large terminal should not trigger too-small message")
	}
	if !strings.Contains(view, "LARGE_TERM_TEST") {
		t.Error("Content should be visible at large terminal size")
	}
}

func TestBDD_UserAdaptsToTerminal_WidthExactlyAtMinHeightBelow(t *testing.T) {
	// Given: width at minimum but height below
	m := tui.NewModel()
	m, _ = updateModel(m, tea.WindowSizeMsg{Width: 40, Height: 14})

	// Then: too-small message is shown (both dimensions must meet minimum)
	if !viewContains(m, "Terminal too small") {
		t.Error("Should show too-small when width is ok but height is below minimum")
	}
}

func TestBDD_UserAdaptsToTerminal_HeightExactlyAtMinWidthBelow(t *testing.T) {
	// Given: height at minimum but width below
	m := tui.NewModel()
	m, _ = updateModel(m, tea.WindowSizeMsg{Width: 39, Height: 15})

	// Then: too-small message is shown
	if !viewContains(m, "Terminal too small") {
		t.Error("Should show too-small when height is ok but width is below minimum")
	}
}

func TestBDD_UserAdaptsToTerminal_TooSmallMessageFormat(t *testing.T) {
	// Given: various too-small dimensions
	cases := []struct {
		width, height int
	}{
		{20, 10},
		{39, 14},
		{35, 40},
		{120, 12},
	}
	for _, tc := range cases {
		t.Run(fmt.Sprintf("%dx%d", tc.width, tc.height), func(t *testing.T) {
			m := tui.NewModel()
			m, _ = updateModel(m, tea.WindowSizeMsg{Width: tc.width, Height: tc.height})
			view := m.View()

			// Then: message includes both current and minimum dimensions
			expected := fmt.Sprintf("%dx%d", tc.width, tc.height)
			if !strings.Contains(view, expected) {
				t.Errorf("Too-small message should include current dimensions %s, got: %q", expected, view)
			}
			if !strings.Contains(view, "40x15") {
				t.Errorf("Too-small message should include minimum dimensions 40x15, got: %q", view)
			}
		})
	}
}

func TestBDD_UserAdaptsToTerminal_QuitFromTooSmallTerminal(t *testing.T) {
	// Given: a model at too-small dimensions
	m := tui.NewModel()
	m, _ = updateModel(m, tea.WindowSizeMsg{Width: 20, Height: 10})

	// When: user presses 'q' to quit
	m, _ = pressKey(m, 'q')

	// Then: the goodbye message is shown (quit works even from too-small state)
	view := m.View()
	if view != "Goodbye!\n" {
		t.Errorf("Quit from too-small terminal should show 'Goodbye!', got: %q", view)
	}
}

func TestBDD_UserAdaptsToTerminal_ZeroDimensions(t *testing.T) {
	// Given: a model with zero dimensions (edge case)
	m := tui.NewModel()
	m, _ = updateModel(m, tea.WindowSizeMsg{Width: 0, Height: 0})

	// Then: too-small message is shown without panic
	view := m.View()
	if !strings.Contains(view, "Terminal too small") {
		t.Errorf("Zero dimensions should show too-small message, got: %q", view)
	}
}

func TestBDD_UserAdaptsToTerminal_ResizeToExactMinimumFromTiny(t *testing.T) {
	// Given: a model at too-small dimensions
	m := tui.NewModel()
	m.AddMessage(tui.Message{Role: tui.RoleAssistant, Content: "BOUNDARY_MSG"})
	m, _ = updateModel(m, tea.WindowSizeMsg{Width: 20, Height: 10})

	// When: resized to exactly the minimum boundary
	m, _ = updateModel(m, tea.WindowSizeMsg{Width: 40, Height: 15})

	// Then: full layout is shown (not the too-small message)
	// Note: at 40x15, viewport is only 1 row tall (15 - footerHeight(11) - 2 borders = 2,
	// vpHeight = max(2-2, 1) = 1), so message content may not fit in the viewport.
	// We verify the layout renders, not that the tiny viewport shows all content.
	if viewContains(m, "Terminal too small") {
		t.Error("Should show full layout at exact minimum after resize from tiny")
	}
	view := m.View()
	if view == "" {
		t.Error("View should not be empty at exact minimum dimensions")
	}
	if !strings.Contains(view, "RUNNING") {
		t.Error("Status title should be visible at exact minimum after resize")
	}
}

func TestBDD_UserAdaptsToTerminal_NewMessageDuringPreInit(t *testing.T) {
	// Given: a model in pre-init state (no WindowSizeMsg yet)
	m := tui.NewModel()
	m.AddMessage(tui.Message{Role: tui.RoleAssistant, Content: "PRE_INIT_NEW_MSG"})

	// Then: view is still empty
	if m.View() != "" {
		t.Error("View should remain empty during pre-init even with messages")
	}

	// When: WindowSizeMsg arrives
	m, _ = updateModel(m, tea.WindowSizeMsg{Width: 120, Height: 40})

	// Then: messages are visible
	if !viewContains(m, "PRE_INIT_NEW_MSG") {
		t.Error("Messages added during pre-init should be visible after WindowSizeMsg")
	}
}

func TestBDD_UserAdaptsToTerminal_TickDuringPreInit(t *testing.T) {
	// Given: a model in pre-init state
	m := tui.NewModel()

	// When: a tick message arrives (should not panic)
	m, _ = updateModel(m, tui.TickMsgForTest())

	// Then: view is still empty, no crash
	if m.View() != "" {
		t.Error("View should remain empty during pre-init even after tick")
	}
}

func TestBDD_UserAdaptsToTerminal_WaitingMessageAtNormalSize(t *testing.T) {
	// Given: a new model at normal size with no messages
	m := setupReadyModel()

	// Then: shows "Waiting for activity..." placeholder
	if !viewContains(m, "Waiting for activity...") {
		t.Error("Empty model at normal size should show 'Waiting for activity...'")
	}
}
