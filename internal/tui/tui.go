package tui

import (
	"fmt"
	"strings"
	"time"

	tea "github.com/charmbracelet/bubbletea"
	"github.com/charmbracelet/lipgloss"
	"github.com/muesli/termenv"
	"github.com/richard-ginsberg/fleeting/internal/config"
	"github.com/richard-ginsberg/fleeting/internal/harness"
	"github.com/richard-ginsberg/fleeting/internal/listen"
	"github.com/richard-ginsberg/fleeting/internal/pi"
	"github.com/richard-ginsberg/fleeting/internal/spool"
	"github.com/richard-ginsberg/fleeting/internal/workspace"
)

type Model struct {
	srv       listen.Host
	cfg       *config.File
	player    *listen.Player
	prefix    bool
	kick      <-chan struct{}
	home      string
	width     int
	height    int
	grid      int
	hubIdx    int
	focus     int
	zoomSpan  int
	colOff    int
	snaps     []listen.Snapshot
	byName    map[string]listen.Snapshot
	msgTo     string
	msgBody   string
	mode      string
	gotoBuf   string
	status    string
	lastErr   string
	extras    map[int]workspace.Cell // global index → ad-hoc session
	launch    []harness.Installed
	savedFP   string
	piStep    int
	piBuf     string
	piDraft   pi.Draft
	piIDs     []string
	piPage    int
	piKind    string
	piEntries []pi.Entry
	help      bool
	helpOff   int
}

type tickMsg time.Time

func New(srv listen.Host, cfg *config.File, home string) Model {
	lipgloss.SetColorProfile(termenv.ANSI256)
	m := Model{
		srv:    srv,
		cfg:    cfg,
		home:   home,
		grid:   cfg.Grid,
		mode:   "",
		extras: map[int]workspace.Cell{},
	}
	m.restoreWorkspace()
	m.upgradePlaceholders()
	return m
}

func (m *Model) restoreWorkspace() {
	ws, err := workspace.Load()
	if err != nil || ws == nil {
		m.savedFP = workspace.Fingerprint(m.workspaceFile())
		return
	}
	m.grid = ws.Grid
	m.colOff = ws.ColOff
	m.zoomSpan = ws.ZoomSpan
	m.focus = ws.Focus
	m.hubIdx = ws.HubIdx
	if m.grid != 3 {
		m.zoomSpan = 0
	}
	if len(m.cfg.Fleets) > 0 && m.hubIdx >= len(m.cfg.Fleets) {
		m.hubIdx = 0
	}
	live := map[string]bool{}
	if m.srv != nil {
		for _, s := range m.srv.Snapshots() {
			if s.Name != "" && s.Alive {
				live[s.Name] = true
			}
		}
	}
	for _, c := range ws.Cells {
		if c.Name == "" {
			continue
		}
		// Map the slot first. If Spawn fails because the daemon still owns
		// this PTY, the cell must still bind or C1 looks empty on -r.
		m.extras[c.Global] = c
		if m.fleetHas(c.Name) || live[c.Name] {
			continue
		}
		if len(c.Cmd) == 0 {
			continue
		}
		fleet := c.Fleet
		if fleet == "" && len(m.cfg.Fleets) > 0 {
			fleet = m.cfg.Fleets[m.hubIdx].ID
		}
		a := config.Agent{
			Name:       c.Name,
			Role:       c.Role,
			Lane:       c.Lane,
			Harness:    c.Harness,
			Cmd:        c.Cmd,
			ResumeGUID: c.GUID,
		}
		if a.Role == "" {
			a.Role = "worker"
		}
		if a.Lane == "" {
			a.Lane = c.Harness
		}
		_ = m.srv.Spawn(fleet, a)
	}
	m.savedFP = workspace.Fingerprint(m.workspaceFile())
}

func (m *Model) upgradePlaceholders() {
	if m.srv == nil {
		return
	}
	for _, s := range m.srv.Snapshots() {
		if !s.Alive || !harness.Placeholder(s.Cmd, s.Screen) {
			continue
		}
		a := config.Agent{
			Name:    s.Name,
			Role:    s.Role,
			Lane:    s.Lane,
			Harness: s.Harness,
			Cmd:     s.Cmd,
			Hub:     s.Hub,
			Peers:   s.Peers,
		}
		if fa := m.agentByName(s.Name); fa != nil {
			a.Role = fa.Role
			a.Lane = fa.Lane
			a.Harness = fa.Harness
			a.Hub = fa.Hub
			a.Peers = fa.Peers
		}
		if a.Harness == "" {
			a.Harness = "omp"
		}
		in, err := harness.Resolve(a.Harness)
		if err != nil {
			continue
		}
		a.Cmd = in.CmdForAgent(a.Name)
		fleet := s.Fleet
		if fleet == "" && len(m.cfg.Fleets) > 0 {
			fleet = m.cfg.Fleets[m.hubIdx].ID
		}
		_ = m.srv.Replace(fleet, a)
	}
}

type kickMsg struct{}

func (m *Model) WithReplay(frames []listen.ReplayFrame) {
	m.player = listen.NewPlayer(frames, 4)
	if m.player != nil {
		m.mode = "replay"
		m.status = "replay 4x  (2/4/8/g=16  L live)"
	}
}

func (m *Model) WithKick(ch <-chan struct{}) {
	m.kick = ch
}

func (m Model) Init() tea.Cmd {
	tick := tea.Tick(time.Millisecond*200, func(t time.Time) tea.Msg { return tickMsg(t) })
	if m.kick == nil {
		return tick
	}
	return tea.Batch(tick, func() tea.Msg {
		<-m.kick
		return kickMsg{}
	})
}

func (m Model) Update(msg tea.Msg) (tea.Model, tea.Cmd) {
	switch msg := msg.(type) {
	case kickMsg:
		return m, tea.Quit
	case tea.WindowSizeMsg:
		m.width, m.height = msg.Width, msg.Height
		m.applyPTYs(m.paneSize())
	case tickMsg:
		m.snaps = m.srv.Snapshots()
		if m.player != nil && !m.player.Live() {
			m.player.Advance(time.Time(msg))
			m.snaps = m.player.Overlay(m.snaps)
			if m.player.Live() {
				m.mode = ""
				m.status = "live"
				m.snaps = m.srv.Snapshots()
			}
		}
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
	if m.mode == "pimodel" {
		return m.piModelKey(msg)
	}
	if m.mode == "quit" {
		return m.quitKey(msg)
	}
	if m.mode == "launch" {
		return m.launchKey(msg)
	}
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
	if m.mode == "replay" && m.player != nil && !m.player.Live() {
		switch s {
		case "2":
			m.player.SetSpeed(2)
			m.status = "replay 2x  (L live)"
			return m, nil
		case "4":
			m.player.SetSpeed(4)
			m.status = "replay 4x  (L live)"
			return m, nil
		case "8":
			m.player.SetSpeed(8)
			m.status = "replay 8x  (L live)"
			return m, nil
		case "g", "6":
			m.player.SetSpeed(16)
			m.status = "replay 16x  (L live)"
			return m, nil
		case "l", "L", "end", "enter":
			m.player.Jump()
			m.mode = ""
			m.status = "live"
			return m, nil
		}
	}
	if m.prefix {
		m.prefix = false
		if s == "d" || s == "D" {
			m.status = "detached"
			return m, tea.Quit
		}
		if s == "ctrl+a" {
			if name := m.focusedName(); name != "" {
				_ = m.srv.Write(name, []byte{0x01})
			}
			return m, nil
		}
	}
	if m.help {
		if isHelpToggle(s) || s == "esc" || msg.Type == tea.KeyEsc {
			m.help = false
			m.helpOff = 0
			return m, nil
		}
		switch s {
		case "up", "pgup":
			if m.helpOff > 0 {
				m.helpOff--
			}
			return m, nil
		case "down", "pgdown":
			m.helpOff++
			return m, nil
		case "ctrl+c":
			if name := m.focusedName(); name != "" && m.srv != nil {
				if b := encodeKey(msg); len(b) > 0 {
					_ = m.srv.Write(name, b)
				}
			}
			return m, nil
		case "ctrl+a":
			m.prefix = true
			m.status = "C-a  d=detach"
			return m, nil
		default:
			return m, nil
		}
	}
	switch s {
	case "?", "f1":
		m.help = true
		m.helpOff = 0
		m.status = "help  ?/F1/Esc close"
		return m, nil
	case "ctrl+a":
		m.prefix = true
		m.status = "C-a  d=detach"
		return m, nil
	case "ctrl+q":
		return m.requestQuit()
	case "ctrl+s":
		return m.saveWorkspace()
	case "tab":
		m.focus++
	case "shift+tab":
		m.focus--
	case "alt+[":
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
			m.zoomSpan = 0
		} else if m.zoomSpan == 0 {
			m.zoomSpan = 2
		} else if m.zoomSpan == 2 {
			m.zoomSpan = 3
		}
		m.applyPTYs(m.paneSize())
		m.colOff = clamp(m.colOff, 0, m.maxOff())
	case "f4":
		if m.zoomSpan == 3 {
			m.zoomSpan = 2
		} else if m.zoomSpan == 2 {
			m.zoomSpan = 0
		} else if m.grid < 6 {
			m.grid++
		}
		m.applyPTYs(m.paneSize())
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
	case "ctrl+o":
		return m.openLaunch()
	case "f5":
		return m.openPiModel()
	case "alt+1", "alt+2", "alt+3", "alt+4", "alt+5", "alt+6", "alt+7", "alt+8", "alt+9":
		m.focus = keypadFocus(int(s[len(s)-1]-'0'), m.grid)
	default:
		if name := m.focusedName(); name != "" && m.srv != nil {
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

func (m Model) applyPTYs(cols, rows int) {
	if m.srv == nil {
		return
	}
	m.srv.Resize(cols, rows)
	if m.zoomSpan < 2 {
		return
	}
	if s := m.focused(); s != nil {
		span := m.zoomSpan
		m.srv.ResizeSession(s.Name, cols*span+(span-1)*gutterSize, rows*span+(span-1))
	}
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
	var base []string
	if m.cfg == nil || len(m.cfg.Fleets) == 0 {
		base = make([]string, 0, len(m.snaps))
		for _, s := range m.snaps {
			base = append(base, s.Name)
		}
	} else {
		fl := m.cfg.Fleets[m.hubIdx]
		base = make([]string, 0, len(fl.Agents))
		for _, a := range fl.Agents {
			base = append(base, a.Name)
		}
	}
	max := len(base)
	for i := range m.extras {
		if i+1 > max {
			max = i + 1
		}
	}
	out := make([]string, max)
	copy(out, base)
	for i, c := range m.extras {
		if i < 0 || i >= len(out) {
			continue
		}
		if i < len(base) && m.liveName(base[i]) {
			continue
		}
		out[i] = c.Name
	}
	return out
}

func (m Model) liveName(name string) bool {
	if name == "" {
		return false
	}
	_, ok := m.byName[name]
	return ok
}

func (m Model) snapAt(global int) *listen.Snapshot {
	names := m.orderedNames()
	if global < 0 || global >= len(names) {
		return nil
	}
	name := names[global]
	if name == "" {
		return nil
	}
	s, ok := m.byName[name]
	if !ok || !s.Alive {
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
	if m.help {
		grid = m.renderHelpOverlay(grid, m.width, gridH)
	}
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
	if m.zoomSpan >= 2 && m.grid == 3 {
		return m.renderZoomed(w, h)
	}
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
			rows = append(rows, cellView(cellW, cellH, vis, n, m.snapAt(gidx), vis == m.focus, pageColor(pg), r, n))
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

func (m Model) renderZoomed(w, h int) string {
	n := m.grid
	span := m.zoomSpan
	cellW, cellH, _, _ := cellGeom(w, h, n)
	pageSize := n * n
	lo, hi := m.hubPages()
	fc := m.focus % n
	fr := m.focus / n
	c0, r0 := zoomOrigin(fc, fr, n, span)

	small := func(c, r int) string {
		vis := r*n + c
		gidx := globalIndex(m.colOff, n, c, r)
		pg := pageOf(gidx, pageSize)
		return cellView(cellW, cellH, vis, n, m.snapAt(gidx), vis == m.focus, pageColor(pg), r, n)
	}
	stackSmall := func(c, r0, r1 int) string {
		var rows []string
		for r := r0; r < r1; r++ {
			if r > r0 {
				gidx := globalIndex(m.colOff, n, c, r)
				rows = append(rows, hairlineANSI(pageANSI(pageOf(gidx, pageSize)), cellW))
			}
			rows = append(rows, small(c, r))
		}
		if len(rows) == 0 {
			return ""
		}
		return lipgloss.JoinVertical(lipgloss.Left, rows...)
	}

	var bands []string
	for r := 0; r < r0; r++ {
		var rowParts []string
		rowParts = append(rowParts, fillANSI(pageANSI(lo), gutterSize, cellH))
		for c := 0; c < n; c++ {
			rowParts = append(rowParts, small(c, r))
			gidx := globalIndex(m.colOff, n, c, r)
			rowParts = append(rowParts, fillANSI(pageANSI(pageOf(gidx, pageSize)), gutterSize, cellH))
		}
		bands = append(bands, lipgloss.JoinHorizontal(lipgloss.Top, rowParts...))
		if r+1 < n {
			bands = append(bands, hairlineANSI(pageANSI(lo), w-2))
		}
	}

	bw := span*cellW + (span-1)*gutterSize
	bh := span*cellH + (span - 1)
	fvis := fr*n + fc
	fgidx := globalIndex(m.colOff, n, fc, fr)
	fpg := pageOf(fgidx, pageSize)
	big := cellView(bw, bh, fvis, n, m.snapAt(fgidx), true, pageColor(fpg), 0, 1)

	var mid []string
	mid = append(mid, fillANSI(pageANSI(lo), gutterSize, bh))
	for c := 0; c < c0; c++ {
		mid = append(mid, stackSmall(c, r0, r0+span))
		mid = append(mid, fillANSI(pageANSI(pageOf(globalIndex(m.colOff, n, c, r0), pageSize)), gutterSize, bh))
	}
	mid = append(mid, big)
	mid = append(mid, fillANSI(pageANSI(fpg), gutterSize, bh))
	for c := c0 + span; c < n; c++ {
		mid = append(mid, stackSmall(c, r0, r0+span))
		mid = append(mid, fillANSI(pageANSI(pageOf(globalIndex(m.colOff, n, c, r0), pageSize)), gutterSize, bh))
	}
	bands = append(bands, lipgloss.JoinHorizontal(lipgloss.Top, mid...))

	for r := r0 + span; r < n; r++ {
		bands = append(bands, hairlineANSI(pageANSI(lo), w-2))
		var rowParts []string
		rowParts = append(rowParts, fillANSI(pageANSI(lo), gutterSize, cellH))
		for c := 0; c < n; c++ {
			rowParts = append(rowParts, small(c, r))
			gidx := globalIndex(m.colOff, n, c, r)
			rowParts = append(rowParts, fillANSI(pageANSI(pageOf(gidx, pageSize)), gutterSize, cellH))
		}
		bands = append(bands, lipgloss.JoinHorizontal(lipgloss.Top, rowParts...))
	}
	splitAt := w
	if span < n {
		splitAt = gutterSize + c0*(cellW+gutterSize) + bw
	}
	inner := lipgloss.JoinVertical(lipgloss.Left, bands...)
	return frameANSI(inner, w, lo, hi, splitAt)
}

func cellView(w, h, vis, gridN int, s *listen.Snapshot, focus bool, page lipgloss.Color, row, borderN int) string {
	fg := page
	if focus {
		fg = lipgloss.Color("15")
	}
	innerW := w - cellBorder
	innerH := h - 1
	if row == 0 || row == borderN-1 {
		innerH = h - cellBorder
	}
	if innerW < 4 {
		innerW = 4
	}
	if innerH < 3 {
		innerH = 3
	}
	addr := visAddr(vis, gridN)
	box := lipgloss.NewStyle().
		Width(innerW).
		MaxWidth(innerW).
		Height(innerH).
		MaxHeight(innerH).
		Border(lipgloss.NormalBorder()).
		BorderTop(row == 0).
		BorderBottom(row == borderN-1).
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
	line := fmt.Sprintf(" %dx%d  focus=%s  zoom=%d  ?/F1 help  C-a d detach  F3/F4  F5 %s-model  alt←/→  ctrl+g  ctrl+o  ctrl+s  ctrl+q", m.grid, m.grid, focus, m.zoomSpan, m.modelKind())
	if m.lastErr != "" {
		line += "  ERR " + m.lastErr
	} else if m.status != "" {
		line += "  " + m.status
	}
	if m.mode == "goto" {
		line = fmt.Sprintf(" goto %s_  (D4 or 16, Enter)", m.gotoBuf)
	}
	if m.mode == "launch" {
		if len(m.launch) == 0 {
			line = " launch: none installed (claude, codex, agent/cursor, omp/pi)  Esc"
		} else {
			var b strings.Builder
			b.WriteString(" launch ")
			for i, in := range m.launch {
				fmt.Fprintf(&b, " %d %s", i+1, in.Title)
			}
			b.WriteString("   Enter/1-9  Esc")
			line = b.String()
		}
	}
	if m.mode == "pimodel" {
		line = m.piModelPrompt()
	}
	if m.mode == "quit" {
		line = " workspace changed — s save and quit  n quit without saving  esc cancel"
	}
	if m.mode == "msg" {
		line = fmt.Sprintf(" msg %s → %s: %s", m.focusedName(), m.msgTo, m.msgBody)
	}
	if m.prefix {
		line = " C-a prefix  d detach  C-a C-a sends ^A to the PTY  (Ctrl+C still goes to the agent)"
	}
	if m.mode == "replay" && m.player != nil && !m.player.Live() {
		line = fmt.Sprintf(" replay %dx  2/4/8/g=16x  L jump live  frame %d/%d", m.player.Speed, m.player.FrameIndex(), m.player.Len())
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
