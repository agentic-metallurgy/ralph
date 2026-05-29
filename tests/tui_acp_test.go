package tests

import (
	"strings"
	"testing"
	"time"

	tea "github.com/charmbracelet/bubbletea"
	"github.com/cloudosai/ralph-go/internal/tui"
)

// addToolRow pushes a RoleTool message through the normal newMessageMsg path.
func addToolRow(t *testing.T, m tui.Model, id, kind, status, content string) tui.Model {
	t.Helper()
	msg := tui.SendMessage(tui.Message{
		Role:      tui.RoleTool,
		Content:   content,
		ToolUseID: id,
		Kind:      kind,
		Status:    status,
	})()
	m, _ = updateModel(m, msg)
	return m
}

// TestToolRowRendersKindAndStatusGlyph verifies an in_progress tool row shows
// the kind icon and the in_progress glyph.
func TestToolRowRendersKindAndStatusGlyph(t *testing.T) {
	model := tui.NewModel()
	model, _ = updateModel(model, tea.WindowSizeMsg{Width: 120, Height: 40})
	model = addToolRow(t, model, "t1", "read", "in_progress", "Read config.go")

	view := model.View()
	if !strings.Contains(view, "Read config.go") {
		t.Errorf("view missing tool title; got:\n%s", view)
	}
	if !strings.Contains(view, "⠋") {
		t.Errorf("view missing in_progress spinner glyph; got:\n%s", view)
	}
	if !strings.Contains(view, "📖") {
		t.Errorf("view missing read kind icon; got:\n%s", view)
	}
}

// TestToolStatusUpdateCompletes verifies a completed update flips the glyph.
func TestToolStatusUpdateCompletes(t *testing.T) {
	model := tui.NewModel()
	model, _ = updateModel(model, tea.WindowSizeMsg{Width: 120, Height: 40})
	model = addToolRow(t, model, "t1", "read", "in_progress", "Read config.go")

	// Flip to completed via the exported helper command.
	doneMsg := tui.SendToolStatusUpdate("t1", "completed")()
	model, _ = updateModel(model, doneMsg)

	view := model.View()
	if !strings.Contains(view, "✓") {
		t.Errorf("view missing completed glyph ✓ after status update; got:\n%s", view)
	}
}

// TestToolStatusUpdateFails verifies a failed update renders the failure glyph.
func TestToolStatusUpdateFails(t *testing.T) {
	model := tui.NewModel()
	model, _ = updateModel(model, tea.WindowSizeMsg{Width: 120, Height: 40})
	model = addToolRow(t, model, "t1", "execute", "in_progress", "Bash: go build")

	failMsg := tui.SendToolStatusUpdate("t1", "failed")()
	model, _ = updateModel(model, failMsg)

	view := model.View()
	if !strings.Contains(view, "✗") {
		t.Errorf("view missing failed glyph ✗ after status update; got:\n%s", view)
	}
}

// TestToolStatusUpdateOnlyMatchingRow verifies a status update mutates only the
// row whose ToolUseID matches and leaves siblings untouched.
func TestToolStatusUpdateOnlyMatchingRow(t *testing.T) {
	model := tui.NewModel()
	model, _ = updateModel(model, tea.WindowSizeMsg{Width: 120, Height: 40})
	model = addToolRow(t, model, "t1", "read", "in_progress", "Read alpha.go")
	model = addToolRow(t, model, "t2", "edit", "in_progress", "Edit beta.go")

	// Complete only t1.
	model, _ = updateModel(model, tui.SendToolStatusUpdate("t1", "completed")())

	view := model.View()
	if !strings.Contains(view, "✓") {
		t.Errorf("expected a completed glyph for t1; got:\n%s", view)
	}
	if !strings.Contains(view, "⠋") {
		t.Errorf("expected t2 to remain in_progress (spinner); got:\n%s", view)
	}
}

// TestToolRowShowsDurationOnCompletion verifies elapsed time is rendered when a
// tool resolves, using a controlled clock.
func TestToolRowShowsDurationOnCompletion(t *testing.T) {
	base := time.Date(2026, 1, 1, 12, 0, 0, 0, time.UTC)
	now := base
	tui.SetTimeNowForTest(func() time.Time { return now })
	defer tui.SetTimeNowForTest(time.Now)

	model := tui.NewModel()
	model, _ = updateModel(model, tea.WindowSizeMsg{Width: 120, Height: 40})
	model = addToolRow(t, model, "t1", "execute", "in_progress", "Bash: go build")

	// Advance the clock 1.4s, then complete the tool.
	now = base.Add(1400 * time.Millisecond)
	model, _ = updateModel(model, tui.SendToolStatusUpdate("t1", "completed")())

	view := model.View()
	if !strings.Contains(view, "1.4s") {
		t.Errorf("expected completed row to show elapsed 1.4s; got:\n%s", view)
	}
}

// TestThinkingIndicatorShownWhenIdle verifies the thinking/waiting indicator
// appears only when no tool is in progress.
func TestThinkingIndicatorShownWhenIdle(t *testing.T) {
	model := tui.NewModel()
	model, _ = updateModel(model, tea.WindowSizeMsg{Width: 120, Height: 40})
	model = addToolRow(t, model, "t1", "read", "in_progress", "Read config.go")

	// While a tool is running, no thinking indicator.
	if strings.Contains(model.View(), "thinking") {
		t.Errorf("did not expect thinking indicator while a tool is in_progress; got:\n%s", model.View())
	}

	// After it completes, the loop is idle → thinking indicator appears.
	model, _ = updateModel(model, tui.SendToolStatusUpdate("t1", "completed")())
	if !strings.Contains(model.View(), "thinking") {
		t.Errorf("expected thinking indicator once idle; got:\n%s", model.View())
	}
}

// TestToolStatusUpdateUnknownIDNoCrash verifies an update for an unknown ID is a
// harmless no-op.
func TestToolStatusUpdateUnknownIDNoCrash(t *testing.T) {
	model := tui.NewModel()
	model, _ = updateModel(model, tea.WindowSizeMsg{Width: 120, Height: 40})
	model = addToolRow(t, model, "t1", "read", "in_progress", "Read config.go")

	model, _ = updateModel(model, tui.SendToolStatusUpdate("does-not-exist", "completed")())

	view := model.View()
	// t1 should still be in_progress since nothing matched.
	if !strings.Contains(view, "⠋") {
		t.Errorf("expected t1 to remain in_progress after no-op update; got:\n%s", view)
	}
}
