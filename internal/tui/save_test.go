package tui

import (
	"fmt"
	"os"
	"path/filepath"
	"testing"

	"github.com/fleeting-ai/fleeting-cli/internal/bus"
	"github.com/fleeting-ai/fleeting-cli/internal/config"
	"github.com/fleeting-ai/fleeting-cli/internal/harness"
	"github.com/fleeting-ai/fleeting-cli/internal/listen"
	"github.com/fleeting-ai/fleeting-cli/internal/workspace"
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

type restoreHost struct {
	snaps   []listen.Snapshot
	spawn   int
	replace int
	err     error
}

func (h *restoreHost) Snapshots() []listen.Snapshot { return h.snaps }
func (h *restoreHost) Write(string, []byte) error   { return nil }
func (h *restoreHost) Spawn(string, config.Agent) error {
	h.spawn++
	return h.err
}
func (h *restoreHost) Replace(string, config.Agent) error {
	h.replace++
	return h.err
}
func (h *restoreHost) Restart(string) error           { return nil }
func (h *restoreHost) Resize(int, int)                {}
func (h *restoreHost) ResizeSession(string, int, int) {}
func (h *restoreHost) RouteMail(bus.Send) error       { return nil }
func (h *restoreHost) Coord(p listen.Packet) (listen.Packet, error) {
	return listen.Packet{OK: true}, nil
}
func (h *restoreHost) Status() listen.Status { return listen.Status{} }

func TestRestoreKeepsLiveAdHocOnC1(t *testing.T) {
	dir := t.TempDir()
	t.Setenv("FLEETING_WORKSPACE", filepath.Join(dir, "workspace.yaml"))
	ws := workspace.File{
		Grid: 3,
		Cells: []workspace.Cell{{
			Global:  6,
			Name:    "luna",
			Harness: "cursor",
			Fleet:   "ad-hoc",
			Cmd:     []string{"/usr/bin/agent"},
		}},
	}
	if err := workspace.Save(ws); err != nil {
		t.Fatal(err)
	}
	h := &restoreHost{
		snaps: []listen.Snapshot{{Name: "luna", Harness: "cursor", Alive: true}},
		err:   fmt.Errorf("session luna already running"),
	}
	m := Model{
		srv:    h,
		grid:   3,
		cfg:    &config.File{Grid: 3, Fleets: []config.Fleet{{ID: "local-core", Agents: []config.Agent{{Name: "nova"}}}}},
		extras: map[int]workspace.Cell{},
	}
	m.restoreWorkspace()
	if h.spawn != 0 {
		t.Fatalf("should not respawn live session, spawn=%d", h.spawn)
	}
	c, ok := m.extras[6]
	if !ok || c.Name != "luna" {
		t.Fatalf("C1 extras %+v", m.extras)
	}
	if m.orderedNames()[6] != "luna" {
		t.Fatalf("ordered names %v", m.orderedNames())
	}
}

func TestUpgradePlaceholdersReplacesOMP(t *testing.T) {
	dir := t.TempDir()
	t.Setenv("HOME", dir)
	omp := filepath.Join(dir, "omp")
	if err := os.WriteFile(omp, []byte("#!/bin/sh\n"), 0o755); err != nil {
		t.Fatal(err)
	}
	t.Setenv("PATH", dir)
	h := &restoreHost{snaps: []listen.Snapshot{{
		Name:    "risa",
		Fleet:   "local-core",
		Harness: "omp",
		Alive:   true,
		Cmd:     harness.MissingCmd("omp", "risa"),
		Screen:  "omp not found on PATH",
	}}}
	m := Model{
		srv: h,
		cfg: &config.File{Fleets: []config.Fleet{{
			ID: "local-core",
			Agents: []config.Agent{
				{Name: "risa", Role: "foreman", Lane: "pace", Harness: "omp", Hub: true},
			},
		}}},
	}
	m.upgradePlaceholders()
	if h.replace != 1 {
		t.Fatalf("replace=%d", h.replace)
	}
}
