package tui

import "testing"

func TestCellAddr(t *testing.T) {
	if cellAddr(0, 0) != "A1" || cellAddr(3, 3) != "D4" {
		t.Fatalf("A1/D4 %s %s", cellAddr(0, 0), cellAddr(3, 3))
	}
}

func TestParseGotoCoordAndLinear(t *testing.T) {
	vis, ok := parseGoto("d4", 4)
	if !ok || vis != 15 {
		t.Fatalf("d4 vis=%d ok=%v", vis, ok)
	}
	vis, ok = parseGoto("16", 4)
	if !ok || vis != 15 {
		t.Fatalf("16 vis=%d ok=%v", vis, ok)
	}
	if _, ok := parseGoto("e1", 4); ok {
		t.Fatal("e1 invalid on 4x4")
	}
}

func TestKeypadFocus3and4(t *testing.T) {
	if keypadFocus(3, 3) != 2 {
		t.Fatal("3x3 alt3")
	}
	// 4x4: Alt+3 is C1 (vis 2), not D1 (vis 3)
	if keypadFocus(3, 4) != 2 {
		t.Fatalf("got %d", keypadFocus(3, 4))
	}
	if keypadFocus(9, 4) != 2*4+2 { // C3
		t.Fatalf("alt9 %d", keypadFocus(9, 4))
	}
}
