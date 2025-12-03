# Developer Guide

## Requirements

- **Go 1.25.3** or compatible version
- **Claude CLI** installed and accessible in your PATH

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

## Building

```bash
git clone https://github.com/cloudosai/ralph-go.git
cd ralph-go
go build -o ralph ./cmd/ralph
```

## Running Tests

```bash
go test ./tests/...
```

## Dependencies

- [Bubble Tea](https://github.com/charmbracelet/bubbletea) - TUI framework
- [Lip Gloss](https://github.com/charmbracelet/lipgloss) - Terminal styling
- [Bubbles](https://github.com/charmbracelet/bubbles) - TUI components
