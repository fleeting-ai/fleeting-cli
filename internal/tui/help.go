package tui

import (
	"strings"

	"github.com/charmbracelet/lipgloss"
)

type helpLine struct {
	keys string
	desc string
}

type helpSection struct {
	title string
	lines []helpLine
}

// helpCatalog is the overlay source of truth: every binding wired in Model.key
// (and replay / prefix / quit), grouped for a typical SSH/PuTTY pane.
func helpCatalog() []helpSection {
	return []helpSection{
		{title: "Grid", lines: []helpLine{
			{"Alt+1–9", "Phone pad focus: A1 B1 C1 / A2 B2 C2 / A3 B3 C3"},
			{"Tab / Shift+Tab", "Cycle cells"},
			{"Ctrl+G", "Goto: type D4 or 16, Enter"},
			{"F3", "Shrink grid; at 3×3 zoom 2×2 then 3×3"},
			{"F4", "Unzoom, then grow grid"},
			{"Alt+← / Alt+→", "Column page"},
			{"F7 / Shift+Alt+←", "Page left by grid width"},
			{"F8 / Shift+Alt+→", "Page right by grid width"},
			{"Alt+[", "Previous hub"},
			{"Alt+] / F2", "Next hub"},
		}},
		{title: "Session", lines: []helpLine{
			{"Ctrl-A then d", "Detach TUI; daemon keeps PTYs"},
			{"Ctrl-A Ctrl-A", "Send ^A to the focused PTY"},
			{"Ctrl+C", "Interrupt in the focused PTY (does not quit Fleeting)"},
			{"Ctrl+O", "Launch menu on a blank cell"},
			{"Ctrl+M", "Bus message to a peer or team (Tab target, Alt+A action)"},
			{"2 / 4 / 8 / g", "Replay speed 2× / 4× / 8× / 16× (catch-up; 6 = 16×)"},
			{"L / End / Enter", "Jump replay to live"},
			{"? / F1", "Toggle this overlay"},
			{"Esc", "Close this overlay"},
		}},
		{title: "Models", lines: []helpLine{
			{"F5", "Local Pi / OMP model wizard (label follows focused cell)"},
		}},
		{title: "Quit", lines: []helpLine{
			{"Ctrl+S", "Save workspace"},
			{"Ctrl+Q", "Quit; if dirty: s save  n don’t  Esc cancel"},
		}},
	}
}

func helpTextLines(width int) []string {
	if width < 24 {
		width = 24
	}
	var out []string
	out = append(out, "Keys  (? / F1 / Esc close)")
	out = append(out, "Ctrl+C → PTY    Ctrl-A d still detaches")
	out = append(out, "")
	for _, sec := range helpCatalog() {
		out = append(out, sec.title)
		for _, ln := range sec.lines {
			out = append(out, wrapHelp("  "+ln.keys+"  "+ln.desc, width)...)
		}
		out = append(out, "")
	}
	for len(out) > 0 && out[len(out)-1] == "" {
		out = out[:len(out)-1]
	}
	return out
}

func wrapHelp(s string, width int) []string {
	if width < 8 || len(s) <= width {
		return []string{s}
	}
	var lines []string
	for len(s) > width {
		cut := strings.LastIndex(s[:width], " ")
		if cut < 8 {
			cut = width
		}
		lines = append(lines, s[:cut])
		s = strings.TrimLeft(s[cut:], " ")
		if s != "" {
			s = "    " + s
		}
	}
	if s != "" {
		lines = append(lines, s)
	}
	return lines
}

func (m Model) renderHelpOverlay(base string, w, h int) string {
	if w < 1 {
		w = 80
	}
	if h < 1 {
		h = 12
	}
	innerW := w - 4
	if innerW > 76 {
		innerW = 76
	}
	if innerW < 28 {
		innerW = w
		if innerW < 20 {
			innerW = 20
		}
	}
	boxInner := innerW - 2
	if boxInner < 16 {
		boxInner = 16
	}
	all := helpTextLines(boxInner)
	innerH := h - 2
	if innerH < 3 {
		innerH = 3
	}
	off := m.helpOff
	if off < 0 {
		off = 0
	}
	needMore := len(all)-off > innerH
	take := innerH
	if needMore {
		take = innerH - 1
	}
	if take < 1 {
		take = 1
	}
	maxOff := len(all) - take
	if maxOff < 0 {
		maxOff = 0
	}
	if off > maxOff {
		off = maxOff
		needMore = off+take < len(all)
	}
	end := off + take
	if end > len(all) {
		end = len(all)
	}
	window := append([]string{}, all[off:end]...)
	if needMore {
		window = append(window, "  … more  ↓  (↑/↓ or PgUp/PgDn)")
	}
	for len(window) < innerH {
		window = append(window, "")
	}
	if len(window) > innerH {
		window = window[:innerH]
		if needMore {
			window[innerH-1] = "  … more  ↓  (↑/↓ or PgUp/PgDn)"
		}
	}
	box := frameHelp(window, boxInner)
	return stampCenter(base, box, w, h)
}

func frameHelp(lines []string, inner int) string {
	if inner < 8 {
		inner = 8
	}
	bar := strings.Repeat("─", inner)
	var b strings.Builder
	b.WriteString("╭" + bar + "╮")
	for _, ln := range lines {
		vis := lipgloss.Width(ln)
		if vis > inner {
			ln = trunc(ln, inner)
			vis = lipgloss.Width(ln)
		}
		b.WriteByte('\n')
		b.WriteString("│")
		b.WriteString(ln)
		if pad := inner - vis; pad > 0 {
			b.WriteString(strings.Repeat(" ", pad))
		}
		b.WriteString("│")
	}
	b.WriteByte('\n')
	b.WriteString("╰" + bar + "╯")
	return b.String()
}

func stampCenter(base, overlay string, w, h int) string {
	bg := strings.Split(strings.TrimRight(base, "\n"), "\n")
	for len(bg) < h {
		bg = append(bg, "")
	}
	if len(bg) > h {
		bg = bg[:h]
	}
	fg := strings.Split(strings.TrimRight(overlay, "\n"), "\n")
	oh := len(fg)
	ow := 0
	for _, ln := range fg {
		if n := lipgloss.Width(ln); n > ow {
			ow = n
		}
	}
	r0 := (h - oh) / 2
	c0 := (w - ow) / 2
	if r0 < 0 {
		r0 = 0
	}
	if c0 < 0 {
		c0 = 0
	}
	for i, ln := range fg {
		r := r0 + i
		if r >= h {
			break
		}
		padR := w - c0 - lipgloss.Width(ln)
		if padR < 0 {
			padR = 0
		}
		bg[r] = strings.Repeat(" ", c0) + ln + strings.Repeat(" ", padR)
	}
	return strings.Join(bg, "\n")
}

func isHelpToggle(s string) bool {
	return s == "?" || s == "f1"
}
