package listen

import (
	"encoding/json"
	"fmt"
	"net"
	"sync"
	"time"

	"github.com/richard-ginsberg/fleeting/internal/config"
)

// Client talks to a running daemon over the Unix socket.
type Client struct {
	c      net.Conn
	mu     sync.Mutex
	enc    *json.Encoder
	dec    *json.Decoder
	pend   chan Packet
	kicked chan struct{}
	closed chan struct{}
}

func Dial() (*Client, error) {
	return DialPath(config.SocketPath())
}

func DialPath(path string) (*Client, error) {
	c, err := net.DialTimeout("unix", path, time.Second)
	if err != nil {
		return nil, err
	}
	cl := &Client{
		c:      c,
		enc:    json.NewEncoder(c),
		dec:    json.NewDecoder(c),
		pend:   make(chan Packet, 8),
		kicked: make(chan struct{}),
		closed: make(chan struct{}),
	}
	go cl.readLoop()
	return cl, nil
}

func (c *Client) readLoop() {
	defer close(c.closed)
	for {
		var p Packet
		if err := c.dec.Decode(&p); err != nil {
			select {
			case <-c.kicked:
			default:
				close(c.kicked)
			}
			return
		}
		if p.Op == "kick" {
			select {
			case <-c.kicked:
			default:
				close(c.kicked)
			}
			continue
		}
		select {
		case c.pend <- p:
		case <-c.closed:
			return
		}
	}
}

func (c *Client) rpc(req Packet) (Packet, error) {
	c.mu.Lock()
	defer c.mu.Unlock()
	if err := c.enc.Encode(req); err != nil {
		return Packet{}, err
	}
	select {
	case p := <-c.pend:
		if p.Err != "" && !p.OK {
			return p, fmt.Errorf("%s", p.Err)
		}
		return p, nil
	case <-time.After(15 * time.Second):
		return Packet{}, fmt.Errorf("daemon rpc timeout (%s)", req.Op)
	case <-c.closed:
		return Packet{}, fmt.Errorf("daemon disconnected")
	}
}

func (c *Client) Close() error {
	return c.c.Close()
}

func (c *Client) Kicked() <-chan struct{} {
	return c.kicked
}

func (c *Client) Attach(steal bool) (Packet, error) {
	p, err := c.rpc(Packet{Op: "attach", Steal: steal})
	if err != nil {
		return p, err
	}
	if !p.OK {
		return p, fmt.Errorf("%s", p.Err)
	}
	return p, nil
}

func (c *Client) DetachOthers() error {
	_, err := c.rpc(Packet{Op: "detach"})
	return err
}

func (c *Client) QueryStatus() (Status, error) {
	p, err := c.rpc(Packet{Op: "status"})
	if err != nil {
		return Status{}, err
	}
	if p.Status == nil {
		return Status{}, fmt.Errorf("no status")
	}
	return *p.Status, nil
}

func (c *Client) Snapshots() []Snapshot {
	p, err := c.rpc(Packet{Op: "list"})
	if err != nil {
		return nil
	}
	return p.Snaps
}

func (c *Client) Write(name string, data []byte) error {
	_, err := c.rpc(Packet{Op: "input", Name: name, Data: string(data)})
	return err
}

func (c *Client) Spawn(fleetID string, a config.Agent) error {
	ag := a
	_, err := c.rpc(Packet{Op: "spawn", Fleet: fleetID, Agent: &ag})
	return err
}

func (c *Client) Restart(name string) error {
	_, err := c.rpc(Packet{Op: "restart", Name: name})
	return err
}

func (c *Client) Resize(cols, rows int) {
	_, _ = c.rpc(Packet{Op: "resize", Cols: cols, Rows: rows})
}

func (c *Client) ResizeSession(name string, cols, rows int) {
	_, _ = c.rpc(Packet{Op: "resize", Name: name, Cols: cols, Rows: rows})
}

func (c *Client) RoutePublic(from, to, body string) error {
	_, err := c.rpc(Packet{Op: "msg", From: from, To: to, Body: body})
	return err
}

func (c *Client) Down() error {
	_, err := c.rpc(Packet{Op: "down"})
	return err
}

func Ping() error {
	cl, err := Dial()
	if err != nil {
		return err
	}
	defer cl.Close()
	_, err = cl.QueryStatus()
	return err
}
