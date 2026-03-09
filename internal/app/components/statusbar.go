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

// KeyHints renders context-sensitive keyboard hints.
func KeyHints(t *theme.Theme, activeScreen screen.Type, activePane PaneID) string {
	dim := lipgloss.NewStyle().Foreground(t.TextDim)
	key := lipgloss.NewStyle().Foreground(t.Secondary).Bold(true)

	switch activeScreen {
	case screen.WorktreeCreate:
		return dim.Render(key.Render("enter") + " create  " + key.Render("esc") + " cancel")
	case screen.Commit:
		return dim.Render(key.Render("tab") + " next field  " + key.Render("enter") + " commit  " + key.Render("esc") + " cancel")
	case screen.Confirm:
		return dim.Render(key.Render("y") + " confirm  " + key.Render("n") + "/" + key.Render("esc") + " cancel")
	case screen.Help:
		return dim.Render(key.Render("esc") + "/" + key.Render("?") + " close")
	case screen.AgentSelect:
		return dim.Render(key.Render("enter") + " launch  " + key.Render("esc") + " cancel")
	case screen.Sessions:
		return dim.Render(key.Render("enter") + " attach  " + key.Render("x") + " kill  " + key.Render("X") + " clean orphans  " + key.Render("esc") + " close")
	case screen.RepoSelect:
		return dim.Render(key.Render("enter") + " select  " + key.Render("esc") + " cancel")
	case screen.WorktreeDetail:
		return dim.Render(key.Render("e") + " editor  " + key.Render("a") + " agent  " + key.Render("c") + " commit  " + key.Render("esc") + " back")
	default:
		if activePane == PaneFileTree {
			return dim.Render(
				key.Render("enter") + " select  " +
					key.Render("l") + " expand  " +
					key.Render("h") + " collapse  " +
					key.Render("bs") + " up dir  " +
					key.Render("f") + " find  " +
					key.Render("ctrl+up") + " worktrees  " +
					key.Render("?") + " help",
			)
		}
		return dim.Render(
			key.Render("n") + " new  " +
				key.Render("enter") + " switch  " +
				key.Render("a") + " agent  " +
				key.Render("e") + " editor  " +
				key.Render("c") + " commit  " +
				key.Render("p") + " push  " +
				key.Render("ctrl+down") + " browse  " +
				key.Render("?") + " help",
		)
	}
}
