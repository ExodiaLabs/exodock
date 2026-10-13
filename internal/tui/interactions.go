package tui

import (
	tea "charm.land/bubbletea/v2"
	"github.com/exodialabs/exodock/internal/core"
)

// Text overlays consume shortcut letters instead of dispatching workspace actions.
func (m *Model) inputKey(msg tea.KeyPressMsg) tea.Cmd {
	key := msg.String()
	live := m.inputMode == "global" || m.inputMode == "palette"
	switch key {
	case "esc":
		m.overlay, m.input = "", ""
	case "enter":
		switch m.inputMode {
		case "global":
			return m.openGlobalSelection()
		case "palette":
			return m.openPaletteSelection()
		case "filter":
			m.overlay = ""
			m.filter, m.selected = m.input, 0
			return m.resetSelection()
		case "logs":
			m.overlay = ""
			m.logQuery = m.input
		}
	case "up", "down", "tab", "shift+tab":
		if live {
			count := len(m.globalResults())
			if m.inputMode == "palette" {
				count = len(m.paletteResults())
			}
			delta := 1
			if key == "up" || key == "shift+tab" {
				delta = -1
			}
			m.menuIndex = max(0, min(max(0, count-1), m.menuIndex+delta))
		}
	case "backspace":
		r := []rune(m.input)
		if len(r) > 0 {
			m.input = string(r[:len(r)-1])
		}
		m.menuIndex = 0
	default:
		if msg.Text != "" {
			m.input = boundedInput(m.input + msg.Text)
			m.menuIndex = 0
		}
	}
	return nil
}

func (m *Model) openGlobalSelection() tea.Cmd {
	results := m.globalResults()
	if len(results) == 0 {
		return nil
	}
	r := results[max(0, min(m.menuIndex, len(results)-1))]
	m.overlay = ""
	for i, kind := range []core.Kind{core.Containers, core.Images, core.Volumes, core.Networks, core.Compose} {
		if kind == r.Kind {
			m.section = i + 1
			break
		}
	}
	m.filter, m.statusFilter = "", ""
	m.selected = 0
	for i, item := range m.resources() {
		if item.ID == r.ID {
			m.selected = i
			break
		}
	}
	m.detailOpen, m.focus, m.tab = true, 2, "Summary"
	return m.resetSelection()
}

func (m *Model) openPaletteSelection() tea.Cmd {
	results := m.paletteResults()
	if len(results) == 0 {
		return nil
	}
	entry := results[max(0, min(m.menuIndex, len(results)-1))]
	m.overlay = ""
	if entry.section >= 0 {
		return m.switchSection(entry.section)
	}
	return m.key(tea.KeyPressMsg{Code: rune(entry.key[0]), Text: entry.key})
}
