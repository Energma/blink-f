package main

import (
	"context"
	"encoding/json"
	"fmt"
	"os"
	"os/user"
	"path/filepath"
	"runtime"
	"strings"

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

type repoInfo struct {
	Name string `json:"name"`
	Path string `json:"path"`
}

// listRepos returns the repos configured in Blink, so a remote client can pick
// one to launch a terminal in instead of typing a path. Names fall back to the
// directory base when unset.
func listRepos() []repoInfo {
	cfg, _ := config.Load()
	repos := make([]repoInfo, 0, len(cfg.Repos))
	for _, r := range cfg.Repos {
		name := r.Name
		if name == "" {
			name = filepath.Base(r.Path)
		}
		repos = append(repos, repoInfo{Name: name, Path: r.Path})
	}
	return repos
}

type dirEntry struct {
	Name   string `json:"name"`
	Path   string `json:"path"`
	IsRepo bool   `json:"isRepo"`
}

type dirListing struct {
	Path    string     `json:"path"`
	Parent  string     `json:"parent"`
	Entries []dirEntry `json:"entries"`
}

// listDir returns the sub-directories of path so a remote client can browse the
// filesystem to find a repo. path defaults to home; ~ is expanded; a file path
// lists its containing directory. Hidden directories are skipped; each entry is
// flagged when it looks like a git repo (has a .git entry).
func listDir(path string) (dirListing, error) {
	home, _ := os.UserHomeDir()
	switch {
	case path == "" || path == "~":
		path = home
	case strings.HasPrefix(path, "~/"):
		path = filepath.Join(home, path[2:])
	}
	abs, err := filepath.Abs(path)
	if err != nil {
		return dirListing{}, fmt.Errorf("resolve path: %w", err)
	}
	info, err := os.Stat(abs)
	if err != nil {
		return dirListing{}, fmt.Errorf("open %s: %w", abs, err)
	}
	if !info.IsDir() {
		abs = filepath.Dir(abs)
	}

	ents, err := os.ReadDir(abs)
	if err != nil {
		return dirListing{}, fmt.Errorf("read dir: %w", err)
	}

	listing := dirListing{Path: abs, Entries: []dirEntry{}}
	if parent := filepath.Dir(abs); parent != abs {
		listing.Parent = parent
	}
	for _, e := range ents {
		name := e.Name()
		if strings.HasPrefix(name, ".") {
			continue
		}
		full := filepath.Join(abs, name)
		fi, err := os.Stat(full) // follows symlinks so linked repos still show
		if err != nil || !fi.IsDir() {
			continue
		}
		isRepo := false
		if _, err := os.Stat(filepath.Join(full, ".git")); err == nil {
			isRepo = true
		}
		listing.Entries = append(listing.Entries, dirEntry{Name: name, Path: full, IsRepo: isRepo})
	}
	return listing, nil
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

// uniqueName returns base, or base-2, base-3, ... — the first name not already
// taken by a tmux session — so each launch is its own independent window.
func uniqueName(svc *tmux.Service, base string) string {
	name := base
	for i := 2; svc.SessionExists(name); i++ {
		name = fmt.Sprintf("%s-%d", base, i)
	}
	return name
}

// sessionBase builds a per-folder session-name prefix from label and dir, so
// names read like "claude_myrepo" / "blink-tui_myrepo".
func sessionBase(label, dir string) string {
	if b := filepath.Base(dir); b != "" && b != "." && b != "/" {
		return tmux.SessionNameForWorktree(label, b)
	}
	return label
}

// launchTUI ensures a tmux session running the Blink TUI itself in dir, so a
// remote client can attach to the full app. dir defaults to the user's home;
// each launch is a fresh window so multiple TUIs can run at once.
func launchTUI(ctx context.Context, dir string) (spawnResult, error) {
	if dir == "" {
		dir, _ = os.UserHomeDir()
	}
	exe, err := os.Executable()
	if err != nil {
		return spawnResult{}, fmt.Errorf("locate blink binary: %w", err)
	}

	cfg, _ := config.Load()
	svc := tmux.NewService(cfg)
	name := uniqueName(svc, sessionBase("blink-tui", dir))

	session, err := svc.EnsureProgramSession(ctx, name, dir, exe)
	if err != nil {
		return spawnResult{}, err
	}
	return spawnResult{Session: session, Path: dir}, nil
}

// runProgram creates a fresh tmux session running command in dir, so a remote
// client can attach and use a full terminal (Claude Code, a shell, anything).
// dir defaults to home; command defaults to the user's shell; label is the
// session-name prefix shown in the UI (e.g. "claude", "sh").
func runProgram(ctx context.Context, dir, command, label string) (spawnResult, error) {
	if dir == "" {
		dir, _ = os.UserHomeDir()
	}
	cfg, _ := config.Load()
	svc := tmux.NewService(cfg)

	if command == "" {
		if command = cfg.Tmux.Shell; command == "" {
			command = os.Getenv("SHELL")
		}
	}
	if label == "" {
		label = "run"
	}

	name := uniqueName(svc, sessionBase(label, dir))
	session, err := svc.EnsureProgramSession(ctx, name, dir, command)
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
