package tests

import (
	"context"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"sync"
	"testing"
	"time"

	"github.com/cloudosai/ralph-go/internal/loop"
)

// mockCommandBuilder creates a command that uses our test helper process
func mockCommandBuilder(ctx context.Context, prompt string) *exec.Cmd {
	// Use the test binary as a mock command
	cmd := exec.CommandContext(ctx, os.Args[0], "-test.run=TestHelperProcess", "--", "claude")
	cmd.Env = append(os.Environ(), "GO_WANT_HELPER_PROCESS=1")
	return cmd
}

// mockErrorCommandBuilder creates a command that returns an error
func mockErrorCommandBuilder(ctx context.Context, prompt string) *exec.Cmd {
	cmd := exec.CommandContext(ctx, os.Args[0], "-test.run=TestHelperProcess", "--", "claude-error")
	cmd.Env = append(os.Environ(), "GO_WANT_HELPER_PROCESS=1")
	return cmd
}

// TestHelperProcess is a helper that allows tests to mock external commands.
// It's invoked by exec.Command when GO_WANT_HELPER_PROCESS=1 is set.
func TestHelperProcess(t *testing.T) {
	if os.Getenv("GO_WANT_HELPER_PROCESS") != "1" {
		return
	}
	defer os.Exit(0)

	args := os.Args
	for len(args) > 0 {
		if args[0] == "--" {
			args = args[1:]
			break
		}
		args = args[1:]
	}
	if len(args) == 0 {
		os.Exit(1)
	}

	cmd := args[0]
	// Check for --resume flag in remaining args
	hasResume := false
	resumeSessionID := ""
	for i, a := range args[1:] {
		if a == "--resume" && i+2 < len(args) {
			hasResume = true
			resumeSessionID = args[i+2]
		}
	}

	switch cmd {
	case "claude":
		// Read from stdin to simulate reading the prompt
		// Output different session_id depending on whether --resume was used
		if hasResume {
			os.Stdout.WriteString(`{"type":"system","session_id":"` + resumeSessionID + `","subtype":"init"}` + "\n")
		} else {
			os.Stdout.WriteString(`{"type":"system","session_id":"fresh-session-001","subtype":"init"}` + "\n")
		}
		os.Stdout.WriteString(`{"type":"assistant","message":{"content":[{"type":"text","text":"test assistant message"}]}}` + "\n")
		os.Stdout.WriteString(`{"type":"result","total_cost_usd":0.001,"usage":{"input_tokens":100,"output_tokens":50}}` + "\n")
	case "claude-slow":
		// Simulate a slow command
		time.Sleep(2 * time.Second)
		os.Stdout.WriteString(`{"type":"result","content":"done"}` + "\n")
	case "claude-error":
		os.Stderr.WriteString("Error: something went wrong\n")
		os.Exit(1)
	case "echo":
		// Simple echo command for basic testing
		os.Stdout.WriteString(strings.Join(args[1:], " ") + "\n")
	}
}

func TestNewLoop(t *testing.T) {
	cfg := loop.Config{
		Iterations: 5,
		Prompt:     "test prompt",
	}

	l := loop.New(cfg)
	if l == nil {
		t.Fatal("New() returned nil")
	}

	if l.IsRunning() {
		t.Error("New loop should not be running initially")
	}
}

func TestLoopOutput(t *testing.T) {
	cfg := loop.Config{
		Iterations: 1,
		Prompt:     "test",
	}

	l := loop.New(cfg)
	output := l.Output()

	if output == nil {
		t.Error("Output() returned nil channel")
	}
}

func TestLoopStartAndStop(t *testing.T) {
	cfg := loop.Config{
		Iterations:     100, // Many iterations so we can test stopping
		Prompt:         "test prompt",
		CommandBuilder: mockCommandBuilder,
		SleepDuration:  10 * time.Millisecond,
	}

	l := loop.New(cfg)
	ctx := context.Background()

	l.Start(ctx)

	if !l.IsRunning() {
		t.Error("Loop should be running after Start()")
	}

	// Give it a moment to start
	time.Sleep(50 * time.Millisecond)

	l.Stop()

	// Give it a moment to stop
	time.Sleep(100 * time.Millisecond)

	if l.IsRunning() {
		t.Error("Loop should not be running after Stop()")
	}
}

func TestLoopContextCancellation(t *testing.T) {
	cfg := loop.Config{
		Iterations:     100, // Many iterations
		Prompt:         "test prompt",
		CommandBuilder: mockCommandBuilder,
		SleepDuration:  10 * time.Millisecond,
	}

	l := loop.New(cfg)
	ctx, cancel := context.WithCancel(context.Background())

	l.Start(ctx)

	// Give it a moment to start
	time.Sleep(50 * time.Millisecond)

	// Cancel the context
	cancel()

	// Give it a moment to respond to cancellation
	time.Sleep(100 * time.Millisecond)

	if l.IsRunning() {
		t.Error("Loop should stop when context is cancelled")
	}
}

func TestLoopEmitsLoopMarkers(t *testing.T) {
	cfg := loop.Config{
		Iterations:     2,
		Prompt:         "test prompt",
		CommandBuilder: mockCommandBuilder,
		SleepDuration:  10 * time.Millisecond,
	}

	l := loop.New(cfg)
	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()

	l.Start(ctx)

	var messages []loop.Message
	var mu sync.Mutex

	// Collect messages in a goroutine
	done := make(chan bool)
	go func() {
		for msg := range l.Output() {
			mu.Lock()
			messages = append(messages, msg)
			mu.Unlock()
		}
		done <- true
	}()

	// Wait for completion or timeout
	select {
	case <-done:
	case <-ctx.Done():
		t.Fatal("Test timed out")
	}

	mu.Lock()
	defer mu.Unlock()

	// Check that we got loop markers
	loopMarkers := 0
	for _, msg := range messages {
		if msg.Type == "loop_marker" {
			loopMarkers++
			if msg.Loop < 1 || msg.Loop > cfg.Iterations {
				t.Errorf("Invalid loop number in marker: %d", msg.Loop)
			}
			if msg.Total != cfg.Iterations {
				t.Errorf("Invalid total in marker: %d, expected %d", msg.Total, cfg.Iterations)
			}
			if !strings.Contains(msg.Content, "LOOP") {
				t.Errorf("Loop marker content should contain 'LOOP': %s", msg.Content)
			}
		}
	}

	if loopMarkers != cfg.Iterations {
		t.Errorf("Expected %d loop markers, got %d", cfg.Iterations, loopMarkers)
	}
}

func TestLoopEmitsCompleteMessage(t *testing.T) {
	cfg := loop.Config{
		Iterations:     1,
		Prompt:         "test prompt",
		CommandBuilder: mockCommandBuilder,
		SleepDuration:  10 * time.Millisecond,
	}

	l := loop.New(cfg)
	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()

	l.Start(ctx)

	var completeFound bool
	for msg := range l.Output() {
		if msg.Type == "complete" {
			completeFound = true
			if !strings.Contains(msg.Content, "COMPLETED") {
				t.Errorf("Complete message should contain 'COMPLETED': %s", msg.Content)
			}
		}
	}

	if !completeFound {
		t.Error("Expected a complete message when loop finishes")
	}
}

func TestLoopConfig(t *testing.T) {
	tests := []struct {
		name       string
		iterations int
		prompt     string
	}{
		{"single iteration", 1, "test"},
		{"multiple iterations", 5, "test prompt content"},
		{"empty prompt", 1, ""},
		{"long prompt", 1, strings.Repeat("x", 10000)},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			cfg := loop.Config{
				Iterations: tt.iterations,
				Prompt:     tt.prompt,
			}

			l := loop.New(cfg)
			if l == nil {
				t.Fatal("New() returned nil")
			}
		})
	}
}

func TestLoopMessageTypes(t *testing.T) {
	// Test that Message struct has expected fields
	msg := loop.Message{
		Type:    "test",
		Content: "content",
		Loop:    1,
		Total:   5,
	}

	if msg.Type != "test" {
		t.Errorf("Expected Type 'test', got %q", msg.Type)
	}
	if msg.Content != "content" {
		t.Errorf("Expected Content 'content', got %q", msg.Content)
	}
	if msg.Loop != 1 {
		t.Errorf("Expected Loop 1, got %d", msg.Loop)
	}
	if msg.Total != 5 {
		t.Errorf("Expected Total 5, got %d", msg.Total)
	}
}

func TestLoopChannelCloses(t *testing.T) {
	cfg := loop.Config{
		Iterations:     1,
		Prompt:         "test",
		CommandBuilder: mockCommandBuilder,
		SleepDuration:  10 * time.Millisecond,
	}

	l := loop.New(cfg)
	ctx := context.Background()

	l.Start(ctx)

	// Drain the channel
	count := 0
	for range l.Output() {
		count++
		if count > 1000 {
			t.Fatal("Channel should close after loop completes")
		}
	}

	// If we get here, the channel closed properly
}

func TestLoopHandlesErrorGracefully(t *testing.T) {
	cfg := loop.Config{
		Iterations:     1,
		Prompt:         "test",
		CommandBuilder: mockErrorCommandBuilder,
		SleepDuration:  10 * time.Millisecond,
	}

	l := loop.New(cfg)
	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()

	l.Start(ctx)

	var errorFound bool
	for msg := range l.Output() {
		if msg.Type == "error" {
			errorFound = true
		}
	}

	// Error message is expected when command fails
	if !errorFound {
		t.Error("Expected an error message for failing command")
	}
}

func TestLoopStopsOnContextDone(t *testing.T) {
	cfg := loop.Config{
		Iterations:     1000, // Large number
		Prompt:         "test",
		CommandBuilder: mockCommandBuilder,
		SleepDuration:  10 * time.Millisecond,
	}

	l := loop.New(cfg)
	ctx, cancel := context.WithCancel(context.Background())

	l.Start(ctx)

	// Let it start
	time.Sleep(50 * time.Millisecond)

	// Cancel immediately
	cancel()

	// Drain messages
	messageCount := 0
	for range l.Output() {
		messageCount++
	}

	// Should have far fewer than 1000 loop markers since we cancelled
	if messageCount > 50 {
		t.Logf("Received %d messages before cancellation, expected fewer", messageCount)
	}
}

func TestLoopIsRunningStateTransitions(t *testing.T) {
	cfg := loop.Config{
		Iterations:     1,
		Prompt:         "test",
		CommandBuilder: mockCommandBuilder,
		SleepDuration:  10 * time.Millisecond,
	}

	l := loop.New(cfg)

	// Initially not running
	if l.IsRunning() {
		t.Error("New loop should not be running")
	}

	ctx := context.Background()
	l.Start(ctx)

	// Should be running after start
	if !l.IsRunning() {
		t.Error("Loop should be running after Start()")
	}

	// Wait for completion
	for range l.Output() {
	}

	// Should not be running after completion
	// Give a moment for the goroutine to finish
	time.Sleep(50 * time.Millisecond)

	if l.IsRunning() {
		t.Error("Loop should not be running after completion")
	}
}

func TestLoopMarkerFormat(t *testing.T) {
	cfg := loop.Config{
		Iterations:     3,
		Prompt:         "test",
		CommandBuilder: mockCommandBuilder,
		SleepDuration:  10 * time.Millisecond,
	}

	l := loop.New(cfg)
	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()

	l.Start(ctx)

	expectedMarkers := []string{
		"======= LOOP 1/3 =======",
		"======= LOOP 2/3 =======",
		"======= LOOP 3/3 =======",
	}

	markers := make([]string, 0)
	for msg := range l.Output() {
		if msg.Type == "loop_marker" {
			markers = append(markers, msg.Content)
		}
	}

	if len(markers) != len(expectedMarkers) {
		t.Errorf("Expected %d markers, got %d", len(expectedMarkers), len(markers))
	}

	for i, expected := range expectedMarkers {
		if i < len(markers) && markers[i] != expected {
			t.Errorf("Marker %d: expected %q, got %q", i, expected, markers[i])
		}
	}
}

func TestLoopCompletionMarkerFormat(t *testing.T) {
	cfg := loop.Config{
		Iterations:     2,
		Prompt:         "test",
		CommandBuilder: mockCommandBuilder,
		SleepDuration:  10 * time.Millisecond,
	}

	l := loop.New(cfg)
	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()

	l.Start(ctx)

	var completionMsg loop.Message
	for msg := range l.Output() {
		if msg.Type == "complete" {
			completionMsg = msg
		}
	}

	expectedContent := "======= COMPLETED 2 ITERATIONS ======="
	if completionMsg.Content != expectedContent {
		t.Errorf("Expected completion message %q, got %q", expectedContent, completionMsg.Content)
	}
}

// TestLoopWithMockScript tests the loop with a mock script
// that simulates the claude command
func TestLoopWithMockScript(t *testing.T) {
	// Create a temporary directory for the mock script
	tmpDir, err := os.MkdirTemp("", "ralph-loop-test-*")
	if err != nil {
		t.Fatalf("Failed to create temp dir: %v", err)
	}
	defer os.RemoveAll(tmpDir)

	// Create a mock claude script that just echoes input
	mockScript := filepath.Join(tmpDir, "claude")
	scriptContent := `#!/bin/bash
# Read from stdin (the prompt)
cat > /dev/null
# Output mock JSON stream
echo '{"type":"system","content":"mock system"}'
echo '{"type":"assistant","content":"mock response"}'
echo '{"type":"result","usage":{"input_tokens":10,"output_tokens":5}}'
`
	if err := os.WriteFile(mockScript, []byte(scriptContent), 0755); err != nil {
		t.Fatalf("Failed to write mock script: %v", err)
	}

	// Note: This test demonstrates the expected behavior but won't actually
	// use the mock script since the loop uses exec.Command("claude", ...)
	// which looks for claude in PATH. This is documented for integration testing.
	t.Log("Mock script created at:", mockScript)
	t.Log("For integration testing, add this directory to PATH")
}

func TestNewLoopWithZeroIterations(t *testing.T) {
	// Testing edge case of zero iterations
	cfg := loop.Config{
		Iterations:     0,
		Prompt:         "test",
		CommandBuilder: mockCommandBuilder,
		SleepDuration:  10 * time.Millisecond,
	}

	l := loop.New(cfg)
	ctx := context.Background()

	l.Start(ctx)

	// Should immediately complete
	var msgs []loop.Message
	for msg := range l.Output() {
		msgs = append(msgs, msg)
	}

	// With 0 iterations, should just get the complete message
	if len(msgs) != 1 {
		t.Errorf("Expected 1 message for 0 iterations, got %d", len(msgs))
	}
	if len(msgs) > 0 && msgs[0].Type != "complete" {
		t.Errorf("Expected complete message, got %s", msgs[0].Type)
	}
}

func TestLoopOutputMessages(t *testing.T) {
	cfg := loop.Config{
		Iterations:     1,
		Prompt:         "test prompt",
		CommandBuilder: mockCommandBuilder,
		SleepDuration:  10 * time.Millisecond,
	}

	l := loop.New(cfg)
	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()

	l.Start(ctx)

	var outputMessages []loop.Message
	for msg := range l.Output() {
		if msg.Type == "output" {
			outputMessages = append(outputMessages, msg)
		}
	}

	// Should have received the mock JSON stream output
	if len(outputMessages) == 0 {
		t.Error("Expected at least one output message from mock command")
	}

	// Check that we received the expected JSON lines
	foundSystemMsg := false
	foundAssistantMsg := false
	foundResultMsg := false

	for _, msg := range outputMessages {
		if strings.Contains(msg.Content, "fresh-session-001") {
			foundSystemMsg = true
		}
		if strings.Contains(msg.Content, "test assistant message") {
			foundAssistantMsg = true
		}
		if strings.Contains(msg.Content, "input_tokens") {
			foundResultMsg = true
		}
	}

	if !foundSystemMsg {
		t.Error("Expected to find system message in output")
	}
	if !foundAssistantMsg {
		t.Error("Expected to find assistant message in output")
	}
	if !foundResultMsg {
		t.Error("Expected to find result message in output")
	}
}

func TestLoopIterationTracking(t *testing.T) {
	cfg := loop.Config{
		Iterations:     3,
		Prompt:         "test",
		CommandBuilder: mockCommandBuilder,
		SleepDuration:  10 * time.Millisecond,
	}

	l := loop.New(cfg)
	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()

	l.Start(ctx)

	// Track which iterations we see
	seenIterations := make(map[int]bool)

	for msg := range l.Output() {
		if msg.Type == "loop_marker" {
			seenIterations[msg.Loop] = true
		}
	}

	// Verify we saw all iterations
	for i := 1; i <= 3; i++ {
		if !seenIterations[i] {
			t.Errorf("Missing loop marker for iteration %d", i)
		}
	}
}

func TestDefaultCommandBuilder(t *testing.T) {
	ctx := context.Background()
	cmd := loop.DefaultCommandBuilder(ctx, "test prompt")

	if cmd == nil {
		t.Fatal("DefaultCommandBuilder returned nil")
	}

	// Check that it creates a claude command
	if cmd.Path == "" {
		t.Error("Command path should not be empty")
	}

	// Check that the args include expected flags
	args := cmd.Args
	hasStreamJson := false
	hasSkipPermissions := false

	for i, arg := range args {
		if arg == "stream-json" && i > 0 && args[i-1] == "--output-format" {
			hasStreamJson = true
		}
		if arg == "--dangerously-skip-permissions" {
			hasSkipPermissions = true
		}
	}

	if !hasStreamJson {
		t.Error("Expected --output-format stream-json flag")
	}
	if !hasSkipPermissions {
		t.Error("Expected --dangerously-skip-permissions flag")
	}
}

func TestSetIterations(t *testing.T) {
	cfg := loop.Config{
		Iterations: 5,
		Prompt:     "test",
	}
	l := loop.New(cfg)

	if l.GetIterations() != 5 {
		t.Errorf("Expected initial iterations 5, got %d", l.GetIterations())
	}

	l.SetIterations(10)
	if l.GetIterations() != 10 {
		t.Errorf("Expected iterations 10 after SetIterations, got %d", l.GetIterations())
	}

	l.SetIterations(3)
	if l.GetIterations() != 3 {
		t.Errorf("Expected iterations 3 after SetIterations, got %d", l.GetIterations())
	}
}

func TestGetIterationsDefault(t *testing.T) {
	cfg := loop.Config{
		Iterations: 7,
		Prompt:     "test",
	}
	l := loop.New(cfg)

	if l.GetIterations() != 7 {
		t.Errorf("Expected iterations 7, got %d", l.GetIterations())
	}
}

func TestSetIterationsDuringRun(t *testing.T) {
	cfg := loop.Config{
		Iterations:     2,
		Prompt:         "test",
		CommandBuilder: mockCommandBuilder,
		SleepDuration:  10 * time.Millisecond,
	}

	l := loop.New(cfg)
	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()

	l.Start(ctx)

	// Increase iterations while running
	l.SetIterations(4)

	var markers []loop.Message
	for msg := range l.Output() {
		if msg.Type == "loop_marker" {
			markers = append(markers, msg)
		}
	}

	// Should have run more than 2 iterations since we increased to 4
	if len(markers) < 3 {
		t.Errorf("Expected at least 3 loop markers after increasing iterations to 4, got %d", len(markers))
	}
}

func TestLoopMultipleIterationsWithOutput(t *testing.T) {
	cfg := loop.Config{
		Iterations:     3,
		Prompt:         "test",
		CommandBuilder: mockCommandBuilder,
		SleepDuration:  10 * time.Millisecond,
	}

	l := loop.New(cfg)
	ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
	defer cancel()

	l.Start(ctx)

	// Count messages by type and iteration
	loopMarkers := 0
	outputMsgs := 0
	completeMsg := false

	for msg := range l.Output() {
		switch msg.Type {
		case "loop_marker":
			loopMarkers++
		case "output":
			outputMsgs++
		case "complete":
			completeMsg = true
		}
	}

	if loopMarkers != 3 {
		t.Errorf("Expected 3 loop markers, got %d", loopMarkers)
	}

	// Each iteration should produce 3 output lines from mock
	if outputMsgs < 9 {
		t.Errorf("Expected at least 9 output messages (3 per iteration), got %d", outputMsgs)
	}

	if !completeMsg {
		t.Error("Expected a completion message")
	}
}

func TestSetSessionID(t *testing.T) {
	cfg := loop.Config{
		Iterations: 1,
		Prompt:     "test",
	}
	l := loop.New(cfg)

	// Default should be empty
	if l.GetSessionID() != "" {
		t.Errorf("Expected empty session ID, got %q", l.GetSessionID())
	}

	l.SetSessionID("session-abc-123")
	if l.GetSessionID() != "session-abc-123" {
		t.Errorf("Expected 'session-abc-123', got %q", l.GetSessionID())
	}

	l.SetSessionID("session-xyz-456")
	if l.GetSessionID() != "session-xyz-456" {
		t.Errorf("Expected 'session-xyz-456', got %q", l.GetSessionID())
	}
}

func TestGetSessionIDDefault(t *testing.T) {
	cfg := loop.Config{
		Iterations: 3,
		Prompt:     "test",
	}
	l := loop.New(cfg)

	if l.GetSessionID() != "" {
		t.Errorf("Expected empty default session ID, got %q", l.GetSessionID())
	}
}

func TestResumeUsesSessionID(t *testing.T) {
	cfg := loop.Config{
		Iterations:     2,
		Prompt:         "test",
		CommandBuilder: mockCommandBuilder,
		SleepDuration:  10 * time.Millisecond,
	}

	l := loop.New(cfg)
	ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
	defer cancel()

	l.Start(ctx)

	// Wait for first loop marker to appear
	var foundFirstMarker bool
	output := l.Output()
	for msg := range output {
		if msg.Type == "loop_marker" && strings.Contains(msg.Content, "LOOP 1/") {
			foundFirstMarker = true
		}
		// Look for the system message with session_id from first iteration
		if msg.Type == "output" && strings.Contains(msg.Content, "fresh-session-001") {
			// Simulate what main.go does: capture session ID
			l.SetSessionID("fresh-session-001")
			break
		}
	}

	if !foundFirstMarker {
		t.Fatal("Never saw first loop marker")
	}

	// Pause the loop
	l.Pause()

	// Wait for STOPPED marker
	for msg := range output {
		if msg.Type == "loop_marker" && strings.Contains(msg.Content, "STOPPED") {
			break
		}
	}

	// Resume the loop
	l.Resume()

	// Now check the resumed iteration output for the same session ID
	// (the mock echoes back the --resume session ID in the system message)
	var foundResumedSession bool
	for msg := range output {
		if msg.Type == "output" && strings.Contains(msg.Content, `"session_id":"fresh-session-001"`) {
			foundResumedSession = true
		}
	}

	if !foundResumedSession {
		t.Error("Expected resumed iteration to use --resume with captured session ID")
	}
}

func TestFreshIterationNoResume(t *testing.T) {
	// Verify that normal (non-paused) iterations don't use --resume
	cfg := loop.Config{
		Iterations:     2,
		Prompt:         "test",
		CommandBuilder: mockCommandBuilder,
		SleepDuration:  10 * time.Millisecond,
	}

	l := loop.New(cfg)
	ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
	defer cancel()

	l.Start(ctx)

	// Both iterations should use fresh-session-001 (the mock's default without --resume)
	freshCount := 0
	for msg := range l.Output() {
		if msg.Type == "output" && strings.Contains(msg.Content, "fresh-session-001") {
			freshCount++
		}
	}

	// Should have 2 fresh sessions (one per iteration)
	if freshCount != 2 {
		t.Errorf("Expected 2 fresh session messages, got %d", freshCount)
	}
}

func TestPauseCapturesSessionID(t *testing.T) {
	cfg := loop.Config{
		Iterations:     100,
		Prompt:         "test",
		CommandBuilder: mockCommandBuilder,
		SleepDuration:  10 * time.Millisecond,
	}

	l := loop.New(cfg)
	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()

	l.Start(ctx)

	// Set a session ID (simulating what main.go does)
	l.SetSessionID("test-session-for-pause")

	// Wait for first output before pausing
	output := l.Output()
	for msg := range output {
		if msg.Type == "output" {
			break
		}
	}

	// Pause
	l.Pause()

	// Verify session ID is still accessible
	if l.GetSessionID() != "test-session-for-pause" {
		t.Errorf("Expected session ID to be preserved after pause, got %q", l.GetSessionID())
	}

	cancel()
}
