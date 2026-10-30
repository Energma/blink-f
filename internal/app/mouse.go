package app

import (
	"time"

	tea "charm.land/bubbletea/v2"

	"github.com/Energma/blink-f/internal/app/components"
	"github.com/Energma/blink-f/internal/app/screen"
)

// doubleClickInterval is the longest gap between two clicks on the same row
// that still counts as a double-click, about what desktops default to.
const doubleClickInterval = 400 * time.Millisecond

// region is the part of the dashboard a mouse event landed on.
type region int

const (
	regionNone region = iota
	regionWorktrees
	regionFileTree
	regionPreview
	regionHints
)

// clickTarget is what a left click hit; two equal targets in quick
// succession make a double-click.
type clickTarget struct {
	region region
	item   int // row's item index, -1 for no item
}

func (r rect) contains(x, y int) bool {
	return x >= r.X && x < r.X+r.W && y >= r.Y && y < r.Y+r.H
}

// regionAt hit-tests a screen cell against the panes.
func (l layout) regionAt(x, y int) region {
	switch {
	case l.Worktrees.contains(x, y):
		return regionWorktrees
	case l.FileTree.contains(x, y):
		return regionFileTree
	case l.Preview.contains(x, y):
		return regionPreview
	case l.Hints.contains(x, y):
		return regionHints
	}
	return regionNone
}

// itemAt maps screen row y inside pane r to the item drawn there, given the
// pane view's rows (see views.Dashboard); -1 for borders and non-item lines.
func itemAt(r rect, y int, rows []int) int {
	line := y - r.Y - 1 // skip the top border
	if line < 0 || line >= r.H-2 || line >= len(rows) {
		return -1
	}
	return rows[line]
}

// paneHintAt finds the pane hint drawn at screen cell x, y inside pane r,
// given the spans from the pane's view; false for anything else, including
// spans that fitLines clipped off the pane.
func paneHintAt(r rect, x, y int, spans []components.HintSpan) (tea.Key, bool) {
	line, col := y-r.Y-1, x-r.X-1 // skip the border
	if line >= r.H-2 || col >= r.W-2 {
		return tea.Key{}, false
	}
	return hintAt(spans, line, col)
}

// hintAt finds the hint at column x of line, skipping display-only hints.
func hintAt(spans []components.HintSpan, line, x int) (tea.Key, bool) {
	for _, s := range spans {
		if s.Line == line && x >= s.X0 && x < s.X1 && s.Hint.Key.Code != 0 {
			return s.Hint.Key, true
		}
	}
	return tea.Key{}, false
}

// mouseActive reports whether the dashboard takes mouse input. Modals stay
// keyboard-only, and nothing reacts while a session switch is in flight, as
// with keys.
func (m *Model) mouseActive() bool {
	return m.screenMgr.Active() == screen.None && !m.switching
}

// pressHint does what pressing key does in the current context, so a click
// on a hint behaves exactly like the keyboard, confirm modals included.
func (m *Model) pressHint(key tea.Key) (tea.Model, tea.Cmd) {
	m.lastClick = clickTarget{}
	return m.update(tea.KeyPressMsg(key))
}

// handleMouseClick focuses and selects what a left click hit; a second click
// on the same item within doubleClickInterval does what enter does there. A
// click on a key hint presses that key. now is passed in so tests control
// the clock.
func (m *Model) handleMouseClick(msg tea.Mouse, now time.Time) (tea.Model, tea.Cmd) {
	if msg.Button != tea.MouseLeft || !m.mouseActive() {
		return m, nil
	}
	l := m.layout()
	target := clickTarget{region: l.regionAt(msg.X, msg.Y), item: -1}
	// Panes are hit-tested as drawn, before the click moves focus: focus
	// decides whether a pane shows its hints, and so where its rows are.
	switch target.region {
	case regionWorktrees:
		_, rows, hints := m.dashboard(l.Worktrees)
		m.activePane = PaneWorktrees
		if key, ok := paneHintAt(l.Worktrees, msg.X, msg.Y, hints); ok {
			return m.pressHint(key)
		}
		target.item = itemAt(l.Worktrees, msg.Y, rows)
	case regionFileTree:
		_, rows, hints := m.fileTree(l.FileTree)
		m.activePane = PaneFileTree
		if key, ok := paneHintAt(l.FileTree, msg.X, msg.Y, hints); ok {
			return m.pressHint(key)
		}
		target.item = itemAt(l.FileTree, msg.Y, rows)
	case regionPreview:
		m.lastClick = clickTarget{}
		return m, m.openSelected()
	case regionHints:
		_, hints := m.navHints(l.Hints)
		if key, ok := hintAt(hints, msg.Y-l.Hints.Y, msg.X-l.Hints.X); ok {
			return m.pressHint(key)
		}
		m.lastClick = clickTarget{}
		return m, nil
	default:
		m.lastClick = clickTarget{}
		return m, nil
	}

	double := target.item >= 0 && target == m.lastClick && now.Sub(m.lastClickAt) <= doubleClickInterval
	if double {
		// A third click starts a new pair rather than firing again.
		m.lastClick = clickTarget{}
	} else {
		m.lastClick = target
		m.lastClickAt = now
	}
	if target.item < 0 {
		return m, nil
	}

	if target.region == regionWorktrees {
		m.cursor = target.item
		if double {
			return m, m.openSelected()
		}
		return m, nil
	}
	m.treeCursor = target.item
	if double {
		return m.handleTreeKey("enter")
	}
	return m, m.treeHoverPreview()
}

// handleMouseWheel moves the cursor of the pane under the pointer one row
// per wheel step, focusing that pane.
func (m *Model) handleMouseWheel(msg tea.Mouse) (tea.Model, tea.Cmd) {
	if !m.mouseActive() {
		return m, nil
	}
	var delta int
	switch msg.Button {
	case tea.MouseWheelUp:
		delta = -1
	case tea.MouseWheelDown:
		delta = 1
	default:
		return m, nil
	}

	switch m.layout().regionAt(msg.X, msg.Y) {
	case regionWorktrees:
		m.activePane = PaneWorktrees
		m.cursor = max(0, min(m.cursor+delta, len(m.filteredIndices)-1))
	case regionFileTree:
		m.activePane = PaneFileTree
		if delta < 0 {
			return m.handleTreeKey("up")
		}
		return m.handleTreeKey("down")
	}
	return m, nil
}
