package views

import (
	"fmt"
	"strings"

	"github.com/Energma/blink-f/internal/app/components"
	"github.com/Energma/blink-f/internal/theme"
	"charm.land/lipgloss/v2"
)

// BranchSelect renders the branch selection overlay. branches is the
// already-filtered list; total is the unfiltered count shown as "n/total".
// current marks the branch checked out in the target worktree.
func BranchSelect(branches []string, total int, filter *components.TextInput, current string, cursor int, t *theme.Theme, width, height int) string {
	title := lipgloss.NewStyle().
		Bold(true).
		Foreground(t.Primary).
		Render("Select Branch")
	count := lipgloss.NewStyle().
		Foreground(t.TextDim).
		Render(fmt.Sprintf("  %d/%d", len(branches), total))

	var b strings.Builder
	b.WriteString(title + count)
	b.WriteString("\n\n")

	b.WriteString(lipgloss.NewStyle().Foreground(t.Secondary).Bold(true).Render("> "))
	b.WriteString(filter.View(true,
		lipgloss.NewStyle().Foreground(t.Text),
		lipgloss.NewStyle().Foreground(t.Primary),
		lipgloss.NewStyle().Foreground(t.Muted), ""))
	b.WriteString("\n\n")

	if len(branches) == 0 {
		msg := "No branches found"
		if filter.Value() != "" {
			msg = "No branches match"
		}
		b.WriteString(lipgloss.NewStyle().
			Foreground(t.TextDim).
			Render(msg))
		b.WriteString("\n")
	} else {
		// The overlay sits below the fixed-height worktree pane (~21 rows with
		// hints and status bar) and needs ~13 rows of its own chrome; sizing
		// the list to what's left keeps the filter and footer on screen.
		maxVisible := height - 34
		if maxVisible < 3 {
			maxVisible = 3
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
			name := branches[i]
			suffix := ""
			if name == current {
				suffix = lipgloss.NewStyle().Foreground(t.Success).Render("  (current)")
			}
			var line string
			if i == cursor {
				line = lipgloss.NewStyle().
					Foreground(t.Primary).
					Bold(true).
					Render("▸ " + name)
			} else {
				line = lipgloss.NewStyle().
					Foreground(t.Text).
					Render("  " + name)
			}
			b.WriteString(line + suffix)
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
		"type to filter  ↑↓ move  enter select  esc back"))

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
