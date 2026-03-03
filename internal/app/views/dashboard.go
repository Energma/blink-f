package views

import (
	"fmt"
	"strings"

	"github.com/energma-dev/blink/internal/app/components"
	"github.com/energma-dev/blink/internal/models"
	"github.com/energma-dev/blink/internal/theme"
	"charm.land/lipgloss/v2"
)

// Dashboard renders the main worktree list view.
func Dashboard(worktrees []models.Worktree, filteredIndices []int, cursor int, filterText string, filterMode bool, t *theme.Theme, width, height int) string {
	var b strings.Builder

	// Header
	header := lipgloss.NewStyle().
		Bold(true).
		Foreground(t.Primary).
		Render("BLINK")
	header += lipgloss.NewStyle().
		Foreground(t.TextDim).
		Render("  git worktree manager")
	b.WriteString(header)
	b.WriteString("\n")

	// Separator
	sep := lipgloss.NewStyle().
		Foreground(t.Border).
		Render(strings.Repeat("─", min(width-2, 80)))
	b.WriteString(sep)
	b.WriteString("\n")

	// Filter bar
	if filterMode {
		filterStyle := lipgloss.NewStyle().Foreground(t.Secondary)
		b.WriteString(filterStyle.Render("/ ") + filterText + "█")
		b.WriteString("\n")
	}

	// Empty state
	if len(worktrees) == 0 {
		empty := lipgloss.NewStyle().
			Foreground(t.TextDim).
			Padding(2, 4).
			Render("No worktrees found.\nPress 'n' to create one, or configure repos in ~/.config/blink/config.yaml")
		b.WriteString(empty)
		return b.String()
	}

	// No filter matches
	if len(filteredIndices) == 0 && filterText != "" {
		noMatch := lipgloss.NewStyle().
			Foreground(t.TextDim).
			Padding(1, 4).
			Render(fmt.Sprintf("No worktrees matching %q", filterText))
		b.WriteString(noMatch)
		return b.String()
	}

	// Worktree list
	maxVisible := height - 8 // Reserve space for header, status bar, hints
	if maxVisible < 3 {
		maxVisible = 3
	}

	// Calculate scroll window
	start := 0
	if cursor >= maxVisible {
		start = cursor - maxVisible + 1
	}
	end := start + maxVisible
	if end > len(filteredIndices) {
		end = len(filteredIndices)
	}

	// Scroll indicator top
	if start > 0 {
		b.WriteString(lipgloss.NewStyle().Foreground(t.TextDim).Render(fmt.Sprintf("  ... %d more above", start)))
		b.WriteString("\n")
	}

	for vi, idx := range filteredIndices[start:end] {
		selected := (start + vi) == cursor
		b.WriteString(components.WorktreeItem(worktrees[idx], selected, t, width-4))
		b.WriteString("\n")
	}

	// Scroll indicator bottom
	if end < len(filteredIndices) {
		b.WriteString(lipgloss.NewStyle().Foreground(t.TextDim).Render(fmt.Sprintf("  ... %d more below", len(filteredIndices)-end)))
	}

	return b.String()
}

func min(a, b int) int {
	if a < b {
		return a
	}
	return b
}
