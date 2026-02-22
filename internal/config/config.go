package config

import (
	"flag"
	"fmt"
	"os"
	"path/filepath"
)

// Default values for configuration
const (
	DefaultIterations = 5
	DefaultSpecFolder = "specs/"
)

// Version is set at build time via -ldflags
var Version = "dev"

// Config holds the configuration for the ralph-go application
type Config struct {
	Iterations  int
	SpecFile    string
	SpecFolder  string
	LoopPrompt  string
	ShowPrompt  bool
	ShowVersion bool
	NoTmux      bool
	Subcommand  string // "plan" or "" (default: build mode)
}

// NewConfig returns a new Config with default values
func NewConfig() *Config {
	return &Config{
		Iterations: DefaultIterations,
		SpecFile:   "",
		SpecFolder: DefaultSpecFolder,
		LoopPrompt: "",
	}
}

// DetectSubcommand checks os.Args for a subcommand ("plan") before flag parsing.
// If found, it removes the subcommand from os.Args so flag.Parse() works correctly.
// Returns the detected subcommand or "".
func DetectSubcommand() string {
	if len(os.Args) > 1 && os.Args[1] == "plan" {
		os.Args = append(os.Args[:1], os.Args[2:]...)
		return "plan"
	}
	return ""
}

// ParseFlags parses command-line flags and returns a Config.
// It defines the flags, parses them, and returns the resulting configuration.
func ParseFlags() *Config {
	cfg := NewConfig()

	// Detect subcommand before flag parsing
	cfg.Subcommand = DetectSubcommand()

	flag.IntVar(&cfg.Iterations, "iterations", DefaultIterations, "Number of loop iterations")
	flag.StringVar(&cfg.SpecFile, "spec-file", "", "Specific spec file to use (overrides spec-folder)")
	flag.StringVar(&cfg.SpecFolder, "spec-folder", DefaultSpecFolder, "Folder containing spec files")
	flag.StringVar(&cfg.LoopPrompt, "loop-prompt", "", "Path to loop prompt override (defaults to embedded prompt.md)")
	flag.BoolVar(&cfg.ShowPrompt, "show-prompt", false, "Print the embedded loop prompt and exit")
	flag.BoolVar(&cfg.ShowVersion, "version", false, "Print version and exit")
	flag.BoolVar(&cfg.NoTmux, "no-tmux", false, "Run without tmux wrapper")

	// Custom usage function to display flags with -- prefix
	flag.Usage = func() {
		fmt.Fprintf(os.Stderr, "Usage: %s [plan] [flags]\n\nSubcommands:\n  plan\tRun in planning mode (uses plan prompt instead of build prompt)\n\nFlags:\n", os.Args[0])
		flag.VisitAll(func(f *flag.Flag) {
			// Format: --flag-name type
			//     description (default: value)
			fmt.Fprintf(os.Stderr, "  --%s", f.Name)
			// Get the type name from the default value
			typeName, usage := flag.UnquoteUsage(f)
			if len(typeName) > 0 {
				fmt.Fprintf(os.Stderr, " %s", typeName)
			}
			fmt.Fprintf(os.Stderr, "\n")
			fmt.Fprintf(os.Stderr, "    \t%s", usage)
			// Show default for non-empty, non-false values
			if f.DefValue != "" && f.DefValue != "false" {
				fmt.Fprintf(os.Stderr, " (default: %s)", f.DefValue)
			}
			fmt.Fprintf(os.Stderr, "\n")
		})
	}

	flag.Parse()

	return cfg
}

// IsPlanMode returns true if the "plan" subcommand was specified
func (c *Config) IsPlanMode() bool {
	return c.Subcommand == "plan"
}

// Validate checks if the configuration is valid.
// It validates:
// - Iterations must be greater than 0
// - If spec-file is provided, it must exist
// - If spec-folder is provided (and spec-file is not), it must exist (unless using custom loop-prompt)
// - If loop-prompt is provided, it must exist
func (c *Config) Validate() error {
	if c.Iterations <= 0 {
		return fmt.Errorf("--iterations must be greater than 0, got %d", c.Iterations)
	}

	if c.SpecFile != "" {
		if err := c.validateFileExists(c.SpecFile, "--spec-file"); err != nil {
			return err
		}
	} else if c.SpecFolder != "" && c.LoopPrompt == "" {
		// Only validate spec-folder when using the default embedded prompt.
		// Custom prompts may not need specs at all.
		if err := c.validateDirExists(c.SpecFolder, "--spec-folder"); err != nil {
			return err
		}
	}

	if c.LoopPrompt != "" {
		if err := c.validateFileExists(c.LoopPrompt, "--loop-prompt"); err != nil {
			return err
		}
	}

	return nil
}

// validateFileExists checks if a file exists at the given path
func (c *Config) validateFileExists(path, flagName string) error {
	absPath, err := filepath.Abs(path)
	if err != nil {
		return fmt.Errorf("%s: invalid path %q: %w", flagName, path, err)
	}

	info, err := os.Stat(absPath)
	if os.IsNotExist(err) {
		return fmt.Errorf("%s: file does not exist: %s", flagName, path)
	}
	if err != nil {
		return fmt.Errorf("%s: cannot access %q: %w", flagName, path, err)
	}
	if info.IsDir() {
		return fmt.Errorf("%s: expected file but got directory: %s", flagName, path)
	}

	return nil
}

// validateDirExists checks if a directory exists at the given path
func (c *Config) validateDirExists(path, flagName string) error {
	absPath, err := filepath.Abs(path)
	if err != nil {
		return fmt.Errorf("%s: invalid path %q: %w", flagName, path, err)
	}

	info, err := os.Stat(absPath)
	if os.IsNotExist(err) {
		return fmt.Errorf("%s: directory does not exist: %s", flagName, path)
	}
	if err != nil {
		return fmt.Errorf("%s: cannot access %q: %w", flagName, path, err)
	}
	if !info.IsDir() {
		return fmt.Errorf("%s: expected directory but got file: %s", flagName, path)
	}

	return nil
}

// String returns a string representation of the Config for debug printing
func (c *Config) String() string {
	return fmt.Sprintf("Config{Iterations: %d, SpecFile: %q, SpecFolder: %q, LoopPrompt: %q}",
		c.Iterations, c.SpecFile, c.SpecFolder, c.LoopPrompt)
}

// GetSpecPath returns the effective spec path to use.
// If SpecFile is set, it returns that. Otherwise, it returns SpecFolder.
func (c *Config) GetSpecPath() string {
	if c.SpecFile != "" {
		return c.SpecFile
	}
	return c.SpecFolder
}

// IsUsingSpecFile returns true if a specific spec file is configured
func (c *Config) IsUsingSpecFile() bool {
	return c.SpecFile != ""
}

// IsUsingCustomPrompt returns true if a custom loop prompt is configured
func (c *Config) IsUsingCustomPrompt() bool {
	return c.LoopPrompt != ""
}
