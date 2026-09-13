package listen

import (
	"bufio"
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
	"github.com/hinshun/vt10x"
	"github.com/richard-ginsberg/fleeting/internal/config"
	"github.com/richard-ginsberg/fleeting/internal/presence"
	"github.com/richard-ginsberg/fleeting/internal/spool"
)

// Server is the local Fleeting listener. Remote hosts will speak the same
// length-prefixed JSON protocol over TCP later; v1 is a Unix socket only.
type Server struct {
	cfg  *config.File
	path string
	mu   sync.Mutex
	sess map[string]*Session
	ln   net.Listener
	home string
}

type Session struct {
	Agent   config.Agent
	FleetID string
	GUID    string
	Cmd     *exec.Cmd
	Pty     *os.File
	VT      vt10x.Terminal
	mu      sync.Mutex
	last    time.Time
	alive   bool
	err     string
	cols    uint16
	rows    uint16
}

type Snapshot struct {
	Name   string    `json:"name"`
	Fleet  string    `json:"fleet"`
	GUID   string    `json:"guid"`
	Role   string    `json:"role"`
	Lane   string    `json:"lane"`
	Hub    bool      `json:"hub"`
	Peers  []string  `json:"peers"`
	Alive  bool      `json:"alive"`
	Err    string    `json:"err,omitempty"`
	Last   time.Time `json:"last"`
	Status string    `json:"status"`
	Screen string    `json:"screen"`
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
		"OMP_PROFILE="+a.Name,
		"TERM=xterm-256color",
		"COLORTERM=truecolor",
	)
	_ = os.MkdirAll(filepath.Join(config.Dir(), "sessions", a.Name), 0o755)
	ptmx, err := pty.Start(cmd)
	if err != nil {
		return fmt.Errorf("%s: %w", a.Name, err)
	}
	const cols, rows = 80, 24
	_ = pty.Setsize(ptmx, &pty.Winsize{Rows: rows, Cols: cols})
	vt := vt10x.New(vt10x.WithSize(cols, rows), vt10x.WithWriter(ptmx))
	se := &Session{
		Agent:   a,
		FleetID: fleetID,
		GUID:    guid,
		Cmd:     cmd,
		Pty:     ptmx,
		VT:      vt,
		alive:   true,
		last:    time.Now(),
		cols:    cols,
		rows:    rows,
	}
	s.mu.Lock()
	s.sess[a.Name] = se
	s.mu.Unlock()
	go se.readLoop()
	go se.waitLoop()
	return nil
}

// Spawn starts a session that is not in the fleet file (blank-cell launch).
func (s *Server) Spawn(fleetID string, a config.Agent) error {
	if a.Name == "" || len(a.Cmd) == 0 {
		return fmt.Errorf("spawn needs name and cmd")
	}
	s.mu.Lock()
	_, exists := s.sess[a.Name]
	s.mu.Unlock()
	if exists {
		return fmt.Errorf("session %s already running", a.Name)
	}
	return s.spawn(fleetID, a)
}

func (se *Session) readLoop() {
	br := bufio.NewReader(se.Pty)
	for {
		if err := se.VT.Parse(br); err != nil {
			return
		}
		se.mu.Lock()
		se.last = time.Now()
		se.mu.Unlock()
	}
}

func (s *Server) Resize(cols, rows int) {
	s.mu.Lock()
	defer s.mu.Unlock()
	for _, se := range s.sess {
		s.resizeLocked(se, cols, rows)
	}
}

func (s *Server) ResizeSession(name string, cols, rows int) {
	s.mu.Lock()
	defer s.mu.Unlock()
	if se := s.sess[name]; se != nil {
		s.resizeLocked(se, cols, rows)
	}
}

func (s *Server) resizeLocked(se *Session, cols, rows int) {
	if se == nil || se.Pty == nil || se.VT == nil {
		return
	}
	if cols < 8 {
		cols = 8
	}
	if rows < 4 {
		rows = 4
	}
	c, r := uint16(cols), uint16(rows)
	if se.cols == c && se.rows == r {
		return
	}
	se.cols, se.rows = c, r
	se.VT.Resize(cols, rows)
	_ = pty.Setsize(se.Pty, &pty.Winsize{Rows: r, Cols: c})
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
		screen := ""
		if se.VT != nil {
			screen = dumpVT(se.VT)
		}
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
	Op   string `json:"op"`
	Name string `json:"name,omitempty"`
	Data string `json:"data,omitempty"`
	To   string `json:"to,omitempty"`
	From string `json:"from,omitempty"`
	Body string `json:"body,omitempty"`
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
