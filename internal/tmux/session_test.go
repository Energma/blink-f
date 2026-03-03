package tmux

import (
	"testing"

	"github.com/stretchr/testify/assert"
)

func TestSessionNameForWorktree(t *testing.T) {
	tests := []struct {
		name     string
		repoName string
		branch   string
		want     string
	}{
		{"simple", "myapp", "main", "myapp_main"},
		{"slash to hyphen", "myapp", "feature/login", "myapp_feature-login"},
		{"dot to underscore", "my.app", "fix/v2.1", "my_app_fix-v2_1"},
		{"multiple replacements", "repo", "release/1.0.0", "repo_release-1_0_0"},
		{"no special chars", "simple", "simple", "simple_simple"},
		{"nested slashes", "repo", "feat/ui/modal", "repo_feat-ui-modal"},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := SessionNameForWorktree(tt.repoName, tt.branch)
			assert.Equal(t, tt.want, got)
		})
	}
}
