package main

import (
	"context"
	"crypto/md5"
	"encoding/hex"
	"fmt"
	"os"
	"os/signal"
	"path/filepath"
	"strings"
	"syscall"
	"time"

	tea "github.com/charmbracelet/bubbletea"
	"github.com/richard-ginsberg/fleeting/internal/config"
	"github.com/richard-ginsberg/fleeting/internal/listen"
	"github.com/richard-ginsberg/fleeting/internal/spool"
	"github.com/richard-ginsberg/fleeting/internal/tui"
)

func main() {
	if len(os.Args) < 2 {
		usage()
		os.Exit(2)
	}
	switch os.Args[1] {
	case "onboard":
		if err := onboard(); err != nil {
			fatal(err)
		}
	case "up":
		if err := up(); err != nil {
			fatal(err)
		}
	case "r":
		if err := resume(os.Args[2:]); err != nil {
			fatal(err)
		}
	case "declare":
		if err := declare(os.Args[2:]); err != nil {
			fatal(err)
		}
	case "help", "-h", "--help":
		usage()
	default:
		usage()
		os.Exit(2)
	}
}

func usage() {
	fmt.Fprintf(os.Stderr, `fleeting — one pane of glass for many agent TUIs

  fleeting onboard          write ~/.fleeting/fleet.yaml and create home
  fleeting up               start local listener + Brady Bunch grid
  fleeting r <guid> --go    resume TUI against a running listener
  fleeting declare --agent NAME --intent TEXT FILE [FILE...]

Config: $FLEETING_CONFIG or ~/.fleeting/fleet.yaml
Listener socket: ~/.fleeting/fleeting.sock (TCP remote later)
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

func up() error {
	cfg, home, err := load()
	if err != nil {
		return err
	}
	ctx, cancel := signal.NotifyContext(context.Background(), os.Interrupt, syscall.SIGTERM)
	defer cancel()
	srv := listen.New(cfg, home)
	if err := srv.Start(ctx); err != nil {
		return err
	}
	fmt.Fprintf(os.Stderr, "listener %s  grid %dx%d\n", config.SocketPath(), cfg.Grid, cfg.Grid)
	m := tui.New(srv, cfg, home)
	p := tea.NewProgram(m, tea.WithAltScreen())
	_, err = p.Run()
	cancel()
	srv.KillAll()
	return err
}

func resume(args []string) error {
	goFlag := false
	var guid string
	for _, a := range args {
		if a == "--go" {
			goFlag = true
			continue
		}
		if !strings.HasPrefix(a, "-") {
			guid = a
		}
	}
	if guid == "" {
		return fmt.Errorf("usage: fleeting r <guid> --go")
	}
	if !goFlag {
		return fmt.Errorf("pass --go to attach")
	}
	cfg, home, err := load()
	if err != nil {
		return err
	}
	// Resume means: bring the same fleet up and focus the matching guid.
	// If the listener is already up this still starts a new local supervisor
	// for v1 (single process). GUID is stored on each session snapshot.
	ctx, cancel := signal.NotifyContext(context.Background(), os.Interrupt, syscall.SIGTERM)
	defer cancel()
	for i := range cfg.Fleets {
		for j := range cfg.Fleets[i].Agents {
			if cfg.Fleets[i].Agents[j].ResumeGUID == "" {
				cfg.Fleets[i].Agents[j].ResumeGUID = guid
			}
		}
	}
	_ = guid
	srv := listen.New(cfg, home)
	if err := srv.Start(ctx); err != nil {
		return err
	}
	m := tui.New(srv, cfg, home)
	p := tea.NewProgram(m, tea.WithAltScreen())
	_, err = p.Run()
	cancel()
	srv.KillAll()
	return err
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
