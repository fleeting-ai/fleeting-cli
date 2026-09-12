package presence

import (
	"strings"
	"time"
)

func Of(alive bool, last time.Time, screen string) string {
	lower := strings.ToLower(screen)
	if strings.Contains(lower, "out of tokens") || strings.Contains(lower, "rate limit") {
		return "tokens"
	}
	if strings.Contains(lower, "?") && strings.Contains(lower, "need") {
		return "attention"
	}
	if !alive {
		if time.Since(last) < 15*time.Second {
			return "stopped"
		}
		return "stopped"
	}
	if time.Since(last) < 4*time.Second {
		return "working"
	}
	if strings.Contains(lower, "waiting") || strings.Contains(lower, "idle") {
		return "attention"
	}
	return "working"
}
