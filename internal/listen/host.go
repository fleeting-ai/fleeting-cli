package listen

import "github.com/richard-ginsberg/fleeting/internal/config"

// Host is the TUI's view of a listener. The daemon Server implements it
// in-process; Client implements it over the Unix socket.
type Host interface {
	Snapshots() []Snapshot
	Write(name string, data []byte) error
	Spawn(fleetID string, a config.Agent) error
	Replace(fleetID string, a config.Agent) error
	Restart(name string) error
	Resize(cols, rows int)
	ResizeSession(name string, cols, rows int)
	RoutePublic(from, to, body string) error
}
