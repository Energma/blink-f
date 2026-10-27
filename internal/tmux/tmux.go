package tmux

import (
	"context"
	"fmt"
	"os"
	"os/exec"

	"github.com/Energma/blink-f/internal/config"
)

// Backends a Service can drive. Both are plain CLIs run via os/exec.
const (
	BackendTmux = "tmux"
	BackendRmx  = "rmx"
)

// Service wraps terminal multiplexer CLI operations. It speaks tmux by
// default and rmx (github.com/Energma/rust-terminal-multiplexer) when
// configured; callers stay backend-agnostic except where a feature only
// exists in tmux (popups, split panes, switch-client).
type Service struct {
	cfg       *config.TmuxConfig
	backend   string
	available bool
}

// NewService creates a Service for the configured multiplexer.
func NewService(cfg *config.Config) *Service {
	return NewServiceFor(cfg, cfg.Multiplexer)
}

// NewServiceFor creates a Service for an explicit backend. The remote
// control server uses this to stay on tmux, which it attaches to directly.
func NewServiceFor(cfg *config.Config, backend string) *Service {
	if backend == "" {
		backend = BackendTmux
	}
	_, err := exec.LookPath(backend)
	// Pass editor command to tmux config for ctrl+q e binding.
	cfg.Tmux.EditorCmd = cfg.Editor.Command
	return &Service{
		cfg:       &cfg.Tmux,
		backend:   backend,
		available: err == nil,
	}
}

// Backend returns the multiplexer name ("tmux" or "rmx").
func (s *Service) Backend() string {
	return s.backend
}

// IsAvailable returns true if the backend is installed and on PATH.
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
		return exec.Command("rmx", "attach", name)
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
