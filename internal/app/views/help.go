package views

import (
	"strings"

	"github.com/Energma/blink-f/internal/theme"
	"charm.land/lipgloss/v2"
)

type keyBinding struct {
	key  string
	desc string
}

// Help renders the keybinding reference overlay.
func Help(t *theme.Theme, width, height int) string {
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
			{"b", "Select base branch"},
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

	var b strings.Builder
	b.WriteString(title)
	b.WriteString("\n\n")

	for i, sec := range sections {
		if i > 0 {
			b.WriteString("\n")
		}
		b.WriteString(section.Render(sec.name))
		b.WriteString("\n")
		for _, kb := range sec.bindings {
			b.WriteString(key.Render(kb.key))
			b.WriteString(desc.Render(kb.desc))
			b.WriteString("\n")
		}
	}

	modalWidth := 52
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
