package components

import (
	"strings"

	"github.com/Energma/blink-f/internal/models"
)

// FilterWorktrees returns worktrees matching the query (fuzzy-ish).
func FilterWorktrees(worktrees []models.Worktree, query string) []int {
	if query == "" {
		indices := make([]int, len(worktrees))
		for i := range indices {
			indices[i] = i
		}
		return indices
	}

	query = strings.ToLower(query)
	var matches []int
	for i, wt := range worktrees {
		name := strings.ToLower(wt.DisplayName())
		path := strings.ToLower(wt.Path)
		if fuzzyMatch(name, query) || fuzzyMatch(path, query) {
			matches = append(matches, i)
		}
	}
	return matches
}

// fuzzyMatch checks if all query chars appear in target in order.
func fuzzyMatch(target, query string) bool {
	if strings.Contains(target, query) {
		return true
	}
	qi := 0
	for i := 0; i < len(target) && qi < len(query); i++ {
		if target[i] == query[qi] {
			qi++
		}
	}
	return qi == len(query)
}

// FilterBranches returns branches matching query, case-insensitive.
// Substring hits come before scattered fuzzy hits so typing "main" puts
// "main" above "maintenance-fix-in" style noise.
func FilterBranches(branches []string, query string) []string {
	if query == "" {
		return branches
	}
	query = strings.ToLower(query)
	var exact, fuzzy []string
	for _, b := range branches {
		lb := strings.ToLower(b)
		switch {
		case strings.Contains(lb, query):
			exact = append(exact, b)
		case fuzzyMatch(lb, query):
			fuzzy = append(fuzzy, b)
		}
	}
	return append(exact, fuzzy...)
}
