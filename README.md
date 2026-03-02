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

### Subcommands

```bash
ralph              # Build mode (default)
ralph build        # Explicit build mode (same as default)
ralph plan         # Planning mode (uses plan prompt)
ralph plan-and-build  # Run planning (1 iter) then building (default 5 iters)
```

### CLI Options

```bash
# Run with defaults (5 iterations, specs/ folder)
ralph

# Custom number of iterations
ralph --iterations 10

# Use a specific spec file
ralph --spec-file /path/to/spec.md

# Specify a different specs folder
ralph --spec-folder /path/to/specs/

# Use a custom loop prompt instead of the embedded default
ralph --loop-prompt /path/to/custom_prompt.md

# Run in CLI mode (no TUI, outputs to stdout/stderr, exits on completion)
ralph --cli
ralph -c

# Chain plan and build in CLI mode
ralph plan --iterations 2 -c; ralph build --iterations 4

# Run plan-and-build (planning then building in one session)
ralph plan-and-build              # 1 plan iteration + 5 build iterations
ralph plan-and-build --iterations 10  # 1 plan iteration + 10 build iterations
```

| Flag | Type | Default | Description |
|------|------|---------|-------------|
| `--iterations` | int | 5 | Number of loop iterations to run |
| `--spec-file` | string | - | Override with a specific spec file |
| `--spec-folder` | string | `specs/` | Directory containing spec files |
| `--loop-prompt` | string | - | Override the embedded prompt with a custom file |
| `--goal` | string | - | Ultimate goal sentence (plan mode) |
| `--cli` / `-c` | bool | false | Run without TUI, output to stdout/stderr, exit on completion |
| `--no-tmux` | bool | false | Skip automatic tmux wrapping |
| `--show-prompt` | bool | false | Print the embedded prompt and exit |
| `--version` | bool | false | Print version and exit |

## Requirements

- **Go 1.25.3** or compatible version
- **Claude CLI** installed and accessible in your PATH

## Credits & Inspiration

Thanks to [@ghuntley](https://github.com/ghuntley) for coining it, sharing it: [Ralph Wiggum as a Software Engineer](https://ghuntley.com/ralph/).
