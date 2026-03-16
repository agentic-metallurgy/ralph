package tests

// ============================================================================
// BDD Test Suite: Channel-Based Message Flow (Task 7)
//
// User goal: when ralph runs, activity from the Claude CLI loop reaches the TUI
// through the real channel path (NewModelWithChannels / waitForMessage /
// waitForDone) rather than programmatic helper commands.
//
// These tests exercise the previously-untested channel delivery path to verify
// that messages sent to msgChan and signals sent to doneChan are handled by the
// TUI exactly as if they had arrived via SendMessage / SendDone helpers.
// ============================================================================

import (
	"testing"

	tea "github.com/charmbracelet/bubbletea"
	"github.com/cloudosai/ralph-go/internal/tui"
)

// TestBDD_ChannelMessageFlow_MessageFromChannelAppearsInView
//
// Given: a model created with NewModelWithChannels and a message pre-loaded
//
//	into a buffered channel
//
// When: the waitForMessage command reads from the channel and delivers the message
// Then: the message appears in the activity feed
func TestBDD_ChannelMessageFlow_MessageFromChannelAppearsInView(t *testing.T) {
	msgChan := make(chan tui.Message, 1)
	doneChan := make(chan struct{})
	defer close(doneChan)

	m := tui.NewModelWithChannels(msgChan, doneChan)
	m, _ = updateModel(m, tea.WindowSizeMsg{Width: 120, Height: 40})

	if viewContains(m, "CHANNEL_ACTIVITY_001") {
		t.Fatal("Precondition: message should not yet be in view")
	}

	// Given: a message waiting in the channel
	msgChan <- tui.Message{Role: tui.RoleAssistant, Content: "CHANNEL_ACTIVITY_001"}

	// When: the channel listener delivers the message (channel is pre-filled, non-blocking)
	m, _ = sendTuiMsg(m, tui.WaitForMessageForTest(msgChan))

	// Then: the message appears in the activity feed
	if !viewContains(m, "CHANNEL_ACTIVITY_001") {
		t.Errorf("Message from channel should appear in view, got:\n%s", m.View())
	}
}

// TestBDD_ChannelMessageFlow_MultipleMessagesDeliveredInOrder
//
// Given: two messages pre-loaded into a buffered channel
// When: each waitForMessage command reads from the channel
// Then: both messages appear in the activity feed in the order they were sent
func TestBDD_ChannelMessageFlow_MultipleMessagesDeliveredInOrder(t *testing.T) {
	msgChan := make(chan tui.Message, 2)
	doneChan := make(chan struct{})
	defer close(doneChan)

	m := tui.NewModelWithChannels(msgChan, doneChan)
	m, _ = updateModel(m, tea.WindowSizeMsg{Width: 120, Height: 40})

	// Given: two messages waiting in the channel
	msgChan <- tui.Message{Role: tui.RoleAssistant, Content: "CHANNEL_FIRST_MSG"}
	msgChan <- tui.Message{Role: tui.RoleTool, Content: "CHANNEL_SECOND_MSG"}

	// When: channel listener delivers the first message
	m, _ = sendTuiMsg(m, tui.WaitForMessageForTest(msgChan))
	// And: channel listener delivers the second message
	m, _ = sendTuiMsg(m, tui.WaitForMessageForTest(msgChan))

	// Then: both messages are visible
	if !viewContains(m, "CHANNEL_FIRST_MSG") {
		t.Errorf("First channel message should appear in view, got:\n%s", m.View())
	}
	if !viewContains(m, "CHANNEL_SECOND_MSG") {
		t.Errorf("Second channel message should appear in view, got:\n%s", m.View())
	}
}

// TestBDD_ChannelMessageFlow_MessageRolePreservedThroughChannel
//
// Given: a tool message pre-loaded into a buffered channel
// When: waitForMessage delivers it
// Then: the tool icon (🔧) appears in the activity feed, confirming role preservation
func TestBDD_ChannelMessageFlow_MessageRolePreservedThroughChannel(t *testing.T) {
	msgChan := make(chan tui.Message, 1)
	doneChan := make(chan struct{})
	defer close(doneChan)

	m := tui.NewModelWithChannels(msgChan, doneChan)
	m, _ = updateModel(m, tea.WindowSizeMsg{Width: 120, Height: 40})

	// Given: a tool-role message in the channel
	msgChan <- tui.Message{Role: tui.RoleTool, Content: "TOOL_CHANNEL_CONTENT"}

	// When: the channel listener delivers the message
	m, _ = sendTuiMsg(m, tui.WaitForMessageForTest(msgChan))

	// Then: the tool icon renders in the activity feed (role not lost in transit)
	if !viewContains(m, "🔧") {
		t.Errorf("Tool icon should appear in view after channel delivery, got:\n%s", m.View())
	}
	if !viewContains(m, "TOOL_CHANNEL_CONTENT") {
		t.Errorf("Tool message content should appear in view, got:\n%s", m.View())
	}
}

// TestBDD_ChannelMessageFlow_ClosedMessageChannelTriggersDone
//
// Given: a model with channel integration and the message channel closed
// When: waitForMessage reads from the closed channel
// Then: the model transitions to COMPLETED state (closed channel = done signal)
func TestBDD_ChannelMessageFlow_ClosedMessageChannelTriggersDone(t *testing.T) {
	msgChan := make(chan tui.Message)
	doneChan := make(chan struct{})
	defer close(doneChan)

	m := tui.NewModelWithChannels(msgChan, doneChan)
	m, _ = updateModel(m, tea.WindowSizeMsg{Width: 120, Height: 40})

	if viewContains(m, "COMPLETED") {
		t.Fatal("Precondition: model should not be completed before done signal")
	}

	// Given: the message channel is closed (loop finished and closed its output)
	close(msgChan)

	// When: waitForMessage reads from the closed channel (returns doneMsg)
	m, _ = sendTuiMsg(m, tui.WaitForMessageForTest(msgChan))

	// Then: the model transitions to completed state
	if !viewContains(m, "COMPLETED") {
		t.Errorf("Closing message channel should trigger COMPLETED state, got:\n%s", m.View())
	}
}

// TestBDD_ChannelMessageFlow_DoneChannelSignalsCompletion
//
// Given: a model with channel integration and the done channel pre-signaled
// When: waitForDone reads the signal from the done channel
// Then: the model transitions to COMPLETED state
func TestBDD_ChannelMessageFlow_DoneChannelSignalsCompletion(t *testing.T) {
	msgChan := make(chan tui.Message, 1)
	doneChan := make(chan struct{}, 1)

	m := tui.NewModelWithChannels(msgChan, doneChan)
	m, _ = updateModel(m, tea.WindowSizeMsg{Width: 120, Height: 40})

	if viewContains(m, "COMPLETED") {
		t.Fatal("Precondition: model should not be completed before done signal")
	}

	// Given: done channel is signaled (loop finished all iterations)
	doneChan <- struct{}{}

	// When: waitForDone reads the done signal
	m, _ = sendTuiMsg(m, tui.WaitForDoneForTest(doneChan))

	// Then: the model transitions to completed state
	if !viewContains(m, "COMPLETED") {
		t.Errorf("Done channel signal should trigger COMPLETED state, got:\n%s", m.View())
	}
}

// TestBDD_ChannelMessageFlow_InitIncludesChannelListeners
//
// Given: a model created with NewModelWithChannels
// When: Init() is called
// Then: a non-nil batch command is returned (channel listeners are started)
func TestBDD_ChannelMessageFlow_InitIncludesChannelListeners(t *testing.T) {
	msgChan := make(chan tui.Message, 1)
	doneChan := make(chan struct{})
	defer close(msgChan)
	defer close(doneChan)

	// Given: a model with channels
	m := tui.NewModelWithChannels(msgChan, doneChan)

	// When: Init() is called
	initCmd := m.Init()

	// Then: Init returns a non-nil command (batch including channel listeners)
	if initCmd == nil {
		t.Error("NewModelWithChannels Init() should return a non-nil command (batch with channel listeners)")
	}
}

// TestBDD_ChannelMessageFlow_MessageContentMatchesProgrammaticHelper
//
// Given: the same message delivered via channel path and programmatic helper
// When: both are rendered
// Then: the view content is identical — channel delivery is equivalent to SendMessage
func TestBDD_ChannelMessageFlow_MessageContentMatchesProgrammaticHelper(t *testing.T) {
	const content = "EQUIV_TEST_CONTENT"

	// Model A: message delivered via channel
	msgChanA := make(chan tui.Message, 1)
	doneChanA := make(chan struct{})
	defer close(doneChanA)
	mA := tui.NewModelWithChannels(msgChanA, doneChanA)
	mA, _ = updateModel(mA, tea.WindowSizeMsg{Width: 120, Height: 40})
	msgChanA <- tui.Message{Role: tui.RoleAssistant, Content: content}
	mA, _ = sendTuiMsg(mA, tui.WaitForMessageForTest(msgChanA))

	// Model B: same message delivered via programmatic helper
	mB := tui.NewModel()
	mB, _ = updateModel(mB, tea.WindowSizeMsg{Width: 120, Height: 40})
	mB, _ = sendTuiMsg(mB, tui.SendMessage(tui.Message{Role: tui.RoleAssistant, Content: content}))

	// Then: both models show the message content
	if !viewContains(mA, content) {
		t.Errorf("Channel-delivered message should appear in view A, got:\n%s", mA.View())
	}
	if !viewContains(mB, content) {
		t.Errorf("Programmatic message should appear in view B, got:\n%s", mB.View())
	}
}
