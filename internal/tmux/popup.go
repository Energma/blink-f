package tmux

import (
	"context"
	"fmt"
	"os/exec"
)

// DisplayPopup opens a tmux popup running the given command.
// Only works when inside a tmux session.
func (s *Service) DisplayPopup(ctx context.Context, command, dir string) error {
	if !s.available {
		return fmt.Errorf("tmux not available")
	}
	if !InsideTmux() {
		return fmt.Errorf("tmux popup requires an active tmux session")
	}

	args := []string{
		"display-popup",
		"-w", s.cfg.PopupWidth,
		"-h", s.cfg.PopupHeight,
		"-d", dir,
		"-E", command,
	}
	return exec.CommandContext(ctx, "tmux", args...).Run()
}

// SendKeys sends key sequences to a tmux session.
func (s *Service) SendKeys(ctx context.Context, session, keys string) error {
	return exec.CommandContext(ctx, "tmux", "send-keys", "-t", session, keys, "Enter").Run()
}
