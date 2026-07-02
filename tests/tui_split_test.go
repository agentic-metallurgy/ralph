package tests

import (
	"fmt"
	"strings"
	"testing"

	tea "github.com/charmbracelet/bubbletea"
	"github.com/charmbracelet/lipgloss"
	"github.com/cloudosai/ralph-go/internal/tui"
)

// sendTo pushes a message through the normal newMessageMsg path so the split
// viewports are refreshed exactly as in production.
func sendTo(t *testing.T, m tui.Model, msg tui.Message) tui.Model {
	t.Helper()
	updated, _ := updateModel(m, tui.SendMessage(msg)())
	return updated
}

// lineContaining returns the first rendered line that contains sub, or "".
func lineContaining(view, sub string) string {
	for _, line := range strings.Split(view, "\n") {
		if strings.Contains(line, sub) {
			return line
		}
	}
	return ""
}

// TestSplit_ThinkingWrappedToFullLength verifies the spec's core requirement:
// a long thinking block is word-wrapped in the left pane and its ENTIRE length
// is visible (both the first and last words appear). On the pre-split layout a
// long single-line block was clipped horizontally by the viewport, so the tail
// would be missing.
func TestSplit_ThinkingWrappedToFullLength(t *testing.T) {
	model := tui.NewModel()
	model, _ = updateModel(model, tea.WindowSizeMsg{Width: 120, Height: 40})

	// ~280 chars: comfortably wider than the ~54-col left pane, so it must wrap
	// to several lines for the tail to be visible.
	middle := strings.Repeat("reasoning step and more detail ", 8)
	content := "THINK_HEAD " + middle + "THINK_TAIL_SENTINEL"
	model = sendTo(t, model, tui.Message{Role: tui.RoleThinking, Content: content})

	view := model.View()
	if !strings.Contains(view, "THINK_HEAD") {
		t.Errorf("thinking pane should show the start of the block; got:\n%s", view)
	}
	if !strings.Contains(view, "THINK_TAIL_SENTINEL") {
		t.Errorf("thinking pane should show the ENTIRE length (tail) via word-wrap; got:\n%s", view)
	}
}

// TestSplit_ThinkingAndToolCoexist verifies thinking narrative and tool-use rows
// both render after the split, with the tool pane keeping its glyph/icon detail.
func TestSplit_ThinkingAndToolCoexist(t *testing.T) {
	model := tui.NewModel()
	model, _ = updateModel(model, tea.WindowSizeMsg{Width: 120, Height: 40})

	model = sendTo(t, model, tui.Message{Role: tui.RoleThinking, Content: "LEFT_THINKING_TEXT"})
	model = addToolRow(t, model, "t1", "read", "in_progress", "Read config.go")

	view := model.View()
	for _, want := range []string{"LEFT_THINKING_TEXT", "Read config.go", "📖", "⠋"} {
		if !strings.Contains(view, want) {
			t.Errorf("split view should contain %q; got:\n%s", want, view)
		}
	}
}

// TestSplit_PanesAreSideBySide verifies the panes are laid out horizontally:
// a thinking message and a tool row added in the same turn land on the SAME
// physical rendered line (left column then right column). This fails on the old
// vertical single-pane layout where they would be on different lines.
func TestSplit_PanesAreSideBySide(t *testing.T) {
	model := tui.NewModel()
	model, _ = updateModel(model, tea.WindowSizeMsg{Width: 120, Height: 40})

	model = sendTo(t, model, tui.Message{Role: tui.RoleThinking, Content: "ALPHA_THINK"})
	model = addToolRow(t, model, "t1", "read", "in_progress", "BETA_TOOL")

	view := model.View()
	line := lineContaining(view, "ALPHA_THINK")
	if line == "" {
		t.Fatalf("expected a line containing ALPHA_THINK; got:\n%s", view)
	}
	if !strings.Contains(line, "BETA_TOOL") {
		t.Errorf("expected ALPHA_THINK (left pane) and BETA_TOOL (right pane) on the same physical line (side-by-side split); got line:\n%q\nfull view:\n%s", line, view)
	}
	// And the left content must come before the right content on that line.
	if strings.Index(line, "ALPHA_THINK") >= strings.Index(line, "BETA_TOOL") {
		t.Errorf("thinking pane should be left of the tool pane; got line:\n%q", line)
	}
}

// TestSplit_ScrollKeysDriveThinkingPane verifies PgUp scrolls the left/thinking
// pane (where the narrative lives) and a tick does not snap it back.
func TestSplit_ScrollKeysDriveThinkingPane(t *testing.T) {
	model := tui.NewModel()
	// Small height so the thinking pane must scroll.
	model, _ = updateModel(model, tea.WindowSizeMsg{Width: 120, Height: 22})

	for i := 0; i < 30; i++ {
		model = sendTo(t, model, tui.Message{Role: tui.RoleThinking, Content: fmt.Sprintf("THINK_LINE_%02d", i)})
	}

	// Newest visible at bottom on arrival.
	if !strings.Contains(model.View(), "THINK_LINE_29") {
		t.Fatalf("thinking pane should auto-scroll to the newest line; got:\n%s", model.View())
	}

	for i := 0; i < 12; i++ {
		model, _ = updateModel(model, tea.KeyMsg{Type: tea.KeyPgUp})
	}
	scrolled := model.View()
	if !strings.Contains(scrolled, "THINK_LINE_00") {
		t.Fatalf("PgUp should scroll the thinking pane to the earliest line; got:\n%s", scrolled)
	}

	// A tick must not reset the scroll position.
	model, _ = updateModel(model, tui.TickMsgForTest())
	if strings.Contains(model.View(), "THINK_LINE_29") {
		t.Error("tick should not snap the thinking pane back to bottom")
	}
	if !strings.Contains(model.View(), "THINK_LINE_00") {
		t.Error("thinking pane scroll position should be preserved across a tick")
	}
}

// TestSplit_ToolMessageKeepsThinkingScroll verifies that a tool row arriving
// does NOT yank the thinking pane back to the bottom. The split's whole point is
// that the thinking pane preserves the user's scroll position; only a new
// narrative message should auto-follow it. (Regression: newMessageMsg used to
// GotoBottom the thinking pane for every message, including tool rows.)
func TestSplit_ToolMessageKeepsThinkingScroll(t *testing.T) {
	model := tui.NewModel()
	model, _ = updateModel(model, tea.WindowSizeMsg{Width: 120, Height: 22})

	for i := 0; i < 30; i++ {
		model = sendTo(t, model, tui.Message{Role: tui.RoleThinking, Content: fmt.Sprintf("THINK_LINE_%02d", i)})
	}
	// Scroll up to the earliest line.
	for i := 0; i < 12; i++ {
		model, _ = updateModel(model, tea.KeyMsg{Type: tea.KeyPgUp})
	}
	if !strings.Contains(model.View(), "THINK_LINE_00") {
		t.Fatalf("precondition: thinking pane should be scrolled to the top; got:\n%s", model.View())
	}

	// A tool row lands in the right pane — it must not disturb the left pane.
	model = addToolRow(t, model, "t1", "read", "in_progress", "Read config.go")
	if strings.Contains(model.View(), "THINK_LINE_29") {
		t.Error("a tool message should not snap the thinking pane back to the bottom")
	}
	if !strings.Contains(model.View(), "THINK_LINE_00") {
		t.Error("thinking pane scroll position should be preserved when a tool row arrives")
	}
}

// TestSplit_NoWidthOverflow verifies the joined panes never exceed the terminal
// width (which would wrap/garble the layout) across a range of sizes.
func TestSplit_NoWidthOverflow(t *testing.T) {
	for _, w := range []int{40, 80, 120, 200} {
		model := tui.NewModel()
		model, _ = updateModel(model, tea.WindowSizeMsg{Width: w, Height: 40})
		model = sendTo(t, model, tui.Message{Role: tui.RoleThinking, Content: "width check"})
		model = addToolRow(t, model, "t1", "execute", "in_progress", "Bash: go build ./...")

		view := model.View()
		for _, line := range strings.Split(view, "\n") {
			if lw := lipgloss.Width(line); lw > w {
				t.Errorf("at terminal width %d a rendered line is %d wide (overflow): %q", w, lw, line)
			}
		}
	}
}
