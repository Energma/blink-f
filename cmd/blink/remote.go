package main

import (
	"context"
	"encoding/json"
	"fmt"
	"os"
	"os/user"
	"path/filepath"
	"runtime"

	"github.com/urfave/cli/v3"

	"github.com/Energma/blink-f/internal/config"
	gitpkg "github.com/Energma/blink-f/internal/git"
	"github.com/Energma/blink-f/internal/tmux"
)

// remoteCommand is the machine-readable control surface for driving Blink from
// another device (phone/web) over SSH. Every subcommand emits JSON on stdout so
// a remote client can parse results without scraping human-formatted output.
func remoteCommand() *cli.Command {
	return &cli.Command{
		Name:  "remote",
		Usage: "Machine-readable control API (JSON) for remote clients",
		Commands: []*cli.Command{
			{
				Name:   "info",
				Usage:  "Identify this machine and its Blink capabilities",
				Action: remoteInfo,
			},
			{
				Name:   "sessions",
				Usage:  "List active sessions with metadata",
				Action: remoteSessions,
			},
			{
				Name:  "spawn",
				Usage: "Create a worktree and session in a target repo",
				Flags: []cli.Flag{
					&cli.StringFlag{Name: "repo", Usage: "Path to the git repository", Required: true},
					&cli.StringFlag{Name: "branch", Usage: "Branch / worktree name", Required: true},
					&cli.StringFlag{Name: "base", Usage: "Base branch to create from"},
				},
				Action: remoteSpawn,
			},
			{
				Name:  "kill",
				Usage: "Kill a session by name",
				Flags: []cli.Flag{
					&cli.StringFlag{Name: "session", Usage: "Session name to kill", Required: true},
				},
				Action: remoteKill,
			},
		},
	}
}

// emitJSON writes v as indented JSON to stdout.
func emitJSON(v any) error {
	enc := json.NewEncoder(os.Stdout)
	enc.SetIndent("", "  ")
	return enc.Encode(v)
}

type machineInfo struct {
	Hostname string `json:"hostname"`
	User     string `json:"user"`
	OS       string `json:"os"`
	Arch     string `json:"arch"`
	Version  string `json:"version"`
	Tmux     bool   `json:"tmux"`
	Sessions int    `json:"sessions"`
}

// The functions below hold the core logic, shared by the CLI subcommands here
// and the HTTP handlers in serve.go. They return data; callers decide how to
// render it (JSON to stdout, or JSON over HTTP).

func gatherMachineInfo(ctx context.Context) machineInfo {
	cfg, _ := config.Load()
	tmuxSvc := tmux.NewService(cfg)

	hostname, _ := os.Hostname()
	username := ""
	if u, err := user.Current(); err == nil {
		username = u.Username
	}
	sessions, _ := tmuxSvc.ListSessions(ctx)

	return machineInfo{
		Hostname: hostname,
		User:     username,
		OS:       runtime.GOOS,
		Arch:     runtime.GOARCH,
		Version:  version,
		Tmux:     tmuxSvc.IsAvailable(),
		Sessions: len(sessions),
	}
}

func listSessionInfos(ctx context.Context) ([]tmux.SessionInfo, error) {
	cfg, _ := config.Load()
	sessions, err := tmux.NewService(cfg).ListSessionsDetailed(ctx)
	if err != nil {
		return nil, fmt.Errorf("list sessions: %w", err)
	}
	if sessions == nil {
		sessions = []tmux.SessionInfo{}
	}
	return sessions, nil
}

type spawnResult struct {
	Session string `json:"session"`
	Branch  string `json:"branch"`
	Path    string `json:"path"`
}

func spawnSession(ctx context.Context, repo, branch, base string) (spawnResult, error) {
	cfg, _ := config.Load()
	svc := gitpkg.NewService()

	root, err := svc.RepoRoot(ctx, repo)
	if err != nil {
		return spawnResult{}, fmt.Errorf("not a git repository: %s", repo)
	}
	wt, err := svc.CreateWorktree(ctx, root, branch, base, cfg.Worktree.BaseDir)
	if err != nil {
		return spawnResult{}, fmt.Errorf("create worktree: %w", err)
	}
	session, err := tmux.NewService(cfg).EnsureSession(ctx, filepath.Base(root), branch, wt.Path)
	if err != nil {
		return spawnResult{}, fmt.Errorf("create session: %w", err)
	}
	return spawnResult{Session: session, Branch: wt.Branch, Path: wt.Path}, nil
}

// launchTUI ensures a tmux session running the Blink TUI itself in dir, so a
// remote client can attach to the full app. dir defaults to the user's home;
// the session is named per-directory so multiple folders can run at once.
func launchTUI(ctx context.Context, dir string) (spawnResult, error) {
	if dir == "" {
		dir, _ = os.UserHomeDir()
	}
	exe, err := os.Executable()
	if err != nil {
		return spawnResult{}, fmt.Errorf("locate blink binary: %w", err)
	}

	base := "blink-tui"
	if b := filepath.Base(dir); b != "" && b != "." && b != "/" {
		base = tmux.SessionNameForWorktree("blink-tui", b)
	}

	cfg, _ := config.Load()
	svc := tmux.NewService(cfg)

	// Allocate a fresh name so each launch is a new window the user can manage
	// independently (blink-tui_repo, blink-tui_repo-2, ...).
	name := base
	for i := 2; svc.SessionExists(name); i++ {
		name = fmt.Sprintf("%s-%d", base, i)
	}

	session, err := svc.EnsureProgramSession(ctx, name, dir, exe)
	if err != nil {
		return spawnResult{}, err
	}
	return spawnResult{Session: session, Path: dir}, nil
}

func killSession(ctx context.Context, session string) error {
	cfg, _ := config.Load()
	tmuxSvc := tmux.NewService(cfg)

	if !tmuxSvc.SessionExists(session) {
		return fmt.Errorf("session not found: %s", session)
	}
	if err := tmuxSvc.KillSession(ctx, session); err != nil {
		return fmt.Errorf("kill session: %w", err)
	}
	return nil
}

func remoteInfo(ctx context.Context, cmd *cli.Command) error {
	return emitJSON(gatherMachineInfo(ctx))
}

func remoteSessions(ctx context.Context, cmd *cli.Command) error {
	sessions, err := listSessionInfos(ctx)
	if err != nil {
		return err
	}
	return emitJSON(sessions)
}

func remoteSpawn(ctx context.Context, cmd *cli.Command) error {
	res, err := spawnSession(ctx, cmd.String("repo"), cmd.String("branch"), cmd.String("base"))
	if err != nil {
		return err
	}
	return emitJSON(res)
}

func remoteKill(ctx context.Context, cmd *cli.Command) error {
	session := cmd.String("session")
	if err := killSession(ctx, session); err != nil {
		return err
	}
	return emitJSON(map[string]string{"killed": session})
}
