# Blink Remote Control

Control Blink from a phone or browser — spawn worktree sessions, list them, and
open a live terminal — from anywhere via [Tailscale](https://tailscale.com).

## How it fits together

```
phone browser / your web UI         <- you build the UI
        |  HTTP + WebSocket
   blink serve  (this server)        <- backend, below
        |  os/exec
   tmux sessions + git worktrees     <- existing Blink machinery
        |  network transport
   Tailscale (100.x.y.z)             <- reachable from anywhere, encrypted
```

Tailscale is pure transport — it is not a UI and your web app never references
it. It just makes the home machine reachable. See "Remote access" below.

## Quick start

```bash
blink serve --host 100.x.y.z          # your Tailscale IP (tailscale ip -4)
```

The server prints a URL, a token, and a **QR code**. Scan it with your phone to
open the web UI with the token already embedded (no typing it on a phone).

Flags:

| Flag | Default | Purpose |
|------|---------|---------|
| `--addr` | `:7890` | listen address |
| `--token` | generated | bearer token clients must present (or `BLINK_REMOTE_TOKEN`) |
| `--host` | system hostname | hostname/IP encoded in the URL + QR (use your Tailscale address) |

## HTTP API

All endpoints require the token, via `Authorization: Bearer <token>` **or**
`?token=<token>` (the query form lets the browser and WebSocket pass it easily).
Responses are JSON.

| Method | Path | Body | Returns |
|--------|------|------|---------|
| `GET` | `/api/info` | — | `{hostname,user,os,arch,version,tmux,sessions}` |
| `GET` | `/api/sessions` | — | `[{name,path,windows,attached}]` |
| `POST` | `/api/spawn` | `{repo,branch,base?}` | `{session,branch,path}` |
| `POST` | `/api/tui` | `{dir?}` | `{session,path}` — runs the Blink TUI in a tmux session (per-folder); attach to it via the WebSocket to use the full app on the phone |
| `POST` | `/api/kill` | `{session}` | `{killed}` |

The session list (`/api/sessions`) shows **all** tmux sessions on the machine,
not only Blink-created ones — so any session, including a Blink TUI, is listed
and attachable.

## WebSocket terminal

`GET /ws/session/{name}?token=<token>` — upgrades to a WebSocket bridged to the
named tmux session's PTY.

- **Binary** messages from the client are written to the PTY as keystrokes.
- **Text** messages are JSON control. Currently: `{"type":"resize","cols":N,"rows":N}`.
- The server sends PTY output back as **binary** messages.

Closing the socket detaches (the session keeps running; reconnect any time).

Wire it to [xterm.js](https://xtermjs.org): send `term.onData` as binary,
write incoming binary to `term.write`, and send a resize control message on fit.
The built-in placeholder page (`GET /`) is a minimal working example to copy.

## Remote access (from outside your network)

Your home machine is behind NAT with no public address. Tailscale solves this
without exposing anything publicly:

1. Install on the PC: `sudo pacman -S tailscale && sudo systemctl enable --now tailscaled && sudo tailscale up`
2. Install the Tailscale app on your phone, sign in with the same account.
3. `tailscale ip -4` on the PC gives your `100.x.y.z` — pass it as `--host`.

The connection is end-to-end encrypted and only your devices can reach it, so
plain HTTP + token is safe over the tailnet. Do **not** expose `blink serve` on
a public IP without Tailscale.

## Keep it running

`blink serve` should run as a background service so it survives logout/reboot
and is always reachable. A user systemd unit is provided:

```bash
cp contrib/systemd/blink-remote.service ~/.config/systemd/user/
# edit ExecStart (blink path, --host) and the token, then:
systemctl --user daemon-reload
systemctl --user enable --now blink-remote
loginctl enable-linger "$USER"     # keep it running while logged out
```

Check it: `systemctl --user status blink-remote`.
