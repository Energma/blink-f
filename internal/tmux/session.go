package tmux

import (
	"context"
	"fmt"
	"os"
	"os/exec"
	"strings"
)

// SessionExists checks if a named tmux session exists.
func (s *Service) SessionExists(name string) bool {
	cmd := exec.Command("tmux", "has-session", "-t", name)
	return cmd.Run() == nil
}

// CreateSession creates a new detached tmux session in dir.
func (s *Service) CreateSession(ctx context.Context, name, dir string) error {
	if !s.available {
		return fmt.Errorf("tmux not available")
	}
	args := []string{"new-session", "-d", "-s", name, "-c", dir}
	if s.cfg.Shell != "" {
		args = append(args, s.cfg.Shell)
	}
	cmd := exec.CommandContext(ctx, "tmux", args...)
	cmd.Env = os.Environ()
	return cmd.Run()
}

// SwitchSession switches to an existing tmux session.
// Uses switch-client inside tmux, attach-session outside.
func (s *Service) SwitchSession(ctx context.Context, name string) error {
	if !s.available {
		return fmt.Errorf("tmux not available")
	}
	if InsideTmux() {
		return exec.CommandContext(ctx, "tmux", "switch-client", "-t", name).Run()
	}
	cmd := exec.CommandContext(ctx, "tmux", "attach-session", "-t", name)
	cmd.Stdin = os.Stdin
	cmd.Stdout = os.Stdout
	cmd.Stderr = os.Stderr
	return cmd.Run()
}

// KillSession kills a named tmux session.
func (s *Service) KillSession(ctx context.Context, name string) error {
	return exec.CommandContext(ctx, "tmux", "kill-session", "-t", name).Run()
}

// ListSessions returns names of running tmux sessions.
func (s *Service) ListSessions(ctx context.Context) ([]string, error) {
	if !s.available {
		return nil, nil
	}
	cmd := exec.CommandContext(ctx, "tmux", "list-sessions", "-F", "#{session_name}")
	out, err := cmd.Output()
	if err != nil {
		return nil, nil // No sessions or tmux not running
	}
	raw := strings.TrimSpace(string(out))
	if raw == "" {
		return nil, nil
	}
	return strings.Split(raw, "\n"), nil
}

// SessionNameForWorktree generates a tmux-safe session name.
func SessionNameForWorktree(repoName, branch string) string {
	name := fmt.Sprintf("%s:%s", repoName, branch)
	// tmux doesn't allow dots or colons (well, some versions do)
	// Replace problematic chars
	name = strings.ReplaceAll(name, "/", "-")
	name = strings.ReplaceAll(name, ".", "_")
	return name
}

// EnsureSession creates a session if it doesn't exist, returns the name.
func (s *Service) EnsureSession(ctx context.Context, repoName, branch, dir string) (string, error) {
	name := SessionNameForWorktree(repoName, branch)
	if s.SessionExists(name) {
		return name, nil
	}
	if err := s.CreateSession(ctx, name, dir); err != nil {
		return "", fmt.Errorf("create session %q: %w", name, err)
	}
	return name, nil
}
