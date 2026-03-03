package git

import (
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
)

func TestFormatConventionalCommit(t *testing.T) {
	tests := []struct {
		name        string
		typ         string
		scope       string
		description string
		want        string
	}{
		{"with scope", "feat", "auth", "add login", "feat(auth): add login"},
		{"without scope", "fix", "", "typo in readme", "fix: typo in readme"},
		{"ci with scope", "ci", "gh", "add workflow", "ci(gh): add workflow"},
		{"refactor no scope", "refactor", "", "simplify handler", "refactor: simplify handler"},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := FormatConventionalCommit(tt.typ, tt.scope, tt.description)
			assert.Equal(t, tt.want, got)
		})
	}
}

func TestConventionalTypes(t *testing.T) {
	types := ConventionalTypes()
	assert.Len(t, types, 10)

	expected := []string{"feat", "fix", "refactor", "chore", "test", "docs", "style", "perf", "ci", "build"}
	assert.Equal(t, expected, types)
}

func TestParseTime(t *testing.T) {
	tests := []struct {
		name    string
		input   string
		wantErr bool
	}{
		{"valid UTC", "2025-01-15T10:30:00Z", false},
		{"valid with offset", "2025-01-15T10:30:00+05:00", false},
		{"invalid", "not-a-date", true},
		{"empty", "", true},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result, err := parseTime(tt.input)
			if tt.wantErr {
				assert.Error(t, err)
			} else {
				assert.NoError(t, err)
				assert.False(t, result.IsZero())
			}
		})
	}

	// Verify parsed values are correct
	t.Run("correct value", func(t *testing.T) {
		result, err := parseTime("2025-01-15T10:30:00Z")
		assert.NoError(t, err)
		assert.Equal(t, 2025, result.Year())
		assert.Equal(t, time.January, result.Month())
		assert.Equal(t, 15, result.Day())
		assert.Equal(t, 10, result.Hour())
		assert.Equal(t, 30, result.Minute())
	})
}
