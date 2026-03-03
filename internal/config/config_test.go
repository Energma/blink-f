package config

import (
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestDefault(t *testing.T) {
	cfg := Default()

	assert.Equal(t, "claude", cfg.Agents.Default)
	assert.Contains(t, cfg.Agents.Providers, "claude")
	assert.Contains(t, cfg.Agents.Providers, "opencode")
	assert.Contains(t, cfg.Agents.Providers, "aider")

	assert.True(t, cfg.Tmux.AutoSession)
	assert.Equal(t, "80%", cfg.Tmux.PopupWidth)
	assert.Equal(t, "zsh", cfg.Tmux.Shell)

	assert.Equal(t, ".worktrees", cfg.Worktree.BaseDir)
	assert.Contains(t, cfg.Worktree.AutoSymlinks, ".env")
	assert.Contains(t, cfg.Worktree.AutoSymlinks, "node_modules")

	assert.True(t, cfg.Git.ConventionalCommits)
	assert.False(t, cfg.Git.AutoPush)

	assert.Equal(t, "n", cfg.Keys.NewWorktree)
	assert.Equal(t, "q", cfg.Keys.Quit)
}

func TestValidateDefaultConfig(t *testing.T) {
	cfg := Default()
	err := Validate(cfg)
	require.NoError(t, err)
}

func TestValidateBadDefaultAgent(t *testing.T) {
	cfg := Default()
	cfg.Agents.Default = "nonexistent"
	err := Validate(cfg)
	assert.Error(t, err)
	assert.Contains(t, err.Error(), "nonexistent")
}

func TestExpandHome(t *testing.T) {
	assert.Equal(t, "/absolute/path", expandHome("/absolute/path"))
	assert.Equal(t, "relative", expandHome("relative"))

	expanded := expandHome("~/test")
	assert.NotEqual(t, "~/test", expanded)
	assert.Contains(t, expanded, "test")
}
