# Fleeting

One CLI pane of glass for many Oh My Pi (and later other) agent TUIs. Live PTY cells in a Brady Bunch grid. Default-deny hub-and-spoke. Hiro and Risa sit at 30,000 feet over every fleet you control.

The **daemon** owns every agent PTY and stays connected to model providers on a stable Linux box. Your SSH/PuTTY/WSL client is just an attachable TUI: close the laptop, lose the tether, or detach on purpose — agents keep running. Reattach later like GNU screen.

Each occupied cell runs the harness for that persona. Oh My Pi is `omp --profile <4-letter-name>` so state lives in `~/.omp/profiles/<name>/` (`--alias` is a shell-shortcut installer and must not be used here). Claude Code is `claude` with `CLAUDE_CONFIG_DIR=~/.fleeting/sessions/<name>/claude`. Codex is `codex` with `CODEX_HOME=~/.fleeting/sessions/<name>/codex`. Empty cells stay empty until you launch a CLI into them. Resize the outer window and each inner PTY is SIGWINCH’d to match.

## Run as a daemon (Linux)

On the machine that should keep agents alive (not your laptop session):

```bash
# once
go install github.com/richard-ginsberg/fleeting/cmd/fleeting@latest
# or from this repo:
go build -o fleeting ./cmd/fleeting

fleeting onboard          # writes ~/.fleeting/fleet.yaml
fleeting daemon           # foreground: systemd, tmux, or `nohup fleeting daemon &`
```

`fleeting daemon` binds `~/.fleeting/fleeting.sock`, writes `~/.fleeting/fleeting.pid`, logs to `~/.fleeting/daemon.log`, and does not need a TTY.

systemd unit sketch:

```ini
[Service]
ExecStart=/usr/local/bin/fleeting daemon
Restart=on-failure
```

From any SSH session on that host:

```bash
fleeting            # start daemon if needed, then attach the grid
fleeting up         # same
```

## Detach and reattach (screen-like)

| Command | What it does |
| --- | --- |
| `fleeting` / `fleeting up` | Start the daemon if the socket is dead, then attach |
| `fleeting -r` / `fleeting attach` | Reattach; error if another TUI is already attached |
| `fleeting -d -r` | Detach the other TUI, then attach (screen `-d -r`) |
| `fleeting -d` | Detach attached clients; daemon and PTYs stay up |
| `fleeting -ls` | List daemon pid, attached/detached, live agents |
| `fleeting down` | Stop the daemon and kill agent PTYs |

**Detach key:** `Ctrl-A` then `d` (GNU screen). That leaves the TUI and returns you to the shell. It does **not** steal `Ctrl-C` — `Ctrl-C` still goes to the focused PTY. `Ctrl-A` then `Ctrl-A` sends a literal `^A` into the agent.

If PuTTY/SSH dies, the daemon keeps the PTYs. Next login:

```bash
fleeting -r
# or just: fleeting
```

On reattach, missed output **replays at 4×** (change with `2` / `4` / `8` / `g`=16×). `L` (or End/Enter) jumps to live.

`Ctrl+Q` leaves the TUI the same way (save prompt if the workspace changed). Agents keep running until `fleeting down`.

## Launch on a blank cell

Focus an empty cell, then **Ctrl+O**. The status bar lists harnesses that are on PATH and executable:

| Menu | Binary (first match) |
| --- | --- |
| Claude Code | `claude` |
| Codex | `codex` |
| Cursor | `agent`, then `cursor-agent`, then `cursor` |
| Oh My Pi | `omp` (`--profile <persona>`, never `--alias`) |
| Pi | `pi` only — not Oh My Pi |

Type `1`–`9` (or Enter if only one is installed). Esc cancels. Occupied cells refuse the menu. Ad-hoc launches pick the next unused 4-letter name from hcom’s gold list (`luna`, `nova`, …). When the process exits, the cell goes blank.

**Ctrl+S** writes `~/.fleeting/workspace.yaml` (grid, zoom, focus, paging, and extra cells). **Ctrl+Q** detaches immediately if nothing changed; otherwise the status bar asks **s** save and quit, **n** quit without saving, **esc** cancel. `fleeting up` restores that workspace on top of `fleet.yaml`.

## Oh My Pi (required for cells)

Ubuntu/WSL:

```bash
curl -fsSL https://omp.sh/install | sh
# or: bun install -g @oh-my-pi/pi-coding-agent
command -v omp && omp --help | head
```

Then from this repo (rebuild if you already ran `up`):

```bash
go run ./cmd/fleeting up
```

Each agent’s OMP profile is `~/.omp/profiles/<name>/`. If `omp` is missing, the cell shows the install line instead of crashing the grid.

## Keys

| Key | Action |
| --- | --- |
| **? / F1** | Help overlay on the grid (same key or Esc closes; Ctrl+C and Ctrl-A are not stolen) |
| **Ctrl-A d** | Detach TUI; daemon keeps PTYs |
| Ctrl-A Ctrl-A | Send Ctrl-A to the focused PTY |
| Alt+← / Alt+→ | Column page |
| F7 / F8 | Full page (Ctrl+arrows stay with OMP) |
| Tab / Shift+Tab | Cycle cells (everything else goes into the focused OMP) |
| Alt+1–9 | Phone pad (top-left 3×3): A1 B1 C1 / A2 B2 C2 / A3 B3 C3 |
| Ctrl+G | Jump: type `D4` or `16`, Enter |
| Ctrl+O | Launch menu on a blank cell |
| F5 | Add/edit local Pi or OMP models (label follows focused cell; Tab probes /v1/models) |
| Ctrl+S | Save workspace (refreshes live cell snapshots: harness, cmd, slot) |
| Ctrl+Q | Detach (prompts to save if the workspace changed) |
| Alt+[ | Previous hub |
| Alt+] / F2 | Next hub |
| F3 | Shrink grid; at 3×3 zoom focus to 2×2 then 3×3 |
| F4 | Unzoom, then grow grid |
| Ctrl+M | Message along an allowed peer edge |
| Ctrl+C | Interrupt in the focused OMP (does not quit Fleeting) |
| 2 / 4 / 8 / g | Replay speed 2× / 4× / 8× / 16× (while catching up) |
| L | Jump replay to live |

## Config

`~/.fleeting/fleet.yaml` — omit `cmd:` to launch the `harness:` binary (`omp --profile <name>`, `claude`, `codex`, Cursor `agent`/`cursor-agent`/`cursor`, or `pi`). Override `cmd:` when you need extra flags. Listener: `~/.fleeting/fleeting.sock`. Saved layout: `~/.fleeting/workspace.yaml`. Pi models: `~/.pi/agent/models.json`. OMP models: `~/.omp/profiles/<persona>/agent/models.yml`. Claude/Codex persona dirs: `~/.fleeting/sessions/<name>/{claude,codex}`.

## Local Pi / OMP models (F5)

Focus a **Pi** or **Oh My Pi** cell — the status bar says `F5 pi-model` or `F5 omp-model`. Press **F5**. If that harness already has models, choose **1 add** or **2 edit**. Then llama.cpp / vLLM / SGLang, host, port. On **model id**, **Tab** GETs `{host}:{port}/v1/models`. Pi writes `~/.pi/agent/models.json`. OMP with `--profile kite` reads **`~/.omp/profiles/kite/agent/models.yml`**, not `~/.omp/agent/models.yml`. YAML is 2-space and omits keys OMP’s schema rejects (`samplingParams`, `thinkingTokenBudgetField`). Existing SaaS providers stay. The focused cell reloads. Use `/model` in the agent to select it.
