package config

import (
	"fmt"
	"os"
	"path/filepath"
	"strings"

	"github.com/richard-ginsberg/fleeting/internal/harness"
	"gopkg.in/yaml.v3"
)

type File struct {
	Operator string  `yaml:"operator"`
	Grid     int     `yaml:"grid"`
	Bus      Bus     `yaml:"bus"`
	Teams    []Team  `yaml:"teams"`
	Fleets   []Fleet `yaml:"fleets"`
}

type Bus struct {
	URL string `yaml:"url"`
	DB  string `yaml:"db"`
}

type Team struct {
	ID      string   `yaml:"id"`
	Members []string `yaml:"members"`
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
	Rank       int      `yaml:"rank"`
	ResumeGUID string   `yaml:"resume"`
}

// RankOf is yaml rank if set, else the role default (foreman 50, judge 40, else 10).
func RankOf(role string, rank int) int {
	if rank > 0 {
		return rank
	}
	switch strings.ToLower(strings.TrimSpace(role)) {
	case "foreman":
		return 50
	case "judge":
		return 40
	default:
		return 10
	}
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
			a.Rank = RankOf(a.Role, a.Rank)
		}
	}
	return f.validateTeams(names)
}

func (f *File) validateTeams(names map[string]bool) error {
	seen := map[string]bool{}
	for i := range f.Teams {
		t := &f.Teams[i]
		id := strings.ToLower(strings.TrimSpace(t.ID))
		if !validTeamID(id) {
			return fmt.Errorf("team[%d] id %q must be lowercase letters, digits, hyphen", i, t.ID)
		}
		if seen[id] {
			return fmt.Errorf("duplicate team id %s", id)
		}
		seen[id] = true
		t.ID = id
		for j, m := range t.Members {
			n := strings.ToLower(strings.TrimSpace(m))
			if len(n) != 4 {
				return fmt.Errorf("team %s member %q must be a 4-letter name", id, m)
			}
			if !names[n] {
				return fmt.Errorf("team %s unknown member %s", id, n)
			}
			t.Members[j] = n
		}
	}
	return nil
}

func validTeamID(id string) bool {
	if id == "" {
		return false
	}
	for i, r := range id {
		if r >= 'a' && r <= 'z' || r >= '0' && r <= '9' {
			continue
		}
		if r == '-' && i > 0 {
			continue
		}
		return false
	}
	return true
}

func (f *File) Agent(name string) *Agent {
	name = strings.ToLower(strings.TrimSpace(name))
	for i := range f.Fleets {
		for j := range f.Fleets[i].Agents {
			if f.Fleets[i].Agents[j].Name == name {
				return &f.Fleets[i].Agents[j]
			}
		}
	}
	return nil
}

func (f *File) TeamsOf(name string) []string {
	name = strings.ToLower(strings.TrimSpace(name))
	var out []string
	for _, t := range f.Teams {
		for _, m := range t.Members {
			if m == name {
				out = append(out, t.ID)
				break
			}
		}
	}
	return out
}

func (f *File) Team(id string) *Team {
	id = strings.ToLower(strings.TrimSpace(id))
	for i := range f.Teams {
		if f.Teams[i].ID == id {
			return &f.Teams[i]
		}
	}
	return nil
}

func (f *File) InTeam(name, team string) bool {
	t := f.Team(team)
	if t == nil {
		return false
	}
	name = strings.ToLower(strings.TrimSpace(name))
	for _, m := range t.Members {
		if m == name {
			return true
		}
	}
	return false
}

// DefaultCmd launches the agent's harness when cmd: is omitted.
// Oh My Pi uses --profile <persona> (never --alias). Claude Code and Codex
// get a bare argv; isolation is CLAUDE_CONFIG_DIR / CODEX_HOME at spawn.
func DefaultCmd(a *Agent) []string {
	id := strings.ToLower(strings.TrimSpace(a.Harness))
	if id == "" {
		id = "omp"
	}
	in, err := harness.Resolve(id)
	if err != nil {
		return harness.MissingCmd(id, a.Name)
	}
	return in.CmdForAgent(a.Name)
}

func Example() string {
	return `# Fleeting fleet file
# Empty cmd: launches the harness binary (omp --profile <name>; claude; codex)
operator: richard
grid: 3

bus:
  url: amqp://guest:guest@127.0.0.1:5672/
  db: sqlite://~/.fleeting/bus.db

teams:
  - id: core
    members: [risa, hiro, nova, bolt, kite, veil]
  - id: review
    members: [hiro, veil]

fleets:
  - id: local-core
    goal: land the first fleeting slice
    agents:
      - name: nova
        role: worker
        lane: ux
        harness: omp
        hub: false
        rank: 10
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
        rank: 50
        peers: [hiro, nova, bolt, kite, veil]
`
}
