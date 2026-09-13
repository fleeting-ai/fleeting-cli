package tui

import (
	"testing"

	"github.com/richard-ginsberg/fleeting/internal/config"
	"github.com/richard-ginsberg/fleeting/internal/listen"
	"github.com/richard-ginsberg/fleeting/internal/workspace"
)

func TestModelKindFollowsFocus(t *testing.T) {
	m := Model{
		grid: 3,
		cfg:  &config.File{Fleets: []config.Fleet{{Agents: []config.Agent{{Name: "nova"}}}}},
		byName: map[string]listen.Snapshot{
			"nova": {Name: "nova", Harness: "omp", Alive: true},
		},
	}
	if m.modelKind() != "omp" {
		t.Fatalf("got %s", m.modelKind())
	}
	m.focus = 8
	m.extras = map[int]workspace.Cell{8: {Global: 8, Name: "luna", Harness: "pi", Cmd: []string{"pi"}}}
	m.byName["luna"] = listen.Snapshot{Name: "luna", Harness: "pi", Alive: true}
	if m.modelKind() != "pi" {
		t.Fatalf("C3 got %s", m.modelKind())
	}
}
