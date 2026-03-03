package main

import (
	"context"
	"fmt"
	"os"
	"os/exec"
	"path/filepath"

	tea "charm.land/bubbletea/v2"
	"github.com/urfave/cli/v3"

	"github.com/Energma/blink-f/internal/agent"
	"github.com/Energma/blink-f/internal/app"
	"github.com/Energma/blink-f/internal/config"
	gitpkg "github.com/Energma/blink-f/internal/git"
	"github.com/Energma/blink-f/internal/tmux"
)

var version = "dev"

func main() {
	cmd := &cli.Command{
		Name:    "blink",
		Usage:   "Fast git worktree management TUI",
		Version: version,
		Action:  runTUI,
		Commands: []*cli.Command{
			{
				Name:  "new",
				Usage: "Create a new worktree from a branch",
				Flags: []cli.Flag{
					&cli.StringFlag{
						Name:    "base",
						Aliases: []string{"b"},
						Usage:   "Base branch to create from",
					},
				},
				Action: newWorktree,
			},
			{
				Name:   "switch",
				Usage:  "Switch to a worktree's tmux session",
				Action: switchWorktree,
			},
			{
				Name:  "agent",
				Usage: "Launch AI agent in current directory",
				Flags: []cli.Flag{
					&cli.StringFlag{
						Name:    "provider",
						Aliases: []string{"p"},
						Usage:   "Agent provider name",
					},
					&cli.BoolFlag{
						Name:  "popup",
						Usage: "Launch in tmux popup instead of session",
					},
				},
				Action: launchAgent,
			},
			{
				Name:   "status",
				Usage:  "Show worktree status overview",
				Action: showStatus,
			},
			{
				Name:   "clean",
				Usage:  "Remove merged worktrees",
				Action: cleanWorktrees,
			},
			{
				Name:   "config",
				Usage:  "Open config in $EDITOR",
				Action: editConfig,
			},
			{
				Name:   "list",
				Usage:  "List all worktrees",
				Action: listWorktrees,
			},
		},
	}

	if err := cmd.Run(context.Background(), os.Args); err != nil {
		fmt.Fprintf(os.Stderr, "error: %v\n", err)
		os.Exit(1)
	}
}

func runTUI(ctx context.Context, cmd *cli.Command) error {
	cfg, err := config.Load()
	if err != nil {
		return fmt.Errorf("load config: %w", err)
	}

	m := app.New(cfg)
	p := tea.NewProgram(m)

	if _, err := p.Run(); err != nil {
		return fmt.Errorf("TUI: %w", err)
	}
	return nil
}

func newWorktree(ctx context.Context, cmd *cli.Command) error {
	branch := cmd.Args().First()
	if branch == "" {
		return fmt.Errorf("usage: blink new <branch>")
	}
	baseBranch := cmd.String("base")

	cfg, _ := config.Load()
	svc := gitpkg.NewService()

	cwd, _ := os.Getwd()
	root, err := svc.RepoRoot(ctx, cwd)
	if err != nil {
		return fmt.Errorf("not a git repository")
	}

	wt, err := svc.CreateWorktree(ctx, root, branch, baseBranch, cfg.Worktree.BaseDir)
	if err != nil {
		return err
	}
	fmt.Printf("Created worktree: %s -> %s\n", wt.Branch, wt.Path)

	// Auto-create tmux session
	if cfg.Tmux.AutoSession {
		tmuxSvc := tmux.NewService(cfg)
		if tmuxSvc.IsAvailable() {
			name, err := tmuxSvc.EnsureSession(ctx, filepath.Base(root), branch, wt.Path)
			if err != nil {
				fmt.Fprintf(os.Stderr, "warning: tmux session: %v\n", err)
			} else {
				fmt.Printf("tmux session: %s\n", name)
			}
		}
	}
	return nil
}

func switchWorktree(ctx context.Context, cmd *cli.Command) error {
	name := cmd.Args().First()
	if name == "" {
		return fmt.Errorf("usage: blink switch <branch-name>")
	}

	cfg, _ := config.Load()
	svc := gitpkg.NewService()
	tmuxSvc := tmux.NewService(cfg)

	if !tmuxSvc.IsAvailable() {
		return fmt.Errorf("tmux not available")
	}

	cwd, _ := os.Getwd()
	root, err := svc.RepoRoot(ctx, cwd)
	if err != nil {
		return fmt.Errorf("not a git repository")
	}

	repoName := filepath.Base(root)
	sessionName := tmux.SessionNameForWorktree(repoName, name)

	if !tmuxSvc.SessionExists(sessionName) {
		// Try to find the worktree and create a session
		wts, err := svc.ListWorktrees(ctx, root)
		if err != nil {
			return err
		}
		for _, wt := range wts {
			if wt.Branch == name {
				_, err = tmuxSvc.EnsureSession(ctx, repoName, name, wt.Path)
				if err != nil {
					return fmt.Errorf("create session: %w", err)
				}
				break
			}
		}
	}

	return tmuxSvc.SwitchSession(ctx, sessionName)
}

func launchAgent(ctx context.Context, cmd *cli.Command) error {
	cfg, _ := config.Load()
	registry := agent.NewRegistry(cfg)
	tmuxSvc := tmux.NewService(cfg)

	providerName := cmd.String("provider")
	if providerName == "" {
		providerName = cfg.Agents.Default
	}

	p, err := registry.Get(providerName)
	if err != nil {
		return err
	}
	if !p.Available() {
		return fmt.Errorf("agent %q not found on PATH", providerName)
	}

	cwd, _ := os.Getwd()

	if cmd.Bool("popup") && tmux.InsideTmux() {
		return agent.LaunchInPopup(ctx, tmuxSvc, p, cwd)
	}

	// Direct launch
	c := agent.LaunchDirect(ctx, p, cwd)
	c.Stdin = os.Stdin
	c.Stdout = os.Stdout
	c.Stderr = os.Stderr
	return c.Run()
}

func showStatus(ctx context.Context, cmd *cli.Command) error {
	svc := gitpkg.NewService()
	cwd, _ := os.Getwd()
	root, err := svc.RepoRoot(ctx, cwd)
	if err != nil {
		return fmt.Errorf("not a git repository")
	}

	wts, err := svc.ListWorktrees(ctx, root)
	if err != nil {
		return err
	}

	cfg, _ := config.Load()
	tmuxSvc := tmux.NewService(cfg)
	sessions, _ := tmuxSvc.ListSessions(ctx)
	sessionSet := make(map[string]bool)
	for _, s := range sessions {
		sessionSet[s] = true
	}

	repoName := filepath.Base(root)
	fmt.Printf("Repository: %s\n", repoName)
	fmt.Printf("Worktrees:  %d\n\n", len(wts))

	for _, wt := range wts {
		st, _ := svc.Status(ctx, wt.Path)
		name := wt.DisplayName()
		if wt.IsMain {
			name += " (main)"
		}

		sessionName := tmux.SessionNameForWorktree(repoName, wt.Branch)
		tmuxStr := ""
		if sessionSet[sessionName] {
			tmuxStr = " [tmux]"
		}

		fmt.Printf("  %-30s %s%s\n", name, st.StatusLine(), tmuxStr)
	}
	return nil
}

func cleanWorktrees(ctx context.Context, cmd *cli.Command) error {
	svc := gitpkg.NewService()
	cwd, _ := os.Getwd()
	root, err := svc.RepoRoot(ctx, cwd)
	if err != nil {
		return fmt.Errorf("not a git repository")
	}

	defaultBranch := svc.DefaultBranch(ctx, root)
	wts, err := svc.ListWorktrees(ctx, root)
	if err != nil {
		return err
	}

	var merged []string
	for _, wt := range wts {
		if wt.IsMain || wt.Branch == defaultBranch {
			continue
		}
		if svc.IsBranchMerged(ctx, root, wt.Branch, defaultBranch) {
			merged = append(merged, wt.Branch)
		}
	}

	if len(merged) == 0 {
		fmt.Println("No merged worktrees to clean up.")
		return nil
	}

	fmt.Printf("Found %d merged worktrees:\n", len(merged))
	for _, b := range merged {
		fmt.Printf("  - %s\n", b)
	}
	fmt.Print("\nRemove these worktrees? [y/N] ")

	var answer string
	_, _ = fmt.Scanln(&answer)
	if answer != "y" && answer != "Y" {
		fmt.Println("Cancelled.")
		return nil
	}

	cfg, _ := config.Load()
	tmuxSvc := tmux.NewService(cfg)
	repoName := filepath.Base(root)

	for _, wt := range wts {
		for _, b := range merged {
			if wt.Branch == b {
				// Kill tmux session
				sessionName := tmux.SessionNameForWorktree(repoName, b)
				if tmuxSvc.SessionExists(sessionName) {
					_ = tmuxSvc.KillSession(ctx, sessionName)
				}
				if err := svc.RemoveWorktree(ctx, root, wt.Path, false); err != nil {
					fmt.Fprintf(os.Stderr, "  error removing %s: %v\n", b, err)
				} else {
					fmt.Printf("  removed: %s\n", b)
				}
			}
		}
	}
	return nil
}

func editConfig(ctx context.Context, cmd *cli.Command) error {
	path, err := config.GlobalConfigPath()
	if err != nil {
		return err
	}

	// Ensure config dir and file exist
	dir := filepath.Dir(path)
	_ = os.MkdirAll(dir, 0o755)
	if _, err := os.Stat(path); os.IsNotExist(err) {
		cfg := config.Default()
		if err := config.Save(cfg); err != nil {
			return fmt.Errorf("create default config: %w", err)
		}
		fmt.Printf("Created default config at %s\n", path)
	}

	editor := os.Getenv("EDITOR")
	if editor == "" {
		editor = "vim"
	}

	c := exec.CommandContext(ctx, editor, path)
	c.Stdin = os.Stdin
	c.Stdout = os.Stdout
	c.Stderr = os.Stderr
	return c.Run()
}

func listWorktrees(ctx context.Context, cmd *cli.Command) error {
	svc := gitpkg.NewService()
	cwd, _ := os.Getwd()
	root, err := svc.RepoRoot(ctx, cwd)
	if err != nil {
		return fmt.Errorf("not a git repository")
	}

	wts, err := svc.ListWorktrees(ctx, root)
	if err != nil {
		return err
	}

	for _, wt := range wts {
		main := ""
		if wt.IsMain {
			main = " (main)"
		}
		fmt.Printf("%-30s %s%s\n", wt.DisplayName(), wt.Path, main)
	}
	return nil
}
