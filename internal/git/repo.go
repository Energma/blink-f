package git

import (
	"context"
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
