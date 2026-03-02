// Package prompt handles loading the loop prompt from embedded resources or file override
package prompt

import (
	"embed"
	"fmt"
	"os"
	"path/filepath"
	"strings"
)

//go:embed assets/prompt.md assets/plan_prompt.md
var embeddedFS embed.FS

const embeddedPromptPath = "assets/prompt.md"
const embeddedPlanPromptPath = "assets/plan_prompt.md"

// Loader provides methods for loading the loop prompt
type Loader struct {
	overridePath string
	planMode     bool
	goal         string
}

// NewLoader creates a new prompt Loader.
// If overridePath is empty, the embedded prompt will be used.
// If overridePath is provided, it will load from that file instead.
// The goal parameter specifies the ultimate goal sentence for the prompt.
func NewLoader(overridePath string, goal string) *Loader {
	return &Loader{
		overridePath: overridePath,
		goal:         goal,
	}
}

// NewPlanLoader creates a prompt Loader for plan mode.
// If overridePath is empty, the embedded plan prompt will be used.
// The goal parameter specifies the ultimate goal sentence for the plan prompt.
func NewPlanLoader(overridePath string, goal string) *Loader {
	return &Loader{
		overridePath: overridePath,
		planMode:     true,
		goal:         goal,
	}
}

// Load returns the prompt content.
// If an override path was configured, it loads from that file.
// Otherwise, it returns the embedded default prompt (build or plan based on mode).
// The $ultimate_goal_placeholder_sentence placeholder is substituted with the goal in both modes.
func (l *Loader) Load() (string, error) {
	var content string
	var err error

	if l.overridePath != "" {
		content, err = l.loadFromFile(l.overridePath)
	} else if l.planMode {
		content, err = l.loadEmbeddedPlan()
	} else {
		content, err = l.loadEmbedded()
	}

	if err != nil {
		return "", err
	}

	content = substituteGoal(content, l.goal)

	return content, nil
}

// loadEmbedded returns the embedded default prompt
func (l *Loader) loadEmbedded() (string, error) {
	content, err := embeddedFS.ReadFile(embeddedPromptPath)
	if err != nil {
		return "", fmt.Errorf("failed to read embedded prompt: %w", err)
	}
	return string(content), nil
}

// loadFromFile reads the prompt from a file path
func (l *Loader) loadFromFile(path string) (string, error) {
	absPath, err := filepath.Abs(path)
	if err != nil {
		return "", fmt.Errorf("invalid prompt path %q: %w", path, err)
	}

	content, err := os.ReadFile(absPath)
	if err != nil {
		return "", fmt.Errorf("failed to read prompt file %q: %w", path, err)
	}

	return string(content), nil
}

// loadEmbeddedPlan returns the embedded plan prompt
func (l *Loader) loadEmbeddedPlan() (string, error) {
	content, err := embeddedFS.ReadFile(embeddedPlanPromptPath)
	if err != nil {
		return "", fmt.Errorf("failed to read embedded plan prompt: %w", err)
	}
	return string(content), nil
}

// substituteGoal replaces the $ultimate_goal_placeholder_sentence placeholder in the prompt content.
// If goal is non-empty, the placeholder is replaced with the goal text (period is kept from template).
// If goal is empty, the placeholder and its trailing ". " are removed for clean output.
func substituteGoal(content, goal string) string {
	if goal == "" {
		return strings.Replace(content, "$ultimate_goal_placeholder_sentence. ", "", 1)
	}
	goal = strings.TrimRight(goal, ".")
	return strings.Replace(content, "$ultimate_goal_placeholder_sentence", goal, 1)
}

// IsUsingOverride returns true if a custom prompt file is configured
func (l *Loader) IsUsingOverride() bool {
	return l.overridePath != ""
}

// IsPlanMode returns true if the loader is configured for plan mode
func (l *Loader) IsPlanMode() bool {
	return l.planMode
}

// GetEmbeddedPrompt is a convenience function to get the embedded prompt directly
func GetEmbeddedPrompt() (string, error) {
	loader := NewLoader("", "")
	return loader.Load()
}

// GetEmbeddedPlanPrompt is a convenience function to get the raw embedded plan prompt template.
// Note: this returns the template with $ultimate_goal_placeholder_sentence placeholder unsubstituted.
func GetEmbeddedPlanPrompt() (string, error) {
	content, err := embeddedFS.ReadFile(embeddedPlanPromptPath)
	if err != nil {
		return "", fmt.Errorf("failed to read embedded plan prompt: %w", err)
	}
	return string(content), nil
}
