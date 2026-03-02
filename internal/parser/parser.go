package parser

import (
	"encoding/json"
	"regexp"
	"strings"
	"time"
)

// MessageType represents the type of Claude message
type MessageType string

const (
	MessageTypeSystem    MessageType = "system"
	MessageTypeAssistant MessageType = "assistant"
	MessageTypeUser      MessageType = "user"
	MessageTypeResult    MessageType = "result"
	MessageTypeRateLimit MessageType = "rate_limit_event"
	MessageTypeUnknown   MessageType = "unknown"
)

// ContentType represents the type of content within a message
type ContentType string

const (
	ContentTypeText       ContentType = "text"
	ContentTypeToolUse    ContentType = "tool_use"
	ContentTypeToolResult ContentType = "tool_result"
)

// Usage represents token usage information from Claude
type Usage struct {
	InputTokens              int64 `json:"input_tokens"`
	OutputTokens             int64 `json:"output_tokens"`
	CacheCreationInputTokens int64 `json:"cache_creation_input_tokens"`
	CacheReadInputTokens     int64 `json:"cache_read_input_tokens"`
}

// RateLimitInfo holds rate limit event data from Claude CLI
type RateLimitInfo struct {
	Status        string `json:"status"`                  // "rejected" means blocked
	ResetsAt      int64  `json:"resetsAt"`                // Unix timestamp when limit resets
	RateLimitType string `json:"rateLimitType,omitempty"` // Type of rate limit (e.g., "token")
}

// ContentItem represents a single content item in a message
type ContentItem struct {
	Type      ContentType            `json:"type"`
	Text      string                 `json:"text,omitempty"`
	ID        string                 `json:"id,omitempty"`          // Tool use ID for tool_use
	Name      string                 `json:"name,omitempty"`        // Tool name for tool_use
	Input     map[string]interface{} `json:"input,omitempty"`       // Tool input for tool_use
	ToolUseID string                 `json:"tool_use_id,omitempty"` // Tool use ID for tool_result
	Content   interface{}            `json:"content,omitempty"`     // Tool result content
}

// InnerMessage represents the message field within an assistant/user message
type InnerMessage struct {
	Content []ContentItem `json:"content"`
	Usage   *Usage        `json:"usage,omitempty"`
}

// ParsedMessage represents a parsed Claude message
type ParsedMessage struct {
	Type            MessageType    `json:"type"`
	SessionID       string         `json:"session_id,omitempty"`
	Message         *InnerMessage  `json:"message,omitempty"`
	TotalCostUSD    float64        `json:"total_cost_usd,omitempty"`
	ParentToolUseID *string        `json:"parent_tool_use_id,omitempty"`
	IsError         bool           `json:"is_error,omitempty"`
	Error           string         `json:"error,omitempty"`
	RateLimitInfo   *RateLimitInfo `json:"rate_limit_info,omitempty"`
	RawJSON         string         `json:"-"` // Original JSON for debugging
}

// LoopMarker represents a loop marker extracted from output
type LoopMarker struct {
	Current int
	Total   int
}

// ParsedContent represents the extracted content from a message
type ParsedContent struct {
	TextContent []string     // Text items
	ToolUses    []ToolUse    // Tool uses
	ToolResults []ToolResult // Tool results
	Thinking    string       // Extracted <thinking> content
}

// ToolUse represents a tool use from the assistant
type ToolUse struct {
	Name      string
	InputJSON string // Truncated JSON preview
}

// ToolResult represents a tool result from the user
type ToolResult struct {
	Content string // Truncated content
}

// TaskReference represents a detected IMPLEMENTATION_PLAN.md task reference
type TaskReference struct {
	Number      int
	Description string // Optional description (e.g., "Track IMPLEMENTATION_PLAN.md Phase/Task")
}

// Parser handles parsing of Claude's stream-json output
type Parser struct {
	systemReminderRegex *regexp.Regexp
	loopMarkerRegex     *regexp.Regexp
	thinkingRegex       *regexp.Regexp
	taskRegex           *regexp.Regexp
	taskWithDescRegex   *regexp.Regexp
}

// NewParser creates a new Parser instance
func NewParser() *Parser {
	return &Parser{
		systemReminderRegex: regexp.MustCompile(`(?s)<system-reminder>.*?</system-reminder>`),
		loopMarkerRegex:     regexp.MustCompile(`LOOP (\d+)/(\d+)`),
		thinkingRegex:       regexp.MustCompile(`(?s)<thinking>(.*?)</thinking>`),
		taskRegex:           regexp.MustCompile(`(?i)TASK\s+(\d+)`),
		taskWithDescRegex:   regexp.MustCompile(`(?i)TASK\s+(\d+)\s*:\s*([^\[\n]+)`),
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
	if msg == nil || msg.Message == nil {
		return &ParsedContent{}
	}

	content := &ParsedContent{
		TextContent: []string{},
		ToolUses:    []ToolUse{},
		ToolResults: []ToolResult{},
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
			if resultText != "" {
				content.ToolResults = append(content.ToolResults, ToolResult{
					Content: resultText,
				})
			}
		}
	}

	return content
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

// GetSessionID returns the session ID from a system message, or empty string if not present
func (p *Parser) GetSessionID(msg *ParsedMessage) string {
	if msg == nil || msg.Type != MessageTypeSystem {
		return ""
	}
	return msg.SessionID
}

// IsRateLimitRejected checks if message is a rate limit rejection.
// Returns (true, resetTime) if rejected, (false, zero) otherwise.
// Handles two patterns:
// - Pattern 1: type: "rate_limit_event" with rate_limit_info.status == "rejected"
// - Pattern 2: is_error: true with error: "rate_limit"
func (p *Parser) IsRateLimitRejected(msg *ParsedMessage) (bool, time.Time) {
	if msg == nil || msg.RateLimitInfo == nil {
		return false, time.Time{}
	}
	if msg.RateLimitInfo.Status != "rejected" {
		return false, time.Time{}
	}
	return true, time.Unix(msg.RateLimitInfo.ResetsAt, 0)
}

// IsSubagentMessage returns true if the message originates from a subagent
// (i.e., has a non-nil, non-empty parent_tool_use_id)
func (p *Parser) IsSubagentMessage(msg *ParsedMessage) bool {
	if msg == nil || msg.ParentToolUseID == nil {
		return false
	}
	return *msg.ParentToolUseID != ""
}

// GetTaskToolUseIDs returns the IDs of any "Task" tool_use content items in the message.
// These IDs correspond to subagents being spawned.
func (p *Parser) GetTaskToolUseIDs(msg *ParsedMessage) []string {
	if msg == nil || msg.Message == nil {
		return nil
	}
	var ids []string
	for _, item := range msg.Message.Content {
		if item.Type == ContentTypeToolUse && item.Name == "Task" && item.ID != "" {
			ids = append(ids, item.ID)
		}
	}
	return ids
}

// ExtractFilePathFromInput extracts a file path or pattern from a tool input map.
// Returns the first match from: file_path, path, pattern, command (truncated), description.
// Returns empty string if no relevant field is found.
func ExtractFilePathFromInput(input map[string]interface{}) string {
	// Try file_path first (Read, Write, Edit)
	if path, ok := input["file_path"].(string); ok && path != "" {
		return path
	}
	// Try path (some tools use this)
	if path, ok := input["path"].(string); ok && path != "" {
		return path
	}
	// Try pattern (Glob, Grep)
	if pattern, ok := input["pattern"].(string); ok && pattern != "" {
		return pattern
	}
	// Try command (Bash) - truncate to first 50 chars
	if cmd, ok := input["command"].(string); ok && cmd != "" {
		if len(cmd) > 50 {
			return cmd[:50] + "..."
		}
		return cmd
	}
	// Try description (Task)
	if desc, ok := input["description"].(string); ok && desc != "" {
		return desc
	}
	return ""
}

// ExtractTaskReference scans text for references to IMPLEMENTATION_PLAN.md tasks
// (e.g., "TASK 6", "Task 3: Some Description"). Returns the last match found,
// or nil if no task reference is detected.
func (p *Parser) ExtractTaskReference(text string) *TaskReference {
	// Try the more specific pattern first (TASK N: description)
	descMatches := p.taskWithDescRegex.FindAllStringSubmatch(text, -1)
	if len(descMatches) > 0 {
		last := descMatches[len(descMatches)-1]
		num := parseInt(last[1])
		if num > 0 {
			return &TaskReference{
				Number:      num,
				Description: strings.TrimSpace(last[2]),
			}
		}
	}

	// Fall back to simple pattern (TASK N)
	matches := p.taskRegex.FindAllStringSubmatch(text, -1)
	if len(matches) > 0 {
		last := matches[len(matches)-1]
		num := parseInt(last[1])
		if num > 0 {
			return &TaskReference{
				Number: num,
			}
		}
	}

	return nil
}
