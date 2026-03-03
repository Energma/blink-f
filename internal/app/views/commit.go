package views

import (
	"fmt"
	"strings"

	"github.com/energma-dev/blink/internal/git"
	"github.com/energma-dev/blink/internal/theme"
	"charm.land/lipgloss/v2"
)

// Commit renders the commit message editor with staged file preview.
func Commit(commitType int, scopeInput, msgInput string, activeField int, conventional bool, changedFiles []string, t *theme.Theme, width int) string {
	title := lipgloss.NewStyle().
		Bold(true).
		Foreground(t.Primary).
		Render("Commit Changes")

	label := lipgloss.NewStyle().Foreground(t.TextDim)
	active := lipgloss.NewStyle().Foreground(t.Secondary).Bold(true)
	inactive := lipgloss.NewStyle().Foreground(t.Text)
	cursor := lipgloss.NewStyle().Foreground(t.Primary).Render("█")

	var b strings.Builder
	b.WriteString(title)
	b.WriteString("\n\n")

	// Changed files preview
	if len(changedFiles) > 0 {
		b.WriteString(label.Render(fmt.Sprintf("Files to commit (%d):", len(changedFiles))))
		b.WriteString("\n")
		maxShow := 8
		for i, f := range changedFiles {
			if i >= maxShow {
				b.WriteString(lipgloss.NewStyle().Foreground(t.Muted).Render(
					fmt.Sprintf("  ... and %d more", len(changedFiles)-maxShow)))
				b.WriteString("\n")
				break
			}
			b.WriteString("  " + lipgloss.NewStyle().Foreground(t.Accent).Render(f))
			b.WriteString("\n")
		}
		b.WriteString("\n")
	} else {
		b.WriteString(lipgloss.NewStyle().Foreground(t.Warning).Render("No changes detected"))
		b.WriteString("\n\n")
	}

	if conventional {
		// Type selector
		b.WriteString(label.Render("Type: "))
		types := git.ConventionalTypes()
		for i, ct := range types {
			if i == commitType {
				b.WriteString(active.Render("[" + ct + "]"))
			} else {
				b.WriteString(inactive.Render(" " + ct + " "))
			}
		}
		b.WriteString("\n")
		if activeField == 0 {
			b.WriteString(lipgloss.NewStyle().Foreground(t.TextDim).Render("  left/right to change type"))
		}
		b.WriteString("\n\n")

		// Scope
		b.WriteString(label.Render("Scope: "))
		if activeField == 1 {
			b.WriteString(inactive.Render(scopeInput) + cursor)
		} else {
			if scopeInput != "" {
				b.WriteString(inactive.Render(scopeInput))
			} else {
				b.WriteString(lipgloss.NewStyle().Foreground(t.Muted).Render("(optional)"))
			}
		}
		b.WriteString("\n\n")
	}

	// Message
	b.WriteString(label.Render("Message: "))
	msgField := 2
	if !conventional {
		msgField = 0
	}
	if activeField == msgField {
		b.WriteString(inactive.Render(msgInput) + cursor)
	} else {
		b.WriteString(inactive.Render(msgInput))
	}
	b.WriteString("\n\n")

	// Preview
	if msgInput != "" {
		preview := msgInput
		if conventional {
			types := git.ConventionalTypes()
			if commitType >= 0 && commitType < len(types) {
				preview = git.FormatConventionalCommit(types[commitType], scopeInput, msgInput)
			}
		}
		b.WriteString(label.Render("Preview: "))
		b.WriteString(lipgloss.NewStyle().Foreground(t.Accent).Bold(true).Render(preview))
		b.WriteString("\n\n")
	}

	// Navigation hints
	nav := "tab: next field  enter: commit  esc: cancel"
	b.WriteString(lipgloss.NewStyle().Foreground(t.TextDim).Render(nav))

	modalWidth := 70
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
