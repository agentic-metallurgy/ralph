package main

import (
	"fmt"
	"os"

	"github.com/cloudosai/ralph-go/internal/config"
	"github.com/cloudosai/ralph-go/internal/prompt"
)

func main() {
	// Parse command-line flags and get configuration
	cfg := config.ParseFlags()

	// Validate configuration
	if err := cfg.Validate(); err != nil {
		fmt.Fprintf(os.Stderr, "Error: %v\n", err)
		os.Exit(1)
	}

	// Load the loop prompt (embedded or from override file)
	promptLoader := prompt.NewLoader(cfg.LoopPrompt)
	promptContent, err := promptLoader.Load()
	if err != nil {
		fmt.Fprintf(os.Stderr, "Error loading prompt: %v\n", err)
		os.Exit(1)
	}

	// Display configuration
	fmt.Println("Ralph-Go Configuration:")
	fmt.Println("=======================")
	fmt.Printf("Iterations:   %d\n", cfg.Iterations)
	if cfg.IsUsingSpecFile() {
		fmt.Printf("Spec File:    %s\n", cfg.SpecFile)
	} else {
		fmt.Printf("Spec Folder:  %s\n", cfg.SpecFolder)
	}
	if promptLoader.IsUsingOverride() {
		fmt.Printf("Loop Prompt:  %s\n", cfg.LoopPrompt)
	} else {
		fmt.Println("Loop Prompt:  (embedded default)")
	}
	fmt.Printf("Prompt Size:  %d bytes\n", len(promptContent))
	fmt.Println()

	// TODO: Initialize TUI visualizer
	// The TUI will be started here to display the Claude loop execution
	// in real-time with a visual interface
	fmt.Println("TODO: Start TUI visualizer")
	fmt.Println("TODO: Run Claude in a loop with the configured parameters")
}
