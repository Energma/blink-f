package app

import (
	"strings"
	"testing"

	"github.com/charmbracelet/x/ansi"
	"github.com/stretchr/testify/assert"

	"github.com/Energma/blink-f/internal/app/screen"
	"github.com/Energma/blink-f/internal/theme"
)

func (r rect) overlaps(o rect) bool {
	return r.X < o.X+o.W && o.X < r.X+r.W && r.Y < o.Y+o.H && o.Y < r.Y+r.H
}

func TestComputeLayout(t *testing.T) {
	tests := []struct {
		name          string
		width, height int
		preview       bool
		wantPreview   bool
		wantUpperH    int
	}{
		{"narrow, preview requested", 80, 40, true, false, 17},
		{"laptop window shows preview", 98, 40, true, true, 17},
		{"wide, preview off", 160, 45, false, false, 17},
		{"wide, preview on", 160, 45, true, true, 17},
		{"exactly at threshold", previewMinWidth, 30, true, true, 16},
		{"short terminal keeps 60/40", 200, 22, true, true, 12},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			l := computeLayout(tt.width, tt.height, tt.preview)

			assert.Equal(t, tt.wantPreview, l.Preview.W > 0)
			assert.Equal(t, tt.wantUpperH, l.Worktrees.H)

			rects := []rect{l.Worktrees, l.FileTree, l.Hints, l.Status}
			if tt.wantPreview {
				rects = append(rects, l.Preview)
			} else {
				assert.Equal(t, rect{}, l.Preview)
			}

			// Every rect sits inside the App padding and on screen.
			for _, r := range rects {
				assert.GreaterOrEqual(t, r.X, appPaddingX)
				assert.GreaterOrEqual(t, r.Y, 0)
				assert.LessOrEqual(t, r.X+r.W, tt.width-appPaddingX)
				assert.LessOrEqual(t, r.Y+r.H, tt.height)
			}
			for i := range rects {
				for j := i + 1; j < len(rects); j++ {
					assert.False(t, rects[i].overlaps(rects[j]), "rect %d overlaps rect %d", i, j)
				}
			}

			// The rows add up to the whole screen, status bar last.
			assert.Equal(t, tt.height-1, l.Status.Y)
			assert.Equal(t, l.Worktrees.W, l.FileTree.W)
			if tt.wantPreview {
				assert.Equal(t, l.Worktrees.X+l.Worktrees.W, l.Preview.X)
				assert.Equal(t, l.Worktrees.H+l.FileTree.H, l.Preview.H)
				assert.Equal(t, tt.width-2*appPaddingX, l.Worktrees.W+l.Preview.W)
				assert.GreaterOrEqual(t, l.Worktrees.W, leftMinWidth)
			} else {
				assert.Equal(t, tt.width-2*appPaddingX, l.Worktrees.W)
				assert.Equal(t, l.Worktrees.H, l.Hints.Y)
			}
		})
	}
}

func TestFitLines(t *testing.T) {
	tests := []struct {
		name string
		in   string
		w, h int
		want string
	}{
		{"fits", "ab\ncd", 5, 5, "ab\ncd"},
		{"keeps the top", "1\n2\n3\n4", 5, 2, "1\n2"},
		{"cuts wide lines", "abcdef", 3, 1, "abc"},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			assert.Equal(t, tt.want, fitLines(tt.in, tt.w, tt.h))
		})
	}
}

func TestPreviewLines(t *testing.T) {
	red := "\x1b[31m"
	tests := []struct {
		name string
		in   string
		w, h int
		want []string // plain text of each line
	}{
		{"trims trailing blank lines", "a\nb\n\n   \n", 10, 10, []string{"a", "b"}},
		{"keeps the bottom", "1\n2\n3\n4\n\n", 10, 2, []string{"3", "4"}},
		{"blank line with only escapes is trimmed", "a\n" + red + "\x1b[0m\n", 10, 10, []string{"a"}},
		{"cuts by cell width", "abcdefgh", 4, 1, []string{"abcd"}},
		{"wide chars are not split", "日本語", 5, 1, []string{"日本"}},
		{"escapes don't count toward width", red + "abcdef", 3, 1, []string{"abc"}},
		{"tabs become spaces", "a\tb", 4, 1, []string{"a   "}},
		{"carriage returns dropped", "a\r\nb\r\n", 10, 10, []string{"a", "b"}},
		{"empty capture", "\n\n", 10, 10, nil},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			out := previewLines(tt.in, tt.w, tt.h)
			if tt.want == nil {
				assert.Empty(t, out)
				return
			}
			lines := strings.Split(out, "\n")
			var plain []string
			for _, line := range lines {
				assert.LessOrEqual(t, ansi.StringWidth(line), tt.w)
				assert.True(t, strings.HasSuffix(line, ansi.ResetStyle), "line %q must end with a reset", line)
				plain = append(plain, ansi.Strip(line))
			}
			assert.Equal(t, tt.want, plain)
		})
	}
}

func TestInjectBorderTitle(t *testing.T) {
	m := &Model{theme: theme.Default()}
	pane := m.renderPane(rect{W: 20, H: 4}, " A long pane title here ", "x", true)
	lines := strings.Split(pane, "\n")
	assert.Len(t, lines, 4)
	for _, line := range lines {
		assert.Equal(t, 20, ansi.StringWidth(line))
	}
	top := ansi.Strip(lines[0])
	assert.True(t, strings.HasPrefix(top, "╭ A long"), top)
	assert.True(t, strings.HasSuffix(top, "─╮"), top)
}

func TestPreviewTickGuard(t *testing.T) {
	m := &Model{
		screenMgr:     screen.NewManager(),
		width:         160,
		height:        45,
		previewOn:     true,
		tmuxAvailable: true,
	}

	assert.NotNil(t, m.previewCmd(""), "first visible update starts the loop")
	assert.True(t, m.previewTicking)
	assert.Nil(t, m.previewCmd(""), "a tick is already in flight")

	// The tick only clears the flag; the next Update re-arms it.
	_, _ = m.update(previewTickMsg{})
	assert.False(t, m.previewTicking)
	assert.NotNil(t, m.previewCmd(""))

	// Hidden: the in-flight tick lands and the loop stops.
	m.previewOn = false
	_, _ = m.update(previewTickMsg{})
	assert.Nil(t, m.previewCmd(""))
	assert.False(t, m.previewTicking)

	// Narrow terminals and a missing multiplexer never start it.
	m.previewOn = true
	m.width = previewMinWidth - 1
	assert.Nil(t, m.previewCmd(""))
	m.width = 160
	m.tmuxAvailable = false
	assert.Nil(t, m.previewCmd(""))
}
