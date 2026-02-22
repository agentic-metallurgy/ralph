package tests

import (
	"testing"

	"github.com/cloudosai/ralph-go/internal/parser"
)

func TestNewParser(t *testing.T) {
	p := parser.NewParser()
	if p == nil {
		t.Error("Expected non-nil parser")
	}
}

func TestParseLineEmpty(t *testing.T) {
	p := parser.NewParser()

	tests := []struct {
		name  string
		input string
	}{
		{"empty string", ""},
		{"whitespace", "   "},
		{"newline", "\n"},
		{"tab", "\t"},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result := p.ParseLine(tt.input)
			if result != nil {
				t.Errorf("Expected nil for %q, got %+v", tt.input, result)
			}
		})
	}
}

func TestParseLineLoopMarker(t *testing.T) {
	p := parser.NewParser()

	// Loop markers should return nil from ParseLine (not JSON)
	loopMarkers := []string{
		"======= LOOP 1/20 =======",
		"======= LOOP 5/10 =======",
		"=======",
	}

	for _, marker := range loopMarkers {
		result := p.ParseLine(marker)
		if result != nil {
			t.Errorf("Expected nil for loop marker %q, got %+v", marker, result)
		}
	}
}

func TestParseLineNonJSON(t *testing.T) {
	p := parser.NewParser()

	nonJSON := []string{
		"hello world",
		"not json at all",
		"[array not supported]",
		"123",
		"true",
	}

	for _, line := range nonJSON {
		result := p.ParseLine(line)
		if result != nil {
			t.Errorf("Expected nil for non-JSON %q, got %+v", line, result)
		}
	}
}

func TestParseLineSystemMessage(t *testing.T) {
	p := parser.NewParser()

	line := `{"type":"system"}`
	result := p.ParseLine(line)

	if result == nil {
		t.Fatal("Expected non-nil result")
	}
	if result.Type != parser.MessageTypeSystem {
		t.Errorf("Expected type 'system', got %q", result.Type)
	}
}

func TestParseLineAssistantMessage(t *testing.T) {
	p := parser.NewParser()

	line := `{"type":"assistant","message":{"content":[{"type":"text","text":"Hello world"}],"usage":{"input_tokens":100,"output_tokens":50}}}`
	result := p.ParseLine(line)

	if result == nil {
		t.Fatal("Expected non-nil result")
	}
	if result.Type != parser.MessageTypeAssistant {
		t.Errorf("Expected type 'assistant', got %q", result.Type)
	}
	if result.Message == nil {
		t.Fatal("Expected non-nil message")
	}
	if len(result.Message.Content) != 1 {
		t.Fatalf("Expected 1 content item, got %d", len(result.Message.Content))
	}
	if result.Message.Content[0].Type != parser.ContentTypeText {
		t.Errorf("Expected content type 'text', got %q", result.Message.Content[0].Type)
	}
	if result.Message.Content[0].Text != "Hello world" {
		t.Errorf("Expected text 'Hello world', got %q", result.Message.Content[0].Text)
	}
}

func TestParseLineAssistantWithUsage(t *testing.T) {
	p := parser.NewParser()

	line := `{"type":"assistant","message":{"content":[],"usage":{"input_tokens":100,"output_tokens":50,"cache_creation_input_tokens":20,"cache_read_input_tokens":10}}}`
	result := p.ParseLine(line)

	if result == nil {
		t.Fatal("Expected non-nil result")
	}
	if result.Message == nil || result.Message.Usage == nil {
		t.Fatal("Expected non-nil usage")
	}

	usage := result.Message.Usage
	if usage.InputTokens != 100 {
		t.Errorf("Expected InputTokens 100, got %d", usage.InputTokens)
	}
	if usage.OutputTokens != 50 {
		t.Errorf("Expected OutputTokens 50, got %d", usage.OutputTokens)
	}
	if usage.CacheCreationInputTokens != 20 {
		t.Errorf("Expected CacheCreationInputTokens 20, got %d", usage.CacheCreationInputTokens)
	}
	if usage.CacheReadInputTokens != 10 {
		t.Errorf("Expected CacheReadInputTokens 10, got %d", usage.CacheReadInputTokens)
	}
}

func TestParseLineResultMessage(t *testing.T) {
	p := parser.NewParser()

	line := `{"type":"result","total_cost_usd":0.000123}`
	result := p.ParseLine(line)

	if result == nil {
		t.Fatal("Expected non-nil result")
	}
	if result.Type != parser.MessageTypeResult {
		t.Errorf("Expected type 'result', got %q", result.Type)
	}
	if result.TotalCostUSD != 0.000123 {
		t.Errorf("Expected TotalCostUSD 0.000123, got %f", result.TotalCostUSD)
	}
}

func TestParseLineUserMessage(t *testing.T) {
	p := parser.NewParser()

	line := `{"type":"user","message":{"content":[{"type":"tool_result","content":"result text"}]}}`
	result := p.ParseLine(line)

	if result == nil {
		t.Fatal("Expected non-nil result")
	}
	if result.Type != parser.MessageTypeUser {
		t.Errorf("Expected type 'user', got %q", result.Type)
	}
}

func TestParseLineToolUse(t *testing.T) {
	p := parser.NewParser()

	line := `{"type":"assistant","message":{"content":[{"type":"tool_use","name":"Bash","input":{"command":"ls -la"}}]}}`
	result := p.ParseLine(line)

	if result == nil {
		t.Fatal("Expected non-nil result")
	}
	if len(result.Message.Content) != 1 {
		t.Fatalf("Expected 1 content item, got %d", len(result.Message.Content))
	}

	content := result.Message.Content[0]
	if content.Type != parser.ContentTypeToolUse {
		t.Errorf("Expected content type 'tool_use', got %q", content.Type)
	}
	if content.Name != "Bash" {
		t.Errorf("Expected tool name 'Bash', got %q", content.Name)
	}
	if content.Input["command"] != "ls -la" {
		t.Errorf("Expected input command 'ls -la', got %v", content.Input["command"])
	}
}

func TestParseLineInvalidJSON(t *testing.T) {
	p := parser.NewParser()

	invalidJSON := []string{
		"{not valid}",
		`{"unclosed":`,
		`{"type":"assistant"`,
	}

	for _, line := range invalidJSON {
		result := p.ParseLine(line)
		if result != nil {
			t.Errorf("Expected nil for invalid JSON %q, got %+v", line, result)
		}
	}
}

func TestParseLoopMarker(t *testing.T) {
	p := parser.NewParser()

	tests := []struct {
		name     string
		input    string
		current  int
		total    int
		isMarker bool
	}{
		{"standard marker", "======= LOOP 1/20 =======", 1, 20, true},
		{"middle loop", "======= LOOP 5/10 =======", 5, 10, true},
		{"last loop", "======= LOOP 20/20 =======", 20, 20, true},
		{"large numbers", "======= LOOP 100/1000 =======", 100, 1000, true},
		{"not a marker", "not a loop marker", 0, 0, false},
		{"json line", `{"type":"system"}`, 0, 0, false},
		{"empty", "", 0, 0, false},
		{"partial marker", "=======", 0, 0, false},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result := p.ParseLoopMarker(tt.input)

			if tt.isMarker {
				if result == nil {
					t.Fatalf("Expected loop marker for %q, got nil", tt.input)
				}
				if result.Current != tt.current {
					t.Errorf("Expected current %d, got %d", tt.current, result.Current)
				}
				if result.Total != tt.total {
					t.Errorf("Expected total %d, got %d", tt.total, result.Total)
				}
			} else {
				if result != nil {
					t.Errorf("Expected nil for %q, got %+v", tt.input, result)
				}
			}
		})
	}
}

func TestStripSystemReminders(t *testing.T) {
	p := parser.NewParser()

	tests := []struct {
		name     string
		input    string
		expected string
	}{
		{
			"no reminder",
			"Hello world",
			"Hello world",
		},
		{
			"single reminder",
			"Before <system-reminder>secret</system-reminder> After",
			"Before  After",
		},
		{
			"multiline reminder",
			"Before <system-reminder>\nThis is\nmultiline\n</system-reminder> After",
			"Before  After",
		},
		{
			"multiple reminders",
			"A <system-reminder>1</system-reminder> B <system-reminder>2</system-reminder> C",
			"A  B  C",
		},
		{
			"empty text",
			"",
			"",
		},
		{
			"only reminder",
			"<system-reminder>only this</system-reminder>",
			"",
		},
		{
			"reminder with whitespace",
			"  <system-reminder>test</system-reminder>  ",
			"",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result := p.StripSystemReminders(tt.input)
			if result != tt.expected {
				t.Errorf("Expected %q, got %q", tt.expected, result)
			}
		})
	}
}

func TestExtractThinking(t *testing.T) {
	p := parser.NewParser()

	tests := []struct {
		name     string
		input    string
		expected string
	}{
		{
			"no thinking",
			"Just regular text",
			"",
		},
		{
			"simple thinking",
			"Before <thinking>This is my thought</thinking> After",
			"This is my thought",
		},
		{
			"multiline thinking",
			"<thinking>\nLine 1\nLine 2\n</thinking>",
			"Line 1\nLine 2",
		},
		{
			"empty",
			"",
			"",
		},
		{
			"unclosed thinking",
			"<thinking>unclosed",
			"",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result := p.ExtractThinking(tt.input)
			if result != tt.expected {
				t.Errorf("Expected %q, got %q", tt.expected, result)
			}
		})
	}
}

func TestExtractContentFromTextMessage(t *testing.T) {
	p := parser.NewParser()

	line := `{"type":"assistant","message":{"content":[{"type":"text","text":"Hello world"}]}}`
	msg := p.ParseLine(line)
	content := p.ExtractContent(msg)

	if len(content.TextContent) != 1 {
		t.Fatalf("Expected 1 text item, got %d", len(content.TextContent))
	}
	if content.TextContent[0] != "Hello world" {
		t.Errorf("Expected 'Hello world', got %q", content.TextContent[0])
	}
}

func TestExtractContentStripsSystemReminders(t *testing.T) {
	p := parser.NewParser()

	line := `{"type":"assistant","message":{"content":[{"type":"text","text":"Before <system-reminder>secret</system-reminder> After"}]}}`
	msg := p.ParseLine(line)
	content := p.ExtractContent(msg)

	if len(content.TextContent) != 1 {
		t.Fatalf("Expected 1 text item, got %d", len(content.TextContent))
	}
	if content.TextContent[0] != "Before  After" {
		t.Errorf("Expected 'Before  After', got %q", content.TextContent[0])
	}
}

func TestExtractContentThinking(t *testing.T) {
	p := parser.NewParser()

	line := `{"type":"assistant","message":{"content":[{"type":"text","text":"<thinking>My thoughts</thinking>"}]}}`
	msg := p.ParseLine(line)
	content := p.ExtractContent(msg)

	if content.Thinking != "My thoughts" {
		t.Errorf("Expected thinking 'My thoughts', got %q", content.Thinking)
	}
	// Text with only thinking should not add to TextContent
	if len(content.TextContent) != 0 {
		t.Errorf("Expected 0 text items (only thinking), got %d", len(content.TextContent))
	}
}

func TestExtractContentToolUse(t *testing.T) {
	p := parser.NewParser()

	line := `{"type":"assistant","message":{"content":[{"type":"tool_use","name":"Read","input":{"file_path":"/test/file.txt"}}]}}`
	msg := p.ParseLine(line)
	content := p.ExtractContent(msg)

	if len(content.ToolUses) != 1 {
		t.Fatalf("Expected 1 tool use, got %d", len(content.ToolUses))
	}
	if content.ToolUses[0].Name != "Read" {
		t.Errorf("Expected tool name 'Read', got %q", content.ToolUses[0].Name)
	}
}

func TestExtractContentToolResult(t *testing.T) {
	p := parser.NewParser()

	line := `{"type":"user","message":{"content":[{"type":"tool_result","content":"Result text here"}]}}`
	msg := p.ParseLine(line)
	content := p.ExtractContent(msg)

	if len(content.ToolResults) != 1 {
		t.Fatalf("Expected 1 tool result, got %d", len(content.ToolResults))
	}
	if content.ToolResults[0].Content != "Result text here" {
		t.Errorf("Expected 'Result text here', got %q", content.ToolResults[0].Content)
	}
}

func TestExtractContentToolResultWithSystemReminder(t *testing.T) {
	p := parser.NewParser()

	line := `{"type":"user","message":{"content":[{"type":"tool_result","content":"Before <system-reminder>secret</system-reminder> After"}]}}`
	msg := p.ParseLine(line)
	content := p.ExtractContent(msg)

	if len(content.ToolResults) != 1 {
		t.Fatalf("Expected 1 tool result, got %d", len(content.ToolResults))
	}
	if content.ToolResults[0].Content != "Before  After" {
		t.Errorf("Expected 'Before  After', got %q", content.ToolResults[0].Content)
	}
}

func TestExtractContentNil(t *testing.T) {
	p := parser.NewParser()

	content := p.ExtractContent(nil)
	if content == nil {
		t.Fatal("Expected non-nil content for nil message")
	}
	if len(content.TextContent) != 0 {
		t.Errorf("Expected empty text content, got %d", len(content.TextContent))
	}
}

func TestGetMessageType(t *testing.T) {
	p := parser.NewParser()

	tests := []struct {
		name     string
		line     string
		expected parser.MessageType
	}{
		{"system", `{"type":"system"}`, parser.MessageTypeSystem},
		{"assistant", `{"type":"assistant"}`, parser.MessageTypeAssistant},
		{"user", `{"type":"user"}`, parser.MessageTypeUser},
		{"result", `{"type":"result"}`, parser.MessageTypeResult},
		{"nil", "", parser.MessageTypeUnknown},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			var msg *parser.ParsedMessage
			if tt.line != "" {
				msg = p.ParseLine(tt.line)
			}
			result := p.GetMessageType(msg)
			if result != tt.expected {
				t.Errorf("Expected %q, got %q", tt.expected, result)
			}
		})
	}
}

func TestGetUsage(t *testing.T) {
	p := parser.NewParser()

	// With usage
	line := `{"type":"assistant","message":{"content":[],"usage":{"input_tokens":100,"output_tokens":50}}}`
	msg := p.ParseLine(line)
	usage := p.GetUsage(msg)

	if usage == nil {
		t.Fatal("Expected non-nil usage")
	}
	if usage.InputTokens != 100 {
		t.Errorf("Expected InputTokens 100, got %d", usage.InputTokens)
	}

	// Without usage
	lineNoUsage := `{"type":"assistant","message":{"content":[]}}`
	msgNoUsage := p.ParseLine(lineNoUsage)
	usageNil := p.GetUsage(msgNoUsage)

	if usageNil != nil {
		t.Errorf("Expected nil usage, got %+v", usageNil)
	}

	// Nil message
	usageFromNil := p.GetUsage(nil)
	if usageFromNil != nil {
		t.Errorf("Expected nil usage from nil message, got %+v", usageFromNil)
	}
}

func TestGetCost(t *testing.T) {
	p := parser.NewParser()

	tests := []struct {
		name     string
		line     string
		expected float64
	}{
		{"result with cost", `{"type":"result","total_cost_usd":0.000123}`, 0.000123},
		{"result no cost", `{"type":"result"}`, 0},
		{"assistant message", `{"type":"assistant"}`, 0},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			msg := p.ParseLine(tt.line)
			cost := p.GetCost(msg)
			if cost != tt.expected {
				t.Errorf("Expected %f, got %f", tt.expected, cost)
			}
		})
	}

	// Nil message
	costFromNil := p.GetCost(nil)
	if costFromNil != 0 {
		t.Errorf("Expected 0 cost from nil message, got %f", costFromNil)
	}
}

func TestToolUseTruncation(t *testing.T) {
	p := parser.NewParser()

	// Create a tool use with very long input
	longInput := `{"type":"assistant","message":{"content":[{"type":"tool_use","name":"Write","input":{"content":"` +
		"This is a very long string that should definitely exceed 150 characters when serialized to JSON. " +
		"We need to make sure the truncation is working correctly so let's add even more text here to be absolutely certain." +
		`"}}]}}`

	msg := p.ParseLine(longInput)
	content := p.ExtractContent(msg)

	if len(content.ToolUses) != 1 {
		t.Fatalf("Expected 1 tool use, got %d", len(content.ToolUses))
	}

	if len(content.ToolUses[0].InputJSON) > 150 {
		t.Errorf("Expected InputJSON to be truncated to 150 chars, got %d", len(content.ToolUses[0].InputJSON))
	}
}

func TestToolResultPreservesFullContent(t *testing.T) {
	p := parser.NewParser()

	// Create a tool result with very long content (exceeds old 200-char limit)
	longContent := "This is a very long result that should definitely exceed 200 characters. " +
		"We need to make sure the full content is preserved without truncation so let's add even more text here. " +
		"Adding more content to make absolutely sure we exceed the old limit by a good margin."

	line := `{"type":"user","message":{"content":[{"type":"tool_result","content":"` + longContent + `"}]}}`

	msg := p.ParseLine(line)
	content := p.ExtractContent(msg)

	if len(content.ToolResults) != 1 {
		t.Fatalf("Expected 1 tool result, got %d", len(content.ToolResults))
	}

	if content.ToolResults[0].Content != longContent {
		t.Errorf("Expected full content to be preserved (len %d), got len %d", len(longContent), len(content.ToolResults[0].Content))
	}
}

func TestMultipleContentItems(t *testing.T) {
	p := parser.NewParser()

	line := `{"type":"assistant","message":{"content":[{"type":"text","text":"First message"},{"type":"text","text":"Second message"},{"type":"tool_use","name":"Bash","input":{"command":"ls"}}]}}`

	msg := p.ParseLine(line)
	content := p.ExtractContent(msg)

	if len(content.TextContent) != 2 {
		t.Errorf("Expected 2 text items, got %d", len(content.TextContent))
	}
	if len(content.ToolUses) != 1 {
		t.Errorf("Expected 1 tool use, got %d", len(content.ToolUses))
	}
}

func TestParseLinePreservesRawJSON(t *testing.T) {
	p := parser.NewParser()

	line := `{"type":"assistant","message":{"content":[]}}`
	msg := p.ParseLine(line)

	if msg.RawJSON != line {
		t.Errorf("Expected RawJSON to be preserved, got %q", msg.RawJSON)
	}
}

func TestParseLineParentToolUseIDNull(t *testing.T) {
	p := parser.NewParser()

	line := `{"type":"assistant","message":{"content":[]},"parent_tool_use_id":null}`
	msg := p.ParseLine(line)

	if msg == nil {
		t.Fatal("Expected non-nil result")
	}
	// JSON null should result in nil pointer
	if msg.ParentToolUseID != nil {
		t.Errorf("Expected nil ParentToolUseID for null value, got %q", *msg.ParentToolUseID)
	}
}

func TestParseLineParentToolUseIDPresent(t *testing.T) {
	p := parser.NewParser()

	line := `{"type":"assistant","message":{"content":[]},"parent_tool_use_id":"toolu_abc123"}`
	msg := p.ParseLine(line)

	if msg == nil {
		t.Fatal("Expected non-nil result")
	}
	if msg.ParentToolUseID == nil {
		t.Fatal("Expected non-nil ParentToolUseID")
	}
	if *msg.ParentToolUseID != "toolu_abc123" {
		t.Errorf("Expected ParentToolUseID 'toolu_abc123', got %q", *msg.ParentToolUseID)
	}
}

func TestParseLineParentToolUseIDAbsent(t *testing.T) {
	p := parser.NewParser()

	line := `{"type":"assistant","message":{"content":[]}}`
	msg := p.ParseLine(line)

	if msg == nil {
		t.Fatal("Expected non-nil result")
	}
	if msg.ParentToolUseID != nil {
		t.Errorf("Expected nil ParentToolUseID when field absent, got %q", *msg.ParentToolUseID)
	}
}

func TestIsSubagentMessage(t *testing.T) {
	p := parser.NewParser()

	tests := []struct {
		name     string
		line     string
		expected bool
	}{
		{
			"nil message",
			"",
			false,
		},
		{
			"no parent_tool_use_id",
			`{"type":"assistant","message":{"content":[]}}`,
			false,
		},
		{
			"null parent_tool_use_id",
			`{"type":"assistant","message":{"content":[]},"parent_tool_use_id":null}`,
			false,
		},
		{
			"empty parent_tool_use_id",
			`{"type":"assistant","message":{"content":[]},"parent_tool_use_id":""}`,
			false,
		},
		{
			"valid parent_tool_use_id",
			`{"type":"assistant","message":{"content":[]},"parent_tool_use_id":"toolu_abc123"}`,
			true,
		},
		{
			"result from subagent",
			`{"type":"result","total_cost_usd":0.01,"parent_tool_use_id":"toolu_xyz789"}`,
			true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			var msg *parser.ParsedMessage
			if tt.line != "" {
				msg = p.ParseLine(tt.line)
			}
			result := p.IsSubagentMessage(msg)
			if result != tt.expected {
				t.Errorf("Expected %v, got %v", tt.expected, result)
			}
		})
	}
}

func TestGetTaskToolUseIDs(t *testing.T) {
	p := parser.NewParser()

	tests := []struct {
		name     string
		line     string
		expected []string
	}{
		{
			"nil message",
			"",
			nil,
		},
		{
			"no tool uses",
			`{"type":"assistant","message":{"content":[{"type":"text","text":"hello"}]}}`,
			nil,
		},
		{
			"non-Task tool use",
			`{"type":"assistant","message":{"content":[{"type":"tool_use","id":"toolu_abc","name":"Read","input":{"file_path":"/test"}}]}}`,
			nil,
		},
		{
			"Task tool use",
			`{"type":"assistant","message":{"content":[{"type":"tool_use","id":"toolu_task1","name":"Task","input":{"prompt":"do something"}}]}}`,
			[]string{"toolu_task1"},
		},
		{
			"multiple Task tool uses",
			`{"type":"assistant","message":{"content":[{"type":"tool_use","id":"toolu_t1","name":"Task","input":{"prompt":"a"}},{"type":"tool_use","id":"toolu_t2","name":"Task","input":{"prompt":"b"}}]}}`,
			[]string{"toolu_t1", "toolu_t2"},
		},
		{
			"mixed tool uses",
			`{"type":"assistant","message":{"content":[{"type":"tool_use","id":"toolu_r1","name":"Read","input":{}},{"type":"tool_use","id":"toolu_t1","name":"Task","input":{"prompt":"a"}},{"type":"text","text":"hi"}]}}`,
			[]string{"toolu_t1"},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			var msg *parser.ParsedMessage
			if tt.line != "" {
				msg = p.ParseLine(tt.line)
			}
			result := p.GetTaskToolUseIDs(msg)
			if len(result) != len(tt.expected) {
				t.Fatalf("Expected %d IDs, got %d: %v", len(tt.expected), len(result), result)
			}
			for i, id := range result {
				if id != tt.expected[i] {
					t.Errorf("Expected ID[%d] %q, got %q", i, tt.expected[i], id)
				}
			}
		})
	}
}

func TestContentItemIDParsed(t *testing.T) {
	p := parser.NewParser()

	line := `{"type":"assistant","message":{"content":[{"type":"tool_use","id":"toolu_abc123","name":"Read","input":{"file_path":"/test"}}]}}`
	msg := p.ParseLine(line)

	if msg == nil || msg.Message == nil {
		t.Fatal("Expected non-nil parsed message")
	}
	if len(msg.Message.Content) != 1 {
		t.Fatalf("Expected 1 content item, got %d", len(msg.Message.Content))
	}
	if msg.Message.Content[0].ID != "toolu_abc123" {
		t.Errorf("Expected ID 'toolu_abc123', got %q", msg.Message.Content[0].ID)
	}
}

func TestContentItemToolUseIDParsed(t *testing.T) {
	p := parser.NewParser()

	line := `{"type":"user","message":{"content":[{"type":"tool_result","tool_use_id":"toolu_abc123","content":"result text"}]}}`
	msg := p.ParseLine(line)

	if msg == nil || msg.Message == nil {
		t.Fatal("Expected non-nil parsed message")
	}
	if len(msg.Message.Content) != 1 {
		t.Fatalf("Expected 1 content item, got %d", len(msg.Message.Content))
	}
	if msg.Message.Content[0].ToolUseID != "toolu_abc123" {
		t.Errorf("Expected ToolUseID 'toolu_abc123', got %q", msg.Message.Content[0].ToolUseID)
	}
}

func TestParseLineSessionID(t *testing.T) {
	p := parser.NewParser()

	tests := []struct {
		name     string
		line     string
		expected string
	}{
		{
			"system message with session_id",
			`{"type":"system","session_id":"sess-abc-123","subtype":"init"}`,
			"sess-abc-123",
		},
		{
			"system message without session_id",
			`{"type":"system","subtype":"init"}`,
			"",
		},
		{
			"assistant message ignores session_id",
			`{"type":"assistant","session_id":"should-be-ignored","message":{"content":[]}}`,
			"",
		},
		{
			"result message ignores session_id",
			`{"type":"result","session_id":"should-be-ignored","total_cost_usd":0.001}`,
			"",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			msg := p.ParseLine(tt.line)
			if msg == nil {
				t.Fatal("Expected non-nil result")
			}
			result := p.GetSessionID(msg)
			if result != tt.expected {
				t.Errorf("Expected session ID %q, got %q", tt.expected, result)
			}
		})
	}
}

func TestGetSessionIDNilMessage(t *testing.T) {
	p := parser.NewParser()
	result := p.GetSessionID(nil)
	if result != "" {
		t.Errorf("Expected empty string for nil message, got %q", result)
	}
}

func TestSessionIDFieldParsed(t *testing.T) {
	p := parser.NewParser()

	line := `{"type":"system","session_id":"my-session-id-12345"}`
	msg := p.ParseLine(line)

	if msg == nil {
		t.Fatal("Expected non-nil result")
	}
	if msg.SessionID != "my-session-id-12345" {
		t.Errorf("Expected SessionID 'my-session-id-12345', got %q", msg.SessionID)
	}
}

func TestExtractTaskReference(t *testing.T) {
	p := parser.NewParser()

	tests := []struct {
		name       string
		input      string
		expectNil  bool
		expectNum  int
		expectDesc string
	}{
		{
			"no task reference",
			"Just a regular message about coding",
			true, 0, "",
		},
		{
			"empty string",
			"",
			true, 0, "",
		},
		{
			"simple TASK N",
			"I will implement TASK 6 now",
			false, 6, "",
		},
		{
			"lowercase task n",
			"working on task 3 implementation",
			false, 3, "",
		},
		{
			"TASK with description",
			"## TASK 6: Track IMPLEMENTATION_PLAN.md Phase/Task",
			false, 6, "Track IMPLEMENTATION_PLAN.md Phase/Task",
		},
		{
			"TASK with description and status bracket",
			"## TASK 1: Replace Control Panel with Hotkey Bar [HIGH PRIORITY]",
			false, 1, "Replace Control Panel with Hotkey Bar",
		},
		{
			"multiple tasks picks last",
			"After completing TASK 3, I will start TASK 5",
			false, 5, "",
		},
		{
			"IMPLEMENTATION_PLAN.md content with description",
			"TASK 2: Fix Message Truncation (spec item 1)",
			false, 2, "Fix Message Truncation (spec item 1)",
		},
		{
			"task number zero ignored",
			"TASK 0 should not match",
			true, 0, "",
		},
		{
			"mixed case",
			"Let me work on Task 7 next",
			false, 7, "",
		},
		{
			"task in tool result content",
			"Read IMPLEMENTATION_PLAN.md:\n## TASK 4: Fix Visual Artifacts [MEDIUM PRIORITY]\n**Status: DONE**",
			false, 4, "Fix Visual Artifacts",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			ref := p.ExtractTaskReference(tt.input)
			if tt.expectNil {
				if ref != nil {
					t.Errorf("Expected nil, got Task %d (desc: %q)", ref.Number, ref.Description)
				}
				return
			}
			if ref == nil {
				t.Fatal("Expected non-nil TaskReference")
			}
			if ref.Number != tt.expectNum {
				t.Errorf("Expected number %d, got %d", tt.expectNum, ref.Number)
			}
			if ref.Description != tt.expectDesc {
				t.Errorf("Expected description %q, got %q", tt.expectDesc, ref.Description)
			}
		})
	}
}
