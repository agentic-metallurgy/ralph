# Ralph-Go Implementation Plan

Port of `ralph-template` (Bash/Python) to a self-contained Go application using Bubble Tea, Lip Gloss, and Bubbles for the TUI.

## Overview

Ralph is a method for iterative software development using Claude AI. This Go implementation will:
1. Run a loop that pipes a prompt to the `claude` CLI
2. Visualize the output in a TUI (similar to the Python visualizer)
3. Track token usage and costs
4. Bundle the default prompt.md with the binary

## CLI Parameters

| Flag | Default | Description |
|------|---------|-------------|
| `--iterations` | 20 | Number of loop iterations |
| `--spec-file` | - | Specific spec file to use |
| `--spec-folder` | `specs/` | Folder containing spec files |
| `--loop-prompt` | embedded | Path to loop prompt (defaults to bundled prompt.md) |

---

## TASKS (Prioritized)

### TASK 1: Project Setup and Basic Structure [HIGH PRIORITY]
**Status**: COMPLETED

Set up the Go project with proper module structure and dependencies.

**Steps**:
1. Initialize go.mod with module path `github.com/cloudosai/ralph-go`
2. Create directory structure:
   ```
   ralph-go/
   ├── cmd/
   │   └── ralph/
   │       └── main.go
   ├── internal/
   │   ├── config/
   │   │   └── config.go
   │   ├── loop/
   │   │   └── loop.go
   │   ├── tui/
   │   │   └── tui.go
   │   └── stats/
   │       └── stats.go
   ├── assets/
   │   └── prompt.md
   ├── tests/
   │   └── .gitkeep
   ├── go.mod
   └── go.sum
   ```
3. Add dependencies: bubble tea, lip gloss, bubbles
4. Create basic main.go with CLI flag parsing

**Validation**:
- [x] `go build ./...` succeeds
- [x] `go test ./...` runs (even if no tests yet)
- [x] Running binary with `--help` shows usage

---

### TASK 2: Configuration and CLI Flags [HIGH PRIORITY]
**Status**: COMPLETED

Implement configuration handling with CLI flags and defaults.

**Steps**:
1. Create `internal/config/config.go` with Config struct
2. Parse flags: `--iterations`, `--spec-file`, `--spec-folder`, `--loop-prompt`
3. Implement defaults and validation
4. Write unit tests for config parsing

**Validation**:
- [x] Config loads with default values
- [x] CLI flags override defaults correctly
- [x] Invalid values produce helpful errors
- [x] Unit tests pass (18 tests in tests/config_test.go)

---

### TASK 3: Embed and Load prompt.md [MEDIUM PRIORITY]
**Status**: COMPLETED

Bundle prompt.md with the binary using Go's embed package.

**Steps**:
1. Copy `prompt.md` from ralph-template to `internal/prompt/assets/prompt.md`
2. Use `//go:embed` to bundle the file
3. Create `internal/prompt/prompt.go` with Loader struct
4. Write unit tests in `tests/prompt_test.go`

**Validation**:
- [x] Embedded prompt loads correctly (704 bytes)
- [x] `--loop-prompt` override works
- [x] Unit tests pass (9 tests for prompt package)

---

### TASK 4: Loop Execution Engine [HIGH PRIORITY]
**Status**: COMPLETED

Implement the core loop that runs Claude CLI iterations.

**Steps**:
1. Create `internal/loop/loop.go`
2. Implement loop that:
   - Runs `claude` with proper flags (--print, --output-format stream-json, etc.)
   - Captures stdout/stderr
   - Emits loop markers (======= LOOP X/Y =======)
   - Handles iteration count
   - Sleeps between iterations
3. Return output via channel for TUI consumption
4. Write unit tests (mock claude command)

**Validation**:
- [x] Loop runs specified number of iterations
- [x] Output is streamed correctly
- [x] Can be cancelled gracefully
- [x] Unit tests pass (22 tests in tests/loop_test.go)

---

### TASK 5: Token Stats Tracking [MEDIUM PRIORITY]
**Status**: COMPLETED

Track token usage and costs, persisting to file.

**Steps**:
1. Create `internal/stats/stats.go`
2. Implement TokenStats struct with:
   - input_tokens, output_tokens
   - cache_creation_tokens, cache_read_tokens
   - total_cost_usd
3. Save/Load from `.claude_stats` JSON file
4. Parse usage from Claude's stream-json output
5. Write unit tests

**Validation**:
- [x] Stats accumulate correctly
- [x] Stats persist and reload
- [x] Cost calculation matches Python version
- [x] Unit tests pass (19 tests in tests/stats_test.go)

---

### TASK 6: JSON Stream Parser [MEDIUM PRIORITY]
**Status**: NOT STARTED

Parse Claude's stream-json output format.

**Steps**:
1. Create parser in `internal/tui/` or dedicated package
2. Handle message types: system, assistant, user, result
3. Extract:
   - Token usage
   - Cost from result messages
   - Text content (strip system-reminders)
   - Tool usage
4. Parse loop markers
5. Write unit tests with sample JSON

**Validation**:
- [ ] All message types parsed correctly
- [ ] System reminders stripped
- [ ] Loop markers detected
- [ ] Unit tests pass

---

### TASK 7: Basic TUI Layout [MEDIUM PRIORITY]
**Status**: NOT STARTED

Create the Bubble Tea TUI with layout matching Python visualizer.

**Steps**:
1. Create `internal/tui/tui.go` with Bubble Tea model
2. Implement layout:
   - Activity panel (top) - scrolling message list
   - Footer (bottom) with 3 columns:
     - Usage & Cost stats
     - Loop Details (current/total, elapsed time)
     - Placeholder
3. Style with Lip Gloss to match Python version colors
4. Write basic tests

**Validation**:
- [ ] TUI renders correctly in terminal
- [ ] Layout matches Python visualizer
- [ ] Colors/styling correct
- [ ] Tests pass

---

### TASK 8: Activity Feed Component [MEDIUM PRIORITY]
**Status**: NOT STARTED

Implement the scrolling activity feed showing Claude's actions.

**Steps**:
1. Create message list component using Bubbles
2. Display messages with role icons:
   - 🤖 Assistant (blue)
   - 🔧 Tool use (purple)
   - 📝 User results (gray)
3. Limit to last 20 messages
4. Auto-scroll to bottom

**Validation**:
- [ ] Messages display correctly
- [ ] Icons and colors match Python version
- [ ] Scrolling works
- [ ] Max messages respected

---

### TASK 9: Footer Stats Panels [MEDIUM PRIORITY]
**Status**: NOT STARTED

Implement the footer panels showing stats.

**Steps**:
1. Create Usage & Cost panel with token counts and cost
2. Create Loop Details panel with progress and elapsed time
3. Create Placeholder panel
4. Style with Lip Gloss

**Validation**:
- [ ] All panels render correctly
- [ ] Stats update in real-time
- [ ] Elapsed time formats correctly (HH:MM:SS)

---

### TASK 10: Integration and Main Loop [HIGH PRIORITY]
**Status**: NOT STARTED

Wire everything together in main.go.

**Steps**:
1. Parse config
2. Load prompt
3. Start TUI
4. Run loop in goroutine
5. Stream output to TUI
6. Handle graceful shutdown (Ctrl+C)

**Validation**:
- [ ] Full application runs end-to-end
- [ ] TUI updates in real-time
- [ ] Stats save on exit
- [ ] Clean shutdown

---

### TASK 11: End-to-End Testing [LOW PRIORITY]
**Status**: NOT STARTED

Create comprehensive tests.

**Steps**:
1. Unit tests for all packages
2. Integration tests with mock claude command
3. Test CLI flag combinations
4. Test error handling

**Validation**:
- [ ] All tests pass
- [ ] Reasonable code coverage
- [ ] Edge cases handled

---

### TASK 12: Documentation and Polish [LOW PRIORITY]
**Status**: NOT STARTED

Final polish and documentation.

**Steps**:
1. Update README.md with usage instructions
2. Add --help output documentation
3. Review and clean up code
4. Ensure linting passes

**Validation**:
- [ ] README is comprehensive
- [ ] `go vet ./...` passes
- [ ] `golint ./...` passes (or staticcheck)

---

## Progress Log

| Date | Task | Status | Notes |
|------|------|--------|-------|
| 2025-12-03 | TASK 1: Project Setup | COMPLETED | Created project structure, go.mod, all packages, CLI flags working |
| 2025-12-03 | TASK 2: Configuration and CLI Flags | COMPLETED | Full validation, defaults, path handling. 18 tests passing |
| 2025-12-03 | TASK 3: Embed and Load prompt.md | COMPLETED | Go embed with Loader struct, 9 tests passing |
| 2025-12-03 | TASK 4: Loop Execution Engine | COMPLETED | Full loop implementation with CommandBuilder for DI, context cancellation, channel-based output. 22 tests passing with mock command builder |
| 2025-12-03 | TASK 5: Token Stats Tracking | COMPLETED | TokenStats with AddUsage, AddCost, TotalTokens, Save/Load methods. 19 tests passing for accumulation, persistence, and Python behavior matching |

---

## Architecture Notes

### Data Flow
```
main.go
    ↓
config.Parse() → Config struct
    ↓
prompt.Load() → prompt content
    ↓
Start TUI (Bubble Tea)
    ↓
Start Loop (goroutine)
    ↓
Loop → claude CLI → stdout
    ↓
Parser → Messages
    ↓
TUI Model updates → Render
```

### Key Dependencies
- `github.com/charmbracelet/bubbletea` - TUI framework
- `github.com/charmbracelet/lipgloss` - Styling
- `github.com/charmbracelet/bubbles` - UI components
