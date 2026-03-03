package views

import (
	"strings"

	"github.com/Energma/blink-f/internal/theme"
	"charm.land/lipgloss/v2"
)

// AgentSelect renders the agent provider picker.
func AgentSelect(providers []string, available []string, selected int, t *theme.Theme, width int) string {
	title := lipgloss.NewStyle().
		Bold(true).
		Foreground(t.Primary).
		Render("Launch Agent")

	avail := make(map[string]bool)
	for _, a := range available {
		avail[a] = true
	}

	var b strings.Builder
	b.WriteString(title)
	b.WriteString("\n\n")

	for i, p := range providers {
		cursor := "  "
		if i == selected {
			cursor = lipgloss.NewStyle().Foreground(t.Primary).Render("> ")
		}

		name := p
		indicator := ""
		if avail[p] {
			indicator = lipgloss.NewStyle().Foreground(t.Success).Render(" (installed)")
		} else {
			indicator = lipgloss.NewStyle().Foreground(t.Error).Render(" (not found)")
		}

		if i == selected {
			name = lipgloss.NewStyle().Bold(true).Foreground(t.Text).Render(name)
		} else {
			name = lipgloss.NewStyle().Foreground(t.TextDim).Render(name)
		}

		b.WriteString(cursor + name + indicator)
		b.WriteString("\n")
	}

	b.WriteString("\n")
	b.WriteString(lipgloss.NewStyle().Foreground(t.TextDim).Render(
		"enter: launch  esc: cancel"))

	modalWidth := 45
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
