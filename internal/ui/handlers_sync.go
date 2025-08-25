package ui

import (
	tea "github.com/charmbracelet/bubbletea"
)

func (m Model) handleSyncKey(key string) (tea.Model, tea.Cmd) {
	switch key {
	case "ctrl+c":
		if m.syncCancel != nil {
			m.syncCancel() // Cancel the sync context
		}
		m.syncing = false
		m.statusMsg = "Synchronisation abgebrochen"
		return m, nil
	}

	// During sync, most keys are ignored to prevent user interference
	// The sync process is controlled by commands, not key presses
	return m, nil
}
