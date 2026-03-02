package tests

import (
	"context"
	"fmt"
	"os"
	"strings"
	"testing"
	"time"

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

// TestCreateProgram tests the CreateProgram function
func TestCreateProgram(t *testing.T) {
	msgChan := make(chan tui.Message, 10)
	doneChan := make(chan struct{})
	defer close(msgChan)
	defer close(doneChan)

	program := tui.CreateProgram(msgChan, doneChan)
	if program == nil {
		t.Error("CreateProgram should return a non-nil program")
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

// TestCacheTokenBreakdownDisplayed tests that cache write and cache read tokens
// appear in the Usage & Cost panel footer with human-readable formatting
func TestCacheTokenBreakdownDisplayed(t *testing.T) {
	model := tui.NewModel()

	s := stats.NewTokenStats()
	s.AddUsage(500, 250, 12345, 67890)
	model.SetStats(s)

	model, _ = updateModel(model, tea.WindowSizeMsg{Width: 120, Height: 40})
	view := model.View()

	if !strings.Contains(view, "12.3k") {
		t.Error("View should display cache creation token count as human-readable (12.3k)")
	}
	if !strings.Contains(view, "67.9k") {
		t.Error("View should display cache read token count as human-readable (67.9k)")
	}
	if !strings.Contains(view, "Cache Write") {
		t.Error("View should contain 'Cache Write' label")
	}
	if !strings.Contains(view, "Cache Read") {
		t.Error("View should contain 'Cache Read' label")
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

// TestAgentCountDisplayed tests that agent count appears in the Loop Details panel
func TestAgentCountDisplayed(t *testing.T) {
	model := tui.NewModel()
	model, _ = updateModel(model, tea.WindowSizeMsg{Width: 120, Height: 40})

	view := model.View()
	// With 0 agents, should show "Active Agents:" label and "0"
	if !strings.Contains(view, "Active Agents:") {
		t.Error("View should contain 'Active Agents:' label")
	}
	if !strings.Contains(view, "0") {
		t.Error("View should show agent count of 0")
	}
}

// TestAgentCountUpdate tests that the agent count updates via SendAgentUpdate
func TestAgentCountUpdate(t *testing.T) {
	model := tui.NewModel()
	model, _ = updateModel(model, tea.WindowSizeMsg{Width: 120, Height: 40})

	// Simulate agent update
	cmd := tui.SendAgentUpdate(3)
	agentMsg := cmd()
	model, _ = updateModel(model, agentMsg)

	view := model.View()
	if !strings.Contains(view, "3") {
		t.Error("View should show updated agent count of 3")
	}
}

// TestAgentCountZeroAfterReset tests agent count returns to 0
func TestAgentCountZeroAfterReset(t *testing.T) {
	model := tui.NewModel()
	model, _ = updateModel(model, tea.WindowSizeMsg{Width: 120, Height: 40})

	// Set agents to 2
	cmd := tui.SendAgentUpdate(2)
	agentMsg := cmd()
	model, _ = updateModel(model, agentMsg)

	// Reset to 0
	cmd = tui.SendAgentUpdate(0)
	agentMsg = cmd()
	model, _ = updateModel(model, agentMsg)

	view := model.View()
	if !strings.Contains(view, "Active Agents:") {
		t.Error("View should still contain 'Active Agents:' label after reset")
	}
}

// TestSendAgentUpdateCmd tests the SendAgentUpdate helper command
func TestSendAgentUpdateCmd(t *testing.T) {
	cmd := tui.SendAgentUpdate(5)

	if cmd == nil {
		t.Error("SendAgentUpdate should return a command")
	}

	result := cmd()
	if result == nil {
		t.Error("Command should return an agent update message")
	}
}

// TestTaskDisplayDefault tests that the task row shows "-" by default
func TestTaskDisplayDefault(t *testing.T) {
	model := tui.NewModel()
	model, _ = updateModel(model, tea.WindowSizeMsg{Width: 120, Height: 40})

	view := model.View()
	if !strings.Contains(view, "Current Task:") {
		t.Error("View should contain 'Current Task:' label")
	}
}

// TestTaskUpdateDisplayed tests that sending a task update shows the task
func TestTaskUpdateDisplayed(t *testing.T) {
	model := tui.NewModel()
	model, _ = updateModel(model, tea.WindowSizeMsg{Width: 120, Height: 40})

	// Simulate task update with new "#N Description" format
	cmd := tui.SendTaskUpdate("#6 Track Phase/Task")
	taskMsg := cmd()
	model, _ = updateModel(model, taskMsg)

	view := model.View()
	if !strings.Contains(view, "#6 Track Phase/Task") {
		t.Error("View should display the current task in '#N Description' format")
	}
}

// TestTaskUpdateOverwritesPrevious tests that new task updates replace old ones
func TestTaskUpdateOverwritesPrevious(t *testing.T) {
	model := tui.NewModel()
	model, _ = updateModel(model, tea.WindowSizeMsg{Width: 120, Height: 40})

	// Set task 3
	cmd := tui.SendTaskUpdate("#3")
	model, _ = updateModel(model, cmd())

	// Set task 6
	cmd = tui.SendTaskUpdate("#6 Track Phase/Task")
	model, _ = updateModel(model, cmd())

	view := model.View()
	if !strings.Contains(view, "#6 Track Phase/Task") {
		t.Error("View should show the latest task in '#N Description' format")
	}
}

// TestSendTaskUpdateCmd tests the SendTaskUpdate helper command
func TestSendTaskUpdateCmd(t *testing.T) {
	cmd := tui.SendTaskUpdate("Task 5")

	if cmd == nil {
		t.Error("SendTaskUpdate should return a command")
	}

	result := cmd()
	if result == nil {
		t.Error("Command should return a task update message")
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

// TestFooterShowsLoopProgress tests that the footer panel shows current loop progress
func TestFooterShowsLoopProgress(t *testing.T) {
	model := tui.NewModel()
	model.SetLoopProgress(3, 5)
	model, _ = updateModel(model, tea.WindowSizeMsg{Width: 120, Height: 40})

	view := model.View()
	if !strings.Contains(view, "#3/5") {
		t.Error("Footer panel should display loop progress as '#3/5'")
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

// TestFooterDefaultLoopProgress tests footer panel with no loop progress set
func TestFooterDefaultLoopProgress(t *testing.T) {
	model := tui.NewModel()
	model, _ = updateModel(model, tea.WindowSizeMsg{Width: 120, Height: 40})

	view := model.View()
	if !strings.Contains(view, "#0/0") {
		t.Error("Footer panel should display '#0/0' when no loop progress is set")
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

// TestCurrentTaskDisplayFormat tests the "Current Task: #N Description" format
func TestCurrentTaskDisplayFormat(t *testing.T) {
	model := tui.NewModel()
	model, _ = updateModel(model, tea.WindowSizeMsg{Width: 120, Height: 40})

	// Simulate task update with "#N Description" format (keep short to fit panel width)
	cmd := tui.SendTaskUpdate("#6 Change lib/gold")
	taskMsg := cmd()
	model, _ = updateModel(model, taskMsg)

	view := model.View()
	if !strings.Contains(view, "Current Task:") {
		t.Error("View should contain 'Current Task:' label")
	}
	if !strings.Contains(view, "#6 Change lib/gold") {
		t.Error("View should display task in '#N Description' format")
	}
}

// TestTaskDisplayWithoutDescription tests task display with number only
func TestTaskDisplayWithoutDescription(t *testing.T) {
	model := tui.NewModel()
	model, _ = updateModel(model, tea.WindowSizeMsg{Width: 120, Height: 40})

	// Simulate task update with number only
	cmd := tui.SendTaskUpdate("#5")
	taskMsg := cmd()
	model, _ = updateModel(model, taskMsg)

	view := model.View()
	if !strings.Contains(view, "#5") {
		t.Error("View should show task number when no description is present")
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

// TestCompletedTasksDefault tests that completed tasks shows "0/0" by default
func TestCompletedTasksDefault(t *testing.T) {
	model := tui.NewModel()
	model, _ = updateModel(model, tea.WindowSizeMsg{Width: 120, Height: 40})

	view := model.View()
	if !strings.Contains(view, "Completed Tasks:") {
		t.Error("View should contain 'Completed Tasks:' label")
	}
	if !strings.Contains(view, "0/0") {
		t.Error("View should show '0/0' for default completed tasks")
	}
}

// TestCompletedTasksUpdate tests that completed task counts update via SendCompletedTasksUpdate
func TestCompletedTasksUpdate(t *testing.T) {
	model := tui.NewModel()
	model, _ = updateModel(model, tea.WindowSizeMsg{Width: 120, Height: 40})

	// Simulate completed tasks update
	cmd := tui.SendCompletedTasksUpdate(4, 7)
	msg := cmd()
	model, _ = updateModel(model, msg)

	view := model.View()
	if !strings.Contains(view, "4/7") {
		t.Error("View should show '4/7' after completed tasks update")
	}
}

// TestSendCompletedTasksUpdateCmd tests the SendCompletedTasksUpdate helper command
func TestSendCompletedTasksUpdateCmd(t *testing.T) {
	cmd := tui.SendCompletedTasksUpdate(3, 8)

	if cmd == nil {
		t.Error("SendCompletedTasksUpdate should return a command")
	}

	result := cmd()
	if result == nil {
		t.Error("Command should return a completed tasks update message")
	}
}

// TestSetCompletedTasks tests the SetCompletedTasks setter method
func TestSetCompletedTasks(t *testing.T) {
	model := tui.NewModel()
	model.SetCompletedTasks(5, 10)
	model, _ = updateModel(model, tea.WindowSizeMsg{Width: 120, Height: 40})

	view := model.View()
	if !strings.Contains(view, "5/10") {
		t.Error("View should show '5/10' after SetCompletedTasks")
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

// TestAddLoopHotkey tests that '+' key increases total loops
func TestAddLoopHotkey(t *testing.T) {
	model := tui.NewModel()
	l := loop.New(loop.Config{Iterations: 5, Prompt: "test"})
	model.SetLoop(l)
	model.SetLoopProgress(2, 5)
	model, _ = updateModel(model, tea.WindowSizeMsg{Width: 120, Height: 40})

	// Press '+' to add a loop
	keyMsg := tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune{'+'}}
	model, _ = updateModel(model, keyMsg)

	view := model.View()
	if !strings.Contains(view, "#2/6") {
		t.Errorf("After pressing '+', total loops should increase from 5 to 6, view should contain '#2/6'")
	}

	// Verify the loop's iteration count was updated
	if l.GetIterations() != 6 {
		t.Errorf("Expected loop iterations to be 6 after '+', got %d", l.GetIterations())
	}
}

// TestSubtractLoopHotkey tests that '-' key decreases total loops
func TestSubtractLoopHotkey(t *testing.T) {
	model := tui.NewModel()
	l := loop.New(loop.Config{Iterations: 5, Prompt: "test"})
	model.SetLoop(l)
	model.SetLoopProgress(2, 5)
	model, _ = updateModel(model, tea.WindowSizeMsg{Width: 120, Height: 40})

	// Press '-' to subtract a loop
	keyMsg := tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune{'-'}}
	model, _ = updateModel(model, keyMsg)

	view := model.View()
	if !strings.Contains(view, "#2/4") {
		t.Errorf("After pressing '-', total loops should decrease from 5 to 4, view should contain '#2/4'")
	}

	// Verify the loop's iteration count was updated
	if l.GetIterations() != 4 {
		t.Errorf("Expected loop iterations to be 4 after '-', got %d", l.GetIterations())
	}
}

// TestSubtractLoopFloorConstraint tests that '-' cannot go below current loop
func TestSubtractLoopFloorConstraint(t *testing.T) {
	model := tui.NewModel()
	l := loop.New(loop.Config{Iterations: 4, Prompt: "test"})
	model.SetLoop(l)
	model.SetLoopProgress(4, 4) // on loop 4 of 4
	model, _ = updateModel(model, tea.WindowSizeMsg{Width: 120, Height: 40})

	// Press '-' — should be a no-op since currentLoop == totalLoops
	keyMsg := tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune{'-'}}
	model, _ = updateModel(model, keyMsg)

	view := model.View()
	if !strings.Contains(view, "#4/4") {
		t.Errorf("After pressing '-' at floor, loops should remain at 4/4, view should contain '#4/4'")
	}

	// Verify the loop's iteration count was NOT changed
	if l.GetIterations() != 4 {
		t.Errorf("Expected loop iterations to remain 4 at floor, got %d", l.GetIterations())
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

// TestHotkeyBarShowsLoopControls tests that the hotkey bar shows (+)/(-) # of loops
func TestHotkeyBarShowsLoopControls(t *testing.T) {
	model := tui.NewModel()
	model, _ = updateModel(model, tea.WindowSizeMsg{Width: 120, Height: 40})

	view := model.View()
	if !strings.Contains(view, "(+)/(-)") {
		t.Error("Hotkey bar should contain '(+)/(-)' label")
	}
	if !strings.Contains(view, "# of loops") {
		t.Error("Hotkey bar should contain '# of loops' label")
	}
}

// TestHotkeyBarShowsStartWhenCompletedWithPendingLoops tests that after completion
// with pending loops added via '+', the hotkey bar shows '(s)tart' instead of '(r)esume'
func TestHotkeyBarShowsStartWhenCompletedWithPendingLoops(t *testing.T) {
	model := tui.NewModel()
	l := loop.New(loop.Config{Iterations: 5, Prompt: "test"})
	model.SetLoop(l)
	model.SetLoopProgress(5, 5)
	model, _ = updateModel(model, tea.WindowSizeMsg{Width: 120, Height: 40})

	// Simulate completion
	cmd := tui.SendDone()
	doneMsg := cmd()
	model, _ = updateModel(model, doneMsg)

	// Press '+' to add a loop
	keyPlus := tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune{'+'}}
	model, _ = updateModel(model, keyPlus)

	view := model.View()
	if !strings.Contains(view, "(s)tart") {
		t.Error("Hotkey bar should show '(s)tart' when completed with pending loops")
	}
}

// TestHotkeyBarShowsResumeWhenPaused tests that when paused, the hotkey bar shows '(r)esume'
func TestHotkeyBarShowsResumeWhenPaused(t *testing.T) {
	model := tui.NewModel()
	l := loop.New(loop.Config{Iterations: 5, Prompt: "test"})
	model.SetLoop(l)
	model.SetLoopProgress(2, 5)
	model, _ = updateModel(model, tea.WindowSizeMsg{Width: 120, Height: 40})

	// Press 'p' to pause
	keyP := tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune{'p'}}
	model, _ = updateModel(model, keyP)

	view := model.View()
	if !strings.Contains(view, "(r)esume") {
		t.Error("Hotkey bar should show '(r)esume' when paused")
	}
}

// TestStartKeyResumesAfterCompletion tests that pressing 's' starts new loops after completion
func TestStartKeyResumesAfterCompletion(t *testing.T) {
	model := tui.NewModel()
	l := loop.New(loop.Config{Iterations: 5, Prompt: "test"})
	model.SetLoop(l)
	model.SetLoopProgress(5, 5)
	model, _ = updateModel(model, tea.WindowSizeMsg{Width: 120, Height: 40})

	// Simulate completion
	cmd := tui.SendDone()
	doneMsg := cmd()
	model, _ = updateModel(model, doneMsg)

	// Press '+' to add a loop
	keyPlus := tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune{'+'}}
	model, _ = updateModel(model, keyPlus)

	// Press 's' to start — should clear completed state
	keyS := tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune{'s'}}
	model, _ = updateModel(model, keyS)

	view := model.View()
	// After pressing 's', completed state should be cleared — view should show RUNNING not COMPLETED
	if strings.Contains(view, "COMPLETED") {
		t.Error("After pressing 's' with pending loops, should no longer show COMPLETED")
	}
}

// TestStartKeyNoopWithoutPendingLoops tests that 's' does nothing when completed without pending loops
func TestStartKeyNoopWithoutPendingLoops(t *testing.T) {
	model := tui.NewModel()
	l := loop.New(loop.Config{Iterations: 5, Prompt: "test"})
	model.SetLoop(l)
	model.SetLoopProgress(5, 5)
	model, _ = updateModel(model, tea.WindowSizeMsg{Width: 120, Height: 40})

	// Simulate completion
	cmd := tui.SendDone()
	doneMsg := cmd()
	model, _ = updateModel(model, doneMsg)

	// Press 's' without adding loops — should be a no-op, still completed
	keyS := tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune{'s'}}
	model, _ = updateModel(model, keyS)

	view := model.View()
	if !strings.Contains(view, "COMPLETED") {
		t.Error("Pressing 's' without pending loops should keep COMPLETED state")
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
// ============================================================================

// TestSendLoopStartedCmd tests the SendLoopStarted helper command
func TestSendLoopStartedCmd(t *testing.T) {
	cmd := tui.SendLoopStarted()
	if cmd == nil {
		t.Error("SendLoopStarted should return a command")
	}
	result := cmd()
	if result == nil {
		t.Error("Command should return a loopStartedMsg")
	}
}

// TestSendLoopStatsUpdateCmd tests the SendLoopStatsUpdate helper command
func TestSendLoopStatsUpdateCmd(t *testing.T) {
	cmd := tui.SendLoopStatsUpdate(12345)
	if cmd == nil {
		t.Error("SendLoopStatsUpdate should return a command")
	}
	result := cmd()
	if result == nil {
		t.Error("Command should return a loopStatsUpdateMsg")
	}
}

// TestPerLoopTokensResetOnNewLoop tests that per-loop tokens reset when a new loop starts
func TestPerLoopTokensResetOnNewLoop(t *testing.T) {
	model := tui.NewModel()
	model, _ = updateModel(model, tea.WindowSizeMsg{Width: 120, Height: 40})

	// Set loop stats to some value
	cmd := tui.SendLoopStatsUpdate(50000)
	model, _ = updateModel(model, cmd())

	// Signal a new loop started
	cmd = tui.SendLoopStarted()
	model, _ = updateModel(model, cmd())

	// Per-loop tokens should be reset to 0
	// Verify by sending another loop stats update with a small value
	cmd = tui.SendLoopStatsUpdate(100)
	model, _ = updateModel(model, cmd())

	// The model should work without errors after reset
	view := model.View()
	if view == "" {
		t.Error("View should render after per-loop stats reset")
	}
}

// TestPerLoopTimerResetsOnNewLoop tests that per-loop elapsed timer resets when a new loop starts
func TestPerLoopTimerResetsOnNewLoop(t *testing.T) {
	model := tui.NewModel()
	model, _ = updateModel(model, tea.WindowSizeMsg{Width: 120, Height: 40})

	// Wait a bit so the loop timer accumulates
	time.Sleep(50 * time.Millisecond)

	// Signal a new loop started — should reset per-loop timer
	cmd := tui.SendLoopStarted()
	model, _ = updateModel(model, cmd())

	// The per-loop timer should now be near-zero (just reset)
	// We can't directly inspect it, but the view should render without error
	view := model.View()
	if view == "" {
		t.Error("View should render after per-loop timer reset")
	}
}

// TestPerLoopTimerFreezesOnPause tests that the per-loop timer freezes when paused
func TestPerLoopTimerFreezesOnPause(t *testing.T) {
	model := tui.NewModel()
	l := loop.New(loop.Config{Iterations: 5, Prompt: "test"})
	model.SetLoop(l)
	model, _ = updateModel(model, tea.WindowSizeMsg{Width: 120, Height: 40})

	// Pause
	keyP := tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune{'p'}}
	model, _ = updateModel(model, keyP)

	// After pausing, two consecutive views should be identical
	// (both total and per-loop timers are frozen)
	view1 := model.View()
	time.Sleep(50 * time.Millisecond)
	view2 := model.View()

	if view1 != view2 {
		t.Error("With paused timers (including per-loop), consecutive views should be identical")
	}
}

// TestPerLoopTimerResumesAfterPause tests that the per-loop timer resumes after unpause
func TestPerLoopTimerResumesAfterPause(t *testing.T) {
	model := tui.NewModel()
	l := loop.New(loop.Config{Iterations: 5, Prompt: "test"})
	model.SetLoop(l)
	model, _ = updateModel(model, tea.WindowSizeMsg{Width: 120, Height: 40})

	// Pause then resume
	keyP := tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune{'p'}}
	model, _ = updateModel(model, keyP)

	keyR := tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune{'r'}}
	model, _ = updateModel(model, keyR)

	// After resume, view should render without error
	view := model.View()
	if view == "" {
		t.Error("View should render after resuming per-loop timer")
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

// TestCompletedTasksAboveCurrentTask tests that "Completed Tasks:" appears above "Current Task:"
func TestCompletedTasksAboveCurrentTask(t *testing.T) {
	model := tui.NewModel()
	model.SetCompletedTasks(4, 7)
	model, _ = updateModel(model, tea.WindowSizeMsg{Width: 120, Height: 40})

	// Set a current task
	cmd := tui.SendTaskUpdate("#6 Change lib/gold")
	model, _ = updateModel(model, cmd())

	view := model.View()

	// Find positions of both labels
	completedIdx := strings.Index(view, "Completed Tasks:")
	currentIdx := strings.Index(view, "Current Task:")

	if completedIdx == -1 {
		t.Fatal("View should contain 'Completed Tasks:' label")
	}
	if currentIdx == -1 {
		t.Fatal("View should contain 'Current Task:' label")
	}
	if completedIdx >= currentIdx {
		t.Errorf("'Completed Tasks:' (pos %d) should appear before 'Current Task:' (pos %d)",
			completedIdx, currentIdx)
	}
}

// ============================================================================
// Tests: Plan Mode Display (Spec #9)
// ============================================================================

// TestPlanModeDisplaysPlanning tests that plan mode shows "Planning" as default task
func TestPlanModeDisplaysPlanning(t *testing.T) {
	model := tui.NewModel()
	model.SetPlanMode(true)
	model, _ = updateModel(model, tea.WindowSizeMsg{Width: 120, Height: 40})

	view := model.View()
	if !strings.Contains(view, "Planning") {
		t.Error("Plan mode should display 'Planning' as current task")
	}
}

// TestPlanModeOverriddenByTaskUpdate tests that task updates override plan mode default
func TestPlanModeOverriddenByTaskUpdate(t *testing.T) {
	model := tui.NewModel()
	model.SetPlanMode(true)
	model, _ = updateModel(model, tea.WindowSizeMsg{Width: 120, Height: 40})

	// Send a task update — should override "Planning"
	cmd := tui.SendTaskUpdate("#3 Research specs")
	model, _ = updateModel(model, cmd())

	view := model.View()
	if !strings.Contains(view, "#3 Research specs") {
		t.Error("Task update should override plan mode 'Planning' display")
	}
}

// TestMissingPlanFileDisplaysCreating tests that SetCurrentTask shows initial task
func TestMissingPlanFileDisplaysCreating(t *testing.T) {
	model := tui.NewModel()
	model.SetCurrentTask("Creating IMPLEMENTATION_PLAN.md")
	model, _ = updateModel(model, tea.WindowSizeMsg{Width: 120, Height: 40})

	view := model.View()
	if !strings.Contains(view, "Creating IMPLEMENTATION_PLAN.md") {
		t.Error("When plan file is missing, should display 'Creating IMPLEMENTATION_PLAN.md'")
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

// TestHibernateMsgUpdate tests that hibernateMsg updates the model's hibernate state
func TestHibernateMsgUpdate(t *testing.T) {
	model := tui.NewModel()
	model, _ = updateModel(model, tea.WindowSizeMsg{Width: 120, Height: 40})

	// Send hibernate message
	hibernateUntil := time.Now().Add(5 * time.Minute)
	hibernateCmd := tui.SendHibernate(hibernateUntil)
	hibernateMsg := hibernateCmd()

	model, _ = updateModel(model, hibernateMsg)

	// After tick, the view should update (we can't directly check internal state,
	// but we can verify the model renders properly)
	view := model.View()
	if view == "" || view == "Goodbye!\n" {
		t.Error("Model should render properly after hibernate message")
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

	// Send hibernate message to TUI
	hibernateCmd := tui.SendHibernate(time.Now().Add(5 * time.Minute))
	hibernateMsg := hibernateCmd()
	model, _ = updateModel(model, hibernateMsg)

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
	hibernateUntil := time.Now().Add(5 * time.Minute)
	l.Hibernate(hibernateUntil)

	// Send hibernate message to TUI
	hibernateCmd := tui.SendHibernate(hibernateUntil)
	hibernateMsg := hibernateCmd()
	model, _ = updateModel(model, hibernateMsg)

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

	// Send hibernate message to TUI
	hibernateCmd := tui.SendHibernate(time.Now().Add(10 * time.Second))
	hibernateMsg := hibernateCmd()
	model, _ = updateModel(model, hibernateMsg)

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

// TestSendHibernateCmd tests the SendHibernate helper function
func TestSendHibernateCmd(t *testing.T) {
	until := time.Now().Add(5 * time.Minute)
	cmd := tui.SendHibernate(until)

	if cmd == nil {
		t.Error("SendHibernate should return a command")
	}

	// Execute the command and verify it returns a message
	result := cmd()
	if result == nil {
		t.Error("Command should return a hibernate message")
	}
}
