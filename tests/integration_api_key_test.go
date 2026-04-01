//go:build integration

package tests

import (
	"context"
	"encoding/json"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"testing"
	"time"
)

// ralphBinary is set by TestMain to point at the built ralph binary.
// promptFile is a minimal prompt file for ralph to use.
var ralphBinary string
var promptFile string

func TestMain(m *testing.M) {
	// Build ralph binary into a temp dir so integration tests can invoke it.
	tmp, err := os.MkdirTemp("", "ralph-integ-*")
	if err != nil {
		panic("cannot create temp dir: " + err.Error())
	}
	defer os.RemoveAll(tmp)

	ralphBinary = filepath.Join(tmp, "ralph")
	build := exec.Command("go", "build", "-o", ralphBinary, "../cmd/ralph")
	build.Stderr = os.Stderr
	if err := build.Run(); err != nil {
		panic("cannot build ralph: " + err.Error())
	}

	// Create a minimal prompt file so ralph doesn't need a specs/ directory.
	promptFile = filepath.Join(tmp, "prompt.md")
	if err := os.WriteFile(promptFile, []byte("Say hello and exit."), 0644); err != nil {
		panic("cannot write prompt file: " + err.Error())
	}

	os.Exit(m.Run())
}

// cleanEnv returns a copy of the current environment with auth-related vars
// removed, plus any extra KEY=VALUE pairs appended.
func cleanEnv(extra ...string) []string {
	strip := map[string]bool{
		"CLAUDE_CODE_ENTRYPOINT": true,
	}
	var env []string
	for _, e := range os.Environ() {
		key := e[:strings.IndexByte(e, '=')]
		if strip[key] {
			continue
		}
		env = append(env, e)
	}
	return append(env, extra...)
}

// initMsg is the subset of fields we care about from the stream-json init message.
type initMsg struct {
	Type         string `json:"type"`
	Subtype      string `json:"subtype"`
	APIKeySource string `json:"apiKeySource"`
}

// assistantMsg captures error fields from an assistant message.
type assistantMsg struct {
	Type    string `json:"type"`
	Error   string `json:"error"`
	IsError bool   `json:"is_error"`
}

// --- Claude CLI integration tests ---

func TestIntegration_ClaudeCLI_APIKeyInherited(t *testing.T) {
	if _, err := exec.LookPath("claude"); err != nil {
		t.Skip("claude CLI not found on PATH")
	}

	ctx, cancel := context.WithTimeout(context.Background(), 30*time.Second)
	defer cancel()

	cmd := exec.CommandContext(ctx, "claude",
		"--print",
		"--output-format", "stream-json",
		"--verbose",
		"--dangerously-skip-permissions",
		"say hello",
	)
	cmd.Env = cleanEnv("ANTHROPIC_API_KEY=sk-ant-test-invalid-key")

	out, _ := cmd.CombinedOutput() // ignore exit code — we expect an error
	lines := strings.Split(string(out), "\n")

	var found bool
	for _, line := range lines {
		var msg initMsg
		if json.Unmarshal([]byte(line), &msg) == nil && msg.Type == "system" && msg.Subtype == "init" {
			if msg.APIKeySource != "ANTHROPIC_API_KEY" {
				t.Errorf("expected apiKeySource=ANTHROPIC_API_KEY, got %q", msg.APIKeySource)
			}
			found = true
			break
		}
	}
	if !found {
		t.Fatalf("no system init message found in output:\n%s", string(out))
	}
}

func TestIntegration_ClaudeCLI_InvalidKey_AuthError(t *testing.T) {
	if _, err := exec.LookPath("claude"); err != nil {
		t.Skip("claude CLI not found on PATH")
	}

	ctx, cancel := context.WithTimeout(context.Background(), 30*time.Second)
	defer cancel()

	cmd := exec.CommandContext(ctx, "claude",
		"--print",
		"--output-format", "stream-json",
		"--verbose",
		"--dangerously-skip-permissions",
		"say hello",
	)
	cmd.Env = cleanEnv("ANTHROPIC_API_KEY=sk-ant-test-invalid-key")

	out, _ := cmd.CombinedOutput()
	lines := strings.Split(string(out), "\n")

	var gotAuthError bool
	for _, line := range lines {
		var msg assistantMsg
		if json.Unmarshal([]byte(line), &msg) == nil && msg.Type == "assistant" {
			if strings.Contains(msg.Error, "authentication") {
				gotAuthError = true
				break
			}
		}
	}
	if !gotAuthError {
		t.Errorf("expected authentication error from claude with invalid API key, output:\n%s", string(out))
	}
}

// --- Ralph end-to-end integration tests ---

func TestIntegration_Ralph_CLI_InvalidKey_DetectsAuthError(t *testing.T) {
	if _, err := exec.LookPath("claude"); err != nil {
		t.Skip("claude CLI not found on PATH")
	}

	ctx, cancel := context.WithTimeout(context.Background(), 30*time.Second)
	defer cancel()

	cmd := exec.CommandContext(ctx, ralphBinary,
		"--cli",
		"--iterations", "1",
		"--no-tmux",
		"--loop-prompt", promptFile,
	)
	cmd.Env = cleanEnv("ANTHROPIC_API_KEY=sk-ant-test-invalid-key")

	out, _ := cmd.CombinedOutput()
	output := string(out)

	// Ralph should detect the auth error and print its OWN error message
	// (not just pass through the raw JSON from claude).
	// Expected: "[error] Authentication failed: ANTHROPIC_API_KEY is set but appears to be invalid."
	if !strings.Contains(output, "[error] Authentication failed") {
		t.Errorf("ralph did not print its own auth error message.\nExpected output containing '[error] Authentication failed'.\nGot:\n%s", output)
	}
}

func TestIntegration_Ralph_CLI_InvalidKey_ExitsNonZero(t *testing.T) {
	if _, err := exec.LookPath("claude"); err != nil {
		t.Skip("claude CLI not found on PATH")
	}

	ctx, cancel := context.WithTimeout(context.Background(), 30*time.Second)
	defer cancel()

	cmd := exec.CommandContext(ctx, ralphBinary,
		"--cli",
		"--iterations", "1",
		"--no-tmux",
		"--loop-prompt", promptFile,
	)
	cmd.Env = cleanEnv("ANTHROPIC_API_KEY=sk-ant-test-invalid-key")

	err := cmd.Run()
	if err == nil {
		t.Error("expected ralph to exit with non-zero code on auth failure, but it exited 0")
	}
}

func TestIntegration_Ralph_CLI_NoKey_ExitsGracefully(t *testing.T) {
	if _, err := exec.LookPath("claude"); err != nil {
		t.Skip("claude CLI not found on PATH")
	}

	// Ralph should not hang indefinitely when there's no auth.
	// Give it 15s — if it hasn't exited by then, it's stuck.
	ctx, cancel := context.WithTimeout(context.Background(), 15*time.Second)
	defer cancel()

	cmd := exec.CommandContext(ctx, ralphBinary,
		"--cli",
		"--iterations", "1",
		"--no-tmux",
		"--loop-prompt", promptFile,
	)
	// Deliberately do NOT set ANTHROPIC_API_KEY
	cmd.Env = cleanEnv()

	out, err := cmd.CombinedOutput()
	output := string(out)

	if ctx.Err() == context.DeadlineExceeded {
		t.Errorf("ralph hung for 15s without exiting — should detect missing auth and stop.\nOutput so far:\n%s", output)
		return
	}

	// It exited within the timeout — log the output for debugging.
	t.Logf("ralph exited (err=%v), output:\n%s", err, output)
}
