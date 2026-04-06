package views

import (
	"fmt"
	"strings"

	"github.com/Energma/blink-f/internal/theme"
	"charm.land/lipgloss/v2"
)

// BranchSelect renders the branch selection overlay.
func BranchSelect(branches []string, cursor int, t *theme.Theme, width, height int) string {
	title := lipgloss.NewStyle().
		Bold(true).
		Foreground(t.Primary).
		Render("Select Branch")

	var b strings.Builder
	b.WriteString(title)
	b.WriteString("\n\n")

	if len(branches) == 0 {
		b.WriteString(lipgloss.NewStyle().
			Foreground(t.TextDim).
			Render("No branches found"))
		b.WriteString("\n")
	} else {
		maxVisible := height - 12
		if maxVisible < 5 {
			maxVisible = 5
		}

		start := 0
		if cursor >= maxVisible {
			start = cursor - maxVisible + 1
		}
		end := start + maxVisible
		if end > len(branches) {
			end = len(branches)
		}

		if start > 0 {
			b.WriteString(lipgloss.NewStyle().Foreground(t.TextDim).
				Render(fmt.Sprintf("  ... %d more above", start)))
			b.WriteString("\n")
		}

		for i := start; i < end; i++ {
			selected := i == cursor
			var line string
			if selected {
				line = lipgloss.NewStyle().
					Foreground(t.Primary).
					Bold(true).
					Render("> " + branches[i])
			} else {
				line = lipgloss.NewStyle().
					Foreground(t.Text).
					Render("  " + branches[i])
			}
			b.WriteString(line)
			b.WriteString("\n")
		}

		if end < len(branches) {
			b.WriteString(lipgloss.NewStyle().Foreground(t.TextDim).
				Render(fmt.Sprintf("  ... %d more below", len(branches)-end)))
			b.WriteString("\n")
		}
	}

	b.WriteString("\n")
	b.WriteString(lipgloss.NewStyle().Foreground(t.TextDim).Render(
		"enter: select  esc: cancel"))

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
