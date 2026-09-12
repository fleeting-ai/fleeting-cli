package listen

import (
	"strings"

	"github.com/hinshun/vt10x"
)

const (
	attrReverse    = 1 << iota
	attrUnderline
	attrBold
	attrGfx
	attrItalic
	attrBlink
)

func dumpVT(t vt10x.Terminal) string {
	if t == nil {
		return ""
	}
	t.Lock()
	defer t.Unlock()
	cols, rows := t.Size()
	var b strings.Builder
	b.Grow(rows * (cols*8 + 8))
	var last style
	for y := 0; y < rows; y++ {
		for x := 0; x < cols; x++ {
			g := t.Cell(x, y)
			st := styleOf(g)
			if st != last {
				b.WriteString(st.sgr())
				last = st
			}
			ch := g.Char
			if ch == 0 {
				ch = ' '
			}
			b.WriteRune(ch)
		}
		if last != (style{}) {
			b.WriteString("\x1b[0m")
			last = style{}
		}
		if y < rows-1 {
			b.WriteByte('\n')
		}
	}
	return b.String()
}

type style struct {
	fg, bg vt10x.Color
	mode   int16
}

func styleOf(g vt10x.Glyph) style {
	return style{fg: g.FG, bg: g.BG, mode: g.Mode}
}

func (s style) sgr() string {
	var b strings.Builder
	b.WriteString("\x1b[0")
	if s.mode&attrBold != 0 {
		b.WriteString(";1")
	}
	if s.mode&attrUnderline != 0 {
		b.WriteString(";4")
	}
	if s.mode&attrItalic != 0 {
		b.WriteString(";3")
	}
	if s.mode&attrBlink != 0 {
		b.WriteString(";5")
	}
	if s.mode&attrReverse != 0 {
		b.WriteString(";7")
	}
	writeColor(&b, s.fg, false)
	writeColor(&b, s.bg, true)
	b.WriteByte('m')
	return b.String()
}

func writeColor(b *strings.Builder, c vt10x.Color, bg bool) {
	if c == vt10x.DefaultFG || c == vt10x.DefaultBG || c == vt10x.DefaultCursor {
		return
	}
	n := uint32(c)
	var fg0, fg8, fg256 string
	if bg {
		fg0, fg8, fg256 = "4", "10", "48"
	} else {
		fg0, fg8, fg256 = "3", "9", "38"
	}
	if n < 8 {
		b.WriteByte(';')
		b.WriteString(fg0)
		b.WriteByte('0' + byte(n))
		return
	}
	if n < 16 {
		b.WriteByte(';')
		b.WriteString(fg8)
		b.WriteByte('0' + byte(n-8))
		return
	}
	if n < 256 {
		b.WriteByte(';')
		b.WriteString(fg256)
		b.WriteString(";5;")
		b.WriteString(itoa(int(n)))
		return
	}
	// leftover: treat as 24-bit if packed
	r := byte(n >> 16)
	g := byte(n >> 8)
	bl := byte(n)
	b.WriteByte(';')
	b.WriteString(fg256)
	b.WriteString(";2;")
	b.WriteString(itoa(int(r)))
	b.WriteByte(';')
	b.WriteString(itoa(int(g)))
	b.WriteByte(';')
	b.WriteString(itoa(int(bl)))
}

func itoa(n int) string {
	if n == 0 {
		return "0"
	}
	var buf [4]byte
	i := len(buf)
	for n > 0 {
		i--
		buf[i] = byte('0' + n%10)
		n /= 10
	}
	return string(buf[i:])
}
