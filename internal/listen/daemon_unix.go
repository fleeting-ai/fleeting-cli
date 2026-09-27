package listen

import (
	"context"
	"fmt"
	"os"
	"os/exec"
	"syscall"
	"time"

	"github.com/richard-ginsberg/fleeting/internal/config"
)

func EnsureDaemon(cfg *config.File, home string) error {
	if Ping() == nil {
		return nil
	}
	exe, err := os.Executable()
	if err != nil {
		return err
	}
	if err := os.MkdirAll(home, 0o755); err != nil {
		return err
	}
	logf, err := os.OpenFile(config.LogPath(), os.O_CREATE|os.O_WRONLY|os.O_APPEND, 0o644)
	if err != nil {
		return err
	}
	defer logf.Close()
	cmd := exec.Command(exe, "daemon")
	cmd.Env = os.Environ()
	cmd.Stdin = nil
	cmd.Stdout = logf
	cmd.Stderr = logf
	cmd.SysProcAttr = &syscall.SysProcAttr{Setsid: true}
	if err := cmd.Start(); err != nil {
		return fmt.Errorf("start daemon: %w", err)
	}
	deadline := time.Now().Add(5 * time.Second)
	for time.Now().Before(deadline) {
		if Ping() == nil {
			return nil
		}
		time.Sleep(50 * time.Millisecond)
	}
	return fmt.Errorf("daemon did not listen on %s (see %s)", config.SocketPath(), config.LogPath())
}

func RunDaemon(ctx context.Context, cfg *config.File, home string) error {
	s := New(cfg, home)
	if err := s.Start(ctx); err != nil {
		return err
	}
	select {
	case <-ctx.Done():
	case <-s.Stopped():
	}
	return nil
}
