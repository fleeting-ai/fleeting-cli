package listen

import (
	"github.com/fleeting-ai/fleeting-cli/internal/bus"
	"github.com/fleeting-ai/fleeting-cli/internal/config"
)

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
	RouteMail(s bus.Send) error
	Coord(p Packet) (Packet, error)
	Status() Status
}
