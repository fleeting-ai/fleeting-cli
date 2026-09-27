package listen

import (
	"context"
	"strings"
	"testing"
	"time"

	"github.com/richard-ginsberg/fleeting/internal/config"
)

func TestAttachDetachKeepsPTYAndReplays(t *testing.T) {
	home := t.TempDir()
	t.Setenv("FLEETING_HOME", home)
	s := New(&config.File{Grid: 3}, home)
	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()
	if err := s.Start(ctx); err != nil {
		t.Fatal(err)
	}
	a := config.Agent{Name: "luna", Cmd: []string{"bash", "-lc", "i=0; while true; do echo tick-$i; i=$((i+1)); sleep 0.05; done"}}
	if err := s.Spawn("t", a); err != nil {
		t.Fatal(err)
	}
	deadline := time.Now().Add(2 * time.Second)
	for time.Now().Before(deadline) && !strings.Contains(joinScreens(s), "tick-") {
		time.Sleep(20 * time.Millisecond)
	}
	if !strings.Contains(joinScreens(s), "tick-") {
		t.Fatal("expected PTY output before attach")
	}
	cl, err := Dial()
	if err != nil {
		t.Fatal(err)
	}
	got, err := cl.Attach(false)
	if err != nil {
		t.Fatal(err)
	}
	if len(got.Replay) == 0 {
		t.Fatal("expected replay frames of missed output")
	}
	if !s.Attached() {
		t.Fatal("server should show attached")
	}
	_ = cl.Close()
	deadline = time.Now().Add(2 * time.Second)
	for time.Now().Before(deadline) && s.Attached() {
		time.Sleep(10 * time.Millisecond)
	}
	if s.Attached() {
		t.Fatal("close should detach")
	}
	if len(s.Snapshots()) == 0 {
		t.Fatal("PTY died on client close")
	}
	time.Sleep(200 * time.Millisecond)
	cl2, err := Dial()
	if err != nil {
		t.Fatal(err)
	}
	defer cl2.Close()
	got2, err := cl2.Attach(false)
	if err != nil {
		t.Fatal(err)
	}
	if len(got2.Replay) == 0 {
		t.Fatal("reattach should replay output missed while detached")
	}
}

func TestStealAttachKicksOther(t *testing.T) {
	home := t.TempDir()
	t.Setenv("FLEETING_HOME", home)
	s := New(&config.File{Grid: 3}, home)
	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()
	if err := s.Start(ctx); err != nil {
		t.Fatal(err)
	}
	a := config.Agent{Name: "nova", Cmd: []string{"bash", "-lc", "sleep 30"}}
	if err := s.Spawn("t", a); err != nil {
		t.Fatal(err)
	}
	cl1, err := Dial()
	if err != nil {
		t.Fatal(err)
	}
	if _, err := cl1.Attach(false); err != nil {
		t.Fatal(err)
	}
	cl2, err := Dial()
	if err != nil {
		t.Fatal(err)
	}
	if _, err := cl2.Attach(false); err == nil {
		t.Fatal("second attach without -d should fail")
	}
	cl2b, err := Dial()
	if err != nil {
		t.Fatal(err)
	}
	defer cl2b.Close()
	if _, err := cl2b.Attach(true); err != nil {
		t.Fatal(err)
	}
	select {
	case <-cl1.Kicked():
	case <-time.After(2 * time.Second):
		t.Fatal("first client was not kicked")
	}
}

func joinScreens(s *Server) string {
	var b strings.Builder
	for _, sn := range s.Snapshots() {
		b.WriteString(sn.Screen)
	}
	return b.String()
}
