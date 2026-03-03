package views

import (
	"strings"

	"github.com/Energma/blink-f/internal/theme"
	"charm.land/lipgloss/v2"
)

// Confirm renders a yes/no confirmation dialog.
func Confirm(title, message string, t *theme.Theme, width int) string {
	titleStyle := lipgloss.NewStyle().
		Bold(true).
		Foreground(t.Warning)

	var b strings.Builder
	b.WriteString(titleStyle.Render(title))
	b.WriteString("\n\n")
	b.WriteString(lipgloss.NewStyle().Foreground(t.Text).Render(message))
	b.WriteString("\n\n")
	b.WriteString(lipgloss.NewStyle().Foreground(t.TextDim).Render(
		"y: confirm  n/esc: cancel"))

	modalWidth := 50
	if modalWidth > width-4 {
		modalWidth = width - 4
	}

	return lipgloss.NewStyle().
		Border(lipgloss.RoundedBorder()).
		BorderForeground(t.Warning).
		Padding(1, 2).
		Width(modalWidth).
		Render(b.String())
}
