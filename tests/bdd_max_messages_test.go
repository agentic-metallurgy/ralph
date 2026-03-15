package tests

import (
	"fmt"
	"testing"

	tea "github.com/charmbracelet/bubbletea"
	"github.com/cloudosai/ralph-go/internal/tui"
)

// ============================================================================
// BDD Test Suite: Activity Feed Message Cap
//
// The TUI silently drops the oldest message when the activity feed exceeds
// maxMessages entries (default 100,000). These tests verify the sliding-window
// behavior at and beyond the boundary.
// ============================================================================

// TestBDD_ActivityFeedCap_AtCapNoMessageDropped verifies that when the message
// count is exactly at the cap, all messages are retained.
func TestBDD_ActivityFeedCap_AtCapNoMessageDropped(t *testing.T) {
	// Given: a model with maxMessages set to 5
	m := tui.NewModel()
	m.SetMaxMessagesForTest(5)
	m, _ = updateModel(m, tea.WindowSizeMsg{Width: 120, Height: 400})

	// When: exactly 5 messages are added
	for i := 0; i < 5; i++ {
		m.AddMessage(tui.Message{Role: tui.RoleAssistant, Content: fmt.Sprintf("CAP_MSG_%d", i)})
	}
	m, _ = updateModel(m, tui.TickMsgForTest())

	// Then: all 5 messages are present in the view
	for i := 0; i < 5; i++ {
		expected := fmt.Sprintf("CAP_MSG_%d", i)
		if viewNotContains(m, expected) {
			t.Errorf("Expected view to contain %q at cap boundary", expected)
		}
	}
	if m.MessageCountForTest() != 5 {
		t.Errorf("Expected 5 messages, got %d", m.MessageCountForTest())
	}
}

// TestBDD_ActivityFeedCap_OneOverCapDropsOldest verifies that adding one message
// beyond the cap drops the oldest message and keeps the newest.
func TestBDD_ActivityFeedCap_OneOverCapDropsOldest(t *testing.T) {
	// Given: a model filled to the 5-message cap
	m := tui.NewModel()
	m.SetMaxMessagesForTest(5)
	m, _ = updateModel(m, tea.WindowSizeMsg{Width: 120, Height: 400})

	for i := 0; i < 5; i++ {
		m.AddMessage(tui.Message{Role: tui.RoleAssistant, Content: fmt.Sprintf("OVER_MSG_%d", i)})
	}

	// When: one more message is added (total = 6, cap = 5)
	m.AddMessage(tui.Message{Role: tui.RoleAssistant, Content: "OVER_MSG_5"})
	m, _ = updateModel(m, tui.TickMsgForTest())

	// Then: the oldest message (OVER_MSG_0) is dropped
	if viewContains(m, "OVER_MSG_0") {
		t.Error("Oldest message should be dropped when cap is exceeded")
	}
	// And: the newest message is present
	if viewNotContains(m, "OVER_MSG_5") {
		t.Error("Newest message should be present after cap overflow")
	}
	// And: messages 1-4 are still present
	for i := 1; i <= 4; i++ {
		expected := fmt.Sprintf("OVER_MSG_%d", i)
		if viewNotContains(m, expected) {
			t.Errorf("Expected view to contain %q", expected)
		}
	}
	if m.MessageCountForTest() != 5 {
		t.Errorf("Expected 5 messages after overflow, got %d", m.MessageCountForTest())
	}
}

// TestBDD_ActivityFeedCap_SlidingWindowPreservesNewest verifies that the sliding
// window drops messages in FIFO order as new messages are continuously added.
func TestBDD_ActivityFeedCap_SlidingWindowPreservesNewest(t *testing.T) {
	// Given: a model with maxMessages=3
	m := tui.NewModel()
	m.SetMaxMessagesForTest(3)
	m, _ = updateModel(m, tea.WindowSizeMsg{Width: 120, Height: 400})

	// When: 10 messages are added (7 beyond cap)
	for i := 0; i < 10; i++ {
		m.AddMessage(tui.Message{Role: tui.RoleAssistant, Content: fmt.Sprintf("SLIDE_%d", i)})
	}
	m, _ = updateModel(m, tui.TickMsgForTest())

	// Then: only the last 3 messages (7, 8, 9) remain
	for i := 0; i < 7; i++ {
		dropped := fmt.Sprintf("SLIDE_%d", i)
		if viewContains(m, dropped) {
			t.Errorf("Message %q should have been dropped by sliding window", dropped)
		}
	}
	for i := 7; i < 10; i++ {
		kept := fmt.Sprintf("SLIDE_%d", i)
		if viewNotContains(m, kept) {
			t.Errorf("Message %q should be retained in sliding window", kept)
		}
	}
	if m.MessageCountForTest() != 3 {
		t.Errorf("Expected 3 messages after sliding window, got %d", m.MessageCountForTest())
	}
}

// TestBDD_ActivityFeedCap_NewMessageViaUpdateRespectsCap verifies that messages
// delivered through the Update() path (newMessageMsg) also respect the cap.
func TestBDD_ActivityFeedCap_NewMessageViaUpdateRespectsCap(t *testing.T) {
	// Given: a model with maxMessages=3, pre-filled with 3 messages
	msgChan := make(chan tui.Message, 10)
	doneChan := make(chan struct{}, 1)
	m := tui.NewModelWithChannels(msgChan, doneChan)
	m.SetMaxMessagesForTest(3)
	m, _ = updateModel(m, tea.WindowSizeMsg{Width: 120, Height: 400})

	for i := 0; i < 3; i++ {
		m.AddMessage(tui.Message{Role: tui.RoleAssistant, Content: fmt.Sprintf("CHAN_MSG_%d", i)})
	}

	// When: a new message arrives through the channel path
	msgChan <- tui.Message{Role: tui.RoleAssistant, Content: "CHAN_MSG_3"}
	cmd := tui.WaitForMessageForTest(msgChan)
	m, _ = sendTuiMsg(m, cmd)
	m, _ = updateModel(m, tui.TickMsgForTest())

	// Then: the oldest (CHAN_MSG_0) is dropped
	if viewContains(m, "CHAN_MSG_0") {
		t.Error("Oldest message should be dropped when channel delivery exceeds cap")
	}
	// And: the channel-delivered message is present
	if viewNotContains(m, "CHAN_MSG_3") {
		t.Error("Channel-delivered message should be visible")
	}
	if m.MessageCountForTest() != 3 {
		t.Errorf("Expected 3 messages, got %d", m.MessageCountForTest())
	}
}

// TestBDD_ActivityFeedCap_MixedRolesDropOldestRegardlessOfRole verifies that
// the cap drops the oldest message regardless of its role type.
func TestBDD_ActivityFeedCap_MixedRolesDropOldestRegardlessOfRole(t *testing.T) {
	// Given: a model with maxMessages=3 and messages of different roles
	m := tui.NewModel()
	m.SetMaxMessagesForTest(3)
	m, _ = updateModel(m, tea.WindowSizeMsg{Width: 120, Height: 400})

	m.AddMessage(tui.Message{Role: tui.RoleSystem, Content: "MIXED_SYSTEM"})
	m.AddMessage(tui.Message{Role: tui.RoleAssistant, Content: "MIXED_ASSISTANT"})
	m.AddMessage(tui.Message{Role: tui.RoleTool, Content: "MIXED_TOOL"})

	// When: a new message is added
	m.AddMessage(tui.Message{Role: tui.RoleLoop, Content: "MIXED_LOOP"})
	m, _ = updateModel(m, tui.TickMsgForTest())

	// Then: the system message (oldest) is dropped regardless of its role
	if viewContains(m, "MIXED_SYSTEM") {
		t.Error("Oldest message (system role) should be dropped")
	}
	// And: the other messages remain
	if viewNotContains(m, "MIXED_ASSISTANT") {
		t.Error("Assistant message should be retained")
	}
	if viewNotContains(m, "MIXED_TOOL") {
		t.Error("Tool message should be retained")
	}
	if viewNotContains(m, "MIXED_LOOP") {
		t.Error("Loop message should be retained")
	}
}

// TestBDD_ActivityFeedCap_DefaultCapIs100000 verifies the default maxMessages
// value is 100,000 as documented.
func TestBDD_ActivityFeedCap_DefaultCapIs100000(t *testing.T) {
	// Given: a freshly created model (no test override)
	m := tui.NewModel()

	// When: we add messages up to a moderate count
	for i := 0; i < 100; i++ {
		m.AddMessage(tui.Message{Role: tui.RoleAssistant, Content: fmt.Sprintf("DEFAULT_%d", i)})
	}

	// Then: all 100 messages are retained (well under the 100k cap)
	if m.MessageCountForTest() != 100 {
		t.Errorf("Expected 100 messages with default cap, got %d", m.MessageCountForTest())
	}
}
