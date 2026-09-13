package tui

import (
	"fmt"
	"path/filepath"
	"strings"

	tea "github.com/charmbracelet/bubbletea"
	"github.com/richard-ginsberg/fleeting/internal/pi"
)

func (m Model) modelKind() string {
	if s := m.focused(); s != nil {
		h := strings.ToLower(s.Harness)
		if h == "omp" || h == "pi" {
			return h
		}
		if strings.Contains(strings.ToLower(s.Lane), "omp") {
			return "omp"
		}
		if strings.Contains(strings.ToLower(s.Lane), "pi") {
			return "pi"
		}
	}
	gidx := m.visualToGlobal(m.focus)
	if c, ok := m.extras[gidx]; ok {
		h := strings.ToLower(c.Harness)
		if h == "omp" || h == "pi" {
			return h
		}
		for _, p := range c.Cmd {
			base := strings.ToLower(filepath.Base(p))
			if base == "omp" {
				return "omp"
			}
			if base == "pi" {
				return "pi"
			}
		}
	}
	return "pi"
}

func (m Model) openPiModel() (tea.Model, tea.Cmd) {
	m.mode = "pimodel"
	m.piKind = m.modelKind()
	m.piBuf = ""
	m.piDraft = pi.Draft{Host: "127.0.0.1", Path: "/v1", ContextWindow: 32768, MaxTokens: 8192}
	m.piIDs = nil
	m.piPage = 0
	m.piEntries = nil
	m.lastErr = ""
	ents, err := pi.ListEntries(pi.ConfigPath(m.piKind))
	if err == nil && len(ents) > 0 {
		m.piEntries = ents
		m.piStep = -1
	} else {
		m.piStep = 0
	}
	return m, nil
}

func (m Model) piLabel() string {
	if m.piKind == "omp" {
		return "omp"
	}
	return "pi"
}

func (m Model) piModelPrompt() string {
	tag := m.piLabel()
	switch m.piStep {
	case -1:
		return fmt.Sprintf(" %s models: 1 add  2 edit  Esc", tag)
	case -2:
		return m.piExistingPrompt()
	case 0:
		return fmt.Sprintf(" %s model engine: 1 llama.cpp  2 vLLM  3 SGLang  Esc", tag)
	case 1:
		return fmt.Sprintf(" %s host [%s]: %s_", tag, m.piDraft.Host, m.piBuf)
	case 2:
		return fmt.Sprintf(" %s port [%d]: %s_", tag, m.piDraft.Port, m.piBuf)
	case 3:
		if len(m.piIDs) > 0 {
			return m.piModelListPrompt()
		}
		return fmt.Sprintf(" %s model id (Tab = probe %s/models): %s_", tag, m.piDraft.BaseURL(), m.piBuf)
	case 4:
		return fmt.Sprintf(" %s api key (empty=local): %s_", tag, m.piBuf)
	case 5:
		return fmt.Sprintf(" %s context tokens [%d]: %s_", tag, m.piDraft.ContextWindow, m.piBuf)
	case 6:
		return fmt.Sprintf(" %s max output tokens [%d]: %s_", tag, m.piDraft.MaxTokens, m.piBuf)
	case 7:
		return fmt.Sprintf(" %s temperature (empty=omit): %s_", tag, m.piBuf)
	case 8:
		return fmt.Sprintf(" %s thinking y/n [n]: %s_", tag, m.piBuf)
	default:
		return " " + tag + " model"
	}
}

func (m Model) piExistingPrompt() string {
	const pageSize = 8
	start := m.piPage * pageSize
	if start >= len(m.piEntries) {
		start = 0
	}
	end := start + pageSize
	if end > len(m.piEntries) {
		end = len(m.piEntries)
	}
	var b strings.Builder
	fmt.Fprintf(&b, " %s edit", m.piLabel())
	for i, e := range m.piEntries[start:end] {
		fmt.Fprintf(&b, "  %d %s/%s", i+1, e.Provider, e.ModelID)
	}
	if end < len(m.piEntries) {
		b.WriteString("  Tab more")
	}
	b.WriteString("  Esc")
	return b.String()
}

func (m Model) piModelListPrompt() string {
	const pageSize = 8
	start := m.piPage * pageSize
	if start >= len(m.piIDs) {
		start = 0
	}
	end := start + pageSize
	if end > len(m.piIDs) {
		end = len(m.piIDs)
	}
	var b strings.Builder
	fmt.Fprintf(&b, " %s models", m.piLabel())
	for i, id := range m.piIDs[start:end] {
		fmt.Fprintf(&b, "  %d %s", i+1, id)
	}
	if end < len(m.piIDs) {
		b.WriteString("  Tab more")
	}
	b.WriteString("  Esc type")
	return b.String()
}

func (m Model) piModelKey(msg tea.KeyMsg) (tea.Model, tea.Cmd) {
	if m.piStep == -2 && len(m.piEntries) > 0 {
		return m.piExistingPickKey(msg)
	}
	if m.piStep == 3 && len(m.piIDs) > 0 {
		return m.piModelPickKey(msg)
	}
	switch msg.Type {
	case tea.KeyEsc:
		m.mode = ""
		m.piBuf = ""
		m.piIDs = nil
		m.piEntries = nil
		return m, nil
	case tea.KeyTab:
		if m.piStep == 3 {
			return m.piModelProbe()
		}
		return m, nil
	case tea.KeyBackspace:
		if len(m.piBuf) > 0 {
			m.piBuf = m.piBuf[:len(m.piBuf)-1]
		}
		return m, nil
	case tea.KeyEnter:
		return m.piModelAdvance()
	}
	if m.piStep == -1 || m.piStep == 0 {
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

func (m Model) piExistingPickKey(msg tea.KeyMsg) (tea.Model, tea.Cmd) {
	const pageSize = 8
	switch msg.Type {
	case tea.KeyEsc:
		m.piStep = -1
		m.piPage = 0
		m.piBuf = ""
		return m, nil
	case tea.KeyTab:
		pages := (len(m.piEntries) + pageSize - 1) / pageSize
		if pages < 1 {
			pages = 1
		}
		m.piPage = (m.piPage + 1) % pages
		return m, nil
	}
	s := msg.String()
	if len(s) == 1 && s[0] >= '1' && s[0] <= '8' {
		i := int(s[0]-'1') + m.piPage*pageSize
		if i >= 0 && i < len(m.piEntries) {
			return m.loadExisting(m.piEntries[i])
		}
	}
	return m, nil
}

func (m Model) loadExisting(e pi.Entry) (tea.Model, tea.Cmd) {
	d, ok := pi.LoadEntry(pi.ConfigPath(m.piKind), e.Provider, e.ModelID)
	if !ok {
		m.lastErr = "could not load " + e.Provider + "/" + e.ModelID
		return m, nil
	}
	m.piDraft = d
	m.piStep = 1
	m.piBuf = ""
	m.piPage = 0
	m.lastErr = ""
	return m, nil
}

func (m Model) piModelProbe() (tea.Model, tea.Cmd) {
	ids, err := pi.ListModels(m.piDraft.BaseURL(), m.piDraft.APIKey)
	if err != nil {
		m.lastErr = err.Error()
		return m, nil
	}
	m.piIDs = ids
	m.piPage = 0
	m.piBuf = ""
	m.lastErr = ""
	m.status = fmt.Sprintf("%d models at %s/models", len(ids), m.piDraft.BaseURL())
	return m, nil
}

func (m Model) piModelPickKey(msg tea.KeyMsg) (tea.Model, tea.Cmd) {
	const pageSize = 8
	switch msg.Type {
	case tea.KeyEsc:
		m.piIDs = nil
		m.piPage = 0
		m.piBuf = ""
		return m, nil
	case tea.KeyTab:
		pages := (len(m.piIDs) + pageSize - 1) / pageSize
		if pages < 1 {
			pages = 1
		}
		m.piPage = (m.piPage + 1) % pages
		return m, nil
	}
	s := msg.String()
	if len(s) == 1 && s[0] >= '1' && s[0] <= '8' {
		i := int(s[0]-'1') + m.piPage*pageSize
		if i >= 0 && i < len(m.piIDs) {
			m.piBuf = m.piIDs[i]
			m.piIDs = nil
			return m.piModelAdvance()
		}
	}
	return m, nil
}

func (m Model) piModelAdvance() (tea.Model, tea.Cmd) {
	v := strings.TrimSpace(m.piBuf)
	switch m.piStep {
	case -1:
		if v == "2" {
			m.piStep = -2
			m.piPage = 0
			m.piBuf = ""
			return m, nil
		}
		m.piStep = 0
		m.piBuf = ""
		m.piDraft = pi.Draft{Host: "127.0.0.1", Path: "/v1", ContextWindow: 32768, MaxTokens: 8192}
		return m, nil
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
	m.piIDs = nil
	m.lastErr = ""
	return m, nil
}

func (m Model) finishPiModel() (tea.Model, tea.Cmd) {
	m.mode = ""
	m.piBuf = ""
	name := m.piDraft.Engine.Provider
	if name == "" {
		name = m.piDraft.Provider
	}
	if err := pi.ApplyKind(m.piKind, name, m.piDraft); err != nil {
		m.lastErr = err.Error()
		return m, nil
	}
	m.status = fmt.Sprintf("wrote %s %s %s → %s", m.piLabel(), name, m.piDraft.ModelID, m.piDraft.BaseURL())
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
