package listen

import "time"

const (
	minReplaySpeed     = 2
	maxReplaySpeed     = 16
	defaultReplaySpeed = 4
	maxReplayFrames    = 240
	replayInterval     = 100 * time.Millisecond
)

// ReplayFrame is one sampled grid of PTY screens while no TUI is attached.
type ReplayFrame struct {
	At      time.Time         `json:"at"`
	Screens map[string]string `json:"screens"`
}

// Player plays missed frames at 2–16× wall clock, or jumps to live.
type Player struct {
	Frames []ReplayFrame
	Speed  int
	i      int
	t0     time.Time
	wall0  time.Time
}

func ClampSpeed(n int) int {
	if n < minReplaySpeed {
		return minReplaySpeed
	}
	if n > maxReplaySpeed {
		return maxReplaySpeed
	}
	return n
}

func NewPlayer(frames []ReplayFrame, speed int) *Player {
	if len(frames) == 0 {
		return nil
	}
	return &Player{
		Frames: frames,
		Speed:  ClampSpeed(speed),
		i:      0,
		t0:     frames[0].At,
		wall0:  time.Now(),
	}
}

func (p *Player) FrameIndex() int {
	if p == nil || len(p.Frames) == 0 {
		return 0
	}
	if p.i >= len(p.Frames) {
		return len(p.Frames)
	}
	return p.i + 1
}

func (p *Player) Len() int {
	if p == nil {
		return 0
	}
	return len(p.Frames)
}

func (p *Player) Live() bool {
	return p == nil || p.i >= len(p.Frames)
}

func (p *Player) SetSpeed(n int) {
	if p == nil || p.Live() {
		return
	}
	now := time.Now()
	elapsed := time.Duration(p.Speed) * now.Sub(p.wall0)
	p.Speed = ClampSpeed(n)
	p.wall0 = now
	p.t0 = p.t0.Add(elapsed)
}

func (p *Player) Jump() {
	if p == nil {
		return
	}
	p.i = len(p.Frames)
}

func (p *Player) Advance(now time.Time) {
	if p == nil || p.Live() {
		return
	}
	elapsed := time.Duration(p.Speed) * now.Sub(p.wall0)
	for p.i < len(p.Frames)-1 && p.Frames[p.i+1].At.Sub(p.t0) <= elapsed {
		p.i++
	}
	if p.i == len(p.Frames)-1 {
		lastGap := p.Frames[len(p.Frames)-1].At.Sub(p.t0)
		if elapsed >= lastGap+replayInterval {
			p.i = len(p.Frames)
		}
	}
}

func (p *Player) Overlay(live []Snapshot) []Snapshot {
	if p.Live() {
		return live
	}
	fr := p.Frames[p.i]
	out := make([]Snapshot, len(live))
	copy(out, live)
	for i := range out {
		if sc, ok := fr.Screens[out[i].Name]; ok {
			out[i].Screen = sc
		}
	}
	return out
}

func (s *Server) pushFrame(fr ReplayFrame) {
	s.replayMu.Lock()
	defer s.replayMu.Unlock()
	s.replay = append(s.replay, fr)
	if len(s.replay) > maxReplayFrames {
		s.replay = append([]ReplayFrame(nil), s.replay[len(s.replay)-maxReplayFrames:]...)
	}
}

func (s *Server) takeReplay() []ReplayFrame {
	s.replayMu.Lock()
	defer s.replayMu.Unlock()
	out := s.replay
	s.replay = nil
	return out
}

func (s *Server) replayLoop(ctx interface{ Done() <-chan struct{} }) {
	t := time.NewTicker(replayInterval)
	defer t.Stop()
	for {
		select {
		case <-ctx.Done():
			return
		case <-t.C:
			if s.Attached() {
				continue
			}
			snaps := s.Snapshots()
			if len(snaps) == 0 {
				continue
			}
			screens := make(map[string]string, len(snaps))
			for _, sn := range snaps {
				screens[sn.Name] = sn.Screen
			}
			s.pushFrame(ReplayFrame{At: time.Now(), Screens: screens})
		}
	}
}
