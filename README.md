# Fleeting

One CLI pane of glass for many agent TUIs (Claude CLI, Codex, Antigravity, Pi, …). Live PTY cells in a Brady Bunch grid. Default-deny hub-and-spoke. Hiro (judge) and Risa (foreman) sit at 30,000 feet over every fleet you control.

Local first. A Fleeting listener owns the sessions — not `tmux attach`. Remote TCP is the next slice.

## Run (WSL, Linux, macOS)

Needs Go 1.22+ and a terminal that can run TUIs.

```bash
cd fleeting
go run ./cmd/fleeting onboard   # writes ~/.fleeting/fleet.yaml
go run ./cmd/fleeting up        # listener + grid
```

Or:

```bash
go build -o fleeting ./cmd/fleeting
./fleeting onboard
./fleeting up
```

Resume a session id:

```bash
./fleeting r <guid> --go
```

Declare work for Hiro (files + md5 + intent):

```bash
./fleeting declare --agent nova --intent "fix grid focus" README.md
```

## Keys

| Key | Action |
| --- | --- |
| Tab / Shift+Tab | Cycle cells |
| 1–9 | Jump to cell |
| [ ] | Switch hub (fleet) |
| + / - | Grid 3×3 … 6×6 |
| m | Message along an allowed peer edge |
| q | Quit |

Bottom bar is the control strip. Top band is Hiro/Risa (counts + recent declarations).

## Config

Dockerfile-style YAML at `~/.fleeting/fleet.yaml` (or `$FLEETING_CONFIG`). Agents have 4-letter names, a lane, optional `hub: true`, and an explicit `peers` list (**default deny**). Set `cmd:` to the real harness when you have one; stubs loop a status line for the demo.

Listener socket: `~/.fleeting/fleeting.sock`. Same JSON protocol is the extension point for remote hosts.
