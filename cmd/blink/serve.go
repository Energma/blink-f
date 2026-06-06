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

	"github.com/Energma/blink-f/internal/config"
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
				Usage: "Address to listen on (overrides remote.addr; default :7890)",
			},
			&cli.StringFlag{
				Name:    "token",
				Usage:   "Bearer token clients must present (overrides remote.token; generated if unset)",
				Sources: cli.EnvVars("BLINK_REMOTE_TOKEN"),
			},
			&cli.StringFlag{
				Name:  "host",
				Usage: "Hostname clients use to reach this server (overrides remote.host); defaults to the system hostname",
			},
		},
		Action: runServe,
	}
}

func runServe(ctx context.Context, cmd *cli.Command) error {
	cfg, _ := config.Load()
	rc := cfg.Remote

	// Precedence: explicit flag/env > remote: config block > built-in default.
	addr := firstNonEmpty(cmd.String("addr"), rc.Addr, ":7890")
	host := firstNonEmpty(cmd.String("host"), rc.Host)
	token := firstNonEmpty(cmd.String("token"), rc.Token)
	if token == "" {
		token = generateToken()
	}
	claudeCmd := firstNonEmpty(rc.ClaudeCmd, "claude")

	srv := &remoteServer{token: token, claudeCmd: claudeCmd}
	handler := srv.routes()

	clientURL := buildClientURL(host, addr, token)
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
	token     string
	claudeCmd string
}

// firstNonEmpty returns the first non-empty string in vals, or "" if none.
func firstNonEmpty(vals ...string) string {
	for _, v := range vals {
		if v != "" {
			return v
		}
	}
	return ""
}

func (s *remoteServer) routes() http.Handler {
	mux := http.NewServeMux()
	mux.HandleFunc("GET /{$}", s.handleIndex)
	mux.HandleFunc("GET /api/info", s.handleInfo)
	mux.HandleFunc("GET /api/sessions", s.handleSessions)
	mux.HandleFunc("GET /api/repos", s.handleRepos)
	mux.HandleFunc("POST /api/spawn", s.handleSpawn)
	mux.HandleFunc("POST /api/kill", s.handleKill)
	mux.HandleFunc("POST /api/tui", s.handleTUI)
	mux.HandleFunc("POST /api/run", s.handleRun)
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
  #tui-keys { display: flex; gap: 6px; padding: 8px; background: #14223a; overflow-x: auto; }
  #tui-keys button { background: #1e3a5f; color: #cfe3ff; border: 0; border-radius: 6px; padding: 9px 12px; font-size: 14px; white-space: nowrap; }
  .scard { display: flex; align-items: center; gap: 10px; }
  .scard > div { flex: 1; min-width: 0; overflow: hidden; }
  .kill { background: #3a2626; color: #f87171; border: 0; border-radius: 6px; padding: 8px 12px; font-size: 14px; }
  .badge { background: #1e3a5f; color: #60a5fa; font-size: 11px; padding: 1px 6px; border-radius: 10px; }
  .shead { color: #9aa0ad; font-size: 12px; text-transform: uppercase; letter-spacing: .05em; margin: 16px 2px 6px; }
  .repos { display: flex; gap: 6px; flex-wrap: wrap; margin-bottom: 8px; }
  .repos button { background: #2c2f3a; color: #cfe3ff; border: 1px solid #3a4150; border-radius: 14px; padding: 6px 12px; font-size: 13px; }
  .repos button.active { background: #1e3a5f; color: #fff; border-color: #60a5fa; }
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
    <div class="name">Quick launch</div>
    <div class="meta" style="margin-bottom:8px">Open a full terminal on your phone — Claude Code, a shell, or the Blink TUI — in any folder. Each launch is its own window you can reopen from the list below.</div>
    <div id="ql-repos" class="repos"></div>
    <input id="ql-dir" placeholder="folder (tap a repo above, or type a path; default: home)" style="width:100%; box-sizing:border-box; background:#0f1014; color:#e6e6e6; border:1px solid #2c2f3a; border-radius:6px; padding:8px; margin-bottom:8px">
    <div style="display:flex; gap:8px; flex-wrap:wrap">
      <button class="launch" data-cmd="claude" data-label="claude" style="background:#c98bdb; color:#000; border:0; border-radius:6px; padding:9px 14px; font-weight:600">Claude Code ▸</button>
      <button class="launch" data-cmd="" data-label="sh" style="background:#4ade80; color:#000; border:0; border-radius:6px; padding:9px 14px; font-weight:600">Shell ▸</button>
      <button id="tui-go" style="background:#60a5fa; color:#000; border:0; border-radius:6px; padding:9px 14px; font-weight:600">Blink TUI ▸</button>
    </div>
    <div id="ql-msg" class="muted" style="margin-top:6px"></div>
  </div>
  <div id="sessions"></div>
</main>

<div id="term-view">
  <div id="term-bar">
    <button id="term-close">‹ Back</button>
    <span id="term-name" class="name"></span>
  </div>
  <div id="term"></div>
  <div id="tui-keys" style="display:none">
    <button data-send="tab">⇥ repos</button>
    <button data-send="S">▤ sessions</button>
    <button data-send="l">→ files</button>
    <button data-send="esc">‹ back</button>
    <button data-send="k">↑</button>
    <button data-send="j">↓</button>
    <button data-send="enter">⏎ select</button>
    <button data-send="r">⟳ refresh</button>
    <button data-send="?">? help</button>
    <button data-send="n">n new</button>
    <button data-send="b">b branch</button>
    <button data-send="d">d del</button>
    <button data-send="D">D clean</button>
    <button data-send="c">c commit</button>
    <button data-send="p">p push</button>
    <button data-send="u">u pull</button>
    <button data-send="s">s stash</button>
  </div>
  <div id="keybar">
    <button data-k="esc">Esc</button>
    <button data-k="enter">⏎</button>
    <button data-k="cc">^C</button>
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
    const map = { esc: '\x1b', enter: '\r', cc: '\x03', tab: '\t', up: '\x1b[A', down: '\x1b[B', right: '\x1b[C', left: '\x1b[D', pipe: '|', tilde: '~' };
    if (map[k]) { wsSend(map[k]); if (term) term.focus(); }
  });

  // Blink TUI navigation buttons (shown only for TUI sessions) — send the
  // app's own keys so you can drive it without typing on a phone.
  const tuiKeyMap = { enter: '\r', tab: '\t', esc: '\x1b', up: '\x1b[A', down: '\x1b[B', left: '\x1b[D', right: '\x1b[C' };
  document.getElementById('tui-keys').addEventListener('click', (e) => {
    const v = e.target.getAttribute('data-send');
    if (v == null) return;
    wsSend(tuiKeyMap[v] || v);
    if (term) term.focus();
  });

  function renderSession(sn) {
    const isTui = sn.name.indexOf('blink-tui') === 0;
    const isClaude = sn.name.indexOf('claude') === 0;
    const card = document.createElement('div');
    card.className = 'card scard';
    const info = document.createElement('div');
    const badge = isTui ? ' <span class="badge">TUI</span>' : (isClaude ? ' <span class="badge">Claude</span>' : '');
    info.innerHTML = '<div class="name">' + sn.name + (sn.attached ? ' <span class="dot">●</span>' : '') +
      badge +
      '</div><div class="meta">' + sn.path + ' · ' + sn.windows + ' win · tap to open</div>';
    card.appendChild(info);
    card.addEventListener('click', () => openTerminal(sn.name));
    const kill = document.createElement('button');
    kill.className = 'kill';
    kill.textContent = '✕';
    kill.addEventListener('click', (ev) => killSessionUI(sn.name, ev));
    card.appendChild(kill);
    return card;
  }

  async function killSessionUI(name, ev) {
    ev.stopPropagation();
    if (!confirm('Close ' + name + '?')) return;
    try {
      await fetch('/api/kill', {
        method: 'POST',
        headers: { 'Authorization': 'Bearer ' + token, 'Content-Type': 'application/json' },
        body: JSON.stringify({ session: name }),
      });
      load();
    } catch (e) { alert(String(e)); }
  }

  function openTerminal(name) {
    document.getElementById('term-name').textContent = name;
    document.getElementById('term-view').style.display = 'flex';
    document.getElementById('tui-keys').style.display = name.indexOf('blink-tui') === 0 ? 'flex' : 'none';

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

  async function launch(path, payload) {
    const msg = document.getElementById('ql-msg');
    msg.textContent = 'launching…';
    try {
      const res = await fetch(path, {
        method: 'POST',
        headers: { 'Authorization': 'Bearer ' + token, 'Content-Type': 'application/json' },
        body: JSON.stringify(payload),
      });
      const data = await res.json();
      if (!res.ok) { msg.textContent = 'error: ' + (data.error || res.status); return; }
      msg.textContent = '';
      openTerminal(data.session);
    } catch (e) { msg.textContent = String(e); }
  }

  document.querySelectorAll('.launch').forEach((btn) => {
    btn.addEventListener('click', () => {
      const dir = document.getElementById('ql-dir').value.trim();
      launch('/api/run', { dir, cmd: btn.getAttribute('data-cmd'), label: btn.getAttribute('data-label') });
    });
  });

  document.getElementById('tui-go').addEventListener('click', () => {
    const dir = document.getElementById('ql-dir').value.trim();
    launch('/api/tui', { dir });
  });

  // Configured repos as quick picks — tap one to target it, then launch.
  async function loadRepos() {
    try {
      const repos = await (await fetch('/api/repos', opts)).json();
      const box = document.getElementById('ql-repos');
      box.innerHTML = '';
      const dirInput = document.getElementById('ql-dir');
      repos.forEach((rp) => {
        const b = document.createElement('button');
        b.textContent = rp.name;
        b.title = rp.path;
        b.addEventListener('click', () => {
          dirInput.value = rp.path;
          box.querySelectorAll('button').forEach((x) => x.classList.remove('active'));
          b.classList.add('active');
        });
        box.appendChild(b);
      });
    } catch (e) { /* repos are optional */ }
  }

  async function load() {
    try {
      const info = await (await fetch('/api/info', opts)).json();
      document.getElementById('status').textContent = 'connected ✓';
      document.getElementById('machine').textContent =
        info.hostname + ' · ' + info.user + ' · ' + info.os + '/' + info.arch + ' · blink ' + info.version;
      if (info.claudeCmd) {
        const cb = document.querySelector('.launch[data-label="claude"]');
        if (cb) cb.setAttribute('data-cmd', info.claudeCmd);
      }
      const sessions = await (await fetch('/api/sessions', opts)).json();
      const el = document.getElementById('sessions');
      el.innerHTML = '';
      if (!sessions.length) { el.innerHTML = '<p class="muted">No sessions yet. Spawn one or open a TUI above.</p>'; return; }
      const isTui = (n) => n.indexOf('blink-tui') === 0;
      const isClaude = (n) => n.indexOf('claude') === 0;
      const section = (title, list) => {
        if (!list.length) return;
        const h = document.createElement('div'); h.className = 'shead'; h.textContent = title;
        el.appendChild(h);
        list.forEach((sn) => el.appendChild(renderSession(sn)));
      };
      section('Blink TUIs', sessions.filter((s) => isTui(s.name)));
      section('Claude', sessions.filter((s) => isClaude(s.name)));
      section('Sessions', sessions.filter((s) => !isTui(s.name) && !isClaude(s.name)));
    } catch (e) {
      document.getElementById('status').textContent = 'error';
      document.getElementById('status').className = '';
      document.getElementById('machine').textContent = String(e);
    }
  }
  load();
  loadRepos();
</script>
</body>
</html>`

func (s *remoteServer) handleInfo(w http.ResponseWriter, r *http.Request) {
	writeJSON(w, http.StatusOK, struct {
		machineInfo
		ClaudeCmd string `json:"claudeCmd"`
	}{gatherMachineInfo(r.Context()), s.claudeCmd})
}

func (s *remoteServer) handleSessions(w http.ResponseWriter, r *http.Request) {
	sessions, err := listSessionInfos(r.Context())
	if err != nil {
		writeJSON(w, http.StatusInternalServerError, map[string]string{"error": err.Error()})
		return
	}
	writeJSON(w, http.StatusOK, sessions)
}

func (s *remoteServer) handleRepos(w http.ResponseWriter, r *http.Request) {
	writeJSON(w, http.StatusOK, listRepos())
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

func (s *remoteServer) handleRun(w http.ResponseWriter, r *http.Request) {
	var body struct {
		Dir   string `json:"dir"`
		Cmd   string `json:"cmd"`
		Label string `json:"label"`
	}
	_ = json.NewDecoder(r.Body).Decode(&body)
	res, err := runProgram(r.Context(), body.Dir, body.Cmd, body.Label)
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
