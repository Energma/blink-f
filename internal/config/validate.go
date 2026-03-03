package config

import (
	"fmt"
	"os"
)

func Validate(cfg *Config) error {
	// Validate repo paths exist
	for i, r := range cfg.Repos {
		expanded := expandHome(r.Path)
		cfg.Repos[i].Path = expanded
		if info, err := os.Stat(expanded); err != nil || !info.IsDir() {
			// Don't fail — just skip invalid repos at runtime
			continue
		}
	}

	// Validate tmux popup dimensions
	if cfg.Tmux.PopupWidth == "" {
		cfg.Tmux.PopupWidth = "80%"
	}
	if cfg.Tmux.PopupHeight == "" {
		cfg.Tmux.PopupHeight = "80%"
	}

	// Validate default agent exists in providers
	if cfg.Agents.Default != "" {
		if _, ok := cfg.Agents.Providers[cfg.Agents.Default]; !ok {
			return fmt.Errorf("default agent %q not found in providers", cfg.Agents.Default)
		}
	}

	return nil
}

func expandHome(path string) string {
	if len(path) < 2 || path[0] != '~' {
		return path
	}
	home, err := os.UserHomeDir()
	if err != nil {
		return path
	}
	return home + path[1:]
}
