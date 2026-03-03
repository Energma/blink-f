package views

import (
	"fmt"
	"strings"

	"github.com/energma-dev/blink/internal/models"
	"github.com/energma-dev/blink/internal/theme"
	"charm.land/lipgloss/v2"
)

// SessionInfo holds display data for a tmux session.
type SessionInfo struct {
	Name      string
	Worktree  *models.Worktree // nil if orphaned
	IsAgent   bool
	Attached  bool
}

// Sessions renders the tmux session management panel.
func Sessions(sessions []SessionInfo, cursor int, t *theme.Theme, width, height int) string {
	title := lipgloss.NewStyle().
		Bold(true).
		Foreground(t.Primary).
		Render("Sessions")

	subtitle := lipgloss.NewStyle().
		Foreground(t.TextDim).
		Render(fmt.Sprintf("  %d active", len(sessions)))

	var b strings.Builder
	b.WriteString(title + subtitle)
	b.WriteString("\n")
	b.WriteString(lipgloss.NewStyle().Foreground(t.Border).Render(strings.Repeat("─", min(width-6, 60))))
	b.WriteString("\n\n")

	if len(sessions) == 0 {
		b.WriteString(lipgloss.NewStyle().Foreground(t.TextDim).Render(
			"  No active tmux sessions.\n  Press enter on a worktree to create one."))
		b.WriteString("\n\n")
		b.WriteString(lipgloss.NewStyle().Foreground(t.TextDim).Render("esc: close"))

		return lipgloss.NewStyle().
			Border(lipgloss.RoundedBorder()).
			BorderForeground(t.Border).
			Padding(1, 2).
			Width(clamp(width-4, 30, 65)).
			Render(b.String())
	}

	key := lipgloss.NewStyle().Foreground(t.Secondary).Bold(true)
	name := lipgloss.NewStyle().Foreground(t.Text)
	muted := lipgloss.NewStyle().Foreground(t.TextDim)
	success := lipgloss.NewStyle().Foreground(t.Success)
	accent := lipgloss.NewStyle().Foreground(t.Accent)
	errStyle := lipgloss.NewStyle().Foreground(t.Error)

	maxVisible := height - 12
	if maxVisible < 3 {
		maxVisible = 3
	}
	start := 0
	if cursor >= maxVisible {
		start = cursor - maxVisible + 1
	}
	end := start + maxVisible
	if end > len(sessions) {
		end = len(sessions)
	}

	for i := start; i < end; i++ {
		s := sessions[i]
		cur := "  "
		if i == cursor {
			cur = lipgloss.NewStyle().Foreground(t.Primary).Render("> ")
		}

		// Session name
		nameStr := s.Name
		if i == cursor {
			nameStr = name.Bold(true).Render(nameStr)
		} else {
			nameStr = muted.Render(nameStr)
		}

		// Indicators
		indicators := ""
		if s.IsAgent {
			indicators += accent.Render(" [agent]")
		}
		if s.Attached {
			indicators += success.Render(" (attached)")
		}

		b.WriteString(cur + nameStr + indicators)
		b.WriteString("\n")

		// Second line: worktree info
		if s.Worktree != nil {
			branch := key.Render(s.Worktree.DisplayName())
			var statusStr string
			if s.Worktree.Status.Clean {
				statusStr = success.Render("clean")
			} else {
				statusStr = errStyle.Render(s.Worktree.Status.StatusLine())
			}
			b.WriteString("    " + branch + "  " + statusStr)
		} else {
			b.WriteString("    " + muted.Render("(no linked worktree)"))
		}
		b.WriteString("\n")
	}

	if end < len(sessions) {
		b.WriteString(muted.Render(fmt.Sprintf("  ... %d more", len(sessions)-end)))
		b.WriteString("\n")
	}

	b.WriteString("\n")
	b.WriteString(lipgloss.NewStyle().Foreground(t.TextDim).Render(
		key.Render("enter") + " attach  " +
			key.Render("x") + " kill  " +
			key.Render("X") + " kill all orphaned  " +
			key.Render("esc") + " close"))

	return lipgloss.NewStyle().
		Border(lipgloss.RoundedBorder()).
		BorderForeground(t.Border).
		Padding(1, 2).
		Width(clamp(width-4, 30, 65)).
		Render(b.String())
}

func clamp(v, lo, hi int) int {
	if v < lo {
		return lo
	}
	if v > hi {
		return hi
	}
	return v
}
