# Ralph - Summary

Ralph is a CLI tool written in Go that automates iterative software development by continuously looping the Claude CLI against a spec file.

## What It Does

Ralph repeatedly invokes the `claude` CLI in a loop (defaulting to 5 iterations), feeding it a prompt that instructs Claude to read an `IMPLEMENTATION_PLAN.md`, implement the highest-priority task, run tests, commit changes, and update the plan. Each iteration builds on the previous one, progressively evolving the codebase.

## How It Works

1. **Loop Engine** (`internal/loop/`): Spawns the `claude` CLI as a subprocess for each iteration, piping a prompt via stdin and streaming JSON output from stdout/stderr. Supports pause, resume, and stop controls.

2. **JSON Parser** (`internal/parser/`): Parses Claude's `stream-json` output line-by-line, extracting message types (assistant, user, system, result), token usage, tool invocations, and cost data.

3. **Terminal UI** (`internal/tui/`): A full-screen TUI built with Bubble Tea and Lip Gloss (Tokyo Night theme). Displays a live activity feed of Claude's actions, a usage/cost panel, loop progress, elapsed time, and a control panel for stopping/resuming the loop.

4. **Stats Tracker** (`internal/stats/`): Tracks cumulative token usage (input, output, cache) and cost across iterations, persisted to a `.ralph.claude_stats` JSON file between sessions.

5. **Prompt Loader** (`internal/prompt/`): Loads the loop prompt from an embedded default (`prompt.md`) or from a user-provided override file.

6. **Config** (`internal/config/`): Parses CLI flags (`--iterations`, `--spec-file`, `--spec-folder`, `--loop-prompt`, `--show-prompt`) and validates paths.

## Key Concept

Ralph is a method, not a model. You write a short spec, Ralph loops Claude over it N times, and you watch the application and spec co-evolve in real time -- pausing to edit specs or restart whenever needed.
