package components

import (
	"fmt"
	"strings"

	tea "charm.land/bubbletea/v2"

	"github.com/Energma/blink-f/internal/app/screen"
	"github.com/Energma/blink-f/internal/models"
	"github.com/Energma/blink-f/internal/theme"
	"charm.land/lipgloss/v2"
)

// Stats holds computed stats for the status bar.
type Stats struct {
	Worktrees  int
	Sessions   int
	Dirty      int
	Clean      int
	Conflicts  int
	Agents     int
}

// ComputeStats computes dashboard stats from worktrees.
func ComputeStats(worktrees []models.Worktree) Stats {
	var s Stats
	s.Worktrees = len(worktrees)
	for _, wt := range worktrees {
		if wt.Status.TmuxSession != "" {
			s.Sessions++
		}
		if wt.Status.AgentRunning {
			s.Agents++
		}
		if wt.Status.Conflicts > 0 {
			s.Conflicts++
		} else if wt.Status.Clean {
			s.Clean++
		} else if wt.Status.Staged > 0 || wt.Status.Modified > 0 || wt.Status.Untracked > 0 {
			s.Dirty++
		}
	}
	return s
}

// StatusBar renders the bottom status bar with rich stats.
func StatusBar(width int, t *theme.Theme, repoName string, stats Stats, tmuxAvailable bool, activeScreen screen.Type, statusMsg, errMsg string) string {
	dim := lipgloss.NewStyle().Foreground(t.TextDim)
	sep := dim.Render(" | ")

	// Left side: repo + stats
	left := lipgloss.NewStyle().Foreground(t.Secondary).Bold(true).Render(" " + repoName)
	left += sep + dim.Render(fmt.Sprintf("%d wt", stats.Worktrees))

	if tmuxAvailable && stats.Sessions > 0 {
		left += sep + lipgloss.NewStyle().Foreground(t.Success).Render(fmt.Sprintf("%d sess", stats.Sessions))
	}
	if stats.Agents > 0 {
		left += sep + lipgloss.NewStyle().Foreground(t.Accent).Render(fmt.Sprintf("%d agent", stats.Agents))
	}
	if stats.Dirty > 0 {
		left += sep + lipgloss.NewStyle().Foreground(t.Warning).Render(fmt.Sprintf("%d dirty", stats.Dirty))
	}
	if stats.Conflicts > 0 {
		left += sep + lipgloss.NewStyle().Foreground(t.Error).Bold(true).Render(fmt.Sprintf("%d conflicts!", stats.Conflicts))
	}

	// Right side: status/error message
	right := ""
	if errMsg != "" {
		right = lipgloss.NewStyle().Foreground(t.Error).Render(errMsg + " ")
	} else if statusMsg != "" {
		right = lipgloss.NewStyle().Foreground(t.Success).Render(statusMsg + " ")
	}

	// Layout
	gap := width - lipgloss.Width(left) - lipgloss.Width(right) - 1
	if gap < 1 {
		gap = 1
	}
	spaces := make([]byte, gap)
	for i := range spaces {
		spaces[i] = ' '
	}

	return lipgloss.NewStyle().
		Foreground(t.TextDim).
		Width(width).
		Render(left + string(spaces) + right)
}

// PaneID mirrors the app.PaneID type for hint rendering.
type PaneID int

const (
	PaneWorktrees PaneID = iota
	PaneFileTree
)

// Hint is one key hint, such as "n new". Key is what a click on the hint
// presses; it is zero for hints that don't stand for a single key.
type Hint struct {
	Name  string // the key as shown, e.g. "C-down"
	Label string
	Key   tea.Key
}

// HintSpan is where RenderHints drew a hint: cells [X0, X1) of line Line.
type HintSpan struct {
	Line   int
	X0, X1 int
	Hint   Hint
}

// charHint is a hint for a printable key, which is shown as itself.
func charHint(r rune, label string) Hint {
	return Hint{Name: string(r), Label: label, Key: tea.Key{Code: r, Text: string(r)}}
}

// keyHint is a hint for a special key, shown as name.
func keyHint(name, label string, code rune, mod tea.KeyMod) Hint {
	return Hint{Name: name, Label: label, Key: tea.Key{Code: code, Mod: mod}}
}

// textHint is a display-only hint covering several keys.
func textHint(name, label string) Hint {
	return Hint{Name: name, Label: label}
}

// RenderHints lays hints out in lines at most width cells wide, wrapping
// between hints and never inside one; groups are set apart by " | ". The
// spans are measured on the rendered cells in the same pass, so a click is
// matched to the hint actually drawn under it. width <= 0 means one line.
func RenderHints(t *theme.Theme, width int, groups ...[]Hint) (string, []HintSpan) {
	// The key sits on a faint chip so it reads as something to press.
	k := lipgloss.NewStyle().Foreground(t.Secondary).Background(t.Border).Bold(true)
	d := lipgloss.NewStyle().Foreground(t.TextDim)
	gap, sep := "  ", d.Render(" | ")

	var lines []string
	var spans []HintSpan
	line, lineW := "", 0
	for _, group := range groups {
		for i, h := range group {
			text := k.Render(h.Name) + " " + d.Render(h.Label)
			w := lipgloss.Width(text)
			prefix := ""
			if lineW > 0 {
				prefix = gap
				if i == 0 {
					prefix = sep
				}
			}
			if lineW > 0 && width > 0 && lineW+lipgloss.Width(prefix)+w > width {
				lines = append(lines, line)
				line, lineW, prefix = "", 0, ""
			}
			x0 := lineW + lipgloss.Width(prefix)
			line += prefix + text
			lineW = x0 + w
			spans = append(spans, HintSpan{Line: len(lines), X0: x0, X1: lineW, Hint: h})
		}
	}
	lines = append(lines, line)
	return strings.Join(lines, "\n"), spans
}

// PaneHints returns the action hints shown at the bottom of a pane, in
// groups of related actions.
func PaneHints(pane PaneID) [][]Hint {
	switch pane {
	case PaneWorktrees:
		return [][]Hint{
			{charHint('n', "new"), charHint('b', "branch"), charHint('d', "delete"), charHint('D', "clean")},
			{charHint('c', "commit"), charHint('p', "push"), charHint('u', "pull"), charHint('s', "stash")},
			{charHint('e', "editor"), charHint('a', "agent")},
		}
	case PaneFileTree:
		return [][]Hint{{
			keyHint("enter/l", "open", tea.KeyEnter, 0),
			charHint('h', "collapse"),
			keyHint("bs", "up dir", tea.KeyBackspace, 0),
			charHint('f', "find"),
		}}
	}
	return nil
}

// NavHints returns the navigation/global hints shown between the panes.
func NavHints(activeScreen screen.Type, activePane PaneID) []Hint {
	enter := keyHint("enter", "", tea.KeyEnter, 0)
	esc := keyHint("esc", "", tea.KeyEscape, 0)
	tab := keyHint("tab", "field", tea.KeyTab, 0)
	with := func(h Hint, label string) Hint {
		h.Label = label
		return h
	}

	switch activeScreen {
	case screen.WorktreeCreate:
		return []Hint{tab, keyHint("C-b", "branches", 'b', tea.ModCtrl), with(enter, "create"), with(esc, "cancel")}
	case screen.BranchSelect:
		return []Hint{textHint("type", "filter"), textHint("↑/↓", "move"), with(enter, "select"), with(esc, "clear/cancel")}
	case screen.Commit:
		return []Hint{tab, textHint("</>", "type"), with(enter, "commit"), with(esc, "cancel")}
	case screen.Confirm:
		return []Hint{charHint('y', "confirm"), charHint('n', "cancel"), with(esc, "cancel")}
	case screen.Help:
		return []Hint{textHint("j/k", "scroll"), charHint('g', "top"), with(esc, "close")}
	case screen.AgentSelect:
		return []Hint{textHint("j/k", "move"), with(enter, "launch"), with(esc, "cancel")}
	case screen.Sessions:
		return []Hint{textHint("j/k", "move"), with(enter, "attach"), charHint('x', "kill"), charHint('X', "clean"), with(esc, "close")}
	case screen.RepoSelect:
		return []Hint{textHint("j/k", "move"), with(enter, "select"), with(esc, "cancel")}
	case screen.WorktreeDetail:
		return []Hint{charHint('e', "editor"), charHint('a', "agent"), charHint('b', "branch"), charHint('c', "commit"), charHint('p', "push"), with(esc, "back")}
	}
	if activePane == PaneFileTree {
		return []Hint{keyHint("C-up", "worktrees", tea.KeyUp, tea.ModCtrl), charHint('/', "filter wt"), charHint('?', "help"), charHint('q', "quit")}
	}
	return []Hint{
		with(enter, "switch"), charHint('l', "detail"), keyHint("tab", "repo", tea.KeyTab, 0), charHint('/', "filter"),
		charHint('S', "sessions"), charHint('v', "preview"), keyHint("C-down", "browse", tea.KeyDown, tea.ModCtrl),
		charHint('?', "help"), charHint('q', "quit"),
	}
}
