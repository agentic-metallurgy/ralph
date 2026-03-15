package tests

// ============================================================================
// BDD Test Suite: Thinking Message Display (Task 3)
//
// User goal: when Claude outputs thinking content (either via extended thinking
// content items or <thinking> tags), the TUI displays it with the thinking
// icon (💭) and italic styling in the activity feed.
//
// These tests verify that RoleThinking messages render correctly in the TUI,
// covering both the existing <thinking> tag path and the new extended thinking
// content item path exercised through the parser.
// ============================================================================

import (
	"testing"

	tea "github.com/charmbracelet/bubbletea"
	"github.com/cloudosai/ralph-go/internal/tui"
)

// TestBDD_ThinkingDisplay_ThinkingMessageShowsIcon
//
// Given: a TUI model with a window size set
// When: a thinking-role message is delivered
// Then: the thinking icon (💭) appears in the activity feed
func TestBDD_ThinkingDisplay_ThinkingMessageShowsIcon(t *testing.T) {
	m := tui.NewModel()
	m, _ = updateModel(m, tea.WindowSizeMsg{Width: 120, Height: 40})

	// When: a thinking message is delivered
	m, _ = sendTuiMsg(m, tui.SendMessage(tui.Message{
		Role:    tui.RoleThinking,
		Content: "Analyzing the problem carefully...",
	}))

	// Then: the thinking icon appears
	if !viewContains(m, "💭") {
		t.Errorf("Thinking icon (💭) should appear in view, got:\n%s", m.View())
	}
}

// TestBDD_ThinkingDisplay_ThinkingContentVisibleInFeed
//
// Given: a TUI model with a window size set
// When: a thinking-role message is delivered with specific content
// Then: the thinking content text appears in the activity feed
func TestBDD_ThinkingDisplay_ThinkingContentVisibleInFeed(t *testing.T) {
	m := tui.NewModel()
	m, _ = updateModel(m, tea.WindowSizeMsg{Width: 120, Height: 40})

	// When: a thinking message is delivered
	m, _ = sendTuiMsg(m, tui.SendMessage(tui.Message{
		Role:    tui.RoleThinking,
		Content: "THINKING_CONTENT_VISIBLE_TEST",
	}))

	// Then: the content appears in the activity feed
	if !viewContains(m, "THINKING_CONTENT_VISIBLE_TEST") {
		t.Errorf("Thinking content should appear in view, got:\n%s", m.View())
	}
}

// TestBDD_ThinkingDisplay_ThinkingAndAssistantMessagesCoexist
//
// Given: a TUI model with a window size set
// When: a thinking message followed by an assistant message are delivered
// Then: both messages appear in the activity feed with their respective icons
func TestBDD_ThinkingDisplay_ThinkingAndAssistantMessagesCoexist(t *testing.T) {
	m := tui.NewModel()
	m, _ = updateModel(m, tea.WindowSizeMsg{Width: 120, Height: 40})

	// When: a thinking message is delivered
	m, _ = sendTuiMsg(m, tui.SendMessage(tui.Message{
		Role:    tui.RoleThinking,
		Content: "THINKING_BEFORE_ANSWER",
	}))
	// And: an assistant message follows
	m, _ = sendTuiMsg(m, tui.SendMessage(tui.Message{
		Role:    tui.RoleAssistant,
		Content: "ASSISTANT_ANSWER_AFTER",
	}))

	// Then: both appear in the activity feed
	if !viewContains(m, "💭") {
		t.Errorf("Thinking icon should appear in view, got:\n%s", m.View())
	}
	if !viewContains(m, "THINKING_BEFORE_ANSWER") {
		t.Errorf("Thinking content should appear in view, got:\n%s", m.View())
	}
	if !viewContains(m, "ASSISTANT_ANSWER_AFTER") {
		t.Errorf("Assistant content should appear in view, got:\n%s", m.View())
	}
}

// TestBDD_ThinkingDisplay_ThinkingViaChannelDelivery
//
// Given: a model created with NewModelWithChannels and a thinking message
//        pre-loaded into the channel
// When: the channel listener delivers the message
// Then: the thinking icon and content appear in the activity feed
func TestBDD_ThinkingDisplay_ThinkingViaChannelDelivery(t *testing.T) {
	msgChan := make(chan tui.Message, 1)
	doneChan := make(chan struct{})
	defer close(doneChan)

	m := tui.NewModelWithChannels(msgChan, doneChan)
	m, _ = updateModel(m, tea.WindowSizeMsg{Width: 120, Height: 40})

	// Given: a thinking message waiting in the channel
	msgChan <- tui.Message{Role: tui.RoleThinking, Content: "CHANNEL_THINKING_TEST"}

	// When: the channel listener delivers the message
	m, _ = sendTuiMsg(m, tui.WaitForMessageForTest(msgChan))

	// Then: the thinking content appears with the correct icon
	if !viewContains(m, "💭") {
		t.Errorf("Thinking icon should appear after channel delivery, got:\n%s", m.View())
	}
	if !viewContains(m, "CHANNEL_THINKING_TEST") {
		t.Errorf("Thinking content should appear after channel delivery, got:\n%s", m.View())
	}
}
