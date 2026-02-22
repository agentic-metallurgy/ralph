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
	Goal        string
	ShowPrompt  bool
	ShowVersion bool
	NoTmux      bool
	Daemon      bool
	Subcommand  string // "plan", "build", or "" (default: build mode)
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

// DetectSubcommand checks os.Args for a subcommand ("plan" or "build") before flag parsing.
// If found, it removes the subcommand from os.Args so flag.Parse() works correctly.
// Returns the detected subcommand or "".
func DetectSubcommand() string {
	if len(os.Args) > 1 {
		switch os.Args[1] {
		case "plan", "build":
			sub := os.Args[1]
			os.Args = append(os.Args[:1], os.Args[2:]...)
			return sub
		}
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
	flag.StringVar(&cfg.Goal, "goal", "", "Ultimate goal sentence for plan mode (used in plan prompt)")
	flag.BoolVar(&cfg.ShowPrompt, "show-prompt", false, "Print the embedded loop prompt and exit")
	flag.BoolVar(&cfg.ShowVersion, "version", false, "Print version and exit")
	flag.BoolVar(&cfg.NoTmux, "no-tmux", false, "Run without tmux wrapper")
	flag.BoolVar(&cfg.Daemon, "daemon", false, "Run without TUI, exit when all loops complete")
	flag.BoolVar(&cfg.Daemon, "d", false, "Run without TUI, exit when all loops complete (shorthand)")

	// Custom usage function to display flags with -- prefix
	flag.Usage = func() {
		fmt.Fprintf(os.Stderr, "Usage: %s [plan|build] [flags]\n\nSubcommands:\n  plan\tRun in planning mode (uses plan prompt instead of build prompt)\n  build\tRun in build mode (default if no subcommand specified)\n\nFlags:\n", os.Args[0])
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
		if err := c.validateSpecsAvailable(c.SpecFolder); err != nil {
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

// validateSpecsAvailable checks that a spec folder exists, is a directory, and contains at least one file.
// Returns user-friendly error messages with guidance on how to fix the issue.
func (c *Config) validateSpecsAvailable(path string) error {
	absPath, err := filepath.Abs(path)
	if err != nil {
		return fmt.Errorf("invalid spec folder path %q: %w", path, err)
	}

	info, err := os.Stat(absPath)
	if os.IsNotExist(err) {
		return fmt.Errorf("no spec files found: %s does not exist\nCreate a specs/ directory with spec files, or use --spec-file or --spec-folder to specify a custom location", path)
	}
	if err != nil {
		return fmt.Errorf("cannot access spec folder %q: %w", path, err)
	}
	if !info.IsDir() {
		return fmt.Errorf("--spec-folder: expected directory but got file: %s", path)
	}

	entries, err := os.ReadDir(absPath)
	if err != nil {
		return fmt.Errorf("cannot read spec folder %q: %w", path, err)
	}
	if len(entries) == 0 {
		return fmt.Errorf("no spec files found: %s is empty\nAdd spec files to the directory, or use --spec-file or --spec-folder to specify a custom location", path)
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
