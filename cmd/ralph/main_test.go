package main

import (
	"context"
	"flag"
	"io"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/cloudosai/ralph-go/internal/config"
	"github.com/cloudosai/ralph-go/internal/loop"
	"github.com/cloudosai/ralph-go/internal/parser"
	"github.com/cloudosai/ralph-go/internal/stats"
)

func TestParseTaskCountsNoFile(t *testing.T) {
	completed, total := parseTaskCounts("/nonexistent/path/IMPLEMENTATION_PLAN.md")
	if completed != 0 || total != 0 {
		t.Errorf("expected (0, 0) for missing file, got (%d, %d)", completed, total)
	}
}

func TestParseTaskCountsEmptyFile(t *testing.T) {
	tmpDir := t.TempDir()
	path := filepath.Join(tmpDir, "IMPLEMENTATION_PLAN.md")
	os.WriteFile(path, []byte(""), 0644)

	completed, total := parseTaskCounts(path)
	if completed != 0 || total != 0 {
		t.Errorf("expected (0, 0) for empty file, got (%d, %d)", completed, total)
	}
}

func TestParseTaskCountsSingleTaskTodo(t *testing.T) {
	tmpDir := t.TempDir()
	path := filepath.Join(tmpDir, "IMPLEMENTATION_PLAN.md")
	content := `# Implementation Plan

## TASK 1: Add feature X
**Priority: HIGH**
**Status: TODO**
`
	os.WriteFile(path, []byte(content), 0644)

	completed, total := parseTaskCounts(path)
	if total != 1 {
		t.Errorf("expected total=1, got %d", total)
	}
	if completed != 0 {
		t.Errorf("expected completed=0, got %d", completed)
	}
}

func TestParseTaskCountsSingleTaskDone(t *testing.T) {
	tmpDir := t.TempDir()
	path := filepath.Join(tmpDir, "IMPLEMENTATION_PLAN.md")
	content := `# Implementation Plan

## TASK 1: Add feature X
**Priority: HIGH**
**Status: DONE**
`
	os.WriteFile(path, []byte(content), 0644)

	completed, total := parseTaskCounts(path)
	if total != 1 {
		t.Errorf("expected total=1, got %d", total)
	}
	if completed != 1 {
		t.Errorf("expected completed=1, got %d", completed)
	}
}

func TestParseTaskCountsNotNeeded(t *testing.T) {
	tmpDir := t.TempDir()
	path := filepath.Join(tmpDir, "IMPLEMENTATION_PLAN.md")
	content := `# Implementation Plan

## TASK 1: Deprecated feature
**Status: NOT NEEDED**
`
	os.WriteFile(path, []byte(content), 0644)

	completed, total := parseTaskCounts(path)
	if total != 1 {
		t.Errorf("expected total=1, got %d", total)
	}
	if completed != 1 {
		t.Errorf("expected completed=1, got %d", completed)
	}
}

func TestParseTaskCountsMultipleTasks(t *testing.T) {
	tmpDir := t.TempDir()
	path := filepath.Join(tmpDir, "IMPLEMENTATION_PLAN.md")
	content := `# Implementation Plan

## TASK 1: First feature
**Status: DONE**

## TASK 2: Second feature
**Status: IN PROGRESS**

## TASK 3: Third feature
**Status: TODO**

## TASK 4: Fourth feature
**Status: DONE**

## TASK 5: Fifth feature
**Status: NOT NEEDED**
`
	os.WriteFile(path, []byte(content), 0644)

	completed, total := parseTaskCounts(path)
	if total != 5 {
		t.Errorf("expected total=5, got %d", total)
	}
	if completed != 3 {
		t.Errorf("expected completed=3 (2 DONE + 1 NOT NEEDED), got %d", completed)
	}
}

func TestParseTaskCountsNoTaskHeaders(t *testing.T) {
	tmpDir := t.TempDir()
	path := filepath.Join(tmpDir, "IMPLEMENTATION_PLAN.md")
	content := `# Implementation Plan

Some general notes about the project.

**Status: DONE**
`
	os.WriteFile(path, []byte(content), 0644)

	completed, total := parseTaskCounts(path)
	if total != 0 {
		t.Errorf("expected total=0 (no ## TASK headers), got %d", total)
	}
	// Status line without a TASK header still counts as completed
	if completed != 1 {
		t.Errorf("expected completed=1 (status line exists), got %d", completed)
	}
}

func TestParseTaskCountsStatusOnSameLineAsHeader(t *testing.T) {
	tmpDir := t.TempDir()
	path := filepath.Join(tmpDir, "IMPLEMENTATION_PLAN.md")
	content := `## TASK 1: Feature — **Status: DONE**
## TASK 2: Other feature
**Status: TODO**
`
	os.WriteFile(path, []byte(content), 0644)

	completed, total := parseTaskCounts(path)
	if total != 2 {
		t.Errorf("expected total=2, got %d", total)
	}
	if completed != 1 {
		t.Errorf("expected completed=1, got %d", completed)
	}
}

func TestParseTaskCountsReflectsFileChanges(t *testing.T) {
	// Verifies that repeated calls to parseTaskCounts pick up file modifications,
	// which is the basis for live recount at iteration boundaries.
	tmpDir := t.TempDir()
	path := filepath.Join(tmpDir, "IMPLEMENTATION_PLAN.md")

	// Initial: 1 done out of 3
	initial := "## TASK 1\n**Status: DONE**\n## TASK 2\n**Status: TODO**\n## TASK 3\n**Status: TODO**\n"
	os.WriteFile(path, []byte(initial), 0644)

	completed, total := parseTaskCounts(path)
	if completed != 1 || total != 3 {
		t.Errorf("initial: expected 1/3, got %d/%d", completed, total)
	}

	// Simulate Claude marking task 2 as DONE during an iteration
	updated := "## TASK 1\n**Status: DONE**\n## TASK 2\n**Status: DONE**\n## TASK 3\n**Status: TODO**\n"
	os.WriteFile(path, []byte(updated), 0644)

	completed, total = parseTaskCounts(path)
	if completed != 2 || total != 3 {
		t.Errorf("after update: expected 2/3, got %d/%d", completed, total)
	}

	// Simulate adding a new task during the session
	expanded := updated + "## TASK 4\n**Status: TODO**\n"
	os.WriteFile(path, []byte(expanded), 0644)

	completed, total = parseTaskCounts(path)
	if completed != 2 || total != 4 {
		t.Errorf("after expansion: expected 2/4, got %d/%d", completed, total)
	}
}

func TestCheckCostPacingDisabled(t *testing.T) {
	// maxCostPerHour=0 means disabled — should be a no-op
	exceeded, hourCost, nextHour := checkCostPacing(&dbContext{}, 0, nil)
	if exceeded {
		t.Error("expected exceeded=false when maxCostPerHour=0")
	}
	if hourCost != 0 {
		t.Errorf("expected hourCost=0, got %f", hourCost)
	}
	if !nextHour.IsZero() {
		t.Errorf("expected zero nextHour, got %v", nextHour)
	}

	// Negative value also means disabled
	exceeded, _, _ = checkCostPacing(&dbContext{}, -1.0, nil)
	if exceeded {
		t.Error("expected exceeded=false when maxCostPerHour<0")
	}
}

func TestCheckCostPacingNilDB(t *testing.T) {
	// dbCtx with nil db — should be a no-op
	exceeded, hourCost, nextHour := checkCostPacing(&dbContext{db: nil}, 1.0, nil)
	if exceeded {
		t.Error("expected exceeded=false when db is nil")
	}
	if hourCost != 0 {
		t.Errorf("expected hourCost=0, got %f", hourCost)
	}
	if !nextHour.IsZero() {
		t.Errorf("expected zero nextHour, got %v", nextHour)
	}

	// Nil dbCtx entirely
	exceeded, _, _ = checkCostPacing(nil, 1.0, nil)
	if exceeded {
		t.Error("expected exceeded=false when dbCtx is nil")
	}
}

func TestMaxCostPerHourFlag(t *testing.T) {
	// Save and restore global state
	origArgs := os.Args
	origCommandLine := flag.CommandLine
	defer func() {
		os.Args = origArgs
		flag.CommandLine = origCommandLine
	}()

	// Reset flag state so ParseFlags can register new flags
	flag.CommandLine = flag.NewFlagSet(os.Args[0], flag.ContinueOnError)
	os.Args = []string{"ralph", "--max-cost-per-hour=1.50", "--show-prompt"}

	cfg := config.ParseFlags()
	if cfg.MaxCostPerHour != 1.50 {
		t.Errorf("expected MaxCostPerHour=1.50, got %f", cfg.MaxCostPerHour)
	}

	// Test default (0 = no limit)
	flag.CommandLine = flag.NewFlagSet(os.Args[0], flag.ContinueOnError)
	os.Args = []string{"ralph", "--show-prompt"}

	cfg = config.ParseFlags()
	if cfg.MaxCostPerHour != 0 {
		t.Errorf("expected MaxCostPerHour=0 (default), got %f", cfg.MaxCostPerHour)
	}
}

func TestHandleLoopMarkerReturnsTrue(t *testing.T) {
	// Verify the isLoopStart detection logic matches what handleLoopMarker uses
	tests := []struct {
		content  string
		expected bool
	}{
		{"======= LOOP 1/5 =======", true},
		{"======= LOOP 2/5 =======", true},
		{"======= STOPPED =======", false},
		{"======= COMPLETED 5 ITERATIONS =======", false},
		{"======= RESUMED =======", false},
		{"======= LOOP 1/5 (RETRY) =======", false},
		{"======= LOOP 3/5 (RETRY) =======", false},
	}

	for _, tt := range tests {
		isLoopStart := isNewLoopStart(tt.content)
		if isLoopStart != tt.expected {
			t.Errorf("isNewLoopStart(%q) = %v, want %v", tt.content, isLoopStart, tt.expected)
		}
	}
}

func TestIsRetryLoopStart(t *testing.T) {
	tests := []struct {
		content  string
		expected bool
	}{
		{"======= LOOP 1/5 (RETRY) =======", true},
		{"======= LOOP 3/5 (RETRY) =======", true},
		{"======= LOOP 1/5 =======", false},
		{"======= STOPPED =======", false},
		{"======= COMPLETED 5 ITERATIONS =======", false},
		{"======= RESUMED =======", false},
	}

	for _, tt := range tests {
		result := isRetryLoopStart(tt.content)
		if result != tt.expected {
			t.Errorf("isRetryLoopStart(%q) = %v, want %v", tt.content, result, tt.expected)
		}
	}
}

// makeNoopResult creates a low-cost main-agent result message (no tool use, cost below threshold)
func makeNoopResult(cost float64) *parser.ParsedMessage {
	return &parser.ParsedMessage{
		Type:         parser.MessageTypeResult,
		TotalCostUSD: cost,
	}
}

// makeProductiveResult creates a main-agent result with tool use in the preceding assistant message
func makeAssistantWithToolUse() *parser.ParsedMessage {
	return &parser.ParsedMessage{
		Type: parser.MessageTypeAssistant,
		Message: &parser.InnerMessage{
			Content: []parser.ContentItem{
				{Type: parser.ContentTypeToolUse, Name: "Read", ID: "tool_1"},
			},
		},
	}
}

func TestExitLoopDetection_ConsecutiveNoops(t *testing.T) {
	// Two consecutive no-op iterations should trigger loop stop
	claudeLoop := loop.New(loop.Config{Iterations: 5, Prompt: "test"})
	jsonParser := parser.NewParser()
	tokenStats := stats.NewTokenStats()
	apiBackoff := loop.NewBackoff()

	var iterEstimate float64
	var subagentCostAccum float64
	var lastResultCost float64
	var iterToolUseCount int
	var noopStreak int

	// First no-op iteration result
	handleParsedMessageCLI(
		makeNoopResult(0.005), claudeLoop, jsonParser, tokenStats, io.Discard,
		&iterEstimate, &subagentCostAccum, &lastResultCost, &iterToolUseCount, &noopStreak, apiBackoff, make(map[string]bool),
	)

	if noopStreak != 1 {
		t.Errorf("expected noopStreak=1 after first no-op, got %d", noopStreak)
	}

	// Simulate new loop start — reset iterToolUseCount but not noopStreak
	iterToolUseCount = 0
	iterEstimate = 0
	subagentCostAccum = 0

	// Second no-op iteration result — should trigger stop
	handleParsedMessageCLI(
		makeNoopResult(0.003), claudeLoop, jsonParser, tokenStats, io.Discard,
		&iterEstimate, &subagentCostAccum, &lastResultCost, &iterToolUseCount, &noopStreak, apiBackoff, make(map[string]bool),
	)

	if noopStreak != 2 {
		t.Errorf("expected noopStreak=2, got %d", noopStreak)
	}

	// The loop should have been stopped
	if claudeLoop.IsRunning() {
		t.Error("expected loop to be stopped after 2 consecutive no-op iterations")
	}
}

func TestExitLoopDetection_ProductiveIterationResetsStreak(t *testing.T) {
	// A productive iteration (with tool use) should reset the noop streak
	claudeLoop := loop.New(loop.Config{Iterations: 5, Prompt: "test"})
	jsonParser := parser.NewParser()
	tokenStats := stats.NewTokenStats()
	apiBackoff := loop.NewBackoff()

	var iterEstimate float64
	var subagentCostAccum float64
	var lastResultCost float64
	var iterToolUseCount int
	var noopStreak int

	// First no-op iteration
	handleParsedMessageCLI(
		makeNoopResult(0.005), claudeLoop, jsonParser, tokenStats, io.Discard,
		&iterEstimate, &subagentCostAccum, &lastResultCost, &iterToolUseCount, &noopStreak, apiBackoff, make(map[string]bool),
	)
	if noopStreak != 1 {
		t.Fatalf("expected noopStreak=1, got %d", noopStreak)
	}

	// Simulate new loop start
	iterToolUseCount = 0
	iterEstimate = 0
	subagentCostAccum = 0

	// Productive iteration: assistant message with tool use, then result with higher cost
	handleParsedMessageCLI(
		makeAssistantWithToolUse(), claudeLoop, jsonParser, tokenStats, io.Discard,
		&iterEstimate, &subagentCostAccum, &lastResultCost, &iterToolUseCount, &noopStreak, apiBackoff, make(map[string]bool),
	)

	handleParsedMessageCLI(
		makeNoopResult(0.50), claudeLoop, jsonParser, tokenStats, io.Discard,
		&iterEstimate, &subagentCostAccum, &lastResultCost, &iterToolUseCount, &noopStreak, apiBackoff, make(map[string]bool),
	)

	if noopStreak != 0 {
		t.Errorf("expected noopStreak reset to 0 after productive iteration, got %d", noopStreak)
	}
}

func TestExitLoopDetection_HighCostNoToolsIsNotNoop(t *testing.T) {
	// A high-cost iteration with no tool use (e.g., planning/thinking) should NOT be a noop
	claudeLoop := loop.New(loop.Config{Iterations: 5, Prompt: "test"})
	jsonParser := parser.NewParser()
	tokenStats := stats.NewTokenStats()
	apiBackoff := loop.NewBackoff()

	var iterEstimate float64
	var subagentCostAccum float64
	var lastResultCost float64
	var iterToolUseCount int
	var noopStreak int

	// High cost result with no tool use — this is legitimate thinking work
	handleParsedMessageCLI(
		makeNoopResult(0.50), claudeLoop, jsonParser, tokenStats, io.Discard,
		&iterEstimate, &subagentCostAccum, &lastResultCost, &iterToolUseCount, &noopStreak, apiBackoff, make(map[string]bool),
	)

	if noopStreak != 0 {
		t.Errorf("expected noopStreak=0 for high-cost iteration, got %d", noopStreak)
	}
}

func TestExitLoopDetection_SubagentResultIgnored(t *testing.T) {
	// Subagent result messages should not affect noop detection
	claudeLoop := loop.New(loop.Config{Iterations: 5, Prompt: "test"})
	jsonParser := parser.NewParser()
	tokenStats := stats.NewTokenStats()
	apiBackoff := loop.NewBackoff()

	var iterEstimate float64
	var subagentCostAccum float64
	var lastResultCost float64
	var iterToolUseCount int
	var noopStreak int

	parentID := "parent-123"
	subagentResult := &parser.ParsedMessage{
		Type:            parser.MessageTypeResult,
		TotalCostUSD:    0.001,
		ParentToolUseID: &parentID,
	}

	handleParsedMessageCLI(
		subagentResult, claudeLoop, jsonParser, tokenStats, io.Discard,
		&iterEstimate, &subagentCostAccum, &lastResultCost, &iterToolUseCount, &noopStreak, apiBackoff, make(map[string]bool),
	)

	if noopStreak != 0 {
		t.Errorf("expected noopStreak=0 after subagent result, got %d", noopStreak)
	}
}

func TestIsAuthenticationText(t *testing.T) {
	tests := []struct {
		text     string
		expected bool
	}{
		{"Please run `claude /login` to authenticate", true},
		{"Not authenticated. Run claude /login.", true},
		{"authentication error occurred", true},
		{"invalid api key provided", true},
		{"AUTHENTICATION_ERROR", true},
		{"rate limit exceeded", false},
		{"API overloaded", false},
		{"normal output text", false},
		{"", false},
	}
	for _, tt := range tests {
		t.Run(tt.text, func(t *testing.T) {
			if got := isAuthenticationText(tt.text); got != tt.expected {
				t.Errorf("isAuthenticationText(%q) = %v, want %v", tt.text, got, tt.expected)
			}
		})
	}
}

func TestHandleParsedMessageCLI_AuthError_StopsLoop(t *testing.T) {
	jsonParser := parser.NewParser()
	tokenStats := stats.NewTokenStats()
	claudeLoop := loop.New(loop.Config{Iterations: 5, Prompt: "test"})
	apiBackoff := loop.NewBackoff()
	var iterEstimate, subagentCostAccum, lastResultCost float64
	var iterToolUseCount, noopStreak int

	line := `{"type":"assistant","is_error":true,"error":"authentication_error"}`
	parsed := jsonParser.ParseLine(line)
	if parsed == nil {
		t.Fatal("Expected non-nil parsed message")
	}

	handleParsedMessageCLI(
		parsed, claudeLoop, jsonParser, tokenStats, io.Discard,
		&iterEstimate, &subagentCostAccum, &lastResultCost, &iterToolUseCount, &noopStreak, apiBackoff, make(map[string]bool),
	)

	if claudeLoop.IsRunning() {
		t.Error("Expected loop to not be running after authentication error")
	}
}

func TestDefaultCommandBuilder_InheritsEnvironment(t *testing.T) {
	// When ANTHROPIC_API_KEY is set in the parent process, the subprocess
	// should inherit it. Go's exec.Cmd inherits the full parent environment
	// when Cmd.Env is nil, so we verify that DefaultCommandBuilder does NOT
	// set Cmd.Env (which would filter the environment).
	ctx := context.Background()
	cmd := loop.DefaultCommandBuilder(ctx, "test prompt")

	if cmd.Env != nil {
		t.Error("expected Cmd.Env to be nil (inherit parent environment), but it was explicitly set")
	}
}

func TestDefaultCommandBuilder_CommandStructure(t *testing.T) {
	// Verify the CLI command is constructed with the expected flags
	ctx := context.Background()
	cmd := loop.DefaultCommandBuilder(ctx, "test prompt")

	// The command should be "claude"
	if cmd.Path == "" {
		// Path may not be resolved if claude isn't installed, but Args[0] should be set
	}
	if len(cmd.Args) < 1 || cmd.Args[0] != "claude" {
		t.Errorf("expected Args[0]='claude', got %v", cmd.Args)
	}

	// Verify expected flags are present
	expectedFlags := []string{"--print", "--output-format", "stream-json", "--dangerously-skip-permissions", "--verbose"}
	args := cmd.Args[1:] // skip the command name
	for _, flag := range expectedFlags {
		found := false
		for _, arg := range args {
			if arg == flag {
				found = true
				break
			}
		}
		if !found {
			t.Errorf("expected flag %q not found in command args: %v", flag, cmd.Args)
		}
	}
}

func TestHandleParsedMessageCLI_AuthError_WithAPIKey(t *testing.T) {
	// When ANTHROPIC_API_KEY is set and an auth error occurs, the error message
	// should indicate the key is invalid rather than asking to set it.
	t.Setenv("ANTHROPIC_API_KEY", "sk-test-invalid-key")

	jsonParser := parser.NewParser()
	tokenStats := stats.NewTokenStats()
	claudeLoop := loop.New(loop.Config{Iterations: 5, Prompt: "test"})
	apiBackoff := loop.NewBackoff()
	var iterEstimate, subagentCostAccum, lastResultCost float64
	var iterToolUseCount, noopStreak int

	line := `{"type":"assistant","is_error":true,"error":"authentication_error"}`
	parsed := jsonParser.ParseLine(line)
	if parsed == nil {
		t.Fatal("Expected non-nil parsed message")
	}

	handleParsedMessageCLI(
		parsed, claudeLoop, jsonParser, tokenStats, io.Discard,
		&iterEstimate, &subagentCostAccum, &lastResultCost, &iterToolUseCount, &noopStreak, apiBackoff, make(map[string]bool),
	)

	if claudeLoop.IsRunning() {
		t.Error("Expected loop to be stopped after authentication error")
	}
}

func TestHandleParsedMessageCLI_AuthError_WithoutAPIKey(t *testing.T) {
	// When ANTHROPIC_API_KEY is NOT set and an auth error occurs, the loop should stop.
	t.Setenv("ANTHROPIC_API_KEY", "")

	jsonParser := parser.NewParser()
	tokenStats := stats.NewTokenStats()
	claudeLoop := loop.New(loop.Config{Iterations: 5, Prompt: "test"})
	apiBackoff := loop.NewBackoff()
	var iterEstimate, subagentCostAccum, lastResultCost float64
	var iterToolUseCount, noopStreak int

	line := `{"type":"assistant","is_error":true,"error":"authentication_error"}`
	parsed := jsonParser.ParseLine(line)
	if parsed == nil {
		t.Fatal("Expected non-nil parsed message")
	}

	handleParsedMessageCLI(
		parsed, claudeLoop, jsonParser, tokenStats, io.Discard,
		&iterEstimate, &subagentCostAccum, &lastResultCost, &iterToolUseCount, &noopStreak, apiBackoff, make(map[string]bool),
	)

	if claudeLoop.IsRunning() {
		t.Error("Expected loop to be stopped after authentication error")
	}
}

func TestExitLoopDetection_ThresholdConstant(t *testing.T) {
	if NoopIterationThreshold < 1 {
		t.Error("NoopIterationThreshold must be at least 1")
	}
	if noopCostThreshold <= 0 {
		t.Error("noopCostThreshold must be positive")
	}
}

// TestAPIBackoffNotResetOnHibernateRetry verifies that when a RETRY loop marker
// arrives after a 529 hit, apiBackoff is NOT reset — the consecutive hit counter
// should continue escalating across the retry cycle.
func TestAPIBackoffNotResetOnHibernateRetry(t *testing.T) {
	apiBackoff := loop.NewBackoff()

	// Step 1: fresh loop start → reset backoff
	freshContent := "======= LOOP 1/5 ======="
	if !isNewLoopStart(freshContent) {
		t.Fatal("expected isNewLoopStart to be true for fresh loop")
	}
	apiBackoff.Reset()
	if apiBackoff.ConsecutiveHits() != 0 {
		t.Fatalf("expected consecutiveHits=0 after reset, got %d", apiBackoff.ConsecutiveHits())
	}

	// Step 2: 529 hit → backoff.Next() increments counter
	_, retryNum, exceeded := apiBackoff.Next()
	if exceeded {
		t.Fatal("did not expect exceeded on first hit")
	}
	if retryNum != 1 {
		t.Fatalf("expected retryNum=1, got %d", retryNum)
	}
	if apiBackoff.ConsecutiveHits() != 1 {
		t.Fatalf("expected consecutiveHits=1 after Next(), got %d", apiBackoff.ConsecutiveHits())
	}

	// Step 3: RETRY loop marker arrives — isNewLoopStart should return false,
	// so apiBackoff.Reset() should NOT be called
	retryContent := "======= LOOP 1/5 (RETRY) ======="
	if isNewLoopStart(retryContent) {
		t.Fatal("expected isNewLoopStart to be false for RETRY marker")
	}
	if !isRetryLoopStart(retryContent) {
		t.Fatal("expected isRetryLoopStart to be true for RETRY marker")
	}
	// Simulate what the real code does: only reset on isNewLoopStart
	if isNewLoopStart(retryContent) {
		apiBackoff.Reset() // should NOT execute
	}

	// Step 4: verify backoff was NOT reset — consecutive hits still 1
	if apiBackoff.ConsecutiveHits() != 1 {
		t.Errorf("expected consecutiveHits=1 (not reset on RETRY), got %d", apiBackoff.ConsecutiveHits())
	}
}

func TestLogFileAppendsAcrossRuns(t *testing.T) {
	tmpDir := t.TempDir()
	logPath := filepath.Join(tmpDir, "ralph.log")

	// First "run": open with append flags, write session 1, close
	f1, err := os.OpenFile(logPath, os.O_APPEND|os.O_CREATE|os.O_WRONLY, 0644)
	if err != nil {
		t.Fatalf("first open failed: %v", err)
	}
	if _, err := f1.WriteString("session 1\n"); err != nil {
		t.Fatalf("first write failed: %v", err)
	}
	f1.Close()

	// Second "run": open with append flags, write session 2, close
	f2, err := os.OpenFile(logPath, os.O_APPEND|os.O_CREATE|os.O_WRONLY, 0644)
	if err != nil {
		t.Fatalf("second open failed: %v", err)
	}
	if _, err := f2.WriteString("session 2\n"); err != nil {
		t.Fatalf("second write failed: %v", err)
	}
	f2.Close()

	// Read the file and assert both sessions are present
	data, err := os.ReadFile(logPath)
	if err != nil {
		t.Fatalf("read failed: %v", err)
	}
	content := string(data)
	if !strings.Contains(content, "session 1") {
		t.Errorf("expected 'session 1' in log file, got: %q", content)
	}
	if !strings.Contains(content, "session 2") {
		t.Errorf("expected 'session 2' in log file, got: %q", content)
	}
}

// TestStartNewLoopNotCalledOnHibernateRetry verifies that when a RETRY loop marker
// arrives, the code path that calls startNewLoop() is NOT reached. We verify this by
// checking that isNewLoopStart returns false for RETRY content, which is the gate
// that prevents startNewLoop from being called in handleLoopMarker and runCLI.
func TestStartNewLoopNotCalledOnHibernateRetry(t *testing.T) {
	// Track whether startNewLoop would be called via the isNewLoopStart gate
	startNewLoopCallCount := 0

	// Simulate a fresh loop start — isNewLoopStart returns true → startNewLoop would be called
	freshContent := "======= LOOP 1/5 ======="
	if isNewLoopStart(freshContent) {
		startNewLoopCallCount++
	}
	if startNewLoopCallCount != 1 {
		t.Fatalf("expected startNewLoop to be called once for fresh loop, got %d", startNewLoopCallCount)
	}

	// Simulate a RETRY marker — isNewLoopStart returns false → startNewLoop NOT called
	retryContent := "======= LOOP 1/5 (RETRY) ======="
	if isNewLoopStart(retryContent) {
		startNewLoopCallCount++
	}

	if startNewLoopCallCount != 1 {
		t.Errorf("expected startNewLoop call count to remain 1 after RETRY marker, got %d", startNewLoopCallCount)
	}

	// Verify isRetryLoopStart returns true — the retry path is taken instead
	if !isRetryLoopStart(retryContent) {
		t.Error("expected isRetryLoopStart to be true for RETRY marker")
	}

	// A subsequent fresh loop start SHOULD increment the counter again
	freshContent2 := "======= LOOP 2/5 ======="
	if isNewLoopStart(freshContent2) {
		startNewLoopCallCount++
	}
	if startNewLoopCallCount != 2 {
		t.Errorf("expected startNewLoop call count=2 after second fresh loop, got %d", startNewLoopCallCount)
	}
}
