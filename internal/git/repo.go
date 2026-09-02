package git

import (
	"context"
	"fmt"
	"path/filepath"
)

// RepoRoot returns the root directory of the git repo containing dir.
func (s *Service) RepoRoot(ctx context.Context, dir string) (string, error) {
	return s.run(ctx, dir, "rev-parse", "--show-toplevel")
}

// CurrentBranch returns the current branch name.
func (s *Service) CurrentBranch(ctx context.Context, dir string) (string, error) {
	return s.run(ctx, dir, "rev-parse", "--abbrev-ref", "HEAD")
}

// DefaultBranch tries to detect the default branch (main/master).
func (s *Service) DefaultBranch(ctx context.Context, dir string) string {
	// Try origin/HEAD first
	ref, err := s.run(ctx, dir, "symbolic-ref", "refs/remotes/origin/HEAD")
	if err == nil && ref != "" {
		return filepath.Base(ref)
	}
	// Fallback: check if main exists
	if _, err := s.run(ctx, dir, "rev-parse", "--verify", "main"); err == nil {
		return "main"
	}
	return "master"
}

// IsGitRepo checks if dir is inside a git repository.
func (s *Service) IsGitRepo(ctx context.Context, dir string) bool {
	_, err := s.run(ctx, dir, "rev-parse", "--git-dir")
	return err == nil
}

// RemoteURL returns the URL of the origin remote.
func (s *Service) RemoteURL(ctx context.Context, dir string) (string, error) {
	return s.run(ctx, dir, "remote", "get-url", "origin")
}

// InitRepo turns dir into a fresh git repository on branch "main" with an
// empty root commit, so worktrees can be created immediately (git worktree add
// needs at least one commit). Existing files are left unstaged on purpose: a
// remote-driven init must never sweep secrets into history. Refuses when dir
// is already inside a repository so it can't create a nested one by accident.
func (s *Service) InitRepo(ctx context.Context, dir string) (string, error) {
	if s.IsGitRepo(ctx, dir) {
		return "", fmt.Errorf("already a git repository: %s", dir)
	}
	if _, err := s.run(ctx, dir, "init"); err != nil {
		return "", err
	}
	if _, err := s.run(ctx, dir, "symbolic-ref", "HEAD", "refs/heads/main"); err != nil {
		return "", err
	}
	if _, err := s.run(ctx, dir, "commit", "--allow-empty", "-m", "chore: initialize repository"); err != nil {
		return "", fmt.Errorf("initial commit: %w", err)
	}
	return s.RepoRoot(ctx, dir)
}
