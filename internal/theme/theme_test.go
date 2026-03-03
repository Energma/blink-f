package theme

import (
	"testing"

	"github.com/stretchr/testify/assert"
)

func TestDefaultTheme(t *testing.T) {
	th := Default()
	assert.NotNil(t, th)
	assert.NotNil(t, th.Primary)
	assert.NotNil(t, th.Secondary)
	assert.NotNil(t, th.Accent)
	assert.NotNil(t, th.Success)
	assert.NotNil(t, th.Warning)
	assert.NotNil(t, th.Error)
	assert.NotNil(t, th.Muted)
	assert.NotNil(t, th.Text)
	assert.NotNil(t, th.TextDim)
	assert.NotNil(t, th.Border)
	assert.NotNil(t, th.Highlight)
	assert.NotNil(t, th.Background)
}

func TestMinimalTheme(t *testing.T) {
	th := Minimal()
	assert.NotNil(t, th)
	assert.NotNil(t, th.Primary)
	assert.NotNil(t, th.Secondary)
	assert.NotNil(t, th.Accent)
	assert.NotNil(t, th.Success)
	assert.NotNil(t, th.Warning)
	assert.NotNil(t, th.Error)
	assert.NotNil(t, th.Muted)
	assert.NotNil(t, th.Text)
	assert.NotNil(t, th.TextDim)
	assert.NotNil(t, th.Border)
	assert.NotNil(t, th.Highlight)
	assert.NotNil(t, th.Background)
}

func TestGet(t *testing.T) {
	tests := []struct {
		name      string
		input     string
		expectMin bool // true if we expect Minimal theme
	}{
		{"default", "default", false},
		{"minimal", "minimal", true},
		{"unknown fallback", "neon", false},
		{"empty fallback", "", false},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := Get(tt.input)
			assert.NotNil(t, got)

			if tt.expectMin {
				// Minimal uses ANSI colors, Default uses hex
				assert.Equal(t, Minimal().Primary, got.Primary)
			} else {
				assert.Equal(t, Default().Primary, got.Primary)
			}
		})
	}
}
