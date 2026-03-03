package views

import (
	"strings"

	"github.com/energma-dev/blink/internal/theme"
	"charm.land/lipgloss/v2"
)

// Create renders the worktree creation form.
func Create(branchInput, baseBranch string, cursorPos int, t *theme.Theme, width int) string {
	title := lipgloss.NewStyle().
		Bold(true).
		Foreground(t.Primary).
		Render("New Worktree")

	label := lipgloss.NewStyle().Foreground(t.TextDim)
	input := lipgloss.NewStyle().Foreground(t.Text)

	var b strings.Builder
	b.WriteString(title)
	b.WriteString("\n\n")

	b.WriteString(label.Render("Branch name: "))
	b.WriteString(input.Render(branchInput))
	b.WriteString(lipgloss.NewStyle().Foreground(t.Primary).Render("█"))
	b.WriteString("\n\n")

	b.WriteString(label.Render("Base branch: "))
	if baseBranch != "" {
		b.WriteString(input.Render(baseBranch))
	} else {
		b.WriteString(lipgloss.NewStyle().Foreground(t.Muted).Render("(default)"))
	}
	b.WriteString("\n\n")

	b.WriteString(lipgloss.NewStyle().Foreground(t.TextDim).Render(
		"enter: create  esc: cancel"))

	modalWidth := 50
	if modalWidth > width-4 {
		modalWidth = width - 4
	}

	return lipgloss.NewStyle().
		Border(lipgloss.RoundedBorder()).
		BorderForeground(t.Border).
		Padding(1, 2).
		Width(modalWidth).
		Render(b.String())
}
