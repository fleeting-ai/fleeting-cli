package harness

import (
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
)

// Catalog is the v1 launch menu. Oh My Pi (`omp`) is not labeled Pi.
var Catalog = []Spec{
	{ID: "claude", Title: "Claude Code", Bins: []string{"claude"}},
	{ID: "codex", Title: "Codex", Bins: []string{"codex"}},
	{ID: "cursor", Title: "Cursor", Bins: []string{"agent", "cursor-agent", "cursor"}},
	{ID: "omp", Title: "Oh My Pi", Bins: []string{"omp"}},
	{ID: "pi", Title: "Pi", Bins: []string{"pi"}},
}

type Spec struct {
	ID    string
	Title string
	Bins  []string
}

type Installed struct {
	Spec
	Path string
	Bin  string
	Cmd  []string
}

var lookPath = exec.LookPath

func InstalledOnPATH() []Installed {
	var out []Installed
	for _, sp := range Catalog {
		if in, ok := resolve(sp); ok {
			out = append(out, in)
		}
	}
	return out
}

func Resolve(id string) (Installed, error) {
	id = strings.ToLower(strings.TrimSpace(id))
	for _, sp := range Catalog {
		if sp.ID == id {
			in, ok := resolve(sp)
			if !ok {
				return Installed{}, fmt.Errorf("%s not installed", sp.Title)
			}
			return in, nil
		}
	}
	return Installed{}, fmt.Errorf("unknown harness %s", id)
}

func resolve(sp Spec) (Installed, bool) {
	for _, bin := range sp.Bins {
		p, err := lookPath(bin)
		if err != nil {
			continue
		}
		if !canExec(p) {
			continue
		}
		return Installed{
			Spec: sp,
			Path: p,
			Bin:  bin,
			Cmd:  []string{p},
		}, true
	}
	return Installed{}, false
}

// CmdForAgent is the argv to spawn. OMP gets --profile <persona>, never --alias.
func (in Installed) CmdForAgent(name string) []string {
	if in.ID == "omp" || filepath.Base(in.Path) == "omp" {
		return []string{in.Path, "--profile", name}
	}
	return []string{in.Path}
}

// MissingCmd is a placeholder PTY when the harness binary is not on PATH.
func MissingCmd(id, name string) []string {
	msg := missingHelp(id)
	return []string{"bash", "-lc", "printf '%b' " + shellQuote(msg) + "; echo persona=" + name + "; sleep 3600"}
}

func missingHelp(id string) string {
	switch strings.ToLower(strings.TrimSpace(id)) {
	case "claude":
		return "claude not found on PATH. Install Claude Code, then restart fleeting:\\n  curl -fsSL https://claude.ai/install.sh | bash\\n"
	case "codex":
		return "codex not found on PATH. Install Codex, then restart fleeting:\\n  npm install -g @openai/codex\\n"
	case "cursor":
		return "Cursor agent CLI not found on PATH (tried agent, cursor-agent, cursor).\\n"
	case "pi":
		return "pi not found on PATH.\\n"
	default:
		return "omp not found on PATH. Install Oh My Pi, then restart fleeting up:\\n  curl -fsSL https://omp.sh/install | sh\\n"
	}
}

func shellQuote(s string) string {
	return "'" + strings.ReplaceAll(s, "'", `'\''`) + "'"
}

// IDFromBin maps a spawned argv0 basename to a catalog id.
func IDFromBin(base string) string {
	switch strings.ToLower(filepath.Base(base)) {
	case "claude":
		return "claude"
	case "codex":
		return "codex"
	case "agent", "cursor-agent", "cursor":
		return "cursor"
	case "omp":
		return "omp"
	case "pi":
		return "pi"
	default:
		return ""
	}
}

func sessionRoot(fleetingHome, name string) string {
	return filepath.Join(fleetingHome, "sessions", name)
}

func ClaudeConfigDir(fleetingHome, name string) string {
	return filepath.Join(sessionRoot(fleetingHome, name), "claude")
}

func CodexHome(fleetingHome, name string) string {
	return filepath.Join(sessionRoot(fleetingHome, name), "codex")
}

// PrepareSession creates per-persona dirs and returns extra env for the child.
// Claude uses CLAUDE_CONFIG_DIR; Codex uses CODEX_HOME.
func PrepareSession(fleetingHome, id, name string) []string {
	id = strings.ToLower(strings.TrimSpace(id))
	_ = os.MkdirAll(sessionRoot(fleetingHome, name), 0o755)
	switch id {
	case "claude":
		dir := ClaudeConfigDir(fleetingHome, name)
		_ = os.MkdirAll(dir, 0o755)
		return []string{"CLAUDE_CONFIG_DIR=" + dir}
	case "codex":
		dir := CodexHome(fleetingHome, name)
		_ = os.MkdirAll(dir, 0o755)
		return []string{"CODEX_HOME=" + dir}
	default:
		return nil
	}
}

// LockDirs are scanned for stale .lock/.pid/.sock files when a cell reaps.
func LockDirs(fleetingHome, name string) []string {
	dirs := []string{
		sessionRoot(fleetingHome, name),
		ClaudeConfigDir(fleetingHome, name),
		CodexHome(fleetingHome, name),
	}
	if home, err := os.UserHomeDir(); err == nil {
		dirs = append(dirs, filepath.Join(home, ".omp", "profiles", name))
	}
	return dirs
}

func canExec(p string) bool {
	st, err := os.Stat(p)
	if err != nil || st.IsDir() {
		return false
	}
	return st.Mode()&0o111 != 0
}
