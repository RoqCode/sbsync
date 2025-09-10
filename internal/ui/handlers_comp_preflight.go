package ui

import (
	tea "github.com/charmbracelet/bubbletea"
	"strings"
)

func (m Model) handleCompPreflightKey(msg tea.KeyMsg) (Model, tea.Cmd) {
	key := msg.String()
	if len(m.compPre.items) == 0 {
		if key == "esc" || key == "b" {
			m.state = stateCompList
		}
		return m, nil
	}
	// Inline rename mode
	if m.compPre.renaming {
		var cmd tea.Cmd
		m.compPre.input, cmd = m.compPre.input.Update(msg)
		switch key {
		case "enter":
			val := strings.TrimSpace(m.compPre.input.Value())
			if val != "" {
				i := m.compPre.listIndex
				if i >= 0 && i < len(m.compPre.items) {
					it := &m.compPre.items[i]
					it.CopyAsNew = true
					it.ForkName = val
					it.State = StateCreate
				}
			}
			m.compPre.renaming = false
			m.updateCompPreflightViewport()
			return m, cmd
		case "esc":
			m.compPre.renaming = false
			m.updateCompPreflightViewport()
			return m, cmd
		}
		// live update while typing
		m.updateCompPreflightViewport()
		return m, cmd
	}
	switch key {
	case "u":
		// Toggle force update for unchanged items
		m.compPre.forceUpdateAll = !m.compPre.forceUpdateAll
		for i := range m.compPre.items {
			it := &m.compPre.items[i]
			if strings.ToLower(it.Issue) == "no changes" {
				if m.compPre.forceUpdateAll {
					it.State = StateUpdate
					it.Skip = false
					it.CopyAsNew = false
				} else {
					it.State = StateSkip
					it.Skip = true
					it.CopyAsNew = false
				}
			}
		}
		m.updateCompPreflightViewport()
		return m, nil
	case "j", "down":
		if m.compPre.listIndex < len(m.compPre.items)-1 {
			m.compPre.listIndex++
		}
		m.updateCompPreflightViewport()
		return m, nil
	case "k", "up":
		if m.compPre.listIndex > 0 {
			m.compPre.listIndex--
		}
		m.updateCompPreflightViewport()
		return m, nil
	case "b", "esc":
		m.state = stateCompList
		return m, nil
	case "enter":
		// Start concurrent apply with live progress
		return m, m.startCompApply()
	case " ":
		// cycle Skip -> Apply -> Fork -> Apply ...
		i := m.compPre.listIndex
		it := &m.compPre.items[i]
		switch it.State {
		case StateSkip:
			if it.Collision {
				it.State = StateUpdate
			} else {
				it.State = StateCreate
			}
			it.Skip = false
			it.CopyAsNew = false
		default:
			// Move to Skip
			it.State = StateSkip
			it.Skip = true
			it.CopyAsNew = false
		}
		m.updateCompPreflightViewport()
		return m, nil
	case "f":
		// fork/rename
		i := m.compPre.listIndex
		it := &m.compPre.items[i]
		it.State = StateCreate
		it.Skip = false
		it.CopyAsNew = true
		// start rename input prefilled
		m.compPre.input.SetValue(it.ForkName)
		if it.ForkName == "" {
			m.compPre.input.SetValue(it.Source.Name + "-copy")
		}
		m.compPre.input.Focus()
		m.compPre.renaming = true
		m.updateCompPreflightViewport()
		return m, nil
	}
	return m, nil
}
