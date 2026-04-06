package components

import (
	"fmt"

	"github.com/Energma/blink-f/internal/app/screen"
	"github.com/Energma/blink-f/internal/models"
	"github.com/Energma/blink-f/internal/theme"
	"charm.land/lipgloss/v2"
)

// Stats holds computed stats for the status bar.
type Stats struct {
	Worktrees  int
	Sessions   int
	Dirty      int
	Clean      int
	Conflicts  int
	Agents     int
}

// ComputeStats computes dashboard stats from worktrees.
func ComputeStats(worktrees []models.Worktree) Stats {
	var s Stats
	s.Worktrees = len(worktrees)
	for _, wt := range worktrees {
		if wt.Status.TmuxSession != "" {
			s.Sessions++
		}
		if wt.Status.AgentRunning {
			s.Agents++
		}
		if wt.Status.Conflicts > 0 {
			s.Conflicts++
		} else if wt.Status.Clean {
			s.Clean++
		} else if wt.Status.Staged > 0 || wt.Status.Modified > 0 || wt.Status.Untracked > 0 {
			s.Dirty++
		}
	}
	return s
}

// StatusBar renders the bottom status bar with rich stats.
func StatusBar(width int, t *theme.Theme, repoName string, stats Stats, tmuxAvailable bool, activeScreen screen.Type, statusMsg, errMsg string) string {
	dim := lipgloss.NewStyle().Foreground(t.TextDim)
	sep := dim.Render(" | ")

	// Left side: repo + stats
	left := lipgloss.NewStyle().Foreground(t.Secondary).Bold(true).Render(" " + repoName)
	left += sep + dim.Render(fmt.Sprintf("%d wt", stats.Worktrees))

	if tmuxAvailable && stats.Sessions > 0 {
		left += sep + lipgloss.NewStyle().Foreground(t.Success).Render(fmt.Sprintf("%d sess", stats.Sessions))
	}
	if stats.Agents > 0 {
		left += sep + lipgloss.NewStyle().Foreground(t.Accent).Render(fmt.Sprintf("%d agent", stats.Agents))
	}
	if stats.Dirty > 0 {
		left += sep + lipgloss.NewStyle().Foreground(t.Warning).Render(fmt.Sprintf("%d dirty", stats.Dirty))
	}
	if stats.Conflicts > 0 {
		left += sep + lipgloss.NewStyle().Foreground(t.Error).Bold(true).Render(fmt.Sprintf("%d conflicts!", stats.Conflicts))
	}

	// Right side: status/error message
	right := ""
	if errMsg != "" {
		right = lipgloss.NewStyle().Foreground(t.Error).Render(errMsg + " ")
	} else if statusMsg != "" {
		right = lipgloss.NewStyle().Foreground(t.Success).Render(statusMsg + " ")
	}

	// Layout
	gap := width - lipgloss.Width(left) - lipgloss.Width(right) - 1
	if gap < 1 {
		gap = 1
	}
	spaces := make([]byte, gap)
	for i := range spaces {
		spaces[i] = ' '
	}

	return lipgloss.NewStyle().
		Foreground(t.TextDim).
		Width(width).
		Render(left + string(spaces) + right)
}

// PaneID mirrors the app.PaneID type for hint rendering.
type PaneID int

const (
	PaneWorktrees PaneID = iota
	PaneFileTree
)

// PaneHints returns the action hints to render inside a pane footer.
func PaneHints(t *theme.Theme, pane PaneID) string {
	k := lipgloss.NewStyle().Foreground(t.Secondary).Bold(true)
	d := lipgloss.NewStyle().Foreground(t.TextDim)
	sep := d.Render(" | ")

	h := func(binding, action string) string {
		return k.Render(binding) + " " + d.Render(action)
	}

	var parts []string
	switch pane {
	case PaneWorktrees:
		parts = []string{
			h("n", "new"),
			h("b", "branch"),
			h("d", "delete"),
			h("D", "clean"),
			sep,
			h("c", "commit"),
			h("p", "push"),
			h("u", "pull"),
			h("s", "stash"),
			sep,
			h("e", "editor"),
			h("a", "agent"),
		}
	case PaneFileTree:
		parts = []string{
			h("enter/l", "open"),
			h("h", "collapse"),
			h("bs", "up dir"),
			h("f", "find"),
		}
	}

	line := ""
	for _, p := range parts {
		if p == sep {
			line += sep
		} else {
			if line != "" && line[len(line)-1] != '|' {
				line += "  "
			}
			line += p
		}
	}
	return line
}

// NavHints returns the single-line navigation/global hints between panes.
func NavHints(t *theme.Theme, activeScreen screen.Type, activePane PaneID) string {
	k := lipgloss.NewStyle().Foreground(t.Secondary).Bold(true)
	d := lipgloss.NewStyle().Foreground(t.TextDim)

	h := func(binding, action string) string {
		return k.Render(binding) + " " + d.Render(action)
	}

	join := func(parts ...string) string {
		out := ""
		for i, p := range parts {
			if i > 0 {
				out += "  "
			}
			out += p
		}
		return out
	}

	switch activeScreen {
	case screen.WorktreeCreate:
		return join(h("tab", "field"), h("C-b", "branches"), h("enter", "create"), h("esc", "cancel"))
	case screen.BranchSelect:
		return join(h("j/k", "move"), h("g/G", "top/bottom"), h("enter", "select"), h("esc", "cancel"))
	case screen.Commit:
		return join(h("tab", "field"), h("</>", "type"), h("enter", "commit"), h("esc", "cancel"))
	case screen.Confirm:
		return join(h("y", "confirm"), h("n", "cancel"), h("esc", "cancel"))
	case screen.Help:
		return join(h("j/k", "scroll"), h("g", "top"), h("esc", "close"))
	case screen.AgentSelect:
		return join(h("j/k", "move"), h("enter", "launch"), h("esc", "cancel"))
	case screen.Sessions:
		return join(h("j/k", "move"), h("enter", "attach"), h("x", "kill"), h("X", "clean"), h("esc", "close"))
	case screen.RepoSelect:
		return join(h("j/k", "move"), h("enter", "select"), h("esc", "cancel"))
	case screen.WorktreeDetail:
		return join(h("e", "editor"), h("a", "agent"), h("b", "branch"), h("c", "commit"), h("p", "push"), h("esc", "back"))
	default:
		if activePane == PaneFileTree {
			return join(h("C-up", "worktrees"), h("/", "filter wt"), h("?", "help"), h("q", "quit"))
		}
		return join(h("enter", "switch"), h("l", "detail"), h("tab", "repo"), h("/", "filter"), h("S", "sessions"), h("C-down", "browse"), h("?", "help"), h("q", "quit"))
	}
}
