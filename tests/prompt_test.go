package tests

import (
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/cloudosai/ralph-go/internal/prompt"
)

func TestNewLoader(t *testing.T) {
	// Test creating loader without override
	loader := prompt.NewLoader("")
	if loader.IsUsingOverride() {
		t.Error("Expected IsUsingOverride() to be false for empty path")
	}

	// Test creating loader with override
	loaderWithOverride := prompt.NewLoader("/some/path.md")
	if !loaderWithOverride.IsUsingOverride() {
		t.Error("Expected IsUsingOverride() to be true for non-empty path")
	}
}

func TestLoadEmbedded(t *testing.T) {
	loader := prompt.NewLoader("")
	content, err := loader.Load()

	if err != nil {
		t.Fatalf("Expected no error loading embedded prompt, got: %v", err)
	}

	if content == "" {
		t.Error("Expected non-empty content from embedded prompt")
	}

	// Verify the content looks like the expected prompt
	expectedPhrases := []string{
		"familiarize yourself with the source code",
		"IMPLEMENTATION_PLAN.md",
		"git add -A",
		"git commit",
	}

	for _, phrase := range expectedPhrases {
		if !strings.Contains(content, phrase) {
			t.Errorf("Expected embedded prompt to contain %q", phrase)
		}
	}
}

func TestLoadFromFile(t *testing.T) {
	// Create a temporary file with custom content
	tmpFile, err := os.CreateTemp("", "ralph-prompt-*.md")
	if err != nil {
		t.Fatalf("Failed to create temp file: %v", err)
	}
	defer os.Remove(tmpFile.Name())

	customContent := "# Custom Prompt\n\nThis is a custom loop prompt for testing.\n"
	if _, err := tmpFile.WriteString(customContent); err != nil {
		t.Fatalf("Failed to write to temp file: %v", err)
	}
	tmpFile.Close()

	loader := prompt.NewLoader(tmpFile.Name())
	content, err := loader.Load()

	if err != nil {
		t.Fatalf("Expected no error loading from file, got: %v", err)
	}

	if content != customContent {
		t.Errorf("Expected content %q, got %q", customContent, content)
	}
}

func TestLoadFromFile_NotExists(t *testing.T) {
	loader := prompt.NewLoader("/nonexistent/path/to/prompt.md")
	_, err := loader.Load()

	if err == nil {
		t.Error("Expected error loading from non-existent file, got nil")
	}
}

func TestLoadFromFile_RelativePath(t *testing.T) {
	// Create a temporary directory and file
	tmpDir, err := os.MkdirTemp(".", "ralph-test-*")
	if err != nil {
		t.Fatalf("Failed to create temp dir: %v", err)
	}
	defer os.RemoveAll(tmpDir)

	tmpFile := filepath.Join(tmpDir, "custom-prompt.md")
	customContent := "Custom content via relative path"
	if err := os.WriteFile(tmpFile, []byte(customContent), 0644); err != nil {
		t.Fatalf("Failed to write temp file: %v", err)
	}

	loader := prompt.NewLoader(tmpFile)
	content, err := loader.Load()

	if err != nil {
		t.Fatalf("Expected no error with relative path, got: %v", err)
	}

	if content != customContent {
		t.Errorf("Expected content %q, got %q", customContent, content)
	}
}

func TestGetEmbeddedPrompt(t *testing.T) {
	content, err := prompt.GetEmbeddedPrompt()

	if err != nil {
		t.Fatalf("Expected no error from GetEmbeddedPrompt, got: %v", err)
	}

	if content == "" {
		t.Error("Expected non-empty content from GetEmbeddedPrompt")
	}

	// Should be same as loading via Loader
	loader := prompt.NewLoader("")
	loaderContent, _ := loader.Load()

	if content != loaderContent {
		t.Error("GetEmbeddedPrompt and Loader.Load() returned different content")
	}
}

func TestIsUsingOverride(t *testing.T) {
	tests := []struct {
		name         string
		overridePath string
		expected     bool
	}{
		{"empty path", "", false},
		{"non-empty path", "custom.md", true},
		{"absolute path", "/some/absolute/path.md", true},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			loader := prompt.NewLoader(tt.overridePath)
			result := loader.IsUsingOverride()
			if result != tt.expected {
				t.Errorf("IsUsingOverride() = %v, expected %v", result, tt.expected)
			}
		})
	}
}

func TestEmbeddedPromptSize(t *testing.T) {
	content, err := prompt.GetEmbeddedPrompt()
	if err != nil {
		t.Fatalf("Failed to get embedded prompt: %v", err)
	}

	// The embedded prompt should be at least 500 bytes (reasonable size check)
	if len(content) < 500 {
		t.Errorf("Embedded prompt seems too small: %d bytes", len(content))
	}

	// And not unreasonably large (e.g., 10KB)
	if len(content) > 10000 {
		t.Errorf("Embedded prompt seems too large: %d bytes", len(content))
	}
}

func TestLoadEmptyFile(t *testing.T) {
	// Create an empty temporary file
	tmpFile, err := os.CreateTemp("", "ralph-empty-*.md")
	if err != nil {
		t.Fatalf("Failed to create temp file: %v", err)
	}
	tmpFile.Close()
	defer os.Remove(tmpFile.Name())

	loader := prompt.NewLoader(tmpFile.Name())
	content, err := loader.Load()

	if err != nil {
		t.Fatalf("Expected no error loading empty file, got: %v", err)
	}

	if content != "" {
		t.Errorf("Expected empty content for empty file, got: %q", content)
	}
}
