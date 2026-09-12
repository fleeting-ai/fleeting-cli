package tui

import (
	"fmt"
	"strings"
)

func pageANSI(page int) int {
	palette := []int{36, 220, 135, 208, 45, 213}
	if page < 0 {
		page = 0
	}
	return palette[page%len(palette)]
}

func ansiBG(color int) string {
	return fmt.Sprintf("\x1b[48;5;%dm", color)
}

func ansiFG(color int) string {
	return fmt.Sprintf("\x1b[38;5;%dm", color)
}

const ansiReset = "\x1b[0m"

func fillANSI(color, w, h int) string {
	if w < 1 {
		w = 1
	}
	if h < 1 {
		h = 1
	}
	line := ansiBG(color) + strings.Repeat(" ", w) + ansiReset
	out := make([]string, h)
	for i := range out {
		out[i] = line
	}
	return strings.Join(out, "\n")
}

func frameANSI(inner string, w, h, loPage, hiPage int) string {
	if w < 4 {
		w = 4
	}
	if h < 3 {
		h = 3
	}
	innerW := w - 2
	innerH := h - 2
	lines := strings.Split(inner, "\n")
	for len(lines) < innerH {
		lines = append(lines, "")
	}
	if len(lines) > innerH {
		lines = lines[:innerH]
	}
	for i, ln := range lines {
		lines[i] = padVisible(ln, innerW)
	}
	lo, hi := pageANSI(loPage), pageANSI(hiPage)
	var top, bot string
	if loPage != hiPage {
		half := innerW / 2
		top = ansiFG(lo) + ansiBG(lo) + "┌" + strings.Repeat("─", half) +
			ansiFG(hi) + ansiBG(hi) + strings.Repeat("─", innerW-half) + "┐" + ansiReset
		bot = ansiFG(lo) + ansiBG(lo) + "└" + strings.Repeat("─", half) +
			ansiFG(hi) + ansiBG(hi) + strings.Repeat("─", innerW-half) + "┘" + ansiReset
	} else {
		bar := strings.Repeat("─", innerW)
		top = ansiFG(lo) + ansiBG(lo) + "┌" + bar + "┐" + ansiReset
		bot = ansiFG(lo) + ansiBG(lo) + "└" + bar + "┘" + ansiReset
	}
	var b strings.Builder
	b.WriteString(top)
	b.WriteByte('\n')
	for _, ln := range lines {
		b.WriteString(ansiFG(lo) + ansiBG(lo) + "│" + ansiReset)
		b.WriteString(ln)
		b.WriteString(ansiFG(hi) + ansiBG(hi) + "│" + ansiReset)
		b.WriteByte('\n')
	}
	b.WriteString(bot)
	return b.String()
}

func padVisible(s string, w int) string {
	n := visibleWidth(s)
	if n > w {
		return truncateVisible(s, w)
	}
	if n < w {
		return s + strings.Repeat(" ", w-n)
	}
	return s
}

func visibleWidth(s string) int {
	n := 0
	inEsc := false
	for _, r := range s {
		if r == 0x1b {
			inEsc = true
			continue
		}
		if inEsc {
			if (r >= 'A' && r <= 'Z') || (r >= 'a' && r <= 'z') {
				inEsc = false
			}
			continue
		}
		n++
	}
	return n
}

func truncateVisible(s string, w int) string {
	n := 0
	inEsc := false
	var b strings.Builder
	for _, r := range s {
		if r == 0x1b {
			inEsc = true
			b.WriteRune(r)
			continue
		}
		if inEsc {
			b.WriteRune(r)
			if (r >= 'A' && r <= 'Z') || (r >= 'a' && r <= 'z') {
				inEsc = false
			}
			continue
		}
		if n >= w {
			break
		}
		b.WriteRune(r)
		n++
	}
	return b.String() + ansiReset
}
