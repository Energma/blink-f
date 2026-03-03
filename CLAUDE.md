# CLAUDE.md — Blink Project Guide

## Project Overview
Blink is a lightweight, keyboard-driven TUI for git worktree management
with tmux integration and AI agent launching. Built by energma. Written in Go using BubbleTea v2.

Module: `github.com/Energma/blink-f`

## Build & Run
```bash
go build -o blink ./cmd/blink/    # Build
go run ./cmd/blink/               # Run directly
make build                        # Build via Makefile
make test                         # Run all tests
make lint                         # Run golangci-lint
```

## Test Commands
```bash
go test ./...                     # All tests
go test ./internal/git/...        # Git package tests
go test -v -run TestParseWorktree ./internal/git/  # Specific test
```

## Architecture
- **BubbleTea v2** (`charm.land/bubbletea/v2`): Elm-style Model-Update-View pattern
- **CLI**: urfave/cli/v3 for subcommands
- **Git**: CLI wrapper via os/exec (NOT go-git) — respects user's config, hooks, credentials
- **Config**: YAML with 5-level cascade (flags > env > local > global > defaults)
- **tmux**: Session management via os/exec — graceful degradation when unavailable

## Key Patterns
- All git operations go through `internal/git/Service` with semaphore concurrency control
- TUI state lives in `internal/app/Model` — single source of truth
- Screen management via push/pop stack in `internal/app/screen/Manager`
- Views are pure functions: read Model state, return rendered strings
- Custom messages (`tea.Msg` types) in `internal/app/messages.go`
- Async operations return `tea.Cmd` that produce messages
- Agent adapter is a tool runner, NOT an AI integration — launches external CLIs

## BubbleTea v2 API Notes
- Import: `charm.land/bubbletea/v2` (NOT `github.com/charmbracelet/bubbletea`)
- `View()` returns `string` (v2 simplified this)
- Keyboard: `tea.KeyPressMsg` with `msg.String()` for matching
- Use `tea.ExecProcess` to suspend TUI for tmux attach

## File Organization
- `cmd/blink/main.go` — CLI entry + subcommands
- `internal/app/` — TUI application (Model, handlers, views)
- `internal/git/` — Git CLI wrapper with semaphore
- `internal/tmux/` — tmux session management
- `internal/agent/` — AI agent provider registry (tool runner only)
- `internal/config/` — YAML config with cascade
- `internal/models/` — Shared data types
- `internal/theme/` — Color themes

## Conventions
- Conventional commits: `feat:`, `fix:`, `refactor:`, `chore:`, `test:`
- Error wrapping: always use `fmt.Errorf("context: %w", err)`
- Tests: table-driven, use `testify/assert`
- No global mutable state
- No emoji in code
