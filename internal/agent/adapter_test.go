package agent

import (
	"testing"

	"github.com/Energma/blink-f/internal/config"
	"github.com/stretchr/testify/assert"
)

func testConfig() *config.Config {
	return &config.Config{
		Agents: config.AgentConfig{
			Default: "claude",
			Providers: map[string]config.ProviderConfig{
				"claude": {Command: "claude", Args: []string{"--chat"}},
				"aider":  {Command: "aider", Args: []string{"--model", "gpt-4"}},
			},
		},
	}
}

func TestGenericProviderAccessors(t *testing.T) {
	p := &GenericProvider{name: "test", command: "mycli", args: []string{"--flag"}}
	assert.Equal(t, "test", p.Name())
	assert.Equal(t, "mycli", p.Command())
	assert.Equal(t, []string{"--flag"}, p.Args())
}

func TestGenericProviderCommandString(t *testing.T) {
	tests := []struct {
		name    string
		command string
		args    []string
		want    string
	}{
		{"no args", "claude", nil, "claude"},
		{"with args", "aider", []string{"--model", "gpt-4"}, "aider --model gpt-4"},
		{"single arg", "vim", []string{"-c"}, "vim -c"},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			p := &GenericProvider{command: tt.command, args: tt.args}
			assert.Equal(t, tt.want, p.CommandString("/tmp"))
		})
	}
}

func TestGenericProviderAvailable(t *testing.T) {
	// "git" should exist on any dev machine / CI
	p := &GenericProvider{command: "git"}
	assert.True(t, p.Available())

	// Nonexistent binary
	p2 := &GenericProvider{command: "nonexistent_binary_xyz_123"}
	assert.False(t, p2.Available())
}

func TestRegistryFromConfig(t *testing.T) {
	cfg := testConfig()
	r := NewRegistry(cfg)

	names := r.List()
	assert.Len(t, names, 2)
	assert.ElementsMatch(t, []string{"claude", "aider"}, names)
}

func TestRegistryGet(t *testing.T) {
	cfg := testConfig()
	r := NewRegistry(cfg)

	p, err := r.Get("claude")
	assert.NoError(t, err)
	assert.Equal(t, "claude", p.Name())
	assert.Equal(t, "claude", p.Command())
	assert.Equal(t, []string{"--chat"}, p.Args())

	_, err = r.Get("nonexistent")
	assert.Error(t, err)
	assert.Contains(t, err.Error(), "not configured")
}

func TestRegistryDefault(t *testing.T) {
	cfg := testConfig()
	r := NewRegistry(cfg)

	p, err := r.Default()
	assert.NoError(t, err)
	assert.Equal(t, "claude", p.Name())

	// Missing default
	cfg2 := &config.Config{
		Agents: config.AgentConfig{
			Default:   "missing",
			Providers: map[string]config.ProviderConfig{},
		},
	}
	r2 := NewRegistry(cfg2)
	_, err = r2.Default()
	assert.Error(t, err)
}

func TestRegistryAvailable(t *testing.T) {
	cfg := &config.Config{
		Agents: config.AgentConfig{
			Default: "git-tool",
			Providers: map[string]config.ProviderConfig{
				"git-tool": {Command: "git"},
				"fake":     {Command: "nonexistent_binary_xyz_123"},
			},
		},
	}
	r := NewRegistry(cfg)

	avail := r.Available()
	assert.Contains(t, avail, "git-tool")
	assert.NotContains(t, avail, "fake")
}
