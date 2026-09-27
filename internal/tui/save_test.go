package tui

import (
	"testing"

	"github.com/richard-ginsberg/fleeting/internal/config"
	"github.com/richard-ginsberg/fleeting/internal/listen"
	"github.com/richard-ginsberg/fleeting/internal/workspace"
)

func TestDirtyWhenZoomChanges(t *testing.T) {
	m := Model{grid: 3, extras: map[int]workspace.Cell{}}
	m.savedFP = workspace.Fingerprint(m.workspaceFile())
	if m.dirty() {
		t.Fatal("fresh should be clean")
	}
	m.zoomSpan = 2
	if !m.dirty() {
		t.Fatal("zoom should dirty")
	}
}

func TestDirtyWhenAdHocCellAdded(t *testing.T) {
	m := Model{grid: 3, extras: map[int]workspace.Cell{}}
	m.savedFP = workspace.Fingerprint(m.workspaceFile())
	m.extras[8] = workspace.Cell{Global: 8, Name: "luna", Harness: "pi", Cmd: []string{"pi"}}
	if !m.dirty() {
		t.Fatal("new cell should dirty")
	}
}

func TestCaptureFillsHarnessFromFleet(t *testing.T) {
	m := Model{
		grid: 3,
		cfg: &config.File{Fleets: []config.Fleet{{
			ID: "local-core",
			Agents: []config.Agent{
				{Name: "nova", Harness: "omp", Cmd: []string{"omp", "--profile", "nova"}},
				{Name: "bolt", Harness: "omp", Cmd: []string{"omp", "--profile", "bolt"}},
				{Name: "kite", Harness: "omp", Lane: "module", Cmd: []string{"omp", "--profile", "kite"}},
			},
		}}},
		byName: map[string]listen.Snapshot{
			"nova": {Name: "nova", Alive: true},
			"bolt": {Name: "bolt", Alive: true},
			"kite": {Name: "kite", Fleet: "local-core", Lane: "module", Alive: true},
		},
		extras: map[int]workspace.Cell{},
	}
	m = m.captureLiveCells()
	c := m.extras[2]
	if c.Name != "kite" || c.Harness != "omp" || len(c.Cmd) == 0 {
		t.Fatalf("captured %+v", c)
	}
	if miss := c.Missing(); len(miss) != 0 {
		t.Fatalf("still missing %v", miss)
	}
}
