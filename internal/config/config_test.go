package config

import (
	"os"
	"path/filepath"
	"testing"
)

func TestValidateFourLetterAndDenyShape(t *testing.T) {
	dir := t.TempDir()
	p := filepath.Join(dir, "fleet.yaml")
	if err := os.WriteFile(p, []byte(Example()), 0o644); err != nil {
		t.Fatal(err)
	}
	f, err := Load(p)
	if err != nil {
		t.Fatal(err)
	}
	if f.Grid != 3 {
		t.Fatalf("grid %d", f.Grid)
	}
	var nova *Agent
	for i := range f.Fleets[0].Agents {
		a := &f.Fleets[0].Agents[i]
		if a.Name == "nova" {
			nova = a
		}
	}
	if nova == nil {
		t.Fatal("missing nova")
	}
	if len(nova.Peers) != 1 || nova.Peers[0] != "risa" {
		t.Fatalf("nova should only peer risa, got %v", nova.Peers)
	}
	if len(nova.Cmd) < 3 || nova.Cmd[1] != "--profile" || nova.Cmd[2] != "nova" {
		if len(nova.Cmd) == 0 || nova.Cmd[0] != "bash" {
			t.Fatalf("expected omp --profile nova, got %v", nova.Cmd)
		}
	}
}
