package components

import (
	"fmt"
	"path/filepath"
	"strings"

	"github.com/Energma/blink-f/internal/models"
	"github.com/Energma/blink-f/internal/theme"
	"charm.land/lipgloss/v2"
)

// WorktreeItem renders a single worktree as a two-line list item.
// Line 1: cursor + branch + status indicators
// Line 2: path + session name + last commit
func WorktreeItem(wt models.Worktree, selected bool, t *theme.Theme, width int) string {
	branch := lipgloss.NewStyle().Foreground(t.Secondary).Bold(true)
	muted := lipgloss.NewStyle().Foreground(t.TextDim)
	success := lipgloss.NewStyle().Foreground(t.Success)
	warn := lipgloss.NewStyle().Foreground(t.Warning)
	errStyle := lipgloss.NewStyle().Foreground(t.Error)
	accent := lipgloss.NewStyle().Foreground(t.Accent)

	var b strings.Builder

	// --- Line 1: branch + status ---
	cursor := "  "
	if selected {
		cursor = lipgloss.NewStyle().Foreground(t.Primary).Bold(true).Render("> ")
	}

	name := wt.DisplayName()
	if wt.IsMain {
		name += " *"
	}
	branchStr := branch.Render(name)

	// Status pill
	var statusStr string
	if wt.Status.Conflicts > 0 {
		statusStr = errStyle.Bold(true).Render(fmt.Sprintf(" !! %d conflicts", wt.Status.Conflicts))
	} else if wt.Status.Clean {
		statusStr = success.Render(" clean")
	} else {
		statusStr = warn.Render(" " + wt.Status.StatusLine())
	}

	// Session/agent indicators as compact badges
	badges := ""
	if wt.Status.TmuxSession != "" {
		badges += success.Render(" [S]")
	}
	if wt.Status.AgentRunning {
		badges += accent.Render(" [A]")
	}

	b.WriteString(cursor + branchStr + statusStr + badges)
	b.WriteString("\n")

	// --- Line 2: path + session name + last commit ---
	indent := "    "
	var details []string

	// Compact path (relative to repo)
	shortPath := wt.Path
	if wt.RepoDir != "" {
		rel, err := filepath.Rel(filepath.Dir(wt.RepoDir), wt.Path)
		if err == nil {
			shortPath = rel
		}
	}
	details = append(details, muted.Render(shortPath))

	// Session name if active
	if wt.Status.TmuxSession != "" {
		details = append(details, success.Render("tmux:"+wt.Status.TmuxSession))
	}

	b.WriteString(indent + strings.Join(details, muted.Render("  |  ")))

	// Last commit on a third line if selected and available
	if selected && wt.Last != nil {
		subj := wt.Last.Subject
		if len(subj) > 50 {
			subj = subj[:47] + "..."
		}
		hash := wt.Last.Hash
		if len(hash) > 7 {
			hash = hash[:7]
		}
		b.WriteString("\n" + indent + muted.Render(hash+" "+subj))
	}

	if selected {
		return lipgloss.NewStyle().
			Background(t.Highlight).
			Foreground(t.Text).
			Width(width).
			Render(b.String())
	}
	return b.String()
}
