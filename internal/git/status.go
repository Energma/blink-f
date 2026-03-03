package git

import (
	"context"
	"strconv"
	"strings"

	"github.com/energma-dev/blink/internal/models"
)

// Status returns the worktree status for the given directory.
func (s *Service) Status(ctx context.Context, dir string) (models.WorktreeStatus, error) {
	var st models.WorktreeStatus

	lines, err := s.runLines(ctx, dir, "status", "--porcelain=v1")
	if err != nil {
		return st, err
	}

	if len(lines) == 0 {
		st.Clean = true
		return st, nil
	}

	for _, line := range lines {
		if len(line) < 2 {
			continue
		}
		x, y := line[0], line[1]

		switch {
		case x == 'U' || y == 'U' || (x == 'A' && y == 'A') || (x == 'D' && y == 'D'):
			st.Conflicts++
		case x == '?' && y == '?':
			st.Untracked++
		default:
			if x != ' ' && x != '?' {
				st.Staged++
			}
			if y != ' ' && y != '?' {
				st.Modified++
			}
		}
	}

	st.Clean = st.Staged == 0 && st.Modified == 0 && st.Untracked == 0 && st.Conflicts == 0

	// Ahead/behind
	ab, err := s.run(ctx, dir, "rev-list", "--left-right", "--count", "HEAD...@{upstream}")
	if err == nil {
		parts := strings.Fields(ab)
		if len(parts) == 2 {
			st.Ahead, _ = strconv.Atoi(parts[0])
			st.Behind, _ = strconv.Atoi(parts[1])
		}
	}

	return st, nil
}

// LastCommit returns the last commit info for a directory.
func (s *Service) LastCommit(ctx context.Context, dir string) (*models.CommitInfo, error) {
	return s.LastCommitInfo(ctx, dir)
}
