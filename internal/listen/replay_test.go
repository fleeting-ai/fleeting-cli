package listen

import (
	"testing"
	"time"
)

func TestPlayerReplaySpeedAndJump(t *testing.T) {
	t0 := time.Unix(1000, 0)
	frames := []ReplayFrame{
		{At: t0, Screens: map[string]string{"luna": "a"}},
		{At: t0.Add(400 * time.Millisecond), Screens: map[string]string{"luna": "b"}},
		{At: t0.Add(800 * time.Millisecond), Screens: map[string]string{"luna": "c"}},
	}
	p := NewPlayer(frames, 4)
	if p.Speed != 4 {
		t.Fatalf("default clamp: %d", p.Speed)
	}
	p.wall0 = time.Unix(0, 0)
	p.t0 = t0
	p.Advance(time.Unix(0, 0).Add(100 * time.Millisecond))
	// 4× * 100ms = 400ms → frame b
	if p.Live() || p.i != 1 {
		t.Fatalf("got i=%d live=%v", p.i, p.Live())
	}
	live := []Snapshot{{Name: "luna", Screen: "NOW", Alive: true}}
	out := p.Overlay(live)
	if out[0].Screen != "b" {
		t.Fatalf("overlay %q", out[0].Screen)
	}
	p.SetSpeed(16)
	if p.Speed != 16 {
		t.Fatal(p.Speed)
	}
	p.Jump()
	if !p.Live() {
		t.Fatal("jump should be live")
	}
	if p.Overlay(live)[0].Screen != "NOW" {
		t.Fatal("live overlay")
	}
}

func TestClampSpeed(t *testing.T) {
	if ClampSpeed(1) != 2 || ClampSpeed(99) != 16 {
		t.Fatal(ClampSpeed(1), ClampSpeed(99))
	}
}
