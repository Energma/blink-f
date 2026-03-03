package views

import (
	"strings"

	"github.com/Energma/blink-f/internal/models"
	"github.com/Energma/blink-f/internal/theme"
	"charm.land/lipgloss/v2"
)

// RepoSelect renders the repo browser.
func RepoSelect(repos []models.Repo, activeRepo, cursor int, t *theme.Theme, width int) string {
	title := lipgloss.NewStyle().
		Bold(true).
		Foreground(t.Primary).
		Render("Repositories")

	var b strings.Builder
	b.WriteString(title)
	b.WriteString("\n\n")

	if len(repos) == 0 {
		b.WriteString(lipgloss.NewStyle().Foreground(t.TextDim).Render(
			"No repos configured.\nAdd repos to ~/.config/blink/config.yaml"))
		b.WriteString("\n\n")
		b.WriteString(lipgloss.NewStyle().Foreground(t.TextDim).Render("esc: close"))

		return lipgloss.NewStyle().
			Border(lipgloss.RoundedBorder()).
			BorderForeground(t.Border).
			Padding(1, 2).
			Width(50).
			Render(b.String())
	}

	for i, r := range repos {
		cur := "  "
		if i == cursor {
			cur = lipgloss.NewStyle().Foreground(t.Primary).Render("> ")
		}

		name := r.DisplayName()
		activeIndicator := ""
		if i == activeRepo {
			activeIndicator = lipgloss.NewStyle().Foreground(t.Success).Render(" (active)")
		}

		if i == cursor {
			name = lipgloss.NewStyle().Bold(true).Foreground(t.Text).Render(name)
		} else {
			name = lipgloss.NewStyle().Foreground(t.TextDim).Render(name)
		}

		path := lipgloss.NewStyle().Foreground(t.Muted).Render("  " + r.Path)

		b.WriteString(cur + name + activeIndicator)
		b.WriteString("\n")
		b.WriteString("    " + path)
		b.WriteString("\n")
	}

	b.WriteString("\n")
	b.WriteString(lipgloss.NewStyle().Foreground(t.TextDim).Render(
		"enter: select  esc: cancel"))

	modalWidth := 55
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
