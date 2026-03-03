# Blink

Fast, keyboard-driven TUI for managing git worktrees, tmux sessions, and AI agents from one place.

![Go](https://img.shields.io/badge/Go-1.25-00ADD8?logo=go&logoColor=white)
![License](https://img.shields.io/badge/license-MIT-blue)
<img width="958" height="922" alt="image" src="https://github.com/user-attachments/assets/06bd28c3-0e6b-4a44-a4d4-3b34605effbb" />


## Why Blink?

Working on multiple branches in the same repo means juggling worktrees, terminal sessions, and AI coding tools across scattered windows. Blink puts it all in one fast interface:

- **One keypress** to create a worktree with a tmux session
- **One keypress** to switch between worktrees
- **One keypress** to launch Claude, OpenCode, or aider in the right directory
- **One keypress** to commit and push

No more getting lost.

## Install

```bash
go install github.com/Energma/blink-f/cmd/blink@latest
```

Or build from source:

```bash
git clone https://github.com/Energma/blink-f.git
cd blink
make build
```

### Requirements

- **Git** (required)
- **tmux** (optional — session features disabled gracefully without it)
- **Go 1.25+** (for building)

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
```

### Environment Variables

| Variable | Description |
|----------|-------------|
| `BLINK_DEFAULT_AGENT` | Override default agent provider |
| `BLINK_THEME` | Override UI theme |
| `BLINK_SHELL` | Override tmux shell |

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
  tmux/              tmux session lifecycle + popup
  agent/             Provider interface + registry (tool runner)
  config/            YAML config with cascade + validation
  models/            Shared data types
  theme/             Color themes
```

- **Git via CLI** — respects your config, hooks, credentials. No go-git.
- **Semaphore-bounded** concurrency for git operations.
- **Graceful degradation** — works without tmux, session features just disable.
- **Single binary** — ~6MB, instant startup.

## License

MIT

---

Built by [energma](https://github.com/energma-dev)
