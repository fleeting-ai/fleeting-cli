package spool

import (
	"encoding/json"
	"os"
	"path/filepath"
	"sync"
	"time"
)

type Event struct {
	Kind  string    `json:"kind"`
	From  string    `json:"from,omitempty"`
	To    string    `json:"to,omitempty"`
	Body  string    `json:"body,omitempty"`
	Files []File    `json:"files,omitempty"`
	At    time.Time `json:"at"`
}

type File struct {
	Path string `json:"path"`
	MD5  string `json:"md5"`
}

type Declaration struct {
	Agent  string `json:"agent"`
	Intent string `json:"intent"`
	Files  []File `json:"files"`
	At     time.Time `json:"at"`
}

var mu sync.Mutex

func Append(home string, e Event) error {
	mu.Lock()
	defer mu.Unlock()
	if err := os.MkdirAll(home, 0o755); err != nil {
		return err
	}
	p := filepath.Join(home, "events.jsonl")
	f, err := os.OpenFile(p, os.O_APPEND|os.O_CREATE|os.O_WRONLY, 0o644)
	if err != nil {
		return err
	}
	defer f.Close()
	b, _ := json.Marshal(e)
	_, err = f.Write(append(b, '\n'))
	return err
}

func Declare(home string, d Declaration) error {
	return Append(home, Event{
		Kind:  "declare",
		From:  d.Agent,
		Body:  d.Intent,
		Files: d.Files,
		At:    d.At,
	})
}

func Tail(home string, n int) []Event {
	p := filepath.Join(home, "events.jsonl")
	b, err := os.ReadFile(p)
	if err != nil {
		return nil
	}
	lines := splitLines(string(b))
	if n > 0 && len(lines) > n {
		lines = lines[len(lines)-n:]
	}
	var out []Event
	for _, ln := range lines {
		var e Event
		if json.Unmarshal([]byte(ln), &e) == nil {
			out = append(out, e)
		}
	}
	return out
}

func splitLines(s string) []string {
	var lines []string
	start := 0
	for i := 0; i < len(s); i++ {
		if s[i] == '\n' {
			if i > start {
				lines = append(lines, s[start:i])
			}
			start = i + 1
		}
	}
	if start < len(s) {
		lines = append(lines, s[start:])
	}
	return lines
}
