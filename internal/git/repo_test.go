package git

import (
	"context"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestInitRepo(t *testing.T) {
	// Pin an identity so the root commit doesn't depend on the host's git config.
	t.Setenv("GIT_AUTHOR_NAME", "test")
	t.Setenv("GIT_AUTHOR_EMAIL", "test@example.com")
	t.Setenv("GIT_COMMITTER_NAME", "test")
	t.Setenv("GIT_COMMITTER_EMAIL", "test@example.com")

	dir := t.TempDir()
	svc := NewService()
	ctx := context.Background()

	assert.False(t, svc.IsGitRepo(ctx, dir))

	root, err := svc.InitRepo(ctx, dir)
	require.NoError(t, err)
	assert.True(t, svc.IsGitRepo(ctx, dir))
	assert.Equal(t, root, mustRepoRoot(t, svc, dir))

	branch, err := svc.CurrentBranch(ctx, dir)
	require.NoError(t, err)
	assert.Equal(t, "main", branch)

	// The root commit exists, so a worktree can be created right away.
	_, err = svc.run(ctx, dir, "rev-parse", "--verify", "HEAD")
	assert.NoError(t, err)

	// A second init on the same folder (or a subfolder) must refuse rather
	// than nest a repo.
	_, err = svc.InitRepo(ctx, dir)
	assert.ErrorContains(t, err, "already a git repository")
}

func mustRepoRoot(t *testing.T, svc *Service, dir string) string {
	t.Helper()
	root, err := svc.RepoRoot(context.Background(), dir)
	require.NoError(t, err)
	return root
}
