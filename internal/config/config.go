package config

import (
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"strings"

	"gopkg.in/yaml.v3"
)

type File struct {
	Operator string  `yaml:"operator"`
	Grid     int     `yaml:"grid"`
	Fleets   []Fleet `yaml:"fleets"`
}

type Fleet struct {
	ID     string  `yaml:"id"`
	Goal   string  `yaml:"goal"`
	Agents []Agent `yaml:"agents"`
}

type Agent struct {
	Name       string   `yaml:"name"`
	Role       string   `yaml:"role"`
	Lane       string   `yaml:"lane"`
	Harness    string   `yaml:"harness"`
	Cmd        []string `yaml:"cmd"`
	Hub        bool     `yaml:"hub"`
	Peers      []string `yaml:"peers"`
	ResumeGUID string   `yaml:"resume"`
}

func Dir() string {
	if d := os.Getenv("FLEETING_HOME"); d != "" {
		return d
	}
	home, err := os.UserHomeDir()
	if err != nil {
		return ".fleeting"
	}
	return filepath.Join(home, ".fleeting")
}

func Path() string {
	if p := os.Getenv("FLEETING_CONFIG"); p != "" {
		return p
	}
	return filepath.Join(Dir(), "fleet.yaml")
}

func SocketPath() string {
	return filepath.Join(Dir(), "fleeting.sock")
}

func PidPath() string {
	return filepath.Join(Dir(), "fleeting.pid")
}

func LogPath() string {
	return filepath.Join(Dir(), "daemon.log")
}

func Load(path string) (*File, error) {
	b, err := os.ReadFile(path)
	if err != nil {
		return nil, err
	}
	var f File
	if err := yaml.Unmarshal(b, &f); err != nil {
		return nil, err
	}
	if err := f.Validate(); err != nil {
		return nil, err
	}
	return &f, nil
}

func (f *File) Validate() error {
	if f.Grid < 3 {
		f.Grid = 3
	}
	if f.Grid > 6 {
		f.Grid = 6
	}
	if strings.TrimSpace(f.Operator) == "" {
		f.Operator = "operator"
	}
	names := map[string]bool{}
	for i := range f.Fleets {
		fl := &f.Fleets[i]
		if fl.ID == "" {
			return fmt.Errorf("fleet[%d] missing id", i)
		}
		for j := range fl.Agents {
			a := &fl.Agents[j]
			n := strings.ToLower(strings.TrimSpace(a.Name))
			if len(n) != 4 {
				return fmt.Errorf("agent %q must be a 4-letter name", a.Name)
			}
			if names[n] {
				return fmt.Errorf("duplicate agent name %s", n)
			}
			names[n] = true
			a.Name = n
			if a.Harness == "" {
				a.Harness = "omp"
			}
			if len(a.Cmd) == 0 {
				a.Cmd = DefaultCmd(a)
			}
		}
	}
	return nil
}

// DefaultCmd launches Oh My Pi with an isolated profile named after the persona.
// Do not pass --alias: that only writes a shell shortcut to bashrc and exits.
func DefaultCmd(a *Agent) []string {
	omp, err := exec.LookPath("omp")
	if err != nil {
		msg := "omp not found on PATH. Install Oh My Pi, then restart fleeting up:\\n  curl -fsSL https://omp.sh/install | sh\\n"
		return []string{"bash", "-lc", "printf '%b' " + shellQuote(msg) + "; echo persona=" + a.Name + "; sleep 3600"}
	}
	return []string{omp, "--profile", a.Name}
}

func shellQuote(s string) string {
	return "'" + strings.ReplaceAll(s, "'", `'\''`) + "'"
}

func Example() string {
	return `# Fleeting fleet file
# Empty cmd: launches omp --profile <name> (isolated OMP state per persona)
operator: richard
grid: 3

fleets:
  - id: local-core
    goal: land the first fleeting slice
    agents:
      - name: nova
        role: worker
        lane: ux
        harness: omp
        hub: false
        peers: [risa]
      - name: bolt
        role: worker
        lane: infra
        harness: omp
        peers: [risa]
      - name: kite
        role: worker
        lane: module
        harness: omp
        peers: [risa]
      - name: veil
        role: worker
        lane: security
        harness: omp
        peers: [hiro, risa]
      - name: hiro
        role: judge
        lane: review
        harness: omp
        hub: true
        peers: [risa, nova, bolt, kite, veil]
      - name: risa
        role: foreman
        lane: pace
        harness: omp
        hub: true
        peers: [hiro, nova, bolt, kite, veil]
`
}
