package tui

import (
	"testing"

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
