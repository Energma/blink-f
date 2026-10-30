package views

import (
	"fmt"
	"strings"

	"github.com/Energma/blink-f/internal/app/components"
	"github.com/Energma/blink-f/internal/models"
	"github.com/Energma/blink-f/internal/theme"
	"charm.land/lipgloss/v2"
)

// Dashboard renders the main worktree list view. rows[i] is the position in
// filteredIndices of the item drawn on line i, or -1, and hints says where
// each pane hint is drawn; both come from the same pass that lays the lines
// out, so a mouse click maps to what is on screen.
func Dashboard(worktrees []models.Worktree, filteredIndices []int, cursor int, filterText string, filterMode bool, focused bool, paneHints [][]components.Hint, t *theme.Theme, width, height int) (string, []int, []components.HintSpan) {
	var b strings.Builder
	var rows []int

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
		return b.String(), rows, nil
	}

	// No filter matches
	if len(filteredIndices) == 0 && filterText != "" {
		noMatch := lipgloss.NewStyle().
			Foreground(t.TextDim).
			Padding(1, 4).
			Render(fmt.Sprintf("No worktrees matching %q", filterText))
		b.WriteString(noMatch)
		return b.String(), rows, nil
	}

	footer, hints := paneFooter(focused, paneHints, t, width)

	// Items are two or three lines tall, so the window is sized in lines.
	items := make([]string, len(filteredIndices))
	item := func(i int) string {
		if items[i] == "" {
			items[i] = components.WorktreeItem(worktrees[filteredIndices[i]], i == cursor, t, width-4)
		}
		return items[i]
	}
	avail := height - lineCount(&b) - footerHeight(footer)
	start, end := scrollWindow(len(filteredIndices), cursor, avail, func(i int) int {
		return lipgloss.Height(item(i))
	})

	// Scroll indicator top
	if start > 0 {
		b.WriteString(lipgloss.NewStyle().Foreground(t.TextDim).Render(fmt.Sprintf("  ... %d more above", start)))
		b.WriteString("\n")
	}

	rows = markRows(rows, lineCount(&b), -1)
	for i := start; i < end; i++ {
		b.WriteString(item(i))
		b.WriteString("\n")
		rows = markRows(rows, lineCount(&b), i)
	}

	// Scroll indicator bottom
	if end < len(filteredIndices) {
		b.WriteString(lipgloss.NewStyle().Foreground(t.TextDim).Render(fmt.Sprintf("  ... %d more below", len(filteredIndices)-end)))
		b.WriteString("\n")
	}

	hints = pinFooter(&b, footer, hints, t, width, height)
	return strings.TrimSuffix(b.String(), "\n"), rows, hints
}

// scrollWindow picks the items [start, end) to draw in avail lines, where
// item i is height(i) lines tall and a "more above/below" line is drawn when
// items are cut off at either end. The cursor's item is always drawn whole,
// with as many items above it as fit, so the list scrolls once the cursor
// would leave the bottom.
func scrollWindow(n, cursor, avail int, height func(int) int) (start, end int) {
	if n == 0 {
		return 0, 0
	}
	cursor = max(0, min(cursor, n-1))
	used := height(cursor)
	if cursor < n-1 {
		used++ // more below
	}
	start = cursor
	for start > 0 {
		above := 0
		if start-1 > 0 {
			above = 1
		}
		if above+used+height(start-1) > avail {
			break
		}
		start--
		used += height(start)
	}

	used = 0
	if start > 0 {
		used = 1
	}
	for end = start; end < n; end++ {
		below := 0
		if end+1 < n {
			below = 1
		}
		if end > cursor && used+height(end)+below > avail {
			break
		}
		used += height(end)
	}
	return start, end
}

// paneFooter renders a focused pane's action hints, wrapped to the pane
// width so none is cut off; "" when the pane shows none.
func paneFooter(focused bool, paneHints [][]components.Hint, t *theme.Theme, width int) (string, []components.HintSpan) {
	if !focused || len(paneHints) == 0 {
		return "", nil
	}
	return components.RenderHints(t, width, paneHints...)
}

// footerHeight is the lines a footer takes, its rule included.
func footerHeight(footer string) int {
	if footer == "" {
		return 0
	}
	return 1 + lipgloss.Height(footer)
}

// pinFooter pads b so the footer ends on the pane's last line, then draws a
// rule and the footer, and moves the hint spans onto the lines they landed
// on. b must end on a finished line.
func pinFooter(b *strings.Builder, footer string, hints []components.HintSpan, t *theme.Theme, width, height int) []components.HintSpan {
	if footer == "" {
		return nil
	}
	for lineCount(b) < height-footerHeight(footer) {
		b.WriteString("\n")
	}
	b.WriteString(lipgloss.NewStyle().Foreground(t.Border).
		Render(strings.Repeat("─", min(width-2, 80))))
	b.WriteString("\n")
	top := lineCount(b)
	b.WriteString(footer)
	for i := range hints {
		hints[i].Line += top
	}
	return hints
}

// markRows extends rows to cover n lines, giving each new line item.
func markRows(rows []int, n, item int) []int {
	for len(rows) < n {
		rows = append(rows, item)
	}
	return rows
}

// lineCount is the number of finished lines in b.
func lineCount(b *strings.Builder) int {
	return strings.Count(b.String(), "\n")
}

