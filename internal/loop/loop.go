// Package loop implements the Claude CLI execution loop.
package loop

import (
	"bufio"
	"context"
	"fmt"
	"io"
	"os/exec"
	"time"
)

// Config holds the loop execution configuration.
type Config struct {
	Iterations int
	Prompt     string // The prompt content to send to Claude
}

// Message represents output from the loop.
type Message struct {
	Type    string // "loop_marker", "output", "error", "complete"
	Content string
	Loop    int
	Total   int
}

// Loop manages the Claude CLI execution loop.
type Loop struct {
	config  Config
	output  chan Message
	cancel  context.CancelFunc
	running bool
}

// New creates a new Loop with the given configuration.
func New(cfg Config) *Loop {
	return &Loop{
		config: cfg,
		output: make(chan Message, 100),
	}
}

// Output returns the channel for receiving loop output.
func (l *Loop) Output() <-chan Message {
	return l.output
}

// Start begins the loop execution in a goroutine.
func (l *Loop) Start(ctx context.Context) {
	ctx, l.cancel = context.WithCancel(ctx)
	l.running = true

	go l.run(ctx)
}

// Stop cancels the running loop.
func (l *Loop) Stop() {
	if l.cancel != nil {
		l.cancel()
	}
	l.running = false
}

// IsRunning returns whether the loop is currently running.
func (l *Loop) IsRunning() bool {
	return l.running
}

// run executes the main loop logic.
func (l *Loop) run(ctx context.Context) {
	defer close(l.output)
	defer func() { l.running = false }()

	for i := 1; i <= l.config.Iterations; i++ {
		select {
		case <-ctx.Done():
			return
		default:
		}

		// Send loop marker
		l.output <- Message{
			Type:    "loop_marker",
			Content: fmt.Sprintf("======= LOOP %d/%d =======", i, l.config.Iterations),
			Loop:    i,
			Total:   l.config.Iterations,
		}

		// Execute Claude CLI
		if err := l.executeIteration(ctx, i); err != nil {
			l.output <- Message{
				Type:    "error",
				Content: err.Error(),
				Loop:    i,
				Total:   l.config.Iterations,
			}
		}

		// Sleep between iterations (except for the last one)
		if i < l.config.Iterations {
			select {
			case <-ctx.Done():
				return
			case <-time.After(1 * time.Second):
			}
		}
	}

	l.output <- Message{
		Type:    "complete",
		Content: fmt.Sprintf("======= COMPLETED %d ITERATIONS =======", l.config.Iterations),
		Loop:    l.config.Iterations,
		Total:   l.config.Iterations,
	}
}

// executeIteration runs a single Claude CLI iteration.
func (l *Loop) executeIteration(ctx context.Context, iteration int) error {
	// Build the command
	cmd := exec.CommandContext(ctx, "claude",
		"--print",
		"--output-format", "stream-json",
		"--dangerously-skip-permissions",
		"--verbose",
	)

	// Set up stdin with the prompt
	stdin, err := cmd.StdinPipe()
	if err != nil {
		return fmt.Errorf("failed to create stdin pipe: %w", err)
	}

	// Set up stdout
	stdout, err := cmd.StdoutPipe()
	if err != nil {
		return fmt.Errorf("failed to create stdout pipe: %w", err)
	}

	// Set up stderr
	stderr, err := cmd.StderrPipe()
	if err != nil {
		return fmt.Errorf("failed to create stderr pipe: %w", err)
	}

	// Start the command
	if err := cmd.Start(); err != nil {
		return fmt.Errorf("failed to start claude: %w", err)
	}

	// Write prompt to stdin
	go func() {
		defer stdin.Close()
		io.WriteString(stdin, l.config.Prompt)
	}()

	// Read stdout in a goroutine
	go l.streamOutput(stdout, iteration)

	// Read stderr in a goroutine
	go l.streamOutput(stderr, iteration)

	// Wait for command to complete
	if err := cmd.Wait(); err != nil {
		// Don't return error for context cancellation
		if ctx.Err() != nil {
			return nil
		}
		return fmt.Errorf("claude command failed: %w", err)
	}

	return nil
}

// streamOutput reads from a reader and sends lines to the output channel.
func (l *Loop) streamOutput(r io.Reader, iteration int) {
	scanner := bufio.NewScanner(r)
	// Increase buffer size for long lines
	buf := make([]byte, 0, 64*1024)
	scanner.Buffer(buf, 1024*1024)

	for scanner.Scan() {
		l.output <- Message{
			Type:    "output",
			Content: scanner.Text(),
			Loop:    iteration,
			Total:   l.config.Iterations,
		}
	}
}
