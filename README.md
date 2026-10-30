# Blink

Fast, keyboard-driven TUI for managing git worktrees, tmux sessions, and AI agents from one place.

![Go](https://img.shields.io/badge/Go-1.25-00ADD8?logo=go&logoColor=white)
![License](https://img.shields.io/badge/license-MIT-blue)
<img width="958" height="922" alt="image" src="https://github.com/user-attachments/assets/06bd28c3-0e6b-4a44-a4d4-3b34605effbb" />

## Install

One command on Linux and macOS:

```bash
curl -fsSL https://raw.githubusercontent.com/Energma/blink-f/main/install.sh | sh
```

On Windows, download the zip from the [releases page](https://github.com/Energma/blink-f/releases)
and put `blink.exe` on your `PATH`. The binary carries its own terminal multiplexer (rmx), so
nothing else needs installing.

From then on:

```bash
blink update          # replace the binary with the latest release, rmx included
blink update --check  # only tell me if there is one
```

Or build from source, with a `rust-mux` checkout next to this repo (needs Go and cargo):

```bash
git clone https://github.com/Energma/rust-terminal-multiplexer.git rust-mux
git clone https://github.com/Energma/blink-f.git
cd blink-f
make build
```

`go install github.com/Energma/blink-f/cmd/blink@latest` also works, but that build has no rmx
inside and needs `rmx` or `tmux` on `PATH`.

### Requirements

- **Git** (required)
- **Go 1.25+** and **Rust/cargo** (for building; cargo builds the bundled [rmx](https://github.com/Energma/rust-terminal-multiplexer))
- **tmux** (optional — only if you set `multiplexer: tmux`)

Session management ships inside the binary: `make build` compiles rmx from a
sibling `../rust-mux` checkout (override with `RMX_SRC=`) and embeds it, and
blink unpacks it to `~/.cache/blink/rmx/` on first run. A plain `go build`
skips the bundle and uses `rmx` or `tmux` from `PATH` instead.

## Why Blink?

Working on multiple branches in the same repo means juggling worktrees, terminal sessions, and AI coding tools across scattered windows. Blink puts it all in one fast interface:

- **One keypress** to create a worktree with a tmux session
- **One keypress** to switch between worktrees
- **One keypress** to launch Claude, OpenCode, or aider in the right directory
- **One keypress** to commit and push

No more getting lost.

## Usage

```bash
blink              # Launch TUI
blink status       # Quick status overview
blink list         # List worktrees
blink new <branch> # Create worktree + tmux session
blink switch <name># Switch to worktree session
blink agent        # Launch AI agent in current worktree
blink clean        # Remove merged worktrees
blink config       # Open config in $EDITOR
```

## Keybindings

### Navigation
| Key | Action |
|-----|--------|
| `j/k` `arrows` | Navigate worktree list |
| `g` / `G` | Jump to top / bottom |
| `enter` | Switch to worktree (tmux session) |
| `l` / `right` | Open detail view |
| `tab` | Switch repository |
| `/` | Filter worktrees |
| `r` | Refresh |

Mouse: click a pane to focus it and a row to select it, double-click to act as `enter`, scroll to
move the cursor, click the preview to attach to its session. Set `ui.mouse: false` to turn it off.

### Worktrees
| Key | Action |
|-----|--------|
| `n` | Create new worktree |
| `d` | Delete worktree |
| `D` | Clean merged worktrees |

### Git
| Key | Action |
|-----|--------|
| `c` | Commit (conventional commit editor) |
| `p` | Push to remote |
| `u` | Pull from remote |
| `s` | Stash changes |

### Sessions & Tools
| Key | Action |
|-----|--------|
| `S` | Manage tmux sessions |
| `a` | Launch AI agent |
| `e` | Open editor in worktree |

### General
| Key | Action |
|-----|--------|
| `?` | Help |
| `esc` | Close modal / cancel |
| `q` | Quit |

## Configuration

Blink uses YAML config with cascade: **CLI flags > env vars > repo-local > global > defaults**.

Global config: `~/.config/blink/config.yaml`
Repo-local: `.blink.yaml` in your repo root

```yaml
repos:
  - path: ~/projects/my-app
    default_branch: main

agents:
  default: claude
  providers:
    claude:   { command: claude, args: [] }
    opencode: { command: opencode, args: [] }
    aider:    { command: aider, args: [] }

editor:
  command: code
  args: ["."]

multiplexer: rmx    # or tmux; shell/auto_session below apply to both

tmux:
  auto_session: true
  popup_width: "80%"
  popup_height: "80%"

worktree:
  base_dir: ".worktrees"
  auto_symlinks: [.env, .env.local, node_modules, vendor]
  cleanup_merged: true

git:
  conventional_commits: true
  auto_push: false

ui:
  theme: default
  show_icons: true
  mouse: true        # false leaves the mouse to the terminal (text selection)
```

### Environment Variables

| Variable | Description |
|----------|-------------|
| `BLINK_DEFAULT_AGENT` | Override default agent provider |
| `BLINK_THEME` | Override UI theme |
| `BLINK_MULTIPLEXER` | Session backend: `rmx` (default) or `tmux` |
| `BLINK_SHELL` | Override session shell |

### rmx and tmux

Worktree sessions are rmx workspaces with the same names tmux sessions had. Things to know:

- Run blink **outside** rmx: rmx can't switch workspaces from inside one of its panes, so `enter`
  attaches from a plain terminal and you come back to blink when you detach (Leader `d`).
- Agents get the whole window (no automatic agent/shell split); split by hand with Leader `-`.

Set `multiplexer: tmux` (or `BLINK_MULTIPLEXER=tmux`) to use tmux from `PATH` instead. tmux
keeps the agent/shell split, `blink agent --popup`, and the `ctrl+q` key table.

## Features

### Worktree Management
Create, switch, delete, and clean git worktrees. Auto-symlinks `.env`, `node_modules`, etc. from main worktree. Auto-creates tmux sessions on worktree creation.

### Conventional Commits
Built-in commit editor with type selector (feat, fix, refactor, chore, etc.), optional scope, and message — with live preview.

### AI Agent Integration
Provider-agnostic agent launcher. Blink doesn't embed AI — it launches external CLIs (claude, opencode, aider) in the right worktree directory via tmux popup or session. Any CLI tool is pluggable via config.

### Session Management
View all tmux sessions, see which are linked to worktrees, which run agents. Attach, kill, or clean orphaned sessions from one panel.

### Multi-Repo Support
Configure multiple repos and switch between them with `tab`. Auto-detects the current directory as a repo on launch.

## Architecture

```
cmd/blink/           CLI entry + subcommands
internal/
  app/               BubbleTea v2 TUI (Model-Update-View)
    views/           Pure render functions (dashboard, detail, commit, etc.)
    components/      Reusable UI pieces (status bar, worktree items, filter)
    screen/          Push/pop screen stack for modals
  git/               Git CLI wrapper with semaphore concurrency
  tmux/              rmx + tmux session lifecycle
  rmxbin/            embedded rmx binary, unpacked on first use
  agent/             Provider interface + registry (tool runner)
  config/            YAML config with cascade + validation
  models/            Shared data types
  theme/             Color themes
```

- **Git via CLI** — respects your config, hooks, credentials. No go-git.
- **Semaphore-bounded** concurrency for git operations.
- **Graceful degradation** — if the bundled rmx can't run, session features just disable.
- **Single binary** — multiplexer included, instant startup.

## License

MIT

---

Built by [energma](https://github.com/Energma)
