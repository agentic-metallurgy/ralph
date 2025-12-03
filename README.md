# Ralph

Ralph is a method. Ralph is not a specific model, assistant, prompt, nor is it an exact spec template.

Ralph is continuously looping on a given prompt with the ability to pause/edit/resume, to iteratively build the application and the spec together.

## Features

- Run Claude CLI in an iterative loop with live visual feedback
- Terminal UI (TUI) displaying streamed Claude output in real-time
- Token usage and cost tracking across iterations
- Embedded default prompt bundled with the binary
- Graceful shutdown with stats persistence

## Prerequisites

- **Go 1.25.3** or compatible version
- **Claude CLI** installed and accessible in your PATH
- Terminal with 256-color support (recommended)

## Building

```bash
# Clone the repository
git clone https://github.com/cloudosai/ralph-go.git
cd ralph-go

# Build the binary
go build -o ralph ./cmd/ralph
```

## Running

### Quickstart

1. Create a new project directory (or use an existing repo)

2. Create `specs/default.md` with 5-10 lines describing what you'd like built

3. Run ralph:
   ```bash
   ./ralph
   ```

### CLI Options

```bash
# Run with defaults (20 iterations, specs/ folder)
./ralph

# Custom number of iterations
./ralph --iterations 10

# Use a specific spec file
./ralph --spec-file /path/to/spec.md

# Specify a different specs folder
./ralph --spec-folder /path/to/specs/

# Use a custom loop prompt instead of the embedded default
./ralph --loop-prompt /path/to/custom_prompt.md
```

| Flag | Type | Default | Description |
|------|------|---------|-------------|
| `--iterations` | int | 20 | Number of loop iterations to run |
| `--spec-file` | string | - | Override with a specific spec file |
| `--spec-folder` | string | `specs/` | Directory containing spec files |
| `--loop-prompt` | string | - | Override the embedded prompt with a custom file |

## How It Works

Ralph executes the Claude CLI in a loop, streaming output through a terminal UI:

1. **Load Configuration** - Parse CLI flags and validate paths
2. **Load Prompt** - Use embedded prompt or custom file
3. **Load Stats** - Restore token usage from previous runs (`.claude_stats`)
4. **Execute Loop** - For each iteration:
   - Run `claude` CLI with streaming JSON output
   - Parse responses and extract token usage/costs
   - Update the TUI with messages and stats
5. **Save Stats** - Persist accumulated token usage on exit

### TUI Layout

```
┌─────────────────────────────────────────────────┐
│  Activity Feed                                  │
│  🤖 Assistant: Working on implementation...     │
│  🔧 Tool: Reading file src/main.go             │
│  📝 Result: File contents loaded               │
│  💰 Cost: $0.0234                              │
├─────────────────────────────────────────────────┤
│  Usage & Cost    │  Loop Details  │            │
│  Tokens: 15,432  │  Loop: 3/20    │            │
│  Cost: $0.0847   │  Time: 00:05:23│            │
└─────────────────────────────────────────────────┘
```

## Iterative Workflows

Ralph enables workflows where:

- You evolve a spec by sending it through Claude 10, 50, or `n` times
- Each iteration informs the next
- You can watch the evolution and catch regressions in real-time
- You can tune specs by observing behavior, cancelling the loop, editing the spec, and resuming
- You can delete `IMPLEMENTATION_PLAN.md` at any point and restart the loop

## Project Structure

```
ralph-go/
├── cmd/ralph/
│   └── main.go              # Application entry point
├── internal/
│   ├── config/              # CLI flag parsing
│   ├── loop/                # Claude CLI execution loop
│   ├── parser/              # JSON stream parser
│   ├── prompt/              # Prompt loading with embed
│   │   └── assets/prompt.md # Embedded default prompt
│   ├── stats/               # Token usage tracking
│   └── tui/                 # Bubble Tea terminal UI
├── tests/                   # Unit tests
├── specs/                   # Spec files directory
├── go.mod
└── README.md
```

## Output Files

- `.claude_stats` - JSON file with accumulated token usage and costs (auto-created/updated)

## Running Tests

```bash
go test ./tests/...
```

## Dependencies

- [Bubble Tea](https://github.com/charmbracelet/bubbletea) - TUI framework
- [Lip Gloss](https://github.com/charmbracelet/lipgloss) - Terminal styling
- [Bubbles](https://github.com/charmbracelet/bubbles) - TUI components

## License

See LICENSE file for details.
