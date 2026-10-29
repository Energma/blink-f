package app

import (
	"testing"

	"github.com/stretchr/testify/assert"
)

func TestRunningSession(t *testing.T) {
	const plain, agent = "repo_feat-x", "repo_feat-x-agent"

	tests := []struct {
		name      string
		running   []string
		agentOnly bool
		want      string
	}{
		{"nothing running asks", nil, false, ""},
		{"plain reopened", []string{plain}, false, plain},
		{"agent reopened", []string{agent}, false, agent},
		{"agent preferred over plain", []string{plain, agent}, false, agent},
		{"a with nothing running asks", nil, true, ""},
		{"a ignores plain shell", []string{plain}, true, ""},
		{"a reopens agent", []string{plain, agent}, true, agent},
		{"other worktrees ignored", []string{"repo_main", "repo_main-agent"}, false, ""},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			set := make(map[string]bool)
			for _, s := range tt.running {
				set[s] = true
			}
			exists := func(name string) bool { return set[name] }
			assert.Equal(t, tt.want, runningSession(exists, "repo", "feat/x", tt.agentOnly))
		})
	}
}
