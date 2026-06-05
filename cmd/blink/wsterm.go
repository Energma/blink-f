package main

import (
	"context"
	"encoding/json"
	"net/http"
	"os"
	"os/exec"

	"github.com/coder/websocket"
	"github.com/creack/pty"

	"github.com/Energma/blink-f/internal/config"
	"github.com/Energma/blink-f/internal/tmux"
)

// handleSessionWS bridges a browser terminal (xterm.js) to a tmux session over
// a WebSocket. It attaches to the session via a PTY and pipes bytes both ways:
//   - binary client messages  -> keystrokes written to the PTY
//   - text client messages    -> JSON control (currently {"type":"resize",...})
//   - PTY output              -> binary messages back to the client
//
// Closing the socket detaches (kills the `tmux attach` process) but leaves the
// session running, so you can reconnect later and pick up where you left off.
func (s *remoteServer) handleSessionWS(w http.ResponseWriter, r *http.Request) {
	name := r.PathValue("name")

	cfg, _ := config.Load()
	tmuxSvc := tmux.NewService(cfg)
	if !tmuxSvc.IsAvailable() {
		http.Error(w, "tmux not available", http.StatusServiceUnavailable)
		return
	}
	if !tmuxSvc.SessionExists(name) {
		http.Error(w, "session not found", http.StatusNotFound)
		return
	}

	// Token auth is the real gate (enforced by middleware via ?token=), so we
	// skip origin verification to allow a separately-hosted UI during dev.
	conn, err := websocket.Accept(w, r, &websocket.AcceptOptions{InsecureSkipVerify: true})
	if err != nil {
		return
	}
	defer conn.CloseNow()

	ctx, cancel := context.WithCancel(r.Context())
	defer cancel()

	cmd := exec.CommandContext(ctx, "tmux", "attach", "-t", name)
	cmd.Env = append(os.Environ(), "TERM=xterm-256color")
	ptmx, err := pty.Start(cmd)
	if err != nil {
		_ = conn.Close(websocket.StatusInternalError, "failed to start pty")
		return
	}
	defer func() {
		_ = ptmx.Close()
		_ = cmd.Wait()
	}()

	// PTY output -> client.
	go func() {
		buf := make([]byte, 4096)
		for {
			n, readErr := ptmx.Read(buf)
			if n > 0 {
				if err := conn.Write(ctx, websocket.MessageBinary, buf[:n]); err != nil {
					cancel()
					return
				}
			}
			if readErr != nil {
				cancel()
				return
			}
		}
	}()

	// Client -> PTY (keystrokes) and control messages (resize).
	for {
		typ, data, err := conn.Read(ctx)
		if err != nil {
			return
		}
		if typ == websocket.MessageText {
			var msg struct {
				Type string `json:"type"`
				Cols uint16 `json:"cols"`
				Rows uint16 `json:"rows"`
			}
			if json.Unmarshal(data, &msg) == nil && msg.Type == "resize" {
				_ = pty.Setsize(ptmx, &pty.Winsize{Rows: msg.Rows, Cols: msg.Cols})
			}
			continue
		}
		if _, err := ptmx.Write(data); err != nil {
			return
		}
	}
}
