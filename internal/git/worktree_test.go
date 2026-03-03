package git

import (
	"testing"

	"github.com/stretchr/testify/assert"
)

func TestParseWorktreeList(t *testing.T) {
	input := `worktree /home/user/myrepo
HEAD abc123def456
branch refs/heads/main

worktree /home/user/myrepo/.worktrees/feature-login
HEAD 789def012345
branch refs/heads/feature/login

worktree /home/user/myrepo/.worktrees/hotfix
HEAD deadbeef1234
detached

`
	wts := parseWorktreeList(input, "/home/user/myrepo")

	assert.Len(t, wts, 3)

	// Main worktree
	assert.Equal(t, "/home/user/myrepo", wts[0].Path)
	assert.Equal(t, "main", wts[0].Branch)
	assert.Equal(t, "abc123def456", wts[0].HEAD)
	assert.True(t, wts[0].IsMain)
	assert.False(t, wts[0].Detached)

	// Feature worktree
	assert.Equal(t, "/home/user/myrepo/.worktrees/feature-login", wts[1].Path)
	assert.Equal(t, "feature/login", wts[1].Branch)
	assert.False(t, wts[1].IsMain)

	// Detached worktree
	assert.Equal(t, "/home/user/myrepo/.worktrees/hotfix", wts[2].Path)
	assert.True(t, wts[2].Detached)
	assert.Equal(t, "", wts[2].Branch)
}

func TestParseWorktreeListEmpty(t *testing.T) {
	wts := parseWorktreeList("", "/home/user/myrepo")
	assert.Empty(t, wts)
}

func TestParseWorktreeListNoTrailingNewline(t *testing.T) {
	input := `worktree /home/user/myrepo
HEAD abc123
branch refs/heads/main`

	wts := parseWorktreeList(input, "/home/user/myrepo")
	assert.Len(t, wts, 1)
	assert.Equal(t, "main", wts[0].Branch)
}
