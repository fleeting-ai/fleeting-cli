package listen

import (
	"context"
	"encoding/json"
	"fmt"
	"net"
	"os"
	"os/exec"
	"path/filepath"
	"sync"
	"time"

	"github.com/creack/pty"
	"github.com/richard-ginsberg/fleeting/internal/config"
	"github.com/richard-ginsberg/fleeting/internal/presence"
	"github.com/richard-ginsberg/fleeting/internal/spool"
)

// Server is the local Fleeting listener. Remote hosts will speak the same
// length-prefixed JSON protocol over TCP later; v1 is a Unix socket only.
type Server struct {
	cfg   *config.File
	path  string
	mu    sync.Mutex
	sess  map[string]*Session
	ln    net.Listener
	home  string
}

type Session struct {
	Agent   config.Agent
	FleetID string
	GUID    string
	Cmd     *exec.Cmd
	Pty     *os.File
	mu      sync.Mutex
	buf     []byte
	last    time.Time
	alive   bool
	err     string
}

type Snapshot struct {
	Name    string    `json:"name"`
	Fleet   string    `json:"fleet"`
	GUID    string    `json:"guid"`
	Role    string    `json:"role"`
	Lane    string    `json:"lane"`
	Hub     bool      `json:"hub"`
	Peers   []string  `json:"peers"`
	Alive   bool      `json:"alive"`
	Err     string    `json:"err,omitempty"`
	Last    time.Time `json:"last"`
	Status  string    `json:"status"`
	Screen  string    `json:"screen"`
}

func New(cfg *config.File, home string) *Server {
	return &Server{
		cfg:  cfg,
		path: config.SocketPath(),
		sess: map[string]*Session{},
		home: home,
	}
}

func (s *Server) Start(ctx context.Context) error {
	if err := os.MkdirAll(filepath.Dir(s.path), 0o755); err != nil {
		return err
	}
	_ = os.Remove(s.path)
	ln, err := net.Listen("unix", s.path)
	if err != nil {
		return err
	}
	s.ln = ln

	for _, fl := range s.cfg.Fleets {
		for _, a := range fl.Agents {
			if err := s.spawn(fl.ID, a); err != nil {
				return err
			}
		}
	}

	go s.accept(ctx)
	go func() {
		<-ctx.Done()
		_ = ln.Close()
		s.KillAll()
	}()
	return nil
}

func (s *Server) spawn(fleetID string, a config.Agent) error {
	guid := a.ResumeGUID
	if guid == "" {
		guid = fmt.Sprintf("%s-%d", a.Name, time.Now().UnixNano())
	}
	cmd := exec.Command(a.Cmd[0], a.Cmd[1:]...)
	cmd.Env = append(os.Environ(),
		"FLEETING_NAME="+a.Name,
		"FLEETING_LANE="+a.Lane,
		"FLEETING_ROLE="+a.Role,
		"FLEETING_FLEET="+fleetID,
		"FLEETING_GUID="+guid,
	)
	ptmx, err := pty.Start(cmd)
	if err != nil {
		return fmt.Errorf("%s: %w", a.Name, err)
	}
	_ = pty.Setsize(ptmx, &pty.Winsize{Rows: 24, Cols: 80})
	se := &Session{
		Agent:   a,
		FleetID: fleetID,
		GUID:    guid,
		Cmd:     cmd,
		Pty:     ptmx,
		alive:   true,
		last:    time.Now(),
		buf:     make([]byte, 0, 8192),
	}
	s.mu.Lock()
	s.sess[a.Name] = se
	s.mu.Unlock()
	go se.readLoop()
	go se.waitLoop()
	return nil
}

func (se *Session) readLoop() {
	buf := make([]byte, 4096)
	for {
		n, err := se.Pty.Read(buf)
		if n > 0 {
			se.mu.Lock()
			se.buf = append(se.buf, buf[:n]...)
			if len(se.buf) > 32*1024 {
				se.buf = se.buf[len(se.buf)-16*1024:]
			}
			se.last = time.Now()
			se.mu.Unlock()
		}
		if err != nil {
			return
		}
	}
}

func (se *Session) waitLoop() {
	err := se.Cmd.Wait()
	se.mu.Lock()
	se.alive = false
	if err != nil {
		se.err = err.Error()
	}
	se.mu.Unlock()
}

func (s *Server) KillAll() {
	s.mu.Lock()
	defer s.mu.Unlock()
	for _, se := range s.sess {
		if se.Pty != nil {
			_ = se.Pty.Close()
		}
		if se.Cmd != nil && se.Cmd.Process != nil {
			_ = se.Cmd.Process.Kill()
		}
	}
}

func (s *Server) Snapshots() []Snapshot {
	s.mu.Lock()
	defer s.mu.Unlock()
	out := make([]Snapshot, 0, len(s.sess))
	for _, se := range s.sess {
		se.mu.Lock()
		screen := tailScreen(se.buf, 12)
		st := presence.Of(se.alive, se.last, screen)
		out = append(out, Snapshot{
			Name:   se.Agent.Name,
			Fleet:  se.FleetID,
			GUID:   se.GUID,
			Role:   se.Agent.Role,
			Lane:   se.Agent.Lane,
			Hub:    se.Agent.Hub,
			Peers:  se.Agent.Peers,
			Alive:  se.alive,
			Err:    se.err,
			Last:   se.last,
			Status: st,
			Screen: screen,
		})
		se.mu.Unlock()
	}
	return out
}

func tailScreen(b []byte, lines int) string {
	s := string(b)
	// keep last N lines
	n := 0
	i := len(s)
	for i > 0 && n < lines {
		i--
		if s[i] == '\n' {
			n++
		}
	}
	if i < 0 {
		i = 0
	}
	if n >= lines && i+1 < len(s) {
		return s[i+1:]
	}
	return s
}

func (s *Server) Write(name string, data []byte) error {
	s.mu.Lock()
	se := s.sess[name]
	s.mu.Unlock()
	if se == nil {
		return fmt.Errorf("unknown agent %s", name)
	}
	_, err := se.Pty.Write(data)
	return err
}

type wire struct {
	Op     string `json:"op"`
	Name   string `json:"name,omitempty"`
	Data   string `json:"data,omitempty"`
	To     string `json:"to,omitempty"`
	From   string `json:"from,omitempty"`
	Body   string `json:"body,omitempty"`
}

func (s *Server) accept(ctx context.Context) {
	for {
		c, err := s.ln.Accept()
		if err != nil {
			select {
			case <-ctx.Done():
				return
			default:
				return
			}
		}
		go s.handle(c)
	}
}

func (s *Server) handle(c net.Conn) {
	defer c.Close()
	dec := json.NewDecoder(c)
	enc := json.NewEncoder(c)
	for {
		var w wire
		if err := dec.Decode(&w); err != nil {
			return
		}
		switch w.Op {
		case "list":
			_ = enc.Encode(s.Snapshots())
		case "input":
			err := s.Write(w.Name, []byte(w.Data))
			_ = enc.Encode(map[string]any{"ok": err == nil, "err": errstr(err)})
		case "msg":
			err := s.RoutePublic(w.From, w.To, w.Body)
			_ = enc.Encode(map[string]any{"ok": err == nil, "err": errstr(err)})
		default:
			_ = enc.Encode(map[string]any{"ok": false, "err": "unknown op"})
		}
	}
}

func errstr(err error) string {
	if err == nil {
		return ""
	}
	return err.Error()
}

func (s *Server) RoutePublic(from, to, body string) error {
	s.mu.Lock()
	src := s.sess[from]
	dst := s.sess[to]
	s.mu.Unlock()
	if src == nil || dst == nil {
		return fmt.Errorf("unknown endpoint")
	}
	allowed := false
	for _, p := range src.Agent.Peers {
		if p == to {
			allowed = true
			break
		}
	}
	if !allowed {
		return fmt.Errorf("denied: %s cannot speak to %s (default deny)", from, to)
	}
	line := fmt.Sprintf("\r\n[fleeting %s→%s] %s\r\n", from, to, body)
	if err := s.Write(to, []byte(line)); err != nil {
		return err
	}
	return spool.Append(s.home, spool.Event{
		Kind: "msg",
		From: from,
		To:   to,
		Body: body,
		At:   time.Now(),
	})
}
