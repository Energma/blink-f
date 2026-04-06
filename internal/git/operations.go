package git

import (
	"context"
	"fmt"
	"strings"
	"time"

	"github.com/Energma/blink-f/internal/models"
)

// Commit stages all changes and commits with the given message.
func (s *Service) Commit(ctx context.Context, dir, message string) (string, error) {
	// Stage all
	if _, err := s.run(ctx, dir, "add", "-A"); err != nil {
		return "", fmt.Errorf("stage: %w", err)
	}
	// Commit
	out, err := s.run(ctx, dir, "commit", "-m", message)
	if err != nil {
		return "", fmt.Errorf("commit: %w", err)
	}
	return out, nil
}

// Push pushes the current branch to origin.
func (s *Service) Push(ctx context.Context, dir string) error {
	branch, err := s.CurrentBranch(ctx, dir)
	if err != nil {
		return fmt.Errorf("get branch: %w", err)
	}
	_, err = s.run(ctx, dir, "push", "-u", "origin", branch)
	if err != nil {
		return fmt.Errorf("push: %w", err)
	}
	return nil
}

// Pull pulls from the upstream branch.
func (s *Service) Pull(ctx context.Context, dir string) error {
	_, err := s.run(ctx, dir, "pull")
	if err != nil {
		return fmt.Errorf("pull: %w", err)
	}
	return nil
}

// Stash stashes all changes.
func (s *Service) Stash(ctx context.Context, dir string) error {
	_, err := s.run(ctx, dir, "stash", "push", "-m",
		fmt.Sprintf("blink auto-stash %s", time.Now().Format("2006-01-02 15:04")))
	return err
}

// StashPop pops the latest stash.
func (s *Service) StashPop(ctx context.Context, dir string) error {
	_, err := s.run(ctx, dir, "stash", "pop")
	return err
}

// BranchList returns local branch names.
func (s *Service) BranchList(ctx context.Context, dir string) ([]string, error) {
	lines, err := s.runLines(ctx, dir, "branch", "--format=%(refname:short)")
	if err != nil {
		return nil, err
	}
	return lines, nil
}

// CheckoutBranch switches the working tree at dir to the given branch.
func (s *Service) CheckoutBranch(ctx context.Context, dir, branch string) error {
	_, err := s.run(ctx, dir, "checkout", branch)
	if err != nil {
		return fmt.Errorf("checkout %s: %w", branch, err)
	}
	return nil
}

// DiffStat returns a compact diff stat for the working directory.
func (s *Service) DiffStat(ctx context.Context, dir string) (string, error) {
	return s.run(ctx, dir, "diff", "--stat")
}

// FilesChanged returns the list of changed files.
func (s *Service) FilesChanged(ctx context.Context, dir string) ([]string, error) {
	out, err := s.run(ctx, dir, "diff", "--name-only")
	if err != nil {
		return nil, err
	}
	staged, err := s.run(ctx, dir, "diff", "--cached", "--name-only")
	if err != nil {
		return nil, err
	}
	files := make(map[string]bool)
	for _, f := range strings.Split(out, "\n") {
		if f = strings.TrimSpace(f); f != "" {
			files[f] = true
		}
	}
	for _, f := range strings.Split(staged, "\n") {
		if f = strings.TrimSpace(f); f != "" {
			files[f] = true
		}
	}
	result := make([]string, 0, len(files))
	for f := range files {
		result = append(result, f)
	}
	return result, nil
}

// parseTime parses an ISO 8601 date string.
func parseTime(s string) (time.Time, error) {
	return time.Parse(time.RFC3339, s)
}

// FormatConventionalCommit builds a conventional commit message.
func FormatConventionalCommit(typ, scope, description string) string {
	if scope != "" {
		return fmt.Sprintf("%s(%s): %s", typ, scope, description)
	}
	return fmt.Sprintf("%s: %s", typ, description)
}

// ConventionalTypes returns the supported conventional commit types.
func ConventionalTypes() []string {
	return []string{"feat", "fix", "refactor", "chore", "test", "docs", "style", "perf", "ci", "build"}
}

// LastCommitInfo returns structured commit info (proper time.Time version).
func (s *Service) LastCommitInfo(ctx context.Context, dir string) (*models.CommitInfo, error) {
	out, err := s.run(ctx, dir, "log", "-1", "--format=%H%n%s%n%an%n%aI")
	if err != nil {
		return nil, err
	}
	lines := strings.SplitN(out, "\n", 4)
	if len(lines) < 4 {
		return nil, nil
	}
	ci := &models.CommitInfo{
		Hash:    lines[0],
		Subject: lines[1],
		Author:  lines[2],
	}
	ci.Date, _ = parseTime(lines[3])
	return ci, nil
}
