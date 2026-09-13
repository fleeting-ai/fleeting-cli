package workspace

import (
	"os"
	"path/filepath"
	"testing"
)

func TestFingerprintIgnoresCellOrder(t *testing.T) {
	a := File{Grid: 3, Cells: []Cell{{Global: 8, Name: "luna"}, {Global: 2, Name: "zara"}}}
	b := File{Grid: 3, Cells: []Cell{{Global: 2, Name: "zara"}, {Global: 8, Name: "luna"}}}
	if !Equal(a, b) {
		t.Fatal("order should not dirty")
	}
}

func TestFingerprintDetectsLayoutChange(t *testing.T) {
	a := File{Grid: 3, ZoomSpan: 0}
	b := File{Grid: 3, ZoomSpan: 2}
	if Equal(a, b) {
		t.Fatal("zoom change is dirty")
	}
}

func TestSaveLoadRoundTrip(t *testing.T) {
	dir := t.TempDir()
	t.Setenv("FLEETING_WORKSPACE", filepath.Join(dir, "workspace.yaml"))
	in := File{Grid: 3, ZoomSpan: 2, Focus: 8, Cells: []Cell{{Global: 8, Name: "luna", Harness: "pi", Cmd: []string{"pi"}}}}
	if err := Save(in); err != nil {
		t.Fatal(err)
	}
	out, err := Load()
	if err != nil || out == nil {
		t.Fatalf("%v %+v", err, out)
	}
	if !Equal(in, *out) {
		t.Fatalf("got %+v", out)
	}
	if _, err := os.Stat(Path()); err != nil {
		t.Fatal(err)
	}
}
