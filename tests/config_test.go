package tests

import (
	"flag"
	"os"
	"path/filepath"
	"testing"

	"github.com/cloudosai/ralph-go/internal/config"
)

func TestNewConfig(t *testing.T) {
	cfg := config.NewConfig()

	if cfg.Iterations != config.DefaultIterations {
		t.Errorf("Expected default iterations %d, got %d", config.DefaultIterations, cfg.Iterations)
	}
	if cfg.SpecFile != "" {
		t.Errorf("Expected empty spec file, got %q", cfg.SpecFile)
	}
	if cfg.SpecFolder != config.DefaultSpecFolder {
		t.Errorf("Expected default spec folder %q, got %q", config.DefaultSpecFolder, cfg.SpecFolder)
	}
	if cfg.LoopPrompt != "" {
		t.Errorf("Expected empty loop prompt, got %q", cfg.LoopPrompt)
	}
}

func TestValidate_ValidConfig(t *testing.T) {
	// Create a temporary directory for spec folder with a spec file
	tmpDir, err := os.MkdirTemp("", "ralph-test-*")
	if err != nil {
		t.Fatalf("Failed to create temp dir: %v", err)
	}
	defer os.RemoveAll(tmpDir)

	// Spec folder must contain at least one file
	if err := os.WriteFile(filepath.Join(tmpDir, "spec.md"), []byte("test spec"), 0644); err != nil {
		t.Fatalf("Failed to create spec file: %v", err)
	}

	cfg := &config.Config{
		Iterations: 10,
		SpecFile:   "",
		SpecFolder: tmpDir,
		LoopPrompt: "",
	}

	if err := cfg.Validate(); err != nil {
		t.Errorf("Expected valid config, got error: %v", err)
	}
}

func TestValidate_InvalidIterations(t *testing.T) {
	tests := []struct {
		name       string
		iterations int
	}{
		{"zero iterations", 0},
		{"negative iterations", -1},
		{"very negative", -100},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			cfg := &config.Config{
				Iterations: tt.iterations,
				SpecFolder: "", // Skip folder validation
			}
			err := cfg.Validate()
			if err == nil {
				t.Errorf("Expected error for iterations=%d, got nil", tt.iterations)
			}
		})
	}
}

func TestValidate_SpecFileExists(t *testing.T) {
	// Create a temporary file
	tmpFile, err := os.CreateTemp("", "ralph-spec-*.md")
	if err != nil {
		t.Fatalf("Failed to create temp file: %v", err)
	}
	tmpFile.Close()
	defer os.Remove(tmpFile.Name())

	cfg := &config.Config{
		Iterations: 1,
		SpecFile:   tmpFile.Name(),
		SpecFolder: "",
	}

	if err := cfg.Validate(); err != nil {
		t.Errorf("Expected valid config with existing spec file, got error: %v", err)
	}
}

func TestValidate_SpecFileNotExists(t *testing.T) {
	cfg := &config.Config{
		Iterations: 1,
		SpecFile:   "/nonexistent/path/to/spec.md",
		SpecFolder: "",
	}

	err := cfg.Validate()
	if err == nil {
		t.Error("Expected error for non-existent spec file, got nil")
	}
}

func TestValidate_SpecFileIsDirectory(t *testing.T) {
	// Create a temporary directory
	tmpDir, err := os.MkdirTemp("", "ralph-test-*")
	if err != nil {
		t.Fatalf("Failed to create temp dir: %v", err)
	}
	defer os.RemoveAll(tmpDir)

	cfg := &config.Config{
		Iterations: 1,
		SpecFile:   tmpDir, // Directory instead of file
		SpecFolder: "",
	}

	err = cfg.Validate()
	if err == nil {
		t.Error("Expected error when spec-file is a directory, got nil")
	}
}

func TestValidate_SpecFolderExists(t *testing.T) {
	// Create a temporary directory with a spec file
	tmpDir, err := os.MkdirTemp("", "ralph-test-*")
	if err != nil {
		t.Fatalf("Failed to create temp dir: %v", err)
	}
	defer os.RemoveAll(tmpDir)

	// Spec folder must contain at least one file
	if err := os.WriteFile(filepath.Join(tmpDir, "spec.md"), []byte("test spec"), 0644); err != nil {
		t.Fatalf("Failed to create spec file: %v", err)
	}

	cfg := &config.Config{
		Iterations: 1,
		SpecFile:   "",
		SpecFolder: tmpDir,
	}

	if err := cfg.Validate(); err != nil {
		t.Errorf("Expected valid config with existing spec folder, got error: %v", err)
	}
}

func TestValidate_SpecFolderNotExists(t *testing.T) {
	cfg := &config.Config{
		Iterations: 1,
		SpecFile:   "",
		SpecFolder: "/nonexistent/path/to/specs/",
	}

	err := cfg.Validate()
	if err == nil {
		t.Error("Expected error for non-existent spec folder, got nil")
	}
}

func TestValidate_SpecFolderIsFile(t *testing.T) {
	// Create a temporary file
	tmpFile, err := os.CreateTemp("", "ralph-test-*")
	if err != nil {
		t.Fatalf("Failed to create temp file: %v", err)
	}
	tmpFile.Close()
	defer os.Remove(tmpFile.Name())

	cfg := &config.Config{
		Iterations: 1,
		SpecFile:   "",
		SpecFolder: tmpFile.Name(), // File instead of directory
	}

	err = cfg.Validate()
	if err == nil {
		t.Error("Expected error when spec-folder is a file, got nil")
	}
}

func TestValidate_LoopPromptExists(t *testing.T) {
	// Create temp dir and file
	tmpDir, err := os.MkdirTemp("", "ralph-test-*")
	if err != nil {
		t.Fatalf("Failed to create temp dir: %v", err)
	}
	defer os.RemoveAll(tmpDir)

	tmpFile, err := os.CreateTemp("", "ralph-prompt-*.md")
	if err != nil {
		t.Fatalf("Failed to create temp file: %v", err)
	}
	tmpFile.Close()
	defer os.Remove(tmpFile.Name())

	cfg := &config.Config{
		Iterations: 1,
		SpecFile:   "",
		SpecFolder: tmpDir,
		LoopPrompt: tmpFile.Name(),
	}

	if err := cfg.Validate(); err != nil {
		t.Errorf("Expected valid config with existing loop prompt, got error: %v", err)
	}
}

func TestValidate_LoopPromptNotExists(t *testing.T) {
	// Create temp dir for spec folder
	tmpDir, err := os.MkdirTemp("", "ralph-test-*")
	if err != nil {
		t.Fatalf("Failed to create temp dir: %v", err)
	}
	defer os.RemoveAll(tmpDir)

	cfg := &config.Config{
		Iterations: 1,
		SpecFile:   "",
		SpecFolder: tmpDir,
		LoopPrompt: "/nonexistent/path/to/prompt.md",
	}

	err = cfg.Validate()
	if err == nil {
		t.Error("Expected error for non-existent loop prompt, got nil")
	}
}

func TestValidate_SpecFileOverridesSpecFolder(t *testing.T) {
	// When spec-file is set, spec-folder validation should be skipped
	// Create a temporary file for spec-file
	tmpFile, err := os.CreateTemp("", "ralph-spec-*.md")
	if err != nil {
		t.Fatalf("Failed to create temp file: %v", err)
	}
	tmpFile.Close()
	defer os.Remove(tmpFile.Name())

	cfg := &config.Config{
		Iterations: 1,
		SpecFile:   tmpFile.Name(),
		SpecFolder: "/nonexistent/path/to/specs/", // Non-existent but should be ignored
	}

	if err := cfg.Validate(); err != nil {
		t.Errorf("Expected spec-file to override spec-folder validation, got error: %v", err)
	}
}

func TestString(t *testing.T) {
	cfg := &config.Config{
		Iterations: 15,
		SpecFile:   "myspec.md",
		SpecFolder: "myspecs/",
		LoopPrompt: "myprompt.md",
	}

	str := cfg.String()
	expectedParts := []string{"15", "myspec.md", "myspecs/", "myprompt.md"}

	for _, part := range expectedParts {
		if !contains(str, part) {
			t.Errorf("Expected String() to contain %q, got: %s", part, str)
		}
	}
}

func TestGetSpecPath(t *testing.T) {
	tests := []struct {
		name       string
		specFile   string
		specFolder string
		expected   string
	}{
		{
			name:       "returns spec file when set",
			specFile:   "custom.md",
			specFolder: "specs/",
			expected:   "custom.md",
		},
		{
			name:       "returns spec folder when file not set",
			specFile:   "",
			specFolder: "myspecs/",
			expected:   "myspecs/",
		},
		{
			name:       "returns empty spec folder when neither set",
			specFile:   "",
			specFolder: "",
			expected:   "",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			cfg := &config.Config{
				SpecFile:   tt.specFile,
				SpecFolder: tt.specFolder,
			}
			result := cfg.GetSpecPath()
			if result != tt.expected {
				t.Errorf("GetSpecPath() = %q, expected %q", result, tt.expected)
			}
		})
	}
}

func TestIsUsingSpecFile(t *testing.T) {
	tests := []struct {
		name     string
		specFile string
		expected bool
	}{
		{"empty spec file", "", false},
		{"non-empty spec file", "spec.md", true},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			cfg := &config.Config{SpecFile: tt.specFile}
			result := cfg.IsUsingSpecFile()
			if result != tt.expected {
				t.Errorf("IsUsingSpecFile() = %v, expected %v", result, tt.expected)
			}
		})
	}
}

func TestIsUsingCustomPrompt(t *testing.T) {
	tests := []struct {
		name       string
		loopPrompt string
		expected   bool
	}{
		{"empty loop prompt", "", false},
		{"non-empty loop prompt", "prompt.md", true},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			cfg := &config.Config{LoopPrompt: tt.loopPrompt}
			result := cfg.IsUsingCustomPrompt()
			if result != tt.expected {
				t.Errorf("IsUsingCustomPrompt() = %v, expected %v", result, tt.expected)
			}
		})
	}
}

func TestValidate_EmptySpecFolder(t *testing.T) {
	// When both spec-file and spec-folder are empty, validation should pass
	// (no path validation needed for empty strings)
	cfg := &config.Config{
		Iterations: 1,
		SpecFile:   "",
		SpecFolder: "",
	}

	// This should pass because no spec folder validation is attempted when empty
	if err := cfg.Validate(); err != nil {
		t.Errorf("Expected valid config with empty spec folder, got error: %v", err)
	}
}

func TestValidate_SpecFolderExistsButEmpty(t *testing.T) {
	// An existing but empty spec folder should fail validation
	tmpDir, err := os.MkdirTemp("", "ralph-test-*")
	if err != nil {
		t.Fatalf("Failed to create temp dir: %v", err)
	}
	defer os.RemoveAll(tmpDir)

	cfg := &config.Config{
		Iterations: 1,
		SpecFile:   "",
		SpecFolder: tmpDir,
	}

	err = cfg.Validate()
	if err == nil {
		t.Error("Expected error for empty spec folder, got nil")
	}
	// Error should mention "empty" and provide guidance
	errMsg := err.Error()
	if !contains(errMsg, "empty") {
		t.Errorf("Expected error to mention 'empty', got: %s", errMsg)
	}
	if !contains(errMsg, "--spec-file") {
		t.Errorf("Expected error to mention '--spec-file' for guidance, got: %s", errMsg)
	}
}

func TestValidate_SpecFolderMissingGuidance(t *testing.T) {
	// When spec folder doesn't exist, error should include guidance
	cfg := &config.Config{
		Iterations: 1,
		SpecFile:   "",
		SpecFolder: "/nonexistent/path/to/specs/",
	}

	err := cfg.Validate()
	if err == nil {
		t.Fatal("Expected error for missing spec folder, got nil")
	}
	errMsg := err.Error()
	if !contains(errMsg, "--spec-file") {
		t.Errorf("Expected error to mention '--spec-file' for guidance, got: %s", errMsg)
	}
	if !contains(errMsg, "--spec-folder") {
		t.Errorf("Expected error to mention '--spec-folder' for guidance, got: %s", errMsg)
	}
}

func TestValidate_CustomLoopPromptSkipsEmptySpecFolder(t *testing.T) {
	// When a custom loop prompt is provided, empty spec folder should not cause an error
	tmpDir, err := os.MkdirTemp("", "ralph-test-*")
	if err != nil {
		t.Fatalf("Failed to create temp dir: %v", err)
	}
	defer os.RemoveAll(tmpDir)

	tmpFile, err := os.CreateTemp("", "ralph-prompt-*.md")
	if err != nil {
		t.Fatalf("Failed to create temp file: %v", err)
	}
	tmpFile.Close()
	defer os.Remove(tmpFile.Name())

	cfg := &config.Config{
		Iterations: 1,
		SpecFile:   "",
		SpecFolder: tmpDir, // empty dir
		LoopPrompt: tmpFile.Name(),
	}

	if err := cfg.Validate(); err != nil {
		t.Errorf("Expected custom loop prompt to skip spec folder validation, got error: %v", err)
	}
}

func TestValidate_SpecFileBypassesEmptyFolderCheck(t *testing.T) {
	// When spec-file is provided, empty spec folder should not cause an error
	tmpFile, err := os.CreateTemp("", "ralph-spec-*.md")
	if err != nil {
		t.Fatalf("Failed to create temp file: %v", err)
	}
	tmpFile.Close()
	defer os.Remove(tmpFile.Name())

	cfg := &config.Config{
		Iterations: 1,
		SpecFile:   tmpFile.Name(),
		SpecFolder: "/nonexistent/empty/specs/", // should be ignored
	}

	if err := cfg.Validate(); err != nil {
		t.Errorf("Expected spec-file to bypass spec folder validation, got error: %v", err)
	}
}

func TestValidate_RelativePaths(t *testing.T) {
	// Create a temporary directory structure in current working dir
	tmpDir, err := os.MkdirTemp(".", "ralph-test-*")
	if err != nil {
		t.Fatalf("Failed to create temp dir: %v", err)
	}
	defer os.RemoveAll(tmpDir)

	// Create a file in the temp dir
	tmpFile := filepath.Join(tmpDir, "spec.md")
	if err := os.WriteFile(tmpFile, []byte("test"), 0644); err != nil {
		t.Fatalf("Failed to create temp file: %v", err)
	}

	// Test with relative path for spec-file
	cfg := &config.Config{
		Iterations: 1,
		SpecFile:   tmpFile, // Relative path
		SpecFolder: "",
	}

	if err := cfg.Validate(); err != nil {
		t.Errorf("Expected valid config with relative spec file path, got error: %v", err)
	}

	// Test with relative path for spec-folder
	cfg = &config.Config{
		Iterations: 1,
		SpecFile:   "",
		SpecFolder: tmpDir, // Relative path
	}

	if err := cfg.Validate(); err != nil {
		t.Errorf("Expected valid config with relative spec folder path, got error: %v", err)
	}
}

func TestVersionVariable(t *testing.T) {
	// Version should have a default value when not set via ldflags
	if config.Version == "" {
		t.Error("Expected Version to have a default value, got empty string")
	}
	if config.Version != "dev" {
		t.Logf("Version is set to %q (expected 'dev' in test)", config.Version)
	}
}

func TestIsPlanMode(t *testing.T) {
	tests := []struct {
		name       string
		subcommand string
		expected   bool
	}{
		{"empty subcommand", "", false},
		{"plan subcommand", "plan", true},
		{"other subcommand", "build", false},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			cfg := &config.Config{Subcommand: tt.subcommand}
			result := cfg.IsPlanMode()
			if result != tt.expected {
				t.Errorf("IsPlanMode() = %v, expected %v", result, tt.expected)
			}
		})
	}
}

func TestSubcommandFieldDefault(t *testing.T) {
	cfg := config.NewConfig()
	if cfg.Subcommand != "" {
		t.Errorf("Expected empty Subcommand by default, got %q", cfg.Subcommand)
	}
	if cfg.IsPlanMode() {
		t.Error("Expected IsPlanMode() to be false by default")
	}
}

func TestGoalFieldDefault(t *testing.T) {
	cfg := config.NewConfig()
	if cfg.Goal != "" {
		t.Errorf("Expected empty Goal by default, got %q", cfg.Goal)
	}
}

func TestGoalFieldSet(t *testing.T) {
	cfg := &config.Config{
		Iterations: 1,
		Goal:       "Build a world-class trading platform",
	}
	if cfg.Goal != "Build a world-class trading platform" {
		t.Errorf("Expected Goal to be set, got %q", cfg.Goal)
	}
}

func TestBuildSubcommandDetected(t *testing.T) {
	cfg := &config.Config{Subcommand: "build"}
	if cfg.IsPlanMode() {
		t.Error("Expected IsPlanMode() to be false for build subcommand")
	}
	if cfg.Subcommand != "build" {
		t.Errorf("Expected Subcommand to be 'build', got %q", cfg.Subcommand)
	}
}

func TestBuildSubcommandBehavesLikeDefault(t *testing.T) {
	// Build subcommand should behave identically to default (no subcommand) mode
	defaultCfg := &config.Config{Subcommand: ""}
	buildCfg := &config.Config{Subcommand: "build"}

	// Both should return false for IsPlanMode
	if defaultCfg.IsPlanMode() != buildCfg.IsPlanMode() {
		t.Error("Expected build subcommand IsPlanMode() to match default mode")
	}
}

func TestCLIFieldDefault(t *testing.T) {
	cfg := config.NewConfig()
	if cfg.CLI {
		t.Error("Expected CLI to be false by default")
	}
}

func TestCLIFieldSet(t *testing.T) {
	cfg := &config.Config{
		Iterations: 1,
		CLI:        true,
	}
	if !cfg.CLI {
		t.Error("Expected CLI to be true when set")
	}
}

func TestDefaultPlanIterationsConstant(t *testing.T) {
	if config.DefaultPlanIterations != 1 {
		t.Errorf("Expected DefaultPlanIterations = 1, got %d", config.DefaultPlanIterations)
	}
	if config.DefaultPlanIterations >= config.DefaultIterations {
		t.Errorf("Expected DefaultPlanIterations (%d) < DefaultIterations (%d)",
			config.DefaultPlanIterations, config.DefaultIterations)
	}
}

func TestParseFlagsPlanModeDefaultsTo1Iteration(t *testing.T) {
	origArgs := os.Args
	origCommandLine := flag.CommandLine
	defer func() {
		os.Args = origArgs
		flag.CommandLine = origCommandLine
	}()

	flag.CommandLine = flag.NewFlagSet("test", flag.ContinueOnError)
	os.Args = []string{"ralph", "plan"}

	cfg := config.ParseFlags()

	if !cfg.IsPlanMode() {
		t.Fatal("Expected plan mode to be detected")
	}
	if cfg.Iterations != config.DefaultPlanIterations {
		t.Errorf("Expected plan mode iterations = %d, got %d", config.DefaultPlanIterations, cfg.Iterations)
	}
}

func TestParseFlagsPlanModeExplicitIterationsHonored(t *testing.T) {
	origArgs := os.Args
	origCommandLine := flag.CommandLine
	defer func() {
		os.Args = origArgs
		flag.CommandLine = origCommandLine
	}()

	flag.CommandLine = flag.NewFlagSet("test", flag.ContinueOnError)
	os.Args = []string{"ralph", "plan", "--iterations", "10"}

	cfg := config.ParseFlags()

	if !cfg.IsPlanMode() {
		t.Fatal("Expected plan mode to be detected")
	}
	if cfg.Iterations != 10 {
		t.Errorf("Expected explicit iterations = 10, got %d", cfg.Iterations)
	}
}

func TestParseFlagsBuildModeKeepsDefaultIterations(t *testing.T) {
	origArgs := os.Args
	origCommandLine := flag.CommandLine
	defer func() {
		os.Args = origArgs
		flag.CommandLine = origCommandLine
	}()

	flag.CommandLine = flag.NewFlagSet("test", flag.ContinueOnError)
	os.Args = []string{"ralph", "build"}

	cfg := config.ParseFlags()

	if cfg.IsPlanMode() {
		t.Error("Expected build mode, not plan mode")
	}
	if cfg.Iterations != config.DefaultIterations {
		t.Errorf("Expected build mode iterations = %d, got %d", config.DefaultIterations, cfg.Iterations)
	}
}

func TestParseFlagsDefaultModeKeepsDefaultIterations(t *testing.T) {
	origArgs := os.Args
	origCommandLine := flag.CommandLine
	defer func() {
		os.Args = origArgs
		flag.CommandLine = origCommandLine
	}()

	flag.CommandLine = flag.NewFlagSet("test", flag.ContinueOnError)
	os.Args = []string{"ralph"}

	cfg := config.ParseFlags()

	if cfg.IsPlanMode() {
		t.Error("Expected default mode, not plan mode")
	}
	if cfg.Iterations != config.DefaultIterations {
		t.Errorf("Expected default mode iterations = %d, got %d", config.DefaultIterations, cfg.Iterations)
	}
}

func TestIsPlanAndBuildMode(t *testing.T) {
	tests := []struct {
		name       string
		subcommand string
		expected   bool
	}{
		{"empty subcommand", "", false},
		{"plan subcommand", "plan", false},
		{"build subcommand", "build", false},
		{"plan-and-build subcommand", "plan-and-build", true},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			cfg := &config.Config{Subcommand: tt.subcommand}
			result := cfg.IsPlanAndBuildMode()
			if result != tt.expected {
				t.Errorf("IsPlanAndBuildMode() = %v, expected %v", result, tt.expected)
			}
		})
	}
}

func TestPlanAndBuildSubcommandDetected(t *testing.T) {
	origArgs := os.Args
	origCommandLine := flag.CommandLine
	defer func() {
		os.Args = origArgs
		flag.CommandLine = origCommandLine
	}()

	flag.CommandLine = flag.NewFlagSet("test", flag.ContinueOnError)
	os.Args = []string{"ralph", "plan-and-build"}

	cfg := config.ParseFlags()

	if !cfg.IsPlanAndBuildMode() {
		t.Fatal("Expected plan-and-build mode to be detected")
	}
	if cfg.Subcommand != "plan-and-build" {
		t.Errorf("Expected Subcommand = 'plan-and-build', got %q", cfg.Subcommand)
	}
}

func TestPlanAndBuildIterationDefaults(t *testing.T) {
	origArgs := os.Args
	origCommandLine := flag.CommandLine
	defer func() {
		os.Args = origArgs
		flag.CommandLine = origCommandLine
	}()

	flag.CommandLine = flag.NewFlagSet("test", flag.ContinueOnError)
	os.Args = []string{"ralph", "plan-and-build"}

	cfg := config.ParseFlags()

	if !cfg.IsPlanAndBuildMode() {
		t.Fatal("Expected plan-and-build mode to be detected")
	}
	// Plan phase should always be 1 iteration
	if cfg.Iterations != config.DefaultPlanIterations {
		t.Errorf("Expected Iterations = %d (plan phase), got %d", config.DefaultPlanIterations, cfg.Iterations)
	}
	// Build phase should default to 5 iterations
	if cfg.BuildIterations != config.DefaultIterations {
		t.Errorf("Expected BuildIterations = %d, got %d", config.DefaultIterations, cfg.BuildIterations)
	}
}

func TestPlanAndBuildExplicitIterations(t *testing.T) {
	origArgs := os.Args
	origCommandLine := flag.CommandLine
	defer func() {
		os.Args = origArgs
		flag.CommandLine = origCommandLine
	}()

	flag.CommandLine = flag.NewFlagSet("test", flag.ContinueOnError)
	os.Args = []string{"ralph", "plan-and-build", "--iterations", "10"}

	cfg := config.ParseFlags()

	if !cfg.IsPlanAndBuildMode() {
		t.Fatal("Expected plan-and-build mode to be detected")
	}
	// Plan phase should always be 1 iteration regardless of --iterations
	if cfg.Iterations != config.DefaultPlanIterations {
		t.Errorf("Expected Iterations = %d (plan phase fixed), got %d", config.DefaultPlanIterations, cfg.Iterations)
	}
	// --iterations should apply to BuildIterations
	if cfg.BuildIterations != 10 {
		t.Errorf("Expected BuildIterations = 10, got %d", cfg.BuildIterations)
	}
}

func TestBuildIterationsFieldDefault(t *testing.T) {
	cfg := config.NewConfig()
	if cfg.BuildIterations != 0 {
		t.Errorf("Expected BuildIterations = 0 by default, got %d", cfg.BuildIterations)
	}
}

// Helper function
func contains(s, substr string) bool {
	return len(s) >= len(substr) && (s == substr || len(s) > 0 && containsHelper(s, substr))
}

func containsHelper(s, substr string) bool {
	for i := 0; i <= len(s)-len(substr); i++ {
		if s[i:i+len(substr)] == substr {
			return true
		}
	}
	return false
}
