package models

import (
	"testing"

	"github.com/stretchr/testify/assert"
)

func TestRepoDisplayName(t *testing.T) {
	tests := []struct {
		name string
		repo Repo
		want string
	}{
		{"name set", Repo{Name: "my-repo", Path: "/home/user/repos/my-repo"}, "my-repo"},
		{"name empty", Repo{Name: "", Path: "/home/user/repos/cool-project"}, "cool-project"},
		{"root path", Repo{Name: "", Path: "/"}, "/"},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			assert.Equal(t, tt.want, tt.repo.DisplayName())
		})
	}
}
