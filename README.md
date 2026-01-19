# Ralph

Ralph is a method. Ralph is not a specific model, assistant, prompt, nor is it an exact spec template.

Ralph is continuously looping on a given prompt with the ability to pause/edit/resume, to iteratively build the application and the spec together.

- You evolve a spec by sending it through Claude 10, 50, or `n` times
- Each iteration informs the next
- You can watch the evolution and catch regressions in real-time
- You can tune specs by observing behavior, cancelling the loop, editing the spec, and resuming
- You can delete `IMPLEMENTATION_PLAN.md` at any point and restart the loop

## Prerequisites

- **Claude CLI** installed and accessible in your PATH
- Terminal with 256-color support (recommended)

## Quickstart

1. Install ralph:
   ```bash
   brew tap agentic-metallurgy/tap
   brew install ralph
   ```

2. Create a new project directory (or use an existing repo)

3. Create `specs/default.md` with 5-10 lines describing what you'd like built

4. Run ralph:
   ```bash
   ralph
   ```

### CLI Options

```bash
# Run with defaults (20 iterations, specs/ folder)
ralph

# Custom number of iterations
ralph --iterations 10

# Use a specific spec file
ralph --spec-file /path/to/spec.md

# Specify a different specs folder
ralph --spec-folder /path/to/specs/

# Use a custom loop prompt instead of the embedded default
ralph --loop-prompt /path/to/custom_prompt.md
```

| Flag | Type | Default | Description |
|------|------|---------|-------------|
| `--iterations` | int | 20 | Number of loop iterations to run |
| `--spec-file` | string | - | Override with a specific spec file |
| `--spec-folder` | string | `specs/` | Directory containing spec files |
| `--loop-prompt` | string | - | Override the embedded prompt with a custom file |

## Development

### Requirements

- **Go 1.25.3** or compatible version
- **Claude CLI** installed and accessible in your PATH

### Building

```bash
git clone https://github.com/cloudosai/ralph-go.git
cd ralph-go
go build -o ralph ./cmd/ralph
```

### Running Tests

```bash
go test ./tests/...
```

## Credits & Inspiration

Thanks to [@ghuntley](https://github.com/ghuntley) for coining it, sharing it: [Ralph Wiggum as a Software Engineer](https://ghuntley.com/ralph/).
