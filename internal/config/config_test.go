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

func TestDefaultCmdClaudeAndCodex(t *testing.T) {
	dir := t.TempDir()
	for _, bin := range []string{"claude", "codex"} {
		p := filepath.Join(dir, bin)
		if err := os.WriteFile(p, []byte("#!/bin/sh\n"), 0o755); err != nil {
			t.Fatal(err)
		}
	}
	t.Setenv("PATH", dir)

	claude := DefaultCmd(&Agent{Name: "luna", Harness: "claude"})
	if len(claude) != 1 || filepath.Base(claude[0]) != "claude" {
		t.Fatalf("claude cmd %v", claude)
	}
	codex := DefaultCmd(&Agent{Name: "nova", Harness: "codex"})
	if len(codex) != 1 || filepath.Base(codex[0]) != "codex" {
		t.Fatalf("codex cmd %v", codex)
	}
}

func TestValidateFillsClaudeCmd(t *testing.T) {
	dir := t.TempDir()
	bin := filepath.Join(dir, "claude")
	if err := os.WriteFile(bin, []byte("#!/bin/sh\n"), 0o755); err != nil {
		t.Fatal(err)
	}
	t.Setenv("PATH", dir)
	p := filepath.Join(dir, "fleet.yaml")
	body := `operator: r
grid: 3
fleets:
  - id: mixed
    agents:
      - name: luna
        role: worker
        lane: code
        harness: claude
`
	if err := os.WriteFile(p, []byte(body), 0o644); err != nil {
		t.Fatal(err)
	}
	f, err := Load(p)
	if err != nil {
		t.Fatal(err)
	}
	a := f.Fleets[0].Agents[0]
	if a.Harness != "claude" || len(a.Cmd) != 1 || filepath.Base(a.Cmd[0]) != "claude" {
		t.Fatalf("got harness=%s cmd=%v", a.Harness, a.Cmd)
	}
}
