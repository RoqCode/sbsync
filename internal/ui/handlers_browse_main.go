package ui

import (
	tea "github.com/charmbracelet/bubbletea"
)

// handleBrowseListKey handles all key events in the browse list state.
// This function now delegates to specialized handlers in other files.
func (m Model) handleBrowseListKey(msg tea.KeyMsg) (Model, tea.Cmd) {
	key := msg.String()

	// Handle input modes first
	if m.filter.prefixing {
		return m.handlePrefixFilterInput(msg)
	}

	if m.search.searching {
		return m.handleSearchInput(msg)
	}

	// Handle search and filter controls
	if newM, cmd := m.handleBrowseSearchAndFilterControls(key); cmd != nil || key == "f" || key == "F" || key == "p" || key == "P" || key == "c" {
		return newM, cmd
	}

	// Handle other key bindings
	switch key {
	case "ctrl+c", "q":
		return m, tea.Quit

	// Tree navigation
	case "l", "h", "H", "L":
		return m.handleBrowseTreeNavigation(key)

	// Cursor movement
	case "j", "down", "k", "up", "ctrl+d", "pgdown", "ctrl+u", "pgup":
		return m.handleBrowseCursorMovement(key)

	// Selection
	case " ":
		return m.handleBrowseSelection()

	// Actions
	case "r", "s":
		return m.handleBrowseActions(key)

	// Go back to mode picker
	case "m":
		m.state = stateModePicker
		m.statusMsg = "Zurück zur Modus-Auswahl."
		return m, nil
	}

	return m, nil
}
