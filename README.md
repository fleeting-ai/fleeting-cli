# Fleeting

One CLI pane of glass for many Oh My Pi (and later other) agent TUIs. Live PTY cells in a Brady Bunch grid. Default-deny hub-and-spoke. Hiro and Risa sit at 30,000 feet over every fleet you control.

Each occupied cell runs `omp --profile <4-letter-name>` so that persona has its own OMP state (`~/.omp/profiles/<name>/`). `--alias` is a shell-shortcut installer and must not be used here. Empty cells stay empty. Resize the outer window and each inner PTY is SIGWINCH’d to match.

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
| Alt+← / Alt+→ | Column page |
| F7 / F8 | Full page (Ctrl+arrows stay with OMP) |
| Tab / Shift+Tab | Cycle cells (everything else goes into the focused OMP) |
| Alt+1–9 | Phone pad (top-left 3×3): A1 B1 C1 / A2 B2 C2 / A3 B3 C3 |
| Ctrl+G | Jump: type `D4` or `16`, Enter |
| F1 / F2 | Switch hub |
| F3 | Shrink grid; at 3×3 zoom focus to 2×2 then 3×3 |
| F4 | Unzoom, then grow grid |
| Ctrl+M | Message along an allowed peer edge |
| Ctrl+C | Interrupt in the focused OMP (does not quit Fleeting) |
| Ctrl+Q | Quit Fleeting |

## Config

`~/.fleeting/fleet.yaml` — omit `cmd:` to launch OMP. Override `cmd:` only for a different harness. Listener: `~/.fleeting/fleeting.sock`.

