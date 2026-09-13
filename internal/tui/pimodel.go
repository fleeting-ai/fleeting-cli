package tui

import (
	"fmt"
	"strings"

	tea "github.com/charmbracelet/bubbletea"
	"github.com/richard-ginsberg/fleeting/internal/pi"
)

func (m Model) openPiModel() (tea.Model, tea.Cmd) {
	m.mode = "pimodel"
	m.piStep = 0
	m.piBuf = ""
	m.piDraft = pi.Draft{Host: "127.0.0.1", Path: "/v1", ContextWindow: 32768, MaxTokens: 8192}
	m.lastErr = ""
	return m, nil
}

func (m Model) piModelPrompt() string {
	switch m.piStep {
	case 0:
		return " pi model engine: 1 llama.cpp  2 vLLM  3 SGLang  Esc"
	case 1:
		return fmt.Sprintf(" host [%s]: %s_", m.piDraft.Host, m.piBuf)
	case 2:
		return fmt.Sprintf(" port [%d]: %s_", m.piDraft.Port, m.piBuf)
	case 3:
		return fmt.Sprintf(" model id: %s_", m.piBuf)
	case 4:
		return fmt.Sprintf(" api key (empty=local): %s_", m.piBuf)
	case 5:
		return fmt.Sprintf(" context tokens [%d]: %s_", m.piDraft.ContextWindow, m.piBuf)
	case 6:
		return fmt.Sprintf(" max output tokens [%d]: %s_", m.piDraft.MaxTokens, m.piBuf)
	case 7:
		return fmt.Sprintf(" temperature (empty=omit): %s_", m.piBuf)
	case 8:
		return fmt.Sprintf(" thinking y/n [n]: %s_", m.piBuf)
	default:
		return " pi model"
	}
}

func (m Model) piModelKey(msg tea.KeyMsg) (tea.Model, tea.Cmd) {
	switch msg.Type {
	case tea.KeyEsc:
		m.mode = ""
		m.piBuf = ""
		return m, nil
	case tea.KeyBackspace:
		if len(m.piBuf) > 0 {
			m.piBuf = m.piBuf[:len(m.piBuf)-1]
		}
		return m, nil
	case tea.KeyEnter:
		return m.piModelAdvance()
	}
	if m.piStep == 0 {
		s := msg.String()
		if s == "1" || s == "2" || s == "3" {
			m.piBuf = s
			return m.piModelAdvance()
		}
	}
	if msg.Type == tea.KeyRunes {
		m.piBuf += string(msg.Runes)
	}
	return m, nil
}

func (m Model) piModelAdvance() (tea.Model, tea.Cmd) {
	v := strings.TrimSpace(m.piBuf)
	switch m.piStep {
	case 0:
		idx := pi.ParseInt(v, 0) - 1
		if idx < 0 || idx >= len(pi.Engines) {
			m.lastErr = "pick 1, 2, or 3"
			return m, nil
		}
		eng := pi.Engines[idx]
		m.piDraft.Engine = eng
		m.piDraft.Port = eng.Port
		m.piDraft.Provider = eng.Provider
	case 1:
		if v != "" {
			m.piDraft.Host = v
		}
	case 2:
		m.piDraft.Port = pi.ParseInt(v, m.piDraft.Port)
	case 3:
		if v == "" {
			m.lastErr = "model id required"
			return m, nil
		}
		m.piDraft.ModelID = v
	case 4:
		m.piDraft.APIKey = v
	case 5:
		m.piDraft.ContextWindow = pi.ParseInt(v, m.piDraft.ContextWindow)
	case 6:
		m.piDraft.MaxTokens = pi.ParseInt(v, m.piDraft.MaxTokens)
	case 7:
		if v != "" {
			m.piDraft.Temperature = pi.ParseFloat(v, 0)
		}
	case 8:
		m.piDraft.Thinking = strings.EqualFold(v, "y") || strings.EqualFold(v, "yes")
		return m.finishPiModel()
	}
	m.piStep++
	m.piBuf = ""
	m.lastErr = ""
	return m, nil
}

func (m Model) finishPiModel() (tea.Model, tea.Cmd) {
	m.mode = ""
	m.piBuf = ""
	name := m.piDraft.Engine.Provider
	if err := pi.Apply(pi.Path(), name, m.piDraft); err != nil {
		m.lastErr = err.Error()
		return m, nil
	}
	m.status = fmt.Sprintf("wrote %s %s → %s", name, m.piDraft.ModelID, m.piDraft.BaseURL())
	if focused := m.focusedName(); focused != "" {
		if err := m.srv.Restart(focused); err != nil {
			m.lastErr = "saved, reload: " + err.Error()
		} else {
			m.status += "  reloaded " + focused
			m.applyPTYs(m.paneSize())
		}
	}
	return m, nil
}
