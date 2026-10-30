package views

import (
	"testing"

	"github.com/stretchr/testify/assert"
)

func TestScrollWindow(t *testing.T) {
	two := func(int) int { return 2 }
	// The cursor's item is three lines, as a selected worktree with a
	// last commit is.
	tallCursor := func(cursor int) func(int) int {
		return func(i int) int {
			if i == cursor {
				return 3
			}
			return 2
		}
	}

	tests := []struct {
		name               string
		n, cursor, avail   int
		height             func(int) int
		wantStart, wantEnd int
	}{
		{"empty", 0, 0, 10, two, 0, 0},
		{"all fit", 4, 3, 10, two, 0, 4},
		{"cut off below leaves a more line", 10, 0, 10, two, 0, 4},
		{"cursor at the bottom", 10, 5, 10, two, 2, 6},
		{"tall cursor pushes the window down", 10, 5, 10, tallCursor(5), 3, 6},
		{"last item needs no more-below line", 10, 9, 10, tallCursor(9), 6, 10},
		{"pane too small still draws the cursor", 10, 5, 1, two, 5, 6},
		{"cursor past the end is clamped", 3, 7, 10, two, 0, 3},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			start, end := scrollWindow(tt.n, tt.cursor, tt.avail, tt.height)
			assert.Equal(t, tt.wantStart, start, "start")
			assert.Equal(t, tt.wantEnd, end, "end")

			if tt.n == 0 || tt.avail < 4 {
				return
			}
			// What is drawn, indicators included, fits in avail.
			used := 0
			if start > 0 {
				used++
			}
			if end < tt.n {
				used++
			}
			for i := start; i < end; i++ {
				used += tt.height(i)
			}
			assert.LessOrEqual(t, used, tt.avail)
		})
	}
}
