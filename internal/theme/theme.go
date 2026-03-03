package theme

import (
	"image/color"

	"charm.land/lipgloss/v2"
)

// Theme defines all colors used by the TUI.
type Theme struct {
	Primary    color.Color
	Secondary  color.Color
	Accent     color.Color
	Success    color.Color
	Warning    color.Color
	Error      color.Color
	Muted      color.Color
	Text       color.Color
	TextDim    color.Color
	Border     color.Color
	Highlight  color.Color
	Background color.Color
}

// Default returns the default color theme.
func Default() *Theme {
	return &Theme{
		Primary:    lipgloss.Color("#7C3AED"),
		Secondary:  lipgloss.Color("#06B6D4"),
		Accent:     lipgloss.Color("#F59E0B"),
		Success:    lipgloss.Color("#10B981"),
		Warning:    lipgloss.Color("#F59E0B"),
		Error:      lipgloss.Color("#EF4444"),
		Muted:      lipgloss.Color("#6B7280"),
		Text:       lipgloss.Color("#F9FAFB"),
		TextDim:    lipgloss.Color("#9CA3AF"),
		Border:     lipgloss.Color("#374151"),
		Highlight:  lipgloss.Color("#7C3AED"),
		Background: lipgloss.Color("#111827"),
	}
}

// Minimal returns a low-color theme for basic terminals.
func Minimal() *Theme {
	return &Theme{
		Primary:    lipgloss.Color("5"),
		Secondary:  lipgloss.Color("6"),
		Accent:     lipgloss.Color("3"),
		Success:    lipgloss.Color("2"),
		Warning:    lipgloss.Color("3"),
		Error:      lipgloss.Color("1"),
		Muted:      lipgloss.Color("8"),
		Text:       lipgloss.Color("15"),
		TextDim:    lipgloss.Color("7"),
		Border:     lipgloss.Color("8"),
		Highlight:  lipgloss.Color("5"),
		Background: lipgloss.Color("0"),
	}
}

// Get returns a theme by name.
func Get(name string) *Theme {
	switch name {
	case "minimal":
		return Minimal()
	default:
		return Default()
	}
}
