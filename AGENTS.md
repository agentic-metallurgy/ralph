# Ralph - Agent Reference

## Build & Test
- Build: `go build -o ralph ./cmd/ralph`
- Test: `go test -v ./tests/`
- Test single: `go test -v -run TestName ./tests/`

## Project Structure
- `cmd/ralph/main.go` — entry point, wires loop/parser/tui together
- `internal/config/` — CLI flags, validation
- `internal/loop/` — Claude CLI execution loop (start/stop/pause/resume)
- `internal/parser/` — stream-json output parser
- `internal/prompt/` — embedded prompt loader (assets/prompt.md, assets/plan_prompt.md)
- `internal/stats/` — token usage tracking, persistence (.claude_stats)
- `internal/tmux/` — auto-wrap in tmux session
- `internal/tui/` — BubbleTea TUI (activity panel, footer, hotkeys)
- `tests/` — all test files
- `specs/` — feature specifications

## Subcommands
- `ralph` — default build mode (uses embedded build prompt)
- `ralph plan` — planning mode (uses embedded plan prompt)

## Key Flags
- `--iterations N` — loop count (default: 5)
- `--version` — print version and exit
- `--spec-file` / `--spec-folder` — spec overrides
- `--loop-prompt` — custom prompt override
- `--show-prompt` — print embedded prompt (respects plan mode)
- `--no-tmux` — skip tmux wrapping
- `--daemon` / `-d` — run without TUI, exit on completion
