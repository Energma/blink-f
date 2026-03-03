package models

import (
	"testing"

	"github.com/stretchr/testify/assert"
)

func TestWorktreeDisplayName(t *testing.T) {
	tests := []struct {
		name string
		wt   Worktree
		want string
	}{
		{"branch", Worktree{Branch: "feature/login"}, "feature/login"},
		{"detached short", Worktree{Detached: true, HEAD: "abc12345"}, "abc12345"},
		{"detached long", Worktree{Detached: true, HEAD: "abc123456789abcd"}, "abc12345"},
		{"path fallback", Worktree{Path: "/home/user/repo"}, "/home/user/repo"},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			assert.Equal(t, tt.want, tt.wt.DisplayName())
		})
	}
}

func TestStatusLine(t *testing.T) {
	tests := []struct {
		name   string
		status WorktreeStatus
		want   string
	}{
		{"clean", WorktreeStatus{Clean: true}, "clean"},
		{"staged only", WorktreeStatus{Staged: 3}, "+3"},
		{"modified only", WorktreeStatus{Modified: 2}, "~2"},
		{"untracked only", WorktreeStatus{Untracked: 1}, "?1"},
		{"mixed", WorktreeStatus{Staged: 1, Modified: 2, Ahead: 3}, "+1 ~2 ↑3"},
		{"conflicts", WorktreeStatus{Conflicts: 2, Modified: 1}, "~1 !2"},
		{"ahead behind", WorktreeStatus{Ahead: 5, Behind: 2}, "↑5 ↓2"},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			assert.Equal(t, tt.want, tt.status.StatusLine())
		})
	}
}
