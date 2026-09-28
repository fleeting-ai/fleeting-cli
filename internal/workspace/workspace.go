package workspace

import (
	"bytes"
	"fmt"
	"os"
	"path/filepath"
	"sort"

	"gopkg.in/yaml.v3"

	"github.com/fleeting-ai/fleeting-cli/internal/config"
)

type File struct {
	Grid     int    `yaml:"grid"`
	ColOff   int    `yaml:"col_off"`
	ZoomSpan int    `yaml:"zoom"`
	Focus    int    `yaml:"focus"`
	HubIdx   int    `yaml:"hub"`
	Cells    []Cell `yaml:"cells"`
}

type Cell struct {
	Global  int      `yaml:"global"`
	Name    string   `yaml:"name"`
	Harness string   `yaml:"harness"`
	Fleet   string   `yaml:"fleet"`
	Role    string   `yaml:"role,omitempty"`
	Lane    string   `yaml:"lane,omitempty"`
	GUID    string   `yaml:"guid,omitempty"`
	Cmd     []string `yaml:"cmd"`
}

func (c Cell) Missing() []string {
	var miss []string
	if c.Name == "" {
		miss = append(miss, "name")
	}
	if c.Harness == "" {
		miss = append(miss, "harness")
	}
	if len(c.Cmd) == 0 {
		miss = append(miss, "cmd")
	}
	if c.Global < 0 {
		miss = append(miss, "slot")
	}
	return miss
}

func Path() string {
	if p := os.Getenv("FLEETING_WORKSPACE"); p != "" {
		return p
	}
	return filepath.Join(config.Dir(), "workspace.yaml")
}

func Load() (*File, error) {
	p := Path()
	b, err := os.ReadFile(p)
	if err != nil {
		if os.IsNotExist(err) {
			return nil, nil
		}
		return nil, err
	}
	var f File
	if err := yaml.Unmarshal(b, &f); err != nil {
		return nil, err
	}
	f.normalize()
	return &f, nil
}

func Save(f File) error {
	f.normalize()
	b, err := yaml.Marshal(&f)
	if err != nil {
		return err
	}
	p := Path()
	if err := os.MkdirAll(filepath.Dir(p), 0o755); err != nil {
		return err
	}
	return os.WriteFile(p, b, 0o644)
}

func (f *File) normalize() {
	if f.Grid < 3 {
		f.Grid = 3
	}
	if f.Grid > 6 {
		f.Grid = 6
	}
	sort.Slice(f.Cells, func(i, j int) bool {
		if f.Cells[i].Global != f.Cells[j].Global {
			return f.Cells[i].Global < f.Cells[j].Global
		}
		return f.Cells[i].Name < f.Cells[j].Name
	})
}

func Fingerprint(f File) string {
	f.normalize()
	b, err := yaml.Marshal(&f)
	if err != nil {
		return ""
	}
	return string(bytes.TrimSpace(b))
}

func Equal(a, b File) bool {
	return Fingerprint(a) == Fingerprint(b)
}

func (f File) String() string {
	return fmt.Sprintf("grid=%d zoom=%d cells=%d", f.Grid, f.ZoomSpan, len(f.Cells))
}
