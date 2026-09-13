package tui

import (
	"fmt"

	tea "github.com/charmbracelet/bubbletea"
	"github.com/richard-ginsberg/fleeting/internal/config"
	"github.com/richard-ginsberg/fleeting/internal/harness"
	"github.com/richard-ginsberg/fleeting/internal/persona"
)

func (m Model) openLaunch() (tea.Model, tea.Cmd) {
	gidx := m.visualToGlobal(m.focus)
	if m.snapAt(gidx) != nil {
		m.lastErr = "cell occupied — pick a blank cell"
		m.status = ""
		return m, nil
	}
	m.launch = harness.InstalledOnPATH()
	m.mode = "launch"
	m.lastErr = ""
	return m, nil
}

func (m Model) launchKey(msg tea.KeyMsg) (tea.Model, tea.Cmd) {
	switch msg.Type {
	case tea.KeyEsc:
		m.mode = ""
		m.launch = nil
		return m, nil
	case tea.KeyEnter:
		if len(m.launch) == 1 {
			return m.pickLaunch(0)
		}
		return m, nil
	}
	if msg.Type == tea.KeyRunes && len(msg.Runes) == 1 {
		d := int(msg.Runes[0] - '1')
		if d >= 0 && d < len(m.launch) {
			return m.pickLaunch(d)
		}
	}
	s := msg.String()
	if len(s) == 1 && s[0] >= '1' && s[0] <= '9' {
		d := int(s[0] - '1')
		if d >= 0 && d < len(m.launch) {
			return m.pickLaunch(d)
		}
	}
	return m, nil
}

func (m Model) pickLaunch(i int) (tea.Model, tea.Cmd) {
	if i < 0 || i >= len(m.launch) {
		return m, nil
	}
	in := m.launch[i]
	gidx := m.visualToGlobal(m.focus)
	if m.snapAt(gidx) != nil {
		m.lastErr = "cell occupied"
		m.mode = ""
		m.launch = nil
		return m, nil
	}
	name := persona.Next(m.takenNames())
	if name == "" {
		m.lastErr = "no free persona name"
		m.mode = ""
		m.launch = nil
		return m, nil
	}
	fleetID := "ad-hoc"
	if len(m.cfg.Fleets) > 0 {
		fleetID = m.cfg.Fleets[m.hubIdx].ID
	}
	a := config.Agent{
		Name:    name,
		Role:    "worker",
		Lane:    in.ID,
		Harness: in.ID,
		Cmd:     in.CmdForAgent(name),
	}
	if err := m.srv.Spawn(fleetID, a); err != nil {
		m.lastErr = err.Error()
		m.mode = ""
		m.launch = nil
		return m, nil
	}
	if m.extras == nil {
		m.extras = map[int]string{}
	}
	m.extras[gidx] = name
	m.mode = ""
	m.launch = nil
	m.lastErr = ""
	m.status = fmt.Sprintf("%s %s", visAddr(m.focus, m.grid), in.Title)
	m.applyPTYs(m.paneSize())
	return m, nil
}

func (m Model) takenNames() map[string]bool {
	taken := map[string]bool{}
	for _, n := range m.orderedNames() {
		if n != "" {
			taken[n] = true
		}
	}
	for _, n := range m.extras {
		taken[n] = true
	}
	return taken
}
