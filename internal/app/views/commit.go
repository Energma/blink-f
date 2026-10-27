package views

import (
	"fmt"
	"strings"

	"github.com/Energma/blink-f/internal/app/components"
	"github.com/Energma/blink-f/internal/git"
	"github.com/Energma/blink-f/internal/theme"
	"charm.land/lipgloss/v2"
)

// Commit renders the commit message editor with staged file preview.
func Commit(commitType int, scopeInput, msgInput *components.TextInput, activeField int, conventional bool, changedFiles []string, t *theme.Theme, width int) string {
	title := lipgloss.NewStyle().
		Bold(true).
		Foreground(t.Primary).
		Render("Commit Changes")

	label := lipgloss.NewStyle().Foreground(t.TextDim)
	active := lipgloss.NewStyle().Foreground(t.Secondary).Bold(true)
	inactive := lipgloss.NewStyle().Foreground(t.Text)
	cursor := lipgloss.NewStyle().Foreground(t.Primary)
	muted := lipgloss.NewStyle().Foreground(t.Muted)

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
		b.WriteString(scopeInput.View(activeField == 1, inactive, cursor, muted, "(optional)"))
		b.WriteString("\n\n")
	}

	// Message
	b.WriteString(label.Render("Message: "))
	msgField := 2
	if !conventional {
		msgField = 0
	}
	b.WriteString(msgInput.View(activeField == msgField, inactive, cursor, muted, ""))
	b.WriteString("\n\n")

	// Preview
	if msgInput.Value() != "" {
		preview := msgInput.Value()
		if conventional {
			types := git.ConventionalTypes()
			if commitType >= 0 && commitType < len(types) {
				preview = git.FormatConventionalCommit(types[commitType], scopeInput.Value(), msgInput.Value())
			}
		}
		b.WriteString(label.Render("Preview: "))
		b.WriteString(lipgloss.NewStyle().Foreground(t.Accent).Bold(true).Render(preview))
		b.WriteString("\n\n")
	}

	// Navigation hints
	nav := "tab/shift+tab field  enter commit  esc cancel\n" +
		"←→ home/end move  ctrl+w del word  ctrl+u clear"
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
