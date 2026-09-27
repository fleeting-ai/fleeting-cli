package tui

import (
	"testing"

	"github.com/richard-ginsberg/fleeting/internal/config"
	"github.com/richard-ginsberg/fleeting/internal/listen"
	"github.com/richard-ginsberg/fleeting/internal/persona"
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
