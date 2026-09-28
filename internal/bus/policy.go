package bus

import (
	"fmt"
	"strings"

	"github.com/fleeting-ai/fleeting-cli/internal/config"
)

type Send struct {
	From    string
	To      string
	Team    string
	Action  string
	Body    string
	Thread  string
	ReplyTo string
}

type Agent struct {
	Name  string
	Role  string
	Rank  int
	Peers []string
}

type World struct {
	Agents map[string]Agent
	Teams  map[string][]string
}

func WorldFrom(cfg *config.File, extra []config.Agent) World {
	w := World{Agents: map[string]Agent{}, Teams: map[string][]string{}}
	if cfg != nil {
		for _, fl := range cfg.Fleets {
			for _, a := range fl.Agents {
				w.Agents[a.Name] = Agent{Name: a.Name, Role: a.Role, Rank: config.RankOf(a.Role, a.Rank), Peers: append([]string{}, a.Peers...)}
			}
		}
		for _, t := range cfg.Teams {
			w.Teams[t.ID] = append([]string{}, t.Members...)
		}
	}
	for _, a := range extra {
		if _, ok := w.Agents[a.Name]; ok {
			continue
		}
		w.Agents[a.Name] = Agent{Name: a.Name, Role: a.Role, Rank: config.RankOf(a.Role, a.Rank), Peers: append([]string{}, a.Peers...)}
	}
	return w
}

func (w World) Get(name string) (Agent, bool) {
	a, ok := w.Agents[strings.ToLower(strings.TrimSpace(name))]
	return a, ok
}

func (w World) Member(name, team string) bool {
	for _, m := range w.Teams[team] {
		if m == name {
			return true
		}
	}
	return false
}

func (w World) TeamsOf(name string) []string {
	var out []string
	for id, members := range w.Teams {
		for _, m := range members {
			if m == name {
				out = append(out, id)
				break
			}
		}
	}
	return out
}

// Allow validates a send and returns one envelope per recipient (order stamped per copy).
func (w World) Allow(s Send) ([]Envelope, error) {
	s.From = strings.ToLower(strings.TrimSpace(s.From))
	s.To = strings.ToLower(strings.TrimSpace(s.To))
	s.Team = strings.ToLower(strings.TrimSpace(s.Team))
	s.Action = strings.ToLower(strings.TrimSpace(s.Action))
	if s.Action == "" {
		s.Action = "fyi"
	}
	body, err := checkBody(s.Body)
	if err != nil {
		return nil, err
	}
	s.Body = body
	switch s.Action {
	case "fyi", "todo", "order", "ack":
	default:
		return nil, fmt.Errorf("unknown action %s", s.Action)
	}
	src, ok := w.Get(s.From)
	if !ok {
		return nil, fmt.Errorf("unknown agent %s", s.From)
	}
	if s.To != "" && s.Team != "" {
		return nil, fmt.Errorf("direct send cannot set team")
	}
	if s.To == "" && s.Team == "" {
		return nil, fmt.Errorf("need --to or --team")
	}

	var recips []string
	team := ""
	if s.Team != "" {
		if s.Action == "ack" {
			return nil, fmt.Errorf("ack is direct only")
		}
		members, ok := w.Teams[s.Team]
		if !ok {
			return nil, fmt.Errorf("unknown team %s", s.Team)
		}
		if !w.Member(s.From, s.Team) {
			return nil, fmt.Errorf("denied: %s is not a member of %s", s.From, s.Team)
		}
		for _, m := range members {
			if m == s.From {
				continue
			}
			if _, ok := w.Get(m); !ok {
				return nil, fmt.Errorf("unknown agent %s", m)
			}
			recips = append(recips, m)
		}
		if len(recips) == 0 {
			return nil, fmt.Errorf("team %s has no other members", s.Team)
		}
		team = s.Team
	} else {
		if s.To == s.From {
			return nil, fmt.Errorf("cannot send to self")
		}
		if _, ok := w.Get(s.To); !ok {
			return nil, fmt.Errorf("unknown agent %s", s.To)
		}
		allowed := false
		for _, p := range src.Peers {
			if p == s.To {
				allowed = true
				break
			}
		}
		if !allowed {
			return nil, fmt.Errorf("denied: %s cannot speak to %s (default deny)", s.From, s.To)
		}
		recips = []string{s.To}
	}

	if s.Action == "ack" {
		if strings.TrimSpace(s.ReplyTo) == "" {
			return nil, fmt.Errorf("ack requires --reply-to")
		}
	}
	if s.Action == "order" {
		for _, name := range recips {
			dst, _ := w.Get(name)
			if src.Rank <= dst.Rank {
				return nil, fmt.Errorf("order rejected: %s rank %d does not outrank %s rank %d", s.From, src.Rank, name, dst.Rank)
			}
		}
	}

	id := NewID()
	at := Now()
	out := make([]Envelope, 0, len(recips))
	for _, name := range recips {
		dst, _ := w.Get(name)
		env := Envelope{
			ID:      id,
			Action:  s.Action,
			From:    s.From,
			To:      name,
			Team:    team,
			Rank:    src.Rank,
			Thread:  s.Thread,
			ReplyTo: s.ReplyTo,
			Body:    s.Body,
			At:      at,
		}
		switch s.Action {
		case "fyi", "ack":
			env.Order = false
		case "order":
			env.Order = true
		case "todo":
			env.Order = src.Rank > dst.Rank
		}
		out = append(out, env)
	}
	return out, nil
}
