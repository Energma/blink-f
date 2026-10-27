package components

import (
	"unicode"

	tea "charm.land/bubbletea/v2"
	"charm.land/lipgloss/v2"
)

// TextInput is a single-line editable field with a movable cursor.
// Value is stored as runes so cursor math and backspace stay correct for
// multi-byte input (accents, CJK, emoji pasted from elsewhere).
type TextInput struct {
	value  []rune
	cursor int
}

// Value returns the current text.
func (t *TextInput) Value() string { return string(t.value) }

// SetValue replaces the text and moves the cursor to the end.
func (t *TextInput) SetValue(s string) {
	t.value = []rune(s)
	t.cursor = len(t.value)
}

// Reset clears the field.
func (t *TextInput) Reset() { t.SetValue("") }

// Insert types s at the cursor. Newlines are dropped since the field is
// single-line; this keeps multi-line pastes from breaking the layout.
func (t *TextInput) Insert(s string) {
	var ins []rune
	for _, r := range s {
		if r == '\n' || r == '\r' {
			continue
		}
		if unicode.IsPrint(r) {
			ins = append(ins, r)
		}
	}
	if len(ins) == 0 {
		return
	}
	v := make([]rune, 0, len(t.value)+len(ins))
	v = append(v, t.value[:t.cursor]...)
	v = append(v, ins...)
	v = append(v, t.value[t.cursor:]...)
	t.value = v
	t.cursor += len(ins)
}

// HandleKey applies an editing key. It returns true when the key was
// consumed, so callers can fall through to their own bindings otherwise.
func (t *TextInput) HandleKey(msg tea.KeyPressMsg) bool {
	switch msg.String() {
	case "left":
		if t.cursor > 0 {
			t.cursor--
		}
	case "right":
		if t.cursor < len(t.value) {
			t.cursor++
		}
	case "home", "ctrl+a":
		t.cursor = 0
	case "end", "ctrl+e":
		t.cursor = len(t.value)
	case "ctrl+left", "alt+left", "alt+b":
		t.cursor = t.prevWord()
	case "ctrl+right", "alt+right", "alt+f":
		t.cursor = t.nextWord()
	case "backspace", "ctrl+h":
		if t.cursor > 0 {
			t.value = append(t.value[:t.cursor-1], t.value[t.cursor:]...)
			t.cursor--
		}
	case "delete", "ctrl+d":
		if t.cursor < len(t.value) {
			t.value = append(t.value[:t.cursor], t.value[t.cursor+1:]...)
		}
	case "ctrl+w", "alt+backspace", "ctrl+backspace":
		start := t.prevWord()
		t.value = append(t.value[:start], t.value[t.cursor:]...)
		t.cursor = start
	case "ctrl+u":
		t.value = t.value[t.cursor:]
		t.cursor = 0
	case "ctrl+k":
		t.value = t.value[:t.cursor]
	default:
		if msg.Text == "" {
			return false
		}
		t.Insert(msg.Text)
	}
	return true
}

// prevWord returns the index of the start of the word before the cursor.
func (t *TextInput) prevWord() int {
	i := t.cursor
	for i > 0 && isWordSep(t.value[i-1]) {
		i--
	}
	for i > 0 && !isWordSep(t.value[i-1]) {
		i--
	}
	return i
}

// nextWord returns the index just past the end of the word after the cursor.
func (t *TextInput) nextWord() int {
	i := t.cursor
	for i < len(t.value) && isWordSep(t.value[i]) {
		i++
	}
	for i < len(t.value) && !isWordSep(t.value[i]) {
		i++
	}
	return i
}

// isWordSep treats branch-name punctuation as word boundaries so ctrl+w on
// "feature/login-form" removes "form", not the whole branch.
func isWordSep(r rune) bool {
	return unicode.IsSpace(r) || r == '/' || r == '-' || r == '_' || r == '.'
}

// View renders the text with a block cursor when focused. An empty
// unfocused field shows placeholder in the placeholder style.
func (t *TextInput) View(focused bool, text, cursor, placeholder lipgloss.Style, placeholderText string) string {
	if !focused {
		if len(t.value) == 0 && placeholderText != "" {
			return placeholder.Render(placeholderText)
		}
		return text.Render(string(t.value))
	}
	before := text.Render(string(t.value[:t.cursor]))
	if t.cursor == len(t.value) {
		return before + cursor.Render("█")
	}
	under := cursor.Reverse(true).Render(string(t.value[t.cursor]))
	after := text.Render(string(t.value[t.cursor+1:]))
	return before + under + after
}
