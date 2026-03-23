package main

import (
	"context"
	"database/sql"
	"fmt"
	"io"
	"os"
	"os/signal"
	"path/filepath"
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

const statsFilePath = ".ralph.claude_stats"
const logFilePath = ".ralph.log"

// dbContext holds database connection and session metadata for stats tracking.
type dbContext struct {
	db        *sql.DB
	sessionID string
	owner     string
	repo      string
	branch    string
}

// loopTracker tracks per-loop state for DB checkpoint flushing.
type loopTracker struct {
	currentLoopID   string
	loopStartTime   time.Time
	loopStartCost   float64
	loopStartSnap   stats.TokenStats
	lastFlushedCost float64
	lastFlushedSnap stats.TokenStats
}

// expandDBPath returns the full path to the stats database (~/.ralph/ralph_stats.db).
func expandDBPath() string {
	home, err := os.UserHomeDir()
	if err != nil {
		return ""
	}
	return filepath.Join(home, ".ralph", "ralph_stats.db")
}

// initDBContext initializes the database and session context. Best-effort: returns
// a dbContext with nil db on any error so callers can proceed without stats.
func initDBContext() *dbContext {
	dbPath := expandDBPath()
	if dbPath == "" {
		fmt.Fprintf(os.Stderr, "Warning: Could not determine home directory for stats DB\n")
		return &dbContext{}
	}

	db, err := stats.InitDB(dbPath)
	if err != nil {
		fmt.Fprintf(os.Stderr, "Warning: Could not initialize stats DB: %v\n", err)
		return &dbContext{}
	}

	sessionID, err := stats.GenerateSessionID()
	if err != nil {
		fmt.Fprintf(os.Stderr, "Warning: Could not generate session ID: %v\n", err)
		sessionID = "000000"
	}

	owner, repo, branch := stats.GetGitContext()

	return &dbContext{
		db:        db,
		sessionID: sessionID,
		owner:     owner,
		repo:      repo,
		branch:    branch,
	}
}

// exportSessionTSV exports session stats as TSV and writes to a file in the current directory.
func exportSessionTSV(dbCtx *dbContext) {
	if dbCtx == nil || dbCtx.db == nil {
		return
	}
	tsv, err := stats.ExportSessionTSV(dbCtx.db, dbCtx.sessionID)
	if err != nil {
		fmt.Fprintf(os.Stderr, "Warning: Could not export session TSV: %v\n", err)
		return
	}
	if tsv == "" {
		return
	}
	filename := dbCtx.sessionID + ".tsv"
	if err := os.WriteFile(filename, []byte(tsv), 0644); err != nil {
		fmt.Fprintf(os.Stderr, "Warning: Could not write TSV file: %v\n", err)
	}
}

// startNewLoop completes the previous loop (if any) and begins tracking a new one.
func (lt *loopTracker) startNewLoop(dbCtx *dbContext, tokenStats *stats.TokenStats, loopNum int) {
	if lt.currentLoopID != "" {
		lt.completeLoop(dbCtx, tokenStats)
	}
	snap := tokenStats.Snapshot()
	lt.currentLoopID = fmt.Sprintf("%s-%d", dbCtx.sessionID, loopNum)
	lt.loopStartTime = time.Now().UTC()
	lt.loopStartCost = snap.TotalCostUSD
	lt.loopStartSnap = snap
	lt.lastFlushedCost = snap.TotalCostUSD
	lt.lastFlushedSnap = snap
}

// flushDelta computes delta stats since last flush and writes a checkpoint row.
func (lt *loopTracker) flushDelta(dbCtx *dbContext, tokenStats *stats.TokenStats) {
	if dbCtx == nil || dbCtx.db == nil || lt.currentLoopID == "" {
		return
	}
	snap := tokenStats.Snapshot()
	deltaCost := snap.TotalCostUSD - lt.lastFlushedCost
	deltaInput := snap.InputTokens - lt.lastFlushedSnap.InputTokens
	if deltaCost <= 0 && deltaInput == 0 {
		return
	}
	err := stats.FlushCheckpoint(dbCtx.db, stats.CheckpointParams{
		LoopID:             lt.currentLoopID,
		SessionID:          dbCtx.sessionID,
		Owner:              dbCtx.owner,
		Repo:               dbCtx.repo,
		Branch:             dbCtx.branch,
		DeltaCost:          deltaCost,
		DeltaInputTokens:   snap.InputTokens - lt.lastFlushedSnap.InputTokens,
		DeltaOutputTokens:  snap.OutputTokens - lt.lastFlushedSnap.OutputTokens,
		DeltaCacheCreation: snap.CacheCreationTokens - lt.lastFlushedSnap.CacheCreationTokens,
		DeltaCacheRead:     snap.CacheReadTokens - lt.lastFlushedSnap.CacheReadTokens,
		Timestamp:          time.Now().UTC().Format(time.RFC3339),
	})
	if err != nil {
		fmt.Fprintf(os.Stderr, "Warning: checkpoint flush failed: %v\n", err)
	}
	lt.lastFlushedCost = snap.TotalCostUSD
	lt.lastFlushedSnap = snap
}

// completeLoop flushes remaining delta and writes the loop_stats summary row.
func (lt *loopTracker) completeLoop(dbCtx *dbContext, tokenStats *stats.TokenStats) {
	if dbCtx == nil || dbCtx.db == nil || lt.currentLoopID == "" {
		return
	}
	lt.flushDelta(dbCtx, tokenStats)
	snap := tokenStats.Snapshot()
	now := time.Now().UTC().Format(time.RFC3339)
	loopInput := snap.InputTokens - lt.loopStartSnap.InputTokens
	loopOutput := snap.OutputTokens - lt.loopStartSnap.OutputTokens
	loopCacheCreation := snap.CacheCreationTokens - lt.loopStartSnap.CacheCreationTokens
	loopCacheRead := snap.CacheReadTokens - lt.loopStartSnap.CacheReadTokens
	err := stats.WriteLoopStats(dbCtx.db, stats.LoopStatsParams{
		LoopID:              lt.currentLoopID,
		SessionID:           dbCtx.sessionID,
		Owner:               dbCtx.owner,
		Repo:                dbCtx.repo,
		Branch:              dbCtx.branch,
		Description:         stats.GetLatestCommitTitle(),
		TotalCost:           snap.TotalCostUSD - lt.loopStartCost,
		InputTokens:         loopInput,
		OutputTokens:        loopOutput,
		CacheCreationTokens: loopCacheCreation,
		CacheReadTokens:     loopCacheRead,
		TotalTokens:         loopInput + loopOutput + loopCacheCreation + loopCacheRead,
		StartTime:           lt.loopStartTime.Format(time.RFC3339),
		FinishTime:          now,
	})
	if err != nil {
		fmt.Fprintf(os.Stderr, "Warning: loop stats write failed: %v\n", err)
	}
	lt.currentLoopID = ""
}

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
			showLoader = prompt.NewPlanLoader("", cfg.Goal, cfg.PlanFile)
		} else if cfg.IsAutoresearchMode() {
			showLoader = prompt.NewAutoresearchLoader("", cfg.Goal, "(experiment content will be loaded at runtime)")
		} else {
			showLoader = prompt.NewLoader("", cfg.Goal, cfg.PlanFile)
		}
		content, err := showLoader.Load()
		if err != nil {
			fmt.Fprintf(os.Stderr, "Error: %v\n", err)
			os.Exit(1)
		}
		fmt.Print(content)
		return
	}

	// Handle autoresearch mode: create template and exit if experiment file doesn't exist
	if cfg.IsAutoresearchMode() {
		experimentFile := cfg.AutoresearchFile
		if experimentFile == "" {
			experimentFile = "specs/experiment.md"
		}

		info, statErr := os.Stat(experimentFile)
		if os.IsNotExist(statErr) || (statErr == nil && info.Size() == 0) {
			// Create template
			templateContent, tmplErr := prompt.GetEmbeddedAutoresearchTemplate()
			if tmplErr != nil {
				fmt.Fprintf(os.Stderr, "Error loading template: %v\n", tmplErr)
				os.Exit(1)
			}
			// Ensure specs/ directory exists
			os.MkdirAll("specs", 0755)
			templatePath := "specs/autoresearch_template.md"
			if writeErr := os.WriteFile(templatePath, []byte(templateContent), 0644); writeErr != nil {
				fmt.Fprintf(os.Stderr, "Error creating template: %v\n", writeErr)
				os.Exit(1)
			}
			fmt.Printf("Created %s\nEdit it with your experiment details, then copy to %s and run `ralph autoresearch` again.\n", templatePath, experimentFile)
			return
		} else if statErr != nil {
			fmt.Fprintf(os.Stderr, "Error checking experiment file: %v\n", statErr)
			os.Exit(1)
		}
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
	if cfg.IsAutoresearchMode() {
		experimentFile := cfg.AutoresearchFile
		if experimentFile == "" {
			experimentFile = "specs/experiment.md"
		}
		experimentContent, readErr := os.ReadFile(experimentFile)
		if readErr != nil {
			fmt.Fprintf(os.Stderr, "Error reading experiment file %s: %v\n", experimentFile, readErr)
			os.Exit(1)
		}
		promptLoader = prompt.NewAutoresearchLoader(cfg.LoopPrompt, cfg.Goal, string(experimentContent))
	} else if cfg.IsPlanMode() {
		promptLoader = prompt.NewPlanLoader(cfg.LoopPrompt, cfg.Goal, cfg.PlanFile)
	} else {
		promptLoader = prompt.NewLoader(cfg.LoopPrompt, cfg.Goal, cfg.PlanFile)
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

	// Initialize DB context for stats tracking (best-effort)
	dbCtx := initDBContext()
	if dbCtx.db != nil {
		defer dbCtx.db.Close()
	}

	// Open log file (truncated each run); fall back to io.Discard on error
	var logFile io.Writer
	logFileHandle, err := os.Create(logFilePath)
	if err != nil {
		fmt.Fprintf(os.Stderr, "Warning: Could not open log file %s: %v\n", logFilePath, err)
		logFile = io.Discard
	} else {
		logFile = logFileHandle
		defer logFileHandle.Close()
	}

	// CLI mode: run without TUI, output to stdout/stderr, exit when complete
	if cfg.CLI {
		var exitCode int
		if cfg.IsPlanAndBuildMode() {
			exitCode = runPlanAndBuildCLI(cfg, tokenStats, logFile, dbCtx)
		} else {
			exitCode = runCLI(cfg, promptContent, tokenStats, logFile, dbCtx)
		}
		if err := tokenStats.Save(statsFilePath); err != nil {
			fmt.Fprintf(os.Stderr, "Warning: Could not save stats: %v\n", err)
		}
		exportSessionTSV(dbCtx)
		os.Exit(exitCode)
	}

	// Plan-and-build mode: run planning (1 iteration) then building (N iterations) in single TUI session
	if cfg.IsPlanAndBuildMode() {
		runPlanAndBuild(cfg, tokenStats, logFile, dbCtx)
		if err := tokenStats.Save(statsFilePath); err != nil {
			fmt.Fprintf(os.Stderr, "Warning: Could not save stats: %v\n", err)
		}
		exportSessionTSV(dbCtx)
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
	completedTasks, totalTasks := parseTaskCounts(cfg.PlanFile)
	model.SetCompletedTasks(completedTasks, totalTasks)

	// Set current mode for TUI display
	if cfg.IsPlanMode() {
		model.SetCurrentMode("Planning")
	} else if cfg.IsAutoresearchMode() {
		model.SetCurrentMode("Researching")
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
	go processLoopOutput(ctx, claudeLoop, jsonParser, tokenStats, msgChan, doneChan, program, logFile, dbCtx)

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
	exportSessionTSV(dbCtx)
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
	logFile io.Writer,
	dbCtx *dbContext,
) {
	defer close(msgChan)

	loopOutput := claudeLoop.Output()
	var loopTotalTokens int64 // per-loop token tracking for tmux status bar
	var iterEstimate float64  // per-iteration estimated cost from token counts
	lt := &loopTracker{}

	// Start per-minute checkpoint ticker
	ticker := time.NewTicker(time.Minute)
	defer ticker.Stop()

	for {
		select {
		case <-ctx.Done():
			lt.completeLoop(dbCtx, tokenStats)
			return
		case <-ticker.C:
			lt.flushDelta(dbCtx, tokenStats)
		case msg, ok := <-loopOutput:
			if !ok {
				// Loop has finished
				lt.completeLoop(dbCtx, tokenStats)
				select {
				case <-doneChan:
				default:
					close(doneChan)
				}
				return
			}

			processMessage(msg, claudeLoop, jsonParser, tokenStats, msgChan, program, &loopTotalTokens, logFile, &iterEstimate, dbCtx, lt)
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
	loopTotalTokens *int64,
	logFile io.Writer,
	iterEstimate *float64,
	dbCtx *dbContext,
	lt *loopTracker,
) {
	switch msg.Type {
	case "loop_marker":
		handleLoopMarker(msg, msgChan, program, loopTotalTokens, iterEstimate, dbCtx, lt, tokenStats)

	case "output":
		// Try to parse as JSON first
		parsed := jsonParser.ParseLine(msg.Content)
		if parsed != nil {
			// Capture session ID from system messages for --resume support
			if sessionID := jsonParser.GetSessionID(parsed); sessionID != "" {
				claudeLoop.SetSessionID(sessionID)
			}
			handleParsedMessage(parsed, claudeLoop, jsonParser, tokenStats, msgChan, program, loopTotalTokens, logFile, iterEstimate)
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
		lt.completeLoop(dbCtx, tokenStats)
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

// handleLoopMarker processes a loop_marker message for TUI mode.
// Shared by processMessage, processPlanPhase, and processBuildPhase.
func handleLoopMarker(msg loop.Message, msgChan chan<- tui.Message, program *tea.Program, loopTotalTokens *int64, iterEstimate *float64, dbCtx *dbContext, lt *loopTracker, tokenStats *stats.TokenStats) {
	program.Send(tui.SendLoopUpdate(msg.Loop, msg.Total)())
	// Detect new loop iteration start (not STOPPED/COMPLETED/RESUMED)
	if isNewLoopStart(msg.Content) {
		lt.startNewLoop(dbCtx, tokenStats, msg.Loop)
		*loopTotalTokens = 0
		*iterEstimate = 0
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
}

// handleParsedMessage processes a parsed JSON message from Claude for TUI mode.
// Shared by standard mode and plan-and-build mode.
func handleParsedMessage(
	parsed *parser.ParsedMessage,
	claudeLoop *loop.Loop,
	jsonParser *parser.Parser,
	tokenStats *stats.TokenStats,
	msgChan chan<- tui.Message,
	program *tea.Program,
	loopTotalTokens *int64,
	logFile io.Writer,
	iterEstimate *float64,
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
		// Estimate cost from token counts and update in real-time
		estimate := stats.EstimateCostFromTokens(
			usage.InputTokens,
			usage.OutputTokens,
			usage.CacheCreationInputTokens,
			usage.CacheReadInputTokens,
		)
		tokenStats.AddCost(estimate)
		*iterEstimate += estimate
		program.Send(tui.SendStatsUpdate(tokenStats)())
		// Also track per-loop tokens for tmux status bar
		loopTokens := usage.InputTokens + usage.OutputTokens + usage.CacheCreationInputTokens + usage.CacheReadInputTokens
		*loopTotalTokens += loopTokens
		program.Send(tui.SendLoopStatsUpdate(*loopTotalTokens)())
	}

	// Extract cost from result messages — reconcile estimate with actual
	if cost := jsonParser.GetCost(parsed); cost > 0 {
		if !jsonParser.IsSubagentMessage(parsed) {
			// Main iteration result: replace accumulated estimate with actual cost
			tokenStats.ReconcileCost(*iterEstimate, cost)
			*iterEstimate = 0
		} else {
			// Subagent result: add actual cost directly
			tokenStats.AddCost(cost)
		}
		program.Send(tui.SendStatsUpdate(tokenStats)())
	}

	// Process message content based on type
	switch parsed.Type {
	case parser.MessageTypeSystem:
		// Skip system messages (as Python version does)
		return

	case parser.MessageTypeAssistant:
		content := jsonParser.ExtractContent(parsed)

		// Display thinking blocks
		if content.Thinking != "" {
			msgChan <- tui.Message{
				Role:    tui.RoleThinking,
				Content: content.Thinking,
			}
			fmt.Fprintf(logFile, "[thinking] %s\n\n", content.Thinking)
		}

		// Display text content and scan for task references
		for _, text := range content.TextContent {
			if text != "" {
				msgChan <- tui.Message{
					Role:    tui.RoleAssistant,
					Content: text,
				}
				fmt.Fprintf(logFile, "[assistant] %s\n\n", text)
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

		// Display tool uses with file path info
		for _, toolUse := range content.ToolUses {
			toolMsg := fmt.Sprintf("Using tool: %s", toolUse.Name)
			if toolUse.FilePath != "" {
				toolMsg = fmt.Sprintf("Using tool: %s — %s", toolUse.Name, toolUse.FilePath)
			}
			msgChan <- tui.Message{
				Role:    tui.RoleTool,
				Content: toolMsg,
			}
		}

	case parser.MessageTypeUser:
		// Skip tool result content in TUI mode (file dumps are too verbose).
		// Still scan for task references in the results.
		content := jsonParser.ExtractContent(parsed)
		for _, toolResult := range content.ToolResults {
			if toolResult.Content != "" {
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

// handleParsedMessageCLI processes a parsed JSON message for CLI mode output.
// Shared by runCLI and both phases of runPlanAndBuildCLI.
func handleParsedMessageCLI(
	parsed *parser.ParsedMessage,
	claudeLoop *loop.Loop,
	jsonParser *parser.Parser,
	tokenStats *stats.TokenStats,
	logFile io.Writer,
	iterEstimate *float64,
) {
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
		// Estimate cost from token counts and update in real-time
		estimate := stats.EstimateCostFromTokens(
			usage.InputTokens,
			usage.OutputTokens,
			usage.CacheCreationInputTokens,
			usage.CacheReadInputTokens,
		)
		tokenStats.AddCost(estimate)
		*iterEstimate += estimate
	}
	// Extract cost from result messages — reconcile estimate with actual
	if cost := jsonParser.GetCost(parsed); cost > 0 {
		if !jsonParser.IsSubagentMessage(parsed) {
			// Main iteration result: replace accumulated estimate with actual cost
			tokenStats.ReconcileCost(*iterEstimate, cost)
			*iterEstimate = 0
		} else {
			// Subagent result: add actual cost directly
			tokenStats.AddCost(cost)
		}
	}
	// Print assistant text and tool use
	if parsed.Type == parser.MessageTypeAssistant {
		content := jsonParser.ExtractContent(parsed)
		if content.Thinking != "" {
			fmt.Fprintf(logFile, "[thinking] %s\n\n", content.Thinking)
		}
		for _, text := range content.TextContent {
			if text != "" {
				fmt.Printf("[assistant] %s\n", text)
				fmt.Fprintf(logFile, "[assistant] %s\n\n", text)
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

// runCLI runs ralph in CLI mode: no TUI, output to stdout/stderr, exit on completion.
func runCLI(cfg *config.Config, promptContent string, tokenStats *stats.TokenStats, logFile io.Writer, dbCtx *dbContext) int {
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
	var iterEstimate float64
	lt := &loopTracker{}

	mode := "build"
	if cfg.IsPlanMode() {
		mode = "plan"
	} else if cfg.IsAutoresearchMode() {
		mode = "autoresearch"
	}
	fmt.Printf("ralph cli: starting %s mode with %d iterations\n", mode, cfg.Iterations)

	// Start per-minute checkpoint ticker
	ticker := time.NewTicker(time.Minute)
	defer ticker.Stop()

	loopOutput := claudeLoop.Output()
	for {
		select {
		case <-ctx.Done():
			lt.completeLoop(dbCtx, tokenStats)
			return 1
		case <-ticker.C:
			lt.flushDelta(dbCtx, tokenStats)
		case msg, ok := <-loopOutput:
			if !ok {
				lt.completeLoop(dbCtx, tokenStats)
				return 0
			}

			switch msg.Type {
			case "loop_marker":
				if isNewLoopStart(msg.Content) {
					lt.startNewLoop(dbCtx, tokenStats, msg.Loop)
					iterEstimate = 0
				}
				fmt.Printf("[loop] %s\n", msg.Content)

			case "output":
				parsed := jsonParser.ParseLine(msg.Content)
				if parsed != nil {
					if sessionID := jsonParser.GetSessionID(parsed); sessionID != "" {
						claudeLoop.SetSessionID(sessionID)
					}
					handleParsedMessageCLI(parsed, claudeLoop, jsonParser, tokenStats, logFile, &iterEstimate)
				}

			case "error":
				fmt.Fprintf(os.Stderr, "[error] %s\n", msg.Content)

			case "complete":
				lt.completeLoop(dbCtx, tokenStats)
				fmt.Printf("[complete] %s\n", msg.Content)
				// In CLI mode, exit on completion instead of waiting
				cancel()
				return 0
			}
		}
	}
}

// runPlanAndBuildCLI runs plan-and-build mode in CLI: planning (1 iteration) then building (N iterations)
// with output to stdout/stderr and no TUI.
func runPlanAndBuildCLI(cfg *config.Config, tokenStats *stats.TokenStats, logFile io.Writer, dbCtx *dbContext) int {
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

	fmt.Println("ralph cli: starting plan-and-build mode")

	// Phase 1: Planning
	fmt.Printf("[phase] Planning (%d iteration)\n", cfg.Iterations)

	planPromptLoader := prompt.NewPlanLoader(cfg.LoopPrompt, cfg.Goal, cfg.PlanFile)
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
	var planIterEstimate float64
	planLt := &loopTracker{}

	// Start per-minute checkpoint ticker for plan phase
	planTicker := time.NewTicker(time.Minute)

	// Process plan loop output
	planOutput := planLoop.Output()
planLoop:
	for {
		select {
		case <-ctx.Done():
			planLt.completeLoop(dbCtx, tokenStats)
			planTicker.Stop()
			return 1
		case <-planTicker.C:
			planLt.flushDelta(dbCtx, tokenStats)
		case msg, ok := <-planOutput:
			if !ok {
				planLt.completeLoop(dbCtx, tokenStats)
				break planLoop
			}

			switch msg.Type {
			case "loop_marker":
				if isNewLoopStart(msg.Content) {
					planLt.startNewLoop(dbCtx, tokenStats, msg.Loop)
					planIterEstimate = 0
				}
				fmt.Printf("[loop] %s\n", msg.Content)

			case "output":
				parsed := jsonParser.ParseLine(msg.Content)
				if parsed != nil {
					if sid := jsonParser.GetSessionID(parsed); sid != "" {
						planLoop.SetSessionID(sid)
						sessionID = sid
					}
					handleParsedMessageCLI(parsed, planLoop, jsonParser, tokenStats, logFile, &planIterEstimate)
				}

			case "error":
				fmt.Fprintf(os.Stderr, "[error] %s\n", msg.Content)

			case "complete":
				planLt.completeLoop(dbCtx, tokenStats)
				fmt.Printf("[complete] %s\n", msg.Content)
				// Get final session ID
				sessionID = planLoop.GetSessionID()
				break planLoop
			}
		}
	}
	planTicker.Stop()

	// Check if context was cancelled
	select {
	case <-ctx.Done():
		return 1
	default:
	}

	// Phase 2: Building
	fmt.Printf("[phase] Building (%d iterations)\n", cfg.BuildIterations)

	buildPromptLoader := prompt.NewLoader(cfg.LoopPrompt, cfg.Goal, cfg.PlanFile)
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

	var buildIterEstimate float64
	buildLt := &loopTracker{}

	// Start per-minute checkpoint ticker for build phase
	buildTicker := time.NewTicker(time.Minute)
	defer buildTicker.Stop()

	// Process build loop output
	buildOutput := buildLoop.Output()
	for {
		select {
		case <-ctx.Done():
			buildLt.completeLoop(dbCtx, tokenStats)
			return 1
		case <-buildTicker.C:
			buildLt.flushDelta(dbCtx, tokenStats)
		case msg, ok := <-buildOutput:
			if !ok {
				buildLt.completeLoop(dbCtx, tokenStats)
				return 0
			}

			switch msg.Type {
			case "loop_marker":
				if isNewLoopStart(msg.Content) {
					buildLt.startNewLoop(dbCtx, tokenStats, msg.Loop)
					buildIterEstimate = 0
				}
				fmt.Printf("[loop] %s\n", msg.Content)

			case "output":
				parsed := jsonParser.ParseLine(msg.Content)
				if parsed != nil {
					if sid := jsonParser.GetSessionID(parsed); sid != "" {
						buildLoop.SetSessionID(sid)
					}
					handleParsedMessageCLI(parsed, buildLoop, jsonParser, tokenStats, logFile, &buildIterEstimate)
				}

			case "error":
				fmt.Fprintf(os.Stderr, "[error] %s\n", msg.Content)

			case "complete":
				buildLt.completeLoop(dbCtx, tokenStats)
				fmt.Printf("[complete] %s\n", msg.Content)
				cancel()
				return 0
			}
		}
	}
}

// runPlanAndBuild runs plan-and-build mode: planning (1 iteration) then building (N iterations)
// in a single TUI session with mode display transitions.
func runPlanAndBuild(cfg *config.Config, tokenStats *stats.TokenStats, logFile io.Writer, dbCtx *dbContext) {
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
	completedTasks, totalTasks := parseTaskCounts(cfg.PlanFile)
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
	go runPlanAndBuildPhases(ctx, cfg, jsonParser, tokenStats, msgChan, doneChan, program, logFile, dbCtx)

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
	logFile io.Writer,
	dbCtx *dbContext,
) {
	defer close(msgChan)

	// Phase 1: Planning
	planPromptLoader := prompt.NewPlanLoader(cfg.LoopPrompt, cfg.Goal, cfg.PlanFile)
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

	// Update TUI with planning phase and set loop reference for hotkey control
	program.Send(tui.SendModeUpdate("Planning")())
	program.Send(tui.SendLoopUpdate(0, cfg.Iterations)())
	program.Send(tui.SendLoopRef(planLoop)())

	// Start the plan loop
	planLoop.Start(ctx)

	// Process plan loop output and wait for completion
	sessionID := processPlanPhase(ctx, planLoop, jsonParser, tokenStats, msgChan, program, logFile, dbCtx)

	// Check if context was cancelled
	select {
	case <-ctx.Done():
		return
	default:
	}

	// Phase 2: Building
	buildPromptLoader := prompt.NewLoader(cfg.LoopPrompt, cfg.Goal, cfg.PlanFile)
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

	// Update TUI with building phase and swap loop reference for hotkey control
	program.Send(tui.SendModeUpdate("Building")())
	program.Send(tui.SendLoopUpdate(0, cfg.BuildIterations)())
	program.Send(tui.SendLoopStarted()())
	program.Send(tui.SendLoopRef(buildLoop)())

	// Start the build loop
	buildLoop.Start(ctx)

	// Process build loop output
	processBuildPhase(ctx, buildLoop, jsonParser, tokenStats, msgChan, doneChan, program, logFile, dbCtx)
}

// processPlanPhase processes the plan loop output and returns the captured session ID
func processPlanPhase(
	ctx context.Context,
	planLoop *loop.Loop,
	jsonParser *parser.Parser,
	tokenStats *stats.TokenStats,
	msgChan chan<- tui.Message,
	program *tea.Program,
	logFile io.Writer,
	dbCtx *dbContext,
) string {
	loopOutput := planLoop.Output()
	var loopTotalTokens int64
	var iterEstimate float64
	lt := &loopTracker{}

	// Start per-minute checkpoint ticker
	ticker := time.NewTicker(time.Minute)
	defer ticker.Stop()

	for {
		select {
		case <-ctx.Done():
			lt.completeLoop(dbCtx, tokenStats)
			return planLoop.GetSessionID()
		case <-ticker.C:
			lt.flushDelta(dbCtx, tokenStats)
		case msg, ok := <-loopOutput:
			if !ok {
				// Plan loop finished
				lt.completeLoop(dbCtx, tokenStats)
				return planLoop.GetSessionID()
			}

			switch msg.Type {
			case "loop_marker":
				handleLoopMarker(msg, msgChan, program, &loopTotalTokens, &iterEstimate, dbCtx, lt, tokenStats)

			case "output":
				parsed := jsonParser.ParseLine(msg.Content)
				if parsed != nil {
					if sessionID := jsonParser.GetSessionID(parsed); sessionID != "" {
						planLoop.SetSessionID(sessionID)
					}
					handleParsedMessage(parsed, planLoop, jsonParser, tokenStats, msgChan, program, &loopTotalTokens, logFile, &iterEstimate)
				}

			case "error":
				msgChan <- tui.Message{
					Role:    tui.RoleSystem,
					Content: fmt.Sprintf("Error: %s", msg.Content),
				}

			case "complete":
				lt.completeLoop(dbCtx, tokenStats)
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
	logFile io.Writer,
	dbCtx *dbContext,
) {
	loopOutput := buildLoop.Output()
	var loopTotalTokens int64
	var iterEstimate float64
	lt := &loopTracker{}

	// Start per-minute checkpoint ticker
	ticker := time.NewTicker(time.Minute)
	defer ticker.Stop()

	for {
		select {
		case <-ctx.Done():
			lt.completeLoop(dbCtx, tokenStats)
			return
		case <-ticker.C:
			lt.flushDelta(dbCtx, tokenStats)
		case msg, ok := <-loopOutput:
			if !ok {
				lt.completeLoop(dbCtx, tokenStats)
				select {
				case <-doneChan:
				default:
					close(doneChan)
				}
				return
			}

			switch msg.Type {
			case "loop_marker":
				handleLoopMarker(msg, msgChan, program, &loopTotalTokens, &iterEstimate, dbCtx, lt, tokenStats)

			case "output":
				parsed := jsonParser.ParseLine(msg.Content)
				if parsed != nil {
					if sessionID := jsonParser.GetSessionID(parsed); sessionID != "" {
						buildLoop.SetSessionID(sessionID)
					}
					handleParsedMessage(parsed, buildLoop, jsonParser, tokenStats, msgChan, program, &loopTotalTokens, logFile, &iterEstimate)
				}

			case "error":
				msgChan <- tui.Message{
					Role:    tui.RoleSystem,
					Content: fmt.Sprintf("Error: %s", msg.Content),
				}

			case "complete":
				lt.completeLoop(dbCtx, tokenStats)
				msgChan <- tui.Message{
					Role:    tui.RoleSystem,
					Content: msg.Content,
				}
				program.Send(tui.SendDone()())
			}
		}
	}
}

// isNewLoopStart returns true if content represents a new loop iteration start
// (contains "LOOP" but not STOPPED/COMPLETED/RESUMED).
func isNewLoopStart(content string) bool {
	return strings.Contains(content, "LOOP") &&
		!strings.Contains(content, "STOPPED") &&
		!strings.Contains(content, "COMPLETED") &&
		!strings.Contains(content, "RESUMED")
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
