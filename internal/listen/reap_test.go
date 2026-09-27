package listen

import (
	"os/exec"
	"strings"
	"testing"
	"time"

	"github.com/richard-ginsberg/fleeting/internal/config"
)

func waitGone(t *testing.T, s *Server, name string) {
	t.Helper()
	deadline := time.Now().Add(2 * time.Second)
	for time.Now().Before(deadline) {
		s.harvestDead()
		s.mu.Lock()
		_, ok := s.sess[name]
		s.mu.Unlock()
		if !ok && len(s.Snapshots()) == 0 {
			return
		}
		time.Sleep(20 * time.Millisecond)
	}
	t.Fatalf("session %s did not reap", name)
}

func TestReapRemovesExitedSession(t *testing.T) {
	s := New(&config.File{}, t.TempDir())
	a := config.Agent{Name: "luna", Cmd: []string{"true"}}
	if err := s.Spawn("ad-hoc", a); err != nil {
		t.Fatal(err)
	}
	waitGone(t, s, "luna")
}

func TestSpawnAfterExitReusesName(t *testing.T) {
	s := New(&config.File{}, t.TempDir())
	a := config.Agent{Name: "luna", Cmd: []string{"true"}}
	if err := s.Spawn("ad-hoc", a); err != nil {
		t.Fatal(err)
	}
	waitGone(t, s, "luna")
	if err := s.Spawn("ad-hoc", a); err != nil {
		t.Fatalf("second launch should succeed: %v", err)
	}
	waitGone(t, s, "luna")
}

func TestSpawnRejectsLiveSession(t *testing.T) {
	s := New(&config.File{}, t.TempDir())
	a := config.Agent{Name: "luna", Cmd: []string{"sleep", "30"}}
	if err := s.Spawn("ad-hoc", a); err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() {
		s.mu.Lock()
		se := s.sess["luna"]
		s.mu.Unlock()
		if se != nil && se.Cmd != nil && se.Cmd.Process != nil {
			_ = se.Cmd.Process.Kill()
		}
	})
	deadline := time.Now().Add(2 * time.Second)
	for time.Now().Before(deadline) && len(s.Snapshots()) == 0 {
		time.Sleep(10 * time.Millisecond)
	}
	err := s.Spawn("ad-hoc", a)
	if err == nil {
		t.Fatal("expected already running")
	}
	if !strings.Contains(err.Error(), "session luna already running") {
		t.Fatalf("got %v", err)
	}
}

func TestSpawnClearsStaleRegistry(t *testing.T) {
	s := New(&config.File{}, t.TempDir())
	s.mu.Lock()
	s.sess["luna"] = &Session{
		Agent: config.Agent{Name: "luna"},
		Cmd:   exec.Command("true"),
		alive: false,
	}
	s.mu.Unlock()
	if err := s.Spawn("ad-hoc", config.Agent{Name: "luna", Cmd: []string{"true"}}); err != nil {
		t.Fatalf("stale map entry should not block: %v", err)
	}
	waitGone(t, s, "luna")
}

func TestSpawnClearsStaleLiveFlagWithoutPID(t *testing.T) {
	s := New(&config.File{}, t.TempDir())
	s.mu.Lock()
	s.sess["luna"] = &Session{
		Agent: config.Agent{Name: "luna"},
		alive: true,
	}
	s.mu.Unlock()
	if err := s.Spawn("ad-hoc", config.Agent{Name: "luna", Cmd: []string{"true"}}); err != nil {
		t.Fatalf("zombie registry should be cleared: %v", err)
	}
	waitGone(t, s, "luna")
}
