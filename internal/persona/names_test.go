package persona

import "testing"

func TestNextSkipsTaken(t *testing.T) {
	taken := map[string]bool{"luna": true, "nova": true}
	if Next(taken) != "nora" {
		t.Fatalf("got %s", Next(taken))
	}
}

func TestNextEmptyTaken(t *testing.T) {
	if Next(nil) != "luna" {
		t.Fatalf("got %s", Next(nil))
	}
}
