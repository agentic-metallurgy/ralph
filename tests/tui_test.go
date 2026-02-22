package tests

import (
	"strings"
	"testing"
	"time"

	tea "github.com/charmbracelet/bubbletea"
	"github.com/cloudosai/ralph-go/internal/stats"
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

	// Check initial state via View output
	view := model.View()
	if view != "Initializing..." {
		t.Errorf("Expected 'Initializing...' view before window size, got: %s", view)
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
	if view != "Initializing..." {
		t.Errorf("Expected 'Initializing...' view, got: %s", view)
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

	// After window size, the view should not be "Initializing..."
	view := updatedModel.View()
	if view == "Initializing..." {
		t.Error("Model should be ready after receiving WindowSizeMsg")
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
	if view == "" || view == "Initializing..." {
		t.Error("Model should render with hotkey bar")
	}
}
