package config

import (
	"fmt"
	"os"
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
	Name        string   `yaml:"name"`
	Role        string   `yaml:"role"`
	Lane        string   `yaml:"lane"`
	Harness     string   `yaml:"harness"`
	Cmd         []string `yaml:"cmd"`
	Hub         bool     `yaml:"hub"`
	Peers       []string `yaml:"peers"`
	ResumeGUID  string   `yaml:"resume"`
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
			if len(a.Cmd) == 0 {
				a.Cmd = DefaultCmd(a)
			}
		}
	}
	return nil
}

func DefaultCmd(a *Agent) []string {
	label := a.Name
	if a.Role != "" {
		label = a.Name + "/" + a.Role
	}
	body := fmt.Sprintf(
		`printf '\033[1m%s\033[0m lane=%s harness=%s\n'; date '+%%H:%%M:%%S'; echo 'stub harness — set cmd: in fleet.yaml'; sleep 2`,
		label, a.Lane, a.Harness,
	)
	return []string{"bash", "-lc", "while true; do " + body + "; done"}
}

func Example() string {
	return `# Fleeting fleet file (Dockerfile-style: declare the image of the team)
operator: richard
grid: 3  # 3..6 Brady Bunch cells on a side

fleets:
  - id: local-core
    goal: land the first fleeting slice
    agents:
      - name: nova
        role: worker
        lane: ux
        harness: pi
        hub: false
        peers: [risa]          # default deny; only these names can be addressed
      - name: bolt
        role: worker
        lane: infra
        harness: claude
        peers: [risa]
      - name: kite
        role: worker
        lane: module
        harness: codex
        peers: [risa]
      - name: veil
        role: worker
        lane: security
        harness: antigravity
        peers: [hiro, risa]
      - name: hiro
        role: judge
        lane: review
        harness: fleeting
        hub: true
        peers: [risa, nova, bolt, kite, veil]
      - name: risa
        role: foreman
        lane: pace
        harness: fleeting
        hub: true
        peers: [hiro, nova, bolt, kite, veil]
`
}
