package views

import (
	"strings"

	"github.com/Energma/blink-f/internal/app/components"
	"github.com/Energma/blink-f/internal/theme"
	"charm.land/lipgloss/v2"
)

// Create renders the worktree creation form.
func Create(branchInput, baseBranch *components.TextInput, activeField int, t *theme.Theme, width int) string {
	title := lipgloss.NewStyle().
		Bold(true).
		Foreground(t.Primary).
		Render("New Worktree")

	label := lipgloss.NewStyle().Foreground(t.TextDim)
	activeLabel := lipgloss.NewStyle().Foreground(t.Secondary).Bold(true)
	input := lipgloss.NewStyle().Foreground(t.Text)
	cursor := lipgloss.NewStyle().Foreground(t.Primary)
	muted := lipgloss.NewStyle().Foreground(t.Muted)

	var b strings.Builder
	b.WriteString(title)
	b.WriteString("\n\n")

	if activeField == 0 {
		b.WriteString(activeLabel.Render("Branch name: "))
	} else {
		b.WriteString(label.Render("Branch name: "))
	}
	b.WriteString(branchInput.View(activeField == 0, input, cursor, muted, "(e.g. feature/login)"))
	b.WriteString("\n\n")

	if activeField == 1 {
		b.WriteString(activeLabel.Render("Base branch: "))
	} else {
		b.WriteString(label.Render("Base branch: "))
	}
	b.WriteString(baseBranch.View(activeField == 1, input, cursor, muted, "(default)"))
	b.WriteString("\n\n")

	b.WriteString(lipgloss.NewStyle().Foreground(t.TextDim).Render(
		"tab field  ctrl+b branches  enter create  esc cancel\n"+
			"←→ home/end move  ctrl+w del word  ctrl+u clear"))

	modalWidth := 60
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
