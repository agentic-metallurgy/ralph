package main

import (
	"context"
	"fmt"
	"os"
	"os/signal"
	"strings"
	"syscall"
	"time"

	tea "github.com/charmbracelet/bubbletea"
	"github.com/cloudosai/ralph-go/internal/config"
	"github.com/cloudosai/ralph-go/internal/loop"
	"github.com/cloudosai/ralph-go/internal/parser"
	"github.com/cloudosai/ralph-go/internal/prompt"
	"github.com/cloudosai/ralph-go/internal/stats"
	"github.com/cloudosai/ralph-go/internal/tmux"
	"github.com/cloudosai/ralph-go/internal/tui"
)

const statsFilePath = ".claude_stats"
const planFilePath = "IMPLEMENTATION_PLAN.md"

func main() {
	// Parse command-line flags and get configuration
	cfg := config.ParseFlags()

	// Handle --version: print version and exit
	if cfg.ShowVersion {
		fmt.Printf("ralph %s\n", config.Version)
		return
	}

	// Handle --show-prompt: print embedded prompt and exit
	if cfg.ShowPrompt {
		var showLoader *prompt.Loader
		if cfg.IsPlanMode() {
			showLoader = prompt.NewPlanLoader("", cfg.Goal)
		} else {
			showLoader = prompt.NewLoader("", cfg.Goal)
		}
		content, err := showLoader.Load()
		if err != nil {
			fmt.Fprintf(os.Stderr, "Error: %v\n", err)
			os.Exit(1)
		}
		fmt.Print(content)
		return
	}

	// Wrap in tmux if not already inside one (skip in CLI mode)
	if !cfg.CLI && tmux.ShouldWrap(cfg.NoTmux) {
		if err := tmux.Wrap(cfg.Subcommand); err != nil {
			fmt.Fprintf(os.Stderr, "Warning: Could not wrap in tmux: %v\n", err)
			// Continue without tmux
		}
	}

	// Validate configuration
	if err := cfg.Validate(); err != nil {
		fmt.Fprintf(os.Stderr, "Error: %v\n", err)
		os.Exit(1)
	}

	// Load the loop prompt (embedded or from override file)
	var promptLoader *prompt.Loader
	if cfg.IsPlanMode() {
		promptLoader = prompt.NewPlanLoader(cfg.LoopPrompt, cfg.Goal)
	} else {
		promptLoader = prompt.NewLoader(cfg.LoopPrompt, cfg.Goal)
	}
	promptContent, err := promptLoader.Load()
	if err != nil {
		fmt.Fprintf(os.Stderr, "Error loading prompt: %v\n", err)
		os.Exit(1)
	}

	// Load existing stats (if any)
	tokenStats, err := stats.LoadTokenStats(statsFilePath)
	if err != nil {
		fmt.Fprintf(os.Stderr, "Warning: Could not load stats: %v\n", err)
		tokenStats = stats.NewTokenStats()
	}

	// CLI mode: run without TUI, output to stdout/stderr, exit when complete
	if cfg.CLI {
		var exitCode int
		if cfg.IsPlanAndBuildMode() {
			exitCode = runPlanAndBuildCLI(cfg, tokenStats)
		} else {
			exitCode = runCLI(cfg, promptContent, tokenStats)
		}
		if err := tokenStats.Save(statsFilePath); err != nil {
			fmt.Fprintf(os.Stderr, "Warning: Could not save stats: %v\n", err)
		}
		os.Exit(exitCode)
	}

	// Plan-and-build mode: run planning (1 iteration) then building (N iterations) in single TUI session
	if cfg.IsPlanAndBuildMode() {
		runPlanAndBuild(cfg, tokenStats)
		if err := tokenStats.Save(statsFilePath); err != nil {
			fmt.Fprintf(os.Stderr, "Warning: Could not save stats: %v\n", err)
		}
		return
	}

	// Set up channels for TUI communication
	msgChan := make(chan tui.Message, 100)
	doneChan := make(chan struct{})

	// Create the loop configuration
	loopConfig := loop.Config{
		Iterations: cfg.Iterations,
		Prompt:     promptContent,
	}

	// Create the loop
	claudeLoop := loop.New(loopConfig)

	// Create tmux status bar (no-op if not inside tmux)
	tmuxBar := tmux.NewStatusBar()

	// Create the TUI model with channels
	model := tui.NewModelWithChannels(msgChan, doneChan)
	model.SetStats(tokenStats)
	model.SetBaseElapsed(time.Duration(tokenStats.TotalElapsedNs))
	model.SetLoopProgress(0, cfg.Iterations)
	model.SetLoop(claudeLoop)
	model.SetTmuxStatusBar(tmuxBar)

	// Parse implementation plan for task counts
	completedTasks, totalTasks := parseTaskCounts(planFilePath)
	model.SetCompletedTasks(completedTasks, totalTasks)

	// Set current mode for TUI display
	if cfg.IsPlanMode() {
		model.SetCurrentMode("Planning")
	} else if cfg.IsPlanAndBuildMode() {
		model.SetCurrentMode("Planning")
	} else {
		model.SetCurrentMode("Building")
	}

	// Create the Bubble Tea program (must be after SetLoop so the model copy has the loop reference)
	program := tea.NewProgram(model, tea.WithAltScreen())

	// Set up context for graceful shutdown
	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()

	// Handle OS signals for graceful shutdown
	sigChan := make(chan os.Signal, 1)
	signal.Notify(sigChan, syscall.SIGINT, syscall.SIGTERM)
	go func() {
		<-sigChan
		cancel()
		close(doneChan)
	}()

	// Create the parser
	jsonParser := parser.NewParser()

	// Start the processing goroutine
	go processLoopOutput(ctx, claudeLoop, jsonParser, tokenStats, msgChan, doneChan, program)

	// Start the loop execution
	claudeLoop.Start(ctx)

	// Run the TUI (blocks until user quits)
	if _, err := program.Run(); err != nil {
		fmt.Fprintf(os.Stderr, "Error running TUI: %v\n", err)
		os.Exit(1)
	}

	// Save stats on exit
	if err := tokenStats.Save(statsFilePath); err != nil {
		fmt.Fprintf(os.Stderr, "Warning: Could not save stats: %v\n", err)
	}
}

// processLoopOutput reads from the loop's output channel, parses JSON, and updates the TUI
func processLoopOutput(
	ctx context.Context,
	claudeLoop *loop.Loop,
	jsonParser *parser.Parser,
	tokenStats *stats.TokenStats,
	msgChan chan<- tui.Message,
	doneChan chan struct{},
	program *tea.Program,
) {
	defer close(msgChan)

	loopOutput := claudeLoop.Output()
	activeAgentIDs := make(map[string]bool)
	var loopTotalTokens int64 // per-loop token tracking for tmux status bar

	for {
		select {
		case <-ctx.Done():
			return
		case msg, ok := <-loopOutput:
			if !ok {
				// Loop has finished
				select {
				case <-doneChan:
				default:
					close(doneChan)
				}
				return
			}

			processMessage(msg, claudeLoop, jsonParser, tokenStats, msgChan, program, activeAgentIDs, &loopTotalTokens)
		}
	}
}

// processMessage handles a single message from the loop
func processMessage(
	msg loop.Message,
	claudeLoop *loop.Loop,
	jsonParser *parser.Parser,
	tokenStats *stats.TokenStats,
	msgChan chan<- tui.Message,
	program *tea.Program,
	activeAgentIDs map[string]bool,
	loopTotalTokens *int64,
) {
	switch msg.Type {
	case "loop_marker":
		// Update loop progress
		program.Send(tui.SendLoopUpdate(msg.Loop, msg.Total)())
		// Detect new loop iteration start (not STOPPED/COMPLETED/RESUMED)
		isLoopStart := strings.Contains(msg.Content, "LOOP") &&
			!strings.Contains(msg.Content, "STOPPED") &&
			!strings.Contains(msg.Content, "COMPLETED") &&
			!strings.Contains(msg.Content, "RESUMED")
		if isLoopStart {
			*loopTotalTokens = 0
			program.Send(tui.SendLoopStarted()())
			program.Send(tui.SendLoopStatsUpdate(0)())
		}
		// Use stop sign emoji for STOPPED messages
		role := tui.RoleLoop
		if strings.Contains(msg.Content, "STOPPED") {
			role = tui.RoleLoopStopped
		}
		msgChan <- tui.Message{
			Role:    role,
			Content: msg.Content,
		}

	case "output":
		// Try to parse as JSON first
		parsed := jsonParser.ParseLine(msg.Content)
		if parsed != nil {
			// Capture session ID from system messages for --resume support
			if sessionID := jsonParser.GetSessionID(parsed); sessionID != "" {
				claudeLoop.SetSessionID(sessionID)
			}
			handleParsedMessage(parsed, claudeLoop, jsonParser, tokenStats, msgChan, program, activeAgentIDs, loopTotalTokens)
		} else {
			// Check if it's a loop marker in the output stream
			loopMarker := jsonParser.ParseLoopMarker(msg.Content)
			if loopMarker != nil {
				program.Send(tui.SendLoopUpdate(loopMarker.Current, loopMarker.Total)())
			}
		}

	case "error":
		msgChan <- tui.Message{
			Role:    tui.RoleSystem,
			Content: fmt.Sprintf("Error: %s", msg.Content),
		}

	case "complete":
		msgChan <- tui.Message{
			Role:    tui.RoleSystem,
			Content: msg.Content,
		}
		// Signal TUI that the loop has completed its current iterations.
		// The loop stays alive waiting for more iterations (post-completion extension),
		// so we send doneMsg explicitly rather than relying on channel closure.
		program.Send(tui.SendDone()())
	}
}

// handleParsedMessage processes a parsed JSON message from Claude
func handleParsedMessage(
	parsed *parser.ParsedMessage,
	claudeLoop *loop.Loop,
	jsonParser *parser.Parser,
	tokenStats *stats.TokenStats,
	msgChan chan<- tui.Message,
	program *tea.Program,
	activeAgentIDs map[string]bool,
	loopTotalTokens *int64,
) {
	// Check for rate limit rejection — enter hibernate state
	if rejected, resetsAt := jsonParser.IsRateLimitRejected(parsed); rejected {
		claudeLoop.Hibernate(resetsAt)
		program.Send(tui.SendHibernate(resetsAt)())
		msgChan <- tui.Message{
			Role:    tui.RoleHibernate,
			Content: fmt.Sprintf("Rate limited until %s", resetsAt.Format(time.Kitchen)),
		}
		return // Don't process further
	}

	// Extract usage information
	if usage := jsonParser.GetUsage(parsed); usage != nil {
		tokenStats.AddUsage(
			usage.InputTokens,
			usage.OutputTokens,
			usage.CacheCreationInputTokens,
			usage.CacheReadInputTokens,
		)
		program.Send(tui.SendStatsUpdate(tokenStats)())
		// Also track per-loop tokens for tmux status bar
		loopTokens := usage.InputTokens + usage.OutputTokens + usage.CacheCreationInputTokens + usage.CacheReadInputTokens
		*loopTotalTokens += loopTokens
		program.Send(tui.SendLoopStatsUpdate(*loopTotalTokens)())
	}

	// Extract cost from result messages
	if cost := jsonParser.GetCost(parsed); cost > 0 {
		tokenStats.AddCost(cost)
		program.Send(tui.SendStatsUpdate(tokenStats)())
	}

	// Track parallel subagents
	// Agents are added when Task tool_use items are detected (below)
	// and removed when result messages with parent_tool_use_id arrive.
	prevCount := len(activeAgentIDs)
	if jsonParser.IsSubagentMessage(parsed) && parsed.Type == parser.MessageTypeResult {
		// Subagent finished - remove from tracking
		parentID := *parsed.ParentToolUseID
		delete(activeAgentIDs, parentID)
	}
	// Track "Task" tool_use items as pending agents (single add path)
	for _, taskID := range jsonParser.GetTaskToolUseIDs(parsed) {
		activeAgentIDs[taskID] = true
	}
	if newCount := len(activeAgentIDs); newCount != prevCount {
		program.Send(tui.SendAgentUpdate(newCount)())
	}

	// Process message content based on type
	switch parsed.Type {
	case parser.MessageTypeSystem:
		// Skip system messages (as Python version does)
		return

	case parser.MessageTypeAssistant:
		content := jsonParser.ExtractContent(parsed)

		// Display text content and scan for task references
		for _, text := range content.TextContent {
			if text != "" {
				msgChan <- tui.Message{
					Role:    tui.RoleAssistant,
					Content: text,
				}
				// Detect IMPLEMENTATION_PLAN.md task references
				if ref := jsonParser.ExtractTaskReference(text); ref != nil {
					taskLabel := fmt.Sprintf("#%d", ref.Number)
					if ref.Description != "" {
						taskLabel = fmt.Sprintf("#%d %s", ref.Number, ref.Description)
					}
					program.Send(tui.SendTaskUpdate(taskLabel)())
				}
			}
		}

		// Display tool uses
		for _, toolUse := range content.ToolUses {
			msgChan <- tui.Message{
				Role:    tui.RoleTool,
				Content: fmt.Sprintf("Using tool: %s", toolUse.Name),
			}
		}

	case parser.MessageTypeUser:
		content := jsonParser.ExtractContent(parsed)

		// Display tool results and scan for task references
		for _, toolResult := range content.ToolResults {
			if toolResult.Content != "" {
				msgChan <- tui.Message{
					Role:    tui.RoleUser,
					Content: toolResult.Content,
				}
				// Detect IMPLEMENTATION_PLAN.md task references in tool results
				if ref := jsonParser.ExtractTaskReference(toolResult.Content); ref != nil {
					taskLabel := fmt.Sprintf("#%d", ref.Number)
					if ref.Description != "" {
						taskLabel = fmt.Sprintf("#%d %s", ref.Number, ref.Description)
					}
					program.Send(tui.SendTaskUpdate(taskLabel)())
				}
			}
		}

	case parser.MessageTypeResult:
		// Result messages are handled above for cost extraction
		// Optionally show completion message
		if parsed.TotalCostUSD > 0 {
			msgChan <- tui.Message{
				Role:    tui.RoleSystem,
				Content: fmt.Sprintf("Iteration cost: $%.6f", parsed.TotalCostUSD),
			}
		}
	}
}

// runCLI runs ralph in CLI mode: no TUI, output to stdout/stderr, exit on completion.
func runCLI(cfg *config.Config, promptContent string, tokenStats *stats.TokenStats) int {
	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()

	// Handle OS signals for graceful shutdown
	sigChan := make(chan os.Signal, 1)
	signal.Notify(sigChan, syscall.SIGINT, syscall.SIGTERM)
	go func() {
		<-sigChan
		cancel()
	}()

	// Create and start the loop
	claudeLoop := loop.New(loop.Config{
		Iterations: cfg.Iterations,
		Prompt:     promptContent,
	})
	claudeLoop.Start(ctx)

	jsonParser := parser.NewParser()
	activeAgentIDs := make(map[string]bool)

	mode := "build"
	if cfg.IsPlanMode() {
		mode = "plan"
	}
	fmt.Printf("ralph cli: starting %s mode with %d iterations\n", mode, cfg.Iterations)

	for msg := range claudeLoop.Output() {
		select {
		case <-ctx.Done():
			return 1
		default:
		}

		switch msg.Type {
		case "loop_marker":
			fmt.Printf("[loop] %s\n", msg.Content)

		case "output":
			parsed := jsonParser.ParseLine(msg.Content)
			if parsed != nil {
				// Capture session ID
				if sessionID := jsonParser.GetSessionID(parsed); sessionID != "" {
					claudeLoop.SetSessionID(sessionID)
				}
				// Check for rate limit rejection — enter hibernate state
				if rejected, resetsAt := jsonParser.IsRateLimitRejected(parsed); rejected {
					claudeLoop.Hibernate(resetsAt)
					fmt.Printf("[hibernate] Rate limited until %s\n", resetsAt.Format(time.Kitchen))
				}
				// Track stats
				if usage := jsonParser.GetUsage(parsed); usage != nil {
					tokenStats.AddUsage(
						usage.InputTokens,
						usage.OutputTokens,
						usage.CacheCreationInputTokens,
						usage.CacheReadInputTokens,
					)
				}
				if cost := jsonParser.GetCost(parsed); cost > 0 {
					tokenStats.AddCost(cost)
				}
				// Track parallel subagents
				// Agents are added when Task tool_use items are detected (below)
				// and removed when result messages with parent_tool_use_id arrive.
				prevCount := len(activeAgentIDs)
				if jsonParser.IsSubagentMessage(parsed) && parsed.Type == parser.MessageTypeResult {
					// Subagent finished - remove from tracking
					parentID := *parsed.ParentToolUseID
					delete(activeAgentIDs, parentID)
				}
				// Track "Task" tool_use items as pending agents (single add path)
				for _, taskID := range jsonParser.GetTaskToolUseIDs(parsed) {
					activeAgentIDs[taskID] = true
				}
				if newCount := len(activeAgentIDs); newCount != prevCount {
					fmt.Printf("[agents] %d active\n", newCount)
				}
				// Print assistant text and tool use
				if parsed.Type == parser.MessageTypeAssistant {
					content := jsonParser.ExtractContent(parsed)
					for _, text := range content.TextContent {
						if text != "" {
							fmt.Printf("[assistant] %s\n", text)
						}
					}
					for _, item := range parsed.Message.Content {
						if item.Type == parser.ContentTypeToolUse {
							filePath := parser.ExtractFilePathFromInput(item.Input)
							if filePath != "" {
								fmt.Printf("[tool] %s: %s\n", item.Name, filePath)
							} else {
								fmt.Printf("[tool] %s\n", item.Name)
							}
						}
					}
				}
				if parsed.Type == parser.MessageTypeResult && parsed.TotalCostUSD > 0 {
					fmt.Printf("[cost] Iteration cost: $%.6f\n", parsed.TotalCostUSD)
				}
			}

		case "error":
			fmt.Fprintf(os.Stderr, "[error] %s\n", msg.Content)

		case "complete":
			fmt.Printf("[complete] %s\n", msg.Content)
			// In CLI mode, exit on completion instead of waiting
			cancel()
			return 0
		}
	}

	return 0
}

// runPlanAndBuildCLI runs plan-and-build mode in CLI: planning (1 iteration) then building (N iterations)
// with output to stdout/stderr and no TUI.
func runPlanAndBuildCLI(cfg *config.Config, tokenStats *stats.TokenStats) int {
	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()

	// Handle OS signals for graceful shutdown
	sigChan := make(chan os.Signal, 1)
	signal.Notify(sigChan, syscall.SIGINT, syscall.SIGTERM)
	go func() {
		<-sigChan
		cancel()
	}()

	jsonParser := parser.NewParser()
	activeAgentIDs := make(map[string]bool)

	fmt.Println("ralph cli: starting plan-and-build mode")

	// Phase 1: Planning
	fmt.Printf("[phase] Planning (%d iteration)\n", cfg.Iterations)

	planPromptLoader := prompt.NewPlanLoader(cfg.LoopPrompt, cfg.Goal)
	planPromptContent, err := planPromptLoader.Load()
	if err != nil {
		fmt.Fprintf(os.Stderr, "[error] Failed to load plan prompt: %v\n", err)
		return 1
	}

	planLoop := loop.New(loop.Config{
		Iterations: cfg.Iterations, // Always 1 for plan phase
		Prompt:     planPromptContent,
	})
	planLoop.Start(ctx)

	var sessionID string

	// Process plan loop output
	for msg := range planLoop.Output() {
		select {
		case <-ctx.Done():
			return 1
		default:
		}

		switch msg.Type {
		case "loop_marker":
			fmt.Printf("[loop] %s\n", msg.Content)

		case "output":
			parsed := jsonParser.ParseLine(msg.Content)
			if parsed != nil {
				// Capture session ID
				if sid := jsonParser.GetSessionID(parsed); sid != "" {
					planLoop.SetSessionID(sid)
					sessionID = sid
				}
				// Track stats
				if usage := jsonParser.GetUsage(parsed); usage != nil {
					tokenStats.AddUsage(
						usage.InputTokens,
						usage.OutputTokens,
						usage.CacheCreationInputTokens,
						usage.CacheReadInputTokens,
					)
				}
				if cost := jsonParser.GetCost(parsed); cost > 0 {
					tokenStats.AddCost(cost)
				}
				// Track parallel subagents
				prevCount := len(activeAgentIDs)
				if jsonParser.IsSubagentMessage(parsed) && parsed.Type == parser.MessageTypeResult {
					parentID := *parsed.ParentToolUseID
					delete(activeAgentIDs, parentID)
				}
				for _, taskID := range jsonParser.GetTaskToolUseIDs(parsed) {
					activeAgentIDs[taskID] = true
				}
				if newCount := len(activeAgentIDs); newCount != prevCount {
					fmt.Printf("[agents] %d active\n", newCount)
				}
				// Print assistant text and tool use
				if parsed.Type == parser.MessageTypeAssistant {
					content := jsonParser.ExtractContent(parsed)
					for _, text := range content.TextContent {
						if text != "" {
							fmt.Printf("[assistant] %s\n", text)
						}
					}
					for _, item := range parsed.Message.Content {
						if item.Type == parser.ContentTypeToolUse {
							filePath := parser.ExtractFilePathFromInput(item.Input)
							if filePath != "" {
								fmt.Printf("[tool] %s: %s\n", item.Name, filePath)
							} else {
								fmt.Printf("[tool] %s\n", item.Name)
							}
						}
					}
				}
				if parsed.Type == parser.MessageTypeResult && parsed.TotalCostUSD > 0 {
					fmt.Printf("[cost] Iteration cost: $%.6f\n", parsed.TotalCostUSD)
				}
			}

		case "error":
			fmt.Fprintf(os.Stderr, "[error] %s\n", msg.Content)

		case "complete":
			fmt.Printf("[complete] %s\n", msg.Content)
			// Get final session ID
			sessionID = planLoop.GetSessionID()
		}
	}

	// Check if context was cancelled
	select {
	case <-ctx.Done():
		return 1
	default:
	}

	// Phase 2: Building
	fmt.Printf("[phase] Building (%d iterations)\n", cfg.BuildIterations)

	buildPromptLoader := prompt.NewLoader(cfg.LoopPrompt, cfg.Goal)
	buildPromptContent, err := buildPromptLoader.Load()
	if err != nil {
		fmt.Fprintf(os.Stderr, "[error] Failed to load build prompt: %v\n", err)
		return 1
	}

	buildLoop := loop.New(loop.Config{
		Iterations: cfg.BuildIterations,
		Prompt:     buildPromptContent,
	})

	// Set the resume session ID from the plan phase
	if sessionID != "" {
		buildLoop.SetResumeSessionID(sessionID)
	}

	buildLoop.Start(ctx)

	// Reset agent tracking for build phase
	activeAgentIDs = make(map[string]bool)

	// Process build loop output
	for msg := range buildLoop.Output() {
		select {
		case <-ctx.Done():
			return 1
		default:
		}

		switch msg.Type {
		case "loop_marker":
			fmt.Printf("[loop] %s\n", msg.Content)

		case "output":
			parsed := jsonParser.ParseLine(msg.Content)
			if parsed != nil {
				// Capture session ID
				if sid := jsonParser.GetSessionID(parsed); sid != "" {
					buildLoop.SetSessionID(sid)
				}
				// Track stats
				if usage := jsonParser.GetUsage(parsed); usage != nil {
					tokenStats.AddUsage(
						usage.InputTokens,
						usage.OutputTokens,
						usage.CacheCreationInputTokens,
						usage.CacheReadInputTokens,
					)
				}
				if cost := jsonParser.GetCost(parsed); cost > 0 {
					tokenStats.AddCost(cost)
				}
				// Track parallel subagents
				prevCount := len(activeAgentIDs)
				if jsonParser.IsSubagentMessage(parsed) && parsed.Type == parser.MessageTypeResult {
					parentID := *parsed.ParentToolUseID
					delete(activeAgentIDs, parentID)
				}
				for _, taskID := range jsonParser.GetTaskToolUseIDs(parsed) {
					activeAgentIDs[taskID] = true
				}
				if newCount := len(activeAgentIDs); newCount != prevCount {
					fmt.Printf("[agents] %d active\n", newCount)
				}
				// Print assistant text and tool use
				if parsed.Type == parser.MessageTypeAssistant {
					content := jsonParser.ExtractContent(parsed)
					for _, text := range content.TextContent {
						if text != "" {
							fmt.Printf("[assistant] %s\n", text)
						}
					}
					for _, item := range parsed.Message.Content {
						if item.Type == parser.ContentTypeToolUse {
							filePath := parser.ExtractFilePathFromInput(item.Input)
							if filePath != "" {
								fmt.Printf("[tool] %s: %s\n", item.Name, filePath)
							} else {
								fmt.Printf("[tool] %s\n", item.Name)
							}
						}
					}
				}
				if parsed.Type == parser.MessageTypeResult && parsed.TotalCostUSD > 0 {
					fmt.Printf("[cost] Iteration cost: $%.6f\n", parsed.TotalCostUSD)
				}
			}

		case "error":
			fmt.Fprintf(os.Stderr, "[error] %s\n", msg.Content)

		case "complete":
			fmt.Printf("[complete] %s\n", msg.Content)
			cancel()
			return 0
		}
	}

	return 0
}

// runPlanAndBuild runs plan-and-build mode: planning (1 iteration) then building (N iterations)
// in a single TUI session with mode display transitions.
func runPlanAndBuild(cfg *config.Config, tokenStats *stats.TokenStats) {
	// Set up channels for TUI communication
	msgChan := make(chan tui.Message, 100)
	doneChan := make(chan struct{})

	// Create tmux status bar (no-op if not inside tmux)
	tmuxBar := tmux.NewStatusBar()

	// Create the TUI model with channels
	model := tui.NewModelWithChannels(msgChan, doneChan)
	model.SetStats(tokenStats)
	model.SetBaseElapsed(time.Duration(tokenStats.TotalElapsedNs))
	model.SetLoopProgress(0, cfg.Iterations)
	model.SetTmuxStatusBar(tmuxBar)

	// Parse implementation plan for task counts
	completedTasks, totalTasks := parseTaskCounts(planFilePath)
	model.SetCompletedTasks(completedTasks, totalTasks)

	// Start in planning mode
	model.SetCurrentMode("Planning")

	// Create the Bubble Tea program
	program := tea.NewProgram(model, tea.WithAltScreen())

	// Set up context for graceful shutdown
	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()

	// Handle OS signals for graceful shutdown
	sigChan := make(chan os.Signal, 1)
	signal.Notify(sigChan, syscall.SIGINT, syscall.SIGTERM)
	go func() {
		<-sigChan
		cancel()
		close(doneChan)
	}()

	// Create the parser
	jsonParser := parser.NewParser()

	// Start the plan-and-build orchestration goroutine
	go runPlanAndBuildPhases(ctx, cfg, jsonParser, tokenStats, msgChan, doneChan, program)

	// Run the TUI (blocks until user quits)
	if _, err := program.Run(); err != nil {
		fmt.Fprintf(os.Stderr, "Error running TUI: %v\n", err)
		os.Exit(1)
	}
}

// runPlanAndBuildPhases orchestrates the plan and build phases sequentially
func runPlanAndBuildPhases(
	ctx context.Context,
	cfg *config.Config,
	jsonParser *parser.Parser,
	tokenStats *stats.TokenStats,
	msgChan chan<- tui.Message,
	doneChan chan struct{},
	program *tea.Program,
) {
	defer close(msgChan)

	// Phase 1: Planning
	planPromptLoader := prompt.NewPlanLoader(cfg.LoopPrompt, cfg.Goal)
	planPromptContent, err := planPromptLoader.Load()
	if err != nil {
		msgChan <- tui.Message{
			Role:    tui.RoleSystem,
			Content: fmt.Sprintf("Error loading plan prompt: %v", err),
		}
		close(doneChan)
		return
	}

	planLoop := loop.New(loop.Config{
		Iterations: cfg.Iterations, // Always 1 for plan phase
		Prompt:     planPromptContent,
	})

	// Update TUI with planning phase
	program.Send(tui.SendModeUpdate("Planning")())
	program.Send(tui.SendLoopUpdate(0, cfg.Iterations)())

	// Start the plan loop
	planLoop.Start(ctx)

	// Process plan loop output and wait for completion
	sessionID := processPlanPhase(ctx, planLoop, jsonParser, tokenStats, msgChan, program)

	// Check if context was cancelled
	select {
	case <-ctx.Done():
		return
	default:
	}

	// Phase 2: Building
	buildPromptLoader := prompt.NewLoader(cfg.LoopPrompt, cfg.Goal)
	buildPromptContent, err := buildPromptLoader.Load()
	if err != nil {
		msgChan <- tui.Message{
			Role:    tui.RoleSystem,
			Content: fmt.Sprintf("Error loading build prompt: %v", err),
		}
		close(doneChan)
		return
	}

	buildLoop := loop.New(loop.Config{
		Iterations: cfg.BuildIterations,
		Prompt:     buildPromptContent,
	})

	// Set the resume session ID from the plan phase
	if sessionID != "" {
		buildLoop.SetResumeSessionID(sessionID)
	}

	// Update TUI with building phase
	program.Send(tui.SendModeUpdate("Building")())
	program.Send(tui.SendLoopUpdate(0, cfg.BuildIterations)())
	program.Send(tui.SendLoopStarted()())

	// Start the build loop
	buildLoop.Start(ctx)

	// Process build loop output
	processBuildPhase(ctx, buildLoop, jsonParser, tokenStats, msgChan, doneChan, program)
}

// processPlanPhase processes the plan loop output and returns the captured session ID
func processPlanPhase(
	ctx context.Context,
	planLoop *loop.Loop,
	jsonParser *parser.Parser,
	tokenStats *stats.TokenStats,
	msgChan chan<- tui.Message,
	program *tea.Program,
) string {
	loopOutput := planLoop.Output()
	activeAgentIDs := make(map[string]bool)
	var loopTotalTokens int64

	for {
		select {
		case <-ctx.Done():
			return planLoop.GetSessionID()
		case msg, ok := <-loopOutput:
			if !ok {
				// Plan loop finished
				return planLoop.GetSessionID()
			}

			switch msg.Type {
			case "loop_marker":
				program.Send(tui.SendLoopUpdate(msg.Loop, msg.Total)())
				isLoopStart := strings.Contains(msg.Content, "LOOP") &&
					!strings.Contains(msg.Content, "STOPPED") &&
					!strings.Contains(msg.Content, "COMPLETED") &&
					!strings.Contains(msg.Content, "RESUMED")
				if isLoopStart {
					loopTotalTokens = 0
					program.Send(tui.SendLoopStarted()())
					program.Send(tui.SendLoopStatsUpdate(0)())
				}
				role := tui.RoleLoop
				if strings.Contains(msg.Content, "STOPPED") {
					role = tui.RoleLoopStopped
				}
				msgChan <- tui.Message{
					Role:    role,
					Content: msg.Content,
				}

			case "output":
				parsed := jsonParser.ParseLine(msg.Content)
				if parsed != nil {
					if sessionID := jsonParser.GetSessionID(parsed); sessionID != "" {
						planLoop.SetSessionID(sessionID)
					}
					handleParsedMessagePlanAndBuild(parsed, jsonParser, tokenStats, msgChan, program, activeAgentIDs, &loopTotalTokens)
				}

			case "error":
				msgChan <- tui.Message{
					Role:    tui.RoleSystem,
					Content: fmt.Sprintf("Error: %s", msg.Content),
				}

			case "complete":
				msgChan <- tui.Message{
					Role:    tui.RoleSystem,
					Content: "Planning phase completed - transitioning to build phase...",
				}
				// Return the session ID for the build phase to use
				return planLoop.GetSessionID()
			}
		}
	}
}

// processBuildPhase processes the build loop output
func processBuildPhase(
	ctx context.Context,
	buildLoop *loop.Loop,
	jsonParser *parser.Parser,
	tokenStats *stats.TokenStats,
	msgChan chan<- tui.Message,
	doneChan chan struct{},
	program *tea.Program,
) {
	loopOutput := buildLoop.Output()
	activeAgentIDs := make(map[string]bool)
	var loopTotalTokens int64

	for {
		select {
		case <-ctx.Done():
			return
		case msg, ok := <-loopOutput:
			if !ok {
				select {
				case <-doneChan:
				default:
					close(doneChan)
				}
				return
			}

			switch msg.Type {
			case "loop_marker":
				program.Send(tui.SendLoopUpdate(msg.Loop, msg.Total)())
				isLoopStart := strings.Contains(msg.Content, "LOOP") &&
					!strings.Contains(msg.Content, "STOPPED") &&
					!strings.Contains(msg.Content, "COMPLETED") &&
					!strings.Contains(msg.Content, "RESUMED")
				if isLoopStart {
					loopTotalTokens = 0
					program.Send(tui.SendLoopStarted()())
					program.Send(tui.SendLoopStatsUpdate(0)())
				}
				role := tui.RoleLoop
				if strings.Contains(msg.Content, "STOPPED") {
					role = tui.RoleLoopStopped
				}
				msgChan <- tui.Message{
					Role:    role,
					Content: msg.Content,
				}

			case "output":
				parsed := jsonParser.ParseLine(msg.Content)
				if parsed != nil {
					if sessionID := jsonParser.GetSessionID(parsed); sessionID != "" {
						buildLoop.SetSessionID(sessionID)
					}
					handleParsedMessagePlanAndBuild(parsed, jsonParser, tokenStats, msgChan, program, activeAgentIDs, &loopTotalTokens)
				}

			case "error":
				msgChan <- tui.Message{
					Role:    tui.RoleSystem,
					Content: fmt.Sprintf("Error: %s", msg.Content),
				}

			case "complete":
				msgChan <- tui.Message{
					Role:    tui.RoleSystem,
					Content: msg.Content,
				}
				program.Send(tui.SendDone()())
			}
		}
	}
}

// handleParsedMessagePlanAndBuild processes a parsed JSON message from Claude for plan-and-build mode
func handleParsedMessagePlanAndBuild(
	parsed *parser.ParsedMessage,
	jsonParser *parser.Parser,
	tokenStats *stats.TokenStats,
	msgChan chan<- tui.Message,
	program *tea.Program,
	activeAgentIDs map[string]bool,
	loopTotalTokens *int64,
) {
	// Extract usage information
	if usage := jsonParser.GetUsage(parsed); usage != nil {
		tokenStats.AddUsage(
			usage.InputTokens,
			usage.OutputTokens,
			usage.CacheCreationInputTokens,
			usage.CacheReadInputTokens,
		)
		program.Send(tui.SendStatsUpdate(tokenStats)())
		loopTokens := usage.InputTokens + usage.OutputTokens + usage.CacheCreationInputTokens + usage.CacheReadInputTokens
		*loopTotalTokens += loopTokens
		program.Send(tui.SendLoopStatsUpdate(*loopTotalTokens)())
	}

	// Extract cost from result messages
	if cost := jsonParser.GetCost(parsed); cost > 0 {
		tokenStats.AddCost(cost)
		program.Send(tui.SendStatsUpdate(tokenStats)())
	}

	// Track parallel subagents
	prevCount := len(activeAgentIDs)
	if jsonParser.IsSubagentMessage(parsed) && parsed.Type == parser.MessageTypeResult {
		parentID := *parsed.ParentToolUseID
		delete(activeAgentIDs, parentID)
	}
	for _, taskID := range jsonParser.GetTaskToolUseIDs(parsed) {
		activeAgentIDs[taskID] = true
	}
	if newCount := len(activeAgentIDs); newCount != prevCount {
		program.Send(tui.SendAgentUpdate(newCount)())
	}

	// Process message content based on type
	switch parsed.Type {
	case parser.MessageTypeSystem:
		return

	case parser.MessageTypeAssistant:
		content := jsonParser.ExtractContent(parsed)

		for _, text := range content.TextContent {
			if text != "" {
				msgChan <- tui.Message{
					Role:    tui.RoleAssistant,
					Content: text,
				}
				if ref := jsonParser.ExtractTaskReference(text); ref != nil {
					taskLabel := fmt.Sprintf("#%d", ref.Number)
					if ref.Description != "" {
						taskLabel = fmt.Sprintf("#%d %s", ref.Number, ref.Description)
					}
					program.Send(tui.SendTaskUpdate(taskLabel)())
				}
			}
		}

		for _, toolUse := range content.ToolUses {
			msgChan <- tui.Message{
				Role:    tui.RoleTool,
				Content: fmt.Sprintf("Using tool: %s", toolUse.Name),
			}
		}

	case parser.MessageTypeUser:
		content := jsonParser.ExtractContent(parsed)

		for _, toolResult := range content.ToolResults {
			if toolResult.Content != "" {
				msgChan <- tui.Message{
					Role:    tui.RoleUser,
					Content: toolResult.Content,
				}
				if ref := jsonParser.ExtractTaskReference(toolResult.Content); ref != nil {
					taskLabel := fmt.Sprintf("#%d", ref.Number)
					if ref.Description != "" {
						taskLabel = fmt.Sprintf("#%d %s", ref.Number, ref.Description)
					}
					program.Send(tui.SendTaskUpdate(taskLabel)())
				}
			}
		}

	case parser.MessageTypeResult:
		if parsed.TotalCostUSD > 0 {
			msgChan <- tui.Message{
				Role:    tui.RoleSystem,
				Content: fmt.Sprintf("Iteration cost: $%.6f", parsed.TotalCostUSD),
			}
		}
	}
}

// parseTaskCounts reads an IMPLEMENTATION_PLAN.md file and returns the number of
// completed (DONE) tasks and the total number of tasks.
func parseTaskCounts(filepath string) (completed, total int) {
	data, err := os.ReadFile(filepath)
	if err != nil {
		return 0, 0
	}

	for _, line := range strings.Split(string(data), "\n") {
		trimmed := strings.TrimSpace(line)
		if strings.HasPrefix(trimmed, "## TASK ") {
			total++
		}
		if strings.Contains(trimmed, "**Status: DONE**") || strings.Contains(trimmed, "**Status: NOT NEEDED**") {
			completed++
		}
	}
	return completed, total
}
