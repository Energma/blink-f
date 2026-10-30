package app

import (
	"strings"

	"github.com/charmbracelet/x/ansi"
)

const (
	// previewMinWidth is the narrowest terminal that still leaves the
	// worktree list readable next to a preview wide enough to tell what an
	// agent is doing. Kept low enough for a half-screen or laptop window,
	// where the preview is most useful.
	previewMinWidth = 90
	// leftMinWidth keeps worktree rows legible when the preview takes the
	// rest of a terminal just over previewMinWidth.
	leftMinWidth   = 50
	maxUpperHeight = 17 // fits 5 items (2 lines each) + header + scroll indicators + border
	minPaneHeight  = 5  // border + 3 content lines
	appPaddingX    = 1  // Styles.App pads one column on each side
)

// rect is a block of terminal cells: column X, row Y (0-based, from the top
// left of the screen), W columns wide and H rows tall, borders included.
type rect struct {
	X, Y, W, H int
}

// layout holds where each part of the dashboard sits on screen. View renders
// from it, so the rects are the true on-screen positions and can be used to
// hit-test mouse events.
type layout struct {
	Worktrees rect
	FileTree  rect
	Preview   rect // zero when the preview is hidden
	Hints     rect // nav hints line
	Status    rect // status bar line
}

// computeLayout splits a width x height terminal into panes. Without the
// preview the nav hints sit between the two stacked panes; with it they move
// to a full-width line above the status bar, because the left column is too
// narrow to hold them and the preview spans the full pane height.
func computeLayout(width, height int, preview bool) layout {
	innerW := max(width-2*appPaddingX, 20)
	// One line each for the nav hints and the status bar.
	paneH := max(height-2, 2*minPaneHeight)

	// 60/40 split, with the upper pane capped so the file tree gets the rest.
	upperH := min(paneH*60/100, maxUpperHeight)
	upperH = max(upperH, minPaneHeight)
	lowerH := max(paneH-upperH, minPaneHeight)

	var l layout
	l.Status = rect{X: appPaddingX, Y: upperH + lowerH + 1, W: innerW, H: 1}

	if !preview || width < previewMinWidth {
		l.Worktrees = rect{X: appPaddingX, Y: 0, W: innerW, H: upperH}
		l.Hints = rect{X: appPaddingX, Y: upperH, W: innerW, H: 1}
		l.FileTree = rect{X: appPaddingX, Y: upperH + 1, W: innerW, H: lowerH}
		return l
	}

	leftW := max(innerW*45/100, leftMinWidth)
	l.Worktrees = rect{X: appPaddingX, Y: 0, W: leftW, H: upperH}
	l.FileTree = rect{X: appPaddingX, Y: upperH, W: leftW, H: lowerH}
	l.Preview = rect{X: appPaddingX + leftW, Y: 0, W: innerW - leftW, H: upperH + lowerH}
	l.Hints = rect{X: appPaddingX, Y: upperH + lowerH, W: innerW, H: 1}
	return l
}

// fitLines clips text to at most h lines of at most w cells, keeping the top.
// Pane views don't always respect their height, and an overflowing pane would
// push everything below it off its rect.
func fitLines(s string, w, h int) string {
	lines := strings.Split(s, "\n")
	if len(lines) > h {
		lines = lines[:h]
	}
	for i, line := range lines {
		lines[i] = ansi.Truncate(line, w, "")
	}
	return strings.Join(lines, "\n")
}

// previewLines turns a pane capture into at most h lines of at most w cells.
// Trailing blank lines are dropped first so the cursor line, where an agent
// is active, sits at the bottom; then the last h lines are kept. Each line
// ends with an SGR reset so a colour cut off mid-line can't bleed into the
// border or the next line.
func previewLines(capture string, w, h int) string {
	capture = strings.ReplaceAll(capture, "\r", "")
	// Truncation measures cells and counts a tab as zero; lipgloss would
	// later expand it to spaces and overflow the pane.
	capture = strings.ReplaceAll(capture, "\t", "    ")
	lines := strings.Split(capture, "\n")
	for len(lines) > 0 && strings.TrimSpace(ansi.Strip(lines[len(lines)-1])) == "" {
		lines = lines[:len(lines)-1]
	}
	if len(lines) > h {
		lines = lines[len(lines)-h:]
	}
	for i, line := range lines {
		lines[i] = ansi.Truncate(line, w, "") + ansi.ResetStyle
	}
	return strings.Join(lines, "\n")
}
