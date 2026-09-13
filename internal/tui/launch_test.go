package tui

import "testing"

func TestAllocSlotNameSkipsTaken(t *testing.T) {
	taken := map[string]bool{"cl00": true, "cl01": true}
	n := allocSlotName("claude", taken)
	if n != "cl02" {
		t.Fatalf("got %s", n)
	}
}

func TestAllocSlotNameUnknownPrefix(t *testing.T) {
	n := allocSlotName("nope", nil)
	if n != "xx00" {
		t.Fatalf("got %s", n)
	}
}
