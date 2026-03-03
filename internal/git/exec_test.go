package git

import (
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
)

func TestNewServiceSemaphoreBounds(t *testing.T) {
	s := NewService()
	assert.NotNil(t, s)

	// Semaphore capacity should be clamped between 4 and 32
	assert.GreaterOrEqual(t, cap(s.sem), 4)
	assert.LessOrEqual(t, cap(s.sem), 32)

	// Default timeout is 30 seconds
	assert.Equal(t, 30*time.Second, s.timeout)
}
