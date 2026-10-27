package tmux

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"os"
	"os/exec"
	"strings"
)

// rmx calls the rmx CLI, which talks to its background server over a local
// socket. Workspaces play the role of tmux sessions.

// insideRmx reports whether this process runs in an rmx pane. rmx sets RMX=1
// in every pane and uses it to refuse nested attaches.
func insideRmx() bool {
	return os.Getenv("RMX") != ""
}

type rmxList struct {
	Workspaces []rmxWorkspace `json:"workspaces"`
}

type rmxWorkspace struct {
	Name     string   `json:"name"`
	Attached int      `json:"attached"`
	Tabs     []rmxTab `json:"tabs"`
}

type rmxTab struct {
	Active bool      `json:"active"`
	Panes  []rmxPane `json:"panes"`
}

type rmxPane struct {
	Active bool   `json:"active"`
	Cwd    string `json:"cwd"`
}

// parseRmxList decodes `rmx list --json` output.
func parseRmxList(data []byte) ([]rmxWorkspace, error) {
	var l rmxList
	if err := json.Unmarshal(data, &l); err != nil {
		return nil, fmt.Errorf("parse rmx list: %w", err)
	}
	return l.Workspaces, nil
}

// cwd returns the folder of the active pane in the active tab, which is
// what the user sees and the closest match to tmux's session_path.
func (w rmxWorkspace) cwd() string {
	for _, t := range w.Tabs {
		if !t.Active {
			continue
		}
		for _, p := range t.Panes {
			if p.Active {
				return p.Cwd
			}
		}
	}
	if len(w.Tabs) > 0 && len(w.Tabs[0].Panes) > 0 {
		return w.Tabs[0].Panes[0].Cwd
	}
	return ""
}

// rmxWorkspaces lists workspaces. No running server means no workspaces,
// not an error, matching how ListSessions treats a stopped tmux server.
func (s *Service) rmxWorkspaces(ctx context.Context) []rmxWorkspace {
	if !s.available {
		return nil
	}
	out, err := exec.CommandContext(ctx, "rmx", "list", "--json").Output()
	if err != nil {
		return nil
	}
	ws, err := parseRmxList(out)
	if err != nil {
		return nil
	}
	return ws
}

// rmxNew creates a detached workspace. command "" runs the configured shell.
// rmx renames a duplicate to "name-2" instead of failing, so a mismatch is
// closed again and reported; callers check existence first, so this only
// happens on a race.
func (s *Service) rmxNew(ctx context.Context, name, dir, command string) error {
	if !s.available {
		return fmt.Errorf("rmx not available")
	}
	args := []string{"new", "-d", "-n", name, "-c", dir}
	if command == "" {
		command = s.cfg.Shell
	}
	if command != "" {
		args = append(args, "--", command)
	}
	var stderr bytes.Buffer
	cmd := exec.CommandContext(ctx, "rmx", args...)
	cmd.Env = os.Environ()
	cmd.Stderr = &stderr
	out, err := cmd.Output()
	if err != nil {
		return fmt.Errorf("rmx new %s: %s: %w", name, strings.TrimSpace(stderr.String()), err)
	}
	var created struct {
		Name string `json:"name"`
	}
	if err := json.Unmarshal(out, &created); err != nil {
		return fmt.Errorf("parse rmx new: %w", err)
	}
	if created.Name != name {
		_ = exec.CommandContext(ctx, "rmx", "close", created.Name).Run()
		return fmt.Errorf("rmx workspace %q already exists", name)
	}
	return nil
}

func (s *Service) rmxExists(name string) bool {
	for _, w := range s.rmxWorkspaces(context.Background()) {
		if w.Name == name {
			return true
		}
	}
	return false
}
