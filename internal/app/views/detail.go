package views

import (
	"fmt"
	"strings"

	"github.com/Energma/blink-f/internal/models"
	"github.com/Energma/blink-f/internal/theme"
	"charm.land/lipgloss/v2"
)

// Detail renders worktree detail panel.
func Detail(wt models.Worktree, files []string, diffStat string, t *theme.Theme, width int) string {
	title := lipgloss.NewStyle().
		Bold(true).
		Foreground(t.Primary).
		Render("Worktree Detail")

	label := lipgloss.NewStyle().Foreground(t.TextDim)
	value := lipgloss.NewStyle().Foreground(t.Text)

	var b strings.Builder
	b.WriteString(title)
	b.WriteString("\n\n")

	b.WriteString(label.Render("Branch:  "))
	b.WriteString(lipgloss.NewStyle().Foreground(t.Secondary).Bold(true).Render(wt.DisplayName()))
	b.WriteString("\n")

	b.WriteString(label.Render("Path:    "))
	b.WriteString(value.Render(wt.Path))
	b.WriteString("\n")

	b.WriteString(label.Render("Status:  "))
	if wt.Status.Clean {
		b.WriteString(lipgloss.NewStyle().Foreground(t.Success).Render("clean"))
	} else {
		b.WriteString(lipgloss.NewStyle().Foreground(t.Warning).Render(wt.Status.StatusLine()))
	}
	b.WriteString("\n")

	if wt.Last != nil {
		b.WriteString(label.Render("Commit:  "))
		hash := wt.Last.Hash
		if len(hash) > 8 {
			hash = hash[:8]
		}
		b.WriteString(lipgloss.NewStyle().Foreground(t.Muted).Render(hash) + " ")
		b.WriteString(value.Render(wt.Last.Subject))
		b.WriteString("\n")
	}

	if len(files) > 0 {
		b.WriteString("\n")
		b.WriteString(label.Render(fmt.Sprintf("Changed files (%d):", len(files))))
		b.WriteString("\n")
		maxFiles := 15
		for i, f := range files {
			if i >= maxFiles {
				b.WriteString(lipgloss.NewStyle().Foreground(t.TextDim).Render(
					fmt.Sprintf("  ... and %d more", len(files)-maxFiles)))
				break
			}
			b.WriteString("  " + lipgloss.NewStyle().Foreground(t.Accent).Render(f))
			b.WriteString("\n")
		}
	}

	if diffStat != "" {
		b.WriteString("\n")
		b.WriteString(lipgloss.NewStyle().Foreground(t.TextDim).Render(diffStat))
	}

	b.WriteString("\n\n")
	hint := lipgloss.NewStyle().Foreground(t.TextDim)
	hKey := lipgloss.NewStyle().Foreground(t.Secondary).Bold(true)
	b.WriteString(hint.Render(
		hKey.Render("e") + " editor  " +
			hKey.Render("a") + " agent  " +
			hKey.Render("c") + " commit  " +
			hKey.Render("p") + " push  " +
			hKey.Render("esc") + " back"))

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
