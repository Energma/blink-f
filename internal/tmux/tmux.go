package tmux

import (
	"context"
	"fmt"
	"os"
	"os/exec"

	"github.com/Energma/blink-f/internal/config"
	"github.com/Energma/blink-f/internal/rmxbin"
)

// Backends a Service can drive. Both are plain CLIs run via os/exec.
const (
	BackendTmux = "tmux"
	BackendRmx  = "rmx"
)

// Service wraps terminal multiplexer CLI operations. It speaks rmx
// (github.com/Energma/rust-terminal-multiplexer, bundled in the blink
// binary) by default and tmux when configured; callers stay
// backend-agnostic except where a feature only exists in tmux (popups,
// split panes, switch-client).
type Service struct {
	cfg       *config.TmuxConfig
	backend   string
	bin       string // executable to run; empty when unavailable
	available bool
}

// NewService creates a Service for the configured multiplexer.
func NewService(cfg *config.Config) *Service {
	backend := cfg.Multiplexer
	if backend == "" {
		backend = BackendRmx
	}
	var bin string
	var err error
	if backend == BackendRmx {
		bin, err = rmxbin.Path()
	} else {
		bin, err = exec.LookPath("tmux")
	}
	// Pass editor command to tmux config for ctrl+q e binding.
	cfg.Tmux.EditorCmd = cfg.Editor.Command
	return &Service{
		cfg:       &cfg.Tmux,
		backend:   backend,
		bin:       bin,
		available: err == nil,
	}
}

// Backend returns the multiplexer name ("tmux" or "rmx").
func (s *Service) Backend() string {
	return s.backend
}

// IsAvailable returns true if the backend can be run: rmx is bundled or on
// PATH, tmux is on PATH.
func (s *Service) IsAvailable() bool {
	return s.available
}

// Inside returns true if this process runs inside a pane of the backend.
func (s *Service) Inside() bool {
	if s.backend == BackendRmx {
		return insideRmx()
	}
	return InsideTmux()
}

// InsideTmux returns true if the current process is inside a tmux session.
func InsideTmux() bool {
	return os.Getenv("TMUX") != ""
}

// AttachCmd returns a command that attaches the terminal to a session. The
// caller runs it with tea.ExecProcess so the TUI is suspended meanwhile.
func (s *Service) AttachCmd(name string) *exec.Cmd {
	if s.backend == BackendRmx {
		return exec.Command(s.bin, "attach", name)
	}
	return exec.Command("tmux", "attach-session", "-t", name)
}

// SwitchClient moves the current client to another session without
// suspending anything. Only tmux can do this; rmx refuses to attach from
// inside one of its own panes and has no switch command.
func (s *Service) SwitchClient(ctx context.Context, name string) error {
	if s.backend == BackendRmx {
		return fmt.Errorf("rmx can't switch workspaces from inside rmx: open %q with Leader s, or run blink outside rmx", name)
	}
	if err := exec.CommandContext(ctx, "tmux", "switch-client", "-t", name).Run(); err != nil {
		return fmt.Errorf("switch to %s: %w", name, err)
	}
	return nil
}
