package agent

import (
	"context"
	"fmt"
	"os/exec"
	"strings"

	"github.com/Energma/blink-f/internal/config"
	"github.com/Energma/blink-f/internal/tmux"
)

// Provider defines how to launch an AI agent tool.
type Provider interface {
	Name() string
	Command() string
	Args() []string
	Available() bool
	CommandString(workDir string) string
}

// Registry holds configured agent providers.
type Registry struct {
	providers map[string]Provider
	defaultP  string
}

// NewRegistry builds a registry from config.
func NewRegistry(cfg *config.Config) *Registry {
	r := &Registry{
		providers: make(map[string]Provider),
		defaultP:  cfg.Agents.Default,
	}
	for name, pc := range cfg.Agents.Providers {
		r.providers[name] = &GenericProvider{
			name:    name,
			command: pc.Command,
			args:    pc.Args,
		}
	}
	return r
}

// Get returns a provider by name.
func (r *Registry) Get(name string) (Provider, error) {
	p, ok := r.providers[name]
	if !ok {
		return nil, fmt.Errorf("agent provider %q not configured", name)
	}
	return p, nil
}

// Default returns the default provider.
func (r *Registry) Default() (Provider, error) {
	return r.Get(r.defaultP)
}

// List returns all provider names.
func (r *Registry) List() []string {
	names := make([]string, 0, len(r.providers))
	for n := range r.providers {
		names = append(names, n)
	}
	return names
}

// Available returns providers that are installed on the system.
func (r *Registry) Available() []string {
	var avail []string
	for _, name := range r.List() {
		p, _ := r.Get(name)
		if p != nil && p.Available() {
			avail = append(avail, name)
		}
	}
	return avail
}

// LaunchInPopup launches an agent in a tmux popup.
func LaunchInPopup(ctx context.Context, tmuxSvc *tmux.Service, provider Provider, workDir string) error {
	cmdStr := provider.CommandString(workDir)
	return tmuxSvc.DisplayPopup(ctx, cmdStr, workDir)
}

// LaunchInSession launches an agent in a new tmux session.
func LaunchInSession(ctx context.Context, tmuxSvc *tmux.Service, provider Provider, sessionName, workDir string) error {
	cmdStr := provider.CommandString(workDir)
	args := []string{"new-session", "-d", "-s", sessionName, "-c", workDir, cmdStr}
	return exec.CommandContext(ctx, "tmux", args...).Run()
}

// LaunchInSplitSession launches an agent in a new tmux session with a
// horizontal split: top pane runs the agent, bottom pane is a shell.
func LaunchInSplitSession(ctx context.Context, tmuxSvc *tmux.Service, provider Provider, sessionName, workDir string) error {
	cmdStr := provider.CommandString(workDir)
	return tmuxSvc.CreateAgentSplitSession(ctx, sessionName, workDir, cmdStr)
}

// LaunchDirect launches an agent directly (no tmux), blocking.
func LaunchDirect(ctx context.Context, provider Provider, workDir string) *exec.Cmd {
	cmd := exec.CommandContext(ctx, provider.Command(), provider.Args()...)
	cmd.Dir = workDir
	return cmd
}

// GenericProvider implements Provider for any CLI tool.
type GenericProvider struct {
	name    string
	command string
	args    []string
}

func (g *GenericProvider) Name() string    { return g.name }
func (g *GenericProvider) Command() string { return g.command }
func (g *GenericProvider) Args() []string  { return g.args }

func (g *GenericProvider) Available() bool {
	_, err := exec.LookPath(g.command)
	return err == nil
}

func (g *GenericProvider) CommandString(workDir string) string {
	parts := append([]string{g.command}, g.args...)
	return strings.Join(parts, " ")
}
