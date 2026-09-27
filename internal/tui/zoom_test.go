package tui

import "testing"

func TestZoomOriginB2CoversA1B1A2(t *testing.T) {
	// B2 is col=1,row=1 on 3x3
	c0, r0 := zoomOrigin(1, 1, 3, 2)
	if c0 != 0 || r0 != 0 {
		t.Fatalf("origin %d,%d", c0, r0)
	}
	if !inZoom(0, 0, c0, r0, 2) || !inZoom(1, 0, c0, r0, 2) || !inZoom(0, 1, c0, r0, 2) || !inZoom(1, 1, c0, r0, 2) {
		t.Fatal("expected A1 B1 A2 B2")
	}
	if inZoom(2, 0, c0, r0, 2) || inZoom(0, 2, c0, r0, 2) {
		t.Fatal("C1 and A3 should stay small")
	}
}
