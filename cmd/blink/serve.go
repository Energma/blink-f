package main

import (
	"context"
	"crypto/rand"
	"crypto/subtle"
	"embed"
	"encoding/hex"
	"encoding/json"
	"fmt"
	"io/fs"
	"net"
	"net/http"
	"os"
	"strings"
	"time"

	"github.com/mdp/qrterminal/v3"
	"github.com/urfave/cli/v3"

	"github.com/Energma/blink-f/internal/config"
)

// webAssets holds the xterm.js bundle so the mobile UI loads everything from
// this server (over Tailscale) instead of a public CDN — much faster and
// reliable on flaky cellular, and it works even with no outbound internet.
//
//go:embed assets/*
var webAssets embed.FS

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
	mux.HandleFunc("GET /api/ls", s.handleLs)
	mux.HandleFunc("GET /api/worktrees", s.handleWorktrees)
	mux.HandleFunc("POST /api/spawn", s.handleSpawn)
	mux.HandleFunc("POST /api/kill", s.handleKill)
	mux.HandleFunc("POST /api/tui", s.handleTUI)
	mux.HandleFunc("POST /api/run", s.handleRun)
	mux.HandleFunc("GET /ws/session/{name}", s.handleSessionWS)
	if sub, err := fs.Sub(webAssets, "assets"); err == nil {
		mux.Handle("GET /assets/", http.StripPrefix("/assets/", http.FileServerFS(sub)))
	}
	return withCORS(s.auth(mux))
}

// withCORS lets one machine's web UI call another machine's API cross-origin
// (so you can manage several PCs from a single page). Auth is still by token, so
// a permissive origin is fine; preflight OPTIONS is answered before auth since
// browsers send it without the Authorization header.
func withCORS(next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Access-Control-Allow-Origin", "*")
		w.Header().Set("Access-Control-Allow-Methods", "GET, POST, OPTIONS")
		w.Header().Set("Access-Control-Allow-Headers", "Authorization, Content-Type")
		if r.Method == http.MethodOptions {
			w.WriteHeader(http.StatusNoContent)
			return
		}
		next.ServeHTTP(w, r)
	})
}

// auth enforces the bearer token via the Authorization header or a ?token=
// query param (the latter lets the eventual WebSocket/UI pass it easily).
func (s *remoteServer) auth(next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		// Static assets (the xterm.js bundle) carry no secrets and are loaded by
		// <script>/<link> tags that can't send a token; serve them unauthenticated.
		if strings.HasPrefix(r.URL.Path, "/assets/") {
			next.ServeHTTP(w, r)
			return
		}
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

// handleIndex serves the mobile web UI. no-store keeps phones from holding onto
// a stale cached page across server upgrades (otherwise new buttons/fixes never
// reach a browser that cached an older version).
func (s *remoteServer) handleIndex(w http.ResponseWriter, r *http.Request) {
	w.Header().Set("Content-Type", "text/html; charset=utf-8")
	w.Header().Set("Cache-Control", "no-store, must-revalidate")
	_, _ = w.Write([]byte(indexHTML))
}

const indexHTML = `<!doctype html>
<html lang="en">
<head>
<meta charset="utf-8">
<meta name="viewport" content="width=device-width, initial-scale=1">
<title>Blink Remote</title>
<link rel="stylesheet" href="/assets/xterm.min.css">
<script defer src="/assets/xterm.min.js"></script>
<script defer src="/assets/addon-fit.min.js"></script>
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
  #term-bar { display: flex; align-items: center; gap: 8px; padding: 10px 14px; background: #1e2027; }
  #term-bar button { background: #2c2f3a; color: #e6e6e6; border: 0; border-radius: 6px; padding: 6px 12px; font-size: 14px; }
  #term-name { flex: 1; min-width: 0; overflow: hidden; text-overflow: ellipsis; white-space: nowrap; }
  #term-switch-list { display: none; position: absolute; top: 52px; left: 8px; right: 8px; max-height: 60vh; overflow-y: auto; background: #1e2027; border: 1px solid #2c2f3a; border-radius: 10px; z-index: 20; padding: 6px; }
  .swrow { display: block; width: 100%; text-align: left; background: #15161a; color: #e6e6e6; border: 1px solid #2c2f3a; border-radius: 8px; padding: 10px 12px; margin-bottom: 6px; font-size: 14px; }
  #term { flex: 1; padding: 4px; min-height: 0; }
  #kbd { display: none; background: #0f1014; padding: 5px 4px calc(5px + env(safe-area-inset-bottom)); }
  .kbrow { display: flex; gap: 4px; margin-bottom: 4px; justify-content: center; }
  .kbkey { flex: 1; min-width: 0; background: #2c2f3a; color: #e6e6e6; border: 0; border-radius: 6px; padding: 11px 0; font-size: 15px; }
  .kbkey:active { background: #4a5163; }
  .kbkey.kbspecial { background: #3a4150; color: #cfe3ff; flex: 1.3; font-size: 13px; }
  .kbkey.kbspace { flex: 4; }
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
  .machbar { display: flex; gap: 6px; flex-wrap: wrap; margin-top: 10px; }
  .mchip { background: #2c2f3a; color: #e6e6e6; border: 1px solid #3a4150; border-radius: 14px; padding: 5px 12px; font-size: 13px; }
  .mchip.active { background: #14532d; color: #fff; border-color: #4ade80; }
  #browse-view { position: fixed; inset: 0; background: #15161a; display: none; flex-direction: column; }
  #browse-bar { display: flex; align-items: center; gap: 12px; padding: 10px 14px; background: #1e2027; }
  #browse-bar button { background: #2c2f3a; color: #e6e6e6; border: 0; border-radius: 6px; padding: 6px 12px; font-size: 14px; }
  #browse-path { font-size: 12px; color: #9aa0ad; overflow: hidden; text-overflow: ellipsis; white-space: nowrap; direction: rtl; }
  #browse-actions { display: flex; align-items: center; gap: 6px; padding: 8px 14px; background: #14223a; }
  #browse-actions .lbl { flex: 1; color: #9aa0ad; font-size: 13px; }
  #browse-actions button { background: #1e3a5f; color: #cfe3ff; border: 0; border-radius: 6px; padding: 8px 12px; font-size: 14px; }
  #browse-git { padding: 8px 14px 0; }
  #browse-git .git-head { display: flex; align-items: center; gap: 10px; color: #9aa0ad; font-size: 12px; text-transform: uppercase; letter-spacing: .05em; margin: 6px 2px; }
  #browse-git .nb { background: #1e3a5f; color: #cfe3ff; border: 0; border-radius: 6px; padding: 6px 10px; font-size: 13px; }
  #browse-git .wt { display: flex; align-items: center; gap: 8px; background: #1e2027; border: 1px solid #2c2f3a; border-radius: 8px; padding: 8px 10px; margin-bottom: 6px; }
  #browse-git .wt .b { color: #cfe3ff; font-weight: 600; }
  #browse-git .wt .open { margin-left: auto; background: #2c2f3a; color: #e6e6e6; border: 0; border-radius: 6px; padding: 6px 10px; font-size: 13px; }
  #browse-list { flex: 1; overflow-y: auto; padding: 10px 14px; }
  #ql-browse { width: 100%; box-sizing: border-box; background: #2c2f3a; color: #cfe3ff; border: 1px solid #3a4150; border-radius: 6px; padding: 8px; margin-bottom: 8px; font-size: 14px; }
</style>
</head>
<body>
<header>
  <h1>Blink Remote <span class="ok" id="status">connecting…</span></h1>
  <div class="meta" id="machine"></div>
  <div class="machbar" id="machbar"></div>
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
    <button id="ql-browse">📁 Browse files…</button>
    <div style="display:flex; gap:8px; flex-wrap:wrap">
      <button class="launch" data-cmd="claude" data-label="claude" style="background:#c98bdb; color:#000; border:0; border-radius:6px; padding:9px 14px; font-weight:600">Claude Code ▸</button>
      <button class="launch" data-cmd="" data-label="sh" style="background:#4ade80; color:#000; border:0; border-radius:6px; padding:9px 14px; font-weight:600">Shell ▸</button>
      <button id="tui-go" style="background:#60a5fa; color:#000; border:0; border-radius:6px; padding:9px 14px; font-weight:600">Blink TUI ▸</button>
    </div>
    <div id="ql-msg" class="muted" style="margin-top:6px"></div>
  </div>
  <div id="sessions"></div>
</main>

<div id="browse-view">
  <div id="browse-bar">
    <button id="browse-close">‹ Back</button>
    <span id="browse-path"></span>
  </div>
  <div id="browse-actions">
    <button id="browse-up">⬆ up</button>
    <span class="lbl">open here ▸</span>
    <button class="bopen" data-act="claude">Claude</button>
    <button class="bopen" data-act="sh">Shell</button>
    <button class="bopen" data-act="tui">TUI</button>
  </div>
  <div id="browse-git"></div>
  <div id="browse-list"></div>
</div>

<div id="term-view">
  <div id="term-bar">
    <button id="term-close">‹ Back</button>
    <span id="term-name" class="name"></span>
    <button id="term-switch" title="switch window">⇄</button>
    <button id="term-kbd" title="toggle keyboard">⌨</button>
  </div>
  <div id="term-switch-list"></div>
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
  <div id="kbd"></div>
</div>

<script>
  // ---- Machines: control several PCs from one page. Each machine is a
  // blink serve URL + token; calls below target the active machine, cross-origin
  // over the tailnet (the server sends permissive CORS). The list lives in
  // localStorage of whichever page you opened ("home" machine).
  function loadMachines() { try { return JSON.parse(localStorage.getItem('blinkMachines')) || []; } catch (e) { return []; } }
  function saveMachines() { localStorage.setItem('blinkMachines', JSON.stringify(machines)); }
  let machines = loadMachines();
  let active = 0;
  (function ensureSelf() {
    const selfTok = new URLSearchParams(location.search).get('token') || '';
    const i = machines.findIndex((m) => m.base === location.origin);
    if (i === -1) { machines.unshift({ name: location.hostname, base: location.origin, token: selfTok }); active = 0; }
    else { if (selfTok) machines[i].token = selfTok; active = i; }
    saveMachines();
  })();
  function M() { return machines[active] || { name: '?', base: location.origin, token: '' }; }
  function aopts() { return { headers: { 'Authorization': 'Bearer ' + M().token } }; }
  function jhdr() { return { 'Authorization': 'Bearer ' + M().token, 'Content-Type': 'application/json' }; }
  function api(p) { return M().base + p; }
  function qtok() { return 'token=' + encodeURIComponent(M().token); }
  function wsBase() { return M().base.replace(/^http/, 'ws'); }

  function parseMachine(url) {
    try { const u = new URL(url.trim()); return { name: u.hostname, base: u.origin, token: u.searchParams.get('token') || '' }; }
    catch (e) { return null; }
  }
  async function addMachine() {
    const url = prompt("Paste the other PC's Blink URL (from its QR / serve output):");
    if (!url) return;
    const m = parseMachine(url);
    if (!m || !m.token) { alert('Could not read a URL with a token in it.'); return; }
    try { const info = await (await fetch(m.base + '/api/info', { headers: { 'Authorization': 'Bearer ' + m.token } })).json(); if (info.hostname) m.name = info.hostname; } catch (e) { }
    const i = machines.findIndex((x) => x.base === m.base);
    if (i >= 0) machines[i] = m; else machines.push(m);
    active = machines.findIndex((x) => x.base === m.base);
    saveMachines(); switchMachine(active);
  }
  function switchMachine(i) { active = i; saveMachines(); renderMachines(); load(); loadRepos(); }
  function renderMachines() {
    const bar = document.getElementById('machbar');
    bar.innerHTML = '';
    machines.forEach((m, i) => {
      const b = document.createElement('button');
      b.className = 'mchip' + (i === active ? ' active' : '');
      b.textContent = m.name;
      b.addEventListener('click', () => switchMachine(i));
      b.addEventListener('contextmenu', (e) => {
        e.preventDefault();
        if (m.base === location.origin) { alert('This is the machine you are connected to; open its own page to remove.'); return; }
        if (confirm('Remove ' + m.name + '?')) { machines.splice(i, 1); if (active >= machines.length) active = machines.length - 1; saveMachines(); switchMachine(active); }
      });
      bar.appendChild(b);
    });
    const add = document.createElement('button');
    add.className = 'mchip'; add.textContent = '＋ PC';
    add.addEventListener('click', addMachine);
    bar.appendChild(add);
  }

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
      await fetch(api('/api/kill'), {
        method: 'POST',
        headers: jhdr(),
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
    applyKbMode(); // suppress the device keyboard, show the on-screen one

    ws = new WebSocket(wsBase() + '/ws/session/' + encodeURIComponent(name) + '?' + qtok());
    ws.binaryType = 'arraybuffer';

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
  }

  // Single persistent resize handler over the current term/ws (re-adding one per
  // openTerminal leaked stale listeners that fired fit() on disposed terminals).
  function sendResize() {
    if (!term || !fit) return;
    try { fit.fit(); } catch (e) { return; }
    if (ws && ws.readyState === 1) ws.send(JSON.stringify({ type: 'resize', cols: term.cols, rows: term.rows }));
  }
  window.addEventListener('resize', sendResize);

  function closeTerminal() {
    setCtrl(false);
    if (ws) ws.close();
    if (term) term.dispose();
    document.getElementById('term-view').style.display = 'none';
    load();
  }
  document.getElementById('term-close').addEventListener('click', () => {
    document.getElementById('term-switch-list').style.display = 'none';
    closeTerminal();
  });

  // ---- On-screen keyboard. The device keyboard is hard to use for a terminal
  // (autocorrect, no Esc/Tab/Ctrl, covers the screen), so we suppress it and
  // drive the PTY from these keys. ⌨ toggles back to the native keyboard.
  const KB_LOWER = [
    ['1','2','3','4','5','6','7','8','9','0'],
    ['q','w','e','r','t','y','u','i','o','p'],
    ['a','s','d','f','g','h','j','k','l'],
    ['{shift}','z','x','c','v','b','n','m','{bs}'],
    ['{sym}','{esc}','{tab}','{space}','{up}','{down}','{ent}'],
  ];
  const KB_UPPER = [
    ['1','2','3','4','5','6','7','8','9','0'],
    ['Q','W','E','R','T','Y','U','I','O','P'],
    ['A','S','D','F','G','H','J','K','L'],
    ['{shift}','Z','X','C','V','B','N','M','{bs}'],
    ['{sym}','{esc}','{tab}','{space}','{up}','{down}','{ent}'],
  ];
  const KB_SYM = [
    ['!','@','#','$','%','^','&','*','(',')'],
    ['-','_','=','+','[',']','{','}','|','\\'],
    ['~','{bt}',"'",'"',':',';','/','?','<','>'],
    ['{abc}','.',',','{left}','{right}','{bs}'],
    ['{abc}','{esc}','{tab}','{space}','{up}','{down}','{ent}'],
  ];
  const KB_LABEL = { '{shift}':'⇧','{bs}':'⌫','{sym}':'?#!','{abc}':'abc','{esc}':'esc','{tab}':'tab','{space}':'space','{up}':'↑','{down}':'↓','{left}':'←','{right}':'→','{ent}':'⏎','{bt}':String.fromCharCode(96) };
  let kbLayer = 'lower', kbShift = false;
  function kbLayout() { return kbLayer === 'sym' ? KB_SYM : (kbShift ? KB_UPPER : KB_LOWER); }
  function renderKbd() {
    const el = document.getElementById('kbd');
    el.innerHTML = '';
    kbLayout().forEach((row) => {
      const r = document.createElement('div');
      r.className = 'kbrow';
      row.forEach((key) => {
        const b = document.createElement('button');
        const special = key.charAt(0) === '{';
        b.className = 'kbkey' + (special ? ' kbspecial' : '') + (key === '{space}' ? ' kbspace' : '');
        b.textContent = KB_LABEL[key] || key;
        b.addEventListener('click', () => kbPress(key));
        r.appendChild(b);
      });
      el.appendChild(r);
    });
  }
  function kbPress(key) {
    switch (key) {
      case '{shift}': kbShift = !kbShift; renderKbd(); return;
      case '{sym}': kbLayer = 'sym'; renderKbd(); return;
      case '{abc}': kbLayer = 'lower'; renderKbd(); return;
      case '{bs}': wsSend('\x7f'); break;
      case '{esc}': wsSend('\x1b'); break;
      case '{tab}': wsSend('\t'); break;
      case '{space}': wsSend(' '); break;
      case '{up}': wsSend('\x1b[A'); break;
      case '{down}': wsSend('\x1b[B'); break;
      case '{left}': wsSend('\x1b[D'); break;
      case '{right}': wsSend('\x1b[C'); break;
      case '{ent}': wsSend('\r'); break;
      case '{bt}': wsSend(String.fromCharCode(96)); break;
      default:
        if (ctrlActive && key.length === 1) {
          const c = key.toUpperCase().charCodeAt(0);
          setCtrl(false);
          if (c >= 64 && c <= 95) { wsSend(String.fromCharCode(c & 31)); break; }
        }
        wsSend(key);
        if (kbShift && kbLayer !== 'sym') { kbShift = false; renderKbd(); } // one-shot shift
    }
    if (term) term.focus();
  }

  let kbMode = 'custom'; // 'custom' = on-screen keyboard (device kbd off); 'native' = device keyboard
  function applyKbMode() {
    const kbd = document.getElementById('kbd');
    const ta = term && term.textarea;
    if (kbMode === 'custom') {
      kbd.style.display = 'block';
      if (ta) { ta.setAttribute('inputmode', 'none'); ta.setAttribute('autocorrect', 'off'); ta.setAttribute('autocapitalize', 'off'); ta.setAttribute('spellcheck', 'false'); }
    } else {
      kbd.style.display = 'none';
      if (ta) { ta.setAttribute('inputmode', 'text'); ta.focus(); }
    }
    setTimeout(sendResize, 50); // terminal height changed; refit
  }
  document.getElementById('term-kbd').addEventListener('click', () => {
    kbMode = kbMode === 'custom' ? 'native' : 'custom';
    applyKbMode();
  });
  renderKbd();

  // ---- Switch to another window/session without leaving the terminal.
  function switchTerminal(name) {
    if (name === document.getElementById('term-name').textContent) return;
    if (ws) ws.close();
    if (term) term.dispose();
    openTerminal(name);
  }
  document.getElementById('term-switch').addEventListener('click', async () => {
    const el = document.getElementById('term-switch-list');
    if (el.style.display === 'block') { el.style.display = 'none'; return; }
    el.innerHTML = '<div class="muted" style="padding:8px">loading…</div>';
    el.style.display = 'block';
    try {
      const sessions = await (await fetch(api('/api/sessions'), aopts())).json();
      const cur = document.getElementById('term-name').textContent;
      el.innerHTML = '';
      (sessions || []).forEach((sn) => {
        const b = document.createElement('button');
        b.className = 'swrow';
        b.textContent = (sn.name === cur ? '● ' : '') + sn.name + '  —  ' + sn.path;
        b.addEventListener('click', () => { el.style.display = 'none'; switchTerminal(sn.name); });
        el.appendChild(b);
      });
      if (!sessions.length) el.innerHTML = '<div class="muted" style="padding:8px">no sessions</div>';
    } catch (e) { el.innerHTML = '<div class="muted" style="padding:8px">' + e + '</div>'; }
  });

  document.getElementById('sp-go').addEventListener('click', async () => {
    const repo = document.getElementById('sp-repo').value.trim();
    const branch = document.getElementById('sp-branch').value.trim();
    const msg = document.getElementById('sp-msg');
    if (!repo || !branch) { msg.textContent = 'repo and branch required'; return; }
    msg.textContent = 'spawning…';
    try {
      const res = await fetch(api('/api/spawn'), {
        method: 'POST',
        headers: jhdr(),
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
      const res = await fetch(api(path), {
        method: 'POST',
        headers: jhdr(),
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
      const repos = await (await fetch(api('/api/repos'), aopts())).json();
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

  // Filesystem browser — find a repo anywhere on disk, then open a terminal in it.
  let browsePath = '';
  function claudeCmdValue() {
    const cb = document.querySelector('.launch[data-label="claude"]');
    return cb ? (cb.getAttribute('data-cmd') || 'claude') : 'claude';
  }
  async function browseTo(path) {
    try {
      const res = await fetch(api('/api/ls?path=' + encodeURIComponent(path || '') + '&' + qtok()));
      const data = await res.json();
      if (!res.ok) { alert('error: ' + (data.error || res.status)); return; }
      browsePath = data.path;
      document.getElementById('browse-path').textContent = data.path;
      const up = document.getElementById('browse-up');
      up.style.visibility = data.parent ? 'visible' : 'hidden';
      up.setAttribute('data-parent', data.parent || '');
      renderGit(data.isRepo, data.path);
      const list = document.getElementById('browse-list');
      list.innerHTML = '';
      if (!data.entries.length) { list.innerHTML = '<p class="muted">No subfolders here. Open a terminal with the buttons above.</p>'; return; }
      data.entries.forEach((en) => {
        const row = document.createElement('div');
        row.className = 'card scard';
        const info = document.createElement('div');
        info.innerHTML = '<div class="name">' + en.name + (en.isRepo ? ' <span class="badge">git</span>' : '') + '</div>';
        row.appendChild(info);
        row.addEventListener('click', () => browseTo(en.path));
        list.appendChild(row);
      });
    } catch (e) { alert(String(e)); }
  }
  // Git panel: shown when the current folder is a repo. Lists worktrees (tap to
  // step into one, then open a terminal there) and creates new branches as
  // worktrees + sessions — the same workflow as the Blink TUI.
  async function renderGit(isRepo, repo) {
    const box = document.getElementById('browse-git');
    box.innerHTML = '';
    if (!isRepo) return;
    const head = document.createElement('div');
    head.className = 'git-head';
    head.innerHTML = '<span>worktrees</span>';
    const nb = document.createElement('button');
    nb.className = 'nb';
    nb.textContent = '＋ new branch';
    nb.addEventListener('click', () => newBranch(repo));
    head.appendChild(nb);
    box.appendChild(head);
    try {
      const wts = await (await fetch(api('/api/worktrees?repo=' + encodeURIComponent(repo) + '&' + qtok()))).json();
      (wts || []).forEach((wt) => {
        const row = document.createElement('div');
        row.className = 'wt';
        row.innerHTML = '<span class="b">' + (wt.branch || '(detached)') + '</span>' + (wt.isMain ? ' <span class="badge">main</span>' : '');
        const open = document.createElement('button');
        open.className = 'open';
        open.textContent = 'open ▸';
        open.addEventListener('click', () => browseTo(wt.path));
        row.appendChild(open);
        box.appendChild(row);
      });
    } catch (e) { /* worktree listing is best-effort */ }
  }

  async function newBranch(repo) {
    const branch = prompt('New branch name (creates a worktree + session):');
    if (!branch) return;
    try {
      const res = await fetch(api('/api/spawn'), {
        method: 'POST',
        headers: jhdr(),
        body: JSON.stringify({ repo, branch: branch.trim() }),
      });
      const data = await res.json();
      if (!res.ok) { alert('error: ' + (data.error || res.status)); return; }
      document.getElementById('browse-view').style.display = 'none';
      openTerminal(data.session);
    } catch (e) { alert(String(e)); }
  }

  function openBrowser() {
    document.getElementById('browse-view').style.display = 'flex';
    browseTo(document.getElementById('ql-dir').value.trim());
  }
  document.getElementById('ql-browse').addEventListener('click', openBrowser);
  document.getElementById('browse-close').addEventListener('click', () => {
    document.getElementById('browse-view').style.display = 'none';
  });
  document.getElementById('browse-up').addEventListener('click', (e) => {
    const p = e.currentTarget.getAttribute('data-parent');
    if (p) browseTo(p);
  });
  document.querySelectorAll('.bopen').forEach((b) => {
    b.addEventListener('click', () => {
      const act = b.getAttribute('data-act');
      document.getElementById('ql-dir').value = browsePath;
      document.getElementById('browse-view').style.display = 'none';
      if (act === 'tui') launch('/api/tui', { dir: browsePath });
      else if (act === 'claude') launch('/api/run', { dir: browsePath, cmd: claudeCmdValue(), label: 'claude' });
      else launch('/api/run', { dir: browsePath, cmd: '', label: 'sh' });
    });
  });

  async function load() {
    try {
      const info = await (await fetch(api('/api/info'), aopts())).json();
      document.getElementById('status').textContent = 'connected ✓';
      document.getElementById('machine').textContent =
        info.hostname + ' · ' + info.user + ' · ' + info.os + '/' + info.arch + ' · blink ' + info.version;
      if (info.hostname && M().name !== info.hostname) { M().name = info.hostname; saveMachines(); renderMachines(); }
      if (info.claudeCmd) {
        const cb = document.querySelector('.launch[data-label="claude"]');
        if (cb) cb.setAttribute('data-cmd', info.claudeCmd);
      }
      const sessions = await (await fetch(api('/api/sessions'), aopts())).json();
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
  renderMachines();
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

func (s *remoteServer) handleLs(w http.ResponseWriter, r *http.Request) {
	listing, err := listDir(r.URL.Query().Get("path"))
	if err != nil {
		writeJSON(w, http.StatusBadRequest, map[string]string{"error": err.Error()})
		return
	}
	writeJSON(w, http.StatusOK, listing)
}

func (s *remoteServer) handleWorktrees(w http.ResponseWriter, r *http.Request) {
	repo := r.URL.Query().Get("repo")
	if repo == "" {
		writeJSON(w, http.StatusBadRequest, map[string]string{"error": "repo is required"})
		return
	}
	wts, err := repoWorktrees(r.Context(), repo)
	if err != nil {
		writeJSON(w, http.StatusBadRequest, map[string]string{"error": err.Error()})
		return
	}
	writeJSON(w, http.StatusOK, wts)
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
