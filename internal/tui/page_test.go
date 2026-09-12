package tui

import "testing"

func TestPageLabelFullAndSplit(t *testing.T) {
	n := 3
	if g := pageLabel(0, n); g != "Page 1" {
		t.Fatalf("off0 %s", g)
	}
	if g := pageLabel(n, n); g != "Page 2" {
		t.Fatalf("full page 2 %s", g)
	}
	if g := pageLabel(1, n); g != "Page 1-2" {
		t.Fatalf("col+1 %s", g)
	}
	if g := pageLabel(2, n); g != "Page 1-2" {
		t.Fatalf("col+2 %s", g)
	}
}

func TestGlobalIndexColumnMajor(t *testing.T) {
	n := 3
	// first window: 0..8
	if globalIndex(0, n, 0, 0) != 0 || globalIndex(0, n, 2, 2) != 8 {
		t.Fatal("page1 window")
	}
	// col+1: indices 1,2,3 / wait col 0 is global col 1: 3,4,5
	if globalIndex(1, n, 0, 0) != 3 {
		t.Fatalf("got %d", globalIndex(1, n, 0, 0))
	}
	if globalIndex(1, n, 2, 2) != 11 {
		t.Fatalf("last of 4-12 window (0-based 3-11) last=%d", globalIndex(1, n, 2, 2))
	}
}
