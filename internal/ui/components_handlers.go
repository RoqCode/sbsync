package ui

import (
	tea "github.com/charmbracelet/bubbletea"
	"time"
)

func (m Model) handleCompListKey(msg tea.KeyMsg) (Model, tea.Cmd) {
	key := msg.String()
	switch key {
	case "q":
		return m, tea.Quit
	case "m":
		m.state = stateModePicker
		m.statusMsg = "Zurück zur Modus-Auswahl."
		return m, nil
	case "j", "down":
		m.comp.listIndex++
		lines := m.visibleCompLines()
		if m.comp.listIndex >= len(lines) {
			m.comp.listIndex = len(lines) - 1
		}
		m.ensureCompCursorVisible()
		m.updateCompBrowseViewport()
		return m, nil
	case "k", "up":
		m.comp.listIndex--
		if m.comp.listIndex < 0 {
			m.comp.listIndex = 0
		}
		m.ensureCompCursorVisible()
		m.updateCompBrowseViewport()
		return m, nil
	case "ctrl+d", "pgdown":
		jump := m.viewport.Height
		if jump <= 0 {
			jump = 10
		}
		m.comp.listIndex += jump
		lines := m.visibleCompLines()
		if m.comp.listIndex > len(lines)-1 {
			m.comp.listIndex = len(lines) - 1
		}
		m.ensureCompCursorVisible()
		m.updateCompBrowseViewport()
		return m, nil
	case "ctrl+u", "pgup":
		jump := m.viewport.Height
		if jump <= 0 {
			jump = 10
		}
		m.comp.listIndex -= jump
		if m.comp.listIndex < 0 {
			m.comp.listIndex = 0
		}
		m.ensureCompCursorVisible()
		m.updateCompBrowseViewport()
		return m, nil
	// h/l were used for group collapse in tree view; no-op in flat list
	case " ":
		// toggle selection and move cursor down by one (like Stories list)
		model := m.visibleCompModel()
		if m.comp.listIndex >= 0 && m.comp.listIndex < len(model) {
			nm := model[m.comp.listIndex].name
			if nm != "" {
				if m.comp.selected[nm] {
					delete(m.comp.selected, nm)
				} else {
					m.comp.selected[nm] = true
				}
			}
			// Move cursor down
			if m.comp.listIndex < len(model)-1 {
				m.comp.listIndex++
			}
		}
		m.ensureCompCursorVisible()
		m.updateCompBrowseViewport()
		return m, nil
	case "f":
		// Start/stop lightweight search (no input component yet)
		if m.comp.search.searching {
			m.comp.search.searching = false
			m.comp.search.query = ""
		} else {
			m.comp.search.searching = true
			// no input widget: use status message as prompt for now
			m.statusMsg = "Suche aktiv – tippe /<text> in späterer Iteration"
		}
		m.updateCompBrowseViewport()
		return m, nil
	case "t":
		// cycle sort key
		m.comp.sortKey = (m.comp.sortKey + 1) % 3
		m.updateCompBrowseViewport()
		return m, nil
	case "o":
		m.comp.sortAsc = !m.comp.sortAsc
		m.updateCompBrowseViewport()
		return m, nil
	case "d":
		// toggle a simple date cutoff of today for now; later accept input
		if m.comp.dateCutoff.IsZero() {
			m.comp.dateCutoff = nowMidnight()
		} else {
			m.comp.dateCutoff = timeZero()
		}
		// reset cursor to top after large filter change
		m.comp.listIndex = 0
		m.ensureCompCursorVisible()
		m.updateCompBrowseViewport()
		return m, nil
	}
	return m, nil
}

// toggleCurrentGroupCollapse is a no-op in flat list mode (kept for compatibility)
func (m *Model) toggleCurrentGroupCollapse(_ bool) { return }

func nowMidnight() time.Time {
	t := time.Now()
	return time.Date(t.Year(), t.Month(), t.Day(), 0, 0, 0, 0, t.Location())
}
func timeZero() time.Time { return time.Time{} }
