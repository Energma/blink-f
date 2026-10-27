package components

import (
	"testing"

	tea "charm.land/bubbletea/v2"
	"github.com/stretchr/testify/assert"
)

func press(s string) tea.KeyPressMsg {
	switch s {
	case "left":
		return tea.KeyPressMsg{Code: tea.KeyLeft}
	case "right":
		return tea.KeyPressMsg{Code: tea.KeyRight}
	case "home":
		return tea.KeyPressMsg{Code: tea.KeyHome}
	case "end":
		return tea.KeyPressMsg{Code: tea.KeyEnd}
	case "backspace":
		return tea.KeyPressMsg{Code: tea.KeyBackspace}
	case "delete":
		return tea.KeyPressMsg{Code: tea.KeyDelete}
	case "ctrl+w", "ctrl+u", "ctrl+k", "ctrl+a", "ctrl+e":
		return tea.KeyPressMsg{Code: rune(s[len(s)-1]), Mod: tea.ModCtrl}
	case "enter":
		return tea.KeyPressMsg{Code: tea.KeyEnter}
	}
	r := []rune(s)[0]
	return tea.KeyPressMsg{Code: r, Text: s}
}

func TestTextInputEditing(t *testing.T) {
	tests := []struct {
		name       string
		start      string
		keys       []string
		want       string
		wantCursor int
	}{
		{"type at end", "", []string{"a", "b", "c"}, "abc", 3},
		{"insert mid", "ac", []string{"left", "b"}, "abc", 2},
		{"backspace mid", "abc", []string{"left", "backspace"}, "ac", 1},
		{"backspace at start is noop", "abc", []string{"home", "backspace"}, "abc", 0},
		{"delete under cursor", "abc", []string{"home", "delete"}, "bc", 0},
		{"utf8 backspace", "čaj", []string{"backspace", "backspace"}, "č", 1},
		{"ctrl+w removes last segment", "feature/login-form", []string{"ctrl+w"}, "feature/login-", 14},
		{"ctrl+w skips trailing sep", "feature/login-", []string{"ctrl+w"}, "feature/", 8},
		{"ctrl+u clears to start", "hello world", []string{"left", "left", "ctrl+u"}, "ld", 0},
		{"ctrl+k clears to end", "hello world", []string{"ctrl+a", "right", "ctrl+k"}, "h", 1},
		{"end after home", "abc", []string{"home", "ctrl+e", "d"}, "abcd", 4},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			var in TextInput
			in.SetValue(tt.start)
			for _, k := range tt.keys {
				in.HandleKey(press(k))
			}
			assert.Equal(t, tt.want, in.Value())
			assert.Equal(t, tt.wantCursor, in.cursor)
		})
	}
}

func TestTextInputIgnoresUnhandledKeys(t *testing.T) {
	var in TextInput
	assert.False(t, in.HandleKey(press("enter")))
	assert.Equal(t, "", in.Value())
}

func TestTextInputInsertDropsNewlines(t *testing.T) {
	var in TextInput
	in.Insert("fix:\nthing\r")
	assert.Equal(t, "fix:thing", in.Value())
}
