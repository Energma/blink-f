package components

import (
	"testing"

	"github.com/Energma/blink-f/internal/models"
	"github.com/stretchr/testify/assert"
)

func TestFilterWorktreesEmpty(t *testing.T) {
	wts := []models.Worktree{
		{Branch: "main"},
		{Branch: "feature/login"},
		{Branch: "fix/bug-123"},
	}

	// Empty query returns all
	indices := FilterWorktrees(wts, "")
	assert.Len(t, indices, 3)
}

func TestFilterWorktreesExact(t *testing.T) {
	wts := []models.Worktree{
		{Branch: "main"},
		{Branch: "feature/login"},
		{Branch: "fix/bug-123"},
	}

	indices := FilterWorktrees(wts, "login")
	assert.Len(t, indices, 1)
	assert.Equal(t, 1, indices[0])
}

func TestFilterWorktreesFuzzy(t *testing.T) {
	wts := []models.Worktree{
		{Branch: "main"},
		{Branch: "feature/login"},
		{Branch: "fix/bug-123"},
	}

	// "flo" matches "feature/login" (f..l..o..g..i..n)
	indices := FilterWorktrees(wts, "flo")
	assert.Len(t, indices, 1)
	assert.Equal(t, 1, indices[0])
}

func TestFilterWorktreesNoMatch(t *testing.T) {
	wts := []models.Worktree{
		{Branch: "main"},
		{Branch: "feature/login"},
	}

	indices := FilterWorktrees(wts, "zzz")
	assert.Empty(t, indices)
}

func TestFilterBranches(t *testing.T) {
	branches := []string{"main", "feature/login", "fix/bug-123", "my-awesome-integration"}
	tests := []struct {
		name  string
		query string
		want  []string
	}{
		{"empty returns all", "", branches},
		{"substring", "login", []string{"feature/login"}},
		{"case insensitive", "LOGIN", []string{"feature/login"}},
		{"substring ranks before fuzzy", "main", []string{"main", "my-awesome-integration"}},
		{"no match", "zzz", nil},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			assert.Equal(t, tt.want, FilterBranches(branches, tt.query))
		})
	}
}
