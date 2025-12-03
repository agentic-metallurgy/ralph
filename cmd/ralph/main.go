package main

import (
	"flag"
	"fmt"
)

// CLI flags
var (
	iterations int
	specFile   string
	specFolder string
	loopPrompt string
)

// init initializes and defines all CLI flags
func init() {
	flag.IntVar(&iterations, "iterations", 20, "Number of loop iterations")
	flag.StringVar(&specFile, "spec-file", "", "Specific spec file to use")
	flag.StringVar(&specFolder, "spec-folder", "specs/", "Folder containing spec files")
	flag.StringVar(&loopPrompt, "loop-prompt", "", "Path to loop prompt override")
}

func main() {
	// Parse command-line flags
	flag.Parse()

	// Display configuration
	fmt.Println("Ralph-Go CLI Configuration:")
	fmt.Println("===========================")
	fmt.Printf("Iterations:   %d\n", iterations)
	fmt.Printf("Spec File:    %s\n", specFile)
	fmt.Printf("Spec Folder:  %s\n", specFolder)
	fmt.Printf("Loop Prompt:  %s\n", loopPrompt)
	fmt.Println()

	// TODO: Initialize TUI visualizer
	// The TUI will be started here to display the Claude loop execution
	// in real-time with a visual interface
	fmt.Println("TODO: Start TUI visualizer")
	fmt.Println("TODO: Run Claude in a loop with the configured parameters")
}
