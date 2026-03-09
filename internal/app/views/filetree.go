package views

import (
	"fmt"
	"strings"

	"github.com/Energma/blink-f/internal/app/components"
	"github.com/Energma/blink-f/internal/theme"
	"charm.land/lipgloss/v2"
)

// FileTree renders the directory tree browser pane.
func FileTree(nodes []*components.TreeNode, cursor int, focused bool, filterMode bool, filterText string, matchIndices []int, t *theme.Theme, width, height int) string {
	var b strings.Builder

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
		height-- // filter bar takes a line
	}

	if len(nodes) == 0 {
		empty := lipgloss.NewStyle().
			Foreground(t.TextDim).
			Padding(1, 2).
			Render("Loading directory tree...")
		b.WriteString(empty)
		return b.String()
	}

	// Build match set for highlighting
	matchSet := make(map[int]bool, len(matchIndices))
	for _, idx := range matchIndices {
		matchSet[idx] = true
	}

	maxVisible := height
	if maxVisible < 3 {
		maxVisible = 3
	}

	// Calculate scroll window
	start := 0
	if cursor >= maxVisible {
		start = cursor - maxVisible + 1
	}
	end := start + maxVisible
	if end > len(nodes) {
		end = len(nodes)
	}

	// Scroll indicator top
	if start > 0 {
		b.WriteString(lipgloss.NewStyle().Foreground(t.TextDim).Render(fmt.Sprintf("  ... %d more above", start)))
		b.WriteString("\n")
	}

	for vi, node := range nodes[start:end] {
		globalIdx := start + vi
		selected := globalIdx == cursor
		isMatch := matchSet[globalIdx]
		b.WriteString(renderTreeNode(node, selected, focused, isMatch, t, width))
		if start+vi < end-1 {
			b.WriteString("\n")
		}
	}

	// Scroll indicator bottom
	if end < len(nodes) {
		b.WriteString("\n")
		b.WriteString(lipgloss.NewStyle().Foreground(t.TextDim).Render(fmt.Sprintf("  ... %d more below", len(nodes)-end)))
	}

	return b.String()
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
