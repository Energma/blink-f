package screen

import (
	"testing"

	"github.com/stretchr/testify/assert"
)

func TestManagerPushPop(t *testing.T) {
	m := NewManager()

	assert.Equal(t, None, m.Active())
	assert.Equal(t, 0, m.Depth())

	m.Push(Help)
	assert.Equal(t, Help, m.Active())
	assert.Equal(t, 1, m.Depth())

	m.Push(Confirm)
	assert.Equal(t, Confirm, m.Active())
	assert.Equal(t, 2, m.Depth())

	popped := m.Pop()
	assert.Equal(t, Confirm, popped)
	assert.Equal(t, Help, m.Active())

	popped = m.Pop()
	assert.Equal(t, Help, popped)
	assert.Equal(t, None, m.Active())

	// Pop on empty returns None
	popped = m.Pop()
	assert.Equal(t, None, popped)
}

func TestManagerClear(t *testing.T) {
	m := NewManager()
	m.Push(Help)
	m.Push(Commit)
	m.Push(AgentSelect)

	m.Clear()
	assert.Equal(t, None, m.Active())
	assert.Equal(t, 0, m.Depth())
}
