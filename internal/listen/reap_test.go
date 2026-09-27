package listen

import (
	"testing"
	"time"

	"github.com/richard-ginsberg/fleeting/internal/config"
)

func TestReapRemovesExitedSession(t *testing.T) {
	s := New(&config.File{}, t.TempDir())
	a := config.Agent{Name: "luna", Cmd: []string{"true"}}
	if err := s.Spawn("ad-hoc", a); err != nil {
		t.Fatal(err)
	}
	deadline := time.Now().Add(2 * time.Second)
	for time.Now().Before(deadline) {
		if len(s.Snapshots()) == 0 {
			s.mu.Lock()
			_, ok := s.sess["luna"]
			s.mu.Unlock()
			if ok {
				t.Fatal("dead session still in map")
			}
			return
		}
		time.Sleep(20 * time.Millisecond)
	}
	t.Fatal("session did not reap")
}
