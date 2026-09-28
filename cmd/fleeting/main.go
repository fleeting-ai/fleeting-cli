package main

import (
	"context"
	"crypto/md5"
	"encoding/hex"
	"fmt"
	"os"
	"os/signal"
	"path/filepath"
	"syscall"
	"time"

	tea "github.com/charmbracelet/bubbletea"
	"github.com/richard-ginsberg/fleeting/internal/cli"
	"github.com/richard-ginsberg/fleeting/internal/config"
	"github.com/richard-ginsberg/fleeting/internal/listen"
	"github.com/richard-ginsberg/fleeting/internal/spool"
	"github.com/richard-ginsberg/fleeting/internal/tui"
)

func main() {
	act, err := cli.Parse(os.Args[1:])
	if err != nil {
		fmt.Fprintln(os.Stderr, err)
		usage()
		os.Exit(2)
	}
	switch act.Kind {
	case "help":
		usage()
	case "onboard":
		if err := onboard(); err != nil {
			fatal(err)
		}
	case "declare":
		if err := declare(act.Declare); err != nil {
			fatal(err)
		}
	case "send":
		runSend(act.Args)
	case "listen":
		runListen(act.Args)
	case "log":
		runLog(act.Args)
	case "card":
		runCard(act.Args)
	case "rules":
		runRules(act.Args)
	case "memory":
		runMemory(act.Args)
	case "bus":
		runBus(act.Args)
	case "daemon":
		if err := runDaemon(); err != nil {
			fatal(err)
		}
	case "down":
		if err := down(); err != nil {
			fatal(err)
		}
	case "list":
		if err := listSessions(); err != nil {
			fatal(err)
		}
	case "detach":
		if err := detachOthers(); err != nil {
			fatal(err)
		}
	case "attach", "up":
		if err := up(act); err != nil {
			fatal(err)
		}
	default:
		usage()
		os.Exit(2)
	}
}

func usage() {
	fmt.Fprintf(os.Stderr, `fleeting — one pane of glass for many agent TUIs

  fleeting                 start the daemon if needed, then attach
  fleeting up              same as above
  fleeting daemon          run the listener in the foreground (systemd / no TTY)
  fleeting -r | attach     reattach (fails if another TUI is attached)
  fleeting -d -r           detach the other TUI, then attach
  fleeting -d              detach attached clients; agents keep running
  fleeting -ls             list the daemon and live agents
  fleeting down            stop the daemon and all agent PTYs
  fleeting onboard         write ~/.fleeting/fleet.yaml
  fleeting declare --agent NAME --intent TEXT FILE [FILE...]
  fleeting send --from NAME (--to NAME|--team ID) [--action fyi|todo|order|ack] -- BODY
  fleeting listen --as NAME [--once]
  fleeting log --as NAME [--team ID] [--all] [--last N]
  fleeting card --as NAME
  fleeting rules | fleeting rules set --from NAME -- BODY | fleeting rules log
  fleeting memory list|get|set|log ...
  fleeting bus

Detach key (inside the TUI): Ctrl-A then d
  Ctrl-A Ctrl-A sends a literal Ctrl-A to the focused PTY.
  Ctrl-C always goes to the focused agent, never detaches.

Missed output on reattach replays at 4× (keys 2/4/8/g=16×, L jumps to live).

Config: $FLEETING_CONFIG or ~/.fleeting/fleet.yaml
Socket: ~/.fleeting/fleeting.sock
PID:    ~/.fleeting/fleeting.pid
Log:    ~/.fleeting/daemon.log
`)
}

func fatal(err error) {
	fmt.Fprintln(os.Stderr, err)
	os.Exit(1)
}

func onboard() error {
	home := config.Dir()
	if err := os.MkdirAll(home, 0o755); err != nil {
		return err
	}
	p := config.Path()
	if _, err := os.Stat(p); err == nil {
		fmt.Println("already exists:", p)
		return nil
	}
	if err := os.WriteFile(p, []byte(config.Example()), 0o644); err != nil {
		return err
	}
	fmt.Println("wrote", p)
	fmt.Println("edit cmd: per agent to attach real harnesses, then: fleeting up")
	return nil
}

func load() (*config.File, string, error) {
	home := config.Dir()
	p := config.Path()
	cfg, err := config.Load(p)
	if err != nil {
		return nil, "", fmt.Errorf("load %s: %w (run fleeting onboard)", p, err)
	}
	return cfg, home, nil
}

func runDaemon() error {
	cfg, home, err := load()
	if err != nil {
		return err
	}
	ctx, cancel := signal.NotifyContext(context.Background(), os.Interrupt, syscall.SIGTERM, syscall.SIGHUP)
	defer cancel()
	fmt.Fprintf(os.Stderr, "fleeting daemon %s pid %d\n", config.SocketPath(), os.Getpid())
	return listen.RunDaemon(ctx, cfg, home)
}

func up(act cli.Action) error {
	cfg, home, err := load()
	if err != nil {
		return err
	}
	if err := listen.EnsureDaemon(cfg, home); err != nil {
		return err
	}
	cl, err := listen.Dial()
	if err != nil {
		return err
	}
	defer cl.Close()
	got, err := cl.Attach(act.Steal)
	if err != nil {
		return err
	}
	m := tui.New(cl, cfg, home)
	m.WithReplay(got.Replay)
	m.WithKick(cl.Kicked())
	fmt.Fprintf(os.Stderr, "attached %s  %d agents  replay %d frames\n", config.SocketPath(), len(got.Snaps), len(got.Replay))
	p := tea.NewProgram(m, tea.WithAltScreen())
	_, err = p.Run()
	return err
}

func listSessions() error {
	cl, err := listen.Dial()
	if err != nil {
		return fmt.Errorf("no daemon (%s): %w", config.SocketPath(), err)
	}
	defer cl.Close()
	st, err := cl.QueryStatus()
	if err != nil {
		return err
	}
	att := "detached"
	if st.Attached {
		att = "attached"
	}
	fmt.Printf("fleeting  pid=%d  %s  %s  %d agents  bus %s  store %s\n", st.PID, att, st.Socket, st.Agents, nz(st.Bus, "down"), nz(st.Store, "down"))
	for _, n := range st.Names {
		fmt.Printf("  %s\n", n)
	}
	if st.Agents == 0 {
		fmt.Println("  (no live PTYs)")
	}
	return nil
}

func detachOthers() error {
	cl, err := listen.Dial()
	if err != nil {
		return fmt.Errorf("no daemon: %w", err)
	}
	defer cl.Close()
	if err := cl.DetachOthers(); err != nil {
		return err
	}
	fmt.Println("detached")
	return nil
}

func down() error {
	cl, err := listen.Dial()
	if err != nil {
		return fmt.Errorf("no daemon: %w", err)
	}
	defer cl.Close()
	_ = cl.Down()
	fmt.Println("daemon stopping")
	return nil
}

func declare(args []string) error {
	home := config.Dir()
	agent := "hiro"
	intent := ""
	var files []string
	for i := 0; i < len(args); i++ {
		switch args[i] {
		case "--agent":
			i++
			if i < len(args) {
				agent = args[i]
			}
		case "--intent":
			i++
			if i < len(args) {
				intent = args[i]
			}
		default:
			files = append(files, args[i])
		}
	}
	if intent == "" || len(files) == 0 {
		return fmt.Errorf("usage: fleeting declare --agent NAME --intent TEXT FILE [FILE...]")
	}
	var hashed []spool.File
	for _, p := range files {
		b, err := os.ReadFile(p)
		if err != nil {
			return err
		}
		sum := md5.Sum(b)
		hashed = append(hashed, spool.File{Path: p, MD5: hex.EncodeToString(sum[:])})
	}
	d := spool.Declaration{
		Agent:  agent,
		Intent: intent,
		Files:  hashed,
		At:     time.Now(),
	}
	if err := spool.Declare(home, d); err != nil {
		return err
	}
	fmt.Printf("hiro queue: %s declared %d files (%s)\n", agent, len(hashed), filepath.Base(files[0]))
	return nil
}
