// Package loop implements the Claude CLI execution loop.
package loop

import (
	"bufio"
	"context"
	"fmt"
	"io"
	"os/exec"
	"sync"
	"time"
)

// CommandBuilder is a function that creates an exec.Cmd for running Claude.
// This allows for dependency injection in tests.
type CommandBuilder func(ctx context.Context, prompt string) *exec.Cmd

// DefaultCommandBuilder creates the standard claude CLI command.
func DefaultCommandBuilder(ctx context.Context, prompt string) *exec.Cmd {
	return exec.CommandContext(ctx, "claude",
		"--print",
		"--output-format", "stream-json",
		"--dangerously-skip-permissions",
		"--verbose",
	)
}

// Config holds the loop execution configuration.
type Config struct {
	Iterations     int
	Prompt         string         // The prompt content to send to Claude
	CommandBuilder CommandBuilder // Optional custom command builder (for testing)
	SleepDuration  time.Duration  // Duration to sleep between iterations (default: 1s)
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
	config           Config
	mu               sync.Mutex // protects config.Iterations, sessionID, resumeSessionID, completedWaiting
	output           chan Message
	cancel           context.CancelFunc
	running          bool
	paused           bool
	completedWaiting bool // loop finished all iterations but stays alive waiting for more
	resumeCh         chan struct{}
	iterationCancel  context.CancelFunc // cancels current iteration only
	sessionID        string             // latest session ID from Claude CLI output
	resumeSessionID  string             // session ID to use with --resume on next iteration
}

// New creates a new Loop with the given configuration.
func New(cfg Config) *Loop {
	// Set defaults
	if cfg.CommandBuilder == nil {
		cfg.CommandBuilder = DefaultCommandBuilder
	}
	if cfg.SleepDuration == 0 {
		cfg.SleepDuration = 1 * time.Second
	}
	return &Loop{
		config:   cfg,
		output:   make(chan Message, 100),
		resumeCh: make(chan struct{}, 1),
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

// IsPaused returns whether the loop is currently paused.
func (l *Loop) IsPaused() bool {
	return l.paused
}

// Pause immediately interrupts the current iteration and pauses the loop.
// Captures the current session ID so the next resume can use --resume.
func (l *Loop) Pause() {
	if !l.paused && l.running {
		l.paused = true
		// Capture session ID for resume
		l.mu.Lock()
		l.resumeSessionID = l.sessionID
		l.mu.Unlock()
		// Cancel the current iteration to interrupt it immediately
		if l.iterationCancel != nil {
			l.iterationCancel()
		}
	}
}

// Resume resumes a paused loop, or wakes a completed-waiting loop to run new iterations.
func (l *Loop) Resume() {
	if l.paused {
		l.paused = false
		l.resumeCh <- struct{}{}
	} else {
		l.mu.Lock()
		cw := l.completedWaiting
		l.mu.Unlock()
		if cw {
			l.resumeCh <- struct{}{}
		}
	}
}

// IsCompletedWaiting returns whether the loop has completed all iterations
// and is waiting for more iterations to be added.
func (l *Loop) IsCompletedWaiting() bool {
	l.mu.Lock()
	defer l.mu.Unlock()
	return l.completedWaiting
}

// SetIterations dynamically adjusts the total iteration count.
// Thread-safe: can be called from any goroutine.
func (l *Loop) SetIterations(n int) {
	l.mu.Lock()
	l.config.Iterations = n
	l.mu.Unlock()
}

// GetIterations returns the current total iteration count.
// Thread-safe: can be called from any goroutine.
func (l *Loop) GetIterations() int {
	l.mu.Lock()
	defer l.mu.Unlock()
	return l.config.Iterations
}

// SetSessionID stores the latest session ID from Claude CLI output.
// Thread-safe: can be called from any goroutine (typically the output processing goroutine).
func (l *Loop) SetSessionID(id string) {
	l.mu.Lock()
	l.sessionID = id
	l.mu.Unlock()
}

// GetSessionID returns the current session ID.
// Thread-safe: can be called from any goroutine.
func (l *Loop) GetSessionID() string {
	l.mu.Lock()
	defer l.mu.Unlock()
	return l.sessionID
}

// run executes the main loop logic.
// After completing all iterations, the goroutine stays alive waiting for more
// iterations to be added (via SetIterations + Resume). This enables the
// post-completion loop extension workflow where users press '+' then 'r'.
func (l *Loop) run(ctx context.Context) {
	defer close(l.output)
	defer func() { l.running = false }()

	i := 1
	for {
		// Inner loop: run iterations until we catch up with GetIterations()
		for ; i <= l.GetIterations(); i++ {
			select {
			case <-ctx.Done():
				return
			default:
			}

			// Check if paused and wait for resume
			if l.paused {
				total := l.GetIterations()
				l.output <- Message{
					Type:    "loop_marker",
					Content: "======= LOOP STOPPED =======",
					Loop:    i,
					Total:   total,
				}
				select {
				case <-ctx.Done():
					return
				case <-l.resumeCh:
					total = l.GetIterations()
					l.output <- Message{
						Type:    "loop_marker",
						Content: "======= LOOP RESUMED =======",
						Loop:    i,
						Total:   total,
					}
				}
			}

			// Send loop marker
			total := l.GetIterations()
			l.output <- Message{
				Type:    "loop_marker",
				Content: fmt.Sprintf("======= LOOP %d/%d =======", i, total),
				Loop:    i,
				Total:   total,
			}

			// Create a cancellable context for this iteration
			iterCtx, iterCancel := context.WithCancel(ctx)
			l.iterationCancel = iterCancel

			// Execute Claude CLI
			err := l.executeIteration(iterCtx, i)
			iterCancel() // clean up
			l.iterationCancel = nil

			// If we were paused (interrupted), don't report as error
			if l.paused {
				total := l.GetIterations()
				l.output <- Message{
					Type:    "loop_marker",
					Content: "======= LOOP STOPPED =======",
					Loop:    i,
					Total:   total,
				}
				select {
				case <-ctx.Done():
					return
				case <-l.resumeCh:
					total = l.GetIterations()
					l.output <- Message{
						Type:    "loop_marker",
						Content: "======= LOOP RESUMED =======",
						Loop:    i,
						Total:   total,
					}
				}
				// Retry this iteration
				i--
				continue
			}

			if err != nil {
				total := l.GetIterations()
				l.output <- Message{
					Type:    "error",
					Content: err.Error(),
					Loop:    i,
					Total:   total,
				}
			}

			// Sleep between iterations (except for the last one)
			if i < l.GetIterations() {
				select {
				case <-ctx.Done():
					return
				case <-time.After(l.config.SleepDuration):
				}
			}
		}

		// All current iterations complete — send completion marker
		completedCount := i - 1
		total := l.GetIterations()
		l.output <- Message{
			Type:    "complete",
			Content: fmt.Sprintf("======= COMPLETED %d ITERATIONS =======", total),
			Loop:    total,
			Total:   total,
		}

		// Enter waiting state: stay alive for potential new iterations
		l.mu.Lock()
		l.completedWaiting = true
		l.mu.Unlock()

		select {
		case <-ctx.Done():
			l.mu.Lock()
			l.completedWaiting = false
			l.mu.Unlock()
			return
		case <-l.resumeCh:
			l.mu.Lock()
			l.completedWaiting = false
			l.mu.Unlock()
		}

		// Check if new iterations were actually added
		newTotal := l.GetIterations()
		if completedCount >= newTotal {
			// No new iterations — go back to waiting
			continue
		}

		// Resume with new iterations
		l.output <- Message{
			Type:    "loop_marker",
			Content: "======= LOOP RESUMED =======",
			Loop:    i,
			Total:   newTotal,
		}
		// i is already at completedCount + 1; inner for-loop will pick up
	}
}

// executeIteration runs a single Claude CLI iteration.
func (l *Loop) executeIteration(ctx context.Context, iteration int) error {
	// Build the command using the configured builder
	cmd := l.config.CommandBuilder(ctx, l.config.Prompt)

	// If resuming after pause, add --resume flag with the captured session ID
	l.mu.Lock()
	resumeID := l.resumeSessionID
	l.resumeSessionID = "" // consume it
	l.mu.Unlock()
	if resumeID != "" {
		cmd.Args = append(cmd.Args, "--resume", resumeID)
	}

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

	// Wait for both streamOutput goroutines to finish before returning,
	// so they don't race against channel close in run()
	var wg sync.WaitGroup
	wg.Add(2)

	// Read stdout in a goroutine
	go func() {
		defer wg.Done()
		l.streamOutput(stdout, iteration)
	}()

	// Read stderr in a goroutine
	go func() {
		defer wg.Done()
		l.streamOutput(stderr, iteration)
	}()

	// Wait for stream readers to finish processing all output BEFORE cmd.Wait(),
	// because cmd.Wait() closes the pipes. Per Go docs: "it is incorrect to call
	// Wait before all reads from the pipe have completed."
	wg.Wait()

	// Wait for command to complete (process already exited at this point)
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
	// Use a 10MB max buffer to handle very large Claude CLI responses
	// (tool results with full file contents, long assistant messages, etc.)
	buf := make([]byte, 0, 64*1024)
	scanner.Buffer(buf, 10*1024*1024)

	for scanner.Scan() {
		l.output <- Message{
			Type:    "output",
			Content: scanner.Text(),
			Loop:    iteration,
			Total:   l.GetIterations(),
		}
	}
	// Report scanner errors (e.g., lines exceeding buffer limit)
	if err := scanner.Err(); err != nil {
		l.output <- Message{
			Type:    "error",
			Content: fmt.Sprintf("output stream error: %v", err),
			Loop:    iteration,
			Total:   l.GetIterations(),
		}
	}
}
