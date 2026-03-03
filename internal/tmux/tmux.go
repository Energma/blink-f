package tmux

import (
	"os"
	"os/exec"

	"github.com/Energma/blink-f/internal/config"
)

// Service wraps tmux CLI operations.
type Service struct {
	cfg       *config.TmuxConfig
	available bool
}

// NewService creates a tmux Service, detecting availability.
func NewService(cfg *config.Config) *Service {
	_, err := exec.LookPath("tmux")
	return &Service{
		cfg:       &cfg.Tmux,
		available: err == nil,
	}
}

// IsAvailable returns true if tmux is installed and on PATH.
func (s *Service) IsAvailable() bool {
	return s.available
}

// InsideTmux returns true if the current process is inside a tmux session.
func InsideTmux() bool {
	return os.Getenv("TMUX") != ""
}
