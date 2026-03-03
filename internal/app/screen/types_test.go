package screen

import (
	"testing"

	"github.com/stretchr/testify/assert"
)

func TestTypeString(t *testing.T) {
	tests := []struct {
		name string
		typ  Type
		want string
	}{
		{"none", None, "none"},
		{"dashboard", Dashboard, "dashboard"},
		{"create", WorktreeCreate, "create"},
		{"detail", WorktreeDetail, "detail"},
		{"confirm", Confirm, "confirm"},
		{"commit", Commit, "commit"},
		{"agent", AgentSelect, "agent"},
		{"repos", RepoSelect, "repos"},
		{"sessions", Sessions, "sessions"},
		{"help", Help, "help"},
		{"filter", Filter, "filter"},
		{"unknown", Type(999), "none"},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			assert.Equal(t, tt.want, tt.typ.String())
		})
	}
}
