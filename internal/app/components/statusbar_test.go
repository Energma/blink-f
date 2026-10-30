package components

import (
	"strings"
	"testing"

	tea "charm.land/bubbletea/v2"
	"github.com/charmbracelet/x/ansi"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/Energma/blink-f/internal/app/screen"
	"github.com/Energma/blink-f/internal/models"
	"github.com/Energma/blink-f/internal/theme"
)

func TestComputeStatsEmpty(t *testing.T) {
	stats := ComputeStats(nil)
	assert.Equal(t, 0, stats.Worktrees)
	assert.Equal(t, 0, stats.Sessions)
	assert.Equal(t, 0, stats.Dirty)
	assert.Equal(t, 0, stats.Clean)
	assert.Equal(t, 0, stats.Conflicts)
	assert.Equal(t, 0, stats.Agents)
}

func TestComputeStats(t *testing.T) {
	tests := []struct {
		name      string
		worktrees []models.Worktree
		want      Stats
	}{
		{
			"all clean",
			[]models.Worktree{
				{Status: models.WorktreeStatus{Clean: true}},
				{Status: models.WorktreeStatus{Clean: true}},
				{Status: models.WorktreeStatus{Clean: true}},
			},
			Stats{Worktrees: 3, Clean: 3},
		},
		{
			"mixed dirty",
			[]models.Worktree{
				{Status: models.WorktreeStatus{Clean: true}},
				{Status: models.WorktreeStatus{Modified: 2}},
				{Status: models.WorktreeStatus{Staged: 1}},
				{Status: models.WorktreeStatus{Untracked: 3}},
			},
			Stats{Worktrees: 4, Clean: 1, Dirty: 3},
		},
		{
			"conflicts counted separately",
			[]models.Worktree{
				{Status: models.WorktreeStatus{Conflicts: 1, Modified: 2}},
				{Status: models.WorktreeStatus{Clean: true}},
			},
			Stats{Worktrees: 2, Clean: 1, Conflicts: 1},
		},
		{
			"sessions and agents",
			[]models.Worktree{
				{Status: models.WorktreeStatus{Clean: true, TmuxSession: "sess-1"}},
				{Status: models.WorktreeStatus{Clean: true, AgentRunning: true}},
				{Status: models.WorktreeStatus{Clean: true, TmuxSession: "sess-2", AgentRunning: true}},
			},
			Stats{Worktrees: 3, Clean: 3, Sessions: 2, Agents: 2},
		},
		{
			"staged counts as dirty",
			[]models.Worktree{
				{Status: models.WorktreeStatus{Staged: 5}},
			},
			Stats{Worktrees: 1, Dirty: 1},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := ComputeStats(tt.worktrees)
			assert.Equal(t, tt.want, got)
		})
	}
}

func TestRenderHints(t *testing.T) {
	th := theme.Default()
	groups := [][]Hint{
		{charHint('n', "new"), keyHint("C-down", "browse", tea.KeyDown, tea.ModCtrl)},
		{textHint("↑/↓", "move"), charHint('q', "quit")},
	}

	tests := []struct {
		name      string
		width     int
		wantLines int
	}{
		{"unbounded is one line", 0, 1},
		{"wide enough for all", 80, 1},
		{"wraps at a group", 20, 2},
		{"wraps inside a group", 15, 4},
		{"narrower than a hint keeps one per line", 3, 4},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			out, spans := RenderHints(th, tt.width, groups...)
			lines := strings.Split(out, "\n")

			assert.Len(t, lines, tt.wantLines)
			require.Len(t, spans, 4)
			for i, s := range spans {
				// The span covers exactly the hint's cells, measured past
				// the styling escapes and the wide arrow glyphs.
				got := ansi.Strip(ansi.Cut(lines[s.Line], s.X0, s.X1))
				assert.Equal(t, s.Hint.Name+" "+s.Hint.Label, got, "span %d", i)
				if i > 0 && spans[i-1].Line != s.Line {
					assert.Zero(t, s.X0, "a wrapped hint starts its line")
				}
			}
			perLine := make([]int, len(lines))
			for _, s := range spans {
				perLine[s.Line]++
			}
			for i, line := range lines {
				if tt.width > 0 && ansi.StringWidth(line) > tt.width {
					// Only a hint that alone is wider than the line may overflow.
					assert.Equal(t, 1, perLine[i], "line %q", ansi.Strip(line))
				}
			}
		})
	}
}

func TestRenderHintsSeparatesGroups(t *testing.T) {
	out, _ := RenderHints(theme.Default(), 0, []Hint{charHint('a', "x"), charHint('b', "y")}, []Hint{charHint('c', "z")})
	assert.Equal(t, "a x  b y | c z", ansi.Strip(out))
}

// Clicks press a hint's Key, so each must read as the key string the
// handlers match on.
func TestHintKeys(t *testing.T) {
	keys := map[string]string{}
	collect := func(hints ...Hint) {
		for _, h := range hints {
			if h.Key.Code != 0 {
				keys[h.Name] = h.Key.String()
			}
		}
	}
	for _, g := range PaneHints(PaneWorktrees) {
		collect(g...)
	}
	for _, g := range PaneHints(PaneFileTree) {
		collect(g...)
	}
	collect(NavHints(screen.None, PaneWorktrees)...)
	collect(NavHints(screen.None, PaneFileTree)...)
	collect(NavHints(screen.WorktreeCreate, PaneWorktrees)...)

	tests := []struct{ name, want string }{
		{"n", "n"}, {"D", "D"}, {"?", "?"}, {"/", "/"}, {"v", "v"},
		{"enter", "enter"}, {"enter/l", "enter"}, {"bs", "backspace"}, {"tab", "tab"},
		{"C-down", "ctrl+down"}, {"C-up", "ctrl+up"}, {"C-b", "ctrl+b"}, {"esc", "esc"},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			assert.Equal(t, tt.want, keys[tt.name])
		})
	}
}
