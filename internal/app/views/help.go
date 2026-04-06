package views

import (
	"fmt"
	"strings"

	"github.com/Energma/blink-f/internal/theme"
	"charm.land/lipgloss/v2"
)

type keyBinding struct {
	key  string
	desc string
}

// Help renders the keybinding reference overlay with scroll support.
func Help(t *theme.Theme, width, height, scroll int) string {
	title := lipgloss.NewStyle().
		Bold(true).
		Foreground(t.Primary).
		Render("Keybindings")

	sections := []struct {
		name     string
		bindings []keyBinding
	}{
		{"Navigation", []keyBinding{
			{"j/k, up/down", "Move cursor"},
			{"g / G", "Jump to top / bottom"},
			{"enter", "Switch to worktree session"},
			{"l / right", "Open detail view"},
			{"tab", "Switch repository"},
			{"/", "Filter worktrees"},
			{"r", "Refresh all"},
		}},
		{"Worktrees", []keyBinding{
			{"n", "Create new worktree"},
			{"b", "Switch branch / checkout"},
			{"d", "Delete worktree"},
			{"D", "Clean merged worktrees"},
		}},
		{"Git", []keyBinding{
			{"c", "Commit changes"},
			{"p", "Push to remote"},
			{"u", "Pull from remote"},
			{"s", "Stash changes"},
		}},
		{"Sessions & Tools", []keyBinding{
			{"S", "Manage tmux sessions"},
			{"a", "Launch AI agent"},
			{"Q", "Kill agent session"},
			{"ctrl+q q", "Detach from session"},
			{"ctrl+q e", "Open editor (tmux)"},
			{"e", "Open editor in worktree"},
		}},
		{"Create Worktree", []keyBinding{
			{"tab", "Switch field"},
			{"ctrl+b", "Browse branches"},
			{"enter", "Create"},
			{"esc", "Cancel"},
		}},
		{"General", []keyBinding{
			{"?", "Toggle help"},
			{"esc", "Close modal / cancel"},
			{"q, ctrl+c", "Quit"},
		}},
	}

	key := lipgloss.NewStyle().
		Foreground(t.Secondary).
		Bold(true).
		Width(16).
		Align(lipgloss.Right)

	desc := lipgloss.NewStyle().
		Foreground(t.Text).
		PaddingLeft(2)

	section := lipgloss.NewStyle().
		Foreground(t.Accent).
		Bold(true)

	// Build all lines first
	var lines []string
	for i, sec := range sections {
		if i > 0 {
			lines = append(lines, "")
		}
		lines = append(lines, section.Render(sec.name))
		for _, kb := range sec.bindings {
			lines = append(lines, key.Render(kb.key)+desc.Render(kb.desc))
		}
	}

	// Calculate visible window
	maxVisible := height - 10
	if maxVisible < 5 {
		maxVisible = 5
	}

	totalLines := len(lines)
	if scroll > totalLines-maxVisible {
		scroll = totalLines - maxVisible
	}
	if scroll < 0 {
		scroll = 0
	}

	end := scroll + maxVisible
	if end > totalLines {
		end = totalLines
	}

	var b strings.Builder
	b.WriteString(title)
	b.WriteString("\n\n")

	if scroll > 0 {
		b.WriteString(lipgloss.NewStyle().Foreground(t.TextDim).
			Render(fmt.Sprintf("  ... %d more above", scroll)))
		b.WriteString("\n")
	}

	for _, line := range lines[scroll:end] {
		b.WriteString(line)
		b.WriteString("\n")
	}

	if end < totalLines {
		b.WriteString(lipgloss.NewStyle().Foreground(t.TextDim).
			Render(fmt.Sprintf("  ... %d more below", totalLines-end)))
	}

	b.WriteString("\n")
	b.WriteString(lipgloss.NewStyle().Foreground(t.TextDim).Render(
		"j/k: scroll  g: top  esc: close"))

	modalWidth := 56
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
