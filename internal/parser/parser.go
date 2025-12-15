package parser

import (
	"encoding/json"
	"regexp"
	"strings"
)

// MessageType represents the type of Claude message
type MessageType string

const (
	MessageTypeSystem    MessageType = "system"
	MessageTypeAssistant MessageType = "assistant"
	MessageTypeUser      MessageType = "user"
	MessageTypeResult    MessageType = "result"
	MessageTypeToolCall  MessageType = "tool_call" // cursor-agent uses this for tool calls
	MessageTypeUnknown   MessageType = "unknown"
)

// ContentType represents the type of content within a message
type ContentType string

const (
	ContentTypeText      ContentType = "text"
	ContentTypeToolUse   ContentType = "tool_use"
	ContentTypeToolResult ContentType = "tool_result"
)

// Usage represents token usage information from Claude
type Usage struct {
	InputTokens              int64 `json:"input_tokens"`
	OutputTokens             int64 `json:"output_tokens"`
	CacheCreationInputTokens int64 `json:"cache_creation_input_tokens"`
	CacheReadInputTokens     int64 `json:"cache_read_input_tokens"`
}

// ContentItem represents a single content item in a message
type ContentItem struct {
	Type    ContentType            `json:"type"`
	Text    string                 `json:"text,omitempty"`
	Name    string                 `json:"name,omitempty"`         // Tool name for tool_use
	Input   map[string]interface{} `json:"input,omitempty"`        // Tool input for tool_use
	Content interface{}            `json:"content,omitempty"`      // Tool result content
}

// InnerMessage represents the message field within an assistant/user message
type InnerMessage struct {
	Content []ContentItem `json:"content"`
	Usage   *Usage        `json:"usage,omitempty"`
}

// CursorToolCall represents a cursor-agent tool call structure
type CursorToolCall struct {
	WriteToolCall *CursorWriteToolCall `json:"writeToolCall,omitempty"`
	ReadToolCall  *CursorReadToolCall  `json:"readToolCall,omitempty"`
}

// CursorWriteToolCall represents a write tool call from cursor-agent
type CursorWriteToolCall struct {
	Args   map[string]interface{} `json:"args,omitempty"`
	Result map[string]interface{} `json:"result,omitempty"`
}

// CursorReadToolCall represents a read tool call from cursor-agent
type CursorReadToolCall struct {
	Args   map[string]interface{} `json:"args,omitempty"`
	Result map[string]interface{} `json:"result,omitempty"`
}

// ParsedMessage represents a parsed Claude/cursor-agent message
type ParsedMessage struct {
	Type         MessageType     `json:"type"`
	Subtype      string          `json:"subtype,omitempty"`      // cursor-agent uses this (init, started, completed)
	Message      *InnerMessage   `json:"message,omitempty"`
	ToolCall     *CursorToolCall `json:"tool_call,omitempty"`    // cursor-agent tool calls
	Model        string          `json:"model,omitempty"`        // cursor-agent includes model in system/init
	DurationMs   int64           `json:"duration_ms,omitempty"`  // cursor-agent result duration
	TotalCostUSD float64         `json:"total_cost_usd,omitempty"`
	RawJSON      string          `json:"-"` // Original JSON for debugging
}

// LoopMarker represents a loop marker extracted from output
type LoopMarker struct {
	Current int
	Total   int
}

// ParsedContent represents the extracted content from a message
type ParsedContent struct {
	TextContent    []string      // Text items
	ToolUses       []ToolUse     // Tool uses
	ToolResults    []ToolResult  // Tool results
	Thinking       string        // Extracted <thinking> content
}

// ToolUse represents a tool use from the assistant
type ToolUse struct {
	Name       string
	InputJSON  string // Truncated JSON preview
}

// ToolResult represents a tool result from the user
type ToolResult struct {
	Content string // Truncated content
}

// Parser handles parsing of Claude's stream-json output
type Parser struct {
	systemReminderRegex *regexp.Regexp
	loopMarkerRegex     *regexp.Regexp
	thinkingRegex       *regexp.Regexp
}

// NewParser creates a new Parser instance
func NewParser() *Parser {
	return &Parser{
		systemReminderRegex: regexp.MustCompile(`(?s)<system-reminder>.*?</system-reminder>`),
		loopMarkerRegex:     regexp.MustCompile(`LOOP (\d+)/(\d+)`),
		thinkingRegex:       regexp.MustCompile(`(?s)<thinking>(.*?)</thinking>`),
	}
}

// ParseLine parses a single line of Claude output
// Returns a ParsedMessage if the line contains valid JSON, nil otherwise
func (p *Parser) ParseLine(line string) *ParsedMessage {
	line = strings.TrimSpace(line)

	// Skip empty lines and loop markers (they're not JSON)
	if line == "" || strings.HasPrefix(line, "=======") {
		return nil
	}

	// Skip non-JSON lines
	if !strings.HasPrefix(line, "{") {
		return nil
	}

	var msg ParsedMessage
	if err := json.Unmarshal([]byte(line), &msg); err != nil {
		return nil
	}

	msg.RawJSON = line
	return &msg
}

// ParseLoopMarker extracts loop marker information from a line
// Returns nil if the line is not a loop marker
func (p *Parser) ParseLoopMarker(line string) *LoopMarker {
	line = strings.TrimSpace(line)

	if !strings.HasPrefix(line, "=======") {
		return nil
	}

	matches := p.loopMarkerRegex.FindStringSubmatch(line)
	if len(matches) != 3 {
		return nil
	}

	var current, total int
	_, _ = strings.NewReader(matches[1]).Read(make([]byte, 0))

	// Parse current and total
	current = parseInt(matches[1])
	total = parseInt(matches[2])

	return &LoopMarker{
		Current: current,
		Total:   total,
	}
}

// parseInt safely parses an integer from a string, returning 0 on error
func parseInt(s string) int {
	var result int
	for _, c := range s {
		if c >= '0' && c <= '9' {
			result = result*10 + int(c-'0')
		}
	}
	return result
}

// StripSystemReminders removes <system-reminder> tags and their content from text
func (p *Parser) StripSystemReminders(text string) string {
	result := p.systemReminderRegex.ReplaceAllString(text, "")
	return strings.TrimSpace(result)
}

// ExtractThinking extracts <thinking> content from text
// Returns empty string if no thinking block is found
func (p *Parser) ExtractThinking(text string) string {
	matches := p.thinkingRegex.FindStringSubmatch(text)
	if len(matches) < 2 {
		return ""
	}
	return strings.TrimSpace(matches[1])
}

// ExtractContent processes a ParsedMessage and extracts its content
func (p *Parser) ExtractContent(msg *ParsedMessage) *ParsedContent {
	if msg == nil {
		return &ParsedContent{}
	}

	content := &ParsedContent{
		TextContent:  []string{},
		ToolUses:     []ToolUse{},
		ToolResults:  []ToolResult{},
	}

	// Handle cursor-agent tool_call messages
	if msg.Type == MessageTypeToolCall && msg.ToolCall != nil {
		toolUse := p.extractCursorToolCall(msg)
		if toolUse != nil {
			content.ToolUses = append(content.ToolUses, *toolUse)
		}
		return content
	}

	// Handle standard Claude messages with content array
	if msg.Message == nil {
		return content
	}

	for _, item := range msg.Message.Content {
		switch item.Type {
		case ContentTypeText:
			text := p.StripSystemReminders(item.Text)
			if text != "" {
				// Check for thinking
				thinking := p.ExtractThinking(text)
				if thinking != "" {
					content.Thinking = thinking
				} else {
					content.TextContent = append(content.TextContent, text)
				}
			}

		case ContentTypeToolUse:
			inputJSON := "{}"
			if item.Input != nil {
				if jsonBytes, err := json.MarshalIndent(item.Input, "", "  "); err == nil {
					inputJSON = string(jsonBytes)
				}
			}
			// Truncate to 150 characters like Python version
			if len(inputJSON) > 150 {
				inputJSON = inputJSON[:150]
			}
			content.ToolUses = append(content.ToolUses, ToolUse{
				Name:      item.Name,
				InputJSON: inputJSON,
			})

		case ContentTypeToolResult:
			resultText := ""
			switch v := item.Content.(type) {
			case string:
				resultText = v
			case map[string]interface{}:
				if jsonBytes, err := json.Marshal(v); err == nil {
					resultText = string(jsonBytes)
				}
			}
			resultText = p.StripSystemReminders(resultText)
			// Truncate to 200 characters like Python version
			if len(resultText) > 200 {
				resultText = resultText[:200]
			}
			if resultText != "" {
				content.ToolResults = append(content.ToolResults, ToolResult{
					Content: resultText,
				})
			}
		}
	}

	return content
}

// extractCursorToolCall extracts tool information from a cursor-agent tool_call message
func (p *Parser) extractCursorToolCall(msg *ParsedMessage) *ToolUse {
	if msg.ToolCall == nil {
		return nil
	}

	// Only report on "started" subtype to avoid duplicates
	if msg.Subtype != "started" {
		return nil
	}

	var toolName string
	var inputJSON string

	if msg.ToolCall.WriteToolCall != nil {
		toolName = "write"
		if path, ok := msg.ToolCall.WriteToolCall.Args["path"].(string); ok {
			toolName = "write: " + path
		}
		if jsonBytes, err := json.MarshalIndent(msg.ToolCall.WriteToolCall.Args, "", "  "); err == nil {
			inputJSON = string(jsonBytes)
		}
	} else if msg.ToolCall.ReadToolCall != nil {
		toolName = "read"
		if path, ok := msg.ToolCall.ReadToolCall.Args["path"].(string); ok {
			toolName = "read: " + path
		}
		if jsonBytes, err := json.MarshalIndent(msg.ToolCall.ReadToolCall.Args, "", "  "); err == nil {
			inputJSON = string(jsonBytes)
		}
	}

	if toolName == "" {
		return nil
	}

	// Truncate to 150 characters
	if len(inputJSON) > 150 {
		inputJSON = inputJSON[:150]
	}

	return &ToolUse{
		Name:      toolName,
		InputJSON: inputJSON,
	}
}

// GetMessageType returns the type of a parsed message
func (p *Parser) GetMessageType(msg *ParsedMessage) MessageType {
	if msg == nil {
		return MessageTypeUnknown
	}
	return msg.Type
}

// GetUsage extracts usage information from a message
// Returns nil if no usage information is present
func (p *Parser) GetUsage(msg *ParsedMessage) *Usage {
	if msg == nil || msg.Message == nil || msg.Message.Usage == nil {
		return nil
	}
	return msg.Message.Usage
}

// GetCost extracts the total cost from a result message
// Returns 0 if no cost information is present
func (p *Parser) GetCost(msg *ParsedMessage) float64 {
	if msg == nil || msg.Type != MessageTypeResult {
		return 0
	}
	return msg.TotalCostUSD
}
