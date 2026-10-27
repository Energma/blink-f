package rmxbin

import (
	"os"
	"path/filepath"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestUnpackWritesExecutableOnce(t *testing.T) {
	root := t.TempDir()
	data := []byte("#!/bin/sh\necho rmx\n")

	p, err := unpack(data, root)
	require.NoError(t, err)
	assert.Equal(t, filepath.Dir(filepath.Dir(p)), root)

	info, err := os.Stat(p)
	require.NoError(t, err)
	assert.NotZero(t, info.Mode()&0o100, "must be executable")

	// A second call reuses the file instead of rewriting it.
	before := info.ModTime()
	again, err := unpack(data, root)
	require.NoError(t, err)
	assert.Equal(t, p, again)
	info, _ = os.Stat(again)
	assert.Equal(t, before, info.ModTime())

	// Different content lands in a different directory.
	other, err := unpack([]byte("other"), root)
	require.NoError(t, err)
	assert.NotEqual(t, filepath.Dir(p), filepath.Dir(other))

	entries, _ := os.ReadDir(filepath.Dir(p))
	assert.Len(t, entries, 1, "no temp files left behind")
}
