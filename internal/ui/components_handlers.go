package ui

import (
	"github.com/charmbracelet/bubbles/textinput"
	tea "github.com/charmbracelet/bubbletea"
	"strings"
	"time"
)

func (m Model) handleCompListKey(msg tea.KeyMsg) (Model, tea.Cmd) {
	key := msg.String()
	// If in input mode, delegate to input handlers
	if m.comp.inputMode == "search" {
		// Update search input
		var cmd tea.Cmd
		m.comp.search.input, cmd = m.comp.search.input.Update(msg)
		switch key {
		case "enter":
			m.comp.search.query = strings.TrimSpace(m.comp.search.input.Value())
			m.comp.inputMode = ""
			m.comp.listIndex = 0
			m.ensureCompCursorVisible()
			m.updateCompBrowseViewport()
			return m, cmd
		case "esc":
			// cancel editing; keep previous query
			m.comp.search.input.SetValue(m.comp.search.query)
			m.comp.inputMode = ""
			m.updateCompBrowseViewport()
			return m, cmd
		}
		// keep viewport content same; header shows input view
		return m, cmd
	}
	if m.comp.inputMode == "date" {
		var cmd tea.Cmd
		m.comp.dateInput, cmd = m.comp.dateInput.Update(msg)
		switch key {
		case "enter":
			val := strings.TrimSpace(m.comp.dateInput.Value())
			if val == "" {
				m.comp.dateCutoff = timeZero()
				m.comp.inputMode = ""
			} else {
				if t := parseTime(val); !t.IsZero() {
					m.comp.dateCutoff = t
					m.comp.inputMode = ""
				} else {
					m.statusMsg = "Ungültiges Datum. Format: YYYY-MM-DD"
					return m, cmd
				}
			}
			m.comp.listIndex = 0
			m.ensureCompCursorVisible()
			m.updateCompBrowseViewport()
			return m, cmd
		case "esc":
			// cancel editing
			m.comp.inputMode = ""
			m.updateCompBrowseViewport()
			return m, cmd
		}
		return m, cmd
	}
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
		// Enter search input mode
		m.comp.inputMode = "search"
		m.comp.search.input.SetValue(m.comp.search.query)
		m.comp.search.input.Focus()
		m.updateCompBrowseViewport()
		return m, nil
	case "F":
		// Clear search
		m.comp.search.searching = false
		m.comp.search.query = ""
		m.comp.search.input.SetValue("")
		m.comp.inputMode = ""
		m.comp.listIndex = 0
		m.ensureCompCursorVisible()
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
		// Enter date input mode
		m.comp.inputMode = "date"
		if !m.comp.dateCutoff.IsZero() {
			m.comp.dateInput.SetValue(m.comp.dateCutoff.Format("2006-01-02"))
		} else {
			m.comp.dateInput.SetValue("")
		}
		m.comp.dateInput.Focus()
		m.updateCompBrowseViewport()
		return m, nil
	case "D":
		// Clear date cutoff
		m.comp.dateCutoff = timeZero()
		m.comp.inputMode = ""
		m.comp.dateInput.SetValue("")
		m.comp.listIndex = 0
		m.ensureCompCursorVisible()
		m.updateCompBrowseViewport()
		return m, nil
	case "s":
		// Build preflight from selected components and enter preflight view
		m.startCompPreflight()
		m.state = stateCompPreflight
		// init rename input
		m.compPre.input = textinput.New()
		m.compPre.input.CharLimit = 200
		m.compPre.input.Width = 40
		m.updateViewportContent()
		return m, nil
	}
	return m, nil
}

func timeZero() time.Time { return time.Time{} }
