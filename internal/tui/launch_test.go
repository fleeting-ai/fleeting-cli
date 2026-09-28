package tui

import (
	"testing"

	"github.com/richard-ginsberg/fleeting/internal/config"
	"github.com/richard-ginsberg/fleeting/internal/harness"
	"github.com/richard-ginsberg/fleeting/internal/listen"
	"github.com/richard-ginsberg/fleeting/internal/persona"
	"github.com/richard-ginsberg/fleeting/internal/workspace"
)

func TestPersonaNextSkipsTaken(t *testing.T) {
	if persona.Next(map[string]bool{"luna": true}) != "nova" {
		t.Fatal("expected nova")
	}
}

func TestSnapAtBlankWhenDead(t *testing.T) {
	m := Model{
		grid: 3,
		cfg: &config.File{
			Fleets: []config.Fleet{{ID: "f", Agents: []config.Agent{{Name: "nova"}}}},
		},
		byName: map[string]listen.Snapshot{
			"nova": {Name: "nova", Alive: false},
		},
	}
	if m.snapAt(0) != nil {
		t.Fatal("dead session should leave the cell blank")
	}
}

func TestOpenLaunchAllowsPlaceholder(t *testing.T) {
	cmd := harness.MissingCmd("omp", "hiro")
	hiro := listen.Snapshot{Name: "hiro", Alive: true, Cmd: cmd, Screen: "omp not found on PATH"}
	m := Model{
		grid:  3,
		focus: 4, // B2
		cfg: &config.File{Grid: 3, Fleets: []config.Fleet{{
			Agents: []config.Agent{
				{Name: "nova"}, {Name: "bolt"}, {Name: "kite"},
				{Name: "veil"}, {Name: "hiro"}, {Name: "risa"},
			},
		}}},
		snaps:  []listen.Snapshot{hiro},
		byName: map[string]listen.Snapshot{"hiro": hiro},
	}
	next, _ := m.openLaunch()
	sm := next.(Model)
	if sm.lastErr != "" {
		t.Fatalf("placeholder should be replaceable, err=%s", sm.lastErr)
	}
	if sm.mode != "launch" {
		t.Fatalf("mode %q", sm.mode)
	}
}

func TestNameForLaunchReusesDeadCellPersona(t *testing.T) {
	m := Model{
		grid: 3,
		cfg:  &config.File{},
		extras: map[int]workspace.Cell{
			2: {Global: 2, Name: "luna", Harness: "omp", Cmd: []string{"omp", "--profile", "luna"}},
		},
	}
	if got := m.nameForLaunch(2); got != "luna" {
		t.Fatalf("dead A3 should relaunch luna, got %q", got)
	}
	in := harness.Installed{Spec: harness.Spec{ID: "omp"}, Path: "/usr/bin/omp"}
	cmd := in.CmdForAgent(m.nameForLaunch(2))
	if len(cmd) != 3 || cmd[1] != "--profile" || cmd[2] != "luna" {
		t.Fatalf("expected omp --profile luna, got %v", cmd)
	}
}
