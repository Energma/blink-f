package git

import (
	"context"
	"fmt"
	"path/filepath"
	"strings"

	"github.com/energma-dev/blink/internal/models"
)

// ListWorktrees returns all worktrees for the repo at repoDir.
func (s *Service) ListWorktrees(ctx context.Context, repoDir string) ([]models.Worktree, error) {
	output, err := s.run(ctx, repoDir, "worktree", "list", "--porcelain")
	if err != nil {
		return nil, fmt.Errorf("list worktrees: %w", err)
	}
	return parseWorktreeList(output, repoDir), nil
}

// CreateWorktree creates a new worktree with a new branch.
func (s *Service) CreateWorktree(ctx context.Context, repoDir, branch, baseBranch, baseDir string) (*models.Worktree, error) {
	if baseDir == "" {
		baseDir = ".worktrees"
	}
	worktreeDir := filepath.Join(repoDir, baseDir, branch)

	args := []string{"worktree", "add", "-b", branch, worktreeDir}
	if baseBranch != "" {
		args = append(args, baseBranch)
	}

	if _, err := s.run(ctx, repoDir, args...); err != nil {
		// Branch might already exist — try without -b
		args = []string{"worktree", "add", worktreeDir, branch}
		if _, err := s.run(ctx, repoDir, args...); err != nil {
			return nil, fmt.Errorf("create worktree %q: %w", branch, err)
		}
	}

	return &models.Worktree{
		Path:    worktreeDir,
		Branch:  branch,
		RepoDir: repoDir,
	}, nil
}

// RemoveWorktree removes a worktree by path.
func (s *Service) RemoveWorktree(ctx context.Context, repoDir, worktreePath string, force bool) error {
	args := []string{"worktree", "remove", worktreePath}
	if force {
		args = append(args, "--force")
	}
	_, err := s.run(ctx, repoDir, args...)
	if err != nil {
		return fmt.Errorf("remove worktree: %w", err)
	}
	return nil
}

// PruneWorktrees cleans up stale worktree metadata.
func (s *Service) PruneWorktrees(ctx context.Context, repoDir string) error {
	_, err := s.run(ctx, repoDir, "worktree", "prune")
	return err
}

// IsBranchMerged checks if branch is merged into target.
func (s *Service) IsBranchMerged(ctx context.Context, repoDir, branch, target string) bool {
	out, err := s.run(ctx, repoDir, "branch", "--merged", target)
	if err != nil {
		return false
	}
	for _, line := range strings.Split(out, "\n") {
		name := strings.TrimSpace(strings.TrimPrefix(line, "* "))
		if name == branch {
			return true
		}
	}
	return false
}

func parseWorktreeList(output, repoDir string) []models.Worktree {
	var worktrees []models.Worktree
	var current models.Worktree

	for _, line := range strings.Split(output, "\n") {
		line = strings.TrimSpace(line)
		switch {
		case strings.HasPrefix(line, "worktree "):
			current.Path = strings.TrimPrefix(line, "worktree ")
		case strings.HasPrefix(line, "HEAD "):
			current.HEAD = strings.TrimPrefix(line, "HEAD ")
		case strings.HasPrefix(line, "branch "):
			ref := strings.TrimPrefix(line, "branch ")
			current.Branch = strings.TrimPrefix(ref, "refs/heads/")
		case line == "bare":
			current.Bare = true
		case line == "detached":
			current.Detached = true
		case line == "":
			if current.Path != "" {
				current.RepoDir = repoDir
				current.IsMain = current.Path == repoDir
				worktrees = append(worktrees, current)
				current = models.Worktree{}
			}
		}
	}
	// Handle last entry if no trailing newline
	if current.Path != "" {
		current.RepoDir = repoDir
		current.IsMain = current.Path == repoDir
		worktrees = append(worktrees, current)
	}
	return worktrees
}
