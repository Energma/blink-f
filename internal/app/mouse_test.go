package app

import (
	"fmt"
	"strings"
	"testing"
	"time"

	tea "charm.land/bubbletea/v2"
	"github.com/charmbracelet/x/ansi"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/Energma/blink-f/internal/app/components"
	"github.com/Energma/blink-f/internal/app/screen"
	"github.com/Energma/blink-f/internal/config"
	"github.com/Energma/blink-f/internal/models"
	"github.com/Energma/blink-f/internal/theme"
)

// newMouseModel builds a ready dashboard at 160x45 with the preview off, n
// worktrees named feat-NN and a file tree of dirs named dir-NN.
func newMouseModel(n int) *Model {
	t := theme.Default()
	m := &Model{
		cfg:           config.Default(),
		theme:         t,
		styles:        NewStyles(t),
		screenMgr:     screen.NewManager(),
		agentSessions: make(map[string]string),
		width:         160,
		height:        45,
		ready:         true,
	}
	for i := range n {
		name := fmt.Sprintf("feat-%02d", i)
		m.worktrees = append(m.worktrees, models.Worktree{Branch: name, Path: "/r/wt/" + name, RepoDir: "/r/repo"})
		m.filteredIndices = append(m.filteredIndices, i)
	}
	m.treeRoot = &components.TreeNode{Name: "root", Path: "/t", Loaded: true, Expanded: true}
	for i := range 5 {
		name := fmt.Sprintf("dir-%02d", i)
		m.treeRoot.Children = append(m.treeRoot.Children, components.TreeNode{Name: name, Path: "/t/" + name, Depth: 1})
	}
	m.treeFlatNodes = components.FlattenVisible(m.treeRoot)
	return m
}

// cellOf finds text on the rendered screen and returns its cell, so clicks
// in tests aim at what View actually draws.
func cellOf(t *testing.T, m *Model, text string) (x, y int) {
	t.Helper()
	for y, line := range strings.Split(m.View().Content, "\n") {
		plain := ansi.Strip(line)
		if i := strings.Index(plain, text); i >= 0 {
			return ansi.StringWidth(plain[:i]), y
		}
	}
	require.Failf(t, "text not on screen", "%q", text)
	return 0, 0
}

func leftClick(x, y int) tea.Mouse {
	return tea.Mouse{X: x, Y: y, Button: tea.MouseLeft}
}

func TestRegionAt(t *testing.T) {
	wide := computeLayout(160, 45, true)
	stacked := computeLayout(160, 45, false)

	tests := []struct {
		name string
		l    layout
		x, y int
		want region
	}{
		{"worktrees top left corner", wide, wide.Worktrees.X, wide.Worktrees.Y, regionWorktrees},
		{"worktrees bottom right corner", wide, wide.Worktrees.X + wide.Worktrees.W - 1, wide.Worktrees.Y + wide.Worktrees.H - 1, regionWorktrees},
		{"file tree below worktrees", wide, wide.FileTree.X + 5, wide.FileTree.Y, regionFileTree},
		{"preview right of worktrees", wide, wide.Worktrees.X + wide.Worktrees.W, 0, regionPreview},
		{"hints line", wide, 10, wide.Hints.Y, regionHints},
		{"status bar", wide, 10, wide.Status.Y, regionNone},
		{"app padding column", wide, 0, 3, regionNone},
		{"stacked hints between panes", stacked, 10, stacked.Hints.Y, regionHints},
		{"stacked file tree", stacked, 150, stacked.FileTree.Y + 2, regionFileTree},
		{"no preview when hidden", stacked, 150, 2, regionWorktrees},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			assert.Equal(t, tt.want, tt.l.regionAt(tt.x, tt.y))
		})
	}
}

func TestDashboardRows(t *testing.T) {
	tests := []struct {
		name       string
		items      int
		cursor     int
		filterMode bool
		firstItem  int // position of the topmost drawn item
		firstLine  int // content line it starts on
	}{
		{"plain list", 4, 0, false, 0, 2},
		{"filter line pushes items down", 4, 0, true, 0, 3},
		{"scrolled list skips the indicator", 20, 12, false, 9, 3},
		{"scrolled with filter", 20, 12, true, 9, 4},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			m := newMouseModel(tt.items)
			m.cursor = tt.cursor
			m.filterMode = tt.filterMode
			r := m.layout().Worktrees
			content, rows, _ := m.dashboard(r)
			lines := strings.Split(content, "\n")

			assert.Equal(t, tt.firstLine, indexOf(rows, tt.firstItem))
			for i := 0; i < tt.firstLine; i++ {
				assert.Equal(t, -1, rows[i], "line %d precedes the items", i)
			}
			// Each item's first line shows that item's branch.
			for i, item := range rows {
				if item < 0 || (i > 0 && rows[i-1] == item) {
					continue
				}
				want := m.worktrees[m.filteredIndices[item]].Branch
				assert.Contains(t, ansi.Strip(lines[i]), want, "line %d", i)
			}
			assert.Contains(t, rows, tt.cursor, "cursor row is drawn")
		})
	}
}

func indexOf(s []int, v int) int {
	for i, x := range s {
		if x == v {
			return i
		}
	}
	return -1
}

func TestMouseClickSelects(t *testing.T) {
	tests := []struct {
		name       string
		items      int
		cursor     int
		filterMode bool
		modal      bool
		click      string // text to click on
		wantCursor int
		wantTree   int
		wantPane   PaneID
	}{
		{"worktree row", 6, 0, false, false, "feat-03", 3, 0, PaneWorktrees},
		{"worktree detail line", 6, 0, false, false, "wt/feat-04", 4, 0, PaneWorktrees},
		{"worktree row in filter mode", 6, 0, true, false, "feat-02", 2, 0, PaneWorktrees},
		{"worktree row in scrolled list", 20, 12, false, false, "feat-10", 10, 0, PaneWorktrees},
		{"header focuses only", 6, 2, false, false, "BLINK", 2, 0, PaneWorktrees},
		{"tree node", 6, 1, false, false, "dir-02/", 1, 3, PaneFileTree},
		{"modal ignores clicks", 6, 1, false, true, "dir-02/", 1, 0, PaneWorktrees},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			m := newMouseModel(tt.items)
			m.cursor = tt.cursor
			m.filterMode = tt.filterMode
			x, y := cellOf(t, m, tt.click)
			if tt.modal {
				m.screenMgr.Push(screen.Confirm)
			}

			_, _ = m.Update(tea.MouseClickMsg(leftClick(x, y)))

			assert.Equal(t, tt.wantCursor, m.cursor)
			assert.Equal(t, tt.wantTree, m.treeCursor)
			assert.Equal(t, tt.wantPane, m.activePane)
		})
	}
}

func TestMouseDoubleClick(t *testing.T) {
	t0 := time.Date(2026, 1, 1, 0, 0, 0, 0, time.UTC)

	tests := []struct {
		name       string
		first      string
		second     string
		gap        time.Duration
		wantSwitch bool
	}{
		{"quick pair opens", "feat-01", "feat-01", 200 * time.Millisecond, true},
		{"at the limit opens", "feat-01", "feat-01", doubleClickInterval, true},
		{"slow pair only selects", "feat-01", "feat-01", doubleClickInterval + time.Millisecond, false},
		{"other item only selects", "feat-01", "feat-02", 100 * time.Millisecond, false},
		{"same item, other line of it", "feat-01", "wt/feat-01", 100 * time.Millisecond, true},
		{"header twice does nothing", "BLINK", "BLINK", 100 * time.Millisecond, false},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			m := newMouseModel(4)
			m.tmuxAvailable = true
			x1, y1 := cellOf(t, m, tt.first)
			x2, y2 := cellOf(t, m, tt.second)

			_, cmd := m.handleMouseClick(leftClick(x1, y1), t0)
			assert.Nil(t, cmd)
			assert.False(t, m.switching)
			_, cmd = m.handleMouseClick(leftClick(x2, y2), t0.Add(tt.gap))

			assert.Equal(t, tt.wantSwitch, m.switching)
			assert.Equal(t, tt.wantSwitch, cmd != nil)
		})
	}
}

func TestMouseDoubleClickTree(t *testing.T) {
	t0 := time.Date(2026, 1, 1, 0, 0, 0, 0, time.UTC)
	m := newMouseModel(2)
	x, y := cellOf(t, m, "dir-01/")

	_, _ = m.handleMouseClick(leftClick(x, y), t0)
	_, cmd := m.handleMouseClick(leftClick(x, y), t0.Add(100*time.Millisecond))

	// enter on an unloaded dir lists it, as the keyboard does.
	assert.Equal(t, 2, m.treeCursor)
	assert.NotNil(t, cmd)
}

func TestMouseClickPreviewAttaches(t *testing.T) {
	m := newMouseModel(3)
	m.previewOn = true
	m.tmuxAvailable = true
	m.cursor = 1
	p := m.layout().Preview
	require.NotZero(t, p.W)

	_, cmd := m.handleMouseClick(leftClick(p.X+p.W/2, p.Y+p.H/2), time.Now())

	assert.True(t, m.switching)
	assert.NotNil(t, cmd)
	assert.Equal(t, 1, m.cursor)
}

func TestMouseWheel(t *testing.T) {
	l := computeLayout(160, 45, false)
	overList := [2]int{10, l.Worktrees.Y + 3}
	overTree := [2]int{10, l.FileTree.Y + 3}
	overHints := [2]int{10, l.Hints.Y}

	tests := []struct {
		name       string
		at         [2]int
		button     tea.MouseButton
		startPane  PaneID
		wantCursor int
		wantTree   int
		wantPane   PaneID
	}{
		{"down over list", overList, tea.MouseWheelDown, PaneFileTree, 2, 0, PaneWorktrees},
		{"up over list", overList, tea.MouseWheelUp, PaneWorktrees, 0, 0, PaneWorktrees},
		{"down over tree", overTree, tea.MouseWheelDown, PaneWorktrees, 1, 1, PaneFileTree},
		{"up over tree stops at top", overTree, tea.MouseWheelUp, PaneWorktrees, 1, 0, PaneFileTree},
		{"over hints ignored", overHints, tea.MouseWheelDown, PaneWorktrees, 1, 0, PaneWorktrees},
		{"sideways ignored", overList, tea.MouseWheelLeft, PaneFileTree, 1, 0, PaneFileTree},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			m := newMouseModel(3)
			m.cursor = 1
			m.activePane = tt.startPane

			_, _ = m.Update(tea.MouseWheelMsg{X: tt.at[0], Y: tt.at[1], Button: tt.button})

			assert.Equal(t, tt.wantCursor, m.cursor)
			assert.Equal(t, tt.wantTree, m.treeCursor)
			assert.Equal(t, tt.wantPane, m.activePane)
		})
	}
}

func TestMouseWheelBottom(t *testing.T) {
	m := newMouseModel(3)
	m.cursor = 2
	r := m.layout().Worktrees

	_, _ = m.Update(tea.MouseWheelMsg{X: r.X + 2, Y: r.Y + 2, Button: tea.MouseWheelDown})

	assert.Equal(t, 2, m.cursor)
}

func TestMouseModeFollowsConfig(t *testing.T) {
	tests := []struct {
		name  string
		mouse bool
		want  tea.MouseMode
	}{
		{"on by default", true, tea.MouseModeCellMotion},
		{"off leaves the terminal alone", false, tea.MouseModeNone},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			m := newMouseModel(1)
			m.cfg.UI.Mouse = tt.mouse
			assert.Equal(t, tt.want, m.View().MouseMode)
		})
	}
}

// keyState is the part of the model a hint's key can change, compared to
// check that a click on a hint does what pressing its key does.
type keyState struct {
	Screen         screen.Type
	Pane           PaneID
	PreviewOn      bool
	FilterMode     bool
	TreeFilterMode bool
	Cursor         int
	TreeCursor     int
	Status         string
	ConfirmTitle   string
	Switching      bool
}

func stateOf(m *Model) keyState {
	return keyState{
		Screen: m.screenMgr.Active(), Pane: m.activePane, PreviewOn: m.previewOn,
		FilterMode: m.filterMode, TreeFilterMode: m.treeFilterMode,
		Cursor: m.cursor, TreeCursor: m.treeCursor,
		Status: m.statusText, ConfirmTitle: m.confirmTitle, Switching: m.switching,
	}
}

func TestHintClickMatchesKey(t *testing.T) {
	tests := []struct {
		name    string
		preview bool
		pane    PaneID
		click   string // hint text to click on
		key     tea.Key
		changed func(*Model) bool // the key did something, so the test is not vacuous
	}{
		{"nav help", false, PaneWorktrees, "? help", tea.Key{Code: '?', Text: "?"},
			func(m *Model) bool { return m.screenMgr.Active() == screen.Help }},
		{"nav preview toggle", false, PaneWorktrees, "v preview", tea.Key{Code: 'v', Text: "v"},
			func(m *Model) bool { return m.previewOn }},
		{"nav focus worktrees from tree", false, PaneFileTree, "C-up worktrees", tea.Key{Code: tea.KeyUp, Mod: tea.ModCtrl},
			func(m *Model) bool { return m.activePane == PaneWorktrees }},
		{"worktree pane hint opens its modal", false, PaneWorktrees, "D clean", tea.Key{Code: 'D', Text: "D"},
			func(m *Model) bool { return m.screenMgr.Active() == screen.Confirm }},
		{"tree pane hint acts in the tree", false, PaneFileTree, "f find", tea.Key{Code: 'f', Text: "f"},
			func(m *Model) bool { return m.treeFilterMode }},
		{"hint on a wrapped line", true, PaneWorktrees, "a agent", tea.Key{Code: 'a', Text: "a"},
			func(m *Model) bool { return m.switching }},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			setup := func() *Model {
				m := newMouseModel(4)
				m.cursor = 1
				m.previewOn = tt.preview
				m.tmuxAvailable = tt.preview // the preview body asks the backend otherwise
				m.activePane = tt.pane
				return m
			}
			clicked, pressed := setup(), setup()
			x, y := cellOf(t, clicked, tt.click)
			if tt.preview {
				_, first := cellOf(t, clicked, "n new")
				require.Equal(t, first+1, y, "the narrow column wraps the hints")
			}

			_, _ = clicked.Update(tea.MouseClickMsg(leftClick(x+1, y)))
			_, _ = pressed.Update(tea.KeyPressMsg(tt.key))

			assert.True(t, tt.changed(pressed), "pressing the key changes the model")
			assert.Equal(t, stateOf(pressed), stateOf(clicked))
		})
	}
}

func TestHintClickMisses(t *testing.T) {
	tests := []struct {
		name  string
		click func(m *Model) (int, int)
	}{
		{"gap between hints", func(m *Model) (int, int) {
			x, y := cellOf(t, m, "? help")
			return x - 1, y
		}},
		{"past the last hint", func(m *Model) (int, int) {
			l := m.layout()
			return l.Hints.X + l.Hints.W - 1, l.Hints.Y
		}},
		{"rule above the pane hints", func(m *Model) (int, int) {
			x, y := cellOf(t, m, "n new")
			return x, y - 1
		}},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			m := newMouseModel(4)
			before := stateOf(m)
			x, y := tt.click(m)

			_, _ = m.Update(tea.MouseClickMsg(leftClick(x, y)))

			assert.Equal(t, before, stateOf(m))
		})
	}
}

// Pane hints are drawn only in the focused pane; a click where the
// unfocused pane would draw them must not press anything.
func TestHintClickNeedsFocusedPane(t *testing.T) {
	m := newMouseModel(4)
	m.activePane = PaneFileTree
	r := m.layout().Worktrees

	_, _ = m.Update(tea.MouseClickMsg(leftClick(r.X+1, r.Y+r.H-2)))

	assert.Equal(t, screen.None, m.screenMgr.Active())
	assert.Equal(t, PaneWorktrees, m.activePane)
}

func TestDashboardScrollKeepsCursorVisible(t *testing.T) {
	for _, preview := range []bool{false, true} {
		for _, filter := range []bool{false, true} {
			t.Run(fmt.Sprintf("preview=%v filter=%v", preview, filter), func(t *testing.T) {
				m := newMouseModel(30)
				m.previewOn = preview
				m.tmuxAvailable = preview
				m.filterMode = filter
				// A last commit makes the selected item three lines.
				for i := range m.worktrees {
					m.worktrees[i].Last = &models.CommitInfo{Hash: "abc1234", Subject: "work"}
				}
				r := m.layout().Worktrees
				for cursor := range m.filteredIndices {
					m.cursor = cursor
					content, rows, hints := m.dashboard(r)
					lines := strings.Split(content, "\n")

					require.LessOrEqual(t, len(lines), r.H-2, "cursor %d: content fits the pane", cursor)
					drawn := 0
					for _, item := range rows {
						if item == cursor {
							drawn++
						}
					}
					assert.Equal(t, 3, drawn, "cursor %d: all three lines drawn", cursor)
					require.NotEmpty(t, hints)
					assert.Equal(t, r.H-3, hints[len(hints)-1].Line, "cursor %d: hints end on the last line", cursor)
					assert.Contains(t, ansi.Strip(lines[len(lines)-1]), hints[len(hints)-1].Hint.Label)
				}
			})
		}
	}
}

func TestNavHintsDropWhole(t *testing.T) {
	m := newMouseModel(1)
	r := rect{W: 40, H: 1}

	line, spans := m.navHints(r)

	assert.LessOrEqual(t, ansi.StringWidth(line), r.W)
	require.NotEmpty(t, spans)
	last := spans[len(spans)-1]
	assert.Equal(t, last.Hint.Name+" "+last.Hint.Label, ansi.Strip(ansi.Cut(line, last.X0, last.X1)))
	assert.Equal(t, last.X1, ansi.StringWidth(line), "no partial hint after the last whole one")
}
