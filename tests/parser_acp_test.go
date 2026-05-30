package tests

import (
	"strings"
	"testing"

	"github.com/cloudosai/ralph-go/internal/parser"
)

// TestClassifyToolKind verifies Claude tool names map to ACP-style kinds.
func TestClassifyToolKind(t *testing.T) {
	cases := []struct {
		name string
		want parser.ToolKind
	}{
		{"Read", parser.ToolKindRead},
		{"NotebookRead", parser.ToolKindRead},
		{"Edit", parser.ToolKindEdit},
		{"MultiEdit", parser.ToolKindEdit},
		{"Write", parser.ToolKindEdit},
		{"Bash", parser.ToolKindExecute},
		{"Glob", parser.ToolKindSearch},
		{"Grep", parser.ToolKindSearch},
		{"WebFetch", parser.ToolKindFetch},
		{"Task", parser.ToolKindThink},
		{"SomeMcpTool", parser.ToolKindOther},
		{"", parser.ToolKindOther},
	}
	for _, c := range cases {
		if got := parser.ClassifyToolKind(c.name); got != c.want {
			t.Errorf("ClassifyToolKind(%q) = %q, want %q", c.name, got, c.want)
		}
	}
}

// TestExtractContentToolUseACPFields verifies ID/Kind/Title/Location are
// populated on the extracted ToolUse.
func TestExtractContentToolUseACPFields(t *testing.T) {
	p := parser.NewParser()

	line := `{"type":"assistant","message":{"content":[{"type":"tool_use","id":"toolu_123","name":"Read","input":{"file_path":"/proj/internal/config.go"}}]}}`
	content := p.ExtractContent(p.ParseLine(line))

	if len(content.ToolUses) != 1 {
		t.Fatalf("Expected 1 tool use, got %d", len(content.ToolUses))
	}
	tu := content.ToolUses[0]
	if tu.ID != "toolu_123" {
		t.Errorf("ID = %q, want %q", tu.ID, "toolu_123")
	}
	if tu.Kind != parser.ToolKindRead {
		t.Errorf("Kind = %q, want %q", tu.Kind, parser.ToolKindRead)
	}
	if tu.Title != "Read config.go" {
		t.Errorf("Title = %q, want %q", tu.Title, "Read config.go")
	}
	if tu.Location != "/proj/internal/config.go" {
		t.Errorf("Location = %q, want %q", tu.Location, "/proj/internal/config.go")
	}
	// FilePath retained as alias for back-compat.
	if tu.FilePath != tu.Location {
		t.Errorf("FilePath = %q, want alias of Location %q", tu.FilePath, tu.Location)
	}
}

// TestExtractContentToolUseTitleVariants checks title construction per kind.
func TestExtractContentToolUseTitleVariants(t *testing.T) {
	p := parser.NewParser()
	cases := []struct {
		name      string
		line      string
		wantTitle string
	}{
		{
			"bash with description",
			`{"type":"assistant","message":{"content":[{"type":"tool_use","name":"Bash","input":{"command":"go build ./...","description":"Build the project"}}]}}`,
			"Build the project",
		},
		{
			"bash without description",
			`{"type":"assistant","message":{"content":[{"type":"tool_use","name":"Bash","input":{"command":"echo hi"}}]}}`,
			"Bash: echo hi",
		},
		{
			"grep pattern",
			`{"type":"assistant","message":{"content":[{"type":"tool_use","name":"Grep","input":{"pattern":"func main"}}]}}`,
			"Grep func main",
		},
		{
			"task description",
			`{"type":"assistant","message":{"content":[{"type":"tool_use","name":"Task","input":{"description":"Explore the codebase"}}]}}`,
			"Explore the codebase",
		},
		{
			"unknown tool falls back to name",
			`{"type":"assistant","message":{"content":[{"type":"tool_use","name":"WeirdTool","input":{}}]}}`,
			"WeirdTool",
		},
	}
	for _, c := range cases {
		t.Run(c.name, func(t *testing.T) {
			content := p.ExtractContent(p.ParseLine(c.line))
			if len(content.ToolUses) != 1 {
				t.Fatalf("Expected 1 tool use, got %d", len(content.ToolUses))
			}
			if got := content.ToolUses[0].Title; got != c.wantTitle {
				t.Errorf("Title = %q, want %q", got, c.wantTitle)
			}
		})
	}
}

// TestExtractContentToolResultStatus verifies tool_result carries the
// correlating ID and error flag for status mapping.
func TestExtractContentToolResultStatus(t *testing.T) {
	p := parser.NewParser()

	t.Run("success", func(t *testing.T) {
		line := `{"type":"user","message":{"content":[{"type":"tool_result","tool_use_id":"toolu_1","content":"ok"}]}}`
		content := p.ExtractContent(p.ParseLine(line))
		if len(content.ToolResults) != 1 {
			t.Fatalf("Expected 1 tool result, got %d", len(content.ToolResults))
		}
		if content.ToolResults[0].ToolUseID != "toolu_1" {
			t.Errorf("ToolUseID = %q, want %q", content.ToolResults[0].ToolUseID, "toolu_1")
		}
		if content.ToolResults[0].IsError {
			t.Error("expected IsError=false for successful result")
		}
	})

	t.Run("failure", func(t *testing.T) {
		line := `{"type":"user","message":{"content":[{"type":"tool_result","tool_use_id":"toolu_2","is_error":true,"content":"boom"}]}}`
		content := p.ExtractContent(p.ParseLine(line))
		if len(content.ToolResults) != 1 {
			t.Fatalf("Expected 1 tool result, got %d", len(content.ToolResults))
		}
		if !content.ToolResults[0].IsError {
			t.Error("expected IsError=true for failed result")
		}
	})

	t.Run("empty content still recorded when id present", func(t *testing.T) {
		line := `{"type":"user","message":{"content":[{"type":"tool_result","tool_use_id":"toolu_3","content":""}]}}`
		content := p.ExtractContent(p.ParseLine(line))
		if len(content.ToolResults) != 1 {
			t.Fatalf("Expected 1 tool result (empty content but with id), got %d", len(content.ToolResults))
		}
		if content.ToolResults[0].ToolUseID != "toolu_3" {
			t.Errorf("ToolUseID = %q, want %q", content.ToolResults[0].ToolUseID, "toolu_3")
		}
	})
}

// TestExtractPlan verifies TodoWrite inputs become ordered PlanItems.
func TestExtractPlan(t *testing.T) {
	input := map[string]interface{}{
		"todos": []interface{}{
			map[string]interface{}{"content": "Add ClassifyToolKind", "status": "completed", "activeForm": "Adding ClassifyToolKind"},
			map[string]interface{}{"content": "Wire dispatch", "status": "in_progress", "activeForm": "Wiring dispatch"},
			map[string]interface{}{"content": "Write tests", "status": "pending", "activeForm": "Writing tests"},
		},
	}
	plan := parser.ExtractPlan(input)
	if len(plan) != 3 {
		t.Fatalf("Expected 3 plan items, got %d", len(plan))
	}
	want := []parser.PlanItem{
		{Content: "Add ClassifyToolKind", Status: parser.PlanCompleted},
		{Content: "Wire dispatch", Status: parser.PlanInProgress},
		{Content: "Write tests", Status: parser.PlanPending},
	}
	for i, w := range want {
		if plan[i] != w {
			t.Errorf("plan[%d] = %+v, want %+v", i, plan[i], w)
		}
	}
}

// TestExtractPlanDefensive checks fallbacks, unknown status, empty-skip, non-todo.
func TestExtractPlanDefensive(t *testing.T) {
	t.Run("activeForm fallback when content absent", func(t *testing.T) {
		input := map[string]interface{}{
			"todos": []interface{}{
				map[string]interface{}{"activeForm": "Doing the thing", "status": "in_progress"},
			},
		}
		plan := parser.ExtractPlan(input)
		if len(plan) != 1 || plan[0].Content != "Doing the thing" {
			t.Fatalf("expected activeForm fallback, got %+v", plan)
		}
	})

	t.Run("unknown status normalizes to pending", func(t *testing.T) {
		input := map[string]interface{}{
			"todos": []interface{}{
				map[string]interface{}{"content": "Mystery", "status": "weird"},
				map[string]interface{}{"content": "NoStatus"},
			},
		}
		plan := parser.ExtractPlan(input)
		if len(plan) != 2 {
			t.Fatalf("expected 2 items, got %d", len(plan))
		}
		if plan[0].Status != parser.PlanPending || plan[1].Status != parser.PlanPending {
			t.Errorf("expected pending normalization, got %+v", plan)
		}
	})

	t.Run("empty-content items skipped", func(t *testing.T) {
		input := map[string]interface{}{
			"todos": []interface{}{
				map[string]interface{}{"content": "", "status": "completed"},
				map[string]interface{}{"status": "pending"},
				map[string]interface{}{"content": "Kept", "status": "pending"},
			},
		}
		plan := parser.ExtractPlan(input)
		if len(plan) != 1 || plan[0].Content != "Kept" {
			t.Fatalf("expected only non-empty item kept, got %+v", plan)
		}
	})

	t.Run("non-todo input returns nil", func(t *testing.T) {
		if plan := parser.ExtractPlan(map[string]interface{}{"file_path": "x.go"}); plan != nil {
			t.Errorf("expected nil for non-todo input, got %+v", plan)
		}
		if plan := parser.ExtractPlan(nil); plan != nil {
			t.Errorf("expected nil for nil input, got %+v", plan)
		}
	})
}

// TestExtractContentPopulatesPlan verifies a TodoWrite tool_use surfaces a Plan
// on ParsedContent, and that the tool use is still counted (noop semantics).
func TestExtractContentPopulatesPlan(t *testing.T) {
	p := parser.NewParser()
	line := `{"type":"assistant","message":{"content":[{"type":"tool_use","id":"toolu_t","name":"TodoWrite","input":{"todos":[{"content":"A","status":"completed"},{"content":"B","status":"in_progress"}]}}]}}`
	content := p.ExtractContent(p.ParseLine(line))
	if len(content.Plan) != 2 {
		t.Fatalf("expected 2 plan items, got %d", len(content.Plan))
	}
	if content.Plan[0].Status != parser.PlanCompleted || content.Plan[1].Status != parser.PlanInProgress {
		t.Errorf("unexpected plan statuses: %+v", content.Plan)
	}
	// The TodoWrite tool use is still present so iterToolUseCount is unaffected.
	if len(content.ToolUses) != 1 {
		t.Errorf("expected TodoWrite to still appear as a tool use, got %d", len(content.ToolUses))
	}
}

// TestExtractContentNoPlanForNonTodo verifies non-TodoWrite tools leave Plan nil.
func TestExtractContentNoPlanForNonTodo(t *testing.T) {
	p := parser.NewParser()
	line := `{"type":"assistant","message":{"content":[{"type":"tool_use","id":"toolu_r","name":"Read","input":{"file_path":"/a/b.go"}}]}}`
	content := p.ExtractContent(p.ParseLine(line))
	if content.Plan != nil {
		t.Errorf("expected nil Plan for non-TodoWrite tool, got %+v", content.Plan)
	}
}

// TestToolTitleTruncation ensures long commands/patterns are truncated.
func TestToolTitleTruncation(t *testing.T) {
	p := parser.NewParser()
	longCmd := strings.Repeat("x", 200)
	line := `{"type":"assistant","message":{"content":[{"type":"tool_use","name":"Bash","input":{"command":"` + longCmd + `"}}]}}`
	content := p.ExtractContent(p.ParseLine(line))
	if len(content.ToolUses) != 1 {
		t.Fatalf("Expected 1 tool use, got %d", len(content.ToolUses))
	}
	if !strings.HasSuffix(content.ToolUses[0].Title, "...") {
		t.Errorf("expected truncated title to end with ..., got %q", content.ToolUses[0].Title)
	}
}
