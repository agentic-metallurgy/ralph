package config

import (
	"fmt"
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
		Iterations: 20,
		SpecFile:   "",
		SpecFolder: "specs/",
		LoopPrompt: "",
	}
}

// Validate checks if the configuration is valid
func (c *Config) Validate() error {
	if c.Iterations <= 0 {
		return fmt.Errorf("iterations must be greater than 0, got %d", c.Iterations)
	}
	return nil
}

// String returns a string representation of the Config for debug printing
func (c *Config) String() string {
	return fmt.Sprintf("Config{Iterations: %d, SpecFile: %q, SpecFolder: %q, LoopPrompt: %q}",
		c.Iterations, c.SpecFile, c.SpecFolder, c.LoopPrompt)
}
