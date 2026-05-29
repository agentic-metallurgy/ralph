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
