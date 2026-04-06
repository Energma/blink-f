package config

import (
	"fmt"
	"os"
	"path/filepath"

	"gopkg.in/yaml.v3"
)

type Config struct {
	Repos    []RepoConfig   `yaml:"repos"`
	Agents   AgentConfig    `yaml:"agents"`
	Editor   EditorConfig   `yaml:"editor"`
	Tmux     TmuxConfig     `yaml:"tmux"`
	Worktree WorktreeConfig `yaml:"worktree"`
	Git      GitConfig      `yaml:"git"`
	UI       UIConfig       `yaml:"ui"`
	Keys     KeyConfig      `yaml:"keybindings"`
}

type RepoConfig struct {
	Path          string `yaml:"path"`
	DefaultBranch string `yaml:"default_branch"`
	Name          string `yaml:"name"`
}

type AgentConfig struct {
	Default   string                    `yaml:"default"`
	Providers map[string]ProviderConfig `yaml:"providers"`
}

type ProviderConfig struct {
	Command string   `yaml:"command"`
	Args    []string `yaml:"args"`
}

type TmuxConfig struct {
	AutoSession bool   `yaml:"auto_session"`
	PopupWidth  string `yaml:"popup_width"`
	PopupHeight string `yaml:"popup_height"`
	Shell       string `yaml:"shell"`
	EditorCmd   string `yaml:"-"` // set at runtime from Editor.Command
}

type WorktreeConfig struct {
	BaseDir      string   `yaml:"base_dir"`
	AutoSymlinks []string `yaml:"auto_symlinks"`
	CleanMerged  bool     `yaml:"cleanup_merged"`
}

type GitConfig struct {
	ConventionalCommits bool `yaml:"conventional_commits"`
	AutoPush            bool `yaml:"auto_push"`
	SignCommits         bool `yaml:"sign_commits"`
}

type EditorConfig struct {
	Command string   `yaml:"command"`
	Args    []string `yaml:"args"`
}

type UIConfig struct {
	Theme     string `yaml:"theme"`
	ShowIcons bool   `yaml:"show_icons"`
}

type KeyConfig struct {
	NewWorktree    string `yaml:"new_worktree"`
	DeleteWorktree string `yaml:"delete_worktree"`
	SwitchWorktree string `yaml:"switch_worktree"`
	LaunchAgent    string `yaml:"launch_agent"`
	Commit         string `yaml:"commit"`
	Push           string `yaml:"push"`
	Pull           string `yaml:"pull"`
	Stash          string `yaml:"stash"`
	Help           string `yaml:"help"`
	Quit           string `yaml:"quit"`
	Refresh        string `yaml:"refresh"`
	Filter         string `yaml:"filter"`
	RepoSwitch     string `yaml:"repo_switch"`
}

func Default() *Config {
	return &Config{
		Agents: AgentConfig{
			Default: "claude",
			Providers: map[string]ProviderConfig{
				"claude":   {Command: "claude", Args: []string{}},
				"opencode": {Command: "opencode", Args: []string{}},
				"aider":    {Command: "aider", Args: []string{}},
			},
		},
		Editor: EditorConfig{
			Command: "code",
			Args:    []string{"."},
		},
		Tmux: TmuxConfig{
			AutoSession: true,
			PopupWidth:  "80%",
			PopupHeight: "80%",
			Shell:       "zsh",
		},
		Worktree: WorktreeConfig{
			BaseDir:      ".worktrees",
			AutoSymlinks: []string{".env", ".env.local", "node_modules", "vendor"},
			CleanMerged:  true,
		},
		Git: GitConfig{
			ConventionalCommits: true,
		},
		UI: UIConfig{
			Theme:     "default",
			ShowIcons: true,
		},
		Keys: KeyConfig{
			NewWorktree:    "n",
			DeleteWorktree: "d",
			SwitchWorktree: "enter",
			LaunchAgent:    "a",
			Commit:         "c",
			Push:           "p",
			Pull:           "u",
			Stash:          "s",
			Help:           "?",
			Quit:           "q",
			Refresh:        "r",
			Filter:         "/",
			RepoSwitch:     "tab",
		},
	}
}

// Load reads config with cascade: global → local → env overrides.
// CLI flag overrides are applied by the caller.
func Load() (*Config, error) {
	cfg := Default()

	// Global config
	globalPath, err := GlobalConfigPath()
	if err == nil {
		_ = loadFile(globalPath, cfg)
	}

	// Repo-local config
	cwd, _ := os.Getwd()
	localPath := filepath.Join(cwd, ".blink.yaml")
	_ = loadFile(localPath, cfg)

	// Environment overrides
	loadEnvOverrides(cfg)

	if err := Validate(cfg); err != nil {
		return nil, fmt.Errorf("config validation: %w", err)
	}

	return cfg, nil
}

func GlobalConfigPath() (string, error) {
	configDir, err := os.UserConfigDir()
	if err != nil {
		home, herr := os.UserHomeDir()
		if herr != nil {
			return "", herr
		}
		configDir = filepath.Join(home, ".config")
	}
	return filepath.Join(configDir, "blink", "config.yaml"), nil
}

func Save(cfg *Config) error {
	path, err := GlobalConfigPath()
	if err != nil {
		return fmt.Errorf("config path: %w", err)
	}
	if err := os.MkdirAll(filepath.Dir(path), 0o755); err != nil {
		return fmt.Errorf("mkdir: %w", err)
	}
	data, err := yaml.Marshal(cfg)
	if err != nil {
		return fmt.Errorf("marshal: %w", err)
	}
	return os.WriteFile(path, data, 0o644)
}

func loadFile(path string, cfg *Config) error {
	data, err := os.ReadFile(path)
	if err != nil {
		return err
	}
	return yaml.Unmarshal(data, cfg)
}

func loadEnvOverrides(cfg *Config) {
	if v := os.Getenv("BLINK_DEFAULT_AGENT"); v != "" {
		cfg.Agents.Default = v
	}
	if v := os.Getenv("BLINK_THEME"); v != "" {
		cfg.UI.Theme = v
	}
	if v := os.Getenv("BLINK_SHELL"); v != "" {
		cfg.Tmux.Shell = v
	}
}
