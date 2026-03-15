# Ralph - Agent Reference

## Build & Test
- Build: `go build -o ralph ./cmd/ralph`
- Test all: `go test -v ./tests/ ./cmd/ralph/`
- Test single: `go test -v -run TestName ./tests/`
- Test main: `go test -v ./cmd/ralph/`

## Project Structure
- `cmd/ralph/main.go` — entry point, wires loop/parser/tui together
- `internal/config/` — CLI flags, validation
- `internal/loop/` — Claude CLI execution loop (start/stop/pause/resume)
- `internal/parser/` — stream-json output parser
- `internal/prompt/` — embedded prompt loader (assets/prompt.md, assets/plan_prompt.md)
- `internal/stats/` — token usage tracking, persistence (.ralph.claude_stats)
- `internal/tmux/` — auto-wrap in tmux session
- `internal/tui/` — BubbleTea TUI (activity panel, footer, hotkeys)
- `tests/` — BDD and unit tests for internal packages
- `cmd/ralph/main_test.go` — tests for main.go functions (parseTaskCounts, isNewLoopStart)
- `specs/` — feature specifications

## Subcommands
- `ralph` — default build mode (uses embedded build prompt)
- `ralph build` — explicit build mode (identical to default)
- `ralph plan` — planning mode (uses embedded plan prompt)
- `ralph plan-and-build` — runs planning (1 iter) then building (default 5 iters) in one session

## Key Flags
- `--iterations N` — loop count (default: 5)
- `--version` — print version and exit
- `--spec-file` / `--spec-folder` — spec overrides
- `--loop-prompt` — custom prompt override
- `--show-prompt` — print embedded prompt (respects plan mode)
- `--no-tmux` — skip tmux wrapping
- `--cli` — run without TUI, output to stdout/stderr, exit on completion
