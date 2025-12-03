package config

import (
	"flag"
	"fmt"
	"os"
	"path/filepath"
)

// Default values for configuration
const (
	DefaultIterations = 20
	DefaultSpecFolder = "specs/"
)

// Config holds the configuration for the ralph-go application
type Config struct {
	Iterations int
	SpecFile   string
	SpecFolder string
	LoopPrompt string
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

// ParseFlags parses command-line flags and returns a Config.
// It defines the flags, parses them, and returns the resulting configuration.
func ParseFlags() *Config {
	cfg := NewConfig()

	flag.IntVar(&cfg.Iterations, "iterations", DefaultIterations, "Number of loop iterations")
	flag.StringVar(&cfg.SpecFile, "spec-file", "", "Specific spec file to use (overrides spec-folder)")
	flag.StringVar(&cfg.SpecFolder, "spec-folder", DefaultSpecFolder, "Folder containing spec files")
	flag.StringVar(&cfg.LoopPrompt, "loop-prompt", "", "Path to loop prompt override (defaults to embedded prompt.md)")

	flag.Parse()

	return cfg
}

// Validate checks if the configuration is valid.
// It validates:
// - Iterations must be greater than 0
// - If spec-file is provided, it must exist
// - If spec-folder is provided (and spec-file is not), it must exist
// - If loop-prompt is provided, it must exist
func (c *Config) Validate() error {
	if c.Iterations <= 0 {
		return fmt.Errorf("--iterations must be greater than 0, got %d", c.Iterations)
	}

	if c.SpecFile != "" {
		if err := c.validateFileExists(c.SpecFile, "--spec-file"); err != nil {
			return err
		}
	} else if c.SpecFolder != "" {
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
