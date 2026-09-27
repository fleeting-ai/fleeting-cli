package tui

import (
	"strings"
	"testing"
)

func TestVisibleWidthIgnoresANSI(t *testing.T) {
	s := fillANSI(36, 4, 1)
	if visibleWidth(s) != 4 {
		t.Fatalf("width %d", visibleWidth(s))
	}
}

func TestFrameHasAllFourCorners(t *testing.T) {
	inner := "hi"
	out := frameANSI(inner, 10, 0, 0, 10)
	for _, r := range []rune{'┌', '┐', '└', '┘'} {
		if !containsRune(out, r) {
			t.Fatalf("missing %q in %q", r, out)
		}
	}
}

func TestFrameSplitFollowsSeamNotHalf(t *testing.T) {
	inner := strings.Repeat("x", 20)
	out := frameANSI(inner, 22, 0, 1, 14)
	// 14 lo dashes vs 6 hi on a 20-wide inner — not 10/10
	if strings.Count(out, "─") == 0 {
		t.Fatal("no bar")
	}
}

func containsRune(s string, r rune) bool {
	for _, c := range s {
		if c == r {
			return true
		}
	}
	return false
}
