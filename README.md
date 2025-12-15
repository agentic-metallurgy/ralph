# Ralph

Ralph is a method. Ralph is not a specific model, assistant, prompt, nor is it an exact spec template.

Ralph is continuously looping on a given prompt with the ability to pause/edit/resume, to iteratively build the application and the spec together.

- You evolve a spec by sending it through Claude, Cursor, or other AI agents 10, 50, or `n` times
- Each iteration informs the next
- You can watch the evolution and catch regressions in real-time
- You can tune specs by observing behavior, cancelling the loop, editing the spec, and resuming
- You can delete `IMPLEMENTATION_PLAN.md` at any point and restart the loop

## Prerequisites

Choose one of the following AI backends:

- **Claude CLI** - Install and authenticate the [Claude CLI](https://docs.anthropic.com/en/docs/claude-cli)
- **Cursor Agent** - Install the [Cursor CLI](https://cursor.com/docs/cli/installation) and set `CURSOR_API_KEY`

Additionally:
- Terminal with 256-color support (recommended)

## Quickstart

1. Install ralph:
   ```bash
   brew tap agentic-metallurgy/ralph
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
# Run with defaults (Claude backend, 5 iterations, specs/ folder)
ralph

# Use cursor-agent instead of Claude
ralph --backend cursor-agent

# Custom number of iterations
ralph --iterations 10

# Use a specific spec file
ralph --spec-file /path/to/spec.md

# Specify a different specs folder
ralph --spec-folder /path/to/specs/

# Use a custom loop prompt instead of the embedded default
ralph --loop-prompt /path/to/custom_prompt.md

# Combine options
ralph --backend cursor-agent --iterations 20 --spec-folder ./my-specs/
```

| Flag | Type | Default | Description |
|------|------|---------|-------------|
| `--backend` | string | `claude` | AI backend to use: `claude` or `cursor-agent` |
| `--iterations` | int | 5 | Number of loop iterations to run |
| `--spec-file` | string | - | Override with a specific spec file |
| `--spec-folder` | string | `specs/` | Directory containing spec files |
| `--loop-prompt` | string | - | Override the embedded prompt with a custom file |

### Backends

#### Claude (default)

Uses the [Claude CLI](https://docs.anthropic.com/en/docs/claude-cli) with agentic capabilities and subagent support.

```bash
ralph --backend claude
```

#### Cursor Agent

Uses [Cursor's headless CLI](https://cursor.com/docs/cli/headless) for non-interactive automation.

```bash
# Install Cursor CLI
curl https://cursor.com/install -fsS | bash

# Set your API key
export CURSOR_API_KEY=your_api_key_here

# Run ralph with cursor-agent
ralph --backend cursor-agent
```

> **Note:** Cursor's subagent feature is still in development. The embedded prompt references "up to 5 subagents" which works with Claude but will be ignored by cursor-agent.

## Credits & Inspiration

Ralph was inspired by:

- **Geoffrey Huntley's** article [Ralph Wiggum as a Software Engineer](https://ghuntley.com/ralph/) - the original vision for iterative spec-driven development with AI
- **HumanLayer's** [Advanced Context Engineering for Coding Agents](https://github.com/humanlayer/advanced-context-engineering-for-coding-agents/blob/main/ace-fca.md) - techniques for building effective AI-assisted development workflows
