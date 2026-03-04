package git

import (
	"bytes"
	"context"
	"fmt"
	"os/exec"
	"runtime"
	"strings"
	"time"
)

// Service provides semaphore-bounded git CLI operations.
type Service struct {
	sem     chan struct{}
	timeout time.Duration
}

func NewService() *Service {
	limit := runtime.NumCPU() * 2
	if limit < 4 {
		limit = 4
	}
	if limit > 32 {
		limit = 32
	}
	return &Service{
		sem:     make(chan struct{}, limit),
		timeout: 30 * time.Second,
	}
}

// run executes a git command in dir, bounded by semaphore.
func (s *Service) run(ctx context.Context, dir string, args ...string) (string, error) {
	// Apply timeout covering both semaphore wait and command execution.
	if _, hasDeadline := ctx.Deadline(); !hasDeadline {
		var cancel context.CancelFunc
		ctx, cancel = context.WithTimeout(ctx, s.timeout)
		defer cancel()
	}

	// Acquire semaphore, respecting context cancellation.
	select {
	case s.sem <- struct{}{}:
	case <-ctx.Done():
		return "", fmt.Errorf("git %s: %w", strings.Join(args, " "), ctx.Err())
	}
	defer func() { <-s.sem }()

	cmd := exec.CommandContext(ctx, "git", args...)
	if dir != "" {
		cmd.Dir = dir
	}

	var stdout, stderr bytes.Buffer
	cmd.Stdout = &stdout
	cmd.Stderr = &stderr

	if err := cmd.Run(); err != nil {
		errMsg := strings.TrimSpace(stderr.String())
		if errMsg == "" {
			errMsg = err.Error()
		}
		return "", fmt.Errorf("git %s: %s", strings.Join(args, " "), errMsg)
	}

	return strings.TrimSpace(stdout.String()), nil
}

// runLines runs a git command and splits output into lines.
func (s *Service) runLines(ctx context.Context, dir string, args ...string) ([]string, error) {
	out, err := s.run(ctx, dir, args...)
	if err != nil {
		return nil, err
	}
	if out == "" {
		return nil, nil
	}
	return strings.Split(out, "\n"), nil
}
