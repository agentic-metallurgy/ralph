package tests

import (
	"context"
	"flag"
	"os"
	"strings"
	"testing"
	"time"

	tea "github.com/charmbracelet/bubbletea"
	"github.com/cloudosai/ralph-go/internal/config"
	"github.com/cloudosai/ralph-go/internal/prompt"
	"github.com/cloudosai/ralph-go/internal/tui"
)

// =============================================================================
// BDD Test Suite: User Runs Autoresearch Mode
//
// User goal: As a user, I want to run an optimization loop that systematically
// improves results through experimentation — the TUI should show "Researching"
// mode, the prompt should include my experiment content and iteration info, and
// when no experiment file exists I should get a useful template.
// =============================================================================

// --- Scenario 1: TUI displays "Researching" mode label ---

func TestBDD_AutoresearchMode_DisplaysResearchingLabel(t *testing.T) {
	// Given: a ready TUI model with mode set to "Researching"
	m := setupReadyModel()
	m.SetCurrentMode("Researching")

	// Force a re-render via tick to pick up mode
	m, _ = updateModel(m, tui.TickMsgForTest())

	// Then: the footer shows "Researching" as the current mode
	if !viewContains(m, "Researching") {
		t.Errorf("Expected footer to display 'Researching' mode label, got:\n%s", m.View())
	}
}

func TestBDD_AutoresearchMode_ResearchingLabelViaMessage(t *testing.T) {
	// Given: a ready model with no mode set
	m := setupReadyModel()

	// When: a mode update message sets "Researching"
	m, _ = sendTuiMsg(m, tui.SendModeUpdate("Researching"))

	// Then: the footer displays "Researching"
	if !viewContains(m, "Researching") {
		t.Error("After SendModeUpdate('Researching'), footer should display 'Researching'")
	}
}

func TestBDD_AutoresearchMode_ResearchingLabelPersistsThroughTick(t *testing.T) {
	// Given: a model in "Researching" mode
	m := setupReadyModel()
	m, _ = sendTuiMsg(m, tui.SendModeUpdate("Researching"))
	if !viewContains(m, "Researching") {
		t.Fatal("Precondition: should show 'Researching' mode")
	}

	// When: a tick occurs (timer refresh)
	m, _ = updateModel(m, tui.TickMsgForTest())

	// Then: "Researching" is still displayed
	if !viewContains(m, "Researching") {
		t.Error("'Researching' mode label should persist through tick refresh")
	}
}

// --- Scenario 2: "Researching" mode coexists with loop progress ---

func TestBDD_AutoresearchMode_ResearchingWithLoopProgress(t *testing.T) {
	// Given: a model in "Researching" mode with loop progress
	m, _ := setupReadyModelWithLoop(2, 5)
	m, _ = sendTuiMsg(m, tui.SendModeUpdate("Researching"))

	// Then: both the mode label and loop progress are visible
	if !viewContains(m, "Researching") {
		t.Error("Expected 'Researching' mode in footer")
	}
	if !viewContains(m, "#2/5") {
		t.Error("Expected loop progress '#2/5' in footer")
	}
}

func TestBDD_AutoresearchMode_ResearchingWithActivityMessages(t *testing.T) {
	// Given: a model in "Researching" mode
	m := setupReadyModel()
	m, _ = sendTuiMsg(m, tui.SendModeUpdate("Researching"))

	// When: activity messages arrive
	m, _ = sendTuiMsg(m, tui.SendMessage(tui.Message{Role: tui.RoleAssistant, Content: "Running experiment baseline"}))
	m, _ = sendTuiMsg(m, tui.SendMessage(tui.Message{Role: tui.RoleTool, Content: "Editing train.py"}))

	// Then: both the mode label and messages are visible
	if !viewContains(m, "Researching") {
		t.Error("'Researching' mode should persist while activity messages arrive")
	}
	if !viewContains(m, "Running experiment baseline") {
		t.Error("Activity message should be visible")
	}
	if !viewContains(m, "Editing train.py") {
		t.Error("Tool message should be visible")
	}
}

// --- Scenario 3: "Researching" mode persists through completion ---

func TestBDD_AutoresearchMode_ResearchingPersistsThroughCompletion(t *testing.T) {
	// Given: a model in "Researching" mode at loop 5/5
	m, _ := setupReadyModelWithLoop(5, 5)
	m, _ = sendTuiMsg(m, tui.SendModeUpdate("Researching"))

	// When: done signal is received
	m, _ = sendTuiMsg(m, tui.SendDone())

	// Then: mode still shows "Researching" and status shows "COMPLETED"
	if !viewContains(m, "Researching") {
		t.Error("'Researching' mode label should persist after completion")
	}
	if !viewContains(m, "COMPLETED") {
		t.Error("Status should show 'COMPLETED' after done signal")
	}
}

// --- Scenario 4: "Researching" differs from "Building" and "Planning" ---

func TestBDD_AutoresearchMode_ResearchingIsDistinctFromOtherModes(t *testing.T) {
	// Given: a ready model
	m := setupReadyModel()

	// When: mode is set to "Researching"
	m, _ = sendTuiMsg(m, tui.SendModeUpdate("Researching"))

	// Then: "Researching" is shown, not "Building" or "Planning"
	if !viewContains(m, "Researching") {
		t.Error("Expected 'Researching' mode label")
	}
	if viewContains(m, "Building") {
		t.Error("'Building' should not appear when mode is 'Researching'")
	}
	if viewContains(m, "Planning") {
		t.Error("'Planning' should not appear when mode is 'Researching'")
	}
}

// --- Scenario 5: Mode transitions to and from "Researching" ---

func TestBDD_AutoresearchMode_TransitionFromBuildingToResearching(t *testing.T) {
	// Given: a model in "Building" mode
	m := setupReadyModel()
	m, _ = sendTuiMsg(m, tui.SendModeUpdate("Building"))
	if !viewContains(m, "Building") {
		t.Fatal("Precondition: should show 'Building' mode")
	}

	// When: mode changes to "Researching"
	m, _ = sendTuiMsg(m, tui.SendModeUpdate("Researching"))

	// Then: "Researching" replaces "Building"
	if !viewContains(m, "Researching") {
		t.Error("Expected 'Researching' after mode transition")
	}
}

// --- Scenario 6: Autoresearch prompt contains experiment content ---

func TestBDD_AutoresearchMode_PromptIncludesExperimentContent(t *testing.T) {
	// Given: an experiment specification describing optimization
	experimentSpec := `# Experiment: Optimize API Latency
## Goal
Reduce p99 latency below 50ms.
## In-Scope Files
- internal/handler.go
## Evaluator
Run benchmarks and report p99.`

	// When: the autoresearch prompt loader builds the prompt
	loader := prompt.NewAutoresearchLoader("", "", experimentSpec)
	content, err := loader.Load()
	if err != nil {
		t.Fatalf("Failed to load autoresearch prompt: %v", err)
	}

	// Then: the prompt contains the full experiment specification
	if !strings.Contains(content, "Optimize API Latency") {
		t.Error("Prompt should contain the experiment title")
	}
	if !strings.Contains(content, "Reduce p99 latency below 50ms") {
		t.Error("Prompt should contain the experiment goal")
	}
	if !strings.Contains(content, "internal/handler.go") {
		t.Error("Prompt should contain the in-scope files")
	}
}

// --- Scenario 7: Autoresearch prompt includes iteration placeholders ---

func TestBDD_AutoresearchMode_PromptHasIterationPlaceholders(t *testing.T) {
	// Given: an autoresearch prompt loaded with experiment content
	loader := prompt.NewAutoresearchLoader("", "", "my experiment")
	content, err := loader.Load()
	if err != nil {
		t.Fatalf("Failed to load prompt: %v", err)
	}

	// Then: the prompt contains $loop_iteration and $loop_total placeholders
	// (these are substituted by the loop at runtime, not by the prompt loader)
	if !strings.Contains(content, "$loop_iteration") {
		t.Error("Autoresearch prompt should contain $loop_iteration placeholder for the loop to substitute")
	}
	if !strings.Contains(content, "$loop_total") {
		t.Error("Autoresearch prompt should contain $loop_total placeholder for the loop to substitute")
	}
}

// --- Scenario 8: Autoresearch prompt includes goal when provided ---

func TestBDD_AutoresearchMode_PromptIncludesGoalWhenProvided(t *testing.T) {
	// Given: a user provides a goal for the autoresearch
	goal := "Minimize inference latency while maintaining accuracy above 95%"

	// When: the prompt is loaded with the goal
	loader := prompt.NewAutoresearchLoader("", goal, "experiment content")
	content, err := loader.Load()
	if err != nil {
		t.Fatalf("Failed to load prompt: %v", err)
	}

	// Then: the goal appears in the prompt
	if !strings.Contains(content, goal) {
		t.Error("Prompt should include the user's goal text")
	}
	// And: the placeholder is gone
	if strings.Contains(content, "$ultimate_goal_placeholder_sentence") {
		t.Error("Goal placeholder should be replaced when a goal is provided")
	}
}

func TestBDD_AutoresearchMode_PromptCleanWhenGoalEmpty(t *testing.T) {
	// Given: no goal is provided
	loader := prompt.NewAutoresearchLoader("", "", "experiment content")
	content, err := loader.Load()
	if err != nil {
		t.Fatalf("Failed to load prompt: %v", err)
	}

	// Then: the goal placeholder is cleanly removed (not left as a visible artifact)
	if strings.Contains(content, "$ultimate_goal_placeholder_sentence") {
		t.Error("Goal placeholder should be removed when goal is empty")
	}
}

// --- Scenario 9: Template creation for new experiments ---

func TestBDD_AutoresearchMode_TemplateAvailableForNewExperiments(t *testing.T) {
	// Given: a user runs autoresearch for the first time (no experiment file)
	// When: the template is requested
	template, err := prompt.GetEmbeddedAutoresearchTemplate()

	// Then: a non-empty template is returned
	if err != nil {
		t.Fatalf("Failed to load autoresearch template: %v", err)
	}
	if len(template) == 0 {
		t.Fatal("Template should not be empty")
	}
}

func TestBDD_AutoresearchMode_TemplateContainsRequiredSections(t *testing.T) {
	// Given: the embedded autoresearch template
	template, err := prompt.GetEmbeddedAutoresearchTemplate()
	if err != nil {
		t.Fatalf("Failed to load template: %v", err)
	}

	// Then: the template contains all required sections for the user to fill in
	requiredSections := []string{
		"Goal",
		"In-Scope Files",
		"Evaluator",
		"Constraints",
	}
	for _, section := range requiredSections {
		if !strings.Contains(template, section) {
			t.Errorf("Template should contain '%s' section", section)
		}
	}
}

func TestBDD_AutoresearchMode_TemplateIsWritableToFile(t *testing.T) {
	// Given: the embedded template content
	template, err := prompt.GetEmbeddedAutoresearchTemplate()
	if err != nil {
		t.Fatalf("Failed to load template: %v", err)
	}

	// When: it is written to a temp file (simulating template creation)
	tmpDir := t.TempDir()
	tmpFile := tmpDir + "/autoresearch_template.md"
	err = os.WriteFile(tmpFile, []byte(template), 0644)
	if err != nil {
		t.Fatalf("Failed to write template to file: %v", err)
	}

	// Then: the file can be read back and matches
	readBack, err := os.ReadFile(tmpFile)
	if err != nil {
		t.Fatalf("Failed to read template back: %v", err)
	}
	if string(readBack) != template {
		t.Error("Template read back from file should match original")
	}
}

// --- Scenario 10: Config detects autoresearch subcommand ---

func TestBDD_AutoresearchMode_SubcommandSetsResearchingContext(t *testing.T) {
	// Given: the user runs "ralph autoresearch"
	origArgs := os.Args
	origCommandLine := flag.CommandLine
	defer func() {
		os.Args = origArgs
		flag.CommandLine = origCommandLine
	}()

	flag.CommandLine = flag.NewFlagSet("test", flag.ContinueOnError)
	os.Args = []string{"ralph", "autoresearch"}

	// When: the subcommand is detected and flags are parsed
	cfg := config.ParseFlags()

	// Then: the config is in autoresearch mode
	if !cfg.IsAutoresearchMode() {
		t.Error("Config should detect autoresearch mode from subcommand")
	}
	// And: AutoresearchFile defaults to empty (will use specs/experiment.md)
	if cfg.AutoresearchFile != "" {
		t.Errorf("AutoresearchFile should default to empty, got %q", cfg.AutoresearchFile)
	}
}

func TestBDD_AutoresearchMode_CustomExperimentFileFromArgs(t *testing.T) {
	// Given: the user runs "ralph autoresearch my_experiment.md"
	origArgs := os.Args
	origCommandLine := flag.CommandLine
	defer func() {
		os.Args = origArgs
		flag.CommandLine = origCommandLine
	}()

	flag.CommandLine = flag.NewFlagSet("test", flag.ContinueOnError)
	os.Args = []string{"ralph", "autoresearch", "my_experiment.md"}

	// When: the subcommand is detected and flags are parsed
	cfg := config.ParseFlags()

	// Then: AutoresearchFile is set to the custom filename
	if cfg.AutoresearchFile != "my_experiment.md" {
		t.Errorf("Expected AutoresearchFile = 'my_experiment.md', got %q", cfg.AutoresearchFile)
	}
}

// --- Scenario 11: Autoresearch skips spec folder validation ---

func TestBDD_AutoresearchMode_RunsWithoutSpecsDirectory(t *testing.T) {
	// Given: autoresearch mode with a spec folder that doesn't exist
	cfg := &config.Config{
		Subcommand: "autoresearch",
		SpecFolder: "/nonexistent/specs/path",
		Iterations: config.DefaultIterations,
	}

	// When: config is validated
	err := cfg.Validate()

	// Then: validation passes (spec folder check is skipped)
	if err != nil {
		t.Errorf("Autoresearch mode should skip spec folder validation, got error: %v", err)
	}
}

// --- Scenario 12: Autoresearch prompt structure ---

func TestBDD_AutoresearchMode_PromptContainsWorkflowInstructions(t *testing.T) {
	// Given: an autoresearch prompt with experiment content
	loader := prompt.NewAutoresearchLoader("", "", "optimize my model")
	content, err := loader.Load()
	if err != nil {
		t.Fatalf("Failed to load prompt: %v", err)
	}

	// Then: the prompt contains the key workflow steps
	workflowSteps := []string{
		"Review",
		"Plan",
		"Implement",
		"Run",
		"Record",
		"Commit",
	}
	for _, step := range workflowSteps {
		if !strings.Contains(content, step) {
			t.Errorf("Autoresearch prompt should contain workflow step '%s'", step)
		}
	}
}

func TestBDD_AutoresearchMode_PromptContainsResultsTsvFormat(t *testing.T) {
	// Given: the autoresearch prompt
	loader := prompt.NewAutoresearchLoader("", "", "experiment")
	content, err := loader.Load()
	if err != nil {
		t.Fatalf("Failed to load prompt: %v", err)
	}

	// Then: the prompt describes the results.tsv logging format
	if !strings.Contains(content, "results.tsv") {
		t.Error("Prompt should reference results.tsv for logging")
	}
	if !strings.Contains(content, "val_bpb") {
		t.Error("Prompt should describe val_bpb column")
	}
	if !strings.Contains(content, "memory_gb") {
		t.Error("Prompt should describe memory_gb column")
	}
}

func TestBDD_AutoresearchMode_PromptContainsSimplicityCriterion(t *testing.T) {
	// Given: the autoresearch prompt
	loader := prompt.NewAutoresearchLoader("", "", "experiment")
	content, err := loader.Load()
	if err != nil {
		t.Fatalf("Failed to load prompt: %v", err)
	}

	// Then: the prompt includes the simplicity criterion from the spec
	if !strings.Contains(content, "Simplicity") {
		t.Error("Prompt should contain simplicity criterion")
	}
	if !strings.Contains(content, "simpler") {
		t.Error("Prompt should reference preference for simpler solutions")
	}
}

func TestBDD_AutoresearchMode_PromptContainsFirstRunBaseline(t *testing.T) {
	// Given: the autoresearch prompt
	loader := prompt.NewAutoresearchLoader("", "", "experiment")
	content, err := loader.Load()
	if err != nil {
		t.Fatalf("Failed to load prompt: %v", err)
	}

	// Then: the prompt tells the agent to establish a baseline on first run
	if !strings.Contains(content, "iteration 1") || !strings.Contains(content, "baseline") {
		t.Error("Prompt should instruct the agent to establish a baseline on the first iteration")
	}
}

// --- Scenario 13: Autoresearch prompt differs from build and plan prompts ---

func TestBDD_AutoresearchMode_PromptIsDistinctFromBuildAndPlan(t *testing.T) {
	// Given: all three prompt types
	arLoader := prompt.NewAutoresearchLoader("", "", "experiment")
	arContent, err := arLoader.Load()
	if err != nil {
		t.Fatalf("Failed to load autoresearch prompt: %v", err)
	}

	buildContent, err := prompt.GetEmbeddedPrompt()
	if err != nil {
		t.Fatalf("Failed to load build prompt: %v", err)
	}

	planContent, err := prompt.GetEmbeddedPlanPrompt()
	if err != nil {
		t.Fatalf("Failed to load plan prompt: %v", err)
	}

	// Then: the autoresearch prompt is different from both
	if arContent == buildContent {
		t.Error("Autoresearch prompt should differ from build prompt")
	}
	if arContent == planContent {
		t.Error("Autoresearch prompt should differ from plan prompt")
	}

	// And: autoresearch prompt contains autoresearch-specific keywords
	if !strings.Contains(arContent, "optimization") || !strings.Contains(arContent, "experiment") {
		t.Error("Autoresearch prompt should contain optimization/experiment keywords")
	}
}

// --- Scenario 14: TUI hotkey behavior in Researching mode ---

func TestBDD_AutoresearchMode_HotkeysWorkInResearchingMode(t *testing.T) {
	// Given: a running model in "Researching" mode with a loop
	m, l := setupReadyModelWithLoop(1, 5)
	m, _ = sendTuiMsg(m, tui.SendModeUpdate("Researching"))

	// Start the loop so pause can work
	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()
	l.Start(ctx)

	// When: user presses '+' to add an iteration
	m, _ = pressKey(m, '+')

	// Then: loop total increases and mode remains "Researching"
	if !viewContains(m, "#1/6") {
		t.Errorf("Expected #1/6 after '+' press, got:\n%s", m.View())
	}
	if !viewContains(m, "Researching") {
		t.Error("'Researching' mode should persist after '+' press")
	}
}

func TestBDD_AutoresearchMode_SubtractLoopInResearchingMode(t *testing.T) {
	// Given: a running model in "Researching" mode with loop 1/5
	m, l := setupReadyModelWithLoop(1, 5)
	m, _ = sendTuiMsg(m, tui.SendModeUpdate("Researching"))

	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()
	l.Start(ctx)

	// When: user presses '-' to subtract an iteration
	m, _ = pressKey(m, '-')

	// Then: loop total decreases and mode remains "Researching"
	if !viewContains(m, "#1/4") {
		t.Errorf("Expected #1/4 after '-' press, got:\n%s", m.View())
	}
	if !viewContains(m, "Researching") {
		t.Error("'Researching' mode should persist after '-' press")
	}
}

// --- Scenario 15: Researching mode in footer field ordering ---

func TestBDD_AutoresearchMode_FooterFieldOrderingWithResearching(t *testing.T) {
	// Given: a model with all footer fields populated in Researching mode
	m, _ := setupReadyModelWithLoop(2, 5)
	m, _ = sendTuiMsg(m, tui.SendCompletedTasksUpdate(1, 3))
	m, _ = sendTuiMsg(m, tui.SendTaskUpdate("#1 Run baseline"))
	m, _ = sendTuiMsg(m, tui.SendModeUpdate("Researching"))

	// Then: footer fields appear in expected order with "Researching" last
	view := m.View()
	loopIdx := strings.Index(view, "Loop:")
	modeIdx := strings.Index(view, "Current Mode:")
	researchIdx := strings.Index(view, "Researching")

	if loopIdx == -1 || modeIdx == -1 || researchIdx == -1 {
		t.Fatalf("Expected Loop, Current Mode, and Researching in view. Loop=%d Mode=%d Researching=%d",
			loopIdx, modeIdx, researchIdx)
	}

	if !(loopIdx < modeIdx) {
		t.Error("Loop should appear before Current Mode in footer")
	}
}

// --- Scenario 16: Autoresearch with custom prompt override ---

func TestBDD_AutoresearchMode_CustomPromptOverrideWorks(t *testing.T) {
	// Given: a user provides a custom prompt file for autoresearch
	tmpFile, err := os.CreateTemp("", "custom_prompt_*.md")
	if err != nil {
		t.Fatalf("Failed to create temp file: %v", err)
	}
	defer os.Remove(tmpFile.Name())

	customPrompt := "Custom autoresearch prompt: $experiment_content\nIteration $loop_iteration of $loop_total"
	tmpFile.WriteString(customPrompt)
	tmpFile.Close()

	// When: the loader uses the override path
	loader := prompt.NewAutoresearchLoader(tmpFile.Name(), "", "my custom experiment")
	content, err := loader.Load()
	if err != nil {
		t.Fatalf("Failed to load custom prompt: %v", err)
	}

	// Then: the custom prompt is used with experiment content substituted
	if !strings.Contains(content, "Custom autoresearch prompt") {
		t.Error("Should use the custom prompt content")
	}
	if !strings.Contains(content, "my custom experiment") {
		t.Error("Experiment content should be substituted in custom prompt")
	}
}

// --- Scenario 17: Researching mode with pause and resume ---

func TestBDD_AutoresearchMode_PauseShowsStoppedWhileResearching(t *testing.T) {
	// Given: a running model in "Researching" mode
	m, l := setupReadyModelWithLoop(2, 5)
	m, _ = sendTuiMsg(m, tui.SendModeUpdate("Researching"))

	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()
	l.Start(ctx)

	// When: user pauses the loop
	m, _ = pressKey(m, 'p')

	// Then: status shows STOPPED but mode still shows "Researching"
	if !viewContains(m, "STOPPED") {
		t.Error("Status should show STOPPED after pause")
	}
	if !viewContains(m, "Researching") {
		t.Error("'Researching' mode should persist during pause")
	}
}

// --- Scenario 18: Researching mode with rate limiting ---

func TestBDD_AutoresearchMode_HibernateShowsRateLimitedWhileResearching(t *testing.T) {
	// Given: a model in "Researching" mode that hits a rate limit
	m, l := setupReadyModelWithLoop(2, 5)
	m, _ = sendTuiMsg(m, tui.SendModeUpdate("Researching"))

	// When: the loop enters hibernate
	until := time.Now().Add(5 * time.Minute)
	l.Hibernate(until)
	m, _ = sendTuiMsg(m, tui.SendHibernate(until))

	// Then: status shows RATE LIMITED but mode still shows "Researching"
	if !viewContains(m, "RATE LIMITED") {
		t.Error("Status should show RATE LIMITED during hibernate")
	}
	if !viewContains(m, "Researching") {
		t.Error("'Researching' mode should persist during rate limiting")
	}
}

// --- Scenario 19: Autoresearch mode with empty experiment content ---

func TestBDD_AutoresearchMode_EmptyExperimentContentProducesValidPrompt(t *testing.T) {
	// Given: autoresearch mode with empty experiment content (edge case)
	loader := prompt.NewAutoresearchLoader("", "", "")
	content, err := loader.Load()
	if err != nil {
		t.Fatalf("Failed to load prompt with empty experiment: %v", err)
	}

	// Then: the prompt is still valid (contains the framework, just no experiment content)
	if !strings.Contains(content, "Autoresearch") {
		t.Error("Prompt should still contain Autoresearch heading even with empty experiment")
	}
	if !strings.Contains(content, "$loop_iteration") {
		t.Error("Prompt should still contain iteration placeholder")
	}
}

// --- Scenario 20: Autoresearch TUI shows loop progress through iterations ---

func TestBDD_AutoresearchMode_LoopProgressThroughIterations(t *testing.T) {
	// Given: a model in "Researching" mode
	m := setupReadyModel()
	m, _ = sendTuiMsg(m, tui.SendModeUpdate("Researching"))

	// When: loop progresses from iteration 1 through 3 of 5
	for i := 1; i <= 3; i++ {
		m, _ = sendTuiMsg(m, tui.SendLoopUpdate(i, 5))
	}

	// Then: progress shows current iteration and mode persists
	if !viewContains(m, "#3/5") {
		t.Error("Expected loop progress '#3/5'")
	}
	if !viewContains(m, "Researching") {
		t.Error("'Researching' mode should persist through iterations")
	}
}

// --- Scenario 21: Adding loops after autoresearch completion ---

func TestBDD_AutoresearchMode_AddLoopAfterResearchCompletion(t *testing.T) {
	// Given: completed autoresearch at 5/5
	m, l := setupReadyModelWithLoop(5, 5)
	m, _ = sendTuiMsg(m, tui.SendModeUpdate("Researching"))
	m, _ = sendTuiMsg(m, tui.SendDone())

	if !viewContains(m, "COMPLETED") {
		t.Fatal("Precondition: should be in COMPLETED state")
	}

	// When: user presses '+' to add more research iterations
	_ = l // loop is available for additional iterations
	m, _ = pressKey(m, '+')

	// Then: total increases and (s)tart is available
	if !viewContains(m, "#5/6") {
		t.Errorf("After '+' post-completion, expected #5/6, got:\n%s", m.View())
	}
	if !viewContains(m, "(s)tart") {
		t.Error("After adding loop post-completion, should show '(s)tart'")
	}
	// And: mode still shows "Researching"
	if !viewContains(m, "Researching") {
		t.Error("'Researching' mode should persist after adding loops post-completion")
	}
}

// --- Scenario 22: Wide terminal renders autoresearch mode correctly ---

func TestBDD_AutoresearchMode_WideTerminalRendersCorrectly(t *testing.T) {
	// Given: a model with a wide terminal in "Researching" mode
	m := tui.NewModel()
	m, _ = updateModel(m, tea.WindowSizeMsg{Width: 200, Height: 50})
	m, _ = sendTuiMsg(m, tui.SendModeUpdate("Researching"))
	m, _ = sendTuiMsg(m, tui.SendLoopUpdate(3, 10))

	// Then: the wide terminal correctly shows both mode and progress
	if !viewContains(m, "Researching") {
		t.Error("Wide terminal should display 'Researching' mode")
	}
	if !viewContains(m, "#3/10") {
		t.Error("Wide terminal should display loop progress")
	}
}

// --- Scenario 23: Narrow terminal renders autoresearch mode ---

func TestBDD_AutoresearchMode_NarrowTerminalRendersGracefully(t *testing.T) {
	// Given: a model with a narrow terminal (minimum viable width)
	m := tui.NewModel()
	m, _ = updateModel(m, tea.WindowSizeMsg{Width: 80, Height: 30})
	m, _ = sendTuiMsg(m, tui.SendModeUpdate("Researching"))

	// Then: the view renders without panic (graceful degradation)
	view := m.View()
	if len(view) == 0 {
		t.Error("View should render something even on narrow terminal")
	}
}
