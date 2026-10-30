package views

import (
	"fmt"
	"strings"

	"github.com/Energma/blink-f/internal/app/components"
	"github.com/Energma/blink-f/internal/theme"
	"charm.land/lipgloss/v2"
)

// FileTree renders the directory tree browser pane. rows[i] is the index in
// nodes of the node drawn on line i, or -1, and hints says where each pane
// hint is drawn, as for Dashboard.
func FileTree(nodes []*components.TreeNode, cursor int, focused bool, filterMode bool, filterText string, matchIndices []int, paneHints [][]components.Hint, t *theme.Theme, width, height int) (string, []int, []components.HintSpan) {
	var b strings.Builder
	var rows []int

	// Filter bar
	if filterMode {
		filterStyle := lipgloss.NewStyle().Foreground(t.Secondary)
		b.WriteString(filterStyle.Render("f/ ") + filterText + "█")
		if len(matchIndices) > 0 {
			b.WriteString(lipgloss.NewStyle().Foreground(t.TextDim).Render(fmt.Sprintf("  %d matches", len(matchIndices))))
		} else if filterText != "" {
			b.WriteString(lipgloss.NewStyle().Foreground(t.Warning).Render("  no matches"))
		}
		b.WriteString("\n")
	}

	if len(nodes) == 0 {
		empty := lipgloss.NewStyle().
			Foreground(t.TextDim).
			Padding(1, 2).
			Render("Loading directory tree...")
		b.WriteString(empty)
		return b.String(), rows, nil
	}

	// Build match set for highlighting
	matchSet := make(map[int]bool, len(matchIndices))
	for _, idx := range matchIndices {
		matchSet[idx] = true
	}

	footer, hints := paneFooter(focused, paneHints, t, width)

	// A long name can wrap a node onto a second line, so measure each.
	lines := make([]string, len(nodes))
	node := func(i int) string {
		if lines[i] == "" {
			lines[i] = renderTreeNode(nodes[i], i == cursor, focused, matchSet[i], t, width)
		}
		return lines[i]
	}
	avail := height - lineCount(&b) - footerHeight(footer)
	start, end := scrollWindow(len(nodes), cursor, avail, func(i int) int {
		return lipgloss.Height(node(i))
	})

	// Scroll indicator top
	if start > 0 {
		b.WriteString(lipgloss.NewStyle().Foreground(t.TextDim).Render(fmt.Sprintf("  ... %d more above", start)))
		b.WriteString("\n")
	}

	rows = markRows(rows, lineCount(&b), -1)
	for i := start; i < end; i++ {
		b.WriteString(node(i))
		b.WriteString("\n")
		rows = markRows(rows, lineCount(&b), i)
	}

	// Scroll indicator bottom
	if end < len(nodes) {
		b.WriteString(lipgloss.NewStyle().Foreground(t.TextDim).Render(fmt.Sprintf("  ... %d more below", len(nodes)-end)))
		b.WriteString("\n")
	}

	hints = pinFooter(&b, footer, hints, t, width, height)
	return strings.TrimSuffix(b.String(), "\n"), rows, hints
}

func renderTreeNode(node *components.TreeNode, selected, paneFocused, isMatch bool, t *theme.Theme, width int) string {
	indent := strings.Repeat("  ", node.Depth)

	// Directory indicator
	indicator := "> "
	if node.Expanded {
		indicator = "v "
	}

	// Name styling
	nameStyle := lipgloss.NewStyle().Foreground(t.Text)
	if node.IsGitRepo {
		nameStyle = lipgloss.NewStyle().Foreground(t.Accent).Bold(true)
	}
	if isMatch {
		nameStyle = nameStyle.Underline(true)
	}

	line := indent + indicator + nameStyle.Render(node.Name+"/")

	// Git repo badge
	if node.IsGitRepo {
		badge := lipgloss.NewStyle().Foreground(t.Success).Render(" [git]")
		line += badge
	}

	// Selection highlight
	if selected && paneFocused {
		line = lipgloss.NewStyle().
			Background(t.Highlight).
			Foreground(t.Text).
			Width(width).
			Render(line)
	} else if selected {
		line = lipgloss.NewStyle().
			Foreground(t.Secondary).
			Width(width).
			Render(line)
	}

	return line
}
