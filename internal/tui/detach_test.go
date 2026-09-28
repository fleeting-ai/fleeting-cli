package tui

import (
	"testing"

	tea "github.com/charmbracelet/bubbletea"
	"github.com/fleeting-ai/fleeting-cli/internal/listen"
)

func TestCtrlADDetachesWithoutCtrlC(t *testing.T) {
	m := Model{}
	next, cmd := m.key(tea.KeyMsg{Type: tea.KeyCtrlA})
	sm := next.(Model)
	if !sm.prefix {
		t.Fatal("ctrl+a should arm prefix")
	}
	if cmd != nil {
		t.Fatal("prefix must not quit")
	}
	next, cmd = sm.key(tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune{'d'}})
	if cmd == nil {
		t.Fatal("ctrl+a d should quit TUI (detach)")
	}
	_ = next
	m2 := Model{}
	next, cmd = m2.key(tea.KeyMsg{Type: tea.KeyCtrlC})
	if cmd != nil {
		t.Fatal("ctrl+c must not detach; it is for the focused PTY")
	}
	_ = next
}

func TestReplayJumpLive(t *testing.T) {
	t0 := listen.ReplayFrame{Screens: map[string]string{"luna": "old"}}
	m := Model{}
	m.WithReplay([]listen.ReplayFrame{t0, t0})
	if m.mode != "replay" {
		t.Fatal(m.mode)
	}
	next, _ := m.key(tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune{'l'}})
	sm := next.(Model)
	if sm.mode != "" || sm.player == nil || !sm.player.Live() {
		t.Fatalf("L should jump live mode=%q live=%v", sm.mode, sm.player.Live())
	}
}
