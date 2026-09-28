package listen

import (
	"context"
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"

	"github.com/fleeting-ai/fleeting-cli/internal/bus"
	"github.com/fleeting-ai/fleeting-cli/internal/config"
	"github.com/fleeting-ai/fleeting-cli/internal/harness"
)

func TestSpawnSetsClaudeAndCodexHomes(t *testing.T) {
	home := t.TempDir()
	t.Setenv("FLEETING_HOME", home)
	s := New(&config.File{}, home)

	claudeBin := filepath.Join(home, "claude")
	if err := os.WriteFile(claudeBin, []byte("#!/bin/sh\nprintenv CLAUDE_CONFIG_DIR > \"$FLEETING_HOME/claude.env\"\nsleep 30\n"), 0o755); err != nil {
		t.Fatal(err)
	}
	if err := s.Spawn("ad-hoc", config.Agent{Name: "luna", Harness: "claude", Cmd: []string{claudeBin}}); err != nil {
		t.Fatal(err)
	}
	waitFile(t, filepath.Join(home, "claude.env"), harness.ClaudeConfigDir(home, "luna"))

	codexBin := filepath.Join(home, "codex")
	if err := os.WriteFile(codexBin, []byte("#!/bin/sh\nprintenv CODEX_HOME > \"$FLEETING_HOME/codex.env\"\nsleep 30\n"), 0o755); err != nil {
		t.Fatal(err)
	}
	if err := s.Spawn("ad-hoc", config.Agent{Name: "nova", Harness: "codex", Cmd: []string{codexBin}}); err != nil {
		t.Fatal(err)
	}
	waitFile(t, filepath.Join(home, "codex.env"), harness.CodexHome(home, "nova"))

	s.KillAll()
}

func TestRouteMailBusDown(t *testing.T) {
	home := t.TempDir()
	t.Setenv("FLEETING_HOME", home)
	s := New(&config.File{Grid: 3, Fleets: []config.Fleet{{ID: "f", Agents: []config.Agent{
		{Name: "nova", Role: "worker", Peers: []string{"risa"}, Cmd: []string{"sleep", "60"}},
		{Name: "risa", Role: "foreman", Peers: []string{"nova"}, Cmd: []string{"sleep", "60"}},
	}}}}, home)
	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()
	if err := s.Start(ctx); err != nil {
		t.Fatal(err)
	}
	defer s.KillAll()
	err := s.RouteMail(bus.Send{From: "risa", To: "nova", Body: "hi"})
	if err == nil {
		t.Fatal("expected bus down error")
	}
}

func TestSpawnInfersHarnessFromClaudeBin(t *testing.T) {
	s := New(&config.File{}, t.TempDir())
	if err := s.Spawn("ad-hoc", config.Agent{Name: "kite", Cmd: []string{"true"}}); err != nil {
		t.Fatal(err)
	}
	waitGone(t, s, "kite")

	home := t.TempDir()
	t.Setenv("FLEETING_HOME", home)
	s = New(&config.File{}, home)
	bin := filepath.Join(home, "claude")
	if err := os.WriteFile(bin, []byte("#!/bin/sh\nsleep 30\n"), 0o755); err != nil {
		t.Fatal(err)
	}
	if err := s.Spawn("ad-hoc", config.Agent{Name: "veil", Cmd: []string{bin}}); err != nil {
		t.Fatal(err)
	}
	s.mu.Lock()
	se := s.sess["veil"]
	got := ""
	if se != nil {
		got = se.Agent.Harness
	}
	s.mu.Unlock()
	s.KillAll()
	if got != "claude" {
		t.Fatalf("inferred harness %q", got)
	}
}

func waitFile(t *testing.T, path, want string) {
	t.Helper()
	deadline := time.Now().Add(2 * time.Second)
	var body []byte
	for time.Now().Before(deadline) {
		body, _ = os.ReadFile(path)
		if strings.TrimSpace(string(body)) == want {
			return
		}
		time.Sleep(20 * time.Millisecond)
	}
	t.Fatalf("%s: got %q want %q", path, strings.TrimSpace(string(body)), want)
}

func TestReplaceKillsPlaceholder(t *testing.T) {
	s := New(&config.File{}, t.TempDir())
	a := config.Agent{Name: "risa", Cmd: []string{"sleep", "30"}}
	if err := s.Spawn("f", a); err != nil {
		t.Fatal(err)
	}
	if err := s.Replace("f", config.Agent{Name: "risa", Cmd: []string{"true"}}); err != nil {
		t.Fatal(err)
	}
	waitGone(t, s, "risa")
}
