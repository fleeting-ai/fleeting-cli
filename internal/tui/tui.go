package tui

import (
	"fmt"
	"sort"
	"strings"
	"time"

	tea "github.com/charmbracelet/bubbletea"
	"github.com/charmbracelet/lipgloss"
	"github.com/richard-ginsberg/fleeting/internal/config"
	"github.com/richard-ginsberg/fleeting/internal/listen"
	"github.com/richard-ginsberg/fleeting/internal/spool"
)

type Model struct {
	srv      *listen.Server
	cfg      *config.File
	home     string
	width    int
	height   int
	grid     int
	hubIdx   int
	focus    int
	snaps    []listen.Snapshot
	msgTo    string
	msgBody  string
	mode     string // "" | "msg"
	status   string
	lastErr  string
}

type tickMsg time.Time

func New(srv *listen.Server, cfg *config.File, home string) Model {
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
		sort.Slice(m.snaps, func(i, j int) bool { return m.snaps[i].Name < m.snaps[j].Name })
		return m, tea.Tick(time.Millisecond*120, func(t time.Time) tea.Msg { return tickMsg(t) })
	case tea.KeyMsg:
		return m.key(msg)
	}
	return m, nil
}

func (m Model) key(msg tea.KeyMsg) (tea.Model, tea.Cmd) {
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
	case "f4":
		if m.grid < 6 {
			m.grid++
		}
		c, r := m.paneSize()
		m.srv.Resize(c, r)
	case "ctrl+m":
		m.mode = "msg"
		m.msgTo = ""
		if se := m.focused(); se != nil && len(se.Peers) > 0 {
			m.msgTo = se.Peers[0]
		}
		m.msgBody = ""
	case "alt+1", "alt+2", "alt+3", "alt+4", "alt+5", "alt+6", "alt+7", "alt+8", "alt+9":
		m.focus = int(s[len(s)-1] - '1')
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
	barH, opH := 3, 6
	gridH := m.height - barH - opH
	if gridH < 8 {
		gridH = 8
	}
	cellW := m.width / n
	cellH := gridH / n
	cols = cellW - 2
	rows = cellH - 3
	if cols < 8 {
		cols = 8
	}
	if rows < 4 {
		rows = 4
	}
	return cols, rows
}

func (m Model) focused() *listen.Snapshot {
	hub := m.hubSnaps()
	if m.focus >= 0 && m.focus < len(hub) {
		return &hub[m.focus]
	}
	return nil
}

func (m Model) focusedName() string {
	if s := m.focused(); s != nil {
		return s.Name
	}
	return ""
}

func (m Model) hubSnaps() []listen.Snapshot {
	if len(m.cfg.Fleets) == 0 {
		return m.snaps
	}
	id := m.cfg.Fleets[m.hubIdx].ID
	var out []listen.Snapshot
	for _, s := range m.snaps {
		if s.Fleet == id {
			out = append(out, s)
		}
	}
	return out
}

func (m Model) View() string {
	if m.width == 0 {
		return "starting…"
	}
	barH := 3
	opH := 6
	gridH := m.height - barH - opH
	if gridH < 8 {
		gridH = 8
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
	cellW := w / n
	if cellW < 12 {
		cellW = 12
	}
	cellH := h / n
	if cellH < 4 {
		cellH = 4
	}
	hub := m.hubSnaps()
	rows := make([]string, 0, n)
	i := 0
	for r := 0; r < n; r++ {
		cols := make([]string, 0, n)
		for c := 0; c < n; c++ {
			focused := i == m.focus
			var snap *listen.Snapshot
			if i < len(hub) {
				snap = &hub[i]
			}
			cols = append(cols, cellView(cellW, cellH, i, snap, focused))
			i++
		}
		rows = append(rows, lipgloss.JoinHorizontal(lipgloss.Top, cols...))
	}
	return lipgloss.JoinVertical(lipgloss.Left, rows...)
}

func cellView(w, h, idx int, s *listen.Snapshot, focus bool) string {
	fg := lipgloss.Color("240")
	if focus {
		fg = lipgloss.Color("81")
	}
	st := lipgloss.NewStyle().Width(w - 2).Height(h - 2).Border(lipgloss.NormalBorder()).BorderForeground(fg)
	if s == nil {
		return st.Render(fmt.Sprintf("%d  —", idx+1))
	}
	dot := statusDot(s.Status)
	title := fmt.Sprintf("%s %s %s", s.Name, dot, s.Lane)
	if s.Hub {
		title += " HUB"
	}
	innerW, innerH := w-2, h-3
	if innerW < 1 {
		innerW = 1
	}
	if innerH < 1 {
		innerH = 1
	}
	body := clipScreen(s.Screen, innerW, innerH)
	return st.Render(title + "\n" + body)
}

func clipScreen(s string, cols, rows int) string {
	lines := strings.Split(strings.TrimRight(s, "\n"), "\n")
	if len(lines) > rows {
		lines = lines[:rows]
	}
	for len(lines) < rows {
		lines = append(lines, "")
	}
	for i, ln := range lines {
		runes := []rune(ln)
		if len(runes) > cols {
			lines[i] = string(runes[:cols])
		} else if len(runes) < cols {
			lines[i] = ln + strings.Repeat(" ", cols-len(runes))
		}
	}
	return strings.Join(lines, "\n")
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
	st := lipgloss.NewStyle().Width(w).Background(lipgloss.Color("236")).Foreground(lipgloss.Color("252"))
	focus := m.focusedName()
	line := fmt.Sprintf(" %dx%d  focus=%s  tab cells  alt+1-9  F1/F2 hub  F3/F4 grid  ctrl+m msg  ctrl+q quit", m.grid, m.grid, focus)
	if m.lastErr != "" {
		line += "  ERR " + m.lastErr
	} else if m.status != "" {
		line += "  " + m.status
	}
	if m.mode == "msg" {
		line = fmt.Sprintf(" msg %s → %s: %s", m.focusedName(), m.msgTo, m.msgBody)
	}
	return st.Render(line)
}

func trunc(s string, n int) string {
	if len(s) <= n {
		return s
	}
	return s[:n-1] + "…"
}
