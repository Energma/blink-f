package app

import (
	"testing"

	"github.com/stretchr/testify/assert"

	"github.com/Energma/blink-f/internal/models"
)

func launchWorktrees() []models.Worktree {
	return []models.Worktree{
		{Path: "/src/repo", Branch: "main", RepoDir: "/src/repo", IsMain: true},
		{Path: "/src/repo-feat-x", Branch: "feat/x", RepoDir: "/src/repo"},
		{Path: "/src/repo-feat-y", Branch: "feat/y", RepoDir: "/src/repo"},
	}
}

func TestFirstLoadSelectsLaunchWorktree(t *testing.T) {
	tests := []struct {
		name   string
		launch string
		want   int
	}{
		{"main worktree", "/src/repo", 0},
		{"linked worktree", "/src/repo-feat-x", 1},
		{"last worktree", "/src/repo-feat-y", 2},
		{"not a listed worktree", "/elsewhere", 0},
		{"launched outside a repo", "", 0},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			m := &Model{launchWorktree: tt.launch}
			_, _ = m.update(worktreesLoadedMsg{worktrees: launchWorktrees()})
			assert.Equal(t, tt.want, m.cursor)
			assert.Empty(t, m.launchWorktree)
		})
	}
}

func TestReloadKeepsUserSelection(t *testing.T) {
	m := &Model{launchWorktree: "/src/repo-feat-x"}
	_, _ = m.update(worktreesLoadedMsg{worktrees: launchWorktrees()})
	m.cursor = 2

	_, _ = m.update(worktreesLoadedMsg{worktrees: launchWorktrees()})
	assert.Equal(t, 2, m.cursor)
}
