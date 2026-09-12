package listen

import (
	"strings"
	"testing"

	"github.com/hinshun/vt10x"
)

func TestDumpVTKeepsANSIColor(t *testing.T) {
	vt := vt10x.New(vt10x.WithSize(10, 2))
	_, _ = vt.Write([]byte("\x1b[31mHi\x1b[0m"))
	s := dumpVT(vt)
	if !strings.Contains(s, "\x1b[") {
		t.Fatalf("expected ANSI in dump, got %q", s)
	}
	if !strings.Contains(s, "Hi") {
		t.Fatalf("expected text, got %q", s)
	}
}
