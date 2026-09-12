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

func hairlineANSI(color, w int) string {
	if w < 1 {
		w = 1
	}
	return ansiFG(color) + strings.Repeat("─", w) + ansiReset
}

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

func frameANSI(inner string, w, loPage, hiPage, splitAt int) string {
	if w < 4 {
		w = 4
	}
	innerW := w - 2
	lines := strings.Split(inner, "\n")
	if len(lines) < 1 {
		lines = []string{""}
	}
	for i, ln := range lines {
		lines[i] = padVisible(ln, innerW)
	}
	lo, hi := pageANSI(loPage), pageANSI(hiPage)
	if splitAt < 0 {
		splitAt = 0
	}
	if splitAt > innerW {
		splitAt = innerW
	}
	var top, bot string
	if loPage != hiPage && splitAt > 0 && splitAt < innerW {
		top = ansiFG(lo) + ansiBG(lo) + "┌" + strings.Repeat("─", splitAt) +
			ansiFG(hi) + ansiBG(hi) + strings.Repeat("─", innerW-splitAt) + "┐" + ansiReset
		bot = ansiFG(lo) + ansiBG(lo) + "└" + strings.Repeat("─", splitAt) +
			ansiFG(hi) + ansiBG(hi) + strings.Repeat("─", innerW-splitAt) + "┘" + ansiReset
	} else {
		c := lo
		if loPage == hiPage {
			c = lo
		}
		bar := strings.Repeat("─", innerW)
		top = ansiFG(c) + ansiBG(c) + "┌" + bar + "┐" + ansiReset
		bot = ansiFG(c) + ansiBG(c) + "└" + bar + "┘" + ansiReset
	}
	var b strings.Builder
	b.WriteString(top)
	b.WriteByte('\n')
	for _, ln := range lines {
		left, right := lo, hi
		if loPage == hiPage {
			right = lo
		}
		b.WriteString(ansiFG(left) + ansiBG(left) + "│" + ansiReset)
		b.WriteString(ln)
		b.WriteString(ansiFG(right) + ansiBG(right) + "│" + ansiReset)
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
