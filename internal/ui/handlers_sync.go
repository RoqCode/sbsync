package ui

import (
	tea "github.com/charmbracelet/bubbletea"
)

func (m Model) handleSyncKey(key string) (tea.Model, tea.Cmd) {
	// During sync, most keys are ignored to prevent user interference
	// The sync process is controlled by commands, not key presses
	// ctrl+c is handled globally in update_main.go
	return m, nil
}
