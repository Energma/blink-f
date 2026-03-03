package screen

// Manager handles a stack of screen overlays.
type Manager struct {
	stack []Type
}

func NewManager() *Manager {
	return &Manager{stack: make([]Type, 0, 4)}
}

// Push adds a screen to the top of the stack.
func (m *Manager) Push(s Type) {
	m.stack = append(m.stack, s)
}

// Pop removes and returns the top screen.
func (m *Manager) Pop() Type {
	if len(m.stack) == 0 {
		return None
	}
	last := m.stack[len(m.stack)-1]
	m.stack = m.stack[:len(m.stack)-1]
	return last
}

// Active returns the current top screen, or None.
func (m *Manager) Active() Type {
	if len(m.stack) == 0 {
		return None
	}
	return m.stack[len(m.stack)-1]
}

// Clear empties the screen stack.
func (m *Manager) Clear() {
	m.stack = m.stack[:0]
}

// Depth returns the number of screens on the stack.
func (m *Manager) Depth() int {
	return len(m.stack)
}
