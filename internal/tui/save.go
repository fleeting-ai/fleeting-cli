package tui

import (
	tea "github.com/charmbracelet/bubbletea"
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

func (m Model) saveWorkspace() (tea.Model, tea.Cmd) {
	if err := workspace.Save(m.workspaceFile()); err != nil {
		m.lastErr = err.Error()
		return m, nil
	}
	m.savedFP = workspace.Fingerprint(m.workspaceFile())
	m.lastErr = ""
	m.status = "workspace saved"
	return m, nil
}

func (m Model) requestQuit() (tea.Model, tea.Cmd) {
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
		if err := workspace.Save(m.workspaceFile()); err != nil {
			m.lastErr = err.Error()
			m.mode = ""
			return m, nil
		}
		return m, tea.Quit
	}
	return m, nil
}
