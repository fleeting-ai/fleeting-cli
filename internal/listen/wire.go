package listen

import (
	"github.com/richard-ginsberg/fleeting/internal/bus"
	"github.com/richard-ginsberg/fleeting/internal/config"
)

// Packet is the Unix-socket JSON protocol (one object per line).
type Packet struct {
	Op      string        `json:"op"`
	Name    string        `json:"name,omitempty"`
	Data    string        `json:"data,omitempty"`
	To      string        `json:"to,omitempty"`
	From    string        `json:"from,omitempty"`
	Body    string        `json:"body,omitempty"`
	Action  string        `json:"action,omitempty"`
	Team    string        `json:"team,omitempty"`
	ReplyTo string        `json:"reply_to,omitempty"`
	Thread  string        `json:"thread,omitempty"`
	Kind    string        `json:"kind,omitempty"`
	Key     string        `json:"key,omitempty"`
	Limit   int           `json:"limit,omitempty"`
	All     bool          `json:"all,omitempty"`
	Steal   bool          `json:"steal,omitempty"`
	Cols    int           `json:"cols,omitempty"`
	Rows    int           `json:"rows,omitempty"`
	Fleet   string        `json:"fleet,omitempty"`
	Agent   *config.Agent `json:"agent,omitempty"`
	OK      bool          `json:"ok"`
	Err     string        `json:"err,omitempty"`
	Snaps   []Snapshot    `json:"snaps,omitempty"`
	Replay  []ReplayFrame `json:"replay,omitempty"`
	Status  *Status       `json:"status,omitempty"`
	Mail    []bus.Record  `json:"mail,omitempty"`
}

// Status is what `fleeting -ls` prints.
type Status struct {
	PID      int      `json:"pid"`
	Socket   string   `json:"socket"`
	Attached bool     `json:"attached"`
	Agents   int      `json:"agents"`
	Names    []string `json:"names,omitempty"`
	Bus      string   `json:"bus,omitempty"`
	Store    string   `json:"store,omitempty"`
	BusURL   string   `json:"bus_url,omitempty"`
	StoreURL string   `json:"store_url,omitempty"`
}
