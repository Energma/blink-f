package components

import (
	"testing"

	"github.com/energma-dev/blink/internal/models"
	"github.com/stretchr/testify/assert"
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
