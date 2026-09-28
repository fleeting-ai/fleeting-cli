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
	"strings"
	"sync"
	"syscall"
	"time"

	"github.com/creack/pty"
	"github.com/hinshun/vt10x"
	"github.com/richard-ginsberg/fleeting/internal/config"
	"github.com/richard-ginsberg/fleeting/internal/harness"
	"github.com/richard-ginsberg/fleeting/internal/presence"
	"github.com/richard-ginsberg/fleeting/internal/spool"
)

// Server is the local Fleeting listener. Remote hosts will speak the same
// length-prefixed JSON protocol over TCP later; v1 is a Unix socket only.
type Server struct {
	cfg      *config.File
	path     string
	mu       sync.Mutex
	sess     map[string]*Session
	ln       net.Listener
	home     string
	attMu    sync.Mutex
	nAttach  int
	attacher *jsonConn
	replayMu sync.Mutex
	replay   []ReplayFrame
	stop     chan struct{}
}

type jsonConn struct {
	net.Conn
	mu  sync.Mutex
	enc *json.Encoder
	dec *json.Decoder
}

func (j *jsonConn) send(p Packet) error {
	j.mu.Lock()
	defer j.mu.Unlock()
	return j.enc.Encode(p)
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
	Name    string    `json:"name"`
	Fleet   string    `json:"fleet"`
	GUID    string    `json:"guid"`
	Role    string    `json:"role"`
	Lane    string    `json:"lane"`
	Harness string    `json:"harness"`
	Cmd     []string  `json:"cmd,omitempty"`
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
		stop: make(chan struct{}),
	}
}

func (s *Server) Start(ctx context.Context) error {
	if err := os.MkdirAll(filepath.Dir(s.path), 0o755); err != nil {
		return err
	}
	if err := probeLive(s.path); err == nil {
		return fmt.Errorf("daemon already running on %s", s.path)
	}
	_ = os.Remove(s.path)
	ln, err := net.Listen("unix", s.path)
	if err != nil {
		return err
	}
	s.ln = ln
	_ = os.WriteFile(config.PidPath(), []byte(fmt.Sprintf("%d\n", os.Getpid())), 0o644)

	for _, fl := range s.cfg.Fleets {
		for _, a := range fl.Agents {
			if err := s.spawn(fl.ID, a); err != nil {
				return err
			}
		}
	}

	go s.accept(ctx)
	go s.replayLoop(ctx)
	go func() {
		select {
		case <-ctx.Done():
		case <-s.stop:
		}
		_ = ln.Close()
		s.KillAll()
		_ = os.Remove(s.path)
		_ = os.Remove(config.PidPath())
	}()
	return nil
}

func probeLive(path string) error {
	c, err := net.DialTimeout("unix", path, 200*time.Millisecond)
	if err != nil {
		return err
	}
	_ = c.Close()
	return nil
}

func (s *Server) spawn(fleetID string, a config.Agent) error {
	guid := a.ResumeGUID
	if guid == "" {
		guid = fmt.Sprintf("%s-%d", a.Name, time.Now().UnixNano())
	}
	if a.Harness == "" && len(a.Cmd) > 0 {
		a.Harness = harness.IDFromBin(a.Cmd[0])
	}
	if len(a.Cmd) == 0 || harness.LooksLikeMissingCmd(a.Cmd) {
		a.Cmd = config.DefaultCmd(&a)
	}
	cmd := exec.Command(a.Cmd[0], a.Cmd[1:]...)
	env := []string{
		"FLEETING_NAME=" + a.Name,
		"FLEETING_LANE=" + a.Lane,
		"FLEETING_ROLE=" + a.Role,
		"FLEETING_FLEET=" + fleetID,
		"FLEETING_GUID=" + guid,
		"OMP_PROFILE=" + a.Name,
		"TERM=xterm-256color",
		"COLORTERM=truecolor",
	}
	env = append(env, harness.PrepareSession(config.Dir(), a.Harness, a.Name)...)
	env = append(env, "PATH="+harness.PATHEnv())
	cmd.Env = append(os.Environ(), env...)
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
	go se.waitLoop(s)
	return nil
}

// Spawn starts a session that is not in the fleet file (blank-cell launch).
func (s *Server) Spawn(fleetID string, a config.Agent) error {
	if a.Name == "" || len(a.Cmd) == 0 {
		return fmt.Errorf("spawn needs name and cmd")
	}
	s.mu.Lock()
	if se := s.sess[a.Name]; se != nil {
		if processLive(se) {
			s.mu.Unlock()
			return fmt.Errorf("session %s already running", a.Name)
		}
		delete(s.sess, a.Name)
		s.mu.Unlock()
		dropSession(se)
		clearStaleLocks(a.Name)
	} else {
		s.mu.Unlock()
	}
	return s.spawn(fleetID, a)
}

// Restart kills a live session and starts the same agent again (Pi model reload).
func (s *Server) Restart(name string) error {
	s.mu.Lock()
	se := s.sess[name]
	if se == nil {
		s.mu.Unlock()
		return fmt.Errorf("unknown agent %s", name)
	}
	a := se.Agent
	fleet := se.FleetID
	if se.Cmd != nil && se.Cmd.Process != nil {
		_ = se.Cmd.Process.Kill()
	}
	s.mu.Unlock()
	deadline := time.Now().Add(3 * time.Second)
	for time.Now().Before(deadline) {
		s.mu.Lock()
		_, ok := s.sess[name]
		s.mu.Unlock()
		if !ok {
			return s.Spawn(fleet, a)
		}
		time.Sleep(30 * time.Millisecond)
	}
	return fmt.Errorf("timeout restarting %s", name)
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

func (se *Session) waitLoop(s *Server) {
	err := se.Cmd.Wait()
	se.mu.Lock()
	se.alive = false
	if err != nil {
		se.err = err.Error()
	}
	name := se.Agent.Name
	se.mu.Unlock()
	s.Reap(name)
}

// processLive is true only while the child PID still exists.
// A dead PTY, Wait() result, or vanished PID must not hold the persona name.
func processLive(se *Session) bool {
	if se == nil {
		return false
	}
	se.mu.Lock()
	defer se.mu.Unlock()
	if !se.alive {
		return false
	}
	cmd := se.Cmd
	if cmd == nil || cmd.Process == nil {
		return false
	}
	if cmd.ProcessState != nil {
		return false
	}
	if err := cmd.Process.Signal(syscall.Signal(0)); err != nil {
		return false
	}
	return true
}

func dropSession(se *Session) {
	if se == nil {
		return
	}
	se.mu.Lock()
	se.alive = false
	ptmx := se.Pty
	se.Pty = nil
	se.mu.Unlock()
	if ptmx != nil {
		_ = ptmx.Close()
	}
}

func clearStaleLocks(name string) {
	if name == "" {
		return
	}
	dirs := harness.LockDirs(config.Dir(), name)
	for _, dir := range dirs {
		ents, err := os.ReadDir(dir)
		if err != nil {
			continue
		}
		for _, e := range ents {
			n := e.Name()
			if strings.HasSuffix(n, ".lock") || strings.HasSuffix(n, ".pid") || strings.HasSuffix(n, ".sock") {
				_ = os.Remove(filepath.Join(dir, n))
			}
		}
	}
}

// Reap drops an exited session so the grid cell goes blank (no stale VT)
// and the 4-letter persona name can be launched again.
func (s *Server) Reap(name string) {
	s.mu.Lock()
	se := s.sess[name]
	if se == nil {
		s.mu.Unlock()
		return
	}
	if processLive(se) {
		s.mu.Unlock()
		return
	}
	delete(s.sess, name)
	s.mu.Unlock()
	dropSession(se)
	clearStaleLocks(name)
}

func (s *Server) harvestDead() {
	s.mu.Lock()
	var dead []string
	for name, se := range s.sess {
		if !processLive(se) {
			dead = append(dead, name)
		}
	}
	s.mu.Unlock()
	for _, name := range dead {
		s.Reap(name)
	}
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
	s.harvestDead()
	s.mu.Lock()
	defer s.mu.Unlock()
	out := make([]Snapshot, 0, len(s.sess))
	for _, se := range s.sess {
		se.mu.Lock()
		if !se.alive {
			se.mu.Unlock()
			continue
		}
		screen := ""
		if se.VT != nil {
			screen = dumpVT(se.VT)
		}
		st := presence.Of(se.alive, se.last, screen)
		out = append(out, Snapshot{
			Name:    se.Agent.Name,
			Fleet:   se.FleetID,
			GUID:    se.GUID,
			Role:    se.Agent.Role,
			Lane:    se.Agent.Lane,
			Harness: se.Agent.Harness,
			Cmd:     append([]string{}, se.Agent.Cmd...),
			Hub:     se.Agent.Hub,
			Peers:   se.Agent.Peers,
			Alive:   se.alive,
			Err:     se.err,
			Last:    se.last,
			Status:  st,
			Screen:  screen,
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
	jc := &jsonConn{Conn: c, enc: json.NewEncoder(c), dec: json.NewDecoder(c)}
	defer func() {
		s.dropAttacher(jc)
		_ = c.Close()
	}()
	for {
		var w Packet
		if err := jc.dec.Decode(&w); err != nil {
			return
		}
		if !s.dispatch(jc, w) {
			return
		}
	}
}

func (s *Server) dispatch(jc *jsonConn, w Packet) bool {
	switch w.Op {
	case "list":
		_ = jc.send(Packet{OK: true, Op: "list", Snaps: s.Snapshots()})
	case "status":
		st := s.Status()
		_ = jc.send(Packet{OK: true, Op: "status", Status: &st, Snaps: s.Snapshots()})
	case "input":
		err := s.Write(w.Name, []byte(w.Data))
		_ = jc.send(Packet{OK: err == nil, Err: errstr(err)})
	case "msg":
		err := s.RoutePublic(w.From, w.To, w.Body)
		_ = jc.send(Packet{OK: err == nil, Err: errstr(err)})
	case "spawn":
		if w.Agent == nil {
			_ = jc.send(Packet{OK: false, Err: "spawn needs agent"})
			break
		}
		err := s.Spawn(w.Fleet, *w.Agent)
		_ = jc.send(Packet{OK: err == nil, Err: errstr(err)})
	case "restart":
		err := s.Restart(w.Name)
		_ = jc.send(Packet{OK: err == nil, Err: errstr(err)})
	case "resize":
		if w.Name != "" {
			s.ResizeSession(w.Name, w.Cols, w.Rows)
		} else {
			s.Resize(w.Cols, w.Rows)
		}
		_ = jc.send(Packet{OK: true})
	case "attach":
		if err := s.takeAttach(jc, w.Steal); err != nil {
			_ = jc.send(Packet{OK: false, Err: err.Error()})
			return true
		}
		st := s.Status()
		_ = jc.send(Packet{OK: true, Op: "attach", Replay: s.takeReplay(), Snaps: s.Snapshots(), Status: &st})
	case "detach":
		s.kickAttacher()
		_ = jc.send(Packet{OK: true, Op: "detach"})
	case "down":
		_ = jc.send(Packet{OK: true, Op: "down"})
		s.requestStop()
		return false
	default:
		_ = jc.send(Packet{OK: false, Err: "unknown op"})
	}
	return true
}

func (s *Server) takeAttach(jc *jsonConn, steal bool) error {
	s.attMu.Lock()
	if s.attacher != nil && s.attacher != jc {
		if !steal {
			s.attMu.Unlock()
			return fmt.Errorf("already attached — fleeting -d -r to steal")
		}
		att := s.attacher
		s.attacher = nil
		s.nAttach = 0
		s.attMu.Unlock()
		_ = att.send(Packet{Op: "kick", OK: true})
		_ = att.Close()
		s.attMu.Lock()
	}
	s.attacher = jc
	s.nAttach = 1
	s.attMu.Unlock()
	return nil
}

func (s *Server) dropAttacher(jc *jsonConn) {
	s.attMu.Lock()
	defer s.attMu.Unlock()
	if s.attacher == jc {
		s.attacher = nil
		s.nAttach = 0
	}
}

func (s *Server) kickAttacher() {
	s.attMu.Lock()
	att := s.attacher
	s.attacher = nil
	s.nAttach = 0
	s.attMu.Unlock()
	if att != nil {
		_ = att.send(Packet{Op: "kick", OK: true})
		_ = att.Close()
	}
}

func (s *Server) Attached() bool {
	s.attMu.Lock()
	defer s.attMu.Unlock()
	return s.nAttach > 0
}

func (s *Server) Status() Status {
	snaps := s.Snapshots()
	names := make([]string, 0, len(snaps))
	for _, sn := range snaps {
		names = append(names, sn.Name)
	}
	return Status{
		PID:      os.Getpid(),
		Socket:   s.path,
		Attached: s.Attached(),
		Agents:   len(snaps),
		Names:    names,
	}
}

func (s *Server) requestStop() {
	select {
	case <-s.stop:
	default:
		close(s.stop)
	}
}

func (s *Server) Stopped() <-chan struct{} {
	return s.stop
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
