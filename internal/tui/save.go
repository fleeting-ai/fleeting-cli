package tui

import (
	"fmt"
	"strings"

	tea "github.com/charmbracelet/bubbletea"
	"github.com/richard-ginsberg/fleeting/internal/config"
	"github.com/richard-ginsberg/fleeting/internal/harness"
	"github.com/richard-ginsberg/fleeting/internal/listen"
	"github.com/richard-ginsberg/fleeting/internal/workspace"
)

func (m Model) workspaceFile() workspace.File {
	cells := make([]workspace.Cell, 0, len(m.extras))
	for _, c := range m.extras {
		cells = append(cells, c)
	}
	return workspace.File{
		Grid:     m.grid,
		ColOff:   m.colOff,
		ZoomSpan: m.zoomSpan,
		Focus:    m.focus,
		HubIdx:   m.hubIdx,
		Cells:    cells,
	}
}

func (m Model) dirty() bool {
	return workspace.Fingerprint(m.workspaceFile()) != m.savedFP
}

func (m Model) captureLiveCells() Model {
	if m.extras == nil {
		m.extras = map[int]workspace.Cell{}
	}
	if m.srv != nil {
		m.snaps = m.srv.Snapshots()
		m.byName = make(map[string]listen.Snapshot, len(m.snaps))
		for _, s := range m.snaps {
			m.byName[s.Name] = s
		}
	}
	names := m.orderedNames()
	for g, name := range names {
		if name == "" {
			continue
		}
		snap, ok := m.byName[name]
		if !ok {
			continue
		}
		cur := m.extras[g]
		cur.Global = g
		cur = fillCell(cur, snap, m.agentByName(name))
		m.extras[g] = cur
	}
	return m
}

func fillCell(c workspace.Cell, snap listen.Snapshot, a *config.Agent) workspace.Cell {
	if c.Name == "" {
		c.Name = snap.Name
	}
	if c.Fleet == "" {
		c.Fleet = snap.Fleet
	}
	if c.Role == "" {
		c.Role = snap.Role
	}
	if c.Lane == "" {
		c.Lane = snap.Lane
	}
	if c.GUID == "" {
		c.GUID = snap.GUID
	}
	if c.Harness == "" {
		c.Harness = snap.Harness
	}
	if len(c.Cmd) == 0 {
		c.Cmd = append([]string{}, snap.Cmd...)
	}
	if a != nil {
		if c.Harness == "" {
			c.Harness = a.Harness
		}
		if len(c.Cmd) == 0 {
			c.Cmd = append([]string{}, a.Cmd...)
		}
		if c.Role == "" {
			c.Role = a.Role
		}
		if c.Lane == "" {
			c.Lane = a.Lane
		}
	}
	if c.Harness == "" && len(c.Cmd) > 0 {
		c.Harness = harness.IDFromBin(c.Cmd[0])
	}
	return c
}

func (m Model) agentByName(name string) *config.Agent {
	if m.cfg == nil {
		return nil
	}
	for i := range m.cfg.Fleets {
		for j := range m.cfg.Fleets[i].Agents {
			if m.cfg.Fleets[i].Agents[j].Name == name {
				return &m.cfg.Fleets[i].Agents[j]
			}
		}
	}
	return nil
}

func (m Model) fleetHas(name string) bool {
	return m.agentByName(name) != nil
}

func (m Model) saveWorkspace() (tea.Model, tea.Cmd) {
	m = m.captureLiveCells()
	f := m.workspaceFile()
	if err := validateCells(f); err != nil {
		m.lastErr = err.Error()
		return m, nil
	}
	if err := workspace.Save(f); err != nil {
		m.lastErr = err.Error()
		return m, nil
	}
	m.savedFP = workspace.Fingerprint(f)
	m.lastErr = ""
	m.status = "workspace saved"
	return m, nil
}

func validateCells(f workspace.File) error {
	for _, c := range f.Cells {
		if miss := c.Missing(); len(miss) > 0 {
			label := c.Name
			if label == "" {
				label = fmt.Sprintf("slot %d", c.Global)
			}
			return fmt.Errorf("save: %s missing %s", label, strings.Join(miss, ", "))
		}
	}
	return nil
}

func (m Model) requestQuit() (tea.Model, tea.Cmd) {
	m = m.captureLiveCells()
	if !m.dirty() {
		return m, tea.Quit
	}
	m.mode = "quit"
	return m, nil
}

func (m Model) quitKey(msg tea.KeyMsg) (tea.Model, tea.Cmd) {
	s := msg.String()
	switch {
	case s == "esc", msg.Type == tea.KeyEsc:
		m.mode = ""
		return m, nil
	case s == "n", s == "N":
		return m, tea.Quit
	case s == "s", s == "S":
		saved, cmd := m.saveWorkspace()
		sm := saved.(Model)
		if sm.lastErr != "" {
			sm.mode = ""
			return sm, cmd
		}
		return sm, tea.Quit
	}
	return m, nil
}
