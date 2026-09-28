package tui

import (
	"bytes"
	"strings"
	"testing"

	tea "github.com/charmbracelet/bubbletea"
	"github.com/richard-ginsberg/fleeting/internal/config"
	"github.com/richard-ginsberg/fleeting/internal/listen"
)

type helpHost struct {
	wrote []byte
}

func (h *helpHost) Snapshots() []listen.Snapshot { return nil }
func (h *helpHost) Write(_ string, data []byte) error {
	h.wrote = append(h.wrote, data...)
	return nil
}
func (h *helpHost) Spawn(string, config.Agent) error { return nil }
func (h *helpHost) Replace(string, config.Agent) error {
	return nil
}
func (h *helpHost) Restart(string) error             { return nil }
func (h *helpHost) Resize(int, int)                  {}
func (h *helpHost) ResizeSession(string, int, int)   {}
func (h *helpHost) RoutePublic(string, string, string) error {
	return nil
}

func TestHelpToggleQuestionAndF1(t *testing.T) {
	m := Model{cfg: &config.File{}}
	next, cmd := m.key(tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune{'?'}})
	if cmd != nil {
		t.Fatal("opening help must not quit")
	}
	sm := next.(Model)
	if !sm.help {
		t.Fatal("? should open overlay")
	}
	next, _ = sm.key(tea.KeyMsg{Type: tea.KeyF1})
	sm = next.(Model)
	if sm.help {
		t.Fatal("F1 should close overlay")
	}
	next, _ = sm.key(tea.KeyMsg{Type: tea.KeyF1})
	sm = next.(Model)
	if !sm.help {
		t.Fatal("F1 should open overlay")
	}
	next, _ = sm.key(tea.KeyMsg{Type: tea.KeyEsc})
	sm = next.(Model)
	if sm.help {
		t.Fatal("Esc should close overlay")
	}
}

func TestHelpDoesNotStealCtrlCOrCtrlA(t *testing.T) {
	h := &helpHost{}
	m := Model{
		cfg:   &config.File{},
		help:  true,
		srv:   h,
		grid:  3,
		focus: 0,
		byName: map[string]listen.Snapshot{
			"luna": {Name: "luna", Alive: true},
		},
	}
	m.cfg.Fleets = []config.Fleet{{ID: "f", Agents: []config.Agent{{Name: "luna"}}}}
	next, cmd := m.key(tea.KeyMsg{Type: tea.KeyCtrlC})
	sm := next.(Model)
	if cmd != nil {
		t.Fatal("ctrl+c must not quit")
	}
	if !sm.help {
		t.Fatal("ctrl+c must leave overlay open")
	}
	if !bytes.Equal(h.wrote, []byte{0x03}) {
		t.Fatalf("ctrl+c should write ^C to PTY, got %q", h.wrote)
	}
	next, cmd = sm.key(tea.KeyMsg{Type: tea.KeyCtrlA})
	sm = next.(Model)
	if cmd != nil {
		t.Fatal("ctrl+a must not quit")
	}
	if !sm.prefix {
		t.Fatal("ctrl+a should still arm detach prefix")
	}
	next, cmd = sm.key(tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune{'d'}})
	if cmd == nil {
		t.Fatal("ctrl+a d should still detach")
	}
}

func TestHelpCatalogHasWiredBindings(t *testing.T) {
	var b strings.Builder
	for _, sec := range helpCatalog() {
		b.WriteString(sec.title)
		b.WriteByte('\n')
		for _, ln := range sec.lines {
			b.WriteString(ln.keys)
			b.WriteByte(' ')
			b.WriteString(ln.desc)
			b.WriteByte('\n')
		}
	}
	text := b.String()
	need := []string{
		"Grid", "Session", "Models", "Quit",
		"Alt+1", "Ctrl+G", "F3", "F4", "Ctrl+O", "F5",
		"Ctrl+S", "Ctrl+Q", "Ctrl-A then d", "Ctrl-A Ctrl-A",
		"2 / 4 / 8 / g", "L / End / Enter", "? / F1",
		"Tab", "Alt+←", "F7", "F8", "Alt+[", "F2", "Ctrl+M", "Ctrl+C",
	}
	for _, n := range need {
		if !strings.Contains(text, n) {
			t.Fatalf("catalog missing %q", n)
		}
	}
}

func TestHelpOverlayClipsWhenShort(t *testing.T) {
	m := Model{help: true, helpOff: 0, width: 40, height: 8}
	grid := strings.Repeat("x", 40) + "\n" + strings.Repeat("y", 40)
	out := m.renderHelpOverlay(grid, 40, 8)
	if !strings.Contains(out, "more") {
		t.Fatalf("expected more hint, got %q", out)
	}
	if strings.Count(out, "\n") > 12 {
		t.Fatal("overlay should not explode height")
	}
}

func TestHelpViewKeepsStatusBar(t *testing.T) {
	m := Model{
		cfg:    &config.File{Operator: "op"},
		grid:   3,
		width:  80,
		height: 24,
		help:   true,
	}
	v := m.View()
	if !strings.Contains(v, "Grid") {
		t.Fatal("overlay Grid section missing")
	}
	if !strings.Contains(v, "more") && !strings.Contains(v, "Session") {
		t.Fatal("expected Session or a more hint on a 24-row terminal")
	}
	if !strings.Contains(v, "?/F1") && !strings.Contains(v, "help") {
		t.Fatalf("status bar should remain, got %q", v[len(v)-200:])
	}
}

func TestF1IsHelpNotHub(t *testing.T) {
	m := Model{cfg: &config.File{Fleets: []config.Fleet{{ID: "a"}, {ID: "b"}}}, hubIdx: 1, grid: 3}
	next, _ := m.key(tea.KeyMsg{Type: tea.KeyF1})
	sm := next.(Model)
	if sm.hubIdx != 1 {
		t.Fatalf("F1 must not switch hub, hubIdx=%d", sm.hubIdx)
	}
	if !sm.help {
		t.Fatal("F1 opens help")
	}
}
