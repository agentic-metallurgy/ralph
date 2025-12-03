// Package prompt handles loading the loop prompt from embedded resources or file override
package prompt

import (
	"embed"
	"fmt"
	"os"
	"path/filepath"
)

//go:embed assets/prompt.md
var embeddedFS embed.FS

const embeddedPromptPath = "assets/prompt.md"

// Loader provides methods for loading the loop prompt
type Loader struct {
	overridePath string
}

// NewLoader creates a new prompt Loader.
// If overridePath is empty, the embedded prompt will be used.
// If overridePath is provided, it will load from that file instead.
func NewLoader(overridePath string) *Loader {
	return &Loader{
		overridePath: overridePath,
	}
}

// Load returns the prompt content.
// If an override path was configured, it loads from that file.
// Otherwise, it returns the embedded default prompt.
func (l *Loader) Load() (string, error) {
	if l.overridePath != "" {
		return l.loadFromFile(l.overridePath)
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

// IsUsingOverride returns true if a custom prompt file is configured
func (l *Loader) IsUsingOverride() bool {
	return l.overridePath != ""
}

// GetEmbeddedPrompt is a convenience function to get the embedded prompt directly
func GetEmbeddedPrompt() (string, error) {
	loader := NewLoader("")
	return loader.Load()
}
