package tui

import (
	"fmt"
	"strings"
	"time"

	tea "github.com/charmbracelet/bubbletea"
	"github.com/charmbracelet/lipgloss"
	"github.com/muesli/termenv"
	"github.com/richard-ginsberg/fleeting/internal/config"
	"github.com/richard-ginsberg/fleeting/internal/listen"
	"github.com/richard-ginsberg/fleeting/internal/spool"
)

type Model struct {
	srv     *listen.Server
	cfg     *config.File
	home    string
	width   int
	height  int
	grid    int
	hubIdx  int
	focus   int
	colOff  int
	snaps   []listen.Snapshot
	byName  map[string]listen.Snapshot
	msgTo   string
	msgBody string
	mode    string
	gotoBuf string
	status  string
	lastErr string
}

type tickMsg time.Time

func New(srv *listen.Server, cfg *config.File, home string) Model {
	lipgloss.SetColorProfile(termenv.ANSI256)
	return Model{
		srv:  srv,
		cfg:  cfg,
		home: home,
		grid: cfg.Grid,
		mode: "",
	}
}

func (m Model) Init() tea.Cmd {
	return tea.Tick(time.Millisecond*200, func(t time.Time) tea.Msg { return tickMsg(t) })
}

func (m Model) Update(msg tea.Msg) (tea.Model, tea.Cmd) {
	switch msg := msg.(type) {
	case tea.WindowSizeMsg:
		m.width, m.height = msg.Width, msg.Height
		c, r := m.paneSize()
		m.srv.Resize(c, r)
	case tickMsg:
		m.snaps = m.srv.Snapshots()
		m.byName = make(map[string]listen.Snapshot, len(m.snaps))
		for _, s := range m.snaps {
			m.byName[s.Name] = s
		}
		return m, tea.Tick(time.Millisecond*120, func(t time.Time) tea.Msg { return tickMsg(t) })
	case tea.KeyMsg:
		return m.key(msg)
	}
	return m, nil
}

func (m Model) key(msg tea.KeyMsg) (tea.Model, tea.Cmd) {
	if m.mode == "goto" {
		switch msg.Type {
		case tea.KeyEsc:
			m.mode = ""
			m.gotoBuf = ""
		case tea.KeyEnter:
			if vis, ok := parseGoto(m.gotoBuf, m.grid); ok {
				m.focus = vis
				m.lastErr = ""
				m.status = visAddr(vis, m.grid)
			} else {
				m.lastErr = "goto " + m.gotoBuf
			}
			m.mode = ""
			m.gotoBuf = ""
		case tea.KeyBackspace:
			if len(m.gotoBuf) > 0 {
				m.gotoBuf = m.gotoBuf[:len(m.gotoBuf)-1]
			}
		default:
			if msg.Type == tea.KeyRunes {
				m.gotoBuf += string(msg.Runes)
			}
		}
		return m, nil
	}
	if m.mode == "msg" {
		switch msg.Type {
		case tea.KeyEsc:
			m.mode = ""
			m.msgBody = ""
		case tea.KeyEnter:
			from := m.focusedName()
			err := m.srv.RoutePublic(from, m.msgTo, m.msgBody)
			if err != nil {
				m.lastErr = err.Error()
			} else {
				m.status = fmt.Sprintf("%s → %s", from, m.msgTo)
				m.lastErr = ""
			}
			m.mode = ""
			m.msgBody = ""
		case tea.KeyBackspace:
			if len(m.msgBody) > 0 {
				m.msgBody = m.msgBody[:len(m.msgBody)-1]
			}
		default:
			if msg.Type == tea.KeyRunes {
				m.msgBody += string(msg.Runes)
			}
		}
		return m, nil
	}

	s := msg.String()
	switch s {
	case "ctrl+q", "ctrl+c":
		return m, tea.Quit
	case "tab":
		m.focus++
	case "shift+tab":
		m.focus--
	case "alt+[", "f1":
		if m.hubIdx > 0 {
			m.hubIdx--
		}
		m.focus = 0
	case "alt+]", "f2":
		if m.hubIdx+1 < len(m.cfg.Fleets) {
			m.hubIdx++
		}
		m.focus = 0
	case "f3":
		if m.grid > 3 {
			m.grid--
		}
		c, r := m.paneSize()
		m.srv.Resize(c, r)
		m.colOff = clamp(m.colOff, 0, m.maxOff())
	case "f4":
		if m.grid < 6 {
			m.grid++
		}
		c, r := m.paneSize()
		m.srv.Resize(c, r)
		m.colOff = clamp(m.colOff, 0, m.maxOff())
	case "ctrl+m":
		m.mode = "msg"
		m.msgTo = ""
		if se := m.focused(); se != nil && len(se.Peers) > 0 {
			m.msgTo = se.Peers[0]
		}
		m.msgBody = ""
	case "alt+left":
		m.colOff = clamp(m.colOff-1, 0, m.maxOff())
	case "alt+right":
		m.colOff = clamp(m.colOff+1, 0, m.maxOff())
	case "f7", "shift+alt+left":
		m.colOff = clamp(m.colOff-m.grid, 0, m.maxOff())
	case "f8", "shift+alt+right":
		m.colOff = clamp(m.colOff+m.grid, 0, m.maxOff())
	case "ctrl+g":
		m.mode = "goto"
		m.gotoBuf = ""
	case "alt+1", "alt+2", "alt+3", "alt+4", "alt+5", "alt+6", "alt+7", "alt+8", "alt+9":
		m.focus = keypadFocus(int(s[len(s)-1]-'0'), m.grid)
	default:
		if name := m.focusedName(); name != "" {
			if b := encodeKey(msg); len(b) > 0 {
				_ = m.srv.Write(name, b)
			}
		}
		return m, nil
	}
	cells := m.grid * m.grid
	if cells > 0 {
		if m.focus < 0 {
			m.focus = cells - 1
		}
		m.focus = m.focus % cells
	}
	return m, nil
}

func (m Model) paneSize() (cols, rows int) {
	if m.width == 0 || m.height == 0 {
		return 80, 24
	}
	n := m.grid
	if n < 1 {
		n = 3
	}
	_, gridH := m.gridBox()
	_, _, cols, rows = cellGeom(m.width, gridH, n)
	return cols, rows
}

func (m Model) gridBox() (w, h int) {
	barH, opH := 3, 6
	h = m.height - barH - opH
	if h < 10 {
		h = 10
	}
	return m.width, h
}

const (
	outerBorder = 2
	gutterSize  = 1
	cellBorder  = 2
	titleRows   = 1
)

func cellGeom(termW, gridH, n int) (cellW, cellH, cols, rows int) {
	if n < 1 {
		n = 3
	}
	availW := termW - outerBorder - gutterSize*(n+1)
	availH := gridH - outerBorder - (n - 1)
	if availW < n {
		availW = n
	}
	if availH < n {
		availH = n
	}
	cellW = availW / n
	cellH = availH / n
	if cellW < 1 {
		cellW = 1
	}
	if cellH < 3 {
		cellH = 3
	}
	cols = cellW - cellBorder
	rows = cellH - 1 - titleRows
	if cols < 8 {
		cols = 8
	}
	if rows < 4 {
		rows = 4
	}
	return cellW, cellH, cols, rows
}

func (m Model) maxOff() int {
	return maxColOffset(len(m.orderedNames()), m.grid)
}

func (m Model) orderedNames() []string {
	if len(m.cfg.Fleets) == 0 {
		out := make([]string, 0, len(m.snaps))
		for _, s := range m.snaps {
			out = append(out, s.Name)
		}
		return out
	}
	fl := m.cfg.Fleets[m.hubIdx]
	out := make([]string, 0, len(fl.Agents))
	for _, a := range fl.Agents {
		out = append(out, a.Name)
	}
	return out
}

func (m Model) snapAt(global int) *listen.Snapshot {
	names := m.orderedNames()
	if global < 0 || global >= len(names) {
		return nil
	}
	s, ok := m.byName[names[global]]
	if !ok {
		return nil
	}
	return &s
}

func (m Model) visualToGlobal(vis int) int {
	n := m.grid
	if n <= 0 {
		return vis
	}
	r := vis / n
	c := vis % n
	return globalIndex(m.colOff, n, c, r)
}

func (m Model) focused() *listen.Snapshot {
	return m.snapAt(m.visualToGlobal(m.focus))
}

func (m Model) focusedName() string {
	if s := m.focused(); s != nil {
		return s.Name
	}
	return ""
}

func (m Model) hubPages() (lo, hi int) {
	n := m.grid
	pageSize := n * n
	lo, hi = -1, -1
	for c := 0; c < n; c++ {
		for r := 0; r < n; r++ {
			p := pageOf(globalIndex(m.colOff, n, c, r), pageSize)
			if lo < 0 || p < lo {
				lo = p
			}
			if p > hi {
				hi = p
			}
		}
	}
	return lo, hi
}

func pageColor(page int) lipgloss.Color {
	// 256-color indexes so PuTTY shows them (truecolor hex often stays gray).
	palette := []string{"36", "220", "135", "208", "45", "213"}
	if page < 0 {
		page = 0
	}
	return lipgloss.Color(palette[page%len(palette)])
}

func (m Model) View() string {
	if m.width == 0 {
		return "starting…"
	}
	barH := 3
	opH := 6
	gridH := m.height - barH - opH
	if gridH < 10 {
		gridH = 10
	}
	grid := m.renderGrid(m.width, gridH)
	op := m.renderOperator(m.width, opH)
	bar := m.renderBar(m.width)
	return lipgloss.JoinVertical(lipgloss.Left, op, grid, bar)
}

func (m Model) renderOperator(w, h int) string {
	style := lipgloss.NewStyle().Width(w).Height(h).Border(lipgloss.NormalBorder()).BorderForeground(lipgloss.Color("63"))
	ev := spool.Tail(m.home, 4)
	var b strings.Builder
	b.WriteString("HIRO · RISA  —  30,000ft  operator=" + m.cfg.Operator)
	if len(m.cfg.Fleets) > 0 {
		b.WriteString("  fleet=" + m.cfg.Fleets[m.hubIdx].ID)
		if g := m.cfg.Fleets[m.hubIdx].Goal; g != "" {
			b.WriteString("  goal=" + g)
		}
	}
	b.WriteString("\n")
	counts := map[string]int{}
	for _, s := range m.snaps {
		counts[s.Status]++
	}
	b.WriteString(fmt.Sprintf("working %d  stopped %d  attention %d  tokens %d  agents %d\n",
		counts["working"], counts["stopped"], counts["attention"], counts["tokens"], len(m.snaps)))
	if len(ev) == 0 {
		b.WriteString("no declarations or messages yet — hiro waits on file+md5+intent; risa lands git")
	} else {
		for _, e := range ev {
			b.WriteString(fmt.Sprintf("%s %s→%s %s\n", e.Kind, e.From, e.To, trunc(e.Body, 60)))
		}
	}
	return style.Render(strings.TrimRight(b.String(), "\n"))
}

func (m Model) renderGrid(w, h int) string {
	n := m.grid
	cellW, cellH, _, _ := cellGeom(w, h, n)
	pageSize := n * n
	lo, hi := m.hubPages()
	colViews := make([]string, 0, n)
	var colH int
	for c := 0; c < n; c++ {
		rows := make([]string, 0, n)
		for r := 0; r < n; r++ {
			vis := r*n + c
			gidx := globalIndex(m.colOff, n, c, r)
			pg := pageOf(gidx, pageSize)
			rows = append(rows, cellView(cellW, cellH, vis, m.snapAt(gidx), vis == m.focus, pageColor(pg), r, n))
			if r+1 < n {
				rows = append(rows, hairlineANSI(pageANSI(pg), cellW))
			}
		}
		stack := lipgloss.JoinVertical(lipgloss.Left, rows...)
		colH = lipgloss.Height(stack)
		colViews = append(colViews, stack)
	}
	var parts []string
	parts = append(parts, fillANSI(pageANSI(lo), gutterSize, colH))
	splitAt := gutterSize
	for c, col := range colViews {
		colPage := pageOf(globalIndex(m.colOff, n, c, 0), pageSize)
		parts = append(parts, col)
		if colPage == lo {
			splitAt += cellW
		}
		next := colPage
		if c+1 < n {
			next = pageOf(globalIndex(m.colOff, n, c+1, 0), pageSize)
		}
		parts = append(parts, fillANSI(pageANSI(next), gutterSize, colH))
		if colPage == lo && next == lo {
			splitAt += gutterSize
		}
	}
	inner := lipgloss.JoinHorizontal(lipgloss.Top, parts...)
	return frameANSI(inner, w, lo, hi, splitAt)
}

func cellView(w, h, vis int, s *listen.Snapshot, focus bool, page lipgloss.Color, row, n int) string {
	fg := page
	if focus {
		fg = lipgloss.Color("15")
	}
	innerW := w - cellBorder
	innerH := h - 1
	if row == 0 || row == n-1 {
		innerH = h - cellBorder
	}
	if innerW < 4 {
		innerW = 4
	}
	if innerH < 3 {
		innerH = 3
	}
	addr := visAddr(vis, n)
	box := lipgloss.NewStyle().
		Width(innerW).
		MaxWidth(innerW).
		Height(innerH).
		MaxHeight(innerH).
		Border(lipgloss.NormalBorder()).
		BorderTop(row == 0).
		BorderBottom(row == n-1).
		BorderForeground(fg)
	if s == nil {
		return box.Render(addr + "  —")
	}
	title := fmt.Sprintf("%s %s %s %s", addr, s.Name, statusDot(s.Status), s.Lane)
	if s.Hub {
		title += " HUB"
	}
	return box.Render(title + "\n" + s.Screen)
}

func statusDot(st string) string {
	switch st {
	case "working":
		return "●"
	case "stopped":
		return "○"
	case "attention":
		return "!"
	case "tokens":
		return "$"
	default:
		return "·"
	}
}

func (m Model) renderBar(w int) string {
	lo, _ := m.hubPages()
	label := pageLabel(m.colOff, m.grid)
	pageBit := ansiBG(pageANSI(lo)) + ansiFG(16) + " " + label + " " + ansiReset
	rest := lipgloss.NewStyle().Background(lipgloss.Color("236")).Foreground(lipgloss.Color("252"))
	focus := m.focusedName()
	line := fmt.Sprintf(" %dx%d  focus=%s  alt←/→ col  F7/F8 page  tab  ctrl+g goto  ctrl+q", m.grid, m.grid, focus)
	if m.lastErr != "" {
		line += "  ERR " + m.lastErr
	} else if m.status != "" {
		line += "  " + m.status
	}
	if m.mode == "goto" {
		line = fmt.Sprintf(" goto %s_  (D4 or 16, Enter)", m.gotoBuf)
	}
	if m.mode == "msg" {
		line = fmt.Sprintf(" msg %s → %s: %s", m.focusedName(), m.msgTo, m.msgBody)
	}
	pad := rest.Width(w - lipgloss.Width(pageBit)).Render(line)
	return lipgloss.JoinHorizontal(lipgloss.Top, pageBit, pad)
}

func trunc(s string, n int) string {
	if len(s) <= n {
		return s
	}
	return s[:n-1] + "…"
}
