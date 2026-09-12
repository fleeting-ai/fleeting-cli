# Fleeting

One CLI pane of glass for many Oh My Pi (and later other) agent TUIs. Live PTY cells in a Brady Bunch grid. Default-deny hub-and-spoke. Hiro and Risa sit at 30,000 feet over every fleet you control.

Each occupied cell runs `omp --alias <4-letter-name>` so the familiar OMP layout (todos, bottom bar) is the cell. Empty cells stay empty. Resize the outer window and each inner PTY is SIGWINCH’d to match.

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

Each agent’s session dir is `~/.fleeting/sessions/<name>`. Persona is `--alias nova` (etc). If `omp` is missing, the cell shows the install line instead of crashing the grid.

## Keys

| Key | Action |
| --- | --- |
| Tab / Shift+Tab | Cycle cells (everything else goes into the focused OMP) |
| Alt+1–9 | Jump to cell |
| F1 / F2 | Switch hub |
| F3 / F4 | Grid 3×3 … 6×6 |
| Ctrl+M | Message along an allowed peer edge |
| Ctrl+Q | Quit Fleeting |

## Config

`~/.fleeting/fleet.yaml` — omit `cmd:` to launch OMP. Override `cmd:` only for a different harness. Listener: `~/.fleeting/fleeting.sock`.

