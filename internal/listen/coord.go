package listen

import (
	"fmt"
	"strings"

	"github.com/richard-ginsberg/fleeting/internal/bus"
)

func (s *Server) Coord(p Packet) (Packet, error) {
	if p.Kind == "" {
		return Packet{}, fmt.Errorf("coord needs kind")
	}
	if s.store == nil {
		return Packet{OK: false, Err: "store down"}, fmt.Errorf("store down")
	}
	out := Packet{OK: true, Op: "coord", Kind: p.Kind}
	var err error
	switch p.Kind {
	case "listen":
		out.Mail, err = s.coordListen(p.Name)
	case "log":
		out.Mail, err = s.coordLog(p)
	case "card":
		out.Body, err = s.coordCard(p.Name)
	case "rules_get":
		out.Body, err = s.coordRulesGet()
	case "rules_set":
		err = s.coordRulesSet(p.From, p.Body)
	case "rules_log":
		out.Body, err = s.coordRulesLog()
	case "memory_get":
		out.Body, err = s.coordMemoryGet(p.From, p.Name, p.Key)
	case "memory_set":
		err = s.coordMemorySet(p.From, p.Name, p.Key, p.Body)
	case "memory_list":
		out.Body, err = s.coordMemoryList(p.From, p.Name)
	case "memory_log":
		out.Body, err = s.coordMemoryLog(p.From, p.Name, p.Key)
	case "onboard_get":
		out.Body, err = s.coordOnboardGet(p.Name)
	case "onboard_set":
		err = s.coordOnboardSet(p.From, p.Name, p.Body)
	default:
		err = fmt.Errorf("unknown coord kind %s", p.Kind)
	}
	if err != nil {
		return Packet{OK: false, Op: "coord", Kind: p.Kind, Err: err.Error()}, err
	}
	return out, nil
}

func (s *Server) coordListen(as string) ([]bus.Record, error) {
	w := s.world()
	if _, ok := w.Get(as); !ok {
		return nil, fmt.Errorf("unknown agent %s", as)
	}
	rec, err := s.store.Unread(as)
	if err != nil || rec == nil {
		return nil, err
	}
	if err := s.store.MarkListened(rec.ID, as); err != nil {
		return nil, err
	}
	rec.ListenedAt = bus.Now()
	return []bus.Record{*rec}, nil
}

func (s *Server) coordLog(p Packet) ([]bus.Record, error) {
	if p.From == "" && p.Name == "" && !p.All {
		n := p.Limit
		if n <= 0 {
			n = 4
		}
		return s.store.LastConsumed(n)
	}
	as := p.Name
	if as == "" {
		as = p.From
	}
	w := s.world()
	src, ok := w.Get(as)
	if !ok {
		return nil, fmt.Errorf("unknown agent %s", as)
	}
	if p.All {
		if !bus.CanLogAll(src.Rank) {
			return nil, fmt.Errorf("denied: %s cannot log --all", as)
		}
		return s.store.Log(bus.LogQuery{All: true, Limit: p.Limit})
	}
	if p.Team != "" {
		if !w.Member(as, p.Team) {
			return nil, fmt.Errorf("denied: %s is not a member of %s", as, p.Team)
		}
		return s.store.Log(bus.LogQuery{As: as, Team: p.Team, Limit: p.Limit})
	}
	return s.store.Log(bus.LogQuery{As: as, Teams: w.TeamsOf(as), Limit: p.Limit})
}

func (s *Server) coordCard(as string) (string, error) {
	w := s.world()
	src, ok := w.Get(as)
	if !ok {
		return "", fmt.Errorf("unknown agent %s", as)
	}
	ob, err := s.store.GetOnboarding(as)
	if err != nil {
		return "", err
	}
	rules, err := s.store.GetRules()
	if err != nil {
		return "", err
	}
	fleet, err := s.store.ListMemory("fleet", 20)
	if err != nil {
		return "", err
	}
	fleetN, _ := s.store.CountMemory("fleet")
	teams := map[string][]bus.Memory{}
	teamNs := map[string]int{}
	for _, id := range w.TeamsOf(as) {
		ms, err := s.store.ListMemory("team:"+id, 20)
		if err != nil {
			return "", err
		}
		n, _ := s.store.CountMemory("team:" + id)
		teams[id] = ms
		teamNs[id] = n
	}
	if bus.CanListAllScopes(src.Rank) {
		// still only print this agent's teams on the card, per spec
		_ = src
	}
	return bus.FormatCard(ob, rules, fleet, fleetN, teams, teamNs), nil
}

func (s *Server) coordRulesGet() (string, error) {
	d, err := s.store.GetRules()
	if err != nil || d == nil {
		return "", err
	}
	return d.Body, nil
}

func (s *Server) coordRulesSet(from, body string) error {
	w := s.world()
	src, ok := w.Get(from)
	if !ok {
		return fmt.Errorf("unknown agent %s", from)
	}
	if !bus.CanAppendRules(src.Rank) {
		return fmt.Errorf("denied: rank >= 40 required to set rules")
	}
	return s.store.AppendRules(from, body)
}

func (s *Server) coordRulesLog() (string, error) {
	docs, err := s.store.RulesLog()
	if err != nil {
		return "", err
	}
	var b strings.Builder
	for _, d := range docs {
		fmt.Fprintf(&b, "v%d %s %s\n%s\n", d.Version, d.UpdatedBy, d.UpdatedAt, d.Body)
	}
	return b.String(), nil
}

func (s *Server) coordMemoryGet(as, scope, key string) (string, error) {
	w := s.world()
	if err := bus.CanReadMemory(w, as, scope); err != nil {
		return "", err
	}
	m, err := s.store.GetMemory(scope, key)
	if err != nil || m == nil {
		return "", err
	}
	return fmt.Sprintf("%s %s v%d %s\n%s\n", m.Scope, m.Key, m.Version, m.UpdatedBy, m.Body), nil
}

func (s *Server) coordMemorySet(from, scope, key, body string) error {
	w := s.world()
	if err := bus.CanWriteMemory(w, from, scope); err != nil {
		return err
	}
	_, err := s.store.AppendMemory(scope, key, from, body)
	return err
}

func (s *Server) coordMemoryList(as, scope string) (string, error) {
	w := s.world()
	src, ok := w.Get(as)
	if !ok {
		return "", fmt.Errorf("unknown agent %s", as)
	}
	var scopes []string
	if scope != "" {
		if err := bus.CanReadMemory(w, as, scope); err != nil {
			return "", err
		}
		scopes = []string{scope}
	} else if bus.CanListAllScopes(src.Rank) {
		scopes = []string{"fleet"}
		for id := range w.Teams {
			scopes = append(scopes, "team:"+id)
		}
	} else {
		scopes = []string{"fleet"}
		for _, id := range w.TeamsOf(as) {
			scopes = append(scopes, "team:"+id)
		}
	}
	var b strings.Builder
	for _, sc := range scopes {
		ms, err := s.store.ListMemory(sc, 50)
		if err != nil {
			return "", err
		}
		fmt.Fprintf(&b, "== %s ==\n", sc)
		for _, m := range ms {
			fmt.Fprintf(&b, "%s v%d %s\n", m.Key, m.Version, m.Body)
		}
	}
	return b.String(), nil
}

func (s *Server) coordMemoryLog(as, scope, key string) (string, error) {
	w := s.world()
	if err := bus.CanReadMemory(w, as, scope); err != nil {
		return "", err
	}
	ms, err := s.store.MemoryLog(scope, key)
	if err != nil {
		return "", err
	}
	var b strings.Builder
	for _, m := range ms {
		fmt.Fprintf(&b, "v%d %s %s\n%s\n", m.Version, m.UpdatedBy, m.UpdatedAt, m.Body)
	}
	return b.String(), nil
}

func (s *Server) coordOnboardGet(name string) (string, error) {
	d, err := s.store.GetOnboarding(name)
	if err != nil || d == nil {
		return "", err
	}
	return d.Body, nil
}

func (s *Server) coordOnboardSet(from, name, body string) error {
	w := s.world()
	if err := bus.CanAppendOnboarding(w, from, name); err != nil {
		return err
	}
	return s.store.AppendOnboarding(name, from, body)
}
