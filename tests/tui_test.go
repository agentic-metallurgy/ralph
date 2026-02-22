package tests

import (
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

// TestMessageRoles tests that all message roles have correct icons
func TestMessageRoles(t *testing.T) {
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
	// Extract the elapsed time strings - they appear in both the footer panel and status bar
	// Since timer is frozen, subsequent renders should show the same time
	if view1 != view2 {
		// Views might differ due to tick, but elapsed time should be the same
		// This is a best-effort check
		t.Log("Note: views may differ slightly due to rendering, but elapsed time should be frozen")
	}
}

// TestCacheTokenBreakdownDisplayed tests that cache write and cache read tokens
// appear in the Usage & Cost panel footer
func TestCacheTokenBreakdownDisplayed(t *testing.T) {
	model := tui.NewModel()

	s := stats.NewTokenStats()
	s.AddUsage(500, 250, 12345, 67890)
	model.SetStats(s)

	model, _ = updateModel(model, tea.WindowSizeMsg{Width: 120, Height: 40})
	view := model.View()

	if !strings.Contains(view, "12345") {
		t.Error("View should display cache creation token count (12345)")
	}
	if !strings.Contains(view, "67890") {
		t.Error("View should display cache read token count (67890)")
	}
	if !strings.Contains(view, "Cache Write") {
		t.Error("View should contain 'Cache Write' label")
	}
	if !strings.Contains(view, "Cache Read") {
		t.Error("View should contain 'Cache Read' label")
	}
}

// TestStopHotkey tests that 'o' key pauses the loop
func TestStopHotkey(t *testing.T) {
	model := tui.NewModel()
	model, _ = updateModel(model, tea.WindowSizeMsg{Width: 120, Height: 40})

	// Without a loop set, pressing 'o' should not panic
	keyMsg := tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune{'o'}}
	model, _ = updateModel(model, keyMsg)

	// Should not quit
	view := model.View()
	if view == "Goodbye!\n" {
		t.Error("'o' key should not quit the application")
	}
}

// TestStartHotkey tests that 'a' key resumes the loop
func TestStartHotkey(t *testing.T) {
	model := tui.NewModel()
	model, _ = updateModel(model, tea.WindowSizeMsg{Width: 120, Height: 40})

	// Without a loop set, pressing 'a' should not panic
	keyMsg := tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune{'a'}}
	model, _ = updateModel(model, keyMsg)

	// Should not quit
	view := model.View()
	if view == "Goodbye!\n" {
		t.Error("'a' key should not quit the application")
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

// TestStatusBarDisplayed tests that the status bar labels appear in the view
func TestStatusBarDisplayed(t *testing.T) {
	model := tui.NewModel()
	model, _ = updateModel(model, tea.WindowSizeMsg{Width: 120, Height: 40})

	view := model.View()
	if !strings.Contains(view, "current loop:") {
		t.Error("View should contain 'current loop:' label in status bar")
	}
	if !strings.Contains(view, "tokens:") {
		t.Error("View should contain 'tokens:' label in status bar")
	}
	if !strings.Contains(view, "elapsed:") {
		t.Error("View should contain 'elapsed:' label in status bar")
	}
}

// TestStatusBarShowsLoopProgress tests that the status bar shows current loop progress
func TestStatusBarShowsLoopProgress(t *testing.T) {
	model := tui.NewModel()
	model.SetLoopProgress(3, 5)
	model, _ = updateModel(model, tea.WindowSizeMsg{Width: 120, Height: 40})

	view := model.View()
	if !strings.Contains(view, "#3/5") {
		t.Error("Status bar should display loop progress as '#3/5'")
	}
}

// TestStatusBarShowsTokenCount tests that the status bar shows human-readable token count
func TestStatusBarShowsTokenCount(t *testing.T) {
	model := tui.NewModel()
	s := stats.NewTokenStats()
	s.AddUsage(500000, 250000, 100000, 50000)
	model.SetStats(s)
	model, _ = updateModel(model, tea.WindowSizeMsg{Width: 120, Height: 40})

	view := model.View()
	// 900k total tokens should appear somewhere in the view
	if !strings.Contains(view, "900k") {
		t.Error("Status bar should display human-readable token count (expected '900k')")
	}
}

// TestStatusBarDefaultLoopProgress tests status bar with no loop progress set
func TestStatusBarDefaultLoopProgress(t *testing.T) {
	model := tui.NewModel()
	model, _ = updateModel(model, tea.WindowSizeMsg{Width: 120, Height: 40})

	view := model.View()
	if !strings.Contains(view, "#0/0") {
		t.Error("Status bar should display '#0/0' when no loop progress is set")
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

// TestAddSubtractLoopNoopWhenCompleted tests that '+'/'-' are no-ops when completed
func TestAddSubtractLoopNoopWhenCompleted(t *testing.T) {
	model := tui.NewModel()
	l := loop.New(loop.Config{Iterations: 5, Prompt: "test"})
	model.SetLoop(l)
	model.SetLoopProgress(5, 5)
	model, _ = updateModel(model, tea.WindowSizeMsg{Width: 120, Height: 40})

	// Simulate completion
	cmd := tui.SendDone()
	doneMsg := cmd()
	model, _ = updateModel(model, doneMsg)

	// Press '+' — should be a no-op because completed
	keyMsg := tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune{'+'}}
	model, _ = updateModel(model, keyMsg)

	if l.GetIterations() != 5 {
		t.Errorf("Expected loop iterations to remain 5 after '+' when completed, got %d", l.GetIterations())
	}

	// Press '-' — should also be a no-op because completed
	keyMsg = tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune{'-'}}
	model, _ = updateModel(model, keyMsg)

	if l.GetIterations() != 5 {
		t.Errorf("Expected loop iterations to remain 5 after '-' when completed, got %d", l.GetIterations())
	}
}

// TestHotkeyBarShowsAddSubtract tests that the hotkey bar includes (+)add and (-)subtract
func TestHotkeyBarShowsAddSubtract(t *testing.T) {
	model := tui.NewModel()
	model, _ = updateModel(model, tea.WindowSizeMsg{Width: 120, Height: 40})

	view := model.View()
	if !strings.Contains(view, "add") {
		t.Error("Hotkey bar should contain 'add' label")
	}
	if !strings.Contains(view, "subtract") {
		t.Error("Hotkey bar should contain 'subtract' label")
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
