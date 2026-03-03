package app

import (
	"github.com/Energma/blink-f/internal/theme"
	"charm.land/lipgloss/v2"
)

// Styles holds all lipgloss styles used by the TUI.
type Styles struct {
	// Layout
	App       lipgloss.Style
	Header    lipgloss.Style
	Content   lipgloss.Style
	StatusBar lipgloss.Style

	// Worktree list
	ItemNormal   lipgloss.Style
	ItemSelected lipgloss.Style
	ItemMain     lipgloss.Style
	BranchName   lipgloss.Style
	PathDim      lipgloss.Style

	// Status indicators
	StatusClean    lipgloss.Style
	StatusDirty    lipgloss.Style
	StatusAhead    lipgloss.Style
	StatusBehind   lipgloss.Style
	StatusConflict lipgloss.Style
	TmuxActive     lipgloss.Style

	// Modal/overlay
	Modal       lipgloss.Style
	ModalTitle  lipgloss.Style
	ModalBorder lipgloss.Style

	// Input
	Input      lipgloss.Style
	InputLabel lipgloss.Style
	Cursor     lipgloss.Style

	// General
	Title   lipgloss.Style
	Subtle  lipgloss.Style
	Bold    lipgloss.Style
	Error   lipgloss.Style
	Success lipgloss.Style
	Warning lipgloss.Style
	Muted   lipgloss.Style
	KeyHint lipgloss.Style
}

func NewStyles(t *theme.Theme) *Styles {
	return &Styles{
		App: lipgloss.NewStyle().
			Padding(0, 1),

		Header: lipgloss.NewStyle().
			Bold(true).
			Foreground(t.Primary).
			Padding(0, 0, 1, 0),

		Content: lipgloss.NewStyle().
			Padding(0),

		StatusBar: lipgloss.NewStyle().
			Foreground(t.TextDim).
			Padding(0, 1),

		ItemNormal: lipgloss.NewStyle().
			Padding(0, 2),

		ItemSelected: lipgloss.NewStyle().
			Foreground(t.Text).
			Background(t.Highlight).
			Bold(true).
			Padding(0, 2),

		ItemMain: lipgloss.NewStyle().
			Foreground(t.Accent).
			Padding(0, 2),

		BranchName: lipgloss.NewStyle().
			Foreground(t.Secondary).
			Bold(true),

		PathDim: lipgloss.NewStyle().
			Foreground(t.TextDim),

		StatusClean: lipgloss.NewStyle().
			Foreground(t.Success),

		StatusDirty: lipgloss.NewStyle().
			Foreground(t.Warning),

		StatusAhead: lipgloss.NewStyle().
			Foreground(t.Secondary),

		StatusBehind: lipgloss.NewStyle().
			Foreground(t.Error),

		StatusConflict: lipgloss.NewStyle().
			Foreground(t.Error).
			Bold(true),

		TmuxActive: lipgloss.NewStyle().
			Foreground(t.Success),

		Modal: lipgloss.NewStyle().
			Border(lipgloss.RoundedBorder()).
			BorderForeground(t.Border).
			Padding(1, 2),

		ModalTitle: lipgloss.NewStyle().
			Bold(true).
			Foreground(t.Primary),

		ModalBorder: lipgloss.NewStyle().
			BorderForeground(t.Primary),

		Input: lipgloss.NewStyle().
			Foreground(t.Text),

		InputLabel: lipgloss.NewStyle().
			Foreground(t.TextDim),

		Cursor: lipgloss.NewStyle().
			Foreground(t.Primary),

		Title: lipgloss.NewStyle().
			Bold(true).
			Foreground(t.Primary),

		Subtle: lipgloss.NewStyle().
			Foreground(t.TextDim),

		Bold: lipgloss.NewStyle().
			Bold(true),

		Error: lipgloss.NewStyle().
			Foreground(t.Error),

		Success: lipgloss.NewStyle().
			Foreground(t.Success),

		Warning: lipgloss.NewStyle().
			Foreground(t.Warning),

		Muted: lipgloss.NewStyle().
			Foreground(t.Muted),

		KeyHint: lipgloss.NewStyle().
			Foreground(t.TextDim),
	}
}
