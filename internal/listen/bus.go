package listen

import (
	"context"
	"fmt"
	"os"
	"strings"
	"time"

	"github.com/fleeting-ai/fleeting-cli/internal/bus"
	"github.com/fleeting-ai/fleeting-cli/internal/config"
)

func (s *Server) startBus(ctx context.Context) {
	db := bus.DBURL(s.cfg.Bus.DB)
	st, err := bus.Open(db)
	if err != nil {
		s.storeErr = err
		fmt.Fprintf(os.Stderr, "fleeting store down: %v\n", err)
	} else {
		s.store = st
		s.storeErr = nil
		_ = st.SeedRules(bus.AgentRules)
		for _, fl := range s.cfg.Fleets {
			for _, a := range fl.Agents {
				s.seedOnboard(a)
			}
		}
	}
	s.broker = bus.NewBroker(bus.AMQPURL(s.cfg.Bus.URL))
	s.hookBroker()
	go s.busRetry(ctx)
}

func (s *Server) hookBroker() {
	if s.broker == nil {
		return
	}
	s.broker.SetHooks(s.store, s.agentAlive, func(name, frame string) error {
		err := s.Write(name, []byte(frame))
		if err != nil {
			s.mu.Lock()
			if se := s.sess[name]; se != nil {
				se.mu.Lock()
				se.err = err.Error()
				se.mu.Unlock()
			}
			s.mu.Unlock()
		}
		return err
	})
}

func (s *Server) agentAlive(name string) bool {
	s.mu.Lock()
	se := s.sess[name]
	s.mu.Unlock()
	return processLive(se)
}

func (s *Server) busRetry(ctx context.Context) {
	for {
		select {
		case <-ctx.Done():
			return
		default:
		}
		if err := s.broker.Connect(); err != nil {
			s.busErr = err
			select {
			case <-ctx.Done():
				return
			case <-time.After(3 * time.Second):
			}
			continue
		}
		s.busErr = nil
		s.declareAll()
		s.hookBroker()
		s.startTeamFanout(ctx)
		s.startLiveConsumers()
		for s.broker.Up() {
			select {
			case <-ctx.Done():
				return
			case <-time.After(2 * time.Second):
			}
		}
	}
}

func (s *Server) declareAll() {
	if s.broker == nil || !s.broker.Up() {
		return
	}
	for _, fl := range s.cfg.Fleets {
		for _, a := range fl.Agents {
			_ = s.broker.DeclareAgent(a.Name)
		}
	}
	s.mu.Lock()
	for name := range s.sess {
		_ = s.broker.DeclareAgent(name)
	}
	s.mu.Unlock()
	for _, t := range s.cfg.Teams {
		_ = s.broker.DeclareTeam(t.ID)
	}
}

func (s *Server) startTeamFanout(ctx context.Context) {
	ranks := map[string]int{}
	w := s.world()
	for n, a := range w.Agents {
		ranks[n] = a.Rank
	}
	for _, t := range s.cfg.Teams {
		team, members := t.ID, append([]string{}, t.Members...)
		go s.broker.StartTeamFanout(ctx, team, members, ranks)
	}
}

func (s *Server) startLiveConsumers() {
	s.mu.Lock()
	var names []string
	for name, se := range s.sess {
		if processLive(se) {
			names = append(names, name)
		}
	}
	s.mu.Unlock()
	for _, n := range names {
		s.broker.StartAgentConsumer(n)
	}
}

func (s *Server) seedOnboard(a config.Agent) {
	if s.store == nil {
		return
	}
	teams := s.cfg.TeamsOf(a.Name)
	_ = s.store.SeedOnboarding(a.Name, bus.SeedCard(a.Name, a.Role, config.RankOf(a.Role, a.Rank), a.Peers, teams))
}

func (s *Server) world() bus.World {
	var extra []config.Agent
	s.mu.Lock()
	for _, se := range s.sess {
		if processLive(se) {
			a := se.Agent
			if a.Rank == 0 {
				a.Rank = config.RankOf(a.Role, a.Rank)
			}
			extra = append(extra, a)
		}
	}
	s.mu.Unlock()
	return bus.WorldFrom(s.cfg, extra)
}

func (s *Server) RouteMail(send bus.Send) error {
	if s.broker == nil || !s.broker.Up() {
		err := s.busErr
		if err == nil {
			err = fmt.Errorf("bus down")
		}
		return err
	}
	copies, err := s.world().Allow(send)
	if err != nil {
		return err
	}
	if send.Team != "" {
		env := copies[0]
		env.To = ""
		env.Order = false
		env.Team = send.Team
		return s.broker.PublishTeam(send.Team, env)
	}
	return s.broker.PublishAgent(copies[0])
}

func (s *Server) RoutePublic(from, to, body string) error {
	return s.RouteMail(bus.Send{From: from, To: to, Body: body})
}

func (s *Server) onSpawnBus(a config.Agent) {
	if a.Rank == 0 {
		a.Rank = config.RankOf(a.Role, a.Rank)
	}
	s.seedOnboard(a)
	if s.broker != nil && s.broker.Up() {
		_ = s.broker.DeclareAgent(a.Name)
		s.broker.StartAgentConsumer(a.Name)
	}
}

func (s *Server) onReapBus(name string) {
	if s.broker != nil {
		s.broker.StopAgentConsumer(name)
	}
}

func busToken(up bool, prefix string) string {
	if up {
		return prefix + " up"
	}
	return prefix + " down"
}

func (s *Server) busStatus() (busLine, storeLine, busURL, storeURL string) {
	busURL = bus.RedactURL(bus.AMQPURL(s.cfg.Bus.URL))
	storeURL = bus.RedactURL(func() string {
		kind, target := bus.ParseDB(bus.DBURL(s.cfg.Bus.DB))
		if kind == "sqlite" && !strings.Contains(target, "://") {
			return "sqlite://" + target
		}
		return target
	}())
	if s.broker != nil && s.broker.Up() {
		busLine = "up"
	} else {
		busLine = "down"
	}
	if s.store != nil && s.store.Ping() == nil {
		storeLine = s.store.Driver() + " up"
	} else {
		storeLine = "down"
	}
	return busLine, storeLine, busURL, storeURL
}
