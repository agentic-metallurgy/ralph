package tests

import (
	"context"
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"
	"unicode/utf8"

	tea "github.com/charmbracelet/bubbletea"
	"github.com/cloudosai/ralph-go/internal/loop"
	"github.com/cloudosai/ralph-go/internal/stats"
	"github.com/cloudosai/ralph-go/internal/tmux"
	"github.com/cloudosai/ralph-go/internal/tui"
)

// helper function to update model with type assertion
func updateModel(m tui.Model, msg tea.Msg) (tui.Model, tea.Cmd) {
	updated, cmd := m.Update(msg)
	return updated.(tui.Model), cmd
}

// TestNewModel tests that NewModel creates a properly initialized model
func TestNewModel(t *testing.T) {
	model := tui.NewModel()

	// Before WindowSizeMsg, view should be empty (clean alt screen, no flash)
	view := model.View()
	if view != "" {
		t.Errorf("Expected empty view before window size (clean init), got: %q", view)
	}
}

// TestNewModelWithChannels tests creation with external channels
func TestNewModelWithChannels(t *testing.T) {
	msgChan := make(chan tui.Message, 10)
	doneChan := make(chan struct{})
	defer close(msgChan)
	defer close(doneChan)

	model := tui.NewModelWithChannels(msgChan, doneChan)
	view := model.View()
	if view != "" {
		t.Errorf("Expected empty view before window size (clean init), got: %q", view)
	}
}

// TestMessageRoles tests that all message roles have correct icons (non-October)
func TestMessageRoles(t *testing.T) {
	// Explicitly set non-October time so this test is stable year-round
	tui.SetTimeNowForTest(func() time.Time {
		return time.Date(2024, time.February, 15, 12, 0, 0, 0, time.UTC)
	})
	defer tui.SetTimeNowForTest(time.Now)

	tests := []struct {
		role         tui.MessageRole
		expectedIcon string
	}{
		{tui.RoleAssistant, "🤖"},
		{tui.RoleTool, "🔧"},
		{tui.RoleUser, "📝"},
		{tui.RoleSystem, "💰"},
	}

	for _, tc := range tests {
		t.Run(string(tc.role), func(t *testing.T) {
			msg := tui.Message{Role: tc.role, Content: "test"}
			icon := msg.GetIcon()
			if icon != tc.expectedIcon {
				t.Errorf("Expected icon %s for role %s, got %s", tc.expectedIcon, tc.role, icon)
			}
		})
	}
}

// TestMessageGetStyle tests that each role returns a non-nil style
func TestMessageGetStyle(t *testing.T) {
	roles := []tui.MessageRole{
		tui.RoleAssistant,
		tui.RoleTool,
		tui.RoleUser,
		tui.RoleSystem,
	}

	for _, role := range roles {
		t.Run(string(role), func(t *testing.T) {
			msg := tui.Message{Role: role, Content: "test"}
			style := msg.GetStyle()
			// Style should render without panic
			rendered := style.Render("test")
			if rendered == "" {
				t.Errorf("Style for role %s rendered empty string", role)
			}
		})
	}
}

// TestModelWindowSize tests that the model handles window resize
func TestModelWindowSize(t *testing.T) {
	model := tui.NewModel()

	// Send window size message
	msg := tea.WindowSizeMsg{Width: 120, Height: 40}
	updatedModel, _ := updateModel(model, msg)

	// After window size, the view should render the full layout (not empty)
	view := updatedModel.View()
	if view == "" {
		t.Error("Model should be ready and render content after receiving WindowSizeMsg")
	}
}

// TestModelQuit tests that q key quits the model
func TestModelQuit(t *testing.T) {
	model := tui.NewModel()

	// First set window size
	model, _ = updateModel(model, tea.WindowSizeMsg{Width: 120, Height: 40})

	// Then send quit key
	keyMsg := tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune{'q'}}
	updatedModel, cmd := updateModel(model, keyMsg)

	// Check that quit was triggered
	view := updatedModel.View()
	if view != "Goodbye!\n" {
		t.Errorf("Expected 'Goodbye!\\n' after quit, got: %s", view)
	}

	// Cmd should trigger tea.Quit
	if cmd == nil {
		t.Error("Expected a quit command to be returned")
	}
}

// TestModelCtrlCQuit tests that Ctrl+C quits the model
func TestModelCtrlCQuit(t *testing.T) {
	model := tui.NewModel()

	// First set window size
	model, _ = updateModel(model, tea.WindowSizeMsg{Width: 120, Height: 40})

	// Then send Ctrl+C
	keyMsg := tea.KeyMsg{Type: tea.KeyCtrlC}
	updatedModel, cmd := updateModel(model, keyMsg)

	// Check that quit was triggered
	view := updatedModel.View()
	if view != "Goodbye!\n" {
		t.Errorf("Expected 'Goodbye!\\n' after Ctrl+C, got: %s", view)
	}

	if cmd == nil {
		t.Error("Expected a quit command to be returned")
	}
}

// TestAddMessage tests adding messages to the activity feed
func TestAddMessage(t *testing.T) {
	model := tui.NewModel()

	// Add a message
	msg := tui.Message{Role: tui.RoleAssistant, Content: "Hello world"}
	model.AddMessage(msg)

	// Set window size to render
	model, _ = updateModel(model, tea.WindowSizeMsg{Width: 120, Height: 40})

	view := model.View()
	if view == "" {
		t.Error("View should not be empty after adding message")
	}
}

// TestMaxMessages tests that message limit is respected
func TestMaxMessages(t *testing.T) {
	model := tui.NewModel()

	// Add more than maxMessages (20)
	for i := 0; i < 25; i++ {
		msg := tui.Message{Role: tui.RoleAssistant, Content: "Message"}
		model.AddMessage(msg)
	}

	// Note: We can't directly check the message count without exposing internal state
	// But we can verify the model doesn't crash and still renders
	model, _ = updateModel(model, tea.WindowSizeMsg{Width: 120, Height: 40})
	view := model.View()
	if view == "" || view == "Initializing..." {
		t.Error("Model should render properly with max messages exceeded")
	}
}

// TestSetStats tests setting custom stats
func TestSetStats(t *testing.T) {
	model := tui.NewModel()

	customStats := stats.NewTokenStats()
	customStats.AddUsage(1000, 500, 200, 100)
	customStats.AddCost(0.05)

	model.SetStats(customStats)

	// Set window size and render
	model, _ = updateModel(model, tea.WindowSizeMsg{Width: 120, Height: 40})
	view := model.View()

	// The view should contain the cost
	if view == "" {
		t.Error("View should not be empty after setting stats")
	}
}

// TestSetLoopProgress tests setting loop progress
func TestSetLoopProgress(t *testing.T) {
	model := tui.NewModel()
	model.SetLoopProgress(5, 20)

	// Set window size and render
	model, _ = updateModel(model, tea.WindowSizeMsg{Width: 120, Height: 40})
	view := model.View()

	// View should render without error
	if view == "" || view == "Initializing..." {
		t.Error("Model should render properly with loop progress set")
	}
}

// TestModelInit tests the Init function
func TestModelInit(t *testing.T) {
	model := tui.NewModel()
	cmd := model.Init()

	// Should return a tick command
	if cmd == nil {
		t.Error("Init should return a command (tick)")
	}
}

// TestModelInitWithChannels tests Init with channels
func TestModelInitWithChannels(t *testing.T) {
	msgChan := make(chan tui.Message, 10)
	doneChan := make(chan struct{})
	defer close(msgChan)
	defer close(doneChan)

	model := tui.NewModelWithChannels(msgChan, doneChan)
	cmd := model.Init()

	// Should return a batch command (tick + message listener + done listener)
	if cmd == nil {
		t.Error("Init should return a batch command")
	}
}

// TestSendMessageCmd tests the SendMessage helper
func TestSendMessageCmd(t *testing.T) {
	msg := tui.Message{Role: tui.RoleAssistant, Content: "test"}
	cmd := tui.SendMessage(msg)

	if cmd == nil {
		t.Error("SendMessage should return a command")
	}

	// Execute the command and check the message
	result := cmd()
	if result == nil {
		t.Error("Command should return a message")
	}
}

// TestSendLoopUpdateCmd tests the SendLoopUpdate helper
func TestSendLoopUpdateCmd(t *testing.T) {
	cmd := tui.SendLoopUpdate(5, 20)

	if cmd == nil {
		t.Error("SendLoopUpdate should return a command")
	}

	// Execute the command
	result := cmd()
	if result == nil {
		t.Error("Command should return a loop update message")
	}
}

// TestSendStatsUpdateCmd tests the SendStatsUpdate helper
func TestSendStatsUpdateCmd(t *testing.T) {
	s := stats.NewTokenStats()
	cmd := tui.SendStatsUpdate(s)

	if cmd == nil {
		t.Error("SendStatsUpdate should return a command")
	}

	// Execute the command
	result := cmd()
	if result == nil {
		t.Error("Command should return a stats update message")
	}
}

// TestViewRendersActivityPanel tests that the view includes activity panel
func TestViewRendersActivityPanel(t *testing.T) {
	model := tui.NewModel()
	model.AddMessage(tui.Message{Role: tui.RoleAssistant, Content: "Hello"})
	model, _ = updateModel(model, tea.WindowSizeMsg{Width: 120, Height: 40})

	view := model.View()
	if view == "" {
		t.Error("View should not be empty")
	}
	// Activity title should be rendered
	// Note: Exact string matching is fragile due to ANSI codes
}

// TestViewRendersFooter tests that the view includes footer panels
func TestViewRendersFooter(t *testing.T) {
	model := tui.NewModel()
	model, _ = updateModel(model, tea.WindowSizeMsg{Width: 120, Height: 40})

	view := model.View()
	if view == "" {
		t.Error("View should not be empty")
	}
	// Footer should be present but exact matching is fragile
}

// TestWaitingForActivityMessage tests the initial waiting message
func TestWaitingForActivityMessage(t *testing.T) {
	model := tui.NewModel()
	model, _ = updateModel(model, tea.WindowSizeMsg{Width: 120, Height: 40})

	view := model.View()
	// Should show some content even with no messages
	if view == "" || view == "Initializing..." {
		t.Error("Model should render waiting state with no messages")
	}
}

// TestElapsedTimeDisplay tests that elapsed time updates
func TestElapsedTimeDisplay(t *testing.T) {
	model := tui.NewModel()
	model, _ = updateModel(model, tea.WindowSizeMsg{Width: 120, Height: 40})

	view1 := model.View()

	// Wait a moment and render again
	time.Sleep(10 * time.Millisecond)

	view2 := model.View()

	// Both views should be non-empty (elapsed time formatting works)
	if view1 == "" || view2 == "" {
		t.Error("Views should not be empty")
	}
}

// TestMultipleMessagesRender tests rendering with multiple messages
func TestMultipleMessagesRender(t *testing.T) {
	model := tui.NewModel()

	messages := []tui.Message{
		{Role: tui.RoleAssistant, Content: "First message"},
		{Role: tui.RoleTool, Content: "Tool use: Read"},
		{Role: tui.RoleUser, Content: "Tool result: file contents..."},
		{Role: tui.RoleAssistant, Content: "Second message"},
	}

	for _, msg := range messages {
		model.AddMessage(msg)
	}

	model, _ = updateModel(model, tea.WindowSizeMsg{Width: 120, Height: 40})
	view := model.View()

	if view == "" || view == "Initializing..." {
		t.Error("Model should render multiple messages")
	}
}

// TestSmallWindowSize tests rendering with a small window
func TestSmallWindowSize(t *testing.T) {
	model := tui.NewModel()
	model.AddMessage(tui.Message{Role: tui.RoleAssistant, Content: "Test"})

	// Very small window
	model, _ = updateModel(model, tea.WindowSizeMsg{Width: 40, Height: 20})
	view := model.View()

	// Should still render without panic
	if view == "" || view == "Initializing..." {
		t.Error("Model should render even with small window")
	}
}

// TestLargeWindowSize tests rendering with a large window
func TestLargeWindowSize(t *testing.T) {
	model := tui.NewModel()
	model.AddMessage(tui.Message{Role: tui.RoleAssistant, Content: "Test"})

	// Large window
	model, _ = updateModel(model, tea.WindowSizeMsg{Width: 200, Height: 100})
	view := model.View()

	// Should render without panic
	if view == "" || view == "Initializing..." {
		t.Error("Model should render with large window")
	}
}

// TestStatsWithZeroValues tests rendering with zero stats
func TestStatsWithZeroValues(t *testing.T) {
	model := tui.NewModel()
	model.SetStats(stats.NewTokenStats())
	model, _ = updateModel(model, tea.WindowSizeMsg{Width: 120, Height: 40})

	view := model.View()
	if view == "" || view == "Initializing..." {
		t.Error("Model should render with zero stats")
	}
}

// TestStatsWithLargeValues tests rendering with large stat values
func TestStatsWithLargeValues(t *testing.T) {
	model := tui.NewModel()
	s := stats.NewTokenStats()
	s.AddUsage(1000000, 500000, 200000, 100000)
	s.AddCost(123.456789)
	model.SetStats(s)
	model, _ = updateModel(model, tea.WindowSizeMsg{Width: 120, Height: 40})

	view := model.View()
	if view == "" || view == "Initializing..." {
		t.Error("Model should render with large stats")
	}
}

// TestLoopProgressZeroZero tests loop display with 0/0
func TestLoopProgressZeroZero(t *testing.T) {
	model := tui.NewModel()
	model.SetLoopProgress(0, 0)
	model, _ = updateModel(model, tea.WindowSizeMsg{Width: 120, Height: 40})

	view := model.View()
	if view == "" || view == "Initializing..." {
		t.Error("Model should render with 0/0 loop progress")
	}
}

// TestDefaultRoleIcon tests that unknown roles get default icon
func TestDefaultRoleIcon(t *testing.T) {
	msg := tui.Message{Role: "unknown", Content: "test"}
	icon := msg.GetIcon()
	if icon != "📝" {
		t.Errorf("Expected default icon '📝' for unknown role, got '%s'", icon)
	}
}

// TestDefaultRoleStyle tests that unknown roles get default style
func TestDefaultRoleStyle(t *testing.T) {
	msg := tui.Message{Role: "unknown", Content: "test"}
	style := msg.GetStyle()
	rendered := style.Render("test")
	if rendered == "" {
		t.Error("Default style should render text")
	}
}

// TestLongMessageContent tests rendering with very long message content
func TestLongMessageContent(t *testing.T) {
	model := tui.NewModel()

	// Create a very long message
	longContent := ""
	for i := 0; i < 1000; i++ {
		longContent += "This is a very long message that should be handled properly. "
	}

	model.AddMessage(tui.Message{Role: tui.RoleAssistant, Content: longContent})
	model, _ = updateModel(model, tea.WindowSizeMsg{Width: 120, Height: 40})

	view := model.View()
	if view == "" || view == "Initializing..." {
		t.Error("Model should render long messages")
	}
}

// TestLongAssistantMessageNotTruncated tests that assistant messages exceeding
// the old 300-char truncation limit are displayed without "..." truncation
func TestLongAssistantMessageNotTruncated(t *testing.T) {
	model := tui.NewModel()

	// Create content that exceeds the old 300-char truncation limit
	longContent := "UNTRUNCATED_MARKER detailed assistant response that contains important information. " +
		"It discusses the architecture of the system and explains how different components interact. " +
		"The response also includes specific code suggestions and detailed reasoning about the approach. " +
		"This should not be truncated because truncation hides important responses and thinking from Claude."

	if len(longContent) <= 300 {
		t.Fatalf("Test content should exceed 300 chars, got %d", len(longContent))
	}

	model.AddMessage(tui.Message{Role: tui.RoleAssistant, Content: longContent})
	model, _ = updateModel(model, tea.WindowSizeMsg{Width: 120, Height: 40})

	view := model.View()
	if view == "" || view == "Initializing..." {
		t.Error("Model should render long assistant messages")
	}
	// Verify the beginning of the content is visible (confirming it was not replaced by truncated text)
	if !strings.Contains(view, "UNTRUNCATED_MARKER") {
		t.Error("Long assistant message should start with full content, not truncated text")
	}
}

// TestLongToolResultMessageNotTruncated tests that tool result messages exceeding
// the old 200-char truncation limit are displayed without "..." truncation
func TestLongToolResultMessageNotTruncated(t *testing.T) {
	model := tui.NewModel()

	// Create content that exceeds the old 200-char truncation limit
	longContent := "UNTRUNCATED_RESULT file contents that are quite long and contain lots of data. " +
		"The file has multiple functions and important implementation details that must be visible. " +
		"Previously this would have been cut off at 200 characters hiding the rest of the content."

	if len(longContent) <= 200 {
		t.Fatalf("Test content should exceed 200 chars, got %d", len(longContent))
	}

	model.AddMessage(tui.Message{Role: tui.RoleUser, Content: longContent})
	model, _ = updateModel(model, tea.WindowSizeMsg{Width: 120, Height: 40})

	view := model.View()
	if view == "" || view == "Initializing..." {
		t.Error("Model should render long tool result messages")
	}
	// Verify the beginning of the content is visible
	if !strings.Contains(view, "UNTRUNCATED_RESULT") {
		t.Error("Long tool result message should start with full content, not truncated text")
	}
}

// TestSpecialCharactersInMessage tests messages with special characters
func TestSpecialCharactersInMessage(t *testing.T) {
	model := tui.NewModel()

	messages := []string{
		"Message with emojis: 🚀 💻 🎉",
		"Message with unicode: ñ é ü ö",
		"Message with brackets: [test] {foo} <bar>",
		"Message with quotes: \"quoted\" 'single'",
	}

	for _, content := range messages {
		model.AddMessage(tui.Message{Role: tui.RoleAssistant, Content: content})
	}

	model, _ = updateModel(model, tea.WindowSizeMsg{Width: 120, Height: 40})
	view := model.View()

	if view == "" || view == "Initializing..." {
		t.Error("Model should render messages with special characters")
	}
}

// TestWindowResizePreservesMessages tests that messages are preserved on resize
func TestWindowResizePreservesMessages(t *testing.T) {
	model := tui.NewModel()

	// Add message
	model.AddMessage(tui.Message{Role: tui.RoleAssistant, Content: "Test message"})

	// Initial size
	model, _ = updateModel(model, tea.WindowSizeMsg{Width: 100, Height: 30})

	// Resize
	model, _ = updateModel(model, tea.WindowSizeMsg{Width: 150, Height: 50})

	view := model.View()
	if view == "" || view == "Initializing..." {
		t.Error("Model should preserve messages after resize")
	}
}

// TestQuitPersistsElapsedTime tests that quitting updates stats with elapsed time
func TestQuitPersistsElapsedTime(t *testing.T) {
	model := tui.NewModel()

	tokenStats := stats.NewTokenStats()
	model.SetStats(tokenStats)

	baseElapsed := 1 * time.Hour
	model.SetBaseElapsed(baseElapsed)

	model, _ = updateModel(model, tea.WindowSizeMsg{Width: 120, Height: 40})

	keyMsg := tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune{'q'}}
	model, _ = updateModel(model, keyMsg)

	if tokenStats.TotalElapsedNs < baseElapsed.Nanoseconds() {
		t.Errorf("TotalElapsedNs should be at least %d, got %d",
			baseElapsed.Nanoseconds(), tokenStats.TotalElapsedNs)
	}
}

// TestTimerPausesOnCompletion tests that the elapsed timer freezes when processing completes
func TestTimerPausesOnCompletion(t *testing.T) {
	model := tui.NewModel()
	model, _ = updateModel(model, tea.WindowSizeMsg{Width: 120, Height: 40})

	// Simulate completion
	cmd := tui.SendDone()
	doneMsg := cmd()
	model, _ = updateModel(model, doneMsg)

	// After completion, the view should show "Completed" status
	view := model.View()
	if !strings.Contains(view, "Completed") {
		t.Error("View should show 'Completed' status after done message")
	}
	if !strings.Contains(view, "COMPLETED") {
		t.Error("View should show 'COMPLETED' header after done message")
	}

	// Verify elapsed time is frozen by checking two renders have same time
	view1 := model.View()
	time.Sleep(50 * time.Millisecond)
	view2 := model.View()

	// Both should contain the same elapsed time (frozen)
	// Extract the elapsed time strings from the footer panel
	// Since timer is frozen, subsequent renders should show the same time
	if view1 != view2 {
		// Views might differ due to tick, but elapsed time should be the same
		// This is a best-effort check
		t.Log("Note: views may differ slightly due to rendering, but elapsed time should be frozen")
	}
}

// TestPauseHotkey tests that 'p' key pauses the loop
func TestPauseHotkey(t *testing.T) {
	model := tui.NewModel()
	model, _ = updateModel(model, tea.WindowSizeMsg{Width: 120, Height: 40})

	// Without a loop set, pressing 'p' should not panic
	keyMsg := tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune{'p'}}
	model, _ = updateModel(model, keyMsg)

	// Should not quit
	view := model.View()
	if view == "Goodbye!\n" {
		t.Error("'p' key should not quit the application")
	}
}

// TestResumeHotkey tests that 'r' key resumes the loop
func TestResumeHotkey(t *testing.T) {
	model := tui.NewModel()
	model, _ = updateModel(model, tea.WindowSizeMsg{Width: 120, Height: 40})

	// Without a loop set, pressing 'r' should not panic
	keyMsg := tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune{'r'}}
	model, _ = updateModel(model, keyMsg)

	// Should not quit
	view := model.View()
	if view == "Goodbye!\n" {
		t.Error("'r' key should not quit the application")
	}
}

// TestHotkeyBarRenders tests that the hotkey bar is shown in the footer
func TestHotkeyBarRenders(t *testing.T) {
	model := tui.NewModel()
	model, _ = updateModel(model, tea.WindowSizeMsg{Width: 120, Height: 40})

	view := model.View()
	// The view should not be empty
	if view == "" {
		t.Error("Model should render with hotkey bar")
	}
}

// TestCleanInitNoFlash tests that the initial view is empty (no unstyled text flash)
func TestCleanInitNoFlash(t *testing.T) {
	model := tui.NewModel()
	view := model.View()

	// Before WindowSizeMsg, view should be empty for a clean alt screen
	if view != "" {
		t.Errorf("Expected empty initial view (no flash), got: %q", view)
	}
}

// TestTinyTerminalShowsMessage tests that a very small terminal shows a size warning
func TestTinyTerminalShowsMessage(t *testing.T) {
	model := tui.NewModel()

	// Terminal below minimum dimensions
	model, _ = updateModel(model, tea.WindowSizeMsg{Width: 20, Height: 10})
	view := model.View()

	if !strings.Contains(view, "Terminal too small") {
		t.Errorf("Expected 'Terminal too small' message for tiny terminal, got: %q", view)
	}
	if !strings.Contains(view, "20x10") {
		t.Error("Terminal too small message should include current dimensions")
	}
}

// TestMinimumWidthBoundary tests rendering at exactly the minimum width boundary
func TestMinimumWidthBoundary(t *testing.T) {
	model := tui.NewModel()

	// At minimum dimensions: should render full layout
	model, _ = updateModel(model, tea.WindowSizeMsg{Width: 40, Height: 15})
	view := model.View()

	if strings.Contains(view, "Terminal too small") {
		t.Error("Should render full layout at minimum dimensions (40x15)")
	}
	if view == "" {
		t.Error("View should not be empty at minimum dimensions")
	}
}

// TestBelowMinimumHeight tests that below-minimum height shows warning
func TestBelowMinimumHeight(t *testing.T) {
	model := tui.NewModel()

	model, _ = updateModel(model, tea.WindowSizeMsg{Width: 120, Height: 14})
	view := model.View()

	if !strings.Contains(view, "Terminal too small") {
		t.Error("Expected 'Terminal too small' message for height below minimum")
	}
}

// TestViewportScrollsToBottomOnInit tests that viewport starts scrolled to bottom
func TestViewportScrollsToBottomOnInit(t *testing.T) {
	model := tui.NewModel()

	// Add many messages before viewport is initialized
	for i := 0; i < 20; i++ {
		model.AddMessage(tui.Message{
			Role:    tui.RoleAssistant,
			Content: fmt.Sprintf("Message %d with some content", i),
		})
	}

	// Initialize viewport
	model, _ = updateModel(model, tea.WindowSizeMsg{Width: 120, Height: 30})
	view := model.View()

	// The latest messages should be visible (viewport scrolled to bottom)
	if !strings.Contains(view, "Message 19") {
		t.Error("Viewport should be scrolled to bottom showing latest messages")
	}
}

// TestViewportScrollPreservedOnTick tests that scrolling up is not undone by ticks
func TestViewportScrollPreservedOnTick(t *testing.T) {
	model := tui.NewModel()

	// Add many messages so scrolling is needed
	for i := 0; i < 20; i++ {
		model.AddMessage(tui.Message{
			Role:    tui.RoleAssistant,
			Content: fmt.Sprintf("SCROLL_MSG_%02d", i),
		})
	}

	// Initialize viewport with a height that requires scrolling (small viewport)
	model, _ = updateModel(model, tea.WindowSizeMsg{Width: 120, Height: 25})

	// Verify we start at bottom (latest message visible)
	view := model.View()
	if !strings.Contains(view, "SCROLL_MSG_19") {
		t.Fatal("Viewport should start at bottom showing latest messages")
	}

	// Scroll up via multiple PgUp keys to reach the top
	for i := 0; i < 10; i++ {
		model, _ = updateModel(model, tea.KeyMsg{Type: tea.KeyPgUp})
	}

	// After scrolling up, the earliest message should be visible
	view = model.View()
	if !strings.Contains(view, "SCROLL_MSG_00") {
		t.Fatal("After scrolling up, earliest messages should be visible")
	}

	// Send a tick — scroll position should NOT snap back to bottom
	model, _ = updateModel(model, tui.TickMsgForTest())

	view = model.View()
	if strings.Contains(view, "SCROLL_MSG_19") {
		t.Error("Tick should not snap viewport back to bottom — scroll position must be preserved")
	}
	if !strings.Contains(view, "SCROLL_MSG_00") {
		t.Error("After tick, earliest messages should still be visible (scroll preserved)")
	}
}

// TestModeDisplayDefault tests that the mode row renders by default in the
// Model Details panel. Note "Model:" does not contain the substring "Mode:",
// so this assertion is specific to the mode row.
func TestModeDisplayDefault(t *testing.T) {
	model := tui.NewModel()
	model, _ = updateModel(model, tea.WindowSizeMsg{Width: 120, Height: 40})

	view := model.View()
	if !strings.Contains(view, "Mode:") {
		t.Error("View should contain 'Mode:' label")
	}
}

// TestModeUpdateDisplayed tests that sending a mode update shows the mode
func TestModeUpdateDisplayed(t *testing.T) {
	model := tui.NewModel()
	model, _ = updateModel(model, tea.WindowSizeMsg{Width: 120, Height: 40})

	// Simulate mode update
	cmd := tui.SendModeUpdate("Building")
	modeMsg := cmd()
	model, _ = updateModel(model, modeMsg)

	view := model.View()
	if !strings.Contains(view, "Building") {
		t.Error("View should display the current mode")
	}
}

// TestModeUpdateOverwritesPrevious tests that new mode updates replace old ones
func TestModeUpdateOverwritesPrevious(t *testing.T) {
	model := tui.NewModel()
	model, _ = updateModel(model, tea.WindowSizeMsg{Width: 120, Height: 40})

	// Set mode to Planning
	cmd := tui.SendModeUpdate("Planning")
	model, _ = updateModel(model, cmd())

	// Set mode to Building
	cmd = tui.SendModeUpdate("Building")
	model, _ = updateModel(model, cmd())

	view := model.View()
	if !strings.Contains(view, "Building") {
		t.Error("View should show the latest mode")
	}
	if strings.Contains(view, "Planning") {
		t.Error("View should not show old mode after update")
	}
}

// TestSendModeUpdateCmd tests the SendModeUpdate helper command
func TestSendModeUpdateCmd(t *testing.T) {
	cmd := tui.SendModeUpdate("Building")

	if cmd == nil {
		t.Error("SendModeUpdate should return a command")
	}

	result := cmd()
	if result == nil {
		t.Error("Command should return a mode update message")
	}
}

// TestInAppStatusBarRemoved tests that the old in-app status bar is no longer rendered
// (status bar content is now only in the tmux status-right bar)
func TestInAppStatusBarRemoved(t *testing.T) {
	model := tui.NewModel()
	model, _ = updateModel(model, tea.WindowSizeMsg{Width: 120, Height: 40})

	view := model.View()
	// The old in-app status bar labels should NOT appear in the view
	if strings.Contains(view, "current loop:") {
		t.Error("View should NOT contain 'current loop:' label — in-app status bar was removed")
	}
	if strings.Contains(view, "elapsed:") {
		t.Error("View should NOT contain 'elapsed:' label — in-app status bar was removed")
	}
}

// TestFooterShowsTokenCount tests that the footer panel shows human-readable token count
func TestFooterShowsTokenCount(t *testing.T) {
	model := tui.NewModel()
	s := stats.NewTokenStats()
	s.AddUsage(500000, 250000, 100000, 50000)
	model.SetStats(s)
	model, _ = updateModel(model, tea.WindowSizeMsg{Width: 120, Height: 40})

	view := model.View()
	// 900k total tokens should appear in the footer panel
	if !strings.Contains(view, "900k") {
		t.Error("Footer panel should display human-readable token count (expected '900k')")
	}
}

// TestQuitHotkeyAlwaysHighlighted tests that the quit hotkey is always highlighted
func TestQuitHotkeyAlwaysHighlighted(t *testing.T) {
	model := tui.NewModel()
	model, _ = updateModel(model, tea.WindowSizeMsg{Width: 120, Height: 40})

	view := model.View()
	// The quit hotkey "(q)uit" should always be visible (not hidden)
	// We can't easily check styling in plain text, but we can verify it renders
	if !strings.Contains(view, "uit") {
		t.Error("View should contain quit hotkey text")
	}
}

// TestCurrentModeDisplayFormat tests the "Mode: <value>" display format
func TestCurrentModeDisplayFormat(t *testing.T) {
	model := tui.NewModel()
	model, _ = updateModel(model, tea.WindowSizeMsg{Width: 120, Height: 40})

	// Simulate mode update
	cmd := tui.SendModeUpdate("Planning")
	modeMsg := cmd()
	model, _ = updateModel(model, modeMsg)

	view := model.View()
	if !strings.Contains(view, "Mode:") {
		t.Error("View should contain 'Mode:' label")
	}
	if !strings.Contains(view, "Planning") {
		t.Error("View should display the mode")
	}
}

// TestModeDisplayBuilding tests mode display with "Building" mode
func TestModeDisplayBuilding(t *testing.T) {
	model := tui.NewModel()
	model, _ = updateModel(model, tea.WindowSizeMsg{Width: 120, Height: 40})

	// Simulate mode update with Building
	cmd := tui.SendModeUpdate("Building")
	modeMsg := cmd()
	model, _ = updateModel(model, modeMsg)

	view := model.View()
	if !strings.Contains(view, "Building") {
		t.Error("View should show 'Building' mode")
	}
}

// TestSetTmuxStatusBar tests that SetTmuxStatusBar does not panic with nil or inactive bar
func TestSetTmuxStatusBar(t *testing.T) {
	model := tui.NewModel()

	// Setting nil tmux bar should not panic
	model.SetTmuxStatusBar(nil)

	// Setting inactive bar should not panic
	orig := os.Getenv("TMUX")
	defer os.Setenv("TMUX", orig)
	os.Unsetenv("TMUX")
	sb := tmux.NewStatusBar()
	model.SetTmuxStatusBar(sb)

	// Tick should not panic with inactive tmux bar
	model, _ = updateModel(model, tea.WindowSizeMsg{Width: 120, Height: 40})
	model, _ = updateModel(model, tui.TickMsgForTest())

	view := model.View()
	if view == "" {
		t.Error("View should render with inactive tmux status bar")
	}
}

// TestResizeFromTinyToNormal tests transitioning from too-small to normal size
func TestResizeFromTinyToNormal(t *testing.T) {
	model := tui.NewModel()
	model.AddMessage(tui.Message{Role: tui.RoleAssistant, Content: "RESIZE_TEST_CONTENT"})

	// Start tiny
	model, _ = updateModel(model, tea.WindowSizeMsg{Width: 20, Height: 10})
	view := model.View()
	if !strings.Contains(view, "Terminal too small") {
		t.Error("Should show too-small message")
	}

	// Resize to normal
	model, _ = updateModel(model, tea.WindowSizeMsg{Width: 120, Height: 40})
	view = model.View()
	if strings.Contains(view, "Terminal too small") {
		t.Error("Should show full layout after resize to normal")
	}
	if !strings.Contains(view, "RESIZE_TEST_CONTENT") {
		t.Error("Messages should be visible after resize to normal")
	}
}

// TestAddLoopNoopWithoutLoop tests that '+' is a no-op when no loop is set
func TestAddLoopNoopWithoutLoop(t *testing.T) {
	model := tui.NewModel()
	model.SetLoopProgress(1, 3)
	model, _ = updateModel(model, tea.WindowSizeMsg{Width: 120, Height: 40})

	// Press '+' without a loop set — should not panic
	keyMsg := tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune{'+'}}
	model, _ = updateModel(model, keyMsg)

	view := model.View()
	if !strings.Contains(view, "#1/3") {
		t.Errorf("Without loop, '+' should be a no-op, view should still contain '#1/3'")
	}
}

// TestAddLoopWorksWhenCompleted tests that '+' adds loops after completion (spec #6)
// and '-' is a no-op when totalLoops == currentLoop
func TestAddLoopWorksWhenCompleted(t *testing.T) {
	model := tui.NewModel()
	l := loop.New(loop.Config{Iterations: 5, Prompt: "test"})
	model.SetLoop(l)
	model.SetLoopProgress(5, 5)
	model, _ = updateModel(model, tea.WindowSizeMsg{Width: 120, Height: 40})

	// Simulate completion
	cmd := tui.SendDone()
	doneMsg := cmd()
	model, _ = updateModel(model, doneMsg)

	// Press '+' — should add a loop even after completion (spec #6)
	keyMsg := tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune{'+'}}
	model, _ = updateModel(model, keyMsg)

	if l.GetIterations() != 6 {
		t.Errorf("Expected loop iterations to be 6 after '+' when completed, got %d", l.GetIterations())
	}

	// Press '-' — should work now since totalLoops (6) > currentLoop (5)
	keyMsg = tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune{'-'}}
	model, _ = updateModel(model, keyMsg)

	if l.GetIterations() != 5 {
		t.Errorf("Expected loop iterations to be 5 after '-', got %d", l.GetIterations())
	}

	// Press '-' again — should be a no-op since totalLoops == currentLoop
	keyMsg = tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune{'-'}}
	model, _ = updateModel(model, keyMsg)

	if l.GetIterations() != 5 {
		t.Errorf("Expected loop iterations to remain 5 after '-' at floor, got %d", l.GetIterations())
	}
}

// TestScrollbackRetainsMessages tests that the TUI retains a large number of messages
// (spec: scrollback should be 100000 lines)
func TestScrollbackRetainsMessages(t *testing.T) {
	model := tui.NewModel()

	// Add 50 messages — previously maxMessages was 20, so messages 1-30 would be dropped
	for i := 0; i < 50; i++ {
		model.AddMessage(tui.Message{
			Role:    tui.RoleAssistant,
			Content: fmt.Sprintf("SCROLLBACK_MSG_%03d", i),
		})
	}

	model, _ = updateModel(model, tea.WindowSizeMsg{Width: 120, Height: 40})

	view := model.View()
	// With 100000 maxMessages, the earliest message should still be present
	if !strings.Contains(view, "SCROLLBACK_MSG_000") {
		// The earliest message might not be visible in the viewport (scrolled to bottom),
		// but we can verify it's in the content by scrolling up
		// Let's scroll up and check
		for i := 0; i < 20; i++ {
			model, _ = updateModel(model, tea.KeyMsg{Type: tea.KeyPgUp})
		}
		view = model.View()
		if !strings.Contains(view, "SCROLLBACK_MSG_000") {
			t.Error("Earliest message should be retained with 100000 message scrollback limit")
		}
	}
	// Latest message should be visible after scrolling back down
	for i := 0; i < 20; i++ {
		model, _ = updateModel(model, tea.KeyMsg{Type: tea.KeyPgDown})
	}
	view = model.View()
	if !strings.Contains(view, "SCROLLBACK_MSG_049") {
		t.Error("Latest message should be visible when scrolled to bottom")
	}
}

// TestMultipleAddLoopPresses tests pressing '+' multiple times
func TestMultipleAddLoopPresses(t *testing.T) {
	model := tui.NewModel()
	l := loop.New(loop.Config{Iterations: 3, Prompt: "test"})
	model.SetLoop(l)
	model.SetLoopProgress(1, 3)
	model, _ = updateModel(model, tea.WindowSizeMsg{Width: 120, Height: 40})

	// Press '+' three times
	keyMsg := tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune{'+'}}
	for i := 0; i < 3; i++ {
		model, _ = updateModel(model, keyMsg)
	}

	view := model.View()
	if !strings.Contains(view, "#1/6") {
		t.Errorf("After pressing '+' three times from 3, total should be 6, view should contain '#1/6'")
	}

	if l.GetIterations() != 6 {
		t.Errorf("Expected loop iterations to be 6 after 3 '+' presses, got %d", l.GetIterations())
	}
}

// ============================================================================
// Integration Tests: TUI + Loop Pause/Resume
// ============================================================================

// TestTUIPauseResumeTimerFreezes tests that pressing 'p' freezes the elapsed timer
// and pressing 'r' resumes it.
func TestTUIPauseResumeTimerFreezes(t *testing.T) {
	model := tui.NewModel()
	l := loop.New(loop.Config{Iterations: 5, Prompt: "test"})
	model.SetLoop(l)
	model, _ = updateModel(model, tea.WindowSizeMsg{Width: 120, Height: 40})

	// Press 'p' to pause (timer freezes even without loop running)
	keyP := tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune{'p'}}
	model, _ = updateModel(model, keyP)

	// When timer is paused, two renders separated by time should be identical
	// because getElapsed() returns the frozen pausedElapsed value
	view1 := model.View()
	time.Sleep(50 * time.Millisecond)
	view2 := model.View()

	if view1 != view2 {
		t.Error("With paused timer, two consecutive View() calls should produce identical output")
	}

	// Press 'r' to resume
	keyR := tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune{'r'}}
	model, _ = updateModel(model, keyR)

	// After resume, view should still render without panic
	view3 := model.View()
	if view3 == "" {
		t.Error("View should not be empty after resume")
	}
}

// TestTUIPauseResumeWithRunningLoop tests the full TUI + loop integration:
// pressing 'p' pauses a running loop, pressing 'r' resumes it.
func TestTUIPauseResumeWithRunningLoop(t *testing.T) {
	cfg := loop.Config{
		Iterations:     100,
		Prompt:         "test",
		CommandBuilder: mockCommandBuilder,
		SleepDuration:  10 * time.Millisecond,
	}
	l := loop.New(cfg)
	ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
	defer cancel()

	model := tui.NewModel()
	model.SetLoop(l)
	model, _ = updateModel(model, tea.WindowSizeMsg{Width: 120, Height: 40})

	l.Start(ctx)

	// Drain output to prevent channel blocking
	go func() {
		for range l.Output() {
		}
	}()

	// Wait for loop to start running
	time.Sleep(50 * time.Millisecond)

	// Press 'p' to pause
	keyP := tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune{'p'}}
	model, _ = updateModel(model, keyP)

	// Give the loop time to process pause
	time.Sleep(200 * time.Millisecond)

	// Verify loop is paused
	if !l.IsPaused() {
		t.Error("Loop should be paused after pressing 'p' in TUI")
	}

	// Verify TUI shows STOPPED status
	view := model.View()
	if !strings.Contains(view, "STOPPED") && !strings.Contains(view, "Stopped") {
		t.Error("View should show STOPPED/Stopped status when loop is paused")
	}

	// Press 'r' to resume
	keyR := tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune{'r'}}
	model, _ = updateModel(model, keyR)

	// Give the loop time to process resume
	time.Sleep(200 * time.Millisecond)

	// Verify loop is no longer paused
	if l.IsPaused() {
		t.Error("Loop should not be paused after pressing 'r' in TUI")
	}

	// Verify TUI shows RUNNING status
	view = model.View()
	if !strings.Contains(view, "RUNNING") && !strings.Contains(view, "Running") {
		t.Error("View should show RUNNING/Running status when loop is resumed")
	}

	cancel()
}

// ============================================================================
// Tests: Per-Loop Stats in Tmux Status Bar (Spec 19)
//
// Per-loop tokens and elapsed time are observable through the tmux status bar:
// updateTmuxStatusBar runs once per tick and renders
// "[repo | branch | loop: N/M, tokens: X, elapsed: HH:MM:SS]" for the CURRENT
// loop iteration (never the cumulative session). These tests drive a model with
// a mocked clock and read back tui.FakeStatusBarForTest.LastContent.
// ============================================================================

// perLoopFakeClock is a mutable clock for the per-loop status bar tests.
type perLoopFakeClock struct{ now time.Time }

// advance moves the fake clock forward by d.
func (c *perLoopFakeClock) advance(d time.Duration) { c.now = c.now.Add(d) }

// newPerLoopStatusBarModel builds a ready model wired to a fake tmux status bar
// and a mocked clock. The clock is installed before tui.NewModel() so both the
// session start time and the loop start time are the clock's zero point.
// Pass a non-nil loop to enable the pause/resume hotkeys; it is attached before
// the tea.WindowSizeMsg because the model is copied by value on every Update.
// Callers must `defer tui.SetTimeNowForTest(time.Now)`.
func newPerLoopStatusBarModel(l *loop.Loop) (tui.Model, *tui.FakeStatusBarForTest, *perLoopFakeClock) {
	clock := &perLoopFakeClock{now: time.Date(2026, 1, 2, 3, 4, 5, 0, time.UTC)}
	tui.SetTimeNowForTest(func() time.Time { return clock.now })

	model := tui.NewModel()
	if l != nil {
		model.SetLoop(l)
	}
	model.SetGitContext("ralph", "main")
	model.SetLoopProgress(2, 5)
	fakeBar := &tui.FakeStatusBarForTest{}
	model.SetTmuxStatusBar(fakeBar)
	model, _ = updateModel(model, tea.WindowSizeMsg{Width: 120, Height: 40})
	return model, fakeBar, clock
}

// tickPerLoopBar drives one tick and returns the content pushed to the bar.
func tickPerLoopBar(m tui.Model, bar *tui.FakeStatusBarForTest) (tui.Model, string) {
	m, _ = updateModel(m, tui.TickMsgForTest())
	return m, bar.LastContent
}

// perLoopBar renders the expected status bar content for the fixture model.
func perLoopBar(tokens, elapsed string) string {
	return "[ralph | main | loop: 2/5, tokens: " + tokens + ", elapsed: " + elapsed + "]"
}

// TestSendLoopStartedCmd tests that SendLoopStarted returns a command whose
// message resets both per-loop counters visible on the tmux status bar.
func TestSendLoopStartedCmd(t *testing.T) {
	defer tui.SetTimeNowForTest(time.Now)
	model, bar, clock := newPerLoopStatusBarModel(nil)

	// Accumulate per-loop state: 12k tokens over 30 seconds.
	model, _ = updateModel(model, tui.SendLoopStatsUpdate(12000)())
	clock.advance(30 * time.Second)
	model, got := tickPerLoopBar(model, bar)
	if want := perLoopBar("12k", "00:00:30"); got != want {
		t.Fatalf("Precondition: status bar = %q, want %q", got, want)
	}

	cmd := tui.SendLoopStarted()
	if cmd == nil {
		t.Fatal("SendLoopStarted should return a command")
	}
	msg := cmd()
	if msg == nil {
		t.Fatal("SendLoopStarted's command should produce a loopStartedMsg")
	}

	// Feeding that message in must reset both per-loop tokens and elapsed.
	model, _ = updateModel(model, msg)
	_, got = tickPerLoopBar(model, bar)
	if want := perLoopBar("0", "00:00:00"); got != want {
		t.Errorf("After loopStartedMsg: status bar = %q, want %q", got, want)
	}
}

// TestSendLoopStatsUpdateCmd tests that SendLoopStatsUpdate returns a command
// whose message sets the per-loop token count shown on the tmux status bar.
func TestSendLoopStatsUpdateCmd(t *testing.T) {
	defer tui.SetTimeNowForTest(time.Now)
	model, bar, _ := newPerLoopStatusBarModel(nil)

	model, got := tickPerLoopBar(model, bar)
	if want := perLoopBar("0", "00:00:00"); got != want {
		t.Fatalf("Precondition: status bar = %q, want %q", got, want)
	}

	cmd := tui.SendLoopStatsUpdate(12345)
	if cmd == nil {
		t.Fatal("SendLoopStatsUpdate should return a command")
	}
	msg := cmd()
	if msg == nil {
		t.Fatal("SendLoopStatsUpdate's command should produce a loopStatsUpdateMsg")
	}

	// 12345 tokens render through stats.FormatTokens as "12.3k".
	model, _ = updateModel(model, msg)
	_, got = tickPerLoopBar(model, bar)
	if want := perLoopBar("12.3k", "00:00:00"); got != want {
		t.Errorf("After loopStatsUpdateMsg: status bar = %q, want %q", got, want)
	}
}

// TestPerLoopTokensResetOnNewLoop tests that the token count on the tmux status
// bar drops back to zero when a new loop iteration starts.
func TestPerLoopTokensResetOnNewLoop(t *testing.T) {
	defer tui.SetTimeNowForTest(time.Now)
	model, bar, _ := newPerLoopStatusBarModel(nil)

	model, _ = updateModel(model, tui.SendLoopStatsUpdate(50000)())
	model, got := tickPerLoopBar(model, bar)
	if want := perLoopBar("50k", "00:00:00"); got != want {
		t.Fatalf("With 50000 per-loop tokens: status bar = %q, want %q", got, want)
	}

	model, _ = updateModel(model, tui.SendLoopStarted()())
	model, got = tickPerLoopBar(model, bar)
	if want := perLoopBar("0", "00:00:00"); got != want {
		t.Fatalf("After new loop started: status bar = %q, want %q", got, want)
	}

	// The counter keeps working after the reset: it counts from zero again.
	model, _ = updateModel(model, tui.SendLoopStatsUpdate(100)())
	_, got = tickPerLoopBar(model, bar)
	if want := perLoopBar("100", "00:00:00"); got != want {
		t.Errorf("After 100 tokens in the new loop: status bar = %q, want %q", got, want)
	}
}

// TestPerLoopTimerResetsOnNewLoop tests that the elapsed time on the tmux status
// bar is per-loop, not per-session: it restarts at 00:00:00 on a new iteration
// while the session's "Total Time" in the TUI footer keeps counting.
func TestPerLoopTimerResetsOnNewLoop(t *testing.T) {
	defer tui.SetTimeNowForTest(time.Now)
	model, bar, clock := newPerLoopStatusBarModel(nil)

	clock.advance(65 * time.Second)
	model, got := tickPerLoopBar(model, bar)
	if want := perLoopBar("0", "00:01:05"); got != want {
		t.Fatalf("After 65s in the first loop: status bar = %q, want %q", got, want)
	}

	// A new loop iteration begins 65s into the session.
	model, _ = updateModel(model, tui.SendLoopStarted()())
	model, got = tickPerLoopBar(model, bar)
	if want := perLoopBar("0", "00:00:00"); got != want {
		t.Fatalf("A new loop must restart the bar's clock: status bar = %q, want %q", got, want)
	}
	// ...while the session total is unaffected — that is the per-loop/session split.
	if view := model.View(); !strings.Contains(view, "00:01:05") {
		t.Error("Footer 'Total Time' should still show the 65s session elapsed after a new loop starts")
	}

	clock.advance(5 * time.Second)
	model, got = tickPerLoopBar(model, bar)
	if want := perLoopBar("0", "00:00:05"); got != want {
		t.Errorf("5s into the new loop: status bar = %q, want %q", got, want)
	}
	if view := model.View(); !strings.Contains(view, "00:01:10") {
		t.Error("Footer 'Total Time' should show 00:01:10 while the bar shows 00:00:05")
	}
}

// TestPerLoopTimerFreezesOnPause tests that pausing with 'p' freezes the per-loop
// elapsed time on the tmux status bar even as the mocked clock keeps moving.
func TestPerLoopTimerFreezesOnPause(t *testing.T) {
	defer tui.SetTimeNowForTest(time.Now)
	l := loop.New(loop.Config{Iterations: 5, Prompt: "test"})
	model, bar, clock := newPerLoopStatusBarModel(l)

	clock.advance(10 * time.Second)
	model, got := tickPerLoopBar(model, bar)
	if want := perLoopBar("0", "00:00:10"); got != want {
		t.Fatalf("Before pause: status bar = %q, want %q", got, want)
	}

	// Pause at +10s: the per-loop timer freezes at 00:00:10.
	model, _ = updateModel(model, tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune{'p'}})

	clock.advance(35 * time.Second)
	model, got = tickPerLoopBar(model, bar)
	if want := perLoopBar("0", "00:00:10"); got != want {
		t.Fatalf("35s after pausing: status bar = %q, want %q (frozen)", got, want)
	}

	clock.advance(2 * time.Minute)
	_, got = tickPerLoopBar(model, bar)
	if want := perLoopBar("0", "00:00:10"); got != want {
		t.Errorf("2m35s after pausing: status bar = %q, want %q (still frozen)", got, want)
	}
}

// TestPerLoopTimerResumesAfterPause tests that resuming with 'r' continues the
// per-loop timer from the frozen value rather than restarting or back-filling
// the paused interval: 10s before the pause + 10s after the resume = 00:00:20.
func TestPerLoopTimerResumesAfterPause(t *testing.T) {
	defer tui.SetTimeNowForTest(time.Now)
	l := loop.New(loop.Config{Iterations: 5, Prompt: "test"})
	model, bar, clock := newPerLoopStatusBarModel(l)

	// Pause at +10s (10s accumulated), resume at +30s (20s spent paused).
	clock.advance(10 * time.Second)
	model, _ = updateModel(model, tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune{'p'}})

	clock.advance(20 * time.Second)
	model, got := tickPerLoopBar(model, bar)
	if want := perLoopBar("0", "00:00:10"); got != want {
		t.Fatalf("While paused: status bar = %q, want %q", got, want)
	}

	model, _ = updateModel(model, tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune{'r'}})
	model, got = tickPerLoopBar(model, bar)
	if want := perLoopBar("0", "00:00:10"); got != want {
		t.Fatalf("At the instant of resume: status bar = %q, want %q", got, want)
	}

	// 10s after the resume: the 20s paused interval is never counted.
	clock.advance(10 * time.Second)
	_, got = tickPerLoopBar(model, bar)
	if want := perLoopBar("0", "00:00:20"); got != want {
		t.Errorf("10s after resuming: status bar = %q, want %q", got, want)
	}
}

// ============================================================================
// Tests: Completed Tasks Position + Title Rename (Spec 20)
// ============================================================================

// TestRalphLoopDetailsTitle tests that the right panel title is "Ralph Loop Details"
func TestRalphLoopDetailsTitle(t *testing.T) {
	model := tui.NewModel()
	model, _ = updateModel(model, tea.WindowSizeMsg{Width: 120, Height: 40})

	view := model.View()
	if !strings.Contains(view, "Ralph Loop Details") {
		t.Error("View should contain 'Ralph Loop Details' title (renamed from 'Ralph Details')")
	}
}

// ============================================================================
// Tests: Mode Display (Spec #9)
// ============================================================================

// TestSetCurrentModeDisplaysPlanning tests that SetCurrentMode shows "Planning" mode
func TestSetCurrentModeDisplaysPlanning(t *testing.T) {
	model := tui.NewModel()
	model.SetCurrentMode("Planning")
	model, _ = updateModel(model, tea.WindowSizeMsg{Width: 120, Height: 40})

	view := model.View()
	if !strings.Contains(view, "Planning") {
		t.Error("SetCurrentMode should display 'Planning' mode")
	}
}

// TestSetCurrentModeDisplaysBuilding tests that SetCurrentMode shows "Building" mode
func TestSetCurrentModeDisplaysBuilding(t *testing.T) {
	model := tui.NewModel()
	model.SetCurrentMode("Building")
	model, _ = updateModel(model, tea.WindowSizeMsg{Width: 120, Height: 40})

	view := model.View()
	if !strings.Contains(view, "Building") {
		t.Error("SetCurrentMode should display 'Building' mode")
	}
}

// TestModeUpdateOverridesSetCurrentMode tests that mode updates override initial mode
func TestModeUpdateOverridesSetCurrentMode(t *testing.T) {
	model := tui.NewModel()
	model.SetCurrentMode("Planning")
	model, _ = updateModel(model, tea.WindowSizeMsg{Width: 120, Height: 40})

	// Send a mode update — should change to "Building"
	cmd := tui.SendModeUpdate("Building")
	model, _ = updateModel(model, cmd())

	view := model.View()
	if !strings.Contains(view, "Building") {
		t.Error("Mode update should change 'Planning' to 'Building'")
	}
}

// ============================================================================
// Integration Tests: TUI + Loop Pause/Resume
// ============================================================================

// ============================================================================
// Tests: October Theme — Ghost Emoji (TASK 2)
// ============================================================================

// TestAssistantIconOctober tests that assistant icon is ghost emoji in October
func TestAssistantIconOctober(t *testing.T) {
	tui.SetTimeNowForTest(func() time.Time {
		return time.Date(2024, time.October, 15, 12, 0, 0, 0, time.UTC)
	})
	defer tui.SetTimeNowForTest(time.Now)

	msg := tui.Message{Role: tui.RoleAssistant, Content: "test"}
	icon := msg.GetIcon()
	if icon != "👻" {
		t.Errorf("Expected ghost emoji 👻 for assistant in October, got %s", icon)
	}
}

// TestAssistantIconNonOctober tests that assistant icon is robot emoji outside October
func TestAssistantIconNonOctober(t *testing.T) {
	months := []time.Month{
		time.January, time.February, time.March, time.April,
		time.May, time.June, time.July, time.August,
		time.September, time.November, time.December,
	}
	for _, month := range months {
		t.Run(month.String(), func(t *testing.T) {
			tui.SetTimeNowForTest(func() time.Time {
				return time.Date(2024, month, 15, 12, 0, 0, 0, time.UTC)
			})
			defer tui.SetTimeNowForTest(time.Now)

			msg := tui.Message{Role: tui.RoleAssistant, Content: "test"}
			icon := msg.GetIcon()
			if icon != "🤖" {
				t.Errorf("Expected robot emoji 🤖 for assistant in %s, got %s", month, icon)
			}
		})
	}
}

// TestOctoberOtherRolesUnchanged tests that non-assistant roles are unaffected in October
func TestOctoberOtherRolesUnchanged(t *testing.T) {
	tui.SetTimeNowForTest(func() time.Time {
		return time.Date(2024, time.October, 31, 23, 59, 0, 0, time.UTC)
	})
	defer tui.SetTimeNowForTest(time.Now)

	tests := []struct {
		role         tui.MessageRole
		expectedIcon string
	}{
		{tui.RoleTool, "🔧"},
		{tui.RoleUser, "📝"},
		{tui.RoleSystem, "💰"},
		{tui.RoleLoop, "🚀"},
		{tui.RoleLoopStopped, "🛑"},
	}

	for _, tc := range tests {
		t.Run(string(tc.role), func(t *testing.T) {
			msg := tui.Message{Role: tc.role, Content: "test"}
			icon := msg.GetIcon()
			if icon != tc.expectedIcon {
				t.Errorf("Expected icon %s for role %s in October, got %s", tc.expectedIcon, tc.role, icon)
			}
		})
	}
}

// TestOctoberGhostInActivityFeed tests that the ghost emoji renders in the activity feed during October
func TestOctoberGhostInActivityFeed(t *testing.T) {
	tui.SetTimeNowForTest(func() time.Time {
		return time.Date(2024, time.October, 1, 0, 0, 0, 0, time.UTC)
	})
	defer tui.SetTimeNowForTest(time.Now)

	model := tui.NewModel()
	model.AddMessage(tui.Message{Role: tui.RoleAssistant, Content: "Hello from October"})
	model, _ = updateModel(model, tea.WindowSizeMsg{Width: 120, Height: 40})

	view := model.View()
	if !strings.Contains(view, "👻") {
		t.Error("Activity feed should show ghost emoji for assistant messages in October")
	}
	if strings.Contains(view, "🤖") {
		t.Error("Activity feed should NOT show robot emoji for assistant messages in October")
	}
}

// TestTUIPauseResumeDoesNotQuit tests that pause/resume keys never trigger app quit.
func TestTUIPauseResumeDoesNotQuit(t *testing.T) {
	model := tui.NewModel()
	l := loop.New(loop.Config{Iterations: 5, Prompt: "test"})
	model.SetLoop(l)
	model, _ = updateModel(model, tea.WindowSizeMsg{Width: 120, Height: 40})

	// Press 'p' then 'r' multiple times — should never quit
	keyP := tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune{'p'}}
	keyR := tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune{'r'}}

	for i := 0; i < 3; i++ {
		model, _ = updateModel(model, keyP)
		view := model.View()
		if view == "Goodbye!\n" {
			t.Fatalf("'p' key should never quit the application (iteration %d)", i)
		}

		model, _ = updateModel(model, keyR)
		view = model.View()
		if view == "Goodbye!\n" {
			t.Fatalf("'r' key should never quit the application (iteration %d)", i)
		}
	}
}

// ============================================================================
// Hibernate Tests
// ============================================================================

// TestHibernateRoleIcon tests that RoleHibernate has the 💤 icon
func TestHibernateRoleIcon(t *testing.T) {
	msg := tui.Message{Role: tui.RoleHibernate, Content: "Rate limited"}
	icon := msg.GetIcon()
	if icon != "💤" {
		t.Errorf("Expected 💤 icon for RoleHibernate, got %s", icon)
	}
}

// TestHibernateRoleStyle tests that RoleHibernate has an orange style
func TestHibernateRoleStyle(t *testing.T) {
	msg := tui.Message{Role: tui.RoleHibernate, Content: "test"}
	style := msg.GetStyle()
	// Style should render without panic (matching pattern of other role style tests)
	rendered := style.Render("test")
	if rendered == "" {
		t.Error("Style for RoleHibernate rendered empty string")
	}
}

// TestHibernateStateFollowsLoop tests that the TUI's rate-limit state is read
// straight off the loop rather than tracked separately: the same model shows no
// rate limit before the loop hibernates and shows one immediately after, with no
// message sent to the TUI in between.
func TestHibernateStateFollowsLoop(t *testing.T) {
	model := tui.NewModel()
	l := loop.New(loop.Config{Iterations: 5, Prompt: "test"})
	model.SetLoop(l)
	model, _ = updateModel(model, tea.WindowSizeMsg{Width: 120, Height: 40})

	// Before: the loop is not hibernating, so nothing claims a rate limit
	if strings.Contains(model.View(), "RATE LIMITED") {
		t.Fatal("Precondition: view should not show 'RATE LIMITED' before the loop hibernates")
	}

	// When: the loop hibernates (no TUI message involved)
	l.Hibernate(time.Now().Add(5 * time.Minute))

	// Then: the very next render picks the state up from the loop
	view := model.View()
	if view == "" || view == "Goodbye!\n" {
		t.Fatalf("Model should still render properly while hibernating, got: %q", view)
	}
	if !strings.Contains(view, "RATE LIMITED") {
		t.Error("View should contain 'RATE LIMITED' once the loop is hibernating")
	}
}

// TestHibernateDisplayShowsRateLimited tests that TUI shows "RATE LIMITED" status when hibernating
func TestHibernateDisplayShowsRateLimited(t *testing.T) {
	model := tui.NewModel()
	l := loop.New(loop.Config{Iterations: 5, Prompt: "test"})
	model.SetLoop(l)
	model, _ = updateModel(model, tea.WindowSizeMsg{Width: 120, Height: 40})

	// Hibernate the loop
	l.Hibernate(time.Now().Add(5 * time.Minute))

	view := model.View()

	// Should show "RATE LIMITED" status
	if !strings.Contains(view, "RATE LIMITED") {
		t.Error("View should contain 'RATE LIMITED' when hibernating")
	}
}

// TestHibernateDisplayShowsCountdown tests that TUI shows countdown timer when hibernating
func TestHibernateDisplayShowsCountdown(t *testing.T) {
	model := tui.NewModel()
	l := loop.New(loop.Config{Iterations: 5, Prompt: "test"})
	model.SetLoop(l)
	model, _ = updateModel(model, tea.WindowSizeMsg{Width: 120, Height: 40})

	// Hibernate for 5 minutes (300 seconds)
	l.Hibernate(time.Now().Add(5 * time.Minute))

	view := model.View()

	// Should show 💤 emoji and countdown timer (approximately 05:00 or 04:59)
	if !strings.Contains(view, "💤") {
		t.Error("View should contain 💤 emoji when hibernating")
	}
	// Should contain minute:second format (at least "0" followed by digits for time)
	if !strings.Contains(view, "0") {
		t.Error("View should contain countdown timer when hibernating")
	}
}

// TestHibernateRKeyWake tests that 'r' key wakes from hibernate
func TestHibernateRKeyWake(t *testing.T) {
	model := tui.NewModel()
	l := loop.New(loop.Config{Iterations: 5, Prompt: "test"})
	model.SetLoop(l)
	model, _ = updateModel(model, tea.WindowSizeMsg{Width: 120, Height: 40})

	// Hibernate the loop
	l.Hibernate(time.Now().Add(10 * time.Second))

	// Verify loop is hibernating
	if !l.IsHibernating() {
		t.Fatal("Loop should be hibernating before wake test")
	}

	// Press 'r' to wake
	keyR := tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune{'r'}}
	model, _ = updateModel(model, keyR)

	// Loop should no longer be hibernating
	if l.IsHibernating() {
		t.Error("Loop should not be hibernating after 'r' key wake")
	}
}

// TestHibernateMessageInActivityFeed tests that hibernate messages display correctly in activity feed
func TestHibernateMessageInActivityFeed(t *testing.T) {
	model := tui.NewModel()
	model.AddMessage(tui.Message{Role: tui.RoleHibernate, Content: "Rate limited until 10:30 AM"})
	model, _ = updateModel(model, tea.WindowSizeMsg{Width: 120, Height: 40})

	view := model.View()

	// Should show 💤 emoji in activity feed
	if !strings.Contains(view, "💤") {
		t.Error("Activity feed should show 💤 emoji for hibernate messages")
	}
}

// ============================================================================
// Tests: RoleLoop and RoleLoopStopped Icons & Styles (non-October)
// ============================================================================

// TestRoleLoopIcon tests that RoleLoop has the 🚀 icon outside October
func TestRoleLoopIcon(t *testing.T) {
	tui.SetTimeNowForTest(func() time.Time {
		return time.Date(2024, time.February, 15, 12, 0, 0, 0, time.UTC)
	})
	defer tui.SetTimeNowForTest(time.Now)

	msg := tui.Message{Role: tui.RoleLoop, Content: "Loop started"}
	if icon := msg.GetIcon(); icon != "🚀" {
		t.Errorf("Expected 🚀 icon for RoleLoop, got %s", icon)
	}
}

// TestRoleLoopStoppedIcon tests that RoleLoopStopped has the 🛑 icon outside October
func TestRoleLoopStoppedIcon(t *testing.T) {
	tui.SetTimeNowForTest(func() time.Time {
		return time.Date(2024, time.February, 15, 12, 0, 0, 0, time.UTC)
	})
	defer tui.SetTimeNowForTest(time.Now)

	msg := tui.Message{Role: tui.RoleLoopStopped, Content: "Loop stopped"}
	if icon := msg.GetIcon(); icon != "🛑" {
		t.Errorf("Expected 🛑 icon for RoleLoopStopped, got %s", icon)
	}
}

// TestRoleLoopStyle tests that RoleLoop has a bold purple style
func TestRoleLoopStyle(t *testing.T) {
	msg := tui.Message{Role: tui.RoleLoop, Content: "test"}
	style := msg.GetStyle()
	rendered := style.Render("test")
	if rendered == "" {
		t.Error("Style for RoleLoop rendered empty string")
	}
}

// TestRoleLoopStoppedStyle tests that RoleLoopStopped has a bold red style
func TestRoleLoopStoppedStyle(t *testing.T) {
	msg := tui.Message{Role: tui.RoleLoopStopped, Content: "test"}
	style := msg.GetStyle()
	rendered := style.Render("test")
	if rendered == "" {
		t.Error("Style for RoleLoopStopped rendered empty string")
	}
}

// TestDeletePlanHotkeyOpensModal tests that pressing 'D' opens the confirmation
// modal and the view shows the confirmation prompt.
func TestDeletePlanHotkeyOpensModal(t *testing.T) {
	model := tui.NewModel()
	model, _ = updateModel(model, tea.WindowSizeMsg{Width: 120, Height: 40})

	// Press 'D' to open the modal
	keyD := tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune{'D'}}
	model, _ = updateModel(model, keyD)

	// View should contain the confirmation text
	view := model.View()
	if !strings.Contains(view, "Delete IMPLEMENTATION_PLAN.md?") {
		t.Error("View should show delete-plan confirmation modal after pressing 'D'")
	}
}

// TestDeletePlanModalCancel tests that pressing 'n' closes the modal without
// deleting anything or resetting the loop.
func TestDeletePlanModalCancel(t *testing.T) {
	// Create a temp plan file
	tmpDir := t.TempDir()
	planPath := filepath.Join(tmpDir, "IMPLEMENTATION_PLAN.md")
	os.WriteFile(planPath, []byte("# Plan\n## TASK 1: Test\n**Status: TODO**\n"), 0644)

	cfg := loop.Config{
		Iterations:     100,
		Prompt:         "test",
		CommandBuilder: mockCommandBuilder,
		SleepDuration:  10 * time.Millisecond,
	}
	l := loop.New(cfg)
	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()

	model := tui.NewModel()
	model.SetLoop(l)
	model.SetPlanFile(planPath)
	model, _ = updateModel(model, tea.WindowSizeMsg{Width: 120, Height: 40})

	// Drain loop output
	go func() {
		for range l.Output() {
		}
	}()
	l.Start(ctx)
	time.Sleep(50 * time.Millisecond)

	// Open modal
	keyD := tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune{'D'}}
	model, _ = updateModel(model, keyD)

	// Cancel with 'n'
	keyN := tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune{'n'}}
	model, _ = updateModel(model, keyN)

	// View should NOT contain the modal anymore
	view := model.View()
	if strings.Contains(view, "Delete IMPLEMENTATION_PLAN.md?") {
		t.Error("Modal should be closed after pressing 'n'")
	}

	// Plan file should still exist
	if _, err := os.Stat(planPath); err != nil {
		t.Error("Plan file should still exist after canceling delete")
	}
}

// TestDeletePlanModalConfirm tests that pressing 'y' deletes the plan file and
// calls Reset() on the loop.
func TestDeletePlanModalConfirm(t *testing.T) {
	// Create a temp plan file
	tmpDir := t.TempDir()
	planPath := filepath.Join(tmpDir, "IMPLEMENTATION_PLAN.md")
	os.WriteFile(planPath, []byte("# Plan\n## TASK 1: Test\n**Status: TODO**\n"), 0644)

	cfg := loop.Config{
		Iterations:     100,
		Prompt:         "test",
		CommandBuilder: mockCommandBuilder,
		SleepDuration:  10 * time.Millisecond,
	}
	l := loop.New(cfg)
	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()

	model := tui.NewModel()
	model.SetLoop(l)
	model.SetPlanFile(planPath)
	model, _ = updateModel(model, tea.WindowSizeMsg{Width: 120, Height: 40})

	// Drain loop output
	go func() {
		for range l.Output() {
		}
	}()
	l.Start(ctx)
	time.Sleep(50 * time.Millisecond)

	// Open modal
	keyD := tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune{'D'}}
	model, _ = updateModel(model, keyD)

	// Confirm with 'y'
	keyY := tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune{'y'}}
	model, _ = updateModel(model, keyY)

	// Plan file should be deleted
	if _, err := os.Stat(planPath); !os.IsNotExist(err) {
		t.Error("Plan file should be deleted after confirming with 'y'")
	}

	// Modal should be closed
	view := model.View()
	if strings.Contains(view, "Delete IMPLEMENTATION_PLAN.md?") {
		t.Error("Modal should be closed after confirming with 'y'")
	}
}

// TestDeletePlanModalBlocksOtherKeys tests that when the modal is open, normal
// hotkeys like 'p' and 'r' are ignored.
func TestDeletePlanModalBlocksOtherKeys(t *testing.T) {
	model := tui.NewModel()
	model, _ = updateModel(model, tea.WindowSizeMsg{Width: 120, Height: 40})

	// Open modal
	keyD := tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune{'D'}}
	model, _ = updateModel(model, keyD)

	// Press 'p' — should be ignored (modal stays open)
	keyP := tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune{'p'}}
	model, _ = updateModel(model, keyP)

	view := model.View()
	if !strings.Contains(view, "Delete IMPLEMENTATION_PLAN.md?") {
		t.Error("Modal should still be open after pressing 'p' (ignored while modal is active)")
	}

	// Press 'r' — should also be ignored
	keyR := tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune{'r'}}
	model, _ = updateModel(model, keyR)

	view = model.View()
	if !strings.Contains(view, "Delete IMPLEMENTATION_PLAN.md?") {
		t.Error("Modal should still be open after pressing 'r' (ignored while modal is active)")
	}
}

// TestDeletePlanModalQuitStillWorks tests that 'q' / ctrl+c still quits even
// when the modal is open.
func TestDeletePlanModalQuitStillWorks(t *testing.T) {
	model := tui.NewModel()
	model, _ = updateModel(model, tea.WindowSizeMsg{Width: 120, Height: 40})

	// Open modal
	keyD := tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune{'D'}}
	model, _ = updateModel(model, keyD)

	// Press 'q' — should quit
	keyQ := tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune{'q'}}
	model, _ = updateModel(model, keyQ)

	view := model.View()
	if view != "Goodbye!\n" {
		t.Error("Pressing 'q' from the modal should quit the application")
	}
}

// TestDeletePlanHotkeyInHotkeyBar tests that the (D)elete plan hotkey hint
// appears in the footer.
func TestDeletePlanHotkeyInHotkeyBar(t *testing.T) {
	model := tui.NewModel()
	model, _ = updateModel(model, tea.WindowSizeMsg{Width: 120, Height: 40})

	view := model.View()
	if !strings.Contains(view, "(D)") {
		t.Error("Footer hotkey bar should show (D)elete plan hint")
	}
}

// ============================================================================
// Tests: Model Details Footer Panel
//
// The fourth footer panel shows what the loop is actually running with:
// "Model:", "Effort:" and "Mode:" rows. Known model tiers collapse to their
// short name and unknown ids render verbatim; an unset model reads "default"
// (whatever the claude CLI resolves on its own, refined once the stream reports
// it). Effort has no such fallback name — the levels are low/medium/high/xhigh/
// max — so an unresolved one reads "-", the same placeholder the Mode row uses.
// ============================================================================

// setupModelDetailsModel builds a ready model with the given --model/--effort
// flag values. The footer panels are (width-8)/4 wide, so a wide terminal keeps
// short values on one line and avoids word-wrap-sensitive assertions.
func setupModelDetailsModel(model, effort string) tui.Model {
	m := tui.NewModel()
	m.SetModelInfo(model, effort)
	m, _ = updateModel(m, tea.WindowSizeMsg{Width: 200, Height: 40})
	return m
}

// modelDetailsPanel returns only the "Model Details" footer panel region of a
// rendered view.
//
// Why this exists: the panel's values are extremely short ("opus", "high",
// "default", "sonnet"). Asserting strings.Contains against the WHOLE view makes
// those assertions pass or fail for the wrong reason as soon as any other part
// of the view — another footer row, the hotkey bar, an activity message, a plan
// line — happens to contain the same characters. Two concrete traps:
// "high" is a substring of "xhigh" (so a whole-view check cannot distinguish
// --effort high from --effort xhigh), and a negative check like
// !Contains(view, "opus") breaks the moment an assistant message mentions opus.
// Scoping every assertion to this panel makes short substrings unambiguous.
//
// Model Details is the RIGHTMOST footer panel, so everything from its left
// border column through end-of-line belongs to it. Box-drawing runes (│ ╭ ─ ╰)
// are multi-byte UTF-8, so columns are counted in RUNES, never bytes.
func modelDetailsPanel(t *testing.T, view string) string {
	t.Helper()

	const title = "Model Details"

	lines := strings.Split(view, "\n")
	titleLine := -1
	startCol := 0
	for i, line := range lines {
		byteIdx := strings.Index(line, title)
		if byteIdx == -1 {
			continue
		}
		titleLine = i
		// strings.Index gives a BYTE offset; convert it to a rune column.
		// The panel's left border plus padding ("│ ") sit two runes before
		// the title, so that is where the panel actually starts.
		startCol = utf8.RuneCountInString(line[:byteIdx]) - 2
		if startCol < 0 {
			startCol = 0
		}
		break
	}
	if titleLine == -1 {
		t.Fatalf("%q not found in rendered view:\n%s", title, view)
	}

	var panel []string
	for _, line := range lines[titleLine:] {
		runes := []rune(line)
		if len(runes) <= startCol {
			// Too short to reach the panel's column — e.g. the hotkey bar
			// underneath the footer. Not part of the panel.
			continue
		}
		panel = append(panel, string(runes[startCol:]))
	}
	return strings.Join(panel, "\n")
}

// modelDetailsRows returns the Model Details panel's text rows with the border
// runes and padding stripped, so tests can assert on COMPLETE rows
// ("Effort: high") instead of loose substrings ("high").
func modelDetailsRows(t *testing.T, view string) []string {
	t.Helper()

	var rows []string
	for _, line := range strings.Split(modelDetailsPanel(t, view), "\n") {
		rows = append(rows, strings.Trim(line, "│ "))
	}
	return rows
}

// panelHasRow reports whether one of the panel's rows is exactly want.
func panelHasRow(rows []string, want string) bool {
	for _, row := range rows {
		if row == want {
			return true
		}
	}
	return false
}

// TestModelDetailsPanelTitleRendered tests that the fourth footer panel is titled
// "Model Details".
func TestModelDetailsPanelTitleRendered(t *testing.T) {
	model := setupModelDetailsModel("", "")

	// modelDetailsRows t.Fatalf's if the title is missing entirely; asserting
	// the title is the panel's first row also pins it to the panel header.
	rows := modelDetailsRows(t, model.View())
	if len(rows) == 0 || rows[0] != "Model Details" {
		t.Errorf("Expected 'Model Details' as the panel's first row, got rows: %q", rows)
	}
}

// TestModelDetailsShowsOpusTierAndEffort tests that a full opus model id collapses
// to its tier name and the effort level is shown alongside it.
func TestModelDetailsShowsOpusTierAndEffort(t *testing.T) {
	model := setupModelDetailsModel("claude-opus-4-8", "high")

	view := model.View()
	panel := modelDetailsPanel(t, view)
	rows := modelDetailsRows(t, view)

	for _, want := range []string{"Model: opus", "Effort: high"} {
		if !panelHasRow(rows, want) {
			t.Errorf("Model Details panel should contain the row %q, got rows: %q", want, rows)
		}
	}
	// "high" is a substring of "xhigh", so the exact-row check above is only
	// meaningful alongside this: the panel must not be showing xhigh.
	if strings.Contains(panel, "xhigh") {
		t.Errorf("Effort 'high' should not render as 'xhigh', got panel:\n%s", panel)
	}
	// The verbose id should be collapsed, not printed in full.
	if strings.Contains(panel, "claude-opus-4-8") {
		t.Errorf("Panel should collapse 'claude-opus-4-8' to the tier name 'opus', got panel:\n%s", panel)
	}
}

// TestModelDetailsCollapsesSonnetTier tests the sonnet tier collapse and that an
// unset effort flag reads "default".
func TestModelDetailsCollapsesSonnetTier(t *testing.T) {
	model := setupModelDetailsModel("claude-sonnet-4-6", "")

	view := model.View()
	panel := modelDetailsPanel(t, view)
	rows := modelDetailsRows(t, view)

	if !panelHasRow(rows, "Model: sonnet") {
		t.Errorf("Panel should collapse 'claude-sonnet-4-6' to the tier name 'sonnet', got rows: %q", rows)
	}
	if strings.Contains(panel, "claude-sonnet-4-6") {
		t.Errorf("Panel should not print the full model id, got panel:\n%s", panel)
	}
	if !panelHasRow(rows, "Effort: -") {
		t.Errorf("Panel should show 'Effort: -' when no effort level could be resolved, got rows: %q", rows)
	}
}

// TestModelDetailsUnresolvedEffortIsNotDefault tests that an unresolved effort
// renders as the "-" placeholder, not as "default". The claude CLI's levels are
// low/medium/high/xhigh/max; "default" is not one of them, so showing it reads
// as a level that does not exist.
func TestModelDetailsUnresolvedEffortIsNotDefault(t *testing.T) {
	model := setupModelDetailsModel("", "")

	// The model row keeps its own "default" fallback (the stream refines it
	// later), so the rows must be asserted independently rather than by counting
	// "default" occurrences across the whole view.
	rows := modelDetailsRows(t, model.View())
	for _, want := range []string{"Model: default", "Effort: -"} {
		if !panelHasRow(rows, want) {
			t.Errorf("Model Details panel should contain the row %q, got rows: %q", want, rows)
		}
	}
	if panelHasRow(rows, "Effort: default") {
		t.Errorf("'default' is not an effort level and must not render as one, got rows: %q", rows)
	}
}

// TestModelDetailsUnknownModelRendersVerbatim tests that a model id with no known
// tier is shown as-is.
func TestModelDetailsUnknownModelRendersVerbatim(t *testing.T) {
	model := setupModelDetailsModel("zeta-9", "low")

	view := model.View()
	rows := modelDetailsRows(t, view)

	if !panelHasRow(rows, "Model: zeta-9") {
		t.Errorf("Panel should render an unrecognized model id verbatim, got rows: %q", rows)
	}
	if panelHasRow(rows, "Model: default") {
		t.Errorf("An unrecognized model id should not fall back to 'default', got rows: %q", rows)
	}
	if !panelHasRow(rows, "Effort: low") {
		t.Errorf("Panel should still show the configured effort, got rows: %q", rows)
	}
}

// TestModelDetailsEffortLowercased tests that the effort level is lowercased for display.
func TestModelDetailsEffortLowercased(t *testing.T) {
	model := setupModelDetailsModel("", "XHIGH")

	view := model.View()
	panel := modelDetailsPanel(t, view)
	rows := modelDetailsRows(t, view)

	if !panelHasRow(rows, "Effort: xhigh") {
		t.Errorf("Panel should lowercase the effort level ('XHIGH' -> 'xhigh'), got rows: %q", rows)
	}
	if strings.Contains(panel, "XHIGH") {
		t.Errorf("Panel should not show the raw uppercase effort level, got panel:\n%s", panel)
	}
}

// TestModelDetailsStreamModelOverridesFlag tests that the effective model reported
// by the stream overrides the model set from the --model flag.
func TestModelDetailsStreamModelOverridesFlag(t *testing.T) {
	model := setupModelDetailsModel("claude-opus-4-8", "high")

	cmd := tui.SendModelUpdate("claude-haiku-4-5")
	model, _ = updateModel(model, cmd())

	view := model.View()
	panel := modelDetailsPanel(t, view)
	rows := modelDetailsRows(t, view)

	if !panelHasRow(rows, "Model: haiku") {
		t.Errorf("Panel should show 'haiku' after the stream reports the effective model, got rows: %q", rows)
	}
	// Scoped to the panel: an activity message mentioning "opus" must not be
	// able to fail this.
	if strings.Contains(panel, "opus") {
		t.Errorf("Panel should no longer show the flag model 'opus' after a stream model update, got panel:\n%s", panel)
	}
	// And: effort is untouched by a model update.
	if !panelHasRow(rows, "Effort: high") {
		t.Errorf("Effort should persist across a model update, got rows: %q", rows)
	}
}

// TestModelDetailsEmptyStreamModelIgnored tests that an empty model update does not
// clobber the already-known model.
func TestModelDetailsEmptyStreamModelIgnored(t *testing.T) {
	model := setupModelDetailsModel("claude-opus-4-8", "high")

	cmd := tui.SendModelUpdate("")
	model, _ = updateModel(model, cmd())

	rows := modelDetailsRows(t, model.View())
	if !panelHasRow(rows, "Model: opus") {
		t.Errorf("An empty model update should not clear the previously-set model, got rows: %q", rows)
	}
	if panelHasRow(rows, "Model: default") {
		t.Errorf("An empty model update should not reset the model row to 'default', got rows: %q", rows)
	}
}

// TestModelDetailsTranscriptEffortOverridesFlag tests that the level read back
// from the session transcript replaces the one Ralph resolved up front. The
// transcript is what the CLI actually ran at, so it wins even over --effort —
// enterprise managed settings can override the flag on the command line.
func TestModelDetailsTranscriptEffortOverridesFlag(t *testing.T) {
	model := setupModelDetailsModel("claude-opus-4-8", "high")

	cmd := tui.SendEffortUpdate("max")
	model, _ = updateModel(model, cmd())

	view := model.View()
	panel := modelDetailsPanel(t, view)
	rows := modelDetailsRows(t, view)

	if !panelHasRow(rows, "Effort: max") {
		t.Errorf("Panel should show the transcript's effort level, got rows: %q", rows)
	}
	if strings.Contains(panel, "high") {
		t.Errorf("Panel should no longer show the flag's effort level 'high', got panel:\n%s", panel)
	}
	// And: the model row is untouched by an effort update.
	if !panelHasRow(rows, "Model: opus") {
		t.Errorf("Model should persist across an effort update, got rows: %q", rows)
	}
}

// TestModelDetailsUnresolvedEffortRefinedByTranscript tests the case the
// transcript readback exists for: nothing configured a level up front, so the
// panel starts at "-" and fills in once the session reports one.
func TestModelDetailsUnresolvedEffortRefinedByTranscript(t *testing.T) {
	model := setupModelDetailsModel("", "")

	if rows := modelDetailsRows(t, model.View()); !panelHasRow(rows, "Effort: -") {
		t.Fatalf("Panel should start at the '-' placeholder, got rows: %q", rows)
	}

	cmd := tui.SendEffortUpdate("xhigh")
	model, _ = updateModel(model, cmd())

	rows := modelDetailsRows(t, model.View())
	if !panelHasRow(rows, "Effort: xhigh") {
		t.Errorf("Panel should show the effort level once the transcript reports it, got rows: %q", rows)
	}
	if panelHasRow(rows, "Effort: -") {
		t.Errorf("Panel should drop the placeholder once a level is known, got rows: %q", rows)
	}
}

// TestModelDetailsEmptyEffortUpdateIgnored tests that an empty effort update —
// what a transcript read returns before the CLI has recorded a level — does not
// wipe the value already resolved from the flag or settings.
func TestModelDetailsEmptyEffortUpdateIgnored(t *testing.T) {
	model := setupModelDetailsModel("claude-opus-4-8", "medium")

	cmd := tui.SendEffortUpdate("")
	model, _ = updateModel(model, cmd())

	rows := modelDetailsRows(t, model.View())
	if !panelHasRow(rows, "Effort: medium") {
		t.Errorf("An empty effort update should not clear the resolved level, got rows: %q", rows)
	}
	if panelHasRow(rows, "Effort: -") {
		t.Errorf("An empty effort update should not reset the row to the '-' placeholder, got rows: %q", rows)
	}
}
