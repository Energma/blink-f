package tmux

import (
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// Captured from `rmx list --json` with one command workspace and one shell.
const rmxListSample = `{"workspaces":[` +
	`{"attached":0,"id":1,"last_used":0,"name":"probe_one","tabs":[{"active":true,"index":1,"name":"sleep","panes":[{"active":true,"cwd":"/tmp","id":1,"index":1,"pid":80442,"title":"sleep"}],"zoomed":false}]},` +
	`{"attached":2,"id":2,"last_used":0,"name":"blink-f_main","tabs":[` +
	`{"active":false,"index":1,"name":"zsh","panes":[{"active":true,"cwd":"/repo","id":2,"index":1,"pid":1,"title":"zsh"}],"zoomed":false},` +
	`{"active":true,"index":2,"name":"zsh","panes":[{"active":false,"cwd":"/repo","id":3,"index":1,"pid":2,"title":"zsh"},{"active":true,"cwd":"/repo/src","id":4,"index":2,"pid":3,"title":"zsh"}],"zoomed":false}]}]}`

func TestParseRmxList(t *testing.T) {
	ws, err := parseRmxList([]byte(rmxListSample))
	require.NoError(t, err)
	require.Len(t, ws, 2)

	assert.Equal(t, "probe_one", ws[0].Name)
	assert.Equal(t, "/tmp", ws[0].cwd())
	assert.Equal(t, 0, ws[0].Attached)

	assert.Equal(t, "blink-f_main", ws[1].Name)
	assert.Equal(t, 2, ws[1].Attached)
	assert.Len(t, ws[1].Tabs, 2)
	// Active pane of the active tab, not the first pane.
	assert.Equal(t, "/repo/src", ws[1].cwd())
}

func TestParseRmxListEmpty(t *testing.T) {
	ws, err := parseRmxList([]byte(`{"workspaces":[]}`))
	require.NoError(t, err)
	assert.Empty(t, ws)
}

func TestParseRmxListInvalid(t *testing.T) {
	_, err := parseRmxList([]byte("rmx: no rmx server is running"))
	assert.Error(t, err)
}

func TestRmxWorkspaceCwdFallback(t *testing.T) {
	w := rmxWorkspace{Tabs: []rmxTab{{Panes: []rmxPane{{Cwd: "/only"}}}}}
	assert.Equal(t, "/only", w.cwd())
	assert.Equal(t, "", rmxWorkspace{}.cwd())
}
