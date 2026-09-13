package harness

import (
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
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
			Cmd:  cmdFor(sp.ID, p),
		}, true
	}
	return Installed{}, false
}

func cmdFor(_, path string) []string {
	return []string{path}
}

// CmdForAgent is the argv to spawn. OMP gets --profile <persona>, never --alias.
func (in Installed) CmdForAgent(name string) []string {
	if in.ID == "omp" || filepath.Base(in.Path) == "omp" {
		return []string{in.Path, "--profile", name}
	}
	return append([]string{}, in.Cmd...)
}

func canExec(p string) bool {
	st, err := os.Stat(p)
	if err != nil || st.IsDir() {
		return false
	}
	return st.Mode()&0o111 != 0
}
