package main

import (
	"context"
	"crypto/rand"
	"crypto/subtle"
	"encoding/hex"
	"encoding/json"
	"fmt"
	"net"
	"net/http"
	"os"
	"strings"
	"time"

	"github.com/mdp/qrterminal/v3"
	"github.com/urfave/cli/v3"
)

// serveCommand starts an HTTP server that exposes the remote control API over
// the network, so a phone/web client (over Tailscale/SSH-forwarded) can drive
// Blink. All endpoints require a bearer token. This is the backend the PWA
// talks to; the WebSocket terminal and static UI are layered on in later slices.
func serveCommand() *cli.Command {
	return &cli.Command{
		Name:  "serve",
		Usage: "Run the remote control HTTP server for phone/web clients",
		Flags: []cli.Flag{
			&cli.StringFlag{
				Name:  "addr",
				Value: ":7890",
				Usage: "Address to listen on",
			},
			&cli.StringFlag{
				Name:    "token",
				Usage:   "Bearer token clients must present (generated if empty)",
				Sources: cli.EnvVars("BLINK_REMOTE_TOKEN"),
			},
			&cli.StringFlag{
				Name:  "host",
				Usage: "Hostname clients use to reach this server (e.g. your Tailscale name); defaults to the system hostname",
			},
		},
		Action: runServe,
	}
}

func runServe(ctx context.Context, cmd *cli.Command) error {
	addr := cmd.String("addr")
	token := cmd.String("token")
	if token == "" {
		token = generateToken()
	}

	srv := &remoteServer{token: token}
	handler := srv.routes()

	clientURL := buildClientURL(cmd.String("host"), addr, token)
	fmt.Fprintf(os.Stderr, "blink remote server listening on %s\n", addr)
	fmt.Fprintf(os.Stderr, "  url:   %s\n", clientURL)
	fmt.Fprintf(os.Stderr, "  token: %s\n\n", token)
	fmt.Fprintln(os.Stderr, "  Scan to connect (no typing the token on your phone):")
	qrterminal.GenerateHalfBlock(clientURL, qrterminal.L, os.Stderr)

	httpSrv := &http.Server{
		Addr:              addr,
		Handler:           handler,
		ReadHeaderTimeout: 10 * time.Second,
	}
	return httpSrv.ListenAndServe()
}

// buildClientURL assembles the URL a remote client opens, with the token
// embedded so scanning the QR needs no manual typing. host overrides the
// system hostname (use it for your Tailscale name, which resolves remotely).
func buildClientURL(host, addr, token string) string {
	if host == "" {
		host, _ = os.Hostname()
	}
	port := strings.TrimPrefix(addr, ":")
	if _, p, err := net.SplitHostPort(addr); err == nil {
		port = p
	}
	return fmt.Sprintf("http://%s:%s/?token=%s", host, port, token)
}

func generateToken() string {
	b := make([]byte, 24)
	_, _ = rand.Read(b)
	return hex.EncodeToString(b)
}

type remoteServer struct {
	token string
}

func (s *remoteServer) routes() http.Handler {
	mux := http.NewServeMux()
	mux.HandleFunc("GET /{$}", s.handleIndex)
	mux.HandleFunc("GET /api/info", s.handleInfo)
	mux.HandleFunc("GET /api/sessions", s.handleSessions)
	mux.HandleFunc("POST /api/spawn", s.handleSpawn)
	mux.HandleFunc("POST /api/kill", s.handleKill)
	mux.HandleFunc("POST /api/tui", s.handleTUI)
	mux.HandleFunc("GET /ws/session/{name}", s.handleSessionWS)
	return s.auth(mux)
}

// auth enforces the bearer token via the Authorization header or a ?token=
// query param (the latter lets the eventual WebSocket/UI pass it easily).
func (s *remoteServer) auth(next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		provided := r.URL.Query().Get("token")
		if h := r.Header.Get("Authorization"); strings.HasPrefix(h, "Bearer ") {
			provided = strings.TrimPrefix(h, "Bearer ")
		}
		if subtle.ConstantTimeCompare([]byte(provided), []byte(s.token)) != 1 {
			writeJSON(w, http.StatusUnauthorized, map[string]string{"error": "unauthorized"})
			return
		}
		next.ServeHTTP(w, r)
	})
}

// handleIndex serves a minimal placeholder page so scanning the QR lands on a
// real "connected" screen instead of a 404. It proves the transport/auth chain
// works and lists live sessions. Replace this with the real web UI (Slice 3).
func (s *remoteServer) handleIndex(w http.ResponseWriter, r *http.Request) {
	w.Header().Set("Content-Type", "text/html; charset=utf-8")
	_, _ = w.Write([]byte(indexHTML))
}

const indexHTML = `<!doctype html>
<html lang="en">
<head>
<meta charset="utf-8">
<meta name="viewport" content="width=device-width, initial-scale=1">
<title>Blink Remote</title>
<link rel="stylesheet" href="https://cdn.jsdelivr.net/npm/@xterm/xterm@5.5.0/css/xterm.min.css">
<script src="https://cdn.jsdelivr.net/npm/@xterm/xterm@5.5.0/lib/xterm.min.js"></script>
<script src="https://cdn.jsdelivr.net/npm/@xterm/addon-fit@0.10.0/lib/addon-fit.min.js"></script>
<style>
  body { font-family: system-ui, sans-serif; margin: 0; background: #15161a; color: #e6e6e6; }
  header { padding: 16px; background: #1e2027; border-bottom: 1px solid #2c2f3a; }
  h1 { font-size: 18px; margin: 0; }
  .ok { color: #4ade80; }
  main { padding: 16px; }
  .card { background: #1e2027; border: 1px solid #2c2f3a; border-radius: 10px; padding: 12px 14px; margin-bottom: 10px; cursor: pointer; }
  .card:active { background: #262934; }
  .name { font-weight: 600; }
  .meta { color: #9aa0ad; font-size: 13px; margin-top: 4px; }
  .dot { color: #4ade80; }
  .muted { color: #6b7280; font-size: 13px; }
  code { background: #0f1014; padding: 1px 5px; border-radius: 4px; }
  #term-view { position: fixed; inset: 0; background: #000; display: none; flex-direction: column; }
  #term-bar { display: flex; align-items: center; gap: 12px; padding: 10px 14px; background: #1e2027; }
  #term-bar button { background: #2c2f3a; color: #e6e6e6; border: 0; border-radius: 6px; padding: 6px 12px; font-size: 14px; }
  #term { flex: 1; padding: 4px; min-height: 0; }
  #keybar { display: flex; gap: 6px; padding: 8px; background: #1e2027; overflow-x: auto; }
  #keybar button { background: #2c2f3a; color: #e6e6e6; border: 0; border-radius: 6px; padding: 9px 13px; font-size: 14px; white-space: nowrap; }
  #keybar button.active { background: #4ade80; color: #000; }
</style>
</head>
<body>
<header>
  <h1>Blink Remote <span class="ok" id="status">connecting…</span></h1>
  <div class="meta" id="machine"></div>
</header>
<main>
  <div class="card" style="cursor:default">
    <div class="name">Spawn session</div>
    <div style="display:flex; gap:8px; margin-top:8px; flex-wrap:wrap">
      <input id="sp-repo" placeholder="repo path (e.g. ~/proj)" style="flex:1 1 140px; min-width:0; background:#0f1014; color:#e6e6e6; border:1px solid #2c2f3a; border-radius:6px; padding:8px">
      <input id="sp-branch" placeholder="branch" style="flex:1 1 100px; min-width:0; background:#0f1014; color:#e6e6e6; border:1px solid #2c2f3a; border-radius:6px; padding:8px">
      <button id="sp-go" style="background:#4ade80; color:#000; border:0; border-radius:6px; padding:8px 14px; font-weight:600">Spawn ▸</button>
    </div>
    <div id="sp-msg" class="muted" style="margin-top:6px"></div>
  </div>
  <div class="card" style="cursor:default">
    <div class="name">Blink TUI</div>
    <div class="meta" style="margin-bottom:8px">Open the full Blink app (folder tree, worktrees, session switching) on your phone.</div>
    <div style="display:flex; gap:8px; flex-wrap:wrap">
      <input id="tui-dir" placeholder="folder (optional, default: home)" style="flex:1 1 160px; min-width:0; background:#0f1014; color:#e6e6e6; border:1px solid #2c2f3a; border-radius:6px; padding:8px">
      <button id="tui-go" style="background:#60a5fa; color:#000; border:0; border-radius:6px; padding:8px 14px; font-weight:600">Open TUI ▸</button>
    </div>
  </div>
  <div id="sessions"></div>
  <p class="muted">Placeholder page. The full UI goes here — it talks to <code>/api/*</code> and <code>/ws/*</code>.</p>
</main>

<div id="term-view">
  <div id="term-bar">
    <button id="term-close">‹ Back</button>
    <span id="term-name" class="name"></span>
  </div>
  <div id="term"></div>
  <div id="keybar">
    <button data-k="esc">Esc</button>
    <button data-k="ctrl" id="kb-ctrl">Ctrl</button>
    <button data-k="tab">Tab</button>
    <button data-k="up">↑</button>
    <button data-k="down">↓</button>
    <button data-k="left">←</button>
    <button data-k="right">→</button>
    <button data-k="pipe">|</button>
    <button data-k="tilde">~</button>
  </div>
</div>

<script>
  const token = new URLSearchParams(location.search).get('token');
  const opts = { headers: { 'Authorization': 'Bearer ' + token } };

  let term, fit, ws, ctrlActive = false;
  function wsSend(d) {
    if (ws && ws.readyState === 1) ws.send(typeof d === 'string' ? new TextEncoder().encode(d) : d);
  }
  function setCtrl(v) {
    ctrlActive = v;
    document.getElementById('kb-ctrl').classList.toggle('active', v);
  }

  // Helper keys phones lack (Esc/Ctrl/Tab/arrows). Ctrl is a one-shot modifier:
  // tap Ctrl then a letter to send the control character (e.g. Ctrl then C = ^C).
  document.getElementById('keybar').addEventListener('click', (e) => {
    const k = e.target.getAttribute('data-k');
    if (!k) return;
    if (k === 'ctrl') { setCtrl(!ctrlActive); return; }
    const map = { esc: '\x1b', tab: '\t', up: '\x1b[A', down: '\x1b[B', right: '\x1b[C', left: '\x1b[D', pipe: '|', tilde: '~' };
    if (map[k]) { wsSend(map[k]); if (term) term.focus(); }
  });

  function openTerminal(name) {
    document.getElementById('term-name').textContent = name;
    document.getElementById('term-view').style.display = 'flex';

    term = new Terminal({ cursorBlink: true, fontSize: 14, theme: { background: '#000000' } });
    fit = new FitAddon.FitAddon();
    term.loadAddon(fit);
    term.open(document.getElementById('term'));
    fit.fit();

    const proto = location.protocol === 'https:' ? 'wss' : 'ws';
    ws = new WebSocket(proto + '://' + location.host + '/ws/session/' + encodeURIComponent(name) + '?token=' + token);
    ws.binaryType = 'arraybuffer';

    const sendResize = () => {
      fit.fit();
      if (ws.readyState === 1) ws.send(JSON.stringify({ type: 'resize', cols: term.cols, rows: term.rows }));
    };
    ws.onopen = sendResize;
    ws.onmessage = (e) => term.write(new Uint8Array(e.data));
    ws.onclose = () => term.write('\r\n[disconnected]\r\n');
    term.onData((d) => {
      if (ctrlActive && d.length === 1) {
        const c = d.toUpperCase().charCodeAt(0);
        setCtrl(false);
        if (c >= 64 && c <= 95) { wsSend(String.fromCharCode(c & 31)); return; } // ^A..^_
      }
      wsSend(d);
    });
    window.addEventListener('resize', sendResize);
  }

  function closeTerminal() {
    setCtrl(false);
    if (ws) ws.close();
    if (term) term.dispose();
    document.getElementById('term-view').style.display = 'none';
    load();
  }
  document.getElementById('term-close').addEventListener('click', closeTerminal);

  document.getElementById('sp-go').addEventListener('click', async () => {
    const repo = document.getElementById('sp-repo').value.trim();
    const branch = document.getElementById('sp-branch').value.trim();
    const msg = document.getElementById('sp-msg');
    if (!repo || !branch) { msg.textContent = 'repo and branch required'; return; }
    msg.textContent = 'spawning…';
    try {
      const res = await fetch('/api/spawn', {
        method: 'POST',
        headers: { 'Authorization': 'Bearer ' + token, 'Content-Type': 'application/json' },
        body: JSON.stringify({ repo, branch }),
      });
      const data = await res.json();
      if (!res.ok) { msg.textContent = 'error: ' + (data.error || res.status); return; }
      msg.textContent = 'created ' + data.session;
      document.getElementById('sp-branch').value = '';
      load();
    } catch (e) { msg.textContent = String(e); }
  });

  document.getElementById('tui-go').addEventListener('click', async () => {
    const dir = document.getElementById('tui-dir').value.trim();
    try {
      const res = await fetch('/api/tui', {
        method: 'POST',
        headers: { 'Authorization': 'Bearer ' + token, 'Content-Type': 'application/json' },
        body: JSON.stringify({ dir }),
      });
      const data = await res.json();
      if (!res.ok) { alert('error: ' + (data.error || res.status)); return; }
      openTerminal(data.session);
    } catch (e) { alert(String(e)); }
  });

  async function load() {
    try {
      const info = await (await fetch('/api/info', opts)).json();
      document.getElementById('status').textContent = 'connected ✓';
      document.getElementById('machine').textContent =
        info.hostname + ' · ' + info.user + ' · ' + info.os + '/' + info.arch + ' · blink ' + info.version;
      const sessions = await (await fetch('/api/sessions', opts)).json();
      const el = document.getElementById('sessions');
      if (!sessions.length) { el.innerHTML = '<p class="muted">No active sessions. Spawn one with the CLI, then refresh.</p>'; return; }
      el.innerHTML = '';
      sessions.forEach((sn) => {
        const card = document.createElement('div');
        card.className = 'card';
        card.innerHTML = '<div class="name">' + sn.name + (sn.attached ? ' <span class="dot">●</span>' : '') +
          '</div><div class="meta">' + sn.path + ' · ' + sn.windows + ' win · tap to open</div>';
        card.addEventListener('click', () => openTerminal(sn.name));
        el.appendChild(card);
      });
    } catch (e) {
      document.getElementById('status').textContent = 'error';
      document.getElementById('status').className = '';
      document.getElementById('machine').textContent = String(e);
    }
  }
  load();
</script>
</body>
</html>`

func (s *remoteServer) handleInfo(w http.ResponseWriter, r *http.Request) {
	writeJSON(w, http.StatusOK, gatherMachineInfo(r.Context()))
}

func (s *remoteServer) handleSessions(w http.ResponseWriter, r *http.Request) {
	sessions, err := listSessionInfos(r.Context())
	if err != nil {
		writeJSON(w, http.StatusInternalServerError, map[string]string{"error": err.Error()})
		return
	}
	writeJSON(w, http.StatusOK, sessions)
}

func (s *remoteServer) handleSpawn(w http.ResponseWriter, r *http.Request) {
	var body struct {
		Repo   string `json:"repo"`
		Branch string `json:"branch"`
		Base   string `json:"base"`
	}
	if err := json.NewDecoder(r.Body).Decode(&body); err != nil {
		writeJSON(w, http.StatusBadRequest, map[string]string{"error": "invalid json"})
		return
	}
	if body.Repo == "" || body.Branch == "" {
		writeJSON(w, http.StatusBadRequest, map[string]string{"error": "repo and branch are required"})
		return
	}
	res, err := spawnSession(r.Context(), body.Repo, body.Branch, body.Base)
	if err != nil {
		writeJSON(w, http.StatusBadRequest, map[string]string{"error": err.Error()})
		return
	}
	writeJSON(w, http.StatusOK, res)
}

func (s *remoteServer) handleTUI(w http.ResponseWriter, r *http.Request) {
	var body struct {
		Dir string `json:"dir"`
	}
	_ = json.NewDecoder(r.Body).Decode(&body)
	res, err := launchTUI(r.Context(), body.Dir)
	if err != nil {
		writeJSON(w, http.StatusBadRequest, map[string]string{"error": err.Error()})
		return
	}
	writeJSON(w, http.StatusOK, res)
}

func (s *remoteServer) handleKill(w http.ResponseWriter, r *http.Request) {
	var body struct {
		Session string `json:"session"`
	}
	if err := json.NewDecoder(r.Body).Decode(&body); err != nil {
		writeJSON(w, http.StatusBadRequest, map[string]string{"error": "invalid json"})
		return
	}
	if body.Session == "" {
		writeJSON(w, http.StatusBadRequest, map[string]string{"error": "session is required"})
		return
	}
	if err := killSession(r.Context(), body.Session); err != nil {
		writeJSON(w, http.StatusBadRequest, map[string]string{"error": err.Error()})
		return
	}
	writeJSON(w, http.StatusOK, map[string]string{"killed": body.Session})
}

func writeJSON(w http.ResponseWriter, status int, v any) {
	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(status)
	_ = json.NewEncoder(w).Encode(v)
}
