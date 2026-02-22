// Package prompt handles loading the loop prompt from embedded resources or file override
package prompt

import (
	"embed"
	"fmt"
	"os"
	"path/filepath"
)

//go:embed assets/prompt.md assets/plan_prompt.md
var embeddedFS embed.FS

const embeddedPromptPath     = "assets/prompt.md"
const embeddedPlanPromptPath = "assets/plan_prompt.md"

// Loader provides methods for loading the loop prompt
type Loader struct {
	overridePath string
	planMode     bool
}

// NewLoader creates a new prompt Loader.
// If overridePath is empty, the embedded prompt will be used.
// If overridePath is provided, it will load from that file instead.
func NewLoader(overridePath string) *Loader {
	return &Loader{
		overridePath: overridePath,
	}
}

// NewPlanLoader creates a prompt Loader for plan mode.
// If overridePath is empty, the embedded plan prompt will be used.
func NewPlanLoader(overridePath string) *Loader {
	return &Loader{
		overridePath: overridePath,
		planMode:     true,
	}
}

// Load returns the prompt content.
// If an override path was configured, it loads from that file.
// Otherwise, it returns the embedded default prompt (build or plan based on mode).
func (l *Loader) Load() (string, error) {
	if l.overridePath != "" {
		return l.loadFromFile(l.overridePath)
	}
	if l.planMode {
		return l.loadEmbeddedPlan()
	}
	return l.loadEmbedded()
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
	loader := NewLoader("")
	return loader.Load()
}

// GetEmbeddedPlanPrompt is a convenience function to get the embedded plan prompt directly
func GetEmbeddedPlanPrompt() (string, error) {
	loader := NewPlanLoader("")
	return loader.Load()
}
