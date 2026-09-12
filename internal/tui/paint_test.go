package tui

import "testing"

func TestVisibleWidthIgnoresANSI(t *testing.T) {
	s := fillANSI(36, 4, 1)
	if visibleWidth(s) != 4 {
		t.Fatalf("width %d", visibleWidth(s))
	}
}

func TestFrameHasAllFourCorners(t *testing.T) {
	inner := "hi"
	out := frameANSI(inner, 10, 5, 0, 0)
	for _, r := range []rune{'┌', '┐', '└', '┘'} {
		if !containsRune(out, r) {
			t.Fatalf("missing %q in %q", r, out)
		}
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
